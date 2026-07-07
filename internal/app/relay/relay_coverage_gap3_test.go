package relay

import (
	"bytes"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	relaycommon "github.com/LurusTech/lurus-hub/internal/adapter/provider/common"
	relayconstant "github.com/LurusTech/lurus-hub/internal/adapter/provider/constant"
	"github.com/LurusTech/lurus-hub/internal/adapter/repo"
	"github.com/LurusTech/lurus-hub/internal/app"
	"github.com/LurusTech/lurus-hub/internal/pkg/common"
	"github.com/LurusTech/lurus-hub/internal/pkg/constant"
	"github.com/LurusTech/lurus-hub/internal/pkg/setting/system_setting"

	"github.com/gin-gonic/gin"
)

// TestVideoFetchByIDRespBodyBuilder_GeminiEnrichment drives the Vertex/Gemini
// enrichment closure inside videoFetchByIDRespBodyBuilder: a Gemini video task
// is fetched from a fake upstream operation endpoint, the parsed status is
// written back to the task, and (for a non /v1/videos/ request URI) a
// standardized video response body is built. This is the lowest-covered
// reachable function; the closure is otherwise skipped for non-Gemini channels.
func TestVideoFetchByIDRespBodyBuilder_GeminiEnrichment(t *testing.T) {
	cleanup := setupRelayDB(t)
	defer cleanup()
	app.InitHttpClient()

	// Allow the loopback fake upstream through SSRF protection for this test.
	fs := system_setting.GetFetchSetting()
	prevSSRF, prevPriv := fs.EnableSSRFProtection, fs.AllowPrivateIp
	fs.EnableSSRFProtection = false
	fs.AllowPrivateIp = true
	defer func() { fs.EnableSSRFProtection = prevSSRF; fs.AllowPrivateIp = prevPriv }()

	// The upstream returns a completed (done) Gemini video operation.
	var sawKey string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawKey = r.Header.Get("x-goog-api-key")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"name":"models/veo/operations/abc","done":true,"response":{"videos":[{"mimeType":"video/mp4"}]}}`))
	}))
	defer srv.Close()

	baseURL := srv.URL
	ch := &repo.Channel{
		Type:    constant.ChannelTypeGemini,
		Status:  common.ChannelStatusEnabled,
		BaseURL: &baseURL,
		Key:     "gm-key",
	}
	if err := repo.DB.Create(ch).Error; err != nil {
		t.Fatalf("seed channel: %v", err)
	}

	// TaskID is the base64url-encoded upstream operation name (Gemini contract).
	localTaskID := base64.RawURLEncoding.EncodeToString([]byte("models/veo/operations/abc"))
	geminiPlatform := constant.TaskPlatform(strconv.Itoa(constant.ChannelTypeGemini))
	seedTask(t, &repo.Task{
		TaskID:    localTaskID,
		UserId:    321,
		ChannelId: ch.Id,
		Platform:  geminiPlatform,
		Action:    "video",
		Status:    repo.TaskStatusInProgress,
	})

	// Request URI must NOT be under /v1/videos/ to hit the standardized-response
	// build branch of the enrichment closure.
	c, _ := newJSONContext(http.MethodGet, "/api/task/video/"+localTaskID, nil)
	c.Set("id", 321)
	c.Params = gin.Params{{Key: "task_id", Value: localTaskID}}

	body, taskErr := videoFetchByIDRespBodyBuilder(c)
	if taskErr != nil {
		t.Fatalf("unexpected taskErr: %+v", taskErr)
	}
	if sawKey != "gm-key" {
		t.Errorf("upstream x-goog-api-key = %q, want gm-key", sawKey)
	}
	// The enrichment closure builds a standardized response with the succeeded
	// status derived from the upstream done=true operation.
	if !bytes.Contains(body, []byte("succeeded")) {
		t.Errorf("expected succeeded status in enriched body, got: %s", body)
	}
	// The task status must have been persisted back as success.
	saved, exist, err := repo.GetByTaskId(321, localTaskID)
	if err != nil || !exist {
		t.Fatalf("reload task: exist=%v err=%v", exist, err)
	}
	if saved.Status != repo.TaskStatusSuccess {
		t.Errorf("persisted task status = %q, want success", saved.Status)
	}
}

// TestRelayTaskSubmit_OriginTaskRebindsChannel drives the origin-task resolution
// branch of RelayTaskSubmit: a submit that references an existing task on a
// DIFFERENT channel must look up the origin task, rebind the outgoing channel
// (base URL + key + type) to the origin task's channel, and dispatch there. It
// proves the async task submit continues end-to-end against the rebound channel.
func TestRelayTaskSubmit_OriginTaskRebindsChannel(t *testing.T) {
	cleanup := setupRelayDB(t)
	defer cleanup()
	app.InitHttpClient()

	u := &repo.User{Username: "suno-origin", Quota: 100_000_000}
	if err := repo.DB.Create(u).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}

	// The origin task's channel is the one the submit must rebind onto.
	var sawKey string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawKey = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"code":"success","message":"","data":"suno-task-rebound"}`))
	}))
	defer srv.Close()

	baseURL := srv.URL
	originChannel := &repo.Channel{
		Type:    constant.ChannelTypeOpenAI,
		Status:  common.ChannelStatusEnabled,
		BaseURL: &baseURL,
		Key:     "sk-origin-suno",
	}
	if err := repo.DB.Create(originChannel).Error; err != nil {
		t.Fatalf("seed origin channel: %v", err)
	}

	seedTask(t, &repo.Task{
		TaskID:    "origin-suno-1",
		UserId:    u.Id,
		ChannelId: originChannel.Id,
		Platform:  constant.TaskPlatformSuno,
		Action:    "music",
		Status:    repo.TaskStatusSuccess,
		Data:      []byte(`{"model":"suno-v3"}`),
	})

	c, _ := taskSubmitContext(t, http.MethodPost, "/suno/submit/music", "music", []byte(`{"prompt":"lofi remix"}`))
	c.Set("platform", string(constant.TaskPlatformSuno))
	c.Set("token_name", "tkn")
	// Deliberately point the incoming channel elsewhere so the origin-task branch
	// must rebind onto the origin channel above.
	common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, "http://127.0.0.1:1/should-not-be-used")
	common.SetContextKey(c, constant.ContextKeyChannelKey, "sk-wrong")

	info := &relaycommon.RelayInfo{
		UserId:       u.Id,
		IsPlayground: true,
	}
	// OriginTaskID lives on the embedded *TaskRelayInfo, which RelayTaskSubmit
	// leaves intact when already set.
	info.TaskRelayInfo = &relaycommon.TaskRelayInfo{OriginTaskID: "origin-suno-1"}

	if taskErr := RelayTaskSubmit(c, info); taskErr != nil {
		t.Fatalf("origin-rebind submit failed: %+v", taskErr)
	}
	// The upstream that actually served the request must be the origin channel
	// (its key was forwarded), proving the rebind took effect.
	if sawKey == "" || !bytes.Contains([]byte(sawKey), []byte("sk-origin-suno")) {
		t.Errorf("upstream Authorization = %q, want origin channel key sk-origin-suno", sawKey)
	}
	saved, exist, err := repo.GetByTaskId(u.Id, "suno-task-rebound")
	if err != nil || !exist {
		t.Fatalf("expected persisted rebound task: exist=%v err=%v", exist, err)
	}
	if saved.ChannelId != originChannel.Id {
		t.Errorf("persisted task channel = %d, want origin channel %d", saved.ChannelId, originChannel.Id)
	}
}

// TestRelayTaskSubmit_VideoRemixResolvesVideoID covers the /v1/videos/{id}/remix
// detection branch: the remix action is inferred from the path, the video_id is
// extracted as the origin task id, and the origin-task lookup runs (here the
// origin task is absent, so it is rejected as task_not_exist — proving the path
// was taken).
func TestRelayTaskSubmit_VideoRemixResolvesVideoID(t *testing.T) {
	cleanup := setupRelayDB(t)
	defer cleanup()

	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos/vid-remix-1/remix", bytes.NewReader([]byte(`{}`)))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "video_id", Value: "vid-remix-1"}}

	info := &relaycommon.RelayInfo{UserId: 501}
	taskErr := RelayTaskSubmit(c, info)
	if taskErr == nil {
		t.Fatalf("expected task_not_exist error for missing remix origin, got nil")
	}
	if info.OriginTaskID != "vid-remix-1" {
		t.Errorf("OriginTaskID = %q, want vid-remix-1 (extracted from path)", info.OriginTaskID)
	}
	if info.Action != constant.TaskActionRemix {
		t.Errorf("Action = %q, want remix (inferred from /remix path)", info.Action)
	}
}

// TestRelayTaskSubmit_OriginChannelDisabled covers the branch where the origin
// task's channel is disabled: the submit is rejected before any upstream call.
func TestRelayTaskSubmit_OriginChannelDisabled(t *testing.T) {
	cleanup := setupRelayDB(t)
	defer cleanup()

	u := &repo.User{Username: "suno-disabled", Quota: 100_000_000}
	if err := repo.DB.Create(u).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	disabledURL := "http://127.0.0.1:1"
	ch := &repo.Channel{Type: constant.ChannelTypeOpenAI, Status: common.ChannelStatusManuallyDisabled, BaseURL: &disabledURL, Key: "k"}
	if err := repo.DB.Create(ch).Error; err != nil {
		t.Fatalf("seed channel: %v", err)
	}
	seedTask(t, &repo.Task{TaskID: "od-1", UserId: u.Id, ChannelId: ch.Id, Platform: constant.TaskPlatformSuno, Action: "music", Status: repo.TaskStatusSuccess})

	c, _ := taskSubmitContext(t, http.MethodPost, "/suno/submit/music", "music", []byte(`{"prompt":"x"}`))
	c.Set("platform", string(constant.TaskPlatformSuno))
	info := &relaycommon.RelayInfo{UserId: u.Id}
	info.TaskRelayInfo = &relaycommon.TaskRelayInfo{OriginTaskID: "od-1"}

	taskErr := RelayTaskSubmit(c, info)
	if taskErr == nil {
		t.Fatalf("expected task_channel_disable error for disabled origin channel, got nil")
	}
}

// TestRelayTaskSubmit_VideoRemixComputesRatios covers the remix parameter/ratio
// computation branch: a /v1/videos/{id}/remix submit resolves the origin task,
// then derives the seconds/size pricing ratios from the origin task's stored
// parameters before dispatch. The observable contract is the populated
// PriceData.OtherRatios (seconds from the origin data, size bumped for the
// wide aspect ratio); the suno origin platform then rejects the remix action.
func TestRelayTaskSubmit_VideoRemixComputesRatios(t *testing.T) {
	cleanup := setupRelayDB(t)
	defer cleanup()
	app.InitHttpClient()

	u := &repo.User{Username: "remix-ratio", Quota: 100_000_000}
	if err := repo.DB.Create(u).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	base := "http://127.0.0.1:1"
	ch := &repo.Channel{Type: constant.ChannelTypeOpenAI, Status: common.ChannelStatusEnabled, BaseURL: &base, Key: "k-remix"}
	if err := repo.DB.Create(ch).Error; err != nil {
		t.Fatalf("seed channel: %v", err)
	}
	seedTask(t, &repo.Task{
		TaskID:    "orig-remix-1",
		UserId:    u.Id,
		ChannelId: ch.Id,
		Platform:  constant.TaskPlatformSuno,
		Action:    "music",
		Status:    repo.TaskStatusSuccess,
		Data:      []byte(`{"model":"suno-v3","seconds":"8","size":"1792x1024"}`),
	})

	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos/orig-remix-1/remix", bytes.NewReader([]byte(`{"prompt":"x"}`)))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "video_id", Value: "orig-remix-1"}}
	c.Set("token_name", "tkn")

	info := &relaycommon.RelayInfo{UserId: u.Id}
	taskErr := RelayTaskSubmit(c, info)

	// The remix ratio block must have populated the pricing ratios from the
	// origin task's stored seconds/size BEFORE the adaptor rejected the action.
	if info.PriceData.OtherRatios == nil {
		t.Fatalf("expected remix ratios computed, OtherRatios is nil (taskErr=%+v)", taskErr)
	}
	if got := info.PriceData.OtherRatios["seconds"]; got != 8 {
		t.Errorf("OtherRatios[seconds] = %v, want 8 (from origin data)", got)
	}
	if got := info.PriceData.OtherRatios["size"]; got < 1.6 {
		t.Errorf("OtherRatios[size] = %v, want ~1.666667 for wide aspect", got)
	}
}

// TestRelayTaskFetch_MusicByIDMode drives RelayTaskFetch through the music
// fetch-by-id builder and asserts the standardized body is copied to the client.
func TestRelayTaskFetch_MusicByIDMode(t *testing.T) {
	cleanup := setupRelayDB(t)
	defer cleanup()

	seedTask(t, &repo.Task{TaskID: "mf-1", UserId: 71, Action: "music", Status: repo.TaskStatusSuccess, FailReason: "https://cdn/song.mp3"})
	c, rec := newJSONContext(http.MethodGet, "/", nil)
	c.Set("id", 71)
	c.Params = gin.Params{{Key: "task_id", Value: "mf-1"}}

	taskErr := RelayTaskFetch(c, relayconstant.RelayModeMusicFetchByID)
	if taskErr != nil {
		t.Fatalf("unexpected taskErr: %+v", taskErr)
	}
	if !strings.Contains(rec.Body.String(), "completed") {
		t.Errorf("expected completed music status written, got: %s", rec.Body.String())
	}
}

// TestRelayTaskFetch_VideoByIDMode drives RelayTaskFetch through the video
// fetch-by-id builder for a non-Gemini channel (generic TaskDto fallback).
func TestRelayTaskFetch_VideoByIDMode(t *testing.T) {
	cleanup := setupRelayDB(t)
	defer cleanup()

	ch := &repo.Channel{Type: constant.ChannelTypeOpenAI}
	if err := repo.DB.Create(ch).Error; err != nil {
		t.Fatalf("seed channel: %v", err)
	}
	seedTask(t, &repo.Task{TaskID: "vf-1", UserId: 72, ChannelId: ch.Id, Action: "video", Status: repo.TaskStatusSuccess})
	c, rec := newJSONContext(http.MethodGet, "/", nil)
	c.Set("id", 72)
	c.Params = gin.Params{{Key: "task_id", Value: "vf-1"}}

	taskErr := RelayTaskFetch(c, relayconstant.RelayModeVideoFetchByID)
	if taskErr != nil {
		t.Fatalf("unexpected taskErr: %+v", taskErr)
	}
	if !strings.Contains(rec.Body.String(), "vf-1") {
		t.Errorf("expected task id in fetched video body, got: %s", rec.Body.String())
	}
}

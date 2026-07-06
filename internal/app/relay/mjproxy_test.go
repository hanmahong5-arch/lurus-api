package relay

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	relaycommon "github.com/LurusTech/lurus-hub/internal/adapter/provider/common"
	relayconstant "github.com/LurusTech/lurus-hub/internal/adapter/provider/constant"
	"github.com/LurusTech/lurus-hub/internal/adapter/repo"
	"github.com/LurusTech/lurus-hub/internal/app"
	"github.com/LurusTech/lurus-hub/internal/pkg/common"
	"github.com/LurusTech/lurus-hub/internal/pkg/setting"

	"github.com/gin-gonic/gin"
)

// ─── getMjRequestPath ────────────────────────────────────────────────────────

func TestGetMjRequestPath(t *testing.T) {
	cases := []struct{ in, want string }{
		// No "/mj-" prefix segment → returned unchanged.
		{"/mj/submit/imagine", "/mj/submit/imagine"},
		// Contains "/mj-" and a "/mj/" split point → normalized to "/mj/<rest>".
		{"/relay/mj-fast/mj/submit/imagine", "/mj/submit/imagine"},
		// Contains "/mj-" but no "/mj/" split → returned unchanged.
		{"/relay/mj-fast/submit", "/relay/mj-fast/submit"},
	}
	for _, tc := range cases {
		if got := getMjRequestPath(tc.in); got != tc.want {
			t.Errorf("getMjRequestPath(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// ─── coverMidjourneyTaskDto ──────────────────────────────────────────────────

func mjGinContext(t *testing.T, body []byte) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	var r *http.Request
	if body == nil {
		r = httptest.NewRequest(http.MethodGet, "/mj/task", nil)
	} else {
		r = httptest.NewRequest(http.MethodPost, "/mj/task", bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
	}
	c.Request = r
	return c, rec
}

func TestCoverMidjourneyTaskDto_ParsesJSONFieldsAndForwardURL(t *testing.T) {
	c, _ := mjGinContext(t, nil)

	origin := &repo.Midjourney{
		MjId:       "mj-1",
		Progress:   "50%",
		Status:     "IN_PROGRESS",
		ImageUrl:   "https://cdn/orig.png",
		VideoUrl:   "https://cdn/orig.mp4",
		Action:     "IMAGINE",
		Prompt:     "a cat",
		Buttons:    `[{"customId":"btn1","label":"U1"}]`,
		VideoUrls:  `[{"url":"https://cdn/v1.mp4"}]`,
		Properties: `{"finalPrompt":"a cat, high detail"}`,
	}

	// Forward URL enabled + non-SUCCESS status → proxied URL with cache-buster.
	prev := setting.MjForwardUrlEnabled
	setting.MjForwardUrlEnabled = true
	defer func() { setting.MjForwardUrlEnabled = prev }()

	got := coverMidjourneyTaskDto(c, origin)
	if got.MjId != "mj-1" || got.Action != "IMAGINE" {
		t.Errorf("basic fields mismatch: %+v", got)
	}
	if got.Buttons == nil {
		t.Errorf("buttons not parsed: %+v", got.Buttons)
	}
	if len(got.VideoUrls) != 1 {
		t.Errorf("video urls not parsed: %+v", got.VideoUrls)
	}
	if got.Properties == nil {
		t.Errorf("properties not parsed")
	}
	// Forward URL rewrites ImageUrl to the /mj/image proxy path (non-empty).
	if got.ImageUrl == "https://cdn/orig.png" || got.ImageUrl == "" {
		t.Errorf("expected forwarded image url, got %q", got.ImageUrl)
	}

	// Forward URL disabled → original upstream image url passed through.
	setting.MjForwardUrlEnabled = false
	got2 := coverMidjourneyTaskDto(c, origin)
	if got2.ImageUrl != "https://cdn/orig.png" {
		t.Errorf("expected passthrough image url, got %q", got2.ImageUrl)
	}
}

// ─── RelayMidjourneyNotify ───────────────────────────────────────────────────

func TestRelayMidjourneyNotify(t *testing.T) {
	cleanup := setupRelayDB(t)
	defer cleanup()

	if err := repo.MjInsert(&repo.Midjourney{MjId: "notify-1", UserId: 1, Status: "IN_PROGRESS"}); err != nil {
		t.Fatalf("seed mj: %v", err)
	}

	// Bind error (malformed JSON).
	c1, _ := mjGinContext(t, []byte(`{bad`))
	if resp := RelayMidjourneyNotify(c1); resp == nil || resp.Code != 4 {
		t.Errorf("expected bind error response, got %+v", resp)
	}

	// Task not found.
	c2, _ := mjGinContext(t, []byte(`{"id":"does-not-exist"}`))
	if resp := RelayMidjourneyNotify(c2); resp == nil || resp.Description != "midjourney_task_not_found" {
		t.Errorf("expected not-found response, got %+v", resp)
	}

	// Success → nil response, task updated.
	c3, _ := mjGinContext(t, []byte(`{"id":"notify-1","status":"SUCCESS","progress":"100%","imageUrl":"https://cdn/done.png"}`))
	if resp := RelayMidjourneyNotify(c3); resp != nil {
		t.Errorf("expected nil on success, got %+v", resp)
	}
	updated := repo.GetByOnlyMJId("notify-1")
	if updated == nil || updated.Status != "SUCCESS" || updated.Progress != "100%" {
		t.Errorf("task not updated: %+v", updated)
	}
}

// ─── RelayMidjourneyTask ─────────────────────────────────────────────────────

func TestRelayMidjourneyTask_FetchAndCondition(t *testing.T) {
	cleanup := setupRelayDB(t)
	defer cleanup()

	if err := repo.MjInsert(&repo.Midjourney{MjId: "t-1", UserId: 5, Status: "SUCCESS", Progress: "100%"}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// Fetch by id — found.
	c, rec := mjGinContext(t, nil)
	c.Set("id", 5)
	c.Params = gin.Params{{Key: "id", Value: "t-1"}}
	if resp := RelayMidjourneyTask(c, relayconstant.RelayModeMidjourneyTaskFetch); resp != nil {
		t.Errorf("expected nil (success), got %+v", resp)
	}
	if rec.Body.Len() == 0 {
		t.Errorf("expected body written for fetch")
	}

	// Fetch by id — not found.
	c2, _ := mjGinContext(t, nil)
	c2.Set("id", 5)
	c2.Params = gin.Params{{Key: "id", Value: "missing"}}
	if resp := RelayMidjourneyTask(c2, relayconstant.RelayModeMidjourneyTaskFetch); resp == nil || resp.Description != "task_no_found" {
		t.Errorf("expected task_no_found, got %+v", resp)
	}

	// Fetch by condition — with ids.
	c3, rec3 := mjGinContext(t, []byte(`{"ids":["t-1"]}`))
	c3.Set("id", 5)
	if resp := RelayMidjourneyTask(c3, relayconstant.RelayModeMidjourneyTaskFetchByCondition); resp != nil {
		t.Errorf("expected nil, got %+v", resp)
	}
	if rec3.Body.Len() == 0 {
		t.Errorf("expected body for condition fetch")
	}

	// Fetch by condition — empty ids → empty array body.
	c4, rec4 := mjGinContext(t, []byte(`{"ids":[]}`))
	c4.Set("id", 5)
	if resp := RelayMidjourneyTask(c4, relayconstant.RelayModeMidjourneyTaskFetchByCondition); resp != nil {
		t.Errorf("expected nil, got %+v", resp)
	}
	if !bytes.Contains(rec4.Body.Bytes(), []byte("[]")) {
		t.Errorf("expected empty array, got %s", rec4.Body.String())
	}

	// Fetch by condition — bind error.
	c5, _ := mjGinContext(t, []byte(`{bad`))
	c5.Set("id", 5)
	if resp := RelayMidjourneyTask(c5, relayconstant.RelayModeMidjourneyTaskFetchByCondition); resp == nil || resp.Code != 4 {
		t.Errorf("expected bind error, got %+v", resp)
	}
}

// ─── RelayMidjourneySubmit early validation branches ─────────────────────────

func TestRelayMidjourneySubmit_ValidationBranches(t *testing.T) {
	cleanup := setupRelayDB(t)
	defer cleanup()

	newInfo := func() *relaycommon.RelayInfo { return &relaycommon.RelayInfo{UserId: 9} }

	// Bind error.
	c, _ := mjGinContext(t, []byte(`{bad`))
	if resp := RelayMidjourneySubmit(c, newInfo()); resp == nil {
		t.Errorf("expected bind error")
	}

	// Imagine with empty prompt → prompt_is_required.
	c2, _ := mjGinContext(t, []byte(`{"prompt":""}`))
	info2 := newInfo()
	info2.RelayMode = relayconstant.RelayModeMidjourneyImagine
	if resp := RelayMidjourneySubmit(c2, info2); resp == nil || resp.Description != "prompt_is_required" {
		t.Errorf("expected prompt_is_required, got %+v", resp)
	}

	// Change with taskId set but origin task not found → task_not_found.
	c3, _ := mjGinContext(t, []byte(`{"taskId":"nope","action":"UPSCALE","index":1}`))
	info3 := newInfo()
	info3.RelayMode = relayconstant.RelayModeMidjourneyChange
	if resp := RelayMidjourneySubmit(c3, info3); resp == nil || resp.Description != "task_not_found" {
		t.Errorf("expected task_not_found, got %+v", resp)
	}

	// SimpleChange with empty content → content_is_required.
	c4, _ := mjGinContext(t, []byte(`{"taskId":"x","content":""}`))
	info4 := newInfo()
	info4.RelayMode = relayconstant.RelayModeMidjourneySimpleChange
	if resp := RelayMidjourneySubmit(c4, info4); resp == nil || resp.Description != "content_is_required" {
		t.Errorf("expected content_is_required, got %+v", resp)
	}
}

// ─── RelaySwapFace validation branches ───────────────────────────────────────

func TestRelaySwapFace_ValidationBranches(t *testing.T) {
	cleanup := setupRelayDB(t)
	defer cleanup()

	// Bind error.
	c, _ := mjGinContext(t, []byte(`{bad`))
	if resp := RelaySwapFace(c, &relaycommon.RelayInfo{}); resp == nil {
		t.Errorf("expected bind error")
	}

	// Missing base64 fields → validation error.
	c2, _ := mjGinContext(t, []byte(`{"sourceBase64":"","targetBase64":""}`))
	if resp := RelaySwapFace(c2, &relaycommon.RelayInfo{}); resp == nil ||
		resp.Description != "sour_base64_and_target_base64_is_required" {
		t.Errorf("expected base64-required error, got %+v", resp)
	}
}

// ─── RelayMidjourneyTaskImageSeed branches ───────────────────────────────────

func TestRelayMidjourneyTaskImageSeed_Branches(t *testing.T) {
	cleanup := setupRelayDB(t)
	defer cleanup()

	// Task not found.
	c, _ := mjGinContext(t, nil)
	c.Set("id", 3)
	c.Params = gin.Params{{Key: "id", Value: "missing"}}
	if resp := RelayMidjourneyTaskImageSeed(c); resp == nil {
		t.Errorf("expected task_no_found error")
	}

	// Task exists but its channel is disabled.
	ch := &repo.Channel{Type: 1, Status: common.ChannelStatusManuallyDisabled}
	if err := repo.DB.Create(ch).Error; err != nil {
		t.Fatalf("seed channel: %v", err)
	}
	if err := repo.MjInsert(&repo.Midjourney{MjId: "seed-1", UserId: 3, ChannelId: ch.Id}); err != nil {
		t.Fatalf("seed mj: %v", err)
	}
	c2, _ := mjGinContext(t, nil)
	c2.Set("id", 3)
	c2.Params = gin.Params{{Key: "id", Value: "seed-1"}}
	if resp := RelayMidjourneyTaskImageSeed(c2); resp == nil {
		t.Errorf("expected disabled-channel error")
	}
}

// ─── RelayMidjourneySubmit happy path (fake MJ upstream) ─────────────────────

func mjPostContext(t *testing.T, urlPath string, body []byte) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	r := httptest.NewRequest(http.MethodPost, urlPath, bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	c.Request = r
	return c, rec
}

func TestRelayMidjourneySubmit_ImagineHappyPath(t *testing.T) {
	cleanup := setupRelayDB(t)
	defer cleanup()
	app.InitHttpClient()

	u := &repo.User{Username: "mj-user", Quota: 100_000_000}
	if err := repo.DB.Create(u).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"code":1,"result":"mj-abc","description":"submitted"}`))
	}))
	defer srv.Close()

	c, rec := mjPostContext(t, "/mj/submit/imagine", []byte(`{"prompt":"a fox in the snow"}`))
	c.Set("base_url", srv.URL)
	c.Set("token_name", "tkn")

	info := &relaycommon.RelayInfo{UserId: u.Id, IsPlayground: true}
	info.RelayMode = relayconstant.RelayModeMidjourneyImagine

	resp := RelayMidjourneySubmit(c, info)
	if resp != nil {
		t.Fatalf("expected nil (success), got %+v", resp)
	}
	if rec.Body.Len() == 0 {
		t.Errorf("expected response body written")
	}
	// The submitted task must be persisted with the upstream MjId.
	saved := repo.GetByOnlyMJId("mj-abc")
	if saved == nil {
		t.Fatalf("expected task persisted for mj-abc")
	}
	if saved.UserId != u.Id || saved.Action != "IMAGINE" {
		t.Errorf("unexpected persisted task: %+v", saved)
	}
}

func TestRelayMidjourneyTaskImageSeed_HappyPath(t *testing.T) {
	cleanup := setupRelayDB(t)
	defer cleanup()
	app.InitHttpClient()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"code":1,"result":"12345","description":"ok"}`))
	}))
	defer srv.Close()

	baseURL := srv.URL
	ch := &repo.Channel{Type: 1, Status: common.ChannelStatusEnabled, BaseURL: &baseURL, Key: "sk-mj"}
	if err := repo.DB.Create(ch).Error; err != nil {
		t.Fatalf("seed channel: %v", err)
	}
	if err := repo.MjInsert(&repo.Midjourney{MjId: "seed-ok", UserId: 8, ChannelId: ch.Id}); err != nil {
		t.Fatalf("seed mj: %v", err)
	}

	c, rec := mjGinContext(t, nil)
	c.Set("id", 8)
	c.Params = gin.Params{{Key: "id", Value: "seed-ok"}}

	if resp := RelayMidjourneyTaskImageSeed(c); resp != nil {
		t.Fatalf("expected nil (success), got %+v", resp)
	}
	if rec.Body.Len() == 0 {
		t.Errorf("expected response body written")
	}
}

var _ = json.Marshal

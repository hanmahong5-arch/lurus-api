package relay

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	relaycommon "github.com/LurusTech/lurus-hub/internal/adapter/provider/common"
	"github.com/LurusTech/lurus-hub/internal/adapter/repo"
	"github.com/LurusTech/lurus-hub/internal/app"
	"github.com/LurusTech/lurus-hub/internal/pkg/common"
	"github.com/LurusTech/lurus-hub/internal/pkg/constant"

	"github.com/gin-gonic/gin"
)

func taskSubmitContext(t *testing.T, method, path, action string, body []byte) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	var r *http.Request
	if body == nil {
		r = httptest.NewRequest(method, path, nil)
	} else {
		r = httptest.NewRequest(method, path, bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
	}
	c.Request = r
	if action != "" {
		c.Params = append(c.Params, gin.Param{Key: "action", Value: action})
	}
	return c, rec
}

// TestRelayTaskSubmit_InvalidPlatform: with no platform and no channel_type the
// task platform resolves empty and GetTaskAdaptor returns nil, so the submit is
// rejected before any upstream call.
func TestRelayTaskSubmit_InvalidPlatform(t *testing.T) {
	cleanup := setupRelayDB(t)
	defer cleanup()

	c, _ := taskSubmitContext(t, http.MethodPost, "/mj/submit", "", []byte(`{}`))
	info := &relaycommon.RelayInfo{}
	taskErr := RelayTaskSubmit(c, info)
	if taskErr == nil {
		t.Fatalf("expected invalid_api_platform error, got nil")
	}
}

// TestRelayTaskSubmit_RemixRequiresVideoID: a remix action with an empty
// video_id path param is rejected up front.
func TestRelayTaskSubmit_RemixRequiresVideoID(t *testing.T) {
	cleanup := setupRelayDB(t)
	defer cleanup()

	c, _ := taskSubmitContext(t, http.MethodPost, "/v1/videos/remix", "", []byte(`{}`))
	info := &relaycommon.RelayInfo{}
	taskErr := RelayTaskSubmit(c, info)
	if taskErr == nil {
		t.Fatalf("expected video_id required error, got nil")
	}
}

// TestRelayTaskSubmit_SunoMusicHappyPath drives a full Suno MUSIC submission
// against a fake upstream: validate → price → quota → build → dispatch →
// DoResponse → persist task. Proves the async task submit path end-to-end.
func TestRelayTaskSubmit_SunoMusicHappyPath(t *testing.T) {
	cleanup := setupRelayDB(t)
	defer cleanup()
	app.InitHttpClient()

	u := &repo.User{Username: "suno-user", Quota: 100_000_000}
	if err := repo.DB.Create(u).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"code":"success","message":"","data":"suno-task-1"}`))
	}))
	defer srv.Close()

	c, _ := taskSubmitContext(t, http.MethodPost, "/suno/submit/music", "music", []byte(`{"prompt":"lofi beat"}`))
	c.Set("platform", string(constant.TaskPlatformSuno))
	c.Set("token_name", "tkn")
	common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, srv.URL)
	common.SetContextKey(c, constant.ContextKeyChannelKey, "sk-suno")

	info := &relaycommon.RelayInfo{UserId: u.Id, IsPlayground: true}

	taskErr := RelayTaskSubmit(c, info)
	if taskErr != nil {
		t.Fatalf("suno submit failed: %+v", taskErr)
	}

	// The task must be persisted with the upstream task id.
	saved, exist, err := repo.GetByTaskId(u.Id, "suno-task-1")
	if err != nil {
		t.Fatalf("GetByTaskId: %v", err)
	}
	if !exist || saved == nil {
		t.Fatalf("expected persisted suno task")
	}
	if saved.Platform != constant.TaskPlatformSuno {
		t.Errorf("platform = %q, want suno", saved.Platform)
	}
}

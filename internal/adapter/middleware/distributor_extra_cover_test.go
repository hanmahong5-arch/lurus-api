package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/LurusTech/lurus-hub/internal/adapter/repo"
	"github.com/LurusTech/lurus-hub/internal/pkg/common"
	"github.com/LurusTech/lurus-hub/internal/pkg/constant"

	"github.com/gin-gonic/gin"
)

// getModelRequest for the async task-fetch paths that must NOT trigger channel
// selection (they operate on an existing task id, not a fresh model request).

func TestGetModelRequest_MJFetch_SkipsSelect(t *testing.T) {
	// A midjourney task-fetch path resolves to a fetch relay mode → no channel
	// selection.
	c, _ := newTestContext(http.MethodGet, "/mj/task/abc-123/fetch", ``, "")
	_, shouldSelect, err := getModelRequest(c)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if shouldSelect {
		t.Errorf("mj task fetch must not select a channel")
	}
	if _, ok := c.Get("relay_mode"); !ok {
		t.Errorf("relay_mode not set for mj path")
	}
}

func TestGetModelRequest_SunoSubmit_ResolvesModel(t *testing.T) {
	// /suno/submit/:action resolves a model name from the task action.
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/suno/submit/music", nil)
	c.Params = gin.Params{{Key: "action", Value: "music"}}

	mr, _, err := getModelRequest(c)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if mr.Model == "" {
		t.Errorf("expected a resolved suno model name, got empty")
	}
	if p, _ := c.Get("platform"); p != string(constant.TaskPlatformSuno) {
		t.Errorf("platform ctx = %v, want suno", p)
	}
}

func TestGetModelRequest_SunoFetch_SkipsSelect(t *testing.T) {
	c, _ := newTestContext(http.MethodGet, "/suno/fetch/task-1", ``, "")
	_, shouldSelect, err := getModelRequest(c)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if shouldSelect {
		t.Errorf("suno fetch must not select a channel")
	}
}

func TestGetModelRequest_VideoGenerationsGet_SkipsSelect(t *testing.T) {
	c, _ := newTestContext(http.MethodGet, "/v1/video/generations", ``, "")
	_, shouldSelect, err := getModelRequest(c)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if shouldSelect {
		t.Errorf("GET video generations must not select a channel")
	}
}

// Distribute playground path: an out-of-scope group in the pg request body is
// rejected before any channel is selected.
func TestDistribute_Playground_UnusableGroup_403(t *testing.T) {
	_, cleanup := setupCoverDB(t)
	defer cleanup()

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(func(c *gin.Context) {
		common.SetContextKey(c, constant.ContextKeyUsingGroup, "default")
		c.Next()
	})
	r.POST("/pg/chat/completions", Distribute(), func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })

	req := httptest.NewRequest(http.MethodPost, "/pg/chat/completions",
		strings.NewReader(`{"model":"gpt-4o","group":"forbidden-group"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403 for out-of-scope playground group; body=%s", w.Code, w.Body.String())
	}
}

// TestDistribute_HappyPath_RoutesToSatisfyingChannel is the tenant-isolation
// KPI: a request for a model served by an enabled channel in the caller's group
// is routed to that channel (200) and the channel id is pinned in context.
func TestDistribute_HappyPath_RoutesToSatisfyingChannel(t *testing.T) {
	db, cleanup := setupCoverDB(t)
	defer cleanup()

	ch := &repo.Channel{
		Id: 20, Name: "svc", TenantId: "default", Key: "k",
		Status: common.ChannelStatusEnabled, Models: "gpt-4o", Group: "default",
	}
	if err := db.Create(ch).Error; err != nil {
		t.Fatalf("create channel: %v", err)
	}
	if err := ch.AddAbilities(db); err != nil {
		t.Fatalf("add abilities: %v", err)
	}

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(func(c *gin.Context) {
		common.SetContextKey(c, constant.ContextKeyUsingGroup, "default")
		c.Next()
	})
	var pinned int
	r.POST("/v1/chat/completions", Distribute(), func(c *gin.Context) {
		pinned = common.GetContextKeyInt(c, constant.ContextKeyChannelId)
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-4o"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 for satisfiable model; body=%s", w.Code, w.Body.String())
	}
	if pinned != 20 {
		t.Errorf("pinned channel = %d, want 20 (request must route to the satisfying channel)", pinned)
	}
}

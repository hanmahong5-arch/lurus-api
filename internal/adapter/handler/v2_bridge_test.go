package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/LurusTech/lurus-hub/internal/adapter/repo"
	"github.com/LurusTech/lurus-hub/internal/pkg/common"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

// buildBridgeRouter wires BridgeExchange behind a minimal cookie-backed
// session middleware so the handler can persist a session without Redis.
func buildBridgeRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	store := cookie.NewStore([]byte("bridge-test-secret"))
	r.Use(sessions.Sessions("test_session", store))
	r.POST("/api/v2/bridge/exchange", BridgeExchange)
	return r
}

// TestBridgeEnabled_RespectsEnv ensures the gate flips with the env var.
// Route registration must consult this — empty env → not registered.
func TestBridgeEnabled_RespectsEnv(t *testing.T) {
	t.Setenv("E2E_BRIDGE_TOKEN", "")
	if BridgeEnabled() {
		t.Errorf("BridgeEnabled() = true, want false when env empty")
	}
	t.Setenv("E2E_BRIDGE_TOKEN", "anything-non-empty")
	if !BridgeEnabled() {
		t.Errorf("BridgeEnabled() = false, want true when env set")
	}
}

// TestBridgeExchange_EmptyEnv: handler must reject even if (somehow) called
// while env is empty — defense in depth in case route registration leaks.
func TestBridgeExchange_EmptyEnv(t *testing.T) {
	ctx := SetupV2TestRouter(t)
	defer ctx.Cleanup()

	t.Setenv("E2E_BRIDGE_TOKEN", "")
	r := buildBridgeRouter()

	req := httptest.NewRequest(http.MethodPost, "/api/v2/bridge/exchange?token=anything&user_id=1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403, body: %s", w.Code, w.Body.String())
	}
}

// TestBridgeExchange_TokenMismatch: wrong token → 403.
func TestBridgeExchange_TokenMismatch(t *testing.T) {
	ctx := SetupV2TestRouter(t)
	defer ctx.Cleanup()

	t.Setenv("E2E_BRIDGE_TOKEN", "correct-token")
	r := buildBridgeRouter()

	req := httptest.NewRequest(http.MethodPost, "/api/v2/bridge/exchange?token=WRONG&user_id=1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403, body: %s", w.Code, w.Body.String())
	}
}

// TestBridgeExchange_MissingToken: empty token query param → 403.
func TestBridgeExchange_MissingToken(t *testing.T) {
	ctx := SetupV2TestRouter(t)
	defer ctx.Cleanup()

	t.Setenv("E2E_BRIDGE_TOKEN", "correct-token")
	r := buildBridgeRouter()

	req := httptest.NewRequest(http.MethodPost, "/api/v2/bridge/exchange?user_id=1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403, body: %s", w.Code, w.Body.String())
	}
}

// TestBridgeExchange_MissingUserID: token ok but user_id missing → 400.
func TestBridgeExchange_MissingUserID(t *testing.T) {
	ctx := SetupV2TestRouter(t)
	defer ctx.Cleanup()

	t.Setenv("E2E_BRIDGE_TOKEN", "correct-token")
	r := buildBridgeRouter()

	req := httptest.NewRequest(http.MethodPost, "/api/v2/bridge/exchange?token=correct-token", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400, body: %s", w.Code, w.Body.String())
	}
}

// TestBridgeExchange_InvalidUserID: non-numeric user_id → 400.
func TestBridgeExchange_InvalidUserID(t *testing.T) {
	ctx := SetupV2TestRouter(t)
	defer ctx.Cleanup()

	t.Setenv("E2E_BRIDGE_TOKEN", "correct-token")
	r := buildBridgeRouter()

	req := httptest.NewRequest(http.MethodPost, "/api/v2/bridge/exchange?token=correct-token&user_id=abc", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400, body: %s", w.Code, w.Body.String())
	}
}

// TestBridgeExchange_UserNotFound: user_id refers to nonexistent user → 404.
func TestBridgeExchange_UserNotFound(t *testing.T) {
	ctx := SetupV2TestRouter(t)
	defer ctx.Cleanup()

	t.Setenv("E2E_BRIDGE_TOKEN", "correct-token")
	r := buildBridgeRouter()

	req := httptest.NewRequest(http.MethodPost, "/api/v2/bridge/exchange?token=correct-token&user_id=99999", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404, body: %s", w.Code, w.Body.String())
	}
}

// TestBridgeExchange_DisabledUser: disabled user → 403 (no session issued).
func TestBridgeExchange_DisabledUser(t *testing.T) {
	ctx := SetupV2TestRouter(t)
	defer ctx.Cleanup()

	disabled := &repo.User{
		Username:    "v2bridgedisabled",
		DisplayName: "Disabled Bridge User",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusDisabled,
		Email:       "disabled@test.local",
		TenantId:    ctx.TenantID,
	}
	if err := ctx.DB.Create(disabled).Error; err != nil {
		t.Fatalf("seed disabled user: %v", err)
	}

	t.Setenv("E2E_BRIDGE_TOKEN", "correct-token")
	r := buildBridgeRouter()

	url := "/api/v2/bridge/exchange?token=correct-token&user_id=" + strconv.Itoa(disabled.Id)
	req := httptest.NewRequest(http.MethodPost, url, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403, body: %s", w.Code, w.Body.String())
	}
}

// TestBridgeExchange_Success: happy path returns 200 + session cookie + slug.
func TestBridgeExchange_Success(t *testing.T) {
	ctx := SetupV2TestRouter(t)
	defer ctx.Cleanup()

	t.Setenv("E2E_BRIDGE_TOKEN", "correct-token")
	r := buildBridgeRouter()

	url := "/api/v2/bridge/exchange?token=correct-token&user_id=" + strconv.Itoa(ctx.NormalUser.Id)
	req := httptest.NewRequest(http.MethodPost, url, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("parse body: %v", err)
	}
	if resp["success"] != true {
		t.Errorf("success = %v, want true", resp["success"])
	}

	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("data field missing")
	}
	if int(data["id"].(float64)) != ctx.NormalUser.Id {
		t.Errorf("id = %v, want %d", data["id"], ctx.NormalUser.Id)
	}
	// tenant_slug must be present — the whole point of the bridge is to
	// preload it for the frontend.
	slug, ok := data["tenant_slug"].(string)
	if !ok || slug == "" {
		t.Errorf("tenant_slug = %v, want non-empty string", data["tenant_slug"])
	}

	// A Set-Cookie must be present so the next request can ride the session.
	if len(w.Header().Values("Set-Cookie")) == 0 {
		t.Errorf("expected Set-Cookie session header, got none")
	}
}

// TestResolveTenantSlug_FallbacksOnMissing covers the three fallback paths
// the resolver guards: empty input, "default" sentinel, and DB miss. The
// bridge is on the login critical path so a slug lookup miss must NOT
// fail the request — it must return "default" so the frontend has a
// routable value.
func TestResolveTenantSlug_FallbacksOnMissing(t *testing.T) {
	ctx := SetupV2TestRouter(t)
	defer ctx.Cleanup()

	if got := resolveTenantSlug(""); got != "default" {
		t.Errorf("resolveTenantSlug(\"\") = %q, want \"default\"", got)
	}
	if got := resolveTenantSlug("default"); got != "default" {
		t.Errorf("resolveTenantSlug(\"default\") = %q, want \"default\"", got)
	}
	if got := resolveTenantSlug("nonexistent-tenant-id"); got != "default" {
		t.Errorf("resolveTenantSlug(unknown) = %q, want \"default\"", got)
	}
}

// TestResolveTenantSlug_LooksUpRealTenant verifies the happy path: a valid
// tenant id resolves to its slug.
func TestResolveTenantSlug_LooksUpRealTenant(t *testing.T) {
	ctx := SetupV2TestRouter(t)
	defer ctx.Cleanup()

	got := resolveTenantSlug(ctx.TenantID)
	// SetupV2TestRouter seeds slug = TenantID for the test tenant.
	if got != ctx.TenantID {
		t.Errorf("resolveTenantSlug(%q) = %q, want %q", ctx.TenantID, got, ctx.TenantID)
	}
}


package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/LurusTech/lurus-hub/internal/adapter/repo"
	"github.com/LurusTech/lurus-hub/internal/pkg/common"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

// mountSessionFallbackProbe drives handleSessionFallback directly (it is the
// OIDCAuth session-cookie recovery path). The probe route pre-seeds the session
// with `preset`, calls handleSessionFallback, and — unless the fallback already
// wrote a response — reports the boolean result plus the resolved identity so
// tests can assert both the control-flow decision and the tenant scoping.
func mountSessionFallbackProbe(preset map[string]any) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	store := cookie.NewStore([]byte("gap-r3-sf-secret"))
	r.Use(sessions.Sessions("session", store))
	r.Use(func(c *gin.Context) {
		s := sessions.Default(c)
		for k, v := range preset {
			s.Set(k, v)
		}
		_ = s.Save()
		c.Next()
	})
	r.GET("/sf", func(c *gin.Context) {
		handled := handleSessionFallback(c)
		if c.IsAborted() {
			return
		}
		tid := ""
		if tc, err := GetTenantContext(c); err == nil && tc != nil {
			tid = tc.TenantID
		}
		c.JSON(http.StatusOK, gin.H{"handled": handled, "tenant_id": tid, "id": c.GetInt("id")})
	})
	return r
}

func probeSessionFallback(t *testing.T, preset map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	mountSessionFallbackProbe(preset).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/sf", nil))
	return w
}

// No session id → fallback declines (:513).
func TestSessionFallback_NoSessionID_Declines(t *testing.T) {
	_, cleanup := setupCoverDB(t)
	defer cleanup()
	w := probeSessionFallback(t, map[string]any{})
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
}

// Non-int session id → fallback declines (:518).
func TestSessionFallback_NonIntID_Declines(t *testing.T) {
	_, cleanup := setupCoverDB(t)
	defer cleanup()
	w := probeSessionFallback(t, map[string]any{"id": "not-int"})
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (non-int id declines)", w.Code)
	}
}

// Negative session id → passes the ==0 guard but fails userID>0, reaching the
// terminal decline (:623).
func TestSessionFallback_NegativeID_Declines(t *testing.T) {
	_, cleanup := setupCoverDB(t)
	defer cleanup()
	w := probeSessionFallback(t, map[string]any{"id": -5})
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (negative id declines)", w.Code)
	}
}

// Non-expired OAuth token but the user row is gone → GetUserById errors →
// fallback declines rather than fabricating a context (:531).
func TestSessionFallback_NonExpiredToken_MissingUser_Declines(t *testing.T) {
	_, cleanup := setupCoverDB(t)
	defer cleanup()
	w := probeSessionFallback(t, map[string]any{
		"id":                     999999,
		"oauth_access_token":     "live-token",
		"oauth_token_expires_at": time.Now().Add(time.Hour).Unix(),
	})
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (missing user declines)", w.Code)
	}
}

// Non-expired token + enabled user with an EMPTY tenant → tenant defaults to
// "default" (:556) and the context is built.
func TestSessionFallback_NonExpiredToken_EmptyTenant_DefaultsAndResolves(t *testing.T) {
	_, cleanup := setupCoverDB(t)
	defer cleanup()
	user := &repo.User{
		Username: "sfuser1", Role: common.RoleCommonUser, Status: common.UserStatusEnabled,
		Email: "sf1@local", TenantId: "", // empty → must default
	}
	if err := repo.DB.Create(user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	w := probeSessionFallback(t, map[string]any{
		"id":                     user.Id,
		"oauth_access_token":     "live-token",
		"oauth_token_expires_at": time.Now().Add(time.Hour).Unix(),
	})
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	if got := w.Body.String(); !strings.Contains(got, `"tenant_id":"default"`) {
		t.Errorf("tenant_id not defaulted to 'default': %s", got)
	}
}

// Expired token + DISABLED user → fallback rejects with 403 even though the
// session cookie is still valid (:588). Fails closed.
func TestSessionFallback_ExpiredToken_DisabledUser_403(t *testing.T) {
	_, cleanup := setupCoverDB(t)
	defer cleanup()
	user := &repo.User{
		Username: "sfdisabled", Role: common.RoleCommonUser, Status: common.UserStatusDisabled,
		Email: "sfd@local", TenantId: "default",
	}
	if err := repo.DB.Create(user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	// No oauth token in session → the "expired/missing token" branch.
	w := probeSessionFallback(t, map[string]any{"id": user.Id})
	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 for disabled user via session fallback; body=%s", w.Code, w.Body.String())
	}
}

// Expired token + enabled user with EMPTY tenant → tenant defaults to "default"
// in the expired branch (:601) and the context is built.
func TestSessionFallback_ExpiredToken_EmptyTenant_DefaultsAndResolves(t *testing.T) {
	_, cleanup := setupCoverDB(t)
	defer cleanup()
	user := &repo.User{
		Username: "sfuser2", Role: common.RoleCommonUser, Status: common.UserStatusEnabled,
		Email: "sf2@local", TenantId: "",
	}
	if err := repo.DB.Create(user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	w := probeSessionFallback(t, map[string]any{"id": user.Id}) // no oauth token → expired branch
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	if got := w.Body.String(); !strings.Contains(got, `"tenant_id":"default"`) {
		t.Errorf("tenant_id not defaulted to 'default': %s", got)
	}
}

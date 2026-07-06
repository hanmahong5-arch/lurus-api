package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/LurusTech/lurus-hub/internal/adapter/repo"
	"github.com/LurusTech/lurus-hub/internal/pkg/common"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

// sessionOIDCRouter builds an OIDCAuth-protected route behind a cookie session
// store, seeding the provided session values. OIDCAuth only reaches
// handleSessionFallback when oidcEnabled is true, which setupIntegrationTest
// arranges — so callers must invoke this within that harness.
func sessionOIDCRouter(values map[string]interface{}) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	store := cookie.NewStore([]byte("oidc-sess-cover"))
	r.Use(sessions.Sessions("session", store))
	r.Use(func(c *gin.Context) {
		s := sessions.Default(c)
		for k, v := range values {
			s.Set(k, v)
		}
		_ = s.Save()
		c.Next()
	})
	r.GET("/p", OIDCAuth(), func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"id": c.GetInt("id")}) })
	return r
}

func TestOIDCAuth_SessionFallback_ValidOAuthToken(t *testing.T) {
	ctx := setupIntegrationTest(t)
	defer ctx.Cleanup()
	r := sessionOIDCRouter(map[string]interface{}{
		"id":                     ctx.User.Id,
		"oauth_access_token":     "opaque-token",
		"oauth_token_expires_at": time.Now().Add(time.Hour).Unix(),
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/p", nil))
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 for valid session with fresh OAuth token; body=%s", w.Code, w.Body.String())
	}
}

func TestOIDCAuth_SessionFallback_ExpiredTokenButSessionUser(t *testing.T) {
	ctx := setupIntegrationTest(t)
	defer ctx.Cleanup()
	// No/expired OAuth token, but a session user id → session-data-only branch.
	r := sessionOIDCRouter(map[string]interface{}{
		"id":                     ctx.User.Id,
		"oauth_token_expires_at": time.Now().Add(-time.Hour).Unix(),
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/p", nil))
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 for session-data-only fallback; body=%s", w.Code, w.Body.String())
	}
}

func TestOIDCAuth_SessionFallback_DisabledUser_403(t *testing.T) {
	ctx := setupIntegrationTest(t)
	defer ctx.Cleanup()
	// Disable the integration user; a valid session cookie must NOT admit them.
	if err := ctx.DB.Model(&repo.User{}).Where("id = ?", ctx.User.Id).Update("status", common.UserStatusDisabled).Error; err != nil {
		t.Fatalf("disable user: %v", err)
	}
	r := sessionOIDCRouter(map[string]interface{}{
		"id":                     ctx.User.Id,
		"oauth_access_token":     "opaque-token",
		"oauth_token_expires_at": time.Now().Add(time.Hour).Unix(),
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/p", nil))
	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403 for disabled user with valid session; body=%s", w.Code, w.Body.String())
	}
}

package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/LurusTech/lurus-hub/internal/adapter/repo"
	"github.com/LurusTech/lurus-hub/internal/domain/entity"
	"github.com/LurusTech/lurus-hub/internal/pkg/common"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

// buildRoleRouter is buildAuthRouter with a configurable minimum role so both
// UserAuth and AdminAuth branches of authHelper can be exercised.
func buildRoleRouter(sessionValues map[string]interface{}, mw func() func(c *gin.Context)) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	store := cookie.NewStore([]byte("role-test-secret"))
	r.Use(sessions.Sessions("session", store))
	r.Use(func(c *gin.Context) {
		s := sessions.Default(c)
		for k, v := range sessionValues {
			s.Set(k, v)
		}
		_ = s.Save()
		c.Next()
	})
	r.GET("/test", mw(), func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })
	return r
}

func serveRole(r *gin.Engine, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestAuthHelper_StatusDisabled(t *testing.T) {
	r := buildRoleRouter(map[string]interface{}{
		"username": "u", "role": common.RoleCommonUser, "id": 0, "status": common.UserStatusDisabled,
	}, UserAuth)
	w := serveRole(r, nil)
	if strings.Contains(w.Body.String(), `"ok":true`) {
		t.Errorf("disabled user reached handler; body=%s", w.Body.String())
	}
}

func TestAuthHelper_RoleInsufficient(t *testing.T) {
	// Common-role session hitting an AdminAuth route → permission denied.
	r := buildRoleRouter(map[string]interface{}{
		"username": "u", "role": common.RoleCommonUser, "id": 0, "status": common.UserStatusEnabled,
	}, AdminAuth)
	w := serveRole(r, nil)
	if strings.Contains(w.Body.String(), `"ok":true`) {
		t.Errorf("under-privileged user reached admin handler; body=%s", w.Body.String())
	}
}

func TestAuthHelper_InvalidStatusType(t *testing.T) {
	r := buildRoleRouter(map[string]interface{}{
		"username": "u", "role": common.RoleCommonUser, "id": 0, "status": "not-an-int",
	}, UserAuth)
	w := serveRole(r, nil)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401 for non-int session status", w.Code)
	}
}

func TestAuthHelper_InvalidRoleType(t *testing.T) {
	r := buildRoleRouter(map[string]interface{}{
		"username": "u", "role": "not-an-int", "id": 0, "status": common.UserStatusEnabled,
	}, UserAuth)
	w := serveRole(r, nil)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401 for non-int session role", w.Code)
	}
}

func TestAuthHelper_UsernameEmpty_Invalid(t *testing.T) {
	r := buildRoleRouter(map[string]interface{}{
		"username": "", "role": common.RoleCommonUser, "id": 0, "status": common.UserStatusEnabled,
	}, UserAuth)
	w := serveRole(r, nil)
	if strings.Contains(w.Body.String(), `"ok":true`) {
		t.Errorf("empty username reached handler; body=%s", w.Body.String())
	}
}

func TestAuthHelper_NoSessionNoToken_401(t *testing.T) {
	r := buildRoleRouter(map[string]interface{}{}, UserAuth) // empty session, no Authorization
	w := serveRole(r, nil)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401 for anonymous", w.Code)
	}
}

func TestAuthHelper_InvalidAccessToken(t *testing.T) {
	_, cleanup := setupCoverDB(t)
	defer cleanup()
	// No session username → access-token path. A bogus bearer is neither a
	// valid identity-session token nor an access token → rejected.
	r := buildRoleRouter(map[string]interface{}{}, UserAuth)
	w := serveRole(r, map[string]string{"Authorization": "Bearer bogus-access-token"})
	if strings.Contains(w.Body.String(), `"ok":true`) {
		t.Errorf("invalid access token reached handler; body=%s", w.Body.String())
	}
}

func TestAuthHelper_TenantDisabled_403(t *testing.T) {
	_, cleanup := setupCoverDB(t)
	defer cleanup()
	// User belongs to a disabled tenant → authHelper's TI-5 lockout (403).
	tn := &entity.Tenant{Id: "tenantD", Slug: "tenantD", Name: "n", Status: entity.TenantStatusDisabled, PlanType: entity.TenantPlanFree, MaxUsers: 10, MaxQuota: 1000}
	if err := repo.DB.Create(tn).Error; err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	user := &repo.User{Username: "tenantuser", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Email: "tu@local", TenantId: "tenantD"}
	if err := repo.DB.Create(user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	r := buildRoleRouter(map[string]interface{}{
		"username": user.Username, "role": user.Role, "id": user.Id, "status": user.Status,
	}, UserAuth)
	w := serveRole(r, nil)
	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403 for member of disabled tenant; body=%s", w.Code, w.Body.String())
	}
}

func TestAuthHelper_HappyPath_SetsContext(t *testing.T) {
	_, cleanup := setupCoverDB(t)
	defer cleanup()
	user := &repo.User{Username: "okuser", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Email: "ok@local", TenantId: "default"}
	if err := repo.DB.Create(user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	r := buildRoleRouter(map[string]interface{}{
		"username": user.Username, "role": user.Role, "id": user.Id, "status": user.Status,
	}, UserAuth)
	w := serveRole(r, nil)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 for valid default-tenant user; body=%s", w.Code, w.Body.String())
	}
}

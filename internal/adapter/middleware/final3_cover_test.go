package middleware

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/LurusTech/lurus-hub/internal/adapter/repo"
	"github.com/LurusTech/lurus-hub/internal/pkg/common"

	"github.com/gin-gonic/gin"
)

// authHelper access-token branch: an anonymous request bearing a valid system
// access token is authenticated by looking it up in the users table.
func TestAuthHelper_AccessTokenSuccess(t *testing.T) {
	_, cleanup := setupCoverDB(t)
	defer cleanup()

	accessTok := common.GetRandomString(32)
	user := &repo.User{
		Username: "atuser", Role: common.RoleCommonUser, Status: common.UserStatusEnabled,
		Email: "at@local", TenantId: "default", AccessToken: &accessTok,
	}
	if err := repo.DB.Create(user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	r := buildRoleRouter(map[string]interface{}{}, UserAuth) // empty session
	w := serveRole(r, map[string]string{"Authorization": "Bearer " + accessTok})
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 for valid access token; body=%s", w.Code, w.Body.String())
	}
}

// authHelper header-identity cross-check via the New-Api-User alias header:
// a mismatching id must be rejected even with a valid session.
func TestAuthHelper_NewApiUserHeaderMismatch(t *testing.T) {
	r := buildRoleRouter(map[string]interface{}{
		"username": "u", "role": common.RoleCommonUser, "id": 0, "status": common.UserStatusEnabled,
	}, UserAuth)
	w := serveRole(r, map[string]string{"New-Api-User": strconv.Itoa(999)})
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401 for mismatched New-Api-User header", w.Code)
	}
}

// getModelRequest for a midjourney *submit* action resolves a model via the
// midjourney request body (the app.GetMjRequestModel path), unlike the
// task-fetch modes which skip channel selection.
func TestGetModelRequest_MJImagineSubmit(t *testing.T) {
	c, _ := newTestContext(http.MethodPost, "/mj/submit/imagine", `{"prompt":"a calico cat"}`, "application/json")
	mr, _, err := getModelRequest(c)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if mr.Model == "" {
		t.Errorf("expected a resolved midjourney model for imagine submit")
	}
	if _, ok := c.Get("relay_mode"); !ok {
		t.Errorf("relay_mode not set for mj submit")
	}
}

// Kling: request carrying only "model" (not "model_name") still resolves.
func TestKlingRequestConvert_ModelFieldFallback(t *testing.T) {
	var newPath string
	r := gin.New()
	r.POST("/kling", KlingRequestConvert(), func(c *gin.Context) {
		newPath = c.Request.URL.Path
		c.Status(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodPost, "/kling", strings.NewReader(`{"model":"kling-v1","prompt":"x","image":"data"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if newPath != "/v1/video/generations" {
		t.Errorf("path = %q, want rewritten video path", newPath)
	}
}

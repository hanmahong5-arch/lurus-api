package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/LurusTech/lurus-hub/internal/adapter/repo"
	"github.com/LurusTech/lurus-hub/internal/pkg/common"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// runOIDCProtected mounts a bare OIDCAuth-guarded route and drives one request.
func runOIDCProtected(token string) *httptest.ResponseRecorder {
	r := gin.New()
	r.GET("/p", OIDCAuth(), func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })
	req := httptest.NewRequest(http.MethodGet, "/p", nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func validIntegrationClaims(ctx *integrationTestContext) OIDCClaims {
	return OIDCClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    ctx.Issuer,
			Subject:   "zitadel_integration_user",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
		Email:             "integration@test.local",
		PreferredUsername: "integration_user",
		OrgID:             "org_integration_test",
		OrgDomain:         "integration.test",
	}
}

// OIDCAuth must 503 when OIDC is disabled, independent of any token.
func TestOIDCAuth_Disabled_503(t *testing.T) {
	prev := oidcEnabled
	oidcEnabled = false
	defer func() { oidcEnabled = prev }()

	w := runOIDCProtected("")
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503 when OIDC disabled", w.Code)
	}
}

// A validly-signed token whose issuer is not in the accepted set is rejected 401.
func TestOIDCAuth_WrongIssuer_401(t *testing.T) {
	ctx := setupIntegrationTest(t)
	defer ctx.Cleanup()

	claims := validIntegrationClaims(ctx)
	claims.Issuer = "https://attacker.example" // not in oidcIssuers
	token := ctx.createJWT(t, claims)

	w := runOIDCProtected(token)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401 for untrusted issuer; body=%s", w.Code, w.Body.String())
	}
}

// A valid token that maps to a DISABLED user must be forbidden (403).
func TestOIDCAuth_DisabledUser_403(t *testing.T) {
	ctx := setupIntegrationTest(t)
	defer ctx.Cleanup()

	// Flip the mapped user to disabled and clear any user cache.
	if err := ctx.DB.Model(&ctx.User).Update("status", common.UserStatusDisabled).Error; err != nil {
		t.Fatalf("disable user: %v", err)
	}

	token := ctx.createJWT(t, validIntegrationClaims(ctx))
	w := runOIDCProtected(token)
	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403 for disabled user; body=%s", w.Code, w.Body.String())
	}
}

// A valid token whose identity mapping points to a non-existent local user must
// fail closed with 500 (the user-load error branch), never silently authorize.
func TestOIDCAuth_MappedUserMissing_500(t *testing.T) {
	ctx := setupIntegrationTest(t)
	defer ctx.Cleanup()

	orphan := &repo.UserIdentityMapping{
		TenantID:    ctx.Tenant.Id,
		IDPSubject:  "orphan-subject",
		LurusUserID: 999999, // no such user row
		Email:       "orphan@test.local",
		IsActive:    true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	if err := ctx.DB.Create(orphan).Error; err != nil {
		t.Fatalf("create orphan mapping: %v", err)
	}

	claims := validIntegrationClaims(ctx)
	claims.Subject = "orphan-subject"
	token := ctx.createJWT(t, claims)

	w := runOIDCProtected(token)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500 when mapped user is missing; body=%s", w.Code, w.Body.String())
	}
}

package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func integrationClaims(ctx *integrationTestContext) OIDCClaims {
	return OIDCClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    ctx.Issuer,
			Subject:   "zitadel_integration_user",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
		Email:             "integration@test.local",
		EmailVerified:     true,
		Name:              "Integration User",
		PreferredUsername: "integration_user",
		OrgID:             "org_integration_test",
		OrgDomain:         "integration.test",
	}
}

func TestOIDCAuth_ExpiredToken_401(t *testing.T) {
	ctx := setupIntegrationTest(t)
	defer ctx.Cleanup()
	claims := integrationClaims(ctx)
	claims.ExpiresAt = jwt.NewNumericDate(time.Now().Add(-time.Hour)) // expired
	token := ctx.createJWT(t, claims)

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	ctx.Router.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401 for expired token; body=%s", w.Code, w.Body.String())
	}
}

func TestOIDCAuth_UnknownKid_401(t *testing.T) {
	ctx := setupIntegrationTest(t)
	defer ctx.Cleanup()
	// Sign with the correct key but advertise a kid the JWKS doesn't have.
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, integrationClaims(ctx))
	token.Header["kid"] = "unknown-kid"
	signed, err := token.SignedString(ctx.PrivateKey)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+signed)
	w := httptest.NewRecorder()
	ctx.Router.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401 for unknown kid; body=%s", w.Code, w.Body.String())
	}
}

func TestOIDCAuth_NoAuthNoSession_401(t *testing.T) {
	ctx := setupIntegrationTest(t)
	defer ctx.Cleanup()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil) // no bearer, no session
	w := httptest.NewRecorder()
	ctx.Router.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401 for unauthenticated request", w.Code)
	}
}

func TestFlexAuth_ValidJWT_Passes(t *testing.T) {
	ctx := setupIntegrationTest(t)
	defer ctx.Cleanup()
	token := ctx.createJWT(t, integrationClaims(ctx))

	r := gin.New()
	r.Use(gin.Recovery())
	r.GET("/flexjwt", FlexAuth(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"id": c.GetInt("id"), "method": c.GetString("auth_method")})
	})
	req := httptest.NewRequest(http.MethodGet, "/flexjwt", nil)
	req.Header.Set("Authorization", "Bearer "+token) // non-"sk-" → JWT branch
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 for valid JWT via FlexAuth; body=%s", w.Code, w.Body.String())
	}
}

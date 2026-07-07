package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang-jwt/jwt/v5"
)

// validateJWT rejects a token signed with a non-RSA method (HS256) even though
// it parses structurally — the keyfunc enforces RSA.
func TestAdminJWTAuth_NonRSAToken_401(t *testing.T) {
	ctx := setupIntegrationTest(t)
	defer ctx.Cleanup()
	claims := adminClaims(ctx.Issuer, map[string]interface{}{"admin": map[string]interface{}{}})
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tok.Header["kid"] = "test-integration-kid"
	signed, err := tok.SignedString([]byte("hmac-secret"))
	if err != nil {
		t.Fatalf("sign HS256: %v", err)
	}
	w := doBearer(mountJWT(AdminJWTAuth()), signed)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401 for non-RSA signed token; body=%s", w.Code, w.Body.String())
	}
}

// OIDCAuth rejects a token whose header carries no kid.
func TestOIDCAuth_MissingKid_401(t *testing.T) {
	ctx := setupIntegrationTest(t)
	defer ctx.Cleanup()
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, integrationClaims(ctx))
	// Intentionally do NOT set tok.Header["kid"].
	signed, err := tok.SignedString(ctx.PrivateKey)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+signed)
	w := httptest.NewRecorder()
	ctx.Router.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401 for token missing kid; body=%s", w.Code, w.Body.String())
	}
}

// applyConfigurableClaims returns cleanly when the token cannot be parsed, even
// with custom claim keys configured (the ParseUnverified error branch).
func TestApplyConfigurableClaims_UnparseableToken(t *testing.T) {
	prev := oidcClaimKeys
	defer func() { oidcClaimKeys = prev }()
	oidcClaimKeys = claimKeySet{
		OrgID: "custom_org", OrgDomain: "org_domain",
		ResourceOwnerID: "resource_owner_id", ResourceOwnerName: "resource_owner_name", Roles: "roles",
	}
	var claims OIDCClaims
	applyConfigurableClaims("this-is-not-a-jwt", &claims) // must not panic
	if claims.OrgID != "" {
		t.Errorf("OrgID = %q, want empty (unparseable token yields no claims)", claims.OrgID)
	}
}

package middleware

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/LurusTech/lurus-hub/internal/adapter/repo"
	"github.com/LurusTech/lurus-hub/internal/domain/entity"

	"github.com/gin-gonic/gin"
)

// End-to-end release gate through the production wiring: a gated product with a
// real release row, an authenticated account (via context), and the free-plan
// entitlement (no identity URL) → NO_ENTITLEMENT 403. Exercises
// lookupReleaseProductID + checkProductEntitlement + ReleaseDownloadGate.
func TestReleaseDownloadGate_EndToEnd_NoEntitlement(t *testing.T) {
	_, cleanup := setupCoverDB(t)
	defer cleanup()

	rel := &entity.Release{Id: 9, ProductId: "lurus-switch", Version: "2.0", Title: "v2", IsPublished: true}
	if err := repo.DB.Create(rel).Error; err != nil {
		t.Fatalf("create release: %v", err)
	}

	_ = os.Setenv("RELEASE_GATED_PRODUCTS", "lurus-switch")
	defer os.Unsetenv("RELEASE_GATED_PRODUCTS")

	r := gin.New()
	r.Use(gin.Recovery())
	// Pre-authenticate the caller so resolveOIDCAccountID short-circuits.
	r.Use(func(c *gin.Context) { c.Set("identity_account_id", int64(42)); c.Next() })
	r.GET("/releases/:id/download", ReleaseDownloadGate(), func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/releases/9/download", nil))
	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403 NO_ENTITLEMENT for free-plan account on gated product; body=%s", w.Code, w.Body.String())
	}
}

// resolveOIDCAccountID JWT paths (needs oidcEnabled + JWKS from the harness).

func TestResolveOIDCAccountID_InvalidJWT(t *testing.T) {
	ctx := setupIntegrationTest(t)
	defer ctx.Cleanup()
	c, _ := newTestContext(http.MethodGet, "/x", "", "")
	c.Request.Header.Set("Authorization", "Bearer not-a-valid-jwt")
	if id, ok := resolveOIDCAccountID(c); ok || id != 0 {
		t.Errorf("resolveOIDCAccountID(bad jwt) = (%d,%v), want (0,false)", id, ok)
	}
}

func TestResolveOIDCAccountID_IssuerMismatch(t *testing.T) {
	ctx := setupIntegrationTest(t)
	defer ctx.Cleanup()
	claims := integrationClaims(ctx)
	claims.Issuer = "https://not-the-issuer"
	token := ctx.createJWT(t, claims)
	c, _ := newTestContext(http.MethodGet, "/x", "", "")
	c.Request.Header.Set("Authorization", "Bearer "+token)
	if id, ok := resolveOIDCAccountID(c); ok || id != 0 {
		t.Errorf("resolveOIDCAccountID(wrong issuer) = (%d,%v), want (0,false)", id, ok)
	}
}

func TestResolveOIDCAccountID_ValidJWT_NoGRPCAccount(t *testing.T) {
	ctx := setupIntegrationTest(t)
	defer ctx.Cleanup()
	token := ctx.createJWT(t, integrationClaims(ctx))
	c, _ := newTestContext(http.MethodGet, "/x", "", "")
	c.Request.Header.Set("Authorization", "Bearer "+token)
	// Signature+issuer valid, but UpsertAccountGRPC has no backend in unit
	// scope → resolves to no account (false). Exercises the full verify path.
	id, ok := resolveOIDCAccountID(c)
	if ok && id <= 0 {
		t.Errorf("resolveOIDCAccountID returned ok with non-positive id %d", id)
	}
}

// redisRateLimiterKeyed duration-elapsed else-branch: with duration 0 the
// window is always considered expired, so a full bucket rolls over (LPush+LTrim)
// instead of rejecting.
func TestRedisRateLimiterKeyed_WindowElapsed_Rolls(t *testing.T) {
	cleanup := withRedis(t)
	defer cleanup()

	c1, _ := newTestContext(http.MethodGet, "/x", "", "")
	redisRateLimiterKeyed(c1, 1, 3600, "ROLL", "idR")
	if c1.IsAborted() {
		t.Fatalf("first request must pass")
	}
	// Second call with duration 0 → elapsed window → rolls over, not aborted.
	c2, _ := newTestContext(http.MethodGet, "/x", "", "")
	redisRateLimiterKeyed(c2, 1, 0, "ROLL", "idR")
	if c2.IsAborted() {
		t.Errorf("request with elapsed window must roll over, not abort")
	}
}

// getModelRequest images/edits with form-encoded body reads the model field.
func TestGetModelRequest_ImagesEdits_FormModel(t *testing.T) {
	c, _ := newTestContext(http.MethodPost, "/v1/images/edits", "model=gpt-image-1&prompt=x", "application/x-www-form-urlencoded")
	mr, _, err := getModelRequest(c)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if mr.Model != "gpt-image-1" {
		t.Errorf("model = %q, want gpt-image-1 (from form field)", mr.Model)
	}
}

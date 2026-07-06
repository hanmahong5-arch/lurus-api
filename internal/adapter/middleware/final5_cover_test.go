package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/LurusTech/lurus-hub/internal/pkg/setting"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// jimeng: an unparseable body aborts with 400 before any rewrite.
func TestJimengRequestConvert_BadBody_400(t *testing.T) {
	r := gin.New()
	r.POST("/jimeng", JimengRequestConvert(), func(c *gin.Context) { c.Status(http.StatusOK) })
	req := httptest.NewRequest(http.MethodPost, "/jimeng?Action=Submit", strings.NewReader(`{bad json`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 for invalid jimeng body", w.Code)
	}
}

// sensitive_action: an authenticated id that maps to no user row → 500.
func TestGetUserVerificationStatus_UserNotFound_500(t *testing.T) {
	_, cleanup := setupCoverDB(t)
	defer cleanup()
	c, w := newTestContext(http.MethodGet, "/v", "", "")
	c.Set("id", 4242424) // no such user
	GetUserVerificationStatus(c)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500 when user lookup fails", w.Code)
	}
}

// validateJWT audience enforcement: a token whose aud is not in the configured
// allowlist is rejected even with a valid signature and issuer.
func TestAdminJWTAuth_AudienceMismatch_401(t *testing.T) {
	ctx := setupIntegrationTest(t)
	defer ctx.Cleanup()

	prev := oidcAllowedAudiences
	oidcAllowedAudiences = []string{"expected-client"}
	defer func() { oidcAllowedAudiences = prev }()

	claims := adminClaims(ctx.Issuer, map[string]interface{}{"admin": map[string]interface{}{}})
	claims.Audience = jwt.ClaimStrings{"some-other-client"}
	token := ctx.createJWT(t, claims)

	w := doBearer(mountJWT(AdminJWTAuth()), token)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401 for audience mismatch; body=%s", w.Code, w.Body.String())
	}
}

// model-rate-limit redis: total token-bucket AND success-count both configured,
// exercising the combined redis handler path over several requests.
func TestModelRateLimit_Redis_TotalAndSuccess(t *testing.T) {
	cleanup := withRedis(t)
	defer cleanup()

	prevEnabled := setting.ModelRequestRateLimitEnabled
	prevCount := setting.ModelRequestRateLimitCount
	prevSuccess := setting.ModelRequestRateLimitSuccessCount
	setting.ModelRequestRateLimitEnabled = true
	setting.ModelRequestRateLimitCount = 5
	setting.ModelRequestRateLimitSuccessCount = 5
	defer func() {
		setting.ModelRequestRateLimitEnabled = prevEnabled
		setting.ModelRequestRateLimitCount = prevCount
		setting.ModelRequestRateLimitSuccessCount = prevSuccess
	}()

	// A couple of requests within budget should pass and be recorded.
	for i := 0; i < 2; i++ {
		if code := runModelRateLimit(771234); code != http.StatusOK {
			t.Errorf("request %d = %d, want 200 within budget", i, code)
		}
	}
}

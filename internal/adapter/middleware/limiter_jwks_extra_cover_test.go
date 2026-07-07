package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/LurusTech/lurus-hub/internal/pkg/common"
	"github.com/LurusTech/lurus-hub/internal/pkg/setting"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// -------------------- memoryRateLimitHandler (RedisEnabled=false) --------------------

func runModelRLMem(uid int) int {
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("id", uid); c.Next() })
	r.GET("/m", ModelRequestRateLimit(), func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/m", nil))
	return w.Code
}

func setModelRateLimit(t *testing.T, redisOn bool, total, success int) {
	t.Helper()
	prevRedis := common.RedisEnabled
	prevEnabled := setting.ModelRequestRateLimitEnabled
	prevCount := setting.ModelRequestRateLimitCount
	prevSuccess := setting.ModelRequestRateLimitSuccessCount
	common.RedisEnabled = redisOn
	setting.ModelRequestRateLimitEnabled = true
	setting.ModelRequestRateLimitCount = total
	setting.ModelRequestRateLimitSuccessCount = success
	t.Cleanup(func() {
		common.RedisEnabled = prevRedis
		setting.ModelRequestRateLimitEnabled = prevEnabled
		setting.ModelRequestRateLimitCount = prevCount
		setting.ModelRequestRateLimitSuccessCount = prevSuccess
	})
}

// In-memory total-count limit trips on the second request within the window.
func TestModelRateLimit_Mem_Total(t *testing.T) {
	setModelRateLimit(t, false /*redis off → memory handler*/, 1 /*total*/, 1000 /*success*/)
	if code := runModelRLMem(660101); code != http.StatusOK {
		t.Fatalf("first = %d, want 200", code)
	}
	if code := runModelRLMem(660101); code != http.StatusTooManyRequests {
		t.Errorf("second = %d, want 429 (memory total-count)", code)
	}
	if code := runModelRLMem(660102); code != http.StatusOK {
		t.Errorf("other user = %d, want 200 (memory buckets isolate by user)", code)
	}
}

// In-memory success-count limit trips on the second request.
func TestModelRateLimit_Mem_Success(t *testing.T) {
	setModelRateLimit(t, false, 0 /*skip total*/, 1 /*success*/)
	if code := runModelRLMem(660201); code != http.StatusOK {
		t.Fatalf("first = %d, want 200", code)
	}
	if code := runModelRLMem(660201); code != http.StatusTooManyRequests {
		t.Errorf("second = %d, want 429 (memory success-count)", code)
	}
}

// -------------------- redisRateLimitHandler token-bucket total path --------------------

// With RedisEnabled + a positive total budget, the token-bucket path denies the
// burst that exceeds the per-window total. Exercised over the shared miniredis.
func TestModelRateLimit_Redis_TotalCount_MiniRedis(t *testing.T) {
	_, _, cleanup := withMiniRedis(t)
	defer cleanup()
	setModelRateLimit(t, true /*redis on*/, 1 /*total budget 1*/, 1000 /*success high*/)

	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("id", 660301); c.Next() })
	r.GET("/m", ModelRequestRateLimit(), func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })
	run := func() int {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/m", nil))
		return w.Code
	}
	first, second := run(), run()
	if first == http.StatusOK && second == http.StatusOK {
		if third := run(); third == http.StatusOK {
			t.Errorf("token-bucket total limit never tripped: %d/%d/%d", first, second, third)
		}
	}
}

// -------------------- VerifyIDTokenWithJWKS error branches --------------------

// Nil manager (OIDC not initialised) → explicit error.
func TestVerifyIDToken_NilManager(t *testing.T) {
	prev := jwksManager
	jwksManager = nil
	defer func() { jwksManager = prev }()

	var claims OIDCClaims
	if _, err := VerifyIDTokenWithJWKS("x.y.z", &claims); err == nil {
		t.Errorf("expected error when jwksManager is nil")
	}
}

// When the initial JWKS fetch fails, the manager is still constructed but marks
// itself updateFailed (the failure branch of NewJWKSManagerWithContext).
func TestNewJWKSManagerWithContext_InitialFetchFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	mgr := NewJWKSManagerWithContext(ctx, srv.URL)
	if mgr == nil {
		t.Fatalf("manager must be non-nil even when the initial fetch fails")
	}
	if !mgr.updateFailed {
		t.Errorf("updateFailed = false, want true after a failed initial JWKS fetch")
	}
}

// A token signed with a non-RSA method is rejected by the keyfunc guard.
func TestVerifyIDToken_WrongSigningMethod(t *testing.T) {
	ctx := setupIntegrationTest(t)
	defer ctx.Cleanup()

	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Issuer:    ctx.Issuer,
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	})
	signed, err := tok.SignedString([]byte("hmac-secret"))
	if err != nil {
		t.Fatalf("sign HS256: %v", err)
	}
	var claims OIDCClaims
	if _, err := VerifyIDTokenWithJWKS(signed, &claims); err == nil {
		t.Errorf("expected error for non-RSA signing method")
	}
}

// An RSA token lacking a kid header cannot be matched to a key → error.
func TestVerifyIDToken_MissingKid(t *testing.T) {
	ctx := setupIntegrationTest(t)
	defer ctx.Cleanup()

	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.RegisteredClaims{
		Issuer:    ctx.Issuer,
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	})
	// Deliberately do NOT set tok.Header["kid"].
	signed, err := tok.SignedString(ctx.PrivateKey)
	if err != nil {
		t.Fatalf("sign RS256: %v", err)
	}
	var claims OIDCClaims
	if _, err := VerifyIDTokenWithJWKS(signed, &claims); err == nil {
		t.Errorf("expected error for token missing kid header")
	}
}

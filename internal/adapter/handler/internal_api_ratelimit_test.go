package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/LurusTech/lurus-hub/internal/adapter/middleware"
	"github.com/LurusTech/lurus-hub/internal/pkg/common"
	"github.com/gin-gonic/gin"
)

// buildRateLimitedRouter mounts InternalApiAuth + a per-key InternalApiRateLimit
// bucket in front of a trivial 200 handler. It deliberately uses its own router
// (not SetupIntegrationRouter's) so the test exercises the real middleware
// chain order — auth first (stamps key id), rate-limit second (keys on it).
func buildRateLimitedRouter(maxNum int, durationSec int64, mark string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	g := r.Group("/internal")
	g.Use(middleware.InternalApiAuth())
	g.Use(middleware.InternalApiRateLimit(maxNum, durationSec, mark))
	g.GET("/ping", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })
	return r
}

func pingWithKey(r *gin.Engine, apiKey string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("GET", "/internal/ping", nil)
	req.Header.Set("X-API-Key", apiKey)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// TestInteg_InternalRateLimit_PerKey429 verifies that a single authenticated key
// is throttled after maxNum requests (429 + Retry-After), and that a DIFFERENT
// key gets its own bucket and is not affected — i.e. the limiter keys on the
// API-key id, not the shared client IP. (P0-3)
func TestInteg_InternalRateLimit_PerKey429(t *testing.T) {
	_, cleanup := SetupIntegrationRouter(t)
	t.Cleanup(cleanup)

	const maxNum = 3
	// Unique mark so the package-global in-memory limiter can't collide with any
	// other test's bucket.
	r := buildRateLimitedRouter(maxNum, 60, "TEST_P0_3_A")

	// First maxNum requests on the all-scopes key (id=1) must pass.
	for i := 0; i < maxNum; i++ {
		w := pingWithKey(r, testApiKeyAllScopes)
		if w.Code != http.StatusOK {
			t.Fatalf("request %d/%d should pass, got %d", i+1, maxNum, w.Code)
		}
	}

	// The next request on the SAME key must be throttled.
	w := pingWithKey(r, testApiKeyAllScopes)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 after %d requests on same key, got %d", maxNum, w.Code)
	}
	if w.Header().Get("Retry-After") == "" {
		t.Error("429 response must carry a Retry-After header")
	}

	// A DIFFERENT key (read-only, id=2) has a separate bucket — not throttled.
	w2 := pingWithKey(r, testApiKeyReadOnly)
	if w2.Code != http.StatusOK {
		t.Fatalf("different key must have its own bucket, got %d", w2.Code)
	}
}

// TestInteg_InternalRateLimit_Disabled verifies the global kill-switch: when
// InternalApiRateLimitEnable is false the limiter is a pass-through.
func TestInteg_InternalRateLimit_Disabled(t *testing.T) {
	_, cleanup := SetupIntegrationRouter(t)
	t.Cleanup(cleanup)

	// The factory reads the enable flag at build time, so set it BEFORE building.
	prev := common.InternalApiRateLimitEnable
	common.InternalApiRateLimitEnable = false
	t.Cleanup(func() { common.InternalApiRateLimitEnable = prev })

	r := buildRateLimitedRouter(1, 60, "TEST_P0_3_B")
	for i := 0; i < 5; i++ {
		w := pingWithKey(r, testApiKeyAllScopes)
		if w.Code != http.StatusOK {
			t.Fatalf("disabled limiter must pass all requests, got %d on req %d", w.Code, i+1)
		}
	}
}

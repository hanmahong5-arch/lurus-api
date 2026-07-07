package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/LurusTech/lurus-hub/internal/pkg/common"
	"github.com/LurusTech/lurus-hub/internal/pkg/setting"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// redisAddr returns the address of a reachable test Redis, or "" to skip.
// Honors TEST_REDIS_ADDR, else tries the disposable-container default.
func redisAddrOrSkip(t *testing.T) *redis.Client {
	t.Helper()
	addr := os.Getenv("TEST_REDIS_ADDR")
	if addr == "" {
		addr = "127.0.0.1:6399"
	}
	rdb := redis.NewClient(&redis.Options{Addr: addr})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		_ = rdb.Close()
		t.Skipf("no test Redis at %s: %v", addr, err)
	}
	return rdb
}

// withRedis installs a real Redis client as common.RDB with RedisEnabled=true
// and returns a cleanup that flushes and restores prior globals.
func withRedis(t *testing.T) func() {
	t.Helper()
	rdb := redisAddrOrSkip(t)
	_ = rdb.FlushDB(context.Background()).Err()

	prevRDB := common.RDB
	prevEnabled := common.RedisEnabled
	common.RDB = rdb
	common.RedisEnabled = true
	return func() {
		_ = rdb.FlushDB(context.Background()).Err()
		_ = rdb.Close()
		common.RDB = prevRDB
		common.RedisEnabled = prevEnabled
	}
}

// -------------------- rate-limit.go redis path --------------------

func TestRedisRateLimiterKeyed_UnderThenOverLimit(t *testing.T) {
	cleanup := withRedis(t)
	defer cleanup()

	// First request under the limit is admitted (LPush path).
	c1, _ := newTestContext(http.MethodGet, "/x", "", "")
	redisRateLimiterKeyed(c1, 1, 3600, "RL", "clientA")
	if c1.IsAborted() {
		t.Fatalf("first redis-limited request must pass")
	}
	// Second request within the window is rejected with 429 + Retry-After.
	c2, _ := newTestContext(http.MethodGet, "/x", "", "")
	redisRateLimiterKeyed(c2, 1, 3600, "RL", "clientA")
	if !c2.IsAborted() || c2.Writer.Status() != http.StatusTooManyRequests {
		t.Errorf("second request aborted=%v status=%d, want abort 429", c2.IsAborted(), c2.Writer.Status())
	}
}

func TestRedisRateLimiter_FactoryEndToEnd(t *testing.T) {
	cleanup := withRedis(t)
	defer cleanup()

	f := rateLimitFactory(1, 3600, "RLF")
	run := func(ip string) int {
		r := gin.New()
		r.GET("/f", f, func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })
		req := httptest.NewRequest(http.MethodGet, "/f", nil)
		req.RemoteAddr = ip + ":1"
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w.Code
	}
	if code := run("10.1.2.3"); code != http.StatusOK {
		t.Errorf("first = %d, want 200", code)
	}
	if code := run("10.1.2.3"); code != http.StatusTooManyRequests {
		t.Errorf("second = %d, want 429", code)
	}
}

// -------------------- model-rate-limit.go redis path --------------------

func runModelRateLimit(uid int) int {
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("id", uid); c.Next() })
	r.GET("/m", ModelRequestRateLimit(), func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/m", nil))
	return w.Code
}

func TestModelRateLimit_Redis_SuccessCountLimit(t *testing.T) {
	cleanup := withRedis(t)
	defer cleanup()

	prevEnabled := setting.ModelRequestRateLimitEnabled
	prevCount := setting.ModelRequestRateLimitCount
	prevSuccess := setting.ModelRequestRateLimitSuccessCount
	setting.ModelRequestRateLimitEnabled = true
	setting.ModelRequestRateLimitCount = 0 // skip token-bucket total path
	setting.ModelRequestRateLimitSuccessCount = 1
	defer func() {
		setting.ModelRequestRateLimitEnabled = prevEnabled
		setting.ModelRequestRateLimitCount = prevCount
		setting.ModelRequestRateLimitSuccessCount = prevSuccess
	}()

	// First success is recorded (recordRedisRequest); the second trips the
	// success-count limit (checkRedisRateLimit deny → 429).
	if code := runModelRateLimit(770001); code != http.StatusOK {
		t.Errorf("first = %d, want 200", code)
	}
	if code := runModelRateLimit(770001); code != http.StatusTooManyRequests {
		t.Errorf("second = %d, want 429 (success-count limit)", code)
	}
}

func TestModelRateLimit_Redis_TotalCountLimit(t *testing.T) {
	cleanup := withRedis(t)
	defer cleanup()

	prevEnabled := setting.ModelRequestRateLimitEnabled
	prevCount := setting.ModelRequestRateLimitCount
	prevSuccess := setting.ModelRequestRateLimitSuccessCount
	setting.ModelRequestRateLimitEnabled = true
	setting.ModelRequestRateLimitCount = 1 // token-bucket total budget = 1
	setting.ModelRequestRateLimitSuccessCount = 1000
	defer func() {
		setting.ModelRequestRateLimitEnabled = prevEnabled
		setting.ModelRequestRateLimitCount = prevCount
		setting.ModelRequestRateLimitSuccessCount = prevSuccess
	}()

	first := runModelRateLimit(770002)
	second := runModelRateLimit(770002)
	// The token bucket admits the first and must deny at least one of the
	// immediate follow-ups (429). Assert the pair is not both-OK.
	if first == http.StatusOK && second == http.StatusOK {
		third := runModelRateLimit(770002)
		if third == http.StatusOK {
			t.Errorf("token-bucket total limit never triggered: %d/%d/%d", first, second, third)
		}
	}
}

package middleware

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/LurusTech/lurus-hub/internal/adapter/repo"
	"github.com/LurusTech/lurus-hub/internal/pkg/common"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

// ---------------- cache / disable-cache / cors ----------------

func TestCache_Headers(t *testing.T) {
	r := gin.New()
	r.GET("/", Cache(), func(c *gin.Context) { c.Status(http.StatusOK) })
	r.GET("/asset", Cache(), func(c *gin.Context) { c.Status(http.StatusOK) })

	// Root → no-cache.
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	if got := w.Header().Get("Cache-Control"); got != "no-cache" {
		t.Errorf("root Cache-Control = %q, want no-cache", got)
	}
	// Non-root → max-age one week.
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, httptest.NewRequest(http.MethodGet, "/asset", nil))
	if got := w2.Header().Get("Cache-Control"); got != "max-age=604800" {
		t.Errorf("asset Cache-Control = %q, want max-age=604800", got)
	}
}

func TestDisableCache_Headers(t *testing.T) {
	r := gin.New()
	r.GET("/x", DisableCache(), func(c *gin.Context) { c.Status(http.StatusOK) })
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/x", nil))
	if w.Header().Get("Pragma") != "no-cache" {
		t.Errorf("Pragma header not set")
	}
	if w.Header().Get("Expires") != "0" {
		t.Errorf("Expires header not set")
	}
}

func TestCORS_HandlesPreflight(t *testing.T) {
	r := gin.New()
	r.Use(CORS())
	r.GET("/x", func(c *gin.Context) { c.Status(http.StatusOK) })
	req := httptest.NewRequest(http.MethodOptions, "/x", nil)
	req.Header.Set("Origin", "https://www.lurus.cn")
	req.Header.Set("Access-Control-Request-Method", "GET")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	// gin-contrib/cors aborts preflight with 204.
	if w.Code != http.StatusNoContent && w.Code != http.StatusOK {
		t.Errorf("preflight status = %d, want 204/200", w.Code)
	}
}

// ---------------- request-id / stats / logger / recover ----------------

func TestRequestId_SetsHeaderAndContext(t *testing.T) {
	var seen string
	r := gin.New()
	r.GET("/x", RequestId(), func(c *gin.Context) {
		seen = c.GetString(common.RequestIdKey)
		c.Status(http.StatusOK)
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/x", nil))
	if seen == "" {
		t.Errorf("request id not set in context")
	}
	if w.Header().Get(common.RequestIdKey) == "" {
		t.Errorf("request id header not set")
	}
}

func TestStatsMiddleware_TracksActiveConnections(t *testing.T) {
	r := gin.New()
	r.GET("/x", StatsMiddleware(), func(c *gin.Context) {
		// During the request, one connection is active.
		if got := GetStats().ActiveConnections; got < 1 {
			t.Errorf("active connections during request = %d, want >=1", got)
		}
		c.Status(http.StatusOK)
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/x", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	// After the request completes the counter returns to its prior value.
	if got := GetStats().ActiveConnections; got != 0 {
		t.Errorf("active connections after request = %d, want 0", got)
	}
}

func TestSetUpLogger_DoesNotPanic(t *testing.T) {
	r := gin.New()
	SetUpLogger(r) // registers a formatter; must not panic
	r.GET("/x", RequestId(), func(c *gin.Context) { c.Status(http.StatusOK) })
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/x", nil))
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

func TestRelayPanicRecover_Catches(t *testing.T) {
	r := gin.New()
	r.GET("/boom", RelayPanicRecover(), func(c *gin.Context) {
		panic("kaboom")
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/boom", nil))
	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500 after recovered panic", w.Code)
	}
}

func TestRelayPanicRecover_PassesThrough(t *testing.T) {
	r := gin.New()
	r.GET("/ok", RelayPanicRecover(), func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/ok", nil))
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 for non-panicking handler", w.Code)
	}
}

// ---------------- gzip DecompressRequestMiddleware ----------------

func TestDecompressRequestMiddleware_GET_PassesThrough(t *testing.T) {
	r := gin.New()
	r.GET("/x", DecompressRequestMiddleware(), func(c *gin.Context) { c.Status(http.StatusOK) })
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/x", nil))
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

func TestDecompressRequestMiddleware_Gzip_Decodes(t *testing.T) {
	var body []byte
	r := gin.New()
	r.POST("/x", DecompressRequestMiddleware(), func(c *gin.Context) {
		b, _ := io.ReadAll(c.Request.Body)
		body = b
		c.Status(http.StatusOK)
	})

	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	_, _ = gw.Write([]byte(`{"model":"gpt-4o"}`))
	_ = gw.Close()

	req := httptest.NewRequest(http.MethodPost, "/x", &buf)
	req.Header.Set("Content-Encoding", "gzip")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if string(body) != `{"model":"gpt-4o"}` {
		t.Errorf("decoded body = %q, want the original JSON", string(body))
	}
}

func TestDecompressRequestMiddleware_BadGzip_400(t *testing.T) {
	r := gin.New()
	r.POST("/x", DecompressRequestMiddleware(), func(c *gin.Context) { c.Status(http.StatusOK) })
	req := httptest.NewRequest(http.MethodPost, "/x", bytes.NewReader([]byte("not-gzip-data")))
	req.Header.Set("Content-Encoding", "gzip")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 for malformed gzip stream", w.Code)
	}
}

func TestDecompressRequestMiddleware_Plain_Wrapped(t *testing.T) {
	r := gin.New()
	r.POST("/x", DecompressRequestMiddleware(), func(c *gin.Context) {
		b, _ := io.ReadAll(c.Request.Body)
		if string(b) != "hello" {
			t.Errorf("body = %q, want hello", string(b))
		}
		c.Status(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodPost, "/x", bytes.NewReader([]byte("hello")))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

// ---------------- jimeng / kling adapters ----------------

func TestJimengRequestConvert_MissingAction_400(t *testing.T) {
	r := gin.New()
	r.POST("/jimeng", JimengRequestConvert(), func(c *gin.Context) { c.Status(http.StatusOK) })
	req := httptest.NewRequest(http.MethodPost, "/jimeng", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 for missing Action", w.Code)
	}
}

func TestJimengRequestConvert_Generate_RewritesPath(t *testing.T) {
	var newPath string
	r := gin.New()
	r.POST("/jimeng", JimengRequestConvert(), func(c *gin.Context) {
		newPath = c.Request.URL.Path
		c.Status(http.StatusOK)
	})
	body := `{"req_key":"jimeng_t2v","prompt":"a cat"}`
	req := httptest.NewRequest(http.MethodPost, "/jimeng?Action=CVSync2AsyncSubmitTask", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	if newPath != "/v1/video/generations" {
		t.Errorf("path = %q, want /v1/video/generations", newPath)
	}
}

func TestJimengRequestConvert_FetchResult_RewritesToGet(t *testing.T) {
	var method string
	r := gin.New()
	r.POST("/jimeng", JimengRequestConvert(), func(c *gin.Context) {
		method = c.Request.Method
		c.Status(http.StatusOK)
	})
	body := `{"req_key":"jimeng_t2v","task_id":"task-123"}`
	req := httptest.NewRequest(http.MethodPost, "/jimeng?Action=CVSync2AsyncGetResult", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	if method != http.MethodGet {
		t.Errorf("method = %q, want GET for fetch-result", method)
	}
}

func TestJimengRequestConvert_FetchResult_MissingTaskId_400(t *testing.T) {
	r := gin.New()
	r.POST("/jimeng", JimengRequestConvert(), func(c *gin.Context) { c.Status(http.StatusOK) })
	body := `{"req_key":"jimeng_t2v"}`
	req := httptest.NewRequest(http.MethodPost, "/jimeng?Action=CVSync2AsyncGetResult", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 for missing task_id", w.Code)
	}
}

func TestKlingRequestConvert_RewritesPath(t *testing.T) {
	var newPath string
	r := gin.New()
	r.POST("/kling", KlingRequestConvert(), func(c *gin.Context) {
		newPath = c.Request.URL.Path
		c.Status(http.StatusOK)
	})
	body := `{"model_name":"kling-v1","prompt":"a dog"}`
	req := httptest.NewRequest(http.MethodPost, "/kling", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if newPath != "/v1/video/generations" {
		t.Errorf("path = %q, want /v1/video/generations", newPath)
	}
}

func TestKlingRequestConvert_BadBody_PassesThrough(t *testing.T) {
	// Kling fails open: a body it can't parse just proceeds unchanged.
	r := gin.New()
	r.POST("/kling", KlingRequestConvert(), func(c *gin.Context) { c.Status(http.StatusOK) })
	req := httptest.NewRequest(http.MethodPost, "/kling", bytes.NewReader([]byte(`{bad`)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 (kling fails open on bad body)", w.Code)
	}
}

// ---------------- sensitive_action ----------------

func TestGetUserVerificationStatus_Unauthenticated_401(t *testing.T) {
	c, w := newTestContext(http.MethodGet, "/v", "", "")
	GetUserVerificationStatus(c) // no "id" set → 401
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401 for unauthenticated user", w.Code)
	}
}

func TestGetUserVerificationStatus_HappyPath(t *testing.T) {
	_, cleanup := setupCoverDB(t)
	defer cleanup()
	user := &repo.User{Username: "veri", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Email: "veri@local", TenantId: "default"}
	if err := repo.DB.Create(user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	c, w := newTestContext(http.MethodGet, "/v", "", "")
	c.Set("id", user.Id)
	GetUserVerificationStatus(c)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
}

// ---------------- secure_verification: ClearSecureVerification ----------------

func TestClearSecureVerification(t *testing.T) {
	r := gin.New()
	store := cookie.NewStore([]byte("clear-secret"))
	r.Use(sessions.Sessions("session", store))
	r.POST("/logout", func(c *gin.Context) {
		ClearSecureVerification(c)
		c.Status(http.StatusOK)
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/logout", nil))
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

// ---------------- pool_balance_check ----------------

func mountPoolBalance(tenantID string) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(func(c *gin.Context) {
		if tenantID != "" {
			c.Set("tenant_context", &TenantContext{TenantID: tenantID})
		}
		c.Next()
	})
	r.GET("/relay", PoolBalanceCheck(), func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })
	return r
}

func TestPoolBalanceCheck_NoTenant_Passes(t *testing.T) {
	_, cleanup := setupCoverDB(t)
	defer cleanup()
	r := mountPoolBalance("") // no tenant context
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/relay", nil))
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 when no tenant identified", w.Code)
	}
}

func TestPoolBalanceCheck_NoPoolRow_Passes(t *testing.T) {
	_, cleanup := setupCoverDB(t)
	defer cleanup()
	r := mountPoolBalance("tenant-nopool")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/relay", nil))
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 when tenant has no pool row (unlimited default)", w.Code)
	}
}

func TestPoolBalanceCheck_UnlimitedPool_Passes(t *testing.T) {
	_, cleanup := setupCoverDB(t)
	defer cleanup()
	if _, err := repo.CreateTenantCreditPool("tenant-unlim", 1, repo.PoolMaxBalanceUnlimited, repo.PoolResetMonthly, 80); err != nil {
		t.Fatalf("create pool: %v", err)
	}
	r := mountPoolBalance("tenant-unlim")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/relay", nil))
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 for unlimited pool", w.Code)
	}
}

func TestPoolBalanceCheck_ExhaustedPool_402(t *testing.T) {
	_, cleanup := setupCoverDB(t)
	defer cleanup()
	// Finite ceiling, zero balance → exhausted → 402. This is the cash-path
	// gate: a Reseller with a drained pool must be blocked before relay spend.
	if _, err := repo.CreateTenantCreditPool("tenant-broke", 1, 10000, repo.PoolResetMonthly, 80); err != nil {
		t.Fatalf("create pool: %v", err)
	}
	r := mountPoolBalance("tenant-broke")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/relay", nil))
	if w.Code != http.StatusPaymentRequired {
		t.Errorf("status = %d, want 402 for exhausted pool; body=%s", w.Code, w.Body.String())
	}
}

func TestPoolBalanceCheck_HealthyPool_Passes(t *testing.T) {
	_, cleanup := setupCoverDB(t)
	defer cleanup()
	pool, err := repo.CreateTenantCreditPool("tenant-flush", 1, 10000, repo.PoolResetMonthly, 80)
	if err != nil {
		t.Fatalf("create pool: %v", err)
	}
	pool.CurrentBalance = 5000
	if err := repo.DB.Save(pool).Error; err != nil {
		t.Fatalf("save balance: %v", err)
	}
	r := mountPoolBalance("tenant-flush")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/relay", nil))
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 for healthy pool; body=%s", w.Code, w.Body.String())
	}
}

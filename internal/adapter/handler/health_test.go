package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/LurusTech/lurus-hub/internal/adapter/repo"
	"github.com/LurusTech/lurus-hub/internal/pkg/common"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

var healthTestDBCounter atomic.Int64

// callHealth invokes GetHealthDetailed against a fresh test context and returns
// the recorder plus the wall-clock the handler took (for the zombie-pod guard).
func callHealth(t *testing.T) (*httptest.ResponseRecorder, time.Duration) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/health", nil)
	start := time.Now()
	GetHealthDetailed(c)
	return w, time.Since(start)
}

func healthDBCheck(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Status string            `json:"status"`
		Checks map[string]string `json:"checks"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("parse health body: %v, raw=%s", err, w.Body.String())
	}
	return body.Checks["database"]
}

func TestGetHealthDetailed_DBOK(t *testing.T) {
	dbName := fmt.Sprintf("file:healthok%d?mode=memory&cache=shared", healthTestDBCounter.Add(1))
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	prevDB, prevRedis := repo.DB, common.RedisEnabled
	repo.DB, common.RedisEnabled = db, false
	t.Cleanup(func() {
		repo.DB, common.RedisEnabled = prevDB, prevRedis
		if sqlDB, e := db.DB(); e == nil {
			_ = sqlDB.Close()
		}
	})

	w, _ := callHealth(t)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body=%s)", w.Code, w.Body.String())
	}
	if got := healthDBCheck(t, w); got != "ok" {
		t.Fatalf("expected database=ok, got %q", got)
	}
}

func TestGetHealthDetailed_ClosedDBReturns503Fast(t *testing.T) {
	dbName := fmt.Sprintf("file:healthdown%d?mode=memory&cache=shared", healthTestDBCounter.Add(1))
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	// Close the underlying pool so PingContext fails fast — stands in for an
	// unreachable DB host without a real network hang. The elapsed<1s assertion
	// is the zombie-pod regression guard: the handler must return within its
	// bounded ping deadline (HealthDBPingTimeout default 1.5s), never wedge.
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql.DB: %v", err)
	}
	_ = sqlDB.Close()

	prevDB, prevRedis := repo.DB, common.RedisEnabled
	repo.DB, common.RedisEnabled = db, false
	t.Cleanup(func() { repo.DB, common.RedisEnabled = prevDB, prevRedis })

	w, elapsed := callHealth(t)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d (body=%s)", w.Code, w.Body.String())
	}
	if got := healthDBCheck(t, w); got != "unreachable" {
		t.Fatalf("expected database=unreachable, got %q", got)
	}
	if elapsed >= time.Second {
		t.Fatalf("zombie-pod guard: /api/health must return well under 1s, took %v", elapsed)
	}
}

func TestGetHealthDetailed_NilDBNotConfigured(t *testing.T) {
	prevDB, prevRedis := repo.DB, common.RedisEnabled
	repo.DB, common.RedisEnabled = nil, false
	t.Cleanup(func() { repo.DB, common.RedisEnabled = prevDB, prevRedis })

	w, _ := callHealth(t)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for nil DB, got %d (body=%s)", w.Code, w.Body.String())
	}
	if got := healthDBCheck(t, w); got != "not_configured" {
		t.Fatalf("expected database=not_configured, got %q", got)
	}
}

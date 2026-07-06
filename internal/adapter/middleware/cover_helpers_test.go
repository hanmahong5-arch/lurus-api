package middleware

import (
	"fmt"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/LurusTech/lurus-hub/internal/adapter/repo"
	"github.com/LurusTech/lurus-hub/internal/domain/entity"
	"github.com/LurusTech/lurus-hub/internal/pkg/common"
	"github.com/LurusTech/lurus-hub/internal/pkg/constant"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

var coverDBCounter atomic.Int64

func init() {
	// The production body-size guard defaults to 0 MB until config init runs
	// (common/init.go), which no unit test triggers. Without a positive cap,
	// GetRequestBody rejects every request body as "too large". Match the
	// production default so body-parsing paths are exercised, not short-circuited.
	if constant.MaxRequestBodyMB <= 0 {
		constant.MaxRequestBodyMB = 64
	}
}

// setupCoverDB spins up an isolated in-memory SQLite database wired into
// repo.DB and migrates the models the middleware layer touches. It mirrors the
// established setupScopeTestRouter pattern (auth_scope_test.go) — the middleware
// unit tests exercise real GORM rows, not mocks, so tenant-isolation and
// channel-selection assertions run against the same query path production uses.
// The returned cleanup restores every global it mutated so tests stay
// -count=1 safe and never leak state between cases.
func setupCoverDB(t *testing.T) (*gorm.DB, func()) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	dbName := fmt.Sprintf("file:cover_test_%d?mode=memory&cache=shared", coverDBCounter.Add(1))
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	tables := []interface{}{
		&repo.User{}, &repo.Token{}, &repo.Channel{}, &entity.Tenant{},
		&entity.TenantCreditPool{}, &entity.UserIdentityMapping{},
		&entity.TenantConfig{}, &entity.Release{}, &entity.ReleaseArtifact{},
		&entity.Ability{},
	}
	for _, tbl := range tables {
		if err := db.AutoMigrate(tbl); err != nil {
			if !strings.Contains(err.Error(), "already exists") {
				t.Fatalf("migrate %T: %v", tbl, err)
			}
		}
	}

	prevDB := repo.DB
	prevSQLite := common.UsingSQLite
	prevPG := common.UsingPostgreSQL
	prevRedis := common.RedisEnabled

	repo.DB = db
	repo.InitCol()
	common.UsingSQLite = true
	common.UsingPostgreSQL = false
	common.RedisEnabled = false

	common.OptionMapRWMutex.Lock()
	if common.OptionMap == nil {
		common.OptionMap = make(map[string]string)
	}
	common.OptionMapRWMutex.Unlock()

	cleanup := func() {
		repo.DB = prevDB
		common.UsingSQLite = prevSQLite
		common.UsingPostgreSQL = prevPG
		common.RedisEnabled = prevRedis
		sqlDB, _ := db.DB()
		if sqlDB != nil {
			_ = sqlDB.Close()
		}
	}
	return db, cleanup
}

// newTestContext builds a detached *gin.Context carrying a fully-formed
// *http.Request so package-internal helpers (getModelRequest, etc.) can be
// driven directly without mounting a full engine.
func newTestContext(method, path, body, contentType string) (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	c.Request = req
	return c, w
}

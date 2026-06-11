package handler

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/LurusTech/lurus-hub/internal/pkg/common"
	"github.com/LurusTech/lurus-hub/internal/adapter/middleware"
	"github.com/LurusTech/lurus-hub/internal/adapter/repo"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// Raw API key strings for test use
const (
	testApiKeyAllScopes  = "lurus_ik_testkey_all_scopes_0000000000"
	testApiKeyReadOnly   = "lurus_ik_testkey_readonly_00000000000"
)

var testDBCounter atomic.Int64

func hashTestKey(key string) string {
	h := sha256.Sum256([]byte(key))
	return hex.EncodeToString(h[:])
}

// SetupIntegrationRouter initializes an in-memory SQLite DB, seeds test data,
// registers internal API routes with auth middleware, and returns the router
// along with a cleanup function.
func SetupIntegrationRouter(t *testing.T) (*gin.Engine, func()) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	dbName := fmt.Sprintf("file:ctrltest%d?mode=memory&cache=shared", testDBCounter.Add(1))
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite :memory: %v", err)
	}

	tables := []interface{}{
		&repo.User{},
		&repo.Token{},
		&repo.Log{},
		&repo.InternalApiKey{},
		&repo.Option{},
		&repo.Setup{},
		&repo.Tenant{},
		&repo.UserIdentityMapping{},
		&repo.TenantConfig{},
	}
	for _, tbl := range tables {
		if err := db.AutoMigrate(tbl); err != nil {
			// SQLite index names are global; duplicates across tables are safe to ignore.
			if strings.Contains(err.Error(), "already exists") {
				continue
			}
			t.Fatalf("auto migrate failed for %T: %v", tbl, err)
		}
	}
	// Save previous state
	prevDB := repo.DB
	prevLogDB := repo.LOG_DB
	prevSQLite := common.UsingSQLite
	prevPG := common.UsingPostgreSQL
	prevRedis := common.RedisEnabled

	repo.DB = db
	repo.LOG_DB = db
	repo.InitCol() // Initialize dialect-specific column quoting for SQLite
	common.UsingSQLite = true
	common.UsingPostgreSQL = false
	common.RedisEnabled = false
	common.QuotaForNewUser = 0
	common.LogConsumeEnabled = false

	// Seed root user (id=1, role=100 admin)
	db.Create(&repo.User{
		Id:          1,
		Username:    "root",
		DisplayName: "Root",
		Role:        common.RoleRootUser,
		Status:      common.UserStatusEnabled,
		Email:       "root@test.local",
	})

	// Seed normal user (id=2)
	db.Create(&repo.User{
		Id:          2,
		Username:    "testuser",
		DisplayName: "Test User",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		Email:       "user@test.local",
	})

	// Seed API key with all scopes
	allScopes, _ := json.Marshal([]string{repo.ScopeAll})
	db.Create(&repo.InternalApiKey{
		Id:      1,
		Name:    "test-all-scopes",
		KeyHash: hashTestKey(testApiKeyAllScopes),
		Scopes:  string(allScopes),
		Enabled: true,
	})

	// Seed read-only API key
	readScopes, _ := json.Marshal([]string{
		repo.ScopeUserRead,
		repo.ScopeQuotaRead, repo.ScopeBalanceRead, repo.ScopeTokenRead,
	})
	db.Create(&repo.InternalApiKey{
		Id:      2,
		Name:    "test-read-only",
		KeyHash: hashTestKey(testApiKeyReadOnly),
		Scopes:  string(readScopes),
		Enabled: true,
	})

	// Build router with internal API routes
	router := gin.Default()
	internal := router.Group("/internal")
	internal.Use(middleware.InternalApiAuth())
	{
		internal.POST("/auth/login", middleware.RequireScope(repo.ScopeAuthLogin), InternalLogin)
		internal.GET("/user/:id", middleware.RequireScope(repo.ScopeUserRead), InternalGetUser)
		internal.GET("/user/by-email/:email", middleware.RequireScope(repo.ScopeUserRead), InternalGetUserByEmail)
		internal.GET("/user/by-phone/:phone", middleware.RequireScope(repo.ScopeUserRead), InternalGetUserByPhone)
		internal.POST("/user", middleware.RequireScope(repo.ScopeUserWrite), InternalCreateUser)
		internal.POST("/user/provision", middleware.RequireScope(repo.ScopeUserWrite), InternalProvisionUser)
		internal.PUT("/user/:id", middleware.RequireScope(repo.ScopeUserWrite), InternalUpdateUser)
		internal.DELETE("/user/:id", middleware.RequireScope(repo.ScopeUserDelete), InternalDeleteUser)
		internal.GET("/quota/user/:id", middleware.RequireScope(repo.ScopeQuotaRead), InternalGetUserQuota)
		internal.POST("/quota/adjust", middleware.RequireScope(repo.ScopeQuotaWrite), InternalAdjustQuota)
		internal.GET("/balance/user/:id", middleware.RequireScope(repo.ScopeBalanceRead), InternalGetUserBalance)
		internal.POST("/balance/topup", middleware.RequireScope(repo.ScopeBalanceWrite), InternalTopupBalance)
		internal.GET("/token/user/:id", middleware.RequireScope(repo.ScopeTokenRead), InternalGetUserTokens)
		internal.POST("/token", middleware.RequireScope(repo.ScopeTokenWrite), InternalCreateToken)
	}

	cleanup := func() {
		repo.DB = prevDB
		repo.LOG_DB = prevLogDB
		common.UsingSQLite = prevSQLite
		common.UsingPostgreSQL = prevPG
		common.RedisEnabled = prevRedis
		sqlDB, _ := db.DB()
		if sqlDB != nil {
			sqlDB.Close()
		}
	}
	return router, cleanup
}

// internalRequest builds and executes an HTTP request against the router.
func internalRequest(router *gin.Engine, method, path string, body interface{}, headers map[string]string) *httptest.ResponseRecorder {
	var req *http.Request
	if body != nil {
		data, _ := json.Marshal(body)
		req = httptest.NewRequest(method, path, bytes.NewReader(data))
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

// parseResponse unmarshals the response body into a generic map.
func parseResponse(t *testing.T, w *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var result map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse response body: %v, raw: %s", err, w.Body.String())
	}
	return result
}

// assertSuccess checks that the response indicates success.
func assertSuccess(t *testing.T, resp map[string]interface{}) {
	t.Helper()
	if success, ok := resp["success"].(bool); !ok || !success {
		t.Errorf("expected success=true, got %v", resp["success"])
	}
}

// assertErrorCode checks that the response contains the expected error code.
func assertErrorCode(t *testing.T, resp map[string]interface{}, code string) {
	t.Helper()
	if ec, ok := resp["error_code"].(string); !ok || ec != code {
		t.Errorf("expected error_code=%q, got %v", code, resp["error_code"])
	}
}

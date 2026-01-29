package controller

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/QuantumNous/lurus-api/internal/pkg/common"
	"github.com/QuantumNous/lurus-api/internal/server/middleware"
	"github.com/QuantumNous/lurus-api/internal/data/model"
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
		&model.User{},
		&model.Token{},
		&model.Log{},
		&model.InternalApiKey{},
		&model.Subscription{},
		&model.TopUp{},
		&model.Option{},
		&model.Setup{},
		&model.Tenant{},
		&model.UserIdentityMapping{},
		&model.TenantConfig{},
	}
	for _, tbl := range tables {
		if err := db.AutoMigrate(tbl); err != nil {
			t.Fatalf("auto migrate failed for %T: %v", tbl, err)
		}
	}
	// Save previous state
	prevDB := model.DB
	prevLogDB := model.LOG_DB
	prevSQLite := common.UsingSQLite
	prevPG := common.UsingPostgreSQL
	prevRedis := common.RedisEnabled

	model.DB = db
	model.LOG_DB = db
	common.UsingSQLite = true
	common.UsingPostgreSQL = false
	common.RedisEnabled = false
	common.QuotaForNewUser = 0
	common.LogConsumeEnabled = false

	// Seed root user (id=1, role=100 admin)
	rootPassword, _ := common.Password2Hash("rootpassword")
	db.Create(&model.User{
		Id:          1,
		Username:    "root",
		Password:    rootPassword,
		DisplayName: "Root",
		Role:        common.RoleRootUser,
		Status:      common.UserStatusEnabled,
		Email:       "root@test.local",
		Phone:       "10000000000",
		AffCode:     common.GetRandomString(8),
	})

	// Seed normal user (id=2)
	normalPassword, _ := common.Password2Hash("userpassword")
	db.Create(&model.User{
		Id:          2,
		Username:    "testuser",
		Password:    normalPassword,
		DisplayName: "Test User",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		Email:       "user@test.local",
		Phone:       "10000000001",
		AffCode:     common.GetRandomString(8),
	})

	// Seed API key with all scopes
	allScopes, _ := json.Marshal([]string{model.ScopeAll})
	db.Create(&model.InternalApiKey{
		Id:      1,
		Name:    "test-all-scopes",
		KeyHash: hashTestKey(testApiKeyAllScopes),
		Scopes:  string(allScopes),
		Enabled: true,
	})

	// Seed read-only API key
	readScopes, _ := json.Marshal([]string{
		model.ScopeUserRead, model.ScopeSubscriptionRead,
		model.ScopeQuotaRead, model.ScopeBalanceRead, model.ScopeTokenRead,
	})
	db.Create(&model.InternalApiKey{
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
		internal.POST("/auth/login", middleware.RequireScope(model.ScopeAuthLogin), InternalLogin)
		internal.GET("/user/:id", middleware.RequireScope(model.ScopeUserRead), InternalGetUser)
		internal.GET("/user/by-email/:email", middleware.RequireScope(model.ScopeUserRead), InternalGetUserByEmail)
		internal.GET("/user/by-phone/:phone", middleware.RequireScope(model.ScopeUserRead), InternalGetUserByPhone)
		internal.POST("/user", middleware.RequireScope(model.ScopeUserWrite), InternalCreateUser)
		internal.PUT("/user/:id", middleware.RequireScope(model.ScopeUserWrite), InternalUpdateUser)
		internal.DELETE("/user/:id", middleware.RequireScope(model.ScopeUserDelete), InternalDeleteUser)
		internal.GET("/subscription/user/:id", middleware.RequireScope(model.ScopeSubscriptionRead), InternalGetUserSubscription)
		internal.POST("/subscription/grant", middleware.RequireScope(model.ScopeSubscriptionWrite), InternalGrantSubscription)
		internal.GET("/quota/user/:id", middleware.RequireScope(model.ScopeQuotaRead), InternalGetUserQuota)
		internal.POST("/quota/adjust", middleware.RequireScope(model.ScopeQuotaWrite), InternalAdjustQuota)
		internal.GET("/balance/user/:id", middleware.RequireScope(model.ScopeBalanceRead), InternalGetUserBalance)
		internal.POST("/balance/topup", middleware.RequireScope(model.ScopeBalanceWrite), InternalTopupBalance)
		internal.GET("/token/user/:id", middleware.RequireScope(model.ScopeTokenRead), InternalGetUserTokens)
		internal.POST("/token", middleware.RequireScope(model.ScopeTokenWrite), InternalCreateToken)
	}

	cleanup := func() {
		model.DB = prevDB
		model.LOG_DB = prevLogDB
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

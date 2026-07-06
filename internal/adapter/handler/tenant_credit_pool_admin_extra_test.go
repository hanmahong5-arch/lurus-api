package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/LurusTech/lurus-hub/internal/adapter/repo"
	"github.com/LurusTech/lurus-hub/internal/domain/entity"
	"github.com/LurusTech/lurus-hub/internal/pkg/common"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

var adminPoolTestDBCounter atomic.Int64

type adminPoolCtx struct {
	router   *gin.Engine
	db       *gorm.DB
	tenantID string
	actorID  int
}

// setupAdminPoolRouter wires the reseller-facing credit-pool admin handlers
// against an isolated in-memory SQLite DB, seeding one tenant and an actor
// user whose "id" is injected into the gin context (mimicking RootJWTAuth).
func setupAdminPoolRouter(t *testing.T, actorHasWallet bool) *adminPoolCtx {
	t.Helper()
	gin.SetMode(gin.TestMode)

	dsn := fmt.Sprintf("file:adminpool%d?mode=memory&cache=shared", adminPoolTestDBCounter.Add(1))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	tables := []interface{}{
		&repo.User{}, &repo.Tenant{},
		&entity.TenantCreditPool{}, &entity.TenantCreditPoolDraw{},
	}
	for _, tbl := range tables {
		if err := db.AutoMigrate(tbl); err != nil && !strings.Contains(err.Error(), "already exists") {
			t.Fatalf("auto migrate %T: %v", tbl, err)
		}
	}

	prevDB := repo.DB
	prevLogDB := repo.LOG_DB
	prevSQLite := common.UsingSQLite
	prevPG := common.UsingPostgreSQL
	prevRedis := common.RedisEnabled

	repo.DB = db
	repo.LOG_DB = db
	repo.InitCol()
	common.UsingSQLite = true
	common.UsingPostgreSQL = false
	common.RedisEnabled = false

	now := time.Now()
	tenantID := "tenant-pooladmin"
	if err := db.Create(&repo.Tenant{
		Id: tenantID, Name: "PoolAdmin", Slug: "pooladmin",
		Status: repo.TenantStatusEnabled, IDPOrgID: "org_pa",
		CreatedAt: now, UpdatedAt: now,
	}).Error; err != nil {
		t.Fatalf("seed tenant: %v", err)
	}

	actor := &repo.User{
		Username: "resadmin", DisplayName: "Reseller Admin",
		Role: common.RoleRootUser, Status: common.UserStatusEnabled,
		Email: "res@test.local", TenantId: tenantID,
	}
	if actorHasWallet {
		acct := int64(9001)
		actor.LurusAccountID = &acct
	}
	if err := db.Create(actor).Error; err != nil {
		t.Fatalf("seed actor: %v", err)
	}

	router := gin.New()
	mockAuth := func(c *gin.Context) {
		c.Set("id", actor.Id)
		c.Set("role", actor.Role)
		c.Next()
	}
	g := router.Group("/api/v2/admin/tenants/:id/credit-pool", mockAuth)
	g.POST("", CreateCreditPool)
	g.GET("", GetCreditPool)
	g.POST("/topup", TopupCreditPool)
	g.GET("/usage", ListCreditPoolUsage)
	g.DELETE("", DeleteCreditPool)

	t.Cleanup(func() {
		repo.DB = prevDB
		repo.LOG_DB = prevLogDB
		common.UsingSQLite = prevSQLite
		common.UsingPostgreSQL = prevPG
		common.RedisEnabled = prevRedis
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})

	return &adminPoolCtx{router: router, db: db, tenantID: tenantID, actorID: actor.Id}
}

func (ctx *adminPoolCtx) do(method, path string, body interface{}) *httptest.ResponseRecorder {
	var rdr *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rdr = bytes.NewReader(b)
	} else {
		rdr = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, rdr)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	ctx.router.ServeHTTP(w, req)
	return w
}

func decodeJSON(t *testing.T, w *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var m map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &m); err != nil {
		t.Fatalf("decode: %v — raw: %s", err, w.Body.String())
	}
	return m
}

func TestCreateCreditPool_Success(t *testing.T) {
	ctx := setupAdminPoolRouter(t, false)
	w := ctx.do(http.MethodPost, "/api/v2/admin/tenants/"+ctx.tenantID+"/credit-pool",
		map[string]interface{}{"max_balance": 10000, "reset_period": "monthly", "alert_threshold_pct": 80})
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201, body: %s", w.Code, w.Body.String())
	}
	resp := decodeJSON(t, w)
	if resp["success"] != true {
		t.Errorf("success = %v", resp["success"])
	}
	// DB side-effect: pool row exists with the right ceiling.
	pool, err := repo.GetTenantCreditPool(ctx.tenantID)
	if err != nil || pool == nil {
		t.Fatalf("pool not persisted: %v", err)
	}
	if pool.MaxBalance != 10000 {
		t.Errorf("MaxBalance = %d, want 10000", pool.MaxBalance)
	}
}

func TestCreateCreditPool_TenantNotFound(t *testing.T) {
	ctx := setupAdminPoolRouter(t, false)
	w := ctx.do(http.MethodPost, "/api/v2/admin/tenants/ghost/credit-pool",
		map[string]interface{}{"max_balance": 1000})
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404, body: %s", w.Code, w.Body.String())
	}
}

func TestCreateCreditPool_Conflict(t *testing.T) {
	ctx := setupAdminPoolRouter(t, false)
	path := "/api/v2/admin/tenants/" + ctx.tenantID + "/credit-pool"
	first := ctx.do(http.MethodPost, path, map[string]interface{}{"max_balance": 1000})
	if first.Code != http.StatusCreated {
		t.Fatalf("first create status = %d", first.Code)
	}
	second := ctx.do(http.MethodPost, path, map[string]interface{}{"max_balance": 2000})
	if second.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409, body: %s", second.Code, second.Body.String())
	}
	resp := decodeJSON(t, second)
	if resp["error_code"] != "POOL_ALREADY_EXISTS" {
		t.Errorf("error_code = %v, want POOL_ALREADY_EXISTS", resp["error_code"])
	}
}

func TestCreateCreditPool_InvalidMaxBalance(t *testing.T) {
	ctx := setupAdminPoolRouter(t, false)
	w := ctx.do(http.MethodPost, "/api/v2/admin/tenants/"+ctx.tenantID+"/credit-pool",
		map[string]interface{}{"max_balance": -5})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400, body: %s", w.Code, w.Body.String())
	}
}

func TestCreateCreditPool_UnlimitedAllowed(t *testing.T) {
	ctx := setupAdminPoolRouter(t, false)
	w := ctx.do(http.MethodPost, "/api/v2/admin/tenants/"+ctx.tenantID+"/credit-pool",
		map[string]interface{}{"max_balance": repo.PoolMaxBalanceUnlimited})
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 for unlimited (-1), body: %s", w.Code, w.Body.String())
	}
}

func TestGetCreditPool_NotFound(t *testing.T) {
	ctx := setupAdminPoolRouter(t, false)
	w := ctx.do(http.MethodGet, "/api/v2/admin/tenants/"+ctx.tenantID+"/credit-pool", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404, body: %s", w.Code, w.Body.String())
	}
	resp := decodeJSON(t, w)
	if resp["error_code"] != "POOL_NOT_FOUND" {
		t.Errorf("error_code = %v, want POOL_NOT_FOUND", resp["error_code"])
	}
}

func TestGetCreditPool_Found(t *testing.T) {
	ctx := setupAdminPoolRouter(t, false)
	ctx.do(http.MethodPost, "/api/v2/admin/tenants/"+ctx.tenantID+"/credit-pool",
		map[string]interface{}{"max_balance": 5000})
	w := ctx.do(http.MethodGet, "/api/v2/admin/tenants/"+ctx.tenantID+"/credit-pool", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body: %s", w.Code, w.Body.String())
	}
	resp := decodeJSON(t, w)
	data := resp["data"].(map[string]interface{})
	if data["max_balance"].(float64) != 5000 {
		t.Errorf("max_balance = %v, want 5000", data["max_balance"])
	}
}

func TestTopupCreditPool_PoolNotFound(t *testing.T) {
	ctx := setupAdminPoolRouter(t, true)
	w := ctx.do(http.MethodPost, "/api/v2/admin/tenants/"+ctx.tenantID+"/credit-pool/topup",
		map[string]interface{}{"amount": 100})
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404, body: %s", w.Code, w.Body.String())
	}
}

func TestTopupCreditPool_BadAmount(t *testing.T) {
	ctx := setupAdminPoolRouter(t, true)
	ctx.do(http.MethodPost, "/api/v2/admin/tenants/"+ctx.tenantID+"/credit-pool",
		map[string]interface{}{"max_balance": 5000})
	// amount omitted -> binding:"required" fails
	w := ctx.do(http.MethodPost, "/api/v2/admin/tenants/"+ctx.tenantID+"/credit-pool/topup",
		map[string]interface{}{"reason": "x"})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400, body: %s", w.Code, w.Body.String())
	}
}

func TestTopupCreditPool_ActorNoWallet(t *testing.T) {
	ctx := setupAdminPoolRouter(t, false) // actor has no LurusAccountID
	ctx.do(http.MethodPost, "/api/v2/admin/tenants/"+ctx.tenantID+"/credit-pool",
		map[string]interface{}{"max_balance": 5000})
	w := ctx.do(http.MethodPost, "/api/v2/admin/tenants/"+ctx.tenantID+"/credit-pool/topup",
		map[string]interface{}{"amount": 100})
	if w.Code != http.StatusPreconditionFailed {
		t.Fatalf("status = %d, want 412, body: %s", w.Code, w.Body.String())
	}
}

func TestListCreditPoolUsage_NotFound(t *testing.T) {
	ctx := setupAdminPoolRouter(t, false)
	w := ctx.do(http.MethodGet, "/api/v2/admin/tenants/"+ctx.tenantID+"/credit-pool/usage", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404, body: %s", w.Code, w.Body.String())
	}
}

func TestListCreditPoolUsage_Empty(t *testing.T) {
	ctx := setupAdminPoolRouter(t, false)
	ctx.do(http.MethodPost, "/api/v2/admin/tenants/"+ctx.tenantID+"/credit-pool",
		map[string]interface{}{"max_balance": 5000})
	w := ctx.do(http.MethodGet, "/api/v2/admin/tenants/"+ctx.tenantID+"/credit-pool/usage?offset=0&limit=10", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body: %s", w.Code, w.Body.String())
	}
	resp := decodeJSON(t, w)
	data := resp["data"].(map[string]interface{})
	if data["total"].(float64) != 0 {
		t.Errorf("total = %v, want 0 for fresh pool", data["total"])
	}
	if data["limit"].(float64) != 10 {
		t.Errorf("limit = %v, want 10", data["limit"])
	}
}

func TestDeleteCreditPool_NotFound(t *testing.T) {
	ctx := setupAdminPoolRouter(t, false)
	w := ctx.do(http.MethodDelete, "/api/v2/admin/tenants/"+ctx.tenantID+"/credit-pool", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404, body: %s", w.Code, w.Body.String())
	}
}

func TestDeleteCreditPool_DrainsPool(t *testing.T) {
	ctx := setupAdminPoolRouter(t, false)
	ctx.do(http.MethodPost, "/api/v2/admin/tenants/"+ctx.tenantID+"/credit-pool",
		map[string]interface{}{"max_balance": 5000})
	w := ctx.do(http.MethodDelete, "/api/v2/admin/tenants/"+ctx.tenantID+"/credit-pool", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body: %s", w.Code, w.Body.String())
	}
	// Side-effect: max_balance is zeroed so the relay gate blocks new debits.
	pool, err := repo.GetTenantCreditPool(ctx.tenantID)
	if err != nil {
		t.Fatalf("get pool: %v", err)
	}
	if pool.MaxBalance != 0 {
		t.Errorf("MaxBalance after drain = %d, want 0", pool.MaxBalance)
	}
}

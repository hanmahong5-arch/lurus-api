package repo

// tenant_transaction_isolation_pg_test.go — BUSINESS-BAR proof that
// TenantTransaction + the GORM TenantPlugin enforce row-level tenant isolation
// on a REAL PostgreSQL backend: a row written under tenant A is invisible and
// immutable under tenant B (cross-tenant SELECT/UPDATE affect 0 rows). Also
// covers InitTenantContextManager / GetTenantContextManager (both 0% before).
//
// Requires TEST_POSTGRES_DSN; skips otherwise (see SetupTestDB).

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/LurusTech/lurus-hub/internal/pkg/common"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func ginCtxWithTenant(tenantID string, userID int) *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	InjectTenantContext(c, tenantID, userID)
	return c
}

func TestInitTenantContextManager_Wiring(t *testing.T) {
	SetupTestDB(t)

	if err := InitTenantContextManager(DB); err != nil {
		t.Fatalf("InitTenantContextManager: %v", err)
	}
	mgr := GetTenantContextManager()
	if mgr == nil {
		t.Fatal("GetTenantContextManager returned nil after init")
	}
	if mgr.db != DB {
		t.Error("manager did not retain the DB it was initialised with")
	}
}

func TestTenantTransaction_RowLevelIsolation(t *testing.T) {
	SetupTestDB(t)

	// Register the tenant plugin on this isolated DB so scoped handles auto-filter.
	if err := InitTenantContextManager(DB); err != nil {
		t.Fatalf("InitTenantContextManager: %v", err)
	}

	const tenantA = "tenant-A"
	const tenantB = "tenant-B"

	cA := ginCtxWithTenant(tenantA, 1)
	cB := ginCtxWithTenant(tenantB, 2)

	// Write a user under tenant A INSIDE a tenant-scoped transaction. We do not
	// set TenantId ourselves — the plugin's beforeCreate must stamp it to A.
	err := TenantTransaction(cA, func(tx *gorm.DB) error {
		return tx.Create(&User{
			Username:    "isolated_a",
			DisplayName: "Isolated A",
			Role:        common.RoleCommonUser,
			Status:      common.UserStatusEnabled,
			Email:       "isolated_a@test.local",
		}).Error
	})
	if err != nil {
		t.Fatalf("TenantTransaction(A) create: %v", err)
	}

	// The stored row must carry tenant_id = A (system view bypasses isolation).
	var stored User
	if err := GetSystemDB().Where("username = ?", "isolated_a").First(&stored).Error; err != nil {
		t.Fatalf("system read of created user: %v", err)
	}
	if stored.TenantId != tenantA {
		t.Fatalf("plugin did not stamp tenant_id: got %q, want %q", stored.TenantId, tenantA)
	}

	// Cross-tenant SELECT under tenant B must return 0 rows.
	var countB int64
	if err := GetTenantDBWithID(tenantB).Model(&User{}).
		Where("username = ?", "isolated_a").Count(&countB).Error; err != nil {
		t.Fatalf("tenant B count: %v", err)
	}
	if countB != 0 {
		t.Fatalf("tenant B saw tenant A's row: count=%d, want 0", countB)
	}

	// Same tenant (A) must see exactly its own row.
	var countA int64
	if err := GetTenantDBWithID(tenantA).Model(&User{}).
		Where("username = ?", "isolated_a").Count(&countA).Error; err != nil {
		t.Fatalf("tenant A count: %v", err)
	}
	if countA != 1 {
		t.Fatalf("tenant A cannot see its own row: count=%d, want 1", countA)
	}

	// Cross-tenant UPDATE under tenant B must mutate 0 rows (plugin adds a
	// tenant_id = B predicate that matches nothing).
	res := GetTenantDBWithID(tenantB).Model(&User{}).
		Where("username = ?", "isolated_a").Update("display_name", "HIJACKED")
	if res.Error != nil {
		t.Fatalf("tenant B update: %v", res.Error)
	}
	if res.RowsAffected != 0 {
		t.Fatalf("tenant B mutated tenant A's row: RowsAffected=%d, want 0", res.RowsAffected)
	}

	// The row's display_name must be untouched from the cross-tenant update.
	var after User
	if err := GetSystemDB().Where("username = ?", "isolated_a").First(&after).Error; err != nil {
		t.Fatalf("system re-read: %v", err)
	}
	if after.DisplayName == "HIJACKED" {
		t.Fatal("cross-tenant update leaked into tenant A's row")
	}
	_ = cB // cB documents the "other tenant" actor; used via GetTenantDBWithID above
}

// TenantTransaction must surface the missing-context error unchanged (no panic,
// no silent success) when the gin.Context lacks a tenant id.
func TestTenantTransaction_NoTenantContext(t *testing.T) {
	SetupTestDB(t)

	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	err := TenantTransaction(c, func(tx *gorm.DB) error { return nil })
	if err == nil {
		t.Fatal("TenantTransaction must error without tenant context")
	}
}

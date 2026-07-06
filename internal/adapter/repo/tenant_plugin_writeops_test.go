package repo

// tenant_plugin_writeops_test.go — extends the cross_tenant isolation sentinel
// to the WRITE side of TenantPlugin (beforeUpdate / beforeDelete, previously
// 0%). The read-side auto-filter is already locked in
// cross_tenant_isolation_honesty_test.go; here we prove a tenant-scoped UPDATE
// or DELETE cannot mutate another tenant's rows. This is a defense-in-depth
// backstop for call sites that route through a tenant-bound DB handle.

import (
	"testing"

	"github.com/LurusTech/lurus-hub/internal/pkg/common"
)

func TestTenantPlugin_BeforeUpdate_ScopesToTenant(t *testing.T) {
	cleanup := setupSQLiteDB(t)
	defer cleanup()
	if err := DB.Use(&TenantPlugin{}); err != nil {
		t.Fatalf("register TenantPlugin: %v", err)
	}

	sys := WithoutTenantIsolation(DB)
	uA := &User{Username: "upd_a", DisplayName: "A", TenantId: "tA", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Email: "a@upd.local", Quota: 100}
	uB := &User{Username: "upd_b", DisplayName: "B", TenantId: "tB", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Email: "b@upd.local", Quota: 100}
	if err := sys.Create(uA).Error; err != nil {
		t.Fatalf("seed uA: %v", err)
	}
	if err := sys.Create(uB).Error; err != nil {
		t.Fatalf("seed uB: %v", err)
	}

	// A tenant-A-scoped UPDATE over the whole users table must touch ONLY tenant A.
	res := WithTenantID(DB, "tA").Model(&User{}).Where("1 = 1").Update("quota", 999)
	if res.Error != nil {
		t.Fatalf("scoped update: %v", res.Error)
	}
	if res.RowsAffected != 1 {
		t.Fatalf("beforeUpdate did not scope: RowsAffected=%d, want 1", res.RowsAffected)
	}

	var reloadA, reloadB User
	sys.First(&reloadA, uA.Id)
	sys.First(&reloadB, uB.Id)
	if reloadA.Quota != 999 {
		t.Errorf("tenant A row not updated: quota=%d", reloadA.Quota)
	}
	if reloadB.Quota != 100 {
		t.Errorf("tenant B row leaked into scoped update: quota=%d, want 100", reloadB.Quota)
	}
}

func TestTenantPlugin_BeforeDelete_ScopesToTenant(t *testing.T) {
	cleanup := setupSQLiteDB(t)
	defer cleanup()
	if err := DB.Use(&TenantPlugin{}); err != nil {
		t.Fatalf("register TenantPlugin: %v", err)
	}

	sys := WithoutTenantIsolation(DB)
	tokA := &Token{UserId: 1, TenantId: "tA", Key: "sk-A-" + common.GetRandomString(20), Status: common.TokenStatusEnabled, Name: "A", CreatedTime: common.GetTimestamp(), ExpiredTime: -1, UnlimitedQuota: true, Group: "default"}
	tokB := &Token{UserId: 2, TenantId: "tB", Key: "sk-B-" + common.GetRandomString(20), Status: common.TokenStatusEnabled, Name: "B", CreatedTime: common.GetTimestamp(), ExpiredTime: -1, UnlimitedQuota: true, Group: "default"}
	if err := sys.Create(tokA).Error; err != nil {
		t.Fatalf("seed tokA: %v", err)
	}
	if err := sys.Create(tokB).Error; err != nil {
		t.Fatalf("seed tokB: %v", err)
	}

	// A tenant-A-scoped DELETE over the whole tokens table must remove ONLY A's.
	res := WithTenantID(DB, "tA").Where("1 = 1").Delete(&Token{})
	if res.Error != nil {
		t.Fatalf("scoped delete: %v", res.Error)
	}
	if res.RowsAffected != 1 {
		t.Fatalf("beforeDelete did not scope: RowsAffected=%d, want 1", res.RowsAffected)
	}

	var remaining []Token
	if err := sys.Find(&remaining).Error; err != nil {
		t.Fatalf("list remaining: %v", err)
	}
	if len(remaining) != 1 || remaining[0].TenantId != "tB" {
		t.Fatalf("beforeDelete leaked cross-tenant: remaining=%v", remaining)
	}
}

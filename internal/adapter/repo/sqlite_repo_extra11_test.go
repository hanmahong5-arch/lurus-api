package repo

import (
	"testing"

	"github.com/LurusTech/lurus-hub/internal/pkg/common"
)

// ─── RefreshPricing / pricing_default helpers ────────────────────────────────

func TestRefreshPricing_NoData_NoError(t *testing.T) {
	setupSQLiteDB(t)
	// Should not panic with empty DB
	RefreshPricing()
}

func TestGetDefaultVendorIcon_Known(t *testing.T) {
	icon := getDefaultVendorIcon("OpenAI")
	if icon == "" {
		t.Error("expected non-empty icon for OpenAI")
	}
}

func TestGetDefaultVendorIcon_Unknown(t *testing.T) {
	icon := getDefaultVendorIcon("UnknownVendorXYZ")
	if icon != "" {
		t.Errorf("expected empty icon for unknown vendor, got %q", icon)
	}
}

func TestGetOrCreateVendor_CreatesNew(t *testing.T) {
	setupSQLiteDB(t)

	vendorMap := make(map[int]*Vendor)
	id := getOrCreateVendor("TestVendor11", vendorMap)
	if id == 0 {
		t.Error("expected non-zero vendor ID after creation")
	}
}

func TestGetOrCreateVendor_ReturnsExisting(t *testing.T) {
	setupSQLiteDB(t)

	vendorMap := make(map[int]*Vendor)
	// First call creates
	id1 := getOrCreateVendor("TestVendor11b", vendorMap)
	// Second call should find in map
	id2 := getOrCreateVendor("TestVendor11b", vendorMap)
	if id1 != id2 {
		t.Errorf("expected same id, got %d vs %d", id1, id2)
	}
}

func TestInitDefaultVendorMapping_CreatesEntry(t *testing.T) {
	setupSQLiteDB(t)

	metaMap := make(map[string]*Model)
	vendorMap := make(map[int]*Vendor)
	ab := AbilityWithChannel{}
	ab.Model = "gpt-4-defaultvendor-test"
	abilities := []AbilityWithChannel{ab}

	initDefaultVendorMapping(metaMap, vendorMap, abilities)

	if _, ok := metaMap["gpt-4-defaultvendor-test"]; !ok {
		t.Error("expected model added to metaMap")
	}
}

func TestInitDefaultVendorMapping_SkipsExisting(t *testing.T) {
	setupSQLiteDB(t)

	metaMap := map[string]*Model{
		"already-exists": {ModelName: "already-exists"},
	}
	vendorMap := make(map[int]*Vendor)
	ab := AbilityWithChannel{}
	ab.Model = "already-exists"
	abilities := []AbilityWithChannel{ab}

	initDefaultVendorMapping(metaMap, vendorMap, abilities)
	// Model should still have just 1 entry
	if len(metaMap) != 1 {
		t.Errorf("expected 1 entry in metaMap, got %d", len(metaMap))
	}
}

// ─── processDailyQuotaResets ──────────────────────────────────────────────────

func TestProcessDailyQuotaResets_NothingToReset(t *testing.T) {
	setupSQLiteDB(t)
	// No users need daily reset
	processDailyQuotaResets()
}

func TestProcessDailyQuotaResets_WithExpiredReset(t *testing.T) {
	setupSQLiteDB(t)

	// seed a user with daily quota that needs reset (last_daily_reset in the past)
	u := &User{
		Username:       "daily-reset-user-11",
		Role:           common.RoleCommonUser,
		Status:         common.UserStatusEnabled,
		TenantId:       "default",
		DailyQuota:     1000,
		DailyUsed:      500,
		LastDailyReset: 0, // epoch → always needs reset
	}
	if err := DB.Create(u).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	processDailyQuotaResets()
}

// ─── recordLogTx ─────────────────────────────────────────────────────────────

func TestRecordLogTx_SystemLog(t *testing.T) {
	setupSQLiteDB(t)

	u := &User{Username: "recordlogtx-user", Role: common.RoleCommonUser,
		Status: common.UserStatusEnabled, TenantId: "default"}
	if err := DB.Create(u).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	// recordLogTx uses LOG_DB which == DB in test setup
	// When LOG_DB == DB, it uses the tx passed in
	tx := DB.Begin()
	defer tx.Rollback()

	recordLogTx(tx, u.Id, LogTypeSystem, "test tx log")
	// tx.Commit() to persist
	if err := tx.Commit().Error; err != nil {
		t.Logf("commit: %v (acceptable)", err)
	}
}

func TestRecordLogTx_ConsumeLogDisabled(t *testing.T) {
	setupSQLiteDB(t)
	common.LogConsumeEnabled = false

	u := &User{Username: "recordlogtx-consume-disabled", Role: common.RoleCommonUser,
		Status: common.UserStatusEnabled, TenantId: "default"}
	if err := DB.Create(u).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	tx := DB.Begin()
	defer tx.Rollback()

	// Should return early (LogTypeConsume + !LogConsumeEnabled)
	recordLogTx(tx, u.Id, LogTypeConsume, "consume log should be skipped")
	tx.Rollback()
}

// ─── ensureUniqueUsername ─────────────────────────────────────────────────────

func TestEnsureUniqueUsername_NoConflict(t *testing.T) {
	setupSQLiteDB(t)

	result := ensureUniqueUsername("unique-username-11xyz", "default")
	if result != "unique-username-11xyz" {
		t.Errorf("expected unchanged username, got %q", result)
	}
}

func TestEnsureUniqueUsername_WithConflict(t *testing.T) {
	setupSQLiteDB(t)

	// seed a user with the base username
	base := "conflict-user-11"
	u := &User{Username: base, Role: common.RoleCommonUser,
		Status: common.UserStatusEnabled, TenantId: "default"}
	if err := DB.Create(u).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	result := ensureUniqueUsername(base, "default")
	if result == base {
		t.Errorf("expected unique suffix, but got same base: %q", result)
	}
}

// ─── GetRootUser ─────────────────────────────────────────────────────────────

func TestGetRootUser_NotFound(t *testing.T) {
	setupSQLiteDB(t)
	root := GetRootUser()
	// No root user seeded, could be nil or empty struct
	_ = root
}

func TestGetRootUser_Found(t *testing.T) {
	setupSQLiteDB(t)

	rootUser := &User{
		Username: "root-user-11",
		Role:     common.RoleRootUser,
		Status:   common.UserStatusEnabled,
		TenantId: "default",
	}
	if err := DB.Create(rootUser).Error; err != nil {
		t.Fatalf("create root user: %v", err)
	}

	got := GetRootUser()
	if got == nil || got.Role != common.RoleRootUser {
		t.Errorf("expected root user, got %v", got)
	}
}

// ─── CheckAndHandleDailyQuotaExhaustion extra paths ─────────────────────────

func TestCheckAndHandleDailyQuotaExhaustion_NoDailyQuota(t *testing.T) {
	setupSQLiteDB(t)

	u := &User{Username: "daily-exhaust-no-quota", Role: common.RoleCommonUser,
		Status: common.UserStatusEnabled, TenantId: "default", DailyQuota: 0}
	if err := DB.Create(u).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	exhausted, err := CheckAndHandleDailyQuotaExhaustion(u.Id, 100)
	if err != nil {
		t.Fatalf("CheckAndHandleDailyQuotaExhaustion: %v", err)
	}
	if exhausted {
		t.Error("expected not exhausted when DailyQuota=0")
	}
}

func TestCheckAndHandleDailyQuotaExhaustion_ExhaustedPath(t *testing.T) {
	setupSQLiteDB(t)

	u := &User{
		Username:       "daily-exhaust-11",
		Role:           common.RoleCommonUser,
		Status:         common.UserStatusEnabled,
		TenantId:       "default",
		DailyQuota:     100,
		DailyUsed:      99,
		LastDailyReset: common.GetTimestamp(), // recent reset
	}
	if err := DB.Create(u).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	// 99 used + 10 to consume > 100 limit → exhausted
	exhausted, err := CheckAndHandleDailyQuotaExhaustion(u.Id, 10)
	if err != nil {
		t.Fatalf("CheckAndHandleDailyQuotaExhaustion: %v", err)
	}
	if !exhausted {
		t.Error("expected exhausted=true")
	}
}

// ─── PostConsumeDailyQuota extra paths ───────────────────────────────────────

func TestPostConsumeDailyQuota_WithDailyQuotaSet(t *testing.T) {
	setupSQLiteDB(t)

	u := &User{
		Username:       "post-consume-daily-11",
		Role:           common.RoleCommonUser,
		Status:         common.UserStatusEnabled,
		TenantId:       "default",
		DailyQuota:     1000,
		DailyUsed:      0,
		LastDailyReset: common.GetTimestamp(),
	}
	if err := DB.Create(u).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	if err := PostConsumeDailyQuota(u.Id, 50); err != nil {
		t.Fatalf("PostConsumeDailyQuota: %v", err)
	}

	var loaded User
	DB.First(&loaded, u.Id)
	if loaded.DailyUsed < 50 {
		t.Errorf("expected DailyUsed>=50, got %d", loaded.DailyUsed)
	}
}

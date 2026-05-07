package app

import (
	"os"
	"testing"
	"time"

	"github.com/LurusTech/lurus-hub/internal/adapter/repo"
	"github.com/LurusTech/lurus-hub/internal/domain/entity"
)

// seedTestTenant creates a tenant in the DB and returns its ID.
// A separate UPDATE is issued after INSERT to force zero-value MaxQuota, because
// GORM Create falls back to the column's DB default (1000000) for zero values.
func seedTestTenant(t *testing.T, maxQuota int64) string {
	t.Helper()
	tenantID := "tenant-test-" + time.Now().Format("150405.000000000")
	tenant := repo.Tenant{
		Id:       tenantID,
		Slug:     "test-slug-" + tenantID,
		Name:     "Test Tenant",
		Status:   entity.TenantStatusEnabled,
		PlanType: entity.TenantPlanFree,
		MaxUsers: 10,
		MaxQuota: 1, // non-zero to bypass GORM default; overwritten below
	}
	if err := repo.DB.Create(&tenant).Error; err != nil {
		t.Fatalf("failed to seed tenant: %v", err)
	}
	// Force-write the requested MaxQuota via explicit column update.
	if err := repo.DB.Model(&repo.Tenant{}).Where("id = ?", tenantID).
		UpdateColumn("max_quota", maxQuota).Error; err != nil {
		t.Fatalf("failed to update tenant max_quota: %v", err)
	}
	return tenantID
}

// seedTenantLog inserts a consume log entry for the current month.
func seedTenantLog(t *testing.T, tenantID string, quota int) {
	t.Helper()
	now := time.Now()
	// Use first of current month to ensure it's in-period
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	log := repo.Log{
		TenantId:  tenantID,
		UserId:    1,
		Type:      repo.LogTypeConsume,
		Quota:     quota,
		CreatedAt: monthStart.Unix() + 3600, // 1 hour into the month
	}
	if err := repo.LOG_DB.Create(&log).Error; err != nil {
		t.Fatalf("failed to seed tenant log: %v", err)
	}
}

// TestEnforceTenantQuota_EmptyTenantID_Skips verifies that an empty tenant ID
// causes the check to be skipped entirely (no error).
func TestEnforceTenantQuota_EmptyTenantID_Skips(t *testing.T) {
	_ = setupServiceTestDB(t)
	prevEnabled := tenantQuotaEnforcementEnabled
	tenantQuotaEnforcementEnabled = true
	t.Cleanup(func() { tenantQuotaEnforcementEnabled = prevEnabled })

	err := enforceTenantQuota("", 100)
	if err != nil {
		t.Errorf("expected nil for empty tenantID, got: %v", err)
	}
}

// TestEnforceTenantQuota_MaxQuotaZero_Skips verifies that MaxQuota=0 means no limit.
func TestEnforceTenantQuota_MaxQuotaZero_Skips(t *testing.T) {
	_ = setupServiceTestDB(t)
	tenantID := seedTestTenant(t, 0) // MaxQuota=0 means unlimited
	prevEnabled := tenantQuotaEnforcementEnabled
	tenantQuotaEnforcementEnabled = true
	t.Cleanup(func() { tenantQuotaEnforcementEnabled = prevEnabled })

	err := enforceTenantQuota(tenantID, 9_999_999)
	if err != nil {
		t.Errorf("expected nil when MaxQuota=0, got: %v", err)
	}
}

// TestEnforceTenantQuota_UnderLimit_Allows verifies that used+requested <= MaxQuota is allowed.
func TestEnforceTenantQuota_UnderLimit_Allows(t *testing.T) {
	db := setupServiceTestDB(t)
	tenantID := seedTestTenant(t, 1_000_000)
	// Seed 500_000 used quota this month
	seedTenantLog(t, tenantID, 500_000)
	_ = db
	prevEnabled := tenantQuotaEnforcementEnabled
	tenantQuotaEnforcementEnabled = true
	t.Cleanup(func() { tenantQuotaEnforcementEnabled = prevEnabled })

	// 500_000 + 499_999 = 999_999 <= 1_000_000 → allowed
	err := enforceTenantQuota(tenantID, 499_999)
	if err != nil {
		t.Errorf("expected nil when under limit, got: %v", err)
	}
}

// TestEnforceTenantQuota_OverLimit_Denies verifies that used+requested > MaxQuota is denied
// AND that the returned error carries RetryAfterUnix pointing at next month UTC start
// so the relay error responder can emit a Retry-After header.
func TestEnforceTenantQuota_OverLimit_Denies(t *testing.T) {
	db := setupServiceTestDB(t)
	tenantID := seedTestTenant(t, 1_000_000)
	// Seed 900_000 used quota this month
	seedTenantLog(t, tenantID, 900_000)
	_ = db
	prevEnabled := tenantQuotaEnforcementEnabled
	tenantQuotaEnforcementEnabled = true
	t.Cleanup(func() { tenantQuotaEnforcementEnabled = prevEnabled })

	// 900_000 + 200_000 = 1_100_000 > 1_000_000 → denied
	err := enforceTenantQuota(tenantID, 200_000)
	if err == nil {
		t.Fatal("expected error when over limit, got nil")
	}

	// Retry-After hint must point at next-month UTC start.
	now := time.Now().UTC()
	expected := time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, time.UTC).Unix()
	if err.RetryAfterUnix != expected {
		t.Errorf("RetryAfterUnix = %d, expected next-month start %d", err.RetryAfterUnix, expected)
	}
	if err.RetryAfterUnix <= now.Unix() {
		t.Errorf("RetryAfterUnix (%d) must be in the future relative to now (%d)", err.RetryAfterUnix, now.Unix())
	}
}

// TestNextMonthStartUnix verifies year-boundary normalization (Dec → next year Jan).
func TestNextMonthStartUnix(t *testing.T) {
	cases := []struct {
		name string
		in   time.Time
		want time.Time
	}{
		{
			name: "mid-month → 1st of next month",
			in:   time.Date(2026, 5, 15, 10, 30, 0, 0, time.UTC),
			want: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "December → January next year",
			in:   time.Date(2026, 12, 25, 23, 59, 59, 0, time.UTC),
			want: time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "first second of month",
			in:   time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
			want: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := nextMonthStartUnix(tc.in)
			if got != tc.want.Unix() {
				t.Errorf("nextMonthStartUnix(%v) = %d (%v), want %d (%v)",
					tc.in, got, time.Unix(got, 0).UTC(), tc.want.Unix(), tc.want)
			}
		})
	}
}

// TestEnforceTenantQuota_EnforcementDisabled_Skips verifies that the global
// kill-switch bypasses all checks.
func TestEnforceTenantQuota_EnforcementDisabled_Skips(t *testing.T) {
	db := setupServiceTestDB(t)
	tenantID := seedTestTenant(t, 100) // tiny limit
	// Exceed limit immediately
	seedTenantLog(t, tenantID, 1000)
	_ = db
	prevEnabled := tenantQuotaEnforcementEnabled
	tenantQuotaEnforcementEnabled = false // enforcement OFF
	t.Cleanup(func() { tenantQuotaEnforcementEnabled = prevEnabled })

	err := enforceTenantQuota(tenantID, 999_999)
	if err != nil {
		t.Errorf("expected nil when enforcement disabled, got: %v", err)
	}
}

// TestEnforceTenantQuota_EnvVarDefault_IsFalse verifies that the env var defaults
// to false (safe default — no enforcement without explicit opt-in).
func TestEnforceTenantQuota_EnvVarDefault_IsFalse(t *testing.T) {
	// Unset the env var and re-read
	prev := os.Getenv("TENANT_QUOTA_ENFORCEMENT_ENABLED")
	os.Unsetenv("TENANT_QUOTA_ENFORCEMENT_ENABLED")
	t.Cleanup(func() {
		if prev != "" {
			os.Setenv("TENANT_QUOTA_ENFORCEMENT_ENABLED", prev)
		}
	})

	// The package-level var is initialized at startup; test the helper directly
	val := parseTenantQuotaEnforcementEnabled()
	if val {
		t.Error("expected false as default when env var is unset")
	}
}

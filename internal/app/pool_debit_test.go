package app

// pool_debit_test.go — P0-3: post-consume pool debit must never be silently
// dropped. Exhaustion falls back to OverdraftDebitPool (negative balance +
// relay_overdraft draw row); unlimited / absent pools skip the debit.

import (
	"testing"

	relaycommon "github.com/LurusTech/lurus-hub/internal/adapter/provider/common"
	"github.com/LurusTech/lurus-hub/internal/adapter/repo"
	"github.com/LurusTech/lurus-hub/internal/domain/entity"
	"github.com/LurusTech/lurus-hub/internal/pkg/common"
	"gorm.io/gorm"
)

// seedPoolTables adds the credit-pool tables to the shared service test DB
// (setupServiceTestDB only migrates user/token/log/channel/option/tenant).
func seedPoolTables(t *testing.T, db *gorm.DB) {
	t.Helper()
	if err := db.AutoMigrate(&entity.TenantCreditPool{}, &entity.TenantCreditPoolDraw{}); err != nil {
		t.Fatalf("migrate pool tables: %v", err)
	}
}

// seedTenantToken creates a token bound to tenantID and returns its ID.
func seedTenantToken(t *testing.T, db *gorm.DB, userId int, tenantID string) int {
	t.Helper()
	token := repo.Token{
		UserId:         userId,
		Key:            common.GetRandomString(32),
		Status:         common.TokenStatusEnabled,
		Name:           "pool-test-token",
		CreatedTime:    common.GetTimestamp(),
		AccessedTime:   common.GetTimestamp(),
		ExpiredTime:    -1,
		UnlimitedQuota: true,
		Group:          "default",
		TenantId:       tenantID,
	}
	if err := db.Create(&token).Error; err != nil {
		t.Fatalf("seed token: %v", err)
	}
	return token.Id
}

func TestDebitTenantPool_ExhaustedWritesOverdraft(t *testing.T) {
	db := setupServiceTestDB(t)
	seedPoolTables(t, db)
	userId := seedTestUser(t, db, 1000)
	tokenId := seedTenantToken(t, db, userId, "t-app-ovd")

	pool, err := repo.CreateTenantCreditPool("t-app-ovd", 1, 1000, repo.PoolResetMonthly, 80)
	if err != nil {
		t.Fatalf("create pool: %v", err)
	}
	if _, err := repo.TopupPool(pool.ID, "t-app-ovd", 3, 1, "seed"); err != nil {
		t.Fatalf("topup: %v", err)
	}

	// Debit 10 against balance 3: DebitPool rejects, fallback must overdraft.
	debitTenantPool(&relaycommon.RelayInfo{TokenId: tokenId}, 10)

	got, err := repo.GetTenantCreditPool("t-app-ovd")
	if err != nil {
		t.Fatalf("readback: %v", err)
	}
	if got.CurrentBalance != -7 {
		t.Errorf("balance = %d, want -7 (3 - 10, debt recorded)", got.CurrentBalance)
	}

	draws, _, err := repo.ListPoolDraws(pool.ID, 0, 50)
	if err != nil {
		t.Fatalf("list draws: %v", err)
	}
	found := false
	for _, d := range draws {
		if d.Reason == repo.PoolDrawReasonOverdraft && d.Amount == 10 && d.TokenID == tokenId {
			found = true
		}
	}
	if !found {
		t.Errorf("no relay_overdraft draw row for token %d — debit was dropped", tokenId)
	}
}

func TestDebitTenantPool_NormalDebitWhenFunded(t *testing.T) {
	db := setupServiceTestDB(t)
	seedPoolTables(t, db)
	userId := seedTestUser(t, db, 1000)
	tokenId := seedTenantToken(t, db, userId, "t-app-ok")

	pool, err := repo.CreateTenantCreditPool("t-app-ok", 1, 1000, repo.PoolResetMonthly, 80)
	if err != nil {
		t.Fatalf("create pool: %v", err)
	}
	if _, err := repo.TopupPool(pool.ID, "t-app-ok", 100, 1, "seed"); err != nil {
		t.Fatalf("topup: %v", err)
	}

	debitTenantPool(&relaycommon.RelayInfo{TokenId: tokenId}, 10)

	got, err := repo.GetTenantCreditPool("t-app-ok")
	if err != nil {
		t.Fatalf("readback: %v", err)
	}
	if got.CurrentBalance != 90 {
		t.Errorf("balance = %d, want 90 (normal relay_debit path)", got.CurrentBalance)
	}
}

func TestDebitTenantPool_UnlimitedPoolSkips(t *testing.T) {
	db := setupServiceTestDB(t)
	seedPoolTables(t, db)
	userId := seedTestUser(t, db, 1000)
	tokenId := seedTenantToken(t, db, userId, "t-app-unl")

	pool, err := repo.CreateTenantCreditPool("t-app-unl", 1, repo.PoolMaxBalanceUnlimited, repo.PoolResetNone, 0)
	if err != nil {
		t.Fatalf("create pool: %v", err)
	}

	debitTenantPool(&relaycommon.RelayInfo{TokenId: tokenId}, 10)

	got, err := repo.GetTenantCreditPool("t-app-unl")
	if err != nil {
		t.Fatalf("readback: %v", err)
	}
	if got.CurrentBalance != 0 {
		t.Errorf("unlimited pool balance = %d, want 0 (debit must be skipped)", got.CurrentBalance)
	}
	draws, total, err := repo.ListPoolDraws(pool.ID, 0, 10)
	if err != nil {
		t.Fatalf("list draws: %v", err)
	}
	if total != 0 || len(draws) != 0 {
		t.Errorf("unlimited pool has %d draws, want 0", total)
	}
}

func TestDebitTenantPool_NoPoolRowSkips(t *testing.T) {
	db := setupServiceTestDB(t)
	seedPoolTables(t, db)
	userId := seedTestUser(t, db, 1000)
	tokenId := seedTenantToken(t, db, userId, "t-app-nopool")

	// No pool row for the tenant: must no-op without error or draws.
	debitTenantPool(&relaycommon.RelayInfo{TokenId: tokenId}, 10)

	var count int64
	if err := db.Model(&entity.TenantCreditPoolDraw{}).Count(&count).Error; err != nil {
		t.Fatalf("count draws: %v", err)
	}
	if count != 0 {
		t.Errorf("draws written without a pool row: %d, want 0", count)
	}
}

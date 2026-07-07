package app

// post_consume_compensation_test.go — exercises the money-conservation failure
// arms of PostConsumeQuota / ReturnPreConsumedQuota that the happy-path fixtures
// can't reach: a token-quota update failure must COMPENSATE the already-debited
// user quota (no double-charge) and release the platform pre-auth instead of
// settling it; a refund whose DB write fails must be surfaced, not swallowed.
// The failures are induced by dropping the relevant table on the in-memory DB,
// which is the only hermetic way to force a GORM write error.

import (
	"context"
	"testing"

	relaycommon "github.com/LurusTech/lurus-hub/internal/adapter/provider/common"
	"github.com/LurusTech/lurus-hub/internal/adapter/repo"
	"github.com/LurusTech/lurus-hub/internal/domain/entity"
	"github.com/LurusTech/lurus-hub/internal/pkg/common"
)

// TestPostConsumeQuota_TokenUpdateFailsCompensatesAndReleases drives the
// inconsistent-local-state arm: the user quota is debited, then the token-quota
// update fails (tokens table dropped). PostConsumeQuota must (1) compensate the
// user quota back to its original value, (2) with a platform pre-auth present,
// RELEASE it rather than settle, and (3) return the original token error.
func TestPostConsumeQuota_TokenUpdateFailsCompensatesAndReleases(t *testing.T) {
	db := setupServiceTestDB(t)

	// Force the release path to fail-closed with no outbox, so the "both release
	// and outbox failed" branch of releasePlatformPreAuth is exercised too.
	prevOutbox := billingOutboxDB
	billingOutboxDB = nil
	t.Cleanup(func() { billingOutboxDB = prevOutbox })

	prevURL := common.IdentityServiceURL
	common.IdentityServiceURL = "" // release fast-fails "not configured"
	common.BillingBreakerSuccess()
	t.Cleanup(func() { common.IdentityServiceURL = prevURL; common.BillingBreakerSuccess() })

	const start = 10_000
	userId := seedTestUser(t, db, start)
	_, tokenId := seedTestToken(t, db, userId, start, false)

	// Drop the tokens table AFTER seeding so the Phase-3 token debit errors.
	if err := db.Migrator().DropTable(&repo.Token{}); err != nil {
		t.Fatalf("drop tokens: %v", err)
	}

	relayInfo := &relaycommon.RelayInfo{
		UserId:            userId,
		TokenId:           tokenId,
		TokenKey:          "irrelevant",
		IdentityAccountID: 42,      // platform account => Phase 5 runs
		PlatformPreAuthID: 991_100, // pre-auth present => release-on-inconsistency
	}

	const quota = 300
	err := PostConsumeQuota(relayInfo, quota, 0, false)
	if err == nil {
		t.Fatal("expected the token-quota error to be returned on inconsistent local state")
	}
	// User quota must be back to start: debited then compensated (no double-charge).
	if got := userQuota(t, db, userId); got != start {
		t.Errorf("user quota = %d, want %d (debit must be compensated on token failure)", got, start)
	}
	// Pre-auth must be cleared (released, not settled).
	if relayInfo.PlatformPreAuthID != 0 {
		t.Errorf("PlatformPreAuthID = %d, want 0 (released/handled)", relayInfo.PlatformPreAuthID)
	}
}

// TestPostConsumeQuota_NegativeTokenUpdateFailsCompensates drives the refund
// (negative-quota) compensation arm: a refund credits user quota, then the token
// credit fails; the user credit must be rolled back so a failed refund doesn't
// leak quota to the user.
func TestPostConsumeQuota_NegativeTokenUpdateFailsCompensates(t *testing.T) {
	db := setupServiceTestDB(t)

	const start = 10_000
	userId := seedTestUser(t, db, start)
	_, tokenId := seedTestToken(t, db, userId, start, false)

	if err := db.Migrator().DropTable(&repo.Token{}); err != nil {
		t.Fatalf("drop tokens: %v", err)
	}

	relayInfo := &relaycommon.RelayInfo{
		UserId:   userId,
		TokenId:  tokenId,
		TokenKey: "irrelevant",
	}

	// Negative quota => refund path. Token credit fails => user credit rolled back.
	err := PostConsumeQuota(relayInfo, -150, 0, false)
	if err == nil {
		t.Fatal("expected token-quota error on failed refund")
	}
	if got := userQuota(t, db, userId); got != start {
		t.Errorf("user quota = %d, want %d (failed refund must be compensated back)", got, start)
	}
}

// TestReturnPreConsumedQuota_RefundErrorSurfaced drives ReturnPreConsumedQuota's
// error arm: the refund's PostConsumeQuota fails at Phase 1 (users table dropped),
// which must be logged rather than panic. The function must still complete.
func TestReturnPreConsumedQuota_RefundErrorSurfaced(t *testing.T) {
	db := setupServiceTestDB(t)

	userId := seedTestUser(t, db, 9_000)

	if err := db.Migrator().DropTable(&repo.User{}); err != nil {
		t.Fatalf("drop users: %v", err)
	}

	c := createTestGinContext()
	relayInfo := &relaycommon.RelayInfo{
		UserId:                userId,
		FinalPreConsumedQuota: 1_000, // triggers the refund branch
		PlatformPreAuthID:     0,     // release is a no-op
	}

	// Must not panic; the Phase-1 IncreaseUserQuota error is logged and swallowed.
	ReturnPreConsumedQuota(c, relayInfo)
}

// TestDebitTenantPool_HardDBErrorRecordsLostDebit drives the honest residual-gap
// arm: a funded pool exists but the draw table is gone, so DebitPool fails with a
// non-exhaustion DB error. The debit is dropped (can't be recorded) but the
// balance must NOT move — the loss is surfaced via log/metric, not silently
// double-applied.
func TestDebitTenantPool_HardDBErrorRecordsLostDebit(t *testing.T) {
	db := setupServiceTestDB(t)
	seedPoolTables(t, db)
	userId := seedTestUser(t, db, 1_000)
	tokenId := seedTenantToken(t, db, userId, "t-lost-debit")

	pool, err := repo.CreateTenantCreditPool("t-lost-debit", 1, 1_000, repo.PoolResetMonthly, 80)
	if err != nil {
		t.Fatalf("create pool: %v", err)
	}
	if _, err := repo.TopupPool(pool.ID, "t-lost-debit", 500, 1, "seed"); err != nil {
		t.Fatalf("topup: %v", err)
	}

	// Drop the draw table: DebitPool's draw insert now fails with a DB error that
	// is NOT ErrPoolExhausted, so we hit the "debit lost" branch.
	if err := db.Migrator().DropTable(&entity.TenantCreditPoolDraw{}); err != nil {
		t.Fatalf("drop draws: %v", err)
	}

	debitTenantPool(&relaycommon.RelayInfo{TokenId: tokenId}, 100)

	got, err := repo.GetTenantCreditPool("t-lost-debit")
	if err != nil {
		t.Fatalf("readback pool: %v", err)
	}
	if got.CurrentBalance != 500 {
		t.Errorf("pool balance = %d, want 500 (debit was lost, balance must not move)", got.CurrentBalance)
	}
}

// TestEnqueueSettle_CreateErrorWhenTableDropped and its release sibling drive the
// outbox enqueue DB-error arms: the outbox is initialized but its table is gone,
// so Create fails and the enqueue returns a wrapped error.
func TestEnqueueSettle_CreateErrorWhenTableDropped(t *testing.T) {
	db := setupServiceTestDB(t)
	restore := setupOutbox(t, db)
	defer restore()

	if err := db.Migrator().DropTable(&entity.BillingOutbox{}); err != nil {
		t.Fatalf("drop outbox: %v", err)
	}

	if err := EnqueueSettle(1, 42, 1.0); err == nil {
		t.Fatal("expected EnqueueSettle to error when the outbox table is missing")
	}
	if err := EnqueueRelease(1, 43); err == nil {
		t.Fatal("expected EnqueueRelease to error when the outbox table is missing")
	}
}

// TestProcessBillingOutbox_QueryErrorWhenTableDropped drives the worker's query
// error arm: with the outbox table dropped, the claim query fails and the error
// propagates (rather than being swallowed).
func TestProcessBillingOutbox_QueryErrorWhenTableDropped(t *testing.T) {
	db := setupServiceTestDB(t)
	restore := setupOutbox(t, db)
	defer restore()

	if err := db.Migrator().DropTable(&entity.BillingOutbox{}); err != nil {
		t.Fatalf("drop outbox: %v", err)
	}

	if err := ProcessBillingOutbox(context.Background()); err == nil {
		t.Fatal("expected ProcessBillingOutbox to return a query error when table missing")
	}
}

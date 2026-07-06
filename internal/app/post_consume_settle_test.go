package app

// post_consume_settle_test.go — covers PostConsumeQuota's platform-settlement
// branch (Phase 5). With a platform account + pre-auth id set and no reachable
// billing backend, SettleWithBreaker fails and the settle is durably enqueued to
// the billing outbox instead of being lost. nats + Redis are forced off so the
// async report goroutines are pure network no-ops that never touch the test DB.

import (
	"testing"
	"time"

	relaycommon "github.com/LurusTech/lurus-hub/internal/adapter/provider/common"
	"github.com/LurusTech/lurus-hub/internal/domain/entity"
	"github.com/LurusTech/lurus-hub/internal/pkg/common"
)

func TestPostConsumeQuota_PreAuthSettleFailsEnqueuesOutbox(t *testing.T) {
	db := setupServiceTestDB(t)
	seedPoolTables(t, db)

	// Force async report goroutines to be DB-free no-ops.
	prevRedis := common.RedisEnabled
	common.RedisEnabled = false
	t.Cleanup(func() { common.RedisEnabled = prevRedis })

	restore := setupOutbox(t, db)
	defer restore()

	userId := seedTestUser(t, db, 100_000)
	key, tokenId := seedTestToken(t, db, userId, 100_000, false)

	const preAuthID int64 = 987654
	relayInfo := &relaycommon.RelayInfo{
		UserId:            userId,
		TokenId:           tokenId,
		TokenKey:          key,
		IdentityAccountID: 42,        // platform account => settlement path
		PlatformPreAuthID: preAuthID, // pre-auth exists => settle (not legacy debit)
	}

	// Settle the request; the billing backend is unreachable so the settle must
	// fall back to the outbox rather than dropping the charge.
	if err := PostConsumeQuota(relayInfo, 500, 0, false); err != nil {
		t.Fatalf("PostConsumeQuota: %v", err)
	}

	// The user quota still moved locally.
	if got := userQuota(t, db, userId); got != 100_000-500 {
		t.Errorf("user quota = %d, want %d", got, 100_000-500)
	}

	// A settle entry for this pre-auth must have been enqueued to the outbox.
	var row entity.BillingOutbox
	if err := db.Where("pre_auth_id = ?", preAuthID).First(&row).Error; err != nil {
		t.Fatalf("no outbox settle row enqueued for failed settle: %v", err)
	}
	if row.Action != outboxActionSettle {
		t.Errorf("outbox action = %q, want settle", row.Action)
	}
	// PlatformPreAuthID must be cleared after handling (settled or enqueued).
	if relayInfo.PlatformPreAuthID != 0 {
		t.Errorf("PlatformPreAuthID = %d, want 0 (marked handled)", relayInfo.PlatformPreAuthID)
	}

	// Give the async report goroutine a moment so it doesn't outlive the DB
	// cleanup; it only makes network calls, but we drain it for hygiene.
	time.Sleep(50 * time.Millisecond)
}

// TestPostConsumeQuota_LegacyDebitPath covers the no-pre-auth settlement branch:
// IdentityAccountID > 0 but PlatformPreAuthID == 0, which fires the legacy
// fire-and-forget wallet debit goroutine. The local user quota must still be
// debited synchronously; the async gRPC debit fails harmlessly.
func TestPostConsumeQuota_LegacyDebitPath(t *testing.T) {
	db := setupServiceTestDB(t)
	seedPoolTables(t, db)

	prevRedis := common.RedisEnabled
	common.RedisEnabled = false
	t.Cleanup(func() { common.RedisEnabled = prevRedis })

	userId := seedTestUser(t, db, 80_000)
	key, tokenId := seedTestToken(t, db, userId, 80_000, false)

	relayInfo := &relaycommon.RelayInfo{
		UserId:            userId,
		TokenId:           tokenId,
		TokenKey:          key,
		IdentityAccountID: 55, // platform account
		PlatformPreAuthID: 0,  // no pre-auth => legacy debit goroutine
	}

	if err := PostConsumeQuota(relayInfo, 700, 0, false); err != nil {
		t.Fatalf("PostConsumeQuota: %v", err)
	}
	if got := userQuota(t, db, userId); got != 80_000-700 {
		t.Errorf("user quota = %d, want %d", got, 80_000-700)
	}

	// Drain the fire-and-forget debit goroutine (network-only) before teardown.
	time.Sleep(50 * time.Millisecond)
}

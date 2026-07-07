package app

// settle_critical_gap_test.go — drives PostConsumeQuota's worst-case settlement
// arm: the platform settle fails AND the durable outbox enqueue also fails, so
// the code emits the CRITICAL "manual reconciliation" log and clears the pre-auth
// (relying on the platform TTL as the safety net). The local user quota must
// still have moved.

import (
	"testing"
	"time"

	relaycommon "github.com/LurusTech/lurus-hub/internal/adapter/provider/common"
	"github.com/LurusTech/lurus-hub/internal/pkg/common"
)

func TestPostConsumeQuota_SettleAndOutboxBothFail(t *testing.T) {
	db := setupServiceTestDB(t)
	seedPoolTables(t, db)

	// Outbox not initialized => EnqueueSettle fails after the settle fails.
	prevOutbox := billingOutboxDB
	billingOutboxDB = nil
	t.Cleanup(func() { billingOutboxDB = prevOutbox })

	// Trip the breaker OPEN so SettleWithBreaker fast-fails (no 5s network wait).
	common.BillingBreakerFailure()
	common.BillingBreakerFailure()
	common.BillingBreakerFailure()
	if !common.BillingBreakerIsOpen() {
		t.Fatal("precondition: breaker should be OPEN")
	}
	t.Cleanup(common.BillingBreakerSuccess)

	const start = 100_000
	userId := seedTestUser(t, db, start)
	key, tokenId := seedTestToken(t, db, userId, start, false)

	relayInfo := &relaycommon.RelayInfo{
		UserId:            userId,
		TokenId:           tokenId,
		TokenKey:          key,
		IdentityAccountID: 42,
		PlatformPreAuthID: 555_000, // settle path
	}

	const quota = 500
	if err := PostConsumeQuota(relayInfo, quota, 0, false); err != nil {
		t.Fatalf("PostConsumeQuota: %v", err)
	}
	// Local quota still moved despite the settle+outbox double failure.
	if got := userQuota(t, db, userId); got != start-quota {
		t.Errorf("user quota = %d, want %d", got, start-quota)
	}
	// Pre-auth marked handled (cleared) even though both remote paths failed.
	if relayInfo.PlatformPreAuthID != 0 {
		t.Errorf("PlatformPreAuthID = %d, want 0 (handled)", relayInfo.PlatformPreAuthID)
	}
	time.Sleep(30 * time.Millisecond) // drain async report goroutine
}

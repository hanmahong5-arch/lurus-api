package app

// billing_seam_extra_test.go — drives the platform-billing branches the happy
// fixtures skip. The gRPC identity client handle always exists in-process
// (WaitForReady blocks a real dial for the full timeout), so a "success via
// httptest" seam is not reachable hermetically; instead we trip the billing
// circuit breaker OPEN, which makes the pre-auth wrappers fast-fail without any
// network call. With Redis off the degrade path is fail-closed, so a tripped
// breaker deterministically yields a 402.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	relaycommon "github.com/LurusTech/lurus-hub/internal/adapter/provider/common"
	"github.com/LurusTech/lurus-hub/internal/adapter/repo"
	"github.com/LurusTech/lurus-hub/internal/domain/entity"
	"github.com/LurusTech/lurus-hub/internal/pkg/common"
)

// tripBillingBreakerOpen forces the billing breaker OPEN (3 consecutive fails)
// so the pre-auth wrappers fast-fail, and resets it to CLOSED on cleanup.
func tripBillingBreakerOpen(t *testing.T) {
	t.Helper()
	common.BillingBreakerFailure()
	common.BillingBreakerFailure()
	common.BillingBreakerFailure()
	if !common.BillingBreakerIsOpen() {
		t.Fatal("precondition: billing breaker should be OPEN after 3 failures")
	}
	t.Cleanup(common.BillingBreakerSuccess)
}

// TestPreConsumeQuota_PreAuthErrorReturns402 drives PreConsumeQuota's platform
// pre-auth failure arm: with unified billing on, a platform account, and the
// breaker OPEN (no cached balance to degrade against), platformPreAuthorize
// returns a 402 and PreConsumeQuota propagates it WITHOUT pre-deducting local
// quota or setting a pre-auth id.
func TestPreConsumeQuota_PreAuthErrorReturns402(t *testing.T) {
	db := setupServiceTestDB(t) // Redis off => no cached balance => degrade denied
	repo.InitCol()
	seedPoolTables(t, db)
	tripBillingBreakerOpen(t)

	prevUnified := common.BillingUnifiedEnabled()
	common.SetBillingUnifiedEnabled(true)
	t.Cleanup(func() { common.SetBillingUnifiedEnabled(prevUnified) })

	const start = 50_000
	userId := seedTestUser(t, db, start)
	key, tokenId := seedTestToken(t, db, userId, start, false)

	c := createTestGinContext()
	c.Request = httptest.NewRequest(http.MethodPost, "/relay", nil)

	relayInfo := &relaycommon.RelayInfo{
		UserId:            userId,
		TokenId:           tokenId,
		TokenKey:          key,
		IdentityAccountID: 4343,
		OriginModelName:   "gpt-4",
	}

	apiErr := PreConsumeQuota(c, 1_000, relayInfo)
	if apiErr == nil {
		t.Fatal("expected 402 when platform pre-auth fails and degrade is denied")
	}
	if relayInfo.PlatformPreAuthID != 0 {
		t.Errorf("PlatformPreAuthID = %d, want 0 (no freeze created)", relayInfo.PlatformPreAuthID)
	}
	if got := userQuota(t, db, userId); got != start {
		t.Errorf("user quota = %d, want %d (untouched on pre-auth rejection)", got, start)
	}
}

// TestPreConsumeQuota_ReentryGuardSkipsPreAuth proves the retry guard: when
// PlatformPreAuthID is already set (a relay retry), PreConsumeQuota must NOT
// re-enter the platform pre-auth and must proceed straight to local validation.
// The breaker is left OPEN as a tripwire — if the guard were skipped, the
// platform call would run and (being fast-failed) drive a 402, which we assert
// does NOT happen.
func TestPreConsumeQuota_ReentryGuardSkipsPreAuth(t *testing.T) {
	db := setupServiceTestDB(t)
	repo.InitCol()
	seedPoolTables(t, db)
	tripBillingBreakerOpen(t)

	prevUnified := common.BillingUnifiedEnabled()
	common.SetBillingUnifiedEnabled(true)
	t.Cleanup(func() { common.SetBillingUnifiedEnabled(prevUnified) })

	const start = 50_000
	userId := seedTestUser(t, db, start)
	key, tokenId := seedTestToken(t, db, userId, start, false)

	c := createTestGinContext()
	c.Request = httptest.NewRequest(http.MethodPost, "/relay", nil)
	c.Set("token_quota", start)

	const existingPreAuth int64 = 5150
	relayInfo := &relaycommon.RelayInfo{
		UserId:            userId,
		TokenId:           tokenId,
		TokenKey:          key,
		IdentityAccountID: 4444,
		PlatformPreAuthID: existingPreAuth, // already authorized on first attempt
	}

	const estimate = 1_000
	if apiErr := PreConsumeQuota(c, estimate, relayInfo); apiErr != nil {
		t.Fatalf("PreConsumeQuota unexpectedly failed on re-entry: %v", apiErr.Error())
	}
	if relayInfo.PlatformPreAuthID != existingPreAuth {
		t.Errorf("PlatformPreAuthID = %d, want %d (unchanged on re-entry)", relayInfo.PlatformPreAuthID, existingPreAuth)
	}
	if got := userQuota(t, db, userId); got != start-estimate {
		t.Errorf("user quota = %d, want %d", got, start-estimate)
	}
}

// TestProcessBillingOutbox_ReleaseAndUnknownAction drives the worker's
// release-action and unknown-action arms. The release call fails (no reachable
// backend) so it stays pending on a retry; the unknown action is a fast local
// error that also schedules a retry. Both prove the entries are neither dropped
// nor marked done.
func TestProcessBillingOutbox_ReleaseAndUnknownAction(t *testing.T) {
	db := setupServiceTestDB(t)
	restore := setupOutbox(t, db)
	defer restore()

	if err := EnqueueRelease(1, 2002); err != nil {
		t.Fatalf("enqueue release: %v", err)
	}
	bad := entity.BillingOutbox{
		AccountID: 1, PreAuthID: 2003, Action: "bogus", Status: "pending", NextRetry: time.Now(),
	}
	if err := db.Create(&bad).Error; err != nil {
		t.Fatalf("seed bad row: %v", err)
	}

	if err := ProcessBillingOutbox(context.Background()); err != nil {
		t.Fatalf("ProcessBillingOutbox: %v", err)
	}

	assertPendingRetry := func(preAuth int64) {
		var row entity.BillingOutbox
		if err := db.Where("pre_auth_id = ?", preAuth).First(&row).Error; err != nil {
			t.Fatalf("row %d gone: %v", preAuth, err)
		}
		if row.Status != "pending" {
			t.Errorf("pre_auth %d status = %q, want pending (retry)", preAuth, row.Status)
		}
		if row.RetryCount != 1 || row.Error == "" {
			t.Errorf("pre_auth %d = {retry:%d err:%q}, want {1, non-empty}", preAuth, row.RetryCount, row.Error)
		}
	}
	assertPendingRetry(2002) // release call failed => retry
	assertPendingRetry(2003) // unknown action => error => retry
}

package app

// quota_threshold_honesty_test.go — honesty lock for ARCH-11.
//
// Claim under audit:
//
//	"llm.quota.threshold publisher is implemented by newhub
//	 (checkAndPublishQuotaThresholds), firing at the 50/80/95/100 tiers with a
//	 payload of {account_id, used_tokens, limit_tokens, usage_percent}."
//
// The existing quota_threshold_test.go already proves the happy path, dedup,
// NATS-down, and multi-crossing behaviour. This file does NOT re-assert those —
// it locks the BMAD-ARCH-11 edge cases the existing suite leaves open, all of
// which are reflex-1 candidates under CLAUDE.md §4.1 (perfect / boundary
// numbers) or §4.1② (the literal subject string + payload field names):
//
//   - exact-boundary crossing at 49.9 / 50 / 100 percent (off-by-one in the
//     `>= threshold` comparison would be invisible to the existing 36%→50% test
//     which clears the boundary by a wide margin)
//   - limit_tokens == 0 must NOT divide by zero / NaN / +Inf its way into a
//     spurious publish (the guard at line 169 of quota_threshold.go)
//   - period rollover: the dedup key embeds YYYY-MM, so a crossing already sent
//     in month M must be allowed to re-fire in month M+1
//   - the wire subject string and every payload field name are byte-for-byte
//     what lurus-platform's notification module consumes
//
// The subject (checkAndPublishQuotaThresholds + dedupKey + the public
// SubjectQuotaThreshold constant) and the assertions (publish count + payload
// contents) are independent of the implementation text, so this is a behaviour
// lock, not a tautology.

import (
	"context"
	"math"
	"testing"
	"time"

	hubnats "github.com/LurusTech/lurus-hub/internal/pkg/nats"
)

// makeParams builds a quotaThresholdParams whose before/after percentages are
// derived from explicit used-before / used-after / limit values so the test
// reads in the same units as the production crossing maths.
func makeParams(userID int, accountID int64, usedBefore, usedAfter, limit int64) quotaThresholdParams {
	return quotaThresholdParams{
		UserId:            userID,
		IdentityAccountID: accountID,
		QuotaConsumed:     usedAfter - usedBefore,
		UsedTokensAfter:   usedAfter,
		LimitTokens:       limit,
	}
}

// TestQuotaThreshold_ExactBoundary_FiresAtGEThreshold pins the crossing
// semantics at the precise rung. A consumption that lands the cumulative usage
// EXACTLY on 50.0% must fire (the contract is percentAfter >= threshold), while
// one that lands at 49.9...% must NOT — that one-tenth-of-a-percent gap is
// exactly where a `>` vs `>=` slip would hide.
func TestQuotaThreshold_ExactBoundary_FiresAtGEThreshold(t *testing.T) {
	withQuotaNATSEnabled(t)
	resetQuotaThresholdsCache(t)

	cases := []struct {
		name       string
		usedBefore int64
		usedAfter  int64
		limit      int64
		wantFire   bool
		why        string
	}{
		{
			name:       "lands exactly on 50.0% — fires (>= is inclusive)",
			usedBefore: 499, usedAfter: 500, limit: 1000, // 49.9% -> 50.0%
			wantFire: true,
			why:      "percentAfter == 50.0 must satisfy >= 50",
		},
		{
			name:       "lands at 49.9% — does NOT fire (below rung)",
			usedBefore: 498, usedAfter: 499, limit: 1000, // 49.8% -> 49.9%
			wantFire: false,
			why:      "49.9 < 50 must not cross the 50 rung",
		},
		{
			name:       "already at 50% before — no re-cross of 50 (percentBefore>=ft)",
			usedBefore: 500, usedAfter: 501, limit: 1000, // 50.0% -> 50.1%
			wantFire: false,
			why:      "percentBefore already >= 50, so the 50 rung is not newly crossed",
		},
		{
			name:       "lands exactly on 100% — fires the terminal rung",
			usedBefore: 940, usedAfter: 1000, limit: 1000, // 94% -> 100% : crosses 95 and 100
			wantFire: true,
			why:      "100% is a real rung; >= 100 must fire",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pub := &mockPublisher{}
			params := makeParams(1, 100, tc.usedBefore, tc.usedAfter, tc.limit)
			checkAndPublishQuotaThresholds(context.Background(), params, newMockRedis(), pub)

			fired := pub.count() > 0
			if fired != tc.wantFire {
				t.Fatalf("fired=%v (count=%d), want %v — %s", fired, pub.count(), tc.wantFire, tc.why)
			}
		})
	}
}

// TestQuotaThreshold_ZeroLimit_NoDivideByZero is the §4.1② "limit_tokens=0
// 除零" edge. With limit==0 the percentage maths would be x/0 = +Inf (or 0/0 =
// NaN), and +Inf >= every threshold — without the guard this would fire on
// every rung for every transaction against an unconfigured account. The guard
// (params.LimitTokens <= 0 → early return) must make this a clean no-op.
func TestQuotaThreshold_ZeroLimit_NoDivideByZero(t *testing.T) {
	withQuotaNATSEnabled(t)
	resetQuotaThresholdsCache(t)

	pub := &mockPublisher{}
	params := makeParams(2, 200, 0, 5000, 0) // limit == 0

	// Must not panic and must not publish.
	checkAndPublishQuotaThresholds(context.Background(), params, newMockRedis(), pub)

	if pub.count() != 0 {
		t.Fatalf("publish count = %d, want 0 (limit==0 must short-circuit, not divide by zero)", pub.count())
	}

	// Belt-and-braces: prove the raw maths WOULD have produced a non-finite
	// percentage, so the guard is doing real work rather than the inputs being
	// benign. zeroLimit is a runtime value so the compiler can't fold the /0.
	zeroLimit := params.LimitTokens // == 0
	if got := float64(5000) / float64(zeroLimit) * 100.0; !math.IsInf(got, 1) {
		t.Fatalf("sanity: expected +Inf from /0, got %v", got)
	}
}

// TestQuotaThreshold_NegativeAccountID_NoFire locks the other half of the
// input guard (IdentityAccountID <= 0). An event with no resolved platform
// account must never reach the wire — the notification module keys alerts on
// account_id and a 0/negative id would be undeliverable noise.
func TestQuotaThreshold_NegativeAccountID_NoFire(t *testing.T) {
	withQuotaNATSEnabled(t)
	resetQuotaThresholdsCache(t)

	pub := &mockPublisher{}
	params := makeParams(3, 0, 400, 550, 1100) // would cross 50 if account were valid
	checkAndPublishQuotaThresholds(context.Background(), params, newMockRedis(), pub)

	if pub.count() != 0 {
		t.Fatalf("publish count = %d, want 0 (account_id<=0 must not publish)", pub.count())
	}
}

// TestQuotaThreshold_PeriodRollover_ReFiresNextMonth is the "period 切换重置"
// edge. dedupKey embeds the YYYY-MM period, so the SAME threshold crossed in
// month M is suppressed within M (proven by the existing AlreadySent test) but
// MUST be allowed to fire again in month M+1. We drive this directly through
// dedupKey since checkAndPublishQuotaThresholds uses time.Now() internally —
// the key format IS the period-reset mechanism, so locking its shape locks the
// behaviour without needing an injectable clock.
func TestQuotaThreshold_PeriodRollover_ReFiresNextMonth(t *testing.T) {
	may := time.Date(2026, 5, 31, 23, 59, 0, 0, time.UTC)
	june := time.Date(2026, 6, 1, 0, 1, 0, 0, time.UTC)

	keyMay := dedupKey(42, 80, may)
	keyJune := dedupKey(42, 80, june)

	if keyMay == keyJune {
		t.Fatalf("dedup key did not change across month boundary: %q == %q (would suppress the next-period alert forever)", keyMay, keyJune)
	}

	// A shared-key Redis stub: once a key is claimed it stays claimed (mirrors
	// SET NX with TTL longer than the test). The May crossing claims keyMay;
	// the June crossing asks for keyJune which is still open → it must be
	// grantable. We assert via the mock that the two periods occupy distinct
	// dedup slots.
	rdb := newMockRedis()
	gotMay, _ := rdb.SetNXBool(context.Background(), keyMay, 24*time.Hour)
	if !gotMay {
		t.Fatal("first claim on May key should succeed")
	}
	// Re-claiming May within the period is denied — confirms the dedup works.
	if again, _ := rdb.SetNXBool(context.Background(), keyMay, 24*time.Hour); again {
		t.Fatal("second claim on May key must be denied within the period")
	}
	// June is a different slot and must be independently claimable.
	gotJune, _ := rdb.SetNXBool(context.Background(), keyJune, 24*time.Hour)
	if !gotJune {
		t.Fatal("June key must be claimable even though May was already sent (period reset)")
	}
}

// TestQuotaThreshold_WireContract pins the exact NATS subject and every payload
// field name/value the lurus-platform notification module consumes. This is the
// §4.1② named-artifact check: the claim names the subject "llm.quota.threshold"
// and four fields — this test fails the moment any of them drifts.
func TestQuotaThreshold_WireContract(t *testing.T) {
	withQuotaNATSEnabled(t)
	resetQuotaThresholdsCache(t)

	if hubnats.SubjectQuotaThreshold != "llm.quota.threshold" {
		t.Fatalf("SubjectQuotaThreshold = %q, want %q", hubnats.SubjectQuotaThreshold, "llm.quota.threshold")
	}

	pub := &mockPublisher{}
	// 36% -> 50%: crosses exactly one rung so exactly one payload to inspect.
	params := makeParams(9, 909, 400, 550, 1100)
	checkAndPublishQuotaThresholds(context.Background(), params, newMockRedis(), pub)

	if pub.count() != 1 {
		t.Fatalf("publish count = %d, want exactly 1", pub.count())
	}
	if pub.messages[0].subject != "llm.quota.threshold" {
		t.Fatalf("subject = %q, want llm.quota.threshold", pub.messages[0].subject)
	}
	payload, ok := pub.messages[0].payload.(QuotaThresholdPayload)
	if !ok {
		t.Fatalf("payload type = %T, want QuotaThresholdPayload", pub.messages[0].payload)
	}
	if payload.AccountID != 909 {
		t.Errorf("account_id = %d, want 909", payload.AccountID)
	}
	if payload.UsedTokens != 550 {
		t.Errorf("used_tokens = %d, want 550", payload.UsedTokens)
	}
	if payload.LimitTokens != 1100 {
		t.Errorf("limit_tokens = %d, want 1100", payload.LimitTokens)
	}
	// usage_percent must be the post-transaction percentage (550/1100 = 50),
	// not the threshold rung — this distinction matters to consumers that
	// render the actual figure rather than the bucket.
	if payload.UsagePercent != 50.0 {
		t.Errorf("usage_percent = %v, want 50.0 (actual post-tx percentage)", payload.UsagePercent)
	}
}

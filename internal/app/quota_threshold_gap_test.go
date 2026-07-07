package app

// quota_threshold_gap_test.go — closes the parser fallbacks and the
// crossing-loop edge arms (negative used-before clamp, dedup-store error) of the
// quota-threshold publisher.

import (
	"context"
	"testing"
	"time"
)

func TestLoadQuotaThresholds_Fallbacks(t *testing.T) {
	def := loadQuotaThresholds("") // the built-in default ladder

	// A malformed entry voids the whole override → default.
	if got := loadQuotaThresholds("50,abc,80"); len(got) != len(def) {
		t.Errorf("malformed override => %v, want default %v", got, def)
	}
	// An out-of-range value likewise voids the override.
	if got := loadQuotaThresholds("50,150"); len(got) != len(def) {
		t.Errorf("out-of-range override => %v, want default %v", got, def)
	}
	// All-empty entries collapse to zero parsed rungs → default.
	if got := loadQuotaThresholds(" , , "); len(got) != len(def) {
		t.Errorf("all-empty override => %v, want default %v", got, def)
	}
}

// TestCheckAndPublishQuotaThresholds_UsedBeforeClamped drives the usedBefore<0
// clamp: when this request's consumption exceeds the recorded used-after total
// (batch-update skew), usedBefore is floored at 0 so the crossing math stays sane.
func TestCheckAndPublishQuotaThresholds_UsedBeforeClamped(t *testing.T) {
	t.Setenv("LLM_QUOTA_NATS_ENABLED", "true")

	pub := &fakeThresholdPublisher{}
	dedup := &fakeDeduper{}

	params := quotaThresholdParams{
		UserId:            7,
		IdentityAccountID: 21,
		TenantID:          "default",
		QuotaConsumed:     2000, // > usedAfter => usedBefore would be negative
		UsedTokensAfter:   600,
		LimitTokens:       1000,
	}

	checkAndPublishQuotaThresholds(context.Background(), params, dedup, pub)

	// usedBefore clamps to 0 => percentBefore=0, percentAfter=60 => crosses 50.
	if len(pub.published) == 0 {
		t.Fatal("expected a crossing publish with usedBefore clamped to 0")
	}
}

// erroringDeduper always fails the dedup claim.
type erroringDeduper struct{}

func (erroringDeduper) SetNXBool(_ context.Context, _ string, _ time.Duration) (bool, error) {
	return false, context.DeadlineExceeded
}

// TestCheckAndPublishQuotaThresholds_DedupErrorSkips drives the dedup-store error
// arm: when the dedup SET NX fails, the crossing is skipped (fail-safe: no
// duplicate publishes) rather than published anyway.
func TestCheckAndPublishQuotaThresholds_DedupErrorSkips(t *testing.T) {
	t.Setenv("LLM_QUOTA_NATS_ENABLED", "true")

	pub := &fakeThresholdPublisher{}

	params := quotaThresholdParams{
		UserId:            8,
		IdentityAccountID: 22,
		TenantID:          "default",
		QuotaConsumed:     300,
		UsedTokensAfter:   600,
		LimitTokens:       1000,
	}

	checkAndPublishQuotaThresholds(context.Background(), params, erroringDeduper{}, pub)

	if len(pub.published) != 0 {
		t.Errorf("published %d events despite dedup error, want 0 (fail-safe skip)", len(pub.published))
	}
}

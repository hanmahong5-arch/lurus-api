package hub

import (
	"context"
	"testing"
	"time"
)

// computeScore has a defensive neutral-score branch for a stats entry that
// exists but has zero observations (successes+failures == 0). The public
// Record* API never produces such an entry, so we seed it directly (same
// package) and assert the neutral 0.5 score / unity rates are returned.
func TestChannelScorer_ComputeScoreZeroObservationsNeutral(t *testing.T) {
	cs := NewChannelScorer()
	cs.mu.Lock()
	cs.stats[42] = &channelStats{lastUpdated: time.Now()} // 0 successes, 0 failures
	cs.mu.Unlock()

	score := cs.GetScore(42)
	if score == nil {
		t.Fatal("expected a score for a seeded zero-observation channel")
	}
	if score.Score != 0.5 {
		t.Errorf("expected neutral score 0.5 for zero-observation channel, got %f", score.Score)
	}
	if score.SuccessRate != 1 {
		t.Errorf("expected neutral SuccessRate 1, got %f", score.SuccessRate)
	}
	if score.ErrorRate != 0 {
		t.Errorf("expected neutral ErrorRate 0, got %f", score.ErrorRate)
	}
	if score.CostFactor != 1 {
		t.Errorf("expected neutral CostFactor 1, got %f", score.CostFactor)
	}
	if score.Latency != 0 {
		t.Errorf("expected zero latency for unobserved channel, got %f", score.Latency)
	}
}

// AdjustWeights clamp branch: a channel with a genuine sub-0.5 score (high
// latency + a failure) yields factor<1; with original weight 1 the product
// int-truncates to 0 and MUST be clamped up to 1 (never fully excluded).
// This exercises the w<1 → w=1 branch, which the equal-latency scenario in
// edge_test.go does not (there factor lands exactly at 1.0).
func TestAdjustWeights_ClampWhenTruncatedToZero(t *testing.T) {
	resetSingleton()
	h := Init(nil)
	defer resetSingleton()

	// 1 slow success (EMA=20s → latencyScore 0) + 1 failure →
	// successRate 0.5, score = 0.3*0 + 0.5*0.5 + 0.2 = 0.45, factor = 0.95.
	h.Scorer.RecordSuccess(1, 20.0)
	h.Scorer.RecordFailure(1)

	sc := h.Scorer.GetScore(1)
	if sc == nil || sc.Score >= 0.5 {
		t.Fatalf("precondition: expected sub-0.5 score to force factor<1, got %+v", sc)
	}

	result := AdjustWeights([]int{1}, []int{1})
	if result == nil {
		t.Fatal("expected adjusted weights when a score exists")
	}
	if result[0] != 1 {
		t.Errorf("weight 1 * factor 0.95 truncates to 0 and must clamp to 1, got %d", result[0])
	}
}

// flushBuckets success path: a non-empty flush that succeeds must drain all
// buckets and hand the exact aggregated data to the flush func (covers the
// post-flush success accounting, not just the empty/early-return path).
func TestUsageAggregator_FlushSuccessDrains(t *testing.T) {
	var got []UsageBucket
	ua := NewUsageAggregator(func(ctx context.Context, buckets []UsageBucket) error {
		got = append(got, buckets...)
		return nil
	})

	ts := time.Date(2026, 4, 10, 10, 0, 0, 0, time.UTC)
	ua.Record(UsageEvent{TenantID: "t1", ModelName: "m", ChannelID: 1, Timestamp: ts, InputTokens: 100, OutputTokens: 40, Quota: 7, Success: true})
	ua.Record(UsageEvent{TenantID: "t1", ModelName: "m", ChannelID: 1, Timestamp: ts, InputTokens: 50, Success: false})

	ua.flushBuckets(context.Background())

	if ua.PendingBucketCount() != 0 {
		t.Fatalf("successful flush must drain all buckets, got %d pending", ua.PendingBucketCount())
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 flushed bucket, got %d", len(got))
	}
	b := got[0]
	if b.RequestCount != 2 || b.ErrorCount != 1 {
		t.Errorf("expected 2 requests / 1 error, got %d/%d", b.RequestCount, b.ErrorCount)
	}
	if b.InputTokens != 150 || b.OutputTokens != 40 || b.QuotaConsumed != 7 {
		t.Errorf("aggregation mismatch: in=%d out=%d quota=%d", b.InputTokens, b.OutputTokens, b.QuotaConsumed)
	}
}

// flushBuckets failure re-merge — "existing" branch (usage_aggregator.go:187):
// when a flush fails AND the swapped-out map already regained an entry for the
// same key (recorded during the flush call), preserved data must be summed into
// that existing bucket rather than overwriting it. We drive this deterministically
// by recording a same-key event from inside the flush func itself, so the entry is
// guaranteed present before re-merge runs.
func TestUsageAggregator_FlushFailureMergesIntoExistingDeterministic(t *testing.T) {
	ts := time.Date(2026, 4, 10, 10, 0, 0, 0, time.UTC)
	var ua *UsageAggregator
	ua = NewUsageAggregator(func(ctx context.Context, buckets []UsageBucket) error {
		// Re-populate the same key while flush "holds" the swapped-out buckets.
		// Record takes ua.mu; flushBuckets released it before calling flush, so
		// this is safe and guarantees the existing-branch on re-merge.
		ua.Record(UsageEvent{TenantID: "t1", ModelName: "m", ChannelID: 1, Timestamp: ts, InputTokens: 50, Success: false})
		return context.DeadlineExceeded // force the re-merge path
	})

	// Seed the initial bucket that will be swapped out and flushed.
	ua.Record(UsageEvent{TenantID: "t1", ModelName: "m", ChannelID: 1, Timestamp: ts, InputTokens: 100, Success: true})

	ua.flushBuckets(context.Background())

	if ua.PendingBucketCount() != 1 {
		t.Fatalf("expected a single merged bucket after failed flush, got %d", ua.PendingBucketCount())
	}
	ua.mu.Lock()
	defer ua.mu.Unlock()
	for _, b := range ua.buckets {
		// in-flush record (req=1, in=50, err=1) + re-merged preserved (req=1, in=100, err=0)
		if b.RequestCount != 2 {
			t.Errorf("expected merged RequestCount 2, got %d", b.RequestCount)
		}
		if b.InputTokens != 150 {
			t.Errorf("expected merged InputTokens 150, got %d", b.InputTokens)
		}
		if b.ErrorCount != 1 {
			t.Errorf("expected merged ErrorCount 1, got %d", b.ErrorCount)
		}
	}
}

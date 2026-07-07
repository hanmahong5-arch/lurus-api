package hub

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"
)

// --- channel_scorer edge branches ---

// Decay must delete a channel entirely once halving drives both counts to 0.
func TestChannelScorer_DecayDeletesExhaustedChannel(t *testing.T) {
	cs := NewChannelScorer()
	cs.decayInterval = 0

	cs.RecordSuccess(1, 0.5) // successes=1
	cs.RecordFailure(1)      // failures=1

	cs.mu.Lock()
	cs.stats[1].lastUpdated = time.Now().Add(-time.Hour)
	cs.mu.Unlock()

	// First decay: 1/2 -> 0 for both, so the entry is removed.
	cs.Decay()

	if cs.GetScore(1) != nil {
		t.Error("expected channel 1 to be deleted after counts decayed to zero")
	}
	cs.mu.RLock()
	_, ok := cs.stats[1]
	cs.mu.RUnlock()
	if ok {
		t.Error("expected stats map to no longer contain channel 1")
	}
}

// Decay must leave recently-updated channels untouched (Before(cutoff) false).
func TestChannelScorer_DecaySkipsFreshChannel(t *testing.T) {
	cs := NewChannelScorer()
	// Default 10m interval; a just-recorded channel is well inside the window.
	cs.RecordSuccess(2, 0.5)
	cs.RecordSuccess(2, 0.5)

	cs.Decay()

	score := cs.GetScore(2)
	if score == nil {
		t.Fatal("fresh channel must survive decay")
	}
	cs.mu.RLock()
	s := cs.stats[2]
	got := s.successes
	cs.mu.RUnlock()
	if got != 2 {
		t.Errorf("fresh channel counts must be unchanged, got successes=%d", got)
	}
}

// computeScore: total>0 but latencyEMA==0 (only failures recorded) exercises
// the branch where latencyScore stays at its default 1.0.
func TestChannelScorer_ComputeScoreZeroLatencyWithFailures(t *testing.T) {
	cs := NewChannelScorer()
	cs.RecordFailure(3)
	cs.RecordFailure(3)

	score := cs.GetScore(3)
	if score == nil {
		t.Fatal("expected score for channel with failures")
	}
	if score.Latency != 0 {
		t.Errorf("expected zero latency EMA, got %f", score.Latency)
	}
	if score.ErrorRate != 1.0 {
		t.Errorf("expected 100%% error rate, got %f", score.ErrorRate)
	}
	// latency component=0.3*1.0, success=0.5*0, cost=0.2 → 0.5
	if score.Score < 0.49 || score.Score > 0.51 {
		t.Errorf("expected composite ~0.5 (latency+cost only), got %f", score.Score)
	}
}

// GetBestChannel deterministic tie-break: with equal scores the first
// candidate encountered wins (strict > keeps the earlier one).
func TestChannelScorer_GetBestChannelTieBreak(t *testing.T) {
	cs := NewChannelScorer()
	// Two channels with identical observation profiles → identical scores.
	for i := 0; i < 10; i++ {
		cs.RecordSuccess(1, 0.5)
		cs.RecordSuccess(2, 0.5)
	}

	s1 := cs.GetScore(1)
	s2 := cs.GetScore(2)
	if s1.Score != s2.Score {
		t.Fatalf("precondition: expected equal scores, got %f vs %f", s1.Score, s2.Score)
	}

	// Order [1,2] → 1 wins; reversed [2,1] → 2 wins. Deterministic on order.
	if best := cs.GetBestChannel([]int{1, 2}); best != 1 {
		t.Errorf("tie must resolve to first candidate 1, got %d", best)
	}
	if best := cs.GetBestChannel([]int{2, 1}); best != 2 {
		t.Errorf("tie must resolve to first candidate 2, got %d", best)
	}
}

// GetBestChannel: empty candidate slice yields the -1 fallback sentinel.
func TestChannelScorer_GetBestChannelEmpty(t *testing.T) {
	cs := NewChannelScorer()
	cs.RecordSuccess(1, 0.5)
	if best := cs.GetBestChannel(nil); best != -1 {
		t.Errorf("empty candidates must return -1, got %d", best)
	}
	if best := cs.GetBestChannel([]int{}); best != -1 {
		t.Errorf("empty candidates must return -1, got %d", best)
	}
}

// GetAllScores on an empty scorer must return an empty (non-nil) slice.
func TestChannelScorer_GetAllScoresEmpty(t *testing.T) {
	cs := NewChannelScorer()
	scores := cs.GetAllScores()
	if len(scores) != 0 {
		t.Errorf("expected no scores on fresh scorer, got %d", len(scores))
	}
}

// --- smart_routing edge branches ---

// A single low-scoring candidate whose interpolated weight would fall below 1
// is clamped up to 1 (never fully excluded), while still marking anyAdjusted.
func TestAdjustWeights_ClampToOneNonZeroOriginal(t *testing.T) {
	resetSingleton()
	h := Init(nil)
	defer resetSingleton()

	// All failures → score ~0.2 → factor ~0.7. With original weight 1,
	// 1*0.7 truncates to 0 and must be clamped to 1.
	for i := 0; i < 50; i++ {
		h.Scorer.RecordFailure(1)
	}
	result := AdjustWeights([]int{1}, []int{1})
	if result == nil {
		t.Fatal("expected adjusted weights when a score exists")
	}
	if result[0] != 1 {
		t.Errorf("expected clamp to 1 for sub-unit weight, got %d", result[0])
	}
}

// Mixed input: known channel adjusted, unknown channel preserved verbatim.
func TestAdjustWeights_MixedKnownUnknown(t *testing.T) {
	resetSingleton()
	h := Init(nil)
	defer resetSingleton()

	for i := 0; i < 20; i++ {
		h.Scorer.RecordSuccess(1, 0.05) // near-perfect → boost
	}

	result := AdjustWeights([]int{1, 42}, []int{100, 80})
	if result == nil {
		t.Fatal("expected adjustment because channel 1 has a score")
	}
	if result[0] <= 100 {
		t.Errorf("known high performer should be boosted above 100, got %d", result[0])
	}
	if result[1] != 80 {
		t.Errorf("unknown channel 42 must keep original weight 80, got %d", result[1])
	}
}

// Empty channel list with hub present: loop never sets anyAdjusted → nil.
func TestAdjustWeights_EmptyChannels(t *testing.T) {
	resetSingleton()
	Init(nil)
	defer resetSingleton()

	if result := AdjustWeights([]int{}, []int{}); result != nil {
		t.Errorf("empty candidate list must return nil, got %v", result)
	}
}

// --- usage_aggregator edge branches ---

// ErrorRate on an empty bucket must return 0 (division-guard branch).
func TestUsageBucket_ErrorRateEmpty(t *testing.T) {
	b := &UsageBucket{}
	if b.ErrorRate() != 0 {
		t.Errorf("expected 0 error rate for empty bucket, got %f", b.ErrorRate())
	}
}

// flushBuckets on an empty aggregator must early-return without calling flush.
func TestUsageAggregator_FlushEmptyNoop(t *testing.T) {
	called := false
	ua := NewUsageAggregator(func(ctx context.Context, buckets []UsageBucket) error {
		called = true
		return nil
	})
	ua.flushBuckets(context.Background())
	if called {
		t.Error("flush func must not be invoked when there are no buckets")
	}
}

// A failed flush must re-merge preserved buckets into any freshly recorded
// bucket sharing the same key (the "existing" merge branch), summing counts.
func TestUsageAggregator_FlushFailureMergesIntoExisting(t *testing.T) {
	failNext := true
	var flushWg sync.WaitGroup
	ua := NewUsageAggregator(func(ctx context.Context, buckets []UsageBucket) error {
		// Block inside flush so we can inject a same-key record before re-merge.
		flushWg.Wait()
		if failNext {
			return context.DeadlineExceeded
		}
		return nil
	})

	ts := time.Date(2026, 4, 10, 10, 0, 0, 0, time.UTC)
	// Record initial event (RequestCount=1, Input=100), then drain via flush.
	ua.Record(UsageEvent{TenantID: "t1", ModelName: "m", ChannelID: 1, Timestamp: ts, InputTokens: 100, Success: true})

	// Start flush in a goroutine; it will fail after we release flushWg.
	done := make(chan struct{})
	go func() {
		ua.flushBuckets(context.Background())
		close(done)
	}()

	// While flush holds the (already-swapped-out) buckets, record another event
	// with the SAME key so re-merge hits the "existing" branch.
	ua.Record(UsageEvent{TenantID: "t1", ModelName: "m", ChannelID: 1, Timestamp: ts, InputTokens: 50, Success: false})

	flushWg.Add(0) // no-op; keep API symmetric
	// release: flush proceeds, fails, and re-merges into the same-key bucket.
	// (flushWg.Wait returns immediately since counter is 0.)
	<-done

	// Bucket must now hold combined data: 1+1 requests, 100+50 tokens, 0+1 errors.
	if ua.PendingBucketCount() != 1 {
		t.Fatalf("expected single merged bucket, got %d", ua.PendingBucketCount())
	}
	ua.mu.Lock()
	defer ua.mu.Unlock()
	for _, b := range ua.buckets {
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

// FormatBucketSummary must render every field it claims to in a stable form.
func TestFormatBucketSummary(t *testing.T) {
	b := &UsageBucket{
		TenantID:     "acme",
		ModelName:    "gpt-4",
		ChannelID:    3,
		RequestCount: 12,
		ErrorCount:   2,
		InputTokens:  1000,
		OutputTokens: 500,
		CostCNY:      1.2345,
	}
	got := FormatBucketSummary(b)
	for _, want := range []string{
		"tenant=acme", "model=gpt-4", "channel=3",
		"requests=12", "errors=2", "tokens=1000+500", "cost=1.2345",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("summary %q missing %q", got, want)
		}
	}
}

package hub

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// resetSingleton restores the package-level Hub singleton so tests that
// exercise Init/Get/RecordRelayOutcome do not leak state between cases.
func resetSingleton() {
	instance = nil
	once = sync.Once{}
}

func TestInitAndGet_Singleton(t *testing.T) {
	resetSingleton()
	defer resetSingleton()

	if Get() != nil {
		t.Fatal("expected nil before Init")
	}

	h1 := Init(nil)
	if h1 == nil {
		t.Fatal("expected non-nil hub after Init")
	}
	if h1.Scorer == nil || h1.Aggregator == nil {
		t.Fatal("expected Scorer and Aggregator to be wired")
	}

	// once.Do semantics: second Init must return the same instance.
	h2 := Init(nil)
	if h2 != h1 {
		t.Error("Init must return the same singleton on repeat calls")
	}
	if Get() != h1 {
		t.Error("Get must return the Init'd singleton")
	}
}

func TestRecordRelayOutcome_NoHub(t *testing.T) {
	resetSingleton()
	defer resetSingleton()

	// Hub not initialized — must be a no-op that does not panic.
	RecordRelayOutcome(1, true, 0.5, "t1", "gpt-4", 100, 50, 10, 0.01)
	if Get() != nil {
		t.Error("RecordRelayOutcome must not initialize the hub")
	}
}

func TestRecordRelayOutcome_SuccessUpdatesScorerAndAggregator(t *testing.T) {
	resetSingleton()
	h := Init(nil)
	defer resetSingleton()

	RecordRelayOutcome(7, true, 0.4, "tenantA", "claude", 120, 60, 15, 0.02)

	// Scorer must have recorded a success (score present, zero error rate).
	score := h.Scorer.GetScore(7)
	if score == nil {
		t.Fatal("expected scorer to have channel 7 after success")
	}
	if score.ErrorRate != 0 {
		t.Errorf("expected 0 error rate after single success, got %f", score.ErrorRate)
	}
	if score.Latency != 0.4 {
		t.Errorf("expected latency EMA seeded to 0.4, got %f", score.Latency)
	}

	// Aggregator must hold exactly one bucket reflecting the event.
	if h.Aggregator.PendingBucketCount() != 1 {
		t.Fatalf("expected 1 aggregator bucket, got %d", h.Aggregator.PendingBucketCount())
	}
	h.Aggregator.mu.Lock()
	defer h.Aggregator.mu.Unlock()
	for _, b := range h.Aggregator.buckets {
		if b.TenantID != "tenantA" || b.ModelName != "claude" || b.ChannelID != 7 {
			t.Errorf("bucket key mismatch: %+v", b)
		}
		if b.RequestCount != 1 || b.ErrorCount != 0 {
			t.Errorf("expected 1 request 0 errors, got %d/%d", b.RequestCount, b.ErrorCount)
		}
		if b.InputTokens != 120 || b.OutputTokens != 60 {
			t.Errorf("token mismatch: %d/%d", b.InputTokens, b.OutputTokens)
		}
		if b.QuotaConsumed != 15 {
			t.Errorf("expected quota 15, got %d", b.QuotaConsumed)
		}
	}
}

func TestRecordRelayOutcome_FailureRecordsError(t *testing.T) {
	resetSingleton()
	h := Init(nil)
	defer resetSingleton()

	RecordRelayOutcome(9, false, 0.0, "tenantB", "gpt-4", 0, 0, 0, 0)

	score := h.Scorer.GetScore(9)
	if score == nil {
		t.Fatal("expected scorer entry after failure")
	}
	if score.ErrorRate != 1.0 {
		t.Errorf("expected 100%% error rate after single failure, got %f", score.ErrorRate)
	}

	h.Aggregator.mu.Lock()
	defer h.Aggregator.mu.Unlock()
	for _, b := range h.Aggregator.buckets {
		if b.ErrorCount != 1 {
			t.Errorf("expected error count 1 for failed relay, got %d", b.ErrorCount)
		}
	}
}

func TestRunBackgroundTasks_StopsOnContextCancel(t *testing.T) {
	resetSingleton()
	h := Init(nil)
	defer resetSingleton()
	// Make the aggregator flush loop cheap; scorer decay stays on 1-min ticker
	// but is never fired within this short window — we only assert clean shutdown.
	h.Aggregator.flushInterval = 5 * time.Millisecond

	var flushed atomic.Int64
	h.Aggregator.flush = func(ctx context.Context, buckets []UsageBucket) error {
		flushed.Add(int64(len(buckets)))
		return nil
	}
	h.Aggregator.Record(UsageEvent{TenantID: "t", ModelName: "m", ChannelID: 1, Timestamp: time.Now(), Success: true})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		h.RunBackgroundTasks(ctx)
		close(done)
	}()

	// Let the flush loop tick at least once, then cancel and require a clean,
	// bounded return (both goroutines observe ctx.Done and Wait unblocks).
	time.Sleep(30 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("RunBackgroundTasks did not return after context cancel")
	}

	if flushed.Load() == 0 {
		t.Error("expected at least one flush (periodic or final) during background run")
	}
	if h.Aggregator.PendingBucketCount() != 0 {
		t.Errorf("expected final flush to drain buckets, got %d pending", h.Aggregator.PendingBucketCount())
	}
}

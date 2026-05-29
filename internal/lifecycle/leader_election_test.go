package lifecycle

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

// TestLeaderTask_RunsOnlyWhenLeader verifies the per-tick leadership gate:
// fn must not run while this node is not the leader, and must run once
// leadership is acquired. The acquire/renew semantics themselves are covered
// deterministically by the repo TryAcquireOrRenew tests; here we exercise only
// the gating with an injected leadership flag.
func TestLeaderTask_RunsOnlyWhenLeader(t *testing.T) {
	var runs atomic.Int64
	var leader atomic.Bool

	task := NewLeaderTask("test-task", 10*time.Millisecond, func(ctx context.Context) error {
		runs.Add(1)
		return nil
	})
	task.isLeader = leader.Load // inject deterministic leadership

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		_ = task.Run(ctx)
		close(done)
	}()
	defer func() {
		cancel()
		<-done
	}()

	// Phase 1: not leader — fn must not run across several ticks.
	time.Sleep(60 * time.Millisecond)
	if got := runs.Load(); got != 0 {
		t.Fatalf("expected 0 runs while not leader, got %d", got)
	}

	// Phase 2: become leader — fn should start running.
	leader.Store(true)
	time.Sleep(60 * time.Millisecond)
	if got := runs.Load(); got == 0 {
		t.Fatal("expected fn to run after becoming leader, got 0")
	}

	// Phase 3: lose leadership — runs must stop growing (allow one in-flight tick).
	leader.Store(false)
	time.Sleep(15 * time.Millisecond)
	stable := runs.Load()
	time.Sleep(50 * time.Millisecond)
	if got := runs.Load(); got > stable {
		t.Fatalf("expected runs to stop after losing leadership: was %d, now %d", stable, got)
	}
}

// TestLeaderTask_StopsOnContextCancel verifies the task returns promptly when
// its context is cancelled.
func TestLeaderTask_StopsOnContextCancel(t *testing.T) {
	var leader atomic.Bool
	leader.Store(true)

	task := NewLeaderTask("stop-task", 10*time.Millisecond, func(ctx context.Context) error { return nil })
	task.isLeader = leader.Load

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		_ = task.Run(ctx)
		close(done)
	}()

	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("LeaderTask.Run did not return after context cancel")
	}
}

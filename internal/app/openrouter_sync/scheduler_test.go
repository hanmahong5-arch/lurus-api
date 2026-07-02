package openrouter_sync

import (
	"context"
	"testing"
	"time"

	"github.com/LurusTech/lurus-hub/internal/pkg/common"
)

// TestAutoSyncWithContext_SlaveNodeReturnsImmediately verifies the slave-node
// early-return path: when IsMasterNode is false the function returns without
// starting the ticker, so it must complete well within a short deadline.
func TestAutoSyncWithContext_SlaveNodeReturnsImmediately(t *testing.T) {
	prev := common.IsMasterNode
	common.IsMasterNode = false
	t.Cleanup(func() { common.IsMasterNode = prev })

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	done := make(chan struct{})
	go func() {
		AutoSyncWithContext(ctx, 60)
		close(done)
	}()

	select {
	case <-done:
		// good — returned immediately
	case <-time.After(500 * time.Millisecond):
		t.Fatal("AutoSyncWithContext did not return on slave node within 500ms")
	}
}

// TestAutoSyncWithContext_DisabledFrequencyReturnsImmediately verifies that
// frequencyMinutes <= 0 causes an immediate return even when IsMasterNode is true.
func TestAutoSyncWithContext_DisabledFrequencyReturnsImmediately(t *testing.T) {
	prev := common.IsMasterNode
	common.IsMasterNode = true
	t.Cleanup(func() { common.IsMasterNode = prev })

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	done := make(chan struct{})
	go func() {
		AutoSyncWithContext(ctx, 0) // frequency=0 disables
		close(done)
	}()

	select {
	case <-done:
		// good
	case <-time.After(500 * time.Millisecond):
		t.Fatal("AutoSyncWithContext did not return for frequency=0 within 500ms")
	}
}

// TestAutoSyncWithContext_NegativeFrequencyReturnsImmediately covers the
// negative branch (frequency < 0).
func TestAutoSyncWithContext_NegativeFrequencyReturnsImmediately(t *testing.T) {
	prev := common.IsMasterNode
	common.IsMasterNode = true
	t.Cleanup(func() { common.IsMasterNode = prev })

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	done := make(chan struct{})
	go func() {
		AutoSyncWithContext(ctx, -1)
		close(done)
	}()

	select {
	case <-done:
		// good
	case <-time.After(500 * time.Millisecond):
		t.Fatal("AutoSyncWithContext did not return for frequency=-1 within 500ms")
	}
}

// TestAutoAggregateWithContext_SlaveNodeReturnsImmediately verifies the
// slave-node early-return path in AutoAggregateWithContext.
func TestAutoAggregateWithContext_SlaveNodeReturnsImmediately(t *testing.T) {
	prev := common.IsMasterNode
	common.IsMasterNode = false
	t.Cleanup(func() { common.IsMasterNode = prev })

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	done := make(chan struct{})
	go func() {
		AutoAggregateWithContext(ctx)
		close(done)
	}()

	select {
	case <-done:
		// good
	case <-time.After(500 * time.Millisecond):
		t.Fatal("AutoAggregateWithContext did not return on slave node within 500ms")
	}
}

// TestAutoAggregateWithContext_MasterContextCancel verifies that a master node
// that enters the ticker loop exits cleanly when the context is cancelled.
// We set IsLeader=false so the initial AggregateOpenRouterUsage call (which
// needs DB) is skipped; cancellation then stops the loop.
func TestAutoAggregateWithContext_MasterContextCancel(t *testing.T) {
	prev := common.IsMasterNode
	common.IsMasterNode = true
	t.Cleanup(func() { common.IsMasterNode = prev })

	prevLeader := common.IsLeader()
	common.SetLeader(false) // suppress the startup DB call
	t.Cleanup(func() { common.SetLeader(prevLeader) })

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		AutoAggregateWithContext(ctx)
		close(done)
	}()

	// Give the goroutine time to enter the for-select loop.
	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case <-done:
		// good — context cancellation stopped the loop
	case <-time.After(2 * time.Second):
		t.Fatal("AutoAggregateWithContext did not stop after context cancel")
	}
}

// TestAutoSyncWithContext_MasterContextCancel verifies that a master node with
// a long ticker exits when the context is cancelled (no DB/HTTP contact because
// the ticker period is long and no tick fires before cancel).
func TestAutoSyncWithContext_MasterContextCancel(t *testing.T) {
	prev := common.IsMasterNode
	common.IsMasterNode = true
	t.Cleanup(func() { common.IsMasterNode = prev })

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		// 999 minutes — ticker will not fire before we cancel
		AutoSyncWithContext(ctx, 999)
		close(done)
	}()

	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case <-done:
		// good
	case <-time.After(2 * time.Second):
		t.Fatal("AutoSyncWithContext did not stop after context cancel")
	}
}

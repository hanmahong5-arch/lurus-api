package openrouter_pool

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/LurusTech/lurus-hub/internal/adapter/repo"
	"github.com/LurusTech/lurus-hub/internal/pkg/common"
)

// These tests close the residual reaper branches the existing PG tests miss:
// the reapChannel non-multi-key early return, the ListOpenRouterMultiKeyChannels
// error wrap in ReapOnce, and the leader-at-boot initial-run-failed log path in
// AutoReapWithContext. Each guards a real operational contract — a single-key
// channel must never be touched by the pool reaper, a listing failure must be
// surfaced (not silently treated as "no channels"), and a boot-time failure
// must not crash the ticker.

// TestReapChannel_NonMultiKey_NoOp calls reapChannel directly with a single-key
// (non-multi-key) channel. The reaper must return (0, nil) immediately: pool
// cooldown logic only applies to multi-key OpenRouter channels; touching a
// single-key channel here would corrupt its state.
func TestReapChannel_NonMultiKey_NoOp(t *testing.T) {
	ch := &repo.Channel{
		Id:   7,
		Type: 20,
		ChannelInfo: repo.ChannelInfo{
			IsMultiKey: false,
			// Even if a stray cooldown map is present, the IsMultiKey guard must
			// fire first and skip it entirely.
			MultiKeyCooldownUntil: map[int]int64{0: 1},
		},
	}
	recovered, err := reapChannel(ch, time.Now())
	if err != nil {
		t.Fatalf("non-multi-key reapChannel must not error, got %v", err)
	}
	if recovered != 0 {
		t.Fatalf("non-multi-key channel must recover 0 keys, got %d", recovered)
	}
}

// TestReapOnce_ListError_Wrapped forces the ListOpenRouterMultiKeyChannels error
// path: with the channels table dropped, the SELECT fails and ReapOnce must
// return a wrapped "list channels" error rather than silently treating it as an
// empty fleet (which would mask an outage and stall all recovery).
func TestReapOnce_ListError_Wrapped(t *testing.T) {
	setupPoolTestDB(t)

	// Drop the channels table so the reaper's initial SELECT errors.
	if err := repo.DB.Migrator().DropTable(&repo.Channel{}); err != nil {
		t.Fatalf("drop channels table: %v", err)
	}

	err := ReapOnce(context.Background(), time.Now)
	if err == nil {
		t.Fatalf("expected a list-channels error when the channels table is missing")
	}
	if !strings.Contains(err.Error(), "list channels") {
		t.Fatalf("error should be wrapped as 'list channels', got %q", err.Error())
	}
}

// TestAutoReapWithContext_LeaderInitialRunFailed_DoesNotCrash drives the
// leader-at-boot branch where the very first ReapOnce fails (channels table
// missing). The ticker loop must log the failure and keep running until the
// context is cancelled — a transient boot-time DB error must never take down
// the reaper goroutine.
func TestAutoReapWithContext_LeaderInitialRunFailed_DoesNotCrash(t *testing.T) {
	setupPoolTestDB(t)

	prevLeader := common.IsLeader()
	common.SetLeader(true)
	t.Cleanup(func() { common.SetLeader(prevLeader) })

	// Drop channels so the initial-run ReapOnce (executed synchronously at boot
	// because this node is leader) fails and hits the "initial run failed" log.
	if err := repo.DB.Migrator().DropTable(&repo.Channel{}); err != nil {
		t.Fatalf("drop channels table: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		AutoReapWithContext(ctx)
		close(done)
	}()

	// Give the goroutine time to run the failing initial sweep, then shut down.
	time.Sleep(150 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("AutoReapWithContext did not return after a failed initial run + cancel")
	}
}

package common

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

// withTestRedis points common.RDB at a real Redis for the duration of a test and
// restores the prior client on cleanup. Skips (not fails) when NEWHUB_REDIS_TEST_ADDR
// is unset, so hermetic `-short` and Redis-less CI jobs are unaffected; the
// disposable-Redis verification runs it by exporting the addr.
func withTestRedis(t *testing.T) {
	t.Helper()
	addr := os.Getenv("NEWHUB_REDIS_TEST_ADDR")
	if addr == "" || testing.Short() {
		t.Skip("set NEWHUB_REDIS_TEST_ADDR and run without -short to exercise the real-Redis degrade path")
	}
	client := redis.NewClient(&redis.Options{Addr: addr})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		t.Fatalf("test Redis at %s unreachable: %v", addr, err)
	}
	if err := client.FlushDB(ctx).Err(); err != nil {
		t.Fatalf("flushdb: %v", err)
	}

	prevRDB, prevEnabled := RDB, RedisEnabled
	RDB, RedisEnabled = client, true
	t.Cleanup(func() {
		RDB, RedisEnabled = prevRDB, prevEnabled
		_ = client.Close()
	})
}

// TestIntegration_DegradeAdmitPath_RealRedis exercises the P1-2 ADMIT path end to
// end against a real Redis — the case the hermetic unit tests can't reach because
// both the wallet-balance cache and the spend-cap reservation require Redis. This
// is the safety-critical "admit on cached credit while billing is down" path, so
// it must be verified against real infrastructure, not just guard-tested.
func TestIntegration_DegradeAdmitPath_RealRedis(t *testing.T) {
	withTestRedis(t)
	resetBillingBreaker(t)
	openBillingBreaker(t)

	const (
		accountID int64 = 9001
		tenantID        = "tenant-admit"
		estimate        = 0.5 // LB; well under the cached balance
	)

	// Round-trip a fresh cached balance through real Redis.
	SetCachedWalletBalance(accountID, 1000.0)
	if bal, ok := GetCachedWalletBalance(accountID); !ok || bal != 1000.0 {
		t.Fatalf("cached balance round-trip failed: bal=%v ok=%v", bal, ok)
	}

	// Generous cap so the admit decision (not the cap) is what we test here.
	prevCap, prevWin := BillingDegradedSpendCapLB, BillingDegradedWindowSec
	BillingDegradedSpendCapLB, BillingDegradedWindowSec = 100.0, 3600
	t.Cleanup(func() { BillingDegradedSpendCapLB, BillingDegradedWindowSec = prevCap, prevWin })

	if !TryDegradedPreAuth(tenantID, accountID, estimate, errBreakerOpen()) {
		t.Fatal("expected ADMIT: breaker open + fresh ample cached balance + under cap")
	}

	// The reservation must have landed in Redis with a window TTL.
	ctx := context.Background()
	reserved, err := RDB.Get(ctx, "billing:degraded:"+tenantID).Float64()
	if err != nil {
		t.Fatalf("degraded-spend key not written: %v", err)
	}
	if reserved != estimate {
		t.Errorf("reserved spend = %v, want %v", reserved, estimate)
	}
	ttl, err := RDB.TTL(ctx, "billing:degraded:"+tenantID).Result()
	if err != nil || ttl <= 0 || ttl > time.Hour {
		t.Errorf("degraded-spend key TTL = %v (err=%v), want (0, 1h]", ttl, err)
	}
}

// TestIntegration_DegradeCapExhaustion_RealRedis proves the per-tenant unsecured-
// spend cap actually bounds admissions: once cumulative reserved spend reaches the
// cap, further requests are denied (fail-closed) even though the breaker is still
// open and the balance is still ample.
func TestIntegration_DegradeCapExhaustion_RealRedis(t *testing.T) {
	withTestRedis(t)
	resetBillingBreaker(t)
	openBillingBreaker(t)

	const (
		accountID int64 = 9002
		tenantID        = "tenant-cap"
		estimate        = 0.5
	)
	SetCachedWalletBalance(accountID, 1000.0)

	prevCap, prevWin := BillingDegradedSpendCapLB, BillingDegradedWindowSec
	BillingDegradedSpendCapLB, BillingDegradedWindowSec = 2.0, 3600 // cap = 4 × 0.5
	t.Cleanup(func() { BillingDegradedSpendCapLB, BillingDegradedWindowSec = prevCap, prevWin })

	admitted := 0
	for i := 0; i < 10; i++ {
		if TryDegradedPreAuth(tenantID, accountID, estimate, errBreakerOpen()) {
			admitted++
		}
	}
	// cap 2.0 / 0.5 per request = exactly 4 admits, then the cap holds the line.
	if admitted != 4 {
		t.Fatalf("cap exhaustion: admitted %d, want 4 (cap=%.1f / est=%.1f)", admitted, 2.0, estimate)
	}

	// A different tenant has an independent cap — not affected by the exhausted one.
	SetCachedWalletBalance(7777, 1000.0)
	if !TryDegradedPreAuth("tenant-other", 7777, estimate, errBreakerOpen()) {
		t.Error("a different tenant must have its own cap budget")
	}
}

func errBreakerOpen() error { return os.ErrDeadlineExceeded } // any non-insufficient_balance error

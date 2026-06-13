package common

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/LurusTech/lurus-hub/internal/pkg/metrics"
)

const (
	// walletCachePrefix is the Redis key prefix for cached wallet available balance.
	walletCachePrefix = "wallet:avail:"
	// walletCacheTTL is how long a cached balance is trusted (30 seconds).
	walletCacheTTL = 30 * time.Second
	// walletTrustThreshold is the minimum available balance (in LB) to skip pre-auth.
	// Users with balance above this threshold get fast-path (no pre-auth call).
	walletTrustThreshold = 10.0 // 10 LB — well above typical single-request cost
)

// GetCachedWalletBalance returns the cached available wallet balance for an account.
// Returns (balance, true) if cached, or (0, false) if not cached or Redis unavailable.
func GetCachedWalletBalance(accountID int64) (float64, bool) {
	if !RedisEnabled || RDB == nil {
		return 0, false
	}
	key := fmt.Sprintf("%s%d", walletCachePrefix, accountID)
	val, err := RDB.Get(context.Background(), key).Result()
	if err != nil {
		metrics.RecordCacheHit("wallet_balance", false)
		return 0, false
	}
	balance, err := strconv.ParseFloat(val, 64)
	if err != nil {
		metrics.RecordCacheHit("wallet_balance", false)
		return 0, false
	}
	metrics.RecordCacheHit("wallet_balance", true)
	return balance, true
}

// SetCachedWalletBalance updates the cached wallet balance after a successful pre-auth or settle.
func SetCachedWalletBalance(accountID int64, balance float64) {
	if !RedisEnabled || RDB == nil {
		return
	}
	key := fmt.Sprintf("%s%d", walletCachePrefix, accountID)
	RDB.Set(context.Background(), key, fmt.Sprintf("%.4f", balance), walletCacheTTL)
}

// InvalidateCachedWalletBalance removes the cached balance (e.g., after settle or topup).
func InvalidateCachedWalletBalance(accountID int64) {
	if !RedisEnabled || RDB == nil {
		return
	}
	key := fmt.Sprintf("%s%d", walletCachePrefix, accountID)
	RDB.Del(context.Background(), key)
}

// ShouldSkipPreAuth returns true if the user has a high cached balance,
// meaning we can trust them to pay and skip the synchronous pre-auth call.
// This reduces latency for premium users who rarely exhaust their balance.
func ShouldSkipPreAuth(accountID int64, estimatedLB float64) bool {
	balance, ok := GetCachedWalletBalance(accountID)
	if !ok {
		return false
	}
	return balance > walletTrustThreshold && balance > estimatedLB*3
}

// TryDegradedPreAuth decides whether a relay may proceed on cached wallet
// balance while the platform billing breaker is OPEN (P1-2 graceful
// degradation). Default is FAIL-CLOSED: it returns true only when every guard
// holds, so a billing outage degrades to "charge from cache, reconcile later"
// for trusted balances rather than a blanket 402 that takes the gateway down
// with the platform. Guards:
//   - the breaker is actually OPEN (not a balance rejection or a healthy path),
//   - the pre-auth error is NOT insufficient_balance (never degrade a real
//     "out of money" — that would hand out free spend),
//   - a fresh cached balance exists (TTL-bounded; absent cache ⇒ no degrade),
//   - the estimate sits comfortably under that balance (same 3× margin as the
//     skip-preauth fast path),
//   - admitting this estimate keeps the tenant under its rolling unsecured-spend
//     cap (reserveDegradedSpend).
// The admitted request then takes the existing no-pre-auth post-consume path
// (legacy wallet debit), identical to the high-balance ShouldSkipPreAuth path —
// so this introduces no new settlement gap. Metric: billing_degraded_total.
func TryDegradedPreAuth(tenantID string, accountID int64, estimatedLB float64, preAuthErr error) bool {
	deny := func() bool {
		metrics.BillingDegradedTotal.WithLabelValues("denied").Inc()
		return false
	}
	balance, haveFreshCache := GetCachedWalletBalance(accountID)
	// Pure policy first (exhaustively unit-tested), then the spend-cap reservation
	// (Redis side effect) only once the policy says yes.
	if !degradeAdmissible(BillingBreakerIsOpen(), haveFreshCache, balance, estimatedLB, preAuthErr) {
		return deny()
	}
	if !reserveDegradedSpend(tenantID, estimatedLB) {
		return deny()
	}
	metrics.BillingDegradedTotal.WithLabelValues("allowed").Inc()
	return true
}

// degradeAdmissible is the PURE degrade policy, separated from all I/O (breaker
// read, cache read, cap reservation) so the safety-critical decision is fully
// unit-testable without Redis. Returns true only when EVERY guard holds — the
// fail-closed default lives here. The caller still applies the per-tenant spend
// cap on top before admitting.
func degradeAdmissible(breakerOpen, haveFreshCache bool, balance, estimatedLB float64, preAuthErr error) bool {
	if !breakerOpen {
		return false // platform healthy ⇒ never skip pre-auth via this path
	}
	if preAuthErr != nil && strings.Contains(preAuthErr.Error(), "insufficient_balance") {
		return false // genuine "out of money" must fail closed, never degrade
	}
	if !haveFreshCache {
		return false // no cached balance to trust
	}
	// Same trust margin as the high-balance skip-preauth fast path: balance must
	// clear the floor AND comfortably (3×) exceed this request's estimate.
	return balance > walletTrustThreshold && balance > estimatedLB*3
}

// reserveDegradedSpend atomically reserves estimatedLB against the tenant's
// rolling degraded-spend cap. Returns false (fail-closed) when Redis is absent —
// without a shared counter the unsecured spend can't be bounded, and unbounded
// degradation is exactly what the cap exists to prevent. Worst-case overshoot is
// one request's estimate (check-then-incr is not a single round-trip), which is
// acceptable for a safety net.
func reserveDegradedSpend(tenantID string, estimatedLB float64) bool {
	if !RedisEnabled || RDB == nil {
		return false
	}
	ctx := context.Background()
	key := fmt.Sprintf("billing:degraded:%s", tenantID)
	cur, _ := RDB.Get(ctx, key).Float64() // missing key ⇒ 0
	if cur >= BillingDegradedSpendCapLB {
		return false
	}
	newVal, err := RDB.IncrByFloat(ctx, key, estimatedLB).Result()
	if err != nil {
		return false
	}
	// Stamp the rolling-window TTL on the first write of a fresh window.
	if newVal == estimatedLB {
		RDB.Expire(ctx, key, time.Duration(BillingDegradedWindowSec)*time.Second)
	}
	return true
}

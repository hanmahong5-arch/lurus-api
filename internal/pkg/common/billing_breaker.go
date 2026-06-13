package common

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/LurusTech/lurus-hub/internal/pkg/metrics"
)

// billingBreaker is a simple circuit breaker for platform billing calls.
// When the platform is unresponsive, this prevents cascading 10s timeouts
// on every request by fast-failing after consecutive failures.
//
// States:
//   - Closed: normal operation, all calls pass through
//   - Open: platform assumed down, calls rejected immediately with clear error
//   - HalfOpen: one probe request allowed; success → Closed, failure → Open
var billingBreaker = &platformBreaker{
	threshold: 3,
	timeout:   15 * time.Second,
}

type platformBreaker struct {
	mu               sync.Mutex
	consecutiveFails int
	lastFailTime     time.Time
	state            int // 0=closed, 1=open, 2=halfopen
	threshold        int
	timeout          time.Duration
}

// BillingBreakerAllow checks if the billing circuit breaker permits a call.
// Returns nil if allowed, or an error describing why the call was rejected.
func BillingBreakerAllow() error {
	billingBreaker.mu.Lock()
	defer billingBreaker.mu.Unlock()

	switch billingBreaker.state {
	case 0: // closed
		return nil
	case 1: // open
		if time.Since(billingBreaker.lastFailTime) >= billingBreaker.timeout {
			billingBreaker.state = 2 // transition to half-open
			metrics.BillingCircuitBreakerState.Set(2)
			return nil
		}
		return fmt.Errorf("billing service temporarily unavailable (circuit open, retry in %ds)",
			int(billingBreaker.timeout.Seconds()-time.Since(billingBreaker.lastFailTime).Seconds()))
	case 2: // half-open — reject concurrent probes
		return fmt.Errorf("billing service recovering (probe in progress)")
	}
	return nil
}

// BillingBreakerIsOpen reports whether the breaker is currently OPEN (platform
// deemed unreachable). Read-only — unlike BillingBreakerAllow it never mutates
// state (no half-open transition), so the P1-2 degrade path can consult it
// without perturbing the breaker. half-open (state 2) returns false: a probe is
// in flight, so we are not in the "platform is down" condition that degradation
// is meant to cover.
func BillingBreakerIsOpen() bool {
	billingBreaker.mu.Lock()
	defer billingBreaker.mu.Unlock()
	return billingBreaker.state == 1
}

// BillingBreakerSuccess records a successful billing call.
func BillingBreakerSuccess() {
	billingBreaker.mu.Lock()
	defer billingBreaker.mu.Unlock()

	if billingBreaker.state != 0 {
		slog.Info("billing circuit breaker closed — platform recovered")
	}
	billingBreaker.consecutiveFails = 0
	billingBreaker.state = 0
	metrics.BillingCircuitBreakerState.Set(0)
}

// BillingBreakerFailure records a failed billing call.
func BillingBreakerFailure() {
	billingBreaker.mu.Lock()
	defer billingBreaker.mu.Unlock()

	billingBreaker.consecutiveFails++
	billingBreaker.lastFailTime = time.Now()

	switch billingBreaker.state {
	case 0: // closed
		if billingBreaker.consecutiveFails >= billingBreaker.threshold {
			billingBreaker.state = 1
			metrics.BillingCircuitBreakerState.Set(1)
			slog.Warn("billing circuit breaker OPEN — platform unreachable",
				"consecutive_failures", billingBreaker.consecutiveFails)
		}
	case 2: // half-open probe failed
		billingBreaker.state = 1
		metrics.BillingCircuitBreakerState.Set(1)
		slog.Warn("billing circuit breaker re-opened — probe failed")
	}
}

// PreAuthorizeWithBreaker wraps PreAuthorizeGRPC with circuit breaker protection.
// When the breaker is open, returns immediately without making a network call.
func PreAuthorizeWithBreaker(ctx context.Context, accountID int64, amount float64,
	productID, referenceID, description string, ttlSeconds int) (*PreAuthResult, error) {

	if err := BillingBreakerAllow(); err != nil {
		return nil, err
	}

	result, err := PreAuthorizeGRPC(ctx, accountID, amount, productID, referenceID, description, ttlSeconds)
	if err != nil {
		BillingBreakerFailure()
		return nil, err
	}

	BillingBreakerSuccess()
	return result, nil
}

// SettleWithBreaker wraps SettlePreAuthGRPC with circuit breaker protection.
func SettleWithBreaker(ctx context.Context, preAuthID int64, actualAmount float64) (*SettlePreAuthResult, error) {
	if err := BillingBreakerAllow(); err != nil {
		return nil, err
	}

	result, err := SettlePreAuthGRPC(ctx, preAuthID, actualAmount)
	if err != nil {
		BillingBreakerFailure()
		return nil, err
	}

	BillingBreakerSuccess()
	return result, nil
}

// ReleaseWithBreaker wraps ReleasePreAuthGRPC with circuit breaker protection.
func ReleaseWithBreaker(ctx context.Context, preAuthID int64) error {
	if err := BillingBreakerAllow(); err != nil {
		return err
	}

	if err := ReleasePreAuthGRPC(ctx, preAuthID); err != nil {
		BillingBreakerFailure()
		return err
	}

	BillingBreakerSuccess()
	return nil
}

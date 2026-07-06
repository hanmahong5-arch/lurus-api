package common

import (
	"testing"
	"time"
)

// TestBillingBreakerAllow_ClosedPasses verifies the closed breaker permits calls.
func TestBillingBreakerAllow_ClosedPasses(t *testing.T) {
	resetBillingBreaker(t)
	if err := BillingBreakerAllow(); err != nil {
		t.Errorf("closed breaker must allow, got %v", err)
	}
}

// TestBillingBreakerAllow_OpenRejects verifies that once OPEN, calls are
// fast-failed with a clear error while the timeout has not elapsed.
func TestBillingBreakerAllow_OpenRejects(t *testing.T) {
	resetBillingBreaker(t)
	openBillingBreaker(t)

	err := BillingBreakerAllow()
	if err == nil {
		t.Fatal("open breaker must reject calls")
	}
	if got := err.Error(); got == "" {
		t.Error("open breaker error should be descriptive")
	}
}

// TestBillingBreakerAllow_HalfOpenTransition drives the OPEN->HalfOpen->Closed
// and OPEN->HalfOpen->Open transitions by manipulating the timeout window.
func TestBillingBreakerAllow_HalfOpenTransition(t *testing.T) {
	resetBillingBreaker(t)
	openBillingBreaker(t)

	// Force the timeout window to elapse by shortening it and back-dating the
	// last failure, so the next Allow transitions OPEN -> HalfOpen (returns nil).
	billingBreaker.mu.Lock()
	origTimeout := billingBreaker.timeout
	billingBreaker.timeout = time.Millisecond
	billingBreaker.lastFailTime = time.Now().Add(-time.Second)
	billingBreaker.mu.Unlock()
	t.Cleanup(func() {
		billingBreaker.mu.Lock()
		billingBreaker.timeout = origTimeout
		billingBreaker.mu.Unlock()
		BillingBreakerSuccess()
	})

	if err := BillingBreakerAllow(); err != nil {
		t.Fatalf("expired-timeout open breaker should transition to half-open (allow probe): %v", err)
	}
	if !isBreakerHalfOpen() {
		t.Fatal("breaker should be half-open after probe admitted")
	}

	// A second concurrent probe while half-open must be rejected.
	if err := BillingBreakerAllow(); err == nil {
		t.Error("half-open breaker must reject a concurrent probe")
	}

	// Half-open probe failure re-opens the breaker.
	BillingBreakerFailure()
	if !BillingBreakerIsOpen() {
		t.Error("failed half-open probe should re-open the breaker")
	}

	// Recover to half-open again, then a success closes it.
	billingBreaker.mu.Lock()
	billingBreaker.timeout = time.Millisecond
	billingBreaker.lastFailTime = time.Now().Add(-time.Second)
	billingBreaker.mu.Unlock()
	if err := BillingBreakerAllow(); err != nil {
		t.Fatalf("should re-enter half-open: %v", err)
	}
	BillingBreakerSuccess()
	if BillingBreakerIsOpen() || isBreakerHalfOpen() {
		t.Error("successful half-open probe should close the breaker")
	}
	if err := BillingBreakerAllow(); err != nil {
		t.Errorf("closed breaker should allow again: %v", err)
	}
}

// TestBillingBreakerFailure_BelowThreshold verifies the breaker stays closed
// until it accumulates `threshold` consecutive failures.
func TestBillingBreakerFailure_BelowThreshold(t *testing.T) {
	resetBillingBreaker(t)
	for i := 0; i < billingBreaker.threshold-1; i++ {
		BillingBreakerFailure()
	}
	if BillingBreakerIsOpen() {
		t.Error("breaker should stay closed below the failure threshold")
	}
	if err := BillingBreakerAllow(); err != nil {
		t.Errorf("sub-threshold breaker should still allow: %v", err)
	}
	// One more crosses the threshold.
	BillingBreakerFailure()
	if !BillingBreakerIsOpen() {
		t.Error("breaker should open at the threshold")
	}
}

// TestBillingBreakerSuccess_ResetsFailures ensures a success clears the
// consecutive-failure count so the breaker won't open prematurely later.
func TestBillingBreakerSuccess_ResetsFailures(t *testing.T) {
	resetBillingBreaker(t)
	BillingBreakerFailure()
	BillingBreakerFailure()
	BillingBreakerSuccess() // reset
	if billingBreaker.consecutiveFails != 0 {
		t.Errorf("success should reset consecutiveFails, got %d", billingBreaker.consecutiveFails)
	}
	// Now a single failure must not open it.
	BillingBreakerFailure()
	if BillingBreakerIsOpen() {
		t.Error("one failure after reset should not open the breaker")
	}
}

func isBreakerHalfOpen() bool {
	billingBreaker.mu.Lock()
	defer billingBreaker.mu.Unlock()
	return billingBreaker.state == 2
}

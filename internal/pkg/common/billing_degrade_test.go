package common

import (
	"errors"
	"testing"
)

// resetBillingBreaker forces the package-global breaker back to closed so each
// test starts from a known state (the breaker is a shared singleton).
func resetBillingBreaker(t *testing.T) {
	t.Helper()
	BillingBreakerSuccess()
	t.Cleanup(BillingBreakerSuccess)
}

// openBillingBreaker drives the breaker to OPEN by recording threshold failures.
func openBillingBreaker(t *testing.T) {
	t.Helper()
	for i := 0; i < billingBreaker.threshold; i++ {
		BillingBreakerFailure()
	}
	if !BillingBreakerIsOpen() {
		t.Fatalf("expected breaker OPEN after %d failures", billingBreaker.threshold)
	}
}

// TestBillingBreakerIsOpen_ReadOnly verifies IsOpen reflects state and, unlike
// BillingBreakerAllow, does not mutate it (no half-open transition side effect).
func TestBillingBreakerIsOpen_ReadOnly(t *testing.T) {
	resetBillingBreaker(t)

	if BillingBreakerIsOpen() {
		t.Fatal("breaker should start closed")
	}
	openBillingBreaker(t)
	// Repeated reads must keep returning true and not flip the breaker to half-open.
	for i := 0; i < 3; i++ {
		if !BillingBreakerIsOpen() {
			t.Fatalf("IsOpen read %d unexpectedly returned false", i)
		}
	}
}

// TestTryDegradedPreAuth_BreakerClosed_Denies: the degrade path must never
// engage while the platform is healthy — that would skip pre-auth on a working
// breaker and hand out unsecured spend.
func TestTryDegradedPreAuth_BreakerClosed_Denies(t *testing.T) {
	resetBillingBreaker(t)
	RedisEnabled = false

	if TryDegradedPreAuth("tenantX", 123, 0.5, errors.New("billing service error")) {
		t.Fatal("degrade must be denied when breaker is closed")
	}
}

// TestTryDegradedPreAuth_InsufficientBalance_Denies: even with the breaker open,
// a genuine insufficient_balance rejection must fail closed (402) — degrading it
// would let a broke user spend for free.
func TestTryDegradedPreAuth_InsufficientBalance_Denies(t *testing.T) {
	resetBillingBreaker(t)
	openBillingBreaker(t)
	RedisEnabled = false

	if TryDegradedPreAuth("tenantX", 123, 0.5, errors.New("insufficient_balance")) {
		t.Fatal("insufficient_balance must never degrade, even with breaker open")
	}
}

// TestTryDegradedPreAuth_NoRedis_FailsClosed: with the breaker open but no Redis,
// neither the cached balance nor the spend cap can be consulted — the safe
// default is to deny (no unbounded unsecured spend).
func TestTryDegradedPreAuth_NoRedis_FailsClosed(t *testing.T) {
	resetBillingBreaker(t)
	openBillingBreaker(t)
	RedisEnabled = false

	if TryDegradedPreAuth("tenantX", 123, 0.5, errors.New("billing service error")) {
		t.Fatal("degrade must fail closed without Redis (cannot bound spend)")
	}
}

// TestDegradeAdmissible_Policy exhaustively covers the PURE degrade policy — the
// admit path included, which the I/O-wrapped TryDegradedPreAuth can't reach in a
// Redis-less unit test. walletTrustThreshold is 10.0 LB.
func TestDegradeAdmissible_Policy(t *testing.T) {
	errOther := errors.New("billing service error")
	errInsufficient := errors.New("insufficient_balance")
	cases := []struct {
		name           string
		breakerOpen    bool
		haveFreshCache bool
		balance        float64
		estimatedLB    float64
		err            error
		want           bool
	}{
		{"admit: open + fresh + ample balance", true, true, 100, 1, errOther, true},
		{"deny: breaker closed (platform healthy)", false, true, 100, 1, errOther, false},
		{"deny: insufficient_balance never degrades", true, true, 100, 1, errInsufficient, false},
		{"deny: no fresh cache", true, false, 100, 1, errOther, false},
		{"deny: balance below trust floor", true, true, 9, 1, errOther, false},
		{"deny: estimate too close to balance (3x margin)", true, true, 12, 5, errOther, false},
		{"admit: exactly clears 3x margin and floor", true, true, 30.01, 10, errOther, true},
		{"deny: balance equals floor (strict >)", true, true, 10, 1, errOther, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := degradeAdmissible(tc.breakerOpen, tc.haveFreshCache, tc.balance, tc.estimatedLB, tc.err)
			if got != tc.want {
				t.Fatalf("degradeAdmissible(open=%v,cache=%v,bal=%.2f,est=%.2f,err=%v) = %v, want %v",
					tc.breakerOpen, tc.haveFreshCache, tc.balance, tc.estimatedLB, tc.err, got, tc.want)
			}
		})
	}
}

// TestReserveDegradedSpend_NoRedis_FailsClosed asserts the cap reservation is
// fail-closed when Redis is unavailable.
func TestReserveDegradedSpend_NoRedis_FailsClosed(t *testing.T) {
	RedisEnabled = false
	if reserveDegradedSpend("tenantX", 1.0) {
		t.Fatal("reserveDegradedSpend must return false without Redis")
	}
}

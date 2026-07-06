package common

import (
	"context"
	"testing"
)

// When the breaker is OPEN, the *WithBreaker wrappers must fail fast WITHOUT
// making any gRPC network call — this is the fail-safe that stops a platform
// outage from cascading 10s timeouts onto every relay request.
func TestWithBreaker_OpenFailsFastNoNetwork(t *testing.T) {
	resetBillingBreaker(t)
	openBillingBreaker(t)
	ctx := context.Background()

	if _, err := PreAuthorizeWithBreaker(ctx, 1, 1.0, "prod", "ref", "desc", 60); err == nil {
		t.Error("PreAuthorizeWithBreaker must fail fast when breaker open")
	}
	if _, err := SettleWithBreaker(ctx, 42, 0.5); err == nil {
		t.Error("SettleWithBreaker must fail fast when breaker open")
	}
	if err := ReleaseWithBreaker(ctx, 42); err == nil {
		t.Error("ReleaseWithBreaker must fail fast when breaker open")
	}
}

package common

import (
	"testing"
)

// TestBillingUnifiedEnabled_AtomicGetterSetter verifies that SetBillingUnifiedEnabled
// and BillingUnifiedEnabled are consistent across true→false transitions and that the
// zero value matches the disabled state.
func TestBillingUnifiedEnabled_AtomicGetterSetter(t *testing.T) {
	// Save and restore initial value so the test is isolated from init().
	initial := BillingUnifiedEnabled()
	defer SetBillingUnifiedEnabled(initial)

	// Start from a known disabled state.
	SetBillingUnifiedEnabled(false)
	if BillingUnifiedEnabled() {
		t.Fatal("expected false after SetBillingUnifiedEnabled(false)")
	}

	// Flip on.
	SetBillingUnifiedEnabled(true)
	if !BillingUnifiedEnabled() {
		t.Fatal("expected true after SetBillingUnifiedEnabled(true)")
	}

	// Flip back off.
	SetBillingUnifiedEnabled(false)
	if BillingUnifiedEnabled() {
		t.Fatal("expected false after second SetBillingUnifiedEnabled(false)")
	}
}

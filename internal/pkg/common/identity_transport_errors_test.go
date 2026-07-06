package common

import (
	"context"
	"net"
	"testing"
)

// TestIdentityClient_TransportErrors points the client at an address that
// refuses connections and asserts every read-side identity call degrades
// gracefully (nil / free-default, no panic) while every money-mutation call
// surfaces an error — the fail-safe contract when platform is unreachable.
func TestIdentityClient_TransportErrors(t *testing.T) {
	// Bind a port, then close it so connections are refused immediately.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	deadURL := "http://" + ln.Addr().String()
	_ = ln.Close()

	prev := IdentityServiceURL
	IdentityServiceURL = deadURL
	t.Cleanup(func() { IdentityServiceURL = prev })
	ctx := context.Background()

	// Read-side: graceful degradation to nil / defaults.
	if m, err := GetAccountByZitadelSub(ctx, "s"); m != nil || err != nil {
		t.Errorf("GetAccountByZitadelSub transport error should degrade to (nil,nil)")
	}
	if m, err := UpsertAccount(ctx, "s", "e", "n", "a"); m != nil || err != nil {
		t.Errorf("UpsertAccount transport error should degrade to (nil,nil)")
	}
	if ent, err := GetEntitlements(ctx, 1, "p"); err != nil || ent["plan_code"] != "free" {
		t.Errorf("GetEntitlements transport error should default to free")
	}
	if ov, _ := GetAccountOverview(ctx, 1, ""); ov != nil {
		t.Error("GetAccountOverview transport error should be nil")
	}
	if wb, _ := GetWalletBalance(ctx, 1); wb != nil {
		t.Error("GetWalletBalance transport error should be nil")
	}
	if m, _ := GetAccountByEmail(ctx, "e"); m != nil {
		t.Error("GetAccountByEmail transport error should be nil")
	}
	if m, _ := GetAccountByZitadelSub_ByAccountID(ctx, 1); m != nil {
		t.Error("GetAccountByAccountID transport error should be nil")
	}
	if pm, _ := GetPaymentMethods(ctx); pm != nil {
		t.Error("GetPaymentMethods transport error should be nil")
	}
	if bs, _ := GetBillingSummary(ctx, 1); bs != nil {
		t.Error("GetBillingSummary transport error should be nil")
	}
	if page, err := ListWalletTransactionsHTTP(ctx, 1, 1, 20); page != nil || err != nil {
		t.Error("ListWalletTransactions transport error should degrade to (nil,nil)")
	}
	ReportLLMUsage(ctx, 1, 1.0) // must not panic

	// Money-mutation side: must surface an error (never silently succeed).
	if _, err := PreAuthorize(ctx, 1, 1, "p", "r", "d", 1); err == nil {
		t.Error("PreAuthorize transport error must surface an error")
	}
	if _, err := SettlePreAuth(ctx, 1, 1); err == nil {
		t.Error("SettlePreAuth transport error must surface an error")
	}
	if err := ReleasePreAuth(ctx, 1); err == nil {
		t.Error("ReleasePreAuth transport error must surface an error")
	}
	if _, err := DebitWallet(ctx, 1, 1, "t", "d", "p", "i"); err == nil {
		t.Error("DebitWallet transport error must surface an error")
	}
	if err := CreditWallet(ctx, 1, 1, "t", "d", "p", "i"); err == nil {
		t.Error("CreditWallet transport error must surface an error")
	}
	if _, err := CreateCheckout(ctx, 1, 1, "alipay", "svc", "i", "r"); err == nil {
		t.Error("CreateCheckout transport error must surface an error")
	}
	if _, err := GetCheckoutStatus(ctx, "o"); err == nil {
		t.Error("GetCheckoutStatus transport error must surface an error")
	}
}

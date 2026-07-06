package common

import (
	"testing"
	"time"

	identityv1 "github.com/LurusTech/lurus-proto-go/identity/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestProtoToIdentityMapping(t *testing.T) {
	created := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	a := &identityv1.Account{
		Id:          77,
		LurusId:     "lurus-77",
		ZitadelSub:  "sub-77",
		Email:       "u@x.com",
		DisplayName: "User 77",
		AvatarUrl:   "http://avatar",
		Status:      2,
		CreatedAt:   timestamppb.New(created),
	}
	m := protoToIdentityMapping(a)
	if m.ID != 77 || m.LurusID != "lurus-77" || m.ZitadelSub != "sub-77" ||
		m.Email != "u@x.com" || m.DisplayName != "User 77" || m.Status != 2 {
		t.Errorf("mapping mismatch: %+v", m)
	}
	if !m.CreatedAt.Equal(created) {
		t.Errorf("created_at mismatch: %v", m.CreatedAt)
	}

	// Nil CreatedAt leaves the zero time.
	m2 := protoToIdentityMapping(&identityv1.Account{Id: 1})
	if !m2.CreatedAt.IsZero() {
		t.Error("nil CreatedAt should stay zero")
	}
}

func TestProtoToAccountOverview(t *testing.T) {
	exp := time.Date(2027, 6, 1, 0, 0, 0, 0, time.UTC)
	ov := &identityv1.AccountOverview{
		Account: &identityv1.AccountSummary{Id: 9, LurusId: "l-9", DisplayName: "Nine", AvatarUrl: "a"},
		Vip: &identityv1.VIPSummary{
			Level: 3, LevelName: "Gold", LevelEn: "gold", Points: 1200,
			LevelExpiresAt: timestamppb.New(exp),
		},
		Wallet: &identityv1.WalletSummary{Balance: 500.5, Frozen: 10.0},
		Subscription: &identityv1.SubscriptionSummary{
			ProductId: "p1", PlanCode: "pro", Status: "active", AutoRenew: true,
			ExpiresAt: timestamppb.New(exp),
		},
		TopupUrl: "http://topup",
	}
	r := protoToAccountOverview(ov)
	if r.Account.ID != 9 || r.Account.DisplayName != "Nine" {
		t.Errorf("account summary mismatch: %+v", r.Account)
	}
	if r.VIP.Level != 3 || r.VIP.LevelName != "Gold" || r.VIP.Points != 1200 {
		t.Errorf("vip mismatch: %+v", r.VIP)
	}
	if r.VIP.LevelExpiresAt == nil {
		t.Error("vip level_expires_at should be set")
	}
	if r.Wallet.Balance != 500.5 || r.Wallet.Frozen != 10.0 {
		t.Errorf("wallet mismatch: %+v", r.Wallet)
	}
	if r.Subscription == nil || r.Subscription.PlanCode != "pro" || !r.Subscription.AutoRenew {
		t.Errorf("subscription mismatch: %+v", r.Subscription)
	}
	if r.Subscription.ExpiresAt == nil || *r.Subscription.ExpiresAt == "" {
		t.Error("subscription expires_at should be set")
	}
	if r.TopupURL != "http://topup" {
		t.Errorf("topup url mismatch: %q", r.TopupURL)
	}
}

func TestProtoToAccountOverview_DefaultsAndNils(t *testing.T) {
	// Empty TopupUrl falls back to the public URL; nil nested summaries are skipped.
	r := protoToAccountOverview(&identityv1.AccountOverview{})
	if r.TopupURL == "" {
		t.Error("empty topup_url should fall back to a default")
	}
	if r.Subscription != nil {
		t.Error("nil subscription should stay nil")
	}
	if r.Account.ID != 0 {
		t.Error("nil account summary should leave zero values")
	}
}

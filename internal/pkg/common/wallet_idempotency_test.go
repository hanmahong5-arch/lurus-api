package common

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestWalletCalls_SendIdempotencyKey asserts every money-mutation HTTP twin
// attaches an Idempotency-Key header. The platform money endpoints reject calls
// without it (400 idempotency_key_required) — the omission was the SEAM S1
// wallet-debit-leg blocker (topup → 402 WALLET_DEBIT_FAILED). Keys are checked
// to match the per-operation derivation so retries dedupe instead of double-charge.
func TestWalletCalls_SendIdempotencyKey(t *testing.T) {
	var gotKey string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("Idempotency-Key")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true, "balance_after": 1.0,
			"preauth_id": 7, "amount": 1.0, "status": "frozen",
			"held_amount": 1.0, "actual_amount": 1.0,
		})
	}))
	defer srv.Close()

	prev := IdentityServiceURL
	IdentityServiceURL = srv.URL
	defer func() { IdentityServiceURL = prev }()

	ctx := context.Background()
	cases := []struct {
		name string
		want string
		call func() error
	}{
		{"DebitWallet", "debit-key-1", func() error {
			_, err := DebitWallet(ctx, 42, 1.0, "pool_topup", "d", "newhub", "debit-key-1")
			return err
		}},
		{"CreditWallet", "credit-key-1", func() error {
			return CreditWallet(ctx, 42, 1.0, "refund", "c", "newhub", "credit-key-1")
		}},
		{"PreAuthorize_usesReferenceID", "ref-9", func() error {
			_, err := PreAuthorize(ctx, 42, 1.0, "newhub", "ref-9", "p", 60)
			return err
		}},
		{"SettlePreAuth_keyedOnPreAuthID", "settle:99", func() error {
			_, err := SettlePreAuth(ctx, 99, 1.0)
			return err
		}},
		{"ReleasePreAuth_keyedOnPreAuthID", "release:99", func() error {
			return ReleasePreAuth(ctx, 99)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotKey = ""
			if err := tc.call(); err != nil {
				t.Fatalf("%s returned error: %v", tc.name, err)
			}
			if gotKey != tc.want {
				t.Errorf("%s: Idempotency-Key = %q, want %q", tc.name, gotKey, tc.want)
			}
		})
	}
}

// TestDebitWallet_EmptyIdempotencyKey_OmitsHeader guards against sending a blank
// Idempotency-Key (platform treats an empty header value as malformed).
func TestDebitWallet_EmptyIdempotencyKey_OmitsHeader(t *testing.T) {
	var present bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, present = r.Header["Idempotency-Key"]
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "balance_after": 1.0})
	}))
	defer srv.Close()

	prev := IdentityServiceURL
	IdentityServiceURL = srv.URL
	defer func() { IdentityServiceURL = prev }()

	if _, err := DebitWallet(context.Background(), 42, 1.0, "t", "d", "newhub", ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if present {
		t.Error("empty idempotencyKey must not send an Idempotency-Key header")
	}
}

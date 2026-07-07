package common

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestPreAuthorize_HTTP(t *testing.T) {
	newIdentityServer(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/wallet/pre-authorize") {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		if r.Header.Get("Idempotency-Key") != "ref-1" {
			t.Errorf("missing idempotency key: %q", r.Header.Get("Idempotency-Key"))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"preauth_id": 55, "amount": 2.5, "status": "held"})
	})
	res, err := PreAuthorize(context.Background(), 1, 2.5, "prod", "ref-1", "desc", 60)
	if err != nil || res == nil || res.PreAuthID != 55 || res.Status != "held" {
		t.Fatalf("unexpected: %+v err=%v", res, err)
	}
}

func TestPreAuthorize_InsufficientBalance(t *testing.T) {
	newIdentityServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": "insufficient_balance"})
	})
	_, err := PreAuthorize(context.Background(), 1, 2.5, "prod", "ref-1", "desc", 60)
	if err == nil || err.Error() != "insufficient_balance" {
		t.Errorf("expected insufficient_balance, got %v", err)
	}
}

func TestPreAuthorize_NotConfigured(t *testing.T) {
	prev := IdentityServiceURL
	IdentityServiceURL = ""
	t.Cleanup(func() { IdentityServiceURL = prev })
	if _, err := PreAuthorize(context.Background(), 1, 1, "p", "r", "d", 1); err == nil {
		t.Error("unconfigured pre-authorize should error")
	}
}

func TestSettlePreAuth_HTTP(t *testing.T) {
	newIdentityServer(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/pre-auth/55/settle") {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		if r.Header.Get("Idempotency-Key") != "settle:55" {
			t.Errorf("settle idempotency key mismatch: %q", r.Header.Get("Idempotency-Key"))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"preauth_id": 55, "status": "settled", "actual_amount": 1.9})
	})
	res, err := SettlePreAuth(context.Background(), 55, 1.9)
	if err != nil || res == nil || res.Status != "settled" || res.ActualAmount != 1.9 {
		t.Fatalf("unexpected: %+v err=%v", res, err)
	}
}

func TestSettlePreAuth_ServerError(t *testing.T) {
	newIdentityServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": "already_settled"})
	})
	if _, err := SettlePreAuth(context.Background(), 55, 1.0); err == nil ||
		!strings.Contains(err.Error(), "already_settled") {
		t.Errorf("expected already_settled error, got %v", err)
	}
}

func TestReleasePreAuth_HTTP(t *testing.T) {
	newIdentityServer(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/pre-auth/55/release") {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	})
	if err := ReleasePreAuth(context.Background(), 55); err != nil {
		t.Errorf("release should succeed: %v", err)
	}
}

func TestReleasePreAuth_Failure(t *testing.T) {
	newIdentityServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": "not_found"})
	})
	if err := ReleasePreAuth(context.Background(), 55); err == nil {
		t.Error("release of missing pre-auth should error")
	}
}

func TestDebitWallet_HTTP(t *testing.T) {
	newIdentityServer(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/accounts/3/wallet/debit") {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		if r.Header.Get("Idempotency-Key") != "idem-9" {
			t.Errorf("debit idempotency key mismatch: %q", r.Header.Get("Idempotency-Key"))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "balance_after": 88.0})
	})
	res, err := DebitWallet(context.Background(), 3, 12.0, "spend", "relay", "prod", "idem-9")
	if err != nil || res == nil || !res.Success || res.BalanceAfter != 88.0 {
		t.Fatalf("unexpected: %+v err=%v", res, err)
	}
}

func TestDebitWallet_InsufficientBalance(t *testing.T) {
	newIdentityServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": "insufficient_balance"})
	})
	if _, err := DebitWallet(context.Background(), 3, 999.0, "spend", "relay", "prod", "idem"); err == nil ||
		err.Error() != "insufficient_balance" {
		t.Errorf("expected insufficient_balance, got %v", err)
	}
}

func TestCreditWallet_HTTP(t *testing.T) {
	newIdentityServer(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/accounts/3/wallet/credit") {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	})
	if err := CreditWallet(context.Background(), 3, 5.0, "refund", "correction", "prod", "idem-c"); err != nil {
		t.Errorf("credit should succeed: %v", err)
	}
}

func TestCreditWallet_Failure(t *testing.T) {
	newIdentityServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	if err := CreditWallet(context.Background(), 3, 5.0, "refund", "x", "prod", "idem"); err == nil {
		t.Error("5xx credit should error")
	}
}

func TestWalletCalls_NotConfigured(t *testing.T) {
	prev := IdentityServiceURL
	IdentityServiceURL = ""
	t.Cleanup(func() { IdentityServiceURL = prev })
	ctx := context.Background()
	if _, err := SettlePreAuth(ctx, 1, 1); err == nil {
		t.Error("settle unconfigured should error")
	}
	if err := ReleasePreAuth(ctx, 1); err == nil {
		t.Error("release unconfigured should error")
	}
	if _, err := DebitWallet(ctx, 1, 1, "t", "d", "p", "i"); err == nil {
		t.Error("debit unconfigured should error")
	}
	if err := CreditWallet(ctx, 1, 1, "t", "d", "p", "i"); err == nil {
		t.Error("credit unconfigured should error")
	}
}

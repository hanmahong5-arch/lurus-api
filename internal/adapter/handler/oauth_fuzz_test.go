package handler

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

// FuzzParseOAuthState tests OAuth state parsing with random inputs
// Run: go test -fuzz=FuzzParseOAuthState -fuzztime=30s ./internal/adapter/handler/...
func FuzzParseOAuthState(f *testing.F) {
	// Seed corpus with valid states
	validState := OAuthStateData{
		TenantSlug:  "my-tenant",
		RedirectURL: "/dashboard",
		Nonce:       "nonce123",
		CreatedAt:   time.Now(),
	}
	validJSON, _ := json.Marshal(validState)
	validEncoded := base64.URLEncoding.EncodeToString(validJSON)

	f.Add(validEncoded)
	f.Add("")
	f.Add("!!!invalid-base64!!!")
	f.Add(base64.URLEncoding.EncodeToString([]byte("not json")))
	f.Add(base64.URLEncoding.EncodeToString([]byte("{}")))
	f.Add(base64.URLEncoding.EncodeToString([]byte(`{"tenant_slug":"a"}`)))
	f.Add(base64.URLEncoding.EncodeToString([]byte(`{"tenant_slug":"<script>alert(1)</script>"}`)))
	f.Add(strings.Repeat("A", 10000)) // large input

	// Add some edge case JSON
	edgeCases := []string{
		`{"tenant_slug":"","redirect_url":"","nonce":"","created_at":"0001-01-01T00:00:00Z"}`,
		`{"tenant_slug":null}`,
		`{"extra_field":"value","tenant_slug":"test"}`,
		`{"tenant_slug":"test","redirect_url":"javascript:alert(1)"}`,
		`{"tenant_slug":"test/../../../etc/passwd"}`,
	}
	for _, ec := range edgeCases {
		f.Add(base64.URLEncoding.EncodeToString([]byte(ec)))
	}

	f.Fuzz(func(t *testing.T, state string) {
		parsed, err := parseOAuthState(state)

		// Invariant 1: should never panic (implicit)

		// Invariant 2: if parsing succeeds, result should be non-nil
		if err == nil && parsed == nil {
			t.Error("parseOAuthState returned nil result without error")
		}

		// Invariant 3: if parsing succeeds, decoded data should be consistent
		if err == nil && parsed != nil {
			// Re-encode and decode to verify consistency
			reencoded, err2 := json.Marshal(parsed)
			if err2 != nil {
				t.Errorf("failed to re-marshal parsed state: %v", err2)
			}
			var reparsed OAuthStateData
			if err3 := json.Unmarshal(reencoded, &reparsed); err3 != nil {
				t.Errorf("failed to re-unmarshal state: %v", err3)
			}
		}

		// Invariant 4: empty input should fail
		if state == "" && err == nil {
			t.Error("empty state should fail parsing")
		}
	})
}

// FuzzGenerateOAuthState tests OAuth state generation with random inputs
// Run: go test -fuzz=FuzzGenerateOAuthState -fuzztime=30s ./internal/adapter/handler/...
func FuzzGenerateOAuthState(f *testing.F) {
	f.Add("my-tenant", "/dashboard")
	f.Add("", "")
	f.Add("tenant", "/")
	f.Add("tenant-with-dashes", "/path/with/slashes")
	f.Add(strings.Repeat("a", 1000), strings.Repeat("b", 1000))
	f.Add("tenant\x00null", "/path\x00null")
	f.Add("tenant<script>", "/path?q=<script>")
	f.Add("租户", "/中文路径")

	f.Fuzz(func(t *testing.T, tenant, redirect string) {
		// Skip invalid UTF-8 inputs as JSON encoding will transform them
		if !utf8.ValidString(tenant) || !utf8.ValidString(redirect) {
			return
		}

		state, nonce, err := generateOAuthState(tenant, redirect)

		// Invariant 1: should never panic (implicit)

		// Invariant 2: if generation succeeds, state and nonce should be non-empty
		if err == nil {
			if state == "" {
				t.Error("generated state is empty")
			}
			if nonce == "" {
				t.Error("generated nonce is empty")
			}
		}

		// Invariant 3: generated state should be parseable
		if err == nil && state != "" {
			parsed, parseErr := parseOAuthState(state)
			if parseErr != nil {
				t.Errorf("generated state not parseable: %v", parseErr)
			}
			if parsed != nil {
				// Verify round-trip
				if parsed.TenantSlug != tenant {
					t.Errorf("tenant mismatch: got %q, want %q", parsed.TenantSlug, tenant)
				}
				if parsed.RedirectURL != redirect {
					t.Errorf("redirect mismatch: got %q, want %q", parsed.RedirectURL, redirect)
				}
				if parsed.Nonce != nonce {
					t.Errorf("nonce mismatch: got %q, want %q", parsed.Nonce, nonce)
				}
			}
		}

		// Invariant 4: state must have payload.signature shape — base64 payload
		// + hex HMAC signature joined by '.' (post-commit 942b2518 format).
		if err == nil && state != "" {
			dotIdx := strings.LastIndex(state, ".")
			if dotIdx < 0 {
				t.Errorf("state missing '.' separator: %q", state)
			} else {
				payload := state[:dotIdx]
				sig := state[dotIdx+1:]
				if _, decodeErr := base64.URLEncoding.DecodeString(payload); decodeErr != nil {
					t.Errorf("state payload is not valid base64: %v", decodeErr)
				}
				if _, decodeErr := hex.DecodeString(sig); decodeErr != nil {
					t.Errorf("state signature is not valid hex: %v", decodeErr)
				}
				// HMAC-SHA256 hex output is exactly 64 chars.
				if len(sig) != 64 {
					t.Errorf("state signature length = %d, want 64 (hex SHA-256)", len(sig))
				}
			}
		}

		// Invariant 5: multiple calls should produce unique nonces
		if err == nil {
			state2, nonce2, _ := generateOAuthState(tenant, redirect)
			if nonce == nonce2 {
				t.Error("nonces should be unique across calls")
			}
			if state == state2 {
				t.Error("states should be unique due to unique nonces")
			}
		}
	})
}

// FuzzOAuthStateRoundTrip verifies generate → parse round-trip integrity.
//
// Since commit 942b2518 the state format is HMAC-signed (payload.signature),
// so we cannot hand-craft a raw base64(JSON) blob and feed it to parseOAuthState
// — that would bypass HMAC verification and never round-trip. Instead, we drive
// the round-trip through the real generate entry point and compare what comes
// out. The `nonce` fuzz arg is preserved for seed-corpus compatibility but is
// asserted indirectly: parsed.Nonce is whatever generateOAuthState minted, not
// the fuzz input, since generateOAuthState owns nonce minting.
func FuzzOAuthStateRoundTrip(f *testing.F) {
	f.Add("tenant", "/path", "nonce123")
	f.Add("", "", "")
	f.Add("a", "/", "n")
	f.Add("multi-tenant-org", "/dashboard/settings?tab=security", "secure-nonce-abc123")

	f.Fuzz(func(t *testing.T, tenant, redirect, _ string) {
		// Skip invalid UTF-8 inputs as JSON encoding will transform them
		if !utf8.ValidString(tenant) || !utf8.ValidString(redirect) {
			return
		}

		// Generate via the real production entry point (HMAC-signed).
		state, generatedNonce, err := generateOAuthState(tenant, redirect)
		if err != nil {
			t.Fatalf("generateOAuthState failed: %v", err)
		}
		if state == "" {
			t.Fatal("generated state is empty")
		}
		if generatedNonce == "" {
			t.Fatal("generated nonce is empty")
		}

		// Parse and verify the round-trip.
		parsed, err := parseOAuthState(state)
		if err != nil {
			t.Fatalf("failed to parse generated state: %v", err)
		}
		if parsed == nil {
			t.Fatal("parseOAuthState returned nil without error")
		}
		if parsed.TenantSlug != tenant {
			t.Errorf("tenant mismatch: got %q, want %q", parsed.TenantSlug, tenant)
		}
		if parsed.RedirectURL != redirect {
			t.Errorf("redirect mismatch: got %q, want %q", parsed.RedirectURL, redirect)
		}
		if parsed.Nonce != generatedNonce {
			t.Errorf("nonce mismatch: got %q, want %q", parsed.Nonce, generatedNonce)
		}
		// CreatedAt must be recent (within last 10s — same window parseOAuthState
		// callers enforce for expiration, just sanity check freshness here).
		if time.Since(parsed.CreatedAt) > 10*time.Second {
			t.Errorf("parsed CreatedAt too old: %v", parsed.CreatedAt)
		}
	})
}

package model

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestInternalApiKeyTableName tests the table name
func TestInternalApiKeyTableName(t *testing.T) {
	key := InternalApiKey{}
	if key.TableName() != "internal_api_keys" {
		t.Errorf("TableName() = %q, want %q", key.TableName(), "internal_api_keys")
	}
}

// TestGetScopes tests scope parsing
func TestGetScopes(t *testing.T) {
	tests := []struct {
		name          string
		scopesJson    string
		expectedLen   int
		expectedFirst string
	}{
		{
			name:          "single_scope",
			scopesJson:    `["user:read"]`,
			expectedLen:   1,
			expectedFirst: "user:read",
		},
		{
			name:          "multiple_scopes",
			scopesJson:    `["user:read","user:write","quota:read"]`,
			expectedLen:   3,
			expectedFirst: "user:read",
		},
		{
			name:          "wildcard_scope",
			scopesJson:    `["*"]`,
			expectedLen:   1,
			expectedFirst: "*",
		},
		{
			name:          "empty_array",
			scopesJson:    `[]`,
			expectedLen:   0,
			expectedFirst: "",
		},
		{
			name:          "empty_string",
			scopesJson:    "",
			expectedLen:   0,
			expectedFirst: "",
		},
		{
			name:          "all_standard_scopes",
			scopesJson:    `["user:read","user:write","subscription:read","subscription:write","quota:read","quota:write","balance:read","balance:write"]`,
			expectedLen:   8,
			expectedFirst: "user:read",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := &InternalApiKey{Scopes: tt.scopesJson}
			scopes := key.GetScopes()

			if len(scopes) != tt.expectedLen {
				t.Errorf("GetScopes() len = %d, want %d", len(scopes), tt.expectedLen)
			}

			if tt.expectedLen > 0 && scopes[0] != tt.expectedFirst {
				t.Errorf("GetScopes()[0] = %q, want %q", scopes[0], tt.expectedFirst)
			}
		})
	}
}

// TestHasScope tests scope checking
func TestHasScope(t *testing.T) {
	tests := []struct {
		name       string
		scopes     []string
		checkScope string
		expected   bool
	}{
		// Exact match tests
		{"exact_match_single", []string{"user:read"}, "user:read", true},
		{"exact_match_in_list", []string{"user:read", "user:write"}, "user:write", true},
		{"exact_match_first", []string{"user:read", "quota:write"}, "user:read", true},
		{"exact_match_last", []string{"user:read", "quota:write"}, "quota:write", true},

		// Wildcard tests
		{"wildcard_matches_any", []string{"*"}, "user:read", true},
		{"wildcard_matches_write", []string{"*"}, "subscription:write", true},
		{"wildcard_matches_balance", []string{"*"}, "balance:write", true},
		{"wildcard_with_others", []string{"user:read", "*"}, "quota:write", true},

		// No match tests
		{"no_match_different", []string{"user:read"}, "user:write", false},
		{"no_match_partial", []string{"user:read"}, "user", false},
		{"no_match_extended", []string{"user:read"}, "user:read:extra", false},
		{"no_match_empty_scopes", []string{}, "user:read", false},

		// Edge cases
		{"case_sensitive_mismatch", []string{"USER:READ"}, "user:read", false},
		{"case_sensitive_match", []string{"user:read"}, "USER:READ", false},
		{"empty_scope_check", []string{"user:read"}, "", false},
		{"whitespace_scope", []string{" user:read "}, "user:read", false},

		// All standard scopes
		{"has_subscription_read", []string{ScopeSubscriptionRead}, ScopeSubscriptionRead, true},
		{"has_subscription_write", []string{ScopeSubscriptionWrite}, ScopeSubscriptionWrite, true},
		{"has_quota_read", []string{ScopeQuotaRead}, ScopeQuotaRead, true},
		{"has_quota_write", []string{ScopeQuotaWrite}, ScopeQuotaWrite, true},
		{"has_balance_read", []string{ScopeBalanceRead}, ScopeBalanceRead, true},
		{"has_balance_write", []string{ScopeBalanceWrite}, ScopeBalanceWrite, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scopesJson, _ := json.Marshal(tt.scopes)
			key := &InternalApiKey{Scopes: string(scopesJson)}

			result := key.HasScope(tt.checkScope)
			if result != tt.expected {
				t.Errorf("HasScope(%q) = %v, want %v (scopes: %v)", tt.checkScope, result, tt.expected, tt.scopes)
			}
		})
	}
}

// TestHashKey tests the key hashing function
func TestHashKey(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		check func(hash string) bool
	}{
		{
			name: "returns_64_char_hex",
			key:  "lurus_ik_test123456789012345678901",
			check: func(hash string) bool {
				return len(hash) == 64
			},
		},
		{
			name: "hex_chars_only",
			key:  "lurus_ik_test123456789012345678901",
			check: func(hash string) bool {
				for _, c := range hash {
					if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
						return false
					}
				}
				return true
			},
		},
		{
			name: "deterministic",
			key:  "same_key_always_same_hash",
			check: func(hash string) bool {
				hash2 := hashKey("same_key_always_same_hash")
				return hash == hash2
			},
		},
		{
			name: "different_keys_different_hashes",
			key:  "key1",
			check: func(hash string) bool {
				hash2 := hashKey("key2")
				return hash != hash2
			},
		},
		{
			name: "empty_key_hashes",
			key:  "",
			check: func(hash string) bool {
				return len(hash) == 64
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash := hashKey(tt.key)
			if !tt.check(hash) {
				t.Errorf("hashKey(%q) check failed, got %q", tt.key, hash)
			}
		})
	}
}

// TestGetAvailableScopes tests the scope list function
func TestGetAvailableScopes(t *testing.T) {
	scopes := GetAvailableScopes()

	// Should return 13 scopes
	if len(scopes) != 13 {
		t.Errorf("GetAvailableScopes() returned %d scopes, want 13", len(scopes))
	}

	// Check required fields
	for i, scope := range scopes {
		if scope["key"] == "" {
			t.Errorf("Scope %d missing 'key'", i)
		}
		if scope["name"] == "" {
			t.Errorf("Scope %d missing 'name'", i)
		}
		if scope["description"] == "" {
			t.Errorf("Scope %d missing 'description'", i)
		}
	}

	// Check all standard scopes are included
	expectedKeys := []string{
		ScopeUserRead, ScopeUserWrite, ScopeUserDelete,
		ScopeSubscriptionRead, ScopeSubscriptionWrite,
		ScopeQuotaRead, ScopeQuotaWrite,
		ScopeBalanceRead, ScopeBalanceWrite,
		ScopeTokenRead, ScopeTokenWrite,
		ScopeAuthLogin,
		ScopeAll,
	}

	for _, expectedKey := range expectedKeys {
		found := false
		for _, scope := range scopes {
			if scope["key"] == expectedKey {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected scope %q not found in GetAvailableScopes()", expectedKey)
		}
	}
}

// TestScopeConstants tests that scope constants are correct
func TestScopeConstants(t *testing.T) {
	tests := []struct {
		name     string
		constant string
		expected string
	}{
		{"ScopeUserRead", ScopeUserRead, "user:read"},
		{"ScopeUserWrite", ScopeUserWrite, "user:write"},
		{"ScopeUserDelete", ScopeUserDelete, "user:delete"},
		{"ScopeSubscriptionRead", ScopeSubscriptionRead, "subscription:read"},
		{"ScopeSubscriptionWrite", ScopeSubscriptionWrite, "subscription:write"},
		{"ScopeQuotaRead", ScopeQuotaRead, "quota:read"},
		{"ScopeQuotaWrite", ScopeQuotaWrite, "quota:write"},
		{"ScopeBalanceRead", ScopeBalanceRead, "balance:read"},
		{"ScopeBalanceWrite", ScopeBalanceWrite, "balance:write"},
		{"ScopeTokenRead", ScopeTokenRead, "token:read"},
		{"ScopeTokenWrite", ScopeTokenWrite, "token:write"},
		{"ScopeAuthLogin", ScopeAuthLogin, "auth:login"},
		{"ScopeAll", ScopeAll, "*"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.constant != tt.expected {
				t.Errorf("%s = %q, want %q", tt.name, tt.constant, tt.expected)
			}
		})
	}
}

// TestKeyFormat tests the expected key format
func TestKeyFormat(t *testing.T) {
	tests := []struct {
		name      string
		key       string
		isValid   bool
		hasPrefix bool
	}{
		{
			name:      "valid_format",
			key:       "lurus_ik_abcdefghijklmnopqrstuvwxyz123456",
			isValid:   true,
			hasPrefix: true,
		},
		{
			name:      "wrong_prefix",
			key:       "sk_abcdefghijklmnopqrstuvwxyz123456",
			isValid:   false,
			hasPrefix: false,
		},
		{
			name:      "no_prefix",
			key:       "abcdefghijklmnopqrstuvwxyz123456",
			isValid:   false,
			hasPrefix: false,
		},
		{
			name:      "empty_key",
			key:       "",
			isValid:   false,
			hasPrefix: false,
		},
		{
			name:      "too_short",
			key:       "lurus_ik_abc",
			isValid:   false,
			hasPrefix: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hasPrefix := strings.HasPrefix(tt.key, "lurus_ik_")
			if hasPrefix != tt.hasPrefix {
				t.Errorf("key %q prefix check = %v, want %v", tt.key, hasPrefix, tt.hasPrefix)
			}

			// Valid key should be 41 characters: lurus_ik_ (9) + 32 random
			isValid := hasPrefix && len(tt.key) == 41
			if isValid != tt.isValid {
				t.Errorf("key %q validity = %v, want %v", tt.key, isValid, tt.isValid)
			}
		})
	}
}

// TestInternalApiKeyStruct tests the struct fields
func TestInternalApiKeyStruct(t *testing.T) {
	key := &InternalApiKey{
		Id:          1,
		Name:        "Test Key",
		KeyHash:     "abc123",
		KeyPrefix:   "lurus_ik_abc123",
		Scopes:      `["user:read"]`,
		CreatedBy:   100,
		CreatedAt:   1704067200,
		LastUsedAt:  1704153600,
		ExpiresAt:   0,
		Enabled:     true,
		Description: "Test description",
	}

	if key.Id != 1 {
		t.Errorf("Id = %d, want 1", key.Id)
	}
	if key.Name != "Test Key" {
		t.Errorf("Name = %q, want %q", key.Name, "Test Key")
	}
	if key.KeyHash != "abc123" {
		t.Errorf("KeyHash = %q, want %q", key.KeyHash, "abc123")
	}
	if key.KeyPrefix != "lurus_ik_abc123" {
		t.Errorf("KeyPrefix = %q, want %q", key.KeyPrefix, "lurus_ik_abc123")
	}
	if key.CreatedBy != 100 {
		t.Errorf("CreatedBy = %d, want 100", key.CreatedBy)
	}
	if !key.Enabled {
		t.Error("Enabled should be true")
	}
	if key.ExpiresAt != 0 {
		t.Errorf("ExpiresAt = %d, want 0 (never expires)", key.ExpiresAt)
	}
}

// TestScopesJsonParsing tests JSON parsing edge cases
func TestScopesJsonParsing(t *testing.T) {
	tests := []struct {
		name       string
		scopesJson string
		expectLen  int
	}{
		{"valid_json", `["user:read"]`, 1},
		{"empty_array", `[]`, 0},
		{"empty_string", "", 0},
		{"invalid_json", `{invalid}`, 0},
		{"null_json", `null`, 0},
		{"whitespace", `  `, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := &InternalApiKey{Scopes: tt.scopesJson}
			scopes := key.GetScopes()
			if len(scopes) != tt.expectLen {
				t.Errorf("GetScopes() len = %d, want %d", len(scopes), tt.expectLen)
			}
		})
	}
}

// BenchmarkHasScope benchmarks scope checking
func BenchmarkHasScope(b *testing.B) {
	scopesJson, _ := json.Marshal([]string{
		"user:read", "user:write", "quota:read", "quota:write",
		"subscription:read", "subscription:write",
	})
	key := &InternalApiKey{Scopes: string(scopesJson)}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key.HasScope("quota:write")
	}
}

// BenchmarkHashKey benchmarks key hashing
func BenchmarkHashKey(b *testing.B) {
	key := "lurus_ik_abcdefghijklmnopqrstuvwxyz123456"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hashKey(key)
	}
}

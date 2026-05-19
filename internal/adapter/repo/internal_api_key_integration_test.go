package repo

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"
	"time"

	"github.com/LurusTech/lurus-hub/internal/pkg/common"
)

func TestApiKey_Create_SHA256Hash(t *testing.T) {
	cleanup := SetupTestDB(t)
	defer cleanup()

	rawKey, apiKey, err := CreateInternalApiKey("test-key", []string{ScopeAll}, 1, 0, "test")
	if err != nil {
		t.Fatalf("CreateInternalApiKey() failed: %v", err)
	}

	// Verify hash matches SHA256 of raw key
	h := sha256.Sum256([]byte(rawKey))
	expectedHash := hex.EncodeToString(h[:])
	if apiKey.KeyHash != expectedHash {
		t.Errorf("KeyHash = %q, want SHA256(%q) = %q", apiKey.KeyHash, rawKey, expectedHash)
	}
}

func TestApiKey_Validate_ValidKey(t *testing.T) {
	cleanup := SetupTestDB(t)
	defer cleanup()

	rawKey, _, err := CreateInternalApiKey("valid-key", []string{ScopeAll}, 1, 0, "test")
	if err != nil {
		t.Fatalf("CreateInternalApiKey() failed: %v", err)
	}

	validated, err := ValidateInternalApiKey(rawKey)
	if err != nil {
		t.Fatalf("ValidateInternalApiKey() failed: %v", err)
	}
	if validated.Name != "valid-key" {
		t.Errorf("Name = %q, want %q", validated.Name, "valid-key")
	}
}

func TestApiKey_Validate_InvalidKey(t *testing.T) {
	cleanup := SetupTestDB(t)
	defer cleanup()

	_, err := ValidateInternalApiKey("lurus_ik_nonexistent_random_string_here")
	if err == nil {
		t.Error("expected error for invalid key, got nil")
	}
}

func TestApiKey_Validate_DisabledKey(t *testing.T) {
	cleanup := SetupTestDB(t)
	defer cleanup()

	rawKey, apiKey, err := CreateInternalApiKey("disabled-key", []string{ScopeAll}, 1, 0, "test")
	if err != nil {
		t.Fatalf("CreateInternalApiKey() failed: %v", err)
	}

	// Disable the key
	DB.Model(&InternalApiKey{}).Where("id = ?", apiKey.Id).Update("enabled", false)

	_, err = ValidateInternalApiKey(rawKey)
	if err == nil {
		t.Error("expected error for disabled key, got nil")
	}
}

func TestApiKey_Validate_ExpiredKey(t *testing.T) {
	cleanup := SetupTestDB(t)
	defer cleanup()

	// Create key that expired 1 hour ago
	pastExpiry := time.Now().Add(-1 * time.Hour).Unix()
	rawKey, _, err := CreateInternalApiKey("expired-key", []string{ScopeAll}, 1, pastExpiry, "test")
	if err != nil {
		t.Fatalf("CreateInternalApiKey() failed: %v", err)
	}

	_, err = ValidateInternalApiKey(rawKey)
	if err == nil {
		t.Error("expected error for expired key, got nil")
	}
}

func TestApiKey_Validate_UpdatesLastUsed(t *testing.T) {
	cleanup := SetupTestDB(t)
	defer cleanup()

	rawKey, apiKey, err := CreateInternalApiKey("lastused-key", []string{ScopeAll}, 1, 0, "test")
	if err != nil {
		t.Fatalf("CreateInternalApiKey() failed: %v", err)
	}

	// Validate to trigger last_used_at update (happens in goroutine)
	_, err = ValidateInternalApiKey(rawKey)
	if err != nil {
		t.Fatalf("ValidateInternalApiKey() failed: %v", err)
	}

	// Sleep briefly to let the async goroutine complete
	time.Sleep(200 * time.Millisecond)

	var updated InternalApiKey
	DB.First(&updated, "id = ?", apiKey.Id)
	if updated.LastUsedAt == 0 {
		t.Error("LastUsedAt should be > 0 after validation")
	}
}

func TestApiKey_HasScope_Wildcard(t *testing.T) {
	scopesJSON, _ := json.Marshal([]string{ScopeAll})
	key := &InternalApiKey{Scopes: string(scopesJSON)}

	testScopes := []string{ScopeUserRead, ScopeUserWrite, ScopeQuotaRead, "anything:custom"}
	for _, s := range testScopes {
		if !key.HasScope(s) {
			t.Errorf("HasScope(%q) = false for wildcard key, want true", s)
		}
	}
}

func TestApiKey_HasScope_Specific(t *testing.T) {
	scopesJSON, _ := json.Marshal([]string{ScopeUserRead})
	key := &InternalApiKey{Scopes: string(scopesJSON)}

	if !key.HasScope(ScopeUserRead) {
		t.Error("HasScope(user:read) = false, want true")
	}
}

func TestApiKey_HasScope_Missing(t *testing.T) {
	scopesJSON, _ := json.Marshal([]string{ScopeUserRead})
	key := &InternalApiKey{Scopes: string(scopesJSON)}

	if key.HasScope(ScopeUserWrite) {
		t.Error("HasScope(user:write) = true for user:read-only key, want false")
	}
}

// TestInternalKeyAllowedForTenant exercises the tenant-whitelist guard
// added by Phase 2 self-audit (2026-05-19). Behaviour:
//
//   - nil / zero-id / empty-tenant inputs → deny
//   - platform admin (ScopeAll = "*") → allow without a whitelist row
//   - narrow-scope key, empty whitelist → deny (fail-closed)
//   - narrow-scope key, matching row → allow
//   - narrow-scope key, wrong tenant or wrong key id → deny
//
// Table is created in-test rather than via AutoMigrate so the migration 013
// SQL stays the single canonical schema source.
func TestInternalKeyAllowedForTenant(t *testing.T) {
	cleanup := SetupTestDB(t)
	defer cleanup()

	if err := DB.Exec(`CREATE TABLE IF NOT EXISTS internal_api_key_tenants (
		api_key_id INTEGER NOT NULL,
		tenant_id  VARCHAR(36) NOT NULL,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		PRIMARY KEY (api_key_id, tenant_id)
	)`).Error; err != nil {
		t.Fatalf("create internal_api_key_tenants: %v", err)
	}

	provScopes, _ := json.Marshal([]string{ScopeProvisioning})
	allScopes, _ := json.Marshal([]string{ScopeAll})

	narrowKey := &InternalApiKey{Id: 1, Scopes: string(provScopes)}
	otherNarrowKey := &InternalApiKey{Id: 2, Scopes: string(provScopes)}
	adminKey := &InternalApiKey{Id: 99, Scopes: string(allScopes)}

	// Guardrails on inputs.
	if InternalKeyAllowedForTenant(nil, "tenant-alpha") {
		t.Error("nil apiKey must be denied")
	}
	if InternalKeyAllowedForTenant(&InternalApiKey{Id: 0, Scopes: string(provScopes)}, "tenant-alpha") {
		t.Error("apiKey.Id=0 must be denied")
	}
	if InternalKeyAllowedForTenant(narrowKey, "") {
		t.Error("empty tenantID must be denied")
	}

	// Platform admin (ScopeAll) → allow without any whitelist row.
	if !InternalKeyAllowedForTenant(adminKey, "tenant-alpha") {
		t.Error("ScopeAll key should bypass whitelist (tenant-alpha)")
	}
	if !InternalKeyAllowedForTenant(adminKey, "tenant-zzzz") {
		t.Error("ScopeAll key should bypass whitelist for any tenant")
	}

	// Narrow key, empty whitelist → deny-all.
	if InternalKeyAllowedForTenant(narrowKey, "tenant-alpha") {
		t.Error("empty whitelist should deny narrow key for tenant-alpha")
	}

	// Authorise narrowKey (id=1) for tenant-alpha only.
	if err := DB.Exec(
		`INSERT INTO internal_api_key_tenants (api_key_id, tenant_id) VALUES (?, ?)`,
		narrowKey.Id, "tenant-alpha",
	).Error; err != nil {
		t.Fatalf("seed whitelist row: %v", err)
	}

	if !InternalKeyAllowedForTenant(narrowKey, "tenant-alpha") {
		t.Error("authorised pair (key=1, tenant-alpha) should be allowed")
	}
	if InternalKeyAllowedForTenant(narrowKey, "tenant-beta") {
		t.Error("narrowKey must not access tenant-beta with no row")
	}
	if InternalKeyAllowedForTenant(otherNarrowKey, "tenant-alpha") {
		t.Error("otherNarrowKey must not access tenant-alpha with no row")
	}
}

// Ensure common import is used
var _ = common.GetTimestamp

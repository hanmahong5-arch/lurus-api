package repo

// internal_api_key_scopeall_honesty_test.go — honesty lock for BMAD-11.
//
// Claim under audit (newhub phase2_self_audit):
//
//	"TestInternalKeyAllowedForTenant skips without TEST_POSTGRES_DSN,
//	 skip-path PASS flagged."
//
// The existing TestInternalKeyAllowedForTenant lives in
// internal_api_key_integration_test.go and calls SetupTestDB(t), which
// t.Skip()s the whole test when TEST_POSTGRES_DSN is unset. That makes the
// claim true in letter — but it leaves the most security-critical branch
// (ScopeAll "*" bypass) and the input-guard branches (nil / zero-id /
// empty-tenant) exercised ONLY behind a skip that almost never runs in a
// developer/CI environment without Postgres. A skip that never asserts is a
// hollow skip-path (CLAUDE.md §4.1③: a t.Skip is not coverage).
//
// This file proves the skip-path is NOT hollow: every branch the integration
// test reaches is independently reachable HERMETICALLY here —
//   - the ScopeAll bypass + the three input guards run with NO database at all
//     (they short-circuit before DB is touched), and
//   - the narrow-scope DB-backed path runs on in-memory SQLite, no Postgres.
//
// The test's subject (InternalKeyAllowedForTenant) and the assertions
// (allow/deny contract) are independent of the implementation text, so this is
// a behaviour lock, not a tautology.

import (
	"encoding/json"
	"testing"
)

// scopesJSON marshals a scope slice the same way CreateInternalApiKey persists
// it. Kept local so the test depends on the wire format, not on a repo helper.
func scopesJSON(t *testing.T, scopes ...string) string {
	t.Helper()
	b, err := json.Marshal(scopes)
	if err != nil {
		t.Fatalf("marshal scopes: %v", err)
	}
	return string(b)
}

// TestInternalKeyAllowedForTenant_ScopeAllBypass_NoDB asserts that the
// ScopeAll wildcard bypass and the three input-guard denials all resolve
// WITHOUT any DB call. We deliberately leave repo.DB pointing at whatever the
// package default is and never install a test DB: if any of these cases fell
// through to the `DB.Table(...)` query it would either panic on a nil/closed
// handle or hang — so a clean PASS is itself evidence the branch short-circuits
// before IO. This is the core of BMAD-11: the bypass logic is real, not a
// placeholder hidden behind a Postgres-gated skip.
func TestInternalKeyAllowedForTenant_ScopeAllBypass_NoDB(t *testing.T) {
	narrow := &InternalApiKey{Id: 1, Scopes: scopesJSON(t, ScopeProvisioning)}
	admin := &InternalApiKey{Id: 99, Scopes: scopesJSON(t, ScopeAll)}
	// A key that lists several narrow scopes plus the wildcard — HasScope must
	// still see the "*" and bypass, regardless of position in the slice.
	adminMixed := &InternalApiKey{Id: 100, Scopes: scopesJSON(t, ScopeUserRead, ScopeQuotaRead, ScopeAll)}

	cases := []struct {
		name    string
		key     *InternalApiKey
		tenant  string
		want    bool
		noDBClr string // why this case must not reach the DB
	}{
		{
			name:    "nil key denied before DB",
			key:     nil,
			tenant:  "tenant-alpha",
			want:    false,
			noDBClr: "nil guard returns false on line 1 of the function",
		},
		{
			name:    "zero id denied before DB",
			key:     &InternalApiKey{Id: 0, Scopes: scopesJSON(t, ScopeProvisioning)},
			tenant:  "tenant-alpha",
			want:    false,
			noDBClr: "Id<=0 guard short-circuits",
		},
		{
			name:    "empty tenant denied before DB",
			key:     narrow,
			tenant:  "",
			want:    false,
			noDBClr: "empty tenantID guard short-circuits",
		},
		{
			name:    "ScopeAll bypass — any tenant allowed without whitelist row",
			key:     admin,
			tenant:  "tenant-alpha",
			want:    true,
			noDBClr: "HasScope(*) returns true before the whitelist query",
		},
		{
			name:    "ScopeAll bypass — a totally different tenant still allowed",
			key:     admin,
			tenant:  "tenant-that-has-no-row-anywhere",
			want:    true,
			noDBClr: "wildcard is tenant-agnostic; no DB lookup",
		},
		{
			name:    "wildcard among narrow scopes still bypasses",
			key:     adminMixed,
			tenant:  "tenant-beta",
			want:    true,
			noDBClr: "HasScope scans the slice and finds '*'",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := InternalKeyAllowedForTenant(tc.key, tc.tenant)
			if got != tc.want {
				t.Fatalf("InternalKeyAllowedForTenant(%+v, %q) = %v, want %v (%s)",
					tc.key, tc.tenant, got, tc.want, tc.noDBClr)
			}
		})
	}
}

// TestInternalKeyAllowedForTenant_NarrowScope_SQLite proves the DB-backed half
// of the authorisation contract runs on in-memory SQLite — no TEST_POSTGRES_DSN
// required — so the narrow-scope whitelist semantics are no longer reachable
// ONLY behind the Postgres-gated integration skip.
//
// The internal_api_key_tenants join table is NOT in setupSQLiteDB's AutoMigrate
// list (it is created from raw migration SQL in production), so we create it
// here with the same shape migration 013 uses. That keeps migration SQL the
// single schema source while still letting the predicate run hermetically.
func TestInternalKeyAllowedForTenant_NarrowScope_SQLite(t *testing.T) {
	cleanup := setupSQLiteDB(t)
	defer cleanup()

	if err := DB.Exec(`CREATE TABLE IF NOT EXISTS internal_api_key_tenants (
		api_key_id INTEGER NOT NULL,
		tenant_id  VARCHAR(36) NOT NULL,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (api_key_id, tenant_id)
	)`).Error; err != nil {
		t.Fatalf("create join table: %v", err)
	}

	narrow := &InternalApiKey{Id: 7, Scopes: scopesJSON(t, ScopeProvisioning)}
	other := &InternalApiKey{Id: 8, Scopes: scopesJSON(t, ScopeProvisioning)}

	// Empty whitelist → fail-closed deny for the narrow key.
	if InternalKeyAllowedForTenant(narrow, "tenant-alpha") {
		t.Fatal("narrow key with empty whitelist must be denied (fail-closed)")
	}

	// Authorise (key 7, tenant-alpha) only.
	if err := DB.Exec(
		`INSERT INTO internal_api_key_tenants (api_key_id, tenant_id) VALUES (?, ?)`,
		narrow.Id, "tenant-alpha",
	).Error; err != nil {
		t.Fatalf("seed whitelist row: %v", err)
	}

	if !InternalKeyAllowedForTenant(narrow, "tenant-alpha") {
		t.Error("authorised (key=7, tenant-alpha) should be allowed")
	}
	// Same key, different tenant → no row → deny (proves the query is tenant-scoped).
	if InternalKeyAllowedForTenant(narrow, "tenant-beta") {
		t.Error("narrow key must not reach tenant-beta with no whitelist row")
	}
	// Different key, the authorised tenant → no row → deny (proves it is key-scoped).
	if InternalKeyAllowedForTenant(other, "tenant-alpha") {
		t.Error("a different narrow key must not inherit key 7's whitelist row")
	}
}

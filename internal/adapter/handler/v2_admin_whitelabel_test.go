package handler

import (
	"net/http"
	"os"
	"strconv"
	"testing"
)

// ============================================================================
// V2 Admin White-Label HMAC Key Tests
//
// Coverage:
//   - env set + valid existing tenant_slug → 200 with 64-char hex key
//   - env set + missing tenant_slug → 400
//   - env set + invalid format tenant_slug → 400
//   - env set + nonexistent tenant_slug → 404
//   - env unset → 500
//   - two different tenant_slugs → two distinct keys (isolation)
//   - same tenant_slug twice → identical key (determinism)
//
// Auth checks (401/403) are enforced by the RootJWTAuth middleware at the
// route-registration layer (see api-v2-router.go). The V2 test router uses
// mock auth that always injects a tenant context, so 401 cannot be observed
// here — that path is covered transitively by other admin handler tests
// that exercise the same middleware in production.
// ============================================================================

const testWhiteLabelMasterSecret = "test-master-secret-do-not-use-in-prod"

// registerWhiteLabelRoute mounts the white-label endpoint under the v2
// admin group of the test router. The setup test router doesn't include
// it by default since it's specific to this test file.
func registerWhiteLabelRoute(ctx *V2TestContext) {
	// The route is registered on the existing test router's admin group.
	// Test router defines `/api/v2/admin` group with mockAuth — we append
	// our route to the same router engine.
	ctx.Router.GET("/api/v2/admin/whitelabel/hmac-key", GetWhiteLabelHMACKey)
}

func TestGetWhiteLabelHMACKey_Success(t *testing.T) {
	ctx := SetupV2TestRouter(t)
	defer ctx.Cleanup()
	registerWhiteLabelRoute(ctx)

	t.Setenv(whitelabelMasterSecretEnv, testWhiteLabelMasterSecret)

	headers := map[string]string{
		"X-Test-User-ID": strconv.Itoa(ctx.RootUser.Id),
	}
	// ctx.TenantID is also the slug in test setup (SetupV2TestRouter uses
	// the same value for both).
	path := "/api/v2/admin/whitelabel/hmac-key?tenant_slug=" + ctx.TenantID
	w := V2Request(ctx.Router, http.MethodGet, path, nil, headers)

	AssertV2Status(t, w, http.StatusOK)
	resp := AssertV2Success(t, w)

	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected data object, got %T", resp["data"])
	}
	hmacKey, ok := data["hmac_key"].(string)
	if !ok {
		t.Fatalf("expected hmac_key string, got %T", data["hmac_key"])
	}
	if len(hmacKey) != 64 {
		t.Errorf("expected 64-char hex key, got %d chars: %q", len(hmacKey), hmacKey)
	}
	// Verify it's actual hex (only 0-9, a-f).
	for i, c := range hmacKey {
		isHex := (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')
		if !isHex {
			t.Errorf("hmac_key contains non-hex char %q at index %d", c, i)
			break
		}
	}
}

func TestGetWhiteLabelHMACKey_MissingTenantSlug(t *testing.T) {
	ctx := SetupV2TestRouter(t)
	defer ctx.Cleanup()
	registerWhiteLabelRoute(ctx)

	t.Setenv(whitelabelMasterSecretEnv, testWhiteLabelMasterSecret)

	headers := map[string]string{
		"X-Test-User-ID": strconv.Itoa(ctx.RootUser.Id),
	}
	w := V2Request(ctx.Router, http.MethodGet, "/api/v2/admin/whitelabel/hmac-key", nil, headers)

	AssertV2Status(t, w, http.StatusBadRequest)
}

func TestGetWhiteLabelHMACKey_InvalidSlugFormat(t *testing.T) {
	ctx := SetupV2TestRouter(t)
	defer ctx.Cleanup()
	registerWhiteLabelRoute(ctx)

	t.Setenv(whitelabelMasterSecretEnv, testWhiteLabelMasterSecret)

	headers := map[string]string{
		"X-Test-User-ID": strconv.Itoa(ctx.RootUser.Id),
	}
	// Contains '@' which isValidTenantSlug rejects.
	w := V2Request(ctx.Router, http.MethodGet, "/api/v2/admin/whitelabel/hmac-key?tenant_slug=bad@slug", nil, headers)

	AssertV2Status(t, w, http.StatusBadRequest)
}

func TestGetWhiteLabelHMACKey_NonexistentTenant(t *testing.T) {
	ctx := SetupV2TestRouter(t)
	defer ctx.Cleanup()
	registerWhiteLabelRoute(ctx)

	t.Setenv(whitelabelMasterSecretEnv, testWhiteLabelMasterSecret)

	headers := map[string]string{
		"X-Test-User-ID": strconv.Itoa(ctx.RootUser.Id),
	}
	w := V2Request(ctx.Router, http.MethodGet, "/api/v2/admin/whitelabel/hmac-key?tenant_slug=does-not-exist-tenant", nil, headers)

	AssertV2Status(t, w, http.StatusNotFound)
}

func TestGetWhiteLabelHMACKey_EnvUnset(t *testing.T) {
	ctx := SetupV2TestRouter(t)
	defer ctx.Cleanup()
	registerWhiteLabelRoute(ctx)

	// Explicitly unset (t.Setenv with empty value triggers Setenv("", ""),
	// which has a different semantic on some platforms — use Unsetenv).
	prev := os.Getenv(whitelabelMasterSecretEnv)
	os.Unsetenv(whitelabelMasterSecretEnv)
	defer os.Setenv(whitelabelMasterSecretEnv, prev)

	headers := map[string]string{
		"X-Test-User-ID": strconv.Itoa(ctx.RootUser.Id),
	}
	path := "/api/v2/admin/whitelabel/hmac-key?tenant_slug=" + ctx.TenantID
	w := V2Request(ctx.Router, http.MethodGet, path, nil, headers)

	AssertV2Status(t, w, http.StatusInternalServerError)
	resp := ParseV2Response(t, w)
	if success, ok := resp["success"].(bool); !ok || success {
		t.Errorf("expected success=false, got %v", resp["success"])
	}
}

func TestGetWhiteLabelHMACKey_Determinism(t *testing.T) {
	ctx := SetupV2TestRouter(t)
	defer ctx.Cleanup()
	registerWhiteLabelRoute(ctx)

	t.Setenv(whitelabelMasterSecretEnv, testWhiteLabelMasterSecret)

	headers := map[string]string{
		"X-Test-User-ID": strconv.Itoa(ctx.RootUser.Id),
	}
	path := "/api/v2/admin/whitelabel/hmac-key?tenant_slug=" + ctx.TenantID

	w1 := V2Request(ctx.Router, http.MethodGet, path, nil, headers)
	AssertV2Status(t, w1, http.StatusOK)
	key1 := AssertV2Success(t, w1)["data"].(map[string]interface{})["hmac_key"].(string)

	w2 := V2Request(ctx.Router, http.MethodGet, path, nil, headers)
	AssertV2Status(t, w2, http.StatusOK)
	key2 := AssertV2Success(t, w2)["data"].(map[string]interface{})["hmac_key"].(string)

	if key1 != key2 {
		t.Errorf("determinism violated: same tenant slug produced different keys\n  %s\n  %s", key1, key2)
	}
}

func TestGetWhiteLabelHMACKey_TenantIsolation(t *testing.T) {
	ctx := SetupV2TestRouter(t)
	defer ctx.Cleanup()
	registerWhiteLabelRoute(ctx)

	t.Setenv(whitelabelMasterSecretEnv, testWhiteLabelMasterSecret)

	// Create a second tenant.
	secondSlug := ctx.TenantID + "-second"
	secondTenant := &struct {
		Id           string
		Slug         string
		Name         string
		Status       int
		ZitadelOrgID string
	}{}
	_ = secondTenant // suppress

	// Use repo to insert a sibling tenant.
	if err := ctx.DB.Exec(
		"INSERT INTO tenants (id, slug, name, status, zitadel_org_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
		secondSlug, secondSlug, "Second Tenant", 1, "org_second_456", "2026-01-01", "2026-01-01",
	).Error; err != nil {
		t.Fatalf("failed to seed second tenant: %v", err)
	}

	headers := map[string]string{
		"X-Test-User-ID": strconv.Itoa(ctx.RootUser.Id),
	}

	w1 := V2Request(ctx.Router, http.MethodGet, "/api/v2/admin/whitelabel/hmac-key?tenant_slug="+ctx.TenantID, nil, headers)
	AssertV2Status(t, w1, http.StatusOK)
	key1 := AssertV2Success(t, w1)["data"].(map[string]interface{})["hmac_key"].(string)

	w2 := V2Request(ctx.Router, http.MethodGet, "/api/v2/admin/whitelabel/hmac-key?tenant_slug="+secondSlug, nil, headers)
	AssertV2Status(t, w2, http.StatusOK)
	key2 := AssertV2Success(t, w2)["data"].(map[string]interface{})["hmac_key"].(string)

	if key1 == key2 {
		t.Errorf("isolation violated: different slugs produced identical keys (%s)", key1)
	}
}

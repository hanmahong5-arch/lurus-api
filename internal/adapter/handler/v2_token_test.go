package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"testing"

	"github.com/LurusTech/lurus-hub/internal/pkg/common"
)

// ============================================================================
// V2 Token Controller Tests
// ============================================================================

func TestListTokensV2_Pagination(t *testing.T) {
	ctx := SetupV2TestRouter(t)
	defer ctx.Cleanup()

	// Create 25 tokens
	for i := 0; i < 25; i++ {
		SeedV2Token(t, ctx, ctx.NormalUser.Id, fmt.Sprintf("Token %d", i))
	}

	// Test first page — handler reads p/size params, returns data.items
	w := V2RequestAsUser(ctx, ctx.NormalUser, http.MethodGet, "/api/v2/test-tenant/tokens?p=1&size=10", nil, nil)
	resp := AssertV2Success(t, w)
	data := resp["data"].(map[string]interface{})

	tokens := data["items"].([]interface{})
	if len(tokens) != 10 {
		t.Errorf("expected 10 tokens on first page, got %d", len(tokens))
	}

	total := int(data["total"].(float64))
	if total != 25 {
		t.Errorf("expected total=25, got %d", total)
	}

	page := int(data["page"].(float64))
	if page != 1 {
		t.Errorf("expected page=1, got %d", page)
	}

	// Test second page
	w = V2RequestAsUser(ctx, ctx.NormalUser, http.MethodGet, "/api/v2/test-tenant/tokens?p=2&size=10", nil, nil)
	resp = AssertV2Success(t, w)
	data = resp["data"].(map[string]interface{})
	tokens = data["items"].([]interface{})
	if len(tokens) != 10 {
		t.Errorf("expected 10 tokens on second page, got %d", len(tokens))
	}

	// Test third page (should have 5 tokens)
	w = V2RequestAsUser(ctx, ctx.NormalUser, http.MethodGet, "/api/v2/test-tenant/tokens?p=3&size=10", nil, nil)
	resp = AssertV2Success(t, w)
	data = resp["data"].(map[string]interface{})
	tokens = data["items"].([]interface{})
	if len(tokens) != 5 {
		t.Errorf("expected 5 tokens on third page, got %d", len(tokens))
	}
}

func TestCreateTokenV2_Success(t *testing.T) {
	ctx := SetupV2TestRouter(t)
	defer ctx.Cleanup()

	body := map[string]interface{}{
		"name":            "My New Token",
		"expired_time":    -1,
		"unlimited_quota": true,
	}

	w := V2RequestAsUser(ctx, ctx.NormalUser, http.MethodPost, "/api/v2/test-tenant/tokens", body, nil)

	AssertV2Status(t, w, http.StatusCreated)
	resp := AssertV2Success(t, w)

	data := resp["data"].(map[string]interface{})
	if data["name"] != "My New Token" {
		t.Errorf("expected name='My New Token', got %v", data["name"])
	}

	// Key should be returned on creation
	key, ok := data["key"].(string)
	if !ok || key == "" {
		t.Error("expected key to be returned on token creation")
	}
	if len(key) < 10 {
		t.Errorf("key seems too short: %s", key)
	}
}

func TestCreateTokenV2_NameTooLong(t *testing.T) {
	ctx := SetupV2TestRouter(t)
	defer ctx.Cleanup()

	// Create a name longer than 50 characters
	longName := make([]byte, 51)
	for i := range longName {
		longName[i] = 'a'
	}

	body := map[string]interface{}{
		"name":            string(longName),
		"unlimited_quota": true,
	}

	w := V2RequestAsUser(ctx, ctx.NormalUser, http.MethodPost, "/api/v2/test-tenant/tokens", body, nil)

	AssertV2Status(t, w, http.StatusBadRequest)
	resp := ParseV2Response(t, w)
	if msg, ok := resp["message"].(string); ok {
		if msg != "令牌名称过长" {
			t.Errorf("unexpected error message: %s", msg)
		}
	}
}

func TestCreateTokenV2_NegativeQuota(t *testing.T) {
	ctx := SetupV2TestRouter(t)
	defer ctx.Cleanup()

	body := map[string]interface{}{
		"name":            "Negative Quota Token",
		"unlimited_quota": false,
		"remain_quota":    -1000,
	}

	w := V2RequestAsUser(ctx, ctx.NormalUser, http.MethodPost, "/api/v2/test-tenant/tokens", body, nil)

	AssertV2Status(t, w, http.StatusBadRequest)
	resp := ParseV2Response(t, w)
	if msg, ok := resp["message"].(string); ok {
		if msg != "额度值不能为负数" {
			t.Errorf("unexpected error message: %s", msg)
		}
	}
}

func TestCreateTokenV2_RequiredName(t *testing.T) {
	ctx := SetupV2TestRouter(t)
	defer ctx.Cleanup()

	body := map[string]interface{}{
		"unlimited_quota": true,
		// Missing "name"
	}

	w := V2RequestAsUser(ctx, ctx.NormalUser, http.MethodPost, "/api/v2/test-tenant/tokens", body, nil)

	AssertV2Status(t, w, http.StatusBadRequest)
}

func TestUpdateTokenV2_Success(t *testing.T) {
	ctx := SetupV2TestRouter(t)
	defer ctx.Cleanup()

	// Create a token first
	token := SeedV2Token(t, ctx, ctx.NormalUser.Id, "Original Name")

	// Update the token
	body := map[string]interface{}{
		"name": "Updated Name",
	}

	path := fmt.Sprintf("/api/v2/test-tenant/tokens/%d", token.Id)
	w := V2RequestAsUser(ctx, ctx.NormalUser, http.MethodPut, path, body, nil)

	AssertV2Status(t, w, http.StatusOK)
	resp := AssertV2Success(t, w)

	data := resp["data"].(map[string]interface{})
	if data["name"] != "Updated Name" {
		t.Errorf("expected name='Updated Name', got %v", data["name"])
	}
}

func TestUpdateTokenV2_ExpiredToEnabled(t *testing.T) {
	ctx := SetupV2TestRouter(t)
	defer ctx.Cleanup()

	// Create an expired token
	token := SeedV2Token(t, ctx, ctx.NormalUser.Id, "Expired Token")
	token.Status = common.TokenStatusExpired
	token.ExpiredTime = common.GetTimestamp() - 3600 // Expired 1 hour ago
	ctx.DB.Save(token)

	// Try to enable it without updating expiration time
	body := map[string]interface{}{
		"status": common.TokenStatusEnabled,
	}

	path := fmt.Sprintf("/api/v2/test-tenant/tokens/%d", token.Id)
	w := V2RequestAsUser(ctx, ctx.NormalUser, http.MethodPut, path, body, nil)

	AssertV2Status(t, w, http.StatusBadRequest)
	resp := ParseV2Response(t, w)
	if msg, ok := resp["message"].(string); ok {
		if msg != "令牌已过期，无法启用，请先修改令牌过期时间，或者设置为永不过期" {
			t.Errorf("unexpected error message: %s", msg)
		}
	}
}

func TestUpdateTokenV2_TenantMismatch(t *testing.T) {
	ctx := SetupV2TestRouter(t)
	defer ctx.Cleanup()

	// Create a token in a different tenant
	otherTenantID := "other-tenant-123"
	token := SeedV2Token(t, ctx, ctx.NormalUser.Id, "Other Tenant Token")
	token.TenantId = otherTenantID
	ctx.DB.Save(token)

	// Try to update from the test tenant
	body := map[string]interface{}{
		"name": "Hacked Name",
	}

	path := fmt.Sprintf("/api/v2/test-tenant/tokens/%d", token.Id)
	w := V2RequestAsUser(ctx, ctx.NormalUser, http.MethodPut, path, body, nil)

	AssertV2Status(t, w, http.StatusForbidden)
	resp := ParseV2Response(t, w)
	if msg, ok := resp["message"].(string); ok {
		if msg != "Access denied" {
			t.Errorf("unexpected error message: %s", msg)
		}
	}
}

func TestDeleteTokenV2_Success(t *testing.T) {
	ctx := SetupV2TestRouter(t)
	defer ctx.Cleanup()

	// Create a token
	token := SeedV2Token(t, ctx, ctx.NormalUser.Id, "Token to Delete")

	// Delete it
	path := fmt.Sprintf("/api/v2/test-tenant/tokens/%d", token.Id)
	w := V2RequestAsUser(ctx, ctx.NormalUser, http.MethodDelete, path, nil, nil)

	AssertV2Status(t, w, http.StatusOK)
	AssertV2Success(t, w)

	// Verify it's deleted
	w = V2RequestAsUser(ctx, ctx.NormalUser, http.MethodGet, "/api/v2/test-tenant/tokens", nil, nil)
	resp := AssertV2Success(t, w)
	data := resp["data"].(map[string]interface{})
	total := int(data["total"].(float64))
	if total != 0 {
		t.Errorf("expected 0 tokens after deletion, got %d", total)
	}
}

func TestDeleteTokenV2_NotOwned(t *testing.T) {
	ctx := SetupV2TestRouter(t)
	defer ctx.Cleanup()

	// Create a token owned by admin user
	token := SeedV2Token(t, ctx, ctx.AdminUser.Id, "Admin Token")

	// Try to delete as normal user
	path := fmt.Sprintf("/api/v2/test-tenant/tokens/%d", token.Id)
	w := V2RequestAsUser(ctx, ctx.NormalUser, http.MethodDelete, path, nil, nil)

	// Should fail - token not found for this user
	AssertV2Status(t, w, http.StatusNotFound)
}

func TestDeleteTokenV2_InvalidID(t *testing.T) {
	ctx := SetupV2TestRouter(t)
	defer ctx.Cleanup()

	// Try to delete with invalid ID
	w := V2RequestAsUser(ctx, ctx.NormalUser, http.MethodDelete, "/api/v2/test-tenant/tokens/invalid", nil, nil)

	AssertV2Status(t, w, http.StatusBadRequest)
}

func TestDeleteTokenV2_NonexistentID(t *testing.T) {
	ctx := SetupV2TestRouter(t)
	defer ctx.Cleanup()

	// Try to delete a token that doesn't exist
	w := V2RequestAsUser(ctx, ctx.NormalUser, http.MethodDelete, "/api/v2/test-tenant/tokens/99999", nil, nil)

	AssertV2Status(t, w, http.StatusNotFound)
}

func TestListTokensV2_UserIsolation(t *testing.T) {
	ctx := SetupV2TestRouter(t)
	defer ctx.Cleanup()

	// Create tokens for both users
	SeedV2Token(t, ctx, ctx.NormalUser.Id, "Normal User Token 1")
	SeedV2Token(t, ctx, ctx.NormalUser.Id, "Normal User Token 2")
	SeedV2Token(t, ctx, ctx.AdminUser.Id, "Admin User Token")

	// List as normal user - should only see their own tokens
	w := V2RequestAsUser(ctx, ctx.NormalUser, http.MethodGet, "/api/v2/test-tenant/tokens", nil, nil)
	resp := AssertV2Success(t, w)
	data := resp["data"].(map[string]interface{})
	total := int(data["total"].(float64))
	if total != 2 {
		t.Errorf("expected 2 tokens for normal user, got %d", total)
	}

	// List as admin user - should only see their own token
	headers := map[string]string{
		"X-Test-Tenant-ID": ctx.TenantID,
		"X-Test-User-ID":   strconv.Itoa(ctx.AdminUser.Id),
	}
	w = V2Request(ctx.Router, http.MethodGet, "/api/v2/test-tenant/tokens", nil, headers)
	resp = AssertV2Success(t, w)
	data = resp["data"].(map[string]interface{})
	total = int(data["total"].(float64))
	if total != 1 {
		t.Errorf("expected 1 token for admin user, got %d", total)
	}
}

// TestListTokensV2_ForbiddenFields guards the tokenView whitelist: the list
// endpoint must never expose the raw bearer key or tenant/linkage internals.
func TestListTokensV2_ForbiddenFields(t *testing.T) {
	ctx := SetupV2TestRouter(t)
	defer ctx.Cleanup()

	SeedV2Token(t, ctx, ctx.NormalUser.Id, "Field Whitelist Token")

	w := V2RequestAsUser(ctx, ctx.NormalUser, http.MethodGet, "/api/v2/test-tenant/tokens", nil, nil)
	resp := AssertV2Success(t, w)
	data := resp["data"].(map[string]interface{})
	items := data["items"].([]interface{})
	if len(items) != 1 {
		t.Fatalf("expected 1 token, got %d", len(items))
	}
	tk := items[0].(map[string]interface{})

	for _, f := range []string{"key", "tenant_id", "user_id", "identity_account_id", "creator_user_id", "DeletedAt"} {
		if _, exists := tk[f]; exists {
			t.Errorf("forbidden field %q leaked through tokenView", f)
		}
	}
	// sanity: the curated fields are present
	if _, ok := tk["name"]; !ok {
		t.Error("expected name field in tokenView")
	}
}

func TestUpdateTokenV2_InvalidID(t *testing.T) {
	ctx := SetupV2TestRouter(t)
	defer ctx.Cleanup()

	body := map[string]interface{}{
		"name": "Updated Name",
	}
	w := V2RequestAsUser(ctx, ctx.NormalUser, http.MethodPut, "/api/v2/test-tenant/tokens/invalid", body, nil)

	AssertV2Status(t, w, http.StatusBadRequest)
	resp := ParseV2Response(t, w)
	if msg, ok := resp["message"].(string); ok {
		if msg != "Invalid token ID" {
			t.Errorf("unexpected error message: %s", msg)
		}
	}
}

func TestUpdateTokenV2_TokenNotFound(t *testing.T) {
	ctx := SetupV2TestRouter(t)
	defer ctx.Cleanup()

	body := map[string]interface{}{
		"name": "Ghost Token",
	}
	w := V2RequestAsUser(ctx, ctx.NormalUser, http.MethodPut, "/api/v2/test-tenant/tokens/99999", body, nil)

	AssertV2Status(t, w, http.StatusNotFound)
	resp := ParseV2Response(t, w)
	if msg, ok := resp["message"].(string); ok {
		if msg != "Token not found" {
			t.Errorf("unexpected error message: %s", msg)
		}
	}
}

func TestUpdateTokenV2_NameValidation(t *testing.T) {
	ctx := SetupV2TestRouter(t)
	defer ctx.Cleanup()

	token := SeedV2Token(t, ctx, ctx.NormalUser.Id, "Original Name")

	// Name exceeding 50 chars
	longName := make([]byte, 51)
	for i := range longName {
		longName[i] = 'x'
	}

	body := map[string]interface{}{
		"name": string(longName),
	}
	path := fmt.Sprintf("/api/v2/test-tenant/tokens/%d", token.Id)
	w := V2RequestAsUser(ctx, ctx.NormalUser, http.MethodPut, path, body, nil)

	AssertV2Status(t, w, http.StatusBadRequest)
	resp := ParseV2Response(t, w)
	// Service layer returns Chinese error message
	if msg, ok := resp["message"].(string); ok {
		if msg != "令牌名称过长" {
			t.Errorf("unexpected error message: %s", msg)
		}
	}
}

func TestCreateTokenV2_UnlimitedQuota(t *testing.T) {
	ctx := SetupV2TestRouter(t)
	defer ctx.Cleanup()

	body := map[string]interface{}{
		"name":            "Unlimited Token",
		"unlimited_quota": true,
		"remain_quota":    0,
	}

	w := V2RequestAsUser(ctx, ctx.NormalUser, http.MethodPost, "/api/v2/test-tenant/tokens", body, nil)

	AssertV2Status(t, w, http.StatusCreated)
	resp := AssertV2Success(t, w)

	data := resp["data"].(map[string]interface{})
	if data["name"] != "Unlimited Token" {
		t.Errorf("expected name='Unlimited Token', got %v", data["name"])
	}
	key, ok := data["key"].(string)
	if !ok || key == "" {
		t.Error("expected key to be returned on token creation")
	}
}

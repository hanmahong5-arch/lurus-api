package handler

import (
	"net/http"
	"testing"

	"github.com/LurusTech/lurus-hub/internal/adapter/repo"
)

// ============================================================================
// V2 Platform Admin — Tenant Tests
// ============================================================================

// TestCreateTenantV2_ResponseReflectsPersistedFields proves the create
// response carries the requested plan_type/max_users/max_quota instead of
// the pre-update defaults (the DB row and the response must agree).
func TestCreateTenantV2_ResponseReflectsPersistedFields(t *testing.T) {
	ctx := SetupV2TestRouter(t)
	defer ctx.Cleanup()

	// CreateTenant is not registered by the shared harness; attach it here.
	ctx.Router.POST("/api/v2/admin/tenants", CreateTenant)

	body := map[string]interface{}{
		"zitadel_org_id": "org_create_resp_test",
		"slug":           "create-resp-test",
		"name":           "Create Resp Test",
		"plan_type":      "pro",
		"max_users":      7,
		"max_quota":      424242,
	}
	w := V2Request(ctx.Router, http.MethodPost, "/api/v2/admin/tenants", body, nil)
	AssertV2Status(t, w, http.StatusCreated)
	resp := AssertV2Success(t, w)

	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected data object in response, got %v", resp["data"])
	}
	if got, _ := data["plan_type"].(string); got != "pro" {
		t.Errorf("response plan_type = %q, want %q", got, "pro")
	}
	if got, _ := data["max_users"].(float64); int(got) != 7 {
		t.Errorf("response max_users = %v, want 7", data["max_users"])
	}
	if got, _ := data["max_quota"].(float64); int64(got) != 424242 {
		t.Errorf("response max_quota = %v, want 424242", data["max_quota"])
	}

	// And the DB row must match what the response claims.
	var stored repo.Tenant
	if err := ctx.DB.Where("zitadel_org_id = ?", "org_create_resp_test").First(&stored).Error; err != nil {
		t.Fatalf("failed to load created tenant: %v", err)
	}
	if stored.PlanType != "pro" || stored.MaxUsers != 7 || stored.MaxQuota != 424242 {
		t.Errorf("stored tenant fields = (%s,%d,%d), want (pro,7,424242)",
			stored.PlanType, stored.MaxUsers, stored.MaxQuota)
	}
}

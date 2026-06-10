package handler

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/LurusTech/lurus-hub/internal/pkg/common"
)

// ============================================================================
// V2 Platform Admin — User Management Tests
// ============================================================================

// TestListAdminUsersV2_Whitelist proves the list endpoint never leaks secret /
// internal columns (access_token, setting, remark, tenant_id, lurus_account_id),
// only the curated whitelist.
func TestListAdminUsersV2_Whitelist(t *testing.T) {
	ctx := SetupV2TestRouter(t)
	defer ctx.Cleanup()

	// Plant a secret access token + remark on a user so a leak would be visible.
	secret := "supersecrettoken0000000000000000"
	ctx.NormalUser.SetAccessToken(secret)
	ctx.NormalUser.Remark = "internal note"
	if err := ctx.DB.Save(ctx.NormalUser).Error; err != nil {
		t.Fatalf("failed to seed user secrets: %v", err)
	}

	w := V2RequestAsUser(ctx, ctx.RootUser, http.MethodGet, "/api/v2/admin/users", nil, []string{"root"})
	AssertV2Status(t, w, http.StatusOK)
	resp := AssertV2Success(t, w)

	data := resp["data"].(map[string]interface{})
	users := data["users"].([]interface{})
	if len(users) == 0 {
		t.Fatal("expected at least one user in the admin list")
	}
	for _, raw := range users {
		u := raw.(map[string]interface{})
		for _, f := range []string{"access_token", "setting", "remark", "tenant_id", "lurus_account_id", "DeletedAt"} {
			if _, exists := u[f]; exists {
				t.Errorf("forbidden field %q leaked through adminUserView", f)
			}
		}
		// The curated fields must be present.
		if _, ok := u["username"]; !ok {
			t.Error("expected username field in adminUserView")
		}
	}
}

// TestListAdminUsersV2_StatusFilter exercises the status filter path.
func TestListAdminUsersV2_StatusFilter(t *testing.T) {
	ctx := SetupV2TestRouter(t)
	defer ctx.Cleanup()

	// Disable the normal user; the others stay enabled.
	ctx.NormalUser.Status = common.UserStatusDisabled
	if err := ctx.DB.Save(ctx.NormalUser).Error; err != nil {
		t.Fatalf("failed to disable user: %v", err)
	}

	path := fmt.Sprintf("/api/v2/admin/users?status=%d", common.UserStatusDisabled)
	w := V2RequestAsUser(ctx, ctx.RootUser, http.MethodGet, path, nil, []string{"root"})
	AssertV2Status(t, w, http.StatusOK)
	resp := AssertV2Success(t, w)

	users := resp["data"].(map[string]interface{})["users"].([]interface{})
	if len(users) != 1 {
		t.Fatalf("expected exactly 1 disabled user, got %d", len(users))
	}
	got := users[0].(map[string]interface{})
	if int(got["id"].(float64)) != ctx.NormalUser.Id {
		t.Errorf("status filter returned the wrong user: %v", got["id"])
	}
}

// TestUpdateAdminUserV2_RoleStatusQuota updates the three mutable fields and
// asserts persistence via the returned view.
func TestUpdateAdminUserV2_RoleStatusQuota(t *testing.T) {
	ctx := SetupV2TestRouter(t)
	defer ctx.Cleanup()

	body := map[string]interface{}{
		"role":   common.RoleAdminUser,
		"status": common.UserStatusDisabled,
		"quota":  4242,
		"group":  "vip",
	}
	path := fmt.Sprintf("/api/v2/admin/users/%d", ctx.NormalUser.Id)
	w := V2RequestAsUser(ctx, ctx.RootUser, http.MethodPut, path, body, []string{"root"})
	AssertV2Status(t, w, http.StatusOK)
	resp := AssertV2Success(t, w)

	d := resp["data"].(map[string]interface{})
	if int(d["role"].(float64)) != common.RoleAdminUser {
		t.Errorf("expected role=%d, got %v", common.RoleAdminUser, d["role"])
	}
	if int(d["status"].(float64)) != common.UserStatusDisabled {
		t.Errorf("expected status=%d, got %v", common.UserStatusDisabled, d["status"])
	}
	if int(d["quota"].(float64)) != 4242 {
		t.Errorf("expected quota=4242, got %v", d["quota"])
	}
	if d["group"] != "vip" {
		t.Errorf("expected group=vip, got %v", d["group"])
	}
}

// TestDeleteAdminUserV2_SelfGuard refuses to let an admin delete their own
// account.
func TestDeleteAdminUserV2_SelfGuard(t *testing.T) {
	ctx := SetupV2TestRouter(t)
	defer ctx.Cleanup()

	path := fmt.Sprintf("/api/v2/admin/users/%d", ctx.RootUser.Id)
	w := V2RequestAsUser(ctx, ctx.RootUser, http.MethodDelete, path, nil, []string{"root"})
	AssertV2Error(t, w, http.StatusBadRequest)
}

// TestDeleteAdminUserV2_Success deletes another (non-root) user.
func TestDeleteAdminUserV2_Success(t *testing.T) {
	ctx := SetupV2TestRouter(t)
	defer ctx.Cleanup()

	path := fmt.Sprintf("/api/v2/admin/users/%d", ctx.NormalUser.Id)
	w := V2RequestAsUser(ctx, ctx.RootUser, http.MethodDelete, path, nil, []string{"root"})
	AssertV2Status(t, w, http.StatusOK)
	AssertV2Success(t, w)
}

// TestAdminUsersV2_NonRootRejected confirms a non-root caller is forbidden from
// every admin user endpoint.
func TestAdminUsersV2_NonRootRejected(t *testing.T) {
	ctx := SetupV2TestRouter(t)
	defer ctx.Cleanup()

	w := V2RequestAsUser(ctx, ctx.NormalUser, http.MethodGet, "/api/v2/admin/users", nil, nil)
	AssertV2Error(t, w, http.StatusForbidden)

	path := fmt.Sprintf("/api/v2/admin/users/%d", ctx.AdminUser.Id)
	w = V2RequestAsUser(ctx, ctx.NormalUser, http.MethodPut, path, map[string]interface{}{"quota": 1}, nil)
	AssertV2Error(t, w, http.StatusForbidden)

	w = V2RequestAsUser(ctx, ctx.NormalUser, http.MethodDelete, path, nil, nil)
	AssertV2Error(t, w, http.StatusForbidden)
}

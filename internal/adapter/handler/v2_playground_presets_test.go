/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
package handler

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/LurusTech/lurus-hub/internal/adapter/middleware"
	"github.com/LurusTech/lurus-hub/internal/adapter/repo"
	"github.com/gin-gonic/gin"
)

func setupPresetTestRouter(t *testing.T) *V2TestContext {
	t.Helper()
	ctx := SetupV2TestRouter(t)

	// Migrate the preset table that isn't in the shared v2 test migration list.
	if err := ctx.DB.AutoMigrate(&repo.PlaygroundPreset{}); err != nil {
		if !strings.Contains(err.Error(), "already exists") {
			t.Fatalf("AutoMigrate PlaygroundPreset: %v", err)
		}
	}

	// Re-use the same mock auth inline so preset routes carry tenant_context.
	mockAuth := buildMockAuth(ctx)
	pg := ctx.Router.Group("/api/v2/:tenant_slug/playground/presets")
	pg.Use(mockAuth)
	{
		pg.GET("", ListPresetsV2)
		pg.POST("", CreatePresetV2)
		pg.DELETE("/:id", DeletePresetV2)
	}
	return ctx
}

// buildMockAuth constructs the same mock middleware used by SetupV2TestRouter
// but without duplicating its full internals — it reads the same X-Test-*
// headers that V2RequestWithContext / V2RequestAsUser inject.
func buildMockAuth(ctx *V2TestContext) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := c.GetHeader("X-Test-Tenant-ID")
		if tenantID == "" {
			tenantID = ctx.TenantID
		}
		userIDStr := c.GetHeader("X-Test-User-ID")
		userID := ctx.NormalUser.Id
		if userIDStr != "" {
			if id, err := fmt.Sscan(userIDStr, &userID); id == 0 || err != nil {
				userID = ctx.NormalUser.Id
			}
		}
		rolesHeader := c.GetHeader("X-Test-Roles")
		var roles []string
		if rolesHeader != "" {
			roles = strings.Split(rolesHeader, ",")
		}
		tenantCtx := &middleware.TenantContext{
			TenantID: tenantID,
			UserID:   userID,
			Email:    "test@test.local",
			Username: "testuser",
			Roles:    roles,
		}
		c.Set("tenant_context", tenantCtx)
		c.Next()
	}
}

// TestV2PresetList_Empty — GET returns empty list when no presets exist.
func TestV2PresetList_Empty(t *testing.T) {
	ctx := setupPresetTestRouter(t)
	defer ctx.Cleanup()

	w := V2RequestWithContext(ctx, "GET", "/api/v2/test-slug/playground/presets", nil)
	resp := AssertV2Success(t, w)

	data, ok := resp["data"].([]interface{})
	if !ok {
		t.Fatalf("expected data to be array, got %T: %v", resp["data"], resp["data"])
	}
	if len(data) != 0 {
		t.Errorf("expected empty list, got %d items", len(data))
	}
}

// TestV2PresetCreate_Happy — POST creates a preset and returns 201.
func TestV2PresetCreate_Happy(t *testing.T) {
	ctx := setupPresetTestRouter(t)
	defer ctx.Cleanup()

	body := map[string]interface{}{
		"name":   "my-preset",
		"prompt": "You are terse.",
		"models": `["gpt-4o"]`,
		"params": `{"temperature":0.5}`,
	}
	w := V2RequestWithContext(ctx, "POST", "/api/v2/test-slug/playground/presets", body)
	AssertV2Status(t, w, http.StatusCreated)
	resp := ParseV2Response(t, w)

	if success, _ := resp["success"].(bool); !success {
		t.Fatalf("expected success=true, got: %s", w.Body.String())
	}
	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected data map, got %T", resp["data"])
	}
	if data["name"] != "my-preset" {
		t.Errorf("unexpected name: %v", data["name"])
	}
}

// TestV2PresetCreate_DupName — second POST with same name returns 409.
func TestV2PresetCreate_DupName(t *testing.T) {
	ctx := setupPresetTestRouter(t)
	defer ctx.Cleanup()

	body := map[string]interface{}{"name": "dup-preset", "prompt": "a"}
	V2RequestWithContext(ctx, "POST", "/api/v2/test-slug/playground/presets", body)

	w := V2RequestWithContext(ctx, "POST", "/api/v2/test-slug/playground/presets", body)
	AssertV2Error(t, w, http.StatusConflict)
}

// TestV2PresetList_ReturnsOwn — list after create returns the created preset.
func TestV2PresetList_ReturnsOwn(t *testing.T) {
	ctx := setupPresetTestRouter(t)
	defer ctx.Cleanup()

	body := map[string]interface{}{"name": "listed-preset", "prompt": "hello"}
	V2RequestWithContext(ctx, "POST", "/api/v2/test-slug/playground/presets", body)

	w := V2RequestWithContext(ctx, "GET", "/api/v2/test-slug/playground/presets", nil)
	resp := AssertV2Success(t, w)

	data, _ := resp["data"].([]interface{})
	if len(data) != 1 {
		t.Fatalf("expected 1 preset, got %d", len(data))
	}
	item, _ := data[0].(map[string]interface{})
	if item["name"] != "listed-preset" {
		t.Errorf("unexpected preset name: %v", item["name"])
	}
}

// TestV2PresetDelete_Happy — owner can delete their own preset.
func TestV2PresetDelete_Happy(t *testing.T) {
	ctx := setupPresetTestRouter(t)
	defer ctx.Cleanup()

	createBody := map[string]interface{}{"name": "to-delete", "prompt": "bye"}
	cw := V2RequestWithContext(ctx, "POST", "/api/v2/test-slug/playground/presets", createBody)
	AssertV2Status(t, cw, http.StatusCreated)

	createResp := ParseV2Response(t, cw)
	dataMap, _ := createResp["data"].(map[string]interface{})
	idFloat, _ := dataMap["id"].(float64)
	idStr := fmt.Sprintf("%d", int(idFloat))

	path := "/api/v2/test-slug/playground/presets/" + idStr
	w := V2RequestWithContext(ctx, "DELETE", path, nil)
	AssertV2Success(t, w)

	// Verify it's gone from the list.
	lw := V2RequestWithContext(ctx, "GET", "/api/v2/test-slug/playground/presets", nil)
	lResp := AssertV2Success(t, lw)
	items, _ := lResp["data"].([]interface{})
	if len(items) != 0 {
		t.Errorf("expected 0 presets after delete, got %d", len(items))
	}
}

// TestV2PresetDelete_OtherUser — user B cannot delete user A's preset (403).
func TestV2PresetDelete_OtherUser(t *testing.T) {
	ctx := setupPresetTestRouter(t)
	defer ctx.Cleanup()

	// normalUser creates a preset.
	createBody := map[string]interface{}{"name": "private-preset", "prompt": "secret"}
	cw := V2RequestAsUser(ctx, ctx.NormalUser, "POST", "/api/v2/test-slug/playground/presets", createBody, nil)
	AssertV2Status(t, cw, http.StatusCreated)

	createResp := ParseV2Response(t, cw)
	dataMap, _ := createResp["data"].(map[string]interface{})
	idFloat, _ := dataMap["id"].(float64)
	idStr := fmt.Sprintf("%d", int(idFloat))

	// adminUser attempts to delete normalUser's preset — must get 403.
	path := "/api/v2/test-slug/playground/presets/" + idStr
	w := V2RequestAsUser(ctx, ctx.AdminUser, "DELETE", path, nil, []string{"admin"})
	AssertV2Error(t, w, http.StatusForbidden)
}

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
	"net/http"
	"strconv"
	"strings"

	"github.com/LurusTech/lurus-hub/internal/adapter/middleware"
	"github.com/LurusTech/lurus-hub/internal/adapter/repo"
	"github.com/LurusTech/lurus-hub/internal/pkg/common"

	"github.com/gin-gonic/gin"
)

// playgroundPresetView is the whitelist projection returned to the client.
// Internal fields (TenantID, UserID) are deliberately excluded.
type playgroundPresetView struct {
	Id        uint   `json:"id"`
	Name      string `json:"name"`
	Prompt    string `json:"prompt"`
	Models    string `json:"models"`
	Params    string `json:"params"`
	CreatedAt int64  `json:"created_at"`
}

func toPresetView(p *repo.PlaygroundPreset) playgroundPresetView {
	return playgroundPresetView{
		Id:        p.Id,
		Name:      p.Name,
		Prompt:    p.Prompt,
		Models:    p.Models,
		Params:    p.Params,
		CreatedAt: p.CreatedAt.Unix(),
	}
}

// ListPresetsV2 returns the calling user's playground presets.
// Route: GET /api/v2/:tenant_slug/playground/presets
func ListPresetsV2(c *gin.Context) {
	tenantCtx, err := middleware.GetTenantContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success":    false,
			"message":    "Tenant context not found",
			"error_code": "UNAUTHENTICATED",
		})
		return
	}

	rows, err := repo.ListPlaygroundPresets(tenantCtx.TenantID, tenantCtx.UserID)
	if err != nil {
		common.SysError("ListPresetsV2: " + err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{
			"success":    false,
			"message":    "Failed to list presets",
			"error_code": "DB_ERROR",
		})
		return
	}

	views := make([]playgroundPresetView, 0, len(rows))
	for _, r := range rows {
		views = append(views, toPresetView(r))
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    views,
	})
}

// createPresetRequest is the accepted body for CreatePresetV2.
type createPresetRequest struct {
	Name   string `json:"name"   binding:"required"`
	Prompt string `json:"prompt"`
	Models string `json:"models"`
	Params string `json:"params"`
}

// CreatePresetV2 saves a new named preset for the current user.
// Route: POST /api/v2/:tenant_slug/playground/presets
// Returns 409 if a preset with the same name already exists for this user.
func CreatePresetV2(c *gin.Context) {
	tenantCtx, err := middleware.GetTenantContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success":    false,
			"message":    "Tenant context not found",
			"error_code": "UNAUTHENTICATED",
		})
		return
	}

	var req createPresetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success":    false,
			"message":    "Invalid request: " + err.Error(),
			"error_code": "INVALID_REQUEST",
		})
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success":    false,
			"message":    "Preset name cannot be empty",
			"error_code": "INVALID_REQUEST",
		})
		return
	}

	// Enforce name uniqueness per (tenant, user) at the application layer.
	exists, err := repo.PlaygroundPresetNameExists(tenantCtx.TenantID, tenantCtx.UserID, name)
	if err != nil {
		common.SysError("CreatePresetV2 name check: " + err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{
			"success":    false,
			"message":    "Failed to check preset name",
			"error_code": "DB_ERROR",
		})
		return
	}
	if exists {
		c.JSON(http.StatusConflict, gin.H{
			"success":    false,
			"message":    "A preset named \"" + name + "\" already exists",
			"error_code": "PRESET_NAME_CONFLICT",
		})
		return
	}

	preset := &repo.PlaygroundPreset{
		TenantID: tenantCtx.TenantID,
		UserID:   tenantCtx.UserID,
		Name:     name,
		Prompt:   req.Prompt,
		Models:   req.Models,
		Params:   req.Params,
	}
	if err := repo.CreatePlaygroundPreset(preset); err != nil {
		common.SysError("CreatePresetV2: " + err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{
			"success":    false,
			"message":    "Failed to create preset",
			"error_code": "DB_ERROR",
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data":    toPresetView(preset),
	})
}

// DeletePresetV2 deletes a preset owned by the calling user.
// Route: DELETE /api/v2/:tenant_slug/playground/presets/:id
// Returns 403 if the preset belongs to a different user.
func DeletePresetV2(c *gin.Context) {
	tenantCtx, err := middleware.GetTenantContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success":    false,
			"message":    "Tenant context not found",
			"error_code": "UNAUTHENTICATED",
		})
		return
	}

	idStr := c.Param("id")
	idU64, parseErr := strconv.ParseUint(idStr, 10, 64)
	if parseErr != nil || idU64 == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success":    false,
			"message":    "Invalid preset id",
			"error_code": "INVALID_ID",
		})
		return
	}
	id := uint(idU64)

	preset, err := repo.GetPlaygroundPreset(id)
	if err != nil {
		common.SysError("DeletePresetV2 fetch: " + err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{
			"success":    false,
			"message":    "Failed to fetch preset",
			"error_code": "DB_ERROR",
		})
		return
	}
	if preset == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success":    false,
			"message":    "Preset not found",
			"error_code": "NOT_FOUND",
		})
		return
	}

	// Owner check: tenant_id must match AND user_id must match.
	if preset.TenantID != tenantCtx.TenantID || preset.UserID != tenantCtx.UserID {
		c.JSON(http.StatusForbidden, gin.H{
			"success":    false,
			"message":    "Not allowed to delete this preset",
			"error_code": "FORBIDDEN",
		})
		return
	}

	if err := repo.DeletePlaygroundPreset(id); err != nil {
		common.SysError("DeletePresetV2: " + err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{
			"success":    false,
			"message":    "Failed to delete preset",
			"error_code": "DB_ERROR",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

package handler

import (
	"net/http"
	"strconv"

	"encoding/json"

	"github.com/LurusTech/lurus-hub/internal/adapter/middleware"
	"github.com/LurusTech/lurus-hub/internal/adapter/repo"
	"github.com/LurusTech/lurus-hub/internal/app"
	"github.com/LurusTech/lurus-hub/internal/app/governance"
	"github.com/LurusTech/lurus-hub/internal/pkg/common"

	"github.com/gin-gonic/gin"
)

// tokenView is the field-whitelisted projection returned by ListTokensV2. It
// excludes the raw Key (the bearer secret — must never appear in a list, only
// once on create), tenant_id (implicit from route) and the platform/provisioning
// linkage IDs (identity_account_id, creator_user_id). Mirrors redemptionView.
type tokenView struct {
	Id                 int     `json:"id"`
	Name               string  `json:"name"`
	Status             int     `json:"status"`
	CreatedTime        int64   `json:"created_time"`
	AccessedTime       int64   `json:"accessed_time"`
	ExpiredTime        int64   `json:"expired_time"`
	RemainQuota        int     `json:"remain_quota"`
	UsedQuota          int     `json:"used_quota"`
	UnlimitedQuota     bool    `json:"unlimited_quota"`
	ModelLimitsEnabled bool    `json:"model_limits_enabled"`
	ModelLimits        string  `json:"model_limits"`
	AllowIps           *string `json:"allow_ips"`
	Group              string  `json:"group"`
}

func toTokenViews(tokens []*repo.Token) []tokenView {
	items := make([]tokenView, 0, len(tokens))
	for _, t := range tokens {
		items = append(items, tokenView{
			Id:                 t.Id,
			Name:               t.Name,
			Status:             t.Status,
			CreatedTime:        t.CreatedTime,
			AccessedTime:       t.AccessedTime,
			ExpiredTime:        t.ExpiredTime,
			RemainQuota:        t.RemainQuota,
			UsedQuota:          t.UsedQuota,
			UnlimitedQuota:     t.UnlimitedQuota,
			ModelLimitsEnabled: t.ModelLimitsEnabled,
			ModelLimits:        t.ModelLimits,
			AllowIps:           t.AllowIps,
			Group:              t.Group,
		})
	}
	return items
}

// ListTokensV2 retrieves the current user's tokens (v2 API with tenant context)
// Route: GET /api/v2/:tenant_slug/tokens
func ListTokensV2(c *gin.Context) {
	// Get tenant context from middleware
	tenantCtx, err := middleware.GetTenantContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "Tenant context not found",
		})
		return
	}

	// Parse pagination parameters (match frontend: p=&size=)
	page, _ := strconv.Atoi(c.DefaultQuery("p", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("size", "20"))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	offset := (page - 1) * pageSize

	// Get tokens for the user (filtered by tenant_id implicitly through user ownership)
	tokens, err := repo.GetAllUserTokens(tenantCtx.UserID, offset, pageSize)
	if err != nil {
		common.SysError("Failed to get tokens: " + err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to retrieve tokens",
		})
		return
	}

	// Get total count
	total, _ := repo.CountUserTokens(tenantCtx.UserID)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"items":     toTokenViews(tokens),
			"total":     total,
			"page":      page,
			"page_size": pageSize,
		},
	})
}

// CreateTokenV2 creates a new token for the current user (v2 API with tenant context)
// Route: POST /api/v2/:tenant_slug/tokens
func CreateTokenV2(c *gin.Context) {
	// Get tenant context from middleware
	tenantCtx, err := middleware.GetTenantContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "Tenant context not found",
		})
		return
	}

	// Parse request body
	var req struct {
		Name               string `json:"name" binding:"required"`
		ExpiredTime        int64  `json:"expired_time"`        // -1 for never expires
		RemainQuota        int    `json:"remain_quota"`        // Initial quota
		UnlimitedQuota     bool   `json:"unlimited_quota"`     // Unlimited quota flag
		ModelLimitsEnabled bool   `json:"model_limits_enabled"` // Enable model limits
		ModelLimits        string `json:"model_limits"`         // JSON string of model limits
		AllowIps           string `json:"allow_ips"`            // Comma-separated allowed IPs
		Group              string `json:"group"`                // Token group
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request parameters",
			"error":   err.Error(),
		})
		return
	}

	// Validate token name
	if err := app.ValidateTokenName(req.Name); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	// Validate quota
	if err := app.ValidateTokenQuota(req.RemainQuota, req.UnlimitedQuota); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	// Generate token key
	key, err := app.GenerateTokenKey()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	// Set default values
	if req.ExpiredTime == 0 {
		req.ExpiredTime = -1 // Never expires
	}
	if req.Group == "" {
		req.Group = "default"
	}

	// Handle optional AllowIps field
	var allowIps *string
	if req.AllowIps != "" {
		allowIps = &req.AllowIps
	}

	// Create token with tenant context
	token := repo.Token{
		UserId:             tenantCtx.UserID,
		TenantId:           tenantCtx.TenantID,
		Name:               req.Name,
		Key:                key,
		CreatedTime:        common.GetTimestamp(),
		AccessedTime:       common.GetTimestamp(),
		ExpiredTime:        req.ExpiredTime,
		RemainQuota:        req.RemainQuota,
		UnlimitedQuota:     req.UnlimitedQuota,
		ModelLimitsEnabled: req.ModelLimitsEnabled,
		ModelLimits:        req.ModelLimits,
		AllowIps:           allowIps,
		Group:              req.Group,
	}

	err = token.Insert()
	if err != nil {
		common.SysError("Failed to create token: " + err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to create token",
		})
		return
	}
	detailBytes, _ := json.Marshal(map[string]string{"name": token.Name})
	governance.RecordAuditEvent(governance.NewAuditEvent(c, governance.ActorUser, tenantCtx.UserID,
		governance.ActionTokenCreated, governance.ResourceToken, token.Id, string(detailBytes)))

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"message": "Token created successfully",
		"data": gin.H{
			"id":   token.Id,
			"name": token.Name,
			"key":  "sk-" + token.Key, // Return full key only on creation
		},
	})
}

// UpdateTokenV2 updates a token (v2 API with tenant context)
// Route: PUT /api/v2/:tenant_slug/tokens/:id
func UpdateTokenV2(c *gin.Context) {
	// Get tenant context from middleware
	tenantCtx, err := middleware.GetTenantContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "Tenant context not found",
		})
		return
	}

	// Get token ID from URL
	tokenID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid token ID",
		})
		return
	}

	// Get existing token (ensures ownership)
	token, err := repo.GetTokenByIds(tokenID, tenantCtx.UserID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "Token not found",
		})
		return
	}

	// Verify tenant ownership
	if token.TenantId != tenantCtx.TenantID {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": "Access denied",
		})
		return
	}

	// Parse request body
	var req struct {
		Name               string `json:"name"`
		ExpiredTime        int64  `json:"expired_time"`
		RemainQuota        int    `json:"remain_quota"`
		UnlimitedQuota     *bool  `json:"unlimited_quota"`
		ModelLimitsEnabled *bool  `json:"model_limits_enabled"`
		ModelLimits        string `json:"model_limits"`
		AllowIps           string `json:"allow_ips"`
		Group              string `json:"group"`
		Status             *int   `json:"status"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request parameters",
			"error":   err.Error(),
		})
		return
	}

	// Update fields if provided
	if req.Name != "" {
		if err := app.ValidateTokenName(req.Name); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
		token.Name = req.Name
	}

	if req.ExpiredTime != 0 {
		token.ExpiredTime = req.ExpiredTime
	}

	if req.UnlimitedQuota != nil {
		token.UnlimitedQuota = *req.UnlimitedQuota
	}

	if !token.UnlimitedQuota && req.RemainQuota != 0 {
		if req.RemainQuota < 0 {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "Quota value cannot be negative",
			})
			return
		}
		token.RemainQuota = req.RemainQuota
	}

	if req.ModelLimitsEnabled != nil {
		token.ModelLimitsEnabled = *req.ModelLimitsEnabled
	}

	if req.ModelLimits != "" {
		token.ModelLimits = req.ModelLimits
	}

	if req.AllowIps != "" {
		token.AllowIps = &req.AllowIps
	}

	if req.Group != "" {
		token.Group = req.Group
	}

	if req.Status != nil {
		if *req.Status == common.TokenStatusEnabled {
			if err := app.CanEnableToken(token); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{
					"success": false,
					"message": err.Error(),
				})
				return
			}
		}
		token.Status = *req.Status
	}

	// Save changes
	err = token.Update()
	if err != nil {
		common.SysError("Failed to update token: " + err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to update token",
		})
		return
	}

	governance.RecordAuditEvent(governance.NewAuditEvent(c, governance.ActorUser, tenantCtx.UserID,
		governance.ActionTokenUpdated, governance.ResourceToken, tokenID, ""))
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Token updated successfully",
		"data":    token,
	})
}

// DeleteTokenV2 deletes a token (v2 API with tenant context)
// Route: DELETE /api/v2/:tenant_slug/tokens/:id
func DeleteTokenV2(c *gin.Context) {
	// Get tenant context from middleware
	tenantCtx, err := middleware.GetTenantContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "Tenant context not found",
		})
		return
	}

	// Get token ID from URL
	tokenID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid token ID",
		})
		return
	}

	// Verify ownership before deletion
	token, err := repo.GetTokenByIds(tokenID, tenantCtx.UserID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "Token not found",
		})
		return
	}

	// Verify tenant ownership
	if token.TenantId != tenantCtx.TenantID {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": "Access denied",
		})
		return
	}

	// Delete token
	err = repo.DeleteTokenById(tokenID, tenantCtx.UserID)
	if err != nil {
		common.SysError("Failed to delete token: " + err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to delete token",
		})
		return
	}
	governance.RecordAuditEvent(governance.NewAuditEvent(c, governance.ActorUser, tenantCtx.UserID,
		governance.ActionTokenDeleted, governance.ResourceToken, tokenID, ""))

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Token deleted successfully",
	})
}

// RotateTokenV2 generates a new key for an existing token, invalidating the old one.
// Route: POST /api/v2/:tenant_slug/tokens/:id/rotate
func RotateTokenV2(c *gin.Context) {
	tenantCtx, err := middleware.GetTenantContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "Tenant context not found"})
		return
	}

	tokenID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid token ID"})
		return
	}

	token, err := repo.GetTokenByIds(tokenID, tenantCtx.UserID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "Token not found"})
		return
	}

	if token.TenantId != tenantCtx.TenantID {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "Access denied"})
		return
	}

	newKey, err := app.GenerateTokenKey()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to generate new key"})
		return
	}

	if err = token.RotateKey(newKey); err != nil {
		common.SysError("Failed to rotate token key: " + err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to rotate token key"})
		return
	}

	governance.RecordAuditEvent(governance.NewAuditEvent(c, governance.ActorUser, tenantCtx.UserID,
		governance.ActionTokenUpdated, governance.ResourceToken, tokenID, `{"action":"rotate"}`))

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Token key rotated successfully",
		"data": gin.H{
			"id":  token.Id,
			"key": "sk-" + newKey,
		},
	})
}

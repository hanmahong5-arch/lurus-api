package controller

import (
	"net/http"
	"strconv"

	"github.com/QuantumNous/lurus-api/internal/pkg/common"
	"github.com/QuantumNous/lurus-api/internal/pkg/logger"
	"github.com/QuantumNous/lurus-api/internal/data/model"
	"github.com/gin-gonic/gin"
)

// ===== User APIs =====

// InternalGetUser gets user info by ID
// GET /internal/user/:id
func InternalGetUser(c *gin.Context) {
	userId, err := strconv.Atoi(c.Param("id"))
	if err != nil || userId <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid user ID",
		})
		return
	}

	user, err := model.GetUserById(userId, false)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success":    false,
			"message":    "User not found",
			"error_code": "USER_NOT_FOUND",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"id":             user.Id,
			"username":       user.Username,
			"display_name":   user.DisplayName,
			"email":          user.Email,
			"phone":          user.Phone,
			"phone_verified": user.PhoneVerified,
			"role":           user.Role,
			"status":         user.Status,
			"group":          user.Group,
		},
	})
}

// InternalGetUserByEmail gets user by email
// GET /internal/user/by-email/:email
func InternalGetUserByEmail(c *gin.Context) {
	email := c.Param("email")
	if email == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Email is required",
		})
		return
	}

	user := &model.User{Email: email}
	if err := user.FillUserByEmail(); err != nil || user.Id == 0 {
		c.JSON(http.StatusNotFound, gin.H{
			"success":    false,
			"message":    "User not found",
			"error_code": "USER_NOT_FOUND",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"id":             user.Id,
			"username":       user.Username,
			"display_name":   user.DisplayName,
			"email":          user.Email,
			"phone":          user.Phone,
			"phone_verified": user.PhoneVerified,
			"role":           user.Role,
			"status":         user.Status,
			"group":          user.Group,
		},
	})
}

// InternalGetUserByPhone gets user by phone
// GET /internal/user/by-phone/:phone
func InternalGetUserByPhone(c *gin.Context) {
	phone := c.Param("phone")
	if phone == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Phone is required",
		})
		return
	}

	user, err := model.GetUserByPhone(phone)
	if err != nil || user == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success":    false,
			"message":    "User not found",
			"error_code": "USER_NOT_FOUND",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"id":             user.Id,
			"username":       user.Username,
			"display_name":   user.DisplayName,
			"email":          user.Email,
			"phone":          user.Phone,
			"phone_verified": user.PhoneVerified,
			"role":           user.Role,
			"status":         user.Status,
			"group":          user.Group,
		},
	})
}

// InternalUpdateUser updates user information
// PUT /internal/user/:id
func InternalUpdateUser(c *gin.Context) {
	userId, err := strconv.Atoi(c.Param("id"))
	if err != nil || userId <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid user ID",
		})
		return
	}

	var req struct {
		DisplayName *string `json:"display_name"`
		Email       *string `json:"email"`
		Phone       *string `json:"phone"`
		Status      *int    `json:"status"`
		Group       *string `json:"group"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request: " + err.Error(),
		})
		return
	}

	// Check user exists
	_, err = model.GetUserById(userId, false)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success":    false,
			"message":    "User not found",
			"error_code": "USER_NOT_FOUND",
		})
		return
	}

	// Build updates map
	updates := make(map[string]interface{})
	if req.DisplayName != nil {
		updates["display_name"] = *req.DisplayName
	}
	if req.Email != nil {
		updates["email"] = *req.Email
	}
	if req.Phone != nil {
		updates["phone"] = *req.Phone
		updates["phone_verified"] = true // Internal API sets phone as verified
	}
	if req.Status != nil {
		updates["status"] = *req.Status
	}
	if req.Group != nil {
		updates["group"] = *req.Group
	}

	if len(updates) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "No fields to update",
		})
		return
	}

	// Perform update
	err = model.DB.Model(&model.User{}).Where("id = ?", userId).Updates(updates).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to update user: " + err.Error(),
		})
		return
	}

	// Log the operation
	keyName := c.GetString("internal_api_key_name")
	common.SysLog("Internal API updated user " + strconv.Itoa(userId) + " via key: " + keyName)

	// Return updated user info
	updatedUser, _ := model.GetUserById(userId, false)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "User updated successfully",
		"data": gin.H{
			"id":             updatedUser.Id,
			"username":       updatedUser.Username,
			"display_name":   updatedUser.DisplayName,
			"email":          updatedUser.Email,
			"phone":          updatedUser.Phone,
			"phone_verified": updatedUser.PhoneVerified,
			"role":           updatedUser.Role,
			"status":         updatedUser.Status,
			"group":          updatedUser.Group,
		},
	})
}

// ===== Subscription APIs =====

// InternalGetUserSubscription gets user's active subscription
// GET /internal/subscription/user/:id
func InternalGetUserSubscription(c *gin.Context) {
	userId, err := strconv.Atoi(c.Param("id"))
	if err != nil || userId <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid user ID",
		})
		return
	}

	sub, err := model.GetActiveSubscriptionByUserId(userId)
	if err != nil || sub == nil {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data": gin.H{
				"subscription": nil,
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"id":           sub.Id,
			"plan_code":    sub.PlanCode,
			"plan_name":    sub.PlanName,
			"status":       sub.Status,
			"daily_quota":  sub.DailyQuota,
			"total_quota":  sub.TotalQuota,
			"started_at":   sub.StartedAt,
			"expires_at":   sub.ExpiresAt,
			"base_group":   sub.BaseGroup,
		},
	})
}

// InternalGrantSubscription grants subscription to a user
// POST /internal/subscription/grant
func InternalGrantSubscription(c *gin.Context) {
	var req struct {
		UserId   int    `json:"user_id" binding:"required"`
		PlanCode string `json:"plan_code" binding:"required"`
		Days     int    `json:"days" binding:"required"`
		Reason   string `json:"reason"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request: " + err.Error(),
		})
		return
	}

	// Validate user ID and days
	if req.UserId <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid user ID",
		})
		return
	}

	if req.Days <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Days must be positive",
		})
		return
	}

	// Get the plan
	plan := model.GetSubscriptionPlanByCode(req.PlanCode)
	if plan == nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid plan code",
		})
		return
	}

	// Create subscription
	sub, err := model.CreateInternalSubscription(req.UserId, plan, req.Days, req.Reason)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to create subscription: " + err.Error(),
		})
		return
	}

	// Get API key name from context for logging
	keyName := c.GetString("internal_api_key_name")
	common.SysLog("Internal API granted subscription to user " + strconv.Itoa(req.UserId) + " via key: " + keyName)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Subscription granted successfully",
		"data":    sub,
	})
}

// ===== Quota APIs =====

// InternalGetUserQuota gets user's quota information
// GET /internal/quota/user/:id
func InternalGetUserQuota(c *gin.Context) {
	userId, err := strconv.Atoi(c.Param("id"))
	if err != nil || userId <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid user ID",
		})
		return
	}

	user, err := model.GetUserById(userId, false)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success":    false,
			"message":    "User not found",
			"error_code": "USER_NOT_FOUND",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"user_id":          user.Id,
			"quota":            user.Quota,
			"used_quota":       user.UsedQuota,
			"daily_quota":      user.DailyQuota,
			"daily_used":       user.DailyUsed,
			"last_daily_reset": user.LastDailyReset,
			"group":            user.Group,
			"base_group":       user.BaseGroup,
			"fallback_group":   user.FallbackGroup,
		},
	})
}

// InternalAdjustQuota adjusts user's quota
// POST /internal/quota/adjust
func InternalAdjustQuota(c *gin.Context) {
	var req struct {
		UserId int    `json:"user_id" binding:"required"`
		Amount int    `json:"amount" binding:"required"` // Positive = add, Negative = deduct
		Reason string `json:"reason" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request: " + err.Error(),
		})
		return
	}

	// Validate user ID is positive
	if req.UserId <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid user ID",
		})
		return
	}

	// Check user exists
	user, err := model.GetUserById(req.UserId, false)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success":    false,
			"message":    "User not found",
			"error_code": "USER_NOT_FOUND",
		})
		return
	}

	// Adjust quota
	if req.Amount > 0 {
		err = model.IncreaseUserQuota(req.UserId, req.Amount, true)
	} else if req.Amount < 0 {
		err = model.DecreaseUserQuota(req.UserId, -req.Amount)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to adjust quota: " + err.Error(),
		})
		return
	}

	// Log the operation
	keyName := c.GetString("internal_api_key_name")
	model.RecordLog(req.UserId, model.LogTypeSystem,
		"Internal API adjusted quota by "+logger.LogQuota(req.Amount)+" via key: "+keyName+". Reason: "+req.Reason)

	// Get updated quota
	newQuota, _ := model.GetUserQuota(req.UserId, true)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Quota adjusted successfully",
		"data": gin.H{
			"user_id":      user.Id,
			"old_quota":    user.Quota,
			"adjustment":   req.Amount,
			"new_quota":    newQuota,
		},
	})
}

// ===== Balance APIs =====

// InternalGetUserBalance gets user's balance
// GET /internal/balance/user/:id
func InternalGetUserBalance(c *gin.Context) {
	userId, err := strconv.Atoi(c.Param("id"))
	if err != nil || userId <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid user ID",
		})
		return
	}

	user, err := model.GetUserById(userId, false)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success":    false,
			"message":    "User not found",
			"error_code": "USER_NOT_FOUND",
		})
		return
	}

	// Convert quota to RMB
	balanceRmb := float64(user.Quota) / common.QuotaPerUnit

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"user_id":     user.Id,
			"balance":     user.Quota,      // Balance in tokens
			"balance_rmb": balanceRmb,      // Balance in RMB
			"used_quota":  user.UsedQuota,
		},
	})
}

// InternalTopupBalance tops up user's balance
// POST /internal/balance/topup
func InternalTopupBalance(c *gin.Context) {
	var req struct {
		UserId    int     `json:"user_id" binding:"required"`
		AmountRmb float64 `json:"amount_rmb" binding:"required"`
		OrderId   string  `json:"order_id"`
		Reason    string  `json:"reason" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request: " + err.Error(),
		})
		return
	}

	if req.UserId <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid user ID",
		})
		return
	}

	if req.AmountRmb <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Amount must be positive",
		})
		return
	}

	// Check user exists
	user, err := model.GetUserById(req.UserId, false)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success":    false,
			"message":    "User not found",
			"error_code": "USER_NOT_FOUND",
		})
		return
	}

	// Convert RMB to tokens
	quotaAmount := int(req.AmountRmb * common.QuotaPerUnit)

	// Add quota
	err = model.IncreaseUserQuota(req.UserId, quotaAmount, true)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to top up: " + err.Error(),
		})
		return
	}

	// Log the operation
	keyName := c.GetString("internal_api_key_name")
	logMsg := "Internal API topped up " + logger.LogQuota(quotaAmount) + " via key: " + keyName + ". Reason: " + req.Reason
	if req.OrderId != "" {
		logMsg += ". Order ID: " + req.OrderId
	}
	model.RecordLog(req.UserId, model.LogTypeTopup, logMsg)

	// Get updated quota
	newQuota, _ := model.GetUserQuota(req.UserId, true)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Balance topped up successfully",
		"data": gin.H{
			"user_id":     user.Id,
			"old_balance": user.Quota,
			"amount":      quotaAmount,
			"amount_rmb":  req.AmountRmb,
			"new_balance": newQuota,
		},
	})
}

// ===== API Key Management (Admin) =====

// AdminListApiKeys lists all internal API keys
// GET /api/admin/api-keys
func AdminListApiKeys(c *gin.Context) {
	keys, err := model.GetAllInternalApiKeys()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to get API keys: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    keys,
	})
}

// AdminGetApiKeyScopes returns available scopes
// GET /api/admin/api-keys/scopes
func AdminGetApiKeyScopes(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    model.GetAvailableScopes(),
	})
}

// AdminCreateApiKey creates a new internal API key
// POST /api/admin/api-keys
func AdminCreateApiKey(c *gin.Context) {
	var req struct {
		Name        string   `json:"name" binding:"required"`
		Scopes      []string `json:"scopes" binding:"required"`
		ExpiresAt   int64    `json:"expires_at"` // 0 = never
		Description string   `json:"description"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request: " + err.Error(),
		})
		return
	}

	adminId := c.GetInt("id")

	// Only root can create keys with wildcard scope
	for _, scope := range req.Scopes {
		if scope == model.ScopeAll {
			userRole := c.GetInt("role")
			if userRole != common.RoleRootUser {
				c.JSON(http.StatusForbidden, gin.H{
					"success": false,
					"message": "Only root user can create keys with full access",
				})
				return
			}
			break
		}
	}

	key, apiKey, err := model.CreateInternalApiKey(req.Name, req.Scopes, adminId, req.ExpiresAt, req.Description)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to create API key: " + err.Error(),
		})
		return
	}

	// IMPORTANT: Only return the full key ONCE during creation
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "API key created successfully. Please save the key now - it won't be shown again!",
		"data": gin.H{
			"key":      key, // Full key - only shown once
			"key_info": apiKey,
		},
	})
}

// AdminDeleteApiKey deletes an API key
// DELETE /api/admin/api-keys/:id
func AdminDeleteApiKey(c *gin.Context) {
	keyId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid key ID",
		})
		return
	}

	err = model.DeleteInternalApiKey(keyId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to delete API key: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "API key deleted successfully",
	})
}

// AdminToggleApiKey enables/disables an API key
// PUT /api/admin/api-keys/:id/toggle
func AdminToggleApiKey(c *gin.Context) {
	keyId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid key ID",
		})
		return
	}

	err = model.ToggleInternalApiKey(keyId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to toggle API key: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "API key toggled successfully",
	})
}

// AdminUpdateApiKey updates an API key
// PUT /api/admin/api-keys/:id
func AdminUpdateApiKey(c *gin.Context) {
	keyId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid key ID",
		})
		return
	}

	var req struct {
		Name        string   `json:"name" binding:"required"`
		Scopes      []string `json:"scopes" binding:"required"`
		ExpiresAt   int64    `json:"expires_at"`
		Description string   `json:"description"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request: " + err.Error(),
		})
		return
	}

	// Only root can update keys with wildcard scope
	for _, scope := range req.Scopes {
		if scope == model.ScopeAll {
			userRole := c.GetInt("role")
			if userRole != common.RoleRootUser {
				c.JSON(http.StatusForbidden, gin.H{
					"success": false,
					"message": "Only root user can assign full access",
				})
				return
			}
			break
		}
	}

	err = model.UpdateInternalApiKey(keyId, req.Name, req.Scopes, req.ExpiresAt, req.Description)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to update API key: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "API key updated successfully",
	})
}

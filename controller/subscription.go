package controller

import (
	"net/http"
	"strconv"
	"time"

	"github.com/QuantumNous/lurus-api/common"
	"github.com/QuantumNous/lurus-api/model"
	"github.com/gin-gonic/gin"
)

// GetSubscriptionPlans returns all available subscription plans
// GET /api/subscription/plans
func GetSubscriptionPlans(c *gin.Context) {
	plans := model.GetSubscriptionPlans()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    plans,
	})
}

// GetCurrentSubscription returns current user's active subscription
// GET /api/subscription/current
func GetCurrentSubscription(c *gin.Context) {
	userId := c.GetInt("id")
	if userId == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "Unauthorized",
		})
		return
	}

	sub, err := model.GetActiveSubscription(userId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to get subscription: " + err.Error(),
		})
		return
	}

	// Get daily quota info
	quotaInfo, err := model.GetUserDailyQuotaInfo(userId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to get quota info: " + err.Error(),
		})
		return
	}

	response := gin.H{
		"subscription": sub,
		"quota":        quotaInfo,
		"has_active":   sub != nil,
	}

	if sub != nil {
		// Calculate days remaining
		daysRemaining := int(time.Until(sub.ExpiresAt).Hours() / 24)
		if daysRemaining < 0 {
			daysRemaining = 0
		}
		response["days_remaining"] = daysRemaining
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    response,
	})
}

// GetSubscriptionHistory returns user's subscription history
// GET /api/subscription/history
func GetSubscriptionHistory(c *gin.Context) {
	userId := c.GetInt("id")
	if userId == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "Unauthorized",
		})
		return
	}

	limitStr := c.DefaultQuery("limit", "10")
	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 || limit > 100 {
		limit = 10
	}

	subs, err := model.GetUserSubscriptions(userId, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to get subscription history: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    subs,
	})
}

// CreateSubscriptionRequest represents the request to create a subscription
type CreateSubscriptionRequest struct {
	PlanCode      string `json:"plan_code" binding:"required"`
	PaymentMethod string `json:"payment_method" binding:"required"` // stripe/epay/creem
	AutoRenew     bool   `json:"auto_renew"`
}

// CreateSubscription creates a new subscription order (pending payment)
// POST /api/subscription/create
func CreateSubscription(c *gin.Context) {
	userId := c.GetInt("id")
	if userId == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "Unauthorized",
		})
		return
	}

	var req CreateSubscriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request: " + err.Error(),
		})
		return
	}

	// Validate payment method
	validMethods := map[string]bool{"stripe": true, "creem": true, "epay": true}
	if !validMethods[req.PaymentMethod] {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid payment method. Supported: stripe, creem, epay",
		})
		return
	}

	// Get plan
	plan := model.GetSubscriptionPlanByCode(req.PlanCode)
	if plan == nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid plan code",
		})
		return
	}

	if !plan.Enabled {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "This plan is currently unavailable",
		})
		return
	}

	// Edge case: Check if user already has a pending subscription
	// Limit to 1 pending subscription per user to prevent abuse
	pendingCount, err := model.GetUserPendingSubscriptionCount(userId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to check pending subscriptions",
		})
		return
	}
	if pendingCount > 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "You already have a pending subscription. Please complete or cancel it first.",
		})
		return
	}

	// Check if user already has an active subscription
	existingSub, err := model.GetActiveSubscription(userId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to check existing subscription",
		})
		return
	}

	// Calculate start and end time
	startTime := time.Now()
	if existingSub != nil {
		// If user has active subscription, new one starts after current expires
		startTime = existingSub.ExpiresAt
	}
	expiresAt := startTime.AddDate(0, 0, plan.Days)

	// Create subscription record
	sub := &model.Subscription{
		UserId:        userId,
		PlanCode:      plan.Code,
		PlanName:      plan.Name,
		Status:        model.SubscriptionStatusPending,
		DailyQuota:    plan.DailyQuota,
		TotalQuota:    plan.TotalQuota,
		BaseGroup:     plan.BaseGroup,
		FallbackGroup: plan.FallbackGroup,
		StartedAt:     startTime,
		ExpiresAt:     expiresAt,
		PaymentMethod: req.PaymentMethod,
		Amount:        plan.Price,
		Currency:      plan.Currency,
		AutoRenew:     req.AutoRenew,
	}

	if err := model.CreateSubscription(sub); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to create subscription: " + err.Error(),
		})
		return
	}

	// Generate payment URL based on payment method
	paymentInfo := gin.H{
		"subscription_id": sub.Id,
		"amount":          plan.Price,
		"currency":        plan.Currency,
		"plan_name":       plan.Name,
	}

	// Payment URL will be generated by the specific payment handler
	// Here we just return the subscription info for the frontend to proceed

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Subscription created, please proceed to payment",
		"data": gin.H{
			"subscription": sub,
			"payment":      paymentInfo,
		},
	})
}

// CancelSubscriptionRenewal cancels auto-renewal for current subscription
// POST /api/subscription/cancel
func CancelSubscriptionRenewal(c *gin.Context) {
	userId := c.GetInt("id")
	if userId == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "Unauthorized",
		})
		return
	}

	sub, err := model.GetActiveSubscription(userId)
	if err != nil || sub == nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "No active subscription found",
		})
		return
	}

	if err := model.CancelSubscription(sub.Id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to cancel subscription: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Subscription auto-renewal cancelled",
	})
}

// ActivateSubscriptionByPayment activates subscription after payment confirmation
// Called by payment webhook handlers
func ActivateSubscriptionByPayment(paymentId string) error {
	sub, err := model.GetSubscriptionByPaymentId(paymentId)
	if err != nil {
		return err
	}

	return model.ActivateSubscription(sub)
}

// ActivateSubscriptionById activates subscription by ID (for admin or internal use)
func ActivateSubscriptionById(subscriptionId int) error {
	sub, err := model.GetSubscriptionById(subscriptionId)
	if err != nil {
		return err
	}

	return model.ActivateSubscription(sub)
}

// === Admin APIs ===

// AdminGetAllSubscriptions returns all subscriptions (admin only)
// GET /api/subscription/admin/all
func AdminGetAllSubscriptions(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	status := c.Query("status")
	userId := c.Query("user_id")

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	offset := (page - 1) * pageSize

	query := model.DB.Model(&model.Subscription{})
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if userId != "" {
		uid, _ := strconv.Atoi(userId)
		if uid > 0 {
			query = query.Where("user_id = ?", uid)
		}
	}

	var total int64
	query.Count(&total)

	var subs []model.Subscription
	if err := query.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&subs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to get subscriptions: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"subscriptions": subs,
			"total":         total,
			"page":          page,
			"page_size":     pageSize,
		},
	})
}

// AdminUpdateSubscriptionPlans updates subscription plans configuration
// PUT /api/subscription/admin/plans
func AdminUpdateSubscriptionPlans(c *gin.Context) {
	var plans []model.SubscriptionPlan
	if err := c.ShouldBindJSON(&plans); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request: " + err.Error(),
		})
		return
	}

	if err := model.UpdateSubscriptionPlans(plans); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to update plans: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Subscription plans updated",
	})
}

// AdminActivateSubscription manually activates a subscription
// POST /api/subscription/admin/:id/activate
func AdminActivateSubscription(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid subscription ID",
		})
		return
	}

	if err := ActivateSubscriptionById(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to activate subscription: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Subscription activated",
	})
}

// AdminExpireSubscription manually expires a subscription
// POST /api/subscription/admin/:id/expire
func AdminExpireSubscription(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid subscription ID",
		})
		return
	}

	sub, err := model.GetSubscriptionById(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "Subscription not found",
		})
		return
	}

	if err := model.ExpireSubscription(sub); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to expire subscription: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Subscription expired",
	})
}

// AdminCreateSubscription creates subscription for user (admin grant)
// POST /api/subscription/admin/grant
func AdminCreateSubscription(c *gin.Context) {
	var req struct {
		UserId    int    `json:"user_id" binding:"required"`
		PlanCode  string `json:"plan_code" binding:"required"`
		Days      int    `json:"days"` // Override plan days if provided
		AutoRenew bool   `json:"auto_renew"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request: " + err.Error(),
		})
		return
	}

	plan := model.GetSubscriptionPlanByCode(req.PlanCode)
	if plan == nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid plan code",
		})
		return
	}

	days := plan.Days
	if req.Days > 0 {
		days = req.Days
	}

	// Check existing subscription
	existingSub, _ := model.GetActiveSubscription(req.UserId)
	startTime := time.Now()
	if existingSub != nil {
		startTime = existingSub.ExpiresAt
	}
	expiresAt := startTime.AddDate(0, 0, days)

	sub := &model.Subscription{
		UserId:        req.UserId,
		PlanCode:      plan.Code,
		PlanName:      plan.Name,
		Status:        model.SubscriptionStatusActive,
		DailyQuota:    plan.DailyQuota,
		TotalQuota:    plan.TotalQuota,
		BaseGroup:     plan.BaseGroup,
		FallbackGroup: plan.FallbackGroup,
		StartedAt:     startTime,
		ExpiresAt:     expiresAt,
		PaymentMethod: "admin_grant",
		Amount:        0,
		Currency:      plan.Currency,
		AutoRenew:     req.AutoRenew,
	}

	if err := model.CreateSubscription(sub); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to create subscription: " + err.Error(),
		})
		return
	}

	// Activate immediately for admin grants
	if err := model.ActivateSubscription(sub); err != nil {
		common.SysLog("Failed to activate admin-granted subscription: " + err.Error())
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Subscription granted",
		"data":    sub,
	})
}

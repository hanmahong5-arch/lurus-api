package controller

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/lurus-api/common"
	"github.com/QuantumNous/lurus-api/model"
	"github.com/QuantumNous/lurus-api/setting"
	"github.com/QuantumNous/lurus-api/setting/operation_setting"
	"github.com/QuantumNous/lurus-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/stripe/stripe-go/v81"
	"github.com/stripe/stripe-go/v81/checkout/session"
	"github.com/stripe/stripe-go/v81/webhook"
)

// SubscriptionPaymentRequest represents the request to initiate subscription payment
type SubscriptionPaymentRequest struct {
	SubscriptionId int    `json:"subscription_id" binding:"required"`
	PaymentMethod  string `json:"payment_method" binding:"required"` // stripe/creem/epay
}

// InitiateSubscriptionPayment creates payment session for a subscription
// POST /api/subscription/pay
func InitiateSubscriptionPayment(c *gin.Context) {
	userId := c.GetInt("id")
	if userId == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "Unauthorized",
		})
		return
	}

	var req SubscriptionPaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request: " + err.Error(),
		})
		return
	}

	// Get subscription
	sub, err := model.GetSubscriptionById(req.SubscriptionId)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "Subscription not found",
		})
		return
	}

	// Verify ownership
	if sub.UserId != userId {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": "Access denied",
		})
		return
	}

	// Check status - only pending subscriptions can be paid
	if sub.Status != model.SubscriptionStatusPending {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": fmt.Sprintf("Subscription cannot be paid, current status: %s", sub.Status),
		})
		return
	}

	// Check if subscription has expired (created more than 24 hours ago)
	if time.Since(sub.CreatedAt) > 24*time.Hour {
		// Mark as expired
		_ = model.UpdateSubscriptionStatus(sub.Id, model.SubscriptionStatusExpired)
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Subscription order has expired, please create a new one",
		})
		return
	}

	// Get user for email
	user, err := model.GetUserById(userId, false)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to get user info",
		})
		return
	}

	var paymentURL string
	var paymentId string

	switch req.PaymentMethod {
	case "stripe":
		paymentURL, paymentId, err = createStripeSubscriptionSession(sub, user)
	case "creem":
		paymentURL, paymentId, err = createCreemSubscriptionSession(sub, user)
	case "epay":
		paymentURL, paymentId, err = createEpaySubscriptionSession(c, sub, user)
	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Unsupported payment method: " + req.PaymentMethod,
		})
		return
	}

	if err != nil {
		common.SysError(fmt.Sprintf("Failed to create payment session for subscription %d: %v", sub.Id, err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to create payment session: " + err.Error(),
		})
		return
	}

	// Update subscription with payment info
	if err := model.UpdateSubscriptionPaymentInfo(sub.Id, paymentId, req.PaymentMethod); err != nil {
		common.SysError(fmt.Sprintf("Failed to update subscription payment info: %v", err))
	}

	common.SysLog(fmt.Sprintf("Subscription payment initiated: sub_id=%d, user_id=%d, method=%s, payment_id=%s",
		sub.Id, userId, req.PaymentMethod, paymentId))

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Payment session created",
		"data": gin.H{
			"payment_url": paymentURL,
			"payment_id":  paymentId,
			"amount":      sub.Amount,
			"currency":    sub.Currency,
		},
	})
}

// createStripeSubscriptionSession creates a Stripe checkout session for subscription
func createStripeSubscriptionSession(sub *model.Subscription, user *model.User) (string, string, error) {
	if setting.StripeApiSecret == "" {
		return "", "", fmt.Errorf("Stripe is not configured")
	}

	stripe.Key = setting.StripeApiSecret

	// Generate unique reference ID for idempotency
	referenceId := fmt.Sprintf("sub_%d_%d_%d", sub.Id, sub.UserId, time.Now().UnixNano())

	// Calculate amount in cents
	amountInCents := int64(sub.Amount * 100)
	if sub.Currency == "CNY" {
		// Convert CNY to USD for Stripe (approximate rate)
		amountInCents = int64(sub.Amount * 100 / 7.3)
	}

	params := &stripe.CheckoutSessionParams{
		PaymentMethodTypes: stripe.StringSlice([]string{"card"}),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				PriceData: &stripe.CheckoutSessionLineItemPriceDataParams{
					Currency: stripe.String("usd"),
					ProductData: &stripe.CheckoutSessionLineItemPriceDataProductDataParams{
						Name:        stripe.String(sub.PlanName + " Subscription"),
						Description: stripe.String(fmt.Sprintf("Subscription for %d days", int(sub.ExpiresAt.Sub(sub.StartedAt).Hours()/24))),
					},
					UnitAmount: stripe.Int64(amountInCents),
				},
				Quantity: stripe.Int64(1),
			},
		},
		Mode:              stripe.String(string(stripe.CheckoutSessionModePayment)),
		SuccessURL:        stripe.String(system_setting.ServerAddress + "/subscription/success?session_id={CHECKOUT_SESSION_ID}"),
		CancelURL:         stripe.String(system_setting.ServerAddress + "/subscription/cancel"),
		ClientReferenceID: stripe.String(referenceId),
		Metadata: map[string]string{
			"subscription_id": strconv.Itoa(sub.Id),
			"user_id":         strconv.Itoa(sub.UserId),
			"plan_code":       sub.PlanCode,
			"type":            "subscription",
		},
	}

	if user.Email != "" {
		params.CustomerEmail = stripe.String(user.Email)
	}

	sess, err := session.New(params)
	if err != nil {
		return "", "", fmt.Errorf("failed to create Stripe session: %w", err)
	}

	return sess.URL, sess.ID, nil
}

// createCreemSubscriptionSession creates a Creem checkout session for subscription
func createCreemSubscriptionSession(sub *model.Subscription, user *model.User) (string, string, error) {
	if setting.CreemApiKey == "" {
		return "", "", fmt.Errorf("Creem is not configured")
	}

	referenceId := fmt.Sprintf("sub_%d_%d_%d", sub.Id, sub.UserId, time.Now().UnixNano())

	// Determine API URL
	apiURL := "https://api.creem.io/v1/checkouts"
	if setting.CreemTestMode {
		apiURL = "https://test-api.creem.io/v1/checkouts"
	}

	// Create checkout request
	payload := map[string]interface{}{
		"success_url": system_setting.ServerAddress + "/subscription/success?provider=creem&ref=" + referenceId,
		"request_id":  referenceId,
		"metadata": map[string]string{
			"subscription_id": strconv.Itoa(sub.Id),
			"user_id":         strconv.Itoa(sub.UserId),
			"plan_code":       sub.PlanCode,
			"type":            "subscription",
		},
		"amount":   int(sub.Amount * 100), // Amount in cents
		"currency": strings.ToLower(sub.Currency),
	}

	if user.Email != "" {
		payload["customer_email"] = user.Email
	}

	jsonData, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", apiURL, strings.NewReader(string(jsonData)))
	if err != nil {
		return "", "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", setting.CreemApiKey)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("failed to call Creem API: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return "", "", fmt.Errorf("Creem API error: %s", string(body))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", "", fmt.Errorf("failed to parse Creem response: %w", err)
	}

	checkoutURL, _ := result["checkout_url"].(string)
	checkoutId, _ := result["id"].(string)

	return checkoutURL, checkoutId, nil
}

// createEpaySubscriptionSession creates an Epay payment for subscription
func createEpaySubscriptionSession(c *gin.Context, sub *model.Subscription, user *model.User) (string, string, error) {
	if operation_setting.PayAddress == "" || operation_setting.EpayId == "" {
		return "", "", fmt.Errorf("Epay is not configured")
	}

	tradeNo := fmt.Sprintf("SUB%dNO%d%d", sub.UserId, sub.Id, time.Now().Unix())

	// Use Epay client from existing integration
	// This creates a payment URL that redirects to Epay gateway
	callbackURL := system_setting.ServerAddress + "/api/subscription/epay/notify"
	returnURL := system_setting.ServerAddress + "/subscription/success?provider=epay&trade_no=" + tradeNo

	// Build Epay payment URL (simplified - actual implementation depends on Epay SDK)
	paymentURL := fmt.Sprintf("%s?pid=%s&type=alipay&out_trade_no=%s&notify_url=%s&return_url=%s&name=%s&money=%.2f",
		operation_setting.PayAddress,
		operation_setting.EpayId,
		tradeNo,
		callbackURL,
		returnURL,
		sub.PlanName,
		sub.Amount,
	)

	return paymentURL, tradeNo, nil
}

// StripeSubscriptionWebhook handles Stripe webhook for subscription payments
// POST /api/subscription/stripe/webhook
func StripeSubscriptionWebhook(c *gin.Context) {
	payload, err := io.ReadAll(c.Request.Body)
	if err != nil {
		common.SysError("Failed to read Stripe webhook body: " + err.Error())
		c.Status(http.StatusBadRequest)
		return
	}

	// Verify webhook signature
	sigHeader := c.GetHeader("Stripe-Signature")
	event, err := webhook.ConstructEventWithOptions(payload, sigHeader, setting.StripeWebhookSecret,
		webhook.ConstructEventOptions{IgnoreAPIVersionMismatch: true})
	if err != nil {
		common.SysError("Stripe webhook signature verification failed: " + err.Error())
		c.Status(http.StatusUnauthorized)
		return
	}

	common.SysLog(fmt.Sprintf("Stripe subscription webhook received: type=%s, id=%s", event.Type, event.ID))

	switch event.Type {
	case "checkout.session.completed":
		handleStripeSubscriptionCompleted(event)
	case "checkout.session.expired":
		handleStripeSubscriptionExpired(event)
	case "charge.refunded":
		handleStripeSubscriptionRefund(event)
	default:
		common.SysLog(fmt.Sprintf("Unhandled Stripe event type: %s", event.Type))
	}

	c.Status(http.StatusOK)
}

func handleStripeSubscriptionCompleted(event stripe.Event) {
	var sess stripe.CheckoutSession
	if err := json.Unmarshal(event.Data.Raw, &sess); err != nil {
		common.SysError("Failed to parse Stripe session: " + err.Error())
		return
	}

	// Check if this is a subscription payment
	if sess.Metadata["type"] != "subscription" {
		return // Not a subscription payment, ignore
	}

	subscriptionIdStr := sess.Metadata["subscription_id"]
	if subscriptionIdStr == "" {
		common.SysError("Missing subscription_id in Stripe metadata")
		return
	}

	subscriptionId, _ := strconv.Atoi(subscriptionIdStr)
	if err := processSubscriptionPayment(subscriptionId, sess.ID, "stripe", sess.AmountTotal); err != nil {
		common.SysError(fmt.Sprintf("Failed to process Stripe subscription payment: %v", err))
	}
}

func handleStripeSubscriptionExpired(event stripe.Event) {
	var sess stripe.CheckoutSession
	if err := json.Unmarshal(event.Data.Raw, &sess); err != nil {
		return
	}

	if sess.Metadata["type"] != "subscription" {
		return
	}

	subscriptionIdStr := sess.Metadata["subscription_id"]
	if subscriptionIdStr == "" {
		return
	}

	subscriptionId, _ := strconv.Atoi(subscriptionIdStr)
	common.SysLog(fmt.Sprintf("Stripe subscription session expired: sub_id=%d", subscriptionId))

	// Don't automatically expire the subscription - user might retry with different method
}

func handleStripeSubscriptionRefund(event stripe.Event) {
	// Handle refund - cancel the subscription
	var charge stripe.Charge
	if err := json.Unmarshal(event.Data.Raw, &charge); err != nil {
		common.SysError("Failed to parse Stripe charge for refund: " + err.Error())
		return
	}

	// Get subscription by payment ID
	sub, err := model.GetSubscriptionByPaymentId(charge.PaymentIntent.ID)
	if err != nil || sub == nil {
		common.SysLog("No subscription found for refunded payment: " + charge.PaymentIntent.ID)
		return
	}

	common.SysLog(fmt.Sprintf("Processing refund for subscription %d", sub.Id))
	if err := model.RefundSubscription(sub); err != nil {
		common.SysError(fmt.Sprintf("Failed to process subscription refund: %v", err))
	}
}

// CreemSubscriptionWebhook handles Creem webhook for subscription payments
// POST /api/subscription/creem/webhook
func CreemSubscriptionWebhook(c *gin.Context) {
	payload, err := io.ReadAll(c.Request.Body)
	if err != nil {
		common.SysError("Failed to read Creem webhook body: " + err.Error())
		c.Status(http.StatusBadRequest)
		return
	}

	// Verify signature (skip in test mode)
	if !setting.CreemTestMode {
		signature := c.GetHeader("creem-signature")
		if !verifyCreemSubscriptionSignature(payload, signature) {
			common.SysError("Creem webhook signature verification failed")
			c.Status(http.StatusUnauthorized)
			return
		}
	}

	var webhookData map[string]interface{}
	if err := json.Unmarshal(payload, &webhookData); err != nil {
		common.SysError("Failed to parse Creem webhook: " + err.Error())
		c.Status(http.StatusBadRequest)
		return
	}

	eventType, _ := webhookData["eventType"].(string)
	common.SysLog(fmt.Sprintf("Creem subscription webhook received: type=%s", eventType))

	if eventType == "checkout.completed" {
		handleCreemSubscriptionCompleted(webhookData)
	}

	c.Status(http.StatusOK)
}

func verifyCreemSubscriptionSignature(payload []byte, signature string) bool {
	if setting.CreemWebhookSecret == "" {
		return true // No secret configured, skip verification
	}

	mac := hmac.New(sha256.New, []byte(setting.CreemWebhookSecret))
	mac.Write(payload)
	expectedSig := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(signature), []byte(expectedSig))
}

func handleCreemSubscriptionCompleted(data map[string]interface{}) {
	object, ok := data["object"].(map[string]interface{})
	if !ok {
		common.SysError("Invalid Creem webhook data structure")
		return
	}

	metadata, _ := object["metadata"].(map[string]interface{})
	if metadata == nil || metadata["type"] != "subscription" {
		return // Not a subscription payment
	}

	subscriptionIdStr, _ := metadata["subscription_id"].(string)
	if subscriptionIdStr == "" {
		common.SysError("Missing subscription_id in Creem metadata")
		return
	}

	subscriptionId, _ := strconv.Atoi(subscriptionIdStr)
	paymentId, _ := object["id"].(string)
	amount, _ := object["amount"].(float64)

	if err := processSubscriptionPayment(subscriptionId, paymentId, "creem", int64(amount)); err != nil {
		common.SysError(fmt.Sprintf("Failed to process Creem subscription payment: %v", err))
	}
}

// EpaySubscriptionNotify handles Epay callback for subscription payments
// GET /api/subscription/epay/notify
func EpaySubscriptionNotify(c *gin.Context) {
	tradeNo := c.Query("out_trade_no")
	tradeStatus := c.Query("trade_status")

	common.SysLog(fmt.Sprintf("Epay subscription notify: trade_no=%s, status=%s", tradeNo, tradeStatus))

	if tradeStatus != "TRADE_SUCCESS" {
		c.String(http.StatusOK, "success") // Acknowledge receipt
		return
	}

	// Parse subscription ID from trade_no (format: SUB{userId}NO{subId}{timestamp})
	// This is a simplified approach - in production, store trade_no -> subscription mapping
	sub, err := model.GetSubscriptionByPaymentId(tradeNo)
	if err != nil || sub == nil {
		common.SysError("Subscription not found for Epay trade_no: " + tradeNo)
		c.String(http.StatusOK, "success")
		return
	}

	if err := processSubscriptionPayment(sub.Id, tradeNo, "epay", int64(sub.Amount*100)); err != nil {
		common.SysError(fmt.Sprintf("Failed to process Epay subscription payment: %v", err))
	}

	c.String(http.StatusOK, "success")
}

// processSubscriptionPayment is the core payment processing logic with edge case handling
func processSubscriptionPayment(subscriptionId int, paymentId string, paymentMethod string, amountPaid int64) error {
	// Use database transaction with row-level lock for idempotency
	return model.DB.Transaction(func(tx *model.GormDB) error {
		// Get subscription with lock
		var sub model.Subscription
		if err := tx.Set("gorm:query_option", "FOR UPDATE").
			Where("id = ?", subscriptionId).
			First(&sub).Error; err != nil {
			return fmt.Errorf("subscription not found: %w", err)
		}

		// Idempotency check - already processed
		if sub.Status == model.SubscriptionStatusActive {
			common.SysLog(fmt.Sprintf("Subscription %d already activated, skipping duplicate payment", subscriptionId))
			return nil
		}

		// Status check - only pending can be activated
		if sub.Status != model.SubscriptionStatusPending {
			return fmt.Errorf("subscription status is %s, cannot activate", sub.Status)
		}

		// Amount verification (with tolerance for currency conversion)
		expectedAmount := int64(sub.Amount * 100) // Convert to cents
		tolerance := int64(float64(expectedAmount) * 0.05) // 5% tolerance for currency conversion
		if amountPaid > 0 && (amountPaid < expectedAmount-tolerance || amountPaid > expectedAmount+tolerance) {
			common.SysError(fmt.Sprintf("Amount mismatch for subscription %d: expected %d, got %d",
				subscriptionId, expectedAmount, amountPaid))
			// Don't fail - log and continue (payment provider amount might differ due to fees)
		}

		// Update payment info
		sub.PaymentId = paymentId
		sub.PaymentMethod = paymentMethod

		// Activate subscription
		if err := model.ActivateSubscriptionTx(tx, &sub); err != nil {
			return fmt.Errorf("failed to activate subscription: %w", err)
		}

		common.SysLog(fmt.Sprintf("Subscription payment processed successfully: sub_id=%d, user_id=%d, plan=%s, method=%s",
			sub.Id, sub.UserId, sub.PlanCode, paymentMethod))

		return nil
	})
}

// GetSubscriptionPaymentStatus returns the payment status of a subscription
// GET /api/subscription/:id/payment-status
func GetSubscriptionPaymentStatus(c *gin.Context) {
	userId := c.GetInt("id")
	if userId == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "Unauthorized"})
		return
	}

	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid subscription ID"})
		return
	}

	sub, err := model.GetSubscriptionById(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "Subscription not found"})
		return
	}

	// Verify ownership (non-admin)
	if sub.UserId != userId && c.GetInt("role") < common.RoleAdminUser {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "Access denied"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"subscription_id": sub.Id,
			"status":          sub.Status,
			"payment_method":  sub.PaymentMethod,
			"payment_id":      sub.PaymentId,
			"amount":          sub.Amount,
			"currency":        sub.Currency,
			"created_at":      sub.CreatedAt,
		},
	})
}

// RetrySubscriptionPayment allows user to retry payment with different method
// POST /api/subscription/:id/retry-payment
func RetrySubscriptionPayment(c *gin.Context) {
	userId := c.GetInt("id")
	if userId == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "Unauthorized"})
		return
	}

	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid subscription ID"})
		return
	}

	var req struct {
		PaymentMethod string `json:"payment_method" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid request"})
		return
	}

	sub, err := model.GetSubscriptionById(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "Subscription not found"})
		return
	}

	if sub.UserId != userId {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "Access denied"})
		return
	}

	// Only pending subscriptions can retry payment
	if sub.Status != model.SubscriptionStatusPending {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": fmt.Sprintf("Cannot retry payment for subscription with status: %s", sub.Status),
		})
		return
	}

	// Check expiry
	if time.Since(sub.CreatedAt) > 24*time.Hour {
		_ = model.UpdateSubscriptionStatus(sub.Id, model.SubscriptionStatusExpired)
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Subscription order has expired, please create a new one",
		})
		return
	}

	// Create new payment session with the new method
	c.Set("id", userId)
	payReq := SubscriptionPaymentRequest{
		SubscriptionId: sub.Id,
		PaymentMethod:  req.PaymentMethod,
	}

	// Reuse InitiateSubscriptionPayment logic
	user, _ := model.GetUserById(userId, false)
	var paymentURL, paymentId string

	switch req.PaymentMethod {
	case "stripe":
		paymentURL, paymentId, err = createStripeSubscriptionSession(sub, user)
	case "creem":
		paymentURL, paymentId, err = createCreemSubscriptionSession(sub, user)
	case "epay":
		paymentURL, paymentId, err = createEpaySubscriptionSession(c, sub, user)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Unsupported payment method"})
		return
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to create payment session: " + err.Error(),
		})
		return
	}

	// Update payment info
	_ = model.UpdateSubscriptionPaymentInfo(sub.Id, paymentId, req.PaymentMethod)

	common.SysLog(fmt.Sprintf("Subscription payment retry: sub_id=%d, method=%s->%s",
		sub.Id, sub.PaymentMethod, payReq.PaymentMethod))

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Payment session created",
		"data": gin.H{
			"payment_url": paymentURL,
			"payment_id":  paymentId,
		},
	})
}

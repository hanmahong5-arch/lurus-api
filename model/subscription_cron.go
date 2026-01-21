package model

import (
	"fmt"
	"time"

	"github.com/QuantumNous/lurus-api/common"
)

// StartSubscriptionCronJobs starts background jobs for subscription management
func StartSubscriptionCronJobs() {
	// Check expired subscriptions every 5 minutes
	go subscriptionExpiryChecker()

	common.SysLog("Subscription cron jobs started")
}

// subscriptionExpiryChecker periodically checks and expires subscriptions
func subscriptionExpiryChecker() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	// Run immediately on start
	processExpiredSubscriptions()

	for range ticker.C {
		processExpiredSubscriptions()
	}
}

// processExpiredSubscriptions finds and expires all overdue subscriptions
func processExpiredSubscriptions() {
	batchSize := 100

	for {
		subs, err := GetExpiredSubscriptions(batchSize)
		if err != nil {
			common.SysLog("Failed to get expired subscriptions: " + err.Error())
			return
		}

		if len(subs) == 0 {
			break
		}

		for _, sub := range subs {
			if err := ExpireSubscription(sub); err != nil {
				common.SysLog(fmt.Sprintf("Failed to expire subscription %d: %s", sub.Id, err.Error()))
				continue
			}
			common.SysLog(fmt.Sprintf("Expired subscription for user %d", sub.UserId))
		}

		// If we got less than batch size, we're done
		if len(subs) < batchSize {
			break
		}
	}
}

// ProcessSubscriptionRenewals handles auto-renewal for subscriptions
// This should be called by a payment cron job
func ProcessSubscriptionRenewals() {
	// Find subscriptions expiring within 24 hours with auto_renew enabled
	var subs []Subscription
	err := DB.Where(
		"status = ? AND auto_renew = ? AND expires_at < ? AND expires_at > ?",
		SubscriptionStatusActive, true,
		time.Now().Add(24*time.Hour), time.Now(),
	).Find(&subs).Error

	if err != nil {
		common.SysLog("Failed to get subscriptions for renewal: " + err.Error())
		return
	}

	for _, sub := range subs {
		// TODO: Trigger payment for renewal
		// This would typically create a new payment intent and charge the user
		common.SysLog(fmt.Sprintf("Subscription %s for user %d needs renewal", sub.PlanCode, sub.UserId))
	}
}

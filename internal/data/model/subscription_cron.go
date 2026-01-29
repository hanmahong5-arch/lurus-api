package model

import (
	"fmt"
	"time"

	"github.com/QuantumNous/lurus-api/internal/pkg/common"
)

// StartSubscriptionCronJobs starts background jobs for subscription management
func StartSubscriptionCronJobs() {
	// Check expired subscriptions every 5 minutes
	go subscriptionExpiryChecker()

	// Cleanup stale pending subscriptions every hour
	go stalePendingSubscriptionCleaner()

	// Process auto-renewals every hour
	go autoRenewalProcessor()

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

// stalePendingSubscriptionCleaner cleans up pending subscriptions older than 24 hours
func stalePendingSubscriptionCleaner() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	// Run immediately on start (with delay to allow system to stabilize)
	time.Sleep(30 * time.Second)
	cleanupStalePendingSubscriptions()

	for range ticker.C {
		cleanupStalePendingSubscriptions()
	}
}

// cleanupStalePendingSubscriptions marks old pending subscriptions as expired
func cleanupStalePendingSubscriptions() {
	batchSize := 100
	// Subscriptions pending for more than 24 hours are considered stale
	staleDuration := 24 * time.Hour
	totalCleaned := 0

	for {
		subs, err := GetPendingSubscriptionsOlderThan(staleDuration, batchSize)
		if err != nil {
			common.SysError("Failed to get stale pending subscriptions: " + err.Error())
			return
		}

		if len(subs) == 0 {
			break
		}

		for _, sub := range subs {
			if err := CleanupStalePendingSubscription(sub); err != nil {
				common.SysError(fmt.Sprintf("Failed to cleanup stale subscription %d: %s", sub.Id, err.Error()))
				continue
			}
			totalCleaned++
		}

		if len(subs) < batchSize {
			break
		}
	}

	if totalCleaned > 0 {
		common.SysLog(fmt.Sprintf("Cleaned up %d stale pending subscriptions", totalCleaned))
	}
}

// autoRenewalProcessor handles automatic subscription renewals
func autoRenewalProcessor() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	// Run immediately on start (with delay)
	time.Sleep(1 * time.Minute)
	ProcessSubscriptionRenewals()

	for range ticker.C {
		ProcessSubscriptionRenewals()
	}
}

// SendExpirationWarning sends warning to users whose subscriptions are expiring soon
func SendExpirationWarning() {
	// Find subscriptions expiring within 3 days
	var subs []Subscription
	err := DB.Where(
		"status = ? AND expires_at > ? AND expires_at < ?",
		SubscriptionStatusActive, time.Now(), time.Now().Add(72*time.Hour),
	).Find(&subs).Error

	if err != nil {
		common.SysError("Failed to get expiring subscriptions for warning: " + err.Error())
		return
	}

	for _, sub := range subs {
		daysRemaining := int(time.Until(sub.ExpiresAt).Hours() / 24)
		common.SysLog(fmt.Sprintf("Subscription expiring soon: user_id=%d, plan=%s, days_remaining=%d",
			sub.UserId, sub.PlanCode, daysRemaining))
		// TODO: Send email/notification to user
	}
}

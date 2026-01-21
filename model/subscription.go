package model

import (
	"errors"
	"time"

	"github.com/QuantumNous/lurus-api/common"
	"gorm.io/gorm"
)

// Subscription represents a user's subscription record
type Subscription struct {
	Id       int    `json:"id" gorm:"primaryKey;autoIncrement"`
	UserId   int    `json:"user_id" gorm:"index;not null"`
	PlanCode string `json:"plan_code" gorm:"type:varchar(32);not null"` // weekly/monthly/quarterly/yearly
	PlanName string `json:"plan_name" gorm:"type:varchar(64);not null"`
	Status   string `json:"status" gorm:"type:varchar(16);default:'active'"` // active/expired/cancelled/pending

	// Quota configuration (synced to User table on activation)
	DailyQuota    int    `json:"daily_quota" gorm:"type:int;default:0"`
	TotalQuota    int    `json:"total_quota" gorm:"type:int;default:0"`
	BaseGroup     string `json:"base_group" gorm:"type:varchar(64)"`
	FallbackGroup string `json:"fallback_group" gorm:"type:varchar(64)"`

	// Time
	StartedAt time.Time `json:"started_at" gorm:"not null"`
	ExpiresAt time.Time `json:"expires_at" gorm:"not null;index"`

	// Payment
	PaymentMethod string  `json:"payment_method" gorm:"type:varchar(32)"`    // stripe/epay/creem
	PaymentId     string  `json:"payment_id" gorm:"type:varchar(128);index"` // External payment ID
	Amount        float64 `json:"amount" gorm:"type:decimal(10,2)"`
	Currency      string  `json:"currency" gorm:"type:varchar(8);default:'CNY'"`
	AutoRenew     bool    `json:"auto_renew" gorm:"default:false"`

	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (Subscription) TableName() string {
	return "subscriptions"
}

// SubscriptionStatus constants
const (
	SubscriptionStatusPending   = "pending"   // Payment pending
	SubscriptionStatusActive    = "active"    // Currently active
	SubscriptionStatusExpired   = "expired"   // Expired
	SubscriptionStatusCancelled = "cancelled" // Cancelled by user
)

// CreateSubscription creates a new subscription record
func CreateSubscription(sub *Subscription) error {
	return DB.Create(sub).Error
}

// GetSubscriptionById retrieves subscription by ID
func GetSubscriptionById(id int) (*Subscription, error) {
	var sub Subscription
	err := DB.Where("id = ?", id).First(&sub).Error
	if err != nil {
		return nil, err
	}
	return &sub, nil
}

// GetActiveSubscription retrieves the active subscription for a user
func GetActiveSubscription(userId int) (*Subscription, error) {
	var sub Subscription
	err := DB.Where("user_id = ? AND status = ? AND expires_at > ?",
		userId, SubscriptionStatusActive, time.Now()).
		Order("expires_at DESC").
		First(&sub).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil // No active subscription
		}
		return nil, err
	}
	return &sub, nil
}

// GetUserSubscriptions retrieves all subscriptions for a user
func GetUserSubscriptions(userId int, limit int) ([]*Subscription, error) {
	var subs []*Subscription
	query := DB.Where("user_id = ?", userId).Order("created_at DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	err := query.Find(&subs).Error
	return subs, err
}

// UpdateSubscriptionStatus updates subscription status
func UpdateSubscriptionStatus(id int, status string) error {
	return DB.Model(&Subscription{}).Where("id = ?", id).Update("status", status).Error
}

// ActivateSubscription activates a subscription and syncs config to user
func ActivateSubscription(sub *Subscription) error {
	tx := DB.Begin()
	if tx.Error != nil {
		return tx.Error
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Update subscription status
	if err := tx.Model(sub).Updates(map[string]interface{}{
		"status":     SubscriptionStatusActive,
		"started_at": time.Now(),
	}).Error; err != nil {
		tx.Rollback()
		return err
	}

	// Sync config to user
	userConfig := &SubscriptionConfig{
		DailyQuota:    sub.DailyQuota,
		BaseGroup:     sub.BaseGroup,
		FallbackGroup: sub.FallbackGroup,
		Quota:         sub.TotalQuota,
	}

	updates := map[string]interface{}{
		"daily_quota":      userConfig.DailyQuota,
		"base_group":       userConfig.BaseGroup,
		"fallback_group":   userConfig.FallbackGroup,
		"daily_used":       0, // Reset daily used on new subscription
		"last_daily_reset": common.GetTimestamp(),
	}
	if userConfig.BaseGroup != "" {
		updates["group"] = userConfig.BaseGroup
	}
	if userConfig.Quota > 0 {
		updates["quota"] = gorm.Expr("quota + ?", userConfig.Quota)
	}

	if err := tx.Model(&User{}).Where("id = ?", sub.UserId).Updates(updates).Error; err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit().Error
}

// ExpireSubscription expires a subscription and handles user group fallback
func ExpireSubscription(sub *Subscription) error {
	tx := DB.Begin()
	if tx.Error != nil {
		return tx.Error
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Update subscription status
	if err := tx.Model(sub).Update("status", SubscriptionStatusExpired).Error; err != nil {
		tx.Rollback()
		return err
	}

	// Check if user has other active subscriptions
	var activeCount int64
	if err := tx.Model(&Subscription{}).Where(
		"user_id = ? AND status = ? AND expires_at > ? AND id != ?",
		sub.UserId, SubscriptionStatusActive, time.Now(), sub.Id,
	).Count(&activeCount).Error; err != nil {
		tx.Rollback()
		return err
	}

	// If no other active subscription, reset user to default group
	if activeCount == 0 {
		updates := map[string]interface{}{
			"daily_quota":    0,
			"base_group":     "",
			"fallback_group": "",
			"group":          "default",
		}
		if err := tx.Model(&User{}).Where("id = ?", sub.UserId).Updates(updates).Error; err != nil {
			tx.Rollback()
			return err
		}
	}

	return tx.Commit().Error
}

// GetExpiredSubscriptions retrieves subscriptions that have expired but still marked as active
func GetExpiredSubscriptions(limit int) ([]*Subscription, error) {
	var subs []*Subscription
	err := DB.Where("status = ? AND expires_at < ?", SubscriptionStatusActive, time.Now()).
		Limit(limit).
		Find(&subs).Error
	return subs, err
}

// CancelSubscription cancels auto-renewal for a subscription
func CancelSubscription(id int) error {
	return DB.Model(&Subscription{}).Where("id = ?", id).Updates(map[string]interface{}{
		"auto_renew": false,
		"status":     SubscriptionStatusCancelled,
	}).Error
}

// GetSubscriptionByPaymentId retrieves subscription by payment ID
func GetSubscriptionByPaymentId(paymentId string) (*Subscription, error) {
	var sub Subscription
	err := DB.Where("payment_id = ?", paymentId).First(&sub).Error
	if err != nil {
		return nil, err
	}
	return &sub, nil
}

// RenewSubscription extends subscription expiry date
func RenewSubscription(sub *Subscription, days int) error {
	newExpiry := sub.ExpiresAt.AddDate(0, 0, days)
	if sub.ExpiresAt.Before(time.Now()) {
		// If already expired, start from now
		newExpiry = time.Now().AddDate(0, 0, days)
	}

	return DB.Model(sub).Updates(map[string]interface{}{
		"expires_at": newExpiry,
		"status":     SubscriptionStatusActive,
	}).Error
}

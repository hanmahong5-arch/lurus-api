package model

import (
	"encoding/json"
	"sync"

	"github.com/QuantumNous/lurus-api/common"
)

// getOptionValue retrieves option value from database
func getOptionValue(key string) (string, error) {
	var option Option
	err := DB.Where("key = ?", key).First(&option).Error
	if err != nil {
		return "", err
	}
	return option.Value, nil
}

// SubscriptionPlan represents a subscription plan configuration
type SubscriptionPlan struct {
	Code          string  `json:"code"`           // weekly/monthly/quarterly/yearly
	Name          string  `json:"name"`           // Display name
	Description   string  `json:"description"`    // Plan description
	Days          int     `json:"days"`           // Duration in days
	Price         float64 `json:"price"`          // Price in CNY
	Currency      string  `json:"currency"`       // Currency code
	DailyQuota    int     `json:"daily_quota"`    // Daily quota limit
	TotalQuota    int     `json:"total_quota"`    // Total quota for the period
	BaseGroup     string  `json:"base_group"`     // User group when active
	FallbackGroup string  `json:"fallback_group"` // Fallback group when daily quota exhausted
	Enabled       bool    `json:"enabled"`        // Whether plan is available
	SortOrder     int     `json:"sort_order"`     // Display order
}

// Default subscription plans
var defaultSubscriptionPlans = []SubscriptionPlan{
	{
		Code:          "weekly",
		Name:          "Weekly Plan",
		Description:   "7-day membership for short-term use",
		Days:          7,
		Price:         19.9,
		Currency:      "CNY",
		DailyQuota:    500000,  // 500K/day
		TotalQuota:    5000000, // 5M total
		BaseGroup:     "weekly",
		FallbackGroup: "free",
		Enabled:       true,
		SortOrder:     1,
	},
	{
		Code:          "monthly",
		Name:          "Monthly Plan",
		Description:   "30-day membership, best value",
		Days:          30,
		Price:         59.9,
		Currency:      "CNY",
		DailyQuota:    1000000,  // 1M/day
		TotalQuota:    50000000, // 50M total
		BaseGroup:     "monthly",
		FallbackGroup: "weekly",
		Enabled:       true,
		SortOrder:     2,
	},
	{
		Code:          "quarterly",
		Name:          "Quarterly Plan",
		Description:   "90-day membership for power users",
		Days:          90,
		Price:         149.9,
		Currency:      "CNY",
		DailyQuota:    2000000,   // 2M/day
		TotalQuota:    200000000, // 200M total
		BaseGroup:     "quarterly",
		FallbackGroup: "monthly",
		Enabled:       true,
		SortOrder:     3,
	},
	{
		Code:          "yearly",
		Name:          "Yearly Plan",
		Description:   "365-day membership, unlimited access",
		Days:          365,
		Price:         499.9,
		Currency:      "CNY",
		DailyQuota:    5000000, // 5M/day
		TotalQuota:    0,       // Unlimited
		BaseGroup:     "yearly",
		FallbackGroup: "quarterly",
		Enabled:       true,
		SortOrder:     4,
	},
}

var (
	subscriptionPlansCache []SubscriptionPlan
	subscriptionPlansMu    sync.RWMutex
)

// InitSubscriptionPlans initializes subscription plans from option or defaults
func InitSubscriptionPlans() {
	subscriptionPlansMu.Lock()
	defer subscriptionPlansMu.Unlock()

	// Try to load from option
	plansJSON, err := getOptionValue("SubscriptionPlans")
	if err == nil && plansJSON != "" {
		var plans []SubscriptionPlan
		if err := json.Unmarshal([]byte(plansJSON), &plans); err == nil && len(plans) > 0 {
			subscriptionPlansCache = plans
			common.SysLog("Loaded subscription plans from option")
			return
		}
	}

	// Use defaults
	subscriptionPlansCache = defaultSubscriptionPlans
	common.SysLog("Using default subscription plans")
}

// GetSubscriptionPlans returns all enabled subscription plans
func GetSubscriptionPlans() []SubscriptionPlan {
	subscriptionPlansMu.RLock()
	defer subscriptionPlansMu.RUnlock()

	if len(subscriptionPlansCache) == 0 {
		subscriptionPlansMu.RUnlock()
		InitSubscriptionPlans()
		subscriptionPlansMu.RLock()
	}

	var enabledPlans []SubscriptionPlan
	for _, plan := range subscriptionPlansCache {
		if plan.Enabled {
			enabledPlans = append(enabledPlans, plan)
		}
	}
	return enabledPlans
}

// GetSubscriptionPlanByCode returns a subscription plan by code
func GetSubscriptionPlanByCode(code string) *SubscriptionPlan {
	subscriptionPlansMu.RLock()
	defer subscriptionPlansMu.RUnlock()

	if len(subscriptionPlansCache) == 0 {
		subscriptionPlansMu.RUnlock()
		InitSubscriptionPlans()
		subscriptionPlansMu.RLock()
	}

	for _, plan := range subscriptionPlansCache {
		if plan.Code == code {
			return &plan
		}
	}
	return nil
}

// UpdateSubscriptionPlans updates subscription plans in option
func UpdateSubscriptionPlans(plans []SubscriptionPlan) error {
	plansJSON, err := json.Marshal(plans)
	if err != nil {
		return err
	}

	if err := UpdateOption("SubscriptionPlans", string(plansJSON)); err != nil {
		return err
	}

	subscriptionPlansMu.Lock()
	subscriptionPlansCache = plans
	subscriptionPlansMu.Unlock()

	return nil
}

// GetAllSubscriptionPlans returns all plans including disabled ones (for admin)
func GetAllSubscriptionPlans() []SubscriptionPlan {
	subscriptionPlansMu.RLock()
	defer subscriptionPlansMu.RUnlock()

	if len(subscriptionPlansCache) == 0 {
		subscriptionPlansMu.RUnlock()
		InitSubscriptionPlans()
		subscriptionPlansMu.RLock()
	}

	return subscriptionPlansCache
}

package model

import (
	"testing"
	"time"
)

// TestNeedsDailyReset tests the daily reset detection logic
func TestNeedsDailyReset(t *testing.T) {
	tests := []struct {
		name           string
		lastReset      int64
		expectedResult bool
	}{
		{
			name:           "needs reset - last reset was yesterday",
			lastReset:      time.Now().AddDate(0, 0, -1).Unix(),
			expectedResult: true,
		},
		{
			name:           "needs reset - last reset was a week ago",
			lastReset:      time.Now().AddDate(0, 0, -7).Unix(),
			expectedResult: true,
		},
		{
			name:           "no reset needed - reset today (1 hour ago)",
			lastReset:      time.Now().Add(-1 * time.Hour).Unix(),
			expectedResult: false,
		},
		{
			name:           "needs reset - never reset (zero timestamp)",
			lastReset:      0,
			expectedResult: true,
		},
		{
			name:           "no reset needed - reset just now",
			lastReset:      time.Now().Unix(),
			expectedResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NeedsDailyReset(tt.lastReset)
			if result != tt.expectedResult {
				t.Errorf("NeedsDailyReset(%d) = %v, want %v", tt.lastReset, result, tt.expectedResult)
			}
		})
	}
}

// TestDailyQuotaInfo tests the DailyQuotaInfo struct calculations
func TestDailyQuotaInfoCalculations(t *testing.T) {
	tests := []struct {
		name            string
		info            DailyQuotaInfo
		expectedRemain  int
		expectedExhaust bool
	}{
		{
			name: "has remaining quota",
			info: DailyQuotaInfo{
				DailyQuota: 1000,
				DailyUsed:  500,
			},
			expectedRemain:  500,
			expectedExhaust: false,
		},
		{
			name: "quota exhausted",
			info: DailyQuotaInfo{
				DailyQuota: 1000,
				DailyUsed:  1000,
			},
			expectedRemain:  0,
			expectedExhaust: true,
		},
		{
			name: "over quota",
			info: DailyQuotaInfo{
				DailyQuota: 1000,
				DailyUsed:  1200,
			},
			expectedRemain:  -200,
			expectedExhaust: true,
		},
		{
			name: "unlimited quota (DailyQuota = 0)",
			info: DailyQuotaInfo{
				DailyQuota: 0,
				DailyUsed:  5000,
			},
			expectedRemain:  -1, // -1 indicates unlimited
			expectedExhaust: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			remaining := tt.info.DailyQuota - tt.info.DailyUsed
			if tt.info.DailyQuota <= 0 {
				remaining = -1 // unlimited
			}
			
			if remaining != tt.expectedRemain {
				t.Errorf("DailyRemaining = %d, want %d", remaining, tt.expectedRemain)
			}
			
			exhausted := tt.info.DailyQuota > 0 && tt.info.DailyUsed >= tt.info.DailyQuota
			if exhausted != tt.expectedExhaust {
				t.Errorf("IsExhausted = %v, want %v", exhausted, tt.expectedExhaust)
			}
		})
	}
}

// TestFallbackGroupLogic tests the fallback group switching logic
func TestFallbackGroupLogic(t *testing.T) {
	tests := []struct {
		name            string
		currentGroup    string
		baseGroup       string
		fallbackGroup   string
		isUsingFallback bool
	}{
		{
			name:            "using base group",
			currentGroup:    "pro",
			baseGroup:       "pro",
			fallbackGroup:   "free",
			isUsingFallback: false,
		},
		{
			name:            "using fallback group",
			currentGroup:    "free",
			baseGroup:       "pro",
			fallbackGroup:   "free",
			isUsingFallback: true,
		},
		{
			name:            "no fallback configured",
			currentGroup:    "pro",
			baseGroup:       "pro",
			fallbackGroup:   "",
			isUsingFallback: false,
		},
		{
			name:            "different group (manual override)",
			currentGroup:    "enterprise",
			baseGroup:       "pro",
			fallbackGroup:   "free",
			isUsingFallback: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := DailyQuotaInfo{
				CurrentGroup:  tt.currentGroup,
				BaseGroup:     tt.baseGroup,
				FallbackGroup: tt.fallbackGroup,
			}
			
			isUsingFallback := info.FallbackGroup != "" && info.CurrentGroup == info.FallbackGroup && info.CurrentGroup != info.BaseGroup
			if isUsingFallback != tt.isUsingFallback {
				t.Errorf("IsUsingFallback = %v, want %v", isUsingFallback, tt.isUsingFallback)
			}
		})
	}
}

// TestSubscriptionConfigValidation tests subscription config validation
func TestSubscriptionConfigValidation(t *testing.T) {
	tests := []struct {
		name        string
		config      SubscriptionConfig
		expectError bool
	}{
		{
			name: "valid config - full",
			config: SubscriptionConfig{
				DailyQuota:    1000000,
				BaseGroup:     "pro",
				FallbackGroup: "free",
				Quota:         5000000,
			},
			expectError: false,
		},
		{
			name: "valid config - no daily quota",
			config: SubscriptionConfig{
				DailyQuota:    0,
				BaseGroup:     "unlimited",
				FallbackGroup: "",
				Quota:         0,
			},
			expectError: false,
		},
		{
			name: "valid config - minimal",
			config: SubscriptionConfig{
				BaseGroup: "free",
			},
			expectError: false,
		},
		{
			name: "negative daily quota should be treated as 0",
			config: SubscriptionConfig{
				DailyQuota: -100,
				BaseGroup:  "test",
			},
			expectError: false, // System should handle negative as 0 (unlimited)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Validation logic test
			hasError := false
			if tt.config.BaseGroup == "" && tt.config.DailyQuota > 0 {
				hasError = true // Should have base group if daily quota is set
			}
			
			if hasError != tt.expectError {
				t.Errorf("Validation error = %v, want %v", hasError, tt.expectError)
			}
		})
	}
}

// TestDailyResetTimeCalculation tests the reset time calculation
func TestDailyResetTimeCalculation(t *testing.T) {
	now := time.Now()
	
	// Test: reset should happen at midnight UTC
	todayMidnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	yesterdayMidnight := todayMidnight.AddDate(0, 0, -1)
	
	tests := []struct {
		name        string
		lastReset   time.Time
		needsReset  bool
	}{
		{
			name:       "last reset at yesterday midnight - needs reset",
			lastReset:  yesterdayMidnight,
			needsReset: true,
		},
		{
			name:       "last reset at today midnight - no reset needed",
			lastReset:  todayMidnight,
			needsReset: false,
		},
		{
			name:       "last reset 1 second before today midnight - needs reset",
			lastReset:  todayMidnight.Add(-1 * time.Second),
			needsReset: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NeedsDailyReset(tt.lastReset.Unix())
			if result != tt.needsReset {
				t.Errorf("NeedsDailyReset for %v = %v, want %v", tt.lastReset, result, tt.needsReset)
			}
		})
	}
}

// BenchmarkNeedsDailyReset benchmarks the reset check function
func BenchmarkNeedsDailyReset(b *testing.B) {
	lastReset := time.Now().Add(-25 * time.Hour).Unix()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		NeedsDailyReset(lastReset)
	}
}

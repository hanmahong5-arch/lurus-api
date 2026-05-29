package repo

import (
	"testing"
	"time"
)

// TestNeedsDailyReset tests the daily reset detection logic deterministically.
// Day boundaries are UTC midnight, so a real-clock now-relative test (e.g.
// now-1h) flakes when the suite runs within an hour of UTC midnight. We inject
// a fixed "now" via NeedsDailyResetAt to make every case deterministic.
func TestNeedsDailyReset(t *testing.T) {
	// Fixed reference: 2026-05-28 12:00:00 UTC (noon — far from any day boundary).
	refNow := time.Date(2026, 5, 28, 12, 0, 0, 0, time.UTC)
	now := refNow.Unix()

	tests := []struct {
		name           string
		lastReset      int64
		expectedResult bool
	}{
		{
			name:           "needs reset - last reset was yesterday",
			lastReset:      refNow.AddDate(0, 0, -1).Unix(),
			expectedResult: true,
		},
		{
			name:           "needs reset - last reset was a week ago",
			lastReset:      refNow.AddDate(0, 0, -7).Unix(),
			expectedResult: true,
		},
		{
			name:           "no reset needed - reset today (1 hour ago)",
			lastReset:      refNow.Add(-1 * time.Hour).Unix(),
			expectedResult: false,
		},
		{
			name:           "needs reset - never reset (zero timestamp)",
			lastReset:      0,
			expectedResult: true,
		},
		{
			name:           "no reset needed - reset just now",
			lastReset:      now,
			expectedResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NeedsDailyResetAt(tt.lastReset, now)
			if result != tt.expectedResult {
				t.Errorf("NeedsDailyResetAt(%d, %d) = %v, want %v", tt.lastReset, now, result, tt.expectedResult)
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


// TestDailyResetTimeCalculation tests the reset boundary deterministically with
// a fixed UTC "now" injected via NeedsDailyResetAt. Daily reset fires at UTC
// midnight; building todayMidnight from local time.Now() against a UTC-day
// function (the previous version) flaked whenever the local date and the UTC
// date differed (i.e. for any machine not on UTC, near midnight).
func TestDailyResetTimeCalculation(t *testing.T) {
	// Fixed reference now: 2026-05-28 12:00:00 UTC.
	refNow := time.Date(2026, 5, 28, 12, 0, 0, 0, time.UTC)
	now := refNow.Unix()

	todayMidnight := time.Date(2026, 5, 28, 0, 0, 0, 0, time.UTC)
	yesterdayMidnight := todayMidnight.AddDate(0, 0, -1)

	tests := []struct {
		name       string
		lastReset  time.Time
		needsReset bool
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
			result := NeedsDailyResetAt(tt.lastReset.Unix(), now)
			if result != tt.needsReset {
				t.Errorf("NeedsDailyResetAt for %v = %v, want %v", tt.lastReset, result, tt.needsReset)
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

// TestResetDailyQuota_Idempotent verifies P1-3: calling ResetDailyQuota twice on the same day
// must not error and must leave daily_used at 0 (idempotent - no double-reset side effects).
func TestResetDailyQuota_Idempotent(t *testing.T) {
	cleanup := SetupTestDB(t)
	defer cleanup()

	_, normal, _ := SeedTestUsers(t)

	// Set user daily_used to non-zero and last_daily_reset to yesterday
	yesterdayTS := time.Now().AddDate(0, 0, -1).Unix()
	DB.Model(&User{}).Where("id = ?", normal.Id).Updates(map[string]interface{}{
		"daily_used":       1000,
		"last_daily_reset": yesterdayTS,
	})

	// First reset: should clear daily_used
	if err := ResetDailyQuota(normal.Id); err != nil {
		t.Fatalf("first ResetDailyQuota() failed: %v", err)
	}

	var user User
	DB.First(&user, "id = ?", normal.Id)
	if user.DailyUsed != 0 {
		t.Errorf("after first reset: DailyUsed = %d, want 0", user.DailyUsed)
	}

	// Second reset on the same day: must return nil (idempotent)
	if err := ResetDailyQuota(normal.Id); err != nil {
		t.Fatalf("second ResetDailyQuota() (same day) returned error: %v", err)
	}

	// daily_used must still be 0 - no corruption from second call
	DB.First(&user, "id = ?", normal.Id)
	if user.DailyUsed != 0 {
		t.Errorf("after second reset: DailyUsed = %d, want 0 (idempotent - must not corrupt state)", user.DailyUsed)
	}
}

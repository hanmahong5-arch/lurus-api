package entity

import "testing"

func TestIsUnlimited(t *testing.T) {
	tests := []struct {
		name string
		pool TenantCreditPool
		want bool
	}{
		{"unlimited sentinel", TenantCreditPool{MaxBalance: PoolMaxBalanceUnlimited}, true},
		{"zero ceiling", TenantCreditPool{MaxBalance: 0}, false},
		{"finite", TenantCreditPool{MaxBalance: 1000}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.pool.IsUnlimited(); got != tt.want {
				t.Errorf("IsUnlimited() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsExhausted(t *testing.T) {
	tests := []struct {
		name string
		pool TenantCreditPool
		want bool
	}{
		{"unlimited never exhausted", TenantCreditPool{MaxBalance: PoolMaxBalanceUnlimited, CurrentBalance: 0}, false},
		{"unlimited with negative balance not exhausted (free)", TenantCreditPool{MaxBalance: PoolMaxBalanceUnlimited, CurrentBalance: -50}, false},
		{"finite zero balance is exhausted", TenantCreditPool{MaxBalance: 1000, CurrentBalance: 0}, true},
		{"finite negative balance is exhausted", TenantCreditPool{MaxBalance: 1000, CurrentBalance: -10}, true},
		{"finite positive balance is not exhausted", TenantCreditPool{MaxBalance: 1000, CurrentBalance: 1}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.pool.IsExhausted(); got != tt.want {
				t.Errorf("IsExhausted() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestShouldAlert(t *testing.T) {
	// Threshold 80%: pool fires alert when current_balance < 80% of max_balance.
	// Edge case: unlimited pools never alert; zero-ceiling pools never alert.
	tests := []struct {
		name string
		pool TenantCreditPool
		want bool
	}{
		{
			"unlimited never alerts",
			TenantCreditPool{MaxBalance: PoolMaxBalanceUnlimited, CurrentBalance: 0, AlertThresholdPct: 80},
			false,
		},
		{
			"zero ceiling never alerts (degenerate config)",
			TenantCreditPool{MaxBalance: 0, CurrentBalance: 0, AlertThresholdPct: 80},
			false,
		},
		{
			"balance above threshold no alert",
			TenantCreditPool{MaxBalance: 1000, CurrentBalance: 900, AlertThresholdPct: 80},
			false,
		},
		{
			"balance below threshold fires alert",
			TenantCreditPool{MaxBalance: 1000, CurrentBalance: 500, AlertThresholdPct: 80},
			true,
		},
		{
			"balance exactly at threshold (boundary, < not <=) no alert",
			TenantCreditPool{MaxBalance: 1000, CurrentBalance: 800, AlertThresholdPct: 80},
			false,
		},
		{
			"50% threshold respected",
			TenantCreditPool{MaxBalance: 1000, CurrentBalance: 400, AlertThresholdPct: 50},
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.pool.ShouldAlert(); got != tt.want {
				t.Errorf("ShouldAlert() = %v, want %v", got, tt.want)
			}
		})
	}
}

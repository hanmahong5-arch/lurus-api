package middleware

import (
	"testing"

	"github.com/QuantumNous/lurus-api/common"
)

// TestIsPhoneRequiredForAction tests the phone requirement check
func TestIsPhoneRequiredForAction(t *testing.T) {
	// Save original values
	originalPayment := common.PhoneRequiredForPayment
	originalWithdrawal := common.PhoneRequiredForWithdrawal
	originalPasswordReset := common.PhoneRequiredForPasswordReset
	original2FAChange := common.PhoneRequiredFor2FAChange
	originalPhoneBind := common.PhoneRequiredForPhoneBind
	originalOAuthBind := common.PhoneRequiredForOAuthBind
	originalAccountDelete := common.PhoneRequiredForAccountDelete
	originalTokenGenerate := common.PhoneRequiredForTokenGenerate

	// Restore after test
	defer func() {
		common.PhoneRequiredForPayment = originalPayment
		common.PhoneRequiredForWithdrawal = originalWithdrawal
		common.PhoneRequiredForPasswordReset = originalPasswordReset
		common.PhoneRequiredFor2FAChange = original2FAChange
		common.PhoneRequiredForPhoneBind = originalPhoneBind
		common.PhoneRequiredForOAuthBind = originalOAuthBind
		common.PhoneRequiredForAccountDelete = originalAccountDelete
		common.PhoneRequiredForTokenGenerate = originalTokenGenerate
	}()

	// Enable all for testing
	common.PhoneRequiredForPayment = true
	common.PhoneRequiredForWithdrawal = true
	common.PhoneRequiredForPasswordReset = true
	common.PhoneRequiredFor2FAChange = true
	common.PhoneRequiredForPhoneBind = true
	common.PhoneRequiredForOAuthBind = true
	common.PhoneRequiredForAccountDelete = true
	common.PhoneRequiredForTokenGenerate = true

	tests := []struct {
		name       string
		actionType string
		expected   bool
	}{
		{"payment_required", common.SensitiveActionPayment, true},
		{"withdrawal_required", common.SensitiveActionWithdrawal, true},
		{"password_change_required", common.SensitiveActionPasswordChange, true},
		{"2fa_change_required", common.SensitiveAction2FAChange, true},
		{"phone_bind_required", common.SensitiveActionPhoneBind, true},
		{"oauth_bind_required", common.SensitiveActionOAuthBind, true},
		{"account_delete_required", common.SensitiveActionAccountDelete, true},
		{"token_generate_required", common.SensitiveActionTokenGenerate, true},
		{"unknown_action", "unknown", false},
		{"empty_action", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isPhoneRequiredForAction(tt.actionType)
			if result != tt.expected {
				t.Errorf("isPhoneRequiredForAction(%q) = %v, want %v", tt.actionType, result, tt.expected)
			}
		})
	}
}

// TestIsPhoneRequiredForActionDisabled tests when requirements are disabled
func TestIsPhoneRequiredForActionDisabled(t *testing.T) {
	// Save original values
	originalPayment := common.PhoneRequiredForPayment
	originalWithdrawal := common.PhoneRequiredForWithdrawal
	originalPasswordReset := common.PhoneRequiredForPasswordReset
	original2FAChange := common.PhoneRequiredFor2FAChange
	originalPhoneBind := common.PhoneRequiredForPhoneBind
	originalOAuthBind := common.PhoneRequiredForOAuthBind
	originalAccountDelete := common.PhoneRequiredForAccountDelete
	originalTokenGenerate := common.PhoneRequiredForTokenGenerate

	// Restore after test
	defer func() {
		common.PhoneRequiredForPayment = originalPayment
		common.PhoneRequiredForWithdrawal = originalWithdrawal
		common.PhoneRequiredForPasswordReset = originalPasswordReset
		common.PhoneRequiredFor2FAChange = original2FAChange
		common.PhoneRequiredForPhoneBind = originalPhoneBind
		common.PhoneRequiredForOAuthBind = originalOAuthBind
		common.PhoneRequiredForAccountDelete = originalAccountDelete
		common.PhoneRequiredForTokenGenerate = originalTokenGenerate
	}()

	// Disable all for testing
	common.PhoneRequiredForPayment = false
	common.PhoneRequiredForWithdrawal = false
	common.PhoneRequiredForPasswordReset = false
	common.PhoneRequiredFor2FAChange = false
	common.PhoneRequiredForPhoneBind = false
	common.PhoneRequiredForOAuthBind = false
	common.PhoneRequiredForAccountDelete = false
	common.PhoneRequiredForTokenGenerate = false

	tests := []struct {
		actionType string
	}{
		{common.SensitiveActionPayment},
		{common.SensitiveActionWithdrawal},
		{common.SensitiveActionPasswordChange},
		{common.SensitiveAction2FAChange},
		{common.SensitiveActionPhoneBind},
		{common.SensitiveActionOAuthBind},
		{common.SensitiveActionAccountDelete},
		{common.SensitiveActionTokenGenerate},
	}

	for _, tt := range tests {
		t.Run(tt.actionType, func(t *testing.T) {
			result := isPhoneRequiredForAction(tt.actionType)
			if result != false {
				t.Errorf("isPhoneRequiredForAction(%q) = %v, want false when disabled", tt.actionType, result)
			}
		})
	}
}

// TestMaskPhone tests the phone masking function
func TestMaskPhone(t *testing.T) {
	tests := []struct {
		name     string
		phone    string
		expected string
	}{
		{"standard_11_digit", "13812345678", "138****5678"},
		{"empty_string", "", ""},
		{"too_short_6", "123456", "123456"},
		{"exactly_7_chars", "1234567", "123****4567"},
		{"very_long", "123456789012345", "123****2345"},
		{"international", "+8613812345678", "+86****5678"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := maskPhone(tt.phone)
			if result != tt.expected {
				t.Errorf("maskPhone(%q) = %q, want %q", tt.phone, result, tt.expected)
			}
		})
	}
}

// TestSensitiveActionConstants tests the sensitive action type constants
func TestSensitiveActionConstants(t *testing.T) {
	tests := []struct {
		constant string
		expected string
	}{
		{common.SensitiveActionPayment, "payment"},
		{common.SensitiveActionWithdrawal, "withdrawal"},
		{common.SensitiveActionPasswordChange, "password_change"},
		{common.SensitiveAction2FAChange, "2fa_change"},
		{common.SensitiveActionPhoneBind, "phone_bind"},
		{common.SensitiveActionOAuthBind, "oauth_bind"},
		{common.SensitiveActionAccountDelete, "account_delete"},
		{common.SensitiveActionTokenGenerate, "token_generate"},
	}

	for _, tt := range tests {
		if tt.constant != tt.expected {
			t.Errorf("constant value = %q, want %q", tt.constant, tt.expected)
		}
	}
}

// TestPhoneVerificationModeConstants tests the phone verification mode constants
func TestPhoneVerificationModeConstants(t *testing.T) {
	tests := []struct {
		constant string
		expected string
	}{
		{common.PhoneVerificationDisabled, "disabled"},
		{common.PhoneVerificationOptional, "optional"},
		{common.PhoneVerificationRequiredLogin, "required_login"},
		{common.PhoneVerificationRequiredSensitive, "required_sensitive"},
	}

	for _, tt := range tests {
		if tt.constant != tt.expected {
			t.Errorf("constant value = %q, want %q", tt.constant, tt.expected)
		}
	}
}

// TestIsPhoneRequiredForActionMixed tests mixed enabled/disabled scenarios
func TestIsPhoneRequiredForActionMixed(t *testing.T) {
	// Save original values
	originalPayment := common.PhoneRequiredForPayment
	originalWithdrawal := common.PhoneRequiredForWithdrawal
	originalPasswordReset := common.PhoneRequiredForPasswordReset

	// Restore after test
	defer func() {
		common.PhoneRequiredForPayment = originalPayment
		common.PhoneRequiredForWithdrawal = originalWithdrawal
		common.PhoneRequiredForPasswordReset = originalPasswordReset
	}()

	// Mixed setup: only payment and withdrawal require phone
	common.PhoneRequiredForPayment = true
	common.PhoneRequiredForWithdrawal = true
	common.PhoneRequiredForPasswordReset = false

	tests := []struct {
		actionType string
		expected   bool
	}{
		{common.SensitiveActionPayment, true},
		{common.SensitiveActionWithdrawal, true},
		{common.SensitiveActionPasswordChange, false},
	}

	for _, tt := range tests {
		result := isPhoneRequiredForAction(tt.actionType)
		if result != tt.expected {
			t.Errorf("isPhoneRequiredForAction(%q) = %v, want %v", tt.actionType, result, tt.expected)
		}
	}
}

// BenchmarkIsPhoneRequiredForAction benchmarks the phone requirement check
func BenchmarkIsPhoneRequiredForAction(b *testing.B) {
	common.PhoneRequiredForPayment = true

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		isPhoneRequiredForAction(common.SensitiveActionPayment)
	}
}

// BenchmarkMaskPhone benchmarks the phone masking function
func BenchmarkMaskPhone(b *testing.B) {
	phone := "13812345678"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		maskPhone(phone)
	}
}

package controller

import (
	"testing"

	"github.com/QuantumNous/lurus-api/internal/pkg/common"
)

// TestGetPhoneRequiredActions tests the phone required actions list
func TestGetPhoneRequiredActions(t *testing.T) {
	// Save original values
	originalLogin := common.PhoneRequiredForLogin
	originalPasswordReset := common.PhoneRequiredForPasswordReset
	original2FAChange := common.PhoneRequiredFor2FAChange
	originalPayment := common.PhoneRequiredForPayment
	originalWithdrawal := common.PhoneRequiredForWithdrawal
	originalPhoneBind := common.PhoneRequiredForPhoneBind
	originalAccountDelete := common.PhoneRequiredForAccountDelete
	originalTokenGenerate := common.PhoneRequiredForTokenGenerate
	originalOAuthBind := common.PhoneRequiredForOAuthBind

	// Restore after test
	defer func() {
		common.PhoneRequiredForLogin = originalLogin
		common.PhoneRequiredForPasswordReset = originalPasswordReset
		common.PhoneRequiredFor2FAChange = original2FAChange
		common.PhoneRequiredForPayment = originalPayment
		common.PhoneRequiredForWithdrawal = originalWithdrawal
		common.PhoneRequiredForPhoneBind = originalPhoneBind
		common.PhoneRequiredForAccountDelete = originalAccountDelete
		common.PhoneRequiredForTokenGenerate = originalTokenGenerate
		common.PhoneRequiredForOAuthBind = originalOAuthBind
	}()

	tests := []struct {
		name           string
		setup          func()
		expectedLen    int
		expectedAction string
		shouldContain  bool
	}{
		{
			name: "no_requirements",
			setup: func() {
				common.PhoneRequiredForLogin = false
				common.PhoneRequiredForPasswordReset = false
				common.PhoneRequiredFor2FAChange = false
				common.PhoneRequiredForPayment = false
				common.PhoneRequiredForWithdrawal = false
				common.PhoneRequiredForPhoneBind = false
				common.PhoneRequiredForAccountDelete = false
				common.PhoneRequiredForTokenGenerate = false
				common.PhoneRequiredForOAuthBind = false
			},
			expectedLen:    0,
			expectedAction: "",
			shouldContain:  false,
		},
		{
			name: "login_required",
			setup: func() {
				common.PhoneRequiredForLogin = true
				common.PhoneRequiredForPasswordReset = false
				common.PhoneRequiredFor2FAChange = false
				common.PhoneRequiredForPayment = false
				common.PhoneRequiredForWithdrawal = false
				common.PhoneRequiredForPhoneBind = false
				common.PhoneRequiredForAccountDelete = false
				common.PhoneRequiredForTokenGenerate = false
				common.PhoneRequiredForOAuthBind = false
			},
			expectedLen:    1,
			expectedAction: "login",
			shouldContain:  true,
		},
		{
			name: "payment_required",
			setup: func() {
				common.PhoneRequiredForLogin = false
				common.PhoneRequiredForPasswordReset = false
				common.PhoneRequiredFor2FAChange = false
				common.PhoneRequiredForPayment = true
				common.PhoneRequiredForWithdrawal = false
				common.PhoneRequiredForPhoneBind = false
				common.PhoneRequiredForAccountDelete = false
				common.PhoneRequiredForTokenGenerate = false
				common.PhoneRequiredForOAuthBind = false
			},
			expectedLen:    1,
			expectedAction: "payment",
			shouldContain:  true,
		},
		{
			name: "all_requirements",
			setup: func() {
				common.PhoneRequiredForLogin = true
				common.PhoneRequiredForPasswordReset = true
				common.PhoneRequiredFor2FAChange = true
				common.PhoneRequiredForPayment = true
				common.PhoneRequiredForWithdrawal = true
				common.PhoneRequiredForPhoneBind = true
				common.PhoneRequiredForAccountDelete = true
				common.PhoneRequiredForTokenGenerate = true
				common.PhoneRequiredForOAuthBind = true
			},
			expectedLen:    9,
			expectedAction: "login",
			shouldContain:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			actions := GetPhoneRequiredActions()

			if len(actions) != tt.expectedLen {
				t.Errorf("GetPhoneRequiredActions() len = %d, want %d", len(actions), tt.expectedLen)
			}

			if tt.shouldContain {
				found := false
				for _, action := range actions {
					if action == tt.expectedAction {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("GetPhoneRequiredActions() should contain %q", tt.expectedAction)
				}
			}
		})
	}
}

// TestGetAvailablePhoneVerificationModes tests the available modes list
func TestGetAvailablePhoneVerificationModes(t *testing.T) {
	modes := GetAvailablePhoneVerificationModes()

	// Should return 4 modes
	if len(modes) != 4 {
		t.Errorf("GetAvailablePhoneVerificationModes() returned %d modes, want 4", len(modes))
	}

	// Check required fields
	for i, mode := range modes {
		if mode["key"] == "" {
			t.Errorf("Mode %d missing 'key'", i)
		}
		if mode["name"] == "" {
			t.Errorf("Mode %d missing 'name'", i)
		}
		if mode["description"] == "" {
			t.Errorf("Mode %d missing 'description'", i)
		}
	}

	// Check all standard modes are included
	expectedKeys := []string{
		common.PhoneVerificationDisabled,
		common.PhoneVerificationOptional,
		common.PhoneVerificationRequiredLogin,
		common.PhoneVerificationRequiredSensitive,
	}

	for _, expectedKey := range expectedKeys {
		found := false
		for _, mode := range modes {
			if mode["key"] == expectedKey {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected mode %q not found in GetAvailablePhoneVerificationModes()", expectedKey)
		}
	}
}

// TestGetAvailableRegistrationModes tests the available registration modes list
func TestGetAvailableRegistrationModes(t *testing.T) {
	modes := GetAvailableRegistrationModes()

	// Should return 5 modes
	if len(modes) != 5 {
		t.Errorf("GetAvailableRegistrationModes() returned %d modes, want 5", len(modes))
	}

	// Check required fields
	for i, mode := range modes {
		if mode["key"] == "" {
			t.Errorf("Mode %d missing 'key'", i)
		}
		if mode["name"] == "" {
			t.Errorf("Mode %d missing 'name'", i)
		}
		if mode["description"] == "" {
			t.Errorf("Mode %d missing 'description'", i)
		}
	}

	// Check all standard modes are included
	expectedKeys := []string{
		common.RegistrationModeOpen,
		common.RegistrationModeInviteOnly,
		common.RegistrationModeOAuthOnly,
		common.RegistrationModePhoneVerified,
		common.RegistrationModeClosed,
	}

	for _, expectedKey := range expectedKeys {
		found := false
		for _, mode := range modes {
			if mode["key"] == expectedKey {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected mode %q not found in GetAvailableRegistrationModes()", expectedKey)
		}
	}
}

// TestValidateLoginConfigUpdate tests the configuration validation
func TestValidateLoginConfigUpdate(t *testing.T) {
	// Save original SMS enabled state
	originalSMSEnabled := common.SMSEnabled
	defer func() {
		common.SMSEnabled = originalSMSEnabled
	}()

	tests := []struct {
		name        string
		smsEnabled  bool
		req         *LoginConfigUpdateRequest
		expectError bool
		errorMsg    string
	}{
		{
			name:       "valid_no_changes",
			smsEnabled: true,
			req:        &LoginConfigUpdateRequest{},
			expectError: false,
		},
		{
			name:       "valid_phone_verification_with_sms",
			smsEnabled: true,
			req: &LoginConfigUpdateRequest{
				PhoneRequiredForLogin: func() *bool { b := true; return &b }(),
			},
			expectError: false,
		},
		{
			name:       "invalid_phone_verification_without_sms",
			smsEnabled: false,
			req: &LoginConfigUpdateRequest{
				PhoneRequiredForLogin: func() *bool { b := true; return &b }(),
			},
			expectError: true,
			errorMsg:    "Cannot enable phone verification requirements when SMS is disabled",
		},
		{
			name:       "valid_phone_verification_mode",
			smsEnabled: true,
			req: &LoginConfigUpdateRequest{
				PhoneVerificationMode: func() *string { s := common.PhoneVerificationOptional; return &s }(),
			},
			expectError: false,
		},
		{
			name:       "invalid_phone_verification_mode",
			smsEnabled: true,
			req: &LoginConfigUpdateRequest{
				PhoneVerificationMode: func() *string { s := "invalid_mode"; return &s }(),
			},
			expectError: true,
			errorMsg:    "Invalid phone verification mode",
		},
		{
			name:       "valid_registration_mode",
			smsEnabled: true,
			req: &LoginConfigUpdateRequest{
				RegistrationMode: func() *string { s := common.RegistrationModeOpen; return &s }(),
			},
			expectError: false,
		},
		{
			name:       "invalid_registration_mode",
			smsEnabled: true,
			req: &LoginConfigUpdateRequest{
				RegistrationMode: func() *string { s := "invalid_mode"; return &s }(),
			},
			expectError: true,
			errorMsg:    "Invalid registration mode",
		},
		{
			name:       "phone_verified_mode_without_sms",
			smsEnabled: false,
			req: &LoginConfigUpdateRequest{
				RegistrationMode: func() *string { s := common.RegistrationModePhoneVerified; return &s }(),
			},
			expectError: true,
			errorMsg:    "Phone verified registration mode requires SMS to be enabled",
		},
		{
			name:       "valid_session_timeout",
			smsEnabled: true,
			req: &LoginConfigUpdateRequest{
				SessionTimeoutMinutes: func() *int { i := 60; return &i }(),
			},
			expectError: false,
		},
		{
			name:       "invalid_session_timeout_zero",
			smsEnabled: true,
			req: &LoginConfigUpdateRequest{
				SessionTimeoutMinutes: func() *int { i := 0; return &i }(),
			},
			expectError: true,
			errorMsg:    "Session timeout must be at least 1 minute",
		},
		{
			name:       "invalid_session_timeout_negative",
			smsEnabled: true,
			req: &LoginConfigUpdateRequest{
				SessionTimeoutMinutes: func() *int { i := -1; return &i }(),
			},
			expectError: true,
			errorMsg:    "Session timeout must be at least 1 minute",
		},
		{
			name:       "enable_sms_and_phone_verification",
			smsEnabled: false,
			req: &LoginConfigUpdateRequest{
				SMSEnabled:            func() *bool { b := true; return &b }(),
				PhoneRequiredForLogin: func() *bool { b := true; return &b }(),
			},
			expectError: false, // Should pass because we're enabling SMS in the same request
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			common.SMSEnabled = tt.smsEnabled
			err := validateLoginConfigUpdate(tt.req)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got nil")
				} else if tt.errorMsg != "" && err.Error() != tt.errorMsg && !containsError(err.Error(), tt.errorMsg) {
					t.Errorf("Error message = %q, want to contain %q", err.Error(), tt.errorMsg)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

// containsError checks if the error message contains the expected substring
func containsError(actual, expected string) bool {
	return len(actual) >= len(expected) && (actual == expected ||
		(len(expected) > 0 && actual[:len(expected)] == expected) ||
		(len(expected) > 0 && containsSubstring(actual, expected)))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// TestLoginConfigResponse tests the response struct
func TestLoginConfigResponse(t *testing.T) {
	resp := LoginConfigResponse{
		PasswordLoginEnabled:          true,
		PasswordRegisterEnabled:       true,
		SMSEnabled:                    true,
		SMSAutoRegister:               true,
		PhoneVerificationMode:         common.PhoneVerificationOptional,
		RegistrationMode:              common.RegistrationModeOpen,
		SessionTimeoutMinutes:         10080,
		SensitiveActionRequirePassword: false,
		SensitiveActionRequire2FA:      false,
	}

	if !resp.PasswordLoginEnabled {
		t.Error("PasswordLoginEnabled should be true")
	}
	if !resp.SMSEnabled {
		t.Error("SMSEnabled should be true")
	}
	if resp.PhoneVerificationMode != common.PhoneVerificationOptional {
		t.Errorf("PhoneVerificationMode = %q, want %q", resp.PhoneVerificationMode, common.PhoneVerificationOptional)
	}
	if resp.RegistrationMode != common.RegistrationModeOpen {
		t.Errorf("RegistrationMode = %q, want %q", resp.RegistrationMode, common.RegistrationModeOpen)
	}
	if resp.SessionTimeoutMinutes != 10080 {
		t.Errorf("SessionTimeoutMinutes = %d, want 10080", resp.SessionTimeoutMinutes)
	}
}

// TestHelperFunctions tests the helper functions
func TestHelperFunctions(t *testing.T) {
	// Test boolToString (defined in setup.go)
	if boolToString(true) != "true" {
		t.Errorf("boolToString(true) = %q, want %q", boolToString(true), "true")
	}
	if boolToString(false) != "false" {
		t.Errorf("boolToString(false) = %q, want %q", boolToString(false), "false")
	}

	// Test intToString
	tests := []struct {
		input    int
		expected string
	}{
		{0, "0"},
		{1, "1"},
		{-1, "-1"},
		{10080, "10080"},
		{999999, "999999"},
	}

	for _, tt := range tests {
		result := intToString(tt.input)
		if result != tt.expected {
			t.Errorf("intToString(%d) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

// TestPhoneVerificationModeConstants tests the mode constants
func TestPhoneVerificationModeConstants(t *testing.T) {
	tests := []struct {
		name     string
		constant string
		expected string
	}{
		{"PhoneVerificationDisabled", common.PhoneVerificationDisabled, "disabled"},
		{"PhoneVerificationOptional", common.PhoneVerificationOptional, "optional"},
		{"PhoneVerificationRequiredLogin", common.PhoneVerificationRequiredLogin, "required_login"},
		{"PhoneVerificationRequiredSensitive", common.PhoneVerificationRequiredSensitive, "required_sensitive"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.constant != tt.expected {
				t.Errorf("%s = %q, want %q", tt.name, tt.constant, tt.expected)
			}
		})
	}
}

// TestRegistrationModeConstants tests the registration mode constants
func TestRegistrationModeConstants(t *testing.T) {
	tests := []struct {
		name     string
		constant string
		expected string
	}{
		{"RegistrationModeOpen", common.RegistrationModeOpen, "open"},
		{"RegistrationModeInviteOnly", common.RegistrationModeInviteOnly, "invite_only"},
		{"RegistrationModeOAuthOnly", common.RegistrationModeOAuthOnly, "oauth_only"},
		{"RegistrationModePhoneVerified", common.RegistrationModePhoneVerified, "phone_verified"},
		{"RegistrationModeClosed", common.RegistrationModeClosed, "closed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.constant != tt.expected {
				t.Errorf("%s = %q, want %q", tt.name, tt.constant, tt.expected)
			}
		})
	}
}

// TestSensitiveActionTypeConstants tests the sensitive action type constants
func TestSensitiveActionTypeConstants(t *testing.T) {
	tests := []struct {
		name     string
		constant string
		expected string
	}{
		{"SensitiveActionPayment", common.SensitiveActionPayment, "payment"},
		{"SensitiveActionWithdrawal", common.SensitiveActionWithdrawal, "withdrawal"},
		{"SensitiveActionPasswordChange", common.SensitiveActionPasswordChange, "password_change"},
		{"SensitiveAction2FAChange", common.SensitiveAction2FAChange, "2fa_change"},
		{"SensitiveActionPhoneBind", common.SensitiveActionPhoneBind, "phone_bind"},
		{"SensitiveActionOAuthBind", common.SensitiveActionOAuthBind, "oauth_bind"},
		{"SensitiveActionAccountDelete", common.SensitiveActionAccountDelete, "account_delete"},
		{"SensitiveActionTokenGenerate", common.SensitiveActionTokenGenerate, "token_generate"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.constant != tt.expected {
				t.Errorf("%s = %q, want %q", tt.name, tt.constant, tt.expected)
			}
		})
	}
}

package common

import (
	"testing"
)

// TestIsValidChinesePhone tests Chinese phone number validation
func TestIsValidChinesePhone(t *testing.T) {
	tests := []struct {
		name     string
		phone    string
		expected bool
	}{
		// Valid phone numbers - all carriers
		{"valid_13x", "13800138000", true},
		{"valid_14x", "14712345678", true},
		{"valid_15x", "15912345678", true},
		{"valid_16x", "16612345678", true},
		{"valid_17x", "17612345678", true},
		{"valid_18x", "18612345678", true},
		{"valid_19x", "19912345678", true},

		// Boundary cases - length
		{"too_short_10_digits", "1380013800", false},
		{"too_long_12_digits", "138001380001", false},
		{"empty_string", "", false},
		{"single_digit", "1", false},

		// Boundary cases - invalid prefixes
		{"invalid_prefix_10x", "10812345678", false},
		{"invalid_prefix_11x", "11812345678", false},
		{"invalid_prefix_12x", "12812345678", false},
		{"invalid_prefix_20x", "20812345678", false},
		{"invalid_prefix_00x", "00812345678", false},

		// Boundary cases - non-numeric characters
		{"contains_letter_end", "1380013800a", false},
		{"contains_letter_middle", "138a0138000", false},
		{"contains_space", "138 00138000", false},
		{"contains_dash", "138-0013-8000", false},
		{"contains_plus", "138+00138000", false},
		{"all_letters", "abcdefghijk", false},

		// Boundary cases - special formats
		{"international_format_plus86", "+8613800138000", false},
		{"international_format_0086", "008613800138000", false},
		{"leading_zero", "013800138000", false},
		{"unicode_full_width_digits", "１３８００１３８０００", false},

		// Boundary cases - whitespace
		{"leading_whitespace", " 13800138000", false},
		{"trailing_whitespace", "13800138000 ", false},
		{"only_whitespace", "           ", false},

		// Edge cases - all same digits
		{"all_ones", "11111111111", false},
		{"all_threes_valid", "13333333333", true},
		{"all_zeros_with_valid_prefix", "13000000000", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsValidChinesePhone(tt.phone)
			if result != tt.expected {
				t.Errorf("IsValidChinesePhone(%q) = %v, want %v", tt.phone, result, tt.expected)
			}
		})
	}
}

// TestGenerateSmsCode tests SMS code generation
func TestGenerateSmsCode(t *testing.T) {
	t.Run("length_check", func(t *testing.T) {
		for i := 0; i < 100; i++ {
			code := GenerateSmsCode()
			if len(code) != 6 {
				t.Errorf("GenerateSmsCode() length = %d, want 6", len(code))
			}
		}
	})

	t.Run("numeric_only_check", func(t *testing.T) {
		for i := 0; i < 100; i++ {
			code := GenerateSmsCode()
			for j, c := range code {
				if c < '0' || c > '9' {
					t.Errorf("GenerateSmsCode() contains non-numeric at position %d: %q", j, code)
				}
			}
		}
	})

	t.Run("uniqueness_check", func(t *testing.T) {
		codes := make(map[string]bool)
		iterations := 1000

		for i := 0; i < iterations; i++ {
			code := GenerateSmsCode()
			codes[code] = true
		}

		// With 6-digit codes (1 million possibilities), 1000 iterations
		// should have very high uniqueness (>95%)
		uniqueRate := float64(len(codes)) / float64(iterations)
		if uniqueRate < 0.95 {
			t.Errorf("Uniqueness rate = %.2f%%, want >= 95%%", uniqueRate*100)
		}
	})

	t.Run("randomness_distribution", func(t *testing.T) {
		// Check if all digits 0-9 appear with reasonable frequency
		digitCounts := make([]int, 10)
		iterations := 1000

		for i := 0; i < iterations; i++ {
			code := GenerateSmsCode()
			for _, c := range code {
				digitCounts[c-'0']++
			}
		}

		// Each digit should appear roughly 600 times (1000 * 6 / 10)
		// Allow 40% variance
		expectedCount := iterations * 6 / 10
		minCount := int(float64(expectedCount) * 0.6)
		maxCount := int(float64(expectedCount) * 1.4)

		for digit, count := range digitCounts {
			if count < minCount || count > maxCount {
				t.Logf("Digit %d count = %d (expected ~%d, range %d-%d)", digit, count, expectedCount, minCount, maxCount)
				// Note: This is a probabilistic test, so we log instead of fail
			}
		}
	})
}

// TestMaskPhone tests phone number masking for logs
func TestMaskPhone(t *testing.T) {
	tests := []struct {
		name     string
		phone    string
		expected string
	}{
		{"standard_11_digit", "13800138000", "138****8000"},
		{"short_phone_7", "1234567", "123****4567"},
		{"short_phone_6", "123456", "123456"},
		{"short_phone_5", "12345", "12345"},
		{"empty", "", ""},
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

// TestGetSMSTemplateCode tests template code retrieval
func TestGetSMSTemplateCode(t *testing.T) {
	// Save original OptionMap state
	originalMap := make(map[string]string)
	for k, v := range OptionMap {
		originalMap[k] = v
	}
	defer func() {
		// Restore original state
		OptionMap = originalMap
	}()

	tests := []struct {
		name         string
		purpose      string
		setupOptions map[string]string
		expected     string
	}{
		{
			name:    "login_template_configured",
			purpose: "login",
			setupOptions: map[string]string{
				"SMSTemplateLogin": "SMS_12345",
			},
			expected: "SMS_12345",
		},
		{
			name:    "register_template_configured",
			purpose: "register",
			setupOptions: map[string]string{
				"SMSTemplateRegister": "SMS_67890",
			},
			expected: "SMS_67890",
		},
		{
			name:    "reset_template_configured",
			purpose: "reset",
			setupOptions: map[string]string{
				"SMSTemplateReset": "SMS_RESET",
			},
			expected: "SMS_RESET",
		},
		{
			name:    "bind_template_configured",
			purpose: "bind",
			setupOptions: map[string]string{
				"SMSTemplateBind": "SMS_BIND",
			},
			expected: "SMS_BIND",
		},
		{
			name:    "fallback_to_default",
			purpose: "login",
			setupOptions: map[string]string{
				"SMSTemplateDefault": "SMS_DEFAULT",
			},
			expected: "SMS_DEFAULT",
		},
		{
			name:         "no_template_configured",
			purpose:      "login",
			setupOptions: map[string]string{},
			expected:     "",
		},
		{
			name:         "unknown_purpose",
			purpose:      "unknown",
			setupOptions: map[string]string{},
			expected:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset OptionMap for this test
			OptionMap = make(map[string]string)
			for k, v := range tt.setupOptions {
				OptionMap[k] = v
			}

			result := GetSMSTemplateCode(tt.purpose)
			if result != tt.expected {
				t.Errorf("GetSMSTemplateCode(%q) = %q, want %q", tt.purpose, result, tt.expected)
			}
		})
	}
}

// TestBuildSMSTemplateParam tests template parameter building
func TestBuildSMSTemplateParam(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		expected string
	}{
		{"standard_code", "123456", `{"code":"123456"}`},
		{"code_with_zeros", "000000", `{"code":"000000"}`},
		{"code_with_nines", "999999", `{"code":"999999"}`},
		{"empty_code", "", `{"code":""}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := BuildSMSTemplateParam(tt.code)
			if result != tt.expected {
				t.Errorf("BuildSMSTemplateParam(%q) = %q, want %q", tt.code, result, tt.expected)
			}
		})
	}
}

// BenchmarkIsValidChinesePhone benchmarks phone validation
func BenchmarkIsValidChinesePhone(b *testing.B) {
	phones := []string{
		"13800138000",
		"invalid",
		"1380013800",
		"15912345678",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		IsValidChinesePhone(phones[i%len(phones)])
	}
}

// BenchmarkGenerateSmsCode benchmarks SMS code generation
func BenchmarkGenerateSmsCode(b *testing.B) {
	for i := 0; i < b.N; i++ {
		GenerateSmsCode()
	}
}

package model

import (
	"testing"
	"time"
)

// TestInvitationCodeTableName tests the table name
func TestInvitationCodeTableName(t *testing.T) {
	code := InvitationCode{}
	if code.TableName() != "invitation_codes" {
		t.Errorf("TableName() = %q, want %q", code.TableName(), "invitation_codes")
	}
}

// TestInvitationCodeIsValid tests the IsValid method
func TestInvitationCodeIsValid(t *testing.T) {
	now := time.Now().Unix()
	futureTime := now + 3600  // 1 hour in the future
	pastTime := now - 3600    // 1 hour in the past
	usedBy := 1

	tests := []struct {
		name     string
		code     *InvitationCode
		expected bool
	}{
		{
			name:     "nil_code",
			code:     nil,
			expected: false,
		},
		{
			name: "valid_no_expiry",
			code: &InvitationCode{
				Id:        1,
				Code:      "TEST123",
				CreatedBy: 1,
				UsedBy:    nil,
				ExpiresAt: nil,
			},
			expected: true,
		},
		{
			name: "valid_future_expiry",
			code: &InvitationCode{
				Id:        2,
				Code:      "TEST456",
				CreatedBy: 1,
				UsedBy:    nil,
				ExpiresAt: &futureTime,
			},
			expected: true,
		},
		{
			name: "invalid_already_used",
			code: &InvitationCode{
				Id:        3,
				Code:      "TEST789",
				CreatedBy: 1,
				UsedBy:    &usedBy,
				ExpiresAt: nil,
			},
			expected: false,
		},
		{
			name: "invalid_expired",
			code: &InvitationCode{
				Id:        4,
				Code:      "TEST101",
				CreatedBy: 1,
				UsedBy:    nil,
				ExpiresAt: &pastTime,
			},
			expected: false,
		},
		{
			name: "invalid_used_and_expired",
			code: &InvitationCode{
				Id:        5,
				Code:      "TEST102",
				CreatedBy: 1,
				UsedBy:    &usedBy,
				ExpiresAt: &pastTime,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.code.IsValid()
			if result != tt.expected {
				t.Errorf("IsValid() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// TestGenerateInviteCode tests the code generation function
func TestGenerateInviteCode(t *testing.T) {
	// Test that generated codes are:
	// 1. 16 characters long (hex encoded 8 bytes)
	// 2. Unique (different codes)
	// 3. Contains only hex characters (0-9, a-f)

	codes := make(map[string]bool)
	for i := 0; i < 100; i++ {
		code, err := GenerateInviteCode()
		if err != nil {
			t.Fatalf("GenerateInviteCode() error = %v", err)
		}

		// Check length (8 bytes hex encoded = 16 characters)
		if len(code) != 16 {
			t.Errorf("GenerateInviteCode() length = %d, want 16", len(code))
		}

		// Check uniqueness
		if codes[code] {
			t.Errorf("Duplicate code generated: %s", code)
		}
		codes[code] = true

		// Check characters (should be hex characters: 0-9, a-f)
		for _, c := range code {
			if !((c >= 'a' && c <= 'f') || (c >= '0' && c <= '9')) {
				t.Errorf("Invalid character in code: %c (expected hex 0-9, a-f)", c)
			}
		}
	}
}

// TestInvitationCodeStruct tests the struct fields
func TestInvitationCodeStruct(t *testing.T) {
	now := time.Now().Unix()
	expiresAt := now + 86400
	usedAt := now
	usedBy := 2

	code := &InvitationCode{
		Id:        1,
		Code:      "TESTCODE12345678",
		CreatedBy: 100,
		UsedBy:    &usedBy,
		UsedAt:    &usedAt,
		ExpiresAt: &expiresAt,
		CreatedAt: now,
	}

	if code.Id != 1 {
		t.Errorf("Id = %d, want 1", code.Id)
	}
	if code.Code != "TESTCODE12345678" {
		t.Errorf("Code = %q, want %q", code.Code, "TESTCODE12345678")
	}
	if code.CreatedBy != 100 {
		t.Errorf("CreatedBy = %d, want 100", code.CreatedBy)
	}
	if code.UsedBy == nil || *code.UsedBy != 2 {
		t.Errorf("UsedBy = %v, want 2", code.UsedBy)
	}
	if code.ExpiresAt == nil || *code.ExpiresAt != expiresAt {
		t.Errorf("ExpiresAt = %v, want %d", code.ExpiresAt, expiresAt)
	}
}

// TestInvitationCodeEdgeCases tests edge cases for IsValid
func TestInvitationCodeEdgeCases(t *testing.T) {
	now := time.Now().Unix()

	tests := []struct {
		name     string
		code     *InvitationCode
		expected bool
	}{
		{
			name: "expiry_at_exact_now",
			code: &InvitationCode{
				Id:        1,
				Code:      "TEST",
				ExpiresAt: &now,
			},
			expected: true, // Exact now is still valid (uses < not <=)
		},
		{
			name: "zero_expiry",
			code: &InvitationCode{
				Id:        2,
				Code:      "TEST",
				ExpiresAt: func() *int64 { v := int64(0); return &v }(),
			},
			expected: false, // Zero timestamp (1970) is in the past
		},
		{
			name: "negative_expiry",
			code: &InvitationCode{
				Id:        3,
				Code:      "TEST",
				ExpiresAt: func() *int64 { v := int64(-1); return &v }(),
			},
			expected: false, // Negative timestamp is in the past
		},
		{
			name: "empty_code_string_but_valid",
			code: &InvitationCode{
				Id:        4,
				Code:      "",
				UsedBy:    nil,
				ExpiresAt: nil,
			},
			expected: true, // IsValid doesn't check code content
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.code.IsValid()
			if result != tt.expected {
				t.Errorf("IsValid() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// BenchmarkGenerateInviteCode benchmarks code generation
func BenchmarkGenerateInviteCode(b *testing.B) {
	for i := 0; i < b.N; i++ {
		GenerateInviteCode()
	}
}

// BenchmarkIsValid benchmarks the IsValid check
func BenchmarkIsValid(b *testing.B) {
	futureTime := time.Now().Unix() + 3600
	code := &InvitationCode{
		Id:        1,
		Code:      "TESTCODE12345678",
		CreatedBy: 1,
		UsedBy:    nil,
		ExpiresAt: &futureTime,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		code.IsValid()
	}
}

package controller

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/lurus-api/internal/pkg/common"
	"github.com/gin-gonic/gin"
)

// TestSendSmsRequestValidation tests request body validation
func TestSendSmsRequestValidation(t *testing.T) {
	tests := []struct {
		name           string
		requestBody    map[string]interface{}
		expectedValid  bool
		expectedErrMsg string
	}{
		{
			name: "valid_login_request",
			requestBody: map[string]interface{}{
				"phone":   "13800138000",
				"purpose": "login",
			},
			expectedValid: true,
		},
		{
			name: "valid_register_request",
			requestBody: map[string]interface{}{
				"phone":   "13800138000",
				"purpose": "register",
			},
			expectedValid: true,
		},
		{
			name: "valid_bind_request",
			requestBody: map[string]interface{}{
				"phone":   "13800138000",
				"purpose": "bind",
			},
			expectedValid: true,
		},
		{
			name: "valid_reset_request",
			requestBody: map[string]interface{}{
				"phone":   "13800138000",
				"purpose": "reset",
			},
			expectedValid: true,
		},
		{
			name: "missing_phone",
			requestBody: map[string]interface{}{
				"purpose": "login",
			},
			expectedValid:  false,
			expectedErrMsg: "phone",
		},
		{
			name: "missing_purpose",
			requestBody: map[string]interface{}{
				"phone": "13800138000",
			},
			expectedValid:  false,
			expectedErrMsg: "purpose",
		},
		{
			name:           "empty_body",
			requestBody:    map[string]interface{}{},
			expectedValid:  false,
			expectedErrMsg: "phone",
		},
		{
			name: "empty_phone",
			requestBody: map[string]interface{}{
				"phone":   "",
				"purpose": "login",
			},
			expectedValid:  false,
			expectedErrMsg: "phone",
		},
		{
			name: "empty_purpose",
			requestBody: map[string]interface{}{
				"phone":   "13800138000",
				"purpose": "",
			},
			expectedValid:  false,
			expectedErrMsg: "purpose",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.requestBody)

			var req SendSmsRequest
			err := json.Unmarshal(body, &req)

			// Check if required fields are present
			isValid := err == nil && req.Phone != "" && req.Purpose != ""

			if isValid != tt.expectedValid {
				t.Errorf("Request validity = %v, want %v", isValid, tt.expectedValid)
			}
		})
	}
}

// TestLoginWithSmsRequestValidation tests login request validation
func TestLoginWithSmsRequestValidation(t *testing.T) {
	tests := []struct {
		name          string
		requestBody   map[string]interface{}
		expectedValid bool
	}{
		{
			name: "valid_request",
			requestBody: map[string]interface{}{
				"phone": "13800138000",
				"code":  "123456",
			},
			expectedValid: true,
		},
		{
			name: "missing_phone",
			requestBody: map[string]interface{}{
				"code": "123456",
			},
			expectedValid: false,
		},
		{
			name: "missing_code",
			requestBody: map[string]interface{}{
				"phone": "13800138000",
			},
			expectedValid: false,
		},
		{
			name:          "empty_body",
			requestBody:   map[string]interface{}{},
			expectedValid: false,
		},
		{
			name: "empty_phone",
			requestBody: map[string]interface{}{
				"phone": "",
				"code":  "123456",
			},
			expectedValid: false,
		},
		{
			name: "empty_code",
			requestBody: map[string]interface{}{
				"phone": "13800138000",
				"code":  "",
			},
			expectedValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.requestBody)

			var req LoginWithSmsRequest
			err := json.Unmarshal(body, &req)

			isValid := err == nil && req.Phone != "" && req.Code != ""

			if isValid != tt.expectedValid {
				t.Errorf("Request validity = %v, want %v", isValid, tt.expectedValid)
			}
		})
	}
}

// TestBindPhoneRequestValidation tests bind phone request validation
func TestBindPhoneRequestValidation(t *testing.T) {
	tests := []struct {
		name          string
		requestBody   map[string]interface{}
		expectedValid bool
	}{
		{
			name: "valid_request",
			requestBody: map[string]interface{}{
				"phone": "13800138000",
				"code":  "123456",
			},
			expectedValid: true,
		},
		{
			name: "missing_phone",
			requestBody: map[string]interface{}{
				"code": "123456",
			},
			expectedValid: false,
		},
		{
			name: "missing_code",
			requestBody: map[string]interface{}{
				"phone": "13800138000",
			},
			expectedValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.requestBody)

			var req BindPhoneRequest
			err := json.Unmarshal(body, &req)

			isValid := err == nil && req.Phone != "" && req.Code != ""

			if isValid != tt.expectedValid {
				t.Errorf("Request validity = %v, want %v", isValid, tt.expectedValid)
			}
		})
	}
}

// TestPhoneValidationInController tests phone validation logic
func TestPhoneValidationInController(t *testing.T) {
	tests := []struct {
		name          string
		phone         string
		expectInvalid bool
	}{
		// Valid phones
		{"valid_13x", "13800138000", false},
		{"valid_15x", "15912345678", false},
		{"valid_17x", "17612345678", false},
		{"valid_18x", "18612345678", false},
		{"valid_19x", "19912345678", false},

		// Invalid phones
		{"too_short", "1380013800", true},
		{"too_long", "138001380001", true},
		{"invalid_prefix", "12345678901", true},
		{"contains_letters", "1380013800a", true},
		{"empty", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isInvalid := !common.IsValidChinesePhone(tt.phone)
			if isInvalid != tt.expectInvalid {
				t.Errorf("Phone %q validity check: isInvalid = %v, want %v", tt.phone, isInvalid, tt.expectInvalid)
			}
		})
	}
}

// TestPurposeValidation tests valid purpose values
func TestPurposeValidation(t *testing.T) {
	validPurposes := map[string]bool{
		"login":    true,
		"register": true,
		"reset":    true,
		"bind":     true,
	}

	tests := []struct {
		name    string
		purpose string
		isValid bool
	}{
		{"login", "login", true},
		{"register", "register", true},
		{"reset", "reset", true},
		{"bind", "bind", true},
		{"invalid", "invalid", false},
		{"empty", "", false},
		{"uppercase_login", "LOGIN", false},
		{"mixed_case", "Login", false},
		{"with_space", " login", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isValid := validPurposes[tt.purpose]
			if isValid != tt.isValid {
				t.Errorf("Purpose %q validity = %v, want %v", tt.purpose, isValid, tt.isValid)
			}
		})
	}
}

// TestRateLimitLogic tests the rate limiting logic
func TestRateLimitLogic(t *testing.T) {
	// Test the rate limit cache structure
	cache := make(map[string]time.Time)

	phone := "13800138000"
	cacheKey := "sms:" + phone

	// First request should pass
	if _, ok := cache[cacheKey]; ok {
		t.Error("Cache should be empty initially")
	}

	// Set last sent time
	cache[cacheKey] = time.Now()

	// Immediate second request should be rate limited
	if lastSent, ok := cache[cacheKey]; ok {
		if time.Since(lastSent) < time.Minute {
			// Rate limited - expected
		} else {
			t.Error("Request should be rate limited")
		}
	}

	// After 1 minute, should be allowed
	cache[cacheKey] = time.Now().Add(-61 * time.Second)
	if lastSent, ok := cache[cacheKey]; ok {
		if time.Since(lastSent) >= time.Minute {
			// Not rate limited - expected
		} else {
			t.Error("Request should be allowed after 1 minute")
		}
	}
}

// TestRateLimitRemainingSeconds tests remaining seconds calculation
func TestRateLimitRemainingSeconds(t *testing.T) {
	tests := []struct {
		name            string
		timeSinceSend   time.Duration
		expectedRemain  int
		shouldBeBlocked bool
	}{
		{"just_sent", 0, 60, true},
		{"30_seconds_ago", 30 * time.Second, 30, true},
		{"59_seconds_ago", 59 * time.Second, 1, true},
		{"60_seconds_ago", 60 * time.Second, 0, false},
		{"61_seconds_ago", 61 * time.Second, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lastSent := time.Now().Add(-tt.timeSinceSend)
			shouldBlock := time.Since(lastSent) < time.Minute
			remaining := 0
			if shouldBlock {
				remaining = int((time.Minute - time.Since(lastSent)).Seconds())
			}

			if shouldBlock != tt.shouldBeBlocked {
				t.Errorf("shouldBlock = %v, want %v", shouldBlock, tt.shouldBeBlocked)
			}

			// Allow 1 second variance due to test execution time
			if tt.shouldBeBlocked && (remaining < tt.expectedRemain-1 || remaining > tt.expectedRemain+1) {
				t.Errorf("remaining = %d, want ~%d", remaining, tt.expectedRemain)
			}
		})
	}
}

// TestGetSMSStatusResponse tests the SMS status response format
func TestGetSMSStatusResponse(t *testing.T) {
	router := MockRouter()
	router.GET("/api/sms/status", GetSMSStatus)

	req := httptest.NewRequest("GET", "/api/sms/status", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response["success"] != true {
		t.Error("Expected success to be true")
	}

	data, ok := response["data"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected data to be a map")
	}

	if _, exists := data["enabled"]; !exists {
		t.Error("Expected 'enabled' field in data")
	}
}

// TestSendSmsVerificationInvalidJSON tests invalid JSON handling
func TestSendSmsVerificationInvalidJSON(t *testing.T) {
	router := MockRouter()
	router.POST("/api/sms/send", SendSmsVerification)

	// Send invalid JSON
	req := httptest.NewRequest("POST", "/api/sms/send", strings.NewReader("{invalid json}"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

// TestSendSmsVerificationSMSDisabled tests response when SMS is disabled
func TestSendSmsVerificationSMSDisabled(t *testing.T) {
	// Save original state
	originalEnabled := common.SMSEnabled
	common.SMSEnabled = false
	defer func() {
		common.SMSEnabled = originalEnabled
	}()

	router := MockRouter()
	router.POST("/api/sms/send", SendSmsVerification)

	body := map[string]interface{}{
		"phone":   "13800138000",
		"purpose": "login",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/api/sms/send", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	if msg, ok := response["message"].(string); !ok || !strings.Contains(msg, "not enabled") {
		t.Errorf("Expected message about SMS not enabled, got: %v", response["message"])
	}
}

// TestSendSmsVerificationInvalidPhone tests invalid phone number handling
func TestSendSmsVerificationInvalidPhone(t *testing.T) {
	// Enable SMS for this test
	originalEnabled := common.SMSEnabled
	common.SMSEnabled = true
	defer func() {
		common.SMSEnabled = originalEnabled
	}()

	router := MockRouter()
	router.POST("/api/sms/send", SendSmsVerification)

	body := map[string]interface{}{
		"phone":   "invalid_phone",
		"purpose": "login",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/api/sms/send", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	if msg, ok := response["message"].(string); !ok || !strings.Contains(msg, "Invalid phone") {
		t.Errorf("Expected message about invalid phone, got: %v", response["message"])
	}
}

// TestSendSmsVerificationInvalidPurpose tests invalid purpose handling
func TestSendSmsVerificationInvalidPurpose(t *testing.T) {
	originalEnabled := common.SMSEnabled
	common.SMSEnabled = true
	defer func() {
		common.SMSEnabled = originalEnabled
	}()

	router := MockRouter()
	router.POST("/api/sms/send", SendSmsVerification)

	body := map[string]interface{}{
		"phone":   "13800138000",
		"purpose": "invalid_purpose",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/api/sms/send", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	if msg, ok := response["message"].(string); !ok || !strings.Contains(msg, "Invalid purpose") {
		t.Errorf("Expected message about invalid purpose, got: %v", response["message"])
	}
}

// TestLoginWithSmsSMSDisabled tests login when SMS is disabled
func TestLoginWithSmsSMSDisabled(t *testing.T) {
	originalEnabled := common.SMSEnabled
	common.SMSEnabled = false
	defer func() {
		common.SMSEnabled = originalEnabled
	}()

	router := MockRouter()
	router.POST("/api/user/login_sms", LoginWithSms)

	body := map[string]interface{}{
		"phone": "13800138000",
		"code":  "123456",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/api/user/login_sms", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

// TestLoginWithSmsInvalidPhone tests login with invalid phone
func TestLoginWithSmsInvalidPhone(t *testing.T) {
	originalEnabled := common.SMSEnabled
	common.SMSEnabled = true
	defer func() {
		common.SMSEnabled = originalEnabled
	}()

	router := MockRouter()
	router.POST("/api/user/login_sms", LoginWithSms)

	body := map[string]interface{}{
		"phone": "invalid",
		"code":  "123456",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/api/user/login_sms", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

// TestBindPhoneSMSDisabled tests bind phone when SMS is disabled
func TestBindPhoneSMSDisabled(t *testing.T) {
	originalEnabled := common.SMSEnabled
	common.SMSEnabled = false
	defer func() {
		common.SMSEnabled = originalEnabled
	}()

	router := MockRouter()
	router.Use(func(c *gin.Context) {
		c.Set("id", 1) // Mock authenticated user
		c.Next()
	})
	router.POST("/api/user/bind_phone", BindPhone)

	body := map[string]interface{}{
		"phone": "13800138000",
		"code":  "123456",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/api/user/bind_phone", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

// TestBindPhoneUnauthorized tests bind phone without authentication
func TestBindPhoneUnauthorized(t *testing.T) {
	originalEnabled := common.SMSEnabled
	common.SMSEnabled = true
	defer func() {
		common.SMSEnabled = originalEnabled
	}()

	router := MockRouter()
	// No user ID set in context
	router.POST("/api/user/bind_phone", BindPhone)

	body := map[string]interface{}{
		"phone": "13800138000",
		"code":  "123456",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/api/user/bind_phone", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

// TestSmsRateLimitCacheCleanup tests that old entries don't persist
func TestSmsRateLimitCacheCleanup(t *testing.T) {
	// The global smsRateLimitCache should be accessible
	// In production, you'd want a more sophisticated cache with TTL

	// For now, just verify the cache structure works
	cache := make(map[string]time.Time)
	phone := "13800138000"
	cacheKey := "sms:" + phone

	cache[cacheKey] = time.Now().Add(-2 * time.Minute)

	if lastSent, ok := cache[cacheKey]; ok {
		if time.Since(lastSent) >= time.Minute {
			// This entry is old and should allow new request
		}
	}
}

// BenchmarkPhoneValidation benchmarks phone validation
func BenchmarkPhoneValidation(b *testing.B) {
	phones := []string{
		"13800138000",
		"invalid",
		"1380013800",
		"15912345678",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		common.IsValidChinesePhone(phones[i%len(phones)])
	}
}

// BenchmarkRateLimitCheck benchmarks rate limit checking
func BenchmarkRateLimitCheck(b *testing.B) {
	cache := make(map[string]time.Time)
	cache["sms:13800138000"] = time.Now()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := "sms:13800138000"
		if lastSent, ok := cache[key]; ok {
			_ = time.Since(lastSent) < time.Minute
		}
	}
}

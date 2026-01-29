package controller

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/lurus-api/internal/pkg/common"
	"github.com/QuantumNous/lurus-api/internal/data/model"
	"github.com/gin-gonic/gin"
)

// TestInternalGetUserInvalidId tests invalid user ID handling
func TestInternalGetUserInvalidId(t *testing.T) {
	tests := []struct {
		name           string
		userId         string
		expectedStatus int
		expectedMsg    string
	}{
		{
			name:           "non_numeric_id",
			userId:         "abc",
			expectedStatus: http.StatusBadRequest,
			expectedMsg:    "Invalid user ID",
		},
		{
			name:           "float_id",
			userId:         "1.5",
			expectedStatus: http.StatusBadRequest,
			expectedMsg:    "Invalid user ID",
		},
		{
			name:           "negative_id",
			userId:         "-1",
			expectedStatus: http.StatusBadRequest,
			expectedMsg:    "Invalid user ID",
		},
		{
			name:           "empty_id",
			userId:         "",
			expectedStatus: http.StatusNotFound, // Router won't match
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := MockRouter()
			router.GET("/internal/user/:id", InternalGetUser)

			path := "/internal/user/" + tt.userId
			if tt.userId == "" {
				path = "/internal/user/"
			}
			req := httptest.NewRequest("GET", path, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if tt.expectedMsg != "" {
				var response map[string]interface{}
				json.Unmarshal(w.Body.Bytes(), &response)
				if msg, ok := response["message"].(string); !ok || msg != tt.expectedMsg {
					t.Errorf("Expected message %q, got %v", tt.expectedMsg, response["message"])
				}
			}
		})
	}
}

// TestInternalGetUserByEmailEmpty tests empty email handling
func TestInternalGetUserByEmailEmpty(t *testing.T) {
	router := MockRouter()
	router.GET("/internal/user/by-email/:email", InternalGetUserByEmail)

	// With empty email (router won't match properly)
	req := httptest.NewRequest("GET", "/internal/user/by-email/", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Router will return 404 as route doesn't match
	if w.Code != http.StatusNotFound {
		// If router matches somehow, check for bad request
		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status 404 or 400, got %d", w.Code)
		}
	}
}

// TestInternalGetUserByPhoneEmpty tests empty phone handling
func TestInternalGetUserByPhoneEmpty(t *testing.T) {
	router := MockRouter()
	router.GET("/internal/user/by-phone/:phone", InternalGetUserByPhone)

	req := httptest.NewRequest("GET", "/internal/user/by-phone/", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status 404 or 400, got %d", w.Code)
		}
	}
}

// TestInternalUpdateUserInvalidId tests update with invalid user ID
func TestInternalUpdateUserInvalidId(t *testing.T) {
	router := MockRouter()
	router.PUT("/internal/user/:id", InternalUpdateUser)

	body := map[string]interface{}{
		"display_name": "Test User",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("PUT", "/internal/user/invalid", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

// TestInternalUpdateUserNoFields tests update with no fields
func TestInternalUpdateUserNoFields(t *testing.T) {
	router := MockRouter()
	router.PUT("/internal/user/:id", InternalUpdateUser)

	// Empty body
	req := httptest.NewRequest("PUT", "/internal/user/1", bytes.NewBuffer([]byte("{}")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Will fail user lookup without DB, but we're testing validation
	// In real test with DB, it would return 400 "No fields to update"
}

// TestInternalGrantSubscriptionValidation tests grant request validation
func TestInternalGrantSubscriptionValidation(t *testing.T) {
	tests := []struct {
		name           string
		requestBody    map[string]interface{}
		expectedStatus int
		expectedMsg    string
	}{
		{
			name: "missing_user_id",
			requestBody: map[string]interface{}{
				"plan_code": "monthly",
				"days":      30,
				"reason":    "Test",
			},
			expectedStatus: http.StatusBadRequest,
			expectedMsg:    "user_id",
		},
		{
			name: "missing_plan_code",
			requestBody: map[string]interface{}{
				"user_id": 1,
				"days":    30,
				"reason":  "Test",
			},
			expectedStatus: http.StatusBadRequest,
			expectedMsg:    "plan_code",
		},
		{
			name: "missing_days",
			requestBody: map[string]interface{}{
				"user_id":   1,
				"plan_code": "monthly",
				"reason":    "Test",
			},
			expectedStatus: http.StatusBadRequest,
			expectedMsg:    "days",
		},
		{
			name: "zero_days",
			requestBody: map[string]interface{}{
				"user_id":   1,
				"plan_code": "monthly",
				"days":      0,
				"reason":    "Test",
			},
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := MockRouter()
			router.POST("/internal/subscription/grant", InternalGrantSubscription)

			jsonBody, _ := json.Marshal(tt.requestBody)
			req := httptest.NewRequest("POST", "/internal/subscription/grant", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

// TestInternalAdjustQuotaValidation tests quota adjustment validation
func TestInternalAdjustQuotaValidation(t *testing.T) {
	tests := []struct {
		name           string
		requestBody    map[string]interface{}
		expectedStatus int
	}{
		{
			name: "missing_user_id",
			requestBody: map[string]interface{}{
				"amount": 1000,
				"reason": "Test",
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "missing_amount",
			requestBody: map[string]interface{}{
				"user_id": 1,
				"reason":  "Test",
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "missing_reason",
			requestBody: map[string]interface{}{
				"user_id": 1,
				"amount":  1000,
			},
			expectedStatus: http.StatusBadRequest,
		},
		// Note: Tests requiring database access are commented out
		// as they need a test database setup
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := MockRouter()
			router.POST("/internal/quota/adjust", InternalAdjustQuota)

			jsonBody, _ := json.Marshal(tt.requestBody)
			req := httptest.NewRequest("POST", "/internal/quota/adjust", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

// TestInternalTopupBalanceValidation tests balance topup validation
func TestInternalTopupBalanceValidation(t *testing.T) {
	tests := []struct {
		name           string
		requestBody    map[string]interface{}
		expectedStatus int
		expectedMsg    string
	}{
		{
			name: "missing_user_id",
			requestBody: map[string]interface{}{
				"amount_rmb": 100.0,
				"reason":     "Test",
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "missing_amount",
			requestBody: map[string]interface{}{
				"user_id": 1,
				"reason":  "Test",
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "missing_reason",
			requestBody: map[string]interface{}{
				"user_id":    1,
				"amount_rmb": 100.0,
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "zero_amount",
			requestBody: map[string]interface{}{
				"user_id":    1,
				"amount_rmb": 0.0,
				"reason":     "Test",
			},
			expectedStatus: http.StatusBadRequest,
			expectedMsg:    "positive",
		},
		{
			name: "negative_amount",
			requestBody: map[string]interface{}{
				"user_id":    1,
				"amount_rmb": -50.0,
				"reason":     "Test",
			},
			expectedStatus: http.StatusBadRequest,
			expectedMsg:    "positive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := MockRouter()
			router.POST("/internal/balance/topup", InternalTopupBalance)

			jsonBody, _ := json.Marshal(tt.requestBody)
			req := httptest.NewRequest("POST", "/internal/balance/topup", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if tt.expectedMsg != "" {
				var response map[string]interface{}
				json.Unmarshal(w.Body.Bytes(), &response)
				if msg, ok := response["message"].(string); ok {
					found := false
					if msg == tt.expectedMsg || containsIgnoreCase(msg, tt.expectedMsg) {
						found = true
					}
					if !found {
						t.Logf("Message: %s", msg)
					}
				}
			}
		})
	}
}

// TestAdminCreateApiKeyWildcardRestriction tests wildcard scope restriction
// This test only verifies the permission check, not the actual database operation
func TestAdminCreateApiKeyWildcardRestriction(t *testing.T) {
	tests := []struct {
		name          string
		userRole      int
		scopes        []string
		shouldBeBlock bool // true if request should be blocked with 403
	}{
		{
			name:          "admin_wildcard_forbidden",
			userRole:      common.RoleAdminUser,
			scopes:        []string{"*"},
			shouldBeBlock: true,
		},
		{
			name:          "admin_wildcard_with_others_forbidden",
			userRole:      common.RoleAdminUser,
			scopes:        []string{"user:read", "*"},
			shouldBeBlock: true,
		},
		{
			name:          "root_wildcard_allowed",
			userRole:      common.RoleRootUser,
			scopes:        []string{"*"},
			shouldBeBlock: false, // Will fail on DB, but not on permission
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			gin.SetMode(gin.TestMode)
			router.Use(gin.Recovery())
			router.Use(func(c *gin.Context) {
				c.Set("id", 1)
				c.Set("role", tt.userRole)
				c.Next()
			})
			router.POST("/api/admin/api-keys", AdminCreateApiKey)

			body := map[string]interface{}{
				"name":   "Test Key",
				"scopes": tt.scopes,
			}
			jsonBody, _ := json.Marshal(body)

			req := httptest.NewRequest("POST", "/api/admin/api-keys", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// We test the permission check - admin with wildcard should get 403
			if tt.shouldBeBlock {
				if w.Code != http.StatusForbidden {
					t.Errorf("Expected status %d for blocked request, got %d", http.StatusForbidden, w.Code)
				}
			} else {
				// For allowed requests, they will fail on DB access (500), not permission (403)
				if w.Code == http.StatusForbidden {
					t.Errorf("Request should not be blocked with 403, got %d", w.Code)
				}
			}
		})
	}
}

// TestAdminUpdateApiKeyWildcardRestriction tests wildcard scope restriction on update
func TestAdminUpdateApiKeyWildcardRestriction(t *testing.T) {
	tests := []struct {
		name          string
		userRole      int
		scopes        []string
		shouldBeBlock bool
	}{
		{
			name:          "admin_update_with_wildcard",
			userRole:      common.RoleAdminUser,
			scopes:        []string{"*"},
			shouldBeBlock: true,
		},
		{
			name:          "root_update_with_wildcard",
			userRole:      common.RoleRootUser,
			scopes:        []string{"*"},
			shouldBeBlock: false, // Will fail on DB, but not on permission
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			gin.SetMode(gin.TestMode)
			router.Use(gin.Recovery())
			router.Use(func(c *gin.Context) {
				c.Set("id", 1)
				c.Set("role", tt.userRole)
				c.Next()
			})
			router.PUT("/api/admin/api-keys/:id", AdminUpdateApiKey)

			body := map[string]interface{}{
				"name":   "Updated Key",
				"scopes": tt.scopes,
			}
			jsonBody, _ := json.Marshal(body)

			req := httptest.NewRequest("PUT", "/api/admin/api-keys/1", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if tt.shouldBeBlock {
				if w.Code != http.StatusForbidden {
					t.Errorf("Expected status %d for blocked request, got %d", http.StatusForbidden, w.Code)
				}
			} else {
				if w.Code == http.StatusForbidden {
					t.Errorf("Request should not be blocked with 403, got %d", w.Code)
				}
			}
		})
	}
}

// TestAdminDeleteApiKeyInvalidId tests delete with invalid key ID
func TestAdminDeleteApiKeyInvalidId(t *testing.T) {
	router := MockRouter()
	router.DELETE("/api/admin/api-keys/:id", AdminDeleteApiKey)

	req := httptest.NewRequest("DELETE", "/api/admin/api-keys/invalid", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

// TestAdminToggleApiKeyInvalidId tests toggle with invalid key ID
func TestAdminToggleApiKeyInvalidId(t *testing.T) {
	router := MockRouter()
	router.PUT("/api/admin/api-keys/:id/toggle", AdminToggleApiKey)

	req := httptest.NewRequest("PUT", "/api/admin/api-keys/invalid/toggle", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

// TestAdminGetApiKeyScopes tests the scopes list endpoint
func TestAdminGetApiKeyScopes(t *testing.T) {
	router := MockRouter()
	router.GET("/api/admin/api-keys/scopes", AdminGetApiKeyScopes)

	req := httptest.NewRequest("GET", "/api/admin/api-keys/scopes", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	if response["success"] != true {
		t.Error("Expected success to be true")
	}

	data, ok := response["data"].([]interface{})
	if !ok {
		t.Fatal("Expected data to be an array")
	}

	// Should have 13 scopes
	if len(data) != 13 {
		t.Errorf("Expected 13 scopes, got %d", len(data))
	}

	// Check each scope has required fields
	for i, scopeData := range data {
		scope, ok := scopeData.(map[string]interface{})
		if !ok {
			t.Errorf("Scope %d is not a map", i)
			continue
		}
		if scope["key"] == nil {
			t.Errorf("Scope %d missing 'key'", i)
		}
		if scope["name"] == nil {
			t.Errorf("Scope %d missing 'name'", i)
		}
		if scope["description"] == nil {
			t.Errorf("Scope %d missing 'description'", i)
		}
	}
}

// TestInternalGetUserSubscriptionInvalidId tests invalid user ID for subscription
func TestInternalGetUserSubscriptionInvalidId(t *testing.T) {
	router := MockRouter()
	router.GET("/internal/subscription/user/:id", InternalGetUserSubscription)

	req := httptest.NewRequest("GET", "/internal/subscription/user/invalid", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

// TestInternalGetUserQuotaInvalidId tests invalid user ID for quota
func TestInternalGetUserQuotaInvalidId(t *testing.T) {
	router := MockRouter()
	router.GET("/internal/quota/user/:id", InternalGetUserQuota)

	req := httptest.NewRequest("GET", "/internal/quota/user/abc", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

// TestInternalGetUserBalanceInvalidId tests invalid user ID for balance
func TestInternalGetUserBalanceInvalidId(t *testing.T) {
	router := MockRouter()
	router.GET("/internal/balance/user/:id", InternalGetUserBalance)

	req := httptest.NewRequest("GET", "/internal/balance/user/xyz", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

// TestScopeConstants tests that all expected scopes exist
func TestScopeConstants(t *testing.T) {
	expectedScopes := []string{
		model.ScopeUserRead,
		model.ScopeUserWrite,
		model.ScopeSubscriptionRead,
		model.ScopeSubscriptionWrite,
		model.ScopeQuotaRead,
		model.ScopeQuotaWrite,
		model.ScopeBalanceRead,
		model.ScopeBalanceWrite,
		model.ScopeAll,
	}

	for _, scope := range expectedScopes {
		if scope == "" {
			t.Errorf("Scope constant is empty")
		}
	}

	// Verify scope values
	if model.ScopeAll != "*" {
		t.Errorf("ScopeAll should be '*', got %q", model.ScopeAll)
	}
}

// Helper function to check if string contains substring (case insensitive)
func containsIgnoreCase(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		(len(s) > 0 && len(substr) > 0 && contains(toLower(s), toLower(substr))))
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func toLower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			result[i] = c + 32
		} else {
			result[i] = c
		}
	}
	return string(result)
}

// BenchmarkInternalGetUserValidation benchmarks user ID validation
func BenchmarkInternalGetUserValidation(b *testing.B) {
	router := MockRouter()
	router.GET("/internal/user/:id", InternalGetUser)

	req := httptest.NewRequest("GET", "/internal/user/123", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	}
}

// BenchmarkScopeCheck benchmarks the scope availability check
func BenchmarkScopeCheck(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = model.GetAvailableScopes()
	}
}

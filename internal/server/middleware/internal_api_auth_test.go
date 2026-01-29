package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/lurus-api/internal/data/model"
	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// Helper to create a test router
func setupTestRouter() *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	return r
}

// TestInternalApiAuthMissingHeader tests missing API key header
func TestInternalApiAuthMissingHeader(t *testing.T) {
	router := setupTestRouter()
	router.Use(InternalApiAuth())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	if response["success"] != false {
		t.Error("Expected success to be false")
	}

	if msg, ok := response["message"].(string); !ok || msg != "API key required" {
		t.Errorf("Expected message 'API key required', got %v", response["message"])
	}
}

// TestInternalApiAuthEmptyHeader tests empty API key header
func TestInternalApiAuthEmptyHeader(t *testing.T) {
	router := setupTestRouter()
	router.Use(InternalApiAuth())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-API-Key", "")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

// TestRequireScopeNoKeyInContext tests RequireScope with no key in context
func TestRequireScopeNoKeyInContext(t *testing.T) {
	router := setupTestRouter()
	// Use RequireScope without InternalApiAuth
	router.Use(RequireScope("user:read"))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected status %d, got %d", http.StatusForbidden, w.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	if msg, ok := response["message"].(string); !ok || msg != "API key not found in context" {
		t.Errorf("Expected message 'API key not found in context', got %v", response["message"])
	}
}

// TestRequireScopeWithValidKey tests RequireScope with a valid key in context
func TestRequireScopeWithValidKey(t *testing.T) {
	router := setupTestRouter()

	// Mock the InternalApiAuth by manually setting context
	router.Use(func(c *gin.Context) {
		scopesJson, _ := json.Marshal([]string{"user:read", "user:write"})
		key := &model.InternalApiKey{
			Id:      1,
			Name:    "Test Key",
			Scopes:  string(scopesJson),
			Enabled: true,
		}
		c.Set("internal_api_key", key)
		c.Next()
	})
	router.Use(RequireScope("user:read"))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}
}

// TestRequireScopeInsufficientPermissions tests RequireScope with insufficient scope
func TestRequireScopeInsufficientPermissions(t *testing.T) {
	router := setupTestRouter()

	// Mock the InternalApiAuth with limited scope
	router.Use(func(c *gin.Context) {
		scopesJson, _ := json.Marshal([]string{"user:read"})
		key := &model.InternalApiKey{
			Id:      1,
			Name:    "Test Key",
			Scopes:  string(scopesJson),
			Enabled: true,
		}
		c.Set("internal_api_key", key)
		c.Next()
	})
	router.Use(RequireScope("user:write")) // Requires write but only has read
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected status %d, got %d", http.StatusForbidden, w.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	if msg, ok := response["message"].(string); ok {
		if msg != "Insufficient permissions. Required scope: user:write" {
			t.Errorf("Unexpected message: %s", msg)
		}
	}
}

// TestRequireScopeWithWildcard tests RequireScope with wildcard scope
func TestRequireScopeWithWildcard(t *testing.T) {
	router := setupTestRouter()

	// Mock with wildcard scope
	router.Use(func(c *gin.Context) {
		scopesJson, _ := json.Marshal([]string{"*"})
		key := &model.InternalApiKey{
			Id:      1,
			Name:    "Root Key",
			Scopes:  string(scopesJson),
			Enabled: true,
		}
		c.Set("internal_api_key", key)
		c.Next()
	})
	router.Use(RequireScope("subscription:write"))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Wildcard scope should grant access, got status %d", w.Code)
	}
}

// TestRequireAnyScopeNoKeyInContext tests RequireAnyScope with no key in context
func TestRequireAnyScopeNoKeyInContext(t *testing.T) {
	router := setupTestRouter()
	router.Use(RequireAnyScope("user:read", "user:write"))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected status %d, got %d", http.StatusForbidden, w.Code)
	}
}

// TestRequireAnyScopeWithMatchingScope tests RequireAnyScope with one matching scope
func TestRequireAnyScopeWithMatchingScope(t *testing.T) {
	tests := []struct {
		name           string
		keyScopes      []string
		requiredScopes []string
		expectStatus   int
	}{
		{
			name:           "has_first_scope",
			keyScopes:      []string{"user:read"},
			requiredScopes: []string{"user:read", "user:write"},
			expectStatus:   http.StatusOK,
		},
		{
			name:           "has_second_scope",
			keyScopes:      []string{"user:write"},
			requiredScopes: []string{"user:read", "user:write"},
			expectStatus:   http.StatusOK,
		},
		{
			name:           "has_both_scopes",
			keyScopes:      []string{"user:read", "user:write"},
			requiredScopes: []string{"user:read", "user:write"},
			expectStatus:   http.StatusOK,
		},
		{
			name:           "has_wildcard",
			keyScopes:      []string{"*"},
			requiredScopes: []string{"subscription:write", "balance:write"},
			expectStatus:   http.StatusOK,
		},
		{
			name:           "has_no_matching_scope",
			keyScopes:      []string{"quota:read"},
			requiredScopes: []string{"user:read", "user:write"},
			expectStatus:   http.StatusForbidden,
		},
		{
			name:           "empty_key_scopes",
			keyScopes:      []string{},
			requiredScopes: []string{"user:read"},
			expectStatus:   http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := setupTestRouter()

			// Mock with specified scopes
			router.Use(func(c *gin.Context) {
				scopesJson, _ := json.Marshal(tt.keyScopes)
				key := &model.InternalApiKey{
					Id:      1,
					Name:    "Test Key",
					Scopes:  string(scopesJson),
					Enabled: true,
				}
				c.Set("internal_api_key", key)
				c.Next()
			})
			router.Use(RequireAnyScope(tt.requiredScopes...))
			router.GET("/test", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"success": true})
			})

			req := httptest.NewRequest("GET", "/test", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tt.expectStatus {
				t.Errorf("Expected status %d, got %d", tt.expectStatus, w.Code)
			}
		})
	}
}

// TestRequireScopeInvalidKeyType tests RequireScope with invalid key type in context
func TestRequireScopeInvalidKeyType(t *testing.T) {
	router := setupTestRouter()

	// Set invalid type in context
	router.Use(func(c *gin.Context) {
		c.Set("internal_api_key", "not a key object")
		c.Next()
	})
	router.Use(RequireScope("user:read"))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	if msg, ok := response["message"].(string); !ok || msg != "Invalid API key type" {
		t.Errorf("Expected message 'Invalid API key type', got %v", response["message"])
	}
}

// TestRequireAnyScopeInvalidKeyType tests RequireAnyScope with invalid key type
func TestRequireAnyScopeInvalidKeyType(t *testing.T) {
	router := setupTestRouter()

	// Set invalid type in context
	router.Use(func(c *gin.Context) {
		c.Set("internal_api_key", 12345)
		c.Next()
	})
	router.Use(RequireAnyScope("user:read", "user:write"))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}
}

// TestContextSetup tests that context values are set correctly
func TestContextSetup(t *testing.T) {
	router := setupTestRouter()

	// Mock InternalApiAuth setting context
	router.Use(func(c *gin.Context) {
		scopesJson, _ := json.Marshal([]string{"user:read", "user:write"})
		key := &model.InternalApiKey{
			Id:      42,
			Name:    "Test Key",
			Scopes:  string(scopesJson),
			Enabled: true,
		}
		c.Set("internal_api_key", key)
		c.Set("internal_api_scopes", key.GetScopes())
		c.Set("internal_api_key_id", key.Id)
		c.Set("internal_api_key_name", key.Name)
		c.Next()
	})

	router.GET("/test", func(c *gin.Context) {
		// Verify context values
		keyId, exists := c.Get("internal_api_key_id")
		if !exists || keyId != 42 {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "key_id not set"})
			return
		}

		keyName, exists := c.Get("internal_api_key_name")
		if !exists || keyName != "Test Key" {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "key_name not set"})
			return
		}

		scopes, exists := c.Get("internal_api_scopes")
		if !exists {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "scopes not set"})
			return
		}

		scopesList, ok := scopes.([]string)
		if !ok || len(scopesList) != 2 {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "scopes invalid"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}
}

// TestMultipleScopeMiddlewares tests chaining multiple RequireScope middlewares
func TestMultipleScopeMiddlewares(t *testing.T) {
	router := setupTestRouter()

	// Mock with multiple scopes
	router.Use(func(c *gin.Context) {
		scopesJson, _ := json.Marshal([]string{"user:read", "user:write"})
		key := &model.InternalApiKey{
			Id:      1,
			Name:    "Test Key",
			Scopes:  string(scopesJson),
			Enabled: true,
		}
		c.Set("internal_api_key", key)
		c.Next()
	})

	// Chain multiple scope requirements
	router.Use(RequireScope("user:read"))
	router.Use(RequireScope("user:write"))

	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}
}

// TestMultipleScopeMiddlewaresPartialMatch tests failing on second scope
func TestMultipleScopeMiddlewaresPartialMatch(t *testing.T) {
	router := setupTestRouter()

	// Mock with only read scope
	router.Use(func(c *gin.Context) {
		scopesJson, _ := json.Marshal([]string{"user:read"})
		key := &model.InternalApiKey{
			Id:      1,
			Name:    "Test Key",
			Scopes:  string(scopesJson),
			Enabled: true,
		}
		c.Set("internal_api_key", key)
		c.Next()
	})

	// Chain multiple scope requirements - will fail on second
	router.Use(RequireScope("user:read"))
	router.Use(RequireScope("user:write"))

	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected status %d, got %d", http.StatusForbidden, w.Code)
	}
}

// BenchmarkRequireScope benchmarks the RequireScope middleware
func BenchmarkRequireScope(b *testing.B) {
	router := setupTestRouter()

	router.Use(func(c *gin.Context) {
		scopesJson, _ := json.Marshal([]string{"user:read", "user:write", "quota:read"})
		key := &model.InternalApiKey{
			Id:      1,
			Name:    "Test Key",
			Scopes:  string(scopesJson),
			Enabled: true,
		}
		c.Set("internal_api_key", key)
		c.Next()
	})
	router.Use(RequireScope("user:read"))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	req := httptest.NewRequest("GET", "/test", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	}
}

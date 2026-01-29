package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/lurus-api/internal/pkg/common"
	"github.com/QuantumNous/lurus-api/internal/data/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// setupMiddlewareTestDB initializes an in-memory SQLite database for integration tests.
// Returns a cleanup function that resets global state.
func setupMiddlewareTestDB(t *testing.T) func() {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite :memory: db: %v", err)
	}

	err = db.AutoMigrate(&model.InternalApiKey{}, &model.User{}, &model.Token{}, &model.Log{})
	if err != nil {
		t.Fatalf("failed to auto-migrate: %v", err)
	}

	oldDB := model.DB
	oldLogDB := model.LOG_DB
	oldUsingSQLite := common.UsingSQLite

	model.DB = db
	model.LOG_DB = db
	common.UsingSQLite = true
	common.RedisEnabled = false

	return func() {
		model.DB = oldDB
		model.LOG_DB = oldLogDB
		common.UsingSQLite = oldUsingSQLite
	}
}

// parseResponseBody decodes the JSON response body into a map.
func parseResponseBody(t *testing.T, w *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var result map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse response body: %v", err)
	}
	return result
}

func init() {
	gin.SetMode(gin.TestMode)
}

func TestInteg_Auth_ValidKey_ContextPopulated(t *testing.T) {
	cleanup := setupMiddlewareTestDB(t)
	defer cleanup()

	rawKey, apiKey, err := model.CreateInternalApiKey(
		"test-key", []string{"user:read", "user:write"}, 1, 0, "integration test key",
	)
	if err != nil {
		t.Fatalf("failed to create api key: %v", err)
	}

	var capturedKeyID int
	var capturedKeyName string
	var capturedScopes []string

	r := gin.New()
	r.Use(InternalApiAuth())
	r.GET("/test", func(c *gin.Context) {
		keyID, _ := c.Get("internal_api_key_id")
		capturedKeyID = keyID.(int)
		keyName, _ := c.Get("internal_api_key_name")
		capturedKeyName = keyName.(string)
		scopes, _ := c.Get("internal_api_scopes")
		capturedScopes = scopes.([]string)
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-API-Key", rawKey)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if capturedKeyID != apiKey.Id {
		t.Errorf("expected key_id=%d, got %d", apiKey.Id, capturedKeyID)
	}
	if capturedKeyName != "test-key" {
		t.Errorf("expected key_name=test-key, got %s", capturedKeyName)
	}
	if len(capturedScopes) != 2 {
		t.Errorf("expected 2 scopes, got %d", len(capturedScopes))
	}
}

func TestInteg_Auth_InvalidKey_401(t *testing.T) {
	cleanup := setupMiddlewareTestDB(t)
	defer cleanup()

	r := gin.New()
	r.Use(InternalApiAuth())
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-API-Key", "lurus_ik_totallyinvalidkeyvalue1234")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestInteg_Auth_ExpiredKey_401(t *testing.T) {
	cleanup := setupMiddlewareTestDB(t)
	defer cleanup()

	// Create key that expired 1 hour ago
	pastTime := time.Now().Unix() - 3600
	rawKey, _, err := model.CreateInternalApiKey(
		"expired-key", []string{"user:read"}, 1, pastTime, "expired key",
	)
	if err != nil {
		t.Fatalf("failed to create api key: %v", err)
	}

	r := gin.New()
	r.Use(InternalApiAuth())
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-API-Key", rawKey)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestInteg_Auth_DisabledKey_401(t *testing.T) {
	cleanup := setupMiddlewareTestDB(t)
	defer cleanup()

	rawKey, apiKey, err := model.CreateInternalApiKey(
		"disabled-key", []string{"user:read"}, 1, 0, "will be disabled",
	)
	if err != nil {
		t.Fatalf("failed to create api key: %v", err)
	}

	// Disable the key
	err = model.DB.Model(&model.InternalApiKey{}).Where("id = ?", apiKey.Id).Update("enabled", false).Error
	if err != nil {
		t.Fatalf("failed to disable key: %v", err)
	}

	r := gin.New()
	r.Use(InternalApiAuth())
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-API-Key", rawKey)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestInteg_Auth_MissingHeader_401(t *testing.T) {
	cleanup := setupMiddlewareTestDB(t)
	defer cleanup()

	r := gin.New()
	r.Use(InternalApiAuth())
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	// No X-API-Key header
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
	body := parseResponseBody(t, w)
	if msg, ok := body["message"].(string); !ok || msg != "API key required" {
		t.Errorf("expected 'API key required' message, got %v", body["message"])
	}
}

// TestInteg_RequireScope uses table-driven subtests for scope authorization checks.
func TestInteg_RequireScope(t *testing.T) {
	tests := []struct {
		name           string
		keyScopes      []string
		requiredScope  string
		expectedStatus int
	}{
		{
			name:           "HasScope_Pass",
			keyScopes:      []string{"user:read"},
			requiredScope:  "user:read",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "MissingScope_403",
			keyScopes:      []string{"user:read"},
			requiredScope:  "user:write",
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "Wildcard_Pass",
			keyScopes:      []string{"*"},
			requiredScope:  "anything:here",
			expectedStatus: http.StatusOK,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cleanup := setupMiddlewareTestDB(t)
			defer cleanup()

			rawKey, _, err := model.CreateInternalApiKey(
				"scope-test-"+tc.name, tc.keyScopes, 1, 0, "scope test",
			)
			if err != nil {
				t.Fatalf("failed to create api key: %v", err)
			}

			r := gin.New()
			r.Use(InternalApiAuth())
			r.Use(RequireScope(tc.requiredScope))
			r.GET("/test", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"success": true})
			})

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.Header.Set("X-API-Key", rawKey)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != tc.expectedStatus {
				t.Errorf("expected %d, got %d: %s", tc.expectedStatus, w.Code, w.Body.String())
			}
		})
	}
}

// TestInteg_RequireAnyScope uses table-driven subtests for multi-scope checks.
func TestInteg_RequireAnyScope(t *testing.T) {
	tests := []struct {
		name           string
		keyScopes      []string
		requiredScopes []string
		expectedStatus int
	}{
		{
			name:           "OneMatch_Pass",
			keyScopes:      []string{"user:read"},
			requiredScopes: []string{"user:read", "user:write"},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "NoneMatch_403",
			keyScopes:      []string{"token:read"},
			requiredScopes: []string{"user:read", "user:write"},
			expectedStatus: http.StatusForbidden,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cleanup := setupMiddlewareTestDB(t)
			defer cleanup()

			rawKey, _, err := model.CreateInternalApiKey(
				"anyscope-test-"+tc.name, tc.keyScopes, 1, 0, "any scope test",
			)
			if err != nil {
				t.Fatalf("failed to create api key: %v", err)
			}

			r := gin.New()
			r.Use(InternalApiAuth())
			r.Use(RequireAnyScope(tc.requiredScopes...))
			r.GET("/test", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"success": true})
			})

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.Header.Set("X-API-Key", rawKey)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != tc.expectedStatus {
				t.Errorf("expected %d, got %d: %s", tc.expectedStatus, w.Code, w.Body.String())
			}
		})
	}
}

func TestInteg_Auth_LastUsedUpdated(t *testing.T) {
	cleanup := setupMiddlewareTestDB(t)
	defer cleanup()

	rawKey, apiKey, err := model.CreateInternalApiKey(
		"last-used-key", []string{"user:read"}, 1, 0, "last used test",
	)
	if err != nil {
		t.Fatalf("failed to create api key: %v", err)
	}

	// Verify initial LastUsedAt is 0
	var keyBefore model.InternalApiKey
	if err := model.DB.First(&keyBefore, apiKey.Id).Error; err != nil {
		t.Fatalf("failed to query key: %v", err)
	}
	if keyBefore.LastUsedAt != 0 {
		t.Fatalf("expected initial LastUsedAt=0, got %d", keyBefore.LastUsedAt)
	}

	r := gin.New()
	r.Use(InternalApiAuth())
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-API-Key", rawKey)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	// Wait for the goroutine that updates LastUsedAt to complete
	time.Sleep(200 * time.Millisecond)

	var keyAfter model.InternalApiKey
	if err := model.DB.First(&keyAfter, apiKey.Id).Error; err != nil {
		t.Fatalf("failed to query key after request: %v", err)
	}
	if keyAfter.LastUsedAt == 0 {
		t.Errorf("expected LastUsedAt > 0 after request, still 0")
	}
}

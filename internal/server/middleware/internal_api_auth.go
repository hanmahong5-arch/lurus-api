package middleware

import (
	"net/http"

	"github.com/QuantumNous/lurus-api/internal/data/model"
	"github.com/gin-gonic/gin"
)

// InternalApiAuth authenticates requests using internal API key
func InternalApiAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get API key from header: X-API-Key: lurus_ik_xxx
		apiKey := c.GetHeader("X-API-Key")
		if apiKey == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"message": "API key required",
			})
			c.Abort()
			return
		}

		// Validate key
		key, err := model.ValidateInternalApiKey(apiKey)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"message": "Invalid or expired API key",
			})
			c.Abort()
			return
		}

		// Store key info in context
		c.Set("internal_api_key", key)
		c.Set("internal_api_scopes", key.GetScopes())
		c.Set("internal_api_key_id", key.Id)
		c.Set("internal_api_key_name", key.Name)

		c.Next()
	}
}

// RequireScope middleware checks if the API key has required scope
func RequireScope(scope string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get key from context
		keyInterface, exists := c.Get("internal_api_key")
		if !exists {
			c.JSON(http.StatusForbidden, gin.H{
				"success": false,
				"message": "API key not found in context",
			})
			c.Abort()
			return
		}

		key, ok := keyInterface.(*model.InternalApiKey)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Invalid API key type",
			})
			c.Abort()
			return
		}

		// Check scope
		if !key.HasScope(scope) {
			c.JSON(http.StatusForbidden, gin.H{
				"success": false,
				"message": "Insufficient permissions. Required scope: " + scope,
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// RequireAnyScope middleware checks if the API key has any of the required scopes
func RequireAnyScope(scopes ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		keyInterface, exists := c.Get("internal_api_key")
		if !exists {
			c.JSON(http.StatusForbidden, gin.H{
				"success": false,
				"message": "API key not found in context",
			})
			c.Abort()
			return
		}

		key, ok := keyInterface.(*model.InternalApiKey)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Invalid API key type",
			})
			c.Abort()
			return
		}

		// Check any scope
		for _, scope := range scopes {
			if key.HasScope(scope) {
				c.Next()
				return
			}
		}

		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": "Insufficient permissions",
		})
		c.Abort()
	}
}

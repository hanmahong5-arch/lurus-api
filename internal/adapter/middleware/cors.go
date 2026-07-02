package middleware

import (
	"github.com/LurusTech/lurus-hub/internal/pkg/config"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func CORS() gin.HandlerFunc {
	corsConfig := cors.DefaultConfig()
	// Load allowed origins from centralized config (env: ALLOWED_ORIGINS)
	corsConfig.AllowOrigins = config.Get().CORS.AllowedOrigins
	corsConfig.AllowCredentials = true
	corsConfig.AllowMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
	// A literal "*" is NOT treated as a wildcard by the CORS spec when
	// AllowCredentials is true — browsers reject the response. List the
	// headers actual callers set (frontend api.js + relay TokenAuth key
	// extraction + internal API key auth) instead.
	corsConfig.AllowHeaders = []string{
		"Origin", "Content-Type", "Authorization",
		"X-API-Key", "lurus-api-User", "New-Api-User", "Cache-Control",
	}
	return cors.New(corsConfig)
}

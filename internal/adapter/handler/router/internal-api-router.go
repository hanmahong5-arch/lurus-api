package router

import (
	"github.com/LurusTech/lurus-hub/internal/adapter/handler"
	"github.com/LurusTech/lurus-hub/internal/adapter/middleware"
	"github.com/LurusTech/lurus-hub/internal/adapter/repo"

	"github.com/gin-gonic/gin"
)

// SetInternalApiRouter sets up internal API routes for service-to-service communication
// These routes use API Key authentication instead of user session auth
func SetInternalApiRouter(router *gin.Engine) {
	internalGroup := router.Group("/internal")
	internalGroup.Use(middleware.InternalApiAuth())

	// User read APIs - query user information
	userReadGroup := internalGroup.Group("/user")
	userReadGroup.Use(middleware.RequireScope(repo.ScopeUserRead))
	{
		userReadGroup.GET("/:id", handler.InternalGetUser)
		userReadGroup.GET("/by-email/:email", handler.InternalGetUserByEmail)
		userReadGroup.GET("/by-phone/:phone", handler.InternalGetUserByPhone)
		userReadGroup.GET("/by-zitadel-sub/:sub", handler.InternalGetUserByZitadelSub)
	}

	// User write APIs - create and modify users
	userWriteGroup := internalGroup.Group("/user")
	userWriteGroup.Use(middleware.RequireScope(repo.ScopeUserWrite))
	{
		userWriteGroup.POST("", handler.InternalCreateUser)
		userWriteGroup.PUT("/:id", handler.InternalUpdateUser)
		userWriteGroup.POST("/provision", handler.InternalProvisionUser)
	}

	// User delete APIs
	userDeleteGroup := internalGroup.Group("/user")
	userDeleteGroup.Use(middleware.RequireScope(repo.ScopeUserDelete))
	{
		userDeleteGroup.DELETE("/:id", handler.InternalDeleteUser)
	}

	// Token read APIs
	tokenReadGroup := internalGroup.Group("/token")
	tokenReadGroup.Use(middleware.RequireScope(repo.ScopeTokenRead))
	{
		tokenReadGroup.GET("/user/:id", handler.InternalGetUserTokens)
		tokenReadGroup.GET("/:id", handler.InternalGetToken)
		tokenReadGroup.GET("/:id/usage", handler.InternalGetTokenUsage)
	}

	// Token write APIs
	tokenWriteGroup := internalGroup.Group("/token")
	tokenWriteGroup.Use(middleware.RequireScope(repo.ScopeTokenWrite))
	{
		tokenWriteGroup.POST("", handler.InternalCreateToken)
		tokenWriteGroup.PUT("/:id", handler.InternalUpdateToken)
		tokenWriteGroup.DELETE("/:id", handler.InternalDeleteToken)
	}

	// Quota APIs - read user quota
	quotaReadGroup := internalGroup.Group("/quota")
	quotaReadGroup.Use(middleware.RequireScope(repo.ScopeQuotaRead))
	{
		quotaReadGroup.GET("/user/:id", handler.InternalGetUserQuota)
	}

	// Quota APIs - adjust user quota
	quotaWriteGroup := internalGroup.Group("/quota")
	quotaWriteGroup.Use(middleware.RequireScope(repo.ScopeQuotaWrite))
	{
		quotaWriteGroup.POST("/adjust", handler.InternalAdjustQuota)
	}

	// Balance APIs - read user balance
	balanceReadGroup := internalGroup.Group("/balance")
	balanceReadGroup.Use(middleware.RequireScope(repo.ScopeBalanceRead))
	{
		balanceReadGroup.GET("/user/:id", handler.InternalGetUserBalance)
	}

	// Balance APIs - top up user balance
	balanceWriteGroup := internalGroup.Group("/balance")
	balanceWriteGroup.Use(middleware.RequireScope(repo.ScopeBalanceWrite))
	{
		balanceWriteGroup.POST("/topup", handler.InternalTopupBalance)
	}

	// Currency APIs - read exchange rates, model pricing, user balance in Lute
	currencyReadGroup := internalGroup.Group("/currency")
	currencyReadGroup.Use(middleware.RequireScope(repo.ScopeCurrencyRead))
	{
		currencyReadGroup.GET("/info", handler.InternalGetCurrencyInfo)
		currencyReadGroup.GET("/models/pricing", handler.InternalGetModelPricing)
		currencyReadGroup.GET("/balance/:id", handler.InternalGetUserBalanceLute)
		currencyReadGroup.GET("/exchanges/:id", handler.InternalGetExchangeHistory)
	}

	// Currency APIs - perform LUC -> LUT exchange
	currencyExchangeGroup := internalGroup.Group("/currency")
	currencyExchangeGroup.Use(middleware.RequireScope(repo.ScopeCurrencyExchange))
	{
		currencyExchangeGroup.POST("/exchange", handler.InternalExchangeLucToLut)
	}

	// Log read APIs - query usage logs by user or token
	logReadGroup := internalGroup.Group("/log")
	logReadGroup.Use(middleware.RequireScope(repo.ScopeLogRead))
	{
		logReadGroup.GET("/user/:id", handler.InternalGetUserLogs)
		logReadGroup.GET("/user/:id/stat", handler.InternalGetUserLogStat)
		logReadGroup.GET("/token/:token_id", handler.InternalGetTokenLogs)
	}

	// Model catalog API - available models with pricing
	modelReadGroup := internalGroup.Group("/models")
	modelReadGroup.Use(middleware.RequireScope(repo.ScopeModelRead))
	{
		modelReadGroup.GET("/catalog", handler.InternalGetModelCatalog)
		modelReadGroup.GET("/video-catalog", handler.InternalGetVideoCatalog)
		modelReadGroup.GET("/video-status", handler.InternalGetVideoStatus)
	}

	// Admin operations (requires the admin scope; ScopeAll keys still pass via
	// HasScope's wildcard, so existing platform-admin keys keep working).
	adminGroup := internalGroup.Group("/admin")
	adminGroup.Use(middleware.RequireScope(repo.ScopeAdmin))
	{
		adminGroup.POST("/backfill-token-accounts", handler.InternalBackfillTokenAccountIDs)
		adminGroup.GET("/convergence-stats", handler.InternalConvergenceStats)
	}

	// Provisioning API — Reseller sub-tenant key issuance / revocation
	// (ADR 2026-05-18 §4.2). Auth: X-API-Key + scope "provisioning".
	provisioningGroup := internalGroup.Group("/v1/provisioning")
	provisioningGroup.Use(middleware.RequireScope(repo.ScopeProvisioning))
	{
		provisioningGroup.POST("/tenants/:slug/keys", handler.CreateProvisionedKey)
		provisioningGroup.GET("/tenants/:slug/keys", handler.ListProvisionedKeys)
		provisioningGroup.DELETE("/tenants/:slug/keys/:key_id", handler.RevokeProvisionedKey)
	}
}

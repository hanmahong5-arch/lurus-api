package router

import (
	"github.com/QuantumNous/lurus-api/internal/server/controller"
	"github.com/QuantumNous/lurus-api/internal/server/middleware"
	"github.com/QuantumNous/lurus-api/internal/data/model"

	"github.com/gin-gonic/gin"
)

// SetInternalApiRouter sets up internal API routes for service-to-service communication
// These routes use API Key authentication instead of user session auth
func SetInternalApiRouter(router *gin.Engine) {
	internalGroup := router.Group("/internal")
	internalGroup.Use(middleware.InternalApiAuth())

	// User APIs - query user information
	userGroup := internalGroup.Group("/user")
	userGroup.Use(middleware.RequireScope(model.ScopeUserRead))
	{
		userGroup.GET("/:id", controller.InternalGetUser)
		userGroup.GET("/by-email/:email", controller.InternalGetUserByEmail)
		userGroup.GET("/by-phone/:phone", controller.InternalGetUserByPhone)
	}

	// User write APIs - modify user information
	userWriteGroup := internalGroup.Group("/user")
	userWriteGroup.Use(middleware.RequireScope(model.ScopeUserWrite))
	{
		userWriteGroup.PUT("/:id", controller.InternalUpdateUser)
	}

	// Subscription APIs - read subscription information
	subReadGroup := internalGroup.Group("/subscription")
	subReadGroup.Use(middleware.RequireScope(model.ScopeSubscriptionRead))
	{
		subReadGroup.GET("/user/:id", controller.InternalGetUserSubscription)
	}

	// Subscription APIs - grant subscriptions
	subWriteGroup := internalGroup.Group("/subscription")
	subWriteGroup.Use(middleware.RequireScope(model.ScopeSubscriptionWrite))
	{
		subWriteGroup.POST("/grant", controller.InternalGrantSubscription)
	}

	// Quota APIs - read user quota
	quotaReadGroup := internalGroup.Group("/quota")
	quotaReadGroup.Use(middleware.RequireScope(model.ScopeQuotaRead))
	{
		quotaReadGroup.GET("/user/:id", controller.InternalGetUserQuota)
	}

	// Quota APIs - adjust user quota
	quotaWriteGroup := internalGroup.Group("/quota")
	quotaWriteGroup.Use(middleware.RequireScope(model.ScopeQuotaWrite))
	{
		quotaWriteGroup.POST("/adjust", controller.InternalAdjustQuota)
	}

	// Balance APIs - read user balance
	balanceReadGroup := internalGroup.Group("/balance")
	balanceReadGroup.Use(middleware.RequireScope(model.ScopeBalanceRead))
	{
		balanceReadGroup.GET("/user/:id", controller.InternalGetUserBalance)
	}

	// Balance APIs - top up user balance
	balanceWriteGroup := internalGroup.Group("/balance")
	balanceWriteGroup.Use(middleware.RequireScope(model.ScopeBalanceWrite))
	{
		balanceWriteGroup.POST("/topup", controller.InternalTopupBalance)
	}
}

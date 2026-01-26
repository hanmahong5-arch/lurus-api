package router

import (
	"github.com/QuantumNous/lurus-api/controller"
	"github.com/QuantumNous/lurus-api/middleware"

	"github.com/gin-gonic/gin"
)

// SetApiV2Router sets up v2 API routes with multi-tenant support
// All v2 routes use Zitadel OAuth authentication
func SetApiV2Router(router *gin.Engine) {
	// V2 API group
	apiV2 := router.Group("/api/v2")
	{
		// ================================================================
		// OAuth Authentication Routes (No authentication required)
		// OAuth 认证路由（无需认证）
		// ================================================================

		// OAuth login redirect - redirects to Zitadel login page
		// OAuth 登录跳转 - 跳转到 Zitadel 登录页面
		apiV2.GET("/:tenant_slug/auth/login", controller.ZitadelLoginRedirect)

		// OAuth callback - handles Zitadel OAuth callback
		// OAuth 回调 - 处理 Zitadel OAuth 回调
		apiV2.GET("/oauth/callback", controller.ZitadelCallback)

		// OAuth logout - logs out from Zitadel
		// OAuth 登出 - 从 Zitadel 登出
		apiV2.POST("/oauth/logout", controller.ZitadelLogout)

		// OAuth token refresh - refreshes access token
		// OAuth Token 刷新 - 刷新访问令牌
		apiV2.POST("/oauth/refresh", controller.RefreshAccessToken)

		// ================================================================
		// Tenant-Specific Routes (Require Zitadel JWT authentication)
		// 租户路由（需要 Zitadel JWT 认证）
		// ================================================================

		tenantRoute := apiV2.Group("/:tenant_slug")
		tenantRoute.Use(middleware.ZitadelAuth()) // Zitadel JWT verification
		{
			// User routes
			// 用户路由
			tenantRoute.GET("/user/me", controller.GetSelfV2)
			tenantRoute.PUT("/user/me", controller.UpdateSelfV2)
			// TODO: Add more user routes

			// Channel routes (Admin only)
			// 渠道路由（仅管理员）
			channelRoute := tenantRoute.Group("/channels")
			{
				channelRoute.GET("", controller.ListChannelsV2)
				channelRoute.GET("/:id", controller.GetChannelV2)

				// Admin-only channel management
				channelRoute.POST("", middleware.RequireRole("admin"), controller.CreateChannelV2)
				channelRoute.PUT("/:id", middleware.RequireRole("admin"), controller.UpdateChannelV2)
				channelRoute.DELETE("/:id", middleware.RequireRole("admin"), controller.DeleteChannelV2)
			}

			// Billing routes
			// 计费路由
			billingRoute := tenantRoute.Group("/billing")
			{
				// Top-up history
				billingRoute.GET("/topups", controller.GetTopUpsV2)
				// Create top-up (initiate payment)
				billingRoute.POST("/topup", controller.TopUpV2)

				// Subscriptions
				billingRoute.GET("/subscriptions", controller.GetSubscriptionsV2)
				billingRoute.POST("/subscribe", controller.SubscribeV2)
				billingRoute.DELETE("/subscriptions/:id", controller.CancelSubscriptionV2)
			}

			// Token (API key) routes
			// Token（API密钥）路由
			tokenRoute := tenantRoute.Group("/tokens")
			{
				tokenRoute.GET("", controller.ListTokensV2)
				tokenRoute.POST("", controller.CreateTokenV2)
				tokenRoute.PUT("/:id", controller.UpdateTokenV2)
				tokenRoute.DELETE("/:id", controller.DeleteTokenV2)
			}

			// Log routes
			// 日志路由
			logRoute := tenantRoute.Group("/logs")
			{
				logRoute.GET("", controller.GetLogsV2)
				// Admin can view all users' logs
				logRoute.GET("/all", middleware.RequireRole("admin"), controller.GetAllLogsV2)
			}

			// Tenant configuration routes (Admin only)
			// 租户配置路由（仅管理员）
			configRoute := tenantRoute.Group("/config")
			configRoute.Use(middleware.RequireRole("admin"))
			{
				configRoute.GET("", controller.GetTenantConfigs)
				configRoute.PUT("/:key", controller.UpdateTenantConfig)
			}

			// Redemption code routes
			// 兑换码路由
			redemptionRoute := tenantRoute.Group("/redemptions")
			{
				// Users can redeem codes
				redemptionRoute.POST("/redeem", controller.RedeemCodeV2)

				// Admin can manage redemption codes
				redemptionRoute.GET("", middleware.RequireRole("admin"), controller.ListRedemptionsV2)
				redemptionRoute.POST("", middleware.RequireRole("admin"), controller.CreateRedemptionV2)
				redemptionRoute.DELETE("/:id", middleware.RequireRole("admin"), controller.DeleteRedemptionV2)
			}
		}

		// ================================================================
		// Platform Admin Routes (System-level, requires Platform Admin role)
		// 平台管理员路由（系统级，需要平台管理员角色）
		// ================================================================

		adminRoute := apiV2.Group("/admin")
		// Note: For Platform Admin routes, we use v1 authentication (session-based)
		// since Platform Admins may manage multiple tenants
		// 注意：平台管理员路由使用 v1 认证（基于 session）
		// 因为平台管理员需要管理多个租户
		adminRoute.Use(middleware.UserAuth(), middleware.RootAuth())
		{
			// Tenant management
			// 租户管理
			tenantMgmt := adminRoute.Group("/tenants")
			{
				tenantMgmt.GET("", controller.ListTenants)
				tenantMgmt.POST("", controller.CreateTenant)
				tenantMgmt.GET("/:id", controller.GetTenant)
				tenantMgmt.PUT("/:id", controller.UpdateTenant)
				tenantMgmt.DELETE("/:id", controller.DeleteTenant)

				// Tenant status management
				tenantMgmt.POST("/:id/enable", controller.EnableTenant)
				tenantMgmt.POST("/:id/disable", controller.DisableTenant)
				tenantMgmt.POST("/:id/suspend", controller.SuspendTenant)

				// Tenant statistics
				tenantMgmt.GET("/:id/stats", controller.GetTenantStats)
			}

			// User identity mapping management (Platform Admin)
			// 用户身份映射管理（平台管理员）
			mappingRoute := adminRoute.Group("/mappings")
			{
				mappingRoute.GET("", controller.ListUserMappingsV2)
				mappingRoute.GET("/:id", controller.GetUserMappingV2)
				mappingRoute.DELETE("/:id", controller.DeleteUserMappingV2)
			}

			// System-wide statistics (Platform Admin)
			// 系统级统计（平台管理员）
			adminRoute.GET("/stats", controller.GetSystemStatsV2)
		}
	}
}

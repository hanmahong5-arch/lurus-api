package router

import (
	"github.com/LurusTech/lurus-hub/internal/adapter/handler"
	"github.com/LurusTech/lurus-hub/internal/adapter/middleware"
	"github.com/LurusTech/lurus-hub/internal/pkg/common"

	"github.com/gin-gonic/gin"
)

// SetApiV2Router sets up v2 API routes.
// Admin operations use AdminJWTAuth; billing uses ZitadelAuth.
func SetApiV2Router(router *gin.Engine) {
	apiV2 := router.Group("/api/v2")
	{
		// ================================================================
		// OAuth / Zitadel Routes (public — handles redirects & callbacks)
		// ================================================================

		apiV2.GET("/:tenant_slug/auth/login", handler.ZitadelLoginRedirect)
		apiV2.GET("/oauth/callback", handler.ZitadelCallback)
		apiV2.GET("/auth/session-info", handler.GetSessionInfo)
		apiV2.POST("/oauth/logout", handler.ZitadelLogout)
		apiV2.POST("/oauth/refresh", handler.RefreshAccessToken)

		// ----------------------------------------------------------------
		// Zita SDK login path (ADR-0011 Layer C). Coexists with the
		// legacy Zitadel-direct routes above during migration. Frontend
		// rewires to this URL in the next session; legacy deletion follows.
		// ----------------------------------------------------------------

		apiV2.GET("/auth/zita-login", handler.ZitaLogin)
		// Logout must be unauthenticated — the gin session may already be
		// expired (the whole point is to clear it + the platform cookie).
		apiV2.GET("/auth/zita-logout", handler.ZitaLogout)
		apiV2.POST("/auth/zita-logout", handler.ZitaLogout)
		if common.ZitaClient != nil {
			apiV2.GET("/me/zita", common.ZitaClient.AuthMiddleware(), handler.GetZitaIdentity)
			apiV2.POST("/auth/zita-bootstrap", middleware.BootstrapRateLimit(), common.ZitaClient.AuthMiddleware(), handler.ZitaBootstrap)
		}

		// Tenant-scoped user endpoint (session auth — called by frontend in V2 mode)
		apiV2.GET("/:tenant_slug/user/me", middleware.UserAuth(), handler.GetSelf)

		// ================================================================
		// Tenant-scoped Token Management (session auth)
		// ================================================================

		tenantTokens := apiV2.Group("/:tenant_slug/tokens")
		tenantTokens.Use(middleware.UserAuth())
		{
			tenantTokens.GET("", handler.ListTokensV2)
			tenantTokens.POST("", handler.CreateTokenV2)
			tenantTokens.PUT("/:id", handler.UpdateTokenV2)
			tenantTokens.DELETE("/:id", handler.DeleteTokenV2)
			tenantTokens.POST("/:id/rotate", handler.RotateTokenV2)
		}

		// ================================================================
		// Tenant-scoped Channel Management (session auth — admin only)
		// ================================================================

		tenantChannels := apiV2.Group("/:tenant_slug/channels")
		tenantChannels.Use(middleware.AdminAuth())
		{
			tenantChannels.GET("", handler.ListChannelsV2)
			tenantChannels.GET("/:id", handler.GetChannelV2)
			tenantChannels.POST("", handler.CreateChannelV2)
			tenantChannels.PUT("/:id", handler.UpdateChannelV2)
			tenantChannels.DELETE("/:id", handler.DeleteChannelV2)
		}

		// ================================================================
		// Tenant-scoped Logs (session auth)
		// ================================================================

		tenantLogs := apiV2.Group("/:tenant_slug/logs")
		tenantLogs.Use(middleware.UserAuth())
		{
			tenantLogs.GET("", handler.GetLogsV2)
			tenantLogs.GET("/all", handler.GetAllLogsV2)
		}

		// Settings — PUT for profile update (GET already registered above)
		apiV2.PUT("/:tenant_slug/user/me", middleware.UserAuth(), handler.UpdateSelfV2)

		// ================================================================
		// Switch Public Routes (no authentication required)
		// ================================================================

		switchGroup := apiV2.Group("/switch")
		{
			switchGroup.GET("/tools/versions", handler.GetToolVersions)
			switchGroup.GET("/presets", handler.ListSwitchPresets)
		}

		apiV2.GET("/tools/download-manifest", handler.GetToolDownloadManifest)

		// ================================================================
		// Platform User Routes (Zitadel JWT auth)
		// ================================================================

		platformUser := apiV2.Group("/user")
		platformUser.Use(middleware.ZitadelAuth())
		{
			platformUser.GET("/identity-overview", handler.GetIdentityOverview)

			billingRoute := platformUser.Group("/billing")
			{
				billingRoute.GET("/summary", handler.GetBillingSummary)
				billingRoute.GET("/payment-methods", handler.GetBillingPaymentMethods)
				billingRoute.POST("/checkout", handler.CreateBillingCheckout)
				billingRoute.GET("/checkout/:order_no/status", handler.GetBillingCheckoutStatus)
			}
		}

		// ================================================================
		// Platform Admin Routes (AdminJWTAuth with root role)
		// ================================================================

		adminRoute := apiV2.Group("/admin")
		adminRoute.Use(middleware.RootJWTAuth())
		{
			tenantMgmt := adminRoute.Group("/tenants")
			{
				tenantMgmt.GET("", handler.ListTenants)
				tenantMgmt.POST("", handler.CreateTenant)
				tenantMgmt.GET("/:id", handler.GetTenant)
				tenantMgmt.PUT("/:id", handler.UpdateTenant)
				tenantMgmt.DELETE("/:id", handler.DeleteTenant)
				tenantMgmt.POST("/:id/enable", handler.EnableTenant)
				tenantMgmt.POST("/:id/disable", handler.DisableTenant)
				tenantMgmt.POST("/:id/suspend", handler.SuspendTenant)
				tenantMgmt.GET("/:id/stats", handler.GetTenantStats)
			}

			mappingRoute := adminRoute.Group("/mappings")
			{
				mappingRoute.GET("", handler.ListUserMappingsV2)
				mappingRoute.GET("/:id", handler.GetUserMappingV2)
				mappingRoute.DELETE("/:id", handler.DeleteUserMappingV2)
			}

			adminRoute.GET("/stats", handler.GetSystemStatsV2)
			adminRoute.POST("/switch/presets", handler.CreateSwitchPreset)

			// Governance (rate-limited: heavy aggregation queries)
			govRoute := adminRoute.Group("/governance")
			govRoute.Use(middleware.CriticalRateLimit())
			{
				govRoute.GET("/channels", handler.GetGovernanceChannelDistribution)
				govRoute.GET("/fingerprints", handler.GetGovernanceFingerprintStats)
				govRoute.GET("/latency", handler.GetGovernanceLatencyStats)
				govRoute.GET("/efficiency", handler.GetGovernanceEfficiencyStats)
			}
			adminRoute.GET("/audit/events", middleware.CriticalRateLimit(), handler.GetAuditEvents)
		}
	}
}

package middleware

import (
	"net/http"

	"github.com/LurusTech/lurus-hub/internal/adapter/repo"

	"github.com/gin-gonic/gin"
)

// TenantSlugGuard enforces that the :tenant_slug in the URL matches the
// tenant the caller actually authenticated into, and that the resolved
// tenant is still enabled. Without this, authHelper (auth.go) only ever
// resolves the CALLER's own tenant — it never looks at the URL slug — so
// every /api/v2/:tenant_slug/* route silently acted on the caller's own
// tenant regardless of what slug was typed in, and a disabled/suspended
// tenant kept working as long as its members' sessions stayed valid.
//
// Must be mounted AFTER UserAuth/AdminAuth so tenant_context is populated
// (mirrors the per-endpoint idiom already used by GetCreditPoolForEndUser
// in tenant_credit_pool.go).
func TenantSlugGuard() gin.HandlerFunc {
	return func(c *gin.Context) {
		slug := c.Param("tenant_slug")
		if slug == "" {
			// Slug-less v2 routes (e.g. /api/v2/user/*, /api/v2/admin/*) don't
			// carry a tenant in the URL — nothing to compare against.
			c.Next()
			return
		}

		tenant, err := repo.GetTenantBySlug(slug)
		if err != nil || tenant == nil {
			c.JSON(http.StatusNotFound, gin.H{
				"success":    false,
				"message":    "Tenant not found",
				"error_code": "TENANT_NOT_FOUND",
			})
			c.Abort()
			return
		}

		tenantCtx, err := GetTenantContext(c)
		if err != nil || tenantCtx == nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success":    false,
				"message":    "Tenant context not found",
				"error_code": "UNAUTHENTICATED",
			})
			c.Abort()
			return
		}

		if tenant.Id != tenantCtx.TenantID {
			c.JSON(http.StatusForbidden, gin.H{
				"success":    false,
				"message":    "Authenticated tenant does not match URL slug",
				"error_code": "TENANT_MISMATCH",
			})
			c.Abort()
			return
		}

		if tenant.IsDisabled() {
			c.JSON(http.StatusForbidden, gin.H{
				"success":    false,
				"message":    "Tenant is disabled or suspended",
				"error_code": "TENANT_DISABLED",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

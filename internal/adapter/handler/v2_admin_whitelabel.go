package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"

	"github.com/LurusTech/lurus-hub/internal/adapter/repo"
	"github.com/LurusTech/lurus-hub/internal/app/governance"
	"github.com/LurusTech/lurus-hub/internal/pkg/common"

	"github.com/gin-gonic/gin"
)

// ============================================================================
// V2 Platform Admin White-Label Controllers
// White-label HMAC key derivation for Switch installer signing.
// ============================================================================

// whitelabelMasterSecretEnv is the environment variable holding the master
// secret used to derive per-tenant HMAC keys. The Switch packager calls
// this endpoint when building Reseller-branded EndUser installers; the
// returned key signs the sidecar whitelabel.json so distribution-time
// tampering (CDN swap, archive edits) is detectable on first launch.
const whitelabelMasterSecretEnv = "LURUS_WHITELABEL_MASTER_SECRET"

// GetWhiteLabelHMACKey derives a deterministic per-tenant HMAC key from
// the configured master secret. Platform admins only.
//
// Route: GET /api/v2/admin/whitelabel/hmac-key?tenant_slug=<slug>
//
// Auth: RootJWTAuth (platform-level — adminRoute group). Tenant slug comes
// from a query parameter because adminRoute is NOT tenant-scoped (no
// :tenant_slug URL binding, and the TenantContext.TenantID is the
// GORM tenant ID, not the slug).
//
// Key derivation: sha256(masterSecret || tenantSlug) → 32 bytes →
// hex.EncodeToString → 64 hex chars. Matches the format expected by the
// Switch sidecar verifier (see 2c-gui-switch/bindings_whitelabel.go).
//
// Fail-fast semantics: if LURUS_WHITELABEL_MASTER_SECRET is unset or empty
// the endpoint returns 500 — we deliberately do NOT fall back to a random
// or hardcoded secret, because that would silently invalidate signatures
// across server restarts and defeat the tamper-detection guarantee.
func GetWhiteLabelHMACKey(c *gin.Context) {
	masterSecret := os.Getenv(whitelabelMasterSecretEnv)
	if masterSecret == "" {
		common.SysError("white-label HMAC requested but " + whitelabelMasterSecretEnv + " not configured")
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": whitelabelMasterSecretEnv + " not configured",
		})
		return
	}

	tenantSlug := c.Query("tenant_slug")
	if tenantSlug == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "tenant_slug query parameter is required",
		})
		return
	}

	if !isValidTenantSlug(tenantSlug) {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "invalid tenant_slug format",
		})
		return
	}

	// Verify the tenant actually exists. Without this check we'd happily
	// derive keys for arbitrary slugs, which is a footgun (no error when
	// the Reseller mistypes their slug at packaging time; mismatched key
	// only surfaces months later on the first EndUser launch).
	if _, err := repo.GetTenantBySlug(tenantSlug); err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "tenant not found",
		})
		return
	}

	// Deterministic derivation. Direct concatenation is fine here — slug
	// charset (a-z, 0-9, -, _) does not collide with anything in a
	// reasonable master secret.
	sum := sha256.Sum256([]byte(masterSecret + tenantSlug))
	hexKey := hex.EncodeToString(sum[:])

	// Audit high-value endpoint access: the derived HMAC key signs Switch
	// installers. Audit records the slug only — never the derived key.
	governance.RecordAuditEvent(governance.NewAuditEvent(c, governance.ActorAdmin, c.GetInt("id"),
		governance.ActionWhitelabelKeyAccessed, governance.ResourceTenant, 0,
		fmt.Sprintf(`{"tenant_slug":%q}`, tenantSlug)))

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"hmac_key": hexKey,
		},
	})
}

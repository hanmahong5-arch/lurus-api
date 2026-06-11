package handler

import (
	"github.com/gin-gonic/gin"
)

// ============================================================================
// V2 Platform Admin — System Options
// Registered under adminRoute (RootJWTAuth, session-fallback). Thin wrappers
// over the existing GetOptions/UpdateOption handlers: the secret-key filtering
// (GetOptions) and per-key validation + audit logging (UpdateOption) are NOT
// re-implemented here — they are delegated so there is a single source of truth.
// ============================================================================

// ListAdminOptionsV2 returns the safe (secret-suffixed keys filtered out)
// key/value option list.
// Route: GET /api/v2/admin/options
func ListAdminOptionsV2(c *gin.Context) {
	if _, ok := requireRoot(c); !ok {
		return
	}
	GetOptions(c)
}

// UpdateAdminOptionV2 updates a single option key. Body: { key, value }.
// Per-key validation (e.g. rejecting GitHubOAuthEnabled without a client id)
// and audit logging are performed by the delegated UpdateOption handler.
// Route: PUT /api/v2/admin/options
func UpdateAdminOptionV2(c *gin.Context) {
	if _, ok := requireRoot(c); !ok {
		return
	}
	UpdateOption(c)
}

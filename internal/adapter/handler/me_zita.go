package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	zita "github.com/hanmahong5-arch/zita-sdk-go"
)

// GetZitaIdentity reports the resolved zita identity for the current
// session. Diagnostic endpoint used during Layer C bring-up to verify
// SDK middleware actually parses platform-issued cookies against the
// newhub deployment. Removed (or hardened) once the v2 user pages
// adopt SDK identity directly.
//
// Requires zita.AuthMiddleware to have run (otherwise ContextKey is
// missing and we 401).
func GetZitaIdentity(c *gin.Context) {
	id, ok := zita.IdentityFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":   "no_identity",
			"message": "zita identity missing from context",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"account_id": id.AccountID,
	})
}

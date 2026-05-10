package handler

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/LurusTech/lurus-hub/internal/adapter/repo"
	"github.com/LurusTech/lurus-hub/internal/pkg/common"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	zita "github.com/hanmahong5-arch/zita-sdk-go"
	"gorm.io/gorm"
)

// ZitaBootstrap converts a platform-issued SDK session into a newhub
// session. Replaces the legacy Zitadel OAuth callback path for the v2
// frontend (ADR-0011 Layer C).
//
// Flow:
//  1. zita.AuthMiddleware validates the lurus_session cookie and stuffs
//     *Identity into context.
//  2. Look up users WHERE lurus_account_id = identity.AccountID.
//  3. If absent, auto-create with username "lurus_<account_id>" — the SDK
//     MVP only carries account_id, so we cannot seed email/displayName
//     from claims (those land with v0.2.x Whoami()).
//  4. Set V1 session vars (id/username/role/status/group) so existing
//     middleware.UserAuth() admits the request, and return user JSON for
//     the frontend's localStorage 'user' shim.
//
// Route: POST /api/v2/auth/zita-bootstrap (must be registered behind
// ZitaClient.AuthMiddleware()).
func ZitaBootstrap(c *gin.Context) {
	id, ok := zita.IdentityFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "zita identity missing — SDK middleware did not validate session",
		})
		return
	}
	if id.AccountID <= 0 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "zita identity carries no account_id",
		})
		return
	}

	user, err := repo.GetUserByLurusAccountID(id.AccountID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		user, err = autoCreateBridgedUser(id.AccountID)
		if err != nil {
			common.SysError(fmt.Sprintf("zita-bootstrap: auto-create user failed (account_id=%d): %v", id.AccountID, err))
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Failed to provision newhub account",
			})
			return
		}
		common.SysLog(fmt.Sprintf("zita-bootstrap: auto-created user %s (id=%d, lurus_account_id=%d)", user.Username, user.Id, id.AccountID))
	} else if err != nil {
		common.SysError(fmt.Sprintf("zita-bootstrap: lookup failed (account_id=%d): %v", id.AccountID, err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to resolve account",
		})
		return
	}

	if user.Status != common.UserStatusEnabled {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": "User account is disabled",
		})
		return
	}

	session := sessions.Default(c)
	session.Set("id", user.Id)
	session.Set("username", user.Username)
	session.Set("role", user.Role)
	session.Set("status", user.Status)
	session.Set("group", user.Group)
	session.Set("identity_account_id", id.AccountID)
	if err := session.Save(); err != nil {
		common.SysError(fmt.Sprintf("zita-bootstrap: session save failed: %v", err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to persist session",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"id":           user.Id,
			"username":     user.Username,
			"display_name": user.DisplayName,
			"role":         user.Role,
			"status":       user.Status,
			"group":        user.Group,
			"email":        user.Email,
		},
	})
}

// autoCreateBridgedUser provisions a minimum-viable newhub user for a
// platform-authenticated visitor with no existing binding. Username is
// derived from account_id (guaranteed unique by the platform); email and
// display_name stay empty until the SDK Whoami() ships and the bridge
// can backfill them.
func autoCreateBridgedUser(accountID int64) (*repo.User, error) {
	username := fmt.Sprintf("lurus_%d", accountID)
	user := &repo.User{
		Username:       username,
		DisplayName:    username,
		Role:           common.RoleCommonUser,
		Status:         common.UserStatusEnabled,
		Group:          "default",
		TenantId:       "default",
		LurusAccountID: &accountID,
	}
	if err := user.Insert(); err != nil {
		return nil, err
	}
	// Re-read by lurus_account_id to pick up DB-assigned id and defaults
	// applied by the Insert path (quota grant, sidebar config init).
	return repo.GetUserByLurusAccountID(accountID)
}

package handler

import (
	"errors"
	"net/http"
	"strings"

	"github.com/LurusTech/lurus-hub/internal/adapter/repo"
	"github.com/LurusTech/lurus-hub/internal/pkg/common"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// GetSwitchUserInfo returns the quota/identity snapshot for the user owning
// the presented relay token.
//
// GET /api/v2/switch/user/info
//
// Contract (mirrors lurus-switch internal/billing/client.go GetUserInfo —
// keep the two in lockstep):
//
//	200: {"success":true,"data":{quota,used_quota,remaining_quota,daily_quota,
//	     group,username,display_name,role}}
//	401: missing/unknown/disabled token or user
//	500: transient lookup failure
//
// Authentication is the raw relay token (Token.Key) in `Authorization`
// (optional "Bearer "/"sk-" prefixes, optional "-<channel>" suffix) — the
// same convention as /api/v2/switch/heartbeat, and deliberately NOT
// middleware.UserAuth(), which resolves User.AccessToken rather than
// Token.Key and would reject every Switch client.
//
// remaining_quota is the amount actually spendable through THIS token: the
// user balance when the token is unlimited, else the token's own remaining
// allowance capped by the user balance.
func GetSwitchUserInfo(c *gin.Context) {
	key := strings.TrimSpace(c.Request.Header.Get("Authorization"))
	if strings.HasPrefix(key, "Bearer ") || strings.HasPrefix(key, "bearer ") {
		key = strings.TrimSpace(key[7:])
	}
	key = strings.TrimPrefix(key, "sk-")
	// sk-<key>-<channel> form (relay convention) — quota lookup ignores channel.
	if idx := strings.IndexByte(key, '-'); idx > 0 {
		key = key[:idx]
	}
	if key == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "missing Authorization token"})
		return
	}

	token, err := repo.GetTokenByKey(key, false)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "token lookup failed"})
		return
	}
	if err != nil || token == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "token not found"})
		return
	}
	if token.Status == common.TokenStatusDisabled {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "token disabled"})
		return
	}

	user, err := repo.GetUserById(token.UserId)
	if err != nil || user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "user not found"})
		return
	}
	if user.Status == common.UserStatusDisabled {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "user disabled"})
		return
	}

	remaining := user.Quota
	if !token.UnlimitedQuota && token.RemainQuota < remaining {
		remaining = token.RemainQuota
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"quota":           user.Quota,
			"used_quota":      user.UsedQuota,
			"remaining_quota": remaining,
			"daily_quota":     user.DailyQuota,
			"group":           user.Group,
			"username":        user.Username,
			"display_name":    user.DisplayName,
			"role":            user.Role,
		},
	})
}

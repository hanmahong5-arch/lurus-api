package middleware

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/LurusTech/lurus-hub/internal/adapter/repo"
	"github.com/LurusTech/lurus-hub/internal/app"
	"github.com/LurusTech/lurus-hub/internal/pkg/common"
)

// CostSpikeLimit is a Gin middleware that blocks LLM relay requests when a
// user exceeds the configured 5-minute quota window. On breach the user is
// auto-disabled and the request is rejected with 429.
//
// Place AFTER TokenAuth (which sets the "id" context key). When Redis is
// unavailable or the feature is disabled, the middleware fails open — better
// to allow legitimate traffic than block on infrastructure issues.
//
// Ported from 2b-svc-newapi/middleware/cost_spike.go (2026-05-07).
func CostSpikeLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !common.CostSpikeProtectionEnabled {
			c.Next()
			return
		}
		userID := c.GetInt("id")
		if userID == 0 {
			c.Next()
			return
		}
		if !common.RedisEnabled || common.RDB == nil {
			c.Next()
			return
		}

		ctx := c.Request.Context()
		windowUsed, err := app.QueryCostSpikeWindow(ctx, common.RDB, userID)
		if err != nil {
			// Fail open on Redis errors — don't block legit traffic.
			common.SysLog(fmt.Sprintf("cost_spike check error user %d: %s", userID, err.Error()))
			c.Next()
			return
		}

		limit := int64(common.CostSpikeHardLimitPer5Min)
		if windowUsed < limit {
			c.Next()
			return
		}

		// Breach: auto-disable and 429.
		if disableErr := repo.DisableUserById(userID); disableErr != nil {
			common.SysLog(fmt.Sprintf("cost_spike disable user %d failed: %s", userID, disableErr.Error()))
		}
		common.SysLogf(
			`{"event":"cost_spike_triggered","user_id":%d,"window_used":%d,"limit":%d,"action":"auto_disabled"}`,
			userID, windowUsed, limit,
		)
		c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
			"error": gin.H{
				"message": "cost spike limit exceeded; account temporarily disabled for safety",
				"type":    "new_api_error",
				"code":    "cost_spike_limit_exceeded",
			},
		})
	}
}

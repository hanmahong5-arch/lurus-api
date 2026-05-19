package middleware

import (
	"errors"
	"net/http"

	"github.com/LurusTech/lurus-hub/internal/adapter/repo"
	"github.com/LurusTech/lurus-hub/internal/pkg/common"
	"github.com/LurusTech/lurus-hub/internal/pkg/metrics"

	"github.com/gin-gonic/gin"
)

// PoolBalanceCheck is the relay-side gate that enforces tenant credit pool
// limits before any upstream provider call. It runs after TokenAuth (so the
// tenant is identified) and before CostSpikeLimit / Distribute (so an
// exhausted pool short-circuits the rest of the relay chain).
//
// Behaviour (ADR 2026-05-18 (tenant-credit-pool) §5 enforcement order):
//
//	no pool row        → bypass (treated as unlimited; back-compat default)
//	unlimited pool     → bypass (MaxBalance == -1 sentinel)
//	exhausted pool     → HTTP 402 with structured body, abort chain
//	any DB error       → log and bypass (fail-open; don't break traffic on
//	                     transient repo issues — schema dedup at debit time
//	                     remains the safety net for over-consumption)
//
// Position in chain: AFTER TokenAuth, BEFORE CostSpikeLimit. Inserted on five
// relay groups: /v1, /mj (+ /:mode/mj), /suno, /v1/audio, /v1beta.
func PoolBalanceCheck() gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantCtx, err := GetTenantContext(c)
		if err != nil || tenantCtx == nil || tenantCtx.TenantID == "" {
			// No tenant identified yet (e.g. anonymous /v1/models call slipping
			// through). Pool gate has no opinion — let downstream decide.
			c.Next()
			return
		}

		tenantID := tenantCtx.TenantID
		pool, err := repo.GetTenantCreditPool(tenantID)
		if err != nil {
			if errors.Is(err, repo.ErrPoolNotFound) {
				// ADR §5: absence of a row = unlimited. Bypass.
				c.Next()
				return
			}
			// Transient DB issue → fail open, but log so ops see it.
			common.SysError("pool_balance_check: tenant=" + tenantID + " err=" + err.Error())
			c.Next()
			return
		}

		if pool.IsUnlimited() {
			c.Next()
			return
		}

		if pool.IsExhausted() {
			c.JSON(http.StatusPaymentRequired, gin.H{
				"error": gin.H{
					"code":      "pool_exhausted",
					"message":   "Tenant credit pool exhausted",
					"tenant_id": tenantID,
				},
			})
			c.Abort()
			return
		}

		// Healthy pool — expose the live balance to dashboards even on the
		// read path so a Reseller sees a fresh value without waiting for the
		// next debit. Cheap: one gauge Set per request.
		metrics.CreditPoolBalance.WithLabelValues(tenantID).Set(float64(pool.CurrentBalance))
		c.Next()
	}
}

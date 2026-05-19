package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/LurusTech/lurus-hub/internal/adapter/repo"
	"github.com/LurusTech/lurus-hub/internal/pkg/common"
	"github.com/LurusTech/lurus-hub/internal/pkg/metrics"

	"github.com/gin-gonic/gin"
)

// Reseller-facing admin handlers for tenant credit pools.
// Routes registered under /api/v2/admin/tenants/:id/credit-pool* in
// api-v2-router.go. Auth: middleware.RootJWTAuth (Platform admin / Reseller).
//
// Canonical: ADR 2026-05-18 (tenant-credit-pool) §4.1.

// CreateCreditPool initialises a credit pool row for a tenant.
// Route: POST /api/v2/admin/tenants/:id/credit-pool
// Body:  { max_balance, reset_period, alert_threshold_pct }
//
// 201 with pool body, or 409 if a pool already exists for the tenant.
// `max_balance == -1` means unlimited (relay gate becomes a no-op).
func CreateCreditPool(c *gin.Context) {
	tenantID := c.Param("id")
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "tenant id required"})
		return
	}
	if _, err := repo.GetTenantByID(tenantID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "Tenant not found"})
		return
	}

	if existing, err := repo.GetTenantCreditPool(tenantID); err == nil && existing != nil {
		c.JSON(http.StatusConflict, gin.H{
			"success":    false,
			"message":    "Credit pool already exists for tenant",
			"error_code": "POOL_ALREADY_EXISTS",
		})
		return
	}

	var req struct {
		MaxBalance        int64  `json:"max_balance"`
		ResetPeriod       string `json:"reset_period"`
		AlertThresholdPct int    `json:"alert_threshold_pct"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid request: " + err.Error()})
		return
	}
	if req.MaxBalance < 0 && req.MaxBalance != repo.PoolMaxBalanceUnlimited {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "max_balance must be >= 0 or -1 (unlimited)",
		})
		return
	}

	actorID := c.GetInt("id")
	pool, err := repo.CreateTenantCreditPool(tenantID, actorID, req.MaxBalance, req.ResetPeriod, req.AlertThresholdPct)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to create pool: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"success": true, "data": pool})
}

// GetCreditPool returns the pool row for a tenant.
// Route: GET /api/v2/admin/tenants/:id/credit-pool
// Returns 404 with ErrPoolNotFound — distinct from the relay-gate semantics
// where absence means "unlimited bypass".
func GetCreditPool(c *gin.Context) {
	tenantID := c.Param("id")
	pool, err := repo.GetTenantCreditPool(tenantID)
	if err != nil {
		if errors.Is(err, repo.ErrPoolNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"success":    false,
				"message":    "Credit pool not configured for tenant",
				"error_code": "POOL_NOT_FOUND",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": pool})
}

// TopupCreditPool funds the pool from the actor's platform wallet.
// Route: POST /api/v2/admin/tenants/:id/credit-pool/topup
// Body:  { amount, reason? }
//
// Flow per ADR §9 Q4 (wallet-debit only — no admin-grant path):
//  1. Lookup pool (must exist).
//  2. DebitWalletGRPC(amount) — if it fails, return 402.
//  3. TopupPool(amount) — if it fails (ErrPoolWouldExceedCeiling), call
//     CreditWalletGRPC for revert; if revert also fails, log STRANDED for ops.
//
// The two-step is necessarily non-atomic across services, so we accept a
// narrow stranded-debit risk and surface it loudly rather than absorbing it.
func TopupCreditPool(c *gin.Context) {
	tenantID := c.Param("id")

	pool, err := repo.GetTenantCreditPool(tenantID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false, "message": "Credit pool not found", "error_code": "POOL_NOT_FOUND",
		})
		return
	}

	var req struct {
		Amount int64  `json:"amount" binding:"required"`
		Reason string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Amount <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "amount must be a positive integer"})
		return
	}

	actorID := c.GetInt("id")
	actor, err := repo.GetUserById(actorID, false)
	if err != nil || actor == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "Actor not found"})
		return
	}
	if actor.LurusAccountID == nil || *actor.LurusAccountID <= 0 {
		c.JSON(http.StatusPreconditionFailed, gin.H{
			"success": false, "message": "Actor has no platform wallet (lurus_account_id unset)",
		})
		return
	}
	accountID := *actor.LurusAccountID

	walletAmount := float64(req.Amount) / 1000.0 // 1 LB ≈ 1000 quota units, matches existing relay accounting
	debit, derr := common.DebitWalletGRPC(
		c.Request.Context(), accountID, walletAmount,
		"pool_topup", "Credit pool topup for tenant "+tenantID, "newhub",
	)
	if derr != nil || debit == nil || !debit.Success {
		c.JSON(http.StatusPaymentRequired, gin.H{
			"success":    false,
			"message":    "Wallet debit failed; topup aborted",
			"error_code": "WALLET_DEBIT_FAILED",
		})
		return
	}

	newBalance, terr := repo.TopupPool(pool.ID, tenantID, req.Amount, actorID, req.Reason)
	if terr != nil {
		// Revert the wallet — best effort.
		if rerr := common.CreditWalletGRPC(
			c.Request.Context(), accountID, walletAmount,
			"pool_topup_revert",
			"Revert: pool topup failed for tenant "+tenantID,
			"newhub",
		); rerr != nil {
			common.SysError("STRANDED wallet debit — pool topup AND revert both failed. " +
				"account=" + strconv.FormatInt(accountID, 10) +
				" tenant=" + tenantID +
				" amount=" + strconv.FormatInt(req.Amount, 10) +
				" pool_err=" + terr.Error() + " revert_err=" + rerr.Error())
		}
		status := http.StatusInternalServerError
		code := "POOL_TOPUP_FAILED"
		if errors.Is(terr, repo.ErrPoolWouldExceedCeiling) {
			status = http.StatusConflict
			code = "POOL_CEILING_EXCEEDED"
		}
		c.JSON(status, gin.H{"success": false, "message": terr.Error(), "error_code": code})
		return
	}

	metrics.CreditPoolBalance.WithLabelValues(tenantID).Set(float64(newBalance))

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"tenant_id":   tenantID,
			"new_balance": newBalance,
			"max_balance": pool.MaxBalance,
		},
	})
}

// ListCreditPoolUsage returns paginated draw history.
// Route: GET /api/v2/admin/tenants/:id/credit-pool/usage?offset=&limit=
func ListCreditPoolUsage(c *gin.Context) {
	tenantID := c.Param("id")
	pool, err := repo.GetTenantCreditPool(tenantID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false, "message": "Credit pool not found", "error_code": "POOL_NOT_FOUND",
		})
		return
	}

	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))

	draws, total, err := repo.ListPoolDraws(pool.ID, offset, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"draws":  draws,
			"total":  total,
			"offset": offset,
			"limit":  limit,
		},
	})
}

// DeleteCreditPool soft-drains the pool: sets max_balance to zero so the
// relay gate blocks new debits, and records a final adjustment draw.
// Route: DELETE /api/v2/admin/tenants/:id/credit-pool
//
// We do NOT hard-delete: the audit ledger references pool_id; keeping the
// row preserves draw history. A future "recreate" path can reset the row.
func DeleteCreditPool(c *gin.Context) {
	tenantID := c.Param("id")
	pool, err := repo.GetTenantCreditPool(tenantID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false, "message": "Credit pool not found", "error_code": "POOL_NOT_FOUND",
		})
		return
	}

	if err := repo.DB.Model(&repo.TenantCreditPool{}).
		Where("id = ?", pool.ID).
		Update("max_balance", 0).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}

	drained := &repo.TenantCreditPoolDraw{
		PoolID:      pool.ID,
		TenantID:    tenantID,
		Direction:   repo.PoolDrawDirectionDebit,
		Amount:      pool.CurrentBalance,
		Reason:      repo.PoolDrawReasonAdjustment,
		ActorUserID: c.GetInt("id"),
	}
	if err := repo.DB.Create(drained).Error; err != nil {
		common.SysError("DeleteCreditPool: drained-draw insert failed: " + err.Error())
	}

	metrics.CreditPoolBalance.WithLabelValues(tenantID).Set(0)
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Credit pool drained"})
}

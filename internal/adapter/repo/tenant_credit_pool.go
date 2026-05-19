package repo

import (
	"errors"
	"fmt"
	"time"

	"github.com/LurusTech/lurus-hub/internal/domain/entity"

	"gorm.io/gorm"
)

// Re-export entity types so handler / app layers can use repo.* without
// importing the entity package directly (matches Tenant / Token convention).
type TenantCreditPool = entity.TenantCreditPool
type TenantCreditPoolDraw = entity.TenantCreditPoolDraw

// Re-export pool-related constants from entity.
const (
	PoolResetNone    = entity.PoolResetNone
	PoolResetDaily   = entity.PoolResetDaily
	PoolResetWeekly  = entity.PoolResetWeekly
	PoolResetMonthly = entity.PoolResetMonthly

	PoolMaxBalanceUnlimited = entity.PoolMaxBalanceUnlimited

	PoolDrawReasonRelayDebit = entity.PoolDrawReasonRelayDebit
	PoolDrawReasonTopup      = entity.PoolDrawReasonTopup
	PoolDrawReasonReset      = entity.PoolDrawReasonReset
	PoolDrawReasonAdjustment = entity.PoolDrawReasonAdjustment
)

// Direction constants (int16, separate const block to preserve type).
const (
	PoolDrawDirectionDebit  = entity.PoolDrawDirectionDebit
	PoolDrawDirectionCredit = entity.PoolDrawDirectionCredit
)

// ErrPoolExhausted is returned by DebitPool when the conditional UPDATE
// affected zero rows — current_balance < requested amount. The relay
// enforcement layer maps this to HTTP 402 (ADR §5 precedence step 4).
var ErrPoolExhausted = errors.New("tenant credit pool exhausted")

// ErrPoolNotFound is returned by GetTenantCreditPool when the tenant has no
// pool row. Callers MUST treat this as "unlimited" — it is not an error
// condition (ADR §5 edge case: no pool row = pool gate skipped).
var ErrPoolNotFound = errors.New("tenant credit pool not found")

// ErrPoolWouldExceedCeiling is returned by TopupPool when the requested
// topup amount would push current_balance past max_balance.
var ErrPoolWouldExceedCeiling = errors.New("topup would exceed pool max_balance ceiling")

// GetTenantCreditPool fetches the pool row for a tenant. Returns
// ErrPoolNotFound when no row exists — callers MUST interpret that as
// "unlimited" and bypass the pool gate (not as an error).
func GetTenantCreditPool(tenantID string) (*TenantCreditPool, error) {
	var pool TenantCreditPool
	err := DB.Where("tenant_id = ?", tenantID).First(&pool).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPoolNotFound
		}
		return nil, fmt.Errorf("get tenant credit pool: %w", err)
	}
	return &pool, nil
}

// CreateTenantCreditPool inserts a fresh pool row for a tenant. Used by the
// Reseller-facing POST /api/v2/admin/tenants/:id/credit-pool endpoint.
// Defaults: resetPeriod = "monthly" (ADR §9 Q1), alertThresholdPct = 80,
// initial CurrentBalance = 0 (Resellers topup separately).
func CreateTenantCreditPool(tenantID string, createdByUserID int, maxBalance int64, resetPeriod string, alertThresholdPct int) (*TenantCreditPool, error) {
	if resetPeriod == "" {
		resetPeriod = PoolResetMonthly
	}
	if alertThresholdPct <= 0 || alertThresholdPct > 100 {
		alertThresholdPct = 80
	}

	now := time.Now()
	pool := &TenantCreditPool{
		TenantID:          tenantID,
		CreatedByUserID:   createdByUserID,
		CurrentBalance:    0,
		MaxBalance:        maxBalance,
		ResetPeriod:       resetPeriod,
		LastResetAt:       now,
		NextResetAt:       nextResetAt(resetPeriod, now),
		AlertThresholdPct: alertThresholdPct,
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	if err := DB.Create(pool).Error; err != nil {
		return nil, fmt.Errorf("create tenant credit pool: %w", err)
	}
	return pool, nil
}

// DebitPool atomically deducts amount from the tenant pool and records a
// draw ledger row in the same transaction. Returns ErrPoolExhausted when
// the pool has insufficient balance.
//
// Atomicity comes from the conditional UPDATE (ADR §7 risk #1):
//
//	UPDATE tenant_credit_pools
//	   SET current_balance = current_balance - ?, updated_at = NOW()
//	 WHERE id = ? AND current_balance >= ?
//
// PostgreSQL READ COMMITTED is sufficient — the implicit row-level lock
// during UPDATE serializes concurrent debits. SERIALIZABLE not needed.
//
// TODO(phase-2 wiring): called by relay middleware after upstream success,
// before BillingOutbox settlement.
func DebitPool(poolID int64, tenantID string, amount int64, tokenID int, logID int64) error {
	if amount <= 0 {
		return fmt.Errorf("debit amount must be positive, got %d", amount)
	}

	return DB.Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&TenantCreditPool{}).
			Where("id = ? AND current_balance >= ?", poolID, amount).
			Updates(map[string]interface{}{
				"current_balance": gorm.Expr("current_balance - ?", amount),
				"updated_at":      time.Now(),
			})
		if result.Error != nil {
			return fmt.Errorf("debit pool update: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return ErrPoolExhausted
		}

		draw := &TenantCreditPoolDraw{
			PoolID:    poolID,
			TenantID:  tenantID,
			TokenID:   tokenID,
			LogID:     logID,
			Direction: PoolDrawDirectionDebit,
			Amount:    amount,
			Reason:    PoolDrawReasonRelayDebit,
			CreatedAt: time.Now(),
		}
		if err := tx.Create(draw).Error; err != nil {
			return fmt.Errorf("debit pool draw insert: %w", err)
		}
		return nil
	})
}

// TopupPool atomically increments the pool balance and writes a credit draw
// row. Returns the new balance for the response payload.
//
// ADR §9 Q4 (Accepted): topup MUST be funded by the Reseller's platform
// wallet. The WalletDebit call lives in the handler, not the repo, so the
// BillingOutbox pattern can wrap both wallet debit and this pool increment
// in a single fail-safe flow. No admin-grant path exists by design.
//
// Returns ErrPoolWouldExceedCeiling when amount + current_balance > max_balance
// (unlimited pools always succeed).
//
// TODO(phase-2 wiring): handler at POST /api/v2/admin/tenants/:id/credit-pool/topup
// MUST call DebitWalletGRPC first, then this function, with outbox-managed
// rollback on partial failure.
func TopupPool(poolID int64, tenantID string, amount int64, actorUserID int, reason string) (int64, error) {
	if amount <= 0 {
		return 0, fmt.Errorf("topup amount must be positive, got %d", amount)
	}
	if reason == "" {
		reason = PoolDrawReasonTopup
	}

	var newBalance int64
	err := DB.Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&TenantCreditPool{}).
			Where(
				"id = ? AND (max_balance = ? OR current_balance + ? <= max_balance)",
				poolID, PoolMaxBalanceUnlimited, amount,
			).
			Updates(map[string]interface{}{
				"current_balance": gorm.Expr("current_balance + ?", amount),
				"updated_at":      time.Now(),
			})
		if result.Error != nil {
			return fmt.Errorf("topup pool update: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return ErrPoolWouldExceedCeiling
		}

		var pool TenantCreditPool
		if err := tx.Select("current_balance").Where("id = ?", poolID).First(&pool).Error; err != nil {
			return fmt.Errorf("topup pool readback: %w", err)
		}
		newBalance = pool.CurrentBalance

		draw := &TenantCreditPoolDraw{
			PoolID:      poolID,
			TenantID:    tenantID,
			Direction:   PoolDrawDirectionCredit,
			Amount:      amount,
			Reason:      reason,
			ActorUserID: actorUserID,
			CreatedAt:   time.Now(),
		}
		if err := tx.Create(draw).Error; err != nil {
			return fmt.Errorf("topup pool draw insert: %w", err)
		}
		return nil
	})
	return newBalance, err
}

// ListPoolDraws returns paginated audit-ledger rows for a pool, ordered by
// created_at DESC. Used by GET /api/v2/admin/tenants/:id/credit-pool/usage.
// Limit is clamped to [1, 200] to bound query cost.
func ListPoolDraws(poolID int64, offset int, limit int) ([]*TenantCreditPoolDraw, int64, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	var draws []*TenantCreditPoolDraw
	var total int64

	if err := DB.Model(&TenantCreditPoolDraw{}).
		Where("pool_id = ?", poolID).
		Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count pool draws: %w", err)
	}

	if err := DB.Where("pool_id = ?", poolID).
		Order("created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&draws).Error; err != nil {
		return nil, 0, fmt.Errorf("list pool draws: %w", err)
	}

	return draws, total, nil
}

// nextResetAt computes the next scheduled reset timestamp for a pool.
// Returns nil for "none" (no scheduled reset — manual topup only).
//
// Daily   → next 00:00 UTC
// Weekly  → next Monday 00:00 UTC
// Monthly → 1st of next month 00:00 UTC (matches OpenRouter limit_reset semantics)
func nextResetAt(period string, from time.Time) *time.Time {
	utc := from.UTC()
	var next time.Time
	switch period {
	case PoolResetDaily:
		next = time.Date(utc.Year(), utc.Month(), utc.Day()+1, 0, 0, 0, 0, time.UTC)
	case PoolResetWeekly:
		daysUntilMonday := (8 - int(utc.Weekday())) % 7
		if daysUntilMonday == 0 {
			daysUntilMonday = 7
		}
		next = time.Date(utc.Year(), utc.Month(), utc.Day()+daysUntilMonday, 0, 0, 0, 0, time.UTC)
	case PoolResetMonthly:
		next = time.Date(utc.Year(), utc.Month()+1, 1, 0, 0, 0, 0, time.UTC)
	case PoolResetNone:
		return nil
	default:
		return nil
	}
	return &next
}

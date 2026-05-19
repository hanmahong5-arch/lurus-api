package entity

import (
	"time"
)

// Pool reset period enum for TenantCreditPool.ResetPeriod.
// "monthly" is the accepted default per ADR 2026-05-18 §9 Q1.
const (
	PoolResetNone    = "none"
	PoolResetDaily   = "daily"
	PoolResetWeekly  = "weekly"
	PoolResetMonthly = "monthly"
)

// PoolMaxBalanceUnlimited is the sentinel meaning "no ceiling". When
// MaxBalance equals this value the enforcement layer skips the pool gate.
const PoolMaxBalanceUnlimited int64 = -1

// Draw direction enum for TenantCreditPoolDraw.Direction.
const (
	PoolDrawDirectionDebit  int16 = 1
	PoolDrawDirectionCredit int16 = -1
)

// Draw reason enum for TenantCreditPoolDraw.Reason.
const (
	PoolDrawReasonRelayDebit = "relay_debit"
	PoolDrawReasonTopup      = "topup"
	PoolDrawReasonReset      = "reset"
	PoolDrawReasonAdjustment = "adjustment"
)

// TenantCreditPool holds per-tenant pre-paid balance plus optional ceiling and
// reset schedule. One row per tenant; absence of a row is interpreted as
// "unlimited" by the enforcement layer (backward-compatible default).
//
// Canonical design: ADR 2026-05-18 (tenant-credit-pool) §3.1.
type TenantCreditPool struct {
	ID                int64      `json:"id"                  gorm:"primaryKey;autoIncrement"`
	TenantID          string     `json:"tenant_id"           gorm:"type:varchar(36);not null;uniqueIndex"`
	ParentTenantID    *string    `json:"parent_tenant_id"    gorm:"type:varchar(36);index"`
	CreatedByUserID   int        `json:"created_by_user_id"  gorm:"not null"`
	CurrentBalance    int64      `json:"current_balance"     gorm:"type:bigint;not null;default:0"`
	MaxBalance        int64      `json:"max_balance"         gorm:"type:bigint;not null;default:-1"`
	ResetPeriod       string     `json:"reset_period"        gorm:"type:varchar(16);not null;default:'monthly'"`
	LastResetAt       time.Time  `json:"last_reset_at"       gorm:"not null;default:CURRENT_TIMESTAMP"`
	NextResetAt       *time.Time `json:"next_reset_at"       gorm:"index"`
	AlertThresholdPct int        `json:"alert_threshold_pct" gorm:"not null;default:80"`
	AlertFiredAt      *time.Time `json:"alert_fired_at,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

// TableName overrides the default GORM table name.
func (TenantCreditPool) TableName() string {
	return "tenant_credit_pools"
}

// IsUnlimited reports whether this pool waives the balance ceiling.
// Unlimited pools are treated as "no pool" by the enforcement layer.
func (p *TenantCreditPool) IsUnlimited() bool {
	return p.MaxBalance == PoolMaxBalanceUnlimited
}

// IsExhausted reports whether a finite-ceiling pool has zero remaining
// balance — the trigger for HTTP 402 in the relay enforcement layer.
func (p *TenantCreditPool) IsExhausted() bool {
	return !p.IsUnlimited() && p.CurrentBalance <= 0
}

// ShouldAlert reports whether the current balance has dipped under the
// configured alert threshold. Returns false for unlimited or zero-ceiling
// pools. Caller is responsible for deduplication via AlertFiredAt.
func (p *TenantCreditPool) ShouldAlert() bool {
	if p.IsUnlimited() || p.MaxBalance <= 0 {
		return false
	}
	threshold := p.MaxBalance * int64(p.AlertThresholdPct) / 100
	return p.CurrentBalance < threshold
}

// TenantCreditPoolDraw is the append-only audit ledger entry for every
// balance-changing event against a pool (debit, topup, scheduled reset,
// manual adjustment). Never updated in place.
//
// Canonical design: ADR 2026-05-18 (tenant-credit-pool) §3.2.
type TenantCreditPoolDraw struct {
	ID          int64     `json:"id"                       gorm:"primaryKey;autoIncrement"`
	PoolID      int64     `json:"pool_id"                  gorm:"not null;index:idx_pool_time,priority:1"`
	TenantID    string    `json:"tenant_id"                gorm:"type:varchar(36);not null;index:idx_tenant_time,priority:1"`
	TokenID     int       `json:"token_id,omitempty"`
	LogID       int64     `json:"log_id,omitempty"`
	Direction   int16     `json:"direction"                gorm:"not null"`
	Amount      int64     `json:"amount"                   gorm:"type:bigint;not null"`
	Reason      string    `json:"reason"                   gorm:"type:varchar(32);not null"`
	ActorUserID int       `json:"actor_user_id,omitempty"`
	CreatedAt   time.Time `json:"created_at"               gorm:"index:idx_pool_time,priority:2;index:idx_tenant_time,priority:2"`
}

// TableName overrides the default GORM table name.
func (TenantCreditPoolDraw) TableName() string {
	return "tenant_credit_pool_draws"
}

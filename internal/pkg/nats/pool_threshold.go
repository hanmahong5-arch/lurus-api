package nats

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/LurusTech/lurus-hub/internal/adapter/repo"
	"github.com/LurusTech/lurus-hub/internal/pkg/common"
)

// poolDedupTTL is the lifetime of the Redis SETNX dedup lock.
// Matches the quota_threshold pattern (24h) intent but applied at pool
// granularity with 1h TTL per ADR 2026-05-18 §9 Q5 — pool alerts re-fire
// per hour while balance remains below threshold, giving ops a visible
// heartbeat without spamming.
const poolDedupTTL = 1 * time.Hour

// poolSchemaDedupWindow is the minimum interval between two alert fires
// for the same pool, enforced via the tenant_credit_pools.alert_fired_at
// column. Matches poolDedupTTL — both layers must agree, since either can
// suppress publication on its own. The DB column is the authoritative
// record for ops dashboards (Redis is best-effort and can be down).
const poolSchemaDedupWindow = 1 * time.Hour

// PoolThresholdPayload is the wire payload published on SubjectPoolThreshold.
// Schema is denormalised on purpose: consumers (notification module, future
// Reseller dashboard) should not have to call back into newhub to render
// the alert. fired_at is the publisher-side wall clock; consumers use it
// for ordering not for billing.
type PoolThresholdPayload struct {
	TenantID       string    `json:"tenant_id"`
	PoolID         int64     `json:"pool_id"`
	CurrentBalance int64     `json:"current_balance"`
	MaxBalance     int64     `json:"max_balance"`
	ThresholdPct   int       `json:"threshold_pct"`
	FiredAt        time.Time `json:"fired_at"`
}

// poolRedisDeduper is the minimal Redis SETNX interface needed for pool
// threshold dedup. Mirrors the redisDeduper interface in
// internal/app/quota_threshold.go to keep mocking patterns uniform.
type poolRedisDeduper interface {
	SetNXBool(ctx context.Context, key string, expiration time.Duration) (bool, error)
}

// poolDB is the minimal DB surface the pool-threshold publisher needs:
// one read of alert_fired_at, one update to set it. Decoupled from
// *gorm.DB so the test layer can stub without spinning a real DB.
type poolDB interface {
	// LastAlertFiredAt returns the alert_fired_at timestamp for the given
	// pool, or nil if the column is NULL / row is missing. Returns
	// (nil, gorm.ErrRecordNotFound) when the pool row doesn't exist —
	// callers should fall back to "no prior alert" semantics.
	LastAlertFiredAt(ctx context.Context, poolID int64) (*time.Time, error)
	// MarkAlertFired atomically sets alert_fired_at = NOW() for the pool.
	MarkAlertFired(ctx context.Context, poolID int64, now time.Time) error
}

// poolPublisher is the minimal publish interface — matches thresholdPublisher
// in quota_threshold.go for test parity.
type poolPublisher interface {
	Publish(ctx context.Context, subject string, payload any) error
}

// gormPoolDB is the production implementation of poolDB, backed by repo.DB.
// Kept tiny on purpose: this package mustn't know about the wider repo API.
type gormPoolDB struct{ db *gorm.DB }

func (g *gormPoolDB) LastAlertFiredAt(ctx context.Context, poolID int64) (*time.Time, error) {
	var row struct {
		AlertFiredAt *time.Time `gorm:"column:alert_fired_at"`
	}
	err := g.db.WithContext(ctx).
		Table("tenant_credit_pools").
		Select("alert_fired_at").
		Where("id = ?", poolID).
		Limit(1).
		Scan(&row).Error
	if err != nil {
		return nil, err
	}
	return row.AlertFiredAt, nil
}

func (g *gormPoolDB) MarkAlertFired(ctx context.Context, poolID int64, now time.Time) error {
	return g.db.WithContext(ctx).
		Table("tenant_credit_pools").
		Where("id = ?", poolID).
		Update("alert_fired_at", now).Error
}

// redisCommonDeduper wraps the common.RDB singleton to implement
// poolRedisDeduper. Returns nil when Redis is disabled so callers can
// treat dedup as "always pass" — schema-level dedup is the authoritative
// fallback (see poolSchemaDedupWindow).
type redisCommonDeduper struct{}

func (redisCommonDeduper) SetNXBool(ctx context.Context, key string, expiration time.Duration) (bool, error) {
	if !common.RedisEnabled || common.RDB == nil {
		// No Redis available — let publication proceed. Schema-level
		// dedup (alert_fired_at) will catch duplicates.
		return true, nil
	}
	return common.RDB.SetNX(ctx, key, "1", expiration).Result()
}

// PublishPoolThreshold emits one llm.pool.threshold event for the tenant's
// credit pool, subject to two independent dedup layers:
//
//  1. Schema dedup: tenant_credit_pools.alert_fired_at < poolSchemaDedupWindow
//     ago → no-op. Authoritative for ops visibility.
//  2. Redis SETNX dedup: pool:threshold:<tenantID>:<poolID> with 1h TTL.
//     Best-effort; protects against the (rare) race where two pods both
//     pass the schema check before either updates alert_fired_at.
//
// Both checks are required (per Lane δ spec). Reason: Redis may be down or
// reset; the DB column is the durable record. On success the function:
//
//   - sets tenant_credit_pools.alert_fired_at = NOW() for the pool (FIRST,
//     fail-closed: if the schema mark cannot be written, the event must not
//     be published — otherwise schema dedup loses track and we risk an
//     alert storm when the Redis SETNX expires)
//   - publishes the JSON payload to SubjectPoolThreshold
//
// Returns an error only for genuine infrastructure failures the caller can
// usefully report (DB unreachable, publish enqueue failed). Suppressions
// (either dedup layer) return nil — the publish is a no-op by design.
func PublishPoolThreshold(ctx context.Context, tenantID string, poolID int64, currentBalance, maxBalance int64, thresholdPct int) error {
	if !Enabled() {
		return nil
	}
	pub := Get()
	if pub == nil {
		return nil
	}
	if tenantID == "" || poolID <= 0 {
		return nil
	}

	return publishPoolThreshold(
		ctx,
		tenantID, poolID, currentBalance, maxBalance, thresholdPct,
		pub,
		redisCommonDeduper{},
		&gormPoolDB{db: repo.DB},
		time.Now().UTC(),
	)
}

// publishPoolThreshold is the testable core. Dependencies are injected so
// the test suite can substitute mocks without touching real Redis / NATS /
// gorm. now is also injected to keep behaviour deterministic across runs.
//
// Order of operations is load-bearing (2026-05-19 Phase 2 self-audit
// reordered Steps 3/4 — see below):
//
//  1. Schema dedup check — cheapest path to no-op, also the durable record
//  2. Redis SETNX — short-window race guard between pods
//  3. Schema mark fired — write FIRST so dedup state is durable before
//     anything reaches the wire. If this fails we never publish and the
//     caller sees an error; the next call re-attempts cleanly.
//  4. NATS publish — only after the dedup state is committed. If publish
//     fails, schema dedup has already locked the slot for the window:
//     ops sees an error and can replay manually, but we won't double-fire.
//
// Errors from steps 1, 3, 4 propagate to the caller. Errors from step 2
// (Redis transient failure) are NOT fatal: the schema check has already
// said "ok to fire", so we proceed and rely on schema dedup for safety.
//
// Trade-off acknowledged: a publish failure now produces a "marked but
// not delivered" state for the dedup window. We prefer this to the
// alternative (publish succeeded, mark failed, Redis TTL expires, alert
// storm), which is the failure mode the audit closed.
func publishPoolThreshold(
	ctx context.Context,
	tenantID string,
	poolID int64,
	currentBalance, maxBalance int64,
	thresholdPct int,
	pub poolPublisher,
	rdb poolRedisDeduper,
	db poolDB,
	now time.Time,
) error {
	// Step 1: schema-level dedup. Pool row missing → fall through (no
	// prior alert recorded, treat as ok to fire). Other DB errors abort.
	prev, err := db.LastAlertFiredAt(ctx, poolID)
	switch {
	case err == nil:
		if prev != nil && now.Sub(*prev) < poolSchemaDedupWindow {
			slog.Debug("pool threshold suppressed by schema dedup",
				"tenant_id", tenantID, "pool_id", poolID,
				"last_fired_at", prev, "window", poolSchemaDedupWindow)
			return nil
		}
	case errors.Is(err, gorm.ErrRecordNotFound):
		// No row — treat as never-fired. Continue.
	default:
		return fmt.Errorf("pool threshold dedup read: %w", err)
	}

	// Step 2: Redis SETNX race guard. Transient Redis failure does not
	// block publication — schema dedup is the safety net.
	if rdb != nil {
		key := poolDedupKey(tenantID, poolID)
		acquired, rerr := rdb.SetNXBool(ctx, key, poolDedupTTL)
		if rerr != nil {
			slog.Warn("pool threshold redis dedup failed; falling through to schema dedup",
				"tenant_id", tenantID, "pool_id", poolID, "key", key, "err", rerr)
		} else if !acquired {
			slog.Debug("pool threshold suppressed by redis dedup",
				"tenant_id", tenantID, "pool_id", poolID, "key", key)
			return nil
		}
	}

	// Step 3: durable schema mark FIRST (fail-closed). If this fails we
	// never publish — caller sees the error and the next call re-attempts.
	// Trades a rare false-suppress (mark succeeded, publish later failed
	// inside the dedup window) for the more severe alert-storm scenario
	// the audit closed.
	if err := db.MarkAlertFired(ctx, poolID, now); err != nil {
		return fmt.Errorf("pool threshold mark fired: %w", err)
	}

	// Step 4: publish to NATS. Use the canonical envelope shape used by
	// other LLM events for downstream notification consumers. If publish
	// fails, schema dedup has already locked the slot — caller may want to
	// replay manually, but we won't double-fire within the window.
	payload := PoolThresholdPayload{
		TenantID:       tenantID,
		PoolID:         poolID,
		CurrentBalance: currentBalance,
		MaxBalance:     maxBalance,
		ThresholdPct:   thresholdPct,
		FiredAt:        now,
	}
	if err := pub.Publish(ctx, SubjectPoolThreshold, payload); err != nil {
		return fmt.Errorf("pool threshold publish: %w", err)
	}

	slog.Info("pool threshold event published",
		"event_id", uuid.NewString(),
		"tenant_id", tenantID,
		"pool_id", poolID,
		"current_balance", currentBalance,
		"max_balance", maxBalance,
		"threshold_pct", thresholdPct)

	return nil
}

// poolDedupKey returns the canonical Redis key for pool-threshold dedup.
// Format: pool:threshold:<tenantID>:<poolID>.
func poolDedupKey(tenantID string, poolID int64) string {
	return fmt.Sprintf("pool:threshold:%s:%d", tenantID, poolID)
}

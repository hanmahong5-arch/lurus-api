package app

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/LurusTech/lurus-hub/internal/app/governance"
	"github.com/LurusTech/lurus-hub/internal/pkg/nats"
	hubnats "github.com/LurusTech/lurus-hub/internal/pkg/nats"
)

// redisClientDeduper wraps *redis.Client to implement redisDeduper.
type redisClientDeduper struct{ c *redis.Client }

func (r *redisClientDeduper) SetNXBool(ctx context.Context, key string, expiration time.Duration) (bool, error) {
	return r.c.SetNX(ctx, key, "1", expiration).Result()
}

// wrapRedis returns a redisDeduper for the given client, or nil if client is nil.
func wrapRedis(c *redis.Client) redisDeduper {
	if c == nil {
		return nil
	}
	return &redisClientDeduper{c: c}
}

// quotaThresholdsDefault is the canonical crossing ladder shipped today.
// Overridable via env LLM_QUOTA_THRESHOLDS — operators wanting the Anthropic
// Console-style 80/90/100 cadence (Phase E4 plan) or the LiteLLM-style
// 50/80/100 cadence set the env once at startup. Invalid values fall back to
// the default rather than firing on every consumption.
var quotaThresholdsDefault = []int{50, 80, 95, 100}

var (
	quotaThresholdsCache     []int
	quotaThresholdsCacheOnce sync.Once
)

// parsedQuotaThresholds reads LLM_QUOTA_THRESHOLDS once and returns the
// sorted, deduplicated set in (0,100]. Returns the default ladder if the env
// is empty or malformed. Single parse — values are cached for process lifetime
// so threshold lookups in the hot path stay branch-cheap.
func parsedQuotaThresholds() []int {
	quotaThresholdsCacheOnce.Do(func() {
		quotaThresholdsCache = loadQuotaThresholds(os.Getenv("LLM_QUOTA_THRESHOLDS"))
	})
	return quotaThresholdsCache
}

// loadQuotaThresholds is the pure parser, exposed for tests so they can probe
// edge cases without the sync.Once lock-in.
func loadQuotaThresholds(env string) []int {
	env = strings.TrimSpace(env)
	if env == "" {
		return append([]int(nil), quotaThresholdsDefault...)
	}
	parts := strings.Split(env, ",")
	out := make([]int, 0, len(parts))
	seen := make(map[int]struct{}, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		v, err := strconv.Atoi(p)
		if err != nil || v <= 0 || v > 100 {
			// One malformed entry voids the override — fall back to default
			// rather than silently dropping it, which would surprise operators.
			return append([]int(nil), quotaThresholdsDefault...)
		}
		if _, dup := seen[v]; dup {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	if len(out) == 0 {
		return append([]int(nil), quotaThresholdsDefault...)
	}
	// Ascending order so the crossing loop emits low → high deterministically
	// when a single transaction crosses multiple rungs.
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j] < out[i] {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}

// QuotaThresholdPayload matches the QuotaThresholdPayload consumed by
// lurus-platform's notification module (modules/notification/internal/pkg/event/types.go).
// Fields: account_id, used_tokens, limit_tokens, usage_percent (0-100).
type QuotaThresholdPayload struct {
	AccountID    int64   `json:"account_id"`
	UsedTokens   int64   `json:"used_tokens"`
	LimitTokens  int64   `json:"limit_tokens"`
	UsagePercent float64 `json:"usage_percent"`
}

// dedupKey returns the Redis SET NX key that prevents duplicate alerts.
// Format: quota_threshold_sent:user:{userId}:{threshold}:{period}
// period is YYYY-MM to bound the TTL naturally (24h TTL handles sub-month edge cases).
func dedupKey(userId int, threshold int, now time.Time) string {
	period := now.Format("2006-01")
	return fmt.Sprintf("quota_threshold_sent:user:%d:%d:%s", userId, threshold, period)
}

// redisDeduper is the minimal interface needed for deduplication.
// SetNXBool performs a Redis SET NX and returns (true, nil) when the key was set
// (first time), (false, nil) when the key already existed, and (_, err) on error.
type redisDeduper interface {
	SetNXBool(ctx context.Context, key string, expiration time.Duration) (bool, error)
}

// thresholdPublisher is the minimal publish interface.
type thresholdPublisher interface {
	Publish(ctx context.Context, subject string, payload any) error
}

// quotaThresholdParams carries all inputs needed for the threshold check.
// UsedTokensAfter and LimitTokens must be derived from a fresh DB fetch
// (called inside the async goroutine, so it doesn't block the relay path).
type quotaThresholdParams struct {
	// UserId is the local newhub user ID (for Redis dedup key).
	UserId int
	// IdentityAccountID is the lurus-platform account ID sent as account_id.
	IdentityAccountID int64
	// TenantID propagates to the audit trail. Phase E4: lets cross-tenant
	// audit queries filter quota threshold events. Empty defaults to "default"
	// inside NewDetachedAuditEvent.
	TenantID string
	// QuotaConsumed is how much was consumed in this transaction (positive).
	QuotaConsumed int64
	// UsedTokensAfter is the user's cumulative used_quota after this transaction.
	UsedTokensAfter int64
	// LimitTokens is the user's total quota ceiling (remaining + used_after).
	LimitTokens int64
}

// checkAndPublishQuotaThresholds fires NATS events for any thresholds crossed
// by the current consumption. It is designed to be called asynchronously and
// must not block or panic the caller.
//
// Crossing definition: before this transaction percentBefore < threshold,
// after percentAfter >= threshold.
//
// Dedup: Redis SET NX with 24h TTL ensures at-most-once per threshold per user per month.
// Fire-and-forget: publish errors are warned but not returned.
func checkAndPublishQuotaThresholds(
	ctx context.Context,
	params quotaThresholdParams,
	rdb redisDeduper,
	pub thresholdPublisher,
) {
	if !nats.Enabled() || pub == nil {
		return
	}
	if params.IdentityAccountID <= 0 || params.LimitTokens <= 0 {
		return
	}

	usedAfter := params.UsedTokensAfter
	usedBefore := usedAfter - params.QuotaConsumed
	if usedBefore < 0 {
		usedBefore = 0
	}

	percentAfter := float64(usedAfter) / float64(params.LimitTokens) * 100.0
	percentBefore := float64(usedBefore) / float64(params.LimitTokens) * 100.0

	now := time.Now()

	for _, threshold := range parsedQuotaThresholds() {
		ft := float64(threshold)
		// Crossing: before < threshold AND after >= threshold.
		if percentBefore >= ft || percentAfter < ft {
			continue
		}

		// Dedup via Redis SET NX.
		if rdb != nil {
			key := dedupKey(params.UserId, threshold, now)
			set, err := rdb.SetNXBool(ctx, key, 24*time.Hour)
			if err != nil {
				slog.Warn("quota threshold dedup failed",
					"key", key, "err", err)
				// On dedup failure, skip to avoid duplicate sends.
				continue
			}
			if !set {
				// Already sent for this threshold this period.
				continue
			}
		}

		// Audit FIRST, publish second. The dedup key has been claimed
		// (set=true above) so this crossing is committed; if NATS publish
		// later fails the audit trail still records that the user crossed
		// the threshold — operators can replay from audit_events. Reversing
		// the order would lose the crossing on publish failure.
		recordQuotaThresholdAudit(params, threshold, percentAfter)

		payload := QuotaThresholdPayload{
			AccountID:    params.IdentityAccountID,
			UsedTokens:   usedAfter,
			LimitTokens:  params.LimitTokens,
			UsagePercent: percentAfter,
		}

		if err := pub.Publish(ctx, hubnats.SubjectQuotaThreshold, payload); err != nil {
			slog.Warn("quota threshold publish failed",
				"account_id", params.IdentityAccountID,
				"threshold", threshold,
				"err", err)
		} else {
			slog.Info("quota threshold event published",
				"account_id", params.IdentityAccountID,
				"threshold", threshold,
				"usage_percent", percentAfter)
		}
	}
}

// recordQuotaThresholdAudit writes one audit_events row per threshold
// crossing. Details payload mirrors the NATS event so an operator triaging
// from either side sees the same numbers. Failure to record never blocks
// the publish path — RecordAuditEvent itself is async and fire-and-forget.
func recordQuotaThresholdAudit(params quotaThresholdParams, threshold int, percentAfter float64) {
	details, err := json.Marshal(map[string]any{
		"threshold_pct": threshold,
		"used_tokens":   params.UsedTokensAfter,
		"limit_tokens":  params.LimitTokens,
		"usage_percent": percentAfter,
		"account_id":    params.IdentityAccountID,
	})
	if err != nil {
		// Marshal failure on a fixed-shape map shouldn't happen — log and
		// emit an empty-details event rather than dropping the audit row.
		slog.Warn("quota threshold audit marshal failed", "err", err)
		details = []byte("{}")
	}
	governance.RecordAuditEvent(governance.NewDetachedAuditEvent(
		params.TenantID,
		governance.ActorSystem,
		params.UserId,
		governance.ActionBillingQuotaThreshold,
		governance.ResourceUser,
		params.UserId,
		string(details),
	))
}

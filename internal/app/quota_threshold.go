package app

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"

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

// quotaThresholds are the crossing points (percent) that trigger a NATS event.
var quotaThresholds = []int{50, 80, 95, 100}

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

	for _, threshold := range quotaThresholds {
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

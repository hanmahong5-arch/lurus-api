package app

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/LurusTech/lurus-hub/internal/pkg/nats"
	hubnats "github.com/LurusTech/lurus-hub/internal/pkg/nats"
)

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

// redisSetNXer is the minimal interface needed for deduplication.
// Matches *redis.Client.SetNX so production code can pass common.RDB directly.
type redisSetNXer interface {
	SetNX(ctx context.Context, key string, value interface{}, expiration time.Duration) interface {
		Result() (bool, error)
	}
}

// thresholdPublisher is the minimal publish interface.
type thresholdPublisher interface {
	Publish(ctx context.Context, subject string, payload any) error
}

// quotaThresholdParams carries all inputs needed for the threshold check.
// Separated from PostConsumeQuota so it can be tested without a real relay session.
type quotaThresholdParams struct {
	// UserId is the local newhub user ID (for Redis dedup key).
	UserId int
	// IdentityAccountID is the lurus-platform account ID sent as account_id.
	IdentityAccountID int64
	// QuotaBefore is the user's remaining quota before this consumption.
	QuotaBefore int64
	// QuotaConsumed is how much was consumed in this transaction.
	QuotaConsumed int64
	// UsedQuotaBefore is the user's historical used_quota before this transaction.
	UsedQuotaBefore int64
}

// checkAndPublishQuotaThresholds fires NATS events for any thresholds crossed
// by the current consumption. It is designed to be called asynchronously and
// must not block or panic the caller.
//
// Dedup: Redis SET NX with 24h TTL ensures at-most-once per threshold per user per month.
// Fire-and-forget: publish errors are warned but not returned.
func checkAndPublishQuotaThresholds(
	ctx context.Context,
	params quotaThresholdParams,
	rdb redisSetNXer,
	pub thresholdPublisher,
) {
	if !nats.Enabled() || pub == nil {
		return
	}
	if params.IdentityAccountID <= 0 {
		return
	}

	// Compute quota state.
	totalMax := params.QuotaBefore + params.UsedQuotaBefore + params.QuotaConsumed
	if totalMax <= 0 {
		return
	}
	usedAfter := params.UsedQuotaBefore + params.QuotaConsumed
	percentAfter := float64(usedAfter) / float64(totalMax) * 100.0
	percentBefore := float64(params.UsedQuotaBefore) / float64(totalMax) * 100.0

	now := time.Now()

	for _, threshold := range quotaThresholds {
		ft := float64(threshold)
		// Check crossing: before < threshold, after >= threshold.
		if percentBefore >= ft || percentAfter < ft {
			continue
		}

		// Dedup via Redis SET NX.
		if rdb != nil {
			key := dedupKey(params.UserId, threshold, now)
			set, err := rdb.SetNX(ctx, key, "1", 24*time.Hour).Result()
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
			LimitTokens:  totalMax,
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

// jsonRoundTrip is a helper for tests to verify payload serialisation.
func jsonRoundTrip(payload QuotaThresholdPayload) ([]byte, error) {
	return json.Marshal(payload)
}

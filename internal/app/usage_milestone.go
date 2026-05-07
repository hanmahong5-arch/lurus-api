package app

// Usage milestone tracker.
//
// Emits llm.usage.milestone events when a user's lifetime token total crosses
// one of a fixed ladder of thresholds (1k, 10k, 100k, 1M). Each threshold
// fires at most once per user, ever.
//
// Storage:
//   - Cumulative token total: Redis INCRBY on key "llm:tokens:<userID>".
//     This is independent of the user.used_quota DB column (which is in
//     billing units, not tokens). Loss of this counter at most causes a
//     single milestone to mis-fire — acceptable trade-off vs. adding a DB
//     column + migration.
//   - Last-fired threshold: Redis SETNX on key
//     "llm:milestone:<userID>:<threshold>". The SETNX result is the dedup
//     primitive — at most one publish per (user, threshold).
//
// If Redis is unavailable (common.RedisEnabled == false or RDB nil) this
// becomes a no-op rather than crashing the relay path.
//
// Ported from newapi service/usage_milestone.go (commit 9ef4e6db, 2026-04-30).

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/LurusTech/lurus-hub/internal/pkg/common"
	hubnats "github.com/LurusTech/lurus-hub/internal/pkg/nats"
)

type milestoneTier struct {
	threshold int64
	label     string
}

var usageMilestones = []milestoneTier{
	{1_000, "first_1k"},
	{10_000, "first_10k"},
	{100_000, "first_100k"},
	{1_000_000, "first_1m"},
}

// milestoneRedisTTL bounds dedup-key memory: one year covers any reasonable
// retention window while preventing unbounded growth.
const milestoneRedisTTL = 365 * 24 * time.Hour

// CheckAndPublishUsageMilestone updates the user's cumulative token counter
// and publishes one event per newly crossed milestone (typically zero or one
// per call). Safe to call from hot paths: returns immediately if Redis is
// unavailable.
//
// totalTokens is the token count for *this* request (prompt + completion);
// the function adds it to the cumulative counter atomically via INCRBY.
func CheckAndPublishUsageMilestone(ctx context.Context, userID int, totalTokens int) {
	if userID <= 0 || totalTokens <= 0 {
		return
	}
	if !common.RedisEnabled || common.RDB == nil {
		return
	}

	cumKey := fmt.Sprintf("llm:tokens:%d", userID)
	newTotal, err := common.RDB.IncrBy(ctx, cumKey, int64(totalTokens)).Result()
	if err != nil {
		common.SysLog("milestone: incr cumulative failed: " + err.Error())
		return
	}
	prevTotal := newTotal - int64(totalTokens)

	// Walk the ladder ascending; for each tier crossed by this request, claim
	// it. Iterating ascending means labels emit in natural order when a single
	// request crosses multiple tiers (rare but possible — fresh account with
	// 200k tokens in one shot).
	for _, m := range usageMilestones {
		if prevTotal >= m.threshold {
			continue
		}
		if newTotal < m.threshold {
			break
		}
		if claimMilestone(ctx, userID, m.threshold) {
			hubnats.PublishUsageMilestone(ctx, userID, newTotal, m.label, "lifetime")
		}
	}
}

// claimMilestone returns true iff this caller is the first to reach the given
// threshold for the user. Redis SETNX with TTL — race-safe across instances.
func claimMilestone(ctx context.Context, userID int, threshold int64) bool {
	key := fmt.Sprintf("llm:milestone:%d:%d", userID, threshold)
	ok, err := common.RDB.SetNX(ctx, key, "1", milestoneRedisTTL).Result()
	if err != nil {
		common.SysLog("milestone: setnx failed: " + err.Error())
		return false
	}
	return ok
}

// MilestoneThresholds exposes the configured ladder for tests.
func MilestoneThresholds() []int64 {
	out := make([]int64, len(usageMilestones))
	for i, m := range usageMilestones {
		out[i] = m.threshold
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// crossedMilestones returns the labels of every threshold whose value is in
// (prev, new]. Pure function — exposed for tests that don't want to spin up
// Redis.
func crossedMilestones(prev, newTotal int64) []string {
	var out []string
	for _, m := range usageMilestones {
		if prev >= m.threshold {
			continue
		}
		if newTotal < m.threshold {
			break
		}
		out = append(out, m.label)
	}
	return out
}

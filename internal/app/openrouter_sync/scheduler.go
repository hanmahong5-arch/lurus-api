package openrouter_sync

import (
	"context"
	"fmt"
	"time"

	"github.com/LurusTech/lurus-hub/internal/pkg/common"
)

// AutoSyncWithContext is the master-only ticker that drives the sync engine.
// frequencyMinutes <= 0 disables the loop; manual API triggers still work.
//
// Caller wires this into cmd/server/main.go alongside other periodic tasks.
func AutoSyncWithContext(ctx context.Context, frequencyMinutes int) {
	if !common.IsMasterNode {
		common.SysLog("openrouter sync: skipped on slave node")
		return
	}
	if frequencyMinutes <= 0 {
		common.SysLog("openrouter sync: disabled (frequency <= 0)")
		return
	}

	ticker := time.NewTicker(time.Duration(frequencyMinutes) * time.Minute)
	defer ticker.Stop()

	common.SysLog(fmt.Sprintf("openrouter sync: started, frequency=%d min", frequencyMinutes))

	engine := NewEngine()

	for {
		select {
		case <-ctx.Done():
			common.SysLog("openrouter sync: stopped")
			return
		case <-ticker.C:
			// HA: only the leader drives the sync engine. All master-capable
			// replicas run this loop; non-leaders idle until they win the lease.
			if !common.IsLeader() {
				continue
			}
			result, err := engine.Run(ctx, nil, false)
			if err != nil {
				common.SysLog("openrouter sync tick failed: " + err.Error())
				continue
			}
			if result.Skipped {
				continue
			}
			common.SysLog(fmt.Sprintf("openrouter sync tick: fetched=%d free=%d added=%d removed=%d breaker=%v",
				result.FetchedTotal, result.FreeTotal, len(result.Added), len(result.Removed), result.CircuitBreakerOn))
		}
	}
}

// AutoAggregateWithContext is the master-only hourly ticker that refreshes
// model_usage_stats from the logs table.
func AutoAggregateWithContext(ctx context.Context) {
	if !common.IsMasterNode {
		common.SysLog("openrouter aggregator: skipped on slave node")
		return
	}

	const interval = 1 * time.Hour
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	common.SysLog("openrouter aggregator: started, interval=1h")

	// Run once on startup so a fresh deploy doesn't wait an hour for first
	// stats — but only if this node already holds leadership at boot.
	if common.IsLeader() {
		if err := AggregateOpenRouterUsage(ctx); err != nil {
			common.SysLog("openrouter aggregator initial run failed: " + err.Error())
		}
	}

	for {
		select {
		case <-ctx.Done():
			common.SysLog("openrouter aggregator: stopped")
			return
		case <-ticker.C:
			// HA: only the leader aggregates; non-leaders idle.
			if !common.IsLeader() {
				continue
			}
			if err := AggregateOpenRouterUsage(ctx); err != nil {
				common.SysLog("openrouter aggregator failed: " + err.Error())
			}
		}
	}
}

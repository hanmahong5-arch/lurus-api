package lifecycle

import (
	"context"
	"fmt"
	"time"

	"github.com/LurusTech/lurus-hub/internal/adapter/repo"
	"github.com/LurusTech/lurus-hub/internal/pkg/common"
)

// auditCleanupBatchSize bounds the per-DELETE row count so a long-overdue
// cleanup doesn't lock the audit_events table in one giant transaction.
// 500 matches DeleteOldLog's batching convention.
const auditCleanupBatchSize = 500

// auditCleanupDefaultInterval is the wall-clock period between cleanup
// passes when AUDIT_CLEANUP_INTERVAL_SECONDS is unset. Daily is enough —
// retention deadlines are at-second precision but operators set them in
// years, so sub-day enforcement granularity carries no business value.
const auditCleanupDefaultInterval = 24 * time.Hour

// StartAuditCleanupWithContext launches the daily audit retention sweep.
// On each tick it deletes audit_events rows whose retention_until has
// passed; rows with retention_until = 0 are preserved (treated as
// "no expiry"). The goroutine exits when ctx is cancelled.
//
// Phase E3: this enforces the per-row 7y default that
// governance.NewAuditEvent assigns, plus any shorter or longer
// retention_until callers override after construction.
func StartAuditCleanupWithContext(ctx context.Context) {
	interval := auditCleanupDefaultInterval
	common.SysLog(fmt.Sprintf("audit retention cleanup started, interval=%s", interval))

	ticker := time.NewTicker(interval)
	common.SafeGoWithContext(ctx, func(c context.Context) {
		defer ticker.Stop()
		// Run once on startup so a fresh deploy reclaims any backlog
		// without waiting a full day.
		runAuditCleanup(c)
		for {
			select {
			case <-c.Done():
				common.SysLog("audit retention cleanup stopped")
				return
			case <-ticker.C:
				runAuditCleanup(c)
			}
		}
	})
}

// runAuditCleanup executes one pass. Errors are logged but do not stop
// the ticker — a transient DB hiccup should not silently disable
// retention enforcement.
func runAuditCleanup(ctx context.Context) {
	now := common.GetTimestamp()
	deleted, err := repo.DeleteExpiredAuditEvents(ctx, now, auditCleanupBatchSize)
	if err != nil {
		common.SysError(fmt.Sprintf("audit cleanup: delete expired events: %v", err))
		return
	}
	if deleted > 0 {
		common.SysLog(fmt.Sprintf("audit cleanup: deleted %d expired events", deleted))
	}
}

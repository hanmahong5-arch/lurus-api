package governance

import (
	"sync/atomic"

	"github.com/LurusTech/lurus-hub/internal/domain/entity"
	"github.com/LurusTech/lurus-hub/internal/pkg/common"

	"github.com/bytedance/gopkg/util/gopool"
	"github.com/gin-gonic/gin"
)

// Audit action constants.
const (
	ActionTokenCreated      = "token.created"
	ActionTokenUpdated      = "token.updated"
	ActionTokenDeleted      = "token.deleted"
	ActionTokenBatchDeleted = "token.batch_deleted"

	ActionChannelUpdated      = "channel.updated"
	ActionChannelDeleted      = "channel.deleted"
	ActionChannelBatchDeleted = "channel.batch_deleted"
	ActionChannelDisabled     = "channel.disabled"
	ActionChannelEnabled      = "channel.enabled"
	ActionChannelTagDisabled  = "channel.tag_disabled"

	ActionAuthFailed        = "auth.failed"
	ActionAuthIPRejected    = "auth.ip_rejected"
	ActionAuthScopeRejected = "auth.scope_rejected" // Phase E2: token lacks scope required for the relay path
	ActionAuthBootstrapped  = "auth.bootstrapped"

	ActionSensitiveBlocked = "security.sensitive_blocked"
)

// Actor type constants.
const (
	ActorUser   = "user"
	ActorAdmin  = "admin"
	ActorSystem = "system"
	ActorToken  = "token"
)

// Resource type constants.
const (
	ResourceToken   = "token"
	ResourceChannel = "channel"
	ResourceUser    = "user"
)

// AuditWriter is the interface for persisting audit events.
// Set via SetAuditWriter during initialization to avoid circular imports.
type AuditWriter interface {
	CreateAuditEvent(event *entity.AuditEvent) error
}

// auditWriterRef stores the global audit writer atomically for safe concurrent access.
var auditWriterRef atomic.Pointer[AuditWriter]

// SetAuditWriter sets the global audit event writer (called once during startup).
func SetAuditWriter(w AuditWriter) {
	auditWriterRef.Store(&w)
}

// RecordAuditEvent asynchronously persists an audit event.
// Safe to call even if no writer is configured (no-op).
func RecordAuditEvent(event *entity.AuditEvent) {
	wp := auditWriterRef.Load()
	if wp == nil || event == nil {
		return
	}
	writer := *wp
	gopool.Go(func() {
		if err := writer.CreateAuditEvent(event); err != nil {
			common.SysLog("failed to record audit event: " + err.Error())
		}
	})
}

// NewAuditEvent creates an AuditEvent from gin context with common fields pre-filled.
func NewAuditEvent(c *gin.Context, actorType string, actorID int, action, resource string, resourceID int, details string) *entity.AuditEvent {
	event := &entity.AuditEvent{
		TenantID:  c.GetString("tenant_id"),
		Timestamp: common.GetTimestamp(),
		ActorType: actorType,
		ActorID:   actorID,
		Action:    action,
		Resource:  resource,
		ResourceID: resourceID,
		Details:   details,
		IP:        c.ClientIP(),
		RequestID: c.GetString(common.RequestIdKey),
	}
	if event.TenantID == "" {
		event.TenantID = "default"
	}
	return event
}

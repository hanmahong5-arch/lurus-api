package entity

// AuditEvent records security-relevant actions for governance audit trails.
type AuditEvent struct {
	ID         int64  `json:"id" gorm:"primaryKey;autoIncrement"`
	TenantID   string `json:"tenant_id" gorm:"type:varchar(36);index;default:'default'"`
	Timestamp  int64  `json:"timestamp" gorm:"bigint;index"`
	ActorType  string `json:"actor_type" gorm:"type:varchar(16)"` // user, admin, system, token
	ActorID    int    `json:"actor_id" gorm:"index"`
	Action     string `json:"action" gorm:"type:varchar(64);index"` // token.created, auth.failed, ...
	Resource   string `json:"resource" gorm:"type:varchar(64)"`     // token, channel, user, setting
	ResourceID int    `json:"resource_id"`
	Details    string `json:"details" gorm:"type:text"` // JSON
	IP         string `json:"ip" gorm:"type:varchar(45);default:''"`
	RequestID  string `json:"request_id" gorm:"type:varchar(36);default:''"`
	// RetentionUntil is the unix-seconds deadline after which the
	// audit_cleanup lifecycle task will delete this row. Zero means
	// "no expiry" — legacy rows backfilled without a policy stay forever
	// until an explicit backfill assigns a value. See migration 016 and
	// internal/lifecycle/audit_cleanup.go.
	RetentionUntil int64 `json:"retention_until" gorm:"bigint;default:0;index:idx_audit_events_retention_until,where:retention_until > 0"`
}

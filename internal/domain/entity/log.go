package entity

// Log represents a log entry (consume, error, system, etc.)
type Log struct {
	Id               int    `json:"id" gorm:"index:idx_created_at_id,priority:1"`
	UserId           int    `json:"user_id" gorm:"index;index:idx_tenant_user_created,priority:2"`
	TenantId         string `json:"tenant_id" gorm:"type:varchar(36);index;index:idx_tenant_user_created,priority:1;default:'default'"` // Tenant isolation
	CreatedAt        int64  `json:"created_at" gorm:"bigint;index:idx_created_at_id,priority:2;index:idx_created_at_type;index:idx_tenant_user_created,priority:3"`
	Type             int    `json:"type" gorm:"index:idx_created_at_type"`
	Content          string `json:"content"`
	Username         string `json:"username" gorm:"index;index:index_username_model_name,priority:2;default:''"`
	TokenName        string `json:"token_name" gorm:"index;default:''"`
	ModelName        string `json:"model_name" gorm:"index;index:index_username_model_name,priority:1;default:''"`
	Quota            int    `json:"quota" gorm:"default:0"`
	PromptTokens     int    `json:"prompt_tokens" gorm:"default:0"`
	CompletionTokens int    `json:"completion_tokens" gorm:"default:0"`
	UseTime          int    `json:"use_time" gorm:"default:0"`
	IsStream         bool   `json:"is_stream"`
	ChannelId        int    `json:"channel" gorm:"index"`
	ChannelName      string `json:"channel_name" gorm:"->"`
	TokenId          int    `json:"token_id" gorm:"default:0;index"`
	Group            string `json:"group" gorm:"index"`
	Ip               string `json:"ip" gorm:"index;default:''"`
	Other              string `json:"other"`
	ChannelType        int    `json:"channel_type" gorm:"default:0;index:idx_gov_channel_type"`
	RelayMode          int    `json:"relay_mode" gorm:"default:0;index:idx_gov_relay_mode"`
	RequestFingerprint string `json:"request_fingerprint" gorm:"type:varchar(16);default:'';index:idx_gov_fingerprint"`
	UpstreamModel      string `json:"upstream_model" gorm:"type:varchar(128);default:''"`
	TotalLatencyMs     int    `json:"total_latency_ms" gorm:"default:0"`
}

// don't use iota, avoid change log type value
const (
	LogTypeUnknown = 0
	LogTypeTopup   = 1
	LogTypeConsume = 2
	LogTypeManage  = 3
	LogTypeSystem  = 4
	LogTypeError   = 5
	LogTypeRefund  = 6
)

type RecordConsumeLogParams struct {
	ChannelId        int                    `json:"channel_id"`
	PromptTokens     int                    `json:"prompt_tokens"`
	CompletionTokens int                    `json:"completion_tokens"`
	ModelName        string                 `json:"model_name"`
	TokenName        string                 `json:"token_name"`
	Quota            int                    `json:"quota"`
	Content          string                 `json:"content"`
	TokenId          int                    `json:"token_id"`
	UseTimeSeconds   int                    `json:"use_time_seconds"`
	IsStream         bool                   `json:"is_stream"`
	Group            string                 `json:"group"`
	Other              map[string]interface{} `json:"other"`
	ChannelType        int                    `json:"channel_type"`
	RelayMode          int                    `json:"relay_mode"`
	RequestFingerprint string                 `json:"request_fingerprint"`
	UpstreamModel      string                 `json:"upstream_model"`
	TotalLatencyMs     int                    `json:"total_latency_ms"`
	LogDetailLevel     string                 `json:"-"` // Governance: "none" skips logging, "full" adds prompt preview
}

// LogQueryParams contains parameters for log queries
type LogQueryParams struct {
	UserID    int    // Filter by user ID (0 for all users)
	TenantID  string // Filter by tenant ID (required for tenant isolation)
	LogType   int    // Filter by log type (0 for all types)
	ModelName string // Filter by model name
	StartTime int64  // Filter logs after this timestamp
	EndTime   int64  // Filter logs before this timestamp
	TokenName string // Filter by token name
	Username  string // Filter by username
	AfterID   int    // Cursor: when > 0, return only rows with id strictly greater (live-tail)
	Offset    int    // Pagination offset
	Limit     int    // Pagination limit
}

type Stat struct {
	Quota int `json:"quota"`
	Rpm   int `json:"rpm"`
	Tpm   int `json:"tpm"`
}

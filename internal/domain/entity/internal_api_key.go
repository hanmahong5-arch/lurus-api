package entity

import "encoding/json"

// InternalApiKey represents an API key for internal services
type InternalApiKey struct {
	Id          int    `json:"id" gorm:"primaryKey"`
	Name        string `json:"name" gorm:"size:100;not null"`
	KeyHash     string `json:"-" gorm:"size:64;uniqueIndex"`
	KeyPrefix   string `json:"key_prefix" gorm:"size:16"`
	Scopes      string `json:"scopes" gorm:"type:text"`
	CreatedBy   int    `json:"created_by"`
	CreatedAt   int64  `json:"created_at" gorm:"autoCreateTime"`
	LastUsedAt  int64  `json:"last_used_at"`
	ExpiresAt   int64  `json:"expires_at"`
	Enabled     bool   `json:"enabled" gorm:"default:true"`
	Description string `json:"description" gorm:"size:500"`
}

func (k InternalApiKey) TableName() string {
	return "internal_api_keys"
}

// Scopes definition
const (
	ScopeUserRead          = "user:read"
	ScopeUserWrite         = "user:write"
	ScopeUserDelete        = "user:delete"
	ScopeSubscriptionRead  = "subscription:read"
	ScopeSubscriptionWrite = "subscription:write"
	ScopeQuotaRead         = "quota:read"
	ScopeQuotaWrite        = "quota:write"
	ScopeBalanceRead       = "balance:read"
	ScopeBalanceWrite      = "balance:write"
	ScopeTokenRead         = "token:read"
	ScopeTokenWrite        = "token:write"
	ScopeCurrencyRead      = "currency:read"
	ScopeCurrencyExchange  = "currency:exchange"
	ScopeAuthLogin         = "auth:login"
	ScopeLogRead           = "log:read"
	ScopeModelRead         = "model:read"
	// ScopeProvisioning gates the Provisioning API (POST/DELETE under
	// /internal/v1/provisioning/tenants/:slug/keys) that Resellers use to
	// programmatically issue or revoke sub-tenant token keys. See
	// ADR 2026-05-18 (tenant-credit-pool) §4.2 and §7 risk #5.
	ScopeProvisioning = "provisioning"
	ScopeAll          = "*"
)

// GetScopes returns the list of scopes for this key (JSON array format)
func (k *InternalApiKey) GetScopes() []string {
	if k.Scopes == "" {
		return []string{}
	}
	var scopes []string
	if err := json.Unmarshal([]byte(k.Scopes), &scopes); err != nil {
		return []string{}
	}
	return scopes
}

// HasScope checks if the key has a specific scope
func (k *InternalApiKey) HasScope(scope string) bool {
	scopes := k.GetScopes()
	for _, s := range scopes {
		if s == ScopeAll || s == scope {
			return true
		}
	}
	return false
}

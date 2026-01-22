package model

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"

	"github.com/QuantumNous/lurus-api/common"
)

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
	ExpiresAt   int64  `json:"expires_at"` // 0 = never expires
	Enabled     bool   `json:"enabled" gorm:"default:true"`
	Description string `json:"description" gorm:"size:500"`
}

func (InternalApiKey) TableName() string {
	return "internal_api_keys"
}

// Scopes definition
const (
	ScopeUserRead          = "user:read"
	ScopeUserWrite         = "user:write"
	ScopeSubscriptionRead  = "subscription:read"
	ScopeSubscriptionWrite = "subscription:write"
	ScopeQuotaRead         = "quota:read"
	ScopeQuotaWrite        = "quota:write"
	ScopeBalanceRead       = "balance:read"
	ScopeBalanceWrite      = "balance:write"
	ScopeAll               = "*" // Superuser scope
)

// GetScopes returns the scopes as a string slice
func (k *InternalApiKey) GetScopes() []string {
	if k.Scopes == "" {
		return []string{}
	}
	var scopes []string
	json.Unmarshal([]byte(k.Scopes), &scopes)
	return scopes
}

// HasScope checks if the key has a specific scope
func (k *InternalApiKey) HasScope(scope string) bool {
	scopes := k.GetScopes()
	for _, s := range scopes {
		if s == scope || s == ScopeAll {
			return true
		}
	}
	return false
}

// hashKey creates SHA256 hash of the API key
func hashKey(key string) string {
	hash := sha256.Sum256([]byte(key))
	return hex.EncodeToString(hash[:])
}

// CreateInternalApiKey generates a new API key
func CreateInternalApiKey(name string, scopes []string, createdBy int, expiresAt int64, description string) (string, *InternalApiKey, error) {
	// Generate random key: lurus_ik_xxxxxxxxxxxxxxxxxxxx
	key := "lurus_ik_" + common.GetRandomString(32)
	keyHash := hashKey(key)
	keyPrefix := key[:16] // First 16 chars for display

	scopesJson, err := json.Marshal(scopes)
	if err != nil {
		return "", nil, err
	}

	apiKey := &InternalApiKey{
		Name:        name,
		KeyHash:     keyHash,
		KeyPrefix:   keyPrefix,
		Scopes:      string(scopesJson),
		CreatedBy:   createdBy,
		ExpiresAt:   expiresAt,
		Enabled:     true,
		Description: description,
	}

	err = DB.Create(apiKey).Error
	if err != nil {
		return "", nil, err
	}

	return key, apiKey, nil
}

// ValidateInternalApiKey validates key and returns the key object
func ValidateInternalApiKey(key string) (*InternalApiKey, error) {
	keyHash := hashKey(key)

	var apiKey InternalApiKey
	err := DB.Where("key_hash = ? AND enabled = ?", keyHash, true).First(&apiKey).Error
	if err != nil {
		return nil, err
	}

	// Check expiration
	if apiKey.ExpiresAt > 0 && apiKey.ExpiresAt < common.GetTimestamp() {
		return nil, errors.New("API key expired")
	}

	// Update last used (non-blocking)
	go func() {
		DB.Model(&apiKey).Update("last_used_at", common.GetTimestamp())
	}()

	return &apiKey, nil
}

// GetAllInternalApiKeys returns all API keys
func GetAllInternalApiKeys() ([]*InternalApiKey, error) {
	var keys []*InternalApiKey
	err := DB.Order("id desc").Find(&keys).Error
	return keys, err
}

// GetInternalApiKeyById returns an API key by ID
func GetInternalApiKeyById(id int) (*InternalApiKey, error) {
	var key InternalApiKey
	err := DB.First(&key, id).Error
	return &key, err
}

// DeleteInternalApiKey deletes an API key
func DeleteInternalApiKey(id int) error {
	return DB.Delete(&InternalApiKey{}, id).Error
}

// ToggleInternalApiKey enables/disables an API key
func ToggleInternalApiKey(id int) error {
	var key InternalApiKey
	err := DB.First(&key, id).Error
	if err != nil {
		return err
	}
	return DB.Model(&key).Update("enabled", !key.Enabled).Error
}

// UpdateInternalApiKey updates an API key
func UpdateInternalApiKey(id int, name string, scopes []string, expiresAt int64, description string) error {
	scopesJson, err := json.Marshal(scopes)
	if err != nil {
		return err
	}

	return DB.Model(&InternalApiKey{}).Where("id = ?", id).Updates(map[string]interface{}{
		"name":        name,
		"scopes":      string(scopesJson),
		"expires_at":  expiresAt,
		"description": description,
	}).Error
}

// GetAvailableScopes returns all available scopes for UI
func GetAvailableScopes() []map[string]string {
	return []map[string]string{
		{"key": ScopeUserRead, "name": "Read User Info", "description": "Get user information by ID, email, or phone"},
		{"key": ScopeUserWrite, "name": "Write User Info", "description": "Update user information"},
		{"key": ScopeSubscriptionRead, "name": "Read Subscription", "description": "Get user subscription status"},
		{"key": ScopeSubscriptionWrite, "name": "Write Subscription", "description": "Grant or modify subscriptions"},
		{"key": ScopeQuotaRead, "name": "Read Quota", "description": "Get user quota information"},
		{"key": ScopeQuotaWrite, "name": "Write Quota", "description": "Adjust user quota"},
		{"key": ScopeBalanceRead, "name": "Read Balance", "description": "Get user balance"},
		{"key": ScopeBalanceWrite, "name": "Write Balance", "description": "Top up user balance"},
		{"key": ScopeAll, "name": "All Permissions", "description": "Full access to all internal APIs"},
	}
}

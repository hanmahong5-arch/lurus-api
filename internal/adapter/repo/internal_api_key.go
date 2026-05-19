package repo

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"

	entity "github.com/LurusTech/lurus-hub/internal/domain/entity"
	"github.com/LurusTech/lurus-hub/internal/pkg/common"
)

type InternalApiKey = entity.InternalApiKey

// Re-export scope constants for existing callers
const (
	ScopeUserRead          = entity.ScopeUserRead
	ScopeUserWrite         = entity.ScopeUserWrite
	ScopeUserDelete        = entity.ScopeUserDelete
	ScopeSubscriptionRead  = entity.ScopeSubscriptionRead
	ScopeSubscriptionWrite = entity.ScopeSubscriptionWrite
	ScopeQuotaRead         = entity.ScopeQuotaRead
	ScopeQuotaWrite        = entity.ScopeQuotaWrite
	ScopeBalanceRead       = entity.ScopeBalanceRead
	ScopeBalanceWrite      = entity.ScopeBalanceWrite
	ScopeTokenRead         = entity.ScopeTokenRead
	ScopeTokenWrite        = entity.ScopeTokenWrite
	ScopeCurrencyRead      = entity.ScopeCurrencyRead
	ScopeCurrencyExchange  = entity.ScopeCurrencyExchange
	ScopeAuthLogin         = entity.ScopeAuthLogin
	ScopeLogRead           = entity.ScopeLogRead
	ScopeModelRead         = entity.ScopeModelRead
	ScopeProvisioning      = entity.ScopeProvisioning
	ScopeAll               = entity.ScopeAll
)

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
	// Capture db reference to avoid nil dereference if DB is reassigned during tests
	db := DB
	go func() {
		if db != nil {
			db.Model(&apiKey).Update("last_used_at", common.GetTimestamp())
		}
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
		{"key": ScopeUserDelete, "name": "Delete User", "description": "Delete user accounts"},
		{"key": ScopeSubscriptionRead, "name": "Read Subscription", "description": "Get user subscription status"},
		{"key": ScopeSubscriptionWrite, "name": "Write Subscription", "description": "Grant or modify subscriptions"},
		{"key": ScopeQuotaRead, "name": "Read Quota", "description": "Get user quota information"},
		{"key": ScopeQuotaWrite, "name": "Write Quota", "description": "Adjust user quota"},
		{"key": ScopeBalanceRead, "name": "Read Balance", "description": "Get user balance"},
		{"key": ScopeBalanceWrite, "name": "Write Balance", "description": "Top up user balance"},
		{"key": ScopeTokenRead, "name": "Read Token", "description": "Get user tokens"},
		{"key": ScopeTokenWrite, "name": "Write Token", "description": "Create user tokens"},
		{"key": ScopeCurrencyRead, "name": "Read Currency", "description": "View exchange rates and model pricing in Lute"},
		{"key": ScopeCurrencyExchange, "name": "Currency Exchange", "description": "Exchange LuCoin to Lute for users"},
		{"key": ScopeLogRead, "name": "Read Logs", "description": "Query usage logs by user or token"},
		{"key": ScopeModelRead, "name": "Read Models", "description": "View model catalog and pricing"},
		{"key": ScopeAuthLogin, "name": "Auth Login", "description": "Authenticate users via login"},
		{"key": ScopeProvisioning, "name": "Provisioning", "description": "Reseller sub-tenant key issuance / revocation"},
		{"key": ScopeAll, "name": "All Permissions", "description": "Full access to all internal APIs"},
	}
}

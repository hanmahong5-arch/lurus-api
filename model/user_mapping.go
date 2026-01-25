package model

import (
	"errors"
	"time"

	"gorm.io/gorm"
)

// UserIdentityMapping maps Zitadel users to lurus users in multi-tenant context
// Allows a single Zitadel user to have different lurus user accounts across tenants
type UserIdentityMapping struct {
	Id              int            `json:"id" gorm:"primaryKey;autoIncrement"`                       // Auto-increment ID
	LurusUserID     int            `json:"lurus_user_id" gorm:"column:lurus_user_id;not null;index"` // Reference to users.id
	ZitadelUserID   string         `json:"zitadel_user_id" gorm:"column:zitadel_user_id;size:128;not null;index"` // Zitadel User ID (from JWT "sub" claim)
	TenantID        string         `json:"tenant_id" gorm:"column:tenant_id;size:36;not null;index"` // Reference to tenants.id

	// User metadata synced from Zitadel
	Email            string         `json:"email" gorm:"size:255;index"`                              // User email (synced from Zitadel)
	DisplayName      string         `json:"display_name" gorm:"column:display_name;size:128"`         // Display name (synced from Zitadel)
	PreferredUsername string        `json:"preferred_username" gorm:"column:preferred_username;size:128"` // Preferred username (synced from Zitadel)

	// Sync metadata
	LastSyncAt       *time.Time     `json:"last_sync_at" gorm:"column:last_sync_at"`                  // Last sync timestamp from Zitadel
	IsActive         bool           `json:"is_active" gorm:"default:true;index"`                      // Whether this mapping is active

	// Timestamps
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
	DeletedAt        gorm.DeletedAt `json:"deleted_at,omitempty" gorm:"index"`
}

// TableName specifies the table name for UserIdentityMapping model
func (UserIdentityMapping) TableName() string {
	return "user_identity_mapping"
}

// GetUserMappingByZitadelID retrieves user mapping by Zitadel User ID and Tenant ID
func GetUserMappingByZitadelID(zitadelUserID string, tenantID string) (*UserIdentityMapping, error) {
	var mapping UserIdentityMapping
	err := DB.Where("zitadel_user_id = ? AND tenant_id = ? AND is_active = ?", zitadelUserID, tenantID, true).
		First(&mapping).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("user mapping not found")
		}
		return nil, err
	}
	return &mapping, nil
}

// GetUserMappingByLurusUserID retrieves user mapping by lurus user ID and tenant ID
func GetUserMappingByLurusUserID(lurusUserID int, tenantID string) (*UserIdentityMapping, error) {
	var mapping UserIdentityMapping
	err := DB.Where("lurus_user_id = ? AND tenant_id = ? AND is_active = ?", lurusUserID, tenantID, true).
		First(&mapping).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("user mapping not found")
		}
		return nil, err
	}
	return &mapping, nil
}

// CreateUserMapping creates a new user identity mapping
func CreateUserMapping(lurusUserID int, zitadelUserID string, tenantID string, email string, displayName string, preferredUsername string) (*UserIdentityMapping, error) {
	// Check if mapping already exists
	existingMapping, _ := GetUserMappingByZitadelID(zitadelUserID, tenantID)
	if existingMapping != nil {
		// Update last sync time
		now := time.Now()
		existingMapping.LastSyncAt = &now
		existingMapping.Email = email
		existingMapping.DisplayName = displayName
		existingMapping.PreferredUsername = preferredUsername
		existingMapping.UpdatedAt = now
		err := DB.Save(existingMapping).Error
		return existingMapping, err
	}

	now := time.Now()
	mapping := &UserIdentityMapping{
		LurusUserID:       lurusUserID,
		ZitadelUserID:     zitadelUserID,
		TenantID:          tenantID,
		Email:             email,
		DisplayName:       displayName,
		PreferredUsername: preferredUsername,
		LastSyncAt:        &now,
		IsActive:          true,
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	err := DB.Create(mapping).Error
	if err != nil {
		return nil, err
	}

	return mapping, nil
}

// UpdateUserMapping updates user mapping metadata
func UpdateUserMapping(id int, updates map[string]interface{}) error {
	updates["updated_at"] = time.Now()
	return DB.Model(&UserIdentityMapping{}).Where("id = ?", id).Updates(updates).Error
}

// DeactivateUserMapping deactivates a user mapping (soft delete)
func DeactivateUserMapping(id int) error {
	return UpdateUserMapping(id, map[string]interface{}{
		"is_active": false,
	})
}

// DeleteUserMapping hard deletes a user mapping
func DeleteUserMapping(id int) error {
	return DB.Delete(&UserIdentityMapping{}, "id = ?", id).Error
}

// ListUserMappingsByTenant retrieves all user mappings for a tenant
func ListUserMappingsByTenant(tenantID string, offset int, limit int) ([]*UserIdentityMapping, int64, error) {
	var mappings []*UserIdentityMapping
	var total int64

	query := DB.Model(&UserIdentityMapping{}).Where("tenant_id = ? AND is_active = ?", tenantID, true)

	// Get total count
	err := query.Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	// Get paginated results
	err = query.Offset(offset).Limit(limit).Order("created_at DESC").Find(&mappings).Error
	if err != nil {
		return nil, 0, err
	}

	return mappings, total, nil
}

// ListUserMappingsByZitadelUser retrieves all mappings for a Zitadel user across tenants
func ListUserMappingsByZitadelUser(zitadelUserID string) ([]*UserIdentityMapping, error) {
	var mappings []*UserIdentityMapping
	err := DB.Where("zitadel_user_id = ? AND is_active = ?", zitadelUserID, true).
		Order("created_at DESC").
		Find(&mappings).Error
	return mappings, err
}

// SyncUserDataFromZitadel syncs user data from Zitadel claims to mapping
func SyncUserDataFromZitadel(mappingID int, email string, displayName string, preferredUsername string) error {
	now := time.Now()
	return UpdateUserMapping(mappingID, map[string]interface{}{
		"email":              email,
		"display_name":       displayName,
		"preferred_username": preferredUsername,
		"last_sync_at":       &now,
	})
}

// GetUserByZitadelID retrieves lurus user by Zitadel user ID and tenant
// This is a helper function that combines mapping lookup and user retrieval
func GetUserByZitadelID(zitadelUserID string, tenantID string) (*User, *UserIdentityMapping, error) {
	// Get mapping
	mapping, err := GetUserMappingByZitadelID(zitadelUserID, tenantID)
	if err != nil {
		return nil, nil, err
	}

	// Get user
	user, err := GetUserById(mapping.LurusUserID, false)
	if err != nil {
		return nil, nil, err
	}

	return user, mapping, nil
}

// CreateUserFromZitadelClaims creates a new lurus user from Zitadel JWT claims
// and establishes the identity mapping
type ZitadelUserClaims struct {
	Sub               string // Zitadel User ID (from "sub" claim)
	Email             string // User email
	EmailVerified     bool   // Email verification status
	Name              string // Full name
	PreferredUsername string // Preferred username
	OrgID             string // Zitadel Organization ID
	OrgDomain         string // Organization domain
}

func CreateUserFromZitadelClaims(claims *ZitadelUserClaims, tenantID string) (*User, *UserIdentityMapping, error) {
	// Check if mapping already exists
	existingMapping, _ := GetUserMappingByZitadelID(claims.Sub, tenantID)
	if existingMapping != nil {
		// User already exists, retrieve and return
		user, err := GetUserById(existingMapping.LurusUserID, false)
		if err != nil {
			return nil, nil, err
		}
		return user, existingMapping, nil
	}

	// Get tenant config for default quota
	tenant, err := GetTenantByID(tenantID)
	if err != nil {
		return nil, nil, err
	}

	// Check if tenant can add more users
	canAdd, err := tenant.CanAddUser()
	if err != nil {
		return nil, nil, err
	}
	if !canAdd {
		return nil, nil, errors.New("tenant has reached maximum user limit")
	}

	// Generate unique username (handle duplicates)
	username := claims.PreferredUsername
	if username == "" {
		username = claims.Email
	}
	username = ensureUniqueUsername(username, tenantID)

	// Get default user quota from tenant config
	defaultQuota := GetTenantConfigInt(tenantID, "quota.new_user_quota", 10000)

	// Create new lurus user
	user := &User{
		Username:    username,
		Email:       claims.Email,
		DisplayName: claims.Name,
		Role:        1, // RoleCommonUser
		Status:      1, // UserStatusEnabled
		Quota:       defaultQuota,
		UsedQuota:   0,
		Group:       "default",
		AffCode:     generateAffCode(),
		// TenantID will be set automatically by GORM plugin in context
	}

	// Note: Password is not set for Zitadel users (they authenticate via Zitadel)
	// If password is required, generate a random strong password
	user.Password = GenerateRandomPassword()

	err = DB.Create(user).Error
	if err != nil {
		return nil, nil, err
	}

	// Create identity mapping
	mapping, err := CreateUserMapping(
		user.Id,
		claims.Sub,
		tenantID,
		claims.Email,
		claims.Name,
		claims.PreferredUsername,
	)
	if err != nil {
		// Rollback user creation if mapping fails
		DB.Delete(user)
		return nil, nil, err
	}

	return user, mapping, nil
}

// ensureUniqueUsername ensures username is unique within tenant
func ensureUniqueUsername(baseUsername string, tenantID string) string {
	username := baseUsername
	suffix := 1

	for {
		var count int64
		DB.Model(&User{}).Where("username = ? AND tenant_id = ?", username, tenantID).Count(&count)
		if count == 0 {
			return username
		}
		username = baseUsername + "_" + string(rune(suffix))
		suffix++
	}
}

// GenerateRandomPassword generates a random strong password for Zitadel users
// Since they authenticate via Zitadel, this password won't be used for login
func GenerateRandomPassword() string {
	// TODO: Implement secure random password generation
	// For now, return a placeholder
	// In production, use crypto/rand
	return "ZITADEL_AUTH_USER_" + time.Now().Format("20060102150405")
}

// GetTenantConfigInt retrieves integer config value from tenant_configs
// This is a placeholder - implement actual function in tenant_config.go
func GetTenantConfigInt(tenantID string, key string, defaultValue int) int {
	// TODO: Implement actual tenant config retrieval
	// For now, return default value
	return defaultValue
}

// generateAffCode generates a unique affiliate code
// This is a placeholder - use existing implementation from user.go
func generateAffCode() string {
	// TODO: Use existing generateAffCode() function from user.go
	return "AFF-" + time.Now().Format("20060102150405")
}

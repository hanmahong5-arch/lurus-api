package model

import (
	"errors"
	"time"

	"gorm.io/gorm"
)

// Tenant represents a multi-tenant SaaS tenant
// Maps to Zitadel Organization
type Tenant struct {
	Id            string         `json:"id" gorm:"primaryKey;size:36"`                                  // UUID (can be custom or match Zitadel Org ID)
	ZitadelOrgID  string         `json:"zitadel_org_id" gorm:"column:zitadel_org_id;size:128;unique;not null;index"` // Zitadel Organization ID
	Slug          string         `json:"slug" gorm:"size:64;unique;not null;index"`                     // URL-friendly identifier (e.g., lurus, customer-a)
	Name          string         `json:"name" gorm:"size:128;not null"`                                 // Display name
	Status        int            `json:"status" gorm:"type:int;default:1;index"`                        // 1=enabled, 2=disabled, 3=suspended

	// Business configuration
	PlanType      string         `json:"plan_type" gorm:"size:32;default:'free';index"`                 // free/pro/enterprise
	MaxUsers      int            `json:"max_users" gorm:"type:int;default:100"`                         // Maximum users allowed
	MaxQuota      int64          `json:"max_quota" gorm:"type:bigint;default:1000000"`                  // Maximum total quota (in tokens)

	// Metadata
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `json:"deleted_at,omitempty" gorm:"index"`
}

// Tenant status constants
const (
	TenantStatusEnabled   = 1  // Tenant is active and operational
	TenantStatusDisabled  = 2  // Tenant is temporarily disabled
	TenantStatusSuspended = 3  // Tenant is suspended (billing issues, violations, etc.)
)

// Tenant plan type constants
const (
	TenantPlanFree       = "free"       // Free plan
	TenantPlanPro        = "pro"        // Pro plan
	TenantPlanEnterprise = "enterprise" // Enterprise plan
)

// TableName specifies the table name for Tenant model
func (Tenant) TableName() string {
	return "tenants"
}

// IsEnabled checks if tenant is enabled
func (t *Tenant) IsEnabled() bool {
	return t.Status == TenantStatusEnabled
}

// IsDisabled checks if tenant is disabled or suspended
func (t *Tenant) IsDisabled() bool {
	return t.Status == TenantStatusDisabled || t.Status == TenantStatusSuspended
}

// GetTenantByID retrieves a tenant by its ID
func GetTenantByID(id string) (*Tenant, error) {
	var tenant Tenant
	err := DB.Where("id = ?", id).First(&tenant).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("tenant not found")
		}
		return nil, err
	}
	return &tenant, nil
}

// GetTenantBySlug retrieves a tenant by its slug
func GetTenantBySlug(slug string) (*Tenant, error) {
	var tenant Tenant
	err := DB.Where("slug = ?", slug).First(&tenant).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("tenant not found")
		}
		return nil, err
	}
	return &tenant, nil
}

// GetTenantByZitadelOrgID retrieves a tenant by Zitadel Organization ID
func GetTenantByZitadelOrgID(orgID string) (*Tenant, error) {
	var tenant Tenant
	err := DB.Where("zitadel_org_id = ?", orgID).First(&tenant).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("tenant not found for Zitadel Org ID")
		}
		return nil, err
	}
	return &tenant, nil
}

// CreateTenantFromZitadel creates a new tenant from Zitadel Organization data
// Auto-called when a user from a new Zitadel Organization logs in
func CreateTenantFromZitadel(orgID string, orgDomain string, orgName string) (*Tenant, error) {
	// Check if tenant already exists
	existingTenant, _ := GetTenantByZitadelOrgID(orgID)
	if existingTenant != nil {
		return existingTenant, nil
	}

	// Generate tenant ID (can use orgID or generate new UUID)
	tenantID := GenerateID() // You can implement this function or use orgID directly

	tenant := &Tenant{
		Id:           tenantID,
		ZitadelOrgID: orgID,
		Slug:         orgDomain, // Use Zitadel org domain as slug
		Name:         orgName,
		Status:       TenantStatusEnabled,
		PlanType:     TenantPlanFree, // Default to free plan
		MaxUsers:     100,
		MaxQuota:     1000000, // 1M tokens
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	err := DB.Create(tenant).Error
	if err != nil {
		return nil, err
	}

	return tenant, nil
}

// UpdateTenant updates tenant information
func UpdateTenant(id string, updates map[string]interface{}) error {
	updates["updated_at"] = time.Now()
	return DB.Model(&Tenant{}).Where("id = ?", id).Updates(updates).Error
}

// DisableTenant disables a tenant
func DisableTenant(id string) error {
	return UpdateTenant(id, map[string]interface{}{
		"status": TenantStatusDisabled,
	})
}

// EnableTenant enables a tenant
func EnableTenant(id string) error {
	return UpdateTenant(id, map[string]interface{}{
		"status": TenantStatusEnabled,
	})
}

// SuspendTenant suspends a tenant (for billing issues or violations)
func SuspendTenant(id string) error {
	return UpdateTenant(id, map[string]interface{}{
		"status": TenantStatusSuspended,
	})
}

// DeleteTenant soft deletes a tenant
func DeleteTenant(id string) error {
	return DB.Delete(&Tenant{}, "id = ?", id).Error
}

// ListTenants retrieves all tenants with pagination
func ListTenants(offset int, limit int, status int) ([]*Tenant, int64, error) {
	var tenants []*Tenant
	var total int64

	query := DB.Model(&Tenant{})

	// Filter by status if provided
	if status > 0 {
		query = query.Where("status = ?", status)
	}

	// Get total count
	err := query.Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	// Get paginated results
	err = query.Offset(offset).Limit(limit).Order("created_at DESC").Find(&tenants).Error
	if err != nil {
		return nil, 0, err
	}

	return tenants, total, nil
}

// GetTenantUserCount returns the number of users in a tenant
func GetTenantUserCount(tenantID string) (int64, error) {
	var count int64
	err := DB.Model(&User{}).Where("tenant_id = ?", tenantID).Count(&count).Error
	return count, err
}

// CanAddUser checks if tenant can add more users (based on max_users limit)
func (t *Tenant) CanAddUser() (bool, error) {
	currentUserCount, err := GetTenantUserCount(t.Id)
	if err != nil {
		return false, err
	}

	return currentUserCount < int64(t.MaxUsers), nil
}

// GenerateID generates a unique ID for tenant
// You can implement this using UUID library or custom logic
func GenerateID() string {
	// TODO: Implement UUID generation
	// For now, using a placeholder
	// In production, use: github.com/google/uuid
	return "tenant-" + time.Now().Format("20060102150405")
}

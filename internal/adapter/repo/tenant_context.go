package repo

import (
	"context"
	"errors"
	"sync"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// TenantContextManager manages tenant context across Gin and GORM
type TenantContextManager struct {
	db *gorm.DB
}

var tenantContextManager *TenantContextManager

// InitTenantContextManager initializes the tenant context manager
// Must be called after DB initialization
func InitTenantContextManager(db *gorm.DB) error {
	// Register tenant plugin
	if err := db.Use(&TenantPlugin{}); err != nil {
		return err
	}

	tenantContextManager = &TenantContextManager{
		db: db,
	}

	return nil
}

// GetTenantContextManager returns the global tenant context manager
func GetTenantContextManager() *TenantContextManager {
	return tenantContextManager
}

// InjectTenantContext injects tenant context into Gin context
// Called by authentication middleware after user authentication
func InjectTenantContext(c *gin.Context, tenantID string, userID int) {
	c.Set("tenant_id", tenantID)
	c.Set("user_id", userID)
}

// GetTenantID retrieves tenant ID from Gin context
func GetTenantID(c *gin.Context) (string, error) {
	tenantID, exists := c.Get("tenant_id")
	if !exists {
		return "", errors.New("tenant_id not found in context")
	}

	tid, ok := tenantID.(string)
	if !ok {
		return "", errors.New("tenant_id is not a string")
	}

	return tid, nil
}

// GetUserID retrieves user ID from Gin context
func GetUserID(c *gin.Context) (int, error) {
	userID, exists := c.Get("user_id")
	if !exists {
		return 0, errors.New("user_id not found in context")
	}

	uid, ok := userID.(int)
	if !ok {
		return 0, errors.New("user_id is not an integer")
	}

	return uid, nil
}

// GetTenantDB returns a GORM DB instance with tenant context injected, which
// engages TenantPlugin's automatic tenant_id filtering. Most handlers instead
// apply their own explicit .Where("tenant_id = ?", ...) filters and never call
// this — the plugin's filtering is a defense-in-depth backstop, not the
// primary enforcement layer.
func GetTenantDB(c *gin.Context) (*gorm.DB, error) {
	tenantID, err := GetTenantID(c)
	if err != nil {
		return nil, err
	}

	// Create context with tenant ID
	ctx := context.WithValue(c.Request.Context(), TenantIDContextKey, tenantID)

	// Return DB instance with tenant context
	return DB.WithContext(ctx), nil
}

// GetTenantDBWithID returns a GORM DB instance with specified tenant ID
// Use this when you need to access a specific tenant's data
func GetTenantDBWithID(tenantID string) *gorm.DB {
	ctx := context.WithValue(context.Background(), TenantIDContextKey, tenantID)
	return DB.WithContext(ctx)
}

// GetSystemDB returns a GORM DB instance without tenant isolation
// Use this ONLY for Platform Admin operations
func GetSystemDB() *gorm.DB {
	return WithoutTenantIsolation(DB)
}

// defaultTenantIDFallback is the literal id used when no tenants row can be
// resolved yet (fresh DB, before migration 021's default-tenant seed runs).
const defaultTenantIDFallback = "default"

var (
	defaultTenantIDOnce sync.Once
	resolvedTenantID    string
)

// ResolveDefaultTenantID returns the id of the tenant that backward-compatible
// v1 routes should operate against. Some environments seeded their default
// tenant row under id "lurus-default" (slug "lurus") instead of the literal
// "default" — hardcoding "default" everywhere then silently targets an orphan
// tenant with no data. Resolved once (sync.Once) via a query mirroring
// migration 021 section 4, and cached for the process lifetime; falls back to
// the "default" literal when no matching row exists yet (fresh DB pre-migration).
func ResolveDefaultTenantID() string {
	defaultTenantIDOnce.Do(func() {
		resolvedTenantID = defaultTenantIDFallback
		if DB == nil {
			return
		}
		var id string
		err := DB.Raw(
			"SELECT id FROM tenants WHERE id = ? OR slug = ? ORDER BY (id = ?) DESC LIMIT 1",
			defaultTenantIDFallback, "lurus", defaultTenantIDFallback,
		).Scan(&id).Error
		if err == nil && id != "" {
			resolvedTenantID = id
		}
	})
	return resolvedTenantID
}

// GetDefaultTenantDB returns a GORM DB instance for the default tenant
// Use this for v1 API backward compatibility
func GetDefaultTenantDB() *gorm.DB {
	return GetTenantDBWithID(ResolveDefaultTenantID())
}

// WithTenantContext wraps a function with tenant context
// Useful for background jobs that need tenant isolation
func WithTenantContext(tenantID string, fn func(db *gorm.DB) error) error {
	db := GetTenantDBWithID(tenantID)
	return fn(db)
}

// TenantTransaction executes a function within a tenant-scoped transaction
func TenantTransaction(c *gin.Context, fn func(tx *gorm.DB) error) error {
	tenantDB, err := GetTenantDB(c)
	if err != nil {
		return err
	}

	return tenantDB.Transaction(fn)
}

// TenantTransactionWithID executes a function within a tenant-scoped transaction with specific tenant ID
func TenantTransactionWithID(tenantID string, fn func(tx *gorm.DB) error) error {
	tenantDB := GetTenantDBWithID(tenantID)
	return tenantDB.Transaction(fn)
}

// SystemTransaction executes a function within a system-level transaction (no tenant isolation)
func SystemTransaction(fn func(tx *gorm.DB) error) error {
	systemDB := GetSystemDB()
	return systemDB.Transaction(fn)
}

// ValidateTenantAccess checks if the current user has access to the specified tenant
// Returns error if tenant mismatch
func ValidateTenantAccess(c *gin.Context, resourceTenantID string) error {
	currentTenantID, err := GetTenantID(c)
	if err != nil {
		return err
	}

	if currentTenantID != resourceTenantID {
		return errors.New("access denied: tenant mismatch")
	}

	return nil
}

// IsPlatformAdmin checks if the current request is from a platform admin
// Platform admins can skip tenant isolation
func IsPlatformAdmin(c *gin.Context) bool {
	// Check if user has platform admin role
	// This can be determined by checking user role or a special header
	role, exists := c.Get("user_role")
	if !exists {
		return false
	}

	// Check if role is platform admin (role code may vary)
	// For now, using role code 100 for platform admin
	roleInt, ok := role.(int)
	if !ok {
		return false
	}

	return roleInt == 100 // RolePlatformAdmin (define this constant in common package)
}

// RequireTenantAccess middleware ensures tenant context exists
// Use this middleware for API routes that require tenant isolation
func RequireTenantAccess() gin.HandlerFunc {
	return func(c *gin.Context) {
		_, err := GetTenantID(c)
		if err != nil {
			c.JSON(401, gin.H{
				"success": false,
				"message": "Tenant context not found. Please authenticate first.",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// SetDefaultTenant middleware sets default tenant for v1 API backward compatibility
// Use this for routes that should always use the default tenant
func SetDefaultTenant() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Inject default tenant context
		InjectTenantContext(c, ResolveDefaultTenantID(), 0)
		c.Next()
	}
}

// TenantSwitchMiddleware allows platform admins to switch tenants via header
// Use this for Platform Admin dashboard
func TenantSwitchMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check if user is platform admin
		if !IsPlatformAdmin(c) {
			c.Next()
			return
		}

		// Check for tenant switch header
		targetTenantID := c.GetHeader("X-Target-Tenant-ID")
		if targetTenantID != "" {
			// Validate tenant exists
			_, err := GetTenantByID(targetTenantID)
			if err != nil {
				c.JSON(404, gin.H{
					"success": false,
					"message": "Target tenant not found",
				})
				c.Abort()
				return
			}

			// Switch to target tenant
			currentUserID, _ := GetUserID(c)
			InjectTenantContext(c, targetTenantID, currentUserID)
		}

		c.Next()
	}
}

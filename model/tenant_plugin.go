package model

import (
	"context"
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// TenantPlugin is a GORM plugin that automatically filters queries by tenant_id
// and sets tenant_id on create operations
type TenantPlugin struct{}

// Name returns the plugin name
func (p *TenantPlugin) Name() string {
	return "TenantPlugin"
}

// Initialize initializes the tenant plugin
func (p *TenantPlugin) Initialize(db *gorm.DB) error {
	// Register callbacks for tenant isolation
	if err := db.Callback().Query().Before("gorm:query").Register("tenant:before_query", beforeQuery); err != nil {
		return err
	}

	if err := db.Callback().Create().Before("gorm:create").Register("tenant:before_create", beforeCreate); err != nil {
		return err
	}

	if err := db.Callback().Update().Before("gorm:update").Register("tenant:before_update", beforeUpdate); err != nil {
		return err
	}

	if err := db.Callback().Delete().Before("gorm:delete").Register("tenant:before_delete", beforeDelete); err != nil {
		return err
	}

	return nil
}

// Context keys for tenant isolation
const (
	TenantIDContextKey     = "tenant_id"       // Current tenant ID
	SkipTenantIsolationKey = "skip_tenant_isolation" // Skip tenant isolation flag
)

// beforeQuery adds tenant_id filter to all SELECT queries
func beforeQuery(db *gorm.DB) {
	// Check if tenant isolation should be skipped
	if skipTenantIsolation(db) {
		return
	}

	// Get tenant ID from context
	tenantID := getTenantIDFromContext(db)
	if tenantID == "" {
		// No tenant ID in context, skip (for system operations)
		return
	}

	// Check if the table has tenant_id column
	if !hasTenantIDColumn(db) {
		return
	}

	// Add WHERE tenant_id = ? clause
	db.Statement.AddClause(clause.Where{
		Exprs: []clause.Expression{
			clause.Expr{SQL: "tenant_id = ?", Vars: []interface{}{tenantID}},
		},
	})
}

// beforeCreate sets tenant_id before creating records
func beforeCreate(db *gorm.DB) {
	// Check if tenant isolation should be skipped
	if skipTenantIsolation(db) {
		return
	}

	// Get tenant ID from context
	tenantID := getTenantIDFromContext(db)
	if tenantID == "" {
		// No tenant ID in context, this is an error for CREATE operations
		db.AddError(errors.New("tenant_id is required for create operations"))
		return
	}

	// Check if the table has tenant_id column
	if !hasTenantIDColumn(db) {
		return
	}

	// Set tenant_id field
	db.Statement.SetColumn("tenant_id", tenantID)
}

// beforeUpdate adds tenant_id filter to UPDATE queries
func beforeUpdate(db *gorm.DB) {
	// Check if tenant isolation should be skipped
	if skipTenantIsolation(db) {
		return
	}

	// Get tenant ID from context
	tenantID := getTenantIDFromContext(db)
	if tenantID == "" {
		// No tenant ID in context, skip (for system operations)
		return
	}

	// Check if the table has tenant_id column
	if !hasTenantIDColumn(db) {
		return
	}

	// Add WHERE tenant_id = ? clause
	db.Statement.AddClause(clause.Where{
		Exprs: []clause.Expression{
			clause.Expr{SQL: "tenant_id = ?", Vars: []interface{}{tenantID}},
		},
	})
}

// beforeDelete adds tenant_id filter to DELETE queries
func beforeDelete(db *gorm.DB) {
	// Check if tenant isolation should be skipped
	if skipTenantIsolation(db) {
		return
	}

	// Get tenant ID from context
	tenantID := getTenantIDFromContext(db)
	if tenantID == "" {
		// No tenant ID in context, skip (for system operations)
		return
	}

	// Check if the table has tenant_id column
	if !hasTenantIDColumn(db) {
		return
	}

	// Add WHERE tenant_id = ? clause
	db.Statement.AddClause(clause.Where{
		Exprs: []clause.Expression{
			clause.Expr{SQL: "tenant_id = ?", Vars: []interface{}{tenantID}},
		},
	})
}

// getTenantIDFromContext retrieves tenant_id from GORM context
func getTenantIDFromContext(db *gorm.DB) string {
	if db.Statement.Context == nil {
		return ""
	}

	tenantID, ok := db.Statement.Context.Value(TenantIDContextKey).(string)
	if !ok {
		return ""
	}

	return tenantID
}

// skipTenantIsolation checks if tenant isolation should be skipped
func skipTenantIsolation(db *gorm.DB) bool {
	if db.Statement.Context == nil {
		return false
	}

	skip, ok := db.Statement.Context.Value(SkipTenantIsolationKey).(bool)
	return ok && skip
}

// hasTenantIDColumn checks if the current table has tenant_id column
func hasTenantIDColumn(db *gorm.DB) bool {
	// Tables that have tenant_id column
	tablesWithTenantID := map[string]bool{
		"users":              true,
		"tokens":             true,
		"channels":           true,
		"topups":             true,
		"subscriptions":      true,
		"redemptions":        true,
		"logs":               true,
		"passkeys":           true,
		"twofa":              true,
		// Add more tables as needed
	}

	tableName := db.Statement.Table
	if tableName == "" {
		// Try to get table name from schema
		if db.Statement.Schema != nil {
			tableName = db.Statement.Schema.Table
		}
	}

	return tablesWithTenantID[tableName]
}

// WithTenantID returns a new DB instance with tenant_id in context
// Use this helper function to inject tenant ID into GORM operations
func WithTenantID(db *gorm.DB, tenantID string) *gorm.DB {
	return db.WithContext(context.WithValue(db.Statement.Context, TenantIDContextKey, tenantID))
}

// WithoutTenantIsolation returns a new DB instance with tenant isolation disabled
// Use this for Platform Admin operations that need to access all tenants
func WithoutTenantIsolation(db *gorm.DB) *gorm.DB {
	ctx := db.Statement.Context
	if ctx == nil {
		ctx = context.Background()
	}
	return db.WithContext(context.WithValue(ctx, SkipTenantIsolationKey, true))
}

// GetTenantIDFromDB retrieves tenant ID from DB context
func GetTenantIDFromDB(db *gorm.DB) string {
	return getTenantIDFromContext(db)
}

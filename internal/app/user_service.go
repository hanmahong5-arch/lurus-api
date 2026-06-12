package app

import (
	"errors"

	"github.com/LurusTech/lurus-hub/internal/pkg/common"
)

const (
	DisplayNameMaxLength = 50
)

// CheckPermission verifies that the acting user has sufficient role to operate on the target user.
// Returns nil if permitted, or an error describing the permission violation.
func CheckPermission(actorRole int, targetRole int) error {
	if actorRole <= targetRole && actorRole != common.RoleRootUser {
		return errors.New("无权操作同权限等级或更高权限等级的用户")
	}
	return nil
}

// CheckRolePromotion verifies that the acting user can promote a target to the given new role.
func CheckRolePromotion(actorRole int, newRole int) error {
	if actorRole <= newRole && actorRole != common.RoleRootUser {
		return errors.New("无权将其他用户权限等级提升到大于等于自己的权限等级")
	}
	return nil
}

// CheckSelfDemotion rejects an actor lowering their own role. Once demoted the
// actor loses admin access and cannot restore it via the API (lockout), which
// is why even the root user is not exempt here. newRole 0 means "not provided"
// in the update payload (repo Edit skips persisting it), so it is ignored.
func CheckSelfDemotion(actorId int, targetId int, actorRole int, newRole int) error {
	if actorId == targetId && newRole != 0 && newRole < actorRole {
		return errors.New("不能降低自己的权限等级")
	}
	return nil
}

// ValidateDisplayName checks that the display name does not exceed the maximum length.
func ValidateDisplayName(displayName string) error {
	if len(displayName) > DisplayNameMaxLength {
		return errors.New("显示名称不能超过50个字符")
	}
	return nil
}

// GetTenantIdFromContext returns the tenant_id from gin context, defaulting to "default".
func GetTenantIdFromContext(tenantId string) string {
	if tenantId == "" {
		return "default"
	}
	return tenantId
}

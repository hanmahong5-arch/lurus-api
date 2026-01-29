package middleware

import (
	"net/http"

	"github.com/QuantumNous/lurus-api/internal/pkg/common"
	"github.com/QuantumNous/lurus-api/internal/data/model"
	"github.com/gin-gonic/gin"
)

// SensitiveActionGuard creates a middleware that checks if phone verification is required
// for a specific sensitive action type. This allows fine-grained control over which
// actions require phone verification.
func SensitiveActionGuard(actionType string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get user ID from context (set by auth middleware)
		userId := c.GetInt("id")
		if userId == 0 {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"message": "User not authenticated",
			})
			c.Abort()
			return
		}

		// Get user info
		user, err := model.GetUserById(userId, false)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Failed to get user information",
			})
			c.Abort()
			return
		}

		// Check if phone verification is required for this action
		requirePhone := isPhoneRequiredForAction(actionType)

		if requirePhone && !user.PhoneVerified {
			c.JSON(http.StatusForbidden, gin.H{
				"success":                    false,
				"message":                    "This operation requires phone verification",
				"require_phone_verification": true,
				"action_type":                actionType,
			})
			c.Abort()
			return
		}

		// Check if 2FA is required for sensitive actions
		if common.SensitiveActionRequire2FA && actionType != common.SensitiveAction2FAChange {
			twoFAEnabled := model.IsTwoFAEnabled(userId)
			if !twoFAEnabled {
				c.JSON(http.StatusForbidden, gin.H{
					"success":           false,
					"message":           "This operation requires two-factor authentication to be enabled",
					"require_2fa_setup": true,
					"action_type":       actionType,
				})
				c.Abort()
				return
			}
			// TODO: Verify current session has passed 2FA verification
			// This would require session-based 2FA verification tracking
		}

		// Check if password re-entry is required (for future implementation)
		if common.SensitiveActionRequirePassword {
			// TODO: Implement password re-entry verification
			// This would require a recent password verification mechanism
		}

		c.Next()
	}
}

// isPhoneRequiredForAction checks if phone verification is required for a specific action
func isPhoneRequiredForAction(actionType string) bool {
	switch actionType {
	case common.SensitiveActionPayment:
		return common.PhoneRequiredForPayment
	case common.SensitiveActionWithdrawal:
		return common.PhoneRequiredForWithdrawal
	case common.SensitiveActionPasswordChange:
		return common.PhoneRequiredForPasswordReset
	case common.SensitiveAction2FAChange:
		return common.PhoneRequiredFor2FAChange
	case common.SensitiveActionPhoneBind:
		return common.PhoneRequiredForPhoneBind
	case common.SensitiveActionOAuthBind:
		return common.PhoneRequiredForOAuthBind
	case common.SensitiveActionAccountDelete:
		return common.PhoneRequiredForAccountDelete
	case common.SensitiveActionTokenGenerate:
		return common.PhoneRequiredForTokenGenerate
	default:
		return false
	}
}

// RequirePhoneVerification is a middleware that requires phone verification for the endpoint
// regardless of action type configuration. This is for endpoints that always need phone verification.
func RequirePhoneVerification() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip if phone verification is globally disabled
		if common.PhoneVerificationMode == common.PhoneVerificationDisabled {
			c.Next()
			return
		}

		userId := c.GetInt("id")
		if userId == 0 {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"message": "User not authenticated",
			})
			c.Abort()
			return
		}

		user, err := model.GetUserById(userId, false)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Failed to get user information",
			})
			c.Abort()
			return
		}

		if !user.PhoneVerified {
			c.JSON(http.StatusForbidden, gin.H{
				"success":                    false,
				"message":                    "Phone verification required for this operation",
				"require_phone_verification": true,
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// CheckPhoneVerificationForLogin is a middleware that checks if phone verification is required
// for login and returns appropriate response for frontend handling
func CheckPhoneVerificationForLogin() gin.HandlerFunc {
	return func(c *gin.Context) {
		// This middleware is called after successful authentication
		// to check if additional phone verification is needed

		if common.PhoneVerificationMode != common.PhoneVerificationRequiredLogin &&
			!common.PhoneRequiredForLogin {
			c.Next()
			return
		}

		userId := c.GetInt("id")
		if userId == 0 {
			c.Next()
			return
		}

		user, err := model.GetUserById(userId, false)
		if err != nil {
			c.Next()
			return
		}

		if !user.PhoneVerified {
			// Set a flag in context that the controller can use to return
			// a special response asking for phone verification
			c.Set("require_phone_verification_for_login", true)
		}

		c.Next()
	}
}

// GetUserVerificationStatus returns the verification status for the current user
// This can be used by endpoints to inform frontend about what verifications are missing
func GetUserVerificationStatus(c *gin.Context) {
	userId := c.GetInt("id")
	if userId == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "User not authenticated",
		})
		return
	}

	user, err := model.GetUserById(userId, false)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to get user information",
		})
		return
	}

	// Build list of actions that require phone verification
	phoneRequiredFor := make([]string, 0)
	if common.PhoneRequiredForPayment {
		phoneRequiredFor = append(phoneRequiredFor, common.SensitiveActionPayment)
	}
	if common.PhoneRequiredForWithdrawal {
		phoneRequiredFor = append(phoneRequiredFor, common.SensitiveActionWithdrawal)
	}
	if common.PhoneRequiredForPasswordReset {
		phoneRequiredFor = append(phoneRequiredFor, common.SensitiveActionPasswordChange)
	}
	if common.PhoneRequiredFor2FAChange {
		phoneRequiredFor = append(phoneRequiredFor, common.SensitiveAction2FAChange)
	}
	if common.PhoneRequiredForPhoneBind {
		phoneRequiredFor = append(phoneRequiredFor, common.SensitiveActionPhoneBind)
	}
	if common.PhoneRequiredForOAuthBind {
		phoneRequiredFor = append(phoneRequiredFor, common.SensitiveActionOAuthBind)
	}
	if common.PhoneRequiredForAccountDelete {
		phoneRequiredFor = append(phoneRequiredFor, common.SensitiveActionAccountDelete)
	}
	if common.PhoneRequiredForTokenGenerate {
		phoneRequiredFor = append(phoneRequiredFor, common.SensitiveActionTokenGenerate)
	}

	// Check 2FA status via model function
	twoFAEnabled := model.IsTwoFAEnabled(userId)

	// Determine if user can perform sensitive actions
	canPerformSensitiveActions := true
	if len(phoneRequiredFor) > 0 && !user.PhoneVerified {
		canPerformSensitiveActions = false
	}
	if common.SensitiveActionRequire2FA && !twoFAEnabled {
		canPerformSensitiveActions = false
	}

	// Email is considered verified if it's non-empty (for this system)
	emailVerified := user.Email != ""

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"phone_verified":                user.PhoneVerified,
			"email_verified":                emailVerified,
			"2fa_enabled":                   twoFAEnabled,
			"phone":                         maskPhone(user.Phone),
			"phone_required_for":            phoneRequiredFor,
			"can_perform_sensitive_actions": canPerformSensitiveActions,
		},
	})
}

// maskPhone masks phone number for privacy (13800138000 -> 138****8000)
func maskPhone(phone string) string {
	if len(phone) < 7 {
		return phone
	}
	return phone[:3] + "****" + phone[len(phone)-4:]
}

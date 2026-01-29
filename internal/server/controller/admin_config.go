package controller

import (
	"net/http"

	"github.com/QuantumNous/lurus-api/internal/pkg/common"
	"github.com/QuantumNous/lurus-api/internal/data/model"
	"github.com/gin-gonic/gin"
)

// LoginConfigResponse represents the full login configuration for admin
type LoginConfigResponse struct {
	// Login Methods
	PasswordLoginEnabled    bool `json:"password_login_enabled"`
	PasswordRegisterEnabled bool `json:"password_register_enabled"`
	SMSEnabled              bool `json:"sms_enabled"`
	SMSAutoRegister         bool `json:"sms_auto_register"`
	GitHubOAuthEnabled      bool `json:"github_oauth_enabled"`
	LinuxDOOAuthEnabled     bool `json:"linuxdo_oauth_enabled"`
	WeChatAuthEnabled       bool `json:"wechat_auth_enabled"`
	TelegramOAuthEnabled    bool `json:"telegram_oauth_enabled"`

	// Phone Verification
	PhoneVerificationMode         string `json:"phone_verification_mode"`
	PhoneRequiredForLogin         bool   `json:"phone_required_for_login"`
	PhoneRequiredForPasswordReset bool   `json:"phone_required_for_password_reset"`
	PhoneRequiredFor2FAChange     bool   `json:"phone_required_for_2fa_change"`
	PhoneRequiredForPayment       bool   `json:"phone_required_for_payment"`
	PhoneRequiredForWithdrawal    bool   `json:"phone_required_for_withdrawal"`
	PhoneRequiredForPhoneBind     bool   `json:"phone_required_for_phone_bind"`
	PhoneRequiredForAccountDelete bool   `json:"phone_required_for_account_delete"`
	PhoneRequiredForTokenGenerate bool   `json:"phone_required_for_token_generate"`
	PhoneRequiredForOAuthBind     bool   `json:"phone_required_for_oauth_bind"`

	// Registration
	RegistrationMode          string `json:"registration_mode"`
	RegisterEnabled           bool   `json:"register_enabled"`
	EmailVerificationEnabled  bool   `json:"email_verification_enabled"`
	InviteCodeRequired        bool   `json:"invite_code_required"`
	TurnstileCheckEnabled     bool   `json:"turnstile_check_enabled"`

	// Security
	SensitiveActionRequirePassword bool `json:"sensitive_action_require_password"`
	SensitiveActionRequire2FA      bool `json:"sensitive_action_require_2fa"`
	SessionTimeoutMinutes          int  `json:"session_timeout_minutes"`
}

// LoginConfigUpdateRequest represents a partial update request for login configuration
type LoginConfigUpdateRequest struct {
	// Login Methods
	PasswordLoginEnabled    *bool `json:"password_login_enabled"`
	PasswordRegisterEnabled *bool `json:"password_register_enabled"`
	SMSEnabled              *bool `json:"sms_enabled"`
	SMSAutoRegister         *bool `json:"sms_auto_register"`

	// Phone Verification
	PhoneVerificationMode         *string `json:"phone_verification_mode"`
	PhoneRequiredForLogin         *bool   `json:"phone_required_for_login"`
	PhoneRequiredForPasswordReset *bool   `json:"phone_required_for_password_reset"`
	PhoneRequiredFor2FAChange     *bool   `json:"phone_required_for_2fa_change"`
	PhoneRequiredForPayment       *bool   `json:"phone_required_for_payment"`
	PhoneRequiredForWithdrawal    *bool   `json:"phone_required_for_withdrawal"`
	PhoneRequiredForPhoneBind     *bool   `json:"phone_required_for_phone_bind"`
	PhoneRequiredForAccountDelete *bool   `json:"phone_required_for_account_delete"`
	PhoneRequiredForTokenGenerate *bool   `json:"phone_required_for_token_generate"`
	PhoneRequiredForOAuthBind     *bool   `json:"phone_required_for_oauth_bind"`

	// Registration
	RegistrationMode         *string `json:"registration_mode"`
	RegisterEnabled          *bool   `json:"register_enabled"`
	EmailVerificationEnabled *bool   `json:"email_verification_enabled"`
	InviteCodeRequired       *bool   `json:"invite_code_required"`

	// Security
	SensitiveActionRequirePassword *bool `json:"sensitive_action_require_password"`
	SensitiveActionRequire2FA      *bool `json:"sensitive_action_require_2fa"`
	SessionTimeoutMinutes          *int  `json:"session_timeout_minutes"`
}

// AdminGetLoginConfig returns the full login configuration for Root users
func AdminGetLoginConfig(c *gin.Context) {
	config := LoginConfigResponse{
		// Login Methods
		PasswordLoginEnabled:    common.PasswordLoginEnabled,
		PasswordRegisterEnabled: common.PasswordRegisterEnabled,
		SMSEnabled:              common.SMSEnabled,
		SMSAutoRegister:         common.SMSAutoRegister,
		GitHubOAuthEnabled:      common.GitHubOAuthEnabled,
		LinuxDOOAuthEnabled:     common.LinuxDOOAuthEnabled,
		WeChatAuthEnabled:       common.WeChatAuthEnabled,
		TelegramOAuthEnabled:    common.TelegramOAuthEnabled,

		// Phone Verification
		PhoneVerificationMode:         common.PhoneVerificationMode,
		PhoneRequiredForLogin:         common.PhoneRequiredForLogin,
		PhoneRequiredForPasswordReset: common.PhoneRequiredForPasswordReset,
		PhoneRequiredFor2FAChange:     common.PhoneRequiredFor2FAChange,
		PhoneRequiredForPayment:       common.PhoneRequiredForPayment,
		PhoneRequiredForWithdrawal:    common.PhoneRequiredForWithdrawal,
		PhoneRequiredForPhoneBind:     common.PhoneRequiredForPhoneBind,
		PhoneRequiredForAccountDelete: common.PhoneRequiredForAccountDelete,
		PhoneRequiredForTokenGenerate: common.PhoneRequiredForTokenGenerate,
		PhoneRequiredForOAuthBind:     common.PhoneRequiredForOAuthBind,

		// Registration
		RegistrationMode:          common.RegistrationMode,
		RegisterEnabled:           common.RegisterEnabled,
		EmailVerificationEnabled:  common.EmailVerificationEnabled,
		InviteCodeRequired:        common.InviteCodeRequired,
		TurnstileCheckEnabled:     common.TurnstileCheckEnabled,

		// Security
		SensitiveActionRequirePassword: common.SensitiveActionRequirePassword,
		SensitiveActionRequire2FA:      common.SensitiveActionRequire2FA,
		SessionTimeoutMinutes:          common.SessionTimeoutMinutes,
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    config,
	})
}

// AdminUpdateLoginConfig updates login configuration (partial update supported)
func AdminUpdateLoginConfig(c *gin.Context) {
	var req LoginConfigUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request parameters",
		})
		return
	}

	// Validate dependencies before applying changes
	if err := validateLoginConfigUpdate(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	// Apply updates
	var updateErrors []string

	// Login Methods
	if req.PasswordLoginEnabled != nil {
		if err := model.UpdateOption("PasswordLoginEnabled", boolToString(*req.PasswordLoginEnabled)); err != nil {
			updateErrors = append(updateErrors, "PasswordLoginEnabled: "+err.Error())
		}
	}
	if req.PasswordRegisterEnabled != nil {
		if err := model.UpdateOption("PasswordRegisterEnabled", boolToString(*req.PasswordRegisterEnabled)); err != nil {
			updateErrors = append(updateErrors, "PasswordRegisterEnabled: "+err.Error())
		}
	}
	if req.SMSEnabled != nil {
		if err := model.UpdateOption("SMSEnabled", boolToString(*req.SMSEnabled)); err != nil {
			updateErrors = append(updateErrors, "SMSEnabled: "+err.Error())
		}
	}
	if req.SMSAutoRegister != nil {
		if err := model.UpdateOption("SMSAutoRegister", boolToString(*req.SMSAutoRegister)); err != nil {
			updateErrors = append(updateErrors, "SMSAutoRegister: "+err.Error())
		}
	}

	// Phone Verification
	if req.PhoneVerificationMode != nil {
		if err := model.UpdateOption("PhoneVerificationMode", *req.PhoneVerificationMode); err != nil {
			updateErrors = append(updateErrors, "PhoneVerificationMode: "+err.Error())
		}
	}
	if req.PhoneRequiredForLogin != nil {
		if err := model.UpdateOption("PhoneRequiredForLogin", boolToString(*req.PhoneRequiredForLogin)); err != nil {
			updateErrors = append(updateErrors, "PhoneRequiredForLogin: "+err.Error())
		}
	}
	if req.PhoneRequiredForPasswordReset != nil {
		if err := model.UpdateOption("PhoneRequiredForPasswordReset", boolToString(*req.PhoneRequiredForPasswordReset)); err != nil {
			updateErrors = append(updateErrors, "PhoneRequiredForPasswordReset: "+err.Error())
		}
	}
	if req.PhoneRequiredFor2FAChange != nil {
		if err := model.UpdateOption("PhoneRequiredFor2FAChange", boolToString(*req.PhoneRequiredFor2FAChange)); err != nil {
			updateErrors = append(updateErrors, "PhoneRequiredFor2FAChange: "+err.Error())
		}
	}
	if req.PhoneRequiredForPayment != nil {
		if err := model.UpdateOption("PhoneRequiredForPayment", boolToString(*req.PhoneRequiredForPayment)); err != nil {
			updateErrors = append(updateErrors, "PhoneRequiredForPayment: "+err.Error())
		}
	}
	if req.PhoneRequiredForWithdrawal != nil {
		if err := model.UpdateOption("PhoneRequiredForWithdrawal", boolToString(*req.PhoneRequiredForWithdrawal)); err != nil {
			updateErrors = append(updateErrors, "PhoneRequiredForWithdrawal: "+err.Error())
		}
	}
	if req.PhoneRequiredForPhoneBind != nil {
		if err := model.UpdateOption("PhoneRequiredForPhoneBind", boolToString(*req.PhoneRequiredForPhoneBind)); err != nil {
			updateErrors = append(updateErrors, "PhoneRequiredForPhoneBind: "+err.Error())
		}
	}
	if req.PhoneRequiredForAccountDelete != nil {
		if err := model.UpdateOption("PhoneRequiredForAccountDelete", boolToString(*req.PhoneRequiredForAccountDelete)); err != nil {
			updateErrors = append(updateErrors, "PhoneRequiredForAccountDelete: "+err.Error())
		}
	}
	if req.PhoneRequiredForTokenGenerate != nil {
		if err := model.UpdateOption("PhoneRequiredForTokenGenerate", boolToString(*req.PhoneRequiredForTokenGenerate)); err != nil {
			updateErrors = append(updateErrors, "PhoneRequiredForTokenGenerate: "+err.Error())
		}
	}
	if req.PhoneRequiredForOAuthBind != nil {
		if err := model.UpdateOption("PhoneRequiredForOAuthBind", boolToString(*req.PhoneRequiredForOAuthBind)); err != nil {
			updateErrors = append(updateErrors, "PhoneRequiredForOAuthBind: "+err.Error())
		}
	}

	// Registration
	if req.RegistrationMode != nil {
		if err := model.UpdateOption("RegistrationMode", *req.RegistrationMode); err != nil {
			updateErrors = append(updateErrors, "RegistrationMode: "+err.Error())
		}
	}
	if req.RegisterEnabled != nil {
		if err := model.UpdateOption("RegisterEnabled", boolToString(*req.RegisterEnabled)); err != nil {
			updateErrors = append(updateErrors, "RegisterEnabled: "+err.Error())
		}
	}
	if req.EmailVerificationEnabled != nil {
		if err := model.UpdateOption("EmailVerificationEnabled", boolToString(*req.EmailVerificationEnabled)); err != nil {
			updateErrors = append(updateErrors, "EmailVerificationEnabled: "+err.Error())
		}
	}
	if req.InviteCodeRequired != nil {
		if err := model.UpdateOption("InviteCodeRequired", boolToString(*req.InviteCodeRequired)); err != nil {
			updateErrors = append(updateErrors, "InviteCodeRequired: "+err.Error())
		}
	}

	// Security
	if req.SensitiveActionRequirePassword != nil {
		if err := model.UpdateOption("SensitiveActionRequirePassword", boolToString(*req.SensitiveActionRequirePassword)); err != nil {
			updateErrors = append(updateErrors, "SensitiveActionRequirePassword: "+err.Error())
		}
	}
	if req.SensitiveActionRequire2FA != nil {
		if err := model.UpdateOption("SensitiveActionRequire2FA", boolToString(*req.SensitiveActionRequire2FA)); err != nil {
			updateErrors = append(updateErrors, "SensitiveActionRequire2FA: "+err.Error())
		}
	}
	if req.SessionTimeoutMinutes != nil {
		if err := model.UpdateOption("SessionTimeoutMinutes", intToString(*req.SessionTimeoutMinutes)); err != nil {
			updateErrors = append(updateErrors, "SessionTimeoutMinutes: "+err.Error())
		}
	}

	if len(updateErrors) > 0 {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Some updates failed",
			"errors":  updateErrors,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Configuration updated successfully",
	})
}

// validateLoginConfigUpdate validates dependencies between configuration options
func validateLoginConfigUpdate(req *LoginConfigUpdateRequest) error {
	// Check if phone verification is being enabled but SMS is disabled
	phoneVerificationEnabled := req.PhoneRequiredForLogin != nil && *req.PhoneRequiredForLogin ||
		req.PhoneRequiredForPasswordReset != nil && *req.PhoneRequiredForPasswordReset ||
		req.PhoneRequiredFor2FAChange != nil && *req.PhoneRequiredFor2FAChange ||
		req.PhoneRequiredForPayment != nil && *req.PhoneRequiredForPayment ||
		req.PhoneRequiredForWithdrawal != nil && *req.PhoneRequiredForWithdrawal ||
		req.PhoneRequiredForPhoneBind != nil && *req.PhoneRequiredForPhoneBind ||
		req.PhoneRequiredForAccountDelete != nil && *req.PhoneRequiredForAccountDelete ||
		req.PhoneRequiredForTokenGenerate != nil && *req.PhoneRequiredForTokenGenerate ||
		req.PhoneRequiredForOAuthBind != nil && *req.PhoneRequiredForOAuthBind

	// Check current SMS status and requested SMS status
	smsWillBeEnabled := common.SMSEnabled
	if req.SMSEnabled != nil {
		smsWillBeEnabled = *req.SMSEnabled
	}

	if phoneVerificationEnabled && !smsWillBeEnabled {
		return &configValidationError{message: "Cannot enable phone verification requirements when SMS is disabled"}
	}

	// Validate phone verification mode
	if req.PhoneVerificationMode != nil {
		switch *req.PhoneVerificationMode {
		case common.PhoneVerificationDisabled,
			common.PhoneVerificationOptional,
			common.PhoneVerificationRequiredLogin,
			common.PhoneVerificationRequiredSensitive:
			// Valid modes
		default:
			return &configValidationError{message: "Invalid phone verification mode: " + *req.PhoneVerificationMode}
		}
	}

	// Validate registration mode
	if req.RegistrationMode != nil {
		switch *req.RegistrationMode {
		case common.RegistrationModeOpen,
			common.RegistrationModeInviteOnly,
			common.RegistrationModeOAuthOnly,
			common.RegistrationModePhoneVerified,
			common.RegistrationModeClosed:
			// Valid modes
		default:
			return &configValidationError{message: "Invalid registration mode: " + *req.RegistrationMode}
		}

		// Check if phone_verified registration mode requires SMS
		if *req.RegistrationMode == common.RegistrationModePhoneVerified && !smsWillBeEnabled {
			return &configValidationError{message: "Phone verified registration mode requires SMS to be enabled"}
		}
	}

	// Validate session timeout
	if req.SessionTimeoutMinutes != nil && *req.SessionTimeoutMinutes < 1 {
		return &configValidationError{message: "Session timeout must be at least 1 minute"}
	}

	return nil
}

type configValidationError struct {
	message string
}

func (e *configValidationError) Error() string {
	return e.message
}

// GetPhoneRequiredActions returns a list of actions that require phone verification
func GetPhoneRequiredActions() []string {
	var actions []string
	if common.PhoneRequiredForLogin {
		actions = append(actions, "login")
	}
	if common.PhoneRequiredForPasswordReset {
		actions = append(actions, "password_reset")
	}
	if common.PhoneRequiredFor2FAChange {
		actions = append(actions, "2fa_change")
	}
	if common.PhoneRequiredForPayment {
		actions = append(actions, "payment")
	}
	if common.PhoneRequiredForWithdrawal {
		actions = append(actions, "withdrawal")
	}
	if common.PhoneRequiredForPhoneBind {
		actions = append(actions, "phone_bind")
	}
	if common.PhoneRequiredForAccountDelete {
		actions = append(actions, "account_delete")
	}
	if common.PhoneRequiredForTokenGenerate {
		actions = append(actions, "token_generate")
	}
	if common.PhoneRequiredForOAuthBind {
		actions = append(actions, "oauth_bind")
	}
	return actions
}

// Helper function for int to string conversion
func intToString(i int) string {
	return common.Interface2String(i)
}

// Note: boolToString is already defined in setup.go within the same package

// GetAvailablePhoneVerificationModes returns all available phone verification modes
func GetAvailablePhoneVerificationModes() []map[string]string {
	return []map[string]string{
		{"key": common.PhoneVerificationDisabled, "name": "Disabled", "description": "Phone verification is completely disabled"},
		{"key": common.PhoneVerificationOptional, "name": "Optional", "description": "Phone verification is optional for users"},
		{"key": common.PhoneVerificationRequiredLogin, "name": "Required for Login", "description": "Phone verification is required before login"},
		{"key": common.PhoneVerificationRequiredSensitive, "name": "Required for Sensitive Actions", "description": "Phone verification is required for sensitive operations"},
	}
}

// GetAvailableRegistrationModes returns all available registration modes
func GetAvailableRegistrationModes() []map[string]string {
	return []map[string]string{
		{"key": common.RegistrationModeOpen, "name": "Open", "description": "Anyone can register"},
		{"key": common.RegistrationModeInviteOnly, "name": "Invite Only", "description": "Registration requires an invitation code"},
		{"key": common.RegistrationModeOAuthOnly, "name": "OAuth Only", "description": "Only OAuth registration is allowed"},
		{"key": common.RegistrationModePhoneVerified, "name": "Phone Verified", "description": "Phone verification is required for registration"},
		{"key": common.RegistrationModeClosed, "name": "Closed", "description": "Registration is closed"},
	}
}

// AdminGetConfigModes returns available configuration modes for admin UI
func AdminGetConfigModes(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"phone_verification_modes": GetAvailablePhoneVerificationModes(),
			"registration_modes":       GetAvailableRegistrationModes(),
		},
	})
}

package controller

import (
	"fmt"
	"net/http"
	"time"

	"github.com/QuantumNous/lurus-api/internal/pkg/common"
	"github.com/QuantumNous/lurus-api/internal/data/model"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

// SMS rate limiting using in-memory cache
var smsRateLimitCache = make(map[string]time.Time)

type SendSmsRequest struct {
	Phone   string `json:"phone" binding:"required"`
	Purpose string `json:"purpose" binding:"required"` // login, register, reset, bind
}

type LoginWithSmsRequest struct {
	Phone string `json:"phone" binding:"required"`
	Code  string `json:"code" binding:"required"`
}

type BindPhoneRequest struct {
	Phone string `json:"phone" binding:"required"`
	Code  string `json:"code" binding:"required"`
}

// SendSmsVerification sends SMS verification code
// POST /api/sms/send
func SendSmsVerification(c *gin.Context) {
	var req SendSmsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request: " + err.Error(),
		})
		return
	}

	// Check if SMS is enabled
	if !common.SMSEnabled {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "SMS service is not enabled",
		})
		return
	}

	// Validate phone format
	if !common.IsValidChinesePhone(req.Phone) {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid phone number format",
		})
		return
	}

	// Validate purpose
	validPurposes := map[string]bool{
		"login":    true,
		"register": true,
		"reset":    true,
		"bind":     true,
	}
	if !validPurposes[req.Purpose] {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid purpose",
		})
		return
	}

	// Rate limiting: check if phone was sent recently (1 minute cooldown)
	cacheKey := "sms:" + req.Phone
	if lastSent, ok := smsRateLimitCache[cacheKey]; ok {
		if time.Since(lastSent) < time.Minute {
			remainingSeconds := int((time.Minute - time.Since(lastSent)).Seconds())
			c.JSON(http.StatusTooManyRequests, gin.H{
				"success": false,
				"message": fmt.Sprintf("Please wait %d seconds before requesting another code", remainingSeconds),
			})
			return
		}
	}

	// For register purpose, check if phone is already taken
	if req.Purpose == "register" {
		if model.IsPhoneAlreadyTaken(req.Phone) {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "Phone number is already registered",
			})
			return
		}
	}

	// For login purpose with auto-register, we don't check phone existence
	// The login handler will create a new user if phone doesn't exist

	// Generate verification code
	code := common.GeneratePhoneVerificationCode()

	// Get the actual SMS template code
	templateCode := common.GetSMSTemplateCode(req.Purpose)
	if templateCode == "" {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "SMS template not configured for this purpose",
		})
		return
	}

	// Build template parameter
	templateParam := common.BuildSMSTemplateParam(code)

	// Send SMS
	if err := common.SendSms(req.Phone, templateCode, templateParam); err != nil {
		common.SysLog(fmt.Sprintf("Failed to send SMS to %s: %v", req.Phone, err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to send SMS verification code",
		})
		return
	}

	// Store verification code
	purpose := common.GetPhonePurpose(req.Purpose)
	common.RegisterPhoneVerificationCode(req.Phone, code, purpose)

	// Update rate limit cache
	smsRateLimitCache[cacheKey] = time.Now()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Verification code sent successfully",
	})
}

// LoginWithSms handles phone login with SMS verification
// POST /api/user/login_sms
func LoginWithSms(c *gin.Context) {
	var req LoginWithSmsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request: " + err.Error(),
		})
		return
	}

	// Check if SMS is enabled
	if !common.SMSEnabled {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "SMS service is not enabled",
		})
		return
	}

	// Validate phone format
	if !common.IsValidChinesePhone(req.Phone) {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid phone number format",
		})
		return
	}

	// Verify code
	purpose := common.GetPhonePurpose("login")
	if !common.VerifyPhoneCode(req.Phone, req.Code, purpose) {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid or expired verification code",
		})
		return
	}

	// Find user by phone
	user, err := model.GetUserByPhone(req.Phone)
	if err != nil {
		// User doesn't exist, check if auto-register is enabled
		if !common.SMSAutoRegister {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "Phone number not registered. SMS auto-registration is disabled.",
			})
			return
		}

		// Check registration mode restrictions
		if common.RegistrationMode == common.RegistrationModeClosed {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "Registration is closed",
			})
			return
		}
		if common.RegistrationMode == common.RegistrationModeInviteOnly {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "Registration requires an invitation code. Please register through the registration page.",
			})
			return
		}

		// Auto-register new user
		user, err = model.CreateUserByPhone(req.Phone)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Failed to create user: " + err.Error(),
			})
			return
		}
	}

	// Check user status
	if user.Status != common.UserStatusEnabled {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": "User account is disabled",
		})
		return
	}

	// Setup login session
	session := sessions.Default(c)
	session.Set("id", user.Id)
	session.Set("username", user.Username)
	session.Set("role", user.Role)
	session.Set("status", user.Status)
	session.Set("group", user.Group)
	err = session.Save()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to save session",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Login successful",
		"data": gin.H{
			"id":           user.Id,
			"username":     user.Username,
			"display_name": user.DisplayName,
			"role":         user.Role,
			"status":       user.Status,
			"group":        user.Group,
			"phone":        user.Phone,
		},
	})
}

// BindPhone binds phone number to current user
// POST /api/user/bind_phone
func BindPhone(c *gin.Context) {
	var req BindPhoneRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request: " + err.Error(),
		})
		return
	}

	// Get current user ID from context
	userId := c.GetInt("id")
	if userId == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "Unauthorized",
		})
		return
	}

	// Check if SMS is enabled
	if !common.SMSEnabled {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "SMS service is not enabled",
		})
		return
	}

	// Validate phone format
	if !common.IsValidChinesePhone(req.Phone) {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid phone number format",
		})
		return
	}

	// Verify code
	purpose := common.GetPhonePurpose("bind")
	if !common.VerifyPhoneCode(req.Phone, req.Code, purpose) {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid or expired verification code",
		})
		return
	}

	// Check if phone is already taken by another user
	existingUser, _ := model.GetUserByPhone(req.Phone)
	if existingUser != nil && existingUser.Id != userId {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Phone number is already bound to another account",
		})
		return
	}

	// Update user phone
	if err := model.UpdateUserPhone(userId, req.Phone); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to bind phone: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Phone bound successfully",
	})
}

// GetSMSStatus returns SMS service status
// GET /api/sms/status
func GetSMSStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"enabled": common.SMSEnabled,
		},
	})
}

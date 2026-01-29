package controller

import (
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/lurus-api/internal/data/model"
	"github.com/QuantumNous/lurus-api/internal/pkg/common"
	"github.com/gin-gonic/gin"
)

var usernameRegexp = regexp.MustCompile(`^[a-zA-Z0-9_]+$`)

// InternalLogin authenticates a user by username and password.
// POST /internal/auth/login
func InternalLogin(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request: " + err.Error(),
		})
		return
	}

	user := &model.User{
		Username: strings.TrimSpace(req.Username),
		Password: req.Password,
	}

	err := user.ValidateAndFill()
	if err != nil {
		if user.Status == common.UserStatusDisabled {
			c.JSON(http.StatusForbidden, gin.H{
				"success":    false,
				"message":    "User is disabled",
				"error_code": "USER_DISABLED",
			})
			return
		}
		c.JSON(http.StatusUnauthorized, gin.H{
			"success":    false,
			"message":    "Invalid credentials",
			"error_code": "AUTH_FAILED",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"user_id":      user.Id,
			"username":     user.Username,
			"display_name": user.DisplayName,
			"email":        user.Email,
			"role":         user.Role,
			"status":       user.Status,
		},
	})
}

// InternalCreateUser creates a new user.
// POST /internal/user
func InternalCreateUser(c *gin.Context) {
	var req struct {
		Username    string `json:"username" binding:"required"`
		Password    string `json:"password" binding:"required"`
		Email       string `json:"email"`
		DisplayName string `json:"display_name"`
		Group       string `json:"group"`
		Quota       int    `json:"quota"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request: " + err.Error(),
		})
		return
	}

	// Validate username format
	username := strings.TrimSpace(req.Username)
	if len(username) < 3 || len(username) > 20 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success":    false,
			"message":    "Username must be 3-20 characters",
			"error_code": "VALIDATION_FAILED",
		})
		return
	}
	if !usernameRegexp.MatchString(username) {
		c.JSON(http.StatusBadRequest, gin.H{
			"success":    false,
			"message":    "Username contains invalid characters",
			"error_code": "VALIDATION_FAILED",
		})
		return
	}

	// Validate password length
	if len(req.Password) < 8 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success":    false,
			"message":    "Password must be at least 8 characters",
			"error_code": "VALIDATION_FAILED",
		})
		return
	}

	// Validate email if provided
	if req.Email != "" && !strings.Contains(req.Email, "@") {
		c.JSON(http.StatusBadRequest, gin.H{
			"success":    false,
			"message":    "Invalid email format",
			"error_code": "VALIDATION_FAILED",
		})
		return
	}

	// Check idempotency
	idempotencyKey := c.GetHeader("X-Idempotency-Key")
	if idempotencyKey != "" {
		// Check if user already created with this key
		existing := &model.User{Username: username}
		if err := model.DB.Where("username = ?", username).First(existing).Error; err == nil && existing.Id > 0 {
			c.JSON(http.StatusOK, gin.H{
				"success": true,
				"data": gin.H{
					"id":           existing.Id,
					"username":     existing.Username,
					"display_name": existing.DisplayName,
					"email":        existing.Email,
					"group":        existing.Group,
					"quota":        existing.Quota,
					"is_duplicate": true,
				},
			})
			return
		}
	}

	// Check for duplicate username
	var existingCount int64
	model.DB.Model(&model.User{}).Where("username = ?", username).Count(&existingCount)
	if existingCount > 0 {
		c.JSON(http.StatusConflict, gin.H{
			"success":    false,
			"message":    "Username already exists",
			"error_code": "USER_EXISTS",
		})
		return
	}

	// Check for duplicate email
	if req.Email != "" {
		var emailCount int64
		model.DB.Model(&model.User{}).Where("email = ?", req.Email).Count(&emailCount)
		if emailCount > 0 {
			c.JSON(http.StatusConflict, gin.H{
				"success":    false,
				"message":    "Email already exists",
				"error_code": "USER_EXISTS",
			})
			return
		}
	}

	// Hash password
	hashedPw, err := common.Password2Hash(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to hash password",
		})
		return
	}

	group := req.Group
	if group == "" {
		group = "default"
	}

	displayName := req.DisplayName
	if displayName == "" {
		displayName = username
	}

	user := &model.User{
		Username:    username,
		Password:    hashedPw,
		Email:       req.Email,
		DisplayName: displayName,
		Group:       group,
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		Quota:       req.Quota,
		AffCode:     common.GetRandomString(8),
	}

	if err := model.DB.Create(user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to create user: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data": gin.H{
			"id":           user.Id,
			"username":     user.Username,
			"display_name": user.DisplayName,
			"email":        user.Email,
			"group":        user.Group,
			"quota":        user.Quota,
		},
	})
}

// InternalDeleteUser deletes a user by ID.
// DELETE /internal/user/:id
func InternalDeleteUser(c *gin.Context) {
	userId, err := strconv.Atoi(c.Param("id"))
	if err != nil || userId <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid user ID",
		})
		return
	}

	// Protect root/admin users
	user, err := model.GetUserById(userId, false)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success":    false,
			"message":    "User not found",
			"error_code": "USER_NOT_FOUND",
		})
		return
	}

	if user.Role >= common.RoleRootUser {
		c.JSON(http.StatusForbidden, gin.H{
			"success":    false,
			"message":    "Cannot delete admin/root user",
			"error_code": "FORBIDDEN",
		})
		return
	}

	err = model.DeleteUserById(userId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to delete user: " + err.Error(),
		})
		return
	}

	keyName := c.GetString("internal_api_key_name")
	common.SysLog("Internal API deleted user " + strconv.Itoa(userId) + " via key: " + keyName)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "User deleted successfully",
	})
}

// InternalGetUserTokens returns paginated tokens for a user.
// GET /internal/token/user/:id
func InternalGetUserTokens(c *gin.Context) {
	userId, err := strconv.Atoi(c.Param("id"))
	if err != nil || userId <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid user ID",
		})
		return
	}

	// Check user exists
	_, err = model.GetUserById(userId, false)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success":    false,
			"message":    "User not found",
			"error_code": "USER_NOT_FOUND",
		})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}

	offset := (page - 1) * pageSize
	tokens, err := model.GetAllUserTokens(userId, offset, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to get tokens: " + err.Error(),
		})
		return
	}

	total, _ := model.CountUserTokens(userId)

	// Clean keys from response
	for _, t := range tokens {
		t.Clean()
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"tokens":    tokens,
			"total":     total,
			"page":      page,
			"page_size": pageSize,
		},
	})
}

// InternalCreateToken creates a new API token for a user.
// POST /internal/token
func InternalCreateToken(c *gin.Context) {
	var req struct {
		UserId         int    `json:"user_id" binding:"required"`
		Name           string `json:"name" binding:"required"`
		UnlimitedQuota bool   `json:"unlimited_quota"`
		RemainQuota    int    `json:"remain_quota"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request: " + err.Error(),
		})
		return
	}

	// Check user exists and is enabled
	user, err := model.GetUserById(req.UserId, false)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success":    false,
			"message":    "User not found",
			"error_code": "USER_NOT_FOUND",
		})
		return
	}

	if user.Status == common.UserStatusDisabled {
		c.JSON(http.StatusForbidden, gin.H{
			"success":    false,
			"message":    "User is disabled",
			"error_code": "USER_DISABLED",
		})
		return
	}

	// Check idempotency
	idempotencyKey := c.GetHeader("X-Idempotency-Key")
	if idempotencyKey != "" {
		var existing model.Token
		if err := model.DB.Where("user_id = ? AND name = ?", req.UserId, req.Name).First(&existing).Error; err == nil && existing.Id > 0 {
			c.JSON(http.StatusOK, gin.H{
				"success": true,
				"data": gin.H{
					"id":           existing.Id,
					"name":         existing.Name,
					"is_duplicate": true,
				},
			})
			return
		}
	}

	token := &model.Token{
		UserId:         req.UserId,
		Name:           req.Name,
		UnlimitedQuota: req.UnlimitedQuota,
		RemainQuota:    req.RemainQuota,
		CreatedTime:    time.Now().Unix(),
		Status:         common.TokenStatusEnabled,
		ExpiredTime:    -1,
		Group:          user.Group,
	}

	err = token.Insert()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to create token: " + err.Error(),
		})
		return
	}

	keyName := c.GetString("internal_api_key_name")
	common.SysLog("Internal API created token for user " + strconv.Itoa(req.UserId) + " via key: " + keyName)

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data": gin.H{
			"id":      token.Id,
			"key":     token.Key,
			"name":    token.Name,
			"warning": "Please save this key - it will not be shown again.",
		},
	})
}

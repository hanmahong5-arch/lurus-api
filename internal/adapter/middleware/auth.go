package middleware

import (
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/LurusTech/lurus-hub/internal/adapter/repo"
	"github.com/LurusTech/lurus-hub/internal/app"
	"github.com/LurusTech/lurus-hub/internal/app/governance"
	"github.com/LurusTech/lurus-hub/internal/pkg/common"
	"github.com/LurusTech/lurus-hub/internal/pkg/constant"
	"github.com/LurusTech/lurus-hub/internal/pkg/logger"
	"github.com/LurusTech/lurus-hub/internal/pkg/setting/ratio_setting"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

func validUserInfo(username string, role int) bool {
	// check username is empty
	if strings.TrimSpace(username) == "" {
		return false
	}
	if !common.IsValidateRole(role) {
		return false
	}
	return true
}

func authHelper(c *gin.Context, minRole int) {
	session := sessions.Default(c)
	username := session.Get("username")
	role := session.Get("role")
	id := session.Get("id")
	status := session.Get("status")
	useAccessToken := false
	if username == nil {
		// Check access token
		accessToken := c.Request.Header.Get("Authorization")
		if accessToken == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"message": "无权进行此操作，未登录且未提供 access token",
			})
			c.Abort()
			return
		}

		// Try lurus-platform session token first (HS256, zero network overhead).
		bearerToken := accessToken
		if strings.HasPrefix(bearerToken, "Bearer ") || strings.HasPrefix(bearerToken, "bearer ") {
			bearerToken = strings.TrimSpace(bearerToken[7:])
		}
		if accountID, err := common.ValidateIdentitySessionToken(bearerToken); err == nil && accountID > 0 {
			// Identity session token validated — resolve to local user via identity account lookup.
			idMapping, _ := common.GetAccountByZitadelSub_ByAccountID(c.Request.Context(), accountID)
			if idMapping != nil && idMapping.ZitadelSub != "" {
				user, _, userErr := repo.GetUserByZitadelID(idMapping.ZitadelSub, "default")
				if userErr == nil && user != nil {
					username = user.Username
					role = user.Role
					id = user.Id
					status = user.Status
					useAccessToken = true
					// Carry identity account ID for wallet bridging.
					c.Set("identity_account_id", accountID)
				}
			}
		}

		// Fall back to lurus-api access token if identity session token didn't match.
		if username == nil {
			user := repo.ValidateAccessToken(accessToken)
			if user != nil && user.Username != "" {
				if !validUserInfo(user.Username, user.Role) {
					c.JSON(http.StatusOK, gin.H{
						"success": false,
						"message": "无权进行此操作，用户信息无效",
					})
					c.Abort()
					return
				}
				// Token is valid
				username = user.Username
				role = user.Role
				id = user.Id
				status = user.Status
				useAccessToken = true
			} else {
				c.JSON(http.StatusOK, gin.H{
					"success": false,
					"message": "无权进行此操作，access token 无效",
				})
				c.Abort()
				return
			}
		}
	}
	// lurus-api-User header is optional. When provided, it must match the authenticated session
	// to prevent header-spoofing mismatches. The authoritative user ID always comes from the
	// verified session or access token - never from the header alone.
	apiUserIdStr := c.Request.Header.Get("lurus-api-User")
	if apiUserIdStr == "" {
		apiUserIdStr = c.Request.Header.Get("New-Api-User")
	}
	if apiUserIdStr != "" {
		apiUserId, err := strconv.Atoi(apiUserIdStr)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"message": "无权进行此操作，用户 ID 格式错误",
			})
			c.Abort()
			return
		}
		if id != apiUserId {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"message": "无权进行此操作，用户 ID 与登录用户不匹配",
			})
			c.Abort()
			return
		}
	}
	statusVal, ok := status.(int)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "invalid session data",
		})
		c.Abort()
		return
	}
	if statusVal == common.UserStatusDisabled {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "用户已被封禁",
		})
		c.Abort()
		return
	}
	roleVal, ok := role.(int)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "invalid session data",
		})
		c.Abort()
		return
	}
	if roleVal < minRole {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "无权进行此操作，权限不足",
		})
		c.Abort()
		return
	}
	usernameVal, ok := username.(string)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "invalid session data",
		})
		c.Abort()
		return
	}
	if !validUserInfo(usernameVal, roleVal) {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "无权进行此操作，用户信息无效",
		})
		c.Abort()
		return
	}
	c.Set("username", username)
	c.Set("role", role)
	c.Set("id", id)
	c.Set("group", session.Get("group"))
	c.Set("user_group", session.Get("group"))
	c.Set("use_access_token", useAccessToken)

	// Propagate user_id to context.Context for structured log correlation.
	c.Request = c.Request.WithContext(common.WithUserID(c.Request.Context(), fmt.Sprintf("%v", id)))

	// Inject tenant context for v1 API tenant isolation
	userId, ok := id.(int)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "invalid session data: user ID is not an integer",
		})
		c.Abort()
		return
	}
	tenantId := "default"
	if userCache, cacheErr := repo.GetUserCache(userId); cacheErr == nil && userCache.TenantId != "" {
		tenantId = userCache.TenantId
	}
	repo.InjectTenantContext(c, tenantId, userId)

	// Also set the structured TenantContext so v2 handlers (which call
	// GetTenantContext) work for users authenticated via session/access-token,
	// not only via Zitadel JWT.
	emailVal, _ := c.Get("email")
	email, _ := emailVal.(string)
	c.Set("tenant_context", &TenantContext{
		TenantID:  tenantId,
		UserID:    userId,
		Email:     email,
		Username:  usernameVal,
		Roles:     []string{},
	})

	c.Next()
}

func TryUserAuth() func(c *gin.Context) {
	return func(c *gin.Context) {
		session := sessions.Default(c)
		id := session.Get("id")
		if id != nil {
			c.Set("id", id)
		}
		c.Next()
	}
}

func UserAuth() func(c *gin.Context) {
	return func(c *gin.Context) {
		authHelper(c, common.RoleCommonUser)
	}
}

func AdminAuth() func(c *gin.Context) {
	return func(c *gin.Context) {
		authHelper(c, common.RoleAdminUser)
	}
}

func RootAuth() func(c *gin.Context) {
	return func(c *gin.Context) {
		authHelper(c, common.RoleRootUser)
	}
}

func WssAuth(c *gin.Context) {

}

func TokenAuth() func(c *gin.Context) {
	return func(c *gin.Context) {
		// 先检测是否为ws
		if c.Request.Header.Get("Sec-WebSocket-Protocol") != "" {
			// Sec-WebSocket-Protocol: realtime, openai-insecure-api-key.sk-xxx, openai-beta.realtime-v1
			// read sk from Sec-WebSocket-Protocol
			key := c.Request.Header.Get("Sec-WebSocket-Protocol")
			parts := strings.Split(key, ",")
			for _, part := range parts {
				part = strings.TrimSpace(part)
				if strings.HasPrefix(part, "openai-insecure-api-key") {
					key = strings.TrimPrefix(part, "openai-insecure-api-key.")
					break
				}
			}
			c.Request.Header.Set("Authorization", "Bearer "+key)
		}
		// 检查path包含/v1/messages 或 /v1/models 
		if strings.Contains(c.Request.URL.Path, "/v1/messages") || strings.Contains(c.Request.URL.Path, "/v1/models") {
			anthropicKey := c.Request.Header.Get("x-api-key")
			if anthropicKey != "" {
				c.Request.Header.Set("Authorization", "Bearer "+anthropicKey)
			}
		}
		// gemini api 从query中获取key
		if strings.HasPrefix(c.Request.URL.Path, "/v1beta/models") ||
			strings.HasPrefix(c.Request.URL.Path, "/v1beta/openai/models") ||
			strings.HasPrefix(c.Request.URL.Path, "/v1/models/") {
			skKey := c.Query("key")
			if skKey != "" {
				c.Request.Header.Set("Authorization", "Bearer "+skKey)
			}
			// 从x-goog-api-key header中获取key
			xGoogKey := c.Request.Header.Get("x-goog-api-key")
			if xGoogKey != "" {
				c.Request.Header.Set("Authorization", "Bearer "+xGoogKey)
			}
		}
		key := c.Request.Header.Get("Authorization")
		parts := make([]string, 0)
		if strings.HasPrefix(key, "Bearer ") || strings.HasPrefix(key, "bearer ") {
			key = strings.TrimSpace(key[7:])
		}
		if key == "" || key == "midjourney-proxy" {
			key = c.Request.Header.Get("mj-api-secret")
			if strings.HasPrefix(key, "Bearer ") || strings.HasPrefix(key, "bearer ") {
				key = strings.TrimSpace(key[7:])
			}
			key = strings.TrimPrefix(key, "sk-")
			parts = strings.Split(key, "-")
			key = parts[0]
		} else {
			key = strings.TrimPrefix(key, "sk-")
			parts = strings.Split(key, "-")
			key = parts[0]
		}
		token, err := repo.ValidateUserToken(key)
		if token != nil {
			id := c.GetInt("id")
			if id == 0 {
				c.Set("id", token.UserId)
			}
		}
		if err != nil {
			governance.RecordAuditEvent(governance.NewAuditEvent(c, governance.ActorToken, 0,
				governance.ActionAuthFailed, governance.ResourceToken, 0,
				fmt.Sprintf(`{"reason":"invalid_token"}`)))
			abortWithOpenAiMessage(c, http.StatusUnauthorized, err.Error())
			return
		}

		allowIps := token.GetIpLimits()
		if len(allowIps) > 0 {
			clientIp := c.ClientIP()
			logger.LogDebug(c, "Token has IP restrictions, checking client IP %s", clientIp)
			ip := net.ParseIP(clientIp)
			if ip == nil {
				abortWithOpenAiMessage(c, http.StatusForbidden, "无法解析客户端 IP 地址")
				return
			}
			if common.IsIpInCIDRList(ip, allowIps) == false {
				governance.RecordAuditEvent(governance.NewAuditEvent(c, governance.ActorToken, token.UserId,
					governance.ActionAuthIPRejected, governance.ResourceToken, token.Id,
					fmt.Sprintf(`{"ip":%q}`, clientIp)))
				abortWithOpenAiMessage(c, http.StatusForbidden, "您的 IP 不在令牌允许访问的列表中")
				return
			}
			logger.LogDebug(c, "Client IP %s passed the token IP restrictions check", clientIp)
		}

		userCache, err := repo.GetUserCache(token.UserId)
		if err != nil {
			abortWithOpenAiMessage(c, http.StatusInternalServerError, err.Error())
			return
		}
		userEnabled := userCache.Status == common.UserStatusEnabled
		if !userEnabled {
			abortWithOpenAiMessage(c, http.StatusForbidden, "用户已被封禁")
			return
		}

		repo.UserBaseWriteContext(userCache, c)

		// Propagate user_id to context.Context for structured log correlation.
		c.Request = c.Request.WithContext(common.WithUserID(c.Request.Context(), fmt.Sprintf("%d", token.UserId)))

		userGroup := userCache.Group
		tokenGroup := token.Group
		if tokenGroup != "" {
			// check common.UserUsableGroups[userGroup]
			if _, ok := app.GetUserUsableGroups(userGroup)[tokenGroup]; !ok {
				abortWithOpenAiMessage(c, http.StatusForbidden, fmt.Sprintf("无权访问 %s 分组", tokenGroup))
				return
			}
			// check group in common.GroupRatio
			if !ratio_setting.ContainsGroupRatio(tokenGroup) {
				if tokenGroup != "auto" {
					abortWithOpenAiMessage(c, http.StatusForbidden, fmt.Sprintf("分组 %s 已被弃用", tokenGroup))
					return
				}
			}
			userGroup = tokenGroup
		}
		common.SetContextKey(c, constant.ContextKeyUsingGroup, userGroup)

		err = SetupContextForToken(c, token, parts...)
		if err != nil {
			return
		}
		c.Next()
	}
}

func SetupContextForToken(c *gin.Context, token *repo.Token, parts ...string) error {
	if token == nil {
		return fmt.Errorf("token is nil")
	}
	c.Set("id", token.UserId)
	c.Set("token_id", token.Id)
	c.Set("token_key", token.Key)
	c.Set("token_name", token.Name)
	c.Set("token_unlimited_quota", token.UnlimitedQuota)
	if !token.UnlimitedQuota {
		c.Set("token_quota", token.RemainQuota)
	}
	if token.ModelLimitsEnabled {
		c.Set("token_model_limit_enabled", true)
		c.Set("token_model_limit", token.GetModelLimitsMap())
	} else {
		c.Set("token_model_limit_enabled", false)
	}
	common.SetContextKey(c, constant.ContextKeyTokenGroup, token.Group)
	common.SetContextKey(c, constant.ContextKeyTokenCrossGroupRetry, token.CrossGroupRetry)
	// Carry identity account ID from token for platform billing (if not already set by session auth).
	if token.IdentityAccountID > 0 {
		if _, exists := c.Get("identity_account_id"); !exists {
			c.Set("identity_account_id", token.IdentityAccountID)
		}
	}
	if len(parts) > 1 {
		if repo.IsAdmin(token.UserId) {
			c.Set("specific_channel_id", parts[1])
		} else {
			abortWithOpenAiMessage(c, http.StatusForbidden, "普通用户不支持指定渠道")
			return fmt.Errorf("普通用户不支持指定渠道")
		}
	}
	return nil
}

// PlaygroundAuth authenticates via session and auto-resolves the user's first
// active token. This lets the web UI playground work without requiring users
// to manually create and select API tokens.
// If a Bearer token IS provided, it falls through to standard TokenAuth behavior.
func PlaygroundAuth() func(c *gin.Context) {
	return func(c *gin.Context) {
		// If Bearer token provided, use standard TokenAuth flow.
		if authHeader := c.Request.Header.Get("Authorization"); authHeader != "" {
			TokenAuth()(c)
			return
		}

		// Session-based: authenticate user first.
		session := sessions.Default(c)
		id := session.Get("id")
		if id == nil {
			abortWithOpenAiMessage(c, http.StatusUnauthorized, "未登录，请先登录")
			return
		}
		userId, ok := id.(int)
		if !ok {
			abortWithOpenAiMessage(c, http.StatusUnauthorized, "会话无效")
			return
		}

		// Find user's first active token.
		tokens, err := repo.GetAllUserTokens(userId, 0, 1)
		if err != nil || len(tokens) == 0 {
			// Auto-create a default token for the user.
			token, createErr := repo.AutoCreateDefaultToken(userId)
			if createErr != nil {
				common.SysError(fmt.Sprintf("PlaygroundAuth: failed to auto-create token for user %d: %v", userId, createErr))
				abortWithOpenAiMessage(c, http.StatusInternalServerError, "无法创建默认令牌")
				return
			}
			tokens = []*repo.Token{token}
			common.SysLog(fmt.Sprintf("PlaygroundAuth: auto-created default token for user %d", userId))
		}

		token := tokens[0]
		if token.Status != common.TokenStatusEnabled {
			abortWithOpenAiMessage(c, http.StatusForbidden, "令牌已禁用，请到令牌管理中启用或创建新令牌")
			return
		}

		// Set up token context for relay.
		c.Set("id", token.UserId)
		if err := SetupContextForToken(c, token); err != nil {
			abortWithOpenAiMessage(c, http.StatusInternalServerError, "令牌上下文初始化失败")
			return
		}

		c.Next()
	}
}

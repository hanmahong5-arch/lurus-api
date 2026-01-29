package controller

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/QuantumNous/lurus-api/internal/pkg/common"
	"github.com/QuantumNous/lurus-api/internal/data/model"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

// OAuth token response from Zitadel
type OAuthTokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	IDToken      string `json:"id_token"` // JWT containing user claims
	Scope        string `json:"scope"`
}

// OAuthStateData contains state information for OAuth flow
type OAuthStateData struct {
	TenantSlug  string    `json:"tenant_slug"`
	RedirectURL string    `json:"redirect_url"`
	Nonce       string    `json:"nonce"`
	CreatedAt   time.Time `json:"created_at"`
}

// ZitadelLoginRedirect redirects user to Zitadel OAuth login page
// Route: GET /api/v2/:tenant_slug/auth/login
func ZitadelLoginRedirect(c *gin.Context) {
	tenantSlug := c.Param("tenant_slug")
	if tenantSlug == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "租户标识不能为空 / Tenant slug is required",
		})
		return
	}

	// Get tenant by slug
	tenant, err := model.GetTenantBySlug(tenantSlug)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "租户不存在 / Tenant not found",
		})
		return
	}

	// Check if tenant is enabled
	if !tenant.IsEnabled() {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": "租户已被禁用或暂停 / Tenant is disabled or suspended",
		})
		return
	}

	// Get redirect URL (where to redirect after login)
	redirectURL := c.Query("redirect_url")
	if redirectURL == "" {
		redirectURL = "/dashboard" // Default redirect
	}

	// Generate state parameter (contains tenant slug + redirect URL + nonce)
	state, err := generateOAuthState(tenantSlug, redirectURL)
	if err != nil {
		common.SysError(fmt.Sprintf("Failed to generate OAuth state: %v", err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Internal server error",
		})
		return
	}

	// Build Zitadel authorization URL
	authURL := buildZitadelAuthURL(tenant.ZitadelOrgID, state)

	// Redirect to Zitadel login page
	c.Redirect(http.StatusFound, authURL)
}

// ZitadelCallback handles OAuth callback from Zitadel
// Route: GET /api/v2/oauth/callback
func ZitadelCallback(c *gin.Context) {
	// Get authorization code and state from query params
	code := c.Query("code")
	state := c.Query("state")

	// Validate parameters
	if code == "" || state == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Missing code or state parameter",
		})
		return
	}

	// Parse and validate state
	stateData, err := parseOAuthState(state)
	if err != nil {
		common.SysError(fmt.Sprintf("Invalid OAuth state: %v", err))
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid state parameter",
		})
		return
	}

	// Check state expiration (5 minutes)
	if time.Since(stateData.CreatedAt) > 5*time.Minute {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "OAuth state expired, please try again",
		})
		return
	}

	// Exchange authorization code for tokens
	tokenResp, err := exchangeCodeForToken(code)
	if err != nil {
		common.SysError(fmt.Sprintf("Failed to exchange code for token: %v", err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to obtain access token",
		})
		return
	}

	// Parse ID token (JWT) to get user claims
	// Note: ID token verification is handled by Zitadel Auth middleware
	// For now, we'll trust the ID token since we obtained it directly from Zitadel

	// TODO: Parse and validate ID token
	// claims, err := parseIDToken(tokenResp.IDToken)

	// For now, we'll create a session and redirect
	// The actual user mapping will happen when they access protected routes with JWT

	// Create session (for v1 API compatibility)
	session := sessions.Default(c)
	session.Set("oauth_access_token", tokenResp.AccessToken)
	session.Set("oauth_refresh_token", tokenResp.RefreshToken)
	session.Set("oauth_id_token", tokenResp.IDToken)
	session.Set("tenant_slug", stateData.TenantSlug)
	session.Save()

	// Redirect to original URL
	redirectURL := stateData.RedirectURL
	if redirectURL == "" {
		redirectURL = "/dashboard"
	}

	// Add tenant slug to redirect URL
	if !strings.Contains(redirectURL, "tenant=") {
		separator := "?"
		if strings.Contains(redirectURL, "?") {
			separator = "&"
		}
		redirectURL = redirectURL + separator + "tenant=" + stateData.TenantSlug
	}

	c.Redirect(http.StatusFound, redirectURL)
}

// ZitadelLogout logs out user from Zitadel and clears session
// Route: POST /api/v2/oauth/logout
func ZitadelLogout(c *gin.Context) {
	// Get ID token from session
	session := sessions.Default(c)
	idToken, _ := session.Get("oauth_id_token").(string)

	// Clear session
	session.Clear()
	session.Save()

	// If ID token exists, redirect to Zitadel logout endpoint
	if idToken != "" {
		postLogoutRedirectURI := os.Getenv("ZITADEL_POST_LOGOUT_REDIRECT_URI")
		if postLogoutRedirectURI == "" {
			postLogoutRedirectURI = "/login"
		}

		logoutURL := fmt.Sprintf(
			"%s/oidc/v1/end_session?id_token_hint=%s&post_logout_redirect_uri=%s",
			os.Getenv("ZITADEL_ISSUER"),
			url.QueryEscape(idToken),
			url.QueryEscape(postLogoutRedirectURI),
		)

		c.Redirect(http.StatusFound, logoutURL)
		return
	}

	// No ID token, just return success
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Logged out successfully",
	})
}

// RefreshAccessToken refreshes the access token using refresh token
// Route: POST /api/v2/oauth/refresh
func RefreshAccessToken(c *gin.Context) {
	// Get refresh token from session or request body
	session := sessions.Default(c)
	refreshToken, _ := session.Get("oauth_refresh_token").(string)

	if refreshToken == "" {
		var req struct {
			RefreshToken string `json:"refresh_token"`
		}
		if err := c.ShouldBindJSON(&req); err == nil {
			refreshToken = req.RefreshToken
		}
	}

	if refreshToken == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Refresh token is required",
		})
		return
	}

	// Exchange refresh token for new access token
	tokenResp, err := refreshAccessToken(refreshToken)
	if err != nil {
		common.SysError(fmt.Sprintf("Failed to refresh access token: %v", err))
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "Failed to refresh access token",
		})
		return
	}

	// Update session
	session.Set("oauth_access_token", tokenResp.AccessToken)
	if tokenResp.RefreshToken != "" {
		session.Set("oauth_refresh_token", tokenResp.RefreshToken)
	}
	session.Save()

	// Return new tokens
	c.JSON(http.StatusOK, gin.H{
		"success":       true,
		"access_token":  tokenResp.AccessToken,
		"refresh_token": tokenResp.RefreshToken,
		"expires_in":    tokenResp.ExpiresIn,
	})
}

// ============================================================================
// Helper functions
// ============================================================================

// generateOAuthState generates a state parameter for OAuth flow
func generateOAuthState(tenantSlug string, redirectURL string) (string, error) {
	// Generate random nonce
	nonceBytes := make([]byte, 16)
	if _, err := rand.Read(nonceBytes); err != nil {
		return "", err
	}
	nonce := base64.URLEncoding.EncodeToString(nonceBytes)

	// Create state data
	stateData := OAuthStateData{
		TenantSlug:  tenantSlug,
		RedirectURL: redirectURL,
		Nonce:       nonce,
		CreatedAt:   time.Now(),
	}

	// Serialize to JSON
	stateJSON, err := json.Marshal(stateData)
	if err != nil {
		return "", err
	}

	// Encode as base64
	state := base64.URLEncoding.EncodeToString(stateJSON)
	return state, nil
}

// parseOAuthState parses and validates the state parameter
func parseOAuthState(state string) (*OAuthStateData, error) {
	// Decode base64
	stateJSON, err := base64.URLEncoding.DecodeString(state)
	if err != nil {
		return nil, fmt.Errorf("invalid base64 encoding: %w", err)
	}

	// Parse JSON
	var stateData OAuthStateData
	if err := json.Unmarshal(stateJSON, &stateData); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	return &stateData, nil
}

// buildZitadelAuthURL builds the Zitadel authorization URL
func buildZitadelAuthURL(orgID string, state string) string {
	issuer := os.Getenv("ZITADEL_ISSUER")
	clientID := os.Getenv("ZITADEL_CLIENT_ID")
	redirectURI := os.Getenv("ZITADEL_REDIRECT_URI")
	scopes := os.Getenv("ZITADEL_OAUTH_SCOPES")
	if scopes == "" {
		scopes = "openid email profile offline_access"
	}

	// Build authorization URL
	authURL := fmt.Sprintf(
		"%s/oauth/v2/authorize?client_id=%s&redirect_uri=%s&response_type=code&scope=%s&state=%s",
		issuer,
		url.QueryEscape(clientID),
		url.QueryEscape(redirectURI),
		url.QueryEscape(scopes),
		url.QueryEscape(state),
	)

	// Add organization hint if provided
	if orgID != "" {
		authURL += "&organization=" + url.QueryEscape(orgID)
	}

	// Add PKCE if enabled
	if os.Getenv("ZITADEL_ENABLE_PKCE") == "true" {
		// TODO: Implement PKCE
		// For now, PKCE is optional
	}

	return authURL
}

// exchangeCodeForToken exchanges authorization code for access token
func exchangeCodeForToken(code string) (*OAuthTokenResponse, error) {
	issuer := os.Getenv("ZITADEL_ISSUER")
	clientID := os.Getenv("ZITADEL_CLIENT_ID")
	clientSecret := os.Getenv("ZITADEL_CLIENT_SECRET")
	redirectURI := os.Getenv("ZITADEL_REDIRECT_URI")

	// Build token request
	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("code", code)
	data.Set("client_id", clientID)
	data.Set("client_secret", clientSecret)
	data.Set("redirect_uri", redirectURI)

	// Send POST request to token endpoint
	resp, err := http.PostForm(issuer+"/oauth/v2/token", data)
	if err != nil {
		return nil, fmt.Errorf("failed to post token request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("token endpoint returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var tokenResp OAuthTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("failed to decode token response: %w", err)
	}

	return &tokenResp, nil
}

// refreshAccessToken refreshes the access token using refresh token
func refreshAccessToken(refreshToken string) (*OAuthTokenResponse, error) {
	issuer := os.Getenv("ZITADEL_ISSUER")
	clientID := os.Getenv("ZITADEL_CLIENT_ID")
	clientSecret := os.Getenv("ZITADEL_CLIENT_SECRET")

	// Build token request
	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("refresh_token", refreshToken)
	data.Set("client_id", clientID)
	data.Set("client_secret", clientSecret)

	// Send POST request to token endpoint
	resp, err := http.PostForm(issuer+"/oauth/v2/token", data)
	if err != nil {
		return nil, fmt.Errorf("failed to post token request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("token endpoint returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var tokenResp OAuthTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("failed to decode token response: %w", err)
	}

	return &tokenResp, nil
}

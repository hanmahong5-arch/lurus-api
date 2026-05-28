package middleware

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/LurusTech/lurus-hub/internal/adapter/repo"
	"github.com/LurusTech/lurus-hub/internal/pkg/common"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// ZitadelClaims represents the JWT claims issued by Zitadel
// Includes both standard OIDC claims and Zitadel-specific claims
type ZitadelClaims struct {
	jwt.RegisteredClaims
	Email             string                 `json:"email"`
	EmailVerified     bool                   `json:"email_verified"`
	Name              string                 `json:"name"`
	PreferredUsername string                 `json:"preferred_username"`
	OrgID             string                 `json:"urn:zitadel:iam:org:id"`
	OrgDomain         string                 `json:"urn:zitadel:iam:org:domain:primary"`
	ResourceOwnerID   string                 `json:"urn:zitadel:iam:user:resourceowner:id"`
	ResourceOwnerName string                 `json:"urn:zitadel:iam:user:resourceowner:name"`
	Roles             map[string]interface{} `json:"urn:zitadel:iam:org:project:roles"`
}

// TenantContext represents the tenant context injected into Gin context
type TenantContext struct {
	TenantID      string   `json:"tenant_id"`       // Tenant ID
	UserID        int      `json:"user_id"`         // Lurus user ID
	ZitadelUserID string   `json:"zitadel_user_id"` // Zitadel user ID
	Email         string   `json:"email"`           // User email
	Username      string   `json:"username"`        // Username
	Roles         []string `json:"roles"`           // User roles in this tenant
}

// JWK represents a JSON Web Key
type JWK struct {
	Kty string `json:"kty"` // Key Type
	Use string `json:"use"` // Public Key Use
	Kid string `json:"kid"` // Key ID
	Alg string `json:"alg"` // Algorithm
	N   string `json:"n"`   // Modulus (for RSA)
	E   string `json:"e"`   // Exponent (for RSA)
}

// JWKSet represents a set of JSON Web Keys
type JWKSet struct {
	Keys []JWK `json:"keys"`
}

// JWKSManager manages JSON Web Keys from Zitadel JWKS endpoint
// Automatically refreshes keys periodically
type JWKSManager struct {
	jwksURI       string
	publicKeys    map[string]*rsa.PublicKey
	mu            sync.RWMutex
	lastUpdate    time.Time
	updateFailed  bool
	refreshTicker *time.Ticker
	// refreshing prevents concurrent refresh attempts (race condition fix)
	refreshing    bool
	refreshMu     sync.Mutex
	// minRefreshInterval prevents too frequent refreshes on key not found
	minRefreshInterval time.Duration
}

// zitadelHTTPClient is the HTTP client used for JWKS fetching.
// Using a dedicated client with timeout prevents indefinite hangs on network issues.
var zitadelHTTPClient = &http.Client{
	Timeout: 15 * time.Second,
}

var (
	jwksManager        *JWKSManager
	jwksManagerOnce    sync.Once
	zitadelIssuer      string
	zitadelJwksURI     string
	zitadelClientID    string
	zitadelEnabled     bool
	jwksRefreshInterval time.Duration = 1 * time.Hour
)

// InitZitadelAuth initializes Zitadel authentication system
// Must be called during application startup
func InitZitadelAuth() error {
	// Load Zitadel configuration from environment variables
	zitadelEnabled = os.Getenv("ZITADEL_ENABLED") == "true"
	if !zitadelEnabled {
		common.SysLog("Zitadel authentication is disabled")
		return nil
	}

	zitadelIssuer = os.Getenv("ZITADEL_ISSUER")
	zitadelJwksURI = os.Getenv("ZITADEL_JWKS_URI")
	zitadelClientID = os.Getenv("ZITADEL_CLIENT_ID")

	// Validate required environment variables
	if zitadelIssuer == "" {
		return errors.New("ZITADEL_ISSUER is not set")
	}
	if zitadelJwksURI == "" {
		return errors.New("ZITADEL_JWKS_URI is not set")
	}
	if zitadelClientID == "" {
		return errors.New("ZITADEL_CLIENT_ID is not set")
	}

	// Initialize JWKS Manager
	jwksManagerOnce.Do(func() {
		jwksManager = NewJWKSManager(zitadelJwksURI)
	})

	common.SysLog("Zitadel authentication initialized successfully")
	common.SysLog(fmt.Sprintf("Zitadel Issuer: %s", zitadelIssuer))
	common.SysLog(fmt.Sprintf("Zitadel JWKS URI: %s", zitadelJwksURI))

	return nil
}

// NewJWKSManager creates a new JWKS Manager instance.
// Deprecated: Use NewJWKSManagerWithContext instead.
func NewJWKSManager(jwksURI string) *JWKSManager {
	return NewJWKSManagerWithContext(context.Background(), jwksURI)
}

// NewJWKSManagerWithContext creates a new JWKS Manager instance with context support.
// The background refresh goroutine exits when ctx is cancelled.
func NewJWKSManagerWithContext(ctx context.Context, jwksURI string) *JWKSManager {
	m := &JWKSManager{
		jwksURI:            jwksURI,
		publicKeys:         make(map[string]*rsa.PublicKey),
		minRefreshInterval: 30 * time.Second, // Prevent refresh more than once per 30 seconds
	}

	// Initial key fetch
	err := m.refreshKeys()
	if err != nil {
		common.SysError(fmt.Sprintf("Failed to fetch JWKS keys: %v", err))
		m.updateFailed = true
	}

	// Start background refresh with context
	common.SafeGoWithContext(ctx, func(c context.Context) {
		m.autoRefreshWithContext(c)
	})

	return m
}

// refreshKeys fetches public keys from Zitadel JWKS endpoint
func (m *JWKSManager) refreshKeys() error {
	common.SysLog(fmt.Sprintf("Fetching JWKS from: %s", m.jwksURI))

	// Fetch JWKS from Zitadel
	resp, err := zitadelHTTPClient.Get(m.jwksURI)
	if err != nil {
		return fmt.Errorf("failed to fetch JWKS: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("JWKS endpoint returned status %d", resp.StatusCode)
	}

	// Parse JWKS response
	var jwkSet JWKSet
	err = json.NewDecoder(resp.Body).Decode(&jwkSet)
	if err != nil {
		return fmt.Errorf("failed to decode JWKS: %w", err)
	}

	// Convert JWKs to RSA public keys
	newKeys := make(map[string]*rsa.PublicKey)
	for _, jwk := range jwkSet.Keys {
		if jwk.Kty != "RSA" {
			continue // Only support RSA keys
		}

		publicKey, err := jwkToRSAPublicKey(jwk)
		if err != nil {
			common.SysError(fmt.Sprintf("Failed to convert JWK to RSA public key (kid=%s): %v", jwk.Kid, err))
			continue
		}

		newKeys[jwk.Kid] = publicKey
	}

	if len(newKeys) == 0 {
		return errors.New("no valid RSA keys found in JWKS")
	}

	// Update keys (thread-safe)
	m.mu.Lock()
	m.publicKeys = newKeys
	m.lastUpdate = time.Now()
	m.updateFailed = false
	m.mu.Unlock()

	common.SysLog(fmt.Sprintf("Successfully refreshed %d JWKS keys", len(newKeys)))

	return nil
}

// autoRefreshWithContext periodically refreshes JWKS keys with context support
func (m *JWKSManager) autoRefreshWithContext(ctx context.Context) {
	m.refreshTicker = time.NewTicker(jwksRefreshInterval)
	defer m.refreshTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			common.SysLog("JWKS auto-refresh stopped")
			return
		case <-m.refreshTicker.C:
			err := m.refreshKeys()
			if err != nil {
				common.SysError(fmt.Sprintf("Auto-refresh JWKS failed: %v", err))
			}
		}
	}
}

// autoRefresh periodically refreshes JWKS keys
// Deprecated: Use autoRefreshWithContext instead.
func (m *JWKSManager) autoRefresh() {
	m.autoRefreshWithContext(context.Background())
}

// getKey retrieves an RSA public key by Key ID (kid)
func (m *JWKSManager) getKey(kid string) (*rsa.PublicKey, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if key, ok := m.publicKeys[kid]; ok {
		return key, nil
	}

	return nil, fmt.Errorf("public key not found for kid: %s", kid)
}

// tryRefreshKeys attempts to refresh keys with protection against concurrent refreshes
// Returns true if refresh was performed (or already in progress), false if skipped due to rate limiting
func (m *JWKSManager) tryRefreshKeys() (refreshed bool, err error) {
	m.refreshMu.Lock()

	// Check if already refreshing
	if m.refreshing {
		m.refreshMu.Unlock()
		// Wait a bit for ongoing refresh to complete
		time.Sleep(100 * time.Millisecond)
		return true, nil // Assume ongoing refresh will succeed
	}

	// Check if refreshed too recently (rate limiting)
	m.mu.RLock()
	lastUpdate := m.lastUpdate
	m.mu.RUnlock()

	if time.Since(lastUpdate) < m.minRefreshInterval {
		m.refreshMu.Unlock()
		return false, nil // Skipped due to rate limiting
	}

	// Mark as refreshing
	m.refreshing = true
	m.refreshMu.Unlock()

	// Ensure we clear the refreshing flag when done
	defer func() {
		m.refreshMu.Lock()
		m.refreshing = false
		m.refreshMu.Unlock()
	}()

	// Perform the actual refresh
	err = m.refreshKeys()
	return true, err
}

// getKeyWithRefresh retrieves a key, attempting to refresh if not found
// This method safely handles concurrent refresh attempts
func (m *JWKSManager) getKeyWithRefresh(kid string) (*rsa.PublicKey, error) {
	// First attempt - try to get key
	key, err := m.getKey(kid)
	if err == nil {
		return key, nil
	}

	// Key not found, try to refresh
	refreshed, refreshErr := m.tryRefreshKeys()
	if refreshErr != nil {
		common.SysError(fmt.Sprintf("Failed to refresh JWKS: %v", refreshErr))
		// Return original error if refresh failed
		return nil, err
	}

	if !refreshed {
		// Rate limited, return original error
		return nil, fmt.Errorf("public key not found for kid: %s (refresh rate limited)", kid)
	}

	// Second attempt - try to get key after refresh
	return m.getKey(kid)
}

// VerifyIDTokenWithJWKS verifies an ID token JWT signature using the JWKS keys.
// This is exported so the OAuth handler can verify ID tokens received from the
// token endpoint, preventing token forgery even if TLS is compromised.
// Returns the parsed jwt.Token with verified claims, or an error.
func VerifyIDTokenWithJWKS(idToken string, claims jwt.Claims) (*jwt.Token, error) {
	if jwksManager == nil {
		return nil, errors.New("JWKS manager not initialized; is Zitadel enabled?")
	}

	token, err := jwt.ParseWithClaims(idToken, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		kid, ok := token.Header["kid"].(string)
		if !ok {
			return nil, errors.New("missing kid in token header")
		}

		return jwksManager.getKeyWithRefresh(kid)
	})
	if err != nil {
		return nil, fmt.Errorf("ID token signature verification failed: %w", err)
	}
	if !token.Valid {
		return nil, errors.New("ID token is not valid")
	}
	return token, nil
}

// jwkToRSAPublicKey converts a JWK to RSA public key
func jwkToRSAPublicKey(jwk JWK) (*rsa.PublicKey, error) {
	// Decode modulus (n)
	nBytes, err := base64.RawURLEncoding.DecodeString(jwk.N)
	if err != nil {
		return nil, fmt.Errorf("failed to decode modulus: %w", err)
	}

	// Decode exponent (e)
	eBytes, err := base64.RawURLEncoding.DecodeString(jwk.E)
	if err != nil {
		return nil, fmt.Errorf("failed to decode exponent: %w", err)
	}

	// Convert to big.Int
	n := new(big.Int).SetBytes(nBytes)
	e := new(big.Int).SetBytes(eBytes)

	// Create RSA public key
	publicKey := &rsa.PublicKey{
		N: n,
		E: int(e.Int64()),
	}

	return publicKey, nil
}

// handleSessionFallback checks for valid session-based OAuth data when no Bearer token is present.
// Returns true if it handled the request (either successfully authenticated via session, or aborted).
// Returns false if no session auth data is available and caller should try other methods.
func handleSessionFallback(c *gin.Context) bool {
	// Check if session middleware is available (prevents panic when session store not configured)
	if _, exists := c.Get(sessions.DefaultKey); !exists {
		return false
	}

	session := sessions.Default(c)

	// Check for session user ID (set during OAuth callback)
	sessionID := session.Get("id")
	if sessionID == nil {
		return false // No session, let caller handle
	}

	userID, ok := sessionID.(int)
	if !ok || userID == 0 {
		return false
	}

	// Check if session has OAuth access token
	accessToken, _ := session.Get("oauth_access_token").(string)
	expiresAt, _ := session.Get("oauth_token_expires_at").(int64)

	// If access token exists and is not expired, use it to validate
	if accessToken != "" && expiresAt > time.Now().Unix() {
		// Valid session with non-expired OAuth token
		// Build tenant context from session data
		user, err := repo.GetUserById(userID, false)
		if err != nil {
			common.SysError(fmt.Sprintf("Session fallback: failed to get user %d: %v", userID, err))
			return false
		}

		// SECURITY (cross-tenant isolation): scope the tenant to the
		// authenticated user's OWN record — never the URL :tenant_slug.
		// Deriving it from the slug let any session-authenticated user
		// assume another tenant's context just by changing the path; worse,
		// downstream guards compare tenantCtx.TenantID against that same
		// slug (e.g. GetCreditPoolForEndUser's TENANT_MISMATCH check), so
		// the slug was being compared to itself and always passed. Mirrors
		// authHelper (auth.go), which scopes tenant via the user's TenantId.
		tenantID := user.TenantId
		if tenantID == "" {
			tenantID = "default"
		}

		// Construct tenant context from session
		tenantCtx := &TenantContext{
			TenantID:      tenantID,
			UserID:        user.Id,
			ZitadelUserID: "", // Not available in session
			Email:         user.Email,
			Username:      user.Username,
			Roles:         []string{},
		}

		// Inject into gin context
		c.Set("tenant_context", tenantCtx)
		c.Set("tenant_id", tenantID)
		c.Set("user_id", user.Id)
		c.Set("id", user.Id)

		c.Next()
		return true
	}

	// Session exists but token expired or missing — try constructing from session data directly
	if userID > 0 {
		user, err := repo.GetUserById(userID, false)
		if err != nil {
			return false
		}

		// SECURITY (cross-tenant isolation): see the non-expired branch
		// above — tenant comes from the user's own record, not the URL
		// :tenant_slug, so a session user cannot read another tenant.
		tenantID := user.TenantId
		if tenantID == "" {
			tenantID = "default"
		}

		tenantCtx := &TenantContext{
			TenantID:      tenantID,
			UserID:        user.Id,
			ZitadelUserID: "",
			Email:         user.Email,
			Username:      user.Username,
			Roles:         []string{},
		}

		c.Set("tenant_context", tenantCtx)
		c.Set("tenant_id", tenantID)
		c.Set("user_id", user.Id)
		c.Set("id", user.Id)

		c.Next()
		return true
	}

	return false
}

// ZitadelAuth is the Gin middleware for Zitadel JWT authentication
// Validates JWT tokens issued by Zitadel and injects tenant context
func ZitadelAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check if Zitadel is enabled
		if !zitadelEnabled {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"success": false,
				"message": "Zitadel authentication is not enabled",
			})
			c.Abort()
			return
		}

		// Extract Bearer token from Authorization header
		authHeader := c.GetHeader("Authorization")
		tokenString := ""

		if authHeader != "" {
			// Remove "Bearer " prefix
			tokenString = strings.TrimPrefix(authHeader, "Bearer ")
			tokenString = strings.TrimPrefix(tokenString, "bearer ")
			if tokenString == authHeader {
				tokenString = "" // No valid Bearer prefix found
			}
		}

		// Session fallback: when no Bearer token, check session for OAuth data
		if tokenString == "" {
			if handled := handleSessionFallback(c); handled {
				return
			}
			// Session fallback failed — no auth available
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"message": "Authentication required: provide Bearer token or establish a session via OAuth login",
			})
			c.Abort()
			return
		}

		// Parse token to get Key ID (kid) from header
		token, err := jwt.ParseWithClaims(tokenString, &ZitadelClaims{}, func(token *jwt.Token) (interface{}, error) {
			// Validate signing method
			if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}

			// Get Key ID from token header
			kid, ok := token.Header["kid"].(string)
			if !ok {
				return nil, errors.New("missing kid in token header")
			}

			// Get public key from JWKS Manager with safe refresh on key not found
			// Uses getKeyWithRefresh to prevent concurrent refresh race conditions
			publicKey, err := jwksManager.getKeyWithRefresh(kid)
			if err != nil {
				return nil, err
			}

			return publicKey, nil
		})

		if err != nil || !token.Valid {
			common.SysLog(fmt.Sprintf("JWT validation failed: %v", err))
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"message": "Token 无效或已过期 / Invalid or expired token",
			})
			c.Abort()
			return
		}

		// Extract claims
		claims, ok := token.Claims.(*ZitadelClaims)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"message": "Invalid token claims",
			})
			c.Abort()
			return
		}

		// Verify issuer
		if claims.Issuer != zitadelIssuer {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"message": fmt.Sprintf("Invalid issuer, expected: %s, got: %s", zitadelIssuer, claims.Issuer),
			})
			c.Abort()
			return
		}

		// Verify audience (optional, can include client ID validation)
		// Note: Zitadel may use project ID as audience

		// Map Zitadel user to lurus user and tenant
		tenantID, lurusUserID, err := mapZitadelUserToLurus(claims)
		if err != nil {
			common.SysError(fmt.Sprintf("User mapping failed: %v", err))
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "用户身份映射失败 / User identity mapping failed",
			})
			c.Abort()
			return
		}

		// Sync account to lurus-platform (upsert) and carry identity_account_id
		// for wallet bridging. Best-effort: platform unavailability does not block auth.
		if im, _ := common.UpsertAccountGRPC(c.Request.Context(), claims.Subject, claims.Email, claims.Name, ""); im != nil {
			c.Set("identity_account_id", im.ID)
		}

		// Extract roles from claims
		roles := extractRoles(claims.Roles)

		// Create tenant context
		tenantCtx := &TenantContext{
			TenantID:      tenantID,
			UserID:        lurusUserID,
			ZitadelUserID: claims.Subject,
			Email:         claims.Email,
			Username:      claims.PreferredUsername,
			Roles:         roles,
		}

		// Inject tenant context into Gin context
		c.Set("tenant_context", tenantCtx)
		c.Set("tenant_id", tenantID)
		c.Set("user_id", lurusUserID)
		c.Set("zitadel_user_id", claims.Subject)

		// Log successful authentication (debug mode)
		if os.Getenv("ZITADEL_DEBUG_LOGGING") == "true" {
			common.SysLog(fmt.Sprintf("User authenticated: tenant=%s, user=%d, email=%s, roles=%v",
				tenantID, lurusUserID, claims.Email, roles))
		}

		c.Next()
	}
}

// mapZitadelUserToLurus maps Zitadel user to lurus user and tenant
// Auto-creates tenant and user if they don't exist
func mapZitadelUserToLurus(claims *ZitadelClaims) (tenantID string, lurusUserID int, err error) {
	// Step 1: Map tenant (Zitadel Org ID → lurus Tenant ID)
	tenant, err := repo.GetTenantByZitadelOrgID(claims.OrgID)
	if err != nil {
		// Tenant doesn't exist, auto-create if enabled
		if os.Getenv("ZITADEL_AUTO_CREATE_TENANT") == "true" {
			tenant, err = repo.CreateTenantFromZitadel(claims.OrgID, claims.OrgDomain, claims.ResourceOwnerName)
			if err != nil {
				return "", 0, fmt.Errorf("failed to create tenant: %w", err)
			}
			common.SysLog(fmt.Sprintf("Auto-created tenant: id=%s, org_id=%s, name=%s",
				tenant.Id, tenant.ZitadelOrgID, tenant.Name))

			// Initialize default configs for new tenant
			err = repo.InitializeDefaultTenantConfigs(tenant.Id)
			if err != nil {
				common.SysError(fmt.Sprintf("Failed to initialize tenant configs: %v", err))
			}
		} else {
			return "", 0, fmt.Errorf("tenant not found for Zitadel Org ID: %s", claims.OrgID)
		}
	}

	tenantID = tenant.Id

	// Check if tenant is enabled
	if !tenant.IsEnabled() {
		return "", 0, fmt.Errorf("tenant is disabled or suspended: %s", tenantID)
	}

	// Step 2: Map user (Zitadel User ID → lurus User ID)
	mapping, err := repo.GetUserMappingByZitadelID(claims.Subject, tenantID)
	if err != nil {
		// User mapping doesn't exist, auto-create if enabled
		if os.Getenv("ZITADEL_AUTO_CREATE_USER") == "true" {
			// Convert claims to model struct
			userClaims := &repo.ZitadelUserClaims{
				Sub:               claims.Subject,
				Email:             claims.Email,
				EmailVerified:     claims.EmailVerified,
				Name:              claims.Name,
				PreferredUsername: claims.PreferredUsername,
				OrgID:             claims.OrgID,
				OrgDomain:         claims.OrgDomain,
			}

			// Create user and mapping
			user, _, err := repo.CreateUserFromZitadelClaims(userClaims, tenantID)
			if err != nil {
				return "", 0, fmt.Errorf("failed to create user: %w", err)
			}

			common.SysLog(fmt.Sprintf("Auto-created user: tenant=%s, lurus_user=%d, zitadel_user=%s, email=%s",
				tenantID, user.Id, claims.Subject, claims.Email))

			return tenantID, user.Id, nil
		} else {
			return "", 0, fmt.Errorf("user mapping not found for Zitadel User ID: %s", claims.Subject)
		}
	}

	// Sync user data from Zitadel (update email, display name, etc.)
	err = repo.SyncUserDataFromZitadel(mapping.Id, claims.Email, claims.Name, claims.PreferredUsername)
	if err != nil {
		common.SysError(fmt.Sprintf("Failed to sync user data: %v", err))
		// Non-fatal error, continue
	}

	return tenantID, mapping.LurusUserID, nil
}

// extractRoles extracts role names from Zitadel roles claim
func extractRoles(rolesMap map[string]interface{}) []string {
	var roles []string
	for role := range rolesMap {
		roles = append(roles, role)
	}
	return roles
}

// GetTenantContext retrieves tenant context from Gin context
func GetTenantContext(c *gin.Context) (*TenantContext, error) {
	ctx, exists := c.Get("tenant_context")
	if !exists {
		return nil, errors.New("tenant context not found")
	}

	tenantCtx, ok := ctx.(*TenantContext)
	if !ok {
		return nil, errors.New("invalid tenant context type")
	}

	return tenantCtx, nil
}

// RequireRole middleware checks if user has a specific role
func RequireRole(requiredRole string) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantCtx, err := GetTenantContext(c)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"message": "Tenant context not found",
			})
			c.Abort()
			return
		}

		// Check if user has required role
		hasRole := false
		for _, role := range tenantCtx.Roles {
			if role == requiredRole {
				hasRole = true
				break
			}
		}

		if !hasRole {
			c.JSON(http.StatusForbidden, gin.H{
				"success": false,
				"message": fmt.Sprintf("Insufficient permissions, required role: %s", requiredRole),
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// RequireAnyRole middleware checks if user has any of the specified roles
func RequireAnyRole(requiredRoles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantCtx, err := GetTenantContext(c)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"message": "Tenant context not found",
			})
			c.Abort()
			return
		}

		// Check if user has any of the required roles
		hasRole := false
		for _, userRole := range tenantCtx.Roles {
			for _, requiredRole := range requiredRoles {
				if userRole == requiredRole {
					hasRole = true
					break
				}
			}
			if hasRole {
				break
			}
		}

		if !hasRole {
			c.JSON(http.StatusForbidden, gin.H{
				"success": false,
				"message": fmt.Sprintf("Insufficient permissions, required roles: %v", requiredRoles),
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

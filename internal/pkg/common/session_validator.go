package common

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// IdentitySessionSecret is the shared HS256 secret for validating lurus-platform session tokens.
// Must match the SESSION_SECRET env var in lurus-platform.
// Re-read post-.env by RefreshIdentityClientEnv (identity_client.go) — see that
// func's comment for why the package-init read alone is not sufficient for
// .env-only deployments.
var IdentitySessionSecret = os.Getenv("IDENTITY_SESSION_SECRET")

// validSessionIssuers lists accepted issuer values for platform session tokens.
// "lurus-platform" is the current issuer; "lurus-identity" is kept for backward compatibility
// with tokens issued before the rename.
var validSessionIssuers = map[string]bool{
	"lurus-platform": true,
	"lurus-identity": true,
}

// ValidateIdentitySessionToken parses and verifies a lurus-platform HS256 session token.
// Returns the lurus account ID embedded in the sub claim, or an error.
func ValidateIdentitySessionToken(tokenStr string) (int64, error) {
	if IdentitySessionSecret == "" {
		return 0, fmt.Errorf("session: identity session secret not configured")
	}

	parts := strings.Split(tokenStr, ".")
	if len(parts) != 3 {
		return 0, fmt.Errorf("session: malformed token: expected 3 parts")
	}

	// Verify HMAC-SHA256 signature before trusting any claims.
	body := parts[0] + "." + parts[1]
	mac := hmac.New(sha256.New, []byte(IdentitySessionSecret))
	mac.Write([]byte(body))
	expectedSig := mac.Sum(nil)

	gotSig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return 0, fmt.Errorf("session: decode signature: %w", err)
	}
	if !hmac.Equal(expectedSig, gotSig) {
		return 0, fmt.Errorf("session: invalid signature")
	}

	// Decode and validate payload.
	payloadJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return 0, fmt.Errorf("session: decode payload: %w", err)
	}
	var claims struct {
		Iss string `json:"iss"`
		Sub string `json:"sub"`
		Exp int64  `json:"exp"`
	}
	if err := json.Unmarshal(payloadJSON, &claims); err != nil {
		return 0, fmt.Errorf("session: parse payload: %w", err)
	}
	if !validSessionIssuers[claims.Iss] {
		return 0, fmt.Errorf("session: unexpected issuer %q", claims.Iss)
	}
	if time.Now().Unix() > claims.Exp {
		return 0, fmt.Errorf("session: token expired")
	}

	// Parse sub: "lurus:<accountID>".
	const subPrefix = "lurus:"
	if !strings.HasPrefix(claims.Sub, subPrefix) {
		return 0, fmt.Errorf("session: invalid sub format: %q", claims.Sub)
	}
	id, err := strconv.ParseInt(claims.Sub[len(subPrefix):], 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("session: invalid account id in sub")
	}
	return id, nil
}

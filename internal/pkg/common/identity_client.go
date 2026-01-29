package common

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

// IdentityServiceURL is the base URL for identity-service
var IdentityServiceURL = getIdentityServiceURL()

func getIdentityServiceURL() string {
	if url := os.Getenv("IDENTITY_SERVICE_URL"); url != "" {
		return url
	}
	return "http://identity-service.lurus-identity.svc.cluster.local:18104"
}

// IdentityMapping represents the unified user identity mapping
type IdentityMapping struct {
	ID            string    `json:"id"`
	ZitadelUserID string    `json:"zitadel_user_id"`
	SupabaseUID   string    `json:"supabase_user_id,omitempty"`
	LurusAPIUID   int       `json:"lurus_api_user_id,omitempty"`
	Email         string    `json:"email"`
	DisplayName   string    `json:"display_name"`
	AvatarURL     string    `json:"avatar_url,omitempty"`
	LinkedAt      time.Time `json:"linked_at"`
	LastSyncAt    time.Time `json:"last_sync_at"`
}

// SyncUserRequest is the request to sync a user with identity-service
type SyncUserRequest struct {
	ZitadelUserID string `json:"zitadel_user_id"`
	LurusAPIUID   int    `json:"lurus_api_user_id"`
	Email         string `json:"email"`
	DisplayName   string `json:"display_name"`
}

// SyncUserResponse is the response from sync operation
type SyncUserResponse struct {
	Success bool            `json:"success"`
	Message string          `json:"message,omitempty"`
	Data    IdentityMapping `json:"data,omitempty"`
}

var identityClient = &http.Client{
	Timeout: 10 * time.Second,
}

// SyncUserWithIdentityService syncs a user to the identity-service
// This creates or updates the identity mapping after OIDC login
func SyncUserWithIdentityService(zitadelUserID string, lurusAPIUID int, email, displayName string) (*IdentityMapping, error) {
	if IdentityServiceURL == "" {
		SysLog("Identity service URL not configured, skipping sync")
		return nil, nil
	}

	req := SyncUserRequest{
		ZitadelUserID: zitadelUserID,
		LurusAPIUID:   lurusAPIUID,
		Email:         email,
		DisplayName:   displayName,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal sync request: %w", err)
	}

	resp, err := identityClient.Post(
		IdentityServiceURL+"/api/v1/users/sync",
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		SysLog(fmt.Sprintf("Failed to sync user with identity-service: %v", err))
		// Don't fail the login, just log the error
		return nil, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		SysLog(fmt.Sprintf("Identity service returned status %d", resp.StatusCode))
		return nil, nil
	}

	var syncResp SyncUserResponse
	if err := json.NewDecoder(resp.Body).Decode(&syncResp); err != nil {
		SysLog(fmt.Sprintf("Failed to decode sync response: %v", err))
		return nil, nil
	}

	if !syncResp.Success {
		SysLog(fmt.Sprintf("Identity service sync failed: %s", syncResp.Message))
		return nil, nil
	}

	return &syncResp.Data, nil
}

// GetIdentityMappingByOidcID retrieves identity mapping by OIDC ID
func GetIdentityMappingByOidcID(oidcID string) (*IdentityMapping, error) {
	if IdentityServiceURL == "" {
		return nil, nil
	}

	resp, err := identityClient.Get(IdentityServiceURL + "/api/v1/users/by-oidc/" + oidcID)
	if err != nil {
		SysLog(fmt.Sprintf("Failed to get identity mapping: %v", err))
		return nil, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, nil
	}

	var mapping IdentityMapping
	if err := json.NewDecoder(resp.Body).Decode(&mapping); err != nil {
		return nil, nil
	}

	return &mapping, nil
}

package handler

import (
	"net/http"
	"testing"

	"github.com/LurusTech/lurus-hub/internal/pkg/common"
)

// ============================================================================
// V2 Platform Admin — System Options Tests
// ============================================================================

func seedOption(key, value string) {
	common.OptionMapRWMutex.Lock()
	if common.OptionMap == nil {
		common.OptionMap = make(map[string]string)
	}
	common.OptionMap[key] = value
	common.OptionMapRWMutex.Unlock()
}

func optionPresent(resp map[string]interface{}, key string) (string, bool) {
	data, ok := resp["data"].([]interface{})
	if !ok {
		return "", false
	}
	for _, raw := range data {
		opt, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		if opt["key"] == key {
			v, _ := opt["value"].(string)
			return v, true
		}
	}
	return "", false
}

// TestListAdminOptionsV2_ExcludesSecrets proves the GET endpoint filters out
// secret-suffixed keys (*Token / *Secret / *Key) while surfacing safe ones.
func TestListAdminOptionsV2_ExcludesSecrets(t *testing.T) {
	ctx := SetupV2TestRouter(t)
	defer ctx.Cleanup()

	seedOption("SystemName", "Lurus Hub")
	seedOption("SomethingSecret", "shh")
	seedOption("SomethingToken", "tok")
	seedOption("SomethingKey", "key")

	w := V2RequestAsUser(ctx, ctx.RootUser, http.MethodGet, "/api/v2/admin/options", nil, []string{"root"})
	AssertV2Status(t, w, http.StatusOK)
	resp := AssertV2Success(t, w)

	if v, ok := optionPresent(resp, "SystemName"); !ok || v != "Lurus Hub" {
		t.Errorf("expected SystemName=Lurus Hub in options, got %q present=%v", v, ok)
	}
	for _, secret := range []string{"SomethingSecret", "SomethingToken", "SomethingKey"} {
		if _, ok := optionPresent(resp, secret); ok {
			t.Errorf("secret-suffixed key %q leaked through the options list", secret)
		}
	}
}

// TestUpdateAdminOptionV2_Persists writes a known key and reads it back.
func TestUpdateAdminOptionV2_Persists(t *testing.T) {
	ctx := SetupV2TestRouter(t)
	defer ctx.Cleanup()

	body := map[string]interface{}{"key": "SystemName", "value": "Renamed Hub"}
	w := V2RequestAsUser(ctx, ctx.RootUser, http.MethodPut, "/api/v2/admin/options", body, []string{"root"})
	AssertV2Status(t, w, http.StatusOK)
	AssertV2Success(t, w)

	w = V2RequestAsUser(ctx, ctx.RootUser, http.MethodGet, "/api/v2/admin/options", nil, []string{"root"})
	resp := AssertV2Success(t, w)
	if v, ok := optionPresent(resp, "SystemName"); !ok || v != "Renamed Hub" {
		t.Errorf("expected persisted SystemName=Renamed Hub, got %q present=%v", v, ok)
	}
}

// TestUpdateAdminOptionV2_ValidationGate confirms the delegated per-key
// validation fires: enabling GitHub OAuth without a client id fails honestly
// (success=false) rather than silently flipping the flag.
func TestUpdateAdminOptionV2_ValidationGate(t *testing.T) {
	ctx := SetupV2TestRouter(t)
	defer ctx.Cleanup()

	prev := common.GitHubClientId
	common.GitHubClientId = ""
	defer func() { common.GitHubClientId = prev }()

	body := map[string]interface{}{"key": "GitHubOAuthEnabled", "value": true}
	w := V2RequestAsUser(ctx, ctx.RootUser, http.MethodPut, "/api/v2/admin/options", body, []string{"root"})
	// UpdateOption returns 200 with success=false for validation failures.
	resp := ParseV2Response(t, w)
	if success, _ := resp["success"].(bool); success {
		t.Error("expected GitHubOAuthEnabled without client id to fail validation, got success=true")
	}
}

// TestAdminOptionsV2_NonRootRejected confirms a non-root caller is forbidden.
func TestAdminOptionsV2_NonRootRejected(t *testing.T) {
	ctx := SetupV2TestRouter(t)
	defer ctx.Cleanup()

	w := V2RequestAsUser(ctx, ctx.NormalUser, http.MethodGet, "/api/v2/admin/options", nil, nil)
	AssertV2Error(t, w, http.StatusForbidden)

	body := map[string]interface{}{"key": "SystemName", "value": "x"}
	w = V2RequestAsUser(ctx, ctx.NormalUser, http.MethodPut, "/api/v2/admin/options", body, nil)
	AssertV2Error(t, w, http.StatusForbidden)
}

// TestUpdateAdminOptionV2_NullValueRejected proves a JSON null value is
// rejected with 400 instead of persisting the literal "<nil>" string.
func TestUpdateAdminOptionV2_NullValueRejected(t *testing.T) {
	ctx := SetupV2TestRouter(t)
	defer ctx.Cleanup()

	seedOption("SystemName", "Lurus Hub")

	body := map[string]interface{}{"key": "SystemName", "value": nil}
	w := V2RequestAsUser(ctx, ctx.RootUser, http.MethodPut, "/api/v2/admin/options", body, []string{"root"})
	AssertV2Error(t, w, http.StatusBadRequest)

	// The stored value must be untouched — in particular not "<nil>".
	common.OptionMapRWMutex.RLock()
	got := common.OptionMap["SystemName"]
	common.OptionMapRWMutex.RUnlock()
	if got != "Lurus Hub" {
		t.Errorf("expected SystemName unchanged after null update, got %q", got)
	}
}

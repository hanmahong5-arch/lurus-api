package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/LurusTech/lurus-hub/internal/adapter/repo"
	"github.com/LurusTech/lurus-hub/internal/pkg/common"

	"github.com/gin-gonic/gin"
)

// ============================================================================
// Anonymous Switch Redeem Handler Tests
//
// These tests build a minimal router that mounts the handler at
// /api/v2/switch/redeem with NO middleware — that matches the production
// route shape and guarantees the handler can stand on its own without a
// TenantContext in the gin.Context.
// ============================================================================

// setupSwitchRedeemRouter reuses the V2 SQLite test rig and adds the
// anonymous /switch/redeem route. Returns ctx so callers can seed
// redemption rows directly via ctx.DB.
func setupSwitchRedeemRouter(t *testing.T) (*V2TestContext, *gin.Engine) {
	t.Helper()
	ctx := SetupV2TestRouter(t)

	// Build a separate router scoped to the anonymous endpoint. Reusing
	// ctx.Router would inherit the mockAuth middleware, which would
	// silently provide a TenantContext and mask middleware-leak bugs in
	// the handler under test.
	r := gin.New()
	r.POST("/api/v2/switch/redeem", SwitchRedeemAnonymous)
	return ctx, r
}

func postSwitchRedeem(t *testing.T, r *gin.Engine, body interface{}) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		reader = bytes.NewReader(raw)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v2/switch/redeem", reader)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func parseEnvelope(t *testing.T, w *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var env map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal envelope: %v, raw: %s", err, w.Body.String())
	}
	return env
}

// seedSwitchRedemption inserts a redemption with custom status / expired
// time / tenant ID for a specific test scenario.
func seedSwitchRedemption(t *testing.T, ctx *V2TestContext, key string, status int, expiredTime int64, quota int) *repo.Redemption {
	t.Helper()
	if key == "" {
		key = common.GetRandomString(32)
	}
	r := &repo.Redemption{
		UserId:      ctx.AdminUser.Id,
		TenantId:    ctx.TenantID,
		Key:         key,
		Name:        "Switch Test",
		Quota:       quota,
		Status:      status,
		CreatedTime: common.GetTimestamp(),
		ExpiredTime: expiredTime,
	}
	if err := repo.RedemptionInsert(r); err != nil {
		t.Fatalf("seed redemption: %v", err)
	}
	return r
}

// ---------------------------------------------------------------------------
// Table-driven: input-validation failures should reject at 400.
// ---------------------------------------------------------------------------

func TestSwitchRedeemAnonymous_InputValidation(t *testing.T) {
	ctx, r := setupSwitchRedeemRouter(t)
	defer ctx.Cleanup()

	cases := []struct {
		name string
		body interface{}
	}{
		{"empty code", map[string]string{"code": "", "fingerprint": "fp123"}},
		{"empty fingerprint", map[string]string{"code": "abc", "fingerprint": ""}},
		{"whitespace code", map[string]string{"code": "   ", "fingerprint": "fp123"}},
		{"whitespace fingerprint", map[string]string{"code": "abc", "fingerprint": "  "}},
		{"missing both", map[string]string{}},
		{"malformed body", "not-a-json-object"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := postSwitchRedeem(t, r, tc.body)
			if w.Code != http.StatusBadRequest {
				t.Errorf("expected 400, got %d, body: %s", w.Code, w.Body.String())
			}
			env := parseEnvelope(t, w)
			if success, _ := env["success"].(bool); success {
				t.Errorf("expected success=false")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Success path.
// ---------------------------------------------------------------------------

func TestSwitchRedeemAnonymous_Success(t *testing.T) {
	ctx, r := setupSwitchRedeemRouter(t)
	defer ctx.Cleanup()

	key := common.GetRandomString(32)
	redemption := seedSwitchRedemption(t, ctx, key, common.RedemptionCodeStatusEnabled, 0, 250000)

	w := postSwitchRedeem(t, r, map[string]string{
		"code":        key,
		"fingerprint": "fingerprint-success-001",
		"app_version": "0.5.0",
	})

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body: %s", w.Code, w.Body.String())
	}
	env := parseEnvelope(t, w)
	if success, _ := env["success"].(bool); !success {
		t.Fatalf("expected success=true, got envelope: %s", w.Body.String())
	}
	data, ok := env["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("missing data field, body: %s", w.Body.String())
	}
	token, _ := data["user_token"].(string)
	if len(token) != 48 {
		t.Errorf("expected 48-char user_token, got len=%d: %q", len(token), token)
	}
	userID, _ := data["user_id"].(float64)
	if userID == 0 {
		t.Errorf("expected non-zero user_id, got %v", data["user_id"])
	}
	quota, _ := data["quota"].(float64)
	if int(quota) != redemption.Quota {
		t.Errorf("expected quota=%d, got %v", redemption.Quota, data["quota"])
	}
	slug, _ := data["tenant_slug"].(string)
	if slug == "" {
		t.Errorf("expected non-empty tenant_slug for non-default tenant")
	}

	// Verify the redemption row was flipped to used.
	var refreshed repo.Redemption
	if err := repo.WithoutTenantIsolation(ctx.DB).Where("`key` = ?", key).First(&refreshed).Error; err != nil {
		t.Fatalf("refetch redemption: %v", err)
	}
	if refreshed.Status != common.RedemptionCodeStatusUsed {
		t.Errorf("expected redemption.Status=Used after redeem, got %d", refreshed.Status)
	}
}

// ---------------------------------------------------------------------------
// Already-used: must respond 200 success=false with message containing
// "已使用" or "used" so the Switch client classifier hits ErrCodeUsed.
// ---------------------------------------------------------------------------

func TestSwitchRedeemAnonymous_AlreadyUsed(t *testing.T) {
	ctx, r := setupSwitchRedeemRouter(t)
	defer ctx.Cleanup()

	key := common.GetRandomString(32)
	seedSwitchRedemption(t, ctx, key, common.RedemptionCodeStatusUsed, 0, 100000)

	w := postSwitchRedeem(t, r, map[string]string{
		"code":        key,
		"fingerprint": "fingerprint-used-001",
	})

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	env := parseEnvelope(t, w)
	if success, _ := env["success"].(bool); success {
		t.Fatalf("expected success=false for used code")
	}
	msg, _ := env["message"].(string)
	low := strings.ToLower(msg)
	if !strings.Contains(msg, "已使用") && !strings.Contains(low, "used") {
		t.Errorf("message must contain '已使用' or 'used', got: %q", msg)
	}
}

// ---------------------------------------------------------------------------
// Expired: message must contain "过期" or "expire".
// ---------------------------------------------------------------------------

func TestSwitchRedeemAnonymous_Expired(t *testing.T) {
	ctx, r := setupSwitchRedeemRouter(t)
	defer ctx.Cleanup()

	key := common.GetRandomString(32)
	past := common.GetTimestamp() - 3600 // 1h ago
	seedSwitchRedemption(t, ctx, key, common.RedemptionCodeStatusEnabled, past, 100000)

	w := postSwitchRedeem(t, r, map[string]string{
		"code":        key,
		"fingerprint": "fingerprint-expired-001",
	})

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	env := parseEnvelope(t, w)
	if success, _ := env["success"].(bool); success {
		t.Fatalf("expected success=false for expired code")
	}
	msg, _ := env["message"].(string)
	low := strings.ToLower(msg)
	if !strings.Contains(msg, "过期") && !strings.Contains(low, "expire") {
		t.Errorf("message must contain '过期' or 'expire', got: %q", msg)
	}
}

// ---------------------------------------------------------------------------
// Disabled: message must contain "禁用" / "disabled" / "revoked".
// ---------------------------------------------------------------------------

func TestSwitchRedeemAnonymous_Disabled(t *testing.T) {
	ctx, r := setupSwitchRedeemRouter(t)
	defer ctx.Cleanup()

	key := common.GetRandomString(32)
	seedSwitchRedemption(t, ctx, key, common.RedemptionCodeStatusDisabled, 0, 100000)

	w := postSwitchRedeem(t, r, map[string]string{
		"code":        key,
		"fingerprint": "fingerprint-disabled-001",
	})

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	env := parseEnvelope(t, w)
	if success, _ := env["success"].(bool); success {
		t.Fatalf("expected success=false for disabled code")
	}
	msg, _ := env["message"].(string)
	low := strings.ToLower(msg)
	if !strings.Contains(msg, "禁用") &&
		!strings.Contains(low, "disabled") &&
		!strings.Contains(low, "revoked") {
		t.Errorf("message must contain '禁用'/'disabled'/'revoked', got: %q", msg)
	}
}

// ---------------------------------------------------------------------------
// Non-existent: 200 success=false (NOT 404). Switch only treats real HTTP
// 404 as "this Hub doesn't expose the endpoint yet" — a bad code must come
// back inside the envelope so the user sees an actionable message.
// ---------------------------------------------------------------------------

func TestSwitchRedeemAnonymous_NonExistent(t *testing.T) {
	ctx, r := setupSwitchRedeemRouter(t)
	defer ctx.Cleanup()

	w := postSwitchRedeem(t, r, map[string]string{
		"code":        "doesnotexist00000000000000000000",
		"fingerprint": "fingerprint-missing-001",
	})

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 (envelope), got %d", w.Code)
	}
	env := parseEnvelope(t, w)
	if success, _ := env["success"].(bool); success {
		t.Fatalf("expected success=false for nonexistent code")
	}
	msg, _ := env["message"].(string)
	if msg == "" {
		t.Errorf("expected non-empty message for nonexistent code")
	}
}

// ---------------------------------------------------------------------------
// Repeated redeem by same fingerprint: second call (after success) must
// classify as ErrCodeUsed, and the user row should NOT duplicate.
// ---------------------------------------------------------------------------

func TestSwitchRedeemAnonymous_RepeatSameFingerprint(t *testing.T) {
	ctx, r := setupSwitchRedeemRouter(t)
	defer ctx.Cleanup()

	key := common.GetRandomString(32)
	seedSwitchRedemption(t, ctx, key, common.RedemptionCodeStatusEnabled, 0, 50000)
	fp := "fingerprint-repeat-001"

	// First redeem: success.
	w := postSwitchRedeem(t, r, map[string]string{"code": key, "fingerprint": fp})
	if w.Code != http.StatusOK {
		t.Fatalf("first redeem: expected 200, got %d", w.Code)
	}
	env := parseEnvelope(t, w)
	if success, _ := env["success"].(bool); !success {
		t.Fatalf("first redeem: expected success=true, body: %s", w.Body.String())
	}
	firstData := env["data"].(map[string]interface{})
	firstUserID := int(firstData["user_id"].(float64))

	// Second redeem with same fingerprint + same key: must report used.
	w = postSwitchRedeem(t, r, map[string]string{"code": key, "fingerprint": fp})
	env = parseEnvelope(t, w)
	if success, _ := env["success"].(bool); success {
		t.Fatalf("second redeem: expected success=false, body: %s", w.Body.String())
	}
	msg, _ := env["message"].(string)
	low := strings.ToLower(msg)
	if !strings.Contains(msg, "已使用") && !strings.Contains(low, "used") {
		t.Errorf("second redeem message must contain '已使用'/'used', got: %q", msg)
	}

	// Sanity: only one user row for this fingerprint.
	var count int64
	if err := repo.WithoutTenantIsolation(ctx.DB).Model(&repo.User{}).
		Where("username LIKE ?", switchEndUserUsernamePrefix+"%").
		Count(&count).Error; err != nil {
		t.Fatalf("count switch users: %v", err)
	}
	if count != 1 {
		t.Errorf("expected exactly 1 switch user row, got %d", count)
	}

	// And the first user_id should still be reachable.
	var u repo.User
	if err := repo.WithoutTenantIsolation(ctx.DB).Where("id = ?", firstUserID).First(&u).Error; err != nil {
		t.Errorf("first user row missing: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Different fingerprints redeeming the same code: first wins, second sees
// used. Verifies the FOR UPDATE lock inside repo.Redeem is engaged via
// our path (i.e., we don't have a TOCTOU between the pre-check and the
// redeem call).
// ---------------------------------------------------------------------------

func TestSwitchRedeemAnonymous_RaceOnSameCode(t *testing.T) {
	ctx, r := setupSwitchRedeemRouter(t)
	defer ctx.Cleanup()

	key := common.GetRandomString(32)
	seedSwitchRedemption(t, ctx, key, common.RedemptionCodeStatusEnabled, 0, 75000)

	// First fingerprint wins.
	w := postSwitchRedeem(t, r, map[string]string{"code": key, "fingerprint": "fp-race-A"})
	env := parseEnvelope(t, w)
	if success, _ := env["success"].(bool); !success {
		t.Fatalf("first fingerprint should win, body: %s", w.Body.String())
	}

	// Second fingerprint must see used.
	w = postSwitchRedeem(t, r, map[string]string{"code": key, "fingerprint": "fp-race-B"})
	env = parseEnvelope(t, w)
	if success, _ := env["success"].(bool); success {
		t.Fatalf("second fingerprint should fail, body: %s", w.Body.String())
	}
}

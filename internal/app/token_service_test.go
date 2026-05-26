package app

import (
	"strings"
	"testing"

	"github.com/LurusTech/lurus-hub/internal/adapter/repo"
	"github.com/LurusTech/lurus-hub/internal/pkg/common"
)

// ---------------------------------------------------------------------------
// ValidateTokenName
// ---------------------------------------------------------------------------

func TestValidateTokenName_ReturnsNilForEmptyName(t *testing.T) {
	if err := ValidateTokenName(""); err != nil {
		t.Errorf("expected nil error for empty name, got: %v", err)
	}
}

func TestValidateTokenName_ReturnsNilForExactMaxLength(t *testing.T) {
	name := strings.Repeat("a", TokenNameMaxLength)
	if err := ValidateTokenName(name); err != nil {
		t.Errorf("expected nil error for name of exactly %d chars, got: %v", TokenNameMaxLength, err)
	}
}

func TestValidateTokenName_ReturnsErrorWhenExceedsMaxLength(t *testing.T) {
	name := strings.Repeat("a", TokenNameMaxLength+1)
	err := ValidateTokenName(name)
	if err == nil {
		t.Fatal("expected error for name exceeding max length, got nil")
	}
	if !strings.Contains(err.Error(), "令牌名称过长") {
		t.Errorf("expected error message to contain '令牌名称过长', got: %v", err)
	}
}

func TestValidateTokenName_ReturnsNilForShortName(t *testing.T) {
	if err := ValidateTokenName("test"); err != nil {
		t.Errorf("expected nil error for short name, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// ValidateTokenQuota
// ---------------------------------------------------------------------------

func TestValidateTokenQuota_ReturnsNilWhenUnlimited(t *testing.T) {
	// Any quota value should be accepted when unlimitedQuota is true.
	cases := []struct {
		name        string
		remainQuota int
	}{
		{"negative quota", -100},
		{"zero quota", 0},
		{"large quota", int(MaxQuotaMultiplier*common.QuotaPerUnit) + 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateTokenQuota(tc.remainQuota, true); err != nil {
				t.Errorf("expected nil when unlimitedQuota=true (%s), got: %v", tc.name, err)
			}
		})
	}
}

func TestValidateTokenQuota_ReturnsErrorForNegativeQuota(t *testing.T) {
	err := ValidateTokenQuota(-1, false)
	if err == nil {
		t.Fatal("expected error for negative quota, got nil")
	}
	if !strings.Contains(err.Error(), "不能为负数") {
		t.Errorf("expected error about negative quota, got: %v", err)
	}
}

func TestValidateTokenQuota_ReturnsNilForZeroQuota(t *testing.T) {
	if err := ValidateTokenQuota(0, false); err != nil {
		t.Errorf("expected nil for zero quota, got: %v", err)
	}
}

func TestValidateTokenQuota_ReturnsNilForValidQuota(t *testing.T) {
	if err := ValidateTokenQuota(1000, false); err != nil {
		t.Errorf("expected nil for valid quota, got: %v", err)
	}
}

func TestValidateTokenQuota_ReturnsNilForExactMaxQuota(t *testing.T) {
	maxQuota := int(MaxQuotaMultiplier * common.QuotaPerUnit)
	if err := ValidateTokenQuota(maxQuota, false); err != nil {
		t.Errorf("expected nil for quota at exact max boundary, got: %v", err)
	}
}

func TestValidateTokenQuota_ReturnsErrorWhenExceedsMax(t *testing.T) {
	maxQuota := int(MaxQuotaMultiplier * common.QuotaPerUnit)
	err := ValidateTokenQuota(maxQuota+1, false)
	if err == nil {
		t.Fatal("expected error for quota exceeding max, got nil")
	}
	if !strings.Contains(err.Error(), "超出有效范围") {
		t.Errorf("expected error about exceeding range, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// CanEnableToken
// ---------------------------------------------------------------------------

func TestCanEnableToken_ReturnsNilForEnabledToken(t *testing.T) {
	token := &repo.Token{
		Status:         common.TokenStatusEnabled,
		ExpiredTime:    -1,
		RemainQuota:    1000,
		UnlimitedQuota: false,
	}
	if err := CanEnableToken(token); err != nil {
		t.Errorf("expected nil for enabled token, got: %v", err)
	}
}

func TestCanEnableToken_ReturnsErrorForExpiredTokenWithPastExpiry(t *testing.T) {
	// ExpiredTime in the past and not -1 => cannot enable.
	token := &repo.Token{
		Status:      common.TokenStatusExpired,
		ExpiredTime: common.GetTimestamp() - 3600, // 1 hour ago
	}
	err := CanEnableToken(token)
	if err == nil {
		t.Fatal("expected error for expired token with past expiry, got nil")
	}
	if !strings.Contains(err.Error(), "令牌已过期") {
		t.Errorf("expected error about expired token, got: %v", err)
	}
}

func TestCanEnableToken_ReturnsNilForExpiredTokenWithNeverExpires(t *testing.T) {
	// ExpiredTime == -1 means "never expires", so it should bypass the expiry check.
	token := &repo.Token{
		Status:      common.TokenStatusExpired,
		ExpiredTime: -1,
	}
	if err := CanEnableToken(token); err != nil {
		t.Errorf("expected nil for expired token with never-expires flag, got: %v", err)
	}
}

func TestCanEnableToken_ReturnsNilForExpiredTokenWithFutureExpiry(t *testing.T) {
	// ExpiredTime in the future => the condition `ExpiredTime <= GetTimestamp()` is false.
	token := &repo.Token{
		Status:      common.TokenStatusExpired,
		ExpiredTime: common.GetTimestamp() + 86400, // 1 day ahead
	}
	if err := CanEnableToken(token); err != nil {
		t.Errorf("expected nil for expired-status token with future expiry, got: %v", err)
	}
}

func TestCanEnableToken_ReturnsErrorForExhaustedTokenWithZeroQuota(t *testing.T) {
	token := &repo.Token{
		Status:         common.TokenStatusExhausted,
		RemainQuota:    0,
		UnlimitedQuota: false,
	}
	err := CanEnableToken(token)
	if err == nil {
		t.Fatal("expected error for exhausted token with zero quota, got nil")
	}
	if !strings.Contains(err.Error(), "额度已用尽") {
		t.Errorf("expected error about exhausted quota, got: %v", err)
	}
}

func TestCanEnableToken_ReturnsErrorForExhaustedTokenWithNegativeQuota(t *testing.T) {
	token := &repo.Token{
		Status:         common.TokenStatusExhausted,
		RemainQuota:    -10,
		UnlimitedQuota: false,
	}
	err := CanEnableToken(token)
	if err == nil {
		t.Fatal("expected error for exhausted token with negative quota, got nil")
	}
	if !strings.Contains(err.Error(), "额度已用尽") {
		t.Errorf("expected error about exhausted quota, got: %v", err)
	}
}

func TestCanEnableToken_ReturnsNilForExhaustedTokenWithUnlimitedQuota(t *testing.T) {
	// UnlimitedQuota bypasses the exhaustion check.
	token := &repo.Token{
		Status:         common.TokenStatusExhausted,
		RemainQuota:    0,
		UnlimitedQuota: true,
	}
	if err := CanEnableToken(token); err != nil {
		t.Errorf("expected nil for exhausted token with unlimited quota, got: %v", err)
	}
}

func TestCanEnableToken_ReturnsNilForExhaustedTokenWithPositiveQuota(t *testing.T) {
	// RemainQuota > 0 means the exhaustion branch condition fails.
	token := &repo.Token{
		Status:         common.TokenStatusExhausted,
		RemainQuota:    500,
		UnlimitedQuota: false,
	}
	if err := CanEnableToken(token); err != nil {
		t.Errorf("expected nil for exhausted token with positive remaining quota, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// CanEnableToken table-driven: combined edge cases
// ---------------------------------------------------------------------------

func TestCanEnableToken_TableDriven(t *testing.T) {
	now := common.GetTimestamp()

	tests := []struct {
		name    string
		token   *repo.Token
		wantErr bool
	}{
		{
			name: "enabled status with unlimited quota",
			token: &repo.Token{
				Status:         common.TokenStatusEnabled,
				ExpiredTime:    -1,
				RemainQuota:    0,
				UnlimitedQuota: true,
			},
			wantErr: false,
		},
		{
			name: "expired status, expiry exactly at current timestamp",
			token: &repo.Token{
				Status:      common.TokenStatusExpired,
				ExpiredTime: now, // <= now is true
			},
			wantErr: true,
		},
		{
			name: "disabled status (not expired or exhausted) passes",
			token: &repo.Token{
				Status:      common.TokenStatusDisabled,
				ExpiredTime: -1,
			},
			wantErr: false,
		},
		{
			name: "exhausted status, zero quota, unlimited false",
			token: &repo.Token{
				Status:         common.TokenStatusExhausted,
				RemainQuota:    0,
				UnlimitedQuota: false,
			},
			wantErr: true,
		},
		{
			name: "exhausted status, positive quota, unlimited false",
			token: &repo.Token{
				Status:         common.TokenStatusExhausted,
				RemainQuota:    1,
				UnlimitedQuota: false,
			},
			wantErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := CanEnableToken(tc.token)
			if (err != nil) != tc.wantErr {
				t.Errorf("CanEnableToken() error = %v, wantErr = %v", err, tc.wantErr)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// NormalizeTokenScopes
// ---------------------------------------------------------------------------

func TestNormalizeTokenScopes_EmptyReturnsEmpty(t *testing.T) {
	// nil and [] must both round-trip to "" — the wire shape `scopes: []`
	// from a UI client is the canonical "no restriction" signal and must
	// not error.
	if got, err := NormalizeTokenScopes(nil); err != nil || got != "" {
		t.Errorf("NormalizeTokenScopes(nil) = (%q, %v), want (\"\", nil)", got, err)
	}
	if got, err := NormalizeTokenScopes([]string{}); err != nil || got != "" {
		t.Errorf("NormalizeTokenScopes([]) = (%q, %v), want (\"\", nil)", got, err)
	}
}

func TestNormalizeTokenScopes_ValidScopesJoined(t *testing.T) {
	got, err := NormalizeTokenScopes([]string{"chat"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "chat" {
		t.Errorf("got %q, want %q", got, "chat")
	}
}

func TestNormalizeTokenScopes_DedupsAndCanonicalOrder(t *testing.T) {
	// "image,chat,chat" must canonicalize to "chat,image" (canonical order
	// per types.ValidTokenScopes) so two equal inputs produce equal stored
	// strings — keeps audit diffs and DB equality checks deterministic.
	got, err := NormalizeTokenScopes([]string{"image", "chat", "chat"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "chat,image" {
		t.Errorf("got %q, want %q", got, "chat,image")
	}
}

func TestNormalizeTokenScopes_TrimsWhitespace(t *testing.T) {
	got, err := NormalizeTokenScopes([]string{"  chat  ", " image "})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "chat,image" {
		t.Errorf("got %q, want %q", got, "chat,image")
	}
}

func TestNormalizeTokenScopes_RejectsUnknownScope(t *testing.T) {
	_, err := NormalizeTokenScopes([]string{"chat", "writeall"})
	if err == nil {
		t.Fatal("expected error for unknown scope, got nil")
	}
	if !strings.Contains(err.Error(), "writeall") {
		t.Errorf("error should name the bad scope, got: %v", err)
	}
}

func TestNormalizeTokenScopes_DropsEmptyEntries(t *testing.T) {
	got, err := NormalizeTokenScopes([]string{"chat", "", "  "})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "chat" {
		t.Errorf("got %q, want %q", got, "chat")
	}
}

// ---------------------------------------------------------------------------
// ApplyTokenUpdate
// ---------------------------------------------------------------------------

func TestApplyTokenUpdate_CopiesAllFields(t *testing.T) {
	allowIps := "192.168.1.0/24"
	source := &repo.Token{
		Name:               "updated-token",
		ExpiredTime:        1700000000,
		RemainQuota:        5000,
		UnlimitedQuota:     true,
		ModelLimitsEnabled: true,
		ModelLimits:        "gpt-4,gpt-3.5-turbo",
		AllowIps:           &allowIps,
		Group:              "vip",
		CrossGroupRetry:    true,
	}

	target := &repo.Token{
		Id:                 42,
		UserId:             7,
		Key:                "sk-original-key",
		Name:               "old-name",
		ExpiredTime:        1600000000,
		RemainQuota:        100,
		UnlimitedQuota:     false,
		ModelLimitsEnabled: false,
		ModelLimits:        "",
		AllowIps:           nil,
		Group:              "default",
		CrossGroupRetry:    false,
	}

	ApplyTokenUpdate(target, source)

	// Verify all copied fields match the source.
	if target.Name != source.Name {
		t.Errorf("Name: got %q, want %q", target.Name, source.Name)
	}
	if target.ExpiredTime != source.ExpiredTime {
		t.Errorf("ExpiredTime: got %d, want %d", target.ExpiredTime, source.ExpiredTime)
	}
	if target.RemainQuota != source.RemainQuota {
		t.Errorf("RemainQuota: got %d, want %d", target.RemainQuota, source.RemainQuota)
	}
	if target.UnlimitedQuota != source.UnlimitedQuota {
		t.Errorf("UnlimitedQuota: got %v, want %v", target.UnlimitedQuota, source.UnlimitedQuota)
	}
	if target.ModelLimitsEnabled != source.ModelLimitsEnabled {
		t.Errorf("ModelLimitsEnabled: got %v, want %v", target.ModelLimitsEnabled, source.ModelLimitsEnabled)
	}
	if target.ModelLimits != source.ModelLimits {
		t.Errorf("ModelLimits: got %q, want %q", target.ModelLimits, source.ModelLimits)
	}
	if target.AllowIps == nil || *target.AllowIps != *source.AllowIps {
		t.Errorf("AllowIps: got %v, want %v", target.AllowIps, source.AllowIps)
	}
	if target.Group != source.Group {
		t.Errorf("Group: got %q, want %q", target.Group, source.Group)
	}
	if target.CrossGroupRetry != source.CrossGroupRetry {
		t.Errorf("CrossGroupRetry: got %v, want %v", target.CrossGroupRetry, source.CrossGroupRetry)
	}
}

func TestApplyTokenUpdate_PreservesNonCopiedFields(t *testing.T) {
	source := &repo.Token{
		Name:        "new-name",
		ExpiredTime: 9999999999,
		RemainQuota: 42,
	}

	target := &repo.Token{
		Id:     100,
		UserId: 55,
		Key:    "sk-keep-this-key",
		Status: common.TokenStatusEnabled,
		Name:   "old-name",
	}

	ApplyTokenUpdate(target, source)

	// Fields NOT in the copy list must remain untouched.
	if target.Id != 100 {
		t.Errorf("Id should be preserved: got %d, want 100", target.Id)
	}
	if target.UserId != 55 {
		t.Errorf("UserId should be preserved: got %d, want 55", target.UserId)
	}
	if target.Key != "sk-keep-this-key" {
		t.Errorf("Key should be preserved: got %q, want %q", target.Key, "sk-keep-this-key")
	}
	if target.Status != common.TokenStatusEnabled {
		t.Errorf("Status should be preserved: got %d, want %d", target.Status, common.TokenStatusEnabled)
	}
}

func TestApplyTokenUpdate_HandlesNilAllowIps(t *testing.T) {
	// Source has nil AllowIps; target should receive nil.
	source := &repo.Token{
		Name:     "nil-ips-token",
		AllowIps: nil,
	}

	existingIps := "10.0.0.1"
	target := &repo.Token{
		AllowIps: &existingIps,
	}

	ApplyTokenUpdate(target, source)

	if target.AllowIps != nil {
		t.Errorf("AllowIps should be nil after update, got: %v", *target.AllowIps)
	}
}

func TestApplyTokenUpdate_HandlesZeroValues(t *testing.T) {
	// Verify that zero-value fields are correctly copied (not skipped).
	source := &repo.Token{
		Name:               "",
		ExpiredTime:        0,
		RemainQuota:        0,
		UnlimitedQuota:     false,
		ModelLimitsEnabled: false,
		ModelLimits:        "",
		AllowIps:           nil,
		Group:              "",
		CrossGroupRetry:    false,
	}

	allowIps := "1.2.3.4"
	target := &repo.Token{
		Name:               "has-name",
		ExpiredTime:        9999,
		RemainQuota:        5000,
		UnlimitedQuota:     true,
		ModelLimitsEnabled: true,
		ModelLimits:        "gpt-4",
		AllowIps:           &allowIps,
		Group:              "premium",
		CrossGroupRetry:    true,
	}

	ApplyTokenUpdate(target, source)

	if target.Name != "" {
		t.Errorf("Name should be empty string, got %q", target.Name)
	}
	if target.ExpiredTime != 0 {
		t.Errorf("ExpiredTime should be 0, got %d", target.ExpiredTime)
	}
	if target.RemainQuota != 0 {
		t.Errorf("RemainQuota should be 0, got %d", target.RemainQuota)
	}
	if target.UnlimitedQuota != false {
		t.Errorf("UnlimitedQuota should be false, got %v", target.UnlimitedQuota)
	}
	if target.ModelLimitsEnabled != false {
		t.Errorf("ModelLimitsEnabled should be false, got %v", target.ModelLimitsEnabled)
	}
	if target.ModelLimits != "" {
		t.Errorf("ModelLimits should be empty, got %q", target.ModelLimits)
	}
	if target.AllowIps != nil {
		t.Errorf("AllowIps should be nil, got %v", *target.AllowIps)
	}
	if target.Group != "" {
		t.Errorf("Group should be empty, got %q", target.Group)
	}
	if target.CrossGroupRetry != false {
		t.Errorf("CrossGroupRetry should be false, got %v", target.CrossGroupRetry)
	}
}

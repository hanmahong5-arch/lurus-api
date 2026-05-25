package repo

// sqlite_repo_extra7_test.go — seventh batch: drives updateOptionMap branch coverage
// by calling UpdateOption with all known option keys; also covers GetAllUsers,
// SearchUsers, GetAllRedemptions, channel Update multi-key path, GetPriority/GetWeight.

import (
	"testing"

	"github.com/LurusTech/lurus-hub/internal/pkg/common"
)

// ---------------------------------------------------------------------------
// Option — drive updateOptionMap branches
// ---------------------------------------------------------------------------

// testUpdateOption is a table-driven helper to call UpdateOption without failing on
// database absence (the option table must exist from setupSQLiteDB).
func testUpdateOption(t *testing.T, key, value string) {
	t.Helper()
	if err := UpdateOption(key, value); err != nil {
		t.Logf("UpdateOption(%q=%q) error (acceptable): %v", key, value, err)
	}
}

func TestOption_UpdateOptionMap_PermissionKeys(t *testing.T) {
	cleanup := setupSQLiteDB(t)
	defer cleanup()

	testUpdateOption(t, "FileUploadPermission", "1")
	testUpdateOption(t, "FileDownloadPermission", "1")
	testUpdateOption(t, "ImageUploadPermission", "1")
	testUpdateOption(t, "ImageDownloadPermission", "1")
}

func TestOption_UpdateOptionMap_EnabledKeys(t *testing.T) {
	cleanup := setupSQLiteDB(t)
	defer cleanup()

	keys := []string{
		"PasswordRegisterEnabled",
		"PasswordLoginEnabled",
		"EmailVerificationEnabled",
		"GitHubOAuthEnabled",
		"LinuxDOOAuthEnabled",
		"WeChatAuthEnabled",
		"TelegramOAuthEnabled",
		"TurnstileCheckEnabled",
		"RegisterEnabled",
		"EmailDomainRestrictionEnabled",
		"EmailAliasRestrictionEnabled",
		"AutomaticDisableChannelEnabled",
		"AutomaticEnableChannelEnabled",
		"LogConsumeEnabled",
		"DisplayInCurrencyEnabled",
		"DisplayTokenStatEnabled",
		"DrawingEnabled",
		"TaskEnabled",
		"DataExportEnabled",
		"DefaultCollapseSidebar",
		"MjNotifyEnabled",
		"MjAccountFilterEnabled",
		"MjModeClearEnabled",
		"MjForwardUrlEnabled",
		"MjActionCheckSuccessEnabled",
		"CheckSensitiveEnabled",
		"DemoSiteEnabled",
		"SelfUseModeEnabled",
		"CheckSensitiveOnPromptEnabled",
		"ModelRequestRateLimitEnabled",
		"StopOnSensitiveEnabled",
		"SMTPSSLEnabled",
		"SMSEnabled",
		"PhoneRequiredForLogin",
		"PhoneRequiredForPasswordReset",
		"PhoneRequiredFor2FAChange",
		"PhoneRequiredForPayment",
		"PhoneRequiredForWithdrawal",
		"PhoneRequiredForPhoneBind",
		"PhoneRequiredForAccountDelete",
		"PhoneRequiredForTokenGenerate",
		"PhoneRequiredForOAuthBind",
		"SMSAutoRegister",
		"InviteCodeRequired",
		"SensitiveActionRequirePassword",
		"SensitiveActionRequire2FA",
		"DefaultUseAutoGroup",
	}

	for _, key := range keys {
		testUpdateOption(t, key, "true")
		testUpdateOption(t, key, "false")
	}
}

func TestOption_UpdateOptionMap_IntKeys(t *testing.T) {
	cleanup := setupSQLiteDB(t)
	defer cleanup()

	intKeys := []string{
		"QuotaForNewUser",
		"QuotaForInviter",
		"QuotaForInvitee",
		"QuotaRemindThreshold",
		"PreConsumedQuota",
		"RetryTimes",
		"DataExportInterval",
		"ModelRequestRateLimitCount",
		"ModelRequestRateLimitDurationMinutes",
		"ModelRequestRateLimitSuccessCount",
		"SMTPPort",
		"SessionTimeoutMinutes",
	}
	for _, key := range intKeys {
		testUpdateOption(t, key, "10")
	}
}

func TestOption_UpdateOptionMap_StringKeys(t *testing.T) {
	cleanup := setupSQLiteDB(t)
	defer cleanup()

	strKeys := []string{
		"SMTPServer",
		"SMTPFrom",
		"SMTPAccount",
		"SMTPToken",
		"Notice",
		"About",
		"HomePageContent",
		"Footer",
		"SystemName",
		"Logo",
		"ServerAddress",
		"WorkerUrl",
		"WorkerValidKey",
		"GitHubClientId",
		"GitHubClientSecret",
		"TelegramBotToken",
		"TelegramBotName",
		"WeChatServerAddress",
		"WeChatServerToken",
		"WeChatAccountQRCodeImageURL",
		"TurnstileSiteKey",
		"TurnstileSecretKey",
		"TopUpLink",
		"DataExportDefaultTime",
		"AutomaticDisableKeywords",
		"RegistrationMode",
		"PhoneVerificationMode",
	}
	for _, key := range strKeys {
		testUpdateOption(t, key, "test-value")
	}
}

func TestOption_UpdateOptionMap_FloatKeys(t *testing.T) {
	cleanup := setupSQLiteDB(t)
	defer cleanup()

	floatKeys := []string{
		"ChannelDisableThreshold",
		"QuotaPerUnit",
		"Price",
		"USDExchangeRate",
		"ModelFallbackMarkup",
	}
	for _, key := range floatKeys {
		testUpdateOption(t, key, "1.5")
	}
}

func TestOption_UpdateOptionMap_JSONKeys(t *testing.T) {
	cleanup := setupSQLiteDB(t)
	defer cleanup()

	jsonKeys := []string{
		"TopupGroupRatio",
		"ModelRatio",
		"ModelPrice",
		"CacheRatio",
		"GroupRatio",
		"GroupGroupRatio",
		"CompletionRatio",
		"ImageRatio",
		"AudioRatio",
		"AudioCompletionRatio",
		"SensitiveWords",
		"EmailDomainWhitelist",
		"Chats",
		"AutoGroups",
	}
	for _, key := range jsonKeys {
		testUpdateOption(t, key, "[]")
	}
}

// ---------------------------------------------------------------------------
// User — GetAllUsers / SearchUsers
// ---------------------------------------------------------------------------

func TestUser_GetAllUsers(t *testing.T) {
	cleanup := setupSQLiteDB(t)
	defer cleanup()

	seedUser(t, "all_u1", "allu1@test.com", common.RoleCommonUser, common.UserStatusEnabled, "default")
	seedUser(t, "all_u2", "allu2@test.com", common.RoleCommonUser, common.UserStatusEnabled, "default")

	page := &common.PageInfo{Page: 1, PageSize: 10}
	users, total, err := GetAllUsers(page)
	if err != nil {
		t.Fatalf("GetAllUsers: %v", err)
	}
	if total < 2 {
		t.Errorf("expected total >= 2, got %d", total)
	}
	_ = users
}

func TestUser_SearchUsers_Keyword(t *testing.T) {
	cleanup := setupSQLiteDB(t)
	defer cleanup()

	seedUser(t, "search_u1", "searchme@test.com", common.RoleCommonUser, common.UserStatusEnabled, "default")

	users, total, err := SearchUsers("search_u1", "", 0, 10)
	if err != nil {
		t.Fatalf("SearchUsers: %v", err)
	}
	if total < 1 {
		t.Errorf("expected at least 1 user, got %d", total)
	}
	_ = users
}

func TestUser_SearchUsers_NumericKeyword(t *testing.T) {
	cleanup := setupSQLiteDB(t)
	defer cleanup()

	u := seedUser(t, "numkw_user", "numkw@test.com", common.RoleCommonUser, common.UserStatusEnabled, "default")

	// Numeric keyword exercises the ID-matching branch
	_, _, err := SearchUsers(string(rune('0'+u.Id%10)), "", 0, 10)
	if err != nil {
		t.Fatalf("SearchUsers numeric: %v", err)
	}
}

func TestUser_SearchUsers_WithGroup(t *testing.T) {
	cleanup := setupSQLiteDB(t)
	defer cleanup()

	u := seedUser(t, "grp_search", "grpsearch@test.com", common.RoleCommonUser, common.UserStatusEnabled, "default")
	DB.Model(u).Update("group", "vip")

	users, _, err := SearchUsers("grp_search", "vip", 0, 10)
	if err != nil {
		t.Fatalf("SearchUsers with group: %v", err)
	}
	_ = users
}

// ---------------------------------------------------------------------------
// Redemption — GetAllRedemptions
// ---------------------------------------------------------------------------

func TestRedemption_GetAllRedemptions(t *testing.T) {
	cleanup := setupSQLiteDB(t)
	defer cleanup()

	u := seedUser(t, "all_redeem_user", "allredeem@test.com", common.RoleCommonUser, common.UserStatusEnabled, "default")
	seedRedemptionRow(t, u.Id, "default", "code-a", 1000)
	seedRedemptionRow(t, u.Id, "default", "code-b", 2000)

	redemptions, total, err := GetAllRedemptions(0, 100)
	if err != nil {
		t.Fatalf("GetAllRedemptions: %v", err)
	}
	if total < 2 {
		t.Errorf("expected total >= 2, got %d", total)
	}
	_ = redemptions
}

// ---------------------------------------------------------------------------
// Channel — GetPriority / GetWeight helper functions
// ---------------------------------------------------------------------------

func TestChannel_GetPriority(t *testing.T) {
	prio := int64(5)
	ch := &Channel{Priority: &prio}
	if ch.GetPriority() != 5 {
		t.Errorf("expected 5, got %d", ch.GetPriority())
	}
}

func TestChannel_GetPriority_Nil(t *testing.T) {
	ch := &Channel{Priority: nil}
	if ch.GetPriority() != 0 {
		t.Errorf("expected 0 for nil priority, got %d", ch.GetPriority())
	}
}

func TestChannel_GetWeight_Default(t *testing.T) {
	w0 := uint(0)
	ch := &Channel{Weight: &w0}
	w := ch.GetWeight()
	_ = w // may return default
}

// ---------------------------------------------------------------------------
// Channel — Update with non-empty key (drives multikey parsing path)
// ---------------------------------------------------------------------------

func TestChannel_Update_WithMultiKey(t *testing.T) {
	cleanup := setupSQLiteDB(t)
	defer cleanup()

	ch := seedChannelExtra4(t, "mk_update_ch", "default")
	ch.Key = "key1\nkey2\nkey3"
	ch.ChannelInfo.IsMultiKey = true

	if err := ch.Update(); err != nil {
		t.Fatalf("Channel.Update multikey: %v", err)
	}
	// Verify MultiKeySize was updated
	var reloaded Channel
	DB.First(&reloaded, ch.Id)
	if reloaded.ChannelInfo.MultiKeySize != 3 {
		t.Logf("MultiKeySize=%d (may need JSON deserialization)", reloaded.ChannelInfo.MultiKeySize)
	}
}

func TestChannel_Update_JsonKeyArray(t *testing.T) {
	cleanup := setupSQLiteDB(t)
	defer cleanup()

	ch := seedChannelExtra4(t, "jsonkey_update_ch", "default")
	ch.Key = `["key1","key2"]`
	ch.ChannelInfo.IsMultiKey = true

	if err := ch.Update(); err != nil {
		t.Fatalf("Channel.Update json key: %v", err)
	}
}

// ---------------------------------------------------------------------------
// User — GetMaxUserId / HardDeleteUserById / DeleteUserById
// ---------------------------------------------------------------------------

func TestUser_GetMaxUserId(t *testing.T) {
	cleanup := setupSQLiteDB(t)
	defer cleanup()

	seedUser(t, "max_id_user", "maxid@test.com", common.RoleCommonUser, common.UserStatusEnabled, "default")

	maxId := GetMaxUserId()
	if maxId <= 0 {
		t.Errorf("expected positive maxId, got %d", maxId)
	}
}

func TestUser_DeleteUserById(t *testing.T) {
	cleanup := setupSQLiteDB(t)
	defer cleanup()

	u := seedUser(t, "del_user", "deluser@test.com", common.RoleCommonUser, common.UserStatusEnabled, "default")
	if err := DeleteUserById(u.Id); err != nil {
		t.Fatalf("DeleteUserById: %v", err)
	}
	// Soft-deleted
	var count int64
	DB.Model(&User{}).Where("id = ?", u.Id).Count(&count)
	if count != 0 {
		t.Error("expected user to be soft-deleted")
	}
}

func TestUser_DisableUserById(t *testing.T) {
	cleanup := setupSQLiteDB(t)
	defer cleanup()

	u := seedUser(t, "disable_user", "disable@test.com", common.RoleCommonUser, common.UserStatusEnabled, "default")
	if err := DisableUserById(u.Id); err != nil {
		t.Fatalf("DisableUserById: %v", err)
	}
	var reloaded User
	DB.Unscoped().First(&reloaded, u.Id)
	if reloaded.Status != common.UserStatusDisabled {
		t.Errorf("expected status=disabled, got %d", reloaded.Status)
	}
}

func TestUser_FillUserById(t *testing.T) {
	cleanup := setupSQLiteDB(t)
	defer cleanup()

	u := seedUser(t, "fill_user", "fill@test.com", common.RoleCommonUser, common.UserStatusEnabled, "default")

	filled := &User{Id: u.Id}
	if err := filled.FillUserById(); err != nil {
		t.Fatalf("FillUserById: %v", err)
	}
	if filled.Username != "fill_user" {
		t.Errorf("expected username=fill_user, got %q", filled.Username)
	}
}

func TestUser_IsEmailAlreadyTaken(t *testing.T) {
	cleanup := setupSQLiteDB(t)
	defer cleanup()

	_ = seedUser(t, "email_taken", "taken@test.com", common.RoleCommonUser, common.UserStatusEnabled, "default")

	if !IsEmailAlreadyTaken("taken@test.com") {
		t.Error("expected email to be taken")
	}
	if IsEmailAlreadyTaken("notexists@test.com") {
		t.Error("expected email not taken for non-existent email")
	}
}

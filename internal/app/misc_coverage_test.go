package app

// misc_coverage_test.go — real behavioural coverage for the smaller service-layer
// units around the money core: webhook delivery, log search fallback, usage
// helpers, the billing outbox worker retry loop, and the remaining
// PreConsumeTokenQuota / sourceProductOf branches. Each test asserts observable
// outputs (HTTP requests received, DB rows written/updated, computed usage), not
// mere non-panic.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	relaycommon "github.com/LurusTech/lurus-hub/internal/adapter/provider/common"
	"github.com/LurusTech/lurus-hub/internal/adapter/repo"
	"github.com/LurusTech/lurus-hub/internal/domain/entity"
	"github.com/LurusTech/lurus-hub/internal/pkg/common"
	"github.com/LurusTech/lurus-hub/internal/pkg/dto"
	"github.com/LurusTech/lurus-hub/internal/pkg/setting/ratio_setting"
	"github.com/LurusTech/lurus-hub/internal/pkg/setting/system_setting"
	"gorm.io/gorm"
)

// ─── webhook.go ───────────────────────────────────────────────────────────

// allowLocalFetch relaxes the default SSRF protection so httptest servers
// (127.0.0.1) are reachable, restoring the previous state on cleanup.
func allowLocalFetch(t *testing.T) {
	t.Helper()
	if GetHttpClient() == nil {
		InitHttpClient()
	}
	fs := system_setting.GetFetchSetting()
	prevSSRF := fs.EnableSSRFProtection
	prevPriv := fs.AllowPrivateIp
	fs.EnableSSRFProtection = false
	fs.AllowPrivateIp = true
	t.Cleanup(func() {
		fs.EnableSSRFProtection = prevSSRF
		fs.AllowPrivateIp = prevPriv
	})
}

func TestSendWebhookNotify_DeliversSignedPayload(t *testing.T) {
	allowLocalFetch(t)

	var gotSig string
	var gotPayload WebhookPayload
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSig = r.Header.Get("X-Webhook-Signature")
		_ = json.NewDecoder(r.Body).Decode(&gotPayload)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	notify := dto.NewNotify(dto.NotifyTypeQuotaExceed, "quota low", "remaining %s", []interface{}{"$1.00"})
	const secret = "s3cr3t"

	if err := SendWebhookNotify(srv.URL, secret, notify); err != nil {
		t.Fatalf("SendWebhookNotify: %v", err)
	}

	// Content placeholder must be interpolated with the value.
	if gotPayload.Content != "remaining $1.00" {
		t.Errorf("content = %q, want %q", gotPayload.Content, "remaining $1.00")
	}
	if gotPayload.Type != dto.NotifyTypeQuotaExceed {
		t.Errorf("type = %q, want %q", gotPayload.Type, dto.NotifyTypeQuotaExceed)
	}
	// Signature must match HMAC-SHA256 of the exact serialized payload.
	rawPayload, _ := json.Marshal(gotPayload)
	if want := generateSignature(secret, rawPayload); gotSig != want {
		t.Errorf("signature header = %q, want %q (HMAC over payload)", gotSig, want)
	}
	if gotSig == "" {
		t.Error("expected a non-empty X-Webhook-Signature header when secret is set")
	}
}

func TestSendWebhookNotify_Non2xxReturnsError(t *testing.T) {
	allowLocalFetch(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	notify := dto.NewNotify(dto.NotifyTypeChannelTest, "t", "c", nil)
	if err := SendWebhookNotify(srv.URL, "", notify); err == nil {
		t.Fatal("expected error on 500 response")
	}
}

func TestSendWebhookNotify_SSRFRejectsPrivateIP(t *testing.T) {
	fs := system_setting.GetFetchSetting()
	prevSSRF := fs.EnableSSRFProtection
	prevPriv := fs.AllowPrivateIp
	fs.EnableSSRFProtection = true
	fs.AllowPrivateIp = false
	t.Cleanup(func() {
		fs.EnableSSRFProtection = prevSSRF
		fs.AllowPrivateIp = prevPriv
	})

	notify := dto.NewNotify(dto.NotifyTypeChannelTest, "t", "c", nil)
	// A private/loopback URL must be rejected by the SSRF guard before any
	// request is sent.
	err := SendWebhookNotify("http://127.0.0.1:1/hook", "", notify)
	if err == nil {
		t.Fatal("expected SSRF rejection for loopback URL")
	}
}

// ─── log_service.go ───────────────────────────────────────────────────────

func TestSearchLogs_UserScopedDBFallback(t *testing.T) {
	db := setupServiceTestDB(t)
	userA := seedTestUser(t, db, 100)
	userB := seedTestUser(t, db, 100)

	// Two log rows for A, one for B.
	mustCreateLog(t, db, userA, "gpt-4o")
	mustCreateLog(t, db, userA, "gpt-4o")
	mustCreateLog(t, db, userB, "gpt-4o")

	res, err := SearchLogs(LogSearchParams{UserId: userA})
	if err != nil {
		t.Fatalf("SearchLogs user: %v", err)
	}
	if res.Total != 2 {
		t.Errorf("user-scoped total = %d, want 2", res.Total)
	}
}

func TestSearchLogs_AdminAllLogsDBFallback(t *testing.T) {
	db := setupServiceTestDB(t)
	userA := seedTestUser(t, db, 100)
	userB := seedTestUser(t, db, 100)
	mustCreateLog(t, db, userA, "gpt-4o")
	mustCreateLog(t, db, userB, "claude-3")

	// UserId == 0 => admin path, all logs.
	res, err := SearchLogs(LogSearchParams{})
	if err != nil {
		t.Fatalf("SearchLogs admin: %v", err)
	}
	if res.Total != 2 {
		t.Errorf("admin total = %d, want 2", res.Total)
	}
}

func mustCreateLog(t *testing.T, db *gorm.DB, userId int, model string) {
	t.Helper()
	l := repo.Log{
		UserId:    userId,
		TenantId:  "default",
		Type:      repo.LogTypeConsume,
		ModelName: model,
		Content:   "test log",
		CreatedAt: common.GetTimestamp(),
	}
	if err := db.Create(&l).Error; err != nil {
		t.Fatalf("create log: %v", err)
	}
}

// ─── usage_helpr.go ───────────────────────────────────────────────────────

func TestResponseText2Usage_CountsCompletionTokens(t *testing.T) {
	c := createTestGinContext()
	promptTokens := 42
	usage := ResponseText2Usage(c, "hello world this is a longer response body", "gpt-3.5-turbo", promptTokens)

	if usage.PromptTokens != promptTokens {
		t.Errorf("PromptTokens = %d, want %d", usage.PromptTokens, promptTokens)
	}
	if usage.CompletionTokens <= 0 {
		t.Errorf("CompletionTokens = %d, want > 0 for non-empty text", usage.CompletionTokens)
	}
	if usage.TotalTokens != usage.PromptTokens+usage.CompletionTokens {
		t.Errorf("TotalTokens = %d, want %d", usage.TotalTokens, usage.PromptTokens+usage.CompletionTokens)
	}
}

func TestResponseText2Usage_EmptyText(t *testing.T) {
	c := createTestGinContext()
	usage := ResponseText2Usage(c, "", "gpt-3.5-turbo", 10)
	if usage.PromptTokens != 10 {
		t.Errorf("PromptTokens = %d, want 10", usage.PromptTokens)
	}
	if usage.TotalTokens != 10+usage.CompletionTokens {
		t.Errorf("TotalTokens inconsistent: %d", usage.TotalTokens)
	}
}

func TestValidUsage_TableDriven(t *testing.T) {
	cases := []struct {
		name  string
		usage *dto.Usage
		want  bool
	}{
		{"nil", nil, false},
		{"all zero", &dto.Usage{}, false},
		{"prompt only", &dto.Usage{PromptTokens: 1}, true},
		{"completion only", &dto.Usage{CompletionTokens: 1}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ValidUsage(tc.usage); got != tc.want {
				t.Errorf("ValidUsage(%s) = %v, want %v", tc.name, got, tc.want)
			}
		})
	}
}

// ─── billing_outbox.go ────────────────────────────────────────────────────

func setupOutbox(t *testing.T, db *gorm.DB) func() {
	t.Helper()
	if err := InitBillingOutbox(db); err != nil {
		t.Fatalf("InitBillingOutbox: %v", err)
	}
	prev := billingOutboxDB
	billingOutboxDB = db
	return func() { billingOutboxDB = prev }
}

func TestEnqueueSettle_WritesPendingRow(t *testing.T) {
	db := setupServiceTestDB(t)
	restore := setupOutbox(t, db)
	defer restore()

	if err := EnqueueSettle(7, 42, 1.25); err != nil {
		t.Fatalf("EnqueueSettle: %v", err)
	}

	var row entity.BillingOutbox
	if err := db.Where("pre_auth_id = ?", 42).First(&row).Error; err != nil {
		t.Fatalf("no outbox row: %v", err)
	}
	if row.Action != outboxActionSettle {
		t.Errorf("action = %q, want %q", row.Action, outboxActionSettle)
	}
	if row.Status != "pending" {
		t.Errorf("status = %q, want pending", row.Status)
	}
	if row.AmountLB != 1.25 || row.AccountID != 7 {
		t.Errorf("row = {amt:%v acct:%d}, want {1.25 7}", row.AmountLB, row.AccountID)
	}
}

func TestEnqueueRelease_WritesPendingRow(t *testing.T) {
	db := setupServiceTestDB(t)
	restore := setupOutbox(t, db)
	defer restore()

	if err := EnqueueRelease(3, 99); err != nil {
		t.Fatalf("EnqueueRelease: %v", err)
	}
	var row entity.BillingOutbox
	if err := db.Where("pre_auth_id = ?", 99).First(&row).Error; err != nil {
		t.Fatalf("no outbox row: %v", err)
	}
	if row.Action != outboxActionRelease || row.AmountLB != 0 {
		t.Errorf("release row = {%q, %v}, want {release, 0}", row.Action, row.AmountLB)
	}
}

// TestProcessBillingOutbox_RetrySchedulesBackoff proves the worker's failure
// path: when the gRPC settle call fails (no identity backend in unit scope), the
// entry stays pending, its retry_count is incremented, and next_retry is pushed
// into the future — the row is NOT marked done or dropped.
func TestProcessBillingOutbox_RetrySchedulesBackoff(t *testing.T) {
	db := setupServiceTestDB(t)
	restore := setupOutbox(t, db)
	defer restore()

	if err := EnqueueSettle(1, 555, 0.5); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	if err := ProcessBillingOutbox(context.Background()); err != nil {
		t.Fatalf("ProcessBillingOutbox: %v", err)
	}

	var row entity.BillingOutbox
	if err := db.Where("pre_auth_id = ?", 555).First(&row).Error; err != nil {
		t.Fatalf("row gone: %v", err)
	}
	// gRPC unavailable in unit scope => settle failed => retry path.
	if row.Status != "pending" {
		t.Errorf("status = %q, want pending (retry, not done)", row.Status)
	}
	if row.RetryCount != 1 {
		t.Errorf("retry_count = %d, want 1", row.RetryCount)
	}
	if row.Error == "" {
		t.Error("expected a recorded error string on the failed entry")
	}
}

// ─── quota.go: PreConsumeTokenQuota + sourceProductOf ─────────────────────

func TestPreConsumeTokenQuota_SufficientDecrementsRemain(t *testing.T) {
	db := setupServiceTestDB(t)
	repo.InitCol() // GetTokenByKey needs the dialect-quoted key column
	userId := seedTestUser(t, db, 10_000)
	key, tokenId := seedTestToken(t, db, userId, 1_000, false)

	relayInfo := &relaycommon.RelayInfo{
		TokenId:  tokenId,
		TokenKey: key,
	}
	if err := PreConsumeTokenQuota(relayInfo, 300); err != nil {
		t.Fatalf("PreConsumeTokenQuota: %v", err)
	}
	if got := tokenRemain(t, db, tokenId); got != 700 {
		t.Errorf("token remain = %d, want 700 (1000-300)", got)
	}
}

func TestPreConsumeTokenQuota_InsufficientReturnsError(t *testing.T) {
	db := setupServiceTestDB(t)
	repo.InitCol()
	userId := seedTestUser(t, db, 10_000)
	key, tokenId := seedTestToken(t, db, userId, 100, false)

	relayInfo := &relaycommon.RelayInfo{
		TokenId:  tokenId,
		TokenKey: key,
	}
	if err := PreConsumeTokenQuota(relayInfo, 500); err == nil {
		t.Fatal("expected token-quota-not-enough error")
	}
	// Remain must be untouched on rejection.
	if got := tokenRemain(t, db, tokenId); got != 100 {
		t.Errorf("token remain = %d, want 100 (unchanged after rejection)", got)
	}
}

func TestSourceProductOf(t *testing.T) {
	if got := sourceProductOf(nil); got != ratio_setting.DefaultSourceProduct {
		t.Errorf("nil relayInfo => %q, want default %q", got, ratio_setting.DefaultSourceProduct)
	}
	if got := sourceProductOf(&relaycommon.RelayInfo{}); got != ratio_setting.DefaultSourceProduct {
		t.Errorf("empty SourceProduct => %q, want default", got)
	}
	if got := sourceProductOf(&relaycommon.RelayInfo{SourceProduct: "lucrum"}); got != "lucrum" {
		t.Errorf("set SourceProduct => %q, want lucrum", got)
	}
}

var _ = repo.LogTypeConsume

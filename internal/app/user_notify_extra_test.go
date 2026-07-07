package app

// user_notify_extra_test.go — coverage for the NotifyUser dispatcher and the
// Bark/Gotify HTTP notification channels. Real httptest servers verify the
// method, path/query, and payload each channel emits; the empty-config branches
// verify the "skip silently" contract (returns nil without sending).

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/LurusTech/lurus-hub/internal/pkg/common"
	"github.com/LurusTech/lurus-hub/internal/pkg/constant"
	"github.com/LurusTech/lurus-hub/internal/pkg/dto"
)

// noRedisNotify forces the notification-limit gate onto the in-memory path so
// tests don't touch a (nil) Redis client, and raises the per-hour notify limit
// above 0 (the env default is 0 = block-all). Restores both on cleanup.
func noRedisNotify(t *testing.T) {
	t.Helper()
	prevRedis := common.RedisEnabled
	prevLimit := constant.NotifyLimitCount
	common.RedisEnabled = false
	constant.NotifyLimitCount = 100
	t.Cleanup(func() {
		common.RedisEnabled = prevRedis
		constant.NotifyLimitCount = prevLimit
	})
}

func TestNotifyUser_BarkDelivers(t *testing.T) {
	noRedisNotify(t)
	allowLocalFetch(t)

	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path + "?" + r.URL.RawQuery
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	setting := dto.UserSetting{
		NotifyType: dto.NotifyTypeBark,
		// Bark URL template with {{title}}/{{content}} placeholders.
		BarkUrl: srv.URL + "/push/{{title}}/{{content}}",
	}
	data := dto.NewNotify(dto.NotifyTypeChannelTest, "Alert", "body text", nil)

	if err := NotifyUser(context.Background(), 90001, "", setting, data); err != nil {
		t.Fatalf("NotifyUser bark: %v", err)
	}
	if !strings.Contains(gotPath, "Alert") {
		t.Errorf("bark path %q should contain the escaped title", gotPath)
	}
}

func TestNotifyUser_BarkEmptyURLSkips(t *testing.T) {
	noRedisNotify(t)
	setting := dto.UserSetting{NotifyType: dto.NotifyTypeBark}
	// No BarkUrl => skip silently, no error.
	if err := NotifyUser(context.Background(), 90002, "", setting, dto.NewNotify("t", "s", "c", nil)); err != nil {
		t.Errorf("expected nil for empty bark url, got %v", err)
	}
}

func TestNotifyUser_GotifyDelivers(t *testing.T) {
	noRedisNotify(t)
	allowLocalFetch(t)

	var gotBody map[string]any
	var gotToken string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotToken = r.URL.Query().Get("token")
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	setting := dto.UserSetting{
		NotifyType:     dto.NotifyTypeGotify,
		GotifyUrl:      srv.URL,
		GotifyToken:    "tok123",
		GotifyPriority: 7,
	}
	data := dto.NewNotify(dto.NotifyTypeChannelTest, "GTitle", "GBody", nil)

	if err := NotifyUser(context.Background(), 90003, "", setting, data); err != nil {
		t.Fatalf("NotifyUser gotify: %v", err)
	}
	if gotToken != "tok123" {
		t.Errorf("gotify token = %q, want tok123", gotToken)
	}
	if gotBody["title"] != "GTitle" || gotBody["message"] != "GBody" {
		t.Errorf("gotify payload = %+v, want title=GTitle message=GBody", gotBody)
	}
	if p, ok := gotBody["priority"].(float64); !ok || int(p) != 7 {
		t.Errorf("gotify priority = %v, want 7", gotBody["priority"])
	}
}

func TestNotifyUser_GotifyMissingTokenSkips(t *testing.T) {
	noRedisNotify(t)
	setting := dto.UserSetting{
		NotifyType: dto.NotifyTypeGotify,
		GotifyUrl:  "http://example.com",
		// no token => skip
	}
	if err := NotifyUser(context.Background(), 90004, "", setting, dto.NewNotify("t", "s", "c", nil)); err != nil {
		t.Errorf("expected nil for missing gotify token, got %v", err)
	}
}

func TestNotifyUser_WebhookDelivers(t *testing.T) {
	noRedisNotify(t)
	allowLocalFetch(t)

	var hit bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hit = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	setting := dto.UserSetting{
		NotifyType: dto.NotifyTypeWebhook,
		WebhookUrl: srv.URL,
	}
	if err := NotifyUser(context.Background(), 90005, "", setting, dto.NewNotify("t", "s", "c", nil)); err != nil {
		t.Fatalf("NotifyUser webhook: %v", err)
	}
	if !hit {
		t.Error("webhook endpoint was not called")
	}
}

func TestNotifyUser_WebhookEmptyURLSkips(t *testing.T) {
	noRedisNotify(t)
	setting := dto.UserSetting{NotifyType: dto.NotifyTypeWebhook}
	if err := NotifyUser(context.Background(), 90006, "", setting, dto.NewNotify("t", "s", "c", nil)); err != nil {
		t.Errorf("expected nil for empty webhook url, got %v", err)
	}
}

func TestNotifyUser_EmailNoAddressSkips(t *testing.T) {
	noRedisNotify(t)
	// Default (empty) NotifyType falls back to email; with no address it must
	// skip silently rather than attempt an SMTP send.
	setting := dto.UserSetting{}
	if err := NotifyUser(context.Background(), 90007, "", setting, dto.NewNotify("t", "s", "c", nil)); err != nil {
		t.Errorf("expected nil for email with no address, got %v", err)
	}
}

// TestNotifyUser_EmailWithAddressAttemptsSend drives sendEmailNotify: with an
// address present and SMTP unconfigured in the test env, SendEmail surfaces a
// "not configured" error, so NotifyUser returns it (rather than silently
// skipping). This exercises the email placeholder-interpolation + send branch.
func TestNotifyUser_EmailWithAddressAttemptsSend(t *testing.T) {
	noRedisNotify(t)
	setting := dto.UserSetting{
		NotifyType:        dto.NotifyTypeEmail,
		NotificationEmail: "user@example.com",
	}
	// Values interpolate into the {{value}} placeholders before send.
	data := dto.NewNotify(dto.NotifyTypeQuotaExceed, "Low quota", "remaining "+dto.ContentValueParam, []interface{}{"$0.10"})
	err := NotifyUser(context.Background(), 90008, "", setting, data)
	if err == nil {
		t.Fatal("expected SMTP-not-configured error from sendEmailNotify")
	}
}

func TestSendBarkNotify_Non2xxErrors(t *testing.T) {
	noRedisNotify(t)
	allowLocalFetch(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	err := sendBarkNotify(srv.URL+"/push", dto.NewNotify("t", "s", "c", nil))
	if err == nil {
		t.Fatal("expected error on 502 bark response")
	}
}

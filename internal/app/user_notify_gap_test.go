package app

// user_notify_gap_test.go — closes the remaining NotifyUser/sender branches: the
// unknown notify-type no-op, and the transport-error arms of the Bark and Gotify
// senders (a dead endpoint that passes SSRF but refuses the connection).

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/LurusTech/lurus-hub/internal/pkg/dto"
)

func TestNotifyUser_UnknownTypeReturnsNil(t *testing.T) {
	noRedisNotify(t)

	setting := dto.UserSetting{NotifyType: "carrier-pigeon"} // unknown => no-op
	notify := dto.NewNotify(dto.NotifyTypeQuotaExceed, "t", "c", nil)
	if err := NotifyUser(context.Background(), uniqueUserID(), "u@example.com", setting, notify); err != nil {
		t.Errorf("unknown notify type should be a nil no-op, got %v", err)
	}
}

// deadURL returns the URL of an httptest server that has been closed, so a
// request to it passes SSRF (loopback allowed) but fails at connect.
func deadURL(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(nil)
	url := srv.URL
	srv.Close()
	return url
}

func TestSendBarkNotify_TransportErrorReturns(t *testing.T) {
	allowLocalFetch(t)
	notify := dto.NewNotify(dto.NotifyTypeQuotaExceed, "t", "c", nil)
	if err := sendBarkNotify(deadURL(t), notify); err == nil {
		t.Fatal("expected a transport error sending Bark to a dead endpoint")
	}
}

func TestSendGotifyNotify_TransportErrorReturns(t *testing.T) {
	allowLocalFetch(t)
	notify := dto.NewNotify(dto.NotifyTypeQuotaExceed, "t", "c", nil)
	if err := sendGotifyNotify(deadURL(t), "tok", 5, notify); err == nil {
		t.Fatal("expected a transport error sending Gotify to a dead endpoint")
	}
}

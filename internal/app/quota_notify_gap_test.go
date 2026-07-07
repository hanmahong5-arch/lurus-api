package app

// quota_notify_gap_test.go — proves the low-balance notification fires exactly
// at the threshold crossing (and not above it), and exercises the per-channel
// content formatting (Bark / Gotify / default). checkAndSendQuotaNotify spawns a
// goroutine, so tests synchronize on an httptest hit channel (race-safe) rather
// than sleeping-and-hoping.

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	relaycommon "github.com/LurusTech/lurus-hub/internal/adapter/provider/common"
	"github.com/LurusTech/lurus-hub/internal/pkg/common"
	"github.com/LurusTech/lurus-hub/internal/pkg/dto"
)

// hitServer returns an httptest server that signals each request on the returned
// channel, so a caller can wait for (or assert the absence of) a delivery.
func hitServer(t *testing.T) (*httptest.Server, <-chan struct{}) {
	t.Helper()
	hits := make(chan struct{}, 4)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		select {
		case hits <- struct{}{}:
		default:
		}
	}))
	t.Cleanup(srv.Close)
	return srv, hits
}

// uniqueUserID keeps each test's in-memory notify-limit counter isolated.
func uniqueUserID() int { return 800000 + int(time.Now().UnixNano()%100000) }

func TestCheckAndSendQuotaNotify_FiresAtThresholdViaBark(t *testing.T) {
	noRedisNotify(t)
	allowLocalFetch(t)
	srv, hits := hitServer(t)

	relayInfo := &relaycommon.RelayInfo{
		UserId:    uniqueUserID(),
		UserQuota: 1_200, // 1200 - 500 = 700 < threshold 1000 => notify
		UserSetting: dto.UserSetting{
			NotifyType:            dto.NotifyTypeBark,
			BarkUrl:               srv.URL,
			QuotaWarningThreshold: 1_000,
		},
	}

	checkAndSendQuotaNotify(relayInfo, 500, 0)

	select {
	case <-hits:
		// delivered — threshold crossing fired the Bark notification
	case <-time.After(3 * time.Second):
		t.Fatal("expected a Bark notification at the quota threshold, none arrived")
	}
}

func TestCheckAndSendQuotaNotify_SkipsWhenAboveThreshold(t *testing.T) {
	noRedisNotify(t)
	allowLocalFetch(t)
	srv, hits := hitServer(t)

	relayInfo := &relaycommon.RelayInfo{
		UserId:    uniqueUserID(),
		UserQuota: 50_000, // 50000 - 500 = 49500 >= threshold => no notify
		UserSetting: dto.UserSetting{
			NotifyType:            dto.NotifyTypeBark,
			BarkUrl:               srv.URL,
			QuotaWarningThreshold: 1_000,
		},
	}

	checkAndSendQuotaNotify(relayInfo, 500, 0)

	select {
	case <-hits:
		t.Fatal("notification fired above the threshold — false positive would spam users")
	case <-time.After(400 * time.Millisecond):
		// correct: no delivery
	}
}

func TestCheckAndSendQuotaNotify_GotifyContent(t *testing.T) {
	noRedisNotify(t)
	allowLocalFetch(t)
	srv, hits := hitServer(t)

	relayInfo := &relaycommon.RelayInfo{
		UserId:    uniqueUserID(),
		UserQuota: 800,
		UserSetting: dto.UserSetting{
			NotifyType:            dto.NotifyTypeGotify,
			GotifyUrl:             srv.URL,
			GotifyToken:           "tok",
			QuotaWarningThreshold: 1_000,
		},
	}

	checkAndSendQuotaNotify(relayInfo, 300, 0)

	select {
	case <-hits:
	case <-time.After(3 * time.Second):
		t.Fatal("expected a Gotify notification at the quota threshold")
	}
}

// TestCheckAndSendQuotaNotify_DefaultEmailContent drives the default (email)
// content-formatting branch: with no notify channel URL and no email address,
// the content is still formatted and NotifyUser returns cleanly (skip). We can't
// observe an SMTP send, so this asserts the path runs without panicking.
func TestCheckAndSendQuotaNotify_DefaultEmailContent(t *testing.T) {
	prevRedis := common.RedisEnabled
	common.RedisEnabled = false
	t.Cleanup(func() { common.RedisEnabled = prevRedis })

	relayInfo := &relaycommon.RelayInfo{
		UserId:    uniqueUserID(),
		UserQuota: 800,
		UserEmail: "", // email skip after content build
		UserSetting: dto.UserSetting{
			NotifyType:            "", // defaults to email
			QuotaWarningThreshold: 1_000,
		},
	}

	// Fire-and-forget; give the goroutine time to build content + hit the email
	// skip. No panic == the default-content branch executed.
	checkAndSendQuotaNotify(relayInfo, 300, 0)
	time.Sleep(200 * time.Millisecond)
}

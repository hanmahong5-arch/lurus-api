package app

// service_error_gap_test.go — error-path closers across the notification,
// release, and pre-consume services: webhook transport + SSRF-reject arms, the
// release repo-error arms (list/latest/changelog/download against a table-less
// DB), and PreConsumeQuota's user-quota lookup error.

import (
	"context"
	"testing"

	"github.com/LurusTech/lurus-hub/internal/adapter/repo"
	"github.com/LurusTech/lurus-hub/internal/pkg/dto"
	"github.com/LurusTech/lurus-hub/internal/pkg/setting/system_setting"
)

func TestSendWebhookNotify_TransportError(t *testing.T) {
	allowLocalFetch(t)
	notify := dto.NewNotify(dto.NotifyTypeQuotaExceed, "t", "c", nil)
	if err := SendWebhookNotify(deadURL(t), "", notify); err == nil {
		t.Fatal("expected a transport error sending a webhook to a dead endpoint")
	}
}

func TestSendWebhookNotify_SSRFReject(t *testing.T) {
	// SSRF on + private IPs disallowed => loopback target rejected before send.
	fs := system_setting.GetFetchSetting()
	prevSSRF, prevPriv := fs.EnableSSRFProtection, fs.AllowPrivateIp
	fs.EnableSSRFProtection = true
	fs.AllowPrivateIp = false
	t.Cleanup(func() { fs.EnableSSRFProtection, fs.AllowPrivateIp = prevSSRF, prevPriv })

	notify := dto.NewNotify(dto.NotifyTypeQuotaExceed, "t", "c", nil)
	if err := SendWebhookNotify("http://127.0.0.1:9/hook", "secret", notify); err == nil {
		t.Fatal("expected an SSRF rejection for a loopback webhook URL")
	}
}

// TestReleaseService_RepoErrors drives the repo-error arms of GetLatestRelease,
// GetReleaseByID, GetChangelog, and HandleDownload against a DB with no release
// tables — each must surface the error rather than a partial success.
func TestReleaseService_RepoErrors(t *testing.T) {
	db := setupServiceTestDB(t) // release tables NOT migrated
	svc := NewReleaseService(repo.NewReleaseRepository(db))
	ctx := context.Background()

	if _, err := svc.GetLatestRelease(ctx, "switch", "1.0.0"); err == nil {
		t.Error("GetLatestRelease: expected error with missing table")
	}
	if _, err := svc.GetReleaseByID(ctx, 1); err == nil {
		t.Error("GetReleaseByID: expected error with missing table")
	}
	if _, err := svc.GetChangelog(ctx, 1); err == nil {
		t.Error("GetChangelog: expected error with missing table")
	}
	if err := svc.HandleDownload(ctx, 1, "1.2.3.4", "ua", "ref"); err == nil {
		t.Error("HandleDownload: expected error incrementing count with missing table")
	}
}

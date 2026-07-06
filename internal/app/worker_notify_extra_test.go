package app

// worker_notify_extra_test.go — covers the worker-proxy delivery branch of the
// webhook / Bark / Gotify senders (system_setting.EnableWorker() == true), which
// routes the outbound request through the Cloudflare-worker relay instead of a
// direct dial. A single httptest server stands in for the worker.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/LurusTech/lurus-hub/internal/pkg/dto"
	"github.com/LurusTech/lurus-hub/internal/pkg/setting/system_setting"
)

// enableWorker points WorkerUrl at a test server and allows http targets,
// restoring the prior config on cleanup.
func enableWorker(t *testing.T, workerURL string) {
	t.Helper()
	if GetHttpClient() == nil {
		InitHttpClient()
	}
	prevURL := system_setting.WorkerUrl
	prevAllow := system_setting.WorkerAllowHttpImageRequestEnabled
	system_setting.WorkerUrl = workerURL
	system_setting.WorkerAllowHttpImageRequestEnabled = true
	t.Cleanup(func() {
		system_setting.WorkerUrl = prevURL
		system_setting.WorkerAllowHttpImageRequestEnabled = prevAllow
	})
}

func TestSendWebhookNotify_WorkerPath(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	enableWorker(t, srv.URL)

	err := SendWebhookNotify("https://hook.example.com/x", "secret", dto.NewNotify(dto.NotifyTypeChannelTest, "t", "c", nil))
	if err != nil {
		t.Fatalf("SendWebhookNotify worker: %v", err)
	}
	if hits == 0 {
		t.Error("worker endpoint was not called for webhook")
	}
}

func TestSendBarkNotify_WorkerPath(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	enableWorker(t, srv.URL)

	err := sendBarkNotify("https://bark.example.com/push/{{title}}/{{content}}", dto.NewNotify("t", "Title", "Body", nil))
	if err != nil {
		t.Fatalf("sendBarkNotify worker: %v", err)
	}
	if hits == 0 {
		t.Error("worker endpoint was not called for bark")
	}
}

func TestSendGotifyNotify_WorkerPath(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	enableWorker(t, srv.URL)

	err := sendGotifyNotify("https://gotify.example.com", "tok", 5, dto.NewNotify("t", "Title", "Body", nil))
	if err != nil {
		t.Fatalf("sendGotifyNotify worker: %v", err)
	}
	if hits == 0 {
		t.Error("worker endpoint was not called for gotify")
	}
}

// TestSendGotifyNotify_PriorityClamped exercises the out-of-range priority
// normalization (priority > 10 => 5) on the direct (non-worker) path.
func TestSendGotifyNotify_PriorityClamped(t *testing.T) {
	allowLocalFetch(t)
	var gotPriority float64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if p, ok := body["priority"].(float64); ok {
			gotPriority = p
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	if err := sendGotifyNotify(srv.URL, "tok", 99, dto.NewNotify("t", "T", "B", nil)); err != nil {
		t.Fatalf("sendGotifyNotify: %v", err)
	}
	if gotPriority != 5 {
		t.Errorf("priority = %v, want 5 (clamped from 99)", gotPriority)
	}
}

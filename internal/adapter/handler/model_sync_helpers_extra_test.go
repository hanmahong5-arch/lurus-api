package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/LurusTech/lurus-hub/internal/pkg/common"
)

func TestNormalizeLocale(t *testing.T) {
	tests := []struct {
		in   string
		want string
		ok   bool
	}{
		{"en", "en", true},
		{"ZH", "zh", true},
		{" ja ", "ja", true},
		{"fr", "", false},
		{"", "", false},
	}
	for _, tt := range tests {
		got, ok := normalizeLocale(tt.in)
		if got != tt.want || ok != tt.ok {
			t.Errorf("normalizeLocale(%q) = (%q,%v), want (%q,%v)", tt.in, got, ok, tt.want, tt.ok)
		}
	}
}

func TestGetUpstreamURLs(t *testing.T) {
	t.Setenv("SYNC_UPSTREAM_BASE", "https://example.test/meta/")
	if base := getUpstreamBase(); base != "https://example.test/meta/" {
		t.Errorf("getUpstreamBase = %q", base)
	}

	t.Run("with_locale", func(t *testing.T) {
		m, v := getUpstreamURLs("en")
		if m != "https://example.test/meta/api/i18n/en/newapi/models.json" {
			t.Errorf("models URL = %q", m)
		}
		if v != "https://example.test/meta/api/i18n/en/newapi/vendors.json" {
			t.Errorf("vendors URL = %q", v)
		}
	})

	t.Run("no_locale", func(t *testing.T) {
		m, v := getUpstreamURLs("klingon")
		if m != "https://example.test/meta/api/newapi/models.json" {
			t.Errorf("models URL = %q", m)
		}
		if v != "https://example.test/meta/api/newapi/vendors.json" {
			t.Errorf("vendors URL = %q", v)
		}
	})
}

func TestContainsField(t *testing.T) {
	fields := []string{"Description", " icon ", "Tags"}
	if !containsField(fields, "description") {
		t.Error("expected case-insensitive match for description")
	}
	if !containsField(fields, "ICON") {
		t.Error("expected trim+case match for icon")
	}
	if containsField(fields, "status") {
		t.Error("did not expect match for status")
	}
	if containsField(nil, "x") {
		t.Error("empty fields should never match")
	}
}

func TestCoalesce(t *testing.T) {
	if coalesce("a", "b") != "a" {
		t.Error("non-empty primary should win")
	}
	if coalesce("  ", "fallback") != "fallback" {
		t.Error("blank primary should fall back")
	}
	if coalesce("", "") != "" {
		t.Error("both empty -> empty")
	}
}

func TestChooseStatus(t *testing.T) {
	if chooseStatus(0, 2) != 2 {
		t.Error("primary 0 -> fallback")
	}
	if chooseStatus(5, 2) != 5 {
		t.Error("nonzero primary wins")
	}
	if chooseStatus(0, 0) != 1 {
		t.Error("both zero -> default 1")
	}
}

func TestFetchJSON_OK_Envelope(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"data":[{"name":"acme","status":1}]}`))
	}))
	defer srv.Close()

	var env upstreamEnvelope[upstreamVendor]
	if err := fetchJSON(context.Background(), srv.URL, &env); err != nil {
		t.Fatalf("fetchJSON err: %v", err)
	}
	if !env.Success || len(env.Data) != 1 || env.Data[0].Name != "acme" {
		t.Errorf("unexpected envelope: %+v", env)
	}
}

func TestFetchJSON_OK_BareArray(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"name":"beta","status":2}]`))
	}))
	defer srv.Close()

	var env upstreamEnvelope[upstreamVendor]
	if err := fetchJSON(context.Background(), srv.URL, &env); err != nil {
		t.Fatalf("fetchJSON err: %v", err)
	}
	if !env.Success || len(env.Data) != 1 || env.Data[0].Name != "beta" {
		t.Errorf("bare-array decode failed: %+v", env)
	}
}

func TestFetchJSON_ServerError(t *testing.T) {
	// keep retry count low so the test is fast
	t.Setenv("SYNC_HTTP_RETRY", "1")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	var env upstreamEnvelope[upstreamVendor]
	err := fetchJSON(context.Background(), srv.URL, &env)
	if err == nil {
		t.Fatal("expected error on 500 response")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error should carry status: %v", err)
	}
}

func TestFetchJSON_304UsesCache(t *testing.T) {
	t.Setenv("SYNC_HTTP_RETRY", "1")
	first := true
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if first {
			w.Header().Set("ETag", `"v1"`)
			_, _ = w.Write([]byte(`{"success":true,"data":[{"name":"cached","status":1}]}`))
			first = false
			return
		}
		if r.Header.Get("If-None-Match") == `"v1"` {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	var env1 upstreamEnvelope[upstreamVendor]
	if err := fetchJSON(context.Background(), srv.URL, &env1); err != nil {
		t.Fatalf("first fetch err: %v", err)
	}
	var env2 upstreamEnvelope[upstreamVendor]
	if err := fetchJSON(context.Background(), srv.URL, &env2); err != nil {
		t.Fatalf("second fetch (304) err: %v", err)
	}
	if len(env2.Data) != 1 || env2.Data[0].Name != "cached" {
		t.Errorf("304 path should reuse cached body: %+v", env2)
	}
}

// Guard against accidental default-env dependency in this test file.
func TestGetUpstreamBase_Default(t *testing.T) {
	// Ensure the env is unset for the default branch.
	t.Setenv("SYNC_UPSTREAM_BASE", "")
	got := getUpstreamBase()
	if got != "https://basellm.github.io/llm-metadata" {
		t.Errorf("default base = %q", got)
	}
	_ = common.GetEnvOrDefaultString // keep import used if refactored
}

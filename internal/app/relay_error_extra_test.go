package app

// relay_error_extra_test.go — covers RelayErrorHandler, which turns an upstream
// error *http.Response into a NewAPIError. No network needed: responses are
// constructed in-memory. Branches: structured OpenAI-style error object,
// non-JSON body (both showBody modes), and oversized-body truncation into the
// UpstreamBodyHint.

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func makeResp(status int, body string, header http.Header) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     header,
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestRelayErrorHandler_StructuredOpenAIError(t *testing.T) {
	body := `{"error":{"message":"rate limit exceeded","type":"rate_limit_error","code":"429"}}`
	h := http.Header{"X-Ratelimit-Reset": {"60"}}
	resp := makeResp(http.StatusTooManyRequests, body, h)
	defer func() { _ = resp.Body.Close() }()
	apiErr := RelayErrorHandler(context.Background(), resp, true)

	if apiErr == nil {
		t.Fatal("expected a NewAPIError")
	}
	if apiErr.StatusCode != http.StatusTooManyRequests {
		t.Errorf("status = %d, want 429", apiErr.StatusCode)
	}
	if !strings.Contains(apiErr.Error(), "rate limit exceeded") {
		t.Errorf("error %q should carry the upstream message", apiErr.Error())
	}
	// Upstream header must be stashed for downstream cooldown logic.
	if apiErr.UpstreamHeader.Get("X-Ratelimit-Reset") != "60" {
		t.Errorf("UpstreamHeader not preserved: %v", apiErr.UpstreamHeader)
	}
	if !strings.Contains(apiErr.UpstreamBodyHint, "rate limit exceeded") {
		t.Errorf("UpstreamBodyHint = %q, want the raw body", apiErr.UpstreamBodyHint)
	}
}

func TestRelayErrorHandler_NonJSONBody_ShowBody(t *testing.T) {
	resp := makeResp(http.StatusBadGateway, "upstream is down", nil)
	defer func() { _ = resp.Body.Close() }()
	apiErr := RelayErrorHandler(context.Background(), resp, true)
	if apiErr == nil {
		t.Fatal("expected a NewAPIError")
	}
	if apiErr.StatusCode != http.StatusBadGateway {
		t.Errorf("status = %d, want 502", apiErr.StatusCode)
	}
	// With showBodyWhenFail, the raw non-JSON body is surfaced in the error.
	if !strings.Contains(apiErr.Error(), "upstream is down") {
		t.Errorf("error %q should include the raw body", apiErr.Error())
	}
}

func TestRelayErrorHandler_NonJSONBody_HideBody(t *testing.T) {
	resp := makeResp(http.StatusInternalServerError, "not json at all", nil)
	defer func() { _ = resp.Body.Close() }()
	apiErr := RelayErrorHandler(context.Background(), resp, false)
	if apiErr == nil {
		t.Fatal("expected a NewAPIError")
	}
	// Without showBodyWhenFail the message is generic (body not leaked to caller),
	// but the raw body is still retained in UpstreamBodyHint for diagnostics.
	if strings.Contains(apiErr.Error(), "not json at all") {
		t.Errorf("error %q must not leak the raw body when showBody=false", apiErr.Error())
	}
	if apiErr.UpstreamBodyHint != "not json at all" {
		t.Errorf("UpstreamBodyHint = %q, want raw body retained", apiErr.UpstreamBodyHint)
	}
}

func TestRelayErrorHandler_OversizedBodyTruncated(t *testing.T) {
	big := strings.Repeat("x", 20*1024) // > 16KB cap
	resp := makeResp(http.StatusBadGateway, big, nil)
	defer func() { _ = resp.Body.Close() }()
	apiErr := RelayErrorHandler(context.Background(), resp, false)
	if apiErr == nil {
		t.Fatal("expected a NewAPIError")
	}
	if len(apiErr.UpstreamBodyHint) != 16*1024 {
		t.Errorf("UpstreamBodyHint len = %d, want 16384 (truncated)", len(apiErr.UpstreamBodyHint))
	}
}

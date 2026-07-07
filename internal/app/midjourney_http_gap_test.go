package app

// midjourney_http_gap_test.go — drives DoMidjourneyHttpRequest against httptest
// servers: success (valid MJ JSON), the invalid-request-body decode error, the
// transport (Do) error, the empty-response-body arm, and the both-unmarshals-fail
// arm. These are the proxy's error-classification branches that map upstream
// failures to MJ error envelopes.

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func mjContext(method, url, body string) *gin.Context {
	c := createTestGinContext()
	c.Request = httptest.NewRequest(method, "/mj", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	return c
}

func TestDoMidjourneyHttpRequest_Success(t *testing.T) {
	allowLocalFetch(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"code":1,"description":"submitted"}`))
	}))
	defer srv.Close()

	c := mjContext(http.MethodPost, srv.URL, `{"prompt":"a cat"}`)
	resp, body, err := DoMidjourneyHttpRequest(c, 5*time.Second, srv.URL)
	if err != nil {
		t.Fatalf("DoMidjourneyHttpRequest: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	if !strings.Contains(string(body), "submitted") {
		t.Errorf("body = %q, want it to contain the upstream description", string(body))
	}
}

func TestDoMidjourneyHttpRequest_InvalidRequestBody(t *testing.T) {
	allowLocalFetch(t)
	// Non-JSON POST body => the initial decode fails before any upstream call.
	c := mjContext(http.MethodPost, "http://127.0.0.1:0", `not-json`)
	resp, _, err := DoMidjourneyHttpRequest(c, 5*time.Second, "http://127.0.0.1:0")
	if err == nil {
		t.Fatal("expected a decode error for an invalid request body")
	}
	if resp == nil || resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected a 500 MJ error envelope, got %+v", resp)
	}
}

func TestDoMidjourneyHttpRequest_TransportError(t *testing.T) {
	allowLocalFetch(t)
	dead := deadURL(t) // closed server => connection refused
	c := mjContext(http.MethodPost, dead, `{"prompt":"x"}`)
	_, _, err := DoMidjourneyHttpRequest(c, 2*time.Second, dead)
	if err == nil {
		t.Fatal("expected a transport error against a dead endpoint")
	}
}

func TestDoMidjourneyHttpRequest_EmptyResponseBody(t *testing.T) {
	allowLocalFetch(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK) // empty body
	}))
	defer srv.Close()

	c := mjContext(http.MethodPost, srv.URL, `{"prompt":"x"}`)
	resp, _, err := DoMidjourneyHttpRequest(c, 5*time.Second, srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Response.Description != "empty_response_body" {
		t.Errorf("description = %q, want empty_response_body", resp.Response.Description)
	}
}

func TestDoMidjourneyHttpRequest_UnmarshalFails(t *testing.T) {
	allowLocalFetch(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`}{not valid json`))
	}))
	defer srv.Close()

	c := mjContext(http.MethodPost, srv.URL, `{"prompt":"x"}`)
	resp, _, err := DoMidjourneyHttpRequest(c, 5*time.Second, srv.URL)
	if err == nil {
		t.Fatal("expected an unmarshal error for a non-JSON upstream body")
	}
	if resp.Response.Description != "unmarshal_response_body_failed" {
		t.Errorf("description = %q, want unmarshal_response_body_failed", resp.Response.Description)
	}
}

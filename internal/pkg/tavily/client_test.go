package tavily

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNew_RejectsEmptyKey(t *testing.T) {
	if _, err := New(""); err == nil {
		t.Fatalf("expected error for empty api key, got nil")
	}
}

func TestNew_AcceptsKeyAndDefaults(t *testing.T) {
	c, err := New("tvly-test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.endpoint != DefaultEndpoint {
		t.Errorf("endpoint = %q, want %q", c.endpoint, DefaultEndpoint)
	}
	if c.http == nil || c.http.Timeout != DefaultTimeout {
		t.Errorf("expected default timeout %v, got %v", DefaultTimeout, c.http.Timeout)
	}
}

func TestSearch_RejectsEmptyQuery(t *testing.T) {
	c, _ := New("tvly-test")
	_, err := c.Search(context.Background(), SearchRequest{})
	if err == nil {
		t.Fatalf("expected error for empty query")
	}
}

func TestSearch_HappyPath(t *testing.T) {
	var receivedBody []byte
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("content-type = %q, want application/json", got)
		}
		receivedBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"answer": "Bitcoin is currently at $50k.",
			"results": [
				{"title": "BTC Price", "url": "https://example.com/btc", "content": "BTC = $50k", "score": 0.9}
			]
		}`))
	}))
	defer ts.Close()

	c, _ := New("tvly-secret", WithEndpoint(ts.URL))
	resp, err := c.Search(context.Background(), SearchRequest{
		Query:         "BTC price today",
		IncludeAnswer: true,
		MaxResults:    3,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Answer == "" || len(resp.Results) != 1 {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if resp.Results[0].URL != "https://example.com/btc" {
		t.Errorf("url mismatch: %q", resp.Results[0].URL)
	}

	// Verify the api_key was injected into the body alongside the request.
	var sent map[string]any
	if err := json.Unmarshal(receivedBody, &sent); err != nil {
		t.Fatalf("could not parse sent body: %v", err)
	}
	if sent["api_key"] != "tvly-secret" {
		t.Errorf("api_key = %v, want tvly-secret", sent["api_key"])
	}
	if sent["query"] != "BTC price today" {
		t.Errorf("query = %v, want BTC price today", sent["query"])
	}
	// Defaults applied: search_depth=basic, max_results stayed at 3.
	if sent["search_depth"] != "basic" {
		t.Errorf("search_depth = %v, want basic", sent["search_depth"])
	}
}

func TestSearch_AppliesDefaults(t *testing.T) {
	var sentBody []byte
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sentBody, _ = io.ReadAll(r.Body)
		_, _ = w.Write([]byte(`{"answer":"","results":[]}`))
	}))
	defer ts.Close()

	c, _ := New("k", WithEndpoint(ts.URL))
	_, _ = c.Search(context.Background(), SearchRequest{Query: "hi"})

	var sent map[string]any
	_ = json.Unmarshal(sentBody, &sent)
	if sent["search_depth"] != "basic" {
		t.Errorf("default search_depth not applied: %v", sent["search_depth"])
	}
	if v, ok := sent["max_results"].(float64); !ok || int(v) != 5 {
		t.Errorf("default max_results not applied: %v", sent["max_results"])
	}
}

func TestSearch_PropagatesNon200(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"invalid api key"}`))
	}))
	defer ts.Close()

	c, _ := New("bad", WithEndpoint(ts.URL))
	_, err := c.Search(context.Background(), SearchRequest{Query: "x"})
	if err == nil {
		t.Fatalf("expected error for 401, got nil")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("error should mention 401: %v", err)
	}
}

func TestSearch_HonorsContextCancel(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Block long enough that the context will be cancelled first.
		time.Sleep(300 * time.Millisecond)
	}))
	defer ts.Close()

	c, _ := New("k", WithEndpoint(ts.URL))
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := c.Search(ctx, SearchRequest{Query: "x"})
	if err == nil {
		t.Fatalf("expected context-cancel error, got nil")
	}
}

func TestSearch_MalformedJSONReturnsError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`not json at all`))
	}))
	defer ts.Close()

	c, _ := New("k", WithEndpoint(ts.URL))
	_, err := c.Search(context.Background(), SearchRequest{Query: "x"})
	if err == nil {
		t.Fatalf("expected decode error, got nil")
	}
}

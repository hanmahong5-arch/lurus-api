// Package tavily wraps the Tavily Search API (https://api.tavily.com) for
// use as a server-side web_search tool. The client is best-effort: every
// public method either returns a populated SearchResponse or a wrapped
// error, never panics, never blocks past the configured timeout.
//
// Auth: bearer-style API key in the JSON body's `api_key` field per
// Tavily's REST spec. The key is read at construction time so callers
// can fail-fast at startup when it's missing.
package tavily

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// DefaultEndpoint is Tavily's production search URL. Override in tests or
// when routing through an outbound proxy by constructing the client with
// WithEndpoint.
const DefaultEndpoint = "https://api.tavily.com/search"

// DefaultTimeout caps the total round-trip including network + Tavily's
// own LLM summarization. Five seconds is generous; the typical
// Tavily search returns in 1-3s. Beyond this we fail open and let the
// caller decide whether to retry or fall through to a non-search answer.
const DefaultTimeout = 5 * time.Second

// SearchDepth controls how aggressively Tavily expands its result set.
// "basic" is faster + cheaper; "advanced" is deeper but ~2x latency.
type SearchDepth string

const (
	DepthBasic    SearchDepth = "basic"
	DepthAdvanced SearchDepth = "advanced"
)

// SearchRequest is the shape we send to Tavily. Only Query is required;
// the rest have sensible Tavily-side defaults but we pass them
// explicitly so the behavior matches across versions.
type SearchRequest struct {
	Query         string      `json:"query"`
	SearchDepth   SearchDepth `json:"search_depth,omitempty"`
	IncludeAnswer bool        `json:"include_answer,omitempty"`
	MaxResults    int         `json:"max_results,omitempty"`
}

// SearchResult is one entry in Tavily's results array.
type SearchResult struct {
	Title   string  `json:"title"`
	URL     string  `json:"url"`
	Content string  `json:"content"`
	Score   float64 `json:"score"`
}

// SearchResponse is the subset of Tavily's response we care about. We
// deliberately drop `query`, `response_time`, `images` etc. since they
// add bytes without value to the chat tool result.
type SearchResponse struct {
	// Answer is Tavily's LLM-generated 2-3 sentence summary. Empty when
	// IncludeAnswer was false or Tavily declined to summarize.
	Answer string `json:"answer"`
	// Results is the per-source list, ordered by Tavily's relevance score
	// descending.
	Results []SearchResult `json:"results"`
}

// Client wraps an *http.Client + Tavily API key + endpoint. Safe for
// concurrent use after construction.
type Client struct {
	apiKey   string
	endpoint string
	http     *http.Client
}

// Option configures a Client.
type Option func(*Client)

// WithEndpoint overrides the default Tavily URL. Tests use this to point
// at an httptest.Server.
func WithEndpoint(url string) Option {
	return func(c *Client) { c.endpoint = url }
}

// WithHTTPClient injects a custom *http.Client. Use this to wire the
// outbound proxy (HTTP_PROXY) or to plug a recorder for golden tests.
func WithHTTPClient(h *http.Client) Option {
	return func(c *Client) { c.http = h }
}

// New constructs a Client. An empty apiKey returns nil + error so the
// caller fails fast at startup; we never return a half-configured Client
// that silently sends keyless requests.
func New(apiKey string, opts ...Option) (*Client, error) {
	if apiKey == "" {
		return nil, errors.New("tavily: api key is required")
	}
	c := &Client{
		apiKey:   apiKey,
		endpoint: DefaultEndpoint,
		http:     &http.Client{Timeout: DefaultTimeout},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c, nil
}

// Search executes a single search. The returned SearchResponse is always
// non-nil on success; failure paths return a wrapped error suitable for
// %w-unwrapping. The ctx deadline supersedes the client timeout when
// shorter.
func (c *Client) Search(ctx context.Context, req SearchRequest) (*SearchResponse, error) {
	if req.Query == "" {
		return nil, errors.New("tavily: query is required")
	}
	// Apply sensible defaults if caller didn't.
	if req.SearchDepth == "" {
		req.SearchDepth = DepthBasic
	}
	if req.MaxResults == 0 {
		req.MaxResults = 5
	}

	// Body wraps the request + injects the api key.
	body := struct {
		SearchRequest
		APIKey string `json:"api_key"`
	}{SearchRequest: req, APIKey: c.apiKey}

	buf, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("tavily: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(buf))
	if err != nil {
		return nil, fmt.Errorf("tavily: build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("tavily: do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("tavily: status %d: %s", resp.StatusCode, string(respBody))
	}

	var out SearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("tavily: decode response: %w", err)
	}
	return &out, nil
}

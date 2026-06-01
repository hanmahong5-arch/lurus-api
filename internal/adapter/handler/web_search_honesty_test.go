package handler

// web_search_honesty_test.go — honesty lock for LUTU-02.
//
// Claim under audit:
//
//	"newhub /api/v2/lutu/search is live; Tavily real results verified; the
//	 handler returns 503 search_disabled when TAVILY_API_KEY is missing and
//	 forwards results otherwise."
//
// The existing web_search_test.go covers: missing-query 400, happy path,
// search_disabled 503, upstream 502, depth=advanced forwarding. This file does
// NOT re-assert those. It locks the LUTU-02 edge list AND records an honest
// finding about two of the claimed edges:
//
//   - "sources 截断5条" / "limit 越界": the BMAD plan lists these as expected
//     behaviour, but the SOURCE (web_search.go PostWebSearch) does NOT cap or
//     truncate — it forwards req.MaxResults verbatim to Tavily and returns
//     EVERY result Tavily sends back. So the honest lock asserts the ACTUAL
//     contract (no server-side truncation; out-of-range limit is forwarded,
//     not rejected) and flags the divergence from the plan rather than
//     fabricating a truncation the code doesn't perform (CLAUDE.md §4.1⑥:
//     marker ≠ behaviour).
//   - empty query → 400 (binding:"required") even when a non-whitespace key is
//     present, proving validation runs before the upstream call.
//   - upstream deadline/timeout → 502 with a body the caller can act on
//     (retryable upstream_error), distinct from the 503 disabled case.
//
// Subject = PostWebSearch behind a stub Tavily server; assertions are on HTTP
// status + response/forwarded body, independent of handler internals.

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// TestPostWebSearch_NoTavilyKey_Returns503Disabled re-confirms the headline
// LUTU-02 claim from a fresh env: with TAVILY_API_KEY empty the handler returns
// 503 search_disabled and never panics (a panic here would crash the gin
// worker / the pod). Distinct from the existing disabled test in that it also
// asserts the process survives (no panic) and the body is the structured
// fallback shape Lutu APP keys on.
func TestPostWebSearch_NoTavilyKey_Returns503Disabled(t *testing.T) {
	t.Setenv("TAVILY_API_KEY", "")
	resetWebSearchClientForTesting()
	t.Cleanup(resetWebSearchClientForTesting)

	c, w := newReqCtx(t, map[string]any{"query": "live btc price"})

	// Must not panic — recover converts a panic into a test failure with context.
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("PostWebSearch panicked with no key (would crash the pod): %v", r)
		}
	}()
	PostWebSearch(c)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503 when TAVILY_API_KEY is empty", w.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["error"] != "search_disabled" {
		t.Fatalf("error = %v, want search_disabled (the fallback marker Lutu keys on)", body["error"])
	}
}

// TestPostWebSearch_EmptyQuery_400 pins that validation precedes any upstream
// call. The stub fatals if hit, so reaching Tavily with an empty query would
// fail the test — proving the binding:"required" guard is the first gate.
func TestPostWebSearch_EmptyQuery_400(t *testing.T) {
	defer withTavilyStub(t, func(http.ResponseWriter, *http.Request) {
		t.Fatalf("Tavily must not be called for an empty query")
	})()

	c, w := newReqCtx(t, map[string]any{"query": ""})
	PostWebSearch(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for empty query", w.Code)
	}
}

// TestPostWebSearch_NoServerSideTruncation is the honest correction to the
// plan's "sources 截断5条" edge. The handler does NOT truncate: when Tavily
// returns 8 results the caller receives all 8. This test LOCKS the real
// behaviour so a future change that silently starts dropping results is caught,
// and documents that the "5-result cap" lives (if anywhere) in Tavily's
// max_results default, not in newhub.
func TestPostWebSearch_NoServerSideTruncation(t *testing.T) {
	var forwarded map[string]any
	defer withTavilyStub(t, func(w http.ResponseWriter, r *http.Request) {
		dec := json.NewDecoder(r.Body)
		_ = dec.Decode(&forwarded)
		// Return EIGHT results — more than the conventional 5 default.
		_, _ = w.Write([]byte(`{
			"answer": "many sources",
			"results": [
				{"title":"r1","url":"https://e/1","content":"c","score":0.9},
				{"title":"r2","url":"https://e/2","content":"c","score":0.8},
				{"title":"r3","url":"https://e/3","content":"c","score":0.7},
				{"title":"r4","url":"https://e/4","content":"c","score":0.6},
				{"title":"r5","url":"https://e/5","content":"c","score":0.5},
				{"title":"r6","url":"https://e/6","content":"c","score":0.4},
				{"title":"r7","url":"https://e/7","content":"c","score":0.3},
				{"title":"r8","url":"https://e/8","content":"c","score":0.2}
			]
		}`))
	})()

	// Caller asks for 50 — an out-of-the-conventional-range value.
	c, w := newReqCtx(t, map[string]any{"query": "x", "max_results": 50})
	PostWebSearch(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", w.Code, w.Body.String())
	}
	var resp WebSearchResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	// HONEST CONTRACT: no server-side truncation. All 8 forwarded results return.
	if len(resp.Results) != 8 {
		t.Fatalf("results = %d, want 8 (handler does NOT truncate; plan's '5-cap' is not enforced here)", len(resp.Results))
	}
	// And the out-of-range limit is forwarded verbatim to Tavily, not rejected
	// or clamped by newhub — proving the cap (if any) is Tavily's concern.
	if got, _ := forwarded["max_results"].(float64); got != 50 {
		t.Fatalf("forwarded max_results = %v, want 50 (handler forwards the limit verbatim, no clamp)", forwarded["max_results"])
	}
}

// TestPostWebSearch_UpstreamTimeout_RetryableError covers the "上游 Tavily
// 超时→可重试错" edge. A 5xx/connection-style upstream failure surfaces as a
// 502 upstream_error — a status the Lutu APP can retry, distinct from the 503
// disabled (do-not-retry) and 400 bad-request (do-not-retry) cases.
func TestPostWebSearch_UpstreamTimeout_RetryableError(t *testing.T) {
	defer withTavilyStub(t, func(w http.ResponseWriter, _ *http.Request) {
		// Tavily returns a 5xx — the realistic "upstream slow/erroring" mode.
		w.WriteHeader(http.StatusGatewayTimeout)
		_, _ = w.Write([]byte(`{"error":"upstream timed out"}`))
	})()

	c, w := newReqCtx(t, map[string]any{"query": "x"})
	PostWebSearch(c)

	if w.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502 (retryable upstream error)", w.Code)
	}
	if !strings.Contains(w.Body.String(), "upstream_error") {
		t.Fatalf("body missing upstream_error marker: %s", w.Body.String())
	}
}

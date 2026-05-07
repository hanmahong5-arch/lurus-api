package relay

import (
	"bytes"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// newTestWriter returns a gin.ResponseWriter wired to an httptest recorder.
// Uses Gin's real ResponseWriter (via gin.CreateTestContext) so embedded-method
// forwarding (Header, WriteHeader, Flush, Status, Size, Written, ...) is
// exercised exactly as in production.
func newTestWriter(t *testing.T) (gin.ResponseWriter, *httptest.ResponseRecorder) {
	t.Helper()
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	return c.Writer, rec
}

func TestBufferedResponseWriter_WritePassesThroughByteIdentical(t *testing.T) {
	// Contract: bytes the user sees must equal bytes the caller wrote. Wrapping
	// must NOT alter, reorder, or mutate any byte.
	innerA, recA := newTestWriter(t)
	w := NewBufferedResponseWriter(innerA, 1024)

	payload := []byte(`{"data":[{"url":"https://cdn.example/a.png"}]}`)
	n, err := w.Write(payload)
	if err != nil {
		t.Fatalf("Write err: %v", err)
	}
	if n != len(payload) {
		t.Fatalf("Write n=%d, want %d", n, len(payload))
	}

	innerB, recB := newTestWriter(t)
	if _, err := innerB.Write(payload); err != nil {
		t.Fatalf("baseline Write err: %v", err)
	}

	if !bytes.Equal(recA.Body.Bytes(), recB.Body.Bytes()) {
		t.Errorf("wrapped bytes differ from baseline:\n wrapped:  %q\n baseline: %q",
			recA.Body.String(), recB.Body.String())
	}
	if !bytes.Equal(w.Bytes(), payload) {
		t.Errorf("buffer mismatch: got %q, want %q", w.Bytes(), payload)
	}
}

func TestBufferedResponseWriter_WriteStringPassesThrough(t *testing.T) {
	inner, rec := newTestWriter(t)
	w := NewBufferedResponseWriter(inner, 1024)

	s := `{"output":["https://cdn.example/r.png"]}`
	n, err := w.WriteString(s)
	if err != nil {
		t.Fatalf("WriteString err: %v", err)
	}
	if n != len(s) {
		t.Fatalf("WriteString n=%d, want %d", n, len(s))
	}
	if rec.Body.String() != s {
		t.Errorf("user-visible bytes mismatch: got %q want %q", rec.Body.String(), s)
	}
	if string(w.Bytes()) != s {
		t.Errorf("buffer: got %q, want %q", w.Bytes(), s)
	}
}

func TestBufferedResponseWriter_CapBoundsBufferButForwardsAllBytes(t *testing.T) {
	// When upstream writes past the cap, every byte MUST still forward to the
	// user. Only the in-memory buffer is bounded.
	inner, rec := newTestWriter(t)
	const cap = 64
	w := NewBufferedResponseWriter(inner, cap)

	big := bytes.Repeat([]byte("X"), cap*4)
	if _, err := w.Write(big); err != nil {
		t.Fatalf("Write err: %v", err)
	}

	if rec.Body.Len() != len(big) {
		t.Errorf("user-visible len=%d, want %d (forwarding broke under cap)",
			rec.Body.Len(), len(big))
	}
	if w.buf.Len() != cap {
		t.Errorf("buffer len=%d, want capped at %d", w.buf.Len(), cap)
	}
	if !w.Truncated() {
		t.Error("Truncated() should be true after exceeding cap")
	}
}

func TestBufferedResponseWriter_DefaultCapWhenNonPositive(t *testing.T) {
	inner, _ := newTestWriter(t)
	w0 := NewBufferedResponseWriter(inner, 0)
	if w0.cap != CapturedBodyMaxBytes {
		t.Errorf("cap=%d, want default %d", w0.cap, CapturedBodyMaxBytes)
	}
	wNeg := NewBufferedResponseWriter(inner, -5)
	if wNeg.cap != CapturedBodyMaxBytes {
		t.Errorf("cap=%d, want default %d", wNeg.cap, CapturedBodyMaxBytes)
	}
}

func TestBufferedResponseWriter_HeaderMethodsForwardViaEmbedding(t *testing.T) {
	// Embedded gin.ResponseWriter must keep working for Header/WriteHeader/Flush.
	inner, rec := newTestWriter(t)
	w := NewBufferedResponseWriter(inner, 1024)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(201)
	if _, err := w.Write([]byte(`{}`)); err != nil {
		t.Fatalf("Write err: %v", err)
	}
	w.Flush()

	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type=%q", got)
	}
	if rec.Code != 201 {
		t.Errorf("status=%d, want 201", rec.Code)
	}
}

func TestExtractImageURL_OpenAIDALLEShape(t *testing.T) {
	body := []byte(`{
		"created": 1700000000,
		"data": [{"url": "https://cdn.example/dalle-1.png"}, {"url": "https://cdn.example/dalle-2.png"}]
	}`)
	got := ExtractImageURL(body)
	want := "https://cdn.example/dalle-1.png"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestExtractImageURL_OpenAIDataWithB64NoURLReturnsEmpty(t *testing.T) {
	// response_format=b64_json — no URL to extract; must quietly return "".
	body := []byte(`{"data":[{"b64_json":"aGVsbG8="}]}`)
	if got := ExtractImageURL(body); got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestExtractImageURL_ReplicateArrayShape(t *testing.T) {
	body := []byte(`{"id":"abc","status":"succeeded","output":["https://replicate.delivery/x.webp","https://replicate.delivery/y.webp"]}`)
	got := ExtractImageURL(body)
	want := "https://replicate.delivery/x.webp"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestExtractImageURL_ReplicateStringOutput(t *testing.T) {
	body := []byte(`{"id":"abc","status":"succeeded","output":"https://replicate.delivery/single.png"}`)
	got := ExtractImageURL(body)
	want := "https://replicate.delivery/single.png"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestExtractImageURL_AliShapeViaOpenAIData(t *testing.T) {
	// Ali adaptor normalizes to dto.ImageResponse{Data:[]ImageData{Url:...}}
	// before writing — captured body matches OpenAI shape.
	body := []byte(`{"created":1700000000,"data":[{"url":"https://dashscope-result.oss.aliyuncs.com/abc.png"}],"metadata":{"task_id":"xyz"}}`)
	got := ExtractImageURL(body)
	want := "https://dashscope-result.oss.aliyuncs.com/abc.png"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestExtractImageURL_JimengShapeViaOpenAIData(t *testing.T) {
	body := []byte(`{"created":1700000000,"data":[{"url":"https://jimeng.cdn.bytedance.com/abc.jpeg"}]}`)
	if got := ExtractImageURL(body); got != "https://jimeng.cdn.bytedance.com/abc.jpeg" {
		t.Errorf("got %q", got)
	}
}

func TestExtractImageURL_GenericFallbackDepth1(t *testing.T) {
	// Future provider with flat {"image_url":"..."} shape.
	body := []byte(`{"image_url":"https://provider.example/img.png","seed":42}`)
	got := ExtractImageURL(body)
	want := "https://provider.example/img.png"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestExtractImageURL_EmptyBodyReturnsEmpty(t *testing.T) {
	if got := ExtractImageURL(nil); got != "" {
		t.Errorf("nil → %q, want empty", got)
	}
	if got := ExtractImageURL([]byte{}); got != "" {
		t.Errorf("empty → %q, want empty", got)
	}
	if got := ExtractImageURL([]byte("   \n  ")); got != "" {
		t.Errorf("whitespace → %q, want empty", got)
	}
}

func TestExtractImageURL_GarbageBodyReturnsEmptyNoPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("ExtractImageURL panicked on garbage: %v", r)
		}
	}()
	cases := [][]byte{
		[]byte("not json at all"),
		[]byte("{"),
		[]byte("[1,2,3]"),                      // top-level array, not object
		[]byte(`{"data": "not-an-array"}`),     // wrong shape under "data"
		[]byte(`{"data": [{"url": 42}]}`),      // url is a number
		[]byte(`{"data": [{"url": null}]}`),    // url is null
		[]byte(`{"output": {"weird": true}}`),  // output is object
	}
	for i, c := range cases {
		if got := ExtractImageURL(c); got != "" {
			t.Errorf("case %d: got %q, want empty (input=%s)", i, got, string(c))
		}
	}
}

func TestExtractImageURL_TruncatedBodyAtCapReturnsEmptyNoPanic(t *testing.T) {
	// Wrapper hitting cap mid-JSON — body is unparseable. Must return "" without
	// panicking and without spamming logs.
	body := []byte(`{"data":[{"url":"https://cdn.example/very-long-image`)
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("panicked: %v", r)
		}
	}()
	if got := ExtractImageURL(body); got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestExtractImageURL_PrefersOpenAIShapeOverGenericFallback(t *testing.T) {
	// Both shapes present — must take the canonical OpenAI path first.
	body := []byte(`{
		"data": [{"url": "https://canonical.example/img.png"}],
		"upload_url": "https://other.example/wrong.png"
	}`)
	if got := ExtractImageURL(body); got != "https://canonical.example/img.png" {
		t.Errorf("got %q", got)
	}
}

func TestExtractImageURL_SkipsEmptyURLs(t *testing.T) {
	body := []byte(`{"data":[{"url":""},{"url":"https://cdn.example/second.png"}]}`)
	if got := ExtractImageURL(body); got != "https://cdn.example/second.png" {
		t.Errorf("got %q", got)
	}
}

func TestBufferedResponseWriter_EndToEndExtractAfterMultipleWrites(t *testing.T) {
	// Realistic flow: adaptor calls c.Writer.WriteHeader, then c.Writer.Write
	// possibly multiple times. Captured buffer should contain the full
	// concatenation and ExtractImageURL should pull the URL.
	inner, rec := newTestWriter(t)
	w := NewBufferedResponseWriter(inner, CapturedBodyMaxBytes)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	chunk1 := []byte(`{"created":1700000000,"data":[{"`)
	chunk2 := []byte(`url":"https://cdn.example/multi-write.png"}]}`)
	if _, err := w.Write(chunk1); err != nil {
		t.Fatalf("write1: %v", err)
	}
	if _, err := w.Write(chunk2); err != nil {
		t.Fatalf("write2: %v", err)
	}

	wantBody := append(append([]byte{}, chunk1...), chunk2...)
	if !bytes.Equal(rec.Body.Bytes(), wantBody) {
		t.Fatalf("user body mismatch: got %q want %q", rec.Body.String(), wantBody)
	}
	url := ExtractImageURL(w.Bytes())
	if url != "https://cdn.example/multi-write.png" {
		t.Errorf("extracted url=%q", url)
	}
}

func TestExtractImageURL_DoesNotMatchWhenURLKeyAbsent(t *testing.T) {
	// Generic fallback ONLY fires when a key contains "url".
	body := []byte(`{"foo":"bar","count":3,"nested":{"deep":"value"}}`)
	if got := ExtractImageURL(body); got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestExtractImageURL_GenericFallbackCaseInsensitive(t *testing.T) {
	// "ImageURL" (Go-ish casing) at depth 1 should match.
	body := []byte(`{"ImageURL":"https://upper.example/u.png"}`)
	if got := ExtractImageURL(body); got != "https://upper.example/u.png" {
		t.Errorf("got %q", got)
	}
}

func TestExtractImageURL_DoesNotPanicOnHTMLBody(t *testing.T) {
	// Some upstreams return HTML error pages. Must not panic.
	body := []byte(`<html><body><h1>504 Gateway Timeout</h1></body></html>`)
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("panicked: %v", r)
		}
	}()
	if got := ExtractImageURL(body); got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

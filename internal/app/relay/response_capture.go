package relay

// response_capture provides a tee-style gin.ResponseWriter that forwards
// bytes to the real client unchanged AND mirrors a capped prefix into an
// in-memory buffer for downstream inspection (e.g. extracting the image URL
// from an upstream JSON body).
//
// Design intent — bytes the user sees are byte-identical to today. The
// wrapper only OBSERVES; it never alters, reorders, or delays writes.
// Buffer is hard capped to defend against accidental OOM if a provider
// streams a huge body.
//
// Ported from newapi commit fc49e72d (2026-05-01) — service/response_capture.go.

import (
	"bytes"
	"encoding/json"
	"strings"

	"github.com/LurusTech/lurus-hub/internal/pkg/common"
	"github.com/gin-gonic/gin"
)

// CapturedBodyMaxBytes caps how many response bytes the wrapper retains.
// 64 KiB is far larger than any image-generation JSON metadata response we
// have seen (DALL-E ~600B, Replicate ~1KB, Ali ~2KB, Jimeng ~1KB), and stays
// well below default Gin recovery limits.
const CapturedBodyMaxBytes = 64 << 10

// BufferedResponseWriter wraps a gin.ResponseWriter to T-off the response
// bytes into a bounded buffer while forwarding every Write/WriteString call
// to the underlying writer untouched.
//
// Embeds gin.ResponseWriter so all other methods (Header, WriteHeader, Flush,
// Hijack, CloseNotify, Status, Size, Written, WriteHeaderNow, Pusher) forward
// by inheritance. Only Write and WriteString are intercepted — those carry
// the body bytes.
type BufferedResponseWriter struct {
	gin.ResponseWriter
	buf       *bytes.Buffer
	cap       int
	truncated bool
}

// NewBufferedResponseWriter wraps w with a buffer capped at maxBytes. If
// maxBytes <= 0, CapturedBodyMaxBytes is used.
func NewBufferedResponseWriter(w gin.ResponseWriter, maxBytes int) *BufferedResponseWriter {
	if maxBytes <= 0 {
		maxBytes = CapturedBodyMaxBytes
	}
	return &BufferedResponseWriter{
		ResponseWriter: w,
		buf:            new(bytes.Buffer),
		cap:            maxBytes,
	}
}

// Write forwards p to the embedded writer first (so the user-visible bytes
// and returned (n, err) match exactly what the underlying writer reports),
// then mirrors as much of p as fits into the buffer without exceeding cap.
func (b *BufferedResponseWriter) Write(p []byte) (int, error) {
	n, err := b.ResponseWriter.Write(p)
	if n > 0 {
		b.appendCapped(p[:n])
	}
	return n, err
}

// WriteString forwards s, then mirrors a capped prefix into the buffer.
func (b *BufferedResponseWriter) WriteString(s string) (int, error) {
	n, err := b.ResponseWriter.WriteString(s)
	if n > 0 {
		b.appendCapped([]byte(s)[:n])
	}
	return n, err
}

// appendCapped writes data into the internal buffer up to the cap, marking
// truncated when more bytes were offered than fit.
func (b *BufferedResponseWriter) appendCapped(data []byte) {
	remaining := b.cap - b.buf.Len()
	if remaining <= 0 {
		b.truncated = true
		return
	}
	if len(data) <= remaining {
		_, _ = b.buf.Write(data)
		return
	}
	_, _ = b.buf.Write(data[:remaining])
	b.truncated = true
}

// Bytes returns a copy-free view of what was buffered. Callers must not
// mutate the slice.
func (b *BufferedResponseWriter) Bytes() []byte {
	if b.buf == nil {
		return nil
	}
	return b.buf.Bytes()
}

// Truncated reports whether any bytes were dropped because cap was reached.
func (b *BufferedResponseWriter) Truncated() bool {
	return b.truncated
}

// ExtractImageURL parses a captured response body and returns the first image
// URL it can find, walking these provider shapes in priority order:
//
//  1. OpenAI / DALL-E / normalized-by-newhub:  {"data": [{"url": "..."}]}
//  2. Replicate raw shape:                      {"output": ["..."]} or {"output":"..."}
//  3. Generic last-resort: any "*url*" string field at depth 1 or 2
//
// On any parse error, shape miss, empty input, or malformed JSON, an empty
// string is returned. NEVER panics. Logs at most one debug line per call.
func ExtractImageURL(body []byte) string {
	body = bytes.TrimSpace(body)
	if len(body) == 0 {
		return ""
	}

	var top map[string]json.RawMessage
	if err := json.Unmarshal(body, &top); err != nil {
		if common.DebugEnabled {
			common.SysLog("ExtractImageURL: unmarshal failed (likely truncated body): " + err.Error())
		}
		return ""
	}

	if raw, ok := top["data"]; ok {
		if url := firstURLFromDataArray(raw); url != "" {
			return url
		}
	}

	if raw, ok := top["output"]; ok {
		if url := firstURLFromOutputField(raw); url != "" {
			return url
		}
	}

	if url := firstURLLikeFieldDepth1(top); url != "" {
		return url
	}
	if url := firstURLLikeFieldDepth2(top); url != "" {
		return url
	}
	return ""
}

// firstURLFromDataArray accepts the raw JSON for a "data" field and returns
// the first non-empty {"url": "..."} string in the array.
func firstURLFromDataArray(raw json.RawMessage) string {
	var arr []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &arr); err != nil {
		return ""
	}
	for _, item := range arr {
		if u, ok := item["url"]; ok {
			var s string
			if json.Unmarshal(u, &s) == nil && s != "" {
				return s
			}
		}
	}
	return ""
}

// firstURLFromOutputField handles Replicate's "output" — string OR array of strings.
func firstURLFromOutputField(raw json.RawMessage) string {
	var single string
	if err := json.Unmarshal(raw, &single); err == nil && single != "" {
		return single
	}
	var arr []string
	if err := json.Unmarshal(raw, &arr); err == nil {
		for _, s := range arr {
			if s != "" {
				return s
			}
		}
	}
	return ""
}

// firstURLLikeFieldDepth1 returns the first top-level string value whose key
// contains "url" (case-insensitive).
func firstURLLikeFieldDepth1(top map[string]json.RawMessage) string {
	for k, v := range top {
		if !strings.Contains(strings.ToLower(k), "url") {
			continue
		}
		var s string
		if json.Unmarshal(v, &s) == nil && s != "" {
			return s
		}
	}
	return ""
}

// firstURLLikeFieldDepth2 looks one level deep: for each top-level object or
// array of objects, scan its fields for "*url*" string values.
func firstURLLikeFieldDepth2(top map[string]json.RawMessage) string {
	for _, v := range top {
		var obj map[string]json.RawMessage
		if json.Unmarshal(v, &obj) == nil {
			if url := firstURLLikeFieldDepth1(obj); url != "" {
				return url
			}
			continue
		}
		var arr []map[string]json.RawMessage
		if json.Unmarshal(v, &arr) == nil {
			for _, item := range arr {
				if url := firstURLLikeFieldDepth1(item); url != "" {
					return url
				}
			}
		}
	}
	return ""
}

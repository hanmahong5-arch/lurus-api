package governance

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"time"
	"testing"

	"github.com/LurusTech/lurus-hub/internal/domain/entity"
	"github.com/LurusTech/lurus-hub/internal/pkg/common"

	"github.com/gin-gonic/gin"
)

// signalingAuditWriter records events and signals each CreateAuditEvent call on
// a channel so tests can deterministically wait for the async goroutine that
// RecordAuditEvent dispatches via gopool.Go, instead of sleeping.
type signalingAuditWriter struct {
	mu     sync.Mutex
	events []*entity.AuditEvent
	err    error
	done   chan struct{}
}

func newSignalingAuditWriter(err error) *signalingAuditWriter {
	return &signalingAuditWriter{err: err, done: make(chan struct{}, 8)}
}

func (m *signalingAuditWriter) CreateAuditEvent(event *entity.AuditEvent) error {
	m.mu.Lock()
	if m.err == nil {
		m.events = append(m.events, event)
	}
	e := m.err
	m.mu.Unlock()
	m.done <- struct{}{}
	return e
}

func (m *signalingAuditWriter) recorded() []*entity.AuditEvent {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]*entity.AuditEvent, len(m.events))
	copy(out, m.events)
	return out
}

// TestRecordAuditEvent_WriterSucceeds covers the happy async path: with a writer
// configured and a non-nil event, RecordAuditEvent must actually hand the event
// to CreateAuditEvent on the pool goroutine. This is the real compliance path —
// the event has to reach the persistence layer for the audit trail to exist.
func TestRecordAuditEvent_WriterSucceeds(t *testing.T) {
	defer auditWriterRef.Store(nil)

	w := newSignalingAuditWriter(nil)
	SetAuditWriter(w)

	ev := &entity.AuditEvent{
		TenantID: "tenant-persist",
		Action:   ActionTokenCreated,
		Resource: ResourceToken,
		ActorID:  7,
	}
	RecordAuditEvent(ev)

	// Wait for the async goroutine to complete the write.
	select {
	case <-w.done:
	case <-time.After(2 * time.Second):
		t.Fatal("CreateAuditEvent was never invoked by the async goroutine")
	}

	got := w.recorded()
	if len(got) != 1 {
		t.Fatalf("expected exactly 1 recorded event, got %d", len(got))
	}
	if got[0] != ev {
		t.Error("recorded event is not the same event that was submitted")
	}
	if got[0].TenantID != "tenant-persist" || got[0].Action != ActionTokenCreated {
		t.Errorf("event fields not preserved through async path: tenant=%q action=%q",
			got[0].TenantID, got[0].Action)
	}
}

// TestRecordAuditEvent_WriterError covers the error branch inside the async
// goroutine: when CreateAuditEvent returns an error, RecordAuditEvent must not
// panic or block the caller — the failure is logged (SysLog) and swallowed so a
// broken audit sink never takes down a live request.
func TestRecordAuditEvent_WriterError(t *testing.T) {
	defer auditWriterRef.Store(nil)

	w := newSignalingAuditWriter(errors.New("db down"))
	SetAuditWriter(w)

	RecordAuditEvent(&entity.AuditEvent{Action: ActionChannelDisabled})

	// The goroutine must still invoke CreateAuditEvent (and hit the error branch).
	select {
	case <-w.done:
	case <-time.After(2 * time.Second):
		t.Fatal("CreateAuditEvent was never invoked on the error path")
	}

	// On error nothing is appended; assert the failure was swallowed cleanly.
	if n := len(w.recorded()); n != 0 {
		t.Errorf("error path should record nothing, got %d events", n)
	}
}

// EnrichContext tests ---------------------------------------------------------

func newEnrichTestContext(body []byte) *gin.Context {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("POST", "/v1/chat/completions", nil)
	if body != nil {
		// Seed the cached body so GetRequestBody returns it without touching
		// the (nil) request body reader.
		c.Set(common.KeyRequestBody, body)
	}
	return c
}

// TestEnrichContext_SetsFingerprint proves the main-relay enrichment path stores
// a deterministic fingerprint in the context so EnrichLogParams can pick it up
// for the audit/anomaly trail.
func TestEnrichContext_SetsFingerprint(t *testing.T) {
	body := []byte(`{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}]}`)
	c := newEnrichTestContext(body)

	EnrichContext(c, 123, "gpt-4o")

	v, exists := c.Get(ctxKeyFingerprint)
	if !exists {
		t.Fatal("expected fingerprint to be set in context")
	}
	fp, ok := v.(string)
	if !ok {
		t.Fatalf("fingerprint should be a string, got %T", v)
	}
	// Must equal the pure ComputeFingerprint over the same inputs (16 hex chars).
	want := ComputeFingerprint(123, "gpt-4o", body)
	if fp != want {
		t.Errorf("fingerprint mismatch: got %q want %q", fp, want)
	}
	if len(fp) != 16 {
		t.Errorf("fingerprint length = %d, want 16", len(fp))
	}
}

// TestEnrichContext_EmptyBodyNoFingerprint covers the early-return branch: with
// an empty request body there is nothing to fingerprint, so the context key must
// stay unset (EnrichLogParams then leaves RequestFingerprint empty).
func TestEnrichContext_EmptyBodyNoFingerprint(t *testing.T) {
	c := newEnrichTestContext([]byte{}) // cached but zero-length

	EnrichContext(c, 1, "gpt-4o")

	if _, exists := c.Get(ctxKeyFingerprint); exists {
		t.Error("empty body should not produce a fingerprint")
	}
}

// TestEnrichContext_NoBodyReadErrorNoFingerprint covers the err!=0 branch of
// GetRequestBody: no cached body and a nil request body reader forces the read
// to fail, and EnrichContext must return without setting anything.
func TestEnrichContext_NoBodyReadErrorNoFingerprint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("POST", "/v1/chat/completions", nil)
	c.Request.Body = errReadCloser{} // read always errors

	EnrichContext(c, 5, "claude-3")

	if _, exists := c.Get(ctxKeyFingerprint); exists {
		t.Error("body read error should not produce a fingerprint")
	}
}

// TestEnrichContext_FingerprintDistinctByToken proves the fingerprint actually
// incorporates tokenID/model — two different tokens over the same body must
// yield different fingerprints (so the anomaly key isn't a constant).
func TestEnrichContext_FingerprintDistinctByToken(t *testing.T) {
	body := []byte(`{"model":"gpt-4o"}`)

	c1 := newEnrichTestContext(body)
	EnrichContext(c1, 100, "gpt-4o")
	fp1, _ := c1.Get(ctxKeyFingerprint)

	c2 := newEnrichTestContext(body)
	EnrichContext(c2, 200, "gpt-4o")
	fp2, _ := c2.Get(ctxKeyFingerprint)

	if fp1 == fp2 {
		t.Errorf("different tokens produced identical fingerprints: %v", fp1)
	}
}

// errReadCloser is a request body whose Read always fails, exercising the
// GetRequestBody error path from EnrichContext.
type errReadCloser struct{}

func (errReadCloser) Read(p []byte) (int, error) { return 0, errors.New("read failed") }
func (errReadCloser) Close() error               { return nil }

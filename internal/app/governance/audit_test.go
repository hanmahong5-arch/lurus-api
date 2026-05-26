package governance

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/LurusTech/lurus-hub/internal/domain/entity"
	"github.com/LurusTech/lurus-hub/internal/pkg/common"

	"github.com/gin-gonic/gin"
)

// mockAuditWriter records events for testing.
type mockAuditWriter struct {
	events []*entity.AuditEvent
	err    error
}

func (m *mockAuditWriter) CreateAuditEvent(event *entity.AuditEvent) error {
	if m.err != nil {
		return m.err
	}
	m.events = append(m.events, event)
	return nil
}

func TestSetAuditWriter_StoresWriter(t *testing.T) {
	// Reset state after test.
	defer auditWriterRef.Store(nil)

	w := &mockAuditWriter{}
	SetAuditWriter(w)

	wp := auditWriterRef.Load()
	if wp == nil {
		t.Fatal("expected writer to be stored")
	}
	if *wp != AuditWriter(w) {
		t.Error("stored writer does not match")
	}
}

func TestRecordAuditEvent_NilWriter(t *testing.T) {
	// Ensure no writer is set.
	auditWriterRef.Store(nil)

	// Should not panic.
	RecordAuditEvent(&entity.AuditEvent{Action: "test"})
}

func TestRecordAuditEvent_NilEvent(t *testing.T) {
	defer auditWriterRef.Store(nil)
	w := &mockAuditWriter{}
	SetAuditWriter(w)

	// Should not panic with nil event.
	RecordAuditEvent(nil)
}

func TestNewAuditEvent_FieldPopulation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("POST", "/test", nil)
	c.Set("tenant_id", "tenant-abc")
	c.Set(common.RequestIdKey, "req-123")

	event := NewAuditEvent(c, ActorUser, 42, ActionTokenCreated, ResourceToken, 100, `{"name":"test"}`)

	if event.TenantID != "tenant-abc" {
		t.Errorf("expected tenant_id=tenant-abc, got %q", event.TenantID)
	}
	if event.ActorType != ActorUser {
		t.Errorf("expected actor_type=user, got %q", event.ActorType)
	}
	if event.ActorID != 42 {
		t.Errorf("expected actor_id=42, got %d", event.ActorID)
	}
	if event.Action != ActionTokenCreated {
		t.Errorf("expected action=token.created, got %q", event.Action)
	}
	if event.Resource != ResourceToken {
		t.Errorf("expected resource=token, got %q", event.Resource)
	}
	if event.ResourceID != 100 {
		t.Errorf("expected resource_id=100, got %d", event.ResourceID)
	}
	if event.Details != `{"name":"test"}` {
		t.Errorf("expected details json, got %q", event.Details)
	}
	if event.RequestID != "req-123" {
		t.Errorf("expected request_id=req-123, got %q", event.RequestID)
	}
	if event.Timestamp <= 0 {
		t.Error("expected timestamp > 0")
	}
}

func TestNewAuditEvent_DefaultTenant(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/", nil)
	// No tenant_id set.

	event := NewAuditEvent(c, ActorSystem, 0, ActionChannelDisabled, ResourceChannel, 1, "")

	if event.TenantID != "default" {
		t.Errorf("expected default tenant, got %q", event.TenantID)
	}
}

func TestNewAuditEvent_DefaultRetention(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("POST", "/test", nil)

	event := NewAuditEvent(c, ActorUser, 1, ActionTokenCreated, ResourceToken, 1, "")

	// retention_until should equal timestamp + 7y. We don't pin the exact
	// timestamp because NewAuditEvent reads the clock; assert the delta.
	delta := event.RetentionUntil - event.Timestamp
	if delta != DefaultAuditRetentionSeconds {
		t.Errorf("expected retention_until - timestamp = %d (7y), got %d",
			DefaultAuditRetentionSeconds, delta)
	}
	if event.RetentionUntil == 0 {
		t.Error("RetentionUntil should not be zero — 0 means 'no expiry' which would skip cleanup")
	}
}

func TestSetAuditWriter_AtomicSafety(t *testing.T) {
	defer auditWriterRef.Store(nil)

	// Concurrent read/write should not race.
	var done atomic.Int32
	go func() {
		for done.Load() == 0 {
			w := &mockAuditWriter{}
			SetAuditWriter(w)
		}
	}()
	go func() {
		for done.Load() == 0 {
			RecordAuditEvent(&entity.AuditEvent{Action: "test"})
		}
	}()

	// Let goroutines run briefly.
	for i := 0; i < 1000; i++ {
		RecordAuditEvent(&entity.AuditEvent{Action: "test"})
	}
	done.Store(1)
}

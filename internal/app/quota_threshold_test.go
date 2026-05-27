package app

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/LurusTech/lurus-hub/internal/app/governance"
	"github.com/LurusTech/lurus-hub/internal/domain/entity"
)

// --- minimal mocks ---

// mockRedis implements redisDeduper; controls whether SetNXBool returns set=true.
type mockRedis struct {
	mu      sync.Mutex
	setKeys map[string]bool // key -> already set
}

func newMockRedis() *mockRedis { return &mockRedis{setKeys: make(map[string]bool)} }

func (m *mockRedis) SetNXBool(_ context.Context, key string, _ time.Duration) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.setKeys[key] {
		return false, nil
	}
	m.setKeys[key] = true
	return true, nil
}

// mockPublisher records every published message.
type mockPublisher struct {
	mu       sync.Mutex
	messages []publishedMsg
	failErr  error // if non-nil, Publish returns this error
}

type publishedMsg struct {
	subject string
	payload any
}

func (p *mockPublisher) Publish(_ context.Context, subject string, payload any) error {
	if p.failErr != nil {
		return p.failErr
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.messages = append(p.messages, publishedMsg{subject, payload})
	return nil
}

func (p *mockPublisher) count() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.messages)
}

// withQuotaNATSEnabled enables the feature flag for the duration of the test.
func withQuotaNATSEnabled(t *testing.T) {
	t.Helper()
	prev := os.Getenv("LLM_QUOTA_NATS_ENABLED")
	os.Setenv("LLM_QUOTA_NATS_ENABLED", "true")
	t.Cleanup(func() { os.Setenv("LLM_QUOTA_NATS_ENABLED", prev) })
}

// --- AC6 tests ---

// AC6(a): usage doesn't cross any threshold → no publish.
func TestCheckAndPublishQuotaThresholds_NoCrossing_NoPublish(t *testing.T) {
	withQuotaNATSEnabled(t)
	rdb := newMockRedis()
	pub := &mockPublisher{}

	// before ≈ 38%, after ≈ 43% — neither crosses 50%.
	params := quotaThresholdParams{
		UserId:            1,
		IdentityAccountID: 100,
		QuotaConsumed:     50,
		UsedTokensAfter:   450, // 450/1050 ≈ 42.8%
		LimitTokens:       1050,
		// usedBefore = 450-50 = 400; percentBefore = 400/1050 ≈ 38.1%
	}

	checkAndPublishQuotaThresholds(context.Background(), params, rdb, pub)

	if pub.count() != 0 {
		t.Errorf("expected 0 publishes, got %d", pub.count())
	}
}

// AC6(b): crosses 50% → exactly one publish on llm.quota.threshold.
func TestCheckAndPublishQuotaThresholds_Cross50_PublishesOnce(t *testing.T) {
	withQuotaNATSEnabled(t)
	rdb := newMockRedis()
	pub := &mockPublisher{}

	// usedBefore = 550-150 = 400; percentBefore = 400/1100 ≈ 36.4% (< 50)
	// usedAfter  = 550;           percentAfter  = 550/1100 = 50% (>= 50)
	params := quotaThresholdParams{
		UserId:            2,
		IdentityAccountID: 200,
		QuotaConsumed:     150,
		UsedTokensAfter:   550,
		LimitTokens:       1100,
	}

	checkAndPublishQuotaThresholds(context.Background(), params, rdb, pub)

	if pub.count() != 1 {
		t.Errorf("expected 1 publish, got %d", pub.count())
	}
	if pub.messages[0].subject != "llm.quota.threshold" {
		t.Errorf("expected subject llm.quota.threshold, got %s", pub.messages[0].subject)
	}
	payload, ok := pub.messages[0].payload.(QuotaThresholdPayload)
	if !ok {
		t.Fatalf("payload type mismatch: %T", pub.messages[0].payload)
	}
	if payload.AccountID != 200 {
		t.Errorf("account_id = %d, want 200", payload.AccountID)
	}
	if payload.UsagePercent < 50.0 {
		t.Errorf("usage_percent = %f, want >= 50", payload.UsagePercent)
	}
	if payload.UsedTokens != 550 {
		t.Errorf("used_tokens = %d, want 550", payload.UsedTokens)
	}
	if payload.LimitTokens != 1100 {
		t.Errorf("limit_tokens = %d, want 1100", payload.LimitTokens)
	}
}

// AC6(c): same threshold not re-published (Redis NX dedup).
func TestCheckAndPublishQuotaThresholds_AlreadySent_NoRepublish(t *testing.T) {
	withQuotaNATSEnabled(t)
	rdb := newMockRedis()
	pub := &mockPublisher{}

	params := quotaThresholdParams{
		UserId:            3,
		IdentityAccountID: 300,
		QuotaConsumed:     150,
		UsedTokensAfter:   550,
		LimitTokens:       1100,
	}

	// First call — should publish once.
	checkAndPublishQuotaThresholds(context.Background(), params, rdb, pub)
	if pub.count() != 1 {
		t.Fatalf("first call: expected 1 publish, got %d", pub.count())
	}

	// Second call with same params — Redis NX returns false, no publish.
	checkAndPublishQuotaThresholds(context.Background(), params, rdb, pub)
	if pub.count() != 1 {
		t.Errorf("second call: expected still 1 (dedup), got %d", pub.count())
	}
}

// AC6(d): NATS publish fails → no panic, main flow unaffected (void return).
func TestCheckAndPublishQuotaThresholds_PublishFails_NoError(t *testing.T) {
	withQuotaNATSEnabled(t)
	rdb := newMockRedis()
	pub := &mockPublisher{failErr: errors.New("simulated NATS publish failure")}

	params := quotaThresholdParams{
		UserId:            4,
		IdentityAccountID: 400,
		QuotaConsumed:     150,
		UsedTokensAfter:   550,
		LimitTokens:       1100,
	}

	// Must not panic; function is fire-and-forget with void return.
	checkAndPublishQuotaThresholds(context.Background(), params, rdb, pub)
}

// AC6(e): LLM_QUOTA_NATS_ENABLED=false → completely skipped.
func TestCheckAndPublishQuotaThresholds_Disabled_Skips(t *testing.T) {
	os.Setenv("LLM_QUOTA_NATS_ENABLED", "false")
	t.Cleanup(func() { os.Unsetenv("LLM_QUOTA_NATS_ENABLED") })

	pub := &mockPublisher{}

	params := quotaThresholdParams{
		UserId:            5,
		IdentityAccountID: 500,
		QuotaConsumed:     150,
		UsedTokensAfter:   550,
		LimitTokens:       1100,
	}

	checkAndPublishQuotaThresholds(context.Background(), params, nil, pub)

	if pub.count() != 0 {
		t.Errorf("expected 0 publishes when disabled, got %d", pub.count())
	}
}

// --- Phase E4 additions: audit + configurable thresholds + boundary cases ---

// capturingAuditWriter records audit events in-memory for assertions.
// gopool.Go inside RecordAuditEvent dispatches asynchronously, so tests must
// poll snapshot() with a deadline rather than reading the slice immediately.
type capturingAuditWriter struct {
	mu     sync.Mutex
	events []*entity.AuditEvent
}

func (m *capturingAuditWriter) CreateAuditEvent(ev *entity.AuditEvent) error {
	m.mu.Lock()
	m.events = append(m.events, ev)
	m.mu.Unlock()
	return nil
}

func (m *capturingAuditWriter) snapshot() []*entity.AuditEvent {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]*entity.AuditEvent, len(m.events))
	copy(out, m.events)
	return out
}

// waitForCount polls until len(events) >= want or timeout fires. Returns the
// final snapshot; fails the test if the timeout is reached. Calibrated to the
// gopool dispatch latency observed locally (single-digit ms); 2s ceiling keeps
// CI flake risk low without blocking obvious failures for too long.
func (m *capturingAuditWriter) waitForCount(t *testing.T, want int, timeout time.Duration) []*entity.AuditEvent {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		snap := m.snapshot()
		if len(snap) >= want {
			return snap
		}
		time.Sleep(5 * time.Millisecond)
	}
	snap := m.snapshot()
	if len(snap) < want {
		t.Fatalf("expected ≥%d audit events, got %d after %v", want, len(snap), timeout)
	}
	return snap
}

// noopAuditWriter restores a benign global writer after a test installs its
// own capturing mock. Cannot use nil because governance.SetAuditWriter stores
// the AuditWriter interface value via atomic.Pointer.
type noopAuditWriter struct{}

func (noopAuditWriter) CreateAuditEvent(*entity.AuditEvent) error { return nil }

func installCapturingAuditWriter(t *testing.T) *capturingAuditWriter {
	t.Helper()
	mock := &capturingAuditWriter{}
	governance.SetAuditWriter(mock)
	t.Cleanup(func() { governance.SetAuditWriter(noopAuditWriter{}) })
	return mock
}

// resetQuotaThresholdsCache forces parsedQuotaThresholds to re-read the env on
// the next call. Required because the sync.Once cache pins the first observed
// value for the process lifetime; tests that flip LLM_QUOTA_THRESHOLDS need to
// invalidate it between cases.
func resetQuotaThresholdsCache(t *testing.T) {
	t.Helper()
	quotaThresholdsCache = nil
	quotaThresholdsCacheOnce = sync.Once{}
	t.Cleanup(func() {
		quotaThresholdsCache = nil
		quotaThresholdsCacheOnce = sync.Once{}
	})
}

// TestLoadQuotaThresholds_Defaults asserts the canonical ladder ships when the
// env is unset or blank. Production behaviour must not change just because
// loadQuotaThresholds is now reachable.
func TestLoadQuotaThresholds_Defaults(t *testing.T) {
	if got := loadQuotaThresholds(""); !reflect.DeepEqual(got, []int{50, 80, 95, 100}) {
		t.Errorf("loadQuotaThresholds(\"\") = %v, want default ladder", got)
	}
	if got := loadQuotaThresholds("   "); !reflect.DeepEqual(got, []int{50, 80, 95, 100}) {
		t.Errorf("loadQuotaThresholds(whitespace) = %v, want default ladder", got)
	}
}

// TestLoadQuotaThresholds_Override covers the Phase E4 use case: operator sets
// LLM_QUOTA_THRESHOLDS=80,90,100 to mirror the Anthropic Console cadence.
// Also verifies ordering normalisation (out-of-order input → ascending output)
// and dedup (repeated values folded once).
func TestLoadQuotaThresholds_Override(t *testing.T) {
	cases := []struct {
		env  string
		want []int
	}{
		{"80,90,100", []int{80, 90, 100}},
		{"50, 80, 100", []int{50, 80, 100}},
		{"100,50,80", []int{50, 80, 100}}, // sort ascending
		{"50,50,80", []int{50, 80}},       // dedup
		{"75", []int{75}},                  // single value
	}
	for _, tc := range cases {
		if got := loadQuotaThresholds(tc.env); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("loadQuotaThresholds(%q) = %v, want %v", tc.env, got, tc.want)
		}
	}
}

// TestLoadQuotaThresholds_InvalidFallsBack asserts that malformed input
// (negative, zero, >100, non-numeric) does NOT silently drop values — the
// override is voided entirely and the default ladder ships. This protects
// against operator typos that would otherwise silently disable alerting.
func TestLoadQuotaThresholds_InvalidFallsBack(t *testing.T) {
	defaults := []int{50, 80, 95, 100}
	cases := []string{
		"80,-10,100", // negative
		"0,80,100",    // zero
		"80,150,100",  // >100
		"abc",         // non-numeric
		"80,foo,100", // partial garbage
	}
	for _, env := range cases {
		if got := loadQuotaThresholds(env); !reflect.DeepEqual(got, defaults) {
			t.Errorf("loadQuotaThresholds(%q) = %v, want default fallback", env, got)
		}
	}
}

// TestCheckAndPublishQuotaThresholds_AuditRecorded asserts that crossing a
// threshold writes one audit_events row per crossing. Phase E4 hard
// requirement — the plan's verification step "audit_events 有记录" depends on
// this hook firing on every Redis-acquired dedup slot.
func TestCheckAndPublishQuotaThresholds_AuditRecorded(t *testing.T) {
	withQuotaNATSEnabled(t)
	resetQuotaThresholdsCache(t)
	audit := installCapturingAuditWriter(t)
	pub := &mockPublisher{}

	params := quotaThresholdParams{
		UserId:            42,
		IdentityAccountID: 4200,
		TenantID:          "acme",
		QuotaConsumed:     150,
		UsedTokensAfter:   550,
		LimitTokens:       1100,
	}

	checkAndPublishQuotaThresholds(context.Background(), params, newMockRedis(), pub)

	events := audit.waitForCount(t, 1, 2*time.Second)
	if len(events) != 1 {
		t.Fatalf("audit count = %d, want 1", len(events))
	}
	ev := events[0]
	if ev.Action != governance.ActionBillingQuotaThreshold {
		t.Errorf("action = %q, want %q", ev.Action, governance.ActionBillingQuotaThreshold)
	}
	if ev.TenantID != "acme" {
		t.Errorf("tenant_id = %q, want %q", ev.TenantID, "acme")
	}
	if ev.ActorType != governance.ActorSystem {
		t.Errorf("actor_type = %q, want %q", ev.ActorType, governance.ActorSystem)
	}
	if ev.ActorID != 42 {
		t.Errorf("actor_id = %d, want 42", ev.ActorID)
	}

	// Details payload must include the threshold value so audit consumers can
	// filter "show me every 80% crossing this month" without joining other tables.
	var details map[string]any
	if err := json.Unmarshal([]byte(ev.Details), &details); err != nil {
		t.Fatalf("details unmarshal: %v", err)
	}
	if got, ok := details["threshold_pct"].(float64); !ok || got != 50 {
		t.Errorf("details.threshold_pct = %v, want 50", details["threshold_pct"])
	}
}

// TestCheckAndPublishQuotaThresholds_AuditPersistsOnPublishFail covers
// CLAUDE.md §4.1 partial-failure self-check: if NATS goes down mid-fire, the
// audit row must still land. Reversing audit/publish order would lose the
// crossing on publish failure — this test pins the current order.
func TestCheckAndPublishQuotaThresholds_AuditPersistsOnPublishFail(t *testing.T) {
	withQuotaNATSEnabled(t)
	resetQuotaThresholdsCache(t)
	audit := installCapturingAuditWriter(t)
	pub := &mockPublisher{failErr: errors.New("nats down")}

	params := quotaThresholdParams{
		UserId:            7,
		IdentityAccountID: 700,
		TenantID:          "default",
		QuotaConsumed:     150,
		UsedTokensAfter:   550,
		LimitTokens:       1100,
	}

	checkAndPublishQuotaThresholds(context.Background(), params, newMockRedis(), pub)

	events := audit.waitForCount(t, 1, 2*time.Second)
	if len(events) != 1 {
		t.Errorf("audit count = %d, want 1 (audit must record even when NATS fails)", len(events))
	}
}

// TestCheckAndPublishQuotaThresholds_MultipleCrossings exercises the case
// where one large consumption straddles two thresholds at once (e.g. jump
// from 30% to 90% spans both 50% and 80%). Each crossing must produce its
// own audit row + NATS event — silently coalescing them would lose information.
func TestCheckAndPublishQuotaThresholds_MultipleCrossings(t *testing.T) {
	withQuotaNATSEnabled(t)
	resetQuotaThresholdsCache(t)
	audit := installCapturingAuditWriter(t)
	pub := &mockPublisher{}

	// usedBefore = 1000-600 = 400; percentBefore = 400/1000 = 40% (< 50, < 80)
	// usedAfter  = 1000;             percentAfter  = 100% (>= 50, 80, 95, 100)
	// → crosses 50, 80, 95, 100 in one shot (4 events).
	params := quotaThresholdParams{
		UserId:            8,
		IdentityAccountID: 800,
		TenantID:          "default",
		QuotaConsumed:     600,
		UsedTokensAfter:   1000,
		LimitTokens:       1000,
	}

	checkAndPublishQuotaThresholds(context.Background(), params, newMockRedis(), pub)

	if pub.count() != 4 {
		t.Errorf("publish count = %d, want 4 (50, 80, 95, 100 all crossed)", pub.count())
	}
	events := audit.waitForCount(t, 4, 2*time.Second)
	if len(events) != 4 {
		t.Errorf("audit count = %d, want 4", len(events))
	}
}

// TestCheckAndPublishQuotaThresholds_TenantIDDefaultsBlank verifies that an
// empty TenantID in params still produces an audit row — NewDetachedAuditEvent
// rewrites blank to "default" so legacy code paths that don't set the field
// continue to produce searchable audit_events rows under the default tenant.
func TestCheckAndPublishQuotaThresholds_TenantIDDefaultsBlank(t *testing.T) {
	withQuotaNATSEnabled(t)
	resetQuotaThresholdsCache(t)
	audit := installCapturingAuditWriter(t)
	pub := &mockPublisher{}

	params := quotaThresholdParams{
		UserId:            9,
		IdentityAccountID: 900,
		// TenantID intentionally omitted
		QuotaConsumed:   150,
		UsedTokensAfter: 550,
		LimitTokens:     1100,
	}

	checkAndPublishQuotaThresholds(context.Background(), params, newMockRedis(), pub)

	events := audit.waitForCount(t, 1, 2*time.Second)
	if events[0].TenantID != "default" {
		t.Errorf("tenant_id = %q, want %q", events[0].TenantID, "default")
	}
}

// TestCheckAndPublishQuotaThresholds_NoCrossingNoAudit asserts the corollary
// of the recording rule: a transaction that doesn't cross any threshold
// produces zero audit_events rows. Without this guard, every relay call would
// add an audit row even at low usage — alert-storm by daily volume.
func TestCheckAndPublishQuotaThresholds_NoCrossingNoAudit(t *testing.T) {
	withQuotaNATSEnabled(t)
	resetQuotaThresholdsCache(t)
	audit := installCapturingAuditWriter(t)
	pub := &mockPublisher{}

	params := quotaThresholdParams{
		UserId:            10,
		IdentityAccountID: 1000,
		TenantID:          "default",
		QuotaConsumed:     50,
		UsedTokensAfter:   450, // 42.8% — below 50
		LimitTokens:       1050,
	}

	checkAndPublishQuotaThresholds(context.Background(), params, newMockRedis(), pub)

	// Wait briefly for any async dispatch then snapshot. Expect exactly 0.
	time.Sleep(100 * time.Millisecond)
	if got := audit.snapshot(); len(got) != 0 {
		t.Errorf("audit count = %d, want 0 (no threshold crossed)", len(got))
	}
}

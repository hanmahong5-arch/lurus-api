package app

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"
)

// --- minimal mocks ---

// mockRedis implements redisSetNXer; controls whether SetNX returns set=true.
type mockRedis struct {
	mu      sync.Mutex
	setKeys map[string]bool // key -> already set
}

func newMockRedis() *mockRedis { return &mockRedis{setKeys: make(map[string]bool)} }

type mockSetNXResult struct{ ok bool; err error }

func (r mockSetNXResult) Result() (bool, error) { return r.ok, r.err }

func (m *mockRedis) SetNX(_ context.Context, key string, _ interface{}, _ time.Duration) interface {
	Result() (bool, error)
} {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.setKeys[key] {
		return mockSetNXResult{ok: false}
	}
	m.setKeys[key] = true
	return mockSetNXResult{ok: true}
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

// helper: enable the feature flag for the duration of the test.
func withQuotaNATSEnabled(t *testing.T) {
	t.Helper()
	prev := os.Getenv("LLM_QUOTA_NATS_ENABLED")
	os.Setenv("LLM_QUOTA_NATS_ENABLED", "true")
	t.Cleanup(func() { os.Setenv("LLM_QUOTA_NATS_ENABLED", prev) })
}

// --- AC6 tests ---

// AC6(a): no threshold crossed → no publish
func TestCheckAndPublishQuotaThresholds_NoCrossing_NoPublish(t *testing.T) {
	withQuotaNATSEnabled(t)
	rdb := newMockRedis()
	pub := &mockPublisher{}

	// 40% → 45%: no 50% crossing
	params := quotaThresholdParams{
		UserId:            1,
		IdentityAccountID: 100,
		QuotaBefore:       600, // remaining before
		QuotaConsumed:     50,
		UsedQuotaBefore:   400, // already used
		// totalMax = 600 + 400 + 50 = 1050; percentBefore = 400/1050 ≈ 38%, after = 450/1050 ≈ 42.8%
	}

	checkAndPublishQuotaThresholds(context.Background(), params, rdb, pub)

	if pub.count() != 0 {
		t.Errorf("expected 0 publishes, got %d", pub.count())
	}
}

// AC6(b): crosses 50% → exactly one publish on llm.quota.threshold
func TestCheckAndPublishQuotaThresholds_Cross50_PublishesOnce(t *testing.T) {
	withQuotaNATSEnabled(t)
	rdb := newMockRedis()
	pub := &mockPublisher{}

	// before ≈ 40%, after ≈ 55%
	params := quotaThresholdParams{
		UserId:            2,
		IdentityAccountID: 200,
		QuotaBefore:       550, // remaining before
		QuotaConsumed:     150,
		UsedQuotaBefore:   400,
		// totalMax = 550+400+150 = 1100; percentBefore=400/1100≈36.4%, after=550/1100=50%
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
		t.Fatalf("payload is not QuotaThresholdPayload")
	}
	if payload.AccountID != 200 {
		t.Errorf("account_id = %d, want 200", payload.AccountID)
	}
	if payload.UsagePercent < 50.0 {
		t.Errorf("usage_percent = %f, want >= 50", payload.UsagePercent)
	}
}

// AC6(c): same threshold not re-published (Redis NX blocks duplicate)
func TestCheckAndPublishQuotaThresholds_AlreadySent_NoRepublish(t *testing.T) {
	withQuotaNATSEnabled(t)
	rdb := newMockRedis()
	pub := &mockPublisher{}

	params := quotaThresholdParams{
		UserId:            3,
		IdentityAccountID: 300,
		QuotaBefore:       550,
		QuotaConsumed:     150,
		UsedQuotaBefore:   400,
	}

	// First call — should publish once.
	checkAndPublishQuotaThresholds(context.Background(), params, rdb, pub)
	if pub.count() != 1 {
		t.Fatalf("first call: expected 1 publish, got %d", pub.count())
	}

	// Second call with same params — Redis NX returns false, no publish.
	checkAndPublishQuotaThresholds(context.Background(), params, rdb, pub)
	if pub.count() != 1 {
		t.Errorf("second call: expected still 1 publish (dedup), got %d", pub.count())
	}
}

// AC6(d): NATS publish fails → function returns without panic; main flow unaffected
func TestCheckAndPublishQuotaThresholds_PublishFails_NoError(t *testing.T) {
	withQuotaNATSEnabled(t)
	rdb := newMockRedis()
	pub := &mockPublisher{failErr: errPublishFailed}

	params := quotaThresholdParams{
		UserId:            4,
		IdentityAccountID: 400,
		QuotaBefore:       550,
		QuotaConsumed:     150,
		UsedQuotaBefore:   400,
	}

	// Must not panic; function is fire-and-forget.
	checkAndPublishQuotaThresholds(context.Background(), params, rdb, pub)
	// No assertion on publish count — the publisher always fails, that's the point.
	// We only verify no panic / no error propagation (void return).
}

// AC6(e): LLM_QUOTA_NATS_ENABLED=false → completely skipped
func TestCheckAndPublishQuotaThresholds_Disabled_Skips(t *testing.T) {
	os.Setenv("LLM_QUOTA_NATS_ENABLED", "false")
	t.Cleanup(func() { os.Unsetenv("LLM_QUOTA_NATS_ENABLED") })

	pub := &mockPublisher{}

	params := quotaThresholdParams{
		UserId:            5,
		IdentityAccountID: 500,
		QuotaBefore:       550,
		QuotaConsumed:     150,
		UsedQuotaBefore:   400,
	}

	checkAndPublishQuotaThresholds(context.Background(), params, nil, pub)

	if pub.count() != 0 {
		t.Errorf("expected 0 publishes when disabled, got %d", pub.count())
	}
}

// errPublishFailed is a sentinel error for AC6(d).
var errPublishFailed = fmt.Errorf("simulated NATS publish failure")

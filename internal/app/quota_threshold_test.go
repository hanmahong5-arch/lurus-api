package app

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"
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

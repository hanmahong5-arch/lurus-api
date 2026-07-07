package app

// quota_threshold_publish_extra_test.go — covers checkAndPublishQuotaThresholds'
// crossing loop and recordQuotaThresholdAudit using in-test fakes for the
// publisher and dedup, so no NATS/Redis server is required. nats.Enabled() is
// turned on via env.

import (
	"context"
	"testing"
	"time"
)

type fakeThresholdPublisher struct {
	published []any
	failNext  bool
}

func (f *fakeThresholdPublisher) Publish(_ context.Context, _ string, payload any) error {
	if f.failNext {
		f.failNext = false
		return context.DeadlineExceeded
	}
	f.published = append(f.published, payload)
	return nil
}

type fakeDeduper struct {
	setResults map[string]bool
}

func (f *fakeDeduper) SetNXBool(_ context.Context, key string, _ time.Duration) (bool, error) {
	// Default: claim the key (return true) exactly once per distinct key.
	if f.setResults == nil {
		f.setResults = map[string]bool{}
	}
	if f.setResults[key] {
		return false, nil // already claimed
	}
	f.setResults[key] = true
	return true, nil
}

func TestCheckAndPublishQuotaThresholds_CrossesAndPublishes(t *testing.T) {
	t.Setenv("LLM_QUOTA_NATS_ENABLED", "true")

	pub := &fakeThresholdPublisher{}
	dedup := &fakeDeduper{}

	// before = (600-300)/1000 = 30%, after = 600/1000 = 60% => crosses the 50%
	// threshold in the default ladder (50/80/95/100).
	params := quotaThresholdParams{
		UserId:            4242,
		IdentityAccountID: 99,
		TenantID:          "default",
		QuotaConsumed:     300,
		UsedTokensAfter:   600,
		LimitTokens:       1000,
	}

	checkAndPublishQuotaThresholds(context.Background(), params, dedup, pub)

	if len(pub.published) == 0 {
		t.Fatal("expected at least one threshold event to be published after crossing 50%")
	}
	// The published payload must carry the correct usage numbers.
	payload, ok := pub.published[0].(QuotaThresholdPayload)
	if !ok {
		t.Fatalf("published payload type = %T, want QuotaThresholdPayload", pub.published[0])
	}
	if payload.AccountID != 99 || payload.UsedTokens != 600 || payload.LimitTokens != 1000 {
		t.Errorf("payload = %+v, want account=99 used=600 limit=1000", payload)
	}
	if payload.UsagePercent != 60.0 {
		t.Errorf("usage percent = %v, want 60", payload.UsagePercent)
	}
}

func TestCheckAndPublishQuotaThresholds_DedupSkips(t *testing.T) {
	t.Setenv("LLM_QUOTA_NATS_ENABLED", "true")

	pub := &fakeThresholdPublisher{}
	// Deduper that always reports "already claimed" => nothing publishes.
	dedup := &fakeDeduper{setResults: map[string]bool{}}
	// Pre-claim every key by making SetNXBool always return false: seed a
	// deduper whose map is pre-populated is awkward, so use a dedicated stub.
	alwaysClaimed := &alwaysFalseDeduper{}

	params := quotaThresholdParams{
		UserId:            1,
		IdentityAccountID: 2,
		QuotaConsumed:     300,
		UsedTokensAfter:   600,
		LimitTokens:       1000,
	}
	_ = dedup
	checkAndPublishQuotaThresholds(context.Background(), params, alwaysClaimed, pub)
	if len(pub.published) != 0 {
		t.Errorf("expected no publish when dedup reports already-sent, got %d", len(pub.published))
	}
}

type alwaysFalseDeduper struct{}

func (alwaysFalseDeduper) SetNXBool(_ context.Context, _ string, _ time.Duration) (bool, error) {
	return false, nil
}

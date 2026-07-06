package nats

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

var errPublishDown = errors.New("simulated jetstream down")

// TestTruncateRune_MultibyteUnderMax covers the branch where the byte length
// exceeds max but the rune count does not, so the string is returned whole
// without slicing into a UTF-8 sequence.
func TestTruncateRune_MultibyteUnderMax(t *testing.T) {
	// "你好" is 6 bytes but 2 runes; with max=3, len(s)=6 > 3 yet len(r)=2 <= 3.
	if got := truncateRune("你好", 3); got != "你好" {
		t.Errorf("truncateRune(\"你好\", 3) = %q, want 你好", got)
	}
}

// decodeEnvelope unmarshals the raw bytes the fake JS received into the
// canonical envelope so tests can assert on the wire shape downstream
// consumers actually see.
func decodeEnvelope(t *testing.T, data []byte) llmEventEnvelope {
	t.Helper()
	var env llmEventEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		t.Fatalf("published bytes are not a valid envelope: %v (raw=%s)", err, data)
	}
	return env
}

// TestPublishImageGenerated_EmitsEnvelope drives the full happy path:
// PublishImageGenerated -> publishLLMEvent -> Publisher.Publish. It asserts
// the subject, account_id and the (rune-truncated) payload a notification
// consumer would render.
func TestPublishImageGenerated_EmitsEnvelope(t *testing.T) {
	fj := &fakeJS{}
	withGlobal(t, fj)

	longPrompt := strings.Repeat("请", 200) // 200 CJK runes, must truncate to 80
	PublishImageGenerated(context.Background(), 4242, "dall-e-3", longPrompt, "https://img/x.png")

	if fj.mu.calls != 1 {
		t.Fatalf("js.Publish calls = %d, want 1", fj.mu.calls)
	}
	if fj.mu.subject != SubjectLLMImageGenerated {
		t.Errorf("subject = %q, want %q", fj.mu.subject, SubjectLLMImageGenerated)
	}

	env := decodeEnvelope(t, fj.mu.data)
	if env.EventType != SubjectLLMImageGenerated {
		t.Errorf("event_type = %q, want %q", env.EventType, SubjectLLMImageGenerated)
	}
	if env.AccountID != 4242 {
		t.Errorf("account_id = %d, want 4242", env.AccountID)
	}
	if env.EventID == "" {
		t.Error("event_id must be a generated uuid, got empty")
	}
	if env.OccurredAt.IsZero() {
		t.Error("occurred_at must be set")
	}

	var payload LLMImageGeneratedPayload
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		t.Fatalf("payload decode: %v", err)
	}
	if payload.ImageURL != "https://img/x.png" {
		t.Errorf("image_url = %q, want https://img/x.png", payload.ImageURL)
	}
	if payload.JobID == "" {
		t.Error("job_id must be a generated uuid, got empty")
	}
	if n := len([]rune(payload.Prompt)); n != 80 {
		t.Errorf("prompt rune length = %d, want 80 (truncated)", n)
	}
}

// TestPublishUsageMilestone_EmitsEnvelope drives PublishUsageMilestone through
// to the wire and asserts the milestone payload fields.
func TestPublishUsageMilestone_EmitsEnvelope(t *testing.T) {
	fj := &fakeJS{}
	withGlobal(t, fj)

	PublishUsageMilestone(context.Background(), 77, 1_000_000, "1M lifetime", "lifetime")

	if fj.mu.calls != 1 {
		t.Fatalf("js.Publish calls = %d, want 1", fj.mu.calls)
	}
	if fj.mu.subject != SubjectLLMUsageMilestone {
		t.Errorf("subject = %q, want %q", fj.mu.subject, SubjectLLMUsageMilestone)
	}

	env := decodeEnvelope(t, fj.mu.data)
	if env.AccountID != 77 {
		t.Errorf("account_id = %d, want 77", env.AccountID)
	}
	if env.EventType != SubjectLLMUsageMilestone {
		t.Errorf("event_type = %q, want %q", env.EventType, SubjectLLMUsageMilestone)
	}

	var payload LLMUsageMilestonePayload
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		t.Fatalf("payload decode: %v", err)
	}
	if payload.Period != "lifetime" || payload.TokensUsed != 1_000_000 || payload.Milestone != "1M lifetime" {
		t.Errorf("payload mismatch: %+v", payload)
	}
}

// TestPublishImageGenerated_NoOpWhenGlobalNil confirms the publisher-not-ready
// guard fires before any wire attempt (NATS disabled / not yet Init'd).
func TestPublishImageGenerated_NoOpWhenGlobalNil(t *testing.T) {
	prev := global
	global = nil
	t.Cleanup(func() { global = prev })

	// Must not panic and must not publish (no publisher installed).
	PublishImageGenerated(context.Background(), 5, "gpt-4o", "hi", "url")
	PublishUsageMilestone(context.Background(), 5, 10, "10", "day")
}

// TestPublishImageGenerated_PublishErrorIsSwallowed drives the fire-and-forget
// contract: when the wire publish fails, publishLLMEvent logs and returns —
// the caller (a relay hot path) must never see an error or panic.
func TestPublishImageGenerated_PublishErrorIsSwallowed(t *testing.T) {
	fj := &fakeJS{failErr: errPublishDown}
	withGlobal(t, fj)

	// Must not panic despite the underlying js.Publish returning an error.
	PublishImageGenerated(context.Background(), 9, "gpt-4o", "hi", "url")

	if fj.mu.calls != 1 {
		t.Errorf("js.Publish should be attempted once, got %d", fj.mu.calls)
	}
}

// TestPublishLLMEvent_MarshalErrorIsSwallowed verifies the marshal-failure
// branch of publishLLMEvent: it logs and returns without touching the wire,
// so a bad payload can never take the publisher down.
func TestPublishLLMEvent_MarshalErrorIsSwallowed(t *testing.T) {
	fj := &fakeJS{}
	p := newPublisher(fj, "LLM_EVENTS")

	// A channel is not JSON-marshalable → json.Marshal errors before the
	// envelope is built, so nothing must reach js.Publish.
	publishLLMEvent(context.Background(), p, SubjectLLMImageGenerated, 1, make(chan int), "gpt-4o")

	if fj.mu.calls != 0 {
		t.Errorf("marshal failure must not publish; js.Publish calls = %d", fj.mu.calls)
	}
}

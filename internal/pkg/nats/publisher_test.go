package nats

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"

	natsgo "github.com/nats-io/nats.go"
)

// fakeJS is a fake natsgo.JetStreamContext that only implements Publish.
// The embedded nil interface satisfies the (large) JetStreamContext surface
// at compile time; any method other than Publish would panic if called, which
// is exactly what we want — the publisher must only touch Publish.
type fakeJS struct {
	natsgo.JetStreamContext // nil embed: satisfies the interface, panics if used

	mu      recorder
	failErr error
}

type recorder struct {
	subject string
	data    []byte
	calls   int
}

func (f *fakeJS) Publish(subj string, data []byte, _ ...natsgo.PubOpt) (*natsgo.PubAck, error) {
	f.mu.calls++
	f.mu.subject = subj
	// copy so later mutations by the caller cannot race the assertion
	f.mu.data = append([]byte(nil), data...)
	if f.failErr != nil {
		return nil, f.failErr
	}
	return &natsgo.PubAck{Stream: "LLM_EVENTS", Sequence: 1}, nil
}

// withGlobal installs a Publisher backed by js as the package global for the
// duration of the test, restoring the previous value on cleanup. Reset is via
// the unexported field, so this must live in-package.
func withGlobal(t *testing.T, js natsgo.JetStreamContext) *fakeJS {
	t.Helper()
	prev := global
	fj, _ := js.(*fakeJS)
	global = newPublisher(js, "LLM_EVENTS")
	t.Cleanup(func() { global = prev })
	return fj
}

// --- env-driven pure branches ---

func TestEnabled(t *testing.T) {
	cases := []struct {
		name string
		set  bool
		val  string
		want bool
	}{
		{"unset defaults false", false, "", false},
		{"empty string false", true, "", false},
		{"true", true, "true", true},
		{"1", true, "1", true},
		{"false", true, "false", false},
		{"0", true, "0", false},
		{"garbage parses false", true, "yesplease", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.set {
				t.Setenv("LLM_QUOTA_NATS_ENABLED", tc.val)
			} else {
				// ensure a clean unset: Setenv restores after test, so set empty
				// via os is not needed — the CI env doesn't define it. Guard by
				// asserting the unset path only when truly unset.
				if _, ok := os.LookupEnv("LLM_QUOTA_NATS_ENABLED"); ok {
					t.Skip("LLM_QUOTA_NATS_ENABLED set in ambient env; unset case not applicable")
				}
			}
			if got := Enabled(); got != tc.want {
				t.Errorf("Enabled()=%v want %v (val=%q)", got, tc.want, tc.val)
			}
		})
	}
}

func TestStreamName(t *testing.T) {
	// default when unset/empty
	t.Setenv("LLM_QUOTA_NATS_STREAM", "")
	if got := streamName(); got != defaultStream {
		t.Errorf("streamName() default = %q, want %q", got, defaultStream)
	}
	// override
	t.Setenv("LLM_QUOTA_NATS_STREAM", "CUSTOM_STREAM")
	if got := streamName(); got != "CUSTOM_STREAM" {
		t.Errorf("streamName() override = %q, want CUSTOM_STREAM", got)
	}
}

func TestNatsURL(t *testing.T) {
	t.Setenv("NATS_URL", "")
	if got := natsURL(); got != natsgo.DefaultURL {
		t.Errorf("natsURL() default = %q, want %q", got, natsgo.DefaultURL)
	}
	t.Setenv("NATS_URL", "nats://example:4222")
	if got := natsURL(); got != "nats://example:4222" {
		t.Errorf("natsURL() override = %q, want nats://example:4222", got)
	}
}

// --- Get / newPublisher ---

func TestGet_NilWhenUninitialised(t *testing.T) {
	prev := global
	global = nil
	t.Cleanup(func() { global = prev })
	if Get() != nil {
		t.Fatal("Get() should be nil when global is unset")
	}
}

func TestGet_ReturnsInstalledPublisher(t *testing.T) {
	fj := &fakeJS{}
	withGlobal(t, fj)
	p := Get()
	if p == nil {
		t.Fatal("Get() returned nil after install")
	}
	if p.stream != "LLM_EVENTS" {
		t.Errorf("stream = %q, want LLM_EVENTS", p.stream)
	}
}

func TestNewPublisher_SetsFields(t *testing.T) {
	fj := &fakeJS{}
	p := newPublisher(fj, "S1")
	if p.js == nil {
		t.Fatal("js not set")
	}
	if p.stream != "S1" {
		t.Errorf("stream = %q, want S1", p.stream)
	}
}

// --- Publisher.Publish ---

func TestPublisher_Publish_MarshalsAndSendsToSubject(t *testing.T) {
	fj := &fakeJS{}
	p := newPublisher(fj, "LLM_EVENTS")

	type body struct {
		A string `json:"a"`
		B int    `json:"b"`
	}
	err := p.Publish(context.Background(), "some.subject", body{A: "x", B: 7})
	if err != nil {
		t.Fatalf("Publish returned error: %v", err)
	}
	if fj.mu.calls != 1 {
		t.Fatalf("js.Publish called %d times, want 1", fj.mu.calls)
	}
	if fj.mu.subject != "some.subject" {
		t.Errorf("subject = %q, want some.subject", fj.mu.subject)
	}
	var got body
	if err := json.Unmarshal(fj.mu.data, &got); err != nil {
		t.Fatalf("published payload is not the marshalled body: %v (raw=%s)", err, fj.mu.data)
	}
	if got.A != "x" || got.B != 7 {
		t.Errorf("payload = %+v, want {x 7}", got)
	}
}

func TestPublisher_Publish_MarshalError(t *testing.T) {
	fj := &fakeJS{}
	p := newPublisher(fj, "LLM_EVENTS")
	// channels are not JSON-marshalable → json.Marshal errors before any send.
	err := p.Publish(context.Background(), "s", make(chan int))
	if err == nil {
		t.Fatal("expected marshal error, got nil")
	}
	if fj.mu.calls != 0 {
		t.Errorf("js.Publish must not be called on marshal failure, got %d", fj.mu.calls)
	}
}

func TestPublisher_Publish_PropagatesJSError(t *testing.T) {
	fj := &fakeJS{failErr: errors.New("stream not found")}
	p := newPublisher(fj, "LLM_EVENTS")
	err := p.Publish(context.Background(), "s", map[string]int{"a": 1})
	if err == nil {
		t.Fatal("expected error from js.Publish, got nil")
	}
	if fj.mu.calls != 1 {
		t.Errorf("js.Publish should have been attempted once, got %d", fj.mu.calls)
	}
}

// --- Init: disabled path is the only unit-reachable branch (no broker) ---

func TestInit_NoOpWhenDisabled(t *testing.T) {
	t.Setenv("LLM_QUOTA_NATS_ENABLED", "false")
	if err := Init(); err != nil {
		t.Fatalf("Init() with NATS disabled must return nil, got %v", err)
	}
}

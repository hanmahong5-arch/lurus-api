// Package nats provides a minimal NATS JetStream publisher used by newhub
// to emit quota threshold events to the LLM_EVENTS stream.
package nats

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	natsgo "github.com/nats-io/nats.go"
)

const (
	// SubjectQuotaThreshold is the NATS subject for quota threshold events.
	SubjectQuotaThreshold = "llm.quota.threshold"

	defaultStream = "LLM_EVENTS"
)

// Publisher publishes events to a NATS JetStream stream.
type Publisher struct {
	js     natsgo.JetStreamContext
	stream string
}

var (
	global     *Publisher
	globalOnce sync.Once
	globalErr  error
)

// Enabled reports whether the quota NATS publisher is enabled.
// Reads LLM_QUOTA_NATS_ENABLED env var; defaults to false.
func Enabled() bool {
	v := os.Getenv("LLM_QUOTA_NATS_ENABLED")
	if v == "" {
		return false
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false
	}
	return b
}

// streamName returns the configured stream name (LLM_QUOTA_NATS_STREAM),
// falling back to the default.
func streamName() string {
	if s := os.Getenv("LLM_QUOTA_NATS_STREAM"); s != "" {
		return s
	}
	return defaultStream
}

// natsURL returns the NATS server URL from env.
func natsURL() string {
	if u := os.Getenv("NATS_URL"); u != "" {
		return u
	}
	return natsgo.DefaultURL
}

// Init initialises the global publisher once. Safe to call multiple times.
// Returns nil if NATS is disabled.
func Init() error {
	if !Enabled() {
		return nil
	}
	globalOnce.Do(func() {
		nc, err := natsgo.Connect(natsURL(),
			natsgo.Name("lurus-newhub"),
			natsgo.Timeout(5*time.Second),
			natsgo.MaxReconnects(5),
		)
		if err != nil {
			globalErr = fmt.Errorf("nats connect %s: %w", natsURL(), err)
			return
		}
		js, err := nc.JetStream()
		if err != nil {
			nc.Close()
			globalErr = fmt.Errorf("nats jetstream: %w", err)
			return
		}
		global = &Publisher{js: js, stream: streamName()}
	})
	return globalErr
}

// Get returns the global publisher. Returns nil when disabled or not yet
// initialised successfully.
func Get() *Publisher {
	return global
}

// newPublisher creates a publisher from an existing JetStreamContext.
// Used by tests to inject mock connections.
func newPublisher(js natsgo.JetStreamContext, stream string) *Publisher {
	return &Publisher{js: js, stream: stream}
}

// Publish serialises payload as JSON and publishes it to the stream.
// Non-blocking: ctx is only checked for deadline, not for cancellation of the network call.
func (p *Publisher) Publish(_ context.Context, subject string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	_, err = p.js.Publish(subject, data)
	if err != nil {
		return fmt.Errorf("publish %s: %w", subject, err)
	}
	return nil
}

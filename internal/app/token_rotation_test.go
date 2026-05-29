package app

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/LurusTech/lurus-hub/internal/adapter/repo"
	"github.com/LurusTech/lurus-hub/internal/app/governance"
	"github.com/LurusTech/lurus-hub/internal/domain/entity"
	"github.com/LurusTech/lurus-hub/internal/pkg/common"
)

const testDay = int64(24 * 60 * 60)

func TestNeedsRotationAt(t *testing.T) {
	const now = int64(1_000_000_000)
	tests := []struct {
		name           string
		autoRotateDays int
		rotatedAt      int64
		want           bool
	}{
		{"disabled_zero_days", 0, now - 100*testDay, false},
		{"disabled_negative_days", -5, now - 100*testDay, false},
		{"exactly_at_interval", 30, now - 30*testDay, true},
		{"just_before_interval", 30, now - 30*testDay + 1, false},
		{"one_second_past_interval", 30, now - 30*testDay - 1, true},
		{"well_past_interval", 7, now - 90*testDay, true},
		{"fresh_not_due", 30, now - 1*testDay, false},
		{"never_rotated_baseline_zero_due", 30, 0, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := NeedsRotationAt(tc.autoRotateDays, tc.rotatedAt, now); got != tc.want {
				t.Errorf("NeedsRotationAt(%d, %d, %d) = %v, want %v",
					tc.autoRotateDays, tc.rotatedAt, now, got, tc.want)
			}
		})
	}
}

// captureAuditWriter records audit events for assertion. CreateAuditEvent runs
// on a background goroutine (RecordAuditEvent is async), so events are read
// from the channel with a timeout.
type captureAuditWriter struct{ events chan *entity.AuditEvent }

func (w *captureAuditWriter) CreateAuditEvent(e *entity.AuditEvent) error {
	w.events <- e
	return nil
}

func TestRotateDueTokens(t *testing.T) {
	db := setupServiceTestDB(t)

	// Capture audit events; restore a discard writer afterward so we do not
	// leak this writer into sibling tests.
	cap := &captureAuditWriter{events: make(chan *entity.AuditEvent, 16)}
	governance.SetAuditWriter(cap)
	t.Cleanup(func() { governance.SetAuditWriter(discardAuditWriter{}) })

	owner := &repo.User{Username: "rot-owner", DisplayName: "rot-owner", Role: 1, Status: common.UserStatusEnabled, Email: "owner@example.com", TenantId: "default"}
	if err := db.Create(owner).Error; err != nil {
		t.Fatalf("seed owner: %v", err)
	}

	now := int64(1_700_000_000)

	mk := func(name, key string, autoRotateDays int, rotatedAt, createdTime int64) *repo.Token {
		tok := &repo.Token{
			UserId:         owner.Id,
			TenantId:       "default",
			Key:            key,
			Status:         common.TokenStatusEnabled,
			Name:           name,
			CreatedTime:    createdTime,
			ExpiredTime:    -1,
			AutoRotateDays: autoRotateDays,
			RotatedAt:      rotatedAt,
		}
		if err := db.Create(tok).Error; err != nil {
			t.Fatalf("seed token %q: %v", name, err)
		}
		return tok
	}

	due := mk("due", "sk-DUE-"+strings.Repeat("a", 40), 30, now-31*testDay, now-100*testDay)
	notDue := mk("not-due", "sk-NOTDUE-"+strings.Repeat("b", 38), 30, now-1*testDay, now-100*testDay)
	neverRotated := mk("never", "sk-NEVER-"+strings.Repeat("c", 39), 30, 0, now-100*testDay)
	disabled := mk("disabled", "sk-OFF-"+strings.Repeat("d", 41), 0, now-100*testDay, now-100*testDay)

	var sentTo []string
	send := func(subject, receiver, content string) error {
		sentTo = append(sentTo, receiver)
		return nil
	}

	rotated, err := RotateDueTokens(context.Background(), now, send)
	if err != nil {
		t.Fatalf("RotateDueTokens: %v", err)
	}
	if rotated != 2 {
		t.Fatalf("rotated = %d, want 2 (due + never-rotated)", rotated)
	}

	reload := func(id int) repo.Token {
		var tok repo.Token
		if err := db.First(&tok, id).Error; err != nil {
			t.Fatalf("reload token %d: %v", id, err)
		}
		return tok
	}

	// Due token: key changed, rotated_at stamped to now.
	if g := reload(due.Id); g.Key == due.Key || g.RotatedAt != now {
		t.Errorf("due token should be rotated: key-changed=%v rotated_at=%d (want %d)", g.Key != due.Key, g.RotatedAt, now)
	}

	// Never-rotated token (baseline = created_time) also rotated.
	if g := reload(neverRotated.Id); g.Key == neverRotated.Key || g.RotatedAt != now {
		t.Errorf("never-rotated token should be rotated: key-changed=%v rotated_at=%d (want %d)", g.Key != neverRotated.Key, g.RotatedAt, now)
	}

	// Not-due token unchanged.
	if g := reload(notDue.Id); g.Key != notDue.Key || g.RotatedAt != now-1*testDay {
		t.Errorf("not-due token must be untouched: key-changed=%v rotated_at=%d", g.Key != notDue.Key, g.RotatedAt)
	}

	// Disabled token (auto_rotate_days=0) untouched.
	if g := reload(disabled.Id); g.Key != disabled.Key {
		t.Error("disabled token must not be rotated")
	}

	// Owner notified once per rotation.
	if len(sentTo) != 2 {
		t.Fatalf("expected 2 owner emails, got %d (%v)", len(sentTo), sentTo)
	}
	for _, to := range sentTo {
		if to != "owner@example.com" {
			t.Errorf("email sent to %q, want owner@example.com", to)
		}
	}

	// Two auth.token_rotated audit events recorded (async).
	for i := 0; i < 2; i++ {
		select {
		case ev := <-cap.events:
			if ev.Action != governance.ActionAuthTokenRotated {
				t.Errorf("audit action = %q, want %q", ev.Action, governance.ActionAuthTokenRotated)
			}
			if ev.ActorType != governance.ActorSystem {
				t.Errorf("audit actor = %q, want %q", ev.ActorType, governance.ActorSystem)
			}
			if !strings.Contains(ev.Details, "old_key_prefix") || !strings.Contains(ev.Details, "auto_rotate") {
				t.Errorf("audit details missing rollback info: %q", ev.Details)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("timed out waiting for audit event %d/2", i+1)
		}
	}
}

type discardAuditWriter struct{}

func (discardAuditWriter) CreateAuditEvent(*entity.AuditEvent) error { return nil }

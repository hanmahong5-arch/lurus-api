package app

// token_rotation_gap_test.go — closes the RotateDueTokens / notifyOwner branches
// the happy-path fixture doesn't reach: the list error, context cancellation
// mid-pass, the unknown-age (baseline==0) skip, the short-key prefix, and the
// three notifyOwner arms (nil sender, no owner email, sender error).

import (
	"context"
	"errors"
	"testing"

	"github.com/LurusTech/lurus-hub/internal/adapter/repo"
	"github.com/LurusTech/lurus-hub/internal/pkg/common"
)

const rotDay = int64(24 * 60 * 60)

// mkRotToken seeds an auto-rotate token owned by ownerId.
func mkRotToken(t *testing.T, ownerId int, key string, autoRotateDays int, rotatedAt, createdTime int64) *repo.Token {
	t.Helper()
	tok := &repo.Token{
		UserId:         ownerId,
		TenantId:       "default",
		Key:            key,
		Status:         common.TokenStatusEnabled,
		Name:           "rot",
		CreatedTime:    createdTime,
		ExpiredTime:    -1,
		AutoRotateDays: autoRotateDays,
		RotatedAt:      rotatedAt,
	}
	if err := repo.DB.Create(tok).Error; err != nil {
		t.Fatalf("seed token: %v", err)
	}
	return tok
}

func TestRotateDueTokens_ListError(t *testing.T) {
	db := setupServiceTestDB(t)
	if err := db.Migrator().DropTable(&repo.Token{}); err != nil {
		t.Fatalf("drop tokens: %v", err)
	}
	if _, err := RotateDueTokens(context.Background(), 1_700_000_000, nil); err == nil {
		t.Fatal("expected a list error when the tokens table is missing")
	}
}

func TestRotateDueTokens_ContextCancelled(t *testing.T) {
	db := setupServiceTestDB(t)
	_ = db
	owner := seedTestUser(t, repo.DB, 1)
	const now = int64(1_700_000_000)
	mkRotToken(t, owner, "sk-DUE-cancel-000000000000000000000000", 30, now-31*rotDay, now-100*rotDay)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancelled before the pass runs

	rotated, err := RotateDueTokens(ctx, now, func(string, string, string) error { return nil })
	if err == nil {
		t.Fatal("expected ctx.Err() when context is cancelled")
	}
	if rotated != 0 {
		t.Errorf("rotated = %d, want 0 (cancelled before any rotation)", rotated)
	}
}

func TestRotateDueTokens_UnknownAgeSkipped(t *testing.T) {
	setupServiceTestDB(t)
	owner := seedTestUser(t, repo.DB, 1)
	const now = int64(1_700_000_000)
	// Neither RotatedAt nor CreatedTime set => unknown age => skip.
	mkRotToken(t, owner, "sk-UNKNOWN-age-00000000000000000000000", 30, 0, 0)

	rotated, err := RotateDueTokens(context.Background(), now, func(string, string, string) error { return nil })
	if err != nil {
		t.Fatalf("RotateDueTokens: %v", err)
	}
	if rotated != 0 {
		t.Errorf("rotated = %d, want 0 (unknown-age token must be skipped)", rotated)
	}
}

func TestRotateDueTokens_NilSenderAndShortKey(t *testing.T) {
	setupServiceTestDB(t)
	owner := seedTestUser(t, repo.DB, 1)
	const now = int64(1_700_000_000)
	// Short key (<= prefix length) exercises keyPrefix's short-circuit.
	tok := mkRotToken(t, owner, "sk-x", 30, now-31*rotDay, now-100*rotDay)

	// Nil sender => notifyOwner returns immediately (no email attempted).
	rotated, err := RotateDueTokens(context.Background(), now, nil)
	if err != nil {
		t.Fatalf("RotateDueTokens: %v", err)
	}
	if rotated != 1 {
		t.Fatalf("rotated = %d, want 1", rotated)
	}
	var reloaded repo.Token
	if err := repo.DB.First(&reloaded, tok.Id).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.Key == "sk-x" {
		t.Error("token key should have been rotated")
	}
}

func TestRotateDueTokens_NoOwnerEmailSkipsNotify(t *testing.T) {
	db := setupServiceTestDB(t)
	// Owner with an empty email => notifyOwner returns at the email guard.
	owner := &repo.User{Username: "no-email", DisplayName: "no-email", Role: 1,
		Status: common.UserStatusEnabled, Email: "", TenantId: "default"}
	if err := db.Create(owner).Error; err != nil {
		t.Fatalf("seed owner: %v", err)
	}
	const now = int64(1_700_000_000)
	mkRotToken(t, owner.Id, "sk-NOEMAIL-0000000000000000000000000000", 30, now-31*rotDay, now-100*rotDay)

	var called bool
	send := func(string, string, string) error { called = true; return nil }
	rotated, err := RotateDueTokens(context.Background(), now, send)
	if err != nil {
		t.Fatalf("RotateDueTokens: %v", err)
	}
	if rotated != 1 {
		t.Fatalf("rotated = %d, want 1", rotated)
	}
	if called {
		t.Error("sender must NOT be called when the owner has no email")
	}
}

func TestRotateDueTokens_SenderErrorLogged(t *testing.T) {
	db := setupServiceTestDB(t)
	owner := &repo.User{Username: "with-email", DisplayName: "with-email", Role: 1,
		Status: common.UserStatusEnabled, Email: "owner@example.com", TenantId: "default"}
	if err := db.Create(owner).Error; err != nil {
		t.Fatalf("seed owner: %v", err)
	}
	const now = int64(1_700_000_000)
	mkRotToken(t, owner.Id, "sk-SENDERR-0000000000000000000000000000", 30, now-31*rotDay, now-100*rotDay)

	send := func(string, string, string) error { return errors.New("smtp down") }
	// Sender error must be logged, not abort the pass — rotation still counts.
	rotated, err := RotateDueTokens(context.Background(), now, send)
	if err != nil {
		t.Fatalf("RotateDueTokens: %v", err)
	}
	if rotated != 1 {
		t.Errorf("rotated = %d, want 1 (sender error must not abort rotation)", rotated)
	}
}

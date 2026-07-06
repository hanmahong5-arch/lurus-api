package nats

import (
	"context"
	"encoding/json"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/LurusTech/lurus-hub/internal/adapter/repo"
	"github.com/LurusTech/lurus-hub/internal/pkg/common"
)

var natsSQLiteCounter atomic.Int64

// tcpRow is a minimal stand-in for the tenant_credit_pools table with just the
// columns gormPoolDB reads/writes. Defining it locally keeps the gormPoolDB SQL
// builder honest (real table name + real alert_fired_at column) without pulling
// the full repo entity into this package's test surface.
type tcpRow struct {
	ID           int64      `gorm:"column:id;primaryKey"`
	AlertFiredAt *time.Time `gorm:"column:alert_fired_at"`
}

func (tcpRow) TableName() string { return "tenant_credit_pools" }

// newPoolSQLite opens a unique in-memory SQLite DB with the tenant_credit_pools
// table migrated, so gormPoolDB's real GORM queries execute against a real
// schema.
func newPoolSQLite(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:natspoolthreshold%d?mode=memory&cache=shared", natsSQLiteCounter.Add(1))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&tcpRow{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return db
}

// --- gormPoolDB.LastAlertFiredAt ---

func TestGormPoolDB_LastAlertFiredAt_NullColumn(t *testing.T) {
	db := newPoolSQLite(t)
	if err := db.Create(&tcpRow{ID: 1, AlertFiredAt: nil}).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}
	g := &gormPoolDB{db: db}

	got, err := g.LastAlertFiredAt(context.Background(), 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Errorf("NULL alert_fired_at should read as nil, got %v", got)
	}
}

func TestGormPoolDB_LastAlertFiredAt_ReturnsTimestamp(t *testing.T) {
	db := newPoolSQLite(t)
	fired := time.Date(2026, 5, 18, 9, 30, 0, 0, time.UTC)
	if err := db.Create(&tcpRow{ID: 2, AlertFiredAt: &fired}).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}
	g := &gormPoolDB{db: db}

	got, err := g.LastAlertFiredAt(context.Background(), 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil {
		t.Fatal("expected a timestamp, got nil")
	}
	if !got.Equal(fired) {
		t.Errorf("alert_fired_at = %v, want %v", got, fired)
	}
}

func TestGormPoolDB_LastAlertFiredAt_MissingRow(t *testing.T) {
	db := newPoolSQLite(t)
	g := &gormPoolDB{db: db}

	// Scan (not First) over a missing row yields no error and a zero row →
	// nil timestamp, which the caller treats as "never fired".
	got, err := g.LastAlertFiredAt(context.Background(), 999)
	if err != nil {
		t.Fatalf("missing row must not error under Scan, got %v", err)
	}
	if got != nil {
		t.Errorf("missing row should read as nil, got %v", got)
	}
}

// --- gormPoolDB.MarkAlertFired ---

func TestGormPoolDB_MarkAlertFired_PersistsTimestamp(t *testing.T) {
	db := newPoolSQLite(t)
	if err := db.Create(&tcpRow{ID: 3, AlertFiredAt: nil}).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}
	g := &gormPoolDB{db: db}

	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	if err := g.MarkAlertFired(context.Background(), 3, now); err != nil {
		t.Fatalf("MarkAlertFired: %v", err)
	}

	got, err := g.LastAlertFiredAt(context.Background(), 3)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if got == nil || !got.Equal(now) {
		t.Errorf("persisted alert_fired_at = %v, want %v", got, now)
	}
}

// --- redisCommonDeduper.SetNXBool (no-Redis branch) ---

func TestRedisCommonDeduper_NoRedisAlwaysAcquires(t *testing.T) {
	prevEnabled := common.RedisEnabled
	prevRDB := common.RDB
	common.RedisEnabled = false
	common.RDB = nil
	t.Cleanup(func() {
		common.RedisEnabled = prevEnabled
		common.RDB = prevRDB
	})

	acquired, err := redisCommonDeduper{}.SetNXBool(context.Background(), "pool:threshold:x:1", time.Hour)
	if err != nil {
		t.Fatalf("no-Redis SetNXBool must not error, got %v", err)
	}
	if !acquired {
		t.Error("no-Redis mode must let publication proceed (acquired=true)")
	}
}

// --- PublishPoolThreshold wrapper: guard branches ---

func TestPublishPoolThreshold_NoOpWhenDisabled(t *testing.T) {
	t.Setenv("LLM_QUOTA_NATS_ENABLED", "false")
	// Even with a publisher installed, disabled must short-circuit to nil.
	fj := &fakeJS{}
	withGlobal(t, fj)

	if err := PublishPoolThreshold(context.Background(), "t", 1, 10, 1000, 80); err != nil {
		t.Fatalf("disabled must return nil, got %v", err)
	}
	if fj.mu.calls != 0 {
		t.Errorf("disabled must not publish, got %d calls", fj.mu.calls)
	}
}

func TestPublishPoolThreshold_NoOpWhenPublisherNil(t *testing.T) {
	t.Setenv("LLM_QUOTA_NATS_ENABLED", "true")
	prev := global
	global = nil
	t.Cleanup(func() { global = prev })

	if err := PublishPoolThreshold(context.Background(), "t", 1, 10, 1000, 80); err != nil {
		t.Fatalf("nil publisher must return nil, got %v", err)
	}
}

func TestPublishPoolThreshold_NoOpOnBadArgs(t *testing.T) {
	t.Setenv("LLM_QUOTA_NATS_ENABLED", "true")
	fj := &fakeJS{}
	withGlobal(t, fj)

	cases := []struct {
		name     string
		tenantID string
		poolID   int64
	}{
		{"empty tenant", "", 1},
		{"zero pool", "t", 0},
		{"negative pool", "t", -3},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := PublishPoolThreshold(context.Background(), tc.tenantID, tc.poolID, 10, 1000, 80); err != nil {
				t.Fatalf("bad args must return nil, got %v", err)
			}
		})
	}
	if fj.mu.calls != 0 {
		t.Errorf("guarded args must never publish, got %d calls", fj.mu.calls)
	}
}

// TestPublishPoolThreshold_EndToEnd wires the real production dependencies the
// wrapper reaches for (repo.DB via gormPoolDB, redisCommonDeduper with Redis
// disabled, the global Publisher) and proves the low-balance alert reaches the
// wire exactly once with the correct tenant/pool payload — and marks the schema
// dedup slot durably.
func TestPublishPoolThreshold_EndToEnd(t *testing.T) {
	t.Setenv("LLM_QUOTA_NATS_ENABLED", "true")

	db := newPoolSQLite(t)
	if err := db.Create(&tcpRow{ID: 55, AlertFiredAt: nil}).Error; err != nil {
		t.Fatalf("seed pool: %v", err)
	}

	prevDB := repo.DB
	repo.DB = db
	t.Cleanup(func() { repo.DB = prevDB })

	prevEnabled := common.RedisEnabled
	prevRDB := common.RDB
	common.RedisEnabled = false
	common.RDB = nil
	t.Cleanup(func() {
		common.RedisEnabled = prevEnabled
		common.RDB = prevRDB
	})

	fj := &fakeJS{}
	withGlobal(t, fj)

	if err := PublishPoolThreshold(context.Background(), "tenant-Z", 55, 120, 1000, 80); err != nil {
		t.Fatalf("PublishPoolThreshold: %v", err)
	}

	if fj.mu.calls != 1 {
		t.Fatalf("expected exactly 1 publish, got %d", fj.mu.calls)
	}
	if fj.mu.subject != SubjectPoolThreshold {
		t.Errorf("subject = %q, want %q", fj.mu.subject, SubjectPoolThreshold)
	}

	var payload PoolThresholdPayload
	if err := json.Unmarshal(fj.mu.data, &payload); err != nil {
		t.Fatalf("payload decode: %v", err)
	}
	if payload.TenantID != "tenant-Z" || payload.PoolID != 55 ||
		payload.CurrentBalance != 120 || payload.MaxBalance != 1000 || payload.ThresholdPct != 80 {
		t.Errorf("payload mismatch: %+v", payload)
	}

	// Schema dedup slot must be committed after a successful fire.
	g := &gormPoolDB{db: db}
	last, err := g.LastAlertFiredAt(context.Background(), 55)
	if err != nil {
		t.Fatalf("read alert_fired_at: %v", err)
	}
	if last == nil {
		t.Error("alert_fired_at must be marked after a successful publish")
	}

	// Second call within the dedup window must suppress (no second publish).
	if err := PublishPoolThreshold(context.Background(), "tenant-Z", 55, 120, 1000, 80); err != nil {
		t.Fatalf("second PublishPoolThreshold: %v", err)
	}
	if fj.mu.calls != 1 {
		t.Errorf("schema dedup must suppress the repeat fire; got %d total calls", fj.mu.calls)
	}
}

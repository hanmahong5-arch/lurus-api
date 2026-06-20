package openrouter_sync

import (
	"testing"
	"time"
)

// aggregator.go: AggregateOpenRouterUsage and LoadUsageStats both require live
// DB connections (repo.DB / repo.LOG_DB / repo.GetModelUsageStatsByChannelType
// via GORM) with no injectable seam. They cannot be tested without a running
// PostgreSQL instance. These functions are therefore skipped here and must be
// covered by the pg-integration suite (go test -tags integration ./...).
//
// What we can cover:
//   - LoadUsageStats output shape: the conversion from []repo.ModelUsageStat to
//     []Stat is a pure loop; we verify it via the Ranker's BuildUsageMap
//     which uses the same Stat type.
//   - AggregateRow struct fields: confirm the type is usable (compile-time).
//   - aggregateQueryTimeout / aggregateLimit constants: verify their values so
//     an accidental change is caught.

func TestAggregateConstants(t *testing.T) {
	if aggregateQueryTimeout != 5*time.Second {
		t.Errorf("aggregateQueryTimeout = %v, want 5s", aggregateQueryTimeout)
	}
	if aggregateLimit != 200 {
		t.Errorf("aggregateLimit = %d, want 200", aggregateLimit)
	}
}

// TestAggregateRow_FieldsAccessible ensures AggregateRow is the expected shape
// and that the zero value is usable — a simple compile + assignment check.
func TestAggregateRow_FieldsAccessible(t *testing.T) {
	row := AggregateRow{ModelName: "test-model:free", Count24h: 42}
	if row.ModelName != "test-model:free" {
		t.Errorf("ModelName = %q, want %q", row.ModelName, "test-model:free")
	}
	if row.Count24h != 42 {
		t.Errorf("Count24h = %d, want 42", row.Count24h)
	}
}

// TestLoadUsageStats_StatConversionShape verifies the Stat type — the same
// struct that LoadUsageStats builds — carries the three fields the ranker
// expects. This gives us a canary if the struct shape drifts.
func TestLoadUsageStats_StatConversionShape(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	s := Stat{
		ModelName:     "google/gemma-3-27b-it:free",
		Count24h:      150,
		LastUpdatedAt: now,
	}
	if s.ModelName == "" {
		t.Error("Stat.ModelName must not be empty")
	}
	if s.Count24h != 150 {
		t.Errorf("Stat.Count24h = %d, want 150", s.Count24h)
	}
	if !s.LastUpdatedAt.Equal(now) {
		t.Errorf("Stat.LastUpdatedAt = %v, want %v", s.LastUpdatedAt, now)
	}

	// Verify that a []Stat feeds correctly into BuildUsageMap (integration of
	// LoadUsageStats output with the ranker).
	stats := []Stat{s}
	m := BuildUsageMap(stats, now)
	if m == nil {
		t.Fatal("BuildUsageMap returned nil for fresh stats")
	}
	if m["google/gemma-3-27b-it:free"] != 150 {
		t.Errorf("UsageMap[model] = %d, want 150", m["google/gemma-3-27b-it:free"])
	}
}

// TestAggregateOpenRouterUsage_NilDBReturnsError verifies the DB-nil guard
// without a real database. repo.DB starts nil in test binaries that do not
// call repo.InitDB, so this path fires naturally in unit test context.
func TestAggregateOpenRouterUsage_NilDBReturnsError(t *testing.T) {
	// repo.DB is nil in the test binary (no InitDB called), so calling
	// AggregateOpenRouterUsage must return an error immediately.
	err := AggregateOpenRouterUsage(nil) //nolint:staticcheck // nil ctx is safe here: function checks repo.DB first
	if err == nil {
		t.Fatal("expected error when repo.DB is nil, got nil")
	}
	const want = "aggregator: DB not initialized"
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
}

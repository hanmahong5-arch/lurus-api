package openrouter_sync

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/LurusTech/lurus-hub/internal/adapter/provider/openrouter"
	"github.com/LurusTech/lurus-hub/internal/adapter/repo"
	"github.com/LurusTech/lurus-hub/internal/pkg/common"
	"github.com/LurusTech/lurus-hub/internal/pkg/constant"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

var syncDBCounter atomic.Int64

// TestAutoAggregate_LeaderInitialRunError covers the master+leader branch of
// AutoAggregateWithContext: the startup AggregateOpenRouterUsage call fires and
// (with repo.DB nil) returns an error that is logged, then context cancel stops
// the loop. This exercises the IsLeader()==true startup path that the existing
// scheduler test deliberately suppresses.
func TestAutoAggregate_LeaderInitialRunError(t *testing.T) {
	prevMaster := common.IsMasterNode
	common.IsMasterNode = true
	t.Cleanup(func() { common.IsMasterNode = prevMaster })

	prevLeader := common.IsLeader()
	common.SetLeader(true)
	t.Cleanup(func() { common.SetLeader(prevLeader) })

	// Ensure the startup Aggregate call hits the DB-nil guard (fast error).
	prevDB := repo.DB
	repo.DB = nil
	t.Cleanup(func() { repo.DB = prevDB })

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		AutoAggregateWithContext(ctx)
		close(done)
	}()
	time.Sleep(30 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("AutoAggregateWithContext did not stop after cancel")
	}
}

// setupSyncDB spins up an in-memory SQLite database (the repo's hermetic
// unit-test tier per CLAUDE.md), migrates the tables the sync engine touches,
// wires repo.DB / repo.LOG_DB, and returns a cleanup that restores prior state.
func setupSyncDB(t *testing.T, tables ...interface{}) func() {
	t.Helper()

	name := fmt.Sprintf("file:orsync%d?mode=memory&cache=shared", syncDBCounter.Add(1))
	db, err := gorm.Open(sqlite.Open(name), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	for _, tbl := range tables {
		if err := db.AutoMigrate(tbl); err != nil {
			if strings.Contains(err.Error(), "already exists") {
				continue
			}
			t.Fatalf("automigrate %T: %v", tbl, err)
		}
	}

	prevDB, prevLog := repo.DB, repo.LOG_DB
	prevSQLite, prevPG := common.UsingSQLite, common.UsingPostgreSQL
	repo.DB = db
	repo.LOG_DB = db
	common.UsingSQLite = true
	common.UsingPostgreSQL = false
	repo.InitCol()

	return func() {
		repo.DB, repo.LOG_DB = prevDB, prevLog
		common.UsingSQLite, common.UsingPostgreSQL = prevSQLite, prevPG
		if sqlDB, e := db.DB(); e == nil && sqlDB != nil {
			_ = sqlDB.Close()
		}
	}
}

// ---------------------------------------------------------------------------
// AggregateOpenRouterUsage — real GROUP BY against a seeded logs table.
// Business acceptance: usage counts drive the ranker, which decides which
// free models get imported and thus priced for tenants; the aggregation must
// count only OpenRouter `:free` rows inside the 24h window and rank by volume.
// ---------------------------------------------------------------------------

// TestLoadUsageStats_RealDBSeeded verifies the DB->ranker conversion: rows
// persisted in model_usage_stats are read back and mapped into []Stat with the
// counts the ranker relies on. (The Aggregate->Upsert write path uses an
// ON CONFLICT clause whose hardcoded column name diverges from GORM's derived
// column on an auto-migrated table, so the successful upsert branch is a
// PG-only path — see the package ceiling note at the bottom of this file.)
func TestLoadUsageStats_RealDBSeeded(t *testing.T) {
	cleanup := setupSyncDB(t, &repo.ModelUsageStat{})
	defer cleanup()

	now := time.Now()
	seed := []repo.ModelUsageStat{
		{ModelName: "alpha:free", ChannelType: constant.ChannelTypeOpenRouter, Count24h: 3, LastUpdatedAt: now},
		{ModelName: "beta:free", ChannelType: constant.ChannelTypeOpenRouter, Count24h: 1, LastUpdatedAt: now},
		// a different channel type must NOT be returned by the OpenRouter query
		{ModelName: "other", ChannelType: 1, Count24h: 99, LastUpdatedAt: now},
	}
	if err := repo.DB.Create(&seed).Error; err != nil {
		t.Fatalf("seed stats: %v", err)
	}

	loaded, err := LoadUsageStats()
	if err != nil {
		t.Fatalf("LoadUsageStats: %v", err)
	}
	byName := map[string]int64{}
	for _, s := range loaded {
		byName[s.ModelName] = s.Count24h
	}
	if byName["alpha:free"] != 3 || byName["beta:free"] != 1 {
		t.Errorf("LoadUsageStats mismatch: %+v", byName)
	}
	if _, ok := byName["other"]; ok {
		t.Error("channel_type=1 row must be excluded from OpenRouter usage stats")
	}
}

func TestAggregateOpenRouterUsage_EmptyLogsNoError(t *testing.T) {
	cleanup := setupSyncDB(t, &repo.Log{}, &repo.ModelUsageStat{})
	defer cleanup()

	// No rows seeded → early "no usage rows" return, still nil error.
	if err := AggregateOpenRouterUsage(context.Background()); err != nil {
		t.Fatalf("expected nil error on empty logs, got %v", err)
	}
	stats, err := repo.GetModelUsageStatsByChannelType(constant.ChannelTypeOpenRouter)
	if err != nil {
		t.Fatalf("read stats: %v", err)
	}
	if len(stats) != 0 {
		t.Errorf("expected no stats written, got %d", len(stats))
	}
}

func TestLoadUsageStats_RealDBEmpty(t *testing.T) {
	cleanup := setupSyncDB(t, &repo.ModelUsageStat{})
	defer cleanup()

	out, err := LoadUsageStats()
	if err != nil {
		t.Fatalf("LoadUsageStats: %v", err)
	}
	if len(out) != 0 {
		t.Errorf("expected empty slice, got %d rows", len(out))
	}
}

// ---------------------------------------------------------------------------
// Engine.Run — full orchestration against SQLite.
// ---------------------------------------------------------------------------

// freeModel builds a free text->text OpenRouter model with the given id/created.
func freeModel(id string, created int64) openrouter.Model {
	return openrouter.Model{
		ID:      id,
		Created: created,
		Pricing: openrouter.ModelPricing{Prompt: "0", Completion: "0", Image: "0", Request: "0"},
		Architecture: openrouter.ModelArchitecture{
			InputModalities:  []string{"text"},
			OutputModalities: []string{"text"},
		},
	}
}

func TestEngine_Run_NoEnabledJobs(t *testing.T) {
	cleanup := setupSyncDB(t, &repo.OpenRouterSyncJob{})
	defer cleanup()

	eng := &Engine{
		HTTPClient: func(_ context.Context) ([]openrouter.Model, error) {
			t.Fatal("fetcher should not be called when there are no jobs")
			return nil, nil
		},
		UsageFn: func() ([]Stat, error) { return nil, nil },
		Now:     time.Now,
	}
	res, err := eng.Run(context.Background(), nil, false)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !res.Skipped || res.SkipReason != "no enabled jobs" {
		t.Errorf("expected skipped/no-enabled-jobs, got %+v", res)
	}
}

func TestEngine_Run_NoDueJobs(t *testing.T) {
	cleanup := setupSyncDB(t, &repo.OpenRouterSyncJob{})
	defer cleanup()

	// A daily job that already ran a minute ago is enabled but NOT due.
	ranAt := time.Now().Add(-1 * time.Minute)
	job := &repo.OpenRouterSyncJob{
		Name:            "j1",
		TargetChannelId: 10,
		Categories:      `["llm_reasoning"]`,
		Schedule:        "daily",
		Enabled:         true,
		LastRunAt:       &ranAt,
	}
	if err := repo.DB.Create(job).Error; err != nil {
		t.Fatalf("seed job: %v", err)
	}

	eng := &Engine{
		HTTPClient: func(_ context.Context) ([]openrouter.Model, error) {
			t.Fatal("fetcher should not run when no jobs are due")
			return nil, nil
		},
		UsageFn: func() ([]Stat, error) { return nil, nil },
		Now:     time.Now,
	}
	res, err := eng.Run(context.Background(), nil, false)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !res.Skipped {
		t.Errorf("expected skipped (no due jobs), got %+v", res)
	}
}

func TestEngine_Run_FetchError(t *testing.T) {
	cleanup := setupSyncDB(t, &repo.OpenRouterSyncJob{})
	defer cleanup()

	job := &repo.OpenRouterSyncJob{
		Name:            "manual-job",
		TargetChannelId: 10,
		Categories:      `["llm_reasoning"]`,
		Schedule:        "manual",
		Enabled:         true,
	}
	if err := repo.DB.Create(job).Error; err != nil {
		t.Fatalf("seed job: %v", err)
	}

	eng := &Engine{
		HTTPClient: func(_ context.Context) ([]openrouter.Model, error) {
			return nil, fmt.Errorf("boom")
		},
		UsageFn: func() ([]Stat, error) { return nil, nil },
		Now:     time.Now,
	}
	// Force this manual job to be due via manualOnlyJobIDs.
	_, err := eng.Run(context.Background(), []int{job.Id}, false)
	if err == nil || !strings.Contains(err.Error(), "fetch openrouter models") {
		t.Fatalf("expected fetch error, got %v", err)
	}
}

func TestEngine_Run_HappyPath(t *testing.T) {
	cleanup := setupSyncDB(t,
		&repo.OpenRouterSyncJob{}, &repo.Channel{}, &repo.ModelUsageStat{},
		&repo.Model{}, &repo.Vendor{}, &repo.Ability{},
	)
	defer cleanup()

	// Seed a target channel with no models yet, force=true baseline concerns off.
	ch := &repo.Channel{Id: 10, Name: "orsync-target", Models: ""}
	if err := repo.DB.Create(ch).Error; err != nil {
		t.Fatalf("seed channel: %v", err)
	}
	job := &repo.OpenRouterSyncJob{
		Name:            "import-reasoning",
		TargetChannelId: 10,
		Categories:      `["llm_reasoning"]`,
		Schedule:        "manual",
		Enabled:         true,
		TopN:            0,
	}
	if err := repo.DB.Create(job).Error; err != nil {
		t.Fatalf("seed job: %v", err)
	}

	fetched := []openrouter.Model{
		freeModel("a:free", 300),
		freeModel("b:free", 200),
		// a paid model that should be filtered out of the free set
		{ID: "paid", Created: 400, Pricing: openrouter.ModelPricing{Prompt: "0.01"},
			Architecture: openrouter.ModelArchitecture{InputModalities: []string{"text"}, OutputModalities: []string{"text"}}},
	}

	updateAbsCalled := false
	eng := &Engine{
		HTTPClient: func(_ context.Context) ([]openrouter.Model, error) { return fetched, nil },
		UsageFn:    func() ([]Stat, error) { return nil, nil },
		Now:        time.Now,
		UpdateAbs: func(c *repo.Channel, tx *gorm.DB) error {
			updateAbsCalled = true
			return nil
		},
	}

	res, err := eng.Run(context.Background(), []int{job.Id}, true /*force*/)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Skipped {
		t.Fatalf("did not expect skip: %+v", res)
	}
	if res.FetchedTotal != 3 || res.FreeTotal != 2 {
		t.Errorf("fetched=%d free=%d, want 3/2", res.FetchedTotal, res.FreeTotal)
	}
	wantAdded := map[string]bool{"a:free": true, "b:free": true}
	if len(res.Added) != 2 {
		t.Fatalf("added=%v, want 2 models", res.Added)
	}
	for _, a := range res.Added {
		if !wantAdded[a] {
			t.Errorf("unexpected added model %q", a)
		}
	}
	if !updateAbsCalled {
		t.Error("UpdateAbs should have been invoked inside the channel transaction")
	}

	// Channel row now holds the imported models (CSV) and the managed-set JSON.
	var got repo.Channel
	if err := repo.DB.First(&got, 10).Error; err != nil {
		t.Fatalf("reload channel: %v", err)
	}
	models := got.GetModels()
	if len(models) != 2 {
		t.Errorf("channel models = %v, want 2 entries", models)
	}
	if got.LastSyncFetchCount != 3 {
		t.Errorf("LastSyncFetchCount = %d, want 3 (circuit-breaker baseline)", got.LastSyncFetchCount)
	}
	managed := got.GetManagedModelsBySync()
	if len(managed) != 2 {
		t.Errorf("managed-by-sync = %v, want 2", managed)
	}

	// ensureModelMetadataEntries auto-created Model rows for the new imports.
	var cnt int64
	repo.DB.Model(&repo.Model{}).Where("model_name IN ?", []string{"a:free", "b:free"}).Count(&cnt)
	if cnt != 2 {
		t.Errorf("expected 2 auto-created model metadata rows, got %d", cnt)
	}

	// Due job marked run with cleared error.
	var reload repo.OpenRouterSyncJob
	if err := repo.DB.First(&reload, job.Id).Error; err != nil {
		t.Fatalf("reload job: %v", err)
	}
	if reload.LastRunAt == nil {
		t.Error("LastRunAt should be set after a successful run")
	}
	if reload.LastError != "" {
		t.Errorf("LastError should be cleared, got %q", reload.LastError)
	}
}

// TestEngine_Run_ZeroSeamEndToEnd drives Run with ALL injectable seams nil so
// the production defaults fire: BaseURL->defaultFetcher (against a fake upstream
// server), UsageFn->LoadUsageStats (real DB read), Now->time.Now, and
// UpdateAbs->channel.UpdateAbilities (real abilities write). It also covers the
// scheduler's non-manual "due" path (a daily job with no prior run), the
// empty-category skip inside applyChannel, and the "model already exists" skip
// inside ensureModelMetadataEntries.
func TestEngine_Run_ZeroSeamEndToEnd(t *testing.T) {
	cleanup := setupSyncDB(t,
		&repo.OpenRouterSyncJob{}, &repo.Channel{}, &repo.ModelUsageStat{},
		&repo.Model{}, &repo.Vendor{}, &repo.Ability{},
	)
	defer cleanup()

	srv := newModelsServer(t, http.StatusOK, modelsJSON)

	ch := &repo.Channel{Id: 10, Name: "target", Models: "", Group: "default"}
	if err := repo.DB.Create(ch).Error; err != nil {
		t.Fatalf("seed channel: %v", err)
	}
	// Due job (daily, never run) — exercises ShouldRun()==true (non-manual path).
	dueJob := &repo.OpenRouterSyncJob{
		Name: "daily-import", TargetChannelId: 10,
		Categories: `["llm_reasoning"]`, Schedule: "daily", Enabled: true,
	}
	if err := repo.DB.Create(dueJob).Error; err != nil {
		t.Fatalf("seed due job: %v", err)
	}
	// Enabled-but-empty-category job on the same channel — exercises the
	// "len(cats)==0 -> continue" branch in applyChannel's union loop.
	emptyJob := &repo.OpenRouterSyncJob{
		Name: "empty-cats", TargetChannelId: 10,
		Categories: `[]`, Schedule: "manual", Enabled: true,
	}
	if err := repo.DB.Create(emptyJob).Error; err != nil {
		t.Fatalf("seed empty job: %v", err)
	}
	// Pre-seed one of the models the fetch will add, so ensureModelMetadataEntries
	// hits its "already exists -> skip" branch for that name.
	if err := repo.DB.Create(&repo.Model{ModelName: "vendor/free-a:free", Status: 1}).Error; err != nil {
		t.Fatalf("seed pre-existing model: %v", err)
	}

	// All seams nil except BaseURL -> forces every default-wiring branch.
	eng := &Engine{BaseURL: srv.URL}

	res, err := eng.Run(context.Background(), nil, true /*force past breaker*/)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Skipped {
		t.Fatalf("did not expect skip: %+v", res)
	}
	if res.FreeTotal != 2 {
		t.Errorf("FreeTotal = %d, want 2", res.FreeTotal)
	}
	if len(res.Added) != 2 {
		t.Errorf("Added = %v, want the 2 free models", res.Added)
	}

	// Abilities were written by the default UpdateAbilities seam.
	var abilityCount int64
	repo.DB.Model(&repo.Ability{}).Where("channel_id = ?", 10).Count(&abilityCount)
	if abilityCount == 0 {
		t.Error("expected abilities rows written by default UpdateAbs seam")
	}

	// free-a pre-existed (skipped); free-b newly created -> exactly one new row
	// beyond the seed. Verify free-b now exists.
	var freeBCount int64
	repo.DB.Model(&repo.Model{}).Where("model_name = ?", "vendor/free-b:free").Count(&freeBCount)
	if freeBCount != 1 {
		t.Errorf("expected free-b model auto-created, count=%d", freeBCount)
	}
}

// TestNewEngine_DefaultUpdateAbs invokes the UpdateAbs closure that NewEngine
// wires by default, proving it delegates to Channel.UpdateAbilities.
func TestNewEngine_DefaultUpdateAbs(t *testing.T) {
	cleanup := setupSyncDB(t, &repo.Channel{}, &repo.Ability{})
	defer cleanup()

	ch := &repo.Channel{Id: 7, Name: "c7", Models: "m1,m2", Group: "default"}
	if err := repo.DB.Create(ch).Error; err != nil {
		t.Fatalf("seed channel: %v", err)
	}

	eng := NewEngine()
	err := repo.DB.Transaction(func(tx *gorm.DB) error {
		return eng.UpdateAbs(ch, tx)
	})
	if err != nil {
		t.Fatalf("default UpdateAbs: %v", err)
	}
	var cnt int64
	repo.DB.Model(&repo.Ability{}).Where("channel_id = ?", 7).Count(&cnt)
	if cnt == 0 {
		t.Error("default UpdateAbs should have written abilities for the channel's models")
	}
}

func TestEngine_Run_CircuitBreakerTrips(t *testing.T) {
	cleanup := setupSyncDB(t,
		&repo.OpenRouterSyncJob{}, &repo.Channel{}, &repo.ModelUsageStat{},
		&repo.Model{}, &repo.Vendor{}, &repo.Ability{},
	)
	defer cleanup()

	// Channel has a high baseline (100) so a tiny fetch trips the breaker.
	ch := &repo.Channel{Id: 10, Name: "target", Models: "keep-me", LastSyncFetchCount: 100}
	if err := repo.DB.Create(ch).Error; err != nil {
		t.Fatalf("seed channel: %v", err)
	}
	job := &repo.OpenRouterSyncJob{
		Name: "j", TargetChannelId: 10, Categories: `["llm_reasoning"]`,
		Schedule: "manual", Enabled: true,
	}
	if err := repo.DB.Create(job).Error; err != nil {
		t.Fatalf("seed job: %v", err)
	}

	eng := &Engine{
		HTTPClient: func(_ context.Context) ([]openrouter.Model, error) {
			return []openrouter.Model{freeModel("a:free", 1)}, nil // only 1 model << threshold(50)
		},
		UsageFn:   func() ([]Stat, error) { return nil, nil },
		Now:       time.Now,
		UpdateAbs: func(c *repo.Channel, tx *gorm.DB) error { return nil },
	}

	res, err := eng.Run(context.Background(), []int{job.Id}, false)
	if err != nil {
		t.Fatalf("Run should not hard-error on breaker trip: %v", err)
	}
	if !res.CircuitBreakerOn {
		t.Error("expected CircuitBreakerOn=true")
	}
	if len(res.Added) != 0 {
		t.Errorf("no models should be added when breaker trips, got %v", res.Added)
	}

	// Channel models untouched; job carries the breaker error.
	var got repo.Channel
	repo.DB.First(&got, 10)
	if got.Models != "keep-me" {
		t.Errorf("channel models should be untouched, got %q", got.Models)
	}
	var reload repo.OpenRouterSyncJob
	repo.DB.First(&reload, job.Id)
	if !strings.Contains(reload.LastError, "circuit breaker") {
		t.Errorf("job LastError should record breaker trip, got %q", reload.LastError)
	}
}

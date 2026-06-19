package openrouter_sync

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	entity "github.com/LurusTech/lurus-hub/internal/domain/entity"

	"github.com/LurusTech/lurus-hub/internal/adapter/provider/openrouter"
	"github.com/LurusTech/lurus-hub/internal/adapter/repo"
)

// ---------------------------------------------------------------------------
// CircuitBreakerError
// ---------------------------------------------------------------------------

func TestCircuitBreakerError_Error(t *testing.T) {
	tests := []struct {
		name     string
		got      int
		baseline int
		want     string
	}{
		{
			name:     "zero baseline",
			got:      3,
			baseline: 0,
			want:     "circuit breaker tripped: fetched 3, baseline 0 (threshold = max(10, baseline*0.5))",
		},
		{
			name:     "non-trivial baseline",
			got:      40,
			baseline: 100,
			want:     "circuit breaker tripped: fetched 40, baseline 100 (threshold = max(10, baseline*0.5))",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := &CircuitBreakerError{Got: tc.got, Baseline: tc.baseline}
			if err.Error() != tc.want {
				t.Errorf("got %q, want %q", err.Error(), tc.want)
			}
		})
	}
}

// TestCircuitBreakerError_IsUnwrappable verifies that errors.As works for
// callers that want to inspect Got/Baseline without string-parsing.
func TestCircuitBreakerError_IsUnwrappable(t *testing.T) {
	original := &CircuitBreakerError{Got: 5, Baseline: 50}
	wrapped := fmt.Errorf("outer: %w", original)

	var target *CircuitBreakerError
	if !errors.As(wrapped, &target) {
		t.Fatal("errors.As should find *CircuitBreakerError through wrapping")
	}
	if target.Got != 5 || target.Baseline != 50 {
		t.Errorf("unwrapped fields: Got=%d Baseline=%d, want Got=5 Baseline=50", target.Got, target.Baseline)
	}
}

// ---------------------------------------------------------------------------
// groupByChannel
// ---------------------------------------------------------------------------

func makeJob(id, channelID int) *entity.OpenRouterSyncJob {
	return &entity.OpenRouterSyncJob{
		Id:              id,
		TargetChannelId: channelID,
		Enabled:         true,
		Categories:      `["llm_reasoning"]`,
	}
}

func TestGroupByChannel_SingleJobDue(t *testing.T) {
	j1 := makeJob(1, 10)
	groups := groupByChannel([]*repo.OpenRouterSyncJob{j1}, []*repo.OpenRouterSyncJob{j1})

	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	g, ok := groups[10]
	if !ok {
		t.Fatal("group for channel 10 not found")
	}
	if len(g.allJobs) != 1 || len(g.dueJobs) != 1 {
		t.Errorf("allJobs=%d dueJobs=%d, want 1/1", len(g.allJobs), len(g.dueJobs))
	}
}

func TestGroupByChannel_NoDueJobsDropsGroup(t *testing.T) {
	j1 := makeJob(1, 10)
	j2 := makeJob(2, 10)
	// j1 is enabled but not due; j2 is due.
	groups := groupByChannel(
		[]*repo.OpenRouterSyncJob{j1, j2},
		[]*repo.OpenRouterSyncJob{j2},
	)

	g := groups[10]
	if g == nil {
		t.Fatal("group for channel 10 should be present because j2 is due")
	}
	if len(g.allJobs) != 2 {
		t.Errorf("allJobs = %d, want 2 (all enabled jobs contribute to managed union)", len(g.allJobs))
	}
	if len(g.dueJobs) != 1 {
		t.Errorf("dueJobs = %d, want 1", len(g.dueJobs))
	}
}

func TestGroupByChannel_TwoChannelsBothDue(t *testing.T) {
	j1 := makeJob(1, 10)
	j2 := makeJob(2, 20)
	groups := groupByChannel(
		[]*repo.OpenRouterSyncJob{j1, j2},
		[]*repo.OpenRouterSyncJob{j1, j2},
	)

	if len(groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(groups))
	}
	if _, ok := groups[10]; !ok {
		t.Error("group for channel 10 missing")
	}
	if _, ok := groups[20]; !ok {
		t.Error("group for channel 20 missing")
	}
}

func TestGroupByChannel_NoDueJobsReturnsEmptyMap(t *testing.T) {
	j1 := makeJob(1, 10)
	// due list is empty — no channel should appear.
	groups := groupByChannel(
		[]*repo.OpenRouterSyncJob{j1},
		[]*repo.OpenRouterSyncJob{},
	)
	if len(groups) != 0 {
		t.Errorf("expected empty groups, got %d", len(groups))
	}
}

func TestGroupByChannel_MultipleJobsSameChannel(t *testing.T) {
	j1 := makeJob(1, 5)
	j2 := makeJob(2, 5)
	j3 := makeJob(3, 5)
	// Only j1 and j3 are due.
	groups := groupByChannel(
		[]*repo.OpenRouterSyncJob{j1, j2, j3},
		[]*repo.OpenRouterSyncJob{j1, j3},
	)

	g := groups[5]
	if g == nil {
		t.Fatal("expected group for channel 5")
	}
	if len(g.allJobs) != 3 {
		t.Errorf("allJobs = %d, want 3", len(g.allJobs))
	}
	if len(g.dueJobs) != 2 {
		t.Errorf("dueJobs = %d, want 2", len(g.dueJobs))
	}
}

// ---------------------------------------------------------------------------
// joinCSV
// ---------------------------------------------------------------------------

func TestJoinCSV(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		want  string
	}{
		{"empty", []string{}, ""},
		{"single", []string{"a"}, "a"},
		{"multiple", []string{"a", "b", "c"}, "a,b,c"},
		{"with spaces in elements", []string{"model x", "model y"}, "model x,model y"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := joinCSV(tc.input)
			if got != tc.want {
				t.Errorf("joinCSV(%v) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// tryLoadUsage
// ---------------------------------------------------------------------------

func TestTryLoadUsage_ReturnsMapOnSuccess(t *testing.T) {
	now := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	fn := func() ([]Stat, error) {
		return []Stat{
			{ModelName: "m1", Count24h: 10, LastUpdatedAt: now.Add(-1 * time.Hour)},
			{ModelName: "m2", Count24h: 5, LastUpdatedAt: now.Add(-2 * time.Hour)},
		}, nil
	}
	m := tryLoadUsage(fn, now)
	if m == nil {
		t.Fatal("expected non-nil UsageMap")
	}
	if m["m1"] != 10 {
		t.Errorf("m1 count = %d, want 10", m["m1"])
	}
	if m["m2"] != 5 {
		t.Errorf("m2 count = %d, want 5", m["m2"])
	}
}

func TestTryLoadUsage_ReturnsNilOnError(t *testing.T) {
	fn := func() ([]Stat, error) {
		return nil, errors.New("db unavailable")
	}
	m := tryLoadUsage(fn, time.Now())
	if m != nil {
		t.Errorf("expected nil UsageMap on error, got %v", m)
	}
}

func TestTryLoadUsage_ReturnsNilWhenAllStale(t *testing.T) {
	now := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	fn := func() ([]Stat, error) {
		return []Stat{
			{ModelName: "old", Count24h: 999, LastUpdatedAt: now.Add(-48 * time.Hour)},
		}, nil
	}
	m := tryLoadUsage(fn, now)
	if m != nil {
		t.Errorf("expected nil when all stats stale, got %v", m)
	}
}

func TestTryLoadUsage_ReturnsNilOnEmptySlice(t *testing.T) {
	fn := func() ([]Stat, error) {
		return []Stat{}, nil
	}
	m := tryLoadUsage(fn, time.Now())
	if m != nil {
		t.Errorf("expected nil for empty stats, got %v", m)
	}
}

// ---------------------------------------------------------------------------
// Engine.Preview — injectable seam, no DB needed
// ---------------------------------------------------------------------------

func TestEngine_Preview_FiltersAndRanks(t *testing.T) {
	now := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)

	// Stub fetcher: returns 3 models — 2 free text, 1 paid, 1 free image
	stubFetcher := func(_ context.Context) ([]openrouter.Model, error) {
		return []openrouter.Model{
			{
				ID:      "text-free-a:free",
				Created: 200,
				Pricing: openrouter.ModelPricing{Prompt: "0", Completion: "0", Image: "0", Request: "0"},
				Architecture: openrouter.ModelArchitecture{
					InputModalities:  []string{"text"},
					OutputModalities: []string{"text"},
				},
			},
			{
				ID:      "text-free-b:free",
				Created: 100,
				Pricing: openrouter.ModelPricing{Prompt: "0", Completion: "0", Image: "0", Request: "0"},
				Architecture: openrouter.ModelArchitecture{
					InputModalities:  []string{"text"},
					OutputModalities: []string{"text"},
				},
			},
			{
				ID:      "paid-model",
				Created: 300,
				Pricing: openrouter.ModelPricing{Prompt: "0.001", Completion: "0.002"},
				Architecture: openrouter.ModelArchitecture{
					InputModalities:  []string{"text"},
					OutputModalities: []string{"text"},
				},
			},
			{
				ID:      "image-free:free",
				Created: 150,
				Pricing: openrouter.ModelPricing{Prompt: "0", Completion: "0", Image: "0", Request: "0"},
				Architecture: openrouter.ModelArchitecture{
					InputModalities:  []string{"text"},
					OutputModalities: []string{"image"},
				},
			},
		}, nil
	}

	job := &entity.OpenRouterSyncJob{
		Categories: `["llm_reasoning"]`,
		TopN:       1,
	}

	eng := &Engine{
		HTTPClient: stubFetcher,
		UsageFn:    func() ([]Stat, error) { return nil, nil },
		Now:        func() time.Time { return now },
	}

	result, err := eng.Preview(context.Background(), job)
	if err != nil {
		t.Fatalf("Preview returned error: %v", err)
	}
	// Only the 2 free text models qualify; TopN=1 takes the newest (Created=200).
	if len(result) != 1 {
		t.Fatalf("expected 1 result, got %d: %v", len(result), result)
	}
	if result[0].ID != "text-free-a:free" {
		t.Errorf("top model = %q, want %q", result[0].ID, "text-free-a:free")
	}
}

func TestEngine_Preview_FetcherError(t *testing.T) {
	eng := &Engine{
		HTTPClient: func(_ context.Context) ([]openrouter.Model, error) {
			return nil, errors.New("network timeout")
		},
		UsageFn: func() ([]Stat, error) { return nil, nil },
		Now:     time.Now,
	}

	job := &entity.OpenRouterSyncJob{Categories: `["llm_reasoning"]`, TopN: 10}
	_, err := eng.Preview(context.Background(), job)
	if err == nil {
		t.Fatal("expected error from fetcher, got nil")
	}
}

func TestEngine_Preview_EmptyFetch(t *testing.T) {
	eng := &Engine{
		HTTPClient: func(_ context.Context) ([]openrouter.Model, error) {
			return []openrouter.Model{}, nil
		},
		UsageFn: func() ([]Stat, error) { return nil, nil },
		Now:     time.Now,
	}

	job := &entity.OpenRouterSyncJob{Categories: `["llm_reasoning"]`, TopN: 10}
	result, err := eng.Preview(context.Background(), job)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty result, got %v", result)
	}
}

func TestEngine_Preview_NoCategoryMatchReturnsEmpty(t *testing.T) {
	stubFetcher := func(_ context.Context) ([]openrouter.Model, error) {
		return []openrouter.Model{
			{
				ID:      "img:free",
				Created: 100,
				Pricing: openrouter.ModelPricing{Prompt: "0", Completion: "0", Image: "0", Request: "0"},
				Architecture: openrouter.ModelArchitecture{
					InputModalities:  []string{"text"},
					OutputModalities: []string{"image"},
				},
			},
		}, nil
	}

	eng := &Engine{
		HTTPClient: stubFetcher,
		UsageFn:    func() ([]Stat, error) { return nil, nil },
		Now:        time.Now,
	}

	// llm_reasoning wants text→text; img:free is image_gen — should be filtered out.
	job := &entity.OpenRouterSyncJob{
		Categories: `["llm_reasoning"]`,
		TopN:       10,
	}
	result, err := eng.Preview(context.Background(), job)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty result, got %v", result)
	}
}

func TestEngine_Preview_UsageFnApplied(t *testing.T) {
	// Two free text models; usage ranks b above a despite a having newer Created.
	now := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	stubFetcher := func(_ context.Context) ([]openrouter.Model, error) {
		return []openrouter.Model{
			{
				ID:      "a:free",
				Created: 200,
				Pricing: openrouter.ModelPricing{Prompt: "0", Completion: "0", Image: "0", Request: "0"},
				Architecture: openrouter.ModelArchitecture{
					InputModalities:  []string{"text"},
					OutputModalities: []string{"text"},
				},
			},
			{
				ID:      "b:free",
				Created: 100,
				Pricing: openrouter.ModelPricing{Prompt: "0", Completion: "0", Image: "0", Request: "0"},
				Architecture: openrouter.ModelArchitecture{
					InputModalities:  []string{"text"},
					OutputModalities: []string{"text"},
				},
			},
		}, nil
	}

	stubUsage := func() ([]Stat, error) {
		return []Stat{
			{ModelName: "b:free", Count24h: 1000, LastUpdatedAt: now.Add(-30 * time.Minute)},
		}, nil
	}

	eng := &Engine{
		HTTPClient: stubFetcher,
		UsageFn:    stubUsage,
		Now:        func() time.Time { return now },
	}

	job := &entity.OpenRouterSyncJob{Categories: `["llm_reasoning"]`, TopN: 0}
	result, err := eng.Preview(context.Background(), job)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 results, got %d", len(result))
	}
	// b:free should be ranked first due to higher usage.
	if result[0].ID != "b:free" {
		t.Errorf("top result = %q, want %q (usage should outrank Created)", result[0].ID, "b:free")
	}
}

// ---------------------------------------------------------------------------
// Engine.Run — DB-bound functions that cannot be tested without a seam
// ---------------------------------------------------------------------------
//
// Engine.Run calls repo.ListEnabledOpenRouterSyncJobs (GORM, no interface
// injection) as its first step, making it untestable in unit context without
// a live PG instance. applyChannel similarly requires repo.DB.WithContext and
// repo.GetChannelForUpdate. These are covered by the pg-integration suite.
//
// NewEngine and defaultFetcher are also excluded: NewEngine just sets fields
// (trivially correct) and defaultFetcher immediately produces an HTTP call.
// Neither has untested logic that warrants a hermetic test.

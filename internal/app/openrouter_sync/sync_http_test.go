package openrouter_sync

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	entity "github.com/LurusTech/lurus-hub/internal/domain/entity"
)

// modelsJSON is a minimal but realistic OpenRouter /v1/models payload: two free
// text models and one paid model. Downstream billing/classification depends on
// this shape being parsed correctly.
const modelsJSON = `{
  "data": [
    {
      "id": "vendor/free-a:free",
      "created": 300,
      "architecture": {"input_modalities": ["text"], "output_modalities": ["text"]},
      "pricing": {"prompt": "0", "completion": "0", "image": "0", "request": "0"}
    },
    {
      "id": "vendor/free-b:free",
      "created": 200,
      "architecture": {"input_modalities": ["text"], "output_modalities": ["text"]},
      "pricing": {"prompt": "0", "completion": "0", "image": "0", "request": "0"}
    },
    {
      "id": "vendor/paid",
      "created": 400,
      "architecture": {"input_modalities": ["text"], "output_modalities": ["text"]},
      "pricing": {"prompt": "0.001", "completion": "0.002"}
    }
  ]
}`

func newModelsServer(t *testing.T, status int, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// TestNewEngine_Defaults verifies NewEngine wires production defaults so a
// zero-config caller gets a usable engine (real fields, not nil seams).
func TestNewEngine_Defaults(t *testing.T) {
	eng := NewEngine()
	if eng == nil {
		t.Fatal("NewEngine returned nil")
	}
	if eng.BaseURL != "https://openrouter.ai/api" {
		t.Errorf("BaseURL = %q, want production default", eng.BaseURL)
	}
	if eng.HTTPClient == nil {
		t.Error("HTTPClient default fetcher must be set")
	}
	if eng.UsageFn == nil {
		t.Error("UsageFn default must be set")
	}
	if eng.Now == nil {
		t.Error("Now default must be set")
	}
	if eng.UpdateAbs == nil {
		t.Error("UpdateAbs default must be set")
	}
}

// TestDefaultFetcher_ParsesServerResponse exercises the real HTTP fetch closure
// (defaultFetcher -> openrouter.FetchModels) against a fake upstream, proving
// the catalog parse path that billing/classification consumes.
func TestDefaultFetcher_ParsesServerResponse(t *testing.T) {
	srv := newModelsServer(t, http.StatusOK, modelsJSON)

	fetch := defaultFetcher(srv.URL)
	models, err := fetch(context.Background())
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if len(models) != 3 {
		t.Fatalf("got %d models, want 3", len(models))
	}
	free := 0
	for i := range models {
		if models[i].IsFree() {
			free++
		}
	}
	if free != 2 {
		t.Errorf("free models = %d, want 2", free)
	}
	if models[0].ID != "vendor/free-a:free" || models[0].Created != 300 {
		t.Errorf("first model parsed wrong: %+v", models[0])
	}
}

// TestDefaultFetcher_HTTPErrorStatus covers the non-200 branch of the fetch.
func TestDefaultFetcher_HTTPErrorStatus(t *testing.T) {
	srv := newModelsServer(t, http.StatusInternalServerError, "upstream boom")

	_, err := defaultFetcher(srv.URL)(context.Background())
	if err == nil {
		t.Fatal("expected error on HTTP 500, got nil")
	}
}

// TestDefaultFetcher_MalformedJSON covers the decode-error branch.
func TestDefaultFetcher_MalformedJSON(t *testing.T) {
	srv := newModelsServer(t, http.StatusOK, "{not-json")

	_, err := defaultFetcher(srv.URL)(context.Background())
	if err == nil {
		t.Fatal("expected decode error on malformed JSON, got nil")
	}
}

// TestEngine_Preview_NilDefaultsUseBaseURL leaves HTTPClient and Now nil so
// Preview must build the default fetcher from BaseURL and fall back to a real
// clock — the zero-seam production path — then fetch+classify against the fake
// server.
func TestEngine_Preview_NilDefaultsUseBaseURL(t *testing.T) {
	srv := newModelsServer(t, http.StatusOK, modelsJSON)

	eng := &Engine{
		BaseURL: srv.URL,
		// HTTPClient nil  -> defaultFetcher(BaseURL)
		// Now nil         -> time.Now
		UsageFn: func() ([]Stat, error) { return nil, nil }, // avoid DB
	}

	job := &entity.OpenRouterSyncJob{Categories: `["llm_reasoning"]`, TopN: 0}
	result, err := eng.Preview(context.Background(), job)
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	// Two free text models qualify for llm_reasoning; paid model excluded.
	if len(result) != 2 {
		t.Fatalf("got %d results, want 2 free text models", len(result))
	}
	ids := map[string]bool{result[0].ID: true, result[1].ID: true}
	if !ids["vendor/free-a:free"] || !ids["vendor/free-b:free"] {
		t.Errorf("unexpected preview models: %v", ids)
	}
	// Sanity: HTTPClient seam was populated as a side effect.
	if eng.HTTPClient == nil {
		t.Error("Preview should have set the default HTTPClient")
	}
	if eng.Now == nil {
		t.Error("Preview should have set the default Now clock")
	}
	_ = time.Now
}

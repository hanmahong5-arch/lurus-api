package types

import (
	"encoding/json"
	"errors"
	"sort"
	"strconv"
	"sync"
	"testing"
)

// TestConstructors_ApplyOptions covers the `for _, op := range ops` branch in
// each of the three NewAPIError constructors — options must actually mutate the
// resulting error (skip-retry drives failover, hide-msg prevents leaks).
func TestConstructors_ApplyOptions(t *testing.T) {
	t.Run("NewErrorWithStatusCode", func(t *testing.T) {
		e := NewErrorWithStatusCode(errors.New("orig"), ErrorCodeBadRequestBody, 422,
			ErrOptionWithSkipRetry(), ErrOptionWithNoRecordErrorLog())
		if !IsSkipRetryError(e) {
			t.Errorf("skip-retry option not applied")
		}
		if IsRecordErrorLog(e) {
			t.Errorf("no-record-error-log option not applied")
		}
	})

	t.Run("WithOpenAIError", func(t *testing.T) {
		e := WithOpenAIError(OpenAIError{Message: "m", Type: "t", Code: "c"}, 400,
			ErrOptionWithHideErrMsg("hidden"), ErrOptionWithSkipRetry())
		if e.Error() != "hidden" {
			t.Errorf("hide-msg option not applied: Error()=%q", e.Error())
		}
		if !IsSkipRetryError(e) {
			t.Errorf("skip-retry option not applied")
		}
	})

	t.Run("WithClaudeError", func(t *testing.T) {
		e := WithClaudeError(ClaudeError{Type: "api_error", Message: "busy"}, 529,
			ErrOptionWithSkipRetry())
		if !IsSkipRetryError(e) {
			t.Errorf("skip-retry option not applied to claude error")
		}
	})
}

// --- RWMap ---------------------------------------------------------------

func TestRWMap_SetGetLenClear(t *testing.T) {
	m := NewRWMap[string, int]()
	if m.Len() != 0 {
		t.Fatalf("new map Len = %d, want 0", m.Len())
	}
	if _, ok := m.Get("missing"); ok {
		t.Errorf("Get on empty map returned ok=true")
	}

	m.Set("a", 1)
	m.Set("b", 2)
	if m.Len() != 2 {
		t.Fatalf("Len after 2 Sets = %d, want 2", m.Len())
	}
	if v, ok := m.Get("a"); !ok || v != 1 {
		t.Errorf("Get(a) = %d,%v want 1,true", v, ok)
	}

	// Overwrite existing key.
	m.Set("a", 10)
	if v, _ := m.Get("a"); v != 10 {
		t.Errorf("Get(a) after overwrite = %d, want 10", v)
	}

	m.Clear()
	if m.Len() != 0 {
		t.Errorf("Len after Clear = %d, want 0", m.Len())
	}
	if _, ok := m.Get("a"); ok {
		t.Errorf("Get(a) after Clear returned ok=true")
	}
}

func TestRWMap_AddAll(t *testing.T) {
	m := NewRWMap[string, int]()
	m.Set("keep", 1)
	m.AddAll(map[string]int{"x": 2, "y": 3, "keep": 99})
	if m.Len() != 3 {
		t.Fatalf("Len after AddAll = %d, want 3", m.Len())
	}
	if v, _ := m.Get("keep"); v != 99 {
		t.Errorf("AddAll should overwrite existing key: got %d, want 99", v)
	}
	if v, _ := m.Get("x"); v != 2 {
		t.Errorf("Get(x) = %d, want 2", v)
	}
}

func TestRWMap_ReadAll_IsCopy(t *testing.T) {
	m := NewRWMap[string, int]()
	m.Set("a", 1)
	m.Set("b", 2)
	snap := m.ReadAll()
	if len(snap) != 2 || snap["a"] != 1 || snap["b"] != 2 {
		t.Fatalf("ReadAll = %v, want {a:1,b:2}", snap)
	}
	// Mutating the snapshot must not affect the map.
	snap["a"] = 999
	delete(snap, "b")
	if v, _ := m.Get("a"); v != 1 {
		t.Errorf("mutating snapshot leaked into map: Get(a)=%d, want 1", v)
	}
	if _, ok := m.Get("b"); !ok {
		t.Errorf("deleting from snapshot leaked into map: b gone")
	}
}

func TestRWMap_MarshalUnmarshalJSON(t *testing.T) {
	m := NewRWMap[string, int]()
	m.Set("one", 1)
	m.Set("two", 2)

	b, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	m2 := NewRWMap[string, int]()
	if err := json.Unmarshal(b, m2); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if m2.Len() != 2 {
		t.Fatalf("round-trip Len = %d, want 2", m2.Len())
	}
	if v, ok := m2.Get("one"); !ok || v != 1 {
		t.Errorf("round-trip Get(one) = %d,%v want 1,true", v, ok)
	}
	if v, ok := m2.Get("two"); !ok || v != 2 {
		t.Errorf("round-trip Get(two) = %d,%v want 2,true", v, ok)
	}
}

func TestRWMap_UnmarshalJSON_Error(t *testing.T) {
	m := NewRWMap[string, int]()
	if err := json.Unmarshal([]byte(`{not valid json`), m); err == nil {
		t.Errorf("UnmarshalJSON on invalid JSON: want error, got nil")
	}
}

func TestLoadFromJsonString(t *testing.T) {
	m := NewRWMap[string, int]()
	m.Set("stale", 42) // must be replaced by the load
	if err := LoadFromJsonString(m, `{"a":1,"b":2}`); err != nil {
		t.Fatalf("LoadFromJsonString error: %v", err)
	}
	if m.Len() != 2 {
		t.Fatalf("Len after load = %d, want 2 (old data must be dropped)", m.Len())
	}
	if _, ok := m.Get("stale"); ok {
		t.Errorf("stale key survived LoadFromJsonString")
	}
	if v, _ := m.Get("a"); v != 1 {
		t.Errorf("Get(a) = %d, want 1", v)
	}

	if err := LoadFromJsonString(m, `{bad`); err == nil {
		t.Errorf("LoadFromJsonString on invalid JSON: want error, got nil")
	}
}

// TestRWMap_ConcurrentAccess exercises the RWMutex under -race.
func TestRWMap_ConcurrentAccess(t *testing.T) {
	m := NewRWMap[int, int]()
	const workers = 8
	const perWorker = 200
	var wg sync.WaitGroup
	wg.Add(workers * 2)
	for w := 0; w < workers; w++ {
		go func(base int) { // writers
			defer wg.Done()
			for i := 0; i < perWorker; i++ {
				m.Set(base*perWorker+i, i)
			}
		}(w)
		go func() { // readers
			defer wg.Done()
			for i := 0; i < perWorker; i++ {
				_ = m.Len()
				_ = m.ReadAll()
				_, _ = m.Get(i)
			}
		}()
	}
	wg.Wait()
	if got := m.Len(); got != workers*perWorker {
		t.Errorf("final Len = %d, want %d", got, workers*perWorker)
	}
}

// --- Set -----------------------------------------------------------------

func TestSet_AddContainsRemoveLen(t *testing.T) {
	s := NewSet[string]()
	if s.Len() != 0 {
		t.Fatalf("new set Len = %d, want 0", s.Len())
	}
	if s.Contains("x") {
		t.Errorf("empty set Contains(x) = true")
	}

	s.Add("x")
	s.Add("y")
	s.Add("x") // duplicate must not grow the set
	if s.Len() != 2 {
		t.Fatalf("Len after adds = %d, want 2", s.Len())
	}
	if !s.Contains("x") || !s.Contains("y") {
		t.Errorf("Contains missed inserted items")
	}

	s.Remove("x")
	if s.Contains("x") {
		t.Errorf("Contains(x) after Remove = true")
	}
	if s.Len() != 1 {
		t.Errorf("Len after Remove = %d, want 1", s.Len())
	}
	// Removing an absent element is a no-op.
	s.Remove("absent")
	if s.Len() != 1 {
		t.Errorf("Len after removing absent = %d, want 1", s.Len())
	}
}

func TestSet_Items(t *testing.T) {
	s := NewSet[int]()
	for _, v := range []int{3, 1, 2, 1} {
		s.Add(v)
	}
	got := s.Items()
	sort.Ints(got)
	want := []int{1, 2, 3}
	if len(got) != len(want) {
		t.Fatalf("Items len = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Items[%d] = %d, want %d", i, got[i], want[i])
		}
	}

	// Empty set yields an empty (non-nil) slice.
	empty := NewSet[int]()
	if items := empty.Items(); len(items) != 0 {
		t.Errorf("empty set Items = %v, want []", items)
	}
}

// --- ChannelError --------------------------------------------------------

func TestNewChannelError(t *testing.T) {
	ce := NewChannelError(7, 3, "openai-main", true, "sk-live-secret", true)
	if ce.ChannelId != 7 {
		t.Errorf("ChannelId = %d, want 7", ce.ChannelId)
	}
	if ce.ChannelType != 3 {
		t.Errorf("ChannelType = %d, want 3", ce.ChannelType)
	}
	if ce.ChannelName != "openai-main" {
		t.Errorf("ChannelName = %q, want openai-main", ce.ChannelName)
	}
	if !ce.IsMultiKey {
		t.Errorf("IsMultiKey = false, want true")
	}
	if !ce.AutoBan {
		t.Errorf("AutoBan = false, want true")
	}
	if ce.UsingKey != "sk-live-secret" {
		t.Errorf("UsingKey = %q, want sk-live-secret", ce.UsingKey)
	}

	// Round-trips through JSON with the documented field names.
	b, err := json.Marshal(ce)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}
	var back map[string]any
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if back["channel_id"].(float64) != 7 {
		t.Errorf("json channel_id = %v, want 7", back["channel_id"])
	}
	if back["auto_ban"] != true {
		t.Errorf("json auto_ban = %v, want true", back["auto_ban"])
	}
}

// --- PriceData -----------------------------------------------------------

func TestPriceData_AddOtherRatio(t *testing.T) {
	p := &PriceData{} // nil OtherRatios map
	p.AddOtherRatio("search", 1.5)
	if p.OtherRatios == nil {
		t.Fatalf("AddOtherRatio did not lazily init the map")
	}
	if got := p.OtherRatios["search"]; got != 1.5 {
		t.Errorf("OtherRatios[search] = %v, want 1.5", got)
	}

	// Non-positive ratios are ignored (must not create a zero-priced entry).
	p.AddOtherRatio("zero", 0)
	p.AddOtherRatio("neg", -3)
	if _, ok := p.OtherRatios["zero"]; ok {
		t.Errorf("zero ratio should not be stored")
	}
	if _, ok := p.OtherRatios["neg"]; ok {
		t.Errorf("negative ratio should not be stored")
	}
	if len(p.OtherRatios) != 1 {
		t.Errorf("OtherRatios size = %d, want 1", len(p.OtherRatios))
	}

	// Overwrite an existing key with a valid ratio.
	p.AddOtherRatio("search", 2.0)
	if got := p.OtherRatios["search"]; got != 2.0 {
		t.Errorf("OtherRatios[search] after overwrite = %v, want 2.0", got)
	}
}

func TestPriceData_ToSetting(t *testing.T) {
	p := &PriceData{
		ModelPrice:      0.5,
		ModelRatio:      2,
		CompletionRatio: 3,
		CacheRatio:      0.25,
		UsePrice:        true,
		ImageRatio:      1.5,
		GroupRatioInfo:  GroupRatioInfo{GroupRatio: 1.2},
	}
	s := p.ToSetting()
	// Spot-check a few load-bearing fields are serialized.
	for _, sub := range []string{"UsePrice: true", "ModelPrice: 0.500000", "GroupRatio: 1.200000"} {
		if !contains(s, sub) {
			t.Errorf("ToSetting() missing %q; got: %s", sub, s)
		}
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (indexOf(haystack, needle) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// Ensure Set works with int keys via a small stress add (also -race relevant
// for the map access even though Set has no internal locking — single goroutine).
func TestSet_ManyElements(t *testing.T) {
	s := NewSet[string]()
	for i := 0; i < 100; i++ {
		s.Add("k" + strconv.Itoa(i))
	}
	if s.Len() != 100 {
		t.Fatalf("Len = %d, want 100", s.Len())
	}
	if !s.Contains("k50") {
		t.Errorf("Contains(k50) = false")
	}
}

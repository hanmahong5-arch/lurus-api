package helper

import (
	"net/http/httptest"
	"testing"

	relaycommon "github.com/LurusTech/lurus-hub/internal/adapter/provider/common"
	"github.com/LurusTech/lurus-hub/internal/pkg/dto"

	"github.com/gin-gonic/gin"
)

func ctxWithMapping(mapping string) *gin.Context {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	if mapping != "" {
		c.Set("model_mapping", mapping)
	}
	return c
}

// newMapInfo builds a RelayInfo whose UpstreamModelName/IsModelMapped live on the
// embedded *ChannelMeta (as they do at runtime after InitChannelMeta).
func newMapInfo(origin, upstream string) *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		OriginModelName: origin,
		ChannelMeta:     &relaycommon.ChannelMeta{UpstreamModelName: upstream},
	}
}

func TestModelMappedHelper(t *testing.T) {
	t.Run("no mapping leaves upstream untouched", func(t *testing.T) {
		c := ctxWithMapping("")
		info := newMapInfo("gpt-4", "gpt-4")
		req := &dto.GeneralOpenAIRequest{Model: "gpt-4"}
		if err := ModelMappedHelper(c, info, req); err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if info.IsModelMapped {
			t.Error("IsModelMapped should be false")
		}
		if req.Model != "gpt-4" {
			t.Errorf("model = %q, want gpt-4", req.Model)
		}
	})

	t.Run("empty object mapping is a no-op", func(t *testing.T) {
		c := ctxWithMapping("{}")
		info := newMapInfo("gpt-4", "gpt-4")
		if err := ModelMappedHelper(c, info, nil); err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if info.IsModelMapped {
			t.Error("IsModelMapped should be false")
		}
	})

	t.Run("single hop remap resolves upstream + rewrites request", func(t *testing.T) {
		c := ctxWithMapping(`{"gpt-4":"gpt-4o"}`)
		info := newMapInfo("gpt-4", "gpt-4")
		req := &dto.GeneralOpenAIRequest{Model: "gpt-4"}
		if err := ModelMappedHelper(c, info, req); err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if !info.IsModelMapped {
			t.Error("IsModelMapped should be true")
		}
		if info.UpstreamModelName != "gpt-4o" {
			t.Errorf("upstream = %q, want gpt-4o", info.UpstreamModelName)
		}
		if req.Model != "gpt-4o" {
			t.Errorf("request model = %q, want gpt-4o", req.Model)
		}
	})

	t.Run("chained remap follows to chain tail", func(t *testing.T) {
		c := ctxWithMapping(`{"a":"b","b":"c","c":"d"}`)
		info := newMapInfo("a", "a")
		if err := ModelMappedHelper(c, info, nil); err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if info.UpstreamModelName != "d" {
			t.Errorf("upstream = %q, want d", info.UpstreamModelName)
		}
	})

	t.Run("origin not in map leaves upstream unchanged", func(t *testing.T) {
		c := ctxWithMapping(`{"other":"x"}`)
		info := newMapInfo("gpt-4", "gpt-4")
		if err := ModelMappedHelper(c, info, nil); err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if info.IsModelMapped {
			t.Error("IsModelMapped should be false when origin absent from map")
		}
		if info.UpstreamModelName != "gpt-4" {
			t.Errorf("upstream = %q, want gpt-4", info.UpstreamModelName)
		}
	})

	t.Run("self map at origin is a no-op", func(t *testing.T) {
		c := ctxWithMapping(`{"gpt-4":"gpt-4"}`)
		info := newMapInfo("gpt-4", "gpt-4")
		if err := ModelMappedHelper(c, info, nil); err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if info.IsModelMapped {
			t.Error("self-map of origin should keep IsModelMapped false")
		}
	})

	t.Run("self map after a hop terminates as mapped", func(t *testing.T) {
		// a -> b, b -> b : chain reaches b which self-maps; not origin so mapped=true
		c := ctxWithMapping(`{"a":"b","b":"b"}`)
		info := newMapInfo("a", "a")
		if err := ModelMappedHelper(c, info, nil); err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if !info.IsModelMapped {
			t.Error("IsModelMapped should be true")
		}
		if info.UpstreamModelName != "b" {
			t.Errorf("upstream = %q, want b", info.UpstreamModelName)
		}
	})

	t.Run("cycle detected", func(t *testing.T) {
		c := ctxWithMapping(`{"a":"b","b":"a"}`)
		info := newMapInfo("a", "a")
		err := ModelMappedHelper(c, info, nil)
		if err == nil || err.Error() != "model_mapping_contains_cycle" {
			t.Fatalf("expected cycle error, got %v", err)
		}
	})

	t.Run("invalid mapping json rejected", func(t *testing.T) {
		c := ctxWithMapping(`{not valid`)
		info := newMapInfo("a", "a")
		err := ModelMappedHelper(c, info, nil)
		if err == nil || err.Error() != "unmarshal_model_mapping_failed" {
			t.Fatalf("expected unmarshal error, got %v", err)
		}
	})

	t.Run("empty mapped value stops chain", func(t *testing.T) {
		c := ctxWithMapping(`{"gpt-4":""}`)
		info := newMapInfo("gpt-4", "gpt-4")
		if err := ModelMappedHelper(c, info, nil); err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if info.IsModelMapped {
			t.Error("empty mapped value should not count as a remap")
		}
	})
}

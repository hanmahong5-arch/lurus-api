package handler

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/LurusTech/lurus-hub/internal/adapter/repo"
	"github.com/LurusTech/lurus-hub/internal/pkg/common"

	"github.com/gin-gonic/gin"
)

// registerV1ModelMetaRoutes migrates the Model/Vendor tables (absent from the
// shared V2 fixture) onto ctx.DB and mounts the model-meta admin handlers.
func registerV1ModelMetaRoutes(t *testing.T, ctx *V2TestContext) {
	t.Helper()
	if err := ctx.DB.AutoMigrate(&repo.Model{}, &repo.Vendor{}); err != nil &&
		!strings.Contains(err.Error(), "already exists") {
		t.Fatalf("migrate model/vendor: %v", err)
	}
	g := ctx.Router.Group("/api/models_meta")
	g.Use(func(c *gin.Context) {
		c.Set("tenant_id", ctx.TenantID)
		c.Set("id", ctx.AdminUser.Id)
		c.Next()
	})
	g.GET("/", GetAllModelsMeta)
	g.GET("/search", SearchModelsMeta)
	g.GET("/:id", GetModelMeta)
	g.POST("/", CreateModelMeta)
	g.PUT("/", UpdateModelMeta)
	g.DELETE("/:id", DeleteModelMeta)
	g.GET("/pricing", GetModelsPricingInfo)
}

func seedModel(t *testing.T, ctx *V2TestContext, name string) *repo.Model {
	t.Helper()
	m := &repo.Model{
		ModelName:   name,
		NameRule:    repo.NameRuleExact,
		Status:      1,
		CreatedTime: common.GetTimestamp(),
	}
	if err := ctx.DB.Create(m).Error; err != nil {
		t.Fatalf("seed model %q: %v", name, err)
	}
	return m
}

func TestCreateModelMeta_V1(t *testing.T) {
	ctx := SetupV2TestRouter(t)
	defer ctx.Cleanup()
	registerV1ModelMetaRoutes(t, ctx)

	w := V2Request(ctx.Router, http.MethodPost, "/api/models_meta/",
		map[string]interface{}{"model_name": "gpt-4o", "name_rule": 0, "status": 1}, nil)
	AssertV2Status(t, w, http.StatusOK)
	AssertV2Success(t, w)

	var count int64
	ctx.DB.Model(&repo.Model{}).Where("model_name = ?", "gpt-4o").Count(&count)
	if count != 1 {
		t.Errorf("expected model persisted, count=%d", count)
	}
}

func TestCreateModelMeta_V1_EmptyName(t *testing.T) {
	ctx := SetupV2TestRouter(t)
	defer ctx.Cleanup()
	registerV1ModelMetaRoutes(t, ctx)

	w := V2Request(ctx.Router, http.MethodPost, "/api/models_meta/",
		map[string]interface{}{"model_name": ""}, nil)
	resp := ParseV2Response(t, w)
	if success, _ := resp["success"].(bool); success {
		t.Errorf("expected failure for empty model_name, body: %s", w.Body.String())
	}
}

func TestCreateModelMeta_V1_Duplicate(t *testing.T) {
	ctx := SetupV2TestRouter(t)
	defer ctx.Cleanup()
	registerV1ModelMetaRoutes(t, ctx)
	seedModel(t, ctx, "dup-model")

	w := V2Request(ctx.Router, http.MethodPost, "/api/models_meta/",
		map[string]interface{}{"model_name": "dup-model"}, nil)
	resp := ParseV2Response(t, w)
	if success, _ := resp["success"].(bool); success {
		t.Errorf("expected duplicate-name failure, body: %s", w.Body.String())
	}
}

func TestGetAllModelsMeta_V1(t *testing.T) {
	ctx := SetupV2TestRouter(t)
	defer ctx.Cleanup()
	registerV1ModelMetaRoutes(t, ctx)
	seedModel(t, ctx, "m-a")
	seedModel(t, ctx, "m-b")

	w := V2Request(ctx.Router, http.MethodGet, "/api/models_meta/?p=1&page_size=10", nil, nil)
	AssertV2Status(t, w, http.StatusOK)
	resp := AssertV2Success(t, w)
	data := resp["data"].(map[string]interface{})
	if int(data["total"].(float64)) != 2 {
		t.Errorf("total = %v, want 2 — body: %s", data["total"], w.Body.String())
	}
}

func TestSearchModelsMeta_V1(t *testing.T) {
	ctx := SetupV2TestRouter(t)
	defer ctx.Cleanup()
	registerV1ModelMetaRoutes(t, ctx)
	seedModel(t, ctx, "gpt-4o-mini")
	seedModel(t, ctx, "claude-3-opus")

	w := V2Request(ctx.Router, http.MethodGet, "/api/models_meta/search?keyword=gpt&p=1&page_size=10", nil, nil)
	AssertV2Status(t, w, http.StatusOK)
	resp := AssertV2Success(t, w)
	data := resp["data"].(map[string]interface{})
	items := data["items"].([]interface{})
	if len(items) != 1 {
		t.Fatalf("expected 1 gpt match, got %d — body: %s", len(items), w.Body.String())
	}
}

func TestGetModelMeta_V1(t *testing.T) {
	ctx := SetupV2TestRouter(t)
	defer ctx.Cleanup()
	registerV1ModelMetaRoutes(t, ctx)
	m := seedModel(t, ctx, "single-model")

	w := V2Request(ctx.Router, http.MethodGet, fmt.Sprintf("/api/models_meta/%d", m.Id), nil, nil)
	AssertV2Status(t, w, http.StatusOK)
	resp := AssertV2Success(t, w)
	data := resp["data"].(map[string]interface{})
	if data["model_name"] != "single-model" {
		t.Errorf("model_name = %v, want single-model", data["model_name"])
	}

	// Non-numeric id -> failure.
	w2 := V2Request(ctx.Router, http.MethodGet, "/api/models_meta/xyz", nil, nil)
	resp2 := ParseV2Response(t, w2)
	if success, _ := resp2["success"].(bool); success {
		t.Errorf("expected failure for non-numeric id, body: %s", w2.Body.String())
	}
}

func TestUpdateModelMeta_V1_StatusOnly(t *testing.T) {
	ctx := SetupV2TestRouter(t)
	defer ctx.Cleanup()
	registerV1ModelMetaRoutes(t, ctx)
	m := seedModel(t, ctx, "to-disable")

	w := V2Request(ctx.Router, http.MethodPut, "/api/models_meta/?status_only=true",
		map[string]interface{}{"id": m.Id, "status": 0}, nil)
	AssertV2Status(t, w, http.StatusOK)
	AssertV2Success(t, w)

	var reloaded repo.Model
	ctx.DB.First(&reloaded, m.Id)
	if reloaded.Status != 0 {
		t.Errorf("status = %d, want 0", reloaded.Status)
	}
}

func TestUpdateModelMeta_V1_MissingID(t *testing.T) {
	ctx := SetupV2TestRouter(t)
	defer ctx.Cleanup()
	registerV1ModelMetaRoutes(t, ctx)

	w := V2Request(ctx.Router, http.MethodPut, "/api/models_meta/",
		map[string]interface{}{"model_name": "no-id"}, nil)
	resp := ParseV2Response(t, w)
	if success, _ := resp["success"].(bool); success {
		t.Errorf("expected failure when id missing, body: %s", w.Body.String())
	}
}

func TestDeleteModelMeta_V1(t *testing.T) {
	ctx := SetupV2TestRouter(t)
	defer ctx.Cleanup()
	registerV1ModelMetaRoutes(t, ctx)
	m := seedModel(t, ctx, "doomed-model")

	w := V2Request(ctx.Router, http.MethodDelete, fmt.Sprintf("/api/models_meta/%d", m.Id), nil, nil)
	AssertV2Status(t, w, http.StatusOK)
	AssertV2Success(t, w)

	var count int64
	ctx.DB.Model(&repo.Model{}).Where("id = ?", m.Id).Count(&count)
	if count != 0 {
		t.Errorf("model not soft-deleted, count=%d", count)
	}
}

func TestGetModelsPricingInfo_V1(t *testing.T) {
	ctx := SetupV2TestRouter(t)
	defer ctx.Cleanup()
	registerV1ModelMetaRoutes(t, ctx)
	seedModel(t, ctx, "priced-model")

	w := V2Request(ctx.Router, http.MethodGet, "/api/models_meta/pricing", nil, nil)
	AssertV2Status(t, w, http.StatusOK)
	resp := AssertV2Success(t, w)
	items := resp["data"].([]interface{})
	if len(items) != 1 {
		t.Fatalf("expected 1 pricing row, got %d — body: %s", len(items), w.Body.String())
	}
	row := items[0].(map[string]interface{})
	if row["model_name"] != "priced-model" {
		t.Errorf("model_name = %v, want priced-model", row["model_name"])
	}
}

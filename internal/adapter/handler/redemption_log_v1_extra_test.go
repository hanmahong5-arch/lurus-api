package handler

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/LurusTech/lurus-hub/internal/adapter/repo"
	"github.com/LurusTech/lurus-hub/internal/domain/entity"
	"github.com/LurusTech/lurus-hub/internal/pkg/common"

	"github.com/gin-gonic/gin"
)

func registerV1RedemptionRoutes(ctx *V2TestContext, userID int) {
	g := ctx.Router.Group("/api/redemption")
	g.Use(func(c *gin.Context) {
		c.Set("tenant_id", ctx.TenantID)
		c.Set("id", userID)
		c.Next()
	})
	g.GET("/", GetAllRedemptions)
	g.GET("/search", SearchRedemptions)
	g.GET("/:id", GetRedemption)
	g.POST("/", AddRedemption)
	g.PUT("/", UpdateRedemption)
	g.DELETE("/:id", DeleteRedemption)
}

func TestGetAllRedemptions_V1(t *testing.T) {
	ctx := SetupV2TestRouter(t)
	defer ctx.Cleanup()
	uid := ctx.AdminUser.Id
	registerV1RedemptionRoutes(ctx, uid)

	SeedV2Redemption(t, ctx, uid)
	SeedV2Redemption(t, ctx, uid)

	w := V2Request(ctx.Router, http.MethodGet, "/api/redemption/?p=1&page_size=10", nil, nil)
	AssertV2Status(t, w, http.StatusOK)
	resp := AssertV2Success(t, w)
	data := resp["data"].(map[string]interface{})
	items := data["items"].([]interface{})
	if len(items) != 2 {
		t.Fatalf("expected 2 redemptions, got %d — body: %s", len(items), w.Body.String())
	}
}

func TestGetRedemption_V1_NotFound(t *testing.T) {
	ctx := SetupV2TestRouter(t)
	defer ctx.Cleanup()
	registerV1RedemptionRoutes(ctx, ctx.AdminUser.Id)

	w := V2Request(ctx.Router, http.MethodGet, "/api/redemption/99999", nil, nil)
	resp := ParseV2Response(t, w)
	if success, _ := resp["success"].(bool); success {
		t.Errorf("expected failure for missing redemption, body: %s", w.Body.String())
	}
}

func TestAddRedemption_V1_Success(t *testing.T) {
	ctx := SetupV2TestRouter(t)
	defer ctx.Cleanup()
	uid := ctx.AdminUser.Id
	registerV1RedemptionRoutes(ctx, uid)

	body := map[string]interface{}{
		"name":         "promo",
		"count":        3,
		"quota":        5000,
		"expired_time": 0,
	}
	w := V2Request(ctx.Router, http.MethodPost, "/api/redemption/", body, nil)
	AssertV2Status(t, w, http.StatusOK)
	resp := AssertV2Success(t, w)
	keys := resp["data"].([]interface{})
	if len(keys) != 3 {
		t.Fatalf("expected 3 generated keys, got %d — body: %s", len(keys), w.Body.String())
	}
	var count int64
	ctx.DB.Model(&repo.Redemption{}).Where("name = ?", "promo").Count(&count)
	if count != 3 {
		t.Errorf("expected 3 persisted redemption rows, got %d", count)
	}
}

func TestAddRedemption_V1_Validation(t *testing.T) {
	ctx := SetupV2TestRouter(t)
	defer ctx.Cleanup()
	registerV1RedemptionRoutes(ctx, ctx.AdminUser.Id)

	cases := []struct {
		name string
		body map[string]interface{}
	}{
		{"empty_name", map[string]interface{}{"name": "", "count": 1, "quota": 10}},
		{"zero_count", map[string]interface{}{"name": "x", "count": 0, "quota": 10}},
		{"over_100_count", map[string]interface{}{"name": "x", "count": 101, "quota": 10}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := V2Request(ctx.Router, http.MethodPost, "/api/redemption/", tc.body, nil)
			resp := ParseV2Response(t, w)
			if success, _ := resp["success"].(bool); success {
				t.Errorf("expected validation failure for %s, body: %s", tc.name, w.Body.String())
			}
		})
	}
}

func TestUpdateRedemption_V1_StatusOnly(t *testing.T) {
	ctx := SetupV2TestRouter(t)
	defer ctx.Cleanup()
	uid := ctx.AdminUser.Id
	registerV1RedemptionRoutes(ctx, uid)

	r := SeedV2Redemption(t, ctx, uid)
	body := map[string]interface{}{"id": r.Id, "status": common.RedemptionCodeStatusDisabled}
	w := V2Request(ctx.Router, http.MethodPut, "/api/redemption/?status_only=true", body, nil)
	AssertV2Status(t, w, http.StatusOK)
	AssertV2Success(t, w)

	var reloaded repo.Redemption
	ctx.DB.First(&reloaded, r.Id)
	if reloaded.Status != common.RedemptionCodeStatusDisabled {
		t.Errorf("status = %d, want disabled(%d)", reloaded.Status, common.RedemptionCodeStatusDisabled)
	}
}

func TestDeleteRedemption_V1(t *testing.T) {
	ctx := SetupV2TestRouter(t)
	defer ctx.Cleanup()
	uid := ctx.AdminUser.Id
	registerV1RedemptionRoutes(ctx, uid)

	r := SeedV2Redemption(t, ctx, uid)
	w := V2Request(ctx.Router, http.MethodDelete, fmt.Sprintf("/api/redemption/%d", r.Id), nil, nil)
	AssertV2Status(t, w, http.StatusOK)
	AssertV2Success(t, w)

	var count int64
	ctx.DB.Model(&repo.Redemption{}).Where("id = ?", r.Id).Count(&count)
	if count != 0 {
		t.Errorf("redemption not deleted, count=%d", count)
	}
}

// ─── Logs ───────────────────────────────────────────────────────────────────

func registerV1LogRoutes(ctx *V2TestContext, userID, logType int) {
	g := ctx.Router.Group("/api/log")
	g.Use(func(c *gin.Context) {
		c.Set("tenant_id", ctx.TenantID)
		c.Set("id", userID)
		c.Set("username", "v2testuser")
		c.Next()
	})
	g.GET("/", GetAllLogs)
	g.GET("/self", GetUserLogs)
	g.GET("/stat", GetLogsStat)
	g.GET("/self/stat", GetLogsSelfStat)
	g.DELETE("/", DeleteHistoryLogs)
}

func seedV1Log(t *testing.T, ctx *V2TestContext, userID int, model string, quota int) {
	t.Helper()
	l := &entity.Log{
		UserId:    userID,
		TenantId:  ctx.TenantID,
		CreatedAt: common.GetTimestamp(),
		Type:      2, // consume log
		Username:  "v2testuser",
		ModelName: model,
		Quota:     quota,
		Group:     "default",
	}
	if err := ctx.DB.Table("logs").Create(l).Error; err != nil {
		t.Fatalf("seed log: %v", err)
	}
}

func TestGetAllLogs_V1(t *testing.T) {
	ctx := SetupV2TestRouter(t)
	defer ctx.Cleanup()
	uid := ctx.NormalUser.Id
	registerV1LogRoutes(ctx, uid, 2)

	seedV1Log(t, ctx, uid, "gpt-4", 100)
	seedV1Log(t, ctx, uid, "gpt-4", 250)

	w := V2Request(ctx.Router, http.MethodGet, "/api/log/?p=1&page_size=10&type=0", nil, nil)
	AssertV2Status(t, w, http.StatusOK)
	resp := AssertV2Success(t, w)
	data := resp["data"].(map[string]interface{})
	items := data["items"].([]interface{})
	if len(items) != 2 {
		t.Fatalf("expected 2 logs, got %d — body: %s", len(items), w.Body.String())
	}
}

func TestGetUserLogs_V1(t *testing.T) {
	ctx := SetupV2TestRouter(t)
	defer ctx.Cleanup()
	uid := ctx.NormalUser.Id
	registerV1LogRoutes(ctx, uid, 2)

	seedV1Log(t, ctx, uid, "claude-3", 40)
	// A log for a different user must be excluded from the self view.
	seedV1Log(t, ctx, ctx.AdminUser.Id, "claude-3", 40)

	w := V2Request(ctx.Router, http.MethodGet, "/api/log/self?p=1&page_size=10&type=0", nil, nil)
	AssertV2Status(t, w, http.StatusOK)
	resp := AssertV2Success(t, w)
	data := resp["data"].(map[string]interface{})
	items := data["items"].([]interface{})
	if len(items) != 1 {
		t.Fatalf("expected 1 own log, got %d — body: %s", len(items), w.Body.String())
	}
}

func TestGetLogsStat_V1(t *testing.T) {
	ctx := SetupV2TestRouter(t)
	defer ctx.Cleanup()
	uid := ctx.NormalUser.Id
	registerV1LogRoutes(ctx, uid, 2)

	seedV1Log(t, ctx, uid, "gpt-4", 100)
	seedV1Log(t, ctx, uid, "gpt-4", 300)

	w := V2Request(ctx.Router, http.MethodGet, "/api/log/stat?type=0", nil, nil)
	AssertV2Status(t, w, http.StatusOK)
	resp := AssertV2Success(t, w)
	data := resp["data"].(map[string]interface{})
	// rpm counts consume rows in the rolling 60s window — both seeded logs qualify.
	if data["rpm"].(float64) != 2 {
		t.Errorf("rpm = %v, want 2 (both seeded consume logs)", data["rpm"])
	}
}

func TestGetLogsSelfStat_V1(t *testing.T) {
	ctx := SetupV2TestRouter(t)
	defer ctx.Cleanup()
	uid := ctx.NormalUser.Id
	registerV1LogRoutes(ctx, uid, 2)

	seedV1Log(t, ctx, uid, "gpt-4", 55)
	w := V2Request(ctx.Router, http.MethodGet, "/api/log/self/stat?type=0", nil, nil)
	AssertV2Status(t, w, http.StatusOK)
	AssertV2Success(t, w)
}

func TestDeleteHistoryLogs_V1(t *testing.T) {
	ctx := SetupV2TestRouter(t)
	defer ctx.Cleanup()
	uid := ctx.NormalUser.Id
	registerV1LogRoutes(ctx, uid, 2)

	// Missing target_timestamp -> success=false.
	w := V2Request(ctx.Router, http.MethodDelete, "/api/log/", nil, nil)
	resp := ParseV2Response(t, w)
	if success, _ := resp["success"].(bool); success {
		t.Errorf("expected failure without target_timestamp, body: %s", w.Body.String())
	}

	// Valid target far in the future deletes the old log we seed.
	seedV1Log(t, ctx, uid, "old-model", 10)
	future := common.GetTimestamp() + 100000
	w2 := V2Request(ctx.Router, http.MethodDelete, fmt.Sprintf("/api/log/?target_timestamp=%d", future), nil, nil)
	AssertV2Status(t, w2, http.StatusOK)
	AssertV2Success(t, w2)
}

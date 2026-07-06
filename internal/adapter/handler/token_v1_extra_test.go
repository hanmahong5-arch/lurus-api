package handler

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/LurusTech/lurus-hub/internal/adapter/repo"
	"github.com/LurusTech/lurus-hub/internal/pkg/common"

	"github.com/gin-gonic/gin"
)

// registerV1TokenRoutes mounts the v1 token handlers on the shared V2 harness
// router with a mock auth layer that sets the session "id" (the token owner)
// and tenant context the handlers read from.
func registerV1TokenRoutes(ctx *V2TestContext, userID int) {
	g := ctx.Router.Group("/api/token")
	g.Use(func(c *gin.Context) {
		c.Set("tenant_id", ctx.TenantID)
		c.Set("id", userID)
		c.Next()
	})
	g.GET("/", GetAllTokens)
	g.GET("/search", SearchTokens)
	g.GET("/:id", GetToken)
	g.POST("/", AddToken)
	g.PUT("/", UpdateToken)
	g.DELETE("/:id", DeleteToken)
	g.POST("/batch", DeleteTokenBatch)
}

func TestGetAllTokens_V1(t *testing.T) {
	ctx := SetupV2TestRouter(t)
	defer ctx.Cleanup()
	uid := ctx.NormalUser.Id
	registerV1TokenRoutes(ctx, uid)

	SeedV2Token(t, ctx, uid, "tok-a")
	SeedV2Token(t, ctx, uid, "tok-b")
	// A token owned by a different user must NOT appear.
	SeedV2Token(t, ctx, ctx.AdminUser.Id, "other-owner")

	w := V2Request(ctx.Router, http.MethodGet, "/api/token/?p=1&page_size=10", nil, nil)
	AssertV2Status(t, w, http.StatusOK)
	resp := AssertV2Success(t, w)
	data := resp["data"].(map[string]interface{})
	items := data["items"].([]interface{})
	if len(items) != 2 {
		t.Fatalf("expected 2 own tokens, got %d — body: %s", len(items), w.Body.String())
	}
}

func TestSearchTokens_V1(t *testing.T) {
	ctx := SetupV2TestRouter(t)
	defer ctx.Cleanup()
	uid := ctx.NormalUser.Id
	registerV1TokenRoutes(ctx, uid)

	SeedV2Token(t, ctx, uid, "production-key")
	SeedV2Token(t, ctx, uid, "staging-key")

	w := V2Request(ctx.Router, http.MethodGet, "/api/token/search?keyword=production", nil, nil)
	AssertV2Status(t, w, http.StatusOK)
	resp := ParseV2Response(t, w)
	items := resp["data"].([]interface{})
	if len(items) != 1 {
		t.Fatalf("expected 1 match for 'production', got %d — body: %s", len(items), w.Body.String())
	}
}

func TestGetToken_V1_OwnershipScoped(t *testing.T) {
	ctx := SetupV2TestRouter(t)
	defer ctx.Cleanup()
	uid := ctx.NormalUser.Id
	registerV1TokenRoutes(ctx, uid)

	own := SeedV2Token(t, ctx, uid, "mine")
	foreign := SeedV2Token(t, ctx, ctx.AdminUser.Id, "not-mine")

	// Own token: 200 with data.
	w := V2Request(ctx.Router, http.MethodGet, fmt.Sprintf("/api/token/%d", own.Id), nil, nil)
	AssertV2Status(t, w, http.StatusOK)
	resp := AssertV2Success(t, w)
	data := resp["data"].(map[string]interface{})
	if int(data["id"].(float64)) != own.Id {
		t.Errorf("returned wrong token id: %v", data["id"])
	}

	// Foreign token: handler scopes by (id, userId) so lookup fails -> success=false.
	w2 := V2Request(ctx.Router, http.MethodGet, fmt.Sprintf("/api/token/%d", foreign.Id), nil, nil)
	resp2 := ParseV2Response(t, w2)
	if success, _ := resp2["success"].(bool); success {
		t.Errorf("expected failure fetching another user's token, body: %s", w2.Body.String())
	}
}

func TestGetToken_V1_BadID(t *testing.T) {
	ctx := SetupV2TestRouter(t)
	defer ctx.Cleanup()
	uid := ctx.NormalUser.Id
	registerV1TokenRoutes(ctx, uid)

	w := V2Request(ctx.Router, http.MethodGet, "/api/token/not-a-number", nil, nil)
	resp := ParseV2Response(t, w)
	if success, _ := resp["success"].(bool); success {
		t.Errorf("expected failure for non-numeric id, body: %s", w.Body.String())
	}
}

func TestAddToken_V1_Success(t *testing.T) {
	ctx := SetupV2TestRouter(t)
	defer ctx.Cleanup()
	uid := ctx.NormalUser.Id
	registerV1TokenRoutes(ctx, uid)

	body := map[string]interface{}{
		"name":            "new-token",
		"remain_quota":    1000,
		"unlimited_quota": false,
		"expired_time":    -1,
	}
	w := V2Request(ctx.Router, http.MethodPost, "/api/token/", body, nil)
	AssertV2Status(t, w, http.StatusOK)
	resp := AssertV2Success(t, w)
	if resp["success"] != true {
		t.Fatalf("expected success, body: %s", w.Body.String())
	}
	// DB side-effect: exactly one token now exists for this user with a generated key.
	var count int64
	ctx.DB.Model(&repo.Token{}).Where("user_id = ? AND name = ?", uid, "new-token").Count(&count)
	if count != 1 {
		t.Errorf("expected 1 persisted token, got %d", count)
	}
}

func TestAddToken_V1_InvalidName(t *testing.T) {
	ctx := SetupV2TestRouter(t)
	defer ctx.Cleanup()
	uid := ctx.NormalUser.Id
	registerV1TokenRoutes(ctx, uid)

	// A name exceeding the validator's limit should be rejected without insert.
	longName := ""
	for i := 0; i < 100; i++ {
		longName += "x"
	}
	body := map[string]interface{}{"name": longName, "remain_quota": 100, "expired_time": -1}
	w := V2Request(ctx.Router, http.MethodPost, "/api/token/", body, nil)
	resp := ParseV2Response(t, w)
	if success, _ := resp["success"].(bool); success {
		t.Errorf("expected validation failure for over-long name, body: %s", w.Body.String())
	}
}

func TestUpdateToken_V1_StatusOnly(t *testing.T) {
	ctx := SetupV2TestRouter(t)
	defer ctx.Cleanup()
	uid := ctx.NormalUser.Id
	registerV1TokenRoutes(ctx, uid)

	tok := SeedV2Token(t, ctx, uid, "to-disable")

	body := map[string]interface{}{
		"id":              tok.Id,
		"name":            tok.Name,
		"status":          common.TokenStatusDisabled,
		"unlimited_quota": true,
		"expired_time":    -1,
	}
	w := V2Request(ctx.Router, http.MethodPut, "/api/token/?status_only=true", body, nil)
	AssertV2Status(t, w, http.StatusOK)
	AssertV2Success(t, w)

	var reloaded repo.Token
	ctx.DB.First(&reloaded, tok.Id)
	if reloaded.Status != common.TokenStatusDisabled {
		t.Errorf("status = %d, want disabled(%d)", reloaded.Status, common.TokenStatusDisabled)
	}
}

func TestDeleteToken_V1(t *testing.T) {
	ctx := SetupV2TestRouter(t)
	defer ctx.Cleanup()
	uid := ctx.NormalUser.Id
	registerV1TokenRoutes(ctx, uid)

	tok := SeedV2Token(t, ctx, uid, "doomed")
	w := V2Request(ctx.Router, http.MethodDelete, fmt.Sprintf("/api/token/%d", tok.Id), nil, nil)
	AssertV2Status(t, w, http.StatusOK)
	AssertV2Success(t, w)

	var count int64
	ctx.DB.Model(&repo.Token{}).Where("id = ?", tok.Id).Count(&count)
	if count != 0 {
		t.Errorf("token not deleted, count=%d", count)
	}
}

func TestDeleteTokenBatch_V1(t *testing.T) {
	ctx := SetupV2TestRouter(t)
	defer ctx.Cleanup()
	uid := ctx.NormalUser.Id
	registerV1TokenRoutes(ctx, uid)

	a := SeedV2Token(t, ctx, uid, "batch-a")
	b := SeedV2Token(t, ctx, uid, "batch-b")

	w := V2Request(ctx.Router, http.MethodPost, "/api/token/batch",
		map[string]interface{}{"ids": []int{a.Id, b.Id}}, nil)
	AssertV2Status(t, w, http.StatusOK)
	resp := AssertV2Success(t, w)
	if int(resp["data"].(float64)) != 2 {
		t.Errorf("expected 2 deleted, got %v", resp["data"])
	}

	// Empty ids -> parameter error (success=false).
	w2 := V2Request(ctx.Router, http.MethodPost, "/api/token/batch",
		map[string]interface{}{"ids": []int{}}, nil)
	resp2 := ParseV2Response(t, w2)
	if success, _ := resp2["success"].(bool); success {
		t.Errorf("expected failure for empty batch, body: %s", w2.Body.String())
	}
}

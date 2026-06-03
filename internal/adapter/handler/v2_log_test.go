package handler

import (
	"net/http"
	"strconv"
	"testing"

	"github.com/LurusTech/lurus-hub/internal/adapter/repo"
)

// ============================================================================
// V2 Log Controller Tests
// ============================================================================

func TestGetLogsV2_UserScope(t *testing.T) {
	ctx := SetupV2TestRouter(t)
	defer ctx.Cleanup()

	// Create logs for normal user
	SeedV2Log(t, ctx, ctx.NormalUser.Id, repo.LogTypeConsume)
	SeedV2Log(t, ctx, ctx.NormalUser.Id, repo.LogTypeTopup)

	// Create logs for admin user
	SeedV2Log(t, ctx, ctx.AdminUser.Id, repo.LogTypeConsume)

	// Get logs as normal user - should only see their own
	w := V2RequestAsUser(ctx, ctx.NormalUser, http.MethodGet, "/api/v2/test-tenant/logs", nil, nil)

	AssertV2Status(t, w, http.StatusOK)
	resp := AssertV2Success(t, w)

	data := resp["data"].(map[string]interface{})
	total := int(data["total"].(float64))
	if total != 2 {
		t.Errorf("expected 2 logs for normal user, got %d", total)
	}
}

func TestGetLogsV2_Filters(t *testing.T) {
	ctx := SetupV2TestRouter(t)
	defer ctx.Cleanup()

	// Create logs with different types
	SeedV2Log(t, ctx, ctx.NormalUser.Id, repo.LogTypeConsume)
	SeedV2Log(t, ctx, ctx.NormalUser.Id, repo.LogTypeConsume)
	SeedV2Log(t, ctx, ctx.NormalUser.Id, repo.LogTypeTopup)

	// Filter by type
	w := V2RequestAsUser(ctx, ctx.NormalUser, http.MethodGet, "/api/v2/test-tenant/logs?type="+strconv.Itoa(repo.LogTypeConsume), nil, nil)

	AssertV2Status(t, w, http.StatusOK)
	resp := AssertV2Success(t, w)

	data := resp["data"].(map[string]interface{})
	total := int(data["total"].(float64))
	if total != 2 {
		t.Errorf("expected 2 consume logs, got %d", total)
	}
}

func TestGetLogsV2_Pagination(t *testing.T) {
	ctx := SetupV2TestRouter(t)
	defer ctx.Cleanup()

	// Create 25 logs
	for i := 0; i < 25; i++ {
		SeedV2Log(t, ctx, ctx.NormalUser.Id, repo.LogTypeConsume)
	}

	// Get first page
	w := V2RequestAsUser(ctx, ctx.NormalUser, http.MethodGet, "/api/v2/test-tenant/logs?page=1&page_size=10", nil, nil)
	resp := AssertV2Success(t, w)

	data := resp["data"].(map[string]interface{})
	logs := data["logs"].([]interface{})
	if len(logs) != 10 {
		t.Errorf("expected 10 logs on first page, got %d", len(logs))
	}

	total := int(data["total"].(float64))
	if total != 25 {
		t.Errorf("expected total=25, got %d", total)
	}

	// Get third page
	w = V2RequestAsUser(ctx, ctx.NormalUser, http.MethodGet, "/api/v2/test-tenant/logs?page=3&page_size=10", nil, nil)
	resp = AssertV2Success(t, w)

	data = resp["data"].(map[string]interface{})
	logs = data["logs"].([]interface{})
	if len(logs) != 5 {
		t.Errorf("expected 5 logs on third page, got %d", len(logs))
	}
}

func TestGetAllLogsV2_TenantScope(t *testing.T) {
	ctx := SetupV2TestRouter(t)
	defer ctx.Cleanup()

	// Create logs for both users in the same tenant
	SeedV2Log(t, ctx, ctx.NormalUser.Id, repo.LogTypeConsume)
	SeedV2Log(t, ctx, ctx.NormalUser.Id, repo.LogTypeTopup)
	SeedV2Log(t, ctx, ctx.AdminUser.Id, repo.LogTypeConsume)

	// Create a log in a different tenant (should not be included)
	otherTenantLog := &repo.Log{
		UserId:    ctx.NormalUser.Id,
		TenantId:  "other-tenant",
		Type:      repo.LogTypeConsume,
		Content:   "Other tenant log",
		CreatedAt: 0,
	}
	ctx.DB.Create(otherTenantLog)

	// Get all logs (admin endpoint, gets all logs in tenant)
	w := V2RequestAsUser(ctx, ctx.AdminUser, http.MethodGet, "/api/v2/test-tenant/logs/all", nil, []string{"admin"})

	AssertV2Status(t, w, http.StatusOK)
	resp := AssertV2Success(t, w)

	data := resp["data"].(map[string]interface{})
	total := int(data["total"].(float64))
	if total != 3 {
		t.Errorf("expected 3 logs in tenant, got %d", total)
	}
}

func TestGetLogsV2_DateRangeFilter(t *testing.T) {
	ctx := SetupV2TestRouter(t)
	defer ctx.Cleanup()

	// Create logs with different timestamps
	now := int64(1700000000) // Fixed timestamp for testing

	log1 := &repo.Log{
		UserId:    ctx.NormalUser.Id,
		TenantId:  ctx.TenantID,
		Type:      repo.LogTypeConsume,
		Content:   "Old log",
		CreatedAt: now - 86400*2, // 2 days ago
	}
	log2 := &repo.Log{
		UserId:    ctx.NormalUser.Id,
		TenantId:  ctx.TenantID,
		Type:      repo.LogTypeConsume,
		Content:   "Recent log",
		CreatedAt: now - 3600, // 1 hour ago
	}
	log3 := &repo.Log{
		UserId:    ctx.NormalUser.Id,
		TenantId:  ctx.TenantID,
		Type:      repo.LogTypeConsume,
		Content:   "Very old log",
		CreatedAt: now - 86400*10, // 10 days ago
	}
	ctx.DB.Create(log1)
	ctx.DB.Create(log2)
	ctx.DB.Create(log3)

	// Filter by date range (last 3 days)
	startTime := now - 86400*3
	endTime := now

	path := "/api/v2/test-tenant/logs?start_time=" + strconv.FormatInt(startTime, 10) + "&end_time=" + strconv.FormatInt(endTime, 10)
	w := V2RequestAsUser(ctx, ctx.NormalUser, http.MethodGet, path, nil, nil)

	AssertV2Status(t, w, http.StatusOK)
	resp := AssertV2Success(t, w)

	data := resp["data"].(map[string]interface{})
	total := int(data["total"].(float64))
	if total != 2 {
		t.Errorf("expected 2 logs in date range, got %d", total)
	}
}

func TestGetLogsV2_EmptyResult(t *testing.T) {
	ctx := SetupV2TestRouter(t)
	defer ctx.Cleanup()

	// Don't create any logs

	w := V2RequestAsUser(ctx, ctx.NormalUser, http.MethodGet, "/api/v2/test-tenant/logs", nil, nil)

	AssertV2Status(t, w, http.StatusOK)
	resp := AssertV2Success(t, w)

	data := resp["data"].(map[string]interface{})
	total := int(data["total"].(float64))
	if total != 0 {
		t.Errorf("expected 0 logs, got %d", total)
	}

	logs := data["logs"]
	if logs == nil {
		// Some implementations return nil for empty array
		return
	}
	if logsArr, ok := logs.([]interface{}); ok && len(logsArr) != 0 {
		t.Errorf("expected empty logs array, got %d items", len(logsArr))
	}
}

func TestGetAllLogsV2_WithFilters(t *testing.T) {
	ctx := SetupV2TestRouter(t)
	defer ctx.Cleanup()

	// Create logs with different types and users
	SeedV2Log(t, ctx, ctx.NormalUser.Id, repo.LogTypeConsume)
	SeedV2Log(t, ctx, ctx.NormalUser.Id, repo.LogTypeTopup)
	SeedV2Log(t, ctx, ctx.AdminUser.Id, repo.LogTypeConsume)

	// Filter by type only
	w := V2RequestAsUser(ctx, ctx.AdminUser, http.MethodGet, "/api/v2/test-tenant/logs/all?type="+strconv.Itoa(repo.LogTypeConsume), nil, []string{"admin"})

	AssertV2Status(t, w, http.StatusOK)
	resp := AssertV2Success(t, w)

	data := resp["data"].(map[string]interface{})
	total := int(data["total"].(float64))
	if total != 2 {
		t.Errorf("expected 2 consume logs, got %d", total)
	}
}

func TestGetAllLogsV2_NonAdminRejected(t *testing.T) {
	ctx := SetupV2TestRouter(t)
	defer ctx.Cleanup()

	SeedV2Log(t, ctx, ctx.NormalUser.Id, repo.LogTypeConsume)

	// /logs/all returns every tenant member's logs (GetTenantLogsWithParams, no
	// user_id filter), so a non-admin tenant member must be rejected. The route is
	// mounted under UserAuth(); the admin gate is enforced in the handler via
	// requireTenantAdmin (see TestSecurityLogsAllRequiresAdmin for full coverage).
	w := V2RequestAsUser(ctx, ctx.NormalUser, http.MethodGet, "/api/v2/test-tenant/logs/all", nil, nil)

	AssertV2Status(t, w, http.StatusForbidden)
}

// TestGetLogsV2_ForbiddenFields guards the logView whitelist: the log endpoints
// must not expose tenant_id, caller IP (PII), the raw `other` payload, or the
// governance-internal fingerprint/upstream-model columns.
func TestGetLogsV2_ForbiddenFields(t *testing.T) {
	ctx := SetupV2TestRouter(t)
	defer ctx.Cleanup()

	sensitive := &repo.Log{
		UserId:             ctx.NormalUser.Id,
		TenantId:           ctx.TenantID,
		Type:               repo.LogTypeConsume,
		Content:            "log with secrets",
		ModelName:          "gpt-4",
		CreatedAt:          1700000000,
		Ip:                 "203.0.113.7",
		Other:              `{"secret":"payload"}`,
		RequestFingerprint: "fp-abc123",
		UpstreamModel:      "gpt-4-internal",
	}
	if err := ctx.DB.Create(sensitive).Error; err != nil {
		t.Fatalf("failed to seed sensitive log: %v", err)
	}

	w := V2RequestAsUser(ctx, ctx.NormalUser, http.MethodGet, "/api/v2/test-tenant/logs", nil, nil)
	resp := AssertV2Success(t, w)
	data := resp["data"].(map[string]interface{})
	logs := data["logs"].([]interface{})
	if len(logs) != 1 {
		t.Fatalf("expected 1 log, got %d", len(logs))
	}
	lg := logs[0].(map[string]interface{})

	for _, f := range []string{"tenant_id", "ip", "other", "request_fingerprint", "upstream_model", "channel_type", "relay_mode"} {
		if _, exists := lg[f]; exists {
			t.Errorf("forbidden field %q leaked through logView", f)
		}
	}
	if _, ok := lg["model_name"]; !ok {
		t.Error("expected model_name field in logView")
	}
}

func TestGetLogsV2_TypeFilter(t *testing.T) {
	ctx := SetupV2TestRouter(t)
	defer ctx.Cleanup()

	// Create logs with different types for the same user
	SeedV2Log(t, ctx, ctx.NormalUser.Id, repo.LogTypeConsume)
	SeedV2Log(t, ctx, ctx.NormalUser.Id, repo.LogTypeConsume)
	SeedV2Log(t, ctx, ctx.NormalUser.Id, repo.LogTypeConsume)
	SeedV2Log(t, ctx, ctx.NormalUser.Id, repo.LogTypeTopup)

	// Filter by topup type on the user's own logs endpoint
	w := V2RequestAsUser(ctx, ctx.NormalUser, http.MethodGet, "/api/v2/test-tenant/logs?type="+strconv.Itoa(repo.LogTypeTopup), nil, nil)

	AssertV2Status(t, w, http.StatusOK)
	resp := AssertV2Success(t, w)

	data := resp["data"].(map[string]interface{})
	total := int(data["total"].(float64))
	if total != 1 {
		t.Errorf("expected 1 topup log, got %d", total)
	}
}

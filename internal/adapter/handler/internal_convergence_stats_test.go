package handler

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/LurusTech/lurus-hub/internal/adapter/middleware"
	"github.com/LurusTech/lurus-hub/internal/adapter/repo"
	"github.com/LurusTech/lurus-hub/internal/pkg/common"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// setupConvergenceRouter extends SetupIntegrationRouter with the
// convergence-stats endpoint so the test can call it through the same
// auth middleware stack.
func setupConvergenceRouter(t *testing.T) (*gin.Engine, func()) {
	t.Helper()
	router, cleanup := SetupIntegrationRouter(t)

	// Attach the new endpoint on the same /internal group that
	// SetupIntegrationRouter already wired with InternalApiAuth().
	// We add a sub-group with admin scope to mirror the production router.
	admin := router.Group("/internal/admin")
	admin.Use(middleware.InternalApiAuth())
	admin.Use(middleware.RequireScope(repo.ScopeAdmin))
	admin.GET("/convergence-stats", InternalConvergenceStats)

	return router, cleanup
}

// convergenceAuthHeaders returns X-API-Key header using the all-scopes test key.
func convergenceAuthHeaders() map[string]string {
	return map[string]string{
		"X-API-Key": testApiKeyAllScopes,
	}
}

// seedToken creates a Token directly in the test DB and fails the test on error.
func seedToken(t *testing.T, tok *repo.Token) {
	t.Helper()
	if err := repo.DB.Create(tok).Error; err != nil {
		t.Fatalf("seed token: %v", err)
	}
}

// TestConvergenceStats_EmptyDB verifies all-zero counts on a fresh DB (only
// the two seed users from SetupIntegrationRouter, zero tokens).
func TestConvergenceStats_EmptyDB(t *testing.T) {
	router, cleanup := setupConvergenceRouter(t)
	t.Cleanup(cleanup)

	w := internalRequest(router, "GET", "/internal/admin/convergence-stats", nil, convergenceAuthHeaders())
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body: %s", w.Code, w.Body.String())
	}

	var got struct {
		UsersTotal      int64 `json:"users_total"`
		UsersLinked     int64 `json:"users_linked"`
		TokensTotal     int64 `json:"tokens_total"`
		TokensLinked    int64 `json:"tokens_linked"`
		TokensUnlimited int64 `json:"tokens_unlimited"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// SetupIntegrationRouter seeds 2 users (root + testuser), neither linked.
	if got.UsersTotal != 2 {
		t.Errorf("users_total: want 2, got %d", got.UsersTotal)
	}
	if got.UsersLinked != 0 {
		t.Errorf("users_linked: want 0, got %d", got.UsersLinked)
	}
	if got.TokensTotal != 0 {
		t.Errorf("tokens_total: want 0, got %d", got.TokensTotal)
	}
	if got.TokensLinked != 0 {
		t.Errorf("tokens_linked: want 0, got %d", got.TokensLinked)
	}
	if got.TokensUnlimited != 0 {
		t.Errorf("tokens_unlimited: want 0, got %d", got.TokensUnlimited)
	}
}

// TestConvergenceStats_MixedData seeds a realistic mix of users and tokens
// and asserts the exact counts the contract specifies.
func TestConvergenceStats_MixedData(t *testing.T) {
	router, cleanup := setupConvergenceRouter(t)
	t.Cleanup(cleanup)

	// --- Users ---------------------------------------------------------
	// Linked user (lurus_account_id set).
	linked := &repo.User{
		Username: "linked_user", TenantId: "default", Email: "linked@test.local",
		Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: "default",
	}
	acctID := int64(5001)
	linked.LurusAccountID = &acctID
	if err := repo.DB.Create(linked).Error; err != nil {
		t.Fatalf("seed linked user: %v", err)
	}

	// Unlinked user (no lurus_account_id).
	unlinked := &repo.User{
		Username: "unlinked_user", TenantId: "default", Email: "unlinked@test.local",
		Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: "default",
	}
	if err := repo.DB.Create(unlinked).Error; err != nil {
		t.Fatalf("seed unlinked user: %v", err)
	}

	// Soft-deleted user — must NOT appear in any count.
	deleted := &repo.User{
		Username: "deleted_user", TenantId: "default", Email: "deleted@test.local",
		Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: "default",
	}
	if err := repo.DB.Create(deleted).Error; err != nil {
		t.Fatalf("seed deleted user: %v", err)
	}
	if err := repo.DB.Delete(deleted).Error; err != nil {
		t.Fatalf("soft-delete user: %v", err)
	}

	// --- Tokens --------------------------------------------------------
	// Token: linked + unlimited (identity_account_id > 0, unlimited_quota = true).
	seedToken(t, &repo.Token{
		UserId: linked.Id, TenantId: "default", Name: "tok-linked-unlimited",
		Key: "cs-tok-linked-unlim-0000000000000000000",
		Status: common.TokenStatusEnabled, ExpiredTime: -1,
		IdentityAccountID: 5001, UnlimitedQuota: true,
	})

	// Token: linked + capped (identity_account_id > 0, unlimited_quota = false).
	seedToken(t, &repo.Token{
		UserId: linked.Id, TenantId: "default", Name: "tok-linked-capped",
		Key: "cs-tok-linked-capped-000000000000000000",
		Status: common.TokenStatusEnabled, ExpiredTime: -1,
		IdentityAccountID: 5001, UnlimitedQuota: false, RemainQuota: 1000,
	})

	// Token: unlinked + unlimited (identity_account_id = 0, unlimited_quota = true).
	seedToken(t, &repo.Token{
		UserId: unlinked.Id, TenantId: "default", Name: "tok-unlinked-unlimited",
		Key: "cs-tok-unlinked-unlim-000000000000000000",
		Status: common.TokenStatusEnabled, ExpiredTime: -1,
		IdentityAccountID: 0, UnlimitedQuota: true,
	})

	// Token: unlinked + capped (neither linked nor unlimited).
	seedToken(t, &repo.Token{
		UserId: unlinked.Id, TenantId: "default", Name: "tok-unlinked-capped",
		Key: "cs-tok-unlinked-capped-00000000000000000",
		Status: common.TokenStatusEnabled, ExpiredTime: -1,
		IdentityAccountID: 0, UnlimitedQuota: false, RemainQuota: 500,
	})

	// Token: pre-inserted as soft-deleted — must NOT appear in any count.
	// We set DeletedAt at insert time so the test does not depend on whether
	// GORM's Delete call propagates back to the same *gorm.DB session.
	now := time.Now()
	if err := repo.DB.Create(&repo.Token{
		UserId: unlinked.Id, TenantId: "default", Name: "tok-deleted",
		Key:               "cs-tok-deleted-000000000000000000000000",
		Status:            common.TokenStatusEnabled, ExpiredTime: -1,
		IdentityAccountID: 0, UnlimitedQuota: true,
		DeletedAt:         gorm.DeletedAt{Time: now, Valid: true},
	}).Error; err != nil {
		t.Fatalf("seed soft-deleted token: %v", err)
	}

	// --- Request -------------------------------------------------------
	w := internalRequest(router, "GET", "/internal/admin/convergence-stats", nil, convergenceAuthHeaders())
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body: %s", w.Code, w.Body.String())
	}

	var got struct {
		UsersTotal      int64 `json:"users_total"`
		UsersLinked     int64 `json:"users_linked"`
		TokensTotal     int64 `json:"tokens_total"`
		TokensLinked    int64 `json:"tokens_linked"`
		TokensUnlimited int64 `json:"tokens_unlimited"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// Seed: 2 from SetupIntegrationRouter + 1 linked + 1 unlinked = 4 (deleted excluded).
	wantUsersTotal := int64(4)
	// Only the linked user carries lurus_account_id.
	wantUsersLinked := int64(1)
	// 5 tokens seeded (4 live + 1 soft-deleted) ⇒ 4 visible.
	wantTokensTotal := int64(4)
	// 2 tokens have identity_account_id = 5001 > 0, but only the non-deleted ones.
	wantTokensLinked := int64(2)
	// 2 tokens are unlimited (tok-linked-unlimited + tok-unlinked-unlimited).
	wantTokensUnlimited := int64(2)

	if got.UsersTotal != wantUsersTotal {
		t.Errorf("users_total: want %d, got %d", wantUsersTotal, got.UsersTotal)
	}
	if got.UsersLinked != wantUsersLinked {
		t.Errorf("users_linked: want %d, got %d", wantUsersLinked, got.UsersLinked)
	}
	if got.TokensTotal != wantTokensTotal {
		t.Errorf("tokens_total: want %d, got %d", wantTokensTotal, got.TokensTotal)
	}
	if got.TokensLinked != wantTokensLinked {
		t.Errorf("tokens_linked: want %d, got %d", wantTokensLinked, got.TokensLinked)
	}
	if got.TokensUnlimited != wantTokensUnlimited {
		t.Errorf("tokens_unlimited: want %d, got %d", wantTokensUnlimited, got.TokensUnlimited)
	}
}

// TestConvergenceStats_RequiresAuth verifies the endpoint rejects unauthenticated requests.
func TestConvergenceStats_RequiresAuth(t *testing.T) {
	router, cleanup := setupConvergenceRouter(t)
	t.Cleanup(cleanup)

	w := internalRequest(router, "GET", "/internal/admin/convergence-stats", nil, nil)
	if w.Code == http.StatusOK {
		t.Errorf("unauthenticated request must not return 200, got %d", w.Code)
	}
}

// TestConvergenceStats_JsonShape verifies the response includes all five
// required keys and no stray fields.
func TestConvergenceStats_JsonShape(t *testing.T) {
	router, cleanup := setupConvergenceRouter(t)
	t.Cleanup(cleanup)

	w := internalRequest(router, "GET", "/internal/admin/convergence-stats", nil, convergenceAuthHeaders())
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(w.Body.Bytes(), &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	required := []string{"users_total", "users_linked", "tokens_total", "tokens_linked", "tokens_unlimited"}
	for _, key := range required {
		if _, ok := raw[key]; !ok {
			t.Errorf("response missing required field %q", key)
		}
	}
	if len(raw) != len(required) {
		t.Errorf("response has %d fields, want exactly %d; keys: %v", len(raw), len(required), raw)
	}
}

// TestConvergenceStats_SoftDeletedTokenExcluded is a focused regression guard:
// a token whose deleted_at is set must not appear in any count bucket.
func TestConvergenceStats_SoftDeletedTokenExcluded(t *testing.T) {
	router, cleanup := setupConvergenceRouter(t)
	t.Cleanup(cleanup)

	// Seed one non-deleted token and one soft-deleted token, both linked + unlimited.
	now := time.Now()

	seedToken(t, &repo.Token{
		UserId: 1, TenantId: "default", Name: "live",
		Key:               "cs-soft-live-000000000000000000000000000",
		Status:            common.TokenStatusEnabled, ExpiredTime: -1,
		IdentityAccountID: 100, UnlimitedQuota: true,
	})

	soft := &repo.Token{
		UserId: 1, TenantId: "default", Name: "dead",
		Key:               "cs-soft-dead-000000000000000000000000000",
		Status:            common.TokenStatusEnabled, ExpiredTime: -1,
		IdentityAccountID: 100, UnlimitedQuota: true,
		DeletedAt: gorm.DeletedAt{Time: now, Valid: true},
	}
	if err := repo.DB.Create(soft).Error; err != nil {
		t.Fatalf("seed soft-deleted token: %v", err)
	}

	w := internalRequest(router, "GET", "/internal/admin/convergence-stats", nil, convergenceAuthHeaders())
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var got struct {
		TokensTotal     int64 `json:"tokens_total"`
		TokensLinked    int64 `json:"tokens_linked"`
		TokensUnlimited int64 `json:"tokens_unlimited"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.TokensTotal != 1 {
		t.Errorf("tokens_total: want 1 (soft-deleted excluded), got %d", got.TokensTotal)
	}
	if got.TokensLinked != 1 {
		t.Errorf("tokens_linked: want 1, got %d", got.TokensLinked)
	}
	if got.TokensUnlimited != 1 {
		t.Errorf("tokens_unlimited: want 1, got %d", got.TokensUnlimited)
	}
}

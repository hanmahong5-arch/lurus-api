package handler

import (
	"net/http"
	"testing"

	"github.com/LurusTech/lurus-hub/internal/adapter/repo"
)

// ============================================================================
// V2 Token Batch-Delete Controller Tests
// ============================================================================

// TestDeleteTokensV2_DeletesOwnedOnly proves the batch endpoint deletes only the
// caller's tokens, returns the deleted count, and silently ignores ids that
// belong to another user (ownership is enforced inside repo.BatchDeleteTokens).
func TestDeleteTokensV2_DeletesOwnedOnly(t *testing.T) {
	ctx := SetupV2TestRouter(t)
	defer ctx.Cleanup()

	own1 := SeedV2Token(t, ctx, ctx.NormalUser.Id, "own-1")
	own2 := SeedV2Token(t, ctx, ctx.NormalUser.Id, "own-2")
	other := SeedV2Token(t, ctx, ctx.AdminUser.Id, "other-user")

	body := map[string]interface{}{
		"ids": []int{own1.Id, own2.Id, other.Id},
	}
	w := V2RequestAsUser(ctx, ctx.NormalUser, http.MethodPost, "/api/v2/test-tenant/tokens/batch-delete", body, nil)
	AssertV2Status(t, w, http.StatusOK)
	resp := AssertV2Success(t, w)

	// Only the caller's two tokens are deleted; the admin's token is left alone.
	if deleted := int(resp["deleted"].(float64)); deleted != 2 {
		t.Errorf("expected deleted=2, got %d", deleted)
	}

	// The caller now has zero tokens.
	w = V2RequestAsUser(ctx, ctx.NormalUser, http.MethodGet, "/api/v2/test-tenant/tokens", nil, nil)
	data := AssertV2Success(t, w)["data"].(map[string]interface{})
	if total := int(data["total"].(float64)); total != 0 {
		t.Errorf("expected 0 tokens for caller after batch delete, got %d", total)
	}

	// The other user's token survives untouched.
	if _, err := repo.GetTokenByIds(other.Id, ctx.AdminUser.Id); err != nil {
		t.Errorf("admin's token was deleted by another user's batch request: %v", err)
	}
}

// TestDeleteTokensV2_EmptyListIsNoOp confirms an empty id list succeeds as a
// no-op (deleted:0) rather than erroring — the UI may submit an empty selection.
func TestDeleteTokensV2_EmptyListIsNoOp(t *testing.T) {
	ctx := SetupV2TestRouter(t)
	defer ctx.Cleanup()

	SeedV2Token(t, ctx, ctx.NormalUser.Id, "keep-me")

	body := map[string]interface{}{"ids": []int{}}
	w := V2RequestAsUser(ctx, ctx.NormalUser, http.MethodPost, "/api/v2/test-tenant/tokens/batch-delete", body, nil)
	AssertV2Status(t, w, http.StatusOK)
	resp := AssertV2Success(t, w)
	if deleted := int(resp["deleted"].(float64)); deleted != 0 {
		t.Errorf("expected deleted=0 for empty list, got %d", deleted)
	}

	// The existing token is untouched.
	w = V2RequestAsUser(ctx, ctx.NormalUser, http.MethodGet, "/api/v2/test-tenant/tokens", nil, nil)
	data := AssertV2Success(t, w)["data"].(map[string]interface{})
	if total := int(data["total"].(float64)); total != 1 {
		t.Errorf("expected 1 token to remain, got %d", total)
	}
}

// TestDeleteTokensV2_RejectsOverCap rejects a batch larger than the bounded cap
// so a runaway/abusive request can't sweep an unbounded set in one transaction.
func TestDeleteTokensV2_RejectsOverCap(t *testing.T) {
	ctx := SetupV2TestRouter(t)
	defer ctx.Cleanup()

	ids := make([]int, maxBatchDeleteTokens+1)
	for i := range ids {
		ids[i] = i + 1
	}

	body := map[string]interface{}{"ids": ids}
	w := V2RequestAsUser(ctx, ctx.NormalUser, http.MethodPost, "/api/v2/test-tenant/tokens/batch-delete", body, nil)
	AssertV2Error(t, w, http.StatusBadRequest)
}

package handler

import (
	"strconv"
	"testing"

	"github.com/LurusTech/lurus-hub/internal/adapter/repo"
	"github.com/LurusTech/lurus-hub/internal/pkg/common"
	"github.com/gin-gonic/gin"
)

// setupUpdateUserRouter wires the v1 admin UpdateUser handler behind a minimal
// auth shim that mirrors session middleware: it resolves the acting user from
// the X-Test-User-ID header and sets the "id"/"role" context keys.
func setupUpdateUserRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.PUT("/api/user/", func(c *gin.Context) {
		actorID, _ := strconv.Atoi(c.GetHeader("X-Test-User-ID"))
		c.Set("id", actorID)
		if actor, err := repo.GetUserById(actorID, false); err == nil {
			c.Set("role", actor.Role)
		}
		c.Next()
	}, UpdateUser)
	return router
}

// TestUpdateUser_SelfDemotionGuard covers the lockout-prevention rule: an
// actor must not be able to lower their own role via PUT /api/user/, while
// same-role self updates and edits to other users keep working.
func TestUpdateUser_SelfDemotionGuard(t *testing.T) {
	tests := []struct {
		name        string
		actor       func(ctx *V2TestContext) *repo.User
		target      func(ctx *V2TestContext) *repo.User
		bodyRole    int // 0 = omitted from payload
		wantSuccess bool
		wantMessage string
		wantRole    int // expected target role in DB after the call
	}{
		{
			name:        "root self-demotion rejected and DB unchanged",
			actor:       func(ctx *V2TestContext) *repo.User { return ctx.RootUser },
			target:      func(ctx *V2TestContext) *repo.User { return ctx.RootUser },
			bodyRole:    common.RoleCommonUser,
			wantSuccess: false,
			wantMessage: "不能降低自己的权限等级",
			wantRole:    common.RoleRootUser,
		},
		{
			name:        "root self update with same role allowed",
			actor:       func(ctx *V2TestContext) *repo.User { return ctx.RootUser },
			target:      func(ctx *V2TestContext) *repo.User { return ctx.RootUser },
			bodyRole:    common.RoleRootUser,
			wantSuccess: true,
			wantRole:    common.RoleRootUser,
		},
		{
			name:        "root self update with role omitted keeps role",
			actor:       func(ctx *V2TestContext) *repo.User { return ctx.RootUser },
			target:      func(ctx *V2TestContext) *repo.User { return ctx.RootUser },
			bodyRole:    0,
			wantSuccess: true,
			wantRole:    common.RoleRootUser,
		},
		{
			name:        "root promoting another user's role still persists",
			actor:       func(ctx *V2TestContext) *repo.User { return ctx.RootUser },
			target:      func(ctx *V2TestContext) *repo.User { return ctx.NormalUser },
			bodyRole:    common.RoleAdminUser,
			wantSuccess: true,
			wantRole:    common.RoleAdminUser,
		},
		{
			name:        "root lowering another user's role still persists",
			actor:       func(ctx *V2TestContext) *repo.User { return ctx.RootUser },
			target:      func(ctx *V2TestContext) *repo.User { return ctx.AdminUser },
			bodyRole:    common.RoleCommonUser,
			wantSuccess: true,
			wantRole:    common.RoleCommonUser,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := SetupV2TestRouter(t)
			defer ctx.Cleanup()
			router := setupUpdateUserRouter()

			actor := tt.actor(ctx)
			target := tt.target(ctx)
			body := map[string]interface{}{
				"id":           target.Id,
				"username":     target.Username,
				"display_name": target.DisplayName,
				"group":        target.Group,
				"quota":        target.Quota,
			}
			if tt.bodyRole != 0 {
				body["role"] = tt.bodyRole
			}

			w := V2Request(router, "PUT", "/api/user/", body, map[string]string{
				"X-Test-User-ID": strconv.Itoa(actor.Id),
			})
			if w.Code != 200 {
				t.Fatalf("expected status 200, got %d, body: %s", w.Code, w.Body.String())
			}
			resp := ParseV2Response(t, w)
			if success, _ := resp["success"].(bool); success != tt.wantSuccess {
				t.Errorf("expected success=%v, got %v, body: %s", tt.wantSuccess, resp["success"], w.Body.String())
			}
			if tt.wantMessage != "" {
				if msg, _ := resp["message"].(string); msg != tt.wantMessage {
					t.Errorf("expected message %q, got %q", tt.wantMessage, msg)
				}
			}

			var after repo.User
			if err := ctx.DB.First(&after, target.Id).Error; err != nil {
				t.Fatalf("failed to reload target user: %v", err)
			}
			if after.Role != tt.wantRole {
				t.Errorf("expected target role %d in DB, got %d", tt.wantRole, after.Role)
			}
		})
	}
}

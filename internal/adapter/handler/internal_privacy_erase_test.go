package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/LurusTech/lurus-hub/internal/adapter/middleware"
	"github.com/LurusTech/lurus-hub/internal/adapter/repo"
	"github.com/LurusTech/lurus-hub/internal/domain/entity"
	"github.com/LurusTech/lurus-hub/internal/pkg/common"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// setupPrivacyEraseRouter mirrors setupFundRouter: isolated in-memory DB,
// all-scopes + read-only API keys, only the privacy routes mounted.
func setupPrivacyEraseRouter(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	dbName := "file:privacyerase_" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	for _, tbl := range []interface{}{
		&repo.User{}, &repo.Token{}, &repo.InternalApiKey{},
		&entity.UserIdentityMapping{}, &entity.AuditEvent{},
		&entity.PrivacyErasureRequest{},
	} {
		if migrateErr := db.AutoMigrate(tbl); migrateErr != nil &&
			!strings.Contains(migrateErr.Error(), "already exists") {
			t.Fatalf("AutoMigrate %T: %v", tbl, migrateErr)
		}
	}

	prevDB, prevLogDB := repo.DB, repo.LOG_DB
	prevSQLite, prevPG, prevRedis := common.UsingSQLite, common.UsingPostgreSQL, common.RedisEnabled
	repo.DB, repo.LOG_DB = db, db
	repo.InitCol()
	common.UsingSQLite, common.UsingPostgreSQL, common.RedisEnabled = true, false, false

	allScopes, _ := json.Marshal([]string{repo.ScopeAll})
	db.Create(&repo.InternalApiKey{
		Id: 20, Name: "privacy-all", KeyHash: hashTestKey(testApiKeyAllScopes),
		Scopes: string(allScopes), Enabled: true,
	})
	readScopes, _ := json.Marshal([]string{repo.ScopeBalanceRead})
	db.Create(&repo.InternalApiKey{
		Id: 21, Name: "privacy-readonly", KeyHash: hashTestKey(testApiKeyReadOnly),
		Scopes: string(readScopes), Enabled: true,
	})

	router := gin.New()
	internalGroup := router.Group("/internal")
	internalGroup.Use(middleware.InternalApiAuth())
	eraseGroup := internalGroup.Group("/v1/privacy")
	eraseGroup.Use(middleware.RequireScope(repo.ScopeUserDelete))
	eraseGroup.POST("/erase", InternalPrivacyErase)
	readGroup := internalGroup.Group("/v1/privacy")
	readGroup.Use(middleware.RequireScope(repo.ScopeUserRead))
	readGroup.GET("/erase/:event_id", InternalGetPrivacyErasure)

	t.Cleanup(func() {
		repo.DB, repo.LOG_DB = prevDB, prevLogDB
		common.UsingSQLite, common.UsingPostgreSQL, common.RedisEnabled = prevSQLite, prevPG, prevRedis
		if sqlDB, _ := db.DB(); sqlDB != nil {
			_ = sqlDB.Close()
		}
	})
	return router, db
}

func seedBoundUser(t *testing.T, db *gorm.DB, accountID int64) *repo.User {
	t.Helper()
	user := &repo.User{
		Username: fmt.Sprintf("bound-%d", accountID), Email: "bound@example.com",
		Status: common.UserStatusEnabled, Group: "default", LurusAccountID: &accountID,
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	for i := 0; i < 2; i++ {
		if err := db.Create(&repo.Token{
			UserId: user.Id, Key: fmt.Sprintf("erasehk%d%024d", accountID, i),
			Status: common.TokenStatusEnabled, Name: fmt.Sprintf("t%d", i),
		}).Error; err != nil {
			t.Fatalf("seed token: %v", err)
		}
	}
	return user
}

func TestInternalPrivacyErase(t *testing.T) {
	router, db := setupPrivacyEraseRouter(t)

	post := func(body map[string]interface{}, key string) (int, map[string]interface{}) {
		w := internalRequest(router, "POST", "/internal/v1/privacy/erase", body,
			map[string]string{"X-API-Key": key})
		return w.Code, parseResponse(t, w)
	}

	t.Run("missing_event_id_400", func(t *testing.T) {
		code, _ := post(map[string]interface{}{"account_id": 1}, testApiKeyAllScopes)
		if code != http.StatusBadRequest {
			t.Errorf("code = %d, want 400", code)
		}
	})

	t.Run("invalid_account_id_400", func(t *testing.T) {
		code, _ := post(map[string]interface{}{"event_id": "evt-x", "account_id": 0}, testApiKeyAllScopes)
		if code != http.StatusBadRequest {
			t.Errorf("code = %d, want 400", code)
		}
	})

	t.Run("scope_missing_403", func(t *testing.T) {
		code, _ := post(map[string]interface{}{"event_id": "evt-403", "account_id": 7}, testApiKeyReadOnly)
		if code != http.StatusForbidden {
			t.Errorf("code = %d, want 403 (key lacks user:delete)", code)
		}
	})

	t.Run("unknown_account_no_data_200", func(t *testing.T) {
		code, resp := post(map[string]interface{}{"event_id": "evt-nodata", "account_id": 999}, testApiKeyAllScopes)
		if code != http.StatusOK {
			t.Fatalf("code = %d, want 200, body=%s", code, mustJSON(resp))
		}
		data := resp["data"].(map[string]interface{})
		if data["status"] != repo.ErasureStatusNoData || data["replayed"] != false {
			t.Errorf("data = %v, want status=no_data replayed=false", data)
		}

		// Replay: same event_id → 200 replayed=true, still exactly one row.
		code2, resp2 := post(map[string]interface{}{"event_id": "evt-nodata", "account_id": 999}, testApiKeyAllScopes)
		if code2 != http.StatusOK {
			t.Fatalf("replay code = %d, want 200", code2)
		}
		if resp2["data"].(map[string]interface{})["replayed"] != true {
			t.Errorf("replay must set replayed=true")
		}
		var n int64
		db.Model(&entity.PrivacyErasureRequest{}).Where("event_id = ?", "evt-nodata").Count(&n)
		if n != 1 {
			t.Errorf("rows for evt-nodata = %d, want 1", n)
		}
	})

	t.Run("bound_user_202_and_synchronous_containment", func(t *testing.T) {
		user := seedBoundUser(t, db, 4242)

		code, resp := post(map[string]interface{}{
			"event_id": "evt-bound", "account_id": 4242, "reason": "user_requested",
		}, testApiKeyAllScopes)
		if code != http.StatusAccepted {
			t.Fatalf("code = %d, want 202, body=%s", code, mustJSON(resp))
		}
		data := resp["data"].(map[string]interface{})
		if data["status"] != repo.ErasureStatusPending {
			t.Errorf("status = %v, want pending", data["status"])
		}

		// Containment: user + all tokens disabled immediately.
		var u repo.User
		db.First(&u, user.Id)
		if u.Status != common.UserStatusDisabled {
			t.Errorf("user status = %d, want disabled", u.Status)
		}
		var enabled int64
		db.Model(&repo.Token{}).Where("user_id = ? AND status = ?", user.Id, common.TokenStatusEnabled).Count(&enabled)
		if enabled != 0 {
			t.Errorf("enabled tokens after intake = %d, want 0", enabled)
		}

		// Replay → 200 (not 202), replayed=true, single row.
		code2, resp2 := post(map[string]interface{}{"event_id": "evt-bound", "account_id": 4242}, testApiKeyAllScopes)
		if code2 != http.StatusOK {
			t.Fatalf("replay code = %d, want 200", code2)
		}
		if resp2["data"].(map[string]interface{})["replayed"] != true {
			t.Errorf("replay must set replayed=true")
		}
	})

	t.Run("idempotent_under_concurrent_replay", func(t *testing.T) {
		seedBoundUser(t, db, 5151)

		const workers = 6
		codes := make([]int, workers)
		var wg sync.WaitGroup
		for i := 0; i < workers; i++ {
			wg.Add(1)
			go func(n int) {
				defer wg.Done()
				w := internalRequest(router, "POST", "/internal/v1/privacy/erase",
					map[string]interface{}{"event_id": "evt-race", "account_id": 5151},
					map[string]string{"X-API-Key": testApiKeyAllScopes})
				codes[n] = w.Code
			}(i)
		}
		wg.Wait()

		for i, c := range codes {
			if c != http.StatusAccepted && c != http.StatusOK {
				t.Errorf("worker %d code = %d, want 200 or 202", i, c)
			}
		}
		var n int64
		db.Model(&entity.PrivacyErasureRequest{}).Where("event_id = ?", "evt-race").Count(&n)
		if n != 1 {
			t.Errorf("rows for evt-race = %d, want exactly 1 (UNIQUE event_id)", n)
		}
	})
}

func TestInternalGetPrivacyErasure(t *testing.T) {
	router, db := setupPrivacyEraseRouter(t)
	_ = db

	t.Run("unknown_event_404", func(t *testing.T) {
		w := internalRequest(router, "GET", "/internal/v1/privacy/erase/evt-ghost", nil,
			map[string]string{"X-API-Key": testApiKeyAllScopes})
		if w.Code != http.StatusNotFound {
			t.Errorf("code = %d, want 404", w.Code)
		}
	})

	t.Run("known_event_200", func(t *testing.T) {
		seedBoundUser(t, db, 6161)
		w := internalRequest(router, "POST", "/internal/v1/privacy/erase",
			map[string]interface{}{"event_id": "evt-poll", "account_id": 6161},
			map[string]string{"X-API-Key": testApiKeyAllScopes})
		if w.Code != http.StatusAccepted {
			t.Fatalf("intake code = %d, want 202", w.Code)
		}

		w2 := internalRequest(router, "GET", "/internal/v1/privacy/erase/evt-poll", nil,
			map[string]string{"X-API-Key": testApiKeyAllScopes})
		if w2.Code != http.StatusOK {
			t.Fatalf("poll code = %d, want 200", w2.Code)
		}
		resp := parseResponse(t, w2)
		data := resp["data"].(map[string]interface{})
		if data["status"] != repo.ErasureStatusPending || data["event_id"] != "evt-poll" {
			t.Errorf("poll data = %v", data)
		}
	})
}

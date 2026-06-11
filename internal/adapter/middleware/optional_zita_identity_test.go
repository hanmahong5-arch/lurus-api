package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/LurusTech/lurus-hub/internal/adapter/repo"
	"github.com/LurusTech/lurus-hub/internal/pkg/common"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	zita "github.com/hanmahong5-arch/zita-sdk-go"
	"gorm.io/gorm"
)

// mintLurusSession forges a platform-format HS256 session JWT
// (iss=lurus-platform, sub=lurus:<accountID>) so the test can exercise the
// SDK cookie path end-to-end without standing up platform-core. Mirrors the
// shape verifySessionToken / ValidateIdentitySessionToken expect.
func mintLurusSession(t *testing.T, accountID int64, secret string, exp time.Time) string {
	t.Helper()
	enc := func(v any) string {
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		return base64.RawURLEncoding.EncodeToString(b)
	}
	header := enc(map[string]string{"alg": "HS256", "typ": "JWT"})
	payload := enc(map[string]any{
		"iss": "lurus-platform",
		"sub": fmt.Sprintf("lurus:%d", accountID),
		"exp": exp.Unix(),
	})
	body := header + "." + payload
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(body))
	return body + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// setupZitaAuthTest wires an in-memory DB + a real zita.Client and returns the
// shared HMAC secret. Restores all swapped globals on cleanup.
func setupZitaAuthTest(t *testing.T) (*gorm.DB, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&repo.User{}, &repo.Token{}, &repo.Tenant{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	const secret = "zita-auth-test-secret-0123456789"
	client, err := zita.NewClient(zita.Config{
		PlatformURL:   "https://identity.test.local",
		SessionSecret: secret,
	})
	if err != nil {
		t.Fatalf("zita.NewClient: %v", err)
	}

	prevDB, prevLogDB := repo.DB, repo.LOG_DB
	prevSQLite, prevRedis := common.UsingSQLite, common.RedisEnabled
	prevClient := common.ZitaClient
	repo.DB, repo.LOG_DB = db, db
	common.UsingSQLite, common.RedisEnabled = true, false
	common.ZitaClient = client
	t.Cleanup(func() {
		repo.DB, repo.LOG_DB = prevDB, prevLogDB
		common.UsingSQLite, common.RedisEnabled = prevSQLite, prevRedis
		common.ZitaClient = prevClient
	})
	return db, secret
}

// buildZitaCookieRouter mounts OptionalZitaIdentity ahead of UserAuth on a
// tenant-scoped probe route — the production wiring from api-v2-router.go.
func buildZitaCookieRouter() *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	store := cookie.NewStore([]byte("zita-cookie-test-secret"))
	r.Use(sessions.Sessions("session", store))
	r.Use(OptionalZitaIdentity())
	r.GET("/api/v2/:tenant_slug/probe", UserAuth(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true, "id": c.GetInt("id")})
	})
	return r
}

func TestOptionalZitaIdentity_CookieAuthenticatesSDKUser(t *testing.T) {
	db, secret := setupZitaAuthTest(t)

	accountID := int64(4242)
	user := &repo.User{
		Username:       "lurus_4242",
		DisplayName:    "Bridged User",
		Role:           common.RoleCommonUser,
		Status:         common.UserStatusEnabled,
		Group:          "default",
		TenantId:       "default",
		LurusAccountID: &accountID,
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	t.Run("valid_cookie_authenticates", func(t *testing.T) {
		token := mintLurusSession(t, accountID, secret, time.Now().Add(time.Hour))
		req := httptest.NewRequest(http.MethodGet, "/api/v2/default/probe", nil)
		req.AddCookie(&http.Cookie{Name: zita.SessionCookieName, Value: token})
		w := httptest.NewRecorder()
		buildZitaCookieRouter().ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 (body=%s)", w.Code, w.Body.String())
		}
		var resp map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		if resp["success"] != true {
			t.Errorf("success = %v, want true", resp["success"])
		}
		if int(resp["id"].(float64)) != user.Id {
			t.Errorf("id = %v, want %d", resp["id"], user.Id)
		}
	})

	t.Run("bearer_token_authenticates", func(t *testing.T) {
		token := mintLurusSession(t, accountID, secret, time.Now().Add(time.Hour))
		req := httptest.NewRequest(http.MethodGet, "/api/v2/default/probe", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		buildZitaCookieRouter().ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 (body=%s)", w.Code, w.Body.String())
		}
	})

	t.Run("no_credentials_rejected", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v2/default/probe", nil)
		w := httptest.NewRecorder()
		buildZitaCookieRouter().ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401 (no creds must be rejected)", w.Code)
		}
	})

	t.Run("expired_cookie_rejected", func(t *testing.T) {
		token := mintLurusSession(t, accountID, secret, time.Now().Add(-time.Hour))
		req := httptest.NewRequest(http.MethodGet, "/api/v2/default/probe", nil)
		req.AddCookie(&http.Cookie{Name: zita.SessionCookieName, Value: token})
		w := httptest.NewRecorder()
		buildZitaCookieRouter().ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401 (expired cookie must not authenticate)", w.Code)
		}
	})

	t.Run("unknown_account_rejected", func(t *testing.T) {
		token := mintLurusSession(t, 999999, secret, time.Now().Add(time.Hour))
		req := httptest.NewRequest(http.MethodGet, "/api/v2/default/probe", nil)
		req.AddCookie(&http.Cookie{Name: zita.SessionCookieName, Value: token})
		w := httptest.NewRecorder()
		buildZitaCookieRouter().ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401 (unbound account must be rejected)", w.Code)
		}
	})

	t.Run("disabled_user_rejected", func(t *testing.T) {
		disabledAccount := int64(5252)
		disabled := &repo.User{
			Username:       "lurus_5252",
			Role:           common.RoleCommonUser,
			Status:         common.UserStatusDisabled,
			Group:          "default",
			TenantId:       "default",
			LurusAccountID: &disabledAccount,
		}
		if err := db.Create(disabled).Error; err != nil {
			t.Fatalf("create disabled user: %v", err)
		}
		token := mintLurusSession(t, disabledAccount, secret, time.Now().Add(time.Hour))
		req := httptest.NewRequest(http.MethodGet, "/api/v2/default/probe", nil)
		req.AddCookie(&http.Cookie{Name: zita.SessionCookieName, Value: token})
		w := httptest.NewRecorder()
		buildZitaCookieRouter().ServeHTTP(w, req)
		if w.Code == http.StatusOK {
			t.Fatalf("disabled user must not authenticate via SDK cookie")
		}
	})
}

package repo

// tenant_context_extra_test.go — coverage for the tenant-context helpers and
// Gin middleware in tenant_context.go (previously ~5%). These are pure
// context/DB-handle plumbing; the assertions check real extracted values,
// access-control decisions, and middleware HTTP outcomes — not err==nil.

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/LurusTech/lurus-hub/internal/pkg/common"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func init() { gin.SetMode(gin.TestMode) }

func newGinCtx() (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	return c, w
}

func TestGetTenantID_GetUserID(t *testing.T) {
	t.Run("present and correct types", func(t *testing.T) {
		c, _ := newGinCtx()
		InjectTenantContext(c, "tenant-x", 42)
		tid, err := GetTenantID(c)
		if err != nil || tid != "tenant-x" {
			t.Fatalf("GetTenantID = %q, %v", tid, err)
		}
		uid, err := GetUserID(c)
		if err != nil || uid != 42 {
			t.Fatalf("GetUserID = %d, %v", uid, err)
		}
	})

	t.Run("missing", func(t *testing.T) {
		c, _ := newGinCtx()
		if _, err := GetTenantID(c); err == nil {
			t.Error("want error when tenant_id absent")
		}
		if _, err := GetUserID(c); err == nil {
			t.Error("want error when user_id absent")
		}
	})

	t.Run("wrong types", func(t *testing.T) {
		c, _ := newGinCtx()
		c.Set("tenant_id", 123)   // not a string
		c.Set("user_id", "hello") // not an int
		if _, err := GetTenantID(c); err == nil {
			t.Error("want error for non-string tenant_id")
		}
		if _, err := GetUserID(c); err == nil {
			t.Error("want error for non-int user_id")
		}
	})
}

func TestValidateTenantAccess(t *testing.T) {
	c, _ := newGinCtx()
	InjectTenantContext(c, "t-owner", 1)

	if err := ValidateTenantAccess(c, "t-owner"); err != nil {
		t.Errorf("same tenant must be allowed: %v", err)
	}
	if err := ValidateTenantAccess(c, "t-other"); err == nil {
		t.Error("cross-tenant access must be denied")
	}

	// No context → error surfaced from GetTenantID.
	empty, _ := newGinCtx()
	if err := ValidateTenantAccess(empty, "anything"); err == nil {
		t.Error("want error when no tenant context")
	}
}

func TestIsPlatformAdmin(t *testing.T) {
	cases := []struct {
		name string
		set  func(c *gin.Context)
		want bool
	}{
		{"role 100 is admin", func(c *gin.Context) { c.Set("user_role", 100) }, true},
		{"role 1 not admin", func(c *gin.Context) { c.Set("user_role", 1) }, false},
		{"missing role", func(c *gin.Context) {}, false},
		{"wrong type", func(c *gin.Context) { c.Set("user_role", "100") }, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, _ := newGinCtx()
			tc.set(c)
			if got := IsPlatformAdmin(c); got != tc.want {
				t.Errorf("IsPlatformAdmin = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestTenantDBHandles(t *testing.T) {
	cleanup := setupSQLiteDB(t)
	defer cleanup()

	// GetTenantDB requires tenant context.
	c, _ := newGinCtx()
	if _, err := GetTenantDB(c); err == nil {
		t.Error("GetTenantDB must fail without tenant context")
	}
	InjectTenantContext(c, "t-1", 1)
	db, err := GetTenantDB(c)
	if err != nil || db == nil {
		t.Fatalf("GetTenantDB: %v", err)
	}
	if GetTenantIDFromDB(db) != "t-1" {
		t.Errorf("tenant id not stamped on handle: %q", GetTenantIDFromDB(db))
	}

	// GetTenantDBWithID stamps the requested id.
	db2 := GetTenantDBWithID("t-2")
	if GetTenantIDFromDB(db2) != "t-2" {
		t.Errorf("GetTenantDBWithID stamp wrong: %q", GetTenantIDFromDB(db2))
	}

	// GetSystemDB carries the skip-isolation flag (no tenant id stamped).
	sys := GetSystemDB()
	if GetTenantIDFromDB(sys) != "" {
		t.Error("system DB must not carry a tenant id")
	}
}

func TestResolveDefaultTenantID_And_DefaultDB(t *testing.T) {
	cleanup := setupSQLiteDB(t)
	defer cleanup()

	// ResolveDefaultTenantID caches process-wide via sync.Once; whichever test
	// fires it first wins. Regardless, it must return a non-empty id and the
	// default-tenant DB handle must be stamped with that same id.
	id := ResolveDefaultTenantID()
	if id == "" {
		t.Fatal("ResolveDefaultTenantID returned empty")
	}
	ddb := GetDefaultTenantDB()
	if GetTenantIDFromDB(ddb) != id {
		t.Errorf("GetDefaultTenantDB stamp %q != resolved %q", GetTenantIDFromDB(ddb), id)
	}
}

func TestWithTenantContext_And_Transactions(t *testing.T) {
	cleanup := setupSQLiteDB(t)
	defer cleanup()

	// WithTenantContext hands the callback a tenant-stamped DB and returns its error.
	seen := ""
	err := WithTenantContext("t-cb", func(db *gorm.DB) error {
		seen = GetTenantIDFromDB(db)
		return nil
	})
	if err != nil {
		t.Fatalf("WithTenantContext: %v", err)
	}
	if seen != "t-cb" {
		t.Errorf("callback saw tenant %q, want t-cb", seen)
	}

	// TenantTransactionWithID commits a real write scoped to the tenant.
	if err := TenantTransactionWithID("t-tx", func(tx *gorm.DB) error {
		u := &User{Username: "tx_user", DisplayName: "x", TenantId: "t-tx", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Email: "tx@local"}
		return tx.Create(u).Error
	}); err != nil {
		t.Fatalf("TenantTransactionWithID: %v", err)
	}
	var cnt int64
	DB.Model(&User{}).Where("username = ?", "tx_user").Count(&cnt)
	if cnt != 1 {
		t.Fatalf("TenantTransactionWithID did not persist row, count=%d", cnt)
	}

	// SystemTransaction runs without isolation.
	if err := SystemTransaction(func(tx *gorm.DB) error {
		u := &User{Username: "sys_user", DisplayName: "y", TenantId: "whatever", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Email: "sys@local"}
		return tx.Create(u).Error
	}); err != nil {
		t.Fatalf("SystemTransaction: %v", err)
	}
	DB.Model(&User{}).Where("username = ?", "sys_user").Count(&cnt)
	if cnt != 1 {
		t.Fatalf("SystemTransaction did not persist row, count=%d", cnt)
	}
}

func TestRequireTenantAccessMiddleware(t *testing.T) {
	run := func(inject bool) int {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
		if inject {
			InjectTenantContext(c, "t-mw", 1)
		}
		nextCalled := false
		mw := RequireTenantAccess()
		mw(c)
		c.Next()
		if !c.IsAborted() {
			nextCalled = true
		}
		_ = nextCalled
		return w.Code
	}
	if code := run(false); code != http.StatusUnauthorized {
		t.Errorf("no tenant → want 401, got %d", code)
	}
	if code := run(true); code == http.StatusUnauthorized {
		t.Errorf("with tenant → must not 401, got %d", code)
	}
}

func TestSetDefaultTenantMiddleware(t *testing.T) {
	cleanup := setupSQLiteDB(t)
	defer cleanup()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	SetDefaultTenant()(c)
	tid, err := GetTenantID(c)
	if err != nil || tid == "" {
		t.Fatalf("SetDefaultTenant must inject a default tenant id, got %q err=%v", tid, err)
	}
}

func TestTenantSwitchMiddleware(t *testing.T) {
	cleanup := setupSQLiteDB(t)
	defer cleanup()
	seedTenant(t, "t-target", "target", "Target Co")

	t.Run("non-admin passes through untouched", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
		c.Request.Header.Set("X-Target-Tenant-ID", "t-target")
		c.Set("user_role", 1) // not admin
		TenantSwitchMiddleware()(c)
		if c.IsAborted() {
			t.Error("non-admin request must not be aborted")
		}
		if _, exists := c.Get("tenant_id"); exists {
			t.Error("non-admin must not have tenant switched")
		}
	})

	t.Run("admin switches to existing tenant", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
		c.Request.Header.Set("X-Target-Tenant-ID", "t-target")
		c.Set("user_role", 100)
		c.Set("user_id", 5)
		TenantSwitchMiddleware()(c)
		if c.IsAborted() {
			t.Fatal("admin switch to existing tenant must not abort")
		}
		tid, _ := GetTenantID(c)
		if tid != "t-target" {
			t.Errorf("want switched tenant t-target, got %q", tid)
		}
	})

	t.Run("admin switch to missing tenant → 404", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
		c.Request.Header.Set("X-Target-Tenant-ID", "t-nope")
		c.Set("user_role", 100)
		TenantSwitchMiddleware()(c)
		if w.Code != http.StatusNotFound {
			t.Errorf("missing target tenant → want 404, got %d", w.Code)
		}
		if !c.IsAborted() {
			t.Error("must abort on missing target tenant")
		}
	})
}

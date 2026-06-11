package repo

import (
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/LurusTech/lurus-hub/internal/pkg/common"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// setupAdminUserTestDB swaps the package-level repo.DB with an in-memory
// sqlite instance migrated with just the User table. Same pattern as
// setupPoolTestDB — restore on cleanup so other tests are unaffected.
var adminUserTestDBCounter atomic.Int64

func setupAdminUserTestDB(t *testing.T) (cleanup func()) {
	t.Helper()
	n := adminUserTestDBCounter.Add(1)
	dsn := fmt.Sprintf("file:adminusers%d?mode=memory&cache=shared", n)
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&User{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	prev := DB
	DB = db
	// The admin user funcs hit commonGroupCol (set by InitCol from the DB
	// type flags) and the user cache (gated on RedisEnabled) — pin both to
	// the sqlite/no-redis configuration and restore afterwards.
	prevSQLite, prevPG, prevRedis := common.UsingSQLite, common.UsingPostgreSQL, common.RedisEnabled
	common.UsingSQLite, common.UsingPostgreSQL, common.RedisEnabled = true, false, false
	InitCol()
	return func() {
		DB = prev
		common.UsingSQLite, common.UsingPostgreSQL, common.RedisEnabled = prevSQLite, prevPG, prevRedis
		InitCol()
	}
}

func seedAdminUser(t *testing.T, username, email, group string, role, status int) *User {
	t.Helper()
	u := &User{Username: username, Email: email, Group: group, Role: role, Status: status}
	if err := DB.Create(u).Error; err != nil {
		t.Fatalf("seed user %s: %v", username, err)
	}
	return u
}

func TestUserRepo_AdminListUsers_Filters(t *testing.T) {
	defer setupAdminUserTestDB(t)()

	alice := seedAdminUser(t, "alice", "alice@example.com", "default", 1, 1)
	seedAdminUser(t, "bob", "bob@example.com", "vip", 1, 1)
	seedAdminUser(t, "carol", "carol@example.com", "default", 100, 2)

	cases := []struct {
		name      string
		keyword   string
		group     string
		status    int
		wantTotal int64
	}{
		{"no filters", "", "", 0, 3},
		{"keyword username", "alice", "", 0, 1},
		{"keyword numeric id", fmt.Sprintf("%d", alice.Id), "", 0, 1},
		{"group filter", "", "vip", 0, 1},
		{"status filter", "", "", 2, 1},
		{"group+status no match", "", "vip", 2, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			users, total, err := AdminListUsers(tc.keyword, tc.group, tc.status, 0, 10)
			if err != nil {
				t.Fatalf("AdminListUsers: %v", err)
			}
			if total != tc.wantTotal {
				t.Errorf("total = %d, want %d", total, tc.wantTotal)
			}
			if int64(len(users)) != tc.wantTotal {
				t.Errorf("len(users) = %d, want %d", len(users), tc.wantTotal)
			}
		})
	}
}

func TestUserRepo_AdminListUsers_Pagination(t *testing.T) {
	defer setupAdminUserTestDB(t)()

	for i := 0; i < 5; i++ {
		seedAdminUser(t, fmt.Sprintf("user%d", i), fmt.Sprintf("u%d@example.com", i), "default", 1, 1)
	}
	users, total, err := AdminListUsers("", "", 0, 2, 2)
	if err != nil {
		t.Fatalf("AdminListUsers: %v", err)
	}
	if total != 5 {
		t.Errorf("total = %d, want 5", total)
	}
	if len(users) != 2 {
		t.Errorf("page len = %d, want 2", len(users))
	}
}

func TestUserRepo_AdminUpdateUser(t *testing.T) {
	defer setupAdminUserTestDB(t)()

	if _, err := AdminUpdateUser(0, 1, 1, 0, "default"); err == nil {
		t.Error("AdminUpdateUser(id=0) should error")
	}
	if _, err := AdminUpdateUser(99999, 1, 1, 0, "default"); err == nil {
		t.Error("AdminUpdateUser(missing id) should error")
	}

	u := seedAdminUser(t, "dave", "dave@example.com", "default", 1, 1)
	// quota=0 must be honoured (map-based update), not skipped as a zero value.
	got, err := AdminUpdateUser(u.Id, 10, 2, 0, "vip")
	if err != nil {
		t.Fatalf("AdminUpdateUser: %v", err)
	}
	if got.Role != 10 || got.Status != 2 || got.Quota != 0 || got.Group != "vip" {
		t.Errorf("returned user = role %d status %d quota %d group %q, want 10/2/0/vip",
			got.Role, got.Status, got.Quota, got.Group)
	}
	var back User
	if err := DB.First(&back, "id = ?", u.Id).Error; err != nil {
		t.Fatalf("readback: %v", err)
	}
	if back.Role != 10 || back.Status != 2 || back.Quota != 0 || back.Group != "vip" {
		t.Errorf("persisted user = role %d status %d quota %d group %q, want 10/2/0/vip",
			back.Role, back.Status, back.Quota, back.Group)
	}
}

func TestUserRepo_AdminDeleteUser(t *testing.T) {
	defer setupAdminUserTestDB(t)()

	if err := AdminDeleteUser(0); err == nil {
		t.Error("AdminDeleteUser(id=0) should error")
	}

	u := seedAdminUser(t, "erin", "erin@example.com", "default", 1, 1)
	if err := AdminDeleteUser(u.Id); err != nil {
		t.Fatalf("AdminDeleteUser: %v", err)
	}
	// Soft delete: excluded from default queries.
	if _, total, err := AdminListUsers("erin", "", 0, 0, 10); err != nil || total != 0 {
		t.Errorf("deleted user still listed (total=%d, err=%v)", total, err)
	}
}

func TestUserRepo_CountUsersByRole(t *testing.T) {
	defer setupAdminUserTestDB(t)()

	seedAdminUser(t, "root1", "r1@example.com", "default", 100, 1)
	seedAdminUser(t, "root2", "r2@example.com", "default", 100, 1)
	seedAdminUser(t, "plain", "p@example.com", "default", 1, 1)

	n, err := CountUsersByRole(100)
	if err != nil {
		t.Fatalf("CountUsersByRole: %v", err)
	}
	if n != 2 {
		t.Errorf("CountUsersByRole(100) = %d, want 2", n)
	}
}

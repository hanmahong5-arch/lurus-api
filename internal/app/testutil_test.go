package app

import (
	"net/http/httptest"
	"testing"

	"github.com/LurusTech/lurus-hub/internal/adapter/repo"
	"github.com/LurusTech/lurus-hub/internal/pkg/common"
	"github.com/glebarez/sqlite"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// setupServiceTestDB creates an in-memory SQLite database with all required
// tables for service-layer tests that need DB access. It wires the global
// repo.DB and repo.LOG_DB, and restores them on cleanup.
func setupServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open in-memory sqlite: %v", err)
	}

	err = db.AutoMigrate(
		&repo.User{},
		&repo.Token{},
		&repo.Log{},
		&repo.Channel{},
		&repo.Option{},
		&repo.Tenant{},
	)
	if err != nil {
		t.Fatalf("failed to auto-migrate: %v", err)
	}

	prevDB := repo.DB
	prevLogDB := repo.LOG_DB
	prevSQLite := common.UsingSQLite
	prevPG := common.UsingPostgreSQL
	prevMySQL := common.UsingMySQL
	prevRedis := common.RedisEnabled

	repo.DB = db
	repo.LOG_DB = db
	common.UsingSQLite = true
	common.UsingPostgreSQL = false
	common.UsingMySQL = false
	common.RedisEnabled = false

	common.OptionMapRWMutex.Lock()
	if common.OptionMap == nil {
		common.OptionMap = make(map[string]string)
	}
	common.OptionMapRWMutex.Unlock()

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			sqlDB.Close()
		}
		repo.DB = prevDB
		repo.LOG_DB = prevLogDB
		common.UsingSQLite = prevSQLite
		common.UsingPostgreSQL = prevPG
		common.UsingMySQL = prevMySQL
		common.RedisEnabled = prevRedis
	})

	return db
}

// seedTestUser creates a test user with the given quota and returns its ID.
func seedTestUser(t *testing.T, db *gorm.DB, quota int) int {
	t.Helper()
	user := repo.User{
		Username:    "testuser-" + common.GetRandomString(6),
		DisplayName: "Test User",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		Email:       "test@test.local",
		Quota:       quota,
		Group:       "default",
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}
	return user.Id
}

// seedTestToken creates a test token for the given user and returns the key and token ID.
func seedTestToken(t *testing.T, db *gorm.DB, userId int, remainQuota int, unlimited bool) (key string, tokenId int) {
	t.Helper()
	key = common.GetRandomString(32)
	token := repo.Token{
		UserId:         userId,
		Key:            key,
		Status:         common.TokenStatusEnabled,
		Name:           "test-token",
		CreatedTime:    common.GetTimestamp(),
		AccessedTime:   common.GetTimestamp(),
		ExpiredTime:    -1,
		RemainQuota:    remainQuota,
		UnlimitedQuota: unlimited,
		Group:          "default",
	}
	if err := db.Create(&token).Error; err != nil {
		t.Fatalf("failed to seed token: %v", err)
	}
	return key, token.Id
}

// createTestGinContext creates a minimal gin context for testing.
func createTestGinContext() *gin.Context {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	return c
}

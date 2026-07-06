package openrouter_pool

import (
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/LurusTech/lurus-hub/internal/adapter/repo"
	"github.com/LurusTech/lurus-hub/internal/pkg/common"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// This file gives the openrouter_pool tests a real PostgreSQL harness, mirroring
// the isolated-DB pattern in internal/adapter/repo/testutil_test.go (which is
// package-private and therefore not importable here). Each call creates a fresh
// database, wires repo.DB to it, and drops it on cleanup — so tests are
// -count-safe and cannot leak state into each other.

var poolTestDBCounter atomic.Int64

var poolDBNameRe = regexp.MustCompile(`(?i)(dbname=)\S+`)

func buildPoolTestDSN(baseDSN, dbName string) string {
	if strings.HasPrefix(baseDSN, "postgres://") || strings.HasPrefix(baseDSN, "postgresql://") {
		if u, err := url.Parse(baseDSN); err == nil {
			u.Path = "/" + dbName
			return u.String()
		}
	}
	if poolDBNameRe.MatchString(baseDSN) {
		return poolDBNameRe.ReplaceAllString(baseDSN, "${1}"+dbName)
	}
	return baseDSN + " dbname=" + dbName
}

// setupPoolTestDB connects to PostgreSQL (via TEST_POSTGRES_DSN), creates an
// isolated database, AutoMigrates the tables the reaper/writer touch, and wires
// repo.DB. It restores the previous globals on cleanup. Skips when the DSN env
// var is unset so contributors without local PG are not broken.
func setupPoolTestDB(t *testing.T) {
	t.Helper()

	baseDSN := os.Getenv("TEST_POSTGRES_DSN")
	if baseDSN == "" {
		t.Skip("TEST_POSTGRES_DSN not set; skipping PostgreSQL integration test")
	}

	dbName := fmt.Sprintf("test_orpool_%d_%d", time.Now().UnixNano(), poolTestDBCounter.Add(1))

	adminDB, err := gorm.Open(postgres.Open(baseDSN), &gorm.Config{})
	if err != nil {
		t.Fatalf("open admin connection: %v", err)
	}
	if err := adminDB.Exec(fmt.Sprintf(`CREATE DATABASE "%s"`, dbName)).Error; err != nil {
		t.Fatalf("create test database %q: %v", dbName, err)
	}
	if sqlDB, err := adminDB.DB(); err == nil {
		_ = sqlDB.Close()
	}

	db, err := gorm.Open(postgres.Open(buildPoolTestDSN(baseDSN, dbName)), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test database %q: %v", dbName, err)
	}

	if err := db.AutoMigrate(&repo.Channel{}, &repo.Ability{}); err != nil {
		t.Fatalf("auto-migrate: %v", err)
	}

	prevDB := repo.DB
	prevPG := common.UsingPostgreSQL
	prevSQLite := common.UsingSQLite
	prevMySQL := common.UsingMySQL
	prevRedis := common.RedisEnabled
	prevMemCache := common.MemoryCacheEnabled

	repo.DB = db
	common.UsingPostgreSQL = true
	common.UsingSQLite = false
	common.UsingMySQL = false
	common.RedisEnabled = false
	common.MemoryCacheEnabled = false

	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
		if dropDB, err := gorm.Open(postgres.Open(baseDSN), &gorm.Config{}); err == nil {
			_ = dropDB.Exec(fmt.Sprintf(`DROP DATABASE IF EXISTS "%s"`, dbName)).Error
			if sqlDB, err := dropDB.DB(); err == nil {
				_ = sqlDB.Close()
			}
		}
		repo.DB = prevDB
		common.UsingPostgreSQL = prevPG
		common.UsingSQLite = prevSQLite
		common.UsingMySQL = prevMySQL
		common.RedisEnabled = prevRedis
		common.MemoryCacheEnabled = prevMemCache
	})
}

// seedMultiKeyChannel persists a fresh OpenRouter multi-key channel with the
// given per-key cooldown deadlines. cooldownUntil[idx] seeds a cooling key;
// indices absent from the map are left enabled. status is the channel-level
// status. Returns the created channel id.
func seedMultiKeyChannel(t *testing.T, size int, status int, cooldownUntil map[int]int64) int {
	t.Helper()

	keys := make([]string, size)
	for i := range keys {
		keys[i] = fmt.Sprintf("sk-or-v1-seed-%d-%d", time.Now().UnixNano(), i)
	}

	statusList := make(map[int]int)
	reason := make(map[int]string)
	disabledTime := make(map[int]int64)
	cooldowns := make(map[int]int64)
	for idx, until := range cooldownUntil {
		statusList[idx] = common.ChannelStatusAutoDisabled
		reason[idx] = "rate limit (auto cooldown)"
		disabledTime[idx] = until - 60
		cooldowns[idx] = until
	}

	ch := &repo.Channel{
		Type:   20, // ChannelTypeOpenRouter
		Key:    strings.Join(keys, "\n"),
		Status: status,
		Name:   "seed-openrouter-pool",
		ChannelInfo: repo.ChannelInfo{
			IsMultiKey:             true,
			MultiKeySize:           size,
			MultiKeyStatusList:     statusList,
			MultiKeyDisabledReason: reason,
			MultiKeyDisabledTime:   disabledTime,
			MultiKeyCooldownUntil:  cooldowns,
		},
	}
	if err := repo.DB.Create(ch).Error; err != nil {
		t.Fatalf("seed channel: %v", err)
	}
	return ch.Id
}

// keysOf returns the persisted key strings for a channel id, in order.
func keysOf(t *testing.T, channelId int) []string {
	t.Helper()
	var ch repo.Channel
	if err := repo.DB.First(&ch, channelId).Error; err != nil {
		t.Fatalf("load channel %d: %v", channelId, err)
	}
	return ch.GetKeys()
}

// reloadChannel returns a fresh copy of the channel from the DB.
func reloadChannel(t *testing.T, channelId int) *repo.Channel {
	t.Helper()
	var ch repo.Channel
	if err := repo.DB.First(&ch, channelId).Error; err != nil {
		t.Fatalf("reload channel %d: %v", channelId, err)
	}
	return &ch
}

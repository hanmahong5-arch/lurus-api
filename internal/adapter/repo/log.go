package repo

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/LurusTech/lurus-hub/internal/domain/entity"
	"github.com/LurusTech/lurus-hub/internal/pkg/common"
	"github.com/LurusTech/lurus-hub/internal/pkg/logger"
	"github.com/LurusTech/lurus-hub/internal/pkg/search"
	"github.com/LurusTech/lurus-hub/internal/pkg/types"

	"github.com/gin-gonic/gin"

	"github.com/bytedance/gopkg/util/gopool"
	"gorm.io/gorm"
)

// Type aliases pointing to entity package
type Log = entity.Log
type RecordConsumeLogParams = entity.RecordConsumeLogParams
type LogQueryParams = entity.LogQueryParams
type Stat = entity.Stat

// Re-export log type constants from entity
const (
	LogTypeUnknown = entity.LogTypeUnknown
	LogTypeTopup   = entity.LogTypeTopup
	LogTypeConsume = entity.LogTypeConsume
	LogTypeManage  = entity.LogTypeManage
	LogTypeSystem  = entity.LogTypeSystem
	LogTypeError   = entity.LogTypeError
	LogTypeRefund  = entity.LogTypeRefund
)

func formatUserLogs(logs []*Log) {
	for i := range logs {
		logs[i].ChannelName = ""
		// Strip Internal-tier governance struct fields (not visible to regular users).
		logs[i].RequestFingerprint = ""
		logs[i].UpstreamModel = ""
		var otherMap map[string]interface{}
		otherMap, _ = common.StrToMap(logs[i].Other)
		if otherMap != nil {
			// Strip Internal-tier fields from Other map (governance classification).
			for _, key := range internalOtherKeys {
				delete(otherMap, key)
			}
		}
		logs[i].Other = common.MapToJsonStr(otherMap)
		logs[i].Id = logs[i].Id % 1024
	}
}

// internalOtherKeys lists Other map keys classified as TierInternal that must
// be stripped before returning logs to non-admin users.
var internalOtherKeys = []string{
	"admin_info",
	"model_ratio",
	"group_ratio",
	"completion_ratio",
	"cache_ratio",
	"cache_creation_ratio",
	"model_price",
	"user_group_ratio",
	"frt",
	"is_model_mapped",
	"upstream_model_name",
	"web_search_price",
	"web_search_call_count",
	"file_search_price",
	"file_search_call_count",
	"image_ratio",
	"audio_ratio",
	"audio_completion_ratio",
	"audio_input_price",
	"image_generation_call_price",
	"data_flow_source",
	"data_flow_dest",
}

func GetLogByKey(key string) (logs []*Log, err error) {
	if os.Getenv("LOG_SQL_DSN") != "" {
		var tk Token
		if err = DB.Model(&Token{}).Where(logKeyCol+"=?", strings.TrimPrefix(key, "sk-")).First(&tk).Error; err != nil {
			return nil, err
		}
		err = LOG_DB.Model(&Log{}).Where("token_id=?", tk.Id).Find(&logs).Error
	} else {
		err = LOG_DB.Joins("left join tokens on tokens.id = logs.token_id").Where("tokens.key = ?", strings.TrimPrefix(key, "sk-")).Find(&logs).Error
	}
	formatUserLogs(logs)
	return logs, err
}

// recordLogTx writes an audit log within a DB transaction when LOG_DB == DB (single-database setup).
// When LOG_DB is a separate database, falls back to LOG_DB (best-effort, no cross-DB transaction).
func recordLogTx(tx *gorm.DB, userId int, logType int, content string) {
	if logType == LogTypeConsume && !common.LogConsumeEnabled {
		return
	}
	username, _ := GetUsernameById(userId, false)
	l := &Log{
		UserId:    userId,
		Username:  username,
		CreatedAt: common.GetTimestamp(),
		Type:      logType,
		Content:   content,
	}
	db := LOG_DB
	if LOG_DB == DB {
		db = tx
	}
	if err := db.Create(l).Error; err != nil {
		common.SysLog("failed to record log in transaction: " + err.Error())
	} else {
		search.SyncLogAsync(convertLogToSearchLog(l))
	}
}

func RecordLog(userId int, logType int, content string) {
	if logType == LogTypeConsume && !common.LogConsumeEnabled {
		return
	}
	username, _ := GetUsernameById(userId, false)
	log := &Log{
		UserId:    userId,
		Username:  username,
		CreatedAt: common.GetTimestamp(),
		Type:      logType,
		Content:   content,
	}
	err := LOG_DB.Create(log).Error
	if err != nil {
		common.SysLog("failed to record log: " + err.Error())
	} else {
		// Async sync to Meilisearch
		// 异步同步到 Meilisearch
		search.SyncLogAsync(convertLogToSearchLog(log))
	}
}

// RecordLogWithTenant writes an audit log with explicit tenant_id.
func RecordLogWithTenant(userId int, tenantID string, logType int, content string) {
	if logType == LogTypeConsume && !common.LogConsumeEnabled {
		return
	}
	username, _ := GetUsernameById(userId, false)
	log := &Log{
		UserId:    userId,
		TenantId:  tenantID,
		Username:  username,
		CreatedAt: common.GetTimestamp(),
		Type:      logType,
		Content:   content,
	}
	err := LOG_DB.Create(log).Error
	if err != nil {
		common.SysLog("failed to record log: " + err.Error())
	} else {
		search.SyncLogAsync(convertLogToSearchLog(log))
	}
}

// convertLogToSearchLog converts model.Log to search.Log
// 将 model.Log 转换为 search.Log
func convertLogToSearchLog(log *Log) *search.Log {
	return &search.Log{
		Id:               log.Id,
		CreatedAt:        log.CreatedAt,
		Type:             log.Type,
		UserId:           log.UserId,
		Username:         log.Username,
		TokenId:          log.TokenId,
		TokenName:        log.TokenName,
		ModelName:        log.ModelName,
		Content:          log.Content,
		Quota:            log.Quota,
		PromptTokens:     log.PromptTokens,
		CompletionTokens: log.CompletionTokens,
		UseTime:          log.UseTime,
		IsStream:         log.IsStream,
		ChannelId:        log.ChannelId,
		ChannelName:      log.ChannelName,
		Group:            log.Group,
		Ip:               log.Ip,
		Other:            log.Other,
		ChannelType:      log.ChannelType,
		RelayMode:        log.RelayMode,
		UpstreamModel:    log.UpstreamModel,
		TotalLatencyMs:   log.TotalLatencyMs,
	}
}

func RecordErrorLog(c *gin.Context, userId int, channelId int, modelName string, tokenName string, content string, tokenId int, useTimeSeconds int,
	isStream bool, group string, other map[string]interface{}) {
	logger.LogInfo(c, fmt.Sprintf("record error log: userId=%d, channelId=%d, modelName=%s, tokenName=%s, content=%s", userId, channelId, modelName, tokenName, content))
	username := c.GetString("username")
	otherStr := common.MapToJsonStr(other)
	// 判断是否需要记录 IP
	needRecordIp := false
	if settingMap, err := GetUserSetting(userId, false); err == nil {
		if settingMap.RecordIpLog {
			needRecordIp = true
		}
	}
	// Extract governance fields from context for error logs.
	channelType := c.GetInt("channel_type")
	tenantId := c.GetString("tenant_id")
	if tenantId == "" {
		tenantId = "default"
	}
	log := &Log{
		UserId:           userId,
		TenantId:         tenantId,
		Username:         username,
		CreatedAt:        common.GetTimestamp(),
		Type:             LogTypeError,
		Content:          content,
		PromptTokens:     0,
		CompletionTokens: 0,
		TokenName:        tokenName,
		ModelName:        modelName,
		Quota:            0,
		ChannelId:        channelId,
		TokenId:          tokenId,
		UseTime:          useTimeSeconds,
		IsStream:         isStream,
		Group:            group,
		Ip: func() string {
			if needRecordIp {
				return c.ClientIP()
			}
			return ""
		}(),
		Other:       otherStr,
		ChannelType: channelType,
	}
	err := LOG_DB.Create(log).Error
	if err != nil {
		logger.LogError(c, "failed to record log: "+err.Error())
	} else {
		// Async sync to Meilisearch
		// 异步同步到 Meilisearch
		search.SyncLogAsync(convertLogToSearchLog(log))
	}
}

func RecordConsumeLog(c *gin.Context, userId int, params RecordConsumeLogParams) {
	if !common.LogConsumeEnabled {
		return
	}
	// Content strategy: skip log record if user opted out via LogDetailLevel="none".
	// NOTE: This only skips the log write — quota deduction and billing settlement
	// have already occurred before RecordConsumeLog is called. Financial records
	// remain intact; only the consume log entry is omitted.
	if params.LogDetailLevel == "none" {
		return
	}
	logger.LogInfo(c, fmt.Sprintf("record consume log: userId=%d, params=%s", userId, common.GetJsonString(params)))
	username := c.GetString("username")
	otherStr := common.MapToJsonStr(params.Other)
	// 判断是否需要记录 IP
	needRecordIp := false
	if settingMap, err := GetUserSetting(userId, false); err == nil {
		if settingMap.RecordIpLog {
			needRecordIp = true
		}
	}
	tenantId := c.GetString("tenant_id")
	if tenantId == "" {
		tenantId = "default"
	}
	log := &Log{
		UserId:           userId,
		TenantId:         tenantId,
		Username:         username,
		CreatedAt:        common.GetTimestamp(),
		Type:             LogTypeConsume,
		Content:          params.Content,
		PromptTokens:     params.PromptTokens,
		CompletionTokens: params.CompletionTokens,
		TokenName:        params.TokenName,
		ModelName:        params.ModelName,
		Quota:            params.Quota,
		ChannelId:        params.ChannelId,
		TokenId:          params.TokenId,
		UseTime:          params.UseTimeSeconds,
		IsStream:         params.IsStream,
		Group:            params.Group,
		Ip: func() string {
			if needRecordIp {
				return c.ClientIP()
			}
			return ""
		}(),
		Other:              otherStr,
		ChannelType:        params.ChannelType,
		RelayMode:          params.RelayMode,
		RequestFingerprint: params.RequestFingerprint,
		UpstreamModel:      params.UpstreamModel,
		TotalLatencyMs:     params.TotalLatencyMs,
	}
	err := LOG_DB.Create(log).Error
	if err != nil {
		logger.LogError(c, "failed to record log: "+err.Error())
	} else {
		// Async sync to Meilisearch
		// 异步同步到 Meilisearch
		search.SyncLogAsync(convertLogToSearchLog(log))
	}
	if common.DataExportEnabled {
		gopool.Go(func() {
			LogQuotaData(userId, username, params.ModelName, params.Quota, common.GetTimestamp(), params.PromptTokens+params.CompletionTokens)
		})
	}
}

func GetAllLogs(logType int, startTimestamp int64, endTimestamp int64, modelName string, username string, tokenName string, startIdx int, num int, channel int, group string) (logs []*Log, total int64, err error) {
	var tx *gorm.DB
	if logType == LogTypeUnknown {
		tx = LOG_DB
	} else {
		tx = LOG_DB.Where("logs.type = ?", logType)
	}

	if modelName != "" {
		tx = tx.Where("logs.model_name like ?", modelName)
	}
	if username != "" {
		tx = tx.Where("logs.username = ?", username)
	}
	if tokenName != "" {
		tx = tx.Where("logs.token_name = ?", tokenName)
	}
	if startTimestamp != 0 {
		tx = tx.Where("logs.created_at >= ?", startTimestamp)
	}
	if endTimestamp != 0 {
		tx = tx.Where("logs.created_at <= ?", endTimestamp)
	}
	if channel != 0 {
		tx = tx.Where("logs.channel_id = ?", channel)
	}
	if group != "" {
		tx = tx.Where("logs."+logGroupCol+" = ?", group)
	}
	err = tx.Model(&Log{}).Count(&total).Error
	if err != nil {
		return nil, 0, err
	}
	err = tx.Order("logs.id desc").Limit(num).Offset(startIdx).Find(&logs).Error
	if err != nil {
		return nil, 0, err
	}

	channelIds := types.NewSet[int]()
	for _, log := range logs {
		if log.ChannelId != 0 {
			channelIds.Add(log.ChannelId)
		}
	}

	if channelIds.Len() > 0 {
		var channels []struct {
			Id   int    `gorm:"column:id"`
			Name string `gorm:"column:name"`
		}
		if err = DB.Table("channels").Select("id, name").Where("id IN ?", channelIds.Items()).Find(&channels).Error; err != nil {
			return logs, total, err
		}
		channelMap := make(map[int]string, len(channels))
		for _, channel := range channels {
			channelMap[channel.Id] = channel.Name
		}
		for i := range logs {
			logs[i].ChannelName = channelMap[logs[i].ChannelId]
		}
	}

	return logs, total, err
}

func GetUserLogs(userId int, logType int, startTimestamp int64, endTimestamp int64, modelName string, tokenName string, startIdx int, num int, group string) (logs []*Log, total int64, err error) {
	var tx *gorm.DB
	if logType == LogTypeUnknown {
		tx = LOG_DB.Where("logs.user_id = ?", userId)
	} else {
		tx = LOG_DB.Where("logs.user_id = ? and logs.type = ?", userId, logType)
	}

	if modelName != "" {
		tx = tx.Where("logs.model_name like ?", modelName)
	}
	if tokenName != "" {
		tx = tx.Where("logs.token_name = ?", tokenName)
	}
	if startTimestamp != 0 {
		tx = tx.Where("logs.created_at >= ?", startTimestamp)
	}
	if endTimestamp != 0 {
		tx = tx.Where("logs.created_at <= ?", endTimestamp)
	}
	if group != "" {
		tx = tx.Where("logs."+logGroupCol+" = ?", group)
	}
	err = tx.Model(&Log{}).Count(&total).Error
	if err != nil {
		return nil, 0, err
	}
	err = tx.Order("logs.id desc").Limit(num).Offset(startIdx).Find(&logs).Error
	if err != nil {
		return nil, 0, err
	}

	formatUserLogs(logs)
	return logs, total, err
}

// logKeywordCondition matches keyword against the integer `type` column only
// when it parses as a number: on PostgreSQL, binding an arbitrary string
// against an integer column raises 22P02 ("invalid input syntax for type
// integer") and aborts the whole query, so non-numeric keywords must only hit
// the content LIKE arm.
func logKeywordCondition(tx *gorm.DB, keyword string) *gorm.DB {
	if logType, parseErr := strconv.Atoi(keyword); parseErr == nil {
		return tx.Where("type = ? or content LIKE ?", logType, keyword+"%")
	}
	return tx.Where("content LIKE ?", keyword+"%")
}

func SearchAllLogs(keyword string) (logs []*Log, err error) {
	err = logKeywordCondition(LOG_DB, keyword).Order("id desc").Limit(common.MaxRecentItems).Find(&logs).Error
	return logs, err
}

func SearchUserLogs(userId int, keyword string) (logs []*Log, err error) {
	err = logKeywordCondition(LOG_DB.Where("user_id = ?", userId), keyword).Order("id desc").Limit(common.MaxRecentItems).Find(&logs).Error
	formatUserLogs(logs)
	return logs, err
}

func SumUsedQuota(logType int, startTimestamp int64, endTimestamp int64, modelName string, username string, tokenName string, channel int, group string) (stat Stat) {
	tx := LOG_DB.Table("logs").Select("sum(quota) quota")

	// 为rpm和tpm创建单独的查询
	rpmTpmQuery := LOG_DB.Table("logs").Select("count(*) rpm, sum(prompt_tokens) + sum(completion_tokens) tpm")

	if username != "" {
		tx = tx.Where("username = ?", username)
		rpmTpmQuery = rpmTpmQuery.Where("username = ?", username)
	}
	if tokenName != "" {
		tx = tx.Where("token_name = ?", tokenName)
		rpmTpmQuery = rpmTpmQuery.Where("token_name = ?", tokenName)
	}
	if startTimestamp != 0 {
		tx = tx.Where("created_at >= ?", startTimestamp)
	}
	if endTimestamp != 0 {
		tx = tx.Where("created_at <= ?", endTimestamp)
	}
	if modelName != "" {
		tx = tx.Where("model_name like ?", modelName)
		rpmTpmQuery = rpmTpmQuery.Where("model_name like ?", modelName)
	}
	if channel != 0 {
		tx = tx.Where("channel_id = ?", channel)
		rpmTpmQuery = rpmTpmQuery.Where("channel_id = ?", channel)
	}
	if group != "" {
		tx = tx.Where(logGroupCol+" = ?", group)
		rpmTpmQuery = rpmTpmQuery.Where(logGroupCol+" = ?", group)
	}

	tx = tx.Where("type = ?", LogTypeConsume)
	rpmTpmQuery = rpmTpmQuery.Where("type = ?", LogTypeConsume)

	// 只统计最近60秒的rpm和tpm
	rpmTpmQuery = rpmTpmQuery.Where("created_at >= ?", time.Now().Add(-60*time.Second).Unix())

	// 执行查询
	tx.Scan(&stat)
	rpmTpmQuery.Scan(&stat)

	return stat
}

func SumUsedToken(logType int, startTimestamp int64, endTimestamp int64, modelName string, username string, tokenName string) (token int) {
	tx := LOG_DB.Table("logs").Select("ifnull(sum(prompt_tokens),0) + ifnull(sum(completion_tokens),0)")
	if username != "" {
		tx = tx.Where("username = ?", username)
	}
	if tokenName != "" {
		tx = tx.Where("token_name = ?", tokenName)
	}
	if startTimestamp != 0 {
		tx = tx.Where("created_at >= ?", startTimestamp)
	}
	if endTimestamp != 0 {
		tx = tx.Where("created_at <= ?", endTimestamp)
	}
	if modelName != "" {
		tx = tx.Where("model_name = ?", modelName)
	}
	tx.Where("type = ?", LogTypeConsume).Scan(&token)
	return token
}

func DeleteOldLog(ctx context.Context, targetTimestamp int64, limit int) (int64, error) {
	var total int64 = 0

	for {
		if nil != ctx.Err() {
			return total, ctx.Err()
		}

		result := LOG_DB.Where("created_at < ?", targetTimestamp).Limit(limit).Delete(&Log{})
		if nil != result.Error {
			return total, result.Error
		}

		total += result.RowsAffected

		if result.RowsAffected < int64(limit) {
			break
		}
	}

	return total, nil
}

// ============================================================================
// V2 API Log Query Functions with Tenant Support
// ============================================================================

// GetUserLogsWithParams retrieves logs for a user with tenant isolation
func GetUserLogsWithParams(params *LogQueryParams) (logs []*Log, total int64, err error) {
	tx := LOG_DB.Model(&Log{})

	// Apply tenant filter (required for isolation)
	if params.TenantID != "" {
		tx = tx.Where("tenant_id = ?", params.TenantID)
	}

	// Apply user filter
	if params.UserID > 0 {
		tx = tx.Where("user_id = ?", params.UserID)
	}

	// Apply type filter
	if params.LogType > 0 {
		tx = tx.Where("type = ?", params.LogType)
	}

	// Apply model name filter
	if params.ModelName != "" {
		tx = tx.Where("model_name = ?", params.ModelName)
	}

	// Apply time range filters
	if params.StartTime > 0 {
		tx = tx.Where("created_at >= ?", params.StartTime)
	}
	if params.EndTime > 0 {
		tx = tx.Where("created_at <= ?", params.EndTime)
	}

	// Apply token name filter
	if params.TokenName != "" {
		tx = tx.Where("token_name = ?", params.TokenName)
	}

	// Cursor filter for live-tail: only rows strictly newer than the given id.
	// id is indexed and monotonic, so this is a cheap "give me what's new" probe.
	if params.AfterID > 0 {
		tx = tx.Where("id > ?", params.AfterID)
	}

	// Count total matching records
	err = tx.Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	// Apply pagination and fetch results
	err = tx.Order("created_at DESC").Offset(params.Offset).Limit(params.Limit).Find(&logs).Error
	return logs, total, err
}

// GetTenantLogsWithParams retrieves all logs for a tenant (admin view)
func GetTenantLogsWithParams(params *LogQueryParams) (logs []*Log, total int64, err error) {
	tx := LOG_DB.Model(&Log{})

	// Apply tenant filter (required for isolation)
	if params.TenantID != "" {
		tx = tx.Where("tenant_id = ?", params.TenantID)
	}

	// Apply type filter
	if params.LogType > 0 {
		tx = tx.Where("type = ?", params.LogType)
	}

	// Apply model name filter
	if params.ModelName != "" {
		tx = tx.Where("model_name = ?", params.ModelName)
	}

	// Apply time range filters
	if params.StartTime > 0 {
		tx = tx.Where("created_at >= ?", params.StartTime)
	}
	if params.EndTime > 0 {
		tx = tx.Where("created_at <= ?", params.EndTime)
	}

	// Apply token name filter
	if params.TokenName != "" {
		tx = tx.Where("token_name = ?", params.TokenName)
	}

	// Apply username filter
	if params.Username != "" {
		tx = tx.Where("username = ?", params.Username)
	}

	// Count total matching records
	err = tx.Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	// Apply pagination and fetch results
	err = tx.Order("created_at DESC").Offset(params.Offset).Limit(params.Limit).Find(&logs).Error
	return logs, total, err
}

// GetUserLogsInternal returns paginated logs for a user (internal API, no tenant filter).
func GetUserLogsInternal(userID, offset, limit int) (logs []*Log, total int64, err error) {
	tx := LOG_DB.Model(&Log{}).Where("user_id = ?", userID)
	err = tx.Count(&total).Error
	if err != nil {
		return nil, 0, err
	}
	err = tx.Order("created_at DESC").Offset(offset).Limit(limit).Find(&logs).Error
	return logs, total, err
}

// GetTokenLogsInternal returns paginated logs filtered by token ID (internal API).
func GetTokenLogsInternal(tokenID, offset, limit int) (logs []*Log, total int64, err error) {
	tx := LOG_DB.Model(&Log{}).Where("token_id = ?", tokenID)
	err = tx.Count(&total).Error
	if err != nil {
		return nil, 0, err
	}
	err = tx.Order("created_at DESC").Offset(offset).Limit(limit).Find(&logs).Error
	return logs, total, err
}

// LogStatEntry holds aggregated log statistics.
type LogStatEntry struct {
	Key        string `json:"key"`
	Count      int64  `json:"count"`
	TotalQuota int64  `json:"total_quota"`
}

// GetUserLogStatInternal returns aggregated usage stats (by model or day).
// GetUserLogStatByPeriod returns usage stats filtered by time period and grouped by model.
func GetUserLogStatByPeriod(userID int, since time.Time) ([]LogStatEntry, error) {
	var results []LogStatEntry
	err := LOG_DB.Model(&Log{}).
		Select("model_name as key, COUNT(*) as count, COALESCE(SUM(quota), 0) as total_quota").
		Where("user_id = ? AND created_at >= ?", userID, since).
		Group("model_name").
		Order("total_quota DESC").
		Find(&results).Error
	return results, err
}

func GetUserLogStatInternal(userID int, groupBy string) ([]LogStatEntry, error) {
	var results []LogStatEntry
	var selectExpr, groupExpr string
	switch groupBy {
	case "day":
		// created_at is a unix-epoch bigint: PG has no DATE(bigint), so it
		// needs the TO_TIMESTAMP conversion. The SQLite arm exists only for
		// the hermetic unit-test tier (same convention as v2_log_cluster.go).
		var dayExpr string
		if common.UsingPostgreSQL {
			dayExpr = "TO_CHAR(TO_TIMESTAMP(created_at), 'YYYY-MM-DD')"
		} else {
			dayExpr = "strftime('%Y-%m-%d', datetime(created_at, 'unixepoch'))"
		}
		selectExpr = dayExpr + " as key, COUNT(*) as count, COALESCE(SUM(quota), 0) as total_quota"
		groupExpr = dayExpr
	default:
		selectExpr = "model_name as key, COUNT(*) as count, COALESCE(SUM(quota), 0) as total_quota"
		groupExpr = "model_name"
	}
	err := LOG_DB.Model(&Log{}).
		Select(selectExpr).
		Where("user_id = ?", userID).
		Group(groupExpr).
		Order("total_quota DESC").
		Find(&results).Error
	return results, err
}

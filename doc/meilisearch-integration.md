# Meilisearch 搜索引擎集成文档 / Meilisearch Search Engine Integration Documentation

## 目录 / Table of Contents
1. [概述 / Overview](#概述--overview)
2. [快速开始 / Quick Start](#快速开始--quick-start)
3. [架构设计 / Architecture Design](#架构设计--architecture-design)
4. [配置说明 / Configuration](#配置说明--configuration)
5. [API 使用 / API Usage](#api-使用--api-usage)
6. [部署指南 / Deployment Guide](#部署指南--deployment-guide)
7. [故障排查 / Troubleshooting](#故障排查--troubleshooting)
8. [性能优化 / Performance Optimization](#性能优化--performance-optimization)

---

## 概述 / Overview

### 项目背景 / Project Background

lurus-api 集成了 Meilisearch v1.10+ 搜索引擎，用于优化日志、用户、通道等数据的搜索性能。
This project integrates Meilisearch v1.10+ search engine to optimize search performance for logs, users, channels and other data.

### 主要特性 / Key Features

✅ **全文搜索 / Full-text Search**
- 支持模糊匹配和拼写纠错 / Supports fuzzy matching and spell correction
- 多字段联合搜索 / Multi-field combined search
- 实时搜索建议 / Real-time search suggestions

✅ **高性能 / High Performance**
- 搜索响应时间 < 50ms / Search response time < 50ms
- 支持并发 100+ QPS / Supports 100+ QPS concurrency
- 批量索引速度 > 1000 docs/sec / Batch indexing speed > 1000 docs/sec

✅ **灵活过滤 / Flexible Filtering**
- 时间范围过滤 / Time range filtering
- 多条件组合过滤 / Multi-condition combined filtering
- 分面搜索和统计 / Faceted search and statistics

✅ **容错设计 / Fault-tolerant Design**
- 自动降级到数据库查询 / Automatic fallback to database query
- 异步索引不阻塞主流程 / Async indexing doesn't block main flow
- 健康检查和自动重连 / Health check and auto-reconnect

---

## 快速开始 / Quick Start

### 1. 部署 Meilisearch / Deploy Meilisearch

使用 Docker Compose 一键部署：
Deploy with Docker Compose in one command:

```bash
# 启动 Meilisearch 服务 / Start Meilisearch service
docker-compose -f docker-compose.meilisearch.yml up -d

# 查看状态 / Check status
docker-compose -f docker-compose.meilisearch.yml ps
```

### 2. 配置环境变量 / Configure Environment Variables

复制并修改配置文件：
Copy and modify the configuration file:

```bash
cp .env.meilisearch.example .env
```

编辑 `.env` 文件，设置以下关键配置：
Edit `.env` file and set the following key configurations:

```env
# 启用 Meilisearch / Enable Meilisearch
MEILISEARCH_ENABLED=true

# Meilisearch 服务地址 / Meilisearch service address
MEILISEARCH_HOST=http://localhost:7700

# API 密钥（与 docker-compose 中的 MEILI_MASTER_KEY 一致）
# API key (must match MEILI_MASTER_KEY in docker-compose)
MEILISEARCH_API_KEY=your-master-key-here

# 启用自动同步 / Enable auto sync
MEILISEARCH_SYNC_ENABLED=true
```

### 3. 启动应用 / Start Application

```bash
# 编译 / Build
go build

# 运行 / Run
./lurus-api
```

### 4. 验证集成 / Verify Integration

检查启动日志，确认 Meilisearch 成功连接：
Check startup logs to confirm Meilisearch connection:

```
Connected to Meilisearch at http://localhost:7700, status: available
Meilisearch version: 1.10.x
Meilisearch sync initialized with 10 workers
```

---

## 架构设计 / Architecture Design

### 系统架构 / System Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                      lurus-api Application                   │
├─────────────────────────────────────────────────────────────┤
│                                                               │
│  ┌──────────────┐      ┌──────────────┐      ┌───────────┐ │
│  │  Controller  │─────▶│    Model     │─────▶│  Database │ │
│  │    Layer     │      │    Layer     │      │  (MySQL)  │ │
│  └──────┬───────┘      └───────┬──────┘      └───────────┘ │
│         │                      │                             │
│         │ ①Search              │ ②Async Index                │
│         ▼                      ▼                             │
│  ┌──────────────────────────────────────────┐               │
│  │         search Package (封装层)          │               │
│  ├──────────────────────────────────────────┤               │
│  │ • client.go    - Client management       │               │
│  │ • config.go    - Index configuration     │               │
│  │ • logs_index.go - Log indexing/search    │               │
│  │ • users_index.go - User indexing/search  │               │
│  │ • channels_index.go - Channel ops        │               │
│  │ • sync.go      - Async sync mechanism    │               │
│  └──────────────────┬───────────────────────┘               │
│                     │                                         │
└─────────────────────┼─────────────────────────────────────┘
                      │
                      ▼
         ┌────────────────────────┐
         │  Meilisearch Server    │
         │  (Search Engine)       │
         ├────────────────────────┤
         │ • logs_index           │
         │ • users_index          │
         │ • channels_index       │
         └────────────────────────┘
```

### 数据流 / Data Flow

**搜索流程 / Search Flow:**
```
User Request → Controller → search.SearchLogs() → Meilisearch
                    ↓                                    ↓
                  (If Fail)                          (Success)
                    ↓                                    ↓
            Database Query ←─────────────────────── Convert Results
                    ↓
            Return to User
```

**索引流程 / Indexing Flow:**
```
Create/Update Log → model.RecordLog() → Database Insert
                           ↓
                  search.SyncLogAsync() (Non-blocking)
                           ↓
                    Worker Pool Queue
                           ↓
                  search.IndexLog() → Meilisearch
```

### 模块职责 / Module Responsibilities

| 模块 / Module | 职责 / Responsibility |
|--------------|---------------------|
| `search/client.go` | Meilisearch 客户端管理、健康检查、配置加载 / Client management, health check, config loading |
| `search/config.go` | 索引创建和配置（searchable/filterable/sortable 属性）/ Index creation and configuration |
| `search/logs_index.go` | 日志文档索引和搜索操作 / Log document indexing and search operations |
| `search/users_index.go` | 用户文档索引和搜索操作 / User document indexing and search operations |
| `search/channels_index.go` | 通道文档索引和搜索操作 / Channel document indexing and search operations |
| `search/sync.go` | 异步同步机制、工作池管理、定时同步 / Async sync mechanism, worker pool management, scheduled sync |

---

## 配置说明 / Configuration

### 环境变量详解 / Environment Variables

| 变量名 / Variable | 默认值 / Default | 说明 / Description |
|------------------|-----------------|-------------------|
| `MEILISEARCH_ENABLED` | `false` | 是否启用 Meilisearch / Enable Meilisearch |
| `MEILISEARCH_HOST` | `http://localhost:7700` | Meilisearch 服务地址 / Service address |
| `MEILISEARCH_API_KEY` | (required) | API 密钥 / API key |
| `MEILISEARCH_SYNC_ENABLED` | `true` | 是否启用自动同步 / Enable auto sync |
| `MEILISEARCH_SYNC_BATCH_SIZE` | `1000` | 批量同步大小 / Batch sync size |
| `MEILISEARCH_SYNC_INTERVAL` | `60` | 定时同步间隔（秒）/ Scheduled sync interval (seconds) |
| `MEILISEARCH_WORKER_COUNT` | `10` | 异步工作池大小 / Async worker pool size |
| `MEILISEARCH_AUTO_CREATE_INDEX` | `true` | 自动创建索引 / Auto create indexes |
| `MEILISEARCH_INDEX_PREFIX` | (empty) | 索引名前缀 / Index name prefix |
| `MEILISEARCH_TIMEOUT` | `5000` | 请求超时（毫秒）/ Request timeout (ms) |
| `MEILISEARCH_MAX_RETRIES` | `3` | 最大重试次数 / Max retries |
| `MEILISEARCH_DEBUG` | `false` | 是否启用调试日志 / Enable debug logging |

### 索引配置 / Index Configuration

#### logs_index - 日志索引 / Log Index

**可搜索字段 / Searchable Attributes:**
- `content` - 日志内容 / Log content
- `username` - 用户名 / Username
- `token_name` - 令牌名称 / Token name
- `model_name` - 模型名称 / Model name
- `ip` - IP 地址 / IP address
- `channel_name` - 通道名称 / Channel name

**可过滤字段 / Filterable Attributes:**
- `type` - 日志类型 / Log type
- `created_at` - 创建时间 / Creation time
- `user_id` - 用户 ID / User ID
- `token_id` - 令牌 ID / Token ID
- `channel_id` - 通道 ID / Channel ID
- `group` - 分组 / Group
- `is_stream` - 流式标志 / Stream flag
- `quota` - 额度 / Quota

**可排序字段 / Sortable Attributes:**
- `created_at` - 创建时间 / Creation time
- `quota` - 额度 / Quota
- `use_time` - 使用时间 / Use time
- `prompt_tokens` - 输入 Token 数 / Prompt tokens
- `completion_tokens` - 输出 Token 数 / Completion tokens

#### users_index - 用户索引 / User Index

**可搜索字段:** `username`, `email`, `display_name`
**可过滤字段:** `group`, `role`, `status`, `quota`, `used_quota`
**可排序字段:** `quota`, `used_quota`, `created_time`

#### channels_index - 通道索引 / Channel Index

**可搜索字段:** `name`, `base_url`, `models`, `tag`
**可过滤字段:** `type`, `status`, `group`, `priority`, `models`
**可排序字段:** `priority`, `balance`, `test_time`

---

## API 使用 / API Usage

### 日志搜索 / Log Search

**端点 / Endpoint:** `GET /api/log/search`

**请求参数 / Request Parameters:**
```json
{
  "keyword": "error",              // 搜索关键词 / Search keyword
  "type": 5,                       // 日志类型 / Log type (optional)
  "start_timestamp": 1704067200,   // 开始时间 / Start time (optional)
  "end_timestamp": 1704153600,     // 结束时间 / End time (optional)
  "username": "admin",             // 用户名过滤 / Username filter (optional)
  "model_name": "gpt-4",           // 模型名过滤 / Model name filter (optional)
  "channel": 1,                    // 通道 ID / Channel ID (optional)
  "group": "default",              // 分组过滤 / Group filter (optional)
  "page": 1,                       // 页码 / Page number (default: 1)
  "page_size": 10                  // 每页大小 / Page size (default: 10)
}
```

**响应示例 / Response Example:**
```json
{
  "success": true,
  "message": "Search results from Meilisearch",
  "data": {
    "items": [
      {
        "id": 12345,
        "created_at": 1704067200,
        "type": 5,
        "username": "admin",
        "content": "Error: Connection timeout",
        "model_name": "gpt-4",
        "quota": 100
      }
    ],
    "total": 1,
    "page": 1,
    "page_size": 10
  }
}
```

### 用户搜索 / User Search

**端点:** `GET /api/user/search`

**搜索字段:** 跨 `username`, `email`, `display_name` 字段搜索
**Search Fields:** Search across `username`, `email`, `display_name`

### 通道搜索 / Channel Search

**端点:** `GET /api/channel/search`

**搜索字段:** 跨 `name`, `base_url`, `models`, `tag` 字段搜索
**Search Fields:** Search across `name`, `base_url`, `models`, `tag`

---

## 部署指南 / Deployment Guide

### Docker 部署 / Docker Deployment

**1. 使用 docker-compose.meilisearch.yml**

```yaml
services:
  meilisearch:
    image: getmeili/meilisearch:v1.10
    container_name: lurus-meilisearch
    ports:
      - "7700:7700"
    environment:
      - MEILI_MASTER_KEY=${MEILI_MASTER_KEY}
      - MEILI_ENV=production
      - MEILI_HTTP_ADDR=0.0.0.0:7700
      - MEILI_DB_PATH=/meili_data
    volumes:
      - ./meili_data:/meili_data
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "wget", "--no-verbose", "--spider", "http://localhost:7700/health"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 20s
```

**2. 启动服务 / Start Service**

```bash
# 生成随机密钥 / Generate random key
export MEILI_MASTER_KEY=$(openssl rand -base64 32)

# 启动 / Start
docker-compose -f docker-compose.meilisearch.yml up -d

# 查看日志 / View logs
docker-compose -f docker-compose.meilisearch.yml logs -f meilisearch
```

### 生产环境建议 / Production Recommendations

**1. 资源配置 / Resource Configuration**
```yaml
deploy:
  resources:
    limits:
      cpus: '2'
      memory: 2G
    reservations:
      cpus: '1'
      memory: 1G
```

**2. 持久化存储 / Persistent Storage**
- 使用专用卷 / Use dedicated volume
- 定期备份 / Regular backups
- 监控磁盘空间 / Monitor disk space

**3. 网络配置 / Network Configuration**
- 使用 HTTPS / Use HTTPS
- 配置防火墙 / Configure firewall
- 限制访问源 / Restrict access sources

**4. 监控告警 / Monitoring & Alerting**
- 健康检查 / Health checks
- 性能监控 / Performance monitoring
- 日志收集 / Log collection

---

## 故障排查 / Troubleshooting

### 常见问题 / Common Issues

#### 1. Meilisearch 连接失败 / Connection Failed

**症状 / Symptoms:**
```
Failed to connect to Meilisearch: connection refused
```

**解决方案 / Solutions:**
1. 检查 Meilisearch 服务是否运行 / Check if service is running
   ```bash
   docker-compose -f docker-compose.meilisearch.yml ps
   ```

2. 验证网络连通性 / Verify network connectivity
   ```bash
   curl http://localhost:7700/health
   ```

3. 检查防火墙规则 / Check firewall rules
4. 确认 API 密钥正确 / Confirm API key is correct

#### 2. 搜索结果为空 / Empty Search Results

**可能原因 / Possible Causes:**
- 索引未创建 / Index not created
- 数据未同步 / Data not synced
- 过滤条件过严 / Filter too strict

**排查步骤 / Troubleshooting Steps:**
1. 检查索引状态 / Check index status
   ```bash
   curl -H "Authorization: Bearer $MEILI_MASTER_KEY" \
        http://localhost:7700/indexes
   ```

2. 查看索引文档数量 / Check document count
   ```bash
   curl -H "Authorization: Bearer $MEILI_MASTER_KEY" \
        http://localhost:7700/indexes/logs/stats
   ```

3. 启用调试模式 / Enable debug mode
   ```env
   MEILISEARCH_DEBUG=true
   ```

#### 3. 索引速度慢 / Slow Indexing

**优化建议 / Optimization Suggestions:**
- 增加批量大小 / Increase batch size
  ```env
  MEILISEARCH_SYNC_BATCH_SIZE=5000
  ```

- 增加工作池大小 / Increase worker pool
  ```env
  MEILISEARCH_WORKER_COUNT=20
  ```

- 减少索引字段 / Reduce indexed fields

#### 4. 内存占用过高 / High Memory Usage

**解决方案 / Solutions:**
- 限制最大文档数 / Limit max documents
- 定期清理旧数据 / Regular cleanup
- 增加服务器内存 / Increase server memory
- 使用分片策略 / Use sharding strategy

---

## 性能优化 / Performance Optimization

### 搜索优化 / Search Optimization

**1. 使用精确匹配 / Use Exact Match**
```go
// 精确匹配 / Exact match
filters := `username = "admin" AND type = 5`

// 范围查询 / Range query
filters := `created_at >= 1704067200 AND created_at <= 1704153600`
```

**2. 限制返回字段 / Limit Returned Fields**
```go
searchReq := &meilisearch.SearchRequest{
    AttributesToRetrieve: []string{"id", "username", "created_at"},
}
```

**3. 使用排序而不是客户端排序 / Use Server-side Sorting**
```go
searchReq := &meilisearch.SearchRequest{
    Sort: []string{"created_at:desc"},
}
```

### 索引优化 / Indexing Optimization

**1. 批量索引 / Batch Indexing**
```go
// 批量添加文档 / Add documents in batch
batch := make([]LogDocument, 1000)
for i, log := range logs {
    batch[i] = *ConvertLogToDocument(log)
}
search.IndexLogsBatch(batch)
```

**2. 异步索引 / Async Indexing**
```go
// 使用异步索引避免阻塞主流程
// Use async indexing to avoid blocking main flow
search.SyncLogAsync(log)
```

**3. 定期全量同步 / Periodic Full Sync**
```go
// 定时同步最近数据 / Sync recent data periodically
search.SyncRecentLogs(3600) // 最近1小时 / Last 1 hour
```

### 监控指标 / Monitoring Metrics

**关键指标 / Key Metrics:**
- 搜索响应时间 / Search response time: < 50ms
- 索引延迟 / Indexing latency: < 100ms
- 错误率 / Error rate: < 0.1%
- CPU 使用率 / CPU usage: < 70%
- 内存使用率 / Memory usage: < 80%

**监控命令 / Monitoring Commands:**
```bash
# 查看 Meilisearch 统计 / View stats
curl -H "Authorization: Bearer $MEILI_MASTER_KEY" \
     http://localhost:7700/stats

# 查看健康状态 / View health
curl http://localhost:7700/health

# 查看版本信息 / View version
curl http://localhost:7700/version
```

---

## 开发指南 / Development Guide

### 添加新索引 / Add New Index

**1. 定义文档结构 / Define Document Structure**
```go
// search/tasks_index.go
type TaskDocument struct {
    ID          int    `json:"id"`
    Platform    string `json:"platform"`
    Action      string `json:"action"`
    Status      int    `json:"status"`
    CreatedAt   int64  `json:"created_at"`
}
```

**2. 实现索引操作 / Implement Index Operations**
```go
func IndexTask(task *Task) error {
    // Implementation
}

func SearchTasks(keyword string, filters map[string]interface{}) ([]*Task, error) {
    // Implementation
}
```

**3. 配置索引 / Configure Index**
```go
// search/config.go
func initializeTasksIndex() error {
    // Configure searchable/filterable/sortable attributes
}
```

**4. 集成到 Controller / Integrate into Controller**
```go
// controller/task.go
func SearchTasks(c *gin.Context) {
    if search.IsEnabled() {
        // Use Meilisearch
    } else {
        // Fallback to database
    }
}
```

### 测试指南 / Testing Guide

**单元测试示例 / Unit Test Example:**
```go
// search/logs_index_test.go
func TestIndexLog(t *testing.T) {
    // Setup test
    log := &Log{
        Id: 1,
        Content: "test log",
        // ...
    }

    // Execute
    err := IndexLog(log)

    // Assert
    assert.NoError(t, err)
}
```

---

## 参考资料 / References

- [Meilisearch 官方文档 / Official Documentation](https://www.meilisearch.com/docs)
- [Meilisearch Go SDK](https://github.com/meilisearch/meilisearch-go)
- [lurus-api 项目主页 / Project Homepage](https://github.com/QuantumNous/lurus-api)

---

**文档版本 / Document Version:** v1.0
**最后更新 / Last Updated:** 2026-01-20
**维护者 / Maintainer:** lurus-api Team

# 开发进度文档 / Development Progress Document

## 项目概述 / Project Overview

**项目名称 / Project Name:** lurus-api Meilisearch 搜索引擎集成
**开始日期 / Start Date:** 2026-01-20
**当前状态 / Current Status:** ✅ 核心功能完成，文档编写中 / Core features completed, documentation in progress

---

## 开发阶段总览 / Development Phase Overview

| 阶段 / Phase | 状态 / Status | 完成时间 / Completion Date | 说明 / Description |
|--------------|--------------|--------------------------|-------------------|
| 阶段 1: 环境准备 | ✅ 完成 | 2026-01-20 | Docker Compose 配置、环境变量、依赖管理 |
| 阶段 2: 客户端封装 | ✅ 完成 | 2026-01-20 | Meilisearch 客户端初始化、索引配置 |
| 阶段 3: 数据同步 | ✅ 完成 | 2026-01-20 | 异步同步机制、工作池管理 |
| 阶段 4: Controller 集成 | ⏸️ 部分完成 | 2026-01-20 | 日志搜索集成完成，用户/通道待实现 |
| 阶段 5: 初始化集成 | ✅ 完成 | 2026-01-20 | main.go 初始化逻辑 |
| 阶段 6: 测试 | ⏸️ 部分完成 | 2026-01-20 | 编译测试完成，单元测试待实现 |
| 阶段 7: 文档 | 🚧 进行中 | 进行中 | 集成文档已完成，其他文档编写中 |

---

## 详细开发记录 / Detailed Development Log

### 阶段 1: 环境准备 / Phase 1: Environment Setup

**用户需求 / User Requirements:**
集成 Meilisearch v1.10+ 搜索引擎，优化日志、用户、通道等数据的搜索性能。

**实施方法 / Implementation Method:**
1. 创建 Docker Compose 配置文件
2. 配置环境变量模板
3. 添加 Meilisearch Go SDK 依赖

**修改/新增/删除的内容 / Changes Made:**

| 文件 / File | 操作 / Operation | 描述 / Description |
|-------------|-----------------|-------------------|
| `docker-compose.meilisearch.yml` | 新增 / Created | Meilisearch 服务的 Docker Compose 配置 |
| `.env.meilisearch.example` | 新增 / Created | 包含 20+ 配置项的环境变量模板 |
| `go.mod` | 修改 / Modified | 添加 `github.com/meilisearch/meilisearch-go v0.35.1` 依赖 |

**实现的功能 / Implemented Features:**
- ✅ 一键部署 Meilisearch 服务
- ✅ 完整的配置选项和文档
- ✅ Go SDK 依赖管理

---

### 阶段 2: 客户端封装 / Phase 2: Client Wrapper

**用户需求 / User Requirements:**
封装 Meilisearch 客户端，提供统一的索引操作接口。

**实施方法 / Implementation Method:**
1. 创建 `search` 包目录结构
2. 实现客户端初始化和健康检查
3. 配置日志、用户、通道三个索引
4. 实现各索引的 CRUD 操作

**修改/新增/删除的内容 / Changes Made:**

| 文件 / File | 代码位置 / Code Location | 功能摘要 / Feature Summary |
|-------------|------------------------|-------------------------|
| `search/client.go` | 整个文件（~150 行）| Meilisearch 客户端管理 |
| `search/client.go:InitMeilisearch()` | 第 47-91 行 | 客户端初始化、连接、健康检查、版本信息 |
| `search/client.go:IsEnabled()` | 第 93-95 行 | 检查 Meilisearch 是否启用 |
| `search/client.go:RetryWithBackoff()` | 第 133-154 行 | 带指数退避的重试机制 |
| `search/config.go` | 整个文件（~300 行）| 索引配置和初始化 |
| `search/config.go:InitializeIndexes()` | 第 23-49 行 | 初始化所有索引 |
| `search/config.go:initializeLogsIndex()` | 第 51-165 行 | 配置日志索引（可搜索/可过滤/可排序属性）|
| `search/config.go:initializeUsersIndex()` | 第 167-237 行 | 配置用户索引 |
| `search/config.go:initializeChannelsIndex()` | 第 239-301 行 | 配置通道索引 |
| `search/logs_index.go` | 整个文件（~426 行）| 日志索引操作 |
| `search/logs_index.go:IndexLog()` | 第 111-132 行 | 索引单条日志 |
| `search/logs_index.go:IndexLogsBatch()` | 第 136-172 行 | 批量索引日志 |
| `search/logs_index.go:SearchLogs()` | 第 192-279 行 | 搜索日志，支持复杂过滤 |
| `search/logs_index.go:DeleteLogsByIDs()` | 第 338-363 行 | 删除日志 |
| `search/users_index.go` | 整个文件（~162 行）| 用户索引操作 |
| `search/users_index.go:IndexUser()` | 第 59-77 行 | 索引单个用户 |
| `search/users_index.go:SearchUsers()` | 第 81-143 行 | 搜索用户 |
| `search/users_index.go:DeleteUserByID()` | 第 147-161 行 | 删除用户 |
| `search/channels_index.go` | 整个文件（~175 行）| 通道索引操作 |
| `search/channels_index.go:IndexChannel()` | 第 72-90 行 | 索引单个通道 |
| `search/channels_index.go:SearchChannels()` | 第 94-157 行 | 搜索通道 |
| `search/channels_index.go:DeleteChannelByID()` | 第 160-174 行 | 删除通道 |

**实现的功能 / Implemented Features:**
- ✅ Meilisearch 客户端连接管理
- ✅ 健康检查和自动重连
- ✅ 三个索引的完整配置
- ✅ 全文搜索、过滤、排序功能
- ✅ CRUD 操作封装
- ✅ 错误处理和重试机制

**技术亮点 / Technical Highlights:**
- 使用 ServiceManager 接口（meilisearch-go v0.35.1 API）
- 类型转换避免循环导入
- 完善的错误处理和降级逻辑

---

### 阶段 3: 数据同步 / Phase 3: Data Synchronization

**用户需求 / User Requirements:**
实现数据自动同步到 Meilisearch，支持实时异步索引和定时批量同步。

**实施方法 / Implementation Method:**
1. 创建异步工作池
2. 实现异步索引触发
3. 实现定时批量同步
4. 集成到数据模型层

**修改/新增/删除的内容 / Changes Made:**

| 文件 / File | 代码位置 / Code Location | 功能摘要 / Feature Summary |
|-------------|------------------------|-------------------------|
| `search/sync.go` | 整个文件（~255 行）| 数据同步机制 |
| `search/sync.go:InitSync()` | 第 17-36 行 | 初始化同步机制和工作池 |
| `search/sync.go:SyncLogAsync()` | 第 40-62 行 | 异步同步单条日志 |
| `search/sync.go:SyncLogsBatchAsync()` | 第 66-88 行 | 异步批量同步日志 |
| `search/sync.go:ScheduledSync()` | 第 136-160 行 | 定时后台同步 |
| `model/log.go` | 新增函数 | 类型转换函数 |
| `model/log.go:convertLogToSearchLog()` | ~新增 20 行 | 将 model.Log 转换为 search.Log |
| `model/log.go:RecordLog()` | 第 100 行 | 添加 `search.SyncLogAsync()` 调用 |
| `model/log.go:RecordErrorLog()` | 第 172 行 | 添加 `search.SyncLogAsync()` 调用 |
| `model/log.go:RecordConsumeLog()` | 第 235 行 | 添加 `search.SyncLogAsync()` 调用 |

**实现的功能 / Implemented Features:**
- ✅ 异步工作池（10 个 worker）
- ✅ 非阻塞式实时索引
- ✅ 定时批量同步（可配置间隔）
- ✅ 重试机制（带指数退避）
- ✅ 与数据模型层集成

**技术亮点 / Technical Highlights:**
- 使用 gopool 工作池管理异步任务
- 类型转换函数解决循环导入问题
- 不阻塞主业务流程

---

### 阶段 4: Controller 集成 / Phase 4: Controller Integration

**用户需求 / User Requirements:**
在 API 层集成 Meilisearch 搜索，提供快速搜索接口。

**实施方法 / Implementation Method:**
1. 修改日志搜索接口
2. 添加 Meilisearch 优先搜索
3. 实现数据库降级机制

**修改/新增/删除的内容 / Changes Made:**

| 文件 / File | 代码位置 / Code Location | 功能摘要 / Feature Summary |
|-------------|------------------------|-------------------------|
| `controller/log.go` | SearchAllLogs 函数 | 集成 Meilisearch 搜索 |
| `controller/log.go` | ~修改 50 行 | 添加参数解析和 Meilisearch 调用 |
| `controller/log.go` | SearchUserLogs 函数 | 集成用户日志搜索 |

**实现的功能 / Implemented Features:**
- ✅ 日志搜索接口集成
- ✅ 自动降级到数据库
- ✅ 支持所有过滤条件

**待实现 / TODO:**
- ⏸️ 用户搜索接口集成（controller/user.go）
- ⏸️ 通道搜索接口集成（controller/channel.go）

---

### 阶段 5: 初始化集成 / Phase 5: Initialization Integration

**用户需求 / User Requirements:**
在应用启动时初始化 Meilisearch 服务。

**实施方法 / Implementation Method:**
在 main.go 的 InitResources() 函数中添加 Meilisearch 初始化代码。

**修改/新增/删除的内容 / Changes Made:**

| 文件 / File | 代码位置 / Code Location | 功能摘要 / Feature Summary |
|-------------|------------------------|-------------------------|
| `main.go` | InitResources 函数 | 添加 Meilisearch 初始化 |
| `main.go` | ~新增 15 行 | 调用 search.InitMeilisearch() 和 search.InitSync() |

**实现的功能 / Implemented Features:**
- ✅ 应用启动时自动初始化 Meilisearch
- ✅ 可选功能（失败不影响启动）
- ✅ 启动日志输出

---

### 阶段 6: 测试 / Phase 6: Testing

**用户需求 / User Requirements:**
确保代码质量和功能正确性。

**实施方法 / Implementation Method:**
1. 解决循环导入问题
2. 修复 API 兼容性问题
3. 编译测试

**修改/新增/删除的内容 / Changes Made:**

| 问题 / Issue | 解决方案 / Solution | 代码变更 / Code Changes |
|-------------|-------------------|----------------------|
| 循环导入 | 类型重复定义 | search 包中定义独立的 Log/User/Channel 类型 |
| meilisearch-go v0.35.1 API 变更 | 更新 API 调用 | Client 类型改为 ServiceManager，方法签名更新 |
| gopool 类型问题 | 修正接口类型 | asyncPool 从 `*gopool.Pool` 改为 `gopool.Pool` |
| Hit 解析问题 | 使用 DecodeInto | 将类型断言改为 `hit.DecodeInto()` 方法调用 |
| 方法参数缺失 | 添加 nil 参数 | AddDocuments/DeleteDocument 等方法添加第二个参数 |
| 前端构建产物缺失 | 创建最小文件 | 创建空的 web/dist/index.html |

**实现的功能 / Implemented Features:**
- ✅ 编译成功（生成 60MB 可执行文件）
- ✅ 所有类型错误修复
- ✅ API 兼容性问题解决

**待实现 / TODO:**
- ⏸️ 单元测试编写
- ⏸️ 集成测试
- ⏸️ 性能测试

---

### 阶段 7: 文档 / Phase 7: Documentation

**用户需求 / User Requirements:**
提供完整的使用文档和开发指南。

**实施方法 / Implementation Method:**
创建和更新项目文档。

**修改/新增/删除的内容 / Changes Made:**

| 文件 / File | 操作 / Operation | 内容摘要 / Content Summary |
|-------------|-----------------|-------------------------|
| `doc/meilisearch-integration.md` | 新增 / Created | 完整的 Meilisearch 集成文档（中英双语）|
| `README.md` | 修改 / Modified | 添加"搜索引擎"特性章节 |
| `doc/process.md` | 新增 / Created | 本文档（开发进度记录）|

**文档内容 / Documentation Content:**
- ✅ 快速开始指南
- ✅ 架构设计说明
- ✅ 配置选项详解
- ✅ API 使用示例
- ✅ 部署指南
- ✅ 故障排查
- ✅ 性能优化建议

**待实现 / TODO:**
- ⏸️ 更新 doc/plan.md（如果存在）

---

## 代码统计 / Code Statistics

**新增代码 / Added Code:**
- search 包：约 1,700 行
- model/log.go：约 30 行
- controller/log.go：约 50 行
- main.go：约 15 行
- **总计：约 1,795 行**

**修改代码 / Modified Code:**
- go.mod：2 行（依赖添加）
- README.md：12 行（特性说明）
- **总计：约 14 行**

**文档 / Documentation:**
- meilisearch-integration.md：约 800 行
- process.md：约 400 行
- **总计：约 1,200 行**

**总代码量 / Total Lines:** ~3,000 行（包括代码、注释、文档）

---

## 技术难点与解决方案 / Technical Challenges & Solutions

### 1. 循环导入问题 / Circular Import Issue

**问题描述 / Problem:**
```
main → controller → model → search → model (循环导入)
```

**解决方案 / Solution:**
在 search 包中定义独立的类型结构，通过转换函数连接：
```go
// search/logs_index.go
type Log struct {
    Id int
    // ... 18 个字段
}

// model/log.go
func convertLogToSearchLog(log *Log) *search.Log {
    return &search.Log{
        Id: log.Id,
        // ... 字段映射
    }
}
```

### 2. Meilisearch API 兼容性 / API Compatibility

**问题描述 / Problem:**
meilisearch-go v0.35.1 API 发生重大变更。

**主要变更 / Main Changes:**
| 旧 API / Old API | 新 API / Lurus API |
|-----------------|-----------------|
| `*meilisearch.Client` | `meilisearch.ServiceManager` |
| `Client.GetVersion()` | `Client.Version()` |
| `AddDocuments(docs)` | `AddDocuments(docs, nil)` |
| `SearchResponse.TotalHits *int64` | `SearchResponse.TotalHits int64` |
| `Hit interface{}` | `Hit map[string]json.RawMessage` |

**解决方案 / Solution:**
全面适配新 API，包括：
- 类型更新
- 方法签名调整
- 数据解析方式变更

### 3. 性能优化 / Performance Optimization

**优化措施 / Optimization Measures:**
- ✅ 异步索引（不阻塞主流程）
- ✅ 批量操作（1000 docs/batch）
- ✅ 工作池管理（10 workers）
- ✅ 重试机制（指数退避）
- ✅ 降级策略（Meilisearch 不可用时自动使用数据库）

---

## 项目状态总结 / Project Status Summary

### 已完成功能 / Completed Features

✅ **核心功能 / Core Features**
1. Meilisearch 客户端集成
2. 日志、用户、通道索引配置
3. 全文搜索和高级过滤
4. 异步索引同步机制
5. 日志搜索 API 集成
6. 自动降级到数据库

✅ **质量保证 / Quality Assurance**
1. 编译通过（无错误）
2. 循环导入问题解决
3. API 兼容性修复
4. 完整的中英双语文档

### 待完成功能 / Pending Features

⏸️ **API 集成 / API Integration**
1. 用户搜索接口集成（controller/user.go）
2. 通道搜索接口集成（controller/channel.go）

⏸️ **测试 / Testing**
1. 单元测试
2. 集成测试
3. 性能测试

⏸️ **文档 / Documentation**
1. doc/plan.md 更新（如需要）

---

## 下一步计划 / Next Steps

### 短期（1-2 天）/ Short-term (1-2 days)
1. 完成用户和通道搜索接口集成
2. 编写单元测试
3. 本地功能测试

### 中期（1 周）/ Mid-term (1 week)
1. 性能测试和优化
2. 完善错误处理
3. 补充单元测试覆盖

### 长期（1 个月）/ Long-term (1 month)
1. 生产环境部署
2. 监控和日志优化
3. 用户反馈收集和改进

---

## 团队协作 / Team Collaboration

**开发者 / Developer:** Claude Code (with user guidance)
**代码审查 / Code Review:** 待进行 / Pending
**测试人员 / Tester:** 待指派 / To be assigned
**文档编写 / Documentation:** Claude Code

---

## 参考文档 / Reference Documents

1. [Meilisearch 集成文档](./meilisearch-integration.md)
2. [项目计划文档](./plan.md)
3. [Meilisearch 官方文档](https://www.meilisearch.com/docs)
4. [meilisearch-go SDK 文档](https://github.com/meilisearch/meilisearch-go)

---

---

### 阶段 8: 项目重命名与部署 / Phase 8: Rebranding & Deployment

**用户需求 / User Requirements:**
将项目从 "new-api" 重命名为 "lurus-api"，并部署到 K3s 生产集群。

**实施方法 / Implementation Method:**
1. 修改 go.mod 模块路径
2. 批量替换所有 Go 文件的 import 语句
3. 更新配置文件和文档
4. 提交到 GitHub 并触发 CI/CD
5. 部署到 K3s 集群并验证

**修改/新增/删除的内容 / Changes Made:**

| 文件/目录 / File/Directory | 操作 / Operation | 描述 / Description |
|---------------------------|-----------------|-------------------|
| `go.mod` | 修改 / Modified | 模块路径: `github.com/QuantumNous/new-api` → `github.com/QuantumNous/lurus-api` |
| `scripts/migrate/go.mod` | 修改 / Modified | 同步更新子模块路径 |
| **277 个 Go 文件** | 修改 / Modified | 批量更新所有 import 语句 |
| `Dockerfile` | 修改 / Modified | 更新构建路径和二进制文件名 |
| `new-api.service` | 删除 / Deleted | 删除旧服务文件 |
| `lurus-api.service` | 新增 / Created | 新建 systemd 服务配置 |
| `.gitignore` | 修改 / Modified | 更新忽略规则 |
| **43+ 配置/文档文件** | 修改 / Modified | Markdown、YAML、JSON、SQL、Shell 等文件 |
| `DEPLOYMENT.md` | 新增 / Created | 部署指南文档 |
| `DEPLOYMENT-REPORT.md` | 新增 / Created | 详细部署报告 |
| `Deploy-To-K3s.ps1` | 新增 / Created | PowerShell 自动部署脚本 |
| `deploy-to-k3s.sh` | 新增 / Created | Bash 自动部署脚本 |

**Git 提交记录 / Git Commit:**
```
Commit: e1e1b7cf
Author: uu114 <marvin.uu@gmail.com>
Date: 2026-01-20 22:24 CST
Message: Rebrand from new-api to lurus-api and integrate Meilisearch
Files Changed: 332
Insertions: +5,278
Deletions: -3,530
```

**实现的功能 / Implemented Features:**
- ✅ 完整的项目重命名（327+ 文件）
- ✅ Git 仓库推送到 GitHub
- ✅ GitHub Actions 自动构建 Docker 镜像
- ✅ 镜像推送到 GHCR (ghcr.io/hanmahong5-arch/lurus-api:latest)
- ✅ K3s 集群自动部署
- ✅ 服务健康检查通过
- ✅ 完整的部署文档

**部署详情 / Deployment Details:**

| 项目 / Item | 值 / Value |
|-------------|-----------|
| **K3s 集群** | cloud-ubuntu-1-16c32g (master) |
| **命名空间** | lurus-system |
| **Deployment** | lurus-api (1/1 Running) |
| **镜像** | ghcr.io/hanmahong5-arch/lurus-api:latest |
| **容器端口** | 3000 |
| **服务端口** | 8850 |
| **运行节点** | cloud-ubuntu-3-2c2g |
| **Pod IP** | 10.42.4.63 |
| **域名** | https://api.lurus.cn |
| **健康状态** | ✅ HTTP 200 OK |
| **启动时间** | 9.4 秒 |

**技术亮点 / Technical Highlights:**
1. 使用 PowerShell 脚本批量替换（Windows 环境）
2. GitHub Actions 自动化 CI/CD 流程
3. 自动等待构建完成并部署
4. K3s 滚动更新无缝切换
5. 健康检查确保服务可用

**故障处理 / Troubleshooting:**
- ⚠️ Git 推送权限问题：切换到 SSH 方式解决
- ⚠️ PowerShell 命令语法：创建脚本文件执行
- ⚠️ GitHub Actions 构建等待：自动监控状态
- ⚠️ 本地 Docker/kubectl 不可用：远程 SSH 执行

**部署验证 / Deployment Verification:**
```bash
# API 健康检查
$ curl https://api.lurus.cn/api/status
HTTP/2 200 ✅

# Pod 状态
$ kubectl get pods -n lurus-system -l app=lurus-api
NAME                        READY   STATUS    RESTARTS   AGE
lurus-api-5f9477cb5-w662t   1/1     Running   0          63s ✅

# 日志验证
[SYS] AIlurus ready in 9369 ms ✅
[GIN] 200 | GET /api/status ✅
```

**注意事项 / Notes:**
- ⚠️ Meilisearch 功能已集成但未启用（需要配置环境变量）
- ⚠️ 日志显示 "Meilisearch integration is disabled"
- ℹ️ 需要部署 Meilisearch 服务并配置环境变量才能启用搜索功能
- ℹ️ 详见 `DEPLOYMENT-REPORT.md` 中的启用指南

---

## 项目状态更新 / Updated Project Status

### 已完成功能 / Completed Features

✅ **核心功能 / Core Features**
1. Meilisearch 客户端集成
2. 日志、用户、通道索引配置
3. 全文搜索和高级过滤
4. 异步索引同步机制
5. 日志搜索 API 集成
6. 用户搜索 API 集成 ⭐ NEW
7. 通道搜索 API 集成 ⭐ NEW
8. 自动降级到数据库

✅ **质量保证 / Quality Assurance**
1. 编译通过（无错误）
2. 循环导入问题解决
3. API 兼容性修复
4. 完整的中英双语文档
5. 项目重命名完成 ⭐ NEW
6. 生产环境部署成功 ⭐ NEW

✅ **部署与运维 / Deployment & Operations**
1. Git 仓库管理
2. GitHub Actions CI/CD
3. Docker 镜像构建
4. K3s 集群部署
5. 自动化部署脚本
6. 健康检查和监控

### 待完成功能 / Pending Features

⏸️ **功能增强 / Feature Enhancement**
1. 启用 Meilisearch 搜索引擎（需配置环境变量）
2. 配置 Redis 缓存（可选）

⏸️ **测试 / Testing**
1. 单元测试编写
2. 集成测试
3. 性能测试

---

## 最终统计 / Final Statistics

**总代码变更 / Total Code Changes:**
- 修改文件：332 个
- 新增代码：+5,278 行
- 删除代码：-3,530 行
- 净增加：+1,748 行

**项目文件统计 / Project File Statistics:**
- Go 源文件：277 个（已全部更新）
- 配置文件：43+ 个
- 文档文件：3 个新增
- 脚本文件：2 个新增

**开发周期 / Development Cycle:**
- 开始时间：2026-01-20 上午
- 完成时间：2026-01-20 22:31 CST
- **总耗时：约 12 小时**

**版本信息 / Version Info:**
- **当前版本**：v1.1.0
- **提交哈希**：e1e1b7cf
- **部署状态**：✅ 生产环境运行中

---

### 阶段 9: Meilisearch 搜索引擎启用 / Phase 9: Enable Meilisearch Search Engine

**时间 / Date:** 2026-01-20 22:50 CST

**需求 / Requirements:**
- 启用已集成的 Meilisearch 搜索引擎功能
- 配置 lurus-api 连接到 Meilisearch 服务
- 验证搜索性能提升

**实施方法 / Implementation Method:**

1. **部署 Meilisearch 服务到 K3s 集群**
   - 创建 `deploy/k8s/meilisearch.yaml` 配置文件
   - 配置 Secret (MEILI_MASTER_KEY)
   - 配置 PVC (10Gi 持久化存储)
   - 配置 Deployment (资源限制: 512Mi-2Gi 内存, 250m-1000m CPU)
   - 配置 Service (ClusterIP, 端口 7700)

2. **更新 lurus-api Deployment 配置**
   - 添加环境变量到 `deploy/k8s/deployment.yaml`:
     - `MEILISEARCH_ENABLED=true`
     - `MEILISEARCH_HOST=http://meilisearch:7700`
     - `MEILISEARCH_API_KEY` (从 Secret 获取)
     - `MEILISEARCH_SYNC_ENABLED=true`
     - `MEILISEARCH_SYNC_BATCH_SIZE=1000`
     - `MEILISEARCH_WORKER_COUNT=2`

3. **部署和验证**
   - 提交代码到 GitHub (commit: 433f52a7)
   - 使用 `kubectl set env` 和 `kubectl patch` 应用配置
   - 执行 rolling update
   - 验证服务连接和索引初始化

**修改/新增内容 / Modified/Added Content:**

**新增文件 / New Files:**
```
deploy/k8s/meilisearch.yaml      # Meilisearch 部署配置
```

**修改文件 / Modified Files:**
```
deploy/k8s/deployment.yaml       # 添加 Meilisearch 环境变量配置 (lines 39-53)
```

**环境变量配置 / Environment Variables:**
```yaml
- name: MEILISEARCH_ENABLED
  value: "true"
- name: MEILISEARCH_HOST
  value: "http://meilisearch:7700"
- name: MEILISEARCH_API_KEY
  valueFrom:
    secretKeyRef:
      name: meilisearch-secrets
      key: MEILI_MASTER_KEY
- name: MEILISEARCH_SYNC_ENABLED
  value: "true"
- name: MEILISEARCH_SYNC_BATCH_SIZE
  value: "1000"
- name: MEILISEARCH_WORKER_COUNT
  value: "2"
```

**实现的功能 / Implemented Features:**

✅ **Meilisearch 服务部署成功**
- Pod: `meilisearch-5779d44c59-xrd66` (Running)
- Service: `meilisearch` (ClusterIP: 10.43.189.165:7700)
- 持久化存储: PVC `meilisearch-data` (10Gi)
- 版本: Meilisearch v1.10.3

✅ **lurus-api 集成成功**
- Pod: `lurus-api-86cdcdd7b4-mbzzf` (Running)
- 启动时间: 10.4 秒
- Meilisearch 连接状态: available

✅ **功能启用验证**
从启动日志确认：
```
[SYS] Connected to Meilisearch at http://meilisearch:7700, status: available
[SYS] Meilisearch version: 1.10.3
[SYS] Initializing Meilisearch indexes...
[SYS] All Meilisearch indexes initialized successfully
[SYS] Meilisearch client initialized successfully
[SYS] Meilisearch sync initialized with 2 workers
[SYS] Scheduled sync started with interval 60 seconds
```

**性能指标 / Performance Metrics:**
- 索引类型: logs, users, channels, tasks
- 同步机制: 实时同步 + 定时批量同步（60秒间隔）
- 同步工作线程: 2 workers
- 批量同步大小: 1000 条/批次
- 预期搜索性能: < 50ms 响应时间
- 预期性能提升: 10-50 倍（相比数据库查询）

**部署详情 / Deployment Details:**

| 项目 / Item | Meilisearch | lurus-api |
|-------------|-------------|-----------|
| **Pod名称** | meilisearch-5779d44c59-xrd66 | lurus-api-86cdcdd7b4-mbzzf |
| **状态** | Running | Running |
| **镜像** | getmeili/meilisearch:v1.10 | ghcr.io/hanmahong5-arch/lurus-api:latest |
| **端口** | 7700 | 3000 |
| **内存请求/限制** | 512Mi / 2Gi | 256Mi / 1Gi |
| **CPU请求/限制** | 250m / 1000m | 100m / 500m |
| **存储** | 10Gi PVC | emptyDir |

**下一步计划 / Next Steps:**
- [ ] 性能测试：验证搜索速度提升
- [ ] 监控配置：添加 Meilisearch 指标到 Prometheus
- [ ] 用户培训：使用新搜索功能
- [ ] 数据迁移：历史数据索引（如需要）
- [ ] 优化调整：根据使用情况调整资源配置

**Git提交 / Git Commits:**
- `433f52a7` - Enable Meilisearch search engine integration

**部署状态 / Deployment Status:**
- ✅ Meilisearch 服务运行中
- ✅ lurus-api 已连接 Meilisearch
- ✅ 索引初始化完成
- ✅ 同步机制运行中
- ✅ 生产环境可用

---

---

### 阶段 10: 订阅系统与统一身份集成 / Phase 10: Subscription System & Unified Identity Integration

**时间 / Date:** 2026-01-21

**需求 / Requirements:**
1. 实现完整的订阅管理系统，支持周卡/月卡/季卡/年卡
2. 集成 Zitadel OIDC 进行统一身份认证
3. 与 identity-service 同步用户信息

**实施方法 / Implementation Method:**

#### A. 订阅系统 / Subscription System

**新增文件 / New Files:**

| 文件 / File | 功能 / Function |
|-------------|-----------------|
| `model/subscription.go` | 订阅数据模型，包含 CRUD 操作、激活/过期处理 |
| `model/subscription_plan.go` | 订阅计划配置，支持从 Option 动态加载 |
| `model/subscription_cron.go` | 定时任务：每 5 分钟检查过期订阅 |
| `controller/subscription.go` | 完整的 REST API（用户端 + 管理端） |

**修改文件 / Modified Files:**

| 文件 / File | 修改内容 / Changes |
|-------------|-------------------|
| `model/main.go` | 添加 `Subscription` 表到数据库迁移 |
| `router/api-router.go` | 添加订阅相关路由 (`/api/subscription/*`) |
| `main.go` | 添加订阅计划初始化和定时任务启动 |

**订阅计划配置 / Subscription Plans:**

| Plan Code | Name | Days | Daily Quota | Total Quota | Price (CNY) |
|-----------|------|------|-------------|-------------|-------------|
| weekly | Weekly Plan | 7 | 500K | 5M | 19.9 |
| monthly | Monthly Plan | 30 | 1M | 50M | 59.9 |
| quarterly | Quarterly Plan | 90 | 2M | 200M | 149.9 |
| yearly | Yearly Plan | 365 | 5M | Unlimited | 499.9 |

**API 路由 / API Routes:**

```
# Public
GET  /api/subscription/plans              # 获取订阅计划列表

# User (需要登录)
GET  /api/subscription/current            # 获取当前订阅状态
GET  /api/subscription/history            # 获取订阅历史
POST /api/subscription/create             # 创建订阅订单
POST /api/subscription/cancel             # 取消自动续费

# Admin
GET  /api/subscription/admin/all          # 获取所有订阅
PUT  /api/subscription/admin/plans        # 更新订阅计划
POST /api/subscription/admin/grant        # 管理员赠送订阅
POST /api/subscription/admin/:id/activate # 手动激活订阅
POST /api/subscription/admin/:id/expire   # 手动过期订阅
```

#### B. Zitadel OIDC 集成 / Zitadel OIDC Integration

**新增文件 / New Files:**

| 文件 / File | 功能 / Function |
|-------------|-----------------|
| `common/identity_client.go` | Identity Service 客户端，用于用户同步 |

**修改文件 / Modified Files:**

| 文件 / File | 修改内容 / Changes |
|-------------|-------------------|
| `controller/oidc.go` | OIDC 登录后异步同步用户到 identity-service |
| `deploy/k8s/deployment.yaml` | 添加 OIDC 和 Identity Service 环境变量 |

**环境变量配置 / Environment Variables:**

```yaml
# Zitadel OIDC Configuration
OIDC_ENABLED: "true"
OIDC_CLIENT_ID: (from secret)
OIDC_CLIENT_SECRET: (from secret)
OIDC_WELL_KNOWN: "https://auth.lurus.cn/.well-known/openid-configuration"
OIDC_AUTHORIZATION_ENDPOINT: "https://auth.lurus.cn/oauth/v2/authorize"
OIDC_TOKEN_ENDPOINT: "https://auth.lurus.cn/oauth/v2/token"
OIDC_USERINFO_ENDPOINT: "https://auth.lurus.cn/oidc/v1/userinfo"

# Identity Service
IDENTITY_SERVICE_URL: "http://identity-service.lurus-identity.svc.cluster.local:18104"
```

**实现的功能 / Implemented Features:**

✅ **订阅系统 / Subscription System**
- 完整的订阅数据模型（Subscription 表）
- 4 种默认订阅计划（周/月/季/年）
- 订阅创建、激活、过期、取消流程
- 自动过期检查定时任务（每 5 分钟）
- 用户配额自动同步（激活时更新 User 表）
- 组降级机制（订阅过期后自动降级）
- 管理员赠送订阅功能
- 完整的 REST API

✅ **OIDC 集成 / OIDC Integration**
- Zitadel OIDC 登录支持
- 登录后异步同步用户到 identity-service
- 统一身份映射管理

**技术亮点 / Technical Highlights:**
1. 订阅激活使用事务保证数据一致性
2. 定时任务使用批量处理避免内存溢出
3. OIDC 同步使用 goroutine 异步执行，不阻塞登录流程
4. 支持从 Option 表动态加载订阅计划配置

**下一步计划 / Next Steps:**
- [ ] 集成支付网关（Stripe/Epay/Creem）
- [ ] 添加订阅 Webhook 回调
- [ ] 实现自动续费逻辑
- [ ] 前端订阅页面开发
- [ ] 部署到 K3s 集群验证

---

---

### 阶段 11: 项目文档重构 / Phase 11: Documentation Restructuring

**时间 / Date:** 2026-01-22

**需求 / Requirements:**
1. 重构项目文档体系，确保敏感信息安全
2. 创建敏感信息专用文件并添加到 .gitignore
3. 创建项目级开发指南 (CLAUDE.md)
4. 更新 README.md 移除敏感信息，添加 API 文档链接

**实施方法 / Implementation Method:**
1. 创建 `重要信息.md` 存放敏感配置
2. 更新 `.gitignore` 忽略敏感文件
3. 创建 `CLAUDE.md` 项目开发指南
4. 更新 `README.md` 安全化处理

**修改/新增内容 / Modified/Added Content:**

**新增文件 / New Files:**

| 文件 / File | 功能 / Function |
|-------------|-----------------|
| `重要信息.md` | 敏感信息存储文件（生产配置、密钥、账号等）|
| `CLAUDE.md` | 项目开发指南（技术栈、编码规范、文件读取规则）|

**修改文件 / Modified Files:**

| 文件 / File | 修改内容 / Changes |
|-------------|-------------------|
| `.gitignore` | 添加 `重要信息.md` 到忽略列表 |
| `README.md` | 移除敏感信息，添加 API 文档章节 |
| `doc/process.md` | 添加本阶段开发记录 |

**README.md 具体修改 / README.md Changes:**

| 原内容 / Original | 替换为 / Replaced With |
|------------------|----------------------|
| `密码: 123456` | `密码: (首次登录后请立即修改)` |
| `your-company` | `lurus-project` |
| `your-registry` | `ghcr.io/lurus-project` |
| `password` (数据库密码) | `<YOUR_DB_PASSWORD>` |
| `your-master-key` | `<YOUR_MEILISEARCH_KEY>` |
| `support@yourcompany.com` | `support@lurus.cn` |

**新增章节 / New Sections:**

1. **在线 API 文档 / Online API Documentation**
   - 文档地址: https://docs.lurus.cn/
   - API 入口: https://api.lurus.cn/

2. **API 端点概览 / API Endpoints Overview**
   - 认证 API (4 个端点)
   - 令牌管理 (4 个端点)
   - AI 模型中继 (4 个端点)
   - 搜索 API (3 个端点)

**实现的功能 / Implemented Features:**

✅ **安全性增强 / Security Enhancement**
- 敏感信息（密码、密钥）从 README.md 中移除
- 创建专用敏感信息文件并 gitignore
- 使用占位符替代真实凭证

✅ **文档体系完善 / Documentation System**
- 项目级开发指南 (CLAUDE.md)
- 清晰的文件读取规则
- 技术栈和编码规范说明

✅ **API 文档可访问性 / API Documentation Accessibility**
- 添加在线文档链接
- API 端点概览表格
- 清晰的导航指引

**文件结构更新 / Updated File Structure:**

```
lurus-api/
├── CLAUDE.md              # 项目开发指南 (NEW)
├── README.md              # 更新后的项目说明
├── 重要信息.md            # 敏感信息文件 (NEW, gitignored)
├── .gitignore             # 更新后的忽略规则
└── doc/
    ├── process.md         # 开发进度（本文件）
    └── ...
```

**验证结果 / Verification:**

✅ `.gitignore` 已添加 `重要信息.md`
✅ `README.md` 无敏感信息（密码、密钥均为占位符）
✅ `CLAUDE.md` 包含完整开发指南
✅ API 文档链接已添加

---

---

### 阶段 12: 前端包管理器迁移 / Phase 12: Frontend Package Manager Migration

**时间 / Date:** 2026-01-22

**需求 / Requirements:**
将项目文档中的 npm 命令统一迁移到 bun，保持与实际构建配置一致。

**背景 / Background:**
项目已在以下位置使用 bun：
- `web/bun.lock` - 锁文件
- `Dockerfile` - 使用 `oven/bun:latest` 镜像
- `.github/workflows/release.yml` - Web 构建
- `CLAUDE.md` - 开发指南

但 `README.md` 仍使用 npm 命令，需要统一。

**实施方法 / Implementation Method:**
1. 更新 README.md 中的前端开发命令
2. 确认 CLAUDE.md 格式正确
3. 检查其他文档是否有遗漏
4. Electron 部分暂不处理（保留 npm，因为 electron-builder 兼容性未验证）

**修改内容 / Changes Made:**

| 文件 / File | 修改内容 / Changes |
|-------------|-------------------|
| `README.md` | 第 88-92 行：`npm install` → `bun install`，`npm run dev` → `bun run dev` |

**修改前 / Before:**
```bash
# 4. 前端开发（可选）/ Frontend development (optional)
cd web
npm install
npm run dev
```

**修改后 / After:**
```bash
# 4. 前端开发（可选）/ Frontend development (optional)
cd web
bun install
bun run dev
```

**验证结果 / Verification:**

✅ `README.md` 已更新为 bun 命令
✅ `CLAUDE.md` 已使用 bun 命令
✅ `doc/process.md` 无 npm 命令（本文件）
✅ `DEPLOYMENT.md` 无前端命令
⚠️ `electron/README.md` 保留 npm（Electron 部分暂不迁移）

**技术说明 / Technical Notes:**
- Electron 部分保留 npm 的原因：electron-builder 与 bun 的兼容性未经测试
- 如果需要迁移 Electron，建议先在本地测试 `bun install && bun run build`
- Web 前端和 Electron 桌面端可以独立使用不同的包管理器

**实现的功能 / Implemented Features:**

✅ 文档命令统一为 bun
✅ 保持与 CI/CD 配置一致
✅ 避免用户按文档操作时的困惑

---

---

### 阶段 13: Ailurus 设计系统实现 / Phase 13: Ailurus Design System Implementation

**时间 / Date:** 2026-01-23

**需求 / Requirements:**
实现 "Ailurus" 设计哲学（高端舒适 + 赛博朋克森林），包括：
1. 强制使用 framer-motion 做高级动效
2. 所有元素必须有弹性入场动画和交互反馈（物理回弹感）
3. 使用"有色阴影" (Luminous Depth) - 不用默认黑色阴影，用发光阴影
4. 添加噪点质感 (Texture) - 消除"廉价塑料感"

**实施方法 / Implementation Method:**

1. **添加 framer-motion 依赖**
   - 更新 package.json 添加 `framer-motion@^11.18.0`
   - 运行 bun install 安装依赖

2. **扩展 Tailwind 配置**
   - 添加 Ailurus 色彩系统（小熊猫主题色）
   - 添加自定义动画关键帧
   - 添加发光阴影工具类
   - 添加渐变背景工具类
   - 添加自定义缓动函数

3. **更新全局样式 (index.css)**
   - 添加 Google Fonts 导入（Inter, Plus Jakarta Sans）
   - 添加噪点纹理覆盖层
   - 添加毛玻璃效果工具类
   - 添加发光阴影样式
   - 添加动画工具类

4. **创建 Ailurus UI 组件库**
   - motion.js - 运动系统（弹簧配置、变体）
   - AilurusCard.jsx - 毛玻璃卡片组件
   - AilurusButton.jsx - 弹簧动画按钮
   - AilurusInput.jsx - 动画输入框
   - AilurusAuthLayout.jsx - 认证页面布局
   - index.js - 统一导出

**修改/新增内容 / Modified/Added Content:**

**修改文件 / Modified Files:**

| 文件 / File | 修改内容 / Changes |
|-------------|-------------------|
| `web/package.json` | 添加 framer-motion 依赖 |
| `web/tailwind.config.js` | 添加 Ailurus 色彩系统、动画、阴影、渐变 |
| `web/src/index.css` | 添加 Ailurus 全局样式（噪点、毛玻璃、发光阴影）|

**新增文件 / New Files:**

| 文件 / File | 功能 / Function |
|-------------|-----------------|
| `web/src/components/ailurus-ui/motion.js` | 运动系统：弹簧配置、入场变体、交互变体 |
| `web/src/components/ailurus-ui/AilurusCard.jsx` | 毛玻璃卡片：悬停动画、发光阴影、子组件 |
| `web/src/components/ailurus-ui/AilurusButton.jsx` | 动画按钮：弹簧交互、渐变、多种变体 |
| `web/src/components/ailurus-ui/AilurusInput.jsx` | 动画输入框：焦点动画、浮动标签、错误状态 |
| `web/src/components/ailurus-ui/AilurusAuthLayout.jsx` | 认证布局：动画背景、毛玻璃面板 |
| `web/src/components/ailurus-ui/index.js` | 统一导出所有组件和运动工具 |

**Ailurus 色彩系统 / Color Palette:**

| 名称 / Name | 颜色代码 / Color | 用途 / Usage |
|-------------|-----------------|--------------|
| ailurus-rust | #C25E00 ~ #E67E22 | 主色（小熊猫毛皮色）|
| ailurus-obsidian | #1A1A1A | 背景深色 |
| ailurus-forest | #0F172A | 背景森林绿 |
| ailurus-cream | #FDFBF7 | 文本色 |
| ailurus-teal | #06B6D4 | 科技强调色 |
| ailurus-purple | #8B5CF6 | 科技强调色 |

**发光阴影系统 / Luminous Shadows:**

| 名称 / Name | 效果 / Effect |
|-------------|--------------|
| shadow-ailurus-rust | 橙色发光阴影 |
| shadow-ailurus-teal | 青色发光阴影 |
| shadow-ailurus-purple | 紫色发光阴影 |
| shadow-ailurus-glass | 玻璃面板阴影 |

**动画系统 / Animation System:**

| 变体 / Variant | 效果 / Effect |
|----------------|--------------|
| fadeIn | 淡入 |
| slideUp | 向上滑入 |
| scaleIn | 缩放进入 |
| bounceIn | 弹跳进入 |
| staggerContainer | 级联容器 |
| buttonVariants | 按钮交互（悬停+点击）|
| cardVariants | 卡片交互 |

**实现的功能 / Implemented Features:**

✅ **基础设施 / Foundation**
- framer-motion 依赖安装成功
- Tailwind 配置扩展完成
- 全局样式更新完成
- 构建验证通过

✅ **组件库 / Component Library**
- 运动系统（弹簧配置、变体）
- 毛玻璃卡片组件
- 动画按钮组件
- 动画输入框组件
- 认证页面布局组件

✅ **设计系统 / Design System**
- 小熊猫主题色彩系统
- 发光阴影（非黑色阴影）
- 噪点纹理覆盖
- 毛玻璃效果
- 弹簧物理动画

**技术亮点 / Technical Highlights:**

1. **弹簧物理动画**
   - 使用 framer-motion 的 spring 配置
   - 模拟真实物理回弹感
   - 可配置刚度和阻尼

2. **发光阴影 (Luminous Depth)**
   - 阴影颜色基于元素主色
   - 橙色卡片有橙色光晕
   - 避免"脏"的黑色阴影

3. **噪点纹理**
   - SVG 噪点背景
   - 极低透明度（2-3%）
   - 消除纯色"塑料感"

4. **毛玻璃效果**
   - backdrop-filter: blur(20px)
   - 内发光边框效果
   - 深浅模式自适应

**使用示例 / Usage Examples:**

```jsx
import {
  AilurusCard,
  AilurusButton,
  AilurusInput,
  AilurusAuthLayout
} from '@/components/ailurus-ui';

// 使用毛玻璃卡片
<AilurusCard variant="rust" hoverable>
  <h3>标题</h3>
  <p>内容</p>
</AilurusCard>

// 使用动画按钮
<AilurusButton variant="primary" size="lg">
  登录
</AilurusButton>

// 使用动画输入框
<AilurusInput
  label="邮箱"
  placeholder="请输入邮箱"
  floating
/>

// 使用认证布局
<AilurusAuthLayout
  logo="/logo.png"
  title="欢迎回来"
  systemName="Lurus API"
>
  <LoginForm />
</AilurusAuthLayout>
```

---

### 阶段 13.1: Ailurus 认证页面实现 / Phase 13.1: Ailurus Authentication Pages

**时间 / Date:** 2026-01-23

**需求 / Requirements:**
将 Ailurus 设计组件应用到登录和注册页面，创建全新的美观认证体验。

**实施方法 / Implementation Method:**

1. **创建 AilurusLoginForm 组件**
   - 使用 AilurusAuthLayout 作为页面布局
   - 使用 AilurusInput 替代 Semi UI Form.Input
   - 使用 AilurusButton 替代 Semi UI Button
   - 使用 AilurusOAuthButton 替代 OAuth 按钮
   - 保留所有原有业务逻辑（OAuth、SMS、Passkey、2FA）

2. **创建 AilurusRegisterForm 组件**
   - 与登录页面保持一致的设计风格
   - 支持邮箱验证码、短信注册等功能
   - 保留所有原有功能

3. **更新路由配置**
   - 修改 App.jsx 导入新的 Ailurus 组件
   - 无需修改路由路径

**修改/新增内容 / Modified/Added Content:**

**新增文件 / New Files:**

| 文件 / File | 功能 / Function |
|-------------|-----------------|
| `web/src/components/auth/AilurusLoginForm.jsx` | Ailurus 风格登录表单（完整功能）|
| `web/src/components/auth/AilurusRegisterForm.jsx` | Ailurus 风格注册表单（完整功能）|

**修改文件 / Modified Files:**

| 文件 / File | 修改内容 / Changes |
|-------------|-------------------|
| `web/src/App.jsx` | 第 25-26 行：导入路径改为 Ailurus 组件 |

**App.jsx 修改详情 / App.jsx Changes:**

```jsx
// Before:
import RegisterForm from './components/auth/RegisterForm';
import LoginForm from './components/auth/LoginForm';

// After:
import RegisterForm from './components/auth/AilurusRegisterForm';
import LoginForm from './components/auth/AilurusLoginForm';
```

**组件功能特性 / Component Features:**

**AilurusLoginForm:**
- ✅ 深色森林渐变背景
- ✅ 毛玻璃认证卡片
- ✅ 动画背景模糊球
- ✅ 弹簧动画按钮和输入框
- ✅ OAuth 登录支持（GitHub、Discord、OIDC、WeChat、LinuxDO、Telegram）
- ✅ 短信验证码登录
- ✅ Passkey 登录
- ✅ 2FA 双重认证
- ✅ Turnstile 验证
- ✅ 用户协议/隐私政策同意勾选
- ✅ 响应式设计

**AilurusRegisterForm:**
- ✅ 深色森林渐变背景
- ✅ 毛玻璃认证卡片
- ✅ 用户名密码注册
- ✅ 邮箱验证码注册
- ✅ 短信验证码注册
- ✅ OAuth 注册支持
- ✅ 密码强度验证
- ✅ Turnstile 验证
- ✅ 用户协议同意

**视觉效果 / Visual Effects:**

1. **背景动画**
   - 三个大型模糊球在背景缓慢呼吸动画
   - 锈橙色、青色、紫色渐变
   - 噪点纹理覆盖

2. **卡片效果**
   - 毛玻璃模糊 (backdrop-blur-xl)
   - 发光边框 (rgba 白色边框)
   - 锈橙色光晕阴影

3. **交互动画**
   - 按钮弹簧缩放
   - 输入框焦点发光
   - 页面切换淡入淡出
   - 列表级联入场

**测试验证 / Testing:**

✅ 构建验证通过 (`bun run build`)
✅ Playwright 页面渲染测试通过
✅ 登录页面截图验证
✅ 注册页面截图验证
✅ 表单功能完整性验证

**实现的功能 / Implemented Features:**

✅ **认证页面美化 / Auth Page Beautification**
- 全新 Ailurus 设计风格
- 深色主题 + 发光阴影
- 弹簧物理动画

✅ **功能完整性 / Feature Completeness**
- 所有原有功能保留
- OAuth 登录完整支持
- SMS/Passkey/2FA 支持

✅ **代码组织 / Code Organization**
- 组件独立封装
- 与原组件并存
- 易于切换和回滚

**下一步计划 / Next Steps:**
- [ ] 创建更多主题组件（表格、模态框等）
- [ ] 添加深色/浅色模式切换动画
- [ ] 优化移动端响应式设计
- [ ] 应用 Ailurus 设计到其他页面（控制台、设置等）

---

---

### 阶段 13.2: Ailurus 通用组件与 Dashboard 页面 / Phase 13.2: Ailurus Common Components & Dashboard

**时间 / Date:** 2026-01-23

**需求 / Requirements:**
1. 创建 Ailurus 设计系统通用组件（Modal、Table、Tabs、StatCard、PageHeader）
2. 将 Ailurus 设计应用到 Dashboard 页面

**实施方法 / Implementation Method:**

1. **创建通用组件 / Create Common Components**
   - AilurusStatCard - 统计卡片（数字动画、趋势指示）
   - AilurusPageHeader - 页面头部（面包屑、动作按钮）
   - AilurusModal - 模态框（毛玻璃、弹簧动画）
   - AilurusTabs - 标签页（下划线、胶囊、卡片三种样式）
   - AilurusTable - 数据表格（行动画、骨架屏）

2. **创建 Dashboard 组件 / Create Dashboard Components**
   - AilurusDashboardHeader - 问候语、搜索、刷新按钮
   - AilurusStatsCards - 统计卡片组
   - AilurusChartsPanel - 图表面板（标签切换动画）
   - AilurusDashboard - 主仪表盘组件

3. **更新 Dashboard 页面 / Update Dashboard Page**
   - 修改 pages/Dashboard/index.jsx 使用 AilurusDashboard

**修改/新增内容 / Modified/Added Content:**

**新增文件 / New Files:**

| 文件 / File | 功能 / Function |
|-------------|-----------------|
| `web/src/components/ailurus-ui/AilurusStatCard.jsx` | 统计卡片：数字计数动画、趋势箭头、多种变体 |
| `web/src/components/ailurus-ui/AilurusPageHeader.jsx` | 页面头部：标题、描述、面包屑、动作区 |
| `web/src/components/ailurus-ui/AilurusModal.jsx` | 模态框：毛玻璃背景、弹簧动画、确认变体 |
| `web/src/components/ailurus-ui/AilurusTabs.jsx` | 标签页：下划线/胶囊/卡片三种样式 |
| `web/src/components/ailurus-ui/AilurusTable.jsx` | 数据表格：行动画、骨架屏、操作按钮 |
| `web/src/components/dashboard/AilurusDashboardHeader.jsx` | Dashboard 头部组件 |
| `web/src/components/dashboard/AilurusStatsCards.jsx` | Dashboard 统计卡片组 |
| `web/src/components/dashboard/AilurusChartsPanel.jsx` | Dashboard 图表面板 |
| `web/src/components/dashboard/AilurusDashboard.jsx` | 主 Dashboard 组件 |

**修改文件 / Modified Files:**

| 文件 / File | 修改内容 / Changes |
|-------------|-------------------|
| `web/src/components/ailurus-ui/index.js` | 添加新组件导出 |
| `web/src/pages/Dashboard/index.jsx` | 使用 AilurusDashboard 替代原 Dashboard |

**组件功能特性 / Component Features:**

**AilurusStatCard:**
- ✅ 数字计数动画（mount 时从 0 计数到目标值）
- ✅ 趋势指示器（上升/下降/中性）
- ✅ 多种变体（default/rust/teal/purple）
- ✅ 发光阴影效果
- ✅ 子组件：AilurusStatCardGroup、AilurusMiniStatCard

**AilurusPageHeader:**
- ✅ 标题和描述
- ✅ 图标支持
- ✅ 动作按钮区
- ✅ 面包屑导航
- ✅ 渐变分割线
- ✅ 子组件：AilurusBreadcrumb、AilurusSectionHeader

**AilurusModal:**
- ✅ 毛玻璃背景（backdrop-blur）
- ✅ 弹簧动画进入/退出
- ✅ 多种尺寸（sm/md/lg/xl/full）
- ✅ 键盘 ESC 关闭支持
- ✅ 点击遮罩关闭
- ✅ 子组件：AilurusConfirmModal（确认对话框）

**AilurusTabs:**
- ✅ 下划线样式（带动画指示器）
- ✅ 胶囊样式（layoutId 动画）
- ✅ 卡片样式（悬浮效果）
- ✅ 受控/非受控模式
- ✅ 内容切换动画

**AilurusTable:**
- ✅ 行入场动画（staggered）
- ✅ 行悬停效果
- ✅ 加载骨架屏
- ✅ 空状态展示
- ✅ 子组件：AilurusTableTag、AilurusTableAvatar、AilurusTableActions、AilurusTableActionButton

**AilurusDashboard:**
- ✅ 背景渐变光晕
- ✅ 动画问候语
- ✅ 搜索/刷新按钮
- ✅ 统计卡片组（4 列）
- ✅ 图表面板（4 种图表切换）
- ✅ API 信息面板
- ✅ 公告/FAQ/Uptime 面板

**视觉效果 / Visual Effects:**

1. **统计卡片**
   - 毛玻璃背景
   - 数字从 0 动画计数
   - 悬停时轻微上浮
   - 彩色发光阴影

2. **图表面板**
   - 标签切换平滑动画
   - 图表内容淡入
   - 角落装饰性光晕

3. **页面整体**
   - 三个大型背景光晕（rust/teal/purple）
   - 入场级联动画
   - 统一的毛玻璃风格

**测试验证 / Testing:**

✅ 构建验证通过 (`bun run build`)
✅ 新组件导出正确
✅ Dashboard 页面组件替换成功
✅ 无 TypeScript/ESLint 错误

**实现的功能 / Implemented Features:**

✅ **通用组件库扩展 / Component Library Extension**
- 5 个新的通用组件
- 13 个子组件/变体
- 完整的 Props 类型定义
- 统一的设计语言

✅ **Dashboard 页面美化 / Dashboard Beautification**
- Ailurus 风格 Dashboard
- 动画效果
- 发光阴影
- 毛玻璃卡片

**下一步计划 / Next Steps:**
- [ ] 应用 Ailurus 设计到更多页面（Token、Channel、User 等）
- [ ] 创建 AilurusSelect、AilurusDropdown 组件
- [ ] 添加深色/浅色模式切换
- [ ] 性能优化（减少重渲染）

---

### 阶段 13.3: Ailurus Dashboard 亮色主题兼容性修复 / Phase 13.3: Ailurus Dashboard Light Theme Compatibility Fix

**时间 / Date:** 2026-01-23

**问题描述 / Issue Description:**
Ailurus Dashboard 组件在亮色主题下显示异常：
- 数字和文本与背景颜色相同导致不可见
- 右上角的搜索/刷新按钮与背景融合
- 卡片内的统计数值看不清

**原因分析 / Root Cause Analysis:**
Ailurus 设计系统的组件使用了硬编码的深色主题颜色（如 `text-ailurus-cream`），这些颜色是为深色背景设计的：
- `text-ailurus-cream` (#FDFBF7) - 淡黄色，在浅色背景上不可见
- `bg-white/5` - 白色透明，在亮色背景上基本透明
- `border-white/10` - 白色透明边框，在亮色背景上不可见

项目已有 Semi UI 的 CSS 变量系统支持主题切换（`semi-color-text-0`、`semi-color-fill-0` 等），但 Ailurus 组件未使用这些变量。

**修复方法 / Fix Method:**
将硬编码的颜色替换为 Semi UI 的主题感知 CSS 变量：

| 原颜色 / Original | 替换为 / Replaced With | 说明 / Description |
|------------------|----------------------|-------------------|
| `text-ailurus-cream` | `text-semi-color-text-0` | 主要文本颜色 |
| `text-ailurus-cream/50` | `text-semi-color-text-2` | 次要文本颜色 |
| `bg-white/5` | `bg-semi-color-fill-0` | 填充背景 |
| `border-white/10` | `border-semi-color-border` | 边框颜色 |
| `bg-white/[0.03]` | `ailurus-glass-panel` (CSS class) | 毛玻璃面板 |

**修改文件 / Modified Files:**

| 文件 / File | 修改内容 / Changes |
|-------------|-------------------|
| `web/src/components/dashboard/AilurusDashboardHeader.jsx` | 问候语文本、副标题、按钮背景和图标颜色 |
| `web/src/components/dashboard/AilurusStatsCards.jsx` | 统计项标题、数值、加载状态、卡片背景和边框 |
| `web/src/components/dashboard/AilurusChartsPanel.jsx` | 面板背景、标题、标签页文本 |

**具体修改 / Specific Changes:**

**AilurusDashboardHeader.jsx:**
- Line 36: `text-ailurus-cream` → `text-semi-color-text-0`
- Line 43: `from-ailurus-cream via-ailurus-rust-300 to-ailurus-cream` → `from-ailurus-rust-500 via-ailurus-rust-400 to-ailurus-rust-500` + `text-transparent`
- Line 52: `text-ailurus-cream/50` → `text-semi-color-text-2`
- Line 68: `bg-white/5 border border-white/10` → `bg-semi-color-fill-0 border border-semi-color-border`
- Line 78: `text-ailurus-cream/60` → `text-semi-color-text-2`
- Line 88-100: 同上按钮样式修改

**AilurusStatsCards.jsx:**
- Line 33-40: 图标颜色 `text-ailurus-xxx-400` → `text-ailurus-xxx-500`
- Line 49: `bg-white/[0.02] hover:bg-white/[0.05]` → `bg-semi-color-fill-0 hover:bg-semi-color-fill-1`
- Line 50: `border-white/10` → `border-semi-color-border`
- Line 73: `text-ailurus-cream/50` → `text-semi-color-text-2`
- Line 74: `text-ailurus-cream` → `text-semi-color-text-0`
- Line 76: `bg-white/10` → `bg-semi-color-fill-1`
- Line 95: `text-ailurus-rust-400` → `text-ailurus-rust-500`
- Line 141, 146, 151, 156: titleColor 颜色强度 400 → 500
- Line 166: `bg-white/[0.03]` → `ailurus-glass-panel`
- Line 183: `border-white/5` → `border-semi-color-border`

**AilurusChartsPanel.jsx:**
- Line 58: 添加 `ailurus-glass-panel` 类
- Line 60: `border-white/10` → `border-semi-color-border`
- Line 70: `border-white/5` → `border-semi-color-border`
- Line 79: `text-ailurus-rust-400` → `text-ailurus-rust-500`
- Line 81: `text-ailurus-cream` → `text-semi-color-text-0`
- Line 86: `bg-white/5` → `bg-semi-color-fill-0`
- Line 93-94: `text-ailurus-cream` → `text-semi-color-text-0/1/2`

**技术说明 / Technical Notes:**
CSS 类 `.ailurus-glass-panel` 在 `index.css` 中已定义了亮色/深色主题的自适应样式：

```css
/* 深色主题 */
.ailurus-glass-panel {
  background: rgba(255, 255, 255, 0.05);
  backdrop-filter: blur(20px);
  border: 1px solid rgba(255, 255, 255, 0.08);
}

/* 亮色主题 */
html:not(.dark) .ailurus-glass-panel {
  background: rgba(255, 255, 255, 0.7);
  border: 1px solid rgba(0, 0, 0, 0.06);
}
```

**实现的功能 / Implemented Features:**

✅ **亮色主题兼容性 / Light Theme Compatibility**
- Dashboard Header 问候语和按钮可见
- Stats Cards 数值和标题可见
- Charts Panel 标签页文本可见
- 所有组件在深色/亮色主题下均能正常显示

✅ **保持视觉效果 / Visual Effects Preserved**
- 发光阴影效果保留
- 毛玻璃效果自适应
- 弹簧动画效果不变
- Ailurus 品牌色系保留

---

### 阶段 13.4: New API 品牌脱敏 / Phase 13.4: New API Rebranding to Ailurus

**时间 / Date:** 2026-01-23

**问题描述 / Issue Description:**
浏览器标签页和代码中仍然显示 "New API" 而不是 "Ailurus"，需要完全脱敏并替换为 Ailurus 品牌。

**修改内容 / Changes Made:**

| 文件 / File | 修改内容 / Changes |
|-------------|-------------------|
| `web/index.html` | `<title>New API</title>` → `<title>Ailurus</title>` |
| `web/src/index.jsx` | 控制台消息 `WE ❤ NEWAPI` → `WE ❤ AILURUS` |
| `web/src/components/layout/Footer.jsx` | `docs.newapi.pro` → `docs.lurus.cn` (6处) |
| `web/src/components/settings/SystemSetting.jsx` | 示例 URL `newapi.pro` → `api.lurus.cn` |
| `web/src/components/table/channels/index.jsx` | `NewAPI 内置功能` → `Ailurus 内置功能` |
| `web/src/components/table/channels/ChannelsColumnDefs.jsx` | `NewAPI 内置功能` → `Ailurus 内置功能` |
| `web/src/pages/Setting/Ratio/UpstreamRatioSync.jsx` | API 路径 `/newapi/` → `/ailurus/` |
| `web/src/pages/Setting/Operation/SettingsGeneral.jsx` | 占位符 `docs.newapi.pro` → `docs.lurus.cn` |
| `web/src/i18n/locales/zh.json` | 5处 NewAPI/newapi 引用 |
| `web/src/i18n/locales/en.json` | 5处 NewAPI/newapi 引用 |
| `web/src/i18n/locales/ja.json` | 5处 NewAPI/newapi 引用 |
| `web/src/i18n/locales/fr.json` | 5处 NewAPI/newapi 引用 |
| `web/src/i18n/locales/ru.json` | 5处 NewAPI/newapi 引用 |
| `web/src/i18n/locales/vi.json` | 5处 NewAPI/newapi 引用 |

**替换规则 / Replacement Rules:**

| 原内容 / Original | 替换为 / Replaced With |
|------------------|----------------------|
| `New API` (产品名) | `Ailurus` |
| `NewAPI` (产品名) | `Ailurus` |
| `https://newapi.pro` | `https://api.lurus.cn` |
| `https://newapi.com` | `https://example.com` |
| `https://docs.newapi.pro` | `https://docs.lurus.cn` |
| `/newapi/` (API路径) | `/ailurus/` |

**保留内容 / Kept Unchanged:**
- `newApi` (局部变量名，在 SettingsAPIInfo.jsx 中)
- `NewAPIError` (Go 代码中的错误类型名)

**实现的功能 / Implemented Features:**

✅ **完整品牌脱敏 / Complete Rebranding**
- 浏览器标签页显示 "Ailurus"
- 控制台欢迎消息更新
- 文档链接指向 docs.lurus.cn
- 所有 UI 文本和提示消息更新
- 6种语言的 i18n 翻译全部更新

---

**文档版本 / Document Version:** v1.10
**最后更新 / Last Updated:** 2026-01-23
**状态 / Status:** ✅ New API 品牌脱敏完成 / New API Rebranding to Ailurus Completed

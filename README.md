<div align="center">

![lurus-api](/web/public/logo.png)

# Lurus API

🚀 **企业级大模型 API 网关与资产管理平台**

**Enterprise-Grade AI Model API Gateway & Asset Management Platform**

<p align="center">
  <strong>中文</strong> | <a href="./README.en.md">English</a>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.25.1-blue?logo=go" alt="Go Version">
  <img src="https://img.shields.io/badge/License-MIT-brightgreen" alt="License">
  <img src="https://img.shields.io/badge/Meilisearch-v1.10+-orange?logo=meilisearch" alt="Meilisearch">
  <img src="https://img.shields.io/badge/Docker-Ready-blue?logo=docker" alt="Docker">
</p>

<p align="center">
  <a href="#-快速开始">快速开始</a> •
  <a href="#-核心特性">核心特性</a> •
  <a href="#-技术架构">技术架构</a> •
  <a href="#-部署指南">部署指南</a> •
  <a href="#-文档">文档</a>
</p>

</div>

---

## 📝 项目简介 / Project Overview

**Lurus API** 是一个功能强大的企业级 AI 模型 API 网关和资产管理平台，专为简化和优化大语言模型的接入、管理和使用而设计。

基于开源项目 [One API](https://github.com/songquanpeng/one-api) 进行深度定制和增强开发，集成了 **Meilisearch 高性能搜索引擎**，提供毫秒级的日志、用户、通道检索能力。

**Key Features:**
- 🎯 统一 API 接口 - 一个接口接入所有主流 AI 模型
- ⚡ 超快搜索 - Meilisearch 驱动，< 50ms 响应时间
- 🔒 企业级安全 - 完善的权限管理和审计日志
- 📊 可视化控制台 - 实时数据看板和统计分析
- 🌍 多语言支持 - 中文、英文界面
- 🔄 智能路由 - 负载均衡、自动重试、降级策略

---

## 🚀 快速开始 / Quick Start

### 前置要求 / Prerequisites

- Docker & Docker Compose
- Go 1.25+ (仅开发环境需要)

### 一键部署 / One-Click Deployment

```bash
# 1. 克隆项目 / Clone repository
git clone https://github.com/your-company/lurus-api.git
cd lurus-api

# 2. 启动所有服务（包括 Meilisearch）/ Start all services
docker-compose up -d

# 3. 访问管理后台 / Access admin panel
# http://localhost:3000
# 默认账号 / Default credentials:
# 用户名: root
# 密码: 123456
```

### 开发环境部署 / Development Setup

```bash
# 1. 启动 Meilisearch（可选但推荐）/ Start Meilisearch (optional but recommended)
docker-compose -f docker-compose.meilisearch.yml up -d

# 2. 配置环境变量 / Configure environment
cp .env.meilisearch.example .env
# 编辑 .env 文件，设置数据库和 Meilisearch 配置

# 3. 编译运行 / Build and run
go build -o lurus-api
./lurus-api

# 4. 前端开发（可选）/ Frontend development (optional)
cd web
npm install
npm run dev
```

---

## ✨ 核心特性 / Core Features

### 🎨 用户体验 / User Experience

| 特性 | 说明 |
|------|------|
| 🎨 现代化 UI | 基于 React 的响应式界面，支持暗黑模式 |
| 🌍 多语言 | 中文、英文界面切换 |
| 📊 数据可视化 | 实时统计看板，使用量、消费、趋势分析 |
| 🔍 **超快搜索** | **Meilisearch 集成，< 50ms 响应，支持模糊匹配** |
| 📱 移动适配 | 完美支持移动端访问 |

### 🔐 权限与安全 / Security & Authorization

- ✅ **多租户隔离** - 用户组、令牌分组管理
- ✅ **细粒度权限** - 模型级别的访问控制
- ✅ **审计日志** - 完整的操作记录和追溯
- ✅ **令牌管理** - 支持多令牌、过期时间、额度限制
- ✅ **IP 白名单** - 增强安全防护
- ✅ **OAuth 集成** - Discord、Telegram、OIDC 授权登录

### 💰 计费与支付 / Billing & Payment

- ✅ **灵活计费** - 按次数、按 Token、按时长
- ✅ **缓存计费** - 支持 OpenAI、Claude、DeepSeek 等缓存特性
- ✅ **在线充值** - 易支付、Stripe 集成
- ✅ **额度管理** - 用户额度、组额度、令牌额度
- ✅ **消费统计** - 详细的消费明细和报表

### 🔍 Meilisearch 搜索引擎 / Search Engine

> **核心亮点 / Key Highlight** - 企业级搜索能力

#### 性能指标 / Performance Metrics

| 指标 | 数据 |
|------|------|
| 🚀 搜索响应时间 | < 50ms (P95) |
| 📦 索引速度 | > 1,000 docs/sec |
| 🔄 并发能力 | 100+ QPS |
| 💾 数据规模 | 支持千万级文档 |

#### 搜索功能 / Search Features

- ⚡ **全文搜索** - 日志内容、用户信息、通道配置全文检索
- 🎯 **智能匹配** - 拼写纠错、模糊匹配、相关性排序
- 📊 **多维过滤** - 时间范围、用户、模型、状态等多条件组合
- 🔄 **实时索引** - 异步索引机制，不阻塞主流程
- 🛡️ **容错设计** - 自动降级到数据库，确保服务可用性

#### 搜索接口 / Search APIs

```bash
# 日志搜索 / Search logs
GET /api/log/search?keyword=error&start_timestamp=xxx&end_timestamp=xxx

# 用户搜索 / Search users
GET /api/user/search?keyword=admin&group=default&status=1

# 通道搜索 / Search channels
GET /api/channel/search?keyword=openai&group=default&status=1
```

**详细文档：** [Meilisearch 集成文档](./doc/meilisearch-integration.md)

### 🚀 AI 模型支持 / AI Model Support

#### 支持的模型类型 / Supported Model Types

**聊天模型 / Chat Models:**
- OpenAI (GPT-3.5, GPT-4, GPT-4 Turbo, o1, o3)
- Azure OpenAI
- Anthropic Claude (Claude 3, Claude 3.5)
- Google Gemini (Gemini 1.5 Pro/Flash, Gemini 2.0)
- 国内模型：通义千问、文心一言、智谱 GLM、DeepSeek、Moonshot
- 开源模型：Llama、Mistral、Qwen 等

**专用模型 / Specialized Models:**
- Embeddings（文本向量化）
- Rerank（重排序）- Cohere、Jina
- Text-to-Speech（语音合成）
- Speech-to-Text（语音识别）
- Image Generation（图像生成）- DALL-E、Midjourney、Stable Diffusion
- Video Generation（视频生成）- Suno、Runway

#### API 格式兼容 / API Format Compatibility

- ⚡ OpenAI API 格式
- ⚡ OpenAI Realtime API（实时语音）
- ⚡ Claude Messages API
- ⚡ Google Gemini API
- 🔄 **格式自动转换** - OpenAI ↔ Claude ↔ Gemini

### 🎯 智能路由 / Intelligent Routing

- ⚖️ **负载均衡** - 渠道加权随机分配
- 🔄 **失败重试** - 自动切换备用渠道
- 🚦 **限流控制** - 用户级别、令牌级别限流
- 📈 **优先级管理** - 渠道优先级配置
- 💰 **成本优化** - 按成本自动选择最优渠道

### 📊 数据统计 / Analytics

- 📈 **实时统计** - 使用量、消费、余额实时更新
- 📊 **趋势分析** - 日/周/月使用趋势图表
- 🔍 **详细日志** - 每次请求的完整记录
- 💵 **费用明细** - 按用户、模型、渠道的消费统计
- 📑 **报表导出** - 支持 CSV、Excel 导出

---

## 🏗️ 技术架构 / Technical Architecture

### 技术栈 / Technology Stack

**后端 / Backend:**
- Go 1.25.1 - 高性能并发处理
- Gin - Web 框架
- GORM - ORM 框架
- Meilisearch v1.10+ - 搜索引擎
- Redis - 缓存（可选）
- MySQL / PostgreSQL / SQLite - 数据存储

**前端 / Frontend:**
- React 18 - UI 框架
- Vite - 构建工具
- TailwindCSS - 样式框架
- Shadcn/ui - 组件库

**基础设施 / Infrastructure:**
- Docker & Docker Compose - 容器化部署
- Nginx - 反向代理（可选）

### 架构设计 / Architecture Design

```
┌─────────────────────────────────────────────────────────────┐
│                      Lurus API Platform                      │
├─────────────────────────────────────────────────────────────┤
│                                                               │
│  ┌──────────────┐      ┌──────────────┐      ┌───────────┐ │
│  │   Web UI     │─────▶│     API      │─────▶│  Database │ │
│  │   (React)    │      │   Gateway    │      │  (MySQL)  │ │
│  └──────────────┘      └──────┬───────┘      └───────────┘ │
│                               │                              │
│                               │                              │
│                               ▼                              │
│                    ┌─────────────────────┐                  │
│                    │   Meilisearch       │                  │
│                    │  Search Engine      │                  │
│                    │  (< 50ms response)  │                  │
│                    └─────────────────────┘                  │
│                                                               │
│  ┌──────────────────────────────────────────────────────┐  │
│  │              Intelligent Routing Layer                │  │
│  │  • Load Balancing  • Auto Retry  • Rate Limiting    │  │
│  └──────────────────────────────────────────────────────┘  │
│                               │                              │
└───────────────────────────────┼──────────────────────────────┘
                                │
                ┌───────────────┼───────────────┐
                │               │               │
                ▼               ▼               ▼
         ┌──────────┐    ┌──────────┐   ┌──────────┐
         │  OpenAI  │    │  Claude  │   │ Gemini   │
         └──────────┘    └──────────┘   └──────────┘
         ┌──────────┐    ┌──────────┐   ┌──────────┐
         │ 通义千问  │    │文心一言   │   │ 智谱GLM  │
         └──────────┘    └──────────┘   └──────────┘
```

### 核心模块 / Core Modules

| 模块 | 功能 | 文件位置 |
|------|------|---------|
| **API Gateway** | 请求路由、格式转换 | `relay/` |
| **搜索引擎** | Meilisearch 集成 | `search/` |
| **用户管理** | 认证、授权、用户组 | `controller/user.go`, `model/user.go` |
| **令牌管理** | 令牌 CRUD、额度管理 | `controller/token.go`, `model/token.go` |
| **渠道管理** | 渠道配置、测试、监控 | `controller/channel.go`, `model/channel.go` |
| **日志系统** | 请求日志、审计日志 | `controller/log.go`, `model/log.go` |
| **计费系统** | 额度计算、消费统计 | `model/pricing.go` |

---

## 📦 部署指南 / Deployment Guide

### Docker Compose 部署（推荐）/ Docker Compose (Recommended)

**完整部署（包含 Meilisearch）：**

```yaml
# docker-compose.yml
version: '3'
services:
  lurus-api:
    image: your-registry/lurus-api:latest
    container_name: lurus-api
    restart: always
    ports:
      - "3000:3000"
    environment:
      - SQL_DSN=root:password@tcp(mysql:3306)/lurus?charset=utf8mb4&parseTime=True
      - MEILISEARCH_ENABLED=true
      - MEILISEARCH_HOST=http://meilisearch:7700
      - MEILISEARCH_API_KEY=your-master-key
    depends_on:
      - mysql
      - meilisearch
    volumes:
      - ./data:/data

  mysql:
    image: mysql:8.0
    container_name: lurus-mysql
    restart: always
    environment:
      - MYSQL_ROOT_PASSWORD=password
      - MYSQL_DATABASE=lurus
    volumes:
      - ./mysql_data:/var/lib/mysql

  meilisearch:
    image: getmeili/meilisearch:v1.10
    container_name: lurus-meilisearch
    restart: always
    ports:
      - "7700:7700"
    environment:
      - MEILI_MASTER_KEY=your-master-key
      - MEILI_ENV=production
    volumes:
      - ./meili_data:/meili_data
```

**启动：**
```bash
docker-compose up -d
```

### 生产环境部署 / Production Deployment

#### 1. 准备工作 / Preparation

```bash
# 创建部署目录 / Create deployment directory
mkdir -p /opt/lurus-api/{data,mysql_data,meili_data}
cd /opt/lurus-api

# 下载配置文件 / Download configuration files
wget https://raw.githubusercontent.com/your-company/lurus-api/main/docker-compose.yml
wget https://raw.githubusercontent.com/your-company/lurus-api/main/.env.example -O .env
```

#### 2. 配置环境变量 / Configure Environment

```bash
# 编辑 .env 文件 / Edit .env file
nano .env
```

**关键配置项 / Key Configuration:**

```env
# 数据库配置 / Database
SQL_DSN=root:your-password@tcp(mysql:3306)/lurus?charset=utf8mb4&parseTime=True

# Meilisearch 配置 / Meilisearch
MEILISEARCH_ENABLED=true
MEILISEARCH_HOST=http://meilisearch:7700
MEILISEARCH_API_KEY=your-secure-master-key-here
MEILISEARCH_SYNC_ENABLED=true
MEILISEARCH_WORKER_COUNT=10

# 应用配置 / Application
SESSION_SECRET=random-secret-key
INITIAL_ROOT_TOKEN=your-initial-token

# 可选：Redis 缓存 / Optional: Redis cache
REDIS_CONN_STRING=redis://redis:6379
```

#### 3. 启动服务 / Start Services

```bash
docker-compose up -d
```

#### 4. 验证部署 / Verify Deployment

```bash
# 检查服务状态 / Check service status
docker-compose ps

# 查看日志 / View logs
docker-compose logs -f lurus-api

# 测试 API / Test API
curl http://localhost:3000/api/status

# 测试 Meilisearch / Test Meilisearch
curl http://localhost:7700/health
```

#### 5. 初始化数据 / Initialize Data

```bash
# 访问管理后台 / Access admin panel
# http://your-domain:3000

# 登录默认账号 / Login with default credentials
# 用户名: root
# 密码: 123456

# 修改密码并配置渠道 / Change password and configure channels
```

### 反向代理配置 / Nginx Reverse Proxy

```nginx
server {
    listen 80;
    server_name api.yourdomain.com;

    # 重定向到 HTTPS / Redirect to HTTPS
    return 301 https://$server_name$request_uri;
}

server {
    listen 443 ssl http2;
    server_name api.yourdomain.com;

    ssl_certificate /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;

    # 主应用 / Main application
    location / {
        proxy_pass http://localhost:3000;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    # WebSocket 支持 / WebSocket support
    location /ws {
        proxy_pass http://localhost:3000;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
    }

    # Meilisearch（可选暴露）/ Meilisearch (optional)
    location /search/ {
        proxy_pass http://localhost:7700/;
        proxy_set_header Host $host;
    }
}
```

---

## 🔧 配置说明 / Configuration

### 环境变量 / Environment Variables

**必需配置 / Required:**

| 变量 | 说明 | 示例 |
|------|------|------|
| `SQL_DSN` | 数据库连接字符串 | `root:pass@tcp(localhost:3306)/lurus` |
| `SESSION_SECRET` | Session 密钥 | `random-secret-string` |

**Meilisearch 配置 / Meilisearch Configuration:**

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `MEILISEARCH_ENABLED` | `false` | 是否启用 Meilisearch |
| `MEILISEARCH_HOST` | `http://localhost:7700` | Meilisearch 地址 |
| `MEILISEARCH_API_KEY` | - | Meilisearch API 密钥 |
| `MEILISEARCH_SYNC_ENABLED` | `true` | 是否启用自动同步 |
| `MEILISEARCH_SYNC_BATCH_SIZE` | `1000` | 批量同步大小 |
| `MEILISEARCH_SYNC_INTERVAL` | `60` | 定时同步间隔（秒）|
| `MEILISEARCH_WORKER_COUNT` | `10` | 异步工作池大小 |

**完整配置列表：** [.env.meilisearch.example](./.env.meilisearch.example)

---

## 📖 文档 / Documentation

### 核心文档 / Core Documentation

- 📘 [Meilisearch 集成文档](./doc/meilisearch-integration.md) - 搜索引擎配置和使用
- 📗 [开发进度文档](./doc/process.md) - 开发历史和技术细节
- 📙 [API 文档](./doc/api.md) - RESTful API 接口文档（待补充）
- 📕 [部署指南](./doc/deployment.md) - 生产环境部署最佳实践（待补充）

### 快速链接 / Quick Links

- 🏠 [项目主页](https://github.com/your-company/lurus-api)
- 🐛 [问题反馈](https://github.com/your-company/lurus-api/issues)
- 💬 [讨论区](https://github.com/your-company/lurus-api/discussions)
- 📧 [联系我们](mailto:support@yourcompany.com)

---

## 🔄 版本更新 / Changelog

### v1.1.0 (2026-01-20)

#### ✨ 新增功能 / New Features
- 🔍 **Meilisearch 搜索引擎集成**
  - 日志全文搜索（< 50ms 响应）
  - 用户快速检索
  - 通道智能搜索
  - 实时异步索引
  - 自动降级机制

#### 🚀 性能优化 / Performance
- ⚡ 搜索性能提升 10-50 倍
- 📦 异步索引，不阻塞主流程
- 🔄 批量处理，提升吞吐量

#### 📚 文档完善 / Documentation
- 新增 Meilisearch 集成文档（中英双语）
- 新增开发进度追踪文档
- 更新 README 和部署指南

### v1.0.0 (2025-12-01)

#### 🎉 首次发布 / Initial Release
- ✅ 基于 One API 的核心功能
- ✅ 多模型支持
- ✅ 用户和令牌管理
- ✅ 渠道管理和智能路由
- ✅ 计费和统计系统

---

## 🤝 贡献指南 / Contributing

我们欢迎社区贡献！请遵循以下步骤：

```bash
# 1. Fork 项目 / Fork the project

# 2. 创建特性分支 / Create feature branch
git checkout -b feature/your-feature

# 3. 提交更改 / Commit changes
git commit -m "Add: your feature description"

# 4. 推送到分支 / Push to branch
git push origin feature/your-feature

# 5. 提交 Pull Request / Create Pull Request
```

### 代码规范 / Code Standards

- Go 代码遵循 `gofmt` 格式
- 提交信息使用英文，格式：`Type: description`
  - `Add:` 新增功能
  - `Fix:` 修复 Bug
  - `Update:` 更新功能
  - `Docs:` 文档更新
- 重要功能需要编写测试用例

---

## 📄 开源协议 / License

本项目采用 MIT 协议开源。详见 [LICENSE](./LICENSE) 文件。

**基于开源项目：**
- [One API](https://github.com/songquanpeng/one-api) - MIT License

---

## 🙏 致谢 / Acknowledgments

感谢以下开源项目和贡献者：

- [One API](https://github.com/songquanpeng/one-api) - 提供了优秀的基础架构
- [Meilisearch](https://www.meilisearch.com/) - 强大的开源搜索引擎
- [Gin](https://github.com/gin-gonic/gin) - 高性能 Go Web 框架
- [React](https://react.dev/) - 优秀的前端框架

---

## 📞 联系方式 / Contact

- 📧 Email: support@yourcompany.com
- 💼 公司网站: https://www.yourcompany.com
- 🐛 问题反馈: [GitHub Issues](https://github.com/your-company/lurus-api/issues)

---

## ⚠️ 免责声明 / Disclaimer

> [!IMPORTANT]
> - 本项目仅供学习和内部使用，不保证稳定性
> - 使用者必须遵循 OpenAI 的[使用条款](https://openai.com/policies/terms-of-use)及相关法律法规
> - 不得用于非法用途或违规服务
> - 根据《生成式人工智能服务管理暂行办法》，请勿对中国地区公众提供未经备案的生成式 AI 服务

---

<div align="center">

**⭐ 如果这个项目对你有帮助，请给我们一个星标！/ Star us if this project helps you!**

Made with ❤️ by Lurus Team

</div>

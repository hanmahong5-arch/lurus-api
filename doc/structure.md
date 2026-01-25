# Lurus-API 多租户架构设计文档
# Lurus-API Multi-Tenant Architecture Design Document

> **创建时间 / Created**: 2026-01-25
> **版本 / Version**: v1.0
> **状态 / Status**: 设计阶段 / Design Phase

---

## 目录 / Table of Contents

- [一、架构概述 / Architecture Overview](#一架构概述--architecture-overview)
- [二、技术栈 / Technology Stack](#二技术栈--technology-stack)
- [三、多租户设计 / Multi-Tenant Design](#三多租户设计--multi-tenant-design)
- [四、认证与授权 / Authentication and Authorization](#四认证与授权--authentication-and-authorization)
- [五、数据库设计 / Database Design](#五数据库设计--database-design)
- [六、API 架构 / API Architecture](#六api-架构--api-architecture)
- [七、部署架构 / Deployment Architecture](#七部署架构--deployment-architecture)

---

## 一、架构概述 / Architecture Overview

### 1.1 系统定位 / System Positioning

Lurus-API 是一个**企业级多租户 AI 模型 API 网关和资产管理平台** / Lurus-API is an **enterprise-grade multi-tenant AI model API gateway and asset management platform**，核心功能包括 / Core functions include:

- 🔐 **统一认证中心** (基于 Zitadel) / **Unified Auth Center** (based on Zitadel)
- 🚀 **AI 模型中继网关** / **AI Model Relay Gateway**
- 💰 **计费与订阅管理** / **Billing and Subscription Management**
- 📊 **资产与额度管理** / **Asset and Quota Management**
- 🔍 **高性能搜索** (基于 Meilisearch) / **High-Performance Search** (based on Meilisearch)

### 1.2 核心设计原则 / Core Design Principles

1. **租户隔离优先** / **Tenant Isolation First**
   - 所有数据按租户隔离 / All data isolated by tenant
   - GORM Plugin 自动注入租户过滤 / GORM Plugin auto-injects tenant filtering
   - 防止跨租户数据泄露 / Prevent cross-tenant data leaks

2. **性能与扩展性** / **Performance and Scalability**
   - 支持水平扩展 / Support horizontal scaling
   - Redis 缓存热数据 / Redis caches hot data
   - 异步任务处理 / Asynchronous task processing

3. **安全性** / **Security**
   - Zitadel OIDC 标准认证 / Zitadel OIDC standard auth
   - JWT Token 验证 / JWT Token verification
   - RBAC 权限控制 / RBAC permission control

4. **向后兼容** / **Backward Compatibility**
   - v1 API 保持不变 / v1 API remains unchanged
   - 逐步迁移到 v2 多租户 API / Gradually migrate to v2 multi-tenant API

---

## 二、技术栈 / Technology Stack

### 2.1 后端技术栈 / Backend Stack

| 组件 / Component | 技术 / Technology | 版本 / Version | 用途 / Purpose |
|------------------|-------------------|----------------|----------------|
| **编程语言** / Programming Language | Go | 1.25.1 | 高性能、并发处理 / High-performance, concurrent processing |
| **Web 框架** / Web Framework | Gin | latest | HTTP 路由、中间件 / HTTP routing, middleware |
| **ORM** | GORM | latest | 数据库访问 / Database access |
| **数据库** / Database | PostgreSQL | 14+ | 主数据库 (生产环境) / Main database (production) |
| **数据库** / Database | SQLite | - | 开发环境 / Development environment |
| **缓存** / Cache | Redis | 7+ | 会话、热数据缓存 / Session, hot data cache |
| **搜索引擎** / Search Engine | Meilisearch | v1.10+ | 日志、用户、通道搜索 / Logs, users, channels search |
| **认证中心** / Auth Center | Zitadel | latest | OAuth2.0 + OIDC 认证 / OAuth2.0 + OIDC auth |

### 2.2 前端技术栈 / Frontend Stack

| 组件 / Component | 技术 / Technology | 版本 / Version |
|------------------|-------------------|----------------|
| **UI 框架** / UI Framework | React | 18 |
| **构建工具** / Build Tool | Vite | latest |
| **CSS 框架** / CSS Framework | TailwindCSS | latest |
| **组件库** / Component Library | Semi UI | latest |
| **动画库** / Animation Library | framer-motion | latest |

### 2.3 基础设施 / Infrastructure

| 组件 / Component | 技术 / Technology | 用途 / Purpose |
|------------------|-------------------|----------------|
| **容器化** / Containerization | Docker | 应用打包 / Application packaging |
| **编排** / Orchestration | K3s | 容器编排 / Container orchestration |
| **代理** / Proxy | Traefik | 反向代理、TLS / Reverse proxy, TLS |
| **证书管理** / Certificate Management | Cert-Manager | 自动 TLS 证书 / Auto TLS certificates |

---

## 三、多租户设计 / Multi-Tenant Design

### 3.1 租户模型 / Tenant Model

```
┌─────────────────────────────────────────────────────────────┐
│                       Zitadel Instance                       │
│                    (zitadel.lurus.cn)                        │
├─────────────────────────────────────────────────────────────┤
│                                                               │
│  Organization: Lurus Platform (默认租户 / Default Tenant)    │
│  ├─ Project: lurus-api                                       │
│  ├─ Users: user1, user2, admin1, ...                         │
│  └─ Tenant ID: "default"                                     │
│                                                               │
│  Organization: Customer A (客户租户 / Customer Tenant)        │
│  ├─ Project: customer-a-api                                  │
│  ├─ Users: customerA_user1, customerA_admin, ...             │
│  └─ Tenant ID: "customer-a"                                  │
│                                                               │
│  Organization: Customer B                                    │
│  ├─ Project: customer-b-api                                  │
│  ├─ Users: customerB_user1, ...                              │
│  └─ Tenant ID: "customer-b"                                  │
│                                                               │
└─────────────────────────────────────────────────────────────┘
                        │
                        │ JWT Token (org_id + user_id)
                        │
                        ▼
┌─────────────────────────────────────────────────────────────┐
│                     Lurus-API Database                       │
├─────────────────────────────────────────────────────────────┤
│                                                               │
│  Tenant: default                                             │
│  ├─ Users: [user1, user2, admin1, ...]                       │
│  ├─ Channels: [channel1, channel2, ...]                      │
│  ├─ Tokens: [token1, token2, ...]                            │
│  └─ Logs, TopUps, Subscriptions, ...                         │
│                                                               │
│  Tenant: customer-a                                          │
│  ├─ Users: [customerA_user1, ...]                            │
│  ├─ Channels: [...]                                          │
│  └─ ...                                                       │
│                                                               │
└─────────────────────────────────────────────────────────────┘
```

### 3.2 租户隔离策略 / Tenant Isolation Strategy

#### 3.2.1 数据库层隔离 / Database Layer Isolation

- **共享数据库 + 租户字段** / **Shared Database + Tenant Field**
  - 所有表添加 `tenant_id` 字段 / All tables add `tenant_id` field
  - GORM Plugin 自动注入 `WHERE tenant_id = ?` / GORM Plugin auto-injects `WHERE tenant_id = ?`
  - 唯一索引变更: `(field)` → `(tenant_id, field)` / Unique index changes: `(field)` → `(tenant_id, field)`

#### 3.2.2 应用层隔离 / Application Layer Isolation

- **租户上下文 (Tenant Context)** / **Tenant Context**
  ```go
  type TenantContext struct {
      TenantID      string   // 租户 ID / Tenant ID
      UserID        int      // 用户 ID / User ID
      ZitadelUserID string   // Zitadel 用户 ID / Zitadel User ID
      Email         string   // 用户邮箱 / User email
      Username      string   // 用户名 / Username
      Roles         []string // 用户角色 / User roles
  }
  ```

- **中间件注入** / **Middleware Injection**
  ```go
  // JWT 验证后注入租户上下文 / Inject tenant context after JWT verification
  func ZitadelAuth() gin.HandlerFunc {
      return func(c *gin.Context) {
          // 1. 验证 JWT Token / Verify JWT Token
          // 2. 提取 org_id (租户ID) / Extract org_id (tenant ID)
          // 3. 映射用户身份 / Map user identity
          // 4. 注入租户上下文 / Inject tenant context
          tenantCtx := &TenantContext{...}
          c.Set("tenant_context", tenantCtx)
          c.Next()
      }
  }
  ```

#### 3.2.3 缓存层隔离 / Cache Layer Isolation

- **Redis Key 命名规范** / **Redis Key Naming Convention**
  ```
  旧格式 / Old Format: user:{user_id}
  新格式 / New Format: tenant:{tenant_id}:user:{user_id}

  示例 / Examples:
  - tenant:default:user:123
  - tenant:customer-a:channel:456
  - tenant:default:token:abc123
  ```

---

## 四、认证与授权 / Authentication and Authorization

### 4.1 认证流程 / Authentication Flow

#### 4.1.1 OAuth2.0 授权码流程 / OAuth2.0 Authorization Code Flow

```
┌─────────┐                                           ┌──────────┐
│ Browser │                                           │ Zitadel  │
└────┬────┘                                           └─────┬────┘
     │                                                       │
     │ 1. GET /api/v2/lurus/auth/login                      │
     ├──────────────────────────────────────┐              │
     │                                       │              │
     │                                       ▼              │
     │                            ┌─────────────────────┐  │
     │                            │ lurus-api (OAuth)   │  │
     │                            └──────────┬──────────┘  │
     │                                       │              │
     │ 2. 302 Redirect to Zitadel           │              │
     │ ◄─────────────────────────────────────┘              │
     │                                                       │
     │ 3. GET /oauth/v2/authorize?...                       │
     ├──────────────────────────────────────────────────────►
     │                                                       │
     │ 4. Zitadel 登录页 / Zitadel Login Page               │
     │ ◄─────────────────────────────────────────────────────
     │                                                       │
     │ 5. POST 用户名/密码 / POST username/password          │
     ├──────────────────────────────────────────────────────►
     │                                                       │
     │ 6. 302 Redirect to lurus-api callback                │
     │ ◄─────────────────────────────────────────────────────
     │                                                       │
     │ 7. GET /oauth/callback?code=xxx                      │
     ├──────────────────────────────────────┐              │
     │                                       │              │
     │                                       ▼              │
     │                            ┌─────────────────────┐  │
     │                            │ lurus-api (Callback)│  │
     │                            └──────────┬──────────┘  │
     │                                       │              │
     │                        8. POST /oauth/v2/token       │
     │                           (exchange code)            │
     │                                       ├──────────────►
     │                                       │              │
     │                        9. access_token + id_token    │
     │                                       ◄──────────────┤
     │                                       │              │
     │                        10. 用户身份映射              │
     │                            (Zitadel User → lurus User)
     │                                       │              │
     │ 11. 302 Redirect to /dashboard        │              │
     │ ◄─────────────────────────────────────┘              │
     │    (Set Session Cookie)                              │
     │                                                       │
```

#### 4.1.2 JWT Token 验证流程 / JWT Token Verification Flow

```
┌─────────┐                                    ┌──────────────┐
│ Client  │                                    │  lurus-api   │
└────┬────┘                                    └──────┬───────┘
     │                                                 │
     │ 1. API Request                                  │
     │    Authorization: Bearer <JWT>                  │
     ├────────────────────────────────────────────────►
     │                                                 │
     │                                        ┌────────▼────────┐
     │                                        │ ZitadelAuth()   │
     │                                        │ Middleware      │
     │                                        └────────┬────────┘
     │                                                 │
     │                                        2. 提取 JWT Token
     │                                        Extract JWT Token
     │                                                 │
     │                                        3. 解析 Token Header
     │                                        Parse Token Header
     │                                        (获取 kid / Get kid)
     │                                                 │
     │                                   ┌─────────────▼──────────────┐
     │                                   │ JWKSManager                │
     │                                   │ (本地缓存公钥 / Local cache)│
     │                                   └─────────────┬──────────────┘
     │                                                 │
     │                                        4. 获取公钥 (by kid)
     │                                        Get Public Key
     │                                                 │
     │                                        5. 验证签名 + Claims
     │                                        Verify Signature + Claims
     │                                                 │
     │                                        6. 用户身份映射
     │                                        User Identity Mapping
     │                                        (org_id → tenant_id)
     │                                        (zitadel_user_id → lurus_user_id)
     │                                                 │
     │                                        7. 注入租户上下文
     │                                        Inject Tenant Context
     │                                                 │
     │ 8. API Response                                │
     │ ◄───────────────────────────────────────────────┤
     │                                                 │
```

### 4.2 授权模型 / Authorization Model

#### 4.2.1 角色定义 / Role Definitions

| 角色 / Role | 说明 / Description | 权限 / Permissions |
|-------------|-------------------|-------------------|
| `admin` | 租户管理员 / Tenant Admin | 租户内所有资源管理 / Manage all resources within tenant |
| `user` | 普通用户 / Regular User | 使用 API、管理自己的 Token / Use API, manage own tokens |
| `billing_manager` | 计费管理员 / Billing Manager | 查看账单、充值、订阅管理 / View bills, recharge, manage subscriptions |

#### 4.2.2 权限检查 / Permission Check

```go
// 检查用户角色 / Check user role
func RequireRole(role string) gin.HandlerFunc {
    return func(c *gin.Context) {
        tenantCtx := getTenantContext(c)
        if !hasRole(tenantCtx.Roles, role) {
            c.JSON(http.StatusForbidden, gin.H{
                "success": false,
                "message": "权限不足 / Insufficient permissions",
            })
            c.Abort()
            return
        }
        c.Next()
    }
}
```

---

## 五、数据库设计 / Database Design

### 5.1 核心表结构 / Core Tables

#### 5.1.1 租户表 (tenants)

```sql
CREATE TABLE tenants (
    id VARCHAR(36) PRIMARY KEY,              -- UUID (对应 Zitadel Organization ID)
    zitadel_org_id VARCHAR(128) UNIQUE NOT NULL, -- Zitadel Organization ID
    slug VARCHAR(64) UNIQUE NOT NULL,        -- 租户标识 (lurus, customer-a)
    name VARCHAR(128) NOT NULL,              -- 租户名称
    status INT DEFAULT 1,                    -- 1=enabled, 2=disabled, 3=suspended
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),

    -- Business Info
    plan_type VARCHAR(32) DEFAULT 'free',   -- free/pro/enterprise
    max_users INT DEFAULT 100,

    INDEX idx_zitadel_org (zitadel_org_id),
    INDEX idx_slug (slug),
    INDEX idx_status (status)
);
```

#### 5.1.2 用户身份映射表 (user_identity_mapping)

```sql
CREATE TABLE user_identity_mapping (
    id SERIAL PRIMARY KEY,
    lurus_user_id INT NOT NULL,              -- lurus users.id
    zitadel_user_id VARCHAR(128) NOT NULL,   -- Zitadel sub (User ID)
    tenant_id VARCHAR(36) NOT NULL,          -- 关联租户
    email VARCHAR(255),                      -- 用户邮箱 (同步自 Zitadel)
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),

    UNIQUE (zitadel_user_id, tenant_id),
    FOREIGN KEY (lurus_user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
    INDEX idx_zitadel_user (zitadel_user_id),
    INDEX idx_lurus_user_tenant (lurus_user_id, tenant_id)
);
```

### 5.2 现有表改造 / Existing Table Refactoring

#### 5.2.1 添加 tenant_id / Add tenant_id

```sql
-- 核心业务表 / Core business tables
ALTER TABLE users ADD COLUMN tenant_id VARCHAR(36);
ALTER TABLE tokens ADD COLUMN tenant_id VARCHAR(36);
ALTER TABLE channels ADD COLUMN tenant_id VARCHAR(36);

-- 计费相关表 / Billing-related tables
ALTER TABLE topups ADD COLUMN tenant_id VARCHAR(36);
ALTER TABLE subscriptions ADD COLUMN tenant_id VARCHAR(36);
ALTER TABLE redemptions ADD COLUMN tenant_id VARCHAR(36);

-- 日志表 / Log tables
ALTER TABLE logs ADD COLUMN tenant_id VARCHAR(36);

-- 创建索引 / Create indexes
CREATE INDEX idx_users_tenant ON users(tenant_id, id);
CREATE INDEX idx_tokens_tenant ON tokens(tenant_id, user_id);
CREATE INDEX idx_channels_tenant ON channels(tenant_id, group);
CREATE INDEX idx_logs_tenant ON logs(tenant_id, created_at);
```

#### 5.2.2 唯一索引改造 / Unique Index Refactoring

```sql
-- 用户表 / Users table
-- 旧索引 / Old: UNIQUE (username)
-- 新索引 / New: UNIQUE (tenant_id, username)
DROP INDEX IF EXISTS users_username_key;
CREATE UNIQUE INDEX idx_users_tenant_username ON users(tenant_id, username);

-- Token 表 / Tokens table
-- 旧索引 / Old: UNIQUE (key)
-- 新索引 / New: UNIQUE (tenant_id, key)
DROP INDEX IF EXISTS tokens_key_key;
CREATE UNIQUE INDEX idx_tokens_tenant_key ON tokens(tenant_id, key);
```

### 5.3 GORM 租户隔离插件 / GORM Tenant Isolation Plugin

```go
// model/tenant_plugin.go
type TenantPlugin struct{}

func (p *TenantPlugin) Name() string {
    return "TenantPlugin"
}

func (p *TenantPlugin) Initialize(db *gorm.DB) error {
    // Register callbacks for tenant isolation
    // 注册回调以实现租户隔离

    // Query callback: auto-inject WHERE tenant_id = ?
    // 查询回调: 自动注入 WHERE tenant_id = ?
    db.Callback().Query().Before("gorm:query").Register("tenant:query", func(db *gorm.DB) {
        if tenantID := getTenantIDFromContext(db); tenantID != "" {
            db.Where("tenant_id = ?", tenantID)
        }
    })

    // Create callback: auto-set tenant_id
    // 创建回调: 自动设置 tenant_id
    db.Callback().Create().Before("gorm:create").Register("tenant:create", func(db *gorm.DB) {
        if tenantID := getTenantIDFromContext(db); tenantID != "" {
            db.Statement.SetColumn("tenant_id", tenantID)
        }
    })

    return nil
}
```

---

## 六、API 架构 / API Architecture

### 6.1 API 版本划分 / API Versioning

```
/api (v1 API - 保持向后兼容 / Backward compatible)
├─ /user/login (Session 认证 / Session auth)
├─ /user/self
├─ /token/
├─ /channel/
└─ ... (所有原有 API / All existing APIs)

/api/v2 (多租户 API - 使用 Zitadel / Multi-tenant API - using Zitadel)
├─ /:tenant_slug/auth/login (OAuth 登录 / OAuth login)
├─ /oauth/callback (OAuth 回调 / OAuth callback)
├─ /:tenant_slug/user/me (Zitadel JWT 认证 / Zitadel JWT auth)
├─ /:tenant_slug/channels
├─ /:tenant_slug/billing/topups
└─ /admin/tenants (Platform Admin Only - 租户管理 / Tenant management)
```

### 6.2 API 路由示例 / API Route Examples

```go
// router/api-router.go

func SetApiRouter(router *gin.Engine) {
    // V1 API (向后兼容 / Backward compatible，默认租户 / Default tenant)
    apiV1 := router.Group("/api")
    apiV1.Use(middleware.DefaultTenantMiddleware())
    {
        apiV1.GET("/status", controller.GetStatus)
        apiV1.POST("/user/login", controller.Login)
        apiV1.GET("/user/self", middleware.UserAuth(), controller.GetSelf)
        // ... 原有路由 / Existing routes
    }

    // V2 API (多租户 + Zitadel 认证 / Multi-tenant + Zitadel auth)
    apiV2 := router.Group("/api/v2")
    {
        // OAuth 登录流程 / OAuth login flow
        apiV2.GET("/:tenant_slug/auth/login", controller.ZitadelLoginRedirect)
        apiV2.GET("/oauth/callback", controller.ZitadelCallback)
        apiV2.POST("/oauth/logout", controller.ZitadelLogout)

        // 租户级 API (需要 Zitadel Token / Requires Zitadel Token)
        tenantRoute := apiV2.Group("/:tenant_slug")
        tenantRoute.Use(middleware.ZitadelAuth()) // Zitadel JWT 验证
        {
            // 用户 API / User API
            tenantRoute.GET("/user/me", controller.GetSelfV2)
            tenantRoute.PUT("/user/me", controller.UpdateSelfV2)

            // Channel API
            tenantRoute.GET("/channels", controller.ListChannelsV2)
            tenantRoute.POST("/channels", middleware.RequireRole("admin"), controller.CreateChannelV2)

            // Billing API
            tenantRoute.GET("/billing/topups", controller.GetTopUpsV2)
            tenantRoute.POST("/billing/topup", controller.TopUpV2)
        }

        // 租户管理 API (Platform Admin Only / 使用 v1 Session 认证)
        adminRoute := apiV2.Group("/admin/tenants")
        adminRoute.Use(middleware.UserAuth(), middleware.RootAuth())
        {
            adminRoute.GET("", controller.ListTenants)
            adminRoute.POST("", controller.CreateTenant)
            adminRoute.GET("/:id", controller.GetTenant)
            adminRoute.PUT("/:id", controller.UpdateTenant)
            adminRoute.DELETE("/:id", controller.DeleteTenant)
        }
    }
}
```

---

## 七、部署架构 / Deployment Architecture

### 7.1 K3s 集群架构 / K3s Cluster Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     K3s Cluster                             │
├─────────────────────────────────────────────────────────────┤
│                                                               │
│  Namespace: lurus-identity                                   │
│  ├─ Zitadel Deployment                                       │
│  ├─ Zitadel Service (ClusterIP)                              │
│  └─ IngressRoute: zitadel.lurus.cn                           │
│                                                               │
│  Namespace: lurus-api                                        │
│  ├─ lurus-api Deployment                                     │
│  ├─ lurus-api Service (ClusterIP)                            │
│  ├─ PostgreSQL StatefulSet                                   │
│  ├─ Redis Deployment                                         │
│  ├─ Meilisearch Deployment                                   │
│  └─ IngressRoute: api.lurus.cn                               │
│                                                               │
│  Namespace: traefik-system                                   │
│  ├─ Traefik Deployment (Reverse Proxy)                       │
│  └─ Cert-Manager (TLS Certificates)                          │
│                                                               │
└─────────────────────────────────────────────────────────────┘
```

### 7.2 流量路由 / Traffic Routing

```
Internet
  │
  │ HTTPS (443)
  │
  ▼
Traefik (Reverse Proxy)
  │
  ├─── zitadel.lurus.cn ──────► Zitadel Service (lurus-identity namespace)
  │                              ├─ OAuth 登录页 / OAuth login page
  │                              ├─ 用户管理界面 / User management UI
  │                              └─ OIDC Endpoints
  │
  ├─── api.lurus.cn ──────────► lurus-api Service (lurus-api namespace)
  │                              ├─ v1 API (Session)
  │                              ├─ v2 API (Zitadel JWT)
  │                              └─ Web UI (React)
  │
  └─── (Internal Services)
         ├─ PostgreSQL (5432)
         ├─ Redis (6379)
         └─ Meilisearch (7700)
```

### 7.3 环境变量配置 / Environment Variables

```env
# Zitadel 配置 / Zitadel Configuration
ZITADEL_ISSUER=https://zitadel.lurus.cn
ZITADEL_CLIENT_ID=123456789@lurus-api
ZITADEL_CLIENT_SECRET=xxx
ZITADEL_REDIRECT_URI=https://api.lurus.cn/oauth/callback
ZITADEL_JWKS_URI=https://zitadel.lurus.cn/oauth/v2/keys

# 数据库配置 / Database Configuration
SQL_DSN=postgresql://user:pass@postgres:5432/lurus?sslmode=disable

# Redis 配置 / Redis Configuration
REDIS_CONN_STRING=redis://redis:6379

# Meilisearch 配置 / Meilisearch Configuration
MEILISEARCH_ENABLED=true
MEILISEARCH_HOST=http://meilisearch:7700
MEILISEARCH_API_KEY=xxx
```

---

## 八、安全设计 / Security Design

### 8.1 认证安全 / Authentication Security

1. **JWT Token 验证** / **JWT Token Verification**
   - JWKS 公钥验证 / JWKS public key verification
   - Token 过期检查 / Token expiration check
   - Issuer 白名单验证 / Issuer whitelist verification

2. **HTTPS 强制** / **HTTPS Enforcement**
   - 所有 API 必须 HTTPS / All APIs require HTTPS
   - Cert-Manager 自动续期 / Cert-Manager auto-renewal

3. **跨域保护** / **CORS Protection**
   - 严格的 CORS 策略 / Strict CORS policy
   - 仅允许信任的域名 / Only allow trusted domains

### 8.2 数据安全 / Data Security

1. **租户隔离** / **Tenant Isolation**
   - GORM Plugin 自动过滤 / GORM Plugin auto-filtering
   - 防止跨租户查询 / Prevent cross-tenant queries

2. **敏感数据加密** / **Sensitive Data Encryption**
   - 密码使用 bcrypt 哈希 / Passwords use bcrypt hash
   - API Key 使用 SHA256 哈希 / API keys use SHA256 hash

3. **审计日志** / **Audit Logs**
   - 记录所有关键操作 / Record all critical operations
   - 包含租户 ID、用户 ID、时间戳 / Include tenant ID, user ID, timestamp

---

## 九、监控与运维 / Monitoring and Operations

### 9.1 健康检查 / Health Checks

```go
// Health check endpoint
// 健康检查端点
GET /api/health
{
    "status": "ok",
    "database": "ok",
    "redis": "ok",
    "meilisearch": "ok",
    "zitadel": "ok"
}
```

### 9.2 性能指标 / Performance Metrics

| 指标 / Metric | 目标 / Target |
|--------------|--------------|
| API 响应时间 (P95) / API Response Time | < 100ms |
| 搜索响应时间 (P95) / Search Response Time | < 50ms |
| 并发请求数 / Concurrent Requests | 1000+ |
| 数据库连接池 / DB Connection Pool | Max 1000 |

---

**文档版本历史 / Document Version History**:
- v1.0 (2026-01-25): 初始版本 / Initial version

**维护者 / Maintainer**: Lurus Team

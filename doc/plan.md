# Lurus-API 多租户 SaaS 改造计划 (基于 Zitadel)
# Lurus-API Multi-Tenant SaaS Transformation Plan (Zitadel-based)

**项目目标 / Project Goal**: 将 lurus-api 从单租户多用户架构改造为多租户 SaaS 平台，使用 Zitadel 作为统一认证中心，支持 5+ 个独立业务作为租户接入

**实施周期 / Timeline**: 1-1.5 个月 (4-6 周) ⚡️

**技术栈 / Tech Stack**:
- **认证中心 / Auth Center**: Zitadel (计划部署在 `lurus-identity` namespace)
- **业务服务 / Business Service**: lurus-api (Go 1.25.1 + Gin + GORM + PostgreSQL + Redis)
- **认证协议 / Auth Protocol**: OAuth2.0 + OIDC (OpenID Connect)

---

## 一、架构设计 / Architecture Design

### 1.1 核心架构对比 / Core Architecture Comparison

#### 原计划方案 (❌ 自己实现 JWT / Implement JWT ourselves)
```
用户 → lurus-api → 自己实现 JWT 签发 → 自己验证 Token
      需要实现 / Need to implement:
      - 用户注册/登录 / User registration/login
      - 密码管理 / Password management
      - OAuth 集成 (GitHub/Google/Discord...) / OAuth integration
      - 2FA/Passkey
      - JWT 签发与验证 / JWT issuance and validation
      - Session 管理 / Session management
      工作量 / Workload: 2-3 个月 / 2-3 months
```

#### 新方案 (✅ 使用 Zitadel / Use Zitadel)
```
┌─────────────────────────────────────────────────────────────┐
│                     Zitadel (认证中心 / Auth Center)          │
│              https://zitadel.lurus.cn (待配置 / To Configure) │
│  ┌──────────────────────────────────────────────────────┐  │
│  │  已内置功能 / Built-in Features:                       │  │
│  │  ✅ 用户注册/登录 / User registration/login            │  │
│  │  ✅ OAuth2.0 / OIDC                                   │  │
│  │  ✅ 密码管理 / 密码重置 / Password management/reset    │  │
│  │  ✅ 多因素认证 (MFA/2FA) / Multi-factor auth          │  │
│  │  ✅ Passkey / WebAuthn                                │  │
│  │  ✅ 社交登录 (Google/GitHub/Microsoft...)             │  │
│  │  ✅ JWT Token 签发 / JWT token issuance               │  │
│  │  ✅ 多租户 (Organizations + Projects) / Multi-tenant │  │
│  │  ✅ RBAC 权限管理 / RBAC permission management        │  │
│  │  ✅ 用户管理界面 / User management UI                  │  │
│  └──────────────────────────────────────────────────────┘  │
└───────────────────────┬─────────────────────────────────────┘
                        │
                        │ 签发 JWT Token / Issue JWT Token
                        │ (org_id, user_id, roles, email...)
                        │
                        ▼
┌─────────────────────────────────────────────────────────────┐
│                 lurus-api (Resource Server)                  │
│  ┌──────────────────────────────────────────────────────┐  │
│  │  需要实现 / Need to implement:                         │  │
│  │  ✅ 验证 Zitadel 签发的 JWT Token (OIDC)              │  │
│  │  ✅ 用户身份映射 (Zitadel User → lurus User)          │  │
│  │  ✅ 租户数据隔离 (org_id → tenant_id)                 │  │
│  │  ✅ 业务逻辑 (计费、Channel 管理、订阅...)             │  │
│  │                                                        │  │
│  │  保持不变 / Keep unchanged:                            │  │
│  │  📦 计费系统 (Stripe/Epay/Creem) / Billing system    │  │
│  │  📦 订阅系统 / Subscription system                     │  │
│  │  📦 兑换码系统 / Redemption system                     │  │
│  │  📦 额度管理 / Quota management                       │  │
│  │  📦 Channel 管理 / Channel management                │  │
│  └──────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘

工作量 / Workload: 1-1.5 个月 (节省 50-60% 开发时间 / Save 50-60% development time)
```

### 1.2 Zitadel 多租户模型 / Zitadel Multi-Tenant Model

Zitadel 使用 **Organization + Project** 模型实现多租户 / Zitadel uses **Organization + Project** model for multi-tenancy：

```
Zitadel Instance (zitadel.lurus.cn)
│
├─ Organization: Lurus Platform (默认组织 / Default org)
│  ├─ Project: lurus-api
│  │  └─ Users: user1, user2, admin1...
│  │
│  ├─ Project: gushen (量化交易 / Quantitative trading)
│     └─ Users: trader1, trader2...
│
├─ Organization: Customer A (客户A / Customer A)
│  ├─ Project: customer-a-api
│     └─ Users: customerA_user1, customerA_admin...
│
├─ Organization: Customer B (客户B / Customer B)
   ├─ Project: customer-b-api
      └─ Users: customerB_user1...
```

**映射关系 / Mapping Relationship**:
- `Zitadel Organization` → `lurus Tenant`
- `Zitadel Project` → `lurus Application/Business`
- `Zitadel User` → `lurus User` (需要建立映射 / Needs mapping)

---

## 二、分阶段实施计划 / Phased Implementation Plan

### 阶段 1: Zitadel 配置与集成 / Phase 1: Zitadel Configuration and Integration (第 1 周 / Week 1)

**目标 / Goal**:
- 配置 Zitadel 实例 / Configure Zitadel instance
- 创建 Organization 和 Project / Create Organization and Project
- 实现 OIDC 客户端注册 / Implement OIDC client registration

**工作内容 / Tasks**:

#### 1.1 配置 Zitadel 实例 / Configure Zitadel Instance

```bash
# 1. 访问 Zitadel 管理界面 / Access Zitadel admin interface
# URL: https://zitadel.lurus.cn (目前待部署 / To be deployed)

# 2. 配置 Zitadel 域名 / Configure Zitadel domain
# Settings → Instance Settings
# - Instance Domain: zitadel.lurus.cn
# - Custom Domain: auth.lurus.cn (可选 / Optional)

# 3. 配置 SMTP (用于邮件验证 / For email verification)
# Settings → SMTP
# - SMTP Server: mail.lurus.cn (使用现有 Stalwart Mail / Use existing Stalwart Mail)
# - Port: 587
# - Username: noreply@lurus.cn
# - Password: ***
```

#### 1.2 创建 Organization 和 Project / Create Organization and Project

**在 Zitadel 中创建 / Create in Zitadel**:

1. **创建默认 Organization / Create default Organization**
   - Name: `Lurus Platform`
   - Domain: `lurus` (primary domain)
   - 记录 Organization ID (例如: `123456789`) / Record Organization ID (e.g., `123456789`)

2. **创建 Project / Create Project**
   - Project Name: `lurus-api`
   - Project Type: `API`
   - Grant Types: `Authorization Code`, `Refresh Token`

3. **创建 Application (OIDC Client) / Create Application (OIDC Client)**
   - Application Name: `lurus-api-backend`
   - Application Type: `API`
   - Authentication Method: `JWT`
   - Redirect URIs:
     - `https://api.lurus.cn/oauth/callback`
     - `http://localhost:8850/oauth/callback` (开发环境 / Development)
   - Post Logout URIs: `https://api.lurus.cn/logout`
   - 记录 `Client ID` 和 `Client Secret` / Record `Client ID` and `Client Secret`

4. **配置 Project Roles / Configure Project Roles**
   - Role: `admin` (管理员 / Administrator)
   - Role: `user` (普通用户 / Regular user)
   - Role: `billing_manager` (计费管理员 / Billing manager)

**工作量 / Workload**: 2-3 天 / 2-3 days
**风险 / Risk**: 低 / Low

---

### 阶段 2: JWT 验证中间件实现 / Phase 2: JWT Verification Middleware Implementation (第 1-2 周 / Week 1-2)

**目标 / Goal**:
- 实现 OIDC JWT Token 验证 / Implement OIDC JWT Token verification
- 实现用户身份映射 / Implement user identity mapping
- 实现租户上下文注入 / Implement tenant context injection

**涉及文件 / Files Involved**:
```
新建 / New:
- middleware/zitadel_auth.go (Zitadel JWT 验证中间件 / Zitadel JWT verification middleware)
- model/user_mapping.go (用户身份映射 / User identity mapping)
- model/tenant.go (租户模型 / Tenant model)

修改 / Modified:
- model/main.go (初始化 JWKS Manager / Initialize JWKS Manager)
- go.mod (添加依赖 / Add dependencies)
```

**工作量 / Workload**: 5-7 天 / 5-7 days
**风险 / Risk**: 中 / Medium

---

### 阶段 3: 数据库迁移与租户隔离 / Phase 3: Database Migration and Tenant Isolation (第 2-3 周 / Week 2-3)

**目标 / Goal**:
- 创建租户表和映射表 / Create tenant and mapping tables
- 为所有表添加 `tenant_id` / Add `tenant_id` to all tables
- 实现 GORM 租户隔离插件 / Implement GORM tenant isolation plugin
- 迁移现有数据 / Migrate existing data

**工作内容 / Tasks**:

1. **数据库迁移脚本 / Database Migration Scripts**
   - `migrations/001_create_tenants.sql`
   - `migrations/002_create_user_mapping.sql`
   - `migrations/003_add_tenant_id.sql`
   - `migrations/004_add_indexes.sql`

2. **GORM 租户隔离插件 / GORM Tenant Isolation Plugin**
   - `model/tenant_plugin.go`

3. **模型层改造 / Model Layer Refactoring**
   - 修改 `model/user.go`, `model/token.go`, `model/channel.go` 等 / Modify `model/user.go`, `model/token.go`, `model/channel.go`, etc.

4. **Redis 缓存改造 / Redis Cache Refactoring**
   - Key 命名变更: `user:{id}` → `tenant:{tid}:user:{id}`

**工作量 / Workload**: 8-10 天 / 8-10 days
**风险 / Risk**: 中 / Medium

---

### 阶段 4: API 路由与 OAuth 登录流程 / Phase 4: API Routes and OAuth Login Flow (第 3-4 周 / Week 3-4)

**目标 / Goal**:
- 实现 OAuth2.0 登录流程 / Implement OAuth2.0 login flow
- 添加 v2 API 路由 / Add v2 API routes
- 实现租户管理 API / Implement tenant management API

**涉及文件 / Files Involved**:
```
新建 / New:
- controller/oauth.go (OAuth 登录流程 / OAuth login flow)
- controller/tenant.go (租户管理 / Tenant management)

修改 / Modified:
- router/api-router.go (添加 v2 API 路由 / Add v2 API routes)
```

**工作量 / Workload**: 5-7 天 / 5-7 days
**风险 / Risk**: 中 / Medium

---

### 阶段 5: 计费系统租户隔离 / Phase 5: Billing System Tenant Isolation (第 4-5 周 / Week 4-5)

**目标 / Goal**:
- 改造 TopUp, Subscription, Redemption / Refactor TopUp, Subscription, Redemption
- Webhook 租户识别 / Webhook tenant identification
- 租户级订阅计划 / Tenant-level subscription plans

**工作量 / Workload**: 5-7 天 / 5-7 days
**风险 / Risk**: 高 (涉及资金安全 / High - involves financial security)

---

### 阶段 6: 测试与文档 / Phase 6: Testing and Documentation (第 5-6 周 / Week 5-6)

**目标 / Goal**:
- 全面测试 / Comprehensive testing
- 编写文档 / Write documentation
- 准备上线 / Prepare for deployment

**工作量 / Workload**: 5-7 天 / 5-7 days

---

## 三、实施检查清单 / Implementation Checklist

### 3.1 Zitadel 配置 / Zitadel Configuration

- [ ] Zitadel 实例可访问 (https://zitadel.lurus.cn) / Zitadel instance accessible
- [ ] 创建 Organization: `Lurus Platform` / Create Organization: `Lurus Platform`
- [ ] 创建 Project: `lurus-api` / Create Project: `lurus-api`
- [ ] 创建 Application (OIDC Client) / Create Application (OIDC Client)
- [ ] 配置 Redirect URIs / Configure Redirect URIs
- [ ] 配置 Project Roles (admin/user/billing_manager) / Configure Project Roles
- [ ] 获取 Client ID 和 Client Secret / Obtain Client ID and Client Secret
- [ ] 配置 SMTP (使用 Stalwart Mail) / Configure SMTP (using Stalwart Mail)
- [ ] 测试 OAuth 登录流程 / Test OAuth login flow

### 3.2 代码实现 / Code Implementation

- [ ] 实现 JWT 验证中间件 (`middleware/zitadel_auth.go`) / Implement JWT verification middleware
- [ ] 实现 JWKS 公钥管理 (`JWKSManager`) / Implement JWKS public key management
- [ ] 实现用户身份映射 (`model/user_mapping.go`) / Implement user identity mapping
- [ ] 实现租户模型 (`model/tenant.go`) / Implement tenant model
- [ ] 实现 OAuth 登录流程 (`controller/oauth.go`) / Implement OAuth login flow
- [ ] 添加 v2 API 路由 (`router/api-router.go`) / Add v2 API routes
- [ ] 实现 GORM 租户隔离插件 / Implement GORM tenant isolation plugin
- [ ] 改造计费系统 / Refactor billing system
- [ ] 单元测试覆盖率 > 80% / Unit test coverage > 80%

### 3.3 数据库 / Database

- [ ] 创建 `tenants` 表 / Create `tenants` table
- [ ] 创建 `user_identity_mapping` 表 / Create `user_identity_mapping` table
- [ ] 创建 `tenant_configs` 表 / Create `tenant_configs` table
- [ ] 为所有表添加 `tenant_id` 字段 / Add `tenant_id` field to all tables
- [ ] 创建索引 / Create indexes
- [ ] 迁移现有数据到默认租户 / Migrate existing data to default tenant
- [ ] 数据完整性验证 / Data integrity verification

### 3.4 测试 / Testing

- [ ] OAuth 登录流程测试 / OAuth login flow test
- [ ] JWT Token 验证测试 / JWT Token verification test
- [ ] 用户身份映射测试 / User identity mapping test
- [ ] 租户数据隔离测试 / Tenant data isolation test
- [ ] 计费系统租户隔离测试 / Billing system tenant isolation test
- [ ] 性能测试 (P95 < 100ms) / Performance test (P95 < 100ms)
- [ ] 安全测试 (Token 伪造、跨租户访问) / Security test (Token forgery, cross-tenant access)

---

## 四、总结 / Summary

**改造规模 / Scope**: 中型重构 / Medium refactoring
**实施周期 / Timeline**: 1-1.5 个月 (4-6 周) ⚡️
**核心优势 / Core Advantages**:
- 🎯 节省 40-50% 开发时间 / Save 40-50% development time
- 🎯 使用成熟的开源认证系统 (Zitadel) / Use mature open-source auth system (Zitadel)
- 🎯 免费获得完整的用户管理界面 / Get complete user management UI for free
- 🎯 支持所有主流社交登录 / Support all major social logins
- 🎯 内置 2FA/Passkey/MFA / Built-in 2FA/Passkey/MFA
- 🎯 审计日志与 GDPR 合规 / Audit logs and GDPR compliance

**关键成功因素 / Key Success Factors**:
1. ✅ Zitadel 配置正确 (Organization + Project + Application) / Correct Zitadel configuration
2. ✅ JWT 验证逻辑实现正确 (JWKS 公钥验证) / Correct JWT verification logic
3. ✅ 用户身份映射机制稳定 (Zitadel User → lurus User) / Stable user identity mapping
4. ✅ GORM Plugin 确保租户隔离安全 / GORM Plugin ensures tenant isolation security
5. ✅ v1 API 向后兼容 / v1 API backward compatibility

**预期成果 / Expected Results**:
- 🎯 lurus-api 成为多租户 SaaS 平台 / lurus-api becomes a multi-tenant SaaS platform
- 🎯 使用 Zitadel 作为统一认证中心 / Use Zitadel as unified auth center
- 🎯 支持 5+ 个独立业务接入 / Support 5+ independent businesses
- 🎯 每个租户独立的 Organization / Each tenant has independent Organization
- 🎯 完整的用户管理和权限控制 / Complete user management and permission control
- 🎯 统一的计费、订阅、额度管理 / Unified billing, subscription, and quota management

---

**下一步行动 / Next Steps**:
1. 部署 Zitadel 实例到 K3s 集群 / Deploy Zitadel instance to K3s cluster
2. 配置 Zitadel (访问 https://zitadel.lurus.cn) / Configure Zitadel
3. 创建默认 Organization 和 Project / Create default Organization and Project
4. 实现 JWT 验证中间件 / Implement JWT verification middleware
5. 开始数据库迁移 / Start database migration
6. 分阶段测试与上线 / Phased testing and deployment

---

**备注 / Notes**: 本方案基于计划部署的 Zitadel 实例，大幅简化了认证系统的实现工作，将重点放在业务逻辑的多租户改造上。/ This plan is based on the planned Zitadel instance deployment, which greatly simplifies the authentication system implementation and focuses on multi-tenant refactoring of business logic.

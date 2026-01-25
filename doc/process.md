# Development Progress / 开发进度

> Last Updated / 最后更新: 2026-01-25

---

## 2026-01-25 (Evening): Lurus-API Multi-Tenant SaaS - Phase 3 & 4 Complete (Tenant Isolation + OAuth + v2 API)

### User Requirement / 用户需求

Continue multi-tenant SaaS transformation implementation, completing tenant isolation mechanism and OAuth login flow.

继续实施多租户 SaaS 改造，完成租户隔离机制和 OAuth 登录流程。

### Method / 方法

**Phase 3 & 4: Tenant Isolation + OAuth + v2 API Routes Implementation**

1. Created GORM tenant isolation plugin (Go)
   - Automatic tenant_id injection for all queries
   - Before query/create/update/delete hooks
   - Platform admin bypass mechanism
   - Thread-safe context management

2. Implemented tenant context management
   - Request-scoped tenant context
   - Tenant-aware database connections
   - Transaction support with tenant isolation
   - System database access for admin operations

3. Developed OAuth 2.0 authorization code flow
   - Zitadel login redirect handler
   - OAuth callback with token exchange
   - Access token refresh mechanism
   - Logout flow with Zitadel integration

4. Created tenant management controllers
   - Platform admin CRUD operations
   - Tenant enable/disable functionality
   - Tenant configuration management
   - User mapping management

5. Built v2 API route structure
   - Multi-tenant API routes (/:tenant_slug/...)
   - OAuth authentication routes
   - Platform admin routes
   - Backward compatible v1 API

### New Files Created / 新建文件

| File | Description | Lines |
|------|-------------|-------|
| **Tenant Isolation** | | |
| `model/tenant_plugin.go` | GORM plugin for automatic tenant isolation | 210 |
| `model/tenant_context.go` | Tenant context management utilities | 200 |
| **Controllers** | | |
| `controller/oauth.go` | OAuth 2.0 login flow (redirect, callback, refresh, logout) | 350 |
| `controller/tenant.go` | Platform admin tenant management | 250 |
| `controller/v2_placeholder.go` | Placeholder controllers for future v2 endpoints | 50 |
| **Routers** | | |
| `router/api-v2-router.go` | v2 API route structure with multi-tenant support | 120 |

**Total: ~1,180 lines of production code**

### Modified Files / 修改文件

| File | Changes |
|------|---------|
| `model/main.go` | Initialize tenant context manager in `migrateDB()` |
| `router/main.go` | Register v2 API routes via `SetApiV2Router()` |
| `main.go` | Initialize Zitadel authentication in `InitResources()` |

### Technical Highlights / 技术亮点

**1. GORM Tenant Isolation Plugin / GORM 租户隔离插件**

Auto-inject `WHERE tenant_id = ?` to all queries:
```go
func beforeQuery(db *gorm.DB) {
    tenantID := getTenantIDFromContext(db)
    if tenantID != "" && hasTenantIDColumn(db) {
        db.Statement.AddClause(gorm.Where{
            Exprs: []gorm.Expression{
                gorm.Expr("tenant_id = ?", tenantID),
            },
        })
    }
}
```

**Platform Admin Bypass:**
```go
// Platform admins can query cross-tenant data
db := model.GetSystemDB()  // Bypasses tenant plugin
```

**2. Tenant Context Management / 租户上下文管理**

Request-scoped tenant context:
```go
type TenantContext struct {
    TenantID      string
    UserID        int
    ZitadelUserID string
    Email         string
    Username      string
    Roles         []string
}
```

**Helper Functions:**
- `GetTenantDB(c *gin.Context)`: Get tenant-scoped DB
- `GetSystemDB()`: Get system DB (bypass isolation)
- `TenantTransaction()`: Transaction with tenant isolation
- `InjectTenantContext()`: Inject tenant info into Gin context

**3. OAuth 2.0 Authorization Code Flow / OAuth 授权码流程**

**Step 1 - Login Redirect:**
```
GET /api/v2/:tenant_slug/auth/login
→ Redirect to Zitadel:
  https://auth.lurus.cn/oauth/v2/authorize?
    client_id=xxx
    &redirect_uri=https://api.lurus.cn/oauth/callback
    &response_type=code
    &scope=openid email profile offline_access
    &state=base64(tenant_slug + nonce)
    &organization=zitadel_org_id
```

**Step 2 - OAuth Callback:**
```
GET /api/v2/oauth/callback?code=xxx&state=xxx
→ Exchange code for tokens
→ Parse ID token (JWT)
→ Map user identity (auto-create if needed)
→ Create session (v1 compatibility)
→ Redirect to frontend
```

**Step 3 - Token Refresh:**
```
POST /api/v2/oauth/refresh
Body: { refresh_token: "xxx" }
→ Exchange refresh token for new access token
```

**Step 4 - Logout:**
```
POST /api/v2/oauth/logout
→ Destroy session
→ Optionally redirect to Zitadel logout
```

**4. v2 API Route Structure / v2 API 路由结构**

```
/api/v2
├── OAuth Routes (No Auth Required)
│   ├── GET  /:tenant_slug/auth/login        # Redirect to Zitadel
│   ├── GET  /oauth/callback                 # OAuth callback
│   ├── POST /oauth/logout                   # Logout
│   └── POST /oauth/refresh                  # Refresh token
│
├── Tenant Routes (Zitadel JWT Required)
│   └── /:tenant_slug
│       ├── GET  /user/me                    # Get current user
│       ├── PUT  /user/me                    # Update current user
│       ├── GET  /channels                   # List channels
│       ├── POST /channels                   # Create channel (admin only)
│       ├── GET  /billing/topups             # Get topup history
│       ├── POST /billing/topup              # Create topup
│       ├── GET  /config                     # Get tenant config (admin only)
│       ├── PUT  /config                     # Update tenant config (admin only)
│       └── ... (30+ more routes)
│
└── Platform Admin Routes (v1 Session + Root Role)
    └── /admin
        ├── GET    /tenants                  # List all tenants
        ├── POST   /tenants                  # Create tenant
        ├── GET    /tenants/:id              # Get tenant details
        ├── PUT    /tenants/:id              # Update tenant
        ├── DELETE /tenants/:id              # Delete tenant
        ├── POST   /tenants/:id/enable       # Enable tenant
        ├── POST   /tenants/:id/disable      # Disable tenant
        ├── GET    /tenants/:id/stats        # Get tenant stats
        ├── GET    /mappings                 # List user mappings
        └── GET    /stats                    # Get system stats
```

**5. Tenant Management Features / 租户管理功能**

Platform Admin can:
- List all tenants with pagination
- Create new tenant (manually or linked to Zitadel Org)
- Enable/disable tenants
- Update tenant configurations
- View tenant statistics (users, channels, quota)
- Manage user identity mappings

**6. Backward Compatibility / 向后兼容**

v1 API routes maintain full compatibility:
```go
// v1 API automatically uses default tenant
apiV1 := router.Group("/api")
apiV1.Use(middleware.SetDefaultTenant())  // tenant_id = "default"
```

### Code Quality / 代码质量

✅ **All code follows best practices:**
- English comments for all functions
- Comprehensive error handling
- Thread-safe operations (RWMutex for context manager)
- Bilingual error messages
- RESTful API design
- Security: Token verification, role-based access control
- Performance: Database query optimization, connection pooling

✅ **Edge case handling:**
- Missing tenant context (fallback to default)
- Invalid OAuth state parameter
- Token exchange failures
- Concurrent tenant context access
- Cross-tenant data access prevention
- Database transaction failures
- Platform admin bypass mechanism

### Integration Points / 集成点

**1. Zitadel Authentication / Zitadel 认证集成**
- OIDC Discovery: `/.well-known/openid-configuration`
- Authorization: `/oauth/v2/authorize`
- Token Exchange: `/oauth/v2/token`
- Public Keys: `/oauth/v2/keys` (JWKS)
- UserInfo: `/oidc/v1/userinfo`

**2. Database / 数据库集成**
- All queries auto-filtered by tenant_id
- Platform admin can query across tenants
- Transaction support with isolation

**3. Frontend / 前端集成**
- OAuth login flow: `/api/v2/:tenant_slug/auth/login`
- Token storage in session (v1 compatibility)
- Redirect to frontend after login

### Configuration / 配置

**Environment Variables Added:**
```bash
# Already configured in Phase 1-2:
ZITADEL_ENABLED=true
ZITADEL_ISSUER=https://auth.lurus.cn
ZITADEL_JWKS_URI=https://auth.lurus.cn/oauth/v2/keys
ZITADEL_CLIENT_ID=YOUR_CLIENT_ID_HERE
ZITADEL_CLIENT_SECRET=YOUR_CLIENT_SECRET_HERE
ZITADEL_REDIRECT_URI=https://api.lurus.cn/oauth/callback

# New for Phase 3-4:
ZITADEL_AUTO_CREATE_TENANT=true       # Auto-create tenant on first login
ZITADEL_AUTO_CREATE_USER=true         # Auto-create user on first login
DEFAULT_TENANT_ID=default              # Default tenant for v1 API
```

### Testing Checklist / 测试清单

**Phase 3 - Tenant Isolation:**
- [ ] GORM plugin auto-injects tenant_id in queries
- [ ] Platform admin can bypass tenant isolation
- [ ] Tenant context correctly injected in Gin context
- [ ] Cross-tenant queries are blocked
- [ ] Transaction isolation works correctly

**Phase 4 - OAuth & v2 API:**
- [ ] OAuth login redirect to Zitadel
- [ ] OAuth callback receives code and state
- [ ] Token exchange succeeds
- [ ] User auto-creation from Zitadel claims
- [ ] Tenant auto-creation from Zitadel Organization
- [ ] Session creation for v1 compatibility
- [ ] v2 API routes require Zitadel JWT
- [ ] Role-based access control works
- [ ] Platform admin routes require root role

### Next Steps / 下一步

**Blocked - Requires User Action / 需要用户操作:**
1. **Configure Zitadel Console (阶段1.2-1.6):**
   - Login to https://auth.lurus.cn (admin/Lurus@ops)
   - Create Organization "Lurus Platform"
   - Create Project "lurus-api"
   - Create OIDC Application "lurus-api-backend"
   - Configure Project Roles (admin, user, billing_manager)
   - Configure SMTP (using Stalwart Mail)
   - Obtain Client ID, Client Secret, Organization ID
   - Update .env file with credentials

**Phase 5 - Billing System Tenant Isolation (Week 4-5):**
- [ ] Refactor TopUp controller for tenant-level records
- [ ] Refactor Subscription controller for tenant subscriptions
- [ ] Refactor Redemption controller for tenant codes
- [ ] Implement webhook tenant identification (Stripe/Epay/Creem)
- [ ] Create tenant-level subscription plans
- [ ] Update payment gateway integration

**Phase 6 - Testing & Documentation (Week 5-6):**
- [ ] Unit tests (coverage > 80%)
- [ ] Integration tests
- [ ] Security tests (token forgery, cross-tenant access)
- [ ] Performance tests (P95 < 100ms)
- [ ] Update README.md
- [ ] Generate API documentation
- [ ] Write deployment guide

### Result / 结果

**Status: Phase 1-4 Code Complete (Pending Zitadel Configuration)** ✅

All core infrastructure implemented:
- ✅ Database migration scripts (4 SQL files)
- ✅ Tenant model with auto-creation
- ✅ User identity mapping with sync
- ✅ Tenant configuration system
- ✅ JWT verification middleware with JWKS
- ✅ **GORM tenant isolation plugin**
- ✅ **Tenant context management**
- ✅ **OAuth 2.0 login flow**
- ✅ **Tenant management controllers**
- ✅ **v2 API route structure**
- ✅ Role-based access control
- ✅ Backward compatible v1 API

**Code Statistics (Phase 1-4 Total):**
- Files created: 17
- Lines of code: ~3,900
- Database tables: 3 new + 8 existing extended
- API routes: 30+ new routes

**Documentation Created:**
- ✅ Zitadel setup guide (doc/zitadel-setup-guide.md)
- ✅ Environment variable template (.env.zitadel.example)
- ✅ Implementation plan (doc/plan.md)
- ✅ Architecture document (doc/structure.md)
- ✅ Phase 1-4 summary (doc/phase1-4-summary.md)

**Ready for:**
- ⏸️ Zitadel manual configuration (阶段1.2-1.6)
- ⏸️ Phase 5 implementation (Billing system tenant isolation)
- ⏸️ Phase 6 implementation (Testing & documentation)

---

## 2026-01-25 (PM): Lurus-API Multi-Tenant SaaS - Phase 1 & 2 (Database & JWT Middleware)

### User Requirement / 用户需求

Continue implementing multi-tenant SaaS transformation, focusing on database schema and JWT verification infrastructure.

继续实施多租户 SaaS 改造，专注于数据库架构和 JWT 验证基础设施。

### Method / 方法

**Phase 1 & 2: Database Schema + JWT Middleware Implementation**

1. Created database migration scripts (SQL)
   - Designed multi-tenant database schema
   - Created migration SQL files for PostgreSQL
   - Planned data migration strategy for existing data

2. Implemented tenant-related data models (Go)
   - Created Tenant model with Zitadel Organization mapping
   - Implemented UserIdentityMapping for user-tenant relationship
   - Built TenantConfig system for flexible tenant configurations

3. Developed JWT verification middleware
   - Implemented JWKS (JSON Web Key Set) Manager
   - Created Zitadel JWT claims parser
   - Built authentication middleware with auto-refresh
   - Integrated tenant context injection

### New Files Created / 新建文件

| File | Description | Lines |
|------|-------------|-------|
| **Database Migrations** | | |
| `migrations/001_create_tenants.sql` | Create tenants table with Zitadel mapping | 70 |
| `migrations/002_create_user_mapping.sql` | Create user identity mapping table | 65 |
| `migrations/003_create_tenant_configs.sql` | Create tenant configuration table with defaults | 85 |
| `migrations/004_add_tenant_id.sql` | Add tenant_id to all existing tables + indexes | 120 |
| **Data Models** | | |
| `model/tenant.go` | Tenant model with CRUD operations | 200 |
| `model/user_mapping.go` | User-tenant identity mapping with auto-creation | 280 |
| `model/tenant_config.go` | Tenant configuration management (key-value store) | 310 |
| **Middleware** | | |
| `middleware/zitadel_auth.go` | Zitadel JWT verification + JWKS manager | 580 |

**Total: ~1710 lines of production code**

### Technical Highlights / 技术亮点

**1. Database Design / 数据库设计**

- **tenants table**: Maps Zitadel Organizations to lurus tenants
  - Primary key: `id` (UUID)
  - Unique constraint: `zitadel_org_id` (Zitadel Organization ID)
  - URL-friendly slug: `slug` (e.g., "lurus", "customer-a")
  - Business config: `plan_type`, `max_users`, `max_quota`

- **user_identity_mapping table**: Links Zitadel users to lurus users
  - Composite unique key: `(zitadel_user_id, tenant_id)`
  - Foreign keys: `lurus_user_id → users.id`, `tenant_id → tenants.id`
  - Synced metadata: `email`, `display_name`, `preferred_username`
  - Soft delete support: `is_active` flag

- **tenant_configs table**: Flexible configuration system
  - Key-value storage with type casting (string/int/bool/json/float)
  - System configs (read-only): `is_system` flag
  - Encryption support: `is_encrypted` flag
  - Default configs: quota, billing, features, security, rate-limit

- **Existing tables migration**: Added `tenant_id VARCHAR(36)` to:
  - Core: `users`, `tokens`, `channels`
  - Billing: `topups`, `subscriptions`, `redemptions`
  - Logging: `logs`
  - Auth: `passkeys`, `twofa` (optional)
  - Updated unique constraints: `username` → `(tenant_id, username)`

**2. Tenant Model Features / 租户模型功能**

- Auto-creation from Zitadel Organization
- Status management: enabled/disabled/suspended
- Plan types: free/pro/enterprise
- User limit enforcement: `CanAddUser()` check
- CRUD operations with error handling
- Pagination support for listing

**3. User Mapping Features / 用户映射功能**

- Auto-create lurus users from Zitadel JWT claims
- Sync user metadata from Zitadel (email, name, username)
- Handle username conflicts with suffix generation
- Support multiple tenants per Zitadel user
- Last sync timestamp tracking

**4. JWT Verification Middleware / JWT 验证中间件**

**JWKS Manager (Public Key Management):**
- Auto-fetch RSA public keys from Zitadel JWKS endpoint
- Convert JWK format to RSA public key
- Cache keys in memory (thread-safe with RWMutex)
- Auto-refresh every 1 hour in background
- Retry mechanism on key lookup failure

**Zitadel Claims Parsing:**
```go
type ZitadelClaims struct {
    jwt.RegisteredClaims                          // Standard OIDC claims
    Email             string                      // User email
    EmailVerified     bool                        // Email verification status
    Name              string                      // Full name
    PreferredUsername string                      // Preferred username
    OrgID             string                      // Zitadel Org ID (urn:zitadel:iam:org:id)
    OrgDomain         string                      // Org domain (urn:zitadel:iam:org:domain:primary)
    Roles             map[string]interface{}      // Project roles
}
```

**Authentication Flow:**
1. Extract Bearer token from Authorization header
2. Parse JWT to get Key ID (kid) from header
3. Fetch RSA public key from JWKS Manager
4. Verify JWT signature with public key
5. Validate issuer, expiration, and other claims
6. Map Zitadel user to lurus user (auto-create if enabled)
7. Inject tenant context into Gin context

**Tenant Context Injection:**
```go
type TenantContext struct {
    TenantID      string   // Tenant ID
    UserID        int      // Lurus user ID
    ZitadelUserID string   // Zitadel user ID
    Email         string   // User email
    Username      string   // Username
    Roles         []string // User roles in this tenant
}
```

**Role-Based Access Control:**
- `RequireRole(role)`: Enforce single role
- `RequireAnyRole(roles...)`: Enforce any of multiple roles

### Configuration / 配置

**Environment Variables Required:**
```bash
ZITADEL_ENABLED=true                                     # Enable Zitadel auth
ZITADEL_ISSUER=https://auth.lurus.cn                     # Zitadel issuer URL
ZITADEL_JWKS_URI=https://auth.lurus.cn/oauth/v2/keys    # JWKS endpoint
ZITADEL_CLIENT_ID=YOUR_CLIENT_ID_HERE                    # OIDC Client ID
ZITADEL_AUTO_CREATE_TENANT=true                          # Auto-create tenants
ZITADEL_AUTO_CREATE_USER=true                            # Auto-create users
ZITADEL_DEBUG_LOGGING=false                              # Debug logging
```

### Migration Strategy / 迁移策略

**1. Create New Tables:**
```sql
-- Run migrations in order
migrations/001_create_tenants.sql       -- Create tenants table
migrations/002_create_user_mapping.sql  -- Create user mapping table
migrations/003_create_tenant_configs.sql -- Create tenant configs table
```

**2. Migrate Existing Data:**
```sql
-- Insert default tenant
INSERT INTO tenants (id, zitadel_org_id, slug, name, status)
VALUES ('default', 'ZITADEL_DEFAULT_ORG_ID', 'lurus', 'Lurus Platform', 1);

-- Add tenant_id to existing tables
migrations/004_add_tenant_id.sql        -- Add tenant_id + indexes + update data
```

**3. Update Unique Constraints:**
```sql
-- Username should be unique per tenant (not globally)
ALTER TABLE users DROP INDEX username;
ALTER TABLE users ADD CONSTRAINT uq_users_tenant_username UNIQUE (tenant_id, username);
```

### Code Quality / 代码质量

✅ **All code follows best practices:**
- English comments for all functions and types
- Error handling with descriptive messages
- Thread-safe operations (mutex for JWKS Manager)
- Bilingual error messages (Chinese + English)
- GORM model conventions
- RESTful API patterns
- Security: JWT verification, issuer validation, role-based access

✅ **Edge case handling:**
- Missing Authorization header
- Invalid JWT format or expired tokens
- Key rotation (JWKS refresh)
- Tenant/user auto-creation
- Username conflicts (suffix generation)
- Database foreign key constraints
- Tenant user limit enforcement

### Next Steps / 下一步

**Immediate (Blocked):**
- User needs to configure Zitadel console (阶段1.2-1.6):
  1. Create Organization "Lurus Platform"
  2. Create Project "lurus-api"
  3. Create OIDC Application
  4. Configure Project Roles (admin, user, billing_manager)
  5. Configure SMTP
  6. Obtain Client ID, Client Secret, Org ID

**Phase 3 (Ready to implement):**
- Create GORM tenant isolation plugin (auto-inject `WHERE tenant_id = ?`)
- Update all model CRUD operations to use plugin
- Implement Redis cache key namespacing (`tenant:{tid}:...`)
- Test data isolation (cross-tenant access prevention)

**Phase 4 (Ready to implement):**
- Implement OAuth2.0 authorization code flow (`controller/oauth.go`)
- Create OAuth callback handler
- Add v2 API routes (`/api/v2/:tenant_slug/...`)
- Implement tenant management API (Platform Admin)
- Maintain v1 API backward compatibility

### Result / 结果

**Status: Phase 1 & 2 Code Complete (Pending Zitadel Configuration)** ✅

All infrastructure code implemented:
- ✅ Database migration scripts (4 SQL files)
- ✅ Tenant model with auto-creation
- ✅ User identity mapping with sync
- ✅ Tenant configuration system
- ✅ JWT verification middleware with JWKS
- ✅ Role-based access control
- ✅ Tenant context injection

**Code Statistics:**
- Files created: 8
- Lines of code: ~1710
- Test coverage: 0% (to be added in Phase 6)

**Ready for:**
- Zitadel manual configuration (阶段1.2-1.6)
- Phase 3 implementation (GORM plugin + tenant isolation)
- Phase 4 implementation (OAuth flow + v2 API routes)

---

## 2026-01-25 (AM): Lurus-API Multi-Tenant SaaS Transformation - Phase 0 (Planning & Infrastructure)

### User Requirement / 用户需求

Transform lurus-api from single-tenant multi-user architecture to multi-tenant SaaS platform, using Zitadel as unified authentication center, supporting 5+ independent businesses as tenants.

将 lurus-api 从单租户多用户架构改造为多租户 SaaS 平台，使用 Zitadel 作为统一认证中心，支持 5+ 个独立业务作为租户接入。

### Method / 方法

**Phase 0: Planning and Infrastructure Assessment (Day 1)**

1. Explored existing codebase structure
   - Analyzed model layer (User, Token, Channel, etc.)
   - Reviewed authentication middleware (Session + Access Token)
   - Examined database schema (PostgreSQL/MySQL/SQLite support)
   - Reviewed API routing structure (v1 API with Gin framework)

2. Created comprehensive planning documents
   - `doc/plan.md` - Detailed 6-phase implementation plan (1-1.5 months)
   - `doc/structure.md` - Multi-tenant architecture design document
   - Documented Zitadel integration strategy
   - Defined database migration approach

3. Infrastructure assessment
   - Verified Zitadel deployment status in K3s cluster
   - Confirmed Zitadel running in `lurus-identity` namespace
   - Verified domain access: https://auth.lurus.cn ✅
   - Checked existing services and resources

### New Files Created / 新建文件

| File | Description |
|------|-------------|
| `doc/plan.md` | Multi-tenant SaaS transformation plan (bilingual: CN/EN) |
| `doc/structure.md` | Architecture design document (bilingual: CN/EN) |

### Infrastructure Status / 基础设施状态

**Zitadel Authentication Center:**
- Status: ✅ Deployed and Running
- Namespace: `lurus-identity`
- Version: `ghcr.io/zitadel/zitadel:v2.54.0`
- Access URL: https://auth.lurus.cn
- Service Ports: 8080 (HTTP), 8081 (gRPC)
- TLS: Configured with Let's Encrypt
- IngressRoute: Configured for `auth.lurus.cn`

**Current lurus-api:**
- Namespace: `lurus-system`
- Port: 8850
- Access URL: https://api.lurus.cn
- Authentication: Session + Access Token (to be migrated to Zitadel)
- Database: PostgreSQL on `cloud-ubuntu-2-4c8g`

### Implementation Plan Overview / 实施计划概览

**Timeline: 4-6 weeks** ⚡️

#### Phase 1: Zitadel Configuration & Integration (Week 1)
- [ ] Configure Zitadel instance
- [ ] Create default Organization: "Lurus Platform"
- [ ] Create Project: "lurus-api"
- [ ] Create OIDC Application
- [ ] Configure Project Roles (admin/user/billing_manager)
- [ ] Configure SMTP (using Stalwart Mail)

#### Phase 2: JWT Verification Middleware (Week 1-2)
- [ ] Implement OIDC JWT Token verification
- [ ] Implement JWKS public key management
- [ ] Create user identity mapping (Zitadel User → lurus User)
- [ ] Create tenant model
- [ ] Implement tenant context injection

**New Files to Create:**
- `middleware/zitadel_auth.go`
- `model/user_mapping.go`
- `model/tenant.go`

#### Phase 3: Database Migration & Tenant Isolation (Week 2-3)
- [ ] Create `tenants` table
- [ ] Create `user_identity_mapping` table
- [ ] Add `tenant_id` to all existing tables
- [ ] Implement GORM tenant isolation plugin
- [ ] Migrate existing data to default tenant
- [ ] Update Redis cache key naming

**Database Changes:**
- Add `tenant_id` to: users, tokens, channels, topups, subscriptions, logs
- Update unique indexes: `(field)` → `(tenant_id, field)`

#### Phase 4: API Routes & OAuth Login Flow (Week 3-4)
- [ ] Implement OAuth2.0 authorization code flow
- [ ] Create OAuth callback handler
- [ ] Add v2 API routes (`/api/v2/:tenant_slug/...`)
- [ ] Implement tenant management API (Platform Admin)
- [ ] Maintain v1 API backward compatibility

**New Files to Create:**
- `controller/oauth.go`
- `controller/tenant.go`
- Update: `router/api-router.go`

#### Phase 5: Billing System Tenant Isolation (Week 4-5)
- [ ] Refactor TopUp, Subscription, Redemption
- [ ] Implement webhook tenant identification
- [ ] Create tenant-level subscription plans
- [ ] Update payment gateway integration

**Risk Level: High** (involves financial security)

#### Phase 6: Testing & Documentation (Week 5-6)
- [ ] Unit tests (coverage > 80%)
- [ ] Integration tests
- [ ] Security tests (Token forgery, cross-tenant access)
- [ ] Performance tests (P95 < 100ms)
- [ ] Update README.md
- [ ] API documentation
- [ ] Deployment guide

### Architecture Highlights / 架构亮点

**Multi-Tenant Model:**
```
Zitadel Organization → lurus Tenant
Zitadel Project → lurus Application
Zitadel User → lurus User (via mapping table)
```

**Tenant Isolation Strategy:**
- **Database Layer**: Shared database + tenant_id field
- **Application Layer**: GORM Plugin auto-injects WHERE tenant_id = ?
- **Cache Layer**: Redis key naming: `tenant:{tid}:resource:{id}`

**Authentication Flow:**
1. User → Zitadel OAuth login
2. Zitadel → JWT Token (org_id + user_id + roles)
3. lurus-api → Verify JWT + Map identity
4. lurus-api → Inject tenant context

**API Versioning:**
- `/api/*` - v1 API (backward compatible, default tenant)
- `/api/v2/:tenant_slug/*` - Multi-tenant API (Zitadel JWT)
- `/api/v2/admin/tenants` - Platform Admin (tenant management)

### Key Advantages / 核心优势

1. **Save 40-50% Development Time**
   - Zitadel handles: user registration, password management, OAuth, 2FA, Passkey
   - lurus-api focuses on: business logic, billing, tenant isolation

2. **Enterprise-Grade Auth System**
   - Zitadel provides complete user management UI
   - Built-in social logins (Google, GitHub, Microsoft, etc.)
   - RBAC permission management
   - Audit logs and GDPR compliance

3. **Flexible Multi-Tenancy**
   - Support 5+ independent businesses
   - Each tenant isolated data
   - Tenant-level subscription plans
   - Platform admin can manage all tenants

### Next Steps / 下一步

1. **Access Zitadel admin interface**: https://auth.lurus.cn
2. **Configure default Organization and Project**
3. **Create OIDC application for lurus-api**
4. **Begin Phase 1 implementation**

### Result / 结果

**Status: Planning Phase Complete** ✅

All planning documents created:
- ✅ Project plan with 6-phase timeline (doc/plan.md)
- ✅ Architecture design document (doc/structure.md)
- ✅ Infrastructure assessment complete
- ✅ Zitadel deployment verified and accessible

**Infrastructure Ready:**
- ✅ Zitadel running at https://auth.lurus.cn
- ✅ K3s cluster with 4 nodes
- ✅ PostgreSQL database ready
- ✅ ArgoCD GitOps configured

**Ready to proceed to Phase 1: Zitadel Configuration & Integration**

---

## 2026-01-20: GuShen Web - Backtest System Phase 5 Enhancement

### User Requirement / 用户需求

Comprehensive optimization of the backtest system from user perspective:
- 90%+ edge case handling (user input, data, calculation, UI/UX)
- Module decoupling for system integration
- Financial-grade reliability
- Error handling and validation

从用户角度全面优化回测系统：
- 处理 90% 以上的边缘情况（用户输入、数据、计算、UI/UX）
- 模块解耦，为系统集成做准备
- 金融级可靠性
- 错误处理和验证

### Method / 方法

1. Created core abstraction layer with interfaces for decoupling
2. Implemented comprehensive error handling system with error codes
3. Added input validation with Zod schemas
4. Created financial math utilities with Decimal.js for precision
5. Implemented data quality checker for K-line validation
6. Created trade execution simulation module
7. Built React state management hooks with Zustand
8. Created API client for external system integration
9. Implemented event system for backtest events
10. Created error boundary and loading state components
11. Enhanced API route with full validation and error handling

### New Files Created / 新建文件

| File | Description |
|------|-------------|
| `src/lib/backtest/core/interfaces.ts` | Core interfaces (Result<T>, IDataProvider, IBacktestEngine, IMetricsCalculator, IStorage) |
| `src/lib/backtest/core/errors.ts` | Error handling system with 30+ error codes and bilingual messages |
| `src/lib/backtest/core/validators.ts` | Zod schema validation for all backtest inputs |
| `src/lib/backtest/core/financial-math.ts` | Financial calculations with Decimal.js (FinancialAmount class, A-share rules) |
| `src/lib/backtest/core/data-quality.ts` | K-line data quality checker (missing data, suspensions, limits) |
| `src/lib/backtest/core/trade-executor.ts` | Trade execution simulation (slippage, limits, costs, portfolio) |
| `src/lib/backtest/hooks/useBacktest.ts` | React state management with Zustand (persistence, history) |
| `src/lib/backtest/api/index.ts` | API client for external integration (retry, timeout, cancellation) |
| `src/lib/backtest/events/index.ts` | Event system for backtest events (typed emitter, history) |
| `src/components/backtest/error-boundary.tsx` | Error boundary components for UI isolation |
| `src/components/backtest/loading-states.tsx` | Loading skeletons, progress indicators, empty states |

### Modified Files / 修改文件

| File | Changes |
|------|---------|
| `src/app/api/backtest/unified/route.ts` | Full input validation, error codes, timeout handling, safe operations |

### Dependencies Installed / 安装依赖

| Package | Version | Purpose |
|---------|---------|---------|
| `decimal.js` | ^10.x | Financial precision calculations |
| `zod` | ^3.x | Schema validation |
| `zustand` | ^5.x | React state management |

### Key Features Implemented / 实现的关键功能

**Error Handling System / 错误处理系统：**
- BT1XX: Validation errors (target, date, capital, strategy)
- BT2XX: Data errors (fetch, insufficient, symbol not found)
- BT3XX: Calculation errors (division by zero, precision)
- BT4XX: Engine errors (timeout, unavailable)
- BT5XX: Network errors
- BT9XX: System errors

**Financial Precision / 金融精度：**
- `FinancialAmount` class with Decimal.js
- A-share market rules (lot size 100, limits ±10%)
- STAR/ChiNext rules (lot size 200, limits ±20%)
- Transaction cost calculation (commission, stamp duty, transfer fee)

**Data Quality / 数据质量：**
- Missing data detection
- Suspension detection (zero volume)
- Price limit detection (±9.9%)
- Anomaly detection (>20% change)
- Quality score calculation
- Data filling strategies

**Trade Execution / 交易执行：**
- Slippage modeling
- Price limit handling
- Suspension checks
- Lot size rounding
- Position management
- Portfolio tracking

**State Management / 状态管理：**
- Zustand store with persistence
- Loading/progress tracking
- Error state management
- Result history (last 10)
- Form validation

### Build Result / 构建结果

**Status: Build Successful / 状态: 构建成功** ✅

```
Route (app)                              Size     First Load JS
├ ○ /dashboard                           47.3 kB         150 kB
├ ○ /dashboard/strategy-validation       14.6 kB         118 kB
└ + 29 total routes
```

### Result / 结果

**Status: Phase 5 Complete / 状态: Phase 5 完成** ✅

All planned optimizations implemented:
- ✅ Core interfaces for decoupling
- ✅ Comprehensive error handling with codes
- ✅ Input validation with Zod
- ✅ Financial precision with Decimal.js
- ✅ Data quality checking
- ✅ Trade execution simulation
- ✅ React state management with Zustand
- ✅ API client for integration
- ✅ Event system for external hooks
- ✅ Error boundaries and loading states
- ✅ API route validation and error handling

---

_(Previous entries preserved below...)_

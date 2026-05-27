# Story H1.1 — SAML 2.0 + SCIM 2.0 Multi-IdP Provisioning

**Epic**: 16 (new) — Enterprise Identity Foundation
**Phase**: E1 (Enterprise Foundation, per 12-month Horizon Plan H1.1)
**Priority**: P0 (H1 表面 — 投标必需)
**Status**: ready-for-dev (blocked on Okta trial tenant config — user action)
**Type**: Feature
**Created**: 2026-05-27
**Estimated effort**: 2 周 (Plan W1-2,与 Sγ E2E + H2 Bet A spike 并行)
**Related ADR**: TBD — `doc/decisions/2026-05-27-d1-newapi-retire.md` (D1 fork decision unblocks)

---

## Objective

让企业客户 IdP (Okta / Azure AD / Google Workspace / 国产 IDaaS) 通过:
1. **SAML 2.0 SSO** — 用户登录跳转到 IdP,IdP 签名 AttributeStatement 回 newhub
2. **SCIM 2.0** — IdP 主动推送 user CRUD 到 newhub (`/scim/v2/Users`),无需人工开账户

接入 newhub,**不再依赖 Zitadel-only OIDC + 人工建 newhub 账户**。

## Why now

H1 表面 5 个核心 PR 之一,企业客户标书 100% 出现:
- "支持 SAML 2.0 SSO" — 否则 IT 安全审核不通过
- "支持 SCIM provisioning" — 否则 IT 不愿管理"再一套账户"

无此能力 = 销售对话即刻 disqualify。

H1.0 D1 = Option A 决定后,认证层不再受 newapi 制约,可以彻底重新设计 (Zitadel OIDC 保留为默认,SAML/SCIM 作为企业增量)。

## Architecture

```
┌─────────────────┐         ┌──────────────────────┐
│  Customer IdP   │         │       newhub         │
│  (Okta/Azure)   │         │                      │
│                 │         │ ┌──────────────────┐ │
│  ┌───────────┐  │ SAML    │ │ middleware/      │ │
│  │ SAML IdP  │──┼─POST───>│ │ saml_auth.go     │ │
│  │ endpoint  │  │ Resp    │ │ (NEW)            │ │
│  └───────────┘  │         │ └──────────────────┘ │
│                 │         │          │           │
│  ┌───────────┐  │ SCIM    │ ┌────────▼─────────┐ │
│  │ SCIM      │──┼─POST───>│ │ handler/scim.go  │ │
│  │ Connector │  │ JSON    │ │ (NEW)            │ │
│  └───────────┘  │         │ └────────┬─────────┘ │
└─────────────────┘         │          │           │
                            │ ┌────────▼─────────┐ │
                            │ │ app/scim_sync.go │ │
                            │ │ (NEW)            │ │
                            │ └────────┬─────────┘ │
                            │          │           │
                            │ ┌────────▼─────────┐ │
                            │ │ repo/user.go     │ │
                            │ │ (extend, scim_   │ │
                            │ │  external_id)    │ │
                            │ └──────────────────┘ │
                            └──────────────────────┘
```

## Implementation Plan

### Phase 1 — SAML 2.0 SP (Service Provider) Implementation (W1)

#### 1.1 Choose SAML library
- **决定**: `github.com/crewjam/saml` (最广泛用,active maintenance,GitHub 2.7k★)
- 不选 `russellhaering/gosaml2` — 文档薄,GitHub 600★,Issue 响应慢

#### 1.2 SP endpoint
- `GET /saml/acs` (Assertion Consumer Service) — IdP 把签名 SAMLResponse POST 到这里
- `GET /saml/metadata` — 给 IdP 配置用的 SP metadata
- `GET /saml/login?idp=<slug>` — 触发 SP-initiated SAML flow

#### 1.3 IdP per-tenant config
- 每 tenant 可配置 ≥1 IdP (允许 multi-IdP — 一个企业内多个部门用不同 IdP)
- 新 table `tenant_idp_configs`:
  ```sql
  id BIGSERIAL PRIMARY KEY
  tenant_id INT NOT NULL REFERENCES tenants(id)
  slug VARCHAR(64) NOT NULL          -- 公开标识 (URL /saml/login?idp=okta-corp)
  type VARCHAR(32) NOT NULL          -- saml | oidc (复用同表)
  metadata_url TEXT                  -- IdP metadata URL (动态拉取证书)
  metadata_xml TEXT                  -- 离线场景: 静态 metadata
  cert_pem TEXT                      -- IdP signing cert (优先级低于 metadata 自动同步)
  entity_id TEXT NOT NULL            -- IdP entityID (urn:...)
  sso_url TEXT NOT NULL              -- IdP SSO endpoint
  scim_token VARCHAR(128)            -- SCIM bearer token (IdP -> newhub)
  attr_email VARCHAR(64) DEFAULT 'email'
  attr_name VARCHAR(64) DEFAULT 'displayName'
  attr_groups VARCHAR(64) DEFAULT 'groups'
  is_active BOOLEAN DEFAULT TRUE
  created_at TIMESTAMP DEFAULT NOW()
  UNIQUE (tenant_id, slug)
  ```

#### 1.4 SAML Response 处理
- 验签: 用 `cert_pem` 或动态拉取 metadata 的证书
- 解析 AttributeStatement:
  - `attr_email` → user.email (主键,duplicate 则 update)
  - `attr_name` → user.display_name
  - `attr_groups` → roles 映射 (Phase 2,先存 raw)
- 创建 / 更新 user (自动建 OR 拒绝,看 tenant 配置 `auto_create_on_saml`)
- 启 session (复用现有 Zitadel session 机制)
- 审计: `ActionAuthLoginSAML` (新)

#### 1.5 接入现有 middleware
- `internal/adapter/middleware/saml_auth.go` (NEW) — 新中间件
- 不修改 `zitadel_auth.go` — 两个 IdP 路径独立 (避免 OIDC/SAML 分支耦合)
- `internal/adapter/handler/router/api-v2-router.go` 加 `/saml/*` 路由族

### Phase 2 — SCIM 2.0 Endpoints (W1.5)

#### 2.1 SCIM endpoints (RFC 7644)
- `POST /scim/v2/Users` — Create user (IdP 推)
- `PUT /scim/v2/Users/:id` — Replace user
- `PATCH /scim/v2/Users/:id` — Partial update (RFC 7644 §3.5.2)
- `DELETE /scim/v2/Users/:id` — Soft delete (deactivate, 不真删)
- `GET /scim/v2/Users/:id` — Read user
- `GET /scim/v2/Users?filter=userName eq "foo"` — Query users
- `GET /scim/v2/ServiceProviderConfig` — 元数据
- `GET /scim/v2/Schemas` — 支持的 schema
- `GET /scim/v2/ResourceTypes` — 支持的资源类型

#### 2.2 Bearer auth
- Authorization header `Bearer <scim_token>` — token 从 tenant_idp_configs.scim_token 来
- 失败 → 401 + 审计 `ActionAuthSCIMRejected` (新)

#### 2.3 User schema 映射 (RFC 7643)
- SCIM `userName` → newhub `users.email` (主键)
- SCIM `externalId` → newhub `users.scim_external_id` (新字段)
- SCIM `name.formatted` → newhub `users.display_name`
- SCIM `active` → newhub `users.status` (1=active, 0=deactivated)
- SCIM `groups` → roles 映射 (复用 Phase 1 raw 存储)

#### 2.4 Migration
- `migrations/017_add_scim_external_id.sql`:
  ```sql
  ALTER TABLE users ADD COLUMN scim_external_id VARCHAR(128) DEFAULT '';
  CREATE INDEX idx_users_scim_external_id
    ON users(scim_external_id)
    WHERE scim_external_id <> '';
  ```
- partial index — 不索引 ''(99% 非 SCIM 用户),避免空间浪费

### Phase 3 — Audit + Edge Cases (W2)

#### 3.1 Audit actions (新)
- `auth.login_saml` — SAML SSO 成功
- `auth.scim_user_created` — SCIM create
- `auth.scim_user_updated` — SCIM update
- `auth.scim_user_deactivated` — SCIM delete (soft)
- `auth.saml_rejected` — 验签失败
- `auth.scim_rejected` — bearer auth 失败

#### 3.2 Edge cases
- **Email 冲突**: SAML 给的 email 与现有 newhub 账户重名 → 默认拒绝 + audit;tenant 可配 `merge_on_email_conflict`
- **SCIM 删除 last admin**: 拒绝 + audit + 返回 SCIM `400 + mutability error`
- **IdP cert rotation**: metadata URL 拉取失败 → 用缓存证书 (TTL 1h) + 告警 metric `saml_cert_refresh_failed`
- **Replay**: SAML AssertionID 在 redis 缓存 (TTL = NotOnOrAfter window) 防 replay
- **Multi-IdP per user**: 一个 email 可同时 owned by 多 IdP — 默认拒绝;tenant 可配 `allow_multi_idp_per_email`

### Phase 4 — UI (W2 末)

- `/console/v2/admin/idp` — Tenant admin 配 IdP (metadata URL / cert / scim_token 生成)
- 复用 Semi UI 既有表单组件
- 不在此 story scope:
  - SAML test connector (Phase 1 完成后另开 Story)
  - SCIM provisioning preview (Phase 2 完成后)

## Critical Files

### NEW

| Path | Purpose |
|---|---|
| `internal/adapter/middleware/saml_auth.go` | SAML SP middleware (verify + session) |
| `internal/adapter/handler/saml.go` | `/saml/*` 路由 (ACS, metadata, login) |
| `internal/adapter/handler/scim.go` | `/scim/v2/*` 路由 |
| `internal/app/saml_sp.go` | SAML business logic (assertion 解析 + user upsert) |
| `internal/app/scim_sync.go` | SCIM business logic (User CRUD + role 映射) |
| `internal/app/governance/audit_action.go` | +6 actions (per Phase 3.1) |
| `internal/domain/entity/tenant_idp.go` | TenantIDPConfig entity |
| `internal/adapter/repo/tenant_idp.go` | TenantIDPConfig repo |
| `migrations/017_add_scim_external_id.sql` | users.scim_external_id + idx |
| `migrations/018_create_tenant_idp_configs.sql` | tenant_idp_configs table |
| `_bmad-output/planning-artifacts/story-h1-1-scim-saml-acceptance.md` | 验收记录 (impl 完成后填) |

### MODIFIED (minimal touch)

| Path | Change |
|---|---|
| `internal/adapter/handler/router/api-v2-router.go` | 注册 `/saml/*` + `/scim/v2/*` |
| `internal/domain/entity/user.go` | +SCIMExternalID 字段 |
| `internal/adapter/repo/user.go` | UpdateBySCIMExternalID method |

## Verification

### Unit tests (≥30 cases)
- SAML Response 验签 (good cert / wrong cert / expired / replay)
- AttributeStatement 解析 (3 IdP 厂商 quirk — Okta / Azure / Google)
- SCIM PATCH 操作 (RFC 7644 §3.5.2 op: add / replace / remove)
- SCIM 查询 filter 解析 (eq / co / sw / pr)
- Tenant IDP config CRUD + scim_token rotation
- Edge cases (email conflict, last admin delete, multi-IdP)

### Integration tests (≥10 cases)
- 全流程 SAML login → 创建 user → session 成功
- 全流程 SCIM create → query → patch → delete (soft)
- Bearer token mismatch → 401
- Cert rotation: metadata URL 拉取失败时用缓存

### E2E (Okta trial)
- 在 Okta trial tenant 配 newhub 为 SAML app + SCIM connector
- 操作:
  - Okta 创建用户 "scim-e2e@test.com" → 5min 内 newhub 出现
  - Okta 改用户 displayName → 5min 内 newhub 同步
  - Okta 用户 SP-initiated login → 跳转 newhub 成功登录
  - Okta 删用户 → 5min 内 newhub deactivate (status=0)
- 测试覆盖在新 Sγ spec: `tests/e2e/story-h1-1-okta-scim-saml.spec.ts`

### Audit verification
```sql
SELECT action, COUNT(*) FROM audit_events
WHERE action IN ('auth.login_saml', 'auth.scim_user_created',
                 'auth.scim_user_updated', 'auth.scim_user_deactivated',
                 'auth.saml_rejected', 'auth.scim_rejected')
GROUP BY action;
-- 每个 action 至少 1 row (说明 audit hook 接通)
```

## Out of Scope

- **OIDC 增强** (添加 Microsoft / Google OIDC IdP) — 独立 Story
- **多 IdP role mapping engine** (基于 IdP group 自动赋 newhub role) — Phase 2 后续 Story
- **SCIM Group resource** (RFC 7643 Group 类型) — 仅支持 User,Group 推后
- **SAML SLO** (Single Logout) — 复杂且企业实际很少用;推后
- **SAML IdP-initiated flow** (RelayState 自定义) — 推到下个 Story
- **SCIM /Bulk endpoint** (RFC 7644 §3.7) — 推迟,先靠多次 /Users 调用

## Risks & Mitigations

| 风险 | 缓解 |
|---|---|
| Okta trial 配置出错 → SAML 验签失败,排错慢 | 先写 unit test 覆盖 Okta-style Response (mock 文件) → Okta trial 上来时用 trace logging 隔离问题 |
| SCIM filter 解析复杂 (RFC 7644 §3.4.2 SCIM filter mini-language) | 第一版只支持 `eq` operator;`co`/`sw` 等推后,IdP 实际用 `eq` 最多 |
| cert rotation 没人盯 → SAML 验签静默失败 | metric + alert: `saml_cert_refresh_failed > 0` for 1h → Page |
| `crewjam/saml` 库 panic on malformed Response | 测试 fuzz 5 个 malformed 样本;handler 用 recover() + audit |
| Email-as-primary 与现有 OIDC user 冲突 | 默认拒绝 + admin UI 提示手动 merge;不 silently 覆盖 |

## Dependencies

### Blocking (user action)
- **Okta trial tenant**: Anita 准备好的 Okta 测试 tenant + admin 权限 + SCIM token 生成能力
- 建议 trial 配置文档保存到: `_bmad-output/planning-artifacts/okta-trial-config.md` (sensitive,gitignored)

### Internal
- D1 = Option A (newapi retire) — ✅ 2026-05-27 决定
- E3 audit taxonomy — ✅ done (commit `acc22131`)
- Zitadel session 机制 (复用) — ✅ 已稳定

## Definition of Done

- [ ] 单测 ≥ 30 cases,全绿 (race detector 开启)
- [ ] 集成测试 ≥ 10 cases,全绿
- [ ] Okta trial E2E:create / update / login / deactivate 4 flow round-trip
- [ ] 6 audit actions 验证在 audit_events 落地
- [ ] migration 017 + 018 STAGE 跑过 (`kubectl exec lurus-pg-0 psql newhub -c "\\dt+ users"` 看 scim_external_id 列)
- [ ] `/scim/v2/ServiceProviderConfig` 返回符合 RFC 7644 §5
- [ ] Acceptance doc 写完 (`story-h1-1-scim-saml-acceptance.md`)

## Estimate breakdown

| Phase | LOC est | Days |
|---|---|---|
| 1 SAML SP | ~600 | 3 |
| 2 SCIM endpoints | ~500 | 2 |
| 3 Audit + edge | ~200 | 1 |
| 4 UI | ~300 | 1 |
| Tests | ~800 | 3 |
| **Total** | **~2,400 LOC** | **10 working days** |

与 plan W1-2 时间窗一致。

---

## Appendix: SCIM RFC References

- RFC 7642 — SCIM use cases
- RFC 7643 — SCIM Core Schema (User / Group)
- RFC 7644 — SCIM Protocol
- Okta SCIM developer doc: https://developer.okta.com/docs/reference/scim/
- Azure AD SCIM connector: https://learn.microsoft.com/azure/active-directory/app-provisioning/use-scim-to-provision-users-and-groups

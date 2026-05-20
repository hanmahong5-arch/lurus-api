# 经销商 FAQ —— 10 个最常问的问题

> 每个答案 60~100 字。所有技术声明都引代码 / ADR 出处，没编造。
> 服务：`2b-svc-newhub`（hub.lurus.cn），STAGE: `test-newhub.lurus.cn` on R6。

---

## Q1. 计费怎么算？

**答**：CreditPool 预付池模式。贵司从 Lurus Platform 钱包 topup 到 pool（一次性扣钱包余额，记 `tenant_credit_pool_draws` ledger），EndUser 每次 relay 调用按 `cost = upstream_cost_cny × (1 + markup%)` 从 pool 实时扣减。`max_balance` 可配 ceiling（`-1` = 不限），balance ≤ `alert_threshold_pct`（默认 80%）发 NATS 告警，余额耗尽下次调用 HTTP 402。来源：`tenant_credit_pool.go:228-310` + `pool_balance_check.go:35-80` + ADR `2026-05-09-cost-aware-routing.md` §Q1。

---

## Q2. 我的客户 Key 在你们那能看到吗？

**答**：技术上能（platform admin scope `*` 可查任意租户），合同上不会。**Reseller 自己的 narrow-scope key**（只有 `provisioning` scope）需要在 `internal_api_key_tenants` 白名单表里有 `(api_key_id, tenant_id)` 行才能跨租户，缺行 = 403 fail-closed。Audit log 记录所有跨租户访问（`GET /api/v2/admin/audit/events`，rate-limited，仅 RootJWTAuth）。来源：`provisioning.go:69-89` + `migrations/013_create_internal_api_key_tenants.sql` + `api-v2-router.go:267`。

---

## Q3. 你们撞墙怎么办？

**答**：代码层 ship：(a) per-channel circuit breaker（`internal/pkg/resilience/circuitbreaker.go`，threshold/timeout 可配，env `CB_THRESHOLD` / `CB_TIMEOUT_SEC`），三态 closed/open/half-open；(b) 多 channel fallback（同一模型可挂多个上游渠道）；(c) `IsUpstreamFailure` 过滤器避免 5xx 误判 retry。**诚实补充**：STAGE chaos drill 仍在排期（`sprint-status.yaml` `wave_a_kickoff` 列为待跑），未拿到完整 RTO/RPO 测量数据。pilot 期会优先补齐。

---

## Q4. 我能换模型 / 路由策略吗？

**答**：四种 per-tenant routing mode：`strict`（pin 模型，默认）/ `family-pinned`（同家族内自动选）/ `quality-tier`（同 quality grade 选最便宜）/ `shadow`（双跑对比但不切，gateway drug）。auto-route 必须 opt-in + 带 quality SLA 合同条款（drift 超阈值自动 revert + 退款）。来源：ADR `doc/decisions/2026-05-09-cost-aware-routing.md` §2。**实现状态**：W4~W8+ 分阶段，pilot 期建议先用 `strict` + `shadow` 看数据。

---

## Q5. 上游 provider 涨价我吃亏吗？

**答**：不会。**Wallet pass-through with markup%** 模式：`cost_to_customer = upstream_cost_cny × (1 + markup%)`，markup% 是合同里固定数字，没有"隐藏 margin"。上游涨价直接 pass-through 到 EndUser 账单，贵司和我方都不吃单价波动。Subscription 部分是 fixed monthly（CFO-predictable）。来源：ADR `2026-05-09-cost-aware-routing.md` §Q1 "Decision: subscription unlocks ... + wallet pass-through with fixed markup%"。

---

## Q6. 客户数据保护？

**答**：每租户隔离：(a) 所有 query 加 `tenant_id` filter（schema 级），如 `GetLogsV2` / `ListTokensV2`；(b) 跨租户访问走 `internal_api_key_tenants` 白名单（Phase 2 self-audit 修复）；(c) Zitadel Organization 一对一映射到 newhub tenant，OIDC token 的 org claim 校验。**At rest 加密**：依赖 R1/R6 underlying PG（K8s Secret 管 SQL_DSN，pgvector 加密 TBD）。来源：`api-v2-router.go:103-127` + `provisioning.go:69-89` + `doc/runbook/tenant-onboarding.md`。

---

## Q7. 怎么对账？

**答**：每个 relay 调用产生：(1) 一条 `logs` 行（含 model / tokens / cost / timestamp / token_id），(2) 一条 `tenant_credit_pool_draws` ledger（append-only，含 `pool_id` / `amount` / `log_id` / `reason`）。SQL 对账：`SELECT SUM(amount) FROM tenant_credit_pool_draws WHERE tenant_id=? AND direction=debit` 应等于 `(initial_balance - current_balance)`。Atomic invariant 已测（`TestDebitPool_AtomicRace`：20 goroutine × 10 vs pool 50 → 恰好 5 ok / 15 reject / 0 negative）。来源：`migrations/012_create_tenant_credit_pools.sql:52-74` + `sprint-status.yaml:241`。

---

## Q8. Switch GUI 怎么白标？

**答**：HMAC-SHA256 签名 sidecar。流程：(1) 我方 `GET /api/v2/admin/whitelabel/hmac-key?tenant_slug=<贵司>`（RootJWTAuth，运维代为）→ 拿 64-hex key；(2) 贵司打包流水线用该 Key 签 `whitelabel.json`（含 logo / 默认 endpoint / 租户 Key）；(3) EndUser 启动 Switch 时验签，篡改即拒载入。派生算法 `sha256(master_secret + tenant_slug)`，确定性，无 DB 行。来源：`v2_admin_whitelabel.go:45-97` + ADR `2026-05-20-orphan-features-3-whitelabel-hmac.md`。

---

## Q9. 你们 down 了我怎么办？

**答**：客户端层面：HTTP 503/502 → Switch 内置 retry + 切备用 endpoint。服务端层面：(a) circuit breaker 自动熔断坏 channel + 切兄弟 channel；(b) multi-key pool 同上游多 key 轮转；(c) Postgres HA（WAL-G backup + streaming replica，story `7-2.1` review 状态）；(d) Redis 故障退化到 cookie session（无单点）。**诚实补充**：当前 monthly uptime `~98%`（目标 99.5%），STAGE chaos drill 未跑通，多 region failover 未实现。pilot 期会公开真实数据，不卖目标值。来源：`sprint-status.yaml:26-28` + `epic-7` Reliability Hard Floor。

---

## Q10. 退出怎么办？

**答**：三步：(1) **数据导出** —— `GET /api/v2/{slug}/logs/export` (CSV, hard cap 50k 行 / 次，分页拉)；token 列表通过 `GET /api/v2/{slug}/tokens` 拉 + 通过 Provisioning API `GET /internal/v1/provisioning/tenants/{slug}/keys` 拉。(2) **Key 失效** —— `DELETE` 单 key 软删 + 我方运维一次性 `DELETE` 整个 tenant（`api-v2-router.go:230` `DeleteTenant`，platform admin）。(3) **剩余 Pool 退款** —— 合同条款占位（pool `current_balance` 反向 credit 回钱包，调用 `CreditWalletGRPC`，原子）。**合同退出条款**：待商务侧填，建议 30 天预通知 + 数据保留 90 天。来源：`api-v2-router.go:230` + `tenant_credit_pool.go:279-284` + 合同模板（待）。

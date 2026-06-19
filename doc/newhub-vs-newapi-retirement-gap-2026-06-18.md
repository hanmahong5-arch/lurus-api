# newhub vs newapi — 退役并入 gap 分析 (ADR D1)

> **日期** 2026-06-18 · **目的** 为 newapi（`2b-svc-newapi`，`QuantumNous/new-api` 开源网关）**退役并入 newhub**（ADR D1, 2026-05-27）提供功能/技术差异与差距清单。
> **方法** 7 个子 agent 跨两仓 read-only 对比，每条 claim 带 `path:line` 锚点、否定式 claim 均经 grep 核验；本文 `✓verified` 项由编排方独立重核（codex 目录、本地登录路由、provider 计数）。
> **范围** 对比两仓 HEAD（newhub `lurus-hub` 六边形重构 + V2 多租户 + 平台计费缝;newapi 扁平 MVC 网关）。两者同源 fork（One API → New API）。

---

## TL;DR（结论）

newhub 技术上**全面更现代**（架构/DB/可观测性/多租户/平台计费）。作为 newapi 退役承接方，功能差距分三类：

- **A 有意架构迁移**（非 bug，守 AGPL FREEZE：身份/计费绝不入本 fork）→ 退役前提 = platform/Zitadel 能力上线 + 数据迁移。
- **B 真功能丢失**（无明显承接方）→ 需产品决策是否补齐或弃用。
- **C 风险/回退** → 建议退役前修。

**退役不是"代码补齐"问题，而是"platform 承接 + 数据迁移 + B 类决策"问题。**

---

## 1. 技术差异（newhub modernizations，newapi 几乎全无）

| 维度 | newapi | newhub | 锚点 |
|---|---|---|---|
| 架构 | 扁平 MVC（`controller/model/relay/router/service`） | 六边形（`internal/{domain,app,adapter,pkg,lifecycle}`），业务逻辑限于 `app/`，adapter 向内依赖 | nh `cmd/server/main.go:17-34` |
| DB | 多 DB（SQLite/MySQL/PG via GORM `chooseDB()`） | **PG-only**，非 `postgres://` boot fast-fail | na `model/main.go:118-175` / nh `adapter/repo/main.go:153-161` |
| 迁移 | 仅 GORM AutoMigrate；`migrations/` 仅 1 个未执行文件 | AutoMigrate + **内嵌 SQL runner**（001–020 baseline 只记账，021+ PG-only 幂等 + `pg_advisory_lock`），`schema_migrations` 追踪 | nh `adapter/repo/main.go:226-237`, `internal/pkg/migration/runner.go` |
| 优雅停机 | `server.Run()` 无信号处理 | `signal.NotifyContext` + `errgroup` + `http.Server.Shutdown(30s)` | nh `cmd/server/main.go:53-59,413-425` |
| metrics | 自建滚动桶（非 Prometheus），无 `/metrics` | Prometheus `promauto` 20+ 指标，`GET /metrics`，ServiceMonitor CRD | nh `adapter/handler/router/main.go:27` |
| tracing | 无 OTel | OTel OTLP-HTTP exporter（`OTEL_*` 开关） | nh `internal/pkg/tracing/tracing.go` |
| 熔断 | 无 | 按渠道 3 态 circuit breaker（`CB_*`），状态入 Prometheus | nh `internal/pkg/resilience/circuitbreaker.go` |
| 多租户 | 无 Tenant 模型 | `Tenant/UserIdentityMapping/TenantConfig/TenantCreditPool` + credit-pool 机制 | nh `adapter/repo/main.go:308-327` |
| 计费 | 本地直付（Stripe/Creem/Waffo SDK） | 平台 outbox（gRPC PreAuthorize + 事务 outbox + SKIP LOCKED 重试） | nh `internal/app/billing_outbox.go:34,55` |
| 其它 newhub 增项 | — | 内部服务 API（`/internal/*`）、V2 多租户 API（`/api/v2/*`）、NATS quota、Meilisearch、governance 生命周期任务（审计清理/密钥轮换/PIPL erase）、HA leader-election migration lease、statement timeout 注入 | nh `internal-api-router.go` / `api-v2-router.go` / `internal/lifecycle/` |

---

## 2. Provider / Endpoint parity 摘要

- **Provider**：newapi 36 非-task provider，newhub 35（逐项矩阵）。**唯一缺失 = `codex`**（channel type 57，OpenAI Responses-API-native + OAuth key）✓verified（newapi 有 `relay/channel/codex/`；newhub `provider/codex` 缺）。`taskcommon` 是重构非缺失（并入 `provider/common/`）。newhub 独有 `task/music`。
- **Endpoint**：newapi ~35 个端点组在 newhub 无等价物（详见下文分类）。newhub 独有 V2 多租户 API、内部服务 API、self-service billing（`/v1/billing/*`）、music relay。

---

## 3. 功能差距分类

### A. 有意架构迁移（非 bug；退役前提 = platform/Zitadel 承接 + 数据迁移）

| 项 | newapi 锚点 | 承接方 | 退役动作 |
|---|---|---|---|
| 本地登录/注册/密码重置 ✓verified | `router/api-router.go:67`（`POST /login`）, register, `/user/reset` | Zitadel OIDC | 非-OAuth 用户迁 Zitadel 账号 |
| 2FA(TOTP) ✓verified | `api-router.go:68`（`/login/2fa`）, `/user/2fa/*` | Zitadel MFA | 重新登记 MFA |
| passkey(WebAuthn) ✓verified | `api-router.go:69-70`, `service/passkey/` | Zitadel | passkey 用户迁移脚本 |
| 社交登录（WeChat/Telegram/custom OAuth） | `api-router.go:50,206-215` | Zitadel IdP | 配置等价 IdP |
| 直付 epay/Stripe/Creem/Waffo + webhooks | `api-router.go:56-102` | platform 计费 outbox | platform 上线 + 渠道对接 |
| 订阅系统（plan/订单/生命周期，11+8+4 路由） | `api-router.go:148-178` | platform（待建） | **决策**：platform 是否做订阅 |
| admin 用户 CRUD（newhub 仅 GET/PUT） | na `api-router.go:135-143` vs nh `api-router.go:204-211` | platform / V2 admin | 用 V2/platform 管理流程 |
| client-app HMAC 签名（`/api/client-apps`） | `api-router.go:191-203` | credit-pool + entitlement | ⚠️ Switch/creator 若发 `X-Client-Sig` 将不校验 |

### B. 真功能丢失（无明显承接方，需产品决策）

| 项 | severity | 锚点 | 影响 |
|---|---|---|---|
| **codex provider + `/v1/responses/compact`** ✓verified | HIGH | na `relay/channel/codex/`, `relay-router.go:111`；nh 无 | 库里若存 type 57 渠道 → relay 命中 nil adaptor → 500 |
| channel affinity 粘性路由（966 行） | MED | na `service/channel_affinity.go` | 长上下文会话失去 KV-cache 粘性（取决于是否实际配置规则） |
| 可配置 retry 状态码 | MED | na `setting/operation_setting/status_code_ranges.go` | newhub `shouldRetry()` 硬编码（`relay.go:450-490`）；仅自定义覆盖时有别 |
| task LockedChannel（任务跨调用锁渠道 + 轮 key） | MED | na `relay/relay_task.go`, `controller/relay.go:536` | 视频/图像任务 submit→poll 可能落不同渠道 → 任务孤立 |
| upstream model 自动更新（detect/apply） | MED | na `api-router.go:270-273` | admin 失去 API 同步上游模型能力 |
| perf-metrics/rankings（TTFT/TPS 看板 + 公开榜） | MED/LOW | na `pkg/perf_metrics/`, `api-router.go:34-40` | 控制台性能面板缺失（Prometheus histogram 是否够用？） |
| checkin 签到 | MED | na `controller/checkin.go` | 签到奖励功能消失 |
| affiliate 推广/额度转移 | MED | na `api-router.go:92,105` | 推广返利路径断 |
| 内容安全 vendor 抽象（`relay/safety/`） | MED | na `relay/safety/`（注:vendor 客户端为 stub，实际走 wordlist） | newhub 只 wordlist；若启用 vendor 审核则丢失 |

### C. 风险/回退（建议退役前修）

| 项 | severity | 锚点 | 说明 |
|---|---|---|---|
| **channel key reveal 安全回退** | MED | na `POST`+SecureVerification `api-router.go:240` → nh `GET` 无校验 `api-router.go:121` | newhub 弱化了前置校验门 |
| token key reveal 语义变更 | MED | na reveal（只读）`api-router.go:281` → nh 仅 V2 `rotate` `api-v2-router.go:95` | rotate 破坏性，会作废旧 key；脚本类客户端 404/误轮换 |
| **migration baseline fresh-PG gap** | HIGH | nh `runner.go:14-22`, `adapter/repo/main.go:31` | fresh PG 上 001–020 只记账不执行；AutoMigrate 不复现的 DDL（`006` seed 行 / `004` 复合 UNIQUE / `008` 列删除）将缺 → **DR restore schema 不全**。需 `021_pg_baseline_gaps.sql` 幂等补齐 |
| TopUpV2/GetTopUpsV2 half-wired | LOW | nh `v2_billing.go`（实现）未在 `api-v2-router.go` 挂载 | 实现存在但路由未注册 |
| Dockerfile Go 镜像未 pin | LOW | nh `Dockerfile` `golang:alpine`（浮动） | 可复现性缺口 |

---

## 4. 退役 checklist（actionable）

退役 newapi 前需逐项确认：

1. **[A-身份]** platform/Zitadel 已承接登录/2FA/passkey/社交登录；非-OAuth 与 MFA/passkey 用户已迁移。
2. **[A-计费]** platform 计费 outbox 上线;直付与 webhook 路径已对接;**订阅系统决策**（platform 做 or 弃用）。
3. **[A-admin]** 用户 CRUD 走 V2/platform 流程;client-app HMAC 弃用已确认（或 Switch/creator 改造）。
4. **[B-codex]** 决定是否移植 codex provider + `/v1/responses/compact`;**先查 newapi 生产库是否存在 type 57 渠道**。
5. **[B-其它]** affinity / retry-codes / task-LockedChannel / upstream-update / perf-metrics / checkin / affiliate / safety 逐项决策（补齐 or 弃用）。
6. **[C-migration]** 出 `021_pg_baseline_gaps.sql`,在 fresh PG / DR restore 验 schema 完整;审 001–004（MySQL 方言,永不执行）的 DDL 是否已在 PG 落地。
7. **[C-安全]** channel/token key reveal 语义与门控对齐;挂上 TopUpV2 或移除。
8. **[数据]** newapi 独有模型（TopUp/Subscription*/Checkin/TwoFA*/Passkey/ClientApp/PerfMetric…）的孤儿行归属决策;Redis v8→v9 共享键验证;`migrations/075_log_add_safety_columns.sql` 是否已应用于生产库。

---

## 附:method 与可信度

- 7 个并行子 agent：3 对比（架构 / 网关·provider / 控制台·计费）+ 4 测试（本次同步硬化 newhub 自有薄弱包,见同 PR）。
- 锚点与否定式 claim 由各 agent grep 核验;`✓verified` 项由编排方独立重核。
- 未独立重核项以 agent 报告为准,标 `path:line` 可自查;严重度对 affinity 等"取决于生产是否实际配置"项需 owner 确认。
- 本文为 point-in-time（两仓 HEAD 2026-06-18）;功能会被补齐,退役前应对 HEAD 复核否定式条目。

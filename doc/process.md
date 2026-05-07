# Development Progress / 开发进度

> Last Updated: 2026-02-25
> Archive: doc/archive/process_v20260205.md (entries before 2026-02-04)
> **New Rule**: 每条目 ≤ 15 行（HARD LIMIT），只记录已完成工作的极简摘要

---

## 2026-02-25: 计费系统安全威胁模型修复（P0+P1）

修复 10 个安全漏洞，覆盖无限充值、签名绕过、竞争条件、幂等等问题。
- P0-1 `subscription_cron.go`: netDelta 分三步（扣费→补额→reset daily），TotalQuota=0 不补 quota
- P0-2 `topup_creem.go/subscription_payment.go`: 删除 TestMode 签名绕过，secret 空时直接拒绝
- P0-3 `topup.go`: 删除跨 Pod 无效的 LockOrder/UnlockOrder，EpayNotify 改走 ManualCompleteTopUp（DB FOR UPDATE）
- P0-4 `subscription.go`: ActivateSubscription 加 FOR UPDATE 幂等检查，状态非 Pending 则拒绝
- P0-5 `internal_api.go`: InternalGrantSubscription 补充 RecordLog 审计
- P0-6 `subscription_payment.go`: 金额不足改为返回 error 拒绝激活，容差改固定 50 cents
- P1-1 `rate-limit.go/api-v2-router.go`: 兑换码接口加 5次/分钟 IP 限速
- P1-2 `topup.go`: AdminCompleteTopUp 补充管理员 ID 审计日志
- P1-3 `user.go`: ResetDailyQuota 加 last_daily_reset < todayStart 幂等条件
- P1-4 `auth.go`: lurus-api-User header 改可选（仅验证不作为 ID 来源）

Verification: `go build ./... → OK`; `go vet ./internal/adapter/middleware/... → OK`
Remaining: P2 系列（int64 溢出、CSRF、JWT aud 验证）待排期。

## 2026-02-25: 计费系统评估 + 自动续费实现

完成多产品计费能力评估，修复自动续费 TODO。
- `subscription_cron.go: processOneAutoRenewal()` — 实现余额扣费自动续费，原子事务（双重检查锁 + gorm FOR UPDATE）
- 修正逻辑：扣 `plan.Price * QuotaPerUnit` + 补 `plan.TotalQuota`，net delta 一次写库
- `doc/billing-system-guide.md` — 修正支付宝/微信状态（仅 OAuth，非支付），更新自动续费描述（24h 触发），新增多产品能力评估章节
- 结论：AI 网关计费 ✅ 可撑，多产品中台 ❌ 需独立服务（推荐路径 B）

Verification: `go build ./... → OK`
Remaining: 自动续费余额不足时邮件通知（TODO 标注），退款/发票系统在 lurus-billing 规划中。

---

## 2026-02-13: Epic 6 Complete — Code Review & Security Hardening

**Completed Stories**: 6-1 ~ 6-10 (对抗性审查 + P0/P1/P2 修复)

**Key Deliverables**:
- P0-1: Git history cleanup (git-filter-repo, 5019 commits, secrets.yaml removed)
- P1: Config externalization (MinIO bucket, CORS, alipay prefix → env/constant)
- P2: context.Background() cleanup (22 fixes in 15 files)
- Tests: 40+ new tests (Alipay 14, Release 13, Model Sync 16)
- Docs: DEPLOY.md, TESTING.md, code review reports

**Verification**: `go test ./... → PASS`, `go build → OK`, Git history clean

**Remaining**: Force push cleaned history + credential rotation (out-of-sprint)

---

## 2026-02-11: Download System + SSO Phase 1

Backend API for release downloads + cross-domain SSO (cookie-based).

**Files**: migrations/005, domain/entity/release*, app/release_service, handler/release

**Verification**: `go build → 93MB`, 编译通过

**Status**: ⏳ 待数据库迁移 + MinIO 配置 + 前后端联调

---

## 2026-02-06: Tech Debt Cleanup

Fixed user_mapping insecure password, removed dead code, created 3 ADRs (HA/v1-deprecation/observability).

**Verification**: `go test ./... → PASS`, `go build → OK`

---

## 2026-02-05: Epic 2-5 Complete — Tests, Performance, Observability, DevEx

**Epic 2**: 187 service tests, 100+ adaptor tests, 50 controller tests, 34 security tests
**Epic 3**: Benchmarks (p95 <50ms), object pools, HA deployment (2 replicas + PDB)
**Epic 4**: Prometheus /metrics (11 types), OpenTelemetry tracing, 10 alerting rules
**Epic 5**: OpenAPI spec (45 endpoints), staging env, 6 runbooks

**Verification**: `go test ./internal/pkg/metrics → 7 PASS`, `go test ./internal/pkg/tracing → 9 PASS`

---

## 2026-02-05: Epic 1 Complete — Multi-Tenant Production Launch

Deployed V2 API to K3s with Zitadel OIDC auth.

**Key Commits**: 80323446b, 6232258ad, d85e5d422, d74a16a65

**Verification**: ArgoCD sync, pod 1/1 Running, /api/status→200, V2 login→302 to auth.lurus.cn

---

## 2026-02-04: Architecture Migration — Hexagonal Restructure

Migrated `biz/data/server` → `domain/app/adapter` (hexagonal architecture).

**Verification**: `go build ./cmd/server → PASS`, `go test ./... → PASS`

> **Archive Note**: Entries before 2026-02-04 in `doc/archive/process_v20260205.md`

---

## 2026-02-25: 修复注册流程邮件发送失败

排查 `SendEmailVerification` 全链路，发现三个根因并逐一修复：

1. `SMTPServer=localhost` → 改为 `stalwart.mail.svc`（K8s 集群内服务名）
2. Stalwart brute-force 封锁 Pod CIDR → 在 `stalwart-config` ConfigMap 加 `[server.allowed-ip] "10.42.0.0/16"=true "10.43.0.0/16"=true`，滚动重启
3. `SMTPAccount=noreply@lurus.cn` → Stalwart 内部目录只支持短账户名，改为 `noreply`；`SMTPFrom` 保持完整地址

Verification: `curl https://api.lurus.cn/api/verification?email=tpy@lurus.cn → {"success":true}`；Stalwart 日志确认 `queue.queue-message-authenticated` + `delivery.completed`

---

## 2026-02-25: 配置 DKIM 签名

Stalwart `auth.dkim.sign = 'rsa-lurus.cn'`（selector=`default`）即实际出站签名配置，`session.data.sign` 为入站处理。
关键修复：`%{file:...}%` 宏在 RocksDB 中不展开，需内联 PEM；将 PKCS#8 转为 PKCS#1 写入 config.toml。

**DNS**: 添加 `default._domainkey.lurus.cn TXT "v=DKIM1; k=rsa; p=MIIBIjAN..."`

**Verification**:
- `dig TXT default._domainkey.lurus.cn +short` → 返回完整公钥记录
- 私钥提取公钥与 DNS 公钥 base64 完全一致（openssl 确认 2048-bit RSA）
- 实测：noreply→QQ Mail 投递成功，`DKIM-Signature: v=1; a=rsa-sha256; s=default; d=lurus.cn`

## 2026-02-25: Security Fix Supplement Tests (P0-1/P0-2/P0-4/P0-6/P1-3/P1-4)
Fixed 1 broken test (empty_secret_test_mode now expects false). Added 6 new/extended test files covering processOneAutoRenewal (5 subtests), verifyCreemSubscriptionSignature (4 subtests), amount tolerance validation (4 subtests), ActivateSubscription idempotency, ResetDailyQuota idempotency, lurus-api-User header (4 subtests).
Also fixed GREATEST() SQLite incompatibility in subscription_cron.go/subscription.go via quotaDeductSafe() helper.
Verification: `go test ./internal/adapter/handler/... -run "TestVerifyCreemSignature|TestVerifyCreemSubscription|TestAmountValidation"` → PASS; `go test ./internal/adapter/repo/... -run "TestProcessOneAutoRenewal|TestSubscription_ActivateSubscription_Idempotent|TestResetDailyQuota_Idempotent"` → PASS; `go test ./internal/adapter/middleware/... -run "TestAuthHelper"` → PASS; `go build ./...` → OK.

## 2026-02-25: 测试DB迁移PG + new-api增量融合
Part1: testutil_test.go 重写，移除 glebarez/sqlite，改用 TEST_POSTGRES_DSN；SetupTestDB 创建独立 test_repo_<nano> 数据库，cleanup 时 DROP；quotaDeductSafe 移除 SQLite 分支直接用 GREATEST。
Part2: (2a) stream_scanner.go TrimSuffix("\r")→TrimSpace+空串跳过；(2b) processHeaderOverride 跳过 Accept-Encoding；(2c) GeminiUsageMetadata 加 ToolUsePromptTokenCount，提取 buildUsageFromGeminiMetadata 消除两处重复；(2d) MiniMax 添加 MiniMax-Text-01/MiniMax-01/minimax-text-01；(2e) Gemini 添加 gemini-2.0-flash-lite/2.5-flash-preview-04-17/2.5-pro-preview-05-06/2.0-flash-thinking-exp-01-21。
Verification: `go build ./...` → OK (0 errors). PostgreSQL 集成测试待 TEST_POSTGRES_DSN 注入后验证。

## 2026-03-17: lurus-api 瘦身 — 删除 v1 auth + 前端清理 + P0 幂等修复

**Phase 1-3 (Go backend)**: 删除 30 个文件（checkin/invitation/OAuth/2FA/Passkey/SMS/admin_config），精简 User entity（移除 Password/OAuth IDs/aff 字段），清理 router 路由 ~80 条。认证统一委托 Zitadel OIDC。
**Phase 4 (Frontend)**: 删除 16 个 React 文件（LoginForm/RegisterForm/2FA/Passkey/OAuth/checkin/affiliate），修改 12 个文件移除 v1 auth 调用。净删 ~6,438 行。
**P0 fix**: `InternalTopupBalance` 加 order_id 幂等检查（查 LOG_DB 已有记录），防止 platform 支付重试导致重复充值。

Verification: `go test ./... → 20/20 PASS`; `cd web && bun run build → OK`; `CGO_ENABLED=0 GOOS=linux go build ./... → OK`
Commit: `24477bcd9` pushed to `origin/main`。
Remaining: AuthSettingPage.jsx 仍有 Passkey 管理员配置（死设置，不影响功能）；P1 async wallet bridge 无重试机制。

## 2026-03-18: Zitadel OIDC 登录修复 (3 项)

1. **Hairpin NAT**: Pod 内 `auth.lurus.cn` 解析到公网 IP 43.226.46.164，回连被拒。修复: deployment 加 `hostAliases` 指向 Traefik ClusterIP `10.43.175.138` + NetworkPolicy 增 kube-system 443/8443 出站规则。
2. **Email fallback**: `CreateUserFromZitadelClaims` 增加 email 回退匹配（跨租户），老用户首次 OIDC 登录自动建 mapping。
3. **租户数据迁移**: root/marvin 的 `tenant_id` 从 `default` 迁移至 `356204220778610952`；tokens/channels/logs/redemptions 同步迁移；重复用户 (id=6,7) 已软删除。

Verification: `kubectl exec ... wget auth.lurus.cn/.well-known/openid-configuration → OK`; Pod OIDC token exchange 畅通。
Commits: `6600e6c52` (ZitadelRedirect), `bd6fe89dc` (email fallback), deploy `5bbb940` (NetworkPolicy).

## 2026-05-05 · 12 个月战略规划落地为 BMAD artifacts

将"从多租户 LLM 网关 → LLM 治理与成本平台"的战略转化为可执行 epic：
- **Phase 1 (Q2)**: E7 可靠性硬基建 / E8 newapi fork 解耦 / E9 modality 瘦身
- **Phase 2 (Q3)**: E10 5-min TTFT / E11 v1 web 退役 / E12 套餐计费
- **Phase 3 (Q4)**: E13 ClickHouse Insights / E14 PII 审计 / E15 智能路由
- **Phase 4 (2027-Q1+)**: 外部 SaaS 准备

战略决策 D1-D4 已 accepted（停止 fork / 内部优先 / 砍 Tier-3 modality / 自建 Insights）。
新增 north star metrics（产品视角替代纯基础设施视角）。
Active sprint 2026-Q2-S1: 并行启动 Story 7-1（provider 熔断器）+ Story 8-1（fork 审计）。

Artifacts: `_bmad-output/planning-artifacts/{epics.md, sprint-status.yaml, story-7-1-*, story-8-1-*}`

## 2026-05-05 · Story 8-1 fork 审计 — 颠覆性发现

按"自己决策"原则先做 8-1（廉价高杠杆调研）。审计揭示 lurus.yaml 描述与代码事实严重脱节：

- newapi / newhub **不是派生关系**，是 `songquanpeng/one-api` 的两个独立 fork
- newhub 无 upstream remote、零 newapi 引用、Lurus 原创仅 2,183 LOC（governance/hub/openrouter_pool/openrouter_sync/nats）
- 双线演化：90 天 newapi 211 commits、newhub 117 commits，各搞各的 quota / 计费 / 事件机制
- newapi prod (87k LOC) / newhub stage (89k LOC)，89% 同源代码重复

→ Epic 8 语义从 "fork decoupling" 变更为 **"consolidation"**。推荐 Option A: retire newapi，整合到 newhub。

Deliverables:
- `doc/decisions/2026-05-05-newapi-newhub-fork-audit.md`（含 Q1-Q4 决策项）
- `scripts/audit/fork-ownership.sh`（quarterly 重跑）
- Story 8-1 → review（待 Anita 选 Option A/B/C）
- Story 8-2/8-3/8-4/8-5 已重写为 Option A 执行路径，blocked 状态


## 2026-05-07 · Story 7-1 完成 — audit 改写 + 失败分类外科修复

按"自己决策"启动 Story 7-1。Recon 阶段发现既有实现：
- `internal/pkg/resilience/circuitbreaker.go` 已有完整 Registry + 状态机
- `internal/adapter/handler/relay.go:242,304,313` 已接入 (Allow / RecordSuccess / RecordFailure)
- `internal/pkg/metrics` 已有 CircuitBreakerState/Trips/Rejections 三指标
- 测试套件 `circuitbreaker_test.go` 7 个 case 全过

唯一真 gap: RecordFailure 不区分 4xx/5xx —— 用户错误也会跳闸，影响其他租户。

Surgical fix:
- 新增 `types.IsUpstreamFailure(*NewAPIError) bool` (37 行) + 17 个表驱动测试
- relay.go:313 加 if-wrap (4 行) —— 唯一调用点

总变更 ~95 LOC（原 Story 估 400 LOC）。Karpathy ②: 简单优先生效。

Verification:
- go build ./internal/... ✅
- go test ./internal/pkg/types/... ✅ 17/17
- go test ./internal/pkg/resilience/... ✅ 无回归
- handler 包 TestListTokensV2_Pagination panic 预存在（git stash 验证 main 同失败）

Status: 7-1 → review (审计 + 修复完成；STAGE 故障注入演练后 → done)

## 2026-05-07 · "全做" 三 Story 收尾

按用户"全做"指令，三 story 并行：

**8-2 newapi port list** (audit only):
- 90 天 newapi 211 commits 过滤 → Lurus 原创 ~12 条
- 4 项需移植 (P0×2 / P1×2): cost-spike protection, auth hardening, llm.image.generated event, usage milestone
- ~2 工作日，待 Anita confirm Option A 后启动 PR
- Skip: admin api (newhub 已等价)、session config (已等价)、deploy/ops、上游 sync

**7-4 quota real backpressure** (code change ~65 LOC):
- 审计揭示 backpressure 已存在: enforceTenantQuota 返回 402 + ErrorCodeTenantQuotaExceeded
- 真 gap = Retry-After header 仅对 OpenRouter 池冷却返回
- 修复: tenant_quota.go 设 RetryAfterUnix=nextMonthStartUnix; relay.go 通用 Retry-After 段
- 运维侧建议: TENANT_QUOTA_ENFORCEMENT_ENABLED 默认翻 true (manifest 决策)
- 测试: NextMonthStartUnix 跨年 / 5 个 tenant_quota 测试全过

**7-2 PG HA ADR** (decision doc, no code):
- 现状: R6 单节点 docker-compose pg:15, 无备份无副本
- 反对 Patroni: 单 host = 假 HA, 4-6 周 ops 投资在 1 tenant 阶段过度
- 推荐分阶段:
  - Phase 1.0 (1 天) wal-g backups → MinIO + restore runbook
  - Phase 1.1 (2 天) streaming replica (待第 2 host)
  - Phase 2.x (3 天) managed PG (待 5+ 付费租户)
- 决策文档: lurus/doc/decisions/2026-05-07-newhub-pg-ha.md (含 Q1-Q4)

Verification:
- go build ./internal/... ✅
- 3 story 测试全过 (IsUpstreamFailure 17/17, NextMonth 3/3, EnforceTenantQuota 5/5)
- 既有 resilience / types / app 包无回归

Sprint 进展: 5 个 review 状态 story (7-1, 7-4, 7-2 ADR, 8-1, 8-2). 下一推荐 7-2.1 (备份, 1 天) — 移除最大数据风险。

## 2026-05-07 (cont.) · Story 7-2.1 wal-g 备份链路完成

按 7-2 ADR Phase 1.0 落地，~1 天工作量真实写到代码：

**6 个新增/修改文件**:
- `deploy/single-node/Dockerfile.postgres-walg` — postgres:15 + wal-g v3.0.0 binary
- `deploy/single-node/archive-wal.sh` — wrapper 处理 WALG_S3_PREFIX 空值（fresh boot 不卡住）
- `deploy/single-node/docker-compose.yml` — PG 切自定义镜像，加 archive_mode=on + archive_command + archive_timeout=60s（RPO ≤1min）
- `deploy/single-node/.env.example` — 7 个新 WALG_* 变量含运维注释
- `doc/runbook/pg-restore.md` — §A 全量 / §B PITR / §C 单表 三条路径 + 故障排查矩阵
- `scripts/pg-restore-drill.sh` — 月度自动化 drill（throwaway PG → fetch LATEST → 表存在断言 → PASS/FAIL）

**Verification**:
- `docker compose config` 渲染 ✅（修了 .env 内联注释被当 value 的 bug）
- `bash -n scripts/pg-restore-drill.sh` ✅
- archive_command 渲染干净（弃用 $$ escape 改用 wrapper script）

**SLO**: RPO ≤5min / RTO ≤30min / 月度 drill 100% pass
**部署**: 操作手册见 Story doc，~10 步 R6 操作

**Sprint 进展**: 6 个 review story (7-1, 7-2 ADR, 7-2.1, 7-4, 8-1, 8-2)。Phase 1 加固期实质性推进。

## 2026-05-07 (cont.) · v2 polish + Story 8-2.1 cost-spike port

**v2 frontend polish**:
- `prettier --write` 把 19 个 hi-fi 文件格式化干净
- HFShell: useLocation 自动派生 active state（不再每页硬编码 active 属性）
- HFShell: topbar 加亮/暗主题切换按钮（持久化到 localStorage `lurus-hf-theme`）
- vite build ✅

**Story 8-2.1 cost-spike protection port** (newapi → newhub):
- Per-user 5-min 滑动窗口 / Redis ZSET / 阈值默认 50000 quota units
- 触发后自动 DisableUserById + HTTP 429
- Files: 6 文件改/新增, ~210 LOC
- 接入点: relay-router.go `relayV1Router.Use(CostSpikeLimit)` after TokenAuth
- 记录点: PostConsumeQuota 异步 hook 内调用 RecordCostSpikeWindow
- 防御场景: agent 死循环、token 流量爆炸 → 单用户 5min 内烧掉巨额钱包
- Tests: 8/8 (parseCostSpikeMember 边界用例) ✅
- go build ✅
- DoD 缺最后一项: STAGE 注入测试


## 2026-05-07 (cont.) · 8-2.2 + 8-2.3 ports done

**8-2.2 auth hardening**:
- 审计揭示原 8-2 audit 过度——newapi commit `da3cb48f` 实际只含 4 个 sub-fix，其中 3 个 newhub 已就位（pprof bind 127.0.0.1 / authHelper comma-ok / DeleteUser 不同 shape）
- 真 gap：VideoProxy 没有 ownership 检查 = 跨租户视频泄露漏洞
- Fix: video_proxy.go 加 `task.UserId != requesterID && role < admin → 403` + 结构化日志
- ~22 LOC，go build pass

**8-2.3 NATS llm.image.generated**:
- Newhub NATS 基础设施已具备（`internal/pkg/nats/publisher.go`），加 typed event helpers
- Files: events.go (130 LOC) + events_test.go (41 LOC) + image_handler.go (+15)
- 公认 envelope 格式跨 newapi/newhub/2l-svc-platform 三方 compat (event_id UUID dedup)
- truncateRune rune-safe 80 字符截断（CJK 安全）
- 4 个 fail-open 路径（nil publisher / userID≤0 / marshal err / publish err）
- image_url 暂留空（newapi fc49e72d 的 response-writer 包裹延后到 8-2.3.1）
- 10/10 unit tests pass


## 2026-05-07 (cont.) · 8-2.4 NATS llm.usage.milestone port

**决策**: 不与现有 `quota.threshold` 合并，加新事件类型
- threshold = 月度% (50/80/95/100% of MaxQuota), 计费单位, 告警语义
- milestone = 终生绝对值 (1k/10k/100k/1M tokens), token 单位, 成就语义
- 跨 fork wire compat：newapi 已分离两类，保持一致

**Files**:
- usage_milestone.go (117 LOC) — tier ladder + INCRBY + SETNX claim
- usage_milestone_test.go (80 LOC, 9 tests) — 纯函数 crossedMilestones + no-op safety
- compatible_handler.go +7 — postConsumeQuota 末尾单点 hook（mirror newapi 的 hook 位置）

**No-op fallbacks**: invalid userID, totalTokens≤0, Redis disabled, INCRBY error
**TTL**: 1 年 dedup 键 — 内存有界，覆盖任意保留窗口
9/9 tests pass · go build pass · DoD 缺 STAGE 事件流验证


## 2026-05-07 (cont.) · 8-2.3.1 image URL extraction follow-up

**Goal**: 让 8-2.3 的 `image_url` 不再永远空，inbox 可渲染缩略图

**Approach**: tee-style `BufferedResponseWriter` 包裹 `c.Writer` 一次（在 ImageHelper 入口）
- 64 KiB 硬上限 buffer，OOM 不可能（用户字节透传不变）
- 3-tier shape walker: OpenAI/DALL-E `{"data":[{"url":...}]}` → Replicate `{"output":...}` → 通用 `*url*` depth-1/2 fallback
- 适配 OpenAI/DALL-E/Replicate/Ali/Jimeng/Gemini/Zhipu (6 个 provider 均归一化到 OpenAI shape)

**Files**:
- response_capture.go (217 LOC) — 写器 + 解析器
- response_capture_test.go (287 LOC, 21 tests) — byte-identical pass-through, cap 边界, 各种 shape, garbage/HTML safety
- image_handler.go (+13/-7) — 入口包裹 + 出口 ExtractImageURL

**Invariants verified**: 字节透传零修改，64 KiB cap 即使 200MB 也安全，HTML/garbage/truncated 不 panic
21/21 tests pass · go build pass · DoD 缺 STAGE 真实事件验证


## 2026-05-07 (cont.) · 9-1 Tier 3 modality usage audit

**Goal**: D3 决策已 accepted（砍 MJ/Suno/Realtime/Music/Video），量化代码足迹便于排期

**Findings**:
- 后端 7,082 LOC + ~46 文件可删
  - MJ: 1,781 LOC · Suno: 314 · Music: 228 · Realtime: 134+43branches · Video: 4,625
- 8 个 ChannelType 待禁用 (2/5/36/50/51/52/54/55)
- 2 张 DB 表 (`midjourneys` / `tasks`) drop
- 关键安全发现：`provider/task/{ali,doubao,gemini,jimeng,vertex}/` 是 video task 变体；chat 实现在 `provider/{name}/`，删除任务路径不影响 chat
- Realtime 在 `provider/openai/relay-openai.go` 有 43 行分支，必须同步删，避免死代码

**排期建议**: 9-1 (this) → 9-2 announce (1d, 需 PROD usage 数据 <5% 收入 / <10 租户/模态) → 90d → 9-3 删码 (2d)

**Live data 待操作员**: 3 条 SQL 写在 story 文档里，跑 PROD `consume_logs` 即可
DoD 缺 PROD 用量数据；纯码审计完成


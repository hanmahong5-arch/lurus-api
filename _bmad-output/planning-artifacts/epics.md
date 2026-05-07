# Lurus Hub - Epic Breakdown

> Last Updated: 2026-05-05
> Service: lurus-hub
> Status: Epic 1-6 完成，Epic 7-15 已规划 (12 个月路线图)

---

## Strategic Direction (2026-05 ~ 2027-Q1)

> **从"多租户 LLM 网关"进化为"LLM 治理与成本平台"**
> 减：modality / fork / auth 模式 / 心智模型
> 深：可靠性 / 观测性 / 合规审计 / 自助体验

| Phase | Window | Theme | Outcome |
|-------|--------|-------|---------|
| **Phase 1** | 2026-Q2 (10 周) | 加固期 | 99.5% SLO 达标，可靠性达到能开新租户的下限 |
| **Phase 2** | 2026-Q3 (10 周) | 入口期 | 5 分钟 TTFT，首次自助付费租户 |
| **Phase 3** | 2026-Q4 (12 周) | 差异化期 | Insights 平台 + 合规审计 = 护城河 |
| **Phase 4** | 2027-Q1+ | 外溢期 | 走出 Lurus 生态，公开 SaaS |

---

## Completed Epics (Archive)

| Epic | Status | Completion Date | Evidence |
|------|--------|-----------------|----------|
| Epic 1: Multi-Tenant Production Launch | ✅ Done | 2026-02-05 | Commits: 80323446b, 6232258ad, d85e5d422 |
| Epic 2: Test Coverage & Quality Gate | ✅ Done | 2026-02-05 | 187 tests PASS, commits: 5bd1263d1, d85e5d422 |
| Epic 3: Gateway Performance & Reliability | ✅ Done | 2026-02-05 | Benchmarks p95 <50ms, commit: fa958e431 |
| Epic 4: Observability Stack | ✅ Done | 2026-02-05 | 16 tests PASS, commits: 85f8e20cc, fa958e431 |
| Epic 5: Developer Experience & Documentation | ✅ Done | 2026-02-05 | 6 runbooks, commits: fa958e431, a2d1222d8 |
| Epic 6: Code Review & Security Hardening | ✅ Done | 2026-02-13 | 3 story docs, commits: f1b102ac6, 439a282c4 |

---

## Active Epics (Phase 1 — 2026-Q2)

### Epic 7: Reliability Hard Floor (P0)

**Goal**: 在租户从 1 → 10 之前，把可靠性基础打到能开新租户的下限。
**Success Criteria**: 月度 SLO ≥99.5%；任一 provider 故障不影响其他 provider 流量；R6 单节点宕机 RTO ≤5 分钟。

| Story | Title | Priority | Status |
|-------|-------|----------|--------|
| 7-1 | Per-Provider Circuit Breaker | P0 | ready-for-dev |
| 7-2 | PostgreSQL HA (Patroni / 主从 + 自动 failover) | P0 | backlog |
| 7-3 | Relay Data Plane 横向扩展 (无状态 + L4 LB) | P0 | backlog |
| 7-4 | Quota 真背压 (NATS 通知 → 强制 429) | P0 | backlog |
| 7-5 | Chaos drill + SLO 仪表盘 | P1 | backlog |

### Epic 8: Newapi / Newhub Consolidation (P0) — *renamed 2026-05-05*

**Audit finding (Story 8-1)**: 原命名 "Fork Decoupling" 基于错误假设。代码事实是 **newapi 和 newhub 是两个独立 fork**（共同祖先 `songquanpeng/one-api`），互不依赖、并行演化。`lurus.yaml` 的"newhub 派生于 newapi"叙事是意图而非事实。
→ 决策文档：`lurus/doc/decisions/2026-05-05-newapi-newhub-fork-audit.md`

**Goal**（修正）: 选定整合策略并执行，消除并行演化的维护税。推荐 Option A：**retire newapi，整合到 newhub**。
**Success Criteria**: 单仓维护；hub.lurus.cn 接管全部 LLM gateway 流量；newapi.lurus.cn 平滑切走或 CNAME。

| Story | Title | Priority | Status |
|-------|-------|----------|--------|
| 8-1 | Newapi/Newhub fork ownership audit | P0 | ✅ review (audit done) |
| 8-2 | Newapi 近期 Lurus 原创移植清单 + 移植 PR | P0 | blocked (Anita decision) |
| 8-3 | Provider parity audit (newhub vs newapi) | P0 | blocked (after 8-2) |
| 8-4 | 切流方案 (流量镜像 + 灰度 + 回滚) | P0 | blocked (after 8-3) |
| 8-5 | newapi 归档 + lurus.yaml 修订 | P1 | blocked (after 8-4 cutover) |

### Epic 9: Modality Slim-Down (P1)

**Goal**: 砍掉 30+ provider × 11 modality 的兼容矩阵中长尾 6 项，专注 Tier-1。
**Success Criteria**: RelayFormat 类型从 11 → 5；维护负担与事故面相应缩减。

| Story | Title | Priority | Status |
|-------|-------|----------|--------|
| 9-1 | 用量审计：确认 Tier-3 (MJ/Suno/Realtime/Music/Video) 真实流量 | P1 | backlog |
| 9-2 | 公告 90 天 deprecate 窗口 + 客户迁移路径 | P1 | backlog |
| 9-3 | Tier-3 代码移除 + 测试清理 | P2 | backlog |

---

## Backlog Epics (Phase 2 — 2026-Q3)

### Epic 10: 5-Minute Time-to-First-Token (P0)
新租户从注册到首次成功 API 调用 ≤5 分钟。
- 10-1 自助注册 + $5 试用额度（platform 钱包预存）
- 10-2 子域名路由 `<tenant>.api.lurus.cn` 替代 path slug
- 10-3 默认 channel pool 自动分配
- 10-4 单页 Quick Start 文档 + 可 copy curl

### Epic 11: v1 Web UI 退役 (P1)
保留 v1 token-only 兼容；OIDC-only web UI；路由表减半。
- 11-1 v1 web UI 流量分析与迁移路径
- 11-2 v2 OIDC 后台对齐 v1 功能（最后差距）
- 11-3 v1 web UI 灰度下线

### Epic 12: 计费分层 (P0)
Free / Pro / Enterprise 套餐替代纯计量计费，降低客户心智成本。
- 12-1 套餐 SKU 与 platform-core 对齐
- 12-2 用量超额 / 套餐升级流程
- 12-3 Self-serve 升级页面

---

## Backlog Epics (Phase 3 — 2026-Q4)

### Epic 13: ClickHouse Insights Plane (P0)
租户自服务成本仪表盘 / 异常告警 / 模型对比。
- 13-1 ClickHouse 接入 + OTel gen_ai.* schema
- 13-2 租户成本仪表盘 (Next.js)
- 13-3 成本异常告警 (3σ / threshold)
- 13-4 模型对比报告 (cost/latency/quality)
- 13-5 对外 OpenAPI (客户 pipe 自家 Datadog)

### Epic 14: PII Audit & Compliance (P0)
出站 token 流 PII 扫描 + 审计存档 + 数据居留地标记，企业付费理由。
- 14-1 PII 扫描器 (presidio / 规则 + LLM 双策略)
- 14-2 审计日志不可篡改存档 (S3 object lock)
- 14-3 数据居留地标签与路由约束

### Epic 15: 智能路由 (P1)
成本/延迟/可用性多目标，自动 failover & 套利。
- 15-1 路由策略引擎 (cost / latency / quality 权重)
- 15-2 自动 failover (基于 7-1 熔断状态)
- 15-3 成本套利 (相同模型多 provider 价差自动选择)

---

## Phase 4 — 2027-Q1+ (Direction Only, Not Yet Sprinted)

- 公开注册 + Stripe 直付（脱离 platform-core 钱包强依赖）
- 与 Helicone / Langfuse 差异化定位（治理 + 计费一体）
- PaaS 化：开放 OpenAPI 给三方接入

---

## Active Sprint (2026-Q2 Sprint 1)

**Window**: 2026-05-05 ~ 2026-06-02 (4 周)
**Focus**: 启动 Phase 1，并行推进 E7 + E8 起点。

| Story | Owner | Status |
|-------|-------|--------|
| 7-1 Per-Provider Circuit Breaker | TBD | ready-for-dev |
| 8-1 Code ownership audit | TBD | ready-for-dev |

后续 Story 在 7-1 / 8-1 落地后基于实际情况展开。

---

## Notes

- 所有 Phase 1+ Story 必须遵循 dev-story workflow，含验证证据才可标 done
- Story 文档存放: `_bmad-output/planning-artifacts/story-X-Y-<slug>.md`
- North Star 指标见 sprint-status.yaml `north_star_metrics:` 段

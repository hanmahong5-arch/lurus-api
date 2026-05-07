# Story 8-1: Newapi/Newhub Code Ownership Audit

**Epic**: 8 - Newapi Fork Decoupling → **renamed: Newapi/Newhub Consolidation** (per audit)
**Priority**: P0
**Status**: review (audit complete; awaiting Anita decision on Option A/B/C)
**Type**: Investigation (no production code changes)
**Created**: 2026-05-05
**Completed**: 2026-05-05

---

## Audit Outcome — 颠覆性发现

调查初期假设（"newhub 是 newapi 的下游 fork"）与代码事实**完全不符**：

| 假设（来自 lurus.yaml） | 代码事实 |
|------------------------|---------|
| newhub 基于 newapi 衍生 | 两个 repo 各自独立从 `songquanpeng/one-api` fork |
| 月度 cherry-pick newapi → newhub | newhub 无 upstream remote、零跨引用、零依赖 |
| newapi 是基座，newhub 在其上 | 两个并行 fork，89% 同源代码重复演化 |

→ Epic 8 语义从 "decouple" 变更为 **"consolidate"**（整合）。

## Deliverables

- ✅ `lurus/doc/decisions/2026-05-05-newapi-newhub-fork-audit.md` — 完整审计报告 + Option A/B/C
- ✅ `lurus/scripts/audit/fork-ownership.sh` — 可重跑的量化脚本（已 sanity-test）
- ✅ 推荐 Option A：retire newapi，整合到 newhub

## Quantitative Snapshot (2026-05-05)

```
2b-svc-newapi  files=511  LOC=87053  90d-commits=211  env=prod
2b-svc-newhub  files=629  LOC=89232  90d-commits=117  env=stage(R6)

Lurus-original (newhub-only):
  governance/         294 LOC
  hub/                601 LOC
  openrouter_pool/    360 LOC
  openrouter_sync/    810 LOC
  nats/               118 LOC
                  --------
                   2,183 LOC  ← 真正的差异化资产

newhub → newapi cross-references: 0
```

## Objective

回答一个看似简单但目前没有定量答案的问题：

> **newhub 当前代码库中，有多少是从 New API 上游 fork 来的、可以同步的；有多少是 Lurus 原创、不该回上游的？**

这个数字决定了 Epic 8 的全部走向：
- 如果原创 < 30%：newhub 应该作为薄层叠加，newapi 作为基座二进制
- 如果原创 30~60%：模块化 + 接口抽象，newapi 作为 Go module 依赖
- 如果原创 > 60%：fork 已实质性"分家"，应该正式独立、停止追上游

## Success Criteria

| # | 验收物 | 形式 |
|---|--------|------|
| 1 | 全代码 ownership 表格 | `doc/decisions/newapi-fork-audit.md` |
| 2 | 按目录 / 包 / 文件粒度归类 | 三档：Upstream / Modified / Original |
| 3 | 量化指标：LOC + 文件数 + 包数 | 表格 |
| 4 | 上游同步成本估算 | 过去 3 个月 cherry-pick 次数 + 冲突解决耗时 |
| 5 | 给出三种解耦路径 + 推荐 | sidecar / library / 独立 fork |
| 6 | 输入到 Story 8-2 (decision doc) | 直接引用 |

## Investigation Plan

### Step 1: 确定 fork 起点

```bash
# 找到 newhub 与 New API upstream 的 fork-base commit
git -C 2b-svc-newhub log --oneline --reverse | head -1
# 期望: 这是 Lurus 接管时的初始 commit，对应 New API 的某个 tag
```

输出: 起始 commit + upstream tag (e.g. `QuantumNous/new-api@v0.5.x`)

### Step 2: 三档分类规则

```
Upstream    = 与上游 tag 完全一致 (git diff <tag> -- <file> 为空)
Modified    = 上游存在 + 本地修改 (有 diff)
Original    = 上游不存在 (新增文件，e.g. tenant_*.go, governance/*, hub/*)
```

实施: 通过 `git diff <upstream-tag> -- <path>` 逐文件分类，脚本输出 CSV。

### Step 3: 按粒度统计

| 粒度 | 输出 |
|------|------|
| 目录级 | 哪些目录 100% original (一定不回上游)：`internal/app/hub`, `internal/app/governance`, `internal/app/openrouter_pool`, `internal/pkg/nats`, etc. |
| 包级 | 哪些包是 Modified 主战场（最大同步冲突来源） |
| 文件级 | 高 LOC 修改文件 top 20 |

### Step 4: 历史同步成本

```bash
# 过去 90 天的上游同步 commit
git log --since="90 days ago" --oneline --grep="upstream\|cherry-pick\|sync"
```

衡量: 次数、平均冲突文件数、估计人时。

### Step 5: 解耦路径分析

| 路径 | 含义 | 优势 | 劣势 |
|------|------|------|------|
| **A. Sidecar** | newapi 独立部署，newhub 调用 newapi REST | 完全解耦 | 多一跳，延迟 +5-10ms |
| **B. Library** | newapi 抽成 Go module，newhub `import` | 进程内调用，无延迟 | 仍受 module API 约束 |
| **C. 正式独立** | 停止追上游，承认 newhub 是独立产品 | 自由度最大 | 失去上游 bug 修复 |

每个路径估算实施工作量（人周）。

### Step 6: 推荐 + 决策文档草稿

输出 `doc/decisions/newapi-fork-audit.md` （含表格 + 推荐 + 风险）。
后续 Story 8-2 在此基础上做正式 ADR (Architecture Decision Record)。

## Files to Produce

| File | Purpose |
|------|---------|
| `doc/decisions/newapi-fork-audit.md` | 主审计报告 |
| `scripts/audit/fork-ownership.sh` | 可重跑的统计脚本（未来定期 audit 用） |
| `_bmad-output/planning-artifacts/story-8-1-newapi-fork-audit.md` (本文件) | Story 跟踪 |

**注意**: 本 Story 不修改任何 production 代码。

## Definition of Done Checklist

- [x] 找到两个 repo 的 fork base（共同祖先：`songquanpeng/one-api`）
- [x] 完成 ownership 分类（独立 fork 关系，非派生）
- [x] 统计 LOC / 文件数 / 包数
- [x] 90 天 commit velocity 回溯
- [x] 三种路径分析 + 工作量估算
- [x] `doc/decisions/2026-05-05-newapi-newhub-fork-audit.md` 完成
- [x] `scripts/audit/fork-ownership.sh` 可重跑（已验证输出）
- [x] 推荐路径（Option A）写入决策文档
- [ ] **Anita review + Option A/B/C 选择** ← 唯一未完项

## Estimated Effort

**8-12 小时**（脚本 + 分析 + 文档）。一周内完成不阻塞其他 Story。

## Risks

| Risk | Mitigation |
|------|------------|
| Fork-base commit 找不到（历史断裂） | 退化方案：用最近的 New API 公开 release 作 anchor，按"如果今天从这个 release 起"反推 |
| Modified 文件分类边界模糊（小改动算 Modified？） | 设阈值：diff >5 LOC 算 Modified，否则 Upstream（轻微 patch 不影响同步） |

## References

- 当前 module path: `github.com/LurusTech/lurus-hub`
- 上游可能的源: `QuantumNous/new-api` (CLAUDE.md 提及)
- 决策文档目录: `doc/decisions/` (governance repo 共享)

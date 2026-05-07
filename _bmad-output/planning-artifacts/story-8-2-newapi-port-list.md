# Story 8-2: Newapi 90-Day Lurus-Original Port List

**Epic**: 8 - Newapi/Newhub Consolidation (Option A)
**Priority**: P0
**Status**: review (audit done; 待 Anita 确认 Option A 后执行 port PR)
**Type**: Investigation + port plan
**Created**: 2026-05-07
**Depends on**: 8-1 (Option A 决策)

---

## Method

1. `git -C 2b-svc-newapi log --since="90 days ago"` — 90 天 commit 全集
2. 排除 upstream sync (`merge:`, `Merge pull request`, `(#NNNN)` 上游 PR 编号)
3. 排除 deploy/ops only (image bumps, k8s tweaks — 这类 newhub 自己的 deploy 已独立)
4. 剩余 = "Lurus 原创 in newapi"
5. 对每条 grep newhub 验证是否已存在等价实现

## Outcome

90 天 newapi 共 **211 commits**，过滤后 Lurus 原创 **~12 条**，其中 **4 条需要移植到 newhub**（Option A 切流前必须达到 parity）。

---

## Port List (按优先级)

### P0 — 切流前必须移植（功能 gap）

#### 1. Cost-Spike Protection (per-user 5-min sliding window)

| 项 | 内容 |
|----|------|
| Newapi 提交 | `86163316` feat / `dfdf6d9b` middleware wire / `cc7969cc` PostConsumeQuota wire |
| 功能 | 单用户 5 分钟内消耗超过 `COST_SPIKE_HARD_LIMIT_PER_5MIN`（默认 50000 quota 单位）→ DisableUserByID 强制停用 |
| Newhub 状态 | ❌ `grep cost.?spike` = 0 命中，完全缺失 |
| 防御场景 | agent 死循环、token 流量爆炸 → 单用户瞬间烧掉巨额钱包 |
| 移植路径 | `middleware/cost_spike.go` (newapi) → `internal/adapter/middleware/cost_spike.go` (newhub) + 接入 PostConsumeQuota |
| 估算工时 | 0.5 天 |
| 风险 | 与 newhub 既有 quota_threshold (50/80/95/100%) 是不同维度 —— 互补，不冲突 |

#### 2. Auth Hardening (timing-safe compare + pprof guard)

| 项 | 内容 |
|----|------|
| Newapi 提交 | `da3cb48f` fix(auth) |
| 功能 | (a) `subtle.ConstantTimeCompare` 替代 `==` 防 timing attack；(b) pprof 端点加 auth middleware |
| Newhub 状态 | ❌ `grep subtle.ConstantTimeCompare` = 0；pprof 仅绑定 127.0.0.1（弱保护） |
| 触动文件 (newapi) | `controller/user.go`, `controller/video_proxy.go`, `main.go`, `middleware/auth.go` |
| Newhub 对应位置 | `internal/adapter/handler/user_handler.go`, `cmd/server/main.go` (pprof 段), `internal/adapter/middleware/auth.go` |
| 估算工时 | 0.3 天 |
| 风险 | 安全修复，零风险，应优先 |

### P1 — 切流前应移植（功能等价但有差异）

#### 3. NATS Event: `llm.image.generated` payload + image_url 回填

| 项 | 内容 |
|----|------|
| Newapi 提交 | `9ef4e6db` feat / `fc49e72d` image_url backfill |
| 功能 | 图像生成成功后向 `LLM_EVENTS` 流发布 `llm.image.generated`，含 image_url、tokens、cost |
| Newhub 状态 | ⚠️ 有 NATS publisher (e3ce272a) + quota_threshold events，但**没有 image-generated event**（grep `image.generated` = 0） |
| 下游消费方 | memorus / admin 看板（依据 lurus.yaml `LLM_EVENTS` consumer 列表） |
| 估算工时 | 0.5 天（newhub 已有 `internal/pkg/nats` 基础设施，只需加一个 publish 点 + payload 类型） |
| 风险 | 下游期望此事件；切流后若缺失会有数据缺口 |

#### 4. Usage Milestone NATS Event

| 项 | 内容 |
|----|------|
| Newapi 提交 | 同 `9ef4e6db` |
| 功能 | 用户累计用量到达里程碑（如 1万 / 10万 quota）发 `llm.usage.milestone` |
| Newhub 状态 | ⚠️ 有 quota_threshold (50/80/95/100% of MaxQuota) 但语义不同：threshold 基于配额比例，milestone 基于绝对用量 |
| 决策 | 评估是否需要第二种事件，或可用 quota_threshold 替代 |
| 估算工时 | 0.3 天 + 决策时间 |
| 风险 | 若下游消费方依赖 absolute milestone，必须移植 |

---

## Skipped — 不需要移植

| 提交 | 类型 | 原因 |
|------|------|------|
| `427ab9df` admin per-user API-key upsert | 已在 newhub 等价实现 | newhub `internal/adapter/handler/internal_api.go` 已有 `/api/admin/api-keys` 全套端点 |
| `0a8a7a61` rename cookie to lurus-session | 已在 newhub | newhub 用 SESSION_COOKIE_DOMAIN（commit 4f7ae227），机制等价 |
| `d0fb79db` Redis session store + cookie flags | 已在 newhub | newhub main.go 已 import `gin-contrib/sessions/redis`（不同 vendor 但功能等价）|
| `1443d5e5` k8s HPA/PDB/securityContext | 不需要 | newhub 有自己的 deploy/k8s/，Option A 切流后由 newhub deploy 接管 |
| `c438a3b6` deploy 配置移除 circuit-breaker | 不需要 | newhub 的 deploy 与 newapi 不共享 |
| 其他 image bump、deploy 配置类 | 不需要 | newhub deploy 独立 |
| `471d1e17/d11904d3` Gemini TTS / 3.1 系列 | 上游 | 走 newhub 自己的上游同步路径 |
| `4a4cf0a0/0da0d806` 其他 fix | 上游 | 同上 |

---

## Total Effort

| 项 | 工时 |
|----|------|
| P0-1 cost-spike | 0.5 天 |
| P0-2 auth hardening | 0.3 天 |
| P1-3 image.generated event | 0.5 天 |
| P1-4 usage milestone（含决策） | 0.3 天 |
| 集成测试 + STAGE 验证 | 0.4 天 |
| **合计** | **~2 个工作日** |

加上 8-3 (provider parity) 和 8-4 (cutover) 估算，整个 Option A 整合在 Phase 1 内完全可行。

---

## Definition of Done Checklist

- [x] 90 天 newapi commits 拉全
- [x] 分类 (Lurus-original / upstream / deploy)
- [x] 每条 Lurus-original commit 在 newhub 验证存在性
- [x] 输出 P0/P1/Skipped 分级
- [x] 估算工时
- [ ] **Anita confirm Option A** → 启动各 P0 项的 PR（每项独立 commit）
- [ ] 4 项移植 PR 全部 merge
- [ ] STAGE 集成测试通过
- [ ] 8-3 (provider parity) 启动

---

## Reproducibility

```bash
# Re-run audit (Q3 等)：
cd lurus
bash -c 'cd 2b-svc-newapi && git log --since="90 days ago" --oneline | grep -vE "Merge pull request|Merge branch|^[a-f0-9]+ \w+:.*\(#[0-9]+\)"'
# 然后逐条对每个新增的 (newapi-only) commit 在 newhub grep 验证
```

## References

- 8-1 audit: `lurus/doc/decisions/2026-05-05-newapi-newhub-fork-audit.md`
- newhub NATS infra: `internal/pkg/nats/`
- newhub admin api: `internal/adapter/handler/internal_api.go`
- newhub session config: `cmd/server/main.go` (SESSION_COOKIE_DOMAIN 段)

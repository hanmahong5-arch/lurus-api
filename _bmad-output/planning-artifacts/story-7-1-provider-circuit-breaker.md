# Story 7-1: Per-Channel Circuit Breaker — Audit + Failure-Classification Fix

**Epic**: 7 - Reliability Hard Floor
**Priority**: P0
**Status**: review (audit + surgical fix complete; await build/E2E in STAGE)
**Type**: Audit + targeted code change
**Created**: 2026-05-05
**Updated**: 2026-05-07

---

## Audit Outcome — 重大调整

原 Story 假设 "grep -r circuit\|breaker → 0 result" → 需新建 `internal/pkg/breaker` + 接入 `api_request.DoRequest`。

**事实**: 范围扩大到 `internal/adapter/handler` 后发现**已有完整实现**：

| 组件 | 路径 | 状态 |
|------|------|------|
| 状态机 (Closed/Open/HalfOpen) | `internal/pkg/resilience/circuitbreaker.go` | ✅ 已实现 |
| Per-channel Registry | 同上 | ✅ |
| 单元测试（state transitions / 隔离 / half-open / reset） | `internal/pkg/resilience/circuitbreaker_test.go` | ✅ 7 个测试 |
| Prometheus 三指标 (state/trips/rejections) | `internal/pkg/metrics/metrics.go` | ✅ |
| 接入 relay handler 重试循环 | `internal/adapter/handler/relay.go:242,304,313` | ✅ |
| Env vars 配置（`CB_THRESHOLD`, `CB_TIMEOUT_SEC`） | `resilience.DefaultConfig()` | ✅ |
| 状态变化日志 | OnStateChange callback → SysLog | ✅ |

**唯一真实 gap**: `RecordFailure(channel.Id)` 被无差别调用 —— 任何 `newAPIError != nil` 都计入失败，包括 4xx 用户错误和 `context.Canceled`。一个发坏请求的用户能把健康渠道的熔断器搞跳闸，影响其他租户。

→ Story 范围从 "build + integrate" 缩到 "**add upstream-failure classifier + apply at single call site**"。

## Success Criteria → Reality Mapping

| # | 原 SC | 当前状态 |
|---|------|---------|
| 1 | Provider 连续失败 → OPEN，快速失败 | ✅ 既有逻辑：OPEN 时跳过 channel，由 retry loop 切换其他 channel |
| 2 | OPEN → HALF-OPEN cooldown，探测成功恢复 | ✅ `resilience` 包已实现 |
| 3 | (provider, channel_id) 隔离 | ✅ Per-channel 隔离（每 channel 1 provider，等价） |
| 4 | Prometheus 指标 | ✅ `lurus_gateway_circuit_breaker_{state,trips_total,rejections_total}` |
| 5 | 状态变化结构化日志 | ✅ 通过 OnStateChange callback |
| 6 | 阈值环境变量 | ✅ `CB_THRESHOLD` / `CB_TIMEOUT_SEC`（与原 Story 命名不同但等效；不重命名以保持兼容） |
| **NEW** | 4xx / ctx.Canceled 不计入失败 | ✅ 本次新增 |

## Delivered Changes

### 1. `internal/pkg/types/error.go` — 新增 `IsUpstreamFailure()`

```go
func IsUpstreamFailure(err *NewAPIError) bool {
    if err == nil { return false }
    if IsChannelError(err) { return true }
    if err.Err != nil && errors.Is(err.Err, context.Canceled) { return false }
    switch err.StatusCode {
    case http.StatusRequestTimeout, http.StatusGatewayTimeout, 524:
        return true
    }
    if err.StatusCode/100 == 5 { return true }
    if err.StatusCode/100 == 4 { return false }
    return true // fail-safe: unclassified → upstream
}
```

| Input | Result | Rationale |
|-------|--------|-----------|
| nil | false | no error |
| `channel:*` 任何 status | true | network/key issue |
| `ctx.Canceled` (含 wrap) | false | user disconnected |
| 408 / 504 / 524 | true | upstream timeout |
| 5xx | true | upstream server error |
| 4xx (其他) | false | user error |
| 0 / 未分类 | true | fail-safe |

### 2. `internal/pkg/types/error_test.go` — 新增（17 个 case）

Table-driven 覆盖上面所有矩阵。`go test ./internal/pkg/types/... → ok`.

### 3. `internal/adapter/handler/relay.go:313` — wrap RecordFailure

```diff
- channelBreakers.RecordFailure(channel.Id)
+ if types.IsUpstreamFailure(newAPIError) {
+     channelBreakers.RecordFailure(channel.Id)
+ }
```

唯一调用点（grep 验证），handler 改动量 4 行。

## Files Touched (final)

| File | Change | LOC |
|------|--------|-----|
| `internal/pkg/types/error.go` | +1 import (context), +37 行 IsUpstreamFailure | +38 |
| `internal/pkg/types/error_test.go` | NEW | +56 |
| `internal/adapter/handler/relay.go` | wrap RecordFailure | +4/-1 |

总计 ~95 LOC (vs 原 Story 估算 ~400 LOC)。Karpathy ②: 简单优先。

## Verification

| 验证 | 命令 | 结果 |
|------|------|------|
| Build | `go build ./internal/...` | ✅ pass |
| 新单测 | `go test -run IsUpstreamFailure -v ./internal/pkg/types/` | ✅ 17/17 |
| 既有 breaker 测试无回归 | `go test ./internal/pkg/resilience/...` | ✅ 7/7 |
| handler 包测试 | `go test -short ./internal/adapter/handler/...` | ⚠️ TestListTokensV2_Pagination 预存在 panic（与本变更无关，已确认 stash 后 main 同样失败） |

## Definition of Done Checklist

- [x] 审计既有实现，识别真实 gap
- [x] 新增 `types.IsUpstreamFailure()` + 17 个表驱动测试
- [x] 单一接入点 wrap（relay.go:313）
- [x] go build pass
- [x] 新测试 pass
- [x] 既有 resilience 测试无回归
- [ ] STAGE 部署 + 故障注入演练（注入 4xx vs 5xx 流量，验证仅 5xx 计入）— 待 deploy window
- [ ] 更新 sprint-status.yaml 至 done
- [ ] doc/process.md 摘要

## Out of Scope (留给 7-2~7-5 或后续)

- 给 metrics 加 `provider` label（当前只有 `channel_id`，每 channel 1 provider 等价；改 label 会破坏既存 dashboard）
- Env var 重命名（`CB_THRESHOLD` → `CIRCUIT_BREAKER_FAILURE_THRESHOLD`；保持现状避免破坏 deploy）
- Task relay 路径熔断（currently 无熔断保护；非阻塞，Phase 1 内单开 7-1b 处理）
- Registry.Cleanup() 周期调用（防内存增长；当前规模 < 100 channel × 200B 可忽略）

## References

- 既有 breaker 实现: `internal/pkg/resilience/circuitbreaker.go`
- 既有测试: `internal/pkg/resilience/circuitbreaker_test.go`
- 接入点: `internal/adapter/handler/relay.go:242,304,313`
- 失败分类函数: `internal/pkg/types/error.go:IsUpstreamFailure`
- shouldRetry 比对: `relay.go:406` (注：retry policy 与 failure policy 不同，故未直接复用)

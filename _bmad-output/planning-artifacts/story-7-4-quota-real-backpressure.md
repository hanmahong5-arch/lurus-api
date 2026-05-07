# Story 7-4: Quota Real Backpressure — Audit + Retry-After Generalization

**Epic**: 7 - Reliability Hard Floor
**Priority**: P0
**Status**: review (audit + Retry-After fix done; STAGE 验证待跑)
**Type**: Audit + targeted code change
**Created**: 2026-05-07

---

## Audit Outcome

原 Story 假设："50/80/95/100% NATS 事件只通知不阻塞，需加强制阻塞返回 429"。

**事实**: 阻塞**已存在**，且符合 RFC 语义：

| 阻塞层 | 位置 | 触发 | 状态码 |
|--------|------|------|--------|
| User-level quota | `pre_consume_quota.go:86` | `userQuota - preConsumedQuota < 0` | 402 Payment Required |
| Tenant-level quota | `tenant_quota.go:57` (called from `pre_consume_quota.go:98`) | `used + preConsumedQuota > tenant.MaxQuota` | 402 Payment Required |
| 50/80/95/100% NATS events | `quota_threshold.go` | progress milestone | 通知，非阻塞（设计如此）|

**402 vs 429**: RFC 7231 — 402 = "需要付费" (quota/billing exceeded)，429 = "速率限制，等会儿重试"。当前选 402 正确。

**真实 gap**:
1. **`Retry-After` header 仅对 OpenRouter 池冷却返回**，租户配额超限和用户额度不足都不带 Retry-After，客户端不知何时重试
2. **`TENANT_QUOTA_ENFORCEMENT_ENABLED` 默认 false** —— tenant 阻塞在生产实质禁用（运维决策，非代码 bug）

→ Story 范围调整为：**通用化 Retry-After 机制**，让任何携带 `RetryAfterUnix` 的错误自动写头。

## Delivered Changes

### 1. `internal/app/tenant_quota.go` — 设置 RetryAfterUnix

```diff
  if used+int64(preConsumedQuota) > tenant.MaxQuota {
      ...
-     return types.NewErrorWithStatusCode(...)
+     apiErr := types.NewErrorWithStatusCode(...)
+     apiErr.RetryAfterUnix = nextMonthStartUnix(time.Now().UTC())
+     return apiErr
  }
+
+ func nextMonthStartUnix(now time.Time) int64 {
+     return time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, time.UTC).Unix()
+ }
```

`time.Date` 自动 normalize 月份溢出（Dec→Jan 次年），无需特判。

### 2. `internal/adapter/handler/relay.go` — 通用 Retry-After

```diff
  if newAPIError.GetErrorCode() == types.ErrorCodeChannelAllKeysCooling && newAPIError.RetryAfterUnix > 0 {
      ... cooling-specific 503 path (unchanged) ...
  }
+
+ // Surface RetryAfterUnix as Retry-After for any error that carries it
+ // (e.g. tenant monthly quota exceeded → next-month rollover).
+ if newAPIError.RetryAfterUnix > 0 {
+     if secs := newAPIError.RetryAfterUnix - time.Now().Unix(); secs > 0 {
+         c.Header("Retry-After", strconv.FormatInt(secs, 10))
+     }
+ }
+
  switch relayFormat { ... }
```

未来其他错误码若也想 Retry-After，只需 set `RetryAfterUnix` 字段，无需再改 handler。

### 3. `internal/app/tenant_quota_test.go` — 测试

- `TestEnforceTenantQuota_OverLimit_Denies` 扩展：验证 `err.RetryAfterUnix == nextMonth.Unix()` 且 `> now`
- `TestNextMonthStartUnix` 新增：3 个 case（中月 / 12 月跨年 / 月初）

## Files Touched

| File | Change | LOC |
|------|--------|-----|
| `internal/app/tenant_quota.go` | +RetryAfterUnix set + nextMonthStartUnix helper | +12/-1 |
| `internal/app/tenant_quota_test.go` | 扩展 OverLimit + 新增 NextMonth 测试 | +44 |
| `internal/adapter/handler/relay.go` | +通用 Retry-After 段 | +9 |

总 ~65 LOC。

## Verification

| 验证 | 命令 | 结果 |
|------|------|------|
| Build | `go build ./internal/...` | ✅ |
| Existing tenant quota tests | `go test -run EnforceTenantQuota -v ./internal/app/` | ✅ 5/5 |
| 新 OverLimit RetryAfter 断言 | 同上 | ✅ |
| NextMonthStartUnix 跨年 | `go test -run NextMonthStart -v ./internal/app/` | ✅ 3/3 |

## Out of Scope (运维 / 后续)

- **`TENANT_QUOTA_ENFORCEMENT_ENABLED` 默认翻 true**: 这是部署时的 env var 决策，不是代码 bug
  → 建议 R6 STAGE 部署 manifest 加 `TENANT_QUOTA_ENFORCEMENT_ENABLED=true`，灰度验证 1 周后写入 PROD manifest
- **User-level Retry-After**: 用户余额不足无明确"何时能用"（需用户主动充值），故不写 Retry-After（符合 RFC 语义：absence = 客户端自定）
- **短突发 429 限流层**: 与月度 402 quota 不同维度。需要时单开 Story（候选名 7-6 burst rate-limit）

## Definition of Done Checklist

- [x] 现有 backpressure 路径审计
- [x] tenant_quota.go 设 RetryAfterUnix
- [x] relay.go 通用 Retry-After header（cooling 路径保留）
- [x] tenant_quota_test.go 验证 RetryAfterUnix 设置正确
- [x] NextMonthStartUnix 跨年测试
- [x] go build pass
- [x] go test pass
- [ ] STAGE 部署，curl 测试观察 Retry-After header（与 7-1 故障演练同步进行）
- [ ] 运维侧决定是否翻 `TENANT_QUOTA_ENFORCEMENT_ENABLED` 默认值

## References

- 既有阻塞: `internal/app/pre_consume_quota.go:86,98`
- 既有 402 错误: `internal/app/tenant_quota.go:57`
- RetryAfterUnix 字段定义: `internal/pkg/types/error.go:110`
- 既有 Retry-After 写头: `internal/adapter/handler/relay.go:117-131` (channel cooling)

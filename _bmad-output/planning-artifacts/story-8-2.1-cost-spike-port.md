# Story 8-2.1: Cost-Spike Protection Port (newapi → newhub)

**Epic**: 8 - Newapi/Newhub Consolidation
**Priority**: P0
**Status**: review (code + test done; STAGE deploy pending)
**Type**: Backend port from newapi
**Created**: 2026-05-07

---

## Goal

Port `cost_spike` per-user 5-min sliding window protection from newapi to newhub.
Defends the wallet against agent loops / runaway scripts that burn through quota
faster than the monthly cap can intervene. On breach, the user account is
auto-disabled and the request is rejected with HTTP 429.

Without this, a buggy agent could drain a tenant's whole balance in minutes
before tenant-MaxQuota or daily-quota checks fire (they trigger at much higher
thresholds).

## Source

newapi commits (per Story 8-2 audit):
- `86163316` feat(newapi): cost-spike protection per-user 5-min sliding window
- `dfdf6d9b` feat(newapi): wire cost-spike middleware into relay router
- `cc7969cc` fix(newapi): wire RecordCostSpikeWindow into PostConsumeQuota

## Files

| File | Type | LOC | Purpose |
|------|------|-----|---------|
| `internal/pkg/common/constants.go` | mod | +9 | `CostSpikeProtectionEnabled`, `CostSpikeHardLimitPer5Min` vars |
| `internal/pkg/common/init.go` | mod | +3 | Env loading: `COST_SPIKE_PROTECTION_ENABLED` (default true), `COST_SPIKE_HARD_LIMIT_PER_5MIN` (default 50000) |
| `internal/adapter/repo/user.go` | mod | +10 | New `DisableUserById(id)` helper (single-column UPDATE) |
| `internal/app/cost_spike.go` | NEW | 86 | `QueryCostSpikeWindow`, `RecordCostSpikeWindow`, `parseCostSpikeMember` |
| `internal/app/cost_spike_test.go` | NEW | 27 | 8 table-driven cases for parser edge cases |
| `internal/adapter/middleware/cost_spike.go` | NEW | 70 | `CostSpikeLimit()` Gin middleware |
| `internal/adapter/handler/router/relay-router.go` | mod | +3 | Mount middleware after `TokenAuth`, before `EntitlementCheck` |
| `internal/app/quota.go` | mod | +2 | Call `RecordCostSpikeWindow` in PostConsumeQuota async hooks (both pre-auth + legacy paths) |

Total: ~210 LOC (vs newapi reference ~150 LOC; newhub adds graceful nil-RDB
handling not present in source).

## Behavior

```
                                                        ┌─ user already over 50k? → 429 + DisableUserById
/v1/* relay  →  TokenAuth  →  CostSpikeLimit  ─────────┤
                                                        └─ ok → continue → relay → PostConsumeQuota → ZAdd window
```

**Fail-open conditions** (middleware lets request through):
- `CostSpikeProtectionEnabled = false` (kill switch)
- `userID == 0` (unauthenticated — let downstream handle)
- `RedisEnabled = false` or `RDB == nil` (infra issue, don't block legit traffic)
- Redis ZRem/ZRange returns error (logged, request continues)

**Configurable via env** (loaded in `common.init()`):
- `COST_SPIKE_PROTECTION_ENABLED=true|false` (default true)
- `COST_SPIKE_HARD_LIMIT_PER_5MIN=50000` (quota units, ~$0.10 at typical ratios)

**Storage** (Redis ZSET, identical layout to newapi for cross-fork compat):
- Key: `cost_spike:user:<userID>`
- Score: `time.Now().UnixMilli()`
- Member: `<ts_ms>:<tokens>` (string)
- TTL: 600s (auto-cleanup on idle users)
- Window evict: ZREMRANGEBYSCORE on each query

## Verification

| Check | Result |
|-------|--------|
| `go build ./internal/...` | ✅ pass |
| `go test -run ParseCostSpike -v ./internal/app/` | ✅ 8/8 |
| Existing `internal/app/...` tests no regression | ✅ |
| Existing `internal/adapter/middleware/...` no regression | ✅ |
| Manual: env-var defaults applied at boot | not yet (need STAGE) |

## What This Doesn't Cover

- **No miniredis-backed integration test** — newapi has one but newhub doesn't yet vendor `alicebob/miniredis`. Trusted via parser unit test + the underlying logic being byte-identical to newapi's already-deployed code.
- **DisableUserById doesn't invalidate user cache** — next call hits stale cache for ≤`SyncFrequency` (60s default). Acceptable: at most 60s of further traffic before fully blocked. Could tighten by adding `repo.InvalidateUserCache(id)` later.
- **No admin "re-enable" flow** — once auto-disabled, admin must `UPDATE users SET status=1 WHERE id=...` manually. Newapi also doesn't ship one. Add to Settings v2 page in a follow-up.
- **No metric** — should expose `lurus_cost_spike_triggered_total{user_id}` for ops alerting. Quick follow-up.

## Operator Verification (R6 STAGE)

After deploy:

```bash
ssh root@100.122.83.20

# Confirm env loaded
docker exec lurus-api env | grep COST_SPIKE
# → COST_SPIKE_PROTECTION_ENABLED=true (or whatever you set)
# → COST_SPIKE_HARD_LIMIT_PER_5MIN=50000

# Synthetic breach test (load-test gen 51k tokens in <5min for one user)
# → expect: HTTP 429, user.status flipped to 2, structured log line

docker exec lurus-redis redis-cli ZRANGE cost_spike:user:1 0 -1 WITHSCORES
# → see populated window after a successful relay
```

## Definition of Done Checklist

- [x] Add CostSpike vars to common.constants
- [x] Wire env loading in common.init
- [x] Add `DisableUserById` repo helper
- [x] Implement `app/cost_spike.go` (Query/Record/parser)
- [x] Implement `middleware/cost_spike.go` (CostSpikeLimit)
- [x] Wire middleware in relay-router after TokenAuth
- [x] Call recorder in PostConsumeQuota async hooks
- [x] Parser unit test (8 cases)
- [x] go build + go test pass
- [ ] STAGE deploy + synthetic breach test
- [ ] sprint-status → done

## References

- Source audit: `_bmad-output/planning-artifacts/story-8-2-newapi-port-list.md`
- Newapi origin: `2b-svc-newapi/middleware/cost_spike.go`, `service/quota.go:569`
- Newhub middleware mounting: `internal/adapter/handler/router/relay-router.go:71`
- Recorder call sites: `internal/app/quota.go:608, 622`

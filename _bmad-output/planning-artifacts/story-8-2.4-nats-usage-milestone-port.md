# Story 8-2.4: NATS llm.usage.milestone Event Port (newapi → newhub)

**Epic**: 8 - Newapi/Newhub Consolidation
**Priority**: P1
**Status**: review (port done; STAGE event-stream verification pending)
**Type**: Backend port from newapi
**Created**: 2026-05-07

---

## Goal

Wire `llm.usage.milestone` NATS event so the unified notification inbox
(`2l-svc-platform/modules/notification`) surfaces lifetime token-usage
achievements (1k / 10k / 100k / 1M tokens cumulative) per user.

Without this, only chat completions and image generations surface in the
inbox — users never see "你已经累计使用 100k tokens" achievement-style
messages, weakening the engagement loop.

## Source

- newapi `9ef4e6db` feat(events): publish image.generated + usage.milestone to LLM_EVENTS
- Reference: `2b-svc-newapi/service/usage_milestone.go` + `service/event_publish.go`

## Decision: Add as new event type, NOT merge

newhub already has `llm.quota.threshold` (added in earlier work). Audit:

| | `quota.threshold` (existing) | `usage.milestone` (this port) |
|-|------------------------------|--------------------------------|
| Trigger | % of MaxQuota cap | Absolute lifetime tokens |
| Cycle | Monthly (resets) | Lifetime (one-shot per tier) |
| Unit | Billing units | Tokens |
| Ladder | 50/80/95/100% | 1k/10k/100k/1M |
| Semantic | Alarm ("budget burn rate") | Achievement ("milestone hit") |
| Dedup | Redis SETNX, monthly key | Redis SETNX, 1-year key |
| Cross-fork wire | Already present | Newapi has it separately too |

These are **complementary, not overlapping**. Merging would conflate
different units and cycles, and would break cross-fork compat with newapi
which already publishes both subjects independently.

## Files

| File | Type | LOC | Purpose |
|------|------|-----|---------|
| `internal/app/usage_milestone.go` | NEW | 117 | Tier ladder + `CheckAndPublishUsageMilestone` + `crossedMilestones` (pure) + `claimMilestone` (Redis SETNX) |
| `internal/app/usage_milestone_test.go` | NEW | 80 | 9 cases: pure-function ladder math + no-op safety on disabled Redis / invalid args |
| `internal/app/relay/compatible_handler.go` | mod | +7 | Single-line hook at end of `postConsumeQuota` (mirrors newapi's hook site) |

(`PublishUsageMilestone` helper itself was already added in 8-2.3 to keep
the NATS package self-contained.)

## Wire Format (cross-fork canonical)

```json
{
  "event_id":   "<uuid>",
  "event_type": "llm.usage.milestone",
  "account_id": <int64>,
  "payload": {
    "period":      "lifetime",
    "tokens_used": 100000,
    "milestone":   "first_100k"
  },
  "occurred_at": "2026-05-07T10:00:00Z"
}
```

`period` is currently always `"lifetime"`. Future tiers may add
`"day"` / `"month"` if we expand the ladder.

## Behaviour

```
postConsumeQuota (compatible_handler)
        ↓
   RecordConsumeLog
        ↓
   if totalTokens > 0:
        ↓
   CheckAndPublishUsageMilestone(ctx, userID, totalTokens)
        ↓
   ┌─ INCRBY llm:tokens:<userID> by totalTokens
   │   → newTotal, prevTotal = newTotal - totalTokens
   ↓
   ┌─ for each tier in [1k, 10k, 100k, 1M]:
   │     if prevTotal >= threshold: continue
   │     if newTotal < threshold:   break  (sorted ascending)
   │     SETNX llm:milestone:<userID>:<threshold> 1 (TTL 1y)
   │     if claimed → PublishUsageMilestone(...)
```

**No-op when**:
- `userID <= 0` or `totalTokens <= 0`
- `common.RedisEnabled == false` or `RDB == nil`
- INCRBY fails (logged + skip; counter resyncs on next call)
- Publisher not initialised (handled by `PublishUsageMilestone`)

Marshal/publish failures are `slog.Warn` only — never propagated.

## Verification

| Check | Result |
|-------|--------|
| `go build ./internal/...` | ✅ |
| `go test -run "Milestone\|Crossed" ./internal/app/` | ✅ 9/9 pass |
| Pure function `crossedMilestones` covers 6 cases | ✅ no crossings, first tier, skip-already-crossed, multi-tier in one shot, exact boundary, all tiers |
| No-op safety: invalid userID, zero/negative totalTokens, Redis disabled | ✅ 2 tests |
| End-to-end: NATS message captured by `2l-svc-platform` consumer | not yet (need STAGE) |

## Out of Scope (Follow-Up)

### `quota.threshold` semantics not changed

Existing `llm.quota.threshold` (50/80/95/100% of MaxQuota) is left
untouched. Both events now coexist; consumer dedups by `event_id`.

### Token counter persistence

The cumulative `llm:tokens:<userID>` lives in Redis only. If Redis is
flushed or the user moves servers, milestones for that user re-fire.
Acceptable trade-off: at most a single re-fire per tier per user. A
DB-backed counter would require a column + migration; can reconsider if
it becomes a UX issue.

### Daily/monthly milestone tiers

Current ladder is lifetime-only. Adding daily ("today's first 1k") or
monthly tiers means new keys + new periods. Could surface in a follow-up
if engagement metrics warrant it.

## Definition of Done Checklist

- [x] Tier ladder + pure `crossedMilestones` function ported
- [x] `CheckAndPublishUsageMilestone` with Redis INCRBY + SETNX claim
- [x] Single-line hook at `compatible_handler.postConsumeQuota` end
- [x] No-op fallbacks: invalid userID, zero totalTokens, Redis disabled, INCRBY error
- [x] go build pass
- [x] 9 unit tests pass (8 ladder math + 2 no-op safety — counts overlap by 1)
- [ ] STAGE deploy + verify event lands in LLM_EVENTS stream (subject `llm.usage.milestone`)
- [ ] sprint-status → done

## References

- Newapi origin: `2b-svc-newapi/service/usage_milestone.go`, hook at `relay/compatible_handler.go:512`
- Newhub publish helper: `internal/pkg/nats/events.go` (added in 8-2.3)
- Existing `quota.threshold`: `internal/app/quota_threshold.go`
- Cross-fork consumer: `2l-svc-platform/modules/notification/internal/pkg/event/types.go`

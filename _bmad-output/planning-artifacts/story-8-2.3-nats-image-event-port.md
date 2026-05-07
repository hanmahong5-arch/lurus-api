# Story 8-2.3: NATS llm.image.generated Event Port (newapi → newhub)

**Epic**: 8 - Newapi/Newhub Consolidation
**Priority**: P1
**Status**: review (port done; image_url extraction landed in 8-2.3.1; STAGE deploy pending)
**Type**: Backend port from newapi
**Created**: 2026-05-07

---

## Goal

Wire `llm.image.generated` NATS event so the unified notification inbox
(`2l-svc-platform/modules/notification`) receives image-generation events
from newhub, matching newapi behaviour.

Without this, image generations through newhub are invisible to the platform
inbox UX (only chat completions surface). Cross-fork compat required since
the consumer module sees events from BOTH gateways and dedups by `event_id`.

## Source

- newapi `9ef4e6db` feat(events): publish image.generated + usage.milestone to LLM_EVENTS
- newapi `fc49e72d` feat(events): backfill image_url in llm.image.generated payload (deferred — see Out of Scope)

## Files

| File | Type | LOC | Purpose |
|------|------|-----|---------|
| `internal/pkg/nats/events.go` | NEW | 130 | Subjects + envelope + typed payload + `PublishImageGenerated` / `PublishUsageMilestone` helpers |
| `internal/pkg/nats/events_test.go` | NEW | 41 | 8 cases for `truncateRune` + 2 no-op-on-invalid-user safety tests |
| `internal/app/relay/image_handler.go` | mod | +15 | Call `hubnats.PublishImageGenerated` after successful `postConsumeQuota` |

## Wire Format (cross-fork canonical)

Mirrors `2l-svc-platform/modules/notification/internal/pkg/event/types.go`:

```json
{
  "event_id":   "<uuid>",
  "event_type": "llm.image.generated",
  "account_id": <int64>,
  "payload": {
    "job_id":    "<uuid>",
    "image_url": "",
    "prompt":    "first 80 runes of user prompt"
  },
  "occurred_at": "2026-05-07T10:00:00Z"
}
```

## Behaviour

```
/v1/images/generations  →  ImageHelper(c, info)
                                ↓
                        upstream provider call
                                ↓
                        write response to client (streamed)
                                ↓
                        postConsumeQuota(...)
                                ↓
                        hubnats.PublishImageGenerated(ctx,
                            UserId, OriginModelName,
                            request.Prompt, "" /*URL deferred*/)
```

**No-op when**:
- `nats.Get() == nil` (publisher not initialised — `LLM_QUOTA_NATS_ENABLED=false` or NATS unreachable on boot)
- `userID <= 0` (anonymous / pre-auth)

Marshal/publish failures are `slog.Warn` only — never propagated; the relay
response is already on the wire by the time we publish.

## Verification

| Check | Result |
|-------|--------|
| `go build ./internal/...` | ✅ |
| `go test ./internal/pkg/nats/` | ✅ 10/10 (truncateRune + invalid-user no-op safety) |
| Existing nats package no regression | ✅ |
| End-to-end: NATS message captured by `2l-svc-platform` consumer | not yet (need STAGE) |

## Out of Scope (Follow-Up)

### Image URL extraction (newapi `fc49e72d`)

Newhub currently publishes `image_url=""`. The newapi backfill wraps the gin
response writer to capture the streamed body, parses provider-specific JSON
shapes (OpenAI/DALL-E, Replicate, Ali, Jimeng, Gemini, Zhipu) into a uniform
URL extractor.

Why deferred:
- ~150 LOC of writer-wrapper plumbing across multiple adaptors
- Consumer falls back to model+prompt rendering when URL is empty (per newapi
  comment: "On parse miss / cap-truncated body the URL is '' and the consumer
  falls back to prompt+model rendering")
- The notification UX still works without it — just no thumbnail
- Better as a separate Story 8-2.3.1 once the rest of 8-2 is integrated

### `LLM_QUOTA_NATS_ENABLED` rename

The current env var name suggests "quota only" but now governs all LLM events.
Suggest rename to `LLM_NATS_ENABLED` in a follow-up — but with backwards-compat
fallback so existing deploys aren't broken. Out of scope here.

## Definition of Done Checklist

- [x] Subjects + envelope + typed payloads added to `internal/pkg/nats/`
- [x] `PublishImageGenerated` helper with no-op fallbacks (nil publisher, userID ≤0)
- [x] `PublishUsageMilestone` helper (foundation for 8-2.4 milestone story)
- [x] Wire publish call in `ImageHelper` success path
- [x] `truncateRune` rune-safe truncation (matches newapi byte-for-byte)
- [x] go build pass
- [x] 10 unit tests pass
- [ ] STAGE deploy + verify event lands in LLM_EVENTS stream (subject `llm.image.generated`)
- [ ] sprint-status → done

## References

- Newapi origin: `2b-svc-newapi/service/event_publish.go` (env), `relay/image_handler.go:160`
- Newhub publisher infra: `internal/pkg/nats/publisher.go` (existing, unchanged)
- Newhub publish call site: `internal/app/relay/image_handler.go:148`
- Cross-fork consumer: `2l-svc-platform/modules/notification/internal/pkg/event/types.go`
- 8-2.4 milestone story: blocked on whether to merge with existing `quota_threshold` events (decision pending)

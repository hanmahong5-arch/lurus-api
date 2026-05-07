# Story 8-2.3.1: Image URL Extraction (newapi → newhub)

**Epic**: 8 - Newapi/Newhub Consolidation
**Priority**: P2 (UX polish — inbox thumbnails)
**Status**: review (port done; STAGE event-stream URL verification pending)
**Type**: Backend port from newapi (follow-up to 8-2.3)
**Created**: 2026-05-07

---

## Goal

Backfill `image_url` in the `llm.image.generated` NATS event so the unified
notification inbox can render image thumbnails instead of falling back to
prompt+model placeholders.

8-2.3 deliberately punted on URL extraction. This follow-up wraps `c.Writer`
with a tee-style buffered writer so we can parse the upstream JSON response
shape (OpenAI/DALL-E / Replicate / Ali / Jimeng / Gemini / Zhipu — all
normalize to `{"data":[{"url":"..."}]}` before client write) without
modifying every provider adaptor individually.

## Source

- newapi `fc49e72d` feat(events): backfill image_url in llm.image.generated payload (2026-05-01)

## Files

| File | Type | LOC | Purpose |
|------|------|-----|---------|
| `internal/app/relay/response_capture.go` | NEW | 217 | `BufferedResponseWriter` (tee-style gin.ResponseWriter, 64 KiB cap) + `ExtractImageURL` (OpenAI/Replicate/generic-fallback shape walker) |
| `internal/app/relay/response_capture_test.go` | NEW | 287 | 21 cases: byte-identical pass-through, cap enforcement, header forwarding via embedding, all provider shapes, empty/garbage/truncated/HTML safety |
| `internal/app/relay/image_handler.go` | mod | +13 / -7 | Wrap `c.Writer` once at `ImageHelper` entry; restore on return; pass `captured.Bytes()` to `ExtractImageURL` after `postConsumeQuota` |

Total ~510 LOC added (most in tests).

## Wire Format Change (8-2.3 → 8-2.3.1)

Before (8-2.3):
```json
{"event_type":"llm.image.generated","payload":{"job_id":"...","image_url":"","prompt":"..."}}
```

After (8-2.3.1):
```json
{"event_type":"llm.image.generated","payload":{"job_id":"...","image_url":"https://cdn.example/dalle.png","prompt":"..."}}
```

`image_url` is still possibly `""` when:
- Provider streams a non-JSON body (HTML error page, binary, etc.)
- Body exceeds 64 KiB cap and is truncated mid-JSON (extremely rare for image metadata)
- Provider uses an unrecognized shape AND no `*url*` field exists at depth 1 or 2
- Response was b64_json (no URL to extract)

In all empty-URL cases the consumer falls back to model+prompt rendering — same as 8-2.3 baseline. No regression.

## Behaviour

```
ImageHelper(c)
   │
   ├─ captured := NewBufferedResponseWriter(c.Writer, 64KiB)
   ├─ originalWriter := c.Writer
   ├─ c.Writer = captured
   ├─ defer { c.Writer = originalWriter }
   │
   ├─ … existing flow (DoRequest / DoResponse / postConsumeQuota) …
   │     adaptor calls c.Writer.Write(...) → captured.Write(...)
   │     → forwards bytes UNCHANGED to user
   │     → mirrors capped prefix into in-memory buffer
   │
   └─ imageURL := ExtractImageURL(captured.Bytes())
       PublishImageGenerated(ctx, userID, model, prompt, imageURL)
```

**Key invariants**:
- Bytes the user sees are byte-identical to pre-port (verified by test
  `TestBufferedResponseWriter_WritePassesThroughByteIdentical`)
- Buffer is hard-capped at 64 KiB — even a 200 MB streamed body cannot OOM
  newhub (`TestBufferedResponseWriter_CapBoundsBufferButForwardsAllBytes`)
- ExtractImageURL never panics (`TestExtractImageURL_*NoPanic` cover
  garbage/truncated/HTML)
- `c.Writer` is restored on return so post-handler middleware sees the
  original writer

## Verification

| Check | Result |
|-------|--------|
| `go build ./internal/...` | ✅ |
| `go test -run "BufferedResponseWriter\|ExtractImageURL" ./internal/app/relay/` | ✅ 21/21 |
| `go test -short ./internal/app/relay/` (full pkg) | ✅ all pass |
| Byte-identical pass-through verified | ✅ baseline-vs-wrapped diff test |
| 64 KiB cap enforced under 256-byte burst | ✅ |
| Header/WriteHeader/Flush forward via embedding | ✅ |
| OpenAI/DALL-E shape | ✅ |
| Replicate array + string output | ✅ |
| Ali / Jimeng (via OpenAI normalization) | ✅ |
| Generic depth-1 / depth-2 fallback | ✅ |
| Empty/whitespace/null/garbage body returns "" | ✅ |
| HTML error body does not panic | ✅ |
| End-to-end: NATS payload contains real URL | not yet (need STAGE) |

## Out of Scope

### Audio / video URL extraction

Same writer-wrap pattern would work for `llm.audio.generated` /
`llm.video.generated`, but those events do not exist yet (no consumer demand).
Defer to a separate story when the inbox UX requires audio thumbnails.

### Streaming response (SSE) capture

Image-generation responses are non-streaming JSON, so the simple buffer-cap
approach works. SSE responses (chat completions) would need a different
strategy — out of scope here.

### Per-provider shape registry

Currently the extractor uses a 3-tier fallback (OpenAI → Replicate →
generic). If a future provider returns a deeply-nested shape that the
generic depth-2 walker misses, we'd add a per-channel-type registry. Not
needed for the 6 providers in production (OpenAI, DALL-E, Replicate, Ali,
Jimeng, Gemini, Zhipu).

## Definition of Done Checklist

- [x] `BufferedResponseWriter` tee-style writer with 64 KiB cap
- [x] `ExtractImageURL` 3-tier shape walker (OpenAI / Replicate / generic)
- [x] Wrap `c.Writer` once in `ImageHelper`, restore on return
- [x] Byte-identical user-visible bytes (verified)
- [x] Cap enforces buffer bound while forwarding all bytes
- [x] Never panics on empty/garbage/truncated/HTML
- [x] go build pass
- [x] 21 unit tests pass
- [ ] STAGE deploy + verify NATS payload `image_url` is populated for OpenAI/DALL-E
- [ ] STAGE pen test: stream a 200 MB body, confirm no OOM
- [ ] sprint-status → done

## References

- Newapi origin: `2b-svc-newapi/service/response_capture.go` + `relay/image_handler.go:23-32, 150-162`
- 8-2.3 baseline (image_url=""): `_bmad-output/planning-artifacts/story-8-2.3-nats-image-event-port.md`
- Cross-fork consumer: `2l-svc-platform/modules/notification/internal/pkg/event/types.go`

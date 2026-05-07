# Story 9-1: Tier 3 Modality Usage Audit

**Epic**: 9 - Modality Slim-Down
**Priority**: P1
**Status**: review (code-only audit complete; live usage data needs DB query — operator action)
**Type**: Code audit + scope estimation
**Created**: 2026-05-07

---

## Goal

Quantify the code footprint and dependency graph of Tier 3 modalities
(Midjourney / Suno / Music / Realtime / Video) so Anita can decide:

1. **Removal scope**: how many LOC, files, DB tables, routes
2. **Risk**: what stays (shared infrastructure) vs what goes (modality-specific)
3. **Sequencing**: can we cut all 5 simultaneously, or stage by usage

D3 decision is "accepted" (cut MJ/Suno/Realtime/Music/Video) — this audit is
**execution prep**, not a re-debate.

## Method

Code-only audit (no DB / log access). Uses:
- File globs by modality name
- `wc -l` aggregation per provider directory
- Channel-type constants (`internal/pkg/constant/channel.go`)
- Route registrations (`internal/adapter/handler/router/*.go`)
- DB schema (`scripts/migrate/init_postgres.sql`)
- Frontend page directories (`web/src/pages/`)

Live usage metrics (per-tenant request counts, revenue, MoM trends) require
running queries against PROD/STAGE DB — separate operator action, not
included here.

## Findings — Code Footprint

### Backend (Go)

| Modality | Core LOC | Files | Notes |
|----------|----------|-------|-------|
| **Midjourney** | 1,781 | 13 | `app/midjourney.go` + `relay/mjproxy_handler.go` + `handler/midjourney.go` + entity/repo/dto/constant/router |
| **Suno** | 314 | 3 | `provider/task/suno/{adaptor,models}.go` + `dto/suno.go` + 1 route group |
| **Music** | 228 | 3 | `provider/task/music/{adaptor,models}.go` + 1 route group |
| **Realtime** | 134 (+ 43 branches) | 2 + branches | `app/relay/websocket.go` + `dto/realtime.go` + 43 lines in `provider/openai/relay-openai.go` + 10 in `token_counter.go` |
| **Video** | 4,625 | ~25 | task adaptors (ali/doubao/gemini/hailuo/jimeng/kling/sora/vertex/vidu) + video handlers + DTOs + middleware adapters |
| **Total** | **~7,082** | **~46** | (excludes shared `task/` infra) |

#### Video sub-breakdown

| Provider | LOC | Status |
|----------|-----|--------|
| ali       | 526 | Video-only task adaptor (chat lives at `provider/ali/`, **keep**) |
| doubao    | 262 | Video task (chat at `provider/doubao/`, **keep**) |
| gemini    | 328 | Video task (chat at `provider/gemini/`, **keep**) |
| hailuo    | 523 | Video-only — fully removable |
| jimeng    | 484 | Video task (image at `provider/jimeng/`, **keep**) |
| kling     | 406 | Video-only — fully removable |
| sora      | 223 | Video-only — fully removable |
| vertex    | 414 | Video task (chat at `provider/vertex/`, **keep**) |
| vidu      | 320 | Video-only — fully removable |
| Handlers  | 1,134 | `handler/{video_proxy,video_proxy_gemini,task_video,internal_video_catalog,swag_video}.go` + `router/video-router.go` |
| Middleware | 119 | `middleware/{kling,jimeng}_adapter.go` |

**Critical**: `task/{ali,doubao,gemini,jimeng,vertex}/` are video-task variants
of providers whose chat path lives at `provider/{name}/`. Deleting the task
variant does NOT touch chat completions. This is the safest split.

### Frontend

| Page | Path | Status |
|------|------|--------|
| Midjourney | `web/src/pages/Midjourney/index.jsx` | Single page |
| Tasks | `web/src/pages/Task/index.jsx` | Used for Suno/Music/Video task logs |
| MJ Logs hooks/components | `web/src/{hooks/mj-logs,components/table/mj-logs}/` | Tier 3 specific |
| Task Logs hooks/components | `web/src/{hooks/task-logs,components/table/task-logs}/` | Tier 3 specific |
| Sider/Footer entries | `SiderBar.jsx`, `Footer.jsx`, `useSidebar.js` | nav links to remove |
| i18n strings | 4 locales (zh/en/ja/vi/ru/fr) × ~14 entries each | translation keys |
| Channel constants | `web/src/constants/channel.constants.js` | 8 channel-type entries |

### Database

| Table | Purpose | Affected by cut |
|-------|---------|-----------------|
| `midjourneys` | MJ task records | DROP (or archive) |
| `tasks` | Suno/Music/Video task records | DROP (or archive) |

Historical billing data in `logs` and `consume_logs` tables retains
references via `model` column — keep those rows, just stop generating new
ones. No schema change required.

### Routes Removed

| Route | Handler | Replaces with |
|-------|---------|----------------|
| `POST/GET /api/mj/*` | `RelayMidjourney` | 410 Gone |
| `POST/GET /:mode/mj/*` | `RelayMidjourney` | 410 Gone |
| `POST/GET /suno/*` | `RelayTask` | 410 Gone |
| `POST /v1/audio/music` | `RelayTask` | 410 Gone |
| `GET  /v1/audio/music/:task_id` | `RelayTask` | 410 Gone |
| `GET  /v1/realtime` (WS) | `Relay(RealtimeFormat)` | 426 Upgrade Required → drop WS handshake |
| `POST /v1/videos`, `/v1/videos/:task_id` | `RelayTask` | 410 Gone |
| `POST /v1/video/generations`, etc. | `RelayTask` | 410 Gone |
| `POST /kling/v1/videos/*` | `RelayTask` | 410 Gone |

`410 Gone` (vs `404`) signals deprecation explicitly so SDK telemetry can
flag callers — important during 90-day deprecation window per 9-2.

### Channel Types Removed

`internal/pkg/constant/channel.go`:

| Const | ID | Modality |
|-------|----|----------|
| `ChannelTypeMidjourney` | 2 | MJ |
| `ChannelTypeMidjourneyPlus` | 5 | MJ |
| `ChannelTypeSunoAPI` | 36 | Suno |
| `ChannelTypeKling` | 50 | Video |
| `ChannelTypeJimeng` | 51 | Video task |
| `ChannelTypeVidu` | 52 | Video |
| `ChannelTypeDoubaoVideo` | 54 | Video |
| `ChannelTypeSora` | 55 | Video |

(8 channel types). Existing channels in DB with these types should be
**hard-disabled** in 9-2 (set `status=disabled`) before code removal in 9-3,
to prevent admin from re-enabling a channel whose backend is gone.

### Ratio Settings

`internal/pkg/setting/ratio_setting/model_ratio.go`: 14 entries match
tier-3 model patterns. Removing these does not affect billing of historical
logs — `consume_logs` rows store the resolved quota at write time.

## Findings — Dependency Risk

| Risk | Severity | Notes |
|------|----------|-------|
| Video task adaptors share base types (`task.Adaptor` interface) with future tier-1 task workflows | LOW | Interface stays; only video implementations leave |
| `task/{ali,doubao,gemini,jimeng,vertex}/` deletion impacts chat? | NO | Chat lives at `provider/{name}/`, separate package |
| Realtime branch logic in `provider/openai/relay-openai.go` (43 lines) | MEDIUM | These branches must be deleted alongside the route, not separately, or tests will hit dead code |
| `tasks` DB table referenced by audit/log queries? | UNKNOWN | Need to grep all `repo/log.go` SQL — defer to 9-3 pre-flight |
| Frontend `Task` page may render non-tier-3 task types if any were added | LOW | Search `task_type` column usage in `repo/task.go` (line 373) — confirm before removing FE |
| MJ/Task channel admin UI references | LOW | `EditChannelModal` shows tier-3 type options; trivial remove |

## Findings — Live Usage (BLOCKED — operator action)

The audit cannot complete a usage profile without DB access. Required:

```sql
-- 1. Tenants using each tier-3 modality (last 30 days)
SELECT model, COUNT(DISTINCT user_id) as tenants, COUNT(*) as requests, SUM(quota) as total_quota
FROM consume_logs
WHERE created_at >= NOW() - INTERVAL '30 days'
  AND (model LIKE 'mj-%' OR model LIKE 'midjourney%'
       OR model LIKE 'suno%' OR model LIKE 'music%'
       OR model LIKE '%video%' OR model LIKE 'kling%'
       OR model LIKE 'sora%' OR model LIKE 'vidu%' OR model LIKE 'hailuo%'
       OR model LIKE '%realtime%')
GROUP BY model
ORDER BY total_quota DESC;

-- 2. Revenue exposure (rough — quota × QuotaPerUnit / 1)
SELECT
  SUM(CASE WHEN model LIKE 'mj-%' OR model LIKE 'midjourney%' THEN quota ELSE 0 END) AS mj_quota,
  SUM(CASE WHEN model LIKE 'suno%' THEN quota ELSE 0 END) AS suno_quota,
  SUM(CASE WHEN model LIKE 'music%' THEN quota ELSE 0 END) AS music_quota,
  SUM(CASE WHEN model LIKE '%video%' OR model LIKE 'kling%' OR model LIKE 'sora%'
            OR model LIKE 'vidu%' OR model LIKE 'hailuo%' THEN quota ELSE 0 END) AS video_quota,
  SUM(CASE WHEN model LIKE '%realtime%' THEN quota ELSE 0 END) AS realtime_quota
FROM consume_logs
WHERE created_at >= NOW() - INTERVAL '30 days';

-- 3. Active channels per tier-3 type
SELECT type, COUNT(*) AS active_channels, COUNT(*) FILTER (WHERE status=1) AS enabled
FROM channels
WHERE type IN (2, 5, 36, 50, 51, 52, 54, 55)
GROUP BY type;
```

Run these on PROD (`platform-pg.lurus-platform.svc:5432`, `lurus_api`
schema) and append results to this doc. The deprecation announcement (9-2)
should cite the actual numbers.

## Recommendation

**Sequencing for Epic 9 execution**:

1. **9-1 (this story)** — code audit ✅ done; pending live usage query
2. **9-2 (announce, ~1 day)** — deprecation banner in admin UI for tier-3
   channels, blog post, `Deprecation` HTTP header on tier-3 endpoints, 90-day
   countdown starts. **Block: live usage data must show <5% revenue
   contribution AND <10 active tenants per modality** (from Q3 query above).
   If any modality exceeds threshold, treat that one specially (migration
   path, customer outreach).
3. **9-3 (code removal, ~2 days)** — after 90 days:
   - Delete files (~7,082 LOC + frontend + i18n)
   - Drop channel types from constant + admin UI
   - Drop `midjourneys` + `tasks` tables (or archive to `*_archived` schema)
   - Tag release `v3.0.0-modality-slim`
   - Update `lurus.yaml` capabilities

**Estimated savings**:
- ~7,082 LOC backend removed
- 8 channel types simplified
- 2 DB tables removed
- ~10 frontend page/hook/component dirs removed
- ~14 model ratio entries cleaned
- Test surface area reduced by ~25%
- Docker image size: marginal (~2 MB Go binary shrink)
- **Cognitive load**: meaningful — every new dev currently has to understand
  task-polling pattern that's used by 5 tier-3 modalities; cutting it removes
  a whole architectural concept from onboarding.

## Definition of Done Checklist

- [x] Backend LOC quantified per modality (7,082 total)
- [x] Frontend page/hook/component dirs identified
- [x] Routes mapped for removal (mj/suno/music/realtime/video)
- [x] Channel type constants enumerated (8 IDs)
- [x] DB tables identified (`midjourneys`, `tasks`)
- [x] Provider chat-vs-task split documented (jimeng/gemini/ali/doubao/vertex preserve chat)
- [x] Realtime entanglement called out (43 branches in `provider/openai`)
- [x] Sequencing recommendation (9-2 → 90d → 9-3)
- [ ] Live usage SQL run against PROD (operator) — required before 9-2 announce
- [ ] sprint-status → done after live usage data appended

## References

- D3 decision: `_bmad-output/planning-artifacts/sprint-status.yaml` `D3_tier3_modality: accepted`
- Epic 9 stories: `9-2 deprecate-announce` (blocks until live data) / `9-3 code-removal` (blocks 90 days post-9-2)
- Channel constants: `internal/pkg/constant/channel.go`
- Routes: `internal/adapter/handler/router/{api,relay,video}-router.go`
- DB schema: `scripts/migrate/init_postgres.sql`

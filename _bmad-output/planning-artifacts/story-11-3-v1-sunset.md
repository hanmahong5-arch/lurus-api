# Story 11-3: V1 Web UI Sunset

**Epic**: 11 (v1 Web UI 退役)
**Priority**: P1 — closes Epic 11
**Status**: ✅ DONE (2026-05-12) — escape-hatch routes deleted, v1 page files removed, build clean, deployed to STAGE
**Sized**: 1 day (post-11-2 single-developer)
**Predecessor**: Story 11-2 (v2 wiring MVP) — shipped 2026-05-12 (`bb524610`)

## Context

Story 11-2 flipped `/console/*` → `/console/v2/*` and left `/console/legacy/*` as a one-sprint escape hatch. This story removes that escape hatch and deletes the v1 page source files, finalising the v1 web UI sunset.

## Scope

### Deleted

| Route (in `web/src/App.jsx`) | Component | File deleted |
|------------------------------|-----------|--------------|
| `/console/legacy/models` | `<ModelPage />` | `web/src/pages/Model/` |
| `/console/legacy/channel` | `<Channel />` | `web/src/pages/Channel/` |
| `/console/legacy/token` | `<Token />` | `web/src/pages/Token/` |
| `/console/legacy/playground` | `<Playground />` | `web/src/pages/Playground/` |
| `/console/legacy/log` | `<Log />` | `web/src/pages/Log/` |
| `/console/legacy/dashboard` | `<Dashboard />` (lazy) | `web/src/pages/Dashboard/` |

Top-level imports for these 6 components removed from `App.jsx`.

### Kept (out of scope — no v2 equivalent yet)

| Route | Reason kept |
|-------|-------------|
| `/console/user` (`User`) | Admin user management, no v2 page |
| `/console/setting` (`Setting`) | Admin system settings (different surface from `/console/v2/settings` which is user-personal) |
| `/console/personal` (`PersonalSetting`) | User profile settings, no v2 equivalent in Story 11-2 scope |
| `/console/topup` (`TopUp`) | Tied to Epic 12 (计费分层) — wait for SKU model |
| `/console/openrouter-sync` (`OpenRouterSync`) | Admin sync tool |
| `/console/redemption` (`Redemption`) | Wave 2 consumer-feature deletion candidate |
| `/console/midjourney` (`Midjourney`) | Wave 2 candidate |
| `/console/task` (`Task`) | Wave 2 candidate |
| `/console/chat/:id?` (`Chat`) | Wave 2 candidate |
| `/chat2link` (`Chat2Link`) | Wave 2 candidate |
| `/pricing` (`Pricing`) | Public marketing surface |
| `/about`, `/user-agreement`, `/privacy-policy` | Public marketing surfaces |

### Pre-existing dead code (NOT touched this story)

| Path | Status |
|------|--------|
| `web/src/pages/ModelDeployment/` | Orphaned before Story 11-3 — no router reference. Left for a future cleanup story so this PR stays scoped. |
| `web/src/hooks/model-deployments/useModelDeploymentSettings.js` | Orphan-by-deletion (was only consumed by the two deleted Model pages). Left as future cleanup — exported symbol, low cost to keep. |

### Backend (NOT changed this story)

V1 API handlers (`/api/{channel,token,log,...}/*`) remain in service. V2 frontend still calls some V1 endpoints (PersonalSetting → `/api/user/self`, TopUp → `/api/user/topup`). Backend v1 sunset is a separate decision tied to consumer-feature deletion (Wave 2).

## Definition of Done

- [x] `/console/legacy/*` routes removed from `web/src/App.jsx`
- [x] V1 page source files deleted (6 directories)
- [x] V1 imports removed from `App.jsx` (Channel, Token, Log, Dashboard, Playground, ModelPage)
- [x] `bun run build` clean — no broken imports, no module-not-found errors
- [x] Prettier formatting clean for modified file
- [x] Story 11-3 doc committed
- [x] Pushed to `origin/main`, GHA built image, deployed to STAGE (`test-newhub.lurus.cn`)

## Risk / Notes

- One-sprint escape hatch promised in Story 11-2 DoD was shortened to ~0 days because the v2 surfaces are confirmed working in the local smoke test. If a regression surfaces in STAGE, the rollback path is `git revert` of this story's commit (v1 page files restorable via git history).
- The pre-existing orphans (`ModelDeployment/`, `useModelDeploymentSettings`) flagged here are NOT in scope per Karpathy #3 (surgical changes) — open a follow-up clean-up issue if desired.
- Wave 2 consumer-feature deletion (Redemption, TopUp, Playground backend, Midjourney, Suno, Chat) remains open and gated on product decision.

## Closes Epic 11

With Story 11-3 done, Epic 11 (v1 Web UI 退役) is complete. Remaining v1 web routes serve admin/marketing/Wave-2-pending surfaces — not legacy-escape-hatch.

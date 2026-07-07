# Private Inference Routing — local demo

Proves, end to end, that a tenant's LLM traffic can be pinned to a private
(on-prem / intranet) inference endpoint with **zero business-code change** —
pure New API channel config (`ChannelTypeCustom = 8` + `base_url`). One
tenant, one channel, one token; a relay call carrying that tenant's token is
dispatched to the private endpoint and no external LLM provider is ever
reachable.

## Files

| File | Role |
|---|---|
| `mock-openai-endpoint.mjs` | Standalone OpenAI-compatible mock "self-hosted inference endpoint". Binds `127.0.0.1` only (loopback) — a request that reaches it provably never left the host. Logs every hit. |
| `seed-tenant-private-endpoint.sql` | The proof: tenant `privacy-demo`, a normal (role=1, non-admin) user, a relay token, and a Custom channel whose `base_url` is the loopback mock. Pure config, `ON CONFLICT DO NOTHING` idempotent. **Do not edit** — it is the artifact the whole demo exists to keep true. |
| `seed-console-viewer.sql` | Additive, separate from the file above. `GET /api/v2/:tenant_slug/channels` is admin-gated, so the role=1 proof-user can never view it — this adds one extra role=10 admin account in the *same* tenant purely so the console screenshot can authenticate. Also self-heals if the proof-user's role ever drifted (see below). |
| `shot-console.mjs` | Playwright script that logs in via the newhub e2e bridge (no manual cookie export) and screenshots a live "this tenant's routing" panel from the real API. |
| `run-demo.sh` | One-click runner tying all of the above together. |
| `.env.demo` | Env for the demo backend only — separate from repo-root `.env`/`.env.dev` so this never touches whatever another local session is pointed at. |

## Quick start

```bash
bash demo/private-inference/run-demo.sh
```

This will, in order:
1. Start (or create) a dedicated demo Postgres — docker container `newhub-pgdemo`, port **5435**, `postgres`/`postgres`/`newhub`. Independent of `docker-compose.dev.yml`'s dev stack (port 5434) so it can't collide with other local work.
2. `go run ./cmd/server` against that DB on port **8099** (waits for GORM auto-migrate + embedded migration runner to finish booting).
3. Apply both seed files (idempotent — safe to re-run).
4. Start the mock private-inference endpoint on port **11434**.
5. Fire one real `POST /v1/chat/completions` as the `privacy-demo` tenant's token, then print the mock endpoint's request log — that log entry **is** the proof: the relay dispatched to the private channel.

Stop the backend + mock when done (docker container is left running so the next `run-demo.sh` is fast):

```bash
bash demo/private-inference/run-demo.sh stop
```

### Prerequisites
Docker Desktop running, Go toolchain, Bun (for the mock endpoint), Node (for the optional screenshot — see below). No `psql` needed on the host; the script execs it inside the container.

### Known environment gotcha (Docker Desktop / Windows)
`docker start` on a container that's been stopped a while can come back with
`HostConfig.PortBindings` correct but the host-side port-forward proxy not
actually bound (`docker inspect` shows `NetworkSettings.Ports` empty) — the
container is healthy but nothing on the host can reach it. `run-demo.sh`
detects this and does a full `docker restart` to fix it; you'll see a line
like `host port 5435 not bound after start — docker restart...` — that's
expected self-healing, not a failure.

## Console screenshot

`shot-console.mjs` logs in automatically via newhub's e2e bridge
(`internal/adapter/handler/v2_bridge.go`, `POST /api/v2/bridge/exchange`) — the
same mechanism the `web/tests/e2e` Playwright suite uses. The route is only
registered on a server booted with `E2E_BRIDGE_TOKEN` set (which `.env.demo`
does, demo-local value only) — it does not exist at all otherwise, so this has
no production exposure. **No manual browser login or cookie export needed.**

```bash
BACKEND_URL=http://localhost:8099 \
E2E_BRIDGE_TOKEN="$(grep '^E2E_BRIDGE_TOKEN=' demo/private-inference/.env.demo | cut -d= -f2-)" \
BRIDGE_USER_ID=9101 TENANT_SLUG=privacy-demo \
node demo/private-inference/shot-console.mjs
```

Screenshot is written to `%TEMP%/hifi-preview/private-routing-console.png`
(override the path at the bottom of the script if you want it elsewhere).

### Known gotcha: run this with `node`, not `bun`
On this host, `chromium.launch()` from `playwright-core` **hangs for the full
180s launch timeout under `bun`** (the browser process spawns — visible in
`tasklist` — but the driver never completes its handshake with it over
`--remote-debugging-pipe`). The exact same script launches in ~2s under
`node`. This looks like a Bun-on-Windows incompatibility with Playwright's
pipe transport, not a script bug — reproduced with a minimal `chromium.launch()`
repro isolated from the rest of this script. If Playwright ever needs to run
from a bun-only context, use the WebSocket transport (`launchServer` +
connect) instead of the default pipe transport, or shell out to `node` as
this script's runner already does.

### If Playwright can't run at all in your environment
Fall back to a manual capture — still no OIDC/platform-identity dependency,
just a browser devtools step:
1. Boot the demo (`run-demo.sh`), then open `http://localhost:8099/api/v2/bridge/exchange?token=<E2E_BRIDGE_TOKEN>&user_id=9101` — note this must be a **POST**, so use devtools/curl, not a plain address-bar GET:
   ```bash
   curl -i -X POST "http://localhost:8099/api/v2/bridge/exchange?token=<token>&user_id=9101"
   ```
   Copy the `Set-Cookie: session=...` value from the response headers.
2. In a real browser, open devtools → Application/Storage → Cookies →
   `http://localhost:8099`, add a cookie named `session` with that value.
3. Navigate to `http://localhost:8099/` then to
   `http://localhost:8099/api/v2/privacy-demo/channels?page=1&page_size=20` —
   the JSON response is the same data `shot-console.mjs` renders.

## Why a separate console-viewer account?

`seed-tenant-private-endpoint.sql`'s whole point is a **role=1 (non-admin)**
tenant user routed by pure config — that's the claim under proof. But newhub's
channel-list API (`GET /api/v2/:tenant_slug/channels`, also the legacy
`GET /api/channel`) is admin-gated
(`internal/adapter/handler/router/api-v2-router.go` — `middleware.AdminAuth()`
/ `requireTenantAdmin`), so a role=1 user can never call it — there is no way
to make the *proof* user also do the *viewing*. `seed-console-viewer.sql`
adds one separate role=10 user in the same tenant for viewing only, and
resets the proof user back to role=1 if an earlier manual test had bumped it
(this repo's demo Postgres had exactly that drift when this runner was built —
fixed idempotently rather than left to rot).

## What this does not cover

Private **RAG** routing (vector-store recall) is a separate concern and is not
touched by this demo.

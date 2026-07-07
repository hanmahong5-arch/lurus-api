# Private Inference Routing — customer delivery mode

This demo's seed (`seed-tenant-private-endpoint.sql`) proves the mechanism
with a loopback mock and a free-mode billing shortcut. A real customer
deployment swaps exactly those two things for production equivalents — the
routing mechanism itself (tenant → Custom channel → `base_url`) does not
change.

## 1. Replace the mock endpoint with the customer's real inference service

The demo channel is:

```
type = 8 (ChannelTypeCustom, OpenAI-compatible)
base_url = http://127.0.0.1:11434   -- the mock
```

For a real customer, `base_url` becomes wherever *their* self-hosted
inference server actually lives — anything that speaks the OpenAI
`/v1/chat/completions` wire format qualifies (vLLM, Ollama, TGI, LocalAI,
their own wrapper, etc.). Concretely:

- **Console path**: tenant admin console → Channels → New Channel → type
  "Custom" → set Base URL to the customer's endpoint, Models to whatever the
  customer's server actually serves, Key to whatever bearer token their
  endpoint expects (or any placeholder string if it doesn't check one — the
  newhub side always forwards it, but doesn't require it to be meaningful).
- **API path**: `POST /api/v2/:tenant_slug/channels` (session auth, tenant
  admin — same admin gate this demo's console-viewer account exists to
  satisfy).
- **What "private" means here**: whatever network `base_url` resolves on is
  where the customer's data goes and nowhere else — newhub does not cache,
  log full request bodies to a third party, or otherwise fan the traffic out.
  It is the customer's own network topology (their VPC, their on-prem subnet,
  their VPN) that makes the endpoint actually unreachable from the internet;
  newhub itself only needs a route to it (any network path — same-host
  loopback, same-VPC private IP, or a customer-side VPN peer — that keeps the
  traffic off the internet satisfies the same claim this demo makes with
  loopback).

## 2. Replace `SelfUseModeEnabled` with real per-model pricing

The demo turns on `SelfUseModeEnabled` so the relay doesn't require a
configured price for `qwen2.5-7b-instruct` — that's a demo-only shortcut, not
something to ship to a paying tenant.

For a real deployment, configure the model's ratio/price instead of skipping
billing:

- **Console path**: tenant admin console → Pricing → set `ModelRatio` /
  `ModelPrice` / `CompletionRatio` for the customer's model name(s). These
  drive the same cost computation the demo's relay response already shows
  under `usage.x_lurus` (`cost_lb`, `model_ratio`, `group_ratio`,
  `balance_remaining`).
- **API path**: `POST /api/v2/:tenant_slug/pricing` (tenant-scoped; see
  `internal/adapter/handler/v2_pricing_write.go`).
- **Unified pre-authorize billing**: if the deployment uses the freeze/settle
  billing path instead of legacy post-hoc debit, that's a separate instance-
  level toggle (`BILLING_UNIFIED_ENABLED`) — orthogonal to per-model pricing,
  set once at deploy time, not per-tenant.
- Turn `SelfUseModeEnabled` back off (it's a global, instance-wide setting —
  confirm no other tenant on the same instance still depends on it being on
  before flipping it for a shared deployment).

## 3. Data-residency talking points (what you can actually show the customer)

Everything below is something you can point at directly, not a claim to take
on faith:

1. **The channel config itself** — `GET /api/v2/:tenant_slug/channels` for
   their tenant shows exactly one (or however many) channel(s), each with the
   `base_url` visible, pointed at their own endpoint. This is the same view
   `shot-console.mjs` renders in this demo.
2. **A live relay call + the upstream's own log** — fire one real request
   through their token and show their inference server's own request log
   recording the hit (in this demo, that's the mock's `[PRIVATE-ENDPOINT HIT]`
   banner; in production it's whatever access log their vLLM/Ollama/TGI
   instance already keeps).
3. **Tenant isolation is structural, not a filter** — the routing decision is
   made by which `channel`/`ability` rows exist for that tenant's group, not
   by an if-statement that could special-case around it; the same code path
   that dispatches every other tenant's requests is what this uses.
4. **No code fork** — this is stock New API channel abstraction
   (`ChannelTypeCustom`); nothing about the private-routing capability
   required a patch to newhub itself. If a customer's security review wants
   to read the dispatch code, it's the same relay code path used for every
   other provider integration, not a special one written for them.
5. Whatever is true of the customer's *own* network path from newhub to
   their `base_url` (VPN-only, no public DNS record, firewalled to a single
   source IP, etc.) is theirs to state and evidence — this doc/demo doesn't
   assert anything about a specific customer's network, only about what
   newhub does and doesn't do with the traffic once it decides where to send
   it.

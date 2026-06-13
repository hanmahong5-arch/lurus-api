# Industrial-Readiness — Owner-Gated Actions (ready-to-execute)

Companion to the 2026-06-12 readiness campaign. The CODE/manifest/CI/runbook work
is autonomous and already on the branch. The actions below touch **root/shared
manifests, platform config, or PROD/基建** — they are made *one-command ready* here
but are **NOT executed**; they wait on owner sign-off (root `CLAUDE.md` boundary).

> Sequencing reminder (from the plan's dependency order): the hardened manifests
> (P0-2 deep readiness, P1-4 spread/preStop) are **on `main` (PR #17, merged 2026-06-13)**
> but **not applied to any cluster yet** (see *Verified state* below — the staging deploy
> is skipped on an empty `STAGING_KUBECONFIG`, so there is no live target to validate against).
> Their PROD apply is blocked on **two** gates simultaneously:
>   1. **P0-5** — how PROD apply is delivered (GitOps vs manual).
>   2. **P1-2** — billing degrade live, so the deep readiness probe can't turn a
>      PG/billing blip into a fleet-wide outage. **Never** ship P0-2 to PROD ahead of P1-2.

## Verified state (2026-06-13 — live cluster + repo inspection)

Three facts were confirmed against the live cluster and the platform repo. They
**correct two assumptions the original plan made** (it assumed newhub already ran on
staging and that the backup gap was a flag flip):

1. **The Hub is not deployed to any reachable cluster.** The reachable cluster runs
   `lurus-newapi` (the open-source base) but **no `lurus-api` / `lurus-newhub`**. The
   `deploy-staging.yml` workflow **builds + pushes the image but skips the cluster apply**
   because the `STAGING_KUBECONFIG` repo secret is empty (it warns and exits 0 — a tracked
   infra gap, `doc/uat-handbook.md §2`). ⇒ the P0-2/P1-4 probe/topology changes **cannot be
   validated on any cluster** until staging is wired. New gating item: **Infra-0** below.
2. **newhub `lurus_api` has ZERO backup coverage** (previously "unconfirmed"). The pg_dump
   CronJob hardcodes `-d identity`, and `lurus_api` is **not a schema inside `identity`**
   (live check — identity's schemas are app/billing/identity/module/notification/public).
   So no backup path touches newhub data today. The fix extends the dump, not a flag (P1-5).
3. **The PG is single-instance `lurus-pg-1` (no HA replica).** A node/pod loss on that one
   PG is a hard newhub outage regardless of newhub replica count — relevant to any availability SLA.

---

## P0-5 — Add newhub PROD to the ArgoCD ApplicationSet (`selfHeal: false` to start)

**Why gated:** edits the root governance repo's shared
`deploy/argocd/appset-services.yaml`; the template currently forces `selfHeal: true`
on every element. Auto-syncing newhub into the **intentionally-empty R1 PROD** before
PMF is exactly the owner's concern — so we start with self-heal OFF (manual sync only)
and let the owner decide when to flip it on.

**Current state:** newhub PROD is *not* in the appset (platform/newapi are). Manifest
changes are invisible to the cluster until someone manually `kubectl apply`s — which is
why this campaign's P0-2/P1-4 manifests are not yet live anywhere but staging.

### Diff (apply to ROOT repo `hanmahong5-arch/lurus`)

The template hardcodes `selfHeal: true`; to allow a per-element override, parameterize it:

```yaml
# in spec.template.spec.syncPolicy.automated:
        automated:
          prune: false
          selfHeal: {{ default true .selfHeal }}   # was: selfHeal: true
```

Then add the element (note `selfHeal: false` + explicit `revision: main`):

```yaml
          - name: lurus-newhub
            namespace: lurus-system
            repo: 2b-svc-newhub
            path: deploy/k8s
            revision: main
            selfHeal: false   # MANUAL sync only until owner approves autosync to R1
```

### Execute (owner-approved only)

```bash
# 1. Land the diff in the root governance repo on a branch, open PR, owner merges.
# 2. ArgoCD picks up the new Application (manual-sync, will show OutOfSync):
ssh root@100.98.57.55 "argocd app get lurus-newhub"
# 3. First sync is DELIBERATE and reviewed (not automatic):
ssh root@100.98.57.55 "argocd app sync lurus-newhub --dry-run"   # inspect plan
ssh root@100.98.57.55 "argocd app sync lurus-newhub"             # apply for real
# 4. Only after the hardened manifest set is verified in PROD AND P1-2 is live,
#    owner may flip selfHeal:true for drift auto-recovery.
```

**Until this is done:** apply P0-2/P1-4 manifests to staging/r6-stage manually
(autonomous), and treat PROD as un-applied. Do NOT hand-apply the deep readiness
probe to the 3-replica PROD without P1-2 (correlation-outage risk).

---

## P1-5 — newhub PG backup coverage + restore drill  ★ SLA #1 blocker

> **CORRECTED 2026-06-12 after verifying the actual artifacts** — the original
> "BACKUP_ENABLED=false ⇒ RPO=∞, just flip it" framing was an oversimplification.
> Ground truth (from `2l-svc-platform/deploy/k8s/cronjobs/pg-backup.yaml` +
> `2l-svc-platform/doc/audit/2026-05-19-pg-backup-incident.md`):
>   - **WAL archiving is WORKING** (≈5900 WAL files, ~5 min fresh) → continuous
>     PITR stream exists; it is NOT a clean RPO=∞.
>   - **CNPG base backups are BROKEN** since ~2026-03-01 (cross-node overlay bug,
>     R2↔R1 `10.43.0.1:443`) → without a recent base, the WAL stream has nothing
>     to replay *onto*. Effective restorable RPO is therefore still unacceptable.
>   - A **pg_dump fallback CronJob** exists, gated by ConfigMap
>     `lurus-platform-backup-config` `BACKUP_ENABLED` (default off), writing to PVC
>     `lurus-pg-backup` + (weekly) MinIO bucket `pg-backups-v2`.
>   - ✅ **CONFIRMED 2026-06-13 (was "unconfirmed"):** the pg_dump CronJob hardcodes
>     `pg_dump -d identity` (file name `lurus-identity-*.dump`), and `lurus_api` is **NOT a
>     schema inside `identity`** (live check — identity's schemas are
>     app/billing/identity/module/notification/public). ⇒ **no backup path covers newhub's
>     `lurus_api` today — coverage is ZERO**, not merely "unconfirmed". Even flipping
>     `BACKUP_ENABLED=true` would back up only `identity`. The fix extends the dump (below).

**Why gated:** touches platform's backup config + storage + (for the base-backup
fix) the CNPG `Cluster` CR, which is **not in any repo** — managed live via
`kubectl edit cluster lurus-pg -n database`. **Escalate earliest even though it
executes late: an unverified-coverage DB vs an enterprise SLA is non-negotiable.**

### (a) Proven 2026-06-13 — only `identity` is dumped, `lurus_api` is not (commands to reproduce)

```bash
# Does any backup path include newhub's lurus_api DB, or only identity?
ssh root@100.98.57.55 "kubectl -n database get cm lurus-platform-backup-config -o yaml 2>/dev/null"
ssh root@100.98.57.55 "kubectl -n database get cronjob -o wide"   # daily-pg-dump target DB?
# Inspect the dump job's --dbname / -d arg in pg-backup.yaml: if it is 'identity'
# only, newhub (lurus_api) is UNPROTECTED and the CronJob must add a lurus_api dump.
```

### (b) Enable + cover newhub (owner)

- **Confirmed: pg_dump covers only `identity`.** Extend the CronJob (platform repo
  `deploy/k8s/cronjobs/pg-backup.yaml`, config diff — not platform *code*) to also dump
  newhub's `lurus_api` DB — add a second dump line in the same job (confirm the exact
  `datname` from newhub's `SQL_DSN` when its Secret is provisioned):

  ```sh
  # alongside the existing `pg_dump -Fc -d identity -f .../lurus-identity-${TS}.dump`:
  pg_dump -Fc -U postgres -h lurus-pg-rw.database.svc.cluster.local \
    -d lurus_api -f "/backups/lurus-api-${TS}.dump"
  ```
  Then set ConfigMap `BACKUP_ENABLED=true`. (Flipping the flag alone backs up only `identity`.)
- For full-cluster PITR: fix the CNPG base-backup overlay-network root cause
  (incident doc §2.3/§5) and confirm `barmanObjectStore` archives resume — this is
  the durable fix; the pg_dump is the interim floor. Storage stays on R6 `/data`
  (295G SSD) / MinIO `pg-backups-v2` per the disk HARD RULE.

### (c) Restore drill — reuse the existing script (no new tooling)

`2l-svc-platform/scripts/drills/backup-restore.sh` (quarterly; spins an ephemeral
`postgres:16` Docker, **never prod**) + this repo's `doc/runbook/pg-restore.md`.
The gap is a **drill record for newhub's DB**, not tooling.

```bash
# Fetch latest dump from R6 and restore into a sandbox, then diff row counts:
bash 2l-svc-platform/scripts/drills/backup-restore.sh --R6_HOST 100.122.83.20
# Ensure the drill targets a lurus_api dump (per (a)); verify schema_migrations=20
# and key table counts against a known checkpoint, then record below.
```

### (d) RPO/RTO target doc (fill after the drill)

| Objective | Target | Measured (drill) |
|---|---|---|
| newhub `lurus_api` covered by a backup? | **YES (target)** | **NO — zero coverage, verified 2026-06-13** |
| RPO (max data loss) | ≤ 6h | _TBD_ |
| RTO (time to restore) | ≤ 1h | _TBD_ |
| Drill date / operator | — | _TBD_ |
| Backup destination + retention | MinIO `pg-backups-v2`, ≥30d | _TBD_ |

**Escalate first:** RPO=∞ vs any SLA is non-negotiable — raise this for approval
before the later-sequenced execution work.

---

## Owner approval checklist (decide; code is ready)

| Item | Action | Why gated |
|---|---|---|
| **Infra-0** | Wire the `STAGING_KUBECONFIG` repo secret so the Hub actually deploys to a staging cluster | **blocks ALL probe/manifest validation** — today the deploy job skips; needs staging cluster creds |
| **P0-5** | Add newhub to ArgoCD appset, `selfHeal:false` start | edits root manifest; autosync vs empty R1 |
| **P1-2 cap** | Sign off `BILLING_DEGRADED_SPEND_CAP_LB` (default 50 LB/tenant/hr) | unsecured-spend business risk |
| **P1-5** | **CONFIRMED 2026-06-13: `lurus_api` backup coverage = ZERO** (pg_dump covers `identity` only; not a schema in identity; CNPG base backup broken since ~03-01). Extend the dump to cover `lurus_api` + run a restore drill | edits platform backup config + storage; **SLA #1** |
| **P2-2** | sealed-secrets choice + decouple shared `IDENTITY_SESSION_SECRET` | controller/secret-custody choice |
| **P2-4** | `hub.lurus.cn` DNS / R1 PROD launch | owner explicitly PMF-gated |

## Deferred by red-team calibration (pre-PMF — do NOT build now)

OTel tracing activation (no real traffic to debug) · automated secret rotation
(manual kubectl suffices at this scale; only the *shared* secret is worth flagging) ·
real HPA autoscaling / aggressive resource tuning (nothing to scale) · PG full circuit
breaker (in-cluster timeout — now backed by `statement_timeout`, P1-1 — suffices) ·
DNS / R1 launch (PMF-gated, runbook-ready only).

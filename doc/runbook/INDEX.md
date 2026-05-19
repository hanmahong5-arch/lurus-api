# Runbooks — Index

Operational playbooks for newhub. One file per failure mode or
recurring procedure. Each runbook starts with **Source** (where the
alert / signal comes from), **Triggered by** (the literal condition),
**Severity**, and **Last review** date.

## Alerts

| Runbook | Trigger | Severity |
|---|---|---|
| [pool-threshold-alert](pool-threshold-alert.md) | `CreditPoolBalanceLow` / `CreditPoolExhausted` Prometheus rules | warning / page |
| [wallet-revert-stranded](wallet-revert-stranded.md) | log line `STRANDED wallet debit` from `tenant_credit_pool.go` | page |

## Procedures (no specific trigger)

| Runbook | When to read |
|---|---|
| [tenant-onboarding](tenant-onboarding.md) | New Reseller signs up — provisioning a tenant + first key |
| [deployment](deployment.md) | Cutting a new image to R6 stage / R1 prod |
| [ha-deployment](ha-deployment.md) | Multi-replica considerations (session secret, batch updates) |
| [staging-environment](staging-environment.md) | Bringing up STAGE on R6 from scratch |
| [database](database.md) | DB shape, common queries, GORM auto-migrate gotchas |
| [pg-restore](pg-restore.md) | Restoring PostgreSQL from backup |
| [incident-response](incident-response.md) | General incident response framework |

## When to add a runbook

- A page-severity alert exists without a runbook → file blocks the
  alert until written.
- A failure mode required manual recovery twice → runbook on second
  occurrence.
- A procedure required >30 min of "look up old Slack threads" → write
  it down.

## Style

- **Symptom** before **Detect** before **Reconcile** before **Recover**
  before **Verify** before **Prevent**. Keep this order.
- Include the exact grep / SQL / curl command. Avoid "check the logs"
  hand-waving.
- Mark any irreversible action with a **4-eyes** requirement.
- Note the source of truth (ADR, audit doc, commit) so future readers
  can verify the runbook hasn't drifted.

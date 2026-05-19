# Story 7-2 Acceptance Report — PostgreSQL HA ADR

**Date**: 2026-05-18
**Status**: `review → managed-PG decision PENDING (Q4)`
**Story doc**: (ADR-driven; managed PG after 5 paying tenants per epic-7 plan)

## Implementation Evidence (markers present)

- ADR captured: decision to defer streaming-replica HA until paid-tenant
  signal validates the operational investment.
- Current PROD: single-node PostgreSQL with WAL-G backups (see Story
  7-2.1 acceptance for backup details).
- Failure mode documented: node loss = RTO = WAL-G restore time
  (~minutes for current DB size).

## Measurement Validity (NOT YET EVIDENCE)

**SELF-VALIDATING — NOT EVIDENCE**: an ADR exists. No streaming
replica has been provisioned. No failover drill has been executed.
The "managed PG after 5 paying tenants" criterion is a planning
decision, not a tested capability.

## STAGE Drill Plan (Q4 — deferred per Q3 cash-path scope)

- Provision streaming replica on R6 (deferred — not in Q3).
- Failover drill: kill primary, observe replica promote, measure RPO/RTO.
- Pass criteria: RPO < 60s, RTO < 5min.

## Status

- markers present: ✅ (ADR)
- measurement meaningful: ⏳ deferred to Q4 by design
- PROD-ready: ✅ for Q3 with WAL-G as documented; HA story is Q4

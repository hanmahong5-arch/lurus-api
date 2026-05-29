-- 018: HA leader-election lease (H1.3).
--
-- GORM AutoMigrate drives the schema from internal/domain/entity.LeaderElection;
-- this file is the PostgreSQL manual-migration counterpart for environments
-- that apply SQL out-of-band. Idempotent — safe to re-run.
--
-- A single row (name = 'master') backs leadership for all master-only
-- background tasks. A process holds leadership while it owns the row and
-- now() < expires_at. See internal/adapter/repo/leader_election.go.

CREATE TABLE IF NOT EXISTS leader_elections (
    name        VARCHAR(64)  PRIMARY KEY,
    holder_id   VARCHAR(128) NOT NULL DEFAULT '',
    acquired_at BIGINT       NOT NULL DEFAULT 0,
    renewed_at  BIGINT       NOT NULL DEFAULT 0,
    expires_at  BIGINT       NOT NULL DEFAULT 0
);

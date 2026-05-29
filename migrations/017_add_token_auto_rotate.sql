-- Migration 017: add automatic key-rotation fields to tokens (Phase H1.4).
--
-- auto_rotate_days > 0 opts a token into scheduled key rotation; the
-- secret-rotation lifecycle task (leader-gated) rotates the key every
-- auto_rotate_days days. 0 (default) disables rotation, preserving the
-- behavior of every existing token.
--
-- rotated_at records the unix-second time of the last automatic rotation;
-- 0 means never auto-rotated, in which case the rotation interval is measured
-- from the token's created_time.
--
-- GORM auto-migrate independently adds these columns for fresh deploys; this
-- file is the PostgreSQL manual-migration counterpart for out-of-band applies.
-- PostgreSQL 11+ adds a column with a constant DEFAULT without rewriting the
-- table, so this is online-safe even on large token tables. Idempotent.

ALTER TABLE tokens ADD COLUMN IF NOT EXISTS auto_rotate_days INTEGER NOT NULL DEFAULT 0;
ALTER TABLE tokens ADD COLUMN IF NOT EXISTS rotated_at BIGINT NOT NULL DEFAULT 0;

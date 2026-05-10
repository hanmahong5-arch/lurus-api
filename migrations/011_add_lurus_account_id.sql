-- Migration: Add lurus_account_id to users for Layer C platform bridge.
--
-- Maps newhub local users to lurus-platform account integer IDs (returned
-- by zita-sdk-go IdentityFromContext). Required by the Layer C bridge
-- endpoint that issues newhub session tokens to SDK-authenticated users.
--
-- Nullable because pre-Layer-C users were created via direct registration
-- and have no platform binding. GORM auto-migrate also adds this column;
-- this SQL serves as the auditable migration record and adds the partial
-- unique index that GORM cannot express directly.
--
-- PostgreSQL 11+ ADD COLUMN without DEFAULT is metadata-only (no rewrite).

ALTER TABLE users ADD COLUMN IF NOT EXISTS lurus_account_id BIGINT;

-- Partial unique index: enforces one-to-one mapping for bound users while
-- allowing multiple NULLs for legacy unbound users. PostgreSQL allows
-- multiple NULLs in a regular unique index by default, but the partial
-- variant is more explicit about intent and avoids confusion.
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_lurus_account_id
  ON users (lurus_account_id)
  WHERE lurus_account_id IS NOT NULL;

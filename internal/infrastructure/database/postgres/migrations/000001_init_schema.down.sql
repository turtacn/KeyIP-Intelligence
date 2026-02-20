-- Migration: 000001_init_schema.down.sql
-- Description: Rollback the initial schema creation.
-- Drops all tables, triggers, and functions in reverse dependency order.

-- ─────────────────────────────────────────────────────────────────────────────
-- Drop tables in reverse dependency order
-- Tables with foreign key references must be dropped before referenced tables
-- ─────────────────────────────────────────────────────────────────────────────

DROP TABLE IF EXISTS workspaces CASCADE;

DROP TABLE IF EXISTS patent_lifecycles CASCADE;

DROP TABLE IF EXISTS portfolios CASCADE;

DROP TABLE IF EXISTS markush_structures CASCADE;

DROP TABLE IF EXISTS claims CASCADE;

DROP TABLE IF EXISTS molecules CASCADE;

DROP TABLE IF EXISTS patents CASCADE;

-- ─────────────────────────────────────────────────────────────────────────────
-- Drop trigger function
-- ─────────────────────────────────────────────────────────────────────────────

DROP FUNCTION IF EXISTS update_updated_at_column() CASCADE;

-- ─────────────────────────────────────────────────────────────────────────────
-- Drop extension (optional, as other migrations may depend on it)
-- ─────────────────────────────────────────────────────────────────────────────

-- DROP EXTENSION IF EXISTS "pgcrypto" CASCADE;


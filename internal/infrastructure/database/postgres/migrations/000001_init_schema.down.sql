-- Migration: 000001_init_schema.down.sql
-- Description: Rollback the initial schema creation.
-- Drops all tables, triggers, and functions in reverse dependency order.

DROP TABLE IF EXISTS workspaces CASCADE;
DROP TABLE IF EXISTS lifecycles CASCADE;
DROP TABLE IF EXISTS portfolios CASCADE;
DROP TABLE IF EXISTS molecules CASCADE;
DROP TABLE IF EXISTS markush_structures CASCADE;
DROP TABLE IF EXISTS patent_claims CASCADE;
DROP TABLE IF EXISTS patents CASCADE;

DROP FUNCTION IF EXISTS update_updated_at_column() CASCADE;

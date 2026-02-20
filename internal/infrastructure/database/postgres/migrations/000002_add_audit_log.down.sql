-- Migration: 000002_add_audit_log.down.sql
-- Description: Rollback audit_logs table and related functions.

-- ─────────────────────────────────────────────────────────────────────────────
-- Drop audit_logs table (CASCADE drops all partitions automatically)
-- ─────────────────────────────────────────────────────────────────────────────

DROP TABLE IF EXISTS audit_logs CASCADE;

-- ─────────────────────────────────────────────────────────────────────────────
-- Drop helper function
-- ─────────────────────────────────────────────────────────────────────────────

DROP FUNCTION IF EXISTS create_audit_log_partition(DATE) CASCADE;


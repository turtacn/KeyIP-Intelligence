-- +migrate Up

-- Add 'pending' to molecule_status enum.
-- The domain entity defines MoleculeStatusPending = "pending", but the original
-- molecule_status ENUM only had 'active', 'archived', 'deleted', 'pending_review'.
-- This adds 'pending' BEFORE 'active' to match the expected lifecycle order:
-- pending -> active -> archived -> deleted
ALTER TYPE molecule_status ADD VALUE IF NOT EXISTS 'pending' BEFORE 'active';

-- +migrate Down

-- PostgreSQL does not support removing individual values from an ENUM type.
-- To revert, one would need to:
--   1. CREATE TYPE molecule_status_new AS ENUM ('active', 'archived', 'deleted', 'pending_review');
--   2. ALTER TABLE molecules ALTER COLUMN status TYPE molecule_status_new USING status::text::molecule_status_new;
--   3. DROP TYPE molecule_status;
--   4. ALTER TYPE molecule_status_new RENAME TO molecule_status;
-- This is intentionally left as a no-op migration since removing enum values
-- cannot be reliably done without knowing the current DB state.
SELECT 1;

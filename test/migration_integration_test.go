// Package clitest provides end-to-end integration tests for the KeyIP-Intelligence
// database migration system. These tests verify that all 7 migrations can be applied
// to a real PostgreSQL instance, that the resulting schema is correct, that data
// can be inserted and queried, and that down migrations roll back cleanly.
//
//go:build integration

package clitest

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/database/postgres"
)

// ─────────────────────────────────────────────────────────────────────────────
// Paths and helpers
// ─────────────────────────────────────────────────────────────────────────────

// migrationsDir returns the "file://" prefixed path to the SQL migration files.
// projectRoot is defined in cli_smoke_test.go (same package, same build tag).
func migrationsDir(t *testing.T) string {
	t.Helper()
	return "file://" + filepath.Join(projectRoot(t), "internal", "infrastructure", "database", "postgres", "migrations")
}

// getTestDBURL returns the database URL or skips the test.
// Checks INTEGRATION_TEST_DB_URL first, then DATABASE_URL.
func getTestDBURL(t *testing.T) string {
	t.Helper()

	dbURL := os.Getenv("INTEGRATION_TEST_DB_URL")
	if dbURL == "" {
		dbURL = os.Getenv("DATABASE_URL")
	}
	if dbURL == "" {
		t.Skip("INTEGRATION_TEST_DB_URL or DATABASE_URL not set; skipping integration test")
	}
	return dbURL
}

// openTestDB opens a database connection and returns it with a cleanup function.
func openTestDB(t *testing.T) (*sql.DB, func()) {
	t.Helper()

	dbURL := getTestDBURL(t)

	db, err := sql.Open("postgres", dbURL)
	require.NoError(t, err, "failed to open database connection")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = db.PingContext(ctx)
	require.NoError(t, err, "failed to ping database")

	cleanup := func() {
		_ = db.Close()
	}

	return db, cleanup
}

// resetAndApplyMigrations fully resets the DB and re-applies all migrations.
func resetAndApplyMigrations(t *testing.T) {
	t.Helper()
	dbURL := getTestDBURL(t)

	err := postgres.ResetDatabase(dbURL, migrationsDir(t))
	require.NoError(t, err, "ResetDatabase should succeed")
}

// rollbackAll rolls back all migrations to version 0.
func rollbackAll(t *testing.T) {
	t.Helper()
	dbURL := getTestDBURL(t)

	err := postgres.RollbackMigration(dbURL, migrationsDir(t), 100)
	if err != nil {
		t.Logf("rollback all (best-effort): %v", err)
	}
}

// tableExists checks if a table exists in the public schema.
func tableExists(ctx context.Context, t *testing.T, db *sql.DB, table string) bool {
	t.Helper()
	var exists bool
	err := db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT FROM information_schema.tables
			WHERE table_schema = 'public' AND table_name = $1
		)
	`, table).Scan(&exists)
	require.NoError(t, err, "error checking table %s", table)
	return exists
}

// ─────────────────────────────────────────────────────────────────────────────
// 4a. Run all up migrations
// ─────────────────────────────────────────────────────────────────────────────

// TestMigration_ApplyAllUp validates that all 7 migrations apply successfully
// and the migration version reaches 7.
func TestMigration_ApplyAllUp(t *testing.T) {
	dbURL := getTestDBURL(t)

	rollbackAll(t)

	err := postgres.RunMigrations(dbURL, migrationsDir(t))
	require.NoError(t, err, "all 7 migrations should apply")

	version, dirty, err := postgres.MigrationStatus(dbURL, migrationsDir(t))
	require.NoError(t, err)
	assert.False(t, dirty, "should not be dirty")
	assert.GreaterOrEqual(t, version, uint(7), "should be at version 7+")
}

// ─────────────────────────────────────────────────────────────────────────────
// 4b. Verify all tables exist with correct structure
// ─────────────────────────────────────────────────────────────────────────────

// TestMigration_AllTablesExist verifies that exactly the expected set of tables
// exists after applying all migrations.
func TestMigration_AllTablesExist(t *testing.T) {
	resetAndApplyMigrations(t)
	db, cleanup := openTestDB(t)
	defer cleanup()

	ctx := context.Background()

	expectedTables := []string{
		// Migration 001 - Patents
		"patents", "patent_claims", "patent_inventors", "patent_priority_claims",
		// Migration 002 - Molecules
		"molecules", "molecule_fingerprints", "molecule_properties", "patent_molecule_relations",
		// Migration 003 - Portfolios
		"portfolios", "portfolio_patents", "patent_valuations", "portfolio_health_scores",
		"portfolio_optimization_suggestions",
		// Migration 004 - Lifecycle
		"patent_annuities", "patent_deadlines", "patent_lifecycle_events", "patent_cost_records",
		// Migration 005 - Users
		"users", "organizations", "organization_members", "roles", "user_roles", "api_keys", "audit_logs",
		// Migration 006 - Workspaces
		"workspaces", "workspace_members", "workspace_projects", "project_patents", "project_molecules",
		"comments", "notifications", "saved_searches",
	}

	for _, table := range expectedTables {
		assert.True(t, tableExists(ctx, t, db, table), "table %s should exist after migrations", table)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// 4c. Verify foreign key constraints and indexes
// ─────────────────────────────────────────────────────────────────────────────

// TestMigration_ForeignKeysExist validates that all expected foreign key
// constraints are created by the migrations.
func TestMigration_ForeignKeysExist(t *testing.T) {
	resetAndApplyMigrations(t)
	db, cleanup := openTestDB(t)
	defer cleanup()

	ctx := context.Background()

	tests := []struct {
		name         string
		constraint   string
		table        string
		refTable     string
		refColumn    string
		deleteAction string
	}{
		{"patent_claims -> patents", "patent_claims_patent_id_fkey", "patent_claims", "patents", "id", "CASCADE"},
		{"patent_inventors -> patents", "patent_inventors_patent_id_fkey", "patent_inventors", "patents", "id", "CASCADE"},
		{"patent_priority_claims -> patents", "patent_priority_claims_patent_id_fkey", "patent_priority_claims", "patents", "id", "CASCADE"},
		{"molecule_fingerprints -> molecules", "molecule_fingerprints_molecule_id_fkey", "molecule_fingerprints", "molecules", "id", "CASCADE"},
		{"molecule_properties -> molecules", "molecule_properties_molecule_id_fkey", "molecule_properties", "molecules", "id", "CASCADE"},
		{"patent_molecule_relations -> patents", "patent_molecule_relations_patent_id_fkey", "patent_molecule_relations", "patents", "id", "CASCADE"},
		{"patent_molecule_relations -> molecules", "patent_molecule_relations_molecule_id_fkey", "patent_molecule_relations", "molecules", "id", "CASCADE"},
		{"portfolio_patents -> portfolios", "portfolio_patents_portfolio_id_fkey", "portfolio_patents", "portfolios", "id", "CASCADE"},
		{"portfolio_patents -> patents", "portfolio_patents_patent_id_fkey", "portfolio_patents", "patents", "id", "CASCADE"},
		{"patent_valuations -> patents", "patent_valuations_patent_id_fkey", "patent_valuations", "patents", "id", "CASCADE"},
		{"portfolio_health_scores -> portfolios", "portfolio_health_scores_portfolio_id_fkey", "portfolio_health_scores", "portfolios", "id", "CASCADE"},
		{"patent_annuities -> patents", "patent_annuities_patent_id_fkey", "patent_annuities", "patents", "id", "CASCADE"},
		{"patent_deadlines -> patents", "patent_deadlines_patent_id_fkey", "patent_deadlines", "patents", "id", "CASCADE"},
		{"patent_lifecycle_events -> patents", "patent_lifecycle_events_patent_id_fkey", "patent_lifecycle_events", "patents", "id", "CASCADE"},
		{"org_members -> organizations", "organization_members_organization_id_fkey", "organization_members", "organizations", "id", "CASCADE"},
		{"org_members -> users", "organization_members_user_id_fkey", "organization_members", "users", "id", "CASCADE"},
		{"user_roles -> users", "user_roles_user_id_fkey", "user_roles", "users", "id", "CASCADE"},
		{"user_roles -> roles", "user_roles_role_id_fkey", "user_roles", "roles", "id", "CASCADE"},
		{"api_keys -> users", "api_keys_user_id_fkey", "api_keys", "users", "id", "CASCADE"},
		{"patents assignee -> users (SET NULL)", "fk_patents_assignee", "patents", "users", "id", "SET NULL"},
		{"portfolios owner -> users (RESTRICT)", "fk_portfolios_owner", "portfolios", "users", "id", "RESTRICT"},
		{"workspaces -> users (RESTRICT)", "workspaces_owner_id_fkey", "workspaces", "users", "id", "RESTRICT"},
		{"workspace_members -> workspaces", "workspace_members_workspace_id_fkey", "workspace_members", "workspaces", "id", "CASCADE"},
		{"workspace_members -> users", "workspace_members_user_id_fkey", "workspace_members", "users", "id", "CASCADE"},
		{"comments -> users", "comments_author_id_fkey", "comments", "users", "id", "CASCADE"},
		{"notifications -> users", "notifications_user_id_fkey", "notifications", "users", "id", "CASCADE"},
		{"saved_searches -> users", "saved_searches_user_id_fkey", "saved_searches", "users", "id", "CASCADE"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var refTable, refCol, deleteRule string
			err := db.QueryRowContext(ctx, `
				SELECT
					confrelid::regclass::text AS ref_table,
					a.attname AS ref_column,
					CASE c.confdeltype
						WHEN 'a' THEN 'NO ACTION'
						WHEN 'r' THEN 'RESTRICT'
						WHEN 'c' THEN 'CASCADE'
						WHEN 'n' THEN 'SET NULL'
						WHEN 'd' THEN 'SET DEFAULT'
						ELSE 'UNKNOWN'
					END AS delete_rule
				FROM pg_constraint c
				JOIN pg_attribute a ON a.attnum = c.confkey[1] AND a.attrelid = c.confrelid
				WHERE c.conname = $1 AND c.contype = 'f'
			`, tt.constraint).Scan(&refTable, &refCol, &deleteRule)
			require.NoError(t, err, "FK %s should exist", tt.constraint)
			assert.Equal(t, tt.refTable, refTable, "FK %s should reference %s", tt.constraint, tt.refTable)
			assert.Equal(t, tt.refColumn, refCol, "FK %s should reference column %s", tt.constraint, tt.refColumn)
			assert.Equal(t, tt.deleteAction, deleteRule, "FK %s delete rule mismatch", tt.constraint)
		})
	}
}

// TestMigration_IndexesExist validates that key indexes are created.
func TestMigration_IndexesExist(t *testing.T) {
	resetAndApplyMigrations(t)
	db, cleanup := openTestDB(t)
	defer cleanup()

	ctx := context.Background()

	expected := []struct {
		indexName string
		tableName string
		isPartial bool
	}{
		{"idx_patents_patent_number", "patents", false},
		{"idx_patents_status", "patents", false},
		{"idx_patents_jurisdiction", "patents", false},
		{"idx_patents_deleted_at", "patents", true},
		{"idx_patent_claims_patent_id", "patent_claims", false},
		{"idx_molecules_inchi_key", "molecules", false},
		{"idx_molecules_deleted_at", "molecules", true},
		{"idx_molecule_props_molecule_id", "molecule_properties", false},
		{"idx_patent_mol_rel_patent_id", "patent_molecule_relations", false},
		{"idx_portfolios_owner_id", "portfolios", false},
		{"idx_portfolios_deleted_at", "portfolios", true},
		{"idx_patent_valuations_patent_id", "patent_valuations", false},
		{"idx_annuities_due_date", "patent_annuities", false},
		{"idx_deadlines_patent_id", "patent_deadlines", false},
		{"idx_lifecycle_events_patent_id", "patent_lifecycle_events", false},
		{"idx_cost_records_patent_id", "patent_cost_records", false},
		{"idx_users_email", "users", false},
		{"idx_users_deleted_at", "users", true},
		{"idx_orgs_slug", "organizations", false},
		{"idx_org_members_user_id", "organization_members", false},
		{"idx_audit_logs_created_at", "audit_logs", false},
		{"idx_workspaces_owner_id", "workspaces", false},
		{"idx_workspaces_deleted_at", "workspaces", true},
		{"idx_comments_resource", "comments", false},
		{"idx_notifications_user_id", "notifications", false},
		{"idx_saved_searches_user_id", "saved_searches", false},
	}

	for _, idx := range expected {
		t.Run(idx.indexName, func(t *testing.T) {
			var indexDef string
			err := db.QueryRowContext(ctx, `
				SELECT indexdef FROM pg_indexes
				WHERE indexname = $1 AND tablename = $2 AND schemaname = 'public'
			`, idx.indexName, idx.tableName).Scan(&indexDef)
			require.NoError(t, err, "index %s should exist on %s", idx.indexName, idx.tableName)

			if idx.isPartial {
				assert.Contains(t, indexDef, "WHERE",
					"index %s should be a partial index", idx.indexName)
			}
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// 4d. Verify NOT NULL constraints
// ─────────────────────────────────────────────────────────────────────────────

// TestMigration_NotNullConstraints verifies that key columns have correct
// NOT NULL / nullable constraints.
func TestMigration_NotNullConstraints(t *testing.T) {
	resetAndApplyMigrations(t)
	db, cleanup := openTestDB(t)
	defer cleanup()

	ctx := context.Background()

	tests := []struct {
		table   string
		column  string
		notNull bool
	}{
		// Patents
		{"patents", "id", true},
		{"patents", "patent_number", true},
		{"patents", "title", true},
		{"patents", "patent_type", true},
		{"patents", "status", true},
		{"patents", "jurisdiction", true},
		{"patents", "source", true},
		{"patents", "created_at", true},
		{"patents", "updated_at", true},
		{"patents", "deleted_at", false},
		{"patents", "raw_data", false},
		{"patents", "assignee_id", false},

		// Patent claims
		{"patent_claims", "patent_id", true},
		{"patent_claims", "claim_number", true},
		{"patent_claims", "claim_text", true},

		// Molecules
		{"molecules", "id", true},
		{"molecules", "smiles", true},
		{"molecules", "canonical_smiles", true},
		{"molecules", "status", true},
		{"molecules", "source", true},

		// Portfolios
		{"portfolios", "name", true},
		{"portfolios", "owner_id", true},
		{"portfolios", "status", true},

		// Patent valuations
		{"patent_valuations", "technical_score", true},
		{"patent_valuations", "composite_score", true},
		{"patent_valuations", "tier", true},

		// Users
		{"users", "email", true},
		{"users", "username", true},
		{"users", "display_name", true},
		{"users", "status", true},
		{"users", "locale", true},
		{"users", "timezone", true},
		{"users", "mfa_enabled", true},
		{"users", "password_hash", false},
		{"users", "avatar_url", false},

		// Workspaces
		{"workspaces", "name", true},
		{"workspaces", "owner_id", true},
		{"workspaces", "workspace_type", true},
		{"workspaces", "is_archived", true},
	}

	for _, tt := range tests {
		t.Run(tt.table+"."+tt.column, func(t *testing.T) {
			var isNullable string
			err := db.QueryRowContext(ctx, `
				SELECT is_nullable FROM information_schema.columns
				WHERE table_schema = 'public' AND table_name = $1 AND column_name = $2
			`, tt.table, tt.column).Scan(&isNullable)
			require.NoError(t, err, "column %s.%s should exist", tt.table, tt.column)

			if tt.notNull {
				assert.Equal(t, "NO", isNullable,
					"column %s.%s should be NOT NULL", tt.table, tt.column)
			} else {
				assert.Equal(t, "YES", isNullable,
					"column %s.%s should be nullable", tt.table, tt.column)
			}
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// 4e. Data insertion and query tests
// ─────────────────────────────────────────────────────────────────────────────

// TestMigration_InsertAndQueryPatents inserts a patent record, verifies
// cascading inserts into related tables, and queries them back.
func TestMigration_InsertAndQueryPatents(t *testing.T) {
	resetAndApplyMigrations(t)
	db, cleanup := openTestDB(t)
	defer cleanup()

	ctx := context.Background()

	_, err := db.ExecContext(ctx, `
		INSERT INTO patents (patent_number, title, patent_type, status, jurisdiction, source)
		VALUES ($1, $2, $3, $4, $5, $6)
	`,
		"US-2024-0000001-A1",
		"Test Patent for Migration Verification",
		"invention",
		"filed",
		"US",
		"manual",
	)
	require.NoError(t, err, "should insert into patents")

	var (
		id           string
		patentNumber string
		title        string
		patentType   string
		status       string
		jurisdiction string
		source       string
		createdAt    time.Time
		updatedAt    time.Time
		filingDate   *time.Time
		assigneeID   *string
		ipcCodes     []byte
		rawData      []byte
		metadata     []byte
	)
	err = db.QueryRowContext(ctx, `
		SELECT id, patent_number, title, patent_type, status, jurisdiction,
		       source, created_at, updated_at, filing_date, assignee_id,
		       ipc_codes, raw_data, metadata
		FROM patents WHERE patent_number = $1
	`, "US-2024-0000001-A1").Scan(
		&id, &patentNumber, &title, &patentType, &status, &jurisdiction,
		&source, &createdAt, &updatedAt, &filingDate, &assigneeID,
		&ipcCodes, &rawData, &metadata,
	)
	require.NoError(t, err, "should query patent back")
	assert.Equal(t, "US-2024-0000001-A1", patentNumber)
	assert.Equal(t, "Test Patent for Migration Verification", title)
	assert.Equal(t, "invention", patentType)
	assert.Equal(t, "filed", status)
	assert.Equal(t, "US", jurisdiction)
	assert.Equal(t, "manual", source)
	assert.NotEmpty(t, id, "id should be auto-generated")
	assert.False(t, createdAt.IsZero(), "created_at should be set")
	assert.False(t, updatedAt.IsZero(), "updated_at should be set")
	assert.Nil(t, filingDate, "filing_date should be NULL (not provided)")
	assert.Nil(t, assigneeID, "assignee_id should be NULL (not provided)")
	t.Logf("Inserted patent: id=%s", id)

	patentID := id

	_, err = db.ExecContext(ctx, `
		INSERT INTO patent_claims (patent_id, claim_number, claim_type, claim_text)
		VALUES ($1, 1, 'independent', 'A test claim for verification.')
	`, patentID)
	require.NoError(t, err, "should insert into patent_claims")

	var claimNumber int
	var claimText string
	err = db.QueryRowContext(ctx, `
		SELECT claim_number, claim_text FROM patent_claims WHERE patent_id = $1
	`, patentID).Scan(&claimNumber, &claimText)
	require.NoError(t, err)
	assert.Equal(t, 1, claimNumber)
	assert.Equal(t, "A test claim for verification.", claimText)

	_, err = db.ExecContext(ctx, `
		INSERT INTO patent_inventors (patent_id, inventor_name, sequence)
		VALUES ($1, 'Test Inventor', 1)
	`, patentID)
	require.NoError(t, err, "should insert into patent_inventors")

	// Verify cascade delete: delete patent, related rows should vanish.
	_, err = db.ExecContext(ctx, `DELETE FROM patents WHERE id = $1`, patentID)
	require.NoError(t, err)

	var cnt int
	err = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM patent_claims WHERE patent_id = $1`, patentID).Scan(&cnt)
	require.NoError(t, err)
	assert.Equal(t, 0, cnt, "claims should be cascade-deleted with patent")
}

// TestMigration_InsertAndQueryMolecules inserts a molecule, verifies related
// tables (fingerprints, properties), and tests cascade behavior.
func TestMigration_InsertAndQueryMolecules(t *testing.T) {
	resetAndApplyMigrations(t)
	db, cleanup := openTestDB(t)
	defer cleanup()

	ctx := context.Background()

	_, err := db.ExecContext(ctx, `
		INSERT INTO molecules (smiles, canonical_smiles, inchi_key, molecular_formula,
		                       molecular_weight, status, name, source)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`,
		"CCO",
		"CCO",
		"LFQSCWFLJHTTHZ-UHFFFAOYSA-N",
		"C2H6O",
		46.07,
		"active",
		"Ethanol",
		"manual",
	)
	require.NoError(t, err, "should insert into molecules")

	var molID string
	var smiles, canonicalSmiles, inchiKey, formula string
	var mw float64
	var status, name string
	err = db.QueryRowContext(ctx, `
		SELECT id, smiles, canonical_smiles, inchi_key, molecular_formula,
		       molecular_weight, status, name
		FROM molecules WHERE inchi_key = $1
	`, "LFQSCWFLJHTTHZ-UHFFFAOYSA-N").Scan(
		&molID, &smiles, &canonicalSmiles, &inchiKey, &formula, &mw, &status, &name,
	)
	require.NoError(t, err)
	assert.Equal(t, "CCO", smiles)
	assert.Equal(t, "Ethanol", name)
	assert.Equal(t, "active", status)
	t.Logf("Inserted molecule: id=%s, mw=%.2f", molID, mw)

	_, err = db.ExecContext(ctx, `
		INSERT INTO molecule_fingerprints (molecule_id, fingerprint_type, parameters)
		VALUES ($1, 'morgan', '{"radius": 2}'::jsonb)
	`, molID)
	require.NoError(t, err, "should insert into molecule_fingerprints")

	_, err = db.ExecContext(ctx, `
		INSERT INTO molecule_properties (molecule_id, property_type, value, unit, data_source)
		VALUES ($1, 'logP', 1.2, '', 'computed')
	`, molID)
	require.NoError(t, err, "should insert into molecule_properties")

	// Verify cascade delete.
	_, err = db.ExecContext(ctx, `DELETE FROM molecules WHERE id = $1`, molID)
	require.NoError(t, err)

	var fpCount int
	err = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM molecule_fingerprints WHERE molecule_id = $1`, molID).Scan(&fpCount)
	require.NoError(t, err)
	assert.Equal(t, 0, fpCount, "fingerprints should cascade-delete with molecule")

	var propCount int
	err = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM molecule_properties WHERE molecule_id = $1`, molID).Scan(&propCount)
	require.NoError(t, err)
	assert.Equal(t, 0, propCount, "properties should cascade-delete with molecule")
}

// TestMigration_InsertAndQueryUsers inserts a user, creates an organization,
// adds the user as a member, and queries all relationships back.
func TestMigration_InsertAndQueryUsers(t *testing.T) {
	resetAndApplyMigrations(t)
	db, cleanup := openTestDB(t)
	defer cleanup()

	ctx := context.Background()

	_, err := db.ExecContext(ctx, `
		INSERT INTO users (email, username, display_name, locale, timezone)
		VALUES ($1, $2, $3, $4, $5)
	`, "test@keyip.example", "testuser", "Test User", "en-US", "UTC")
	require.NoError(t, err, "should insert into users")

	var userID string
	var email, username, status, locale string
	err = db.QueryRowContext(ctx, `
		SELECT id, email, username, status, locale FROM users WHERE email = $1
	`, "test@keyip.example").Scan(&userID, &email, &username, &status, &locale)
	require.NoError(t, err)
	assert.Equal(t, "testuser", username)
	assert.Equal(t, "pending_verification", status, "default user status should be pending_verification")
	assert.Equal(t, "en-US", locale)

	_, err = db.ExecContext(ctx, `
		INSERT INTO organizations (name, slug, plan)
		VALUES ($1, $2, $3)
	`, "Test Organization", "test-org", "free")
	require.NoError(t, err)

	var orgID string
	err = db.QueryRowContext(ctx, `SELECT id FROM organizations WHERE slug = $1`, "test-org").Scan(&orgID)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, `
		INSERT INTO organization_members (organization_id, user_id, role)
		VALUES ($1, $2, 'admin')
	`, orgID, userID)
	require.NoError(t, err, "should insert into organization_members")

	var role string
	err = db.QueryRowContext(ctx, `
		SELECT role FROM organization_members
		WHERE organization_id = $1 AND user_id = $2
	`, orgID, userID).Scan(&role)
	require.NoError(t, err)
	assert.Equal(t, "admin", role)

	// Verify audit_logs table exists (no data inserted, just structural).
	assert.True(t, tableExists(ctx, t, db, "audit_logs"), "audit_logs table should exist")

	// Verify built-in roles exist (seeded by migration 005).
	expectedRoles := []string{"super_admin", "org_admin", "patent_analyst", "researcher", "viewer"}
	for _, roleName := range expectedRoles {
		var exists bool
		err := db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM roles WHERE name = $1)`, roleName).Scan(&exists)
		require.NoError(t, err)
		assert.True(t, exists, "role %s should be seeded", roleName)
	}
}

// TestMigration_InsertAndQueryWorkspaces verifies the workspace-related tables
// and foreign key relationships.
func TestMigration_InsertAndQueryWorkspaces(t *testing.T) {
	resetAndApplyMigrations(t)
	db, cleanup := openTestDB(t)
	defer cleanup()

	ctx := context.Background()

	_, err := db.ExecContext(ctx, `
		INSERT INTO users (email, username, display_name)
		VALUES ($1, $2, $3)
	`, "workspace-test@keyip.example", "wsuser", "Workspace User")
	require.NoError(t, err)

	var userID string
	err = db.QueryRowContext(ctx, `SELECT id FROM users WHERE email = $1`, "workspace-test@keyip.example").Scan(&userID)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, `
		INSERT INTO workspaces (name, workspace_type, owner_id, max_collaborators)
		VALUES ($1, $2, $3, $4)
	`, "Test Workspace", "personal", userID, 5)
	require.NoError(t, err)

	var wsID string
	err = db.QueryRowContext(ctx, `SELECT id FROM workspaces WHERE name = $1`, "Test Workspace").Scan(&wsID)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, `
		INSERT INTO workspace_members (workspace_id, user_id, role)
		VALUES ($1, $2, 'owner')
	`, wsID, userID)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, `
		INSERT INTO workspace_projects (workspace_id, name, project_type, status)
		VALUES ($1, $2, $3, $4)
	`, wsID, "Test Project", "analysis", "active")
	require.NoError(t, err)

	var projID string
	err = db.QueryRowContext(ctx, `SELECT id FROM workspace_projects WHERE name = $1`, "Test Project").Scan(&projID)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, `
		INSERT INTO comments (author_id, resource_type, resource_id, content)
		VALUES ($1, $2, $3, $4)
	`, userID, "project", projID, "Test comment for verification.")
	require.NoError(t, err)

	var commentCnt int
	err = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM comments WHERE resource_id = $1`, projID).Scan(&commentCnt)
	require.NoError(t, err)
	assert.Equal(t, 1, commentCnt, "should have one comment")

	// Clean up workspace (cascade should clean up members, projects, comments).
	_, err = db.ExecContext(ctx, `DELETE FROM workspaces WHERE id = $1`, wsID)
	require.NoError(t, err)

	err = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM workspace_members WHERE workspace_id = $1`, wsID).Scan(&commentCnt)
	require.NoError(t, err)
	assert.Equal(t, 0, commentCnt, "workspace_members should cascade-delete")
}

// TestMigration_InsertAndQueryPortfolioLifecycle tests portfolio and lifecycle
// data insertion with cross-table relationships.
func TestMigration_InsertAndQueryPortfolioLifecycle(t *testing.T) {
	resetAndApplyMigrations(t)
	db, cleanup := openTestDB(t)
	defer cleanup()

	ctx := context.Background()

	_, err := db.ExecContext(ctx, `
		INSERT INTO users (email, username, display_name)
		VALUES ($1, $2, $3)
	`, "portfolio-lifecycle@keyip.example", "pluser", "PL User")
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, `
		INSERT INTO patents (patent_number, title, patent_type, status, jurisdiction, source)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, "US-2024-PORTFOLIO-TEST", "Portfolio Test Patent", "invention", "granted", "US", "manual")
	require.NoError(t, err)

	var userID, patentID string
	err = db.QueryRowContext(ctx, `SELECT id FROM patents WHERE patent_number = $1`, "US-2024-PORTFOLIO-TEST").Scan(&patentID)
	require.NoError(t, err)
	err = db.QueryRowContext(ctx, `SELECT id FROM users WHERE email = $1`, "portfolio-lifecycle@keyip.example").Scan(&userID)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, `
		INSERT INTO portfolios (name, owner_id, status)
		VALUES ($1, $2, $3)
	`, "Test Portfolio", userID, "active")
	require.NoError(t, err)

	var portfolioID string
	err = db.QueryRowContext(ctx, `SELECT id FROM portfolios WHERE name = $1`, "Test Portfolio").Scan(&portfolioID)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, `
		INSERT INTO portfolio_patents (portfolio_id, patent_id)
		VALUES ($1, $2)
	`, portfolioID, patentID)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, `
		INSERT INTO patent_valuations (patent_id, portfolio_id, technical_score, legal_score,
		                               market_score, strategic_score, composite_score,
		                               tier, valuation_method, scoring_details)
		VALUES ($1, $2, 85.0, 70.0, 60.0, 75.0, 72.5, 'A', 'automated', '{}'::jsonb)
	`, patentID, portfolioID)
	require.NoError(t, err)

	var compositeScore float64
	var tier string
	err = db.QueryRowContext(ctx, `
		SELECT composite_score, tier FROM patent_valuations
		WHERE patent_id = $1
	`, patentID).Scan(&compositeScore, &tier)
	require.NoError(t, err)
	assert.InDelta(t, 72.5, compositeScore, 0.01)
	assert.Equal(t, "A", tier)

	_, err = db.ExecContext(ctx, `
		INSERT INTO patent_annuities (patent_id, year_number, due_date, status, amount, currency)
		VALUES ($1, 1, $2, 'upcoming', 5000, 'USD')
	`, patentID, time.Now().AddDate(1, 0, 0))
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, `
		INSERT INTO patent_deadlines (patent_id, deadline_type, title, description,
		                              due_date, original_due_date, priority)
		VALUES ($1, 'filing_anniversary', 'Year 1 Anniversary', 'First year anniversary check',
		        $2, $2, 'high')
	`, patentID, time.Now().AddDate(1, 0, 0))
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, `
		INSERT INTO patent_lifecycle_events (patent_id, event_type, event_date, title)
		VALUES ($1, 'grant', $2, 'Patent granted')
	`, patentID, time.Now())
	require.NoError(t, err)

	var eventCount int
	err = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM patent_lifecycle_events WHERE patent_id = $1`, patentID).Scan(&eventCount)
	require.NoError(t, err)
	assert.Equal(t, 1, eventCount, "should have one lifecycle event")

	_, err = db.ExecContext(ctx, `
		INSERT INTO patent_cost_records (patent_id, cost_type, amount, currency, incurred_date)
		VALUES ($1, 'filing', 15000, 'USD', $2)
	`, patentID, time.Now())
	require.NoError(t, err)

	var costCount int
	err = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM patent_cost_records WHERE patent_id = $1`, patentID).Scan(&costCount)
	require.NoError(t, err)
	assert.Equal(t, 1, costCount, "should have one cost record")
}

// TestMigration_EnumValues verifies that all PostgreSQL ENUM types have the
// expected values after migrations (including 007's addition of 'pending').
func TestMigration_EnumValues(t *testing.T) {
	resetAndApplyMigrations(t)
	db, cleanup := openTestDB(t)
	defer cleanup()

	ctx := context.Background()

	expectedEnums := map[string][]string{
		"patent_status":       {"draft", "filed", "published", "under_examination", "granted", "rejected", "withdrawn", "expired", "lapsed", "invalidated"},
		"patent_type":         {"invention", "utility_model", "design", "plant", "provisional"},
		"molecule_status":     {"pending", "active", "archived", "deleted", "pending_review"},
		"portfolio_status":    {"active", "archived", "draft"},
		"valuation_tier":      {"S", "A", "B", "C", "D"},
		"annuity_status":      {"upcoming", "due", "overdue", "paid", "grace_period", "waived", "abandoned"},
		"lifecycle_event_type": {"filing", "publication", "examination_request", "office_action", "response_filed", "grant", "annuity_payment", "annuity_missed", "renewal", "assignment", "license", "opposition", "invalidation", "expiry", "restoration", "abandonment", "status_change", "custom"},
		"deadline_status":     {"active", "completed", "missed", "extended", "waived"},
		"user_status":         {"active", "inactive", "suspended", "pending_verification"},
		"workspace_type":      {"personal", "team", "project"},
		"notification_type":   {"deadline_reminder", "annuity_due", "task_assigned", "comment_mention", "analysis_complete", "portfolio_alert", "system_announcement", "invitation"},
	}

	for enumName, expectedValues := range expectedEnums {
		t.Run(enumName, func(t *testing.T) {
			rows, err := db.QueryContext(ctx, `
				SELECT enumlabel FROM pg_enum
				WHERE enumtypid = $1::regtype
				ORDER BY enumsortorder
			`, enumName)
			require.NoError(t, err, "enum %s should exist", enumName)
			defer rows.Close()

			var actualValues []string
			for rows.Next() {
				var val string
				require.NoError(t, rows.Scan(&val))
				actualValues = append(actualValues, val)
			}
			require.NoError(t, rows.Err())

			assert.Equal(t, expectedValues, actualValues,
				"enum %s values mismatch", enumName)
		})
	}
}

// TestMigration_DefaultValues verifies that columns have correct default values.
func TestMigration_DefaultValues(t *testing.T) {
	resetAndApplyMigrations(t)
	db, cleanup := openTestDB(t)
	defer cleanup()

	ctx := context.Background()

	tests := []struct {
		table        string
		column       string
		defaultMatch string
	}{
		{"patents", "patent_type", "'invention'::patent_type"},
		{"patents", "status", "'draft'::patent_status"},
		{"patents", "source", "'manual'"},
		{"patents", "created_at", "now()"},
		{"molecules", "status", "'active'::molecule_status"},
		{"users", "status", "'pending_verification'::user_status"},
		{"users", "locale", "'zh-CN'"},
		{"users", "timezone", "'Asia/Shanghai'"},
		{"users", "mfa_enabled", "false"},
		{"organizations", "plan", "'free'"},
		{"organizations", "max_members", "5"},
		{"organizations", "max_patents", "100"},
		{"api_keys", "is_active", "true"},
		{"api_keys", "rate_limit", "1000"},
		{"patent_annuities", "status", "'upcoming'::annuity_status"},
		{"patent_deadlines", "status", "'active'::deadline_status"},
		{"workspaces", "is_archived", "false"},
	}

	for _, tt := range tests {
		t.Run(tt.table+"."+tt.column, func(t *testing.T) {
			var defaultVal *string
			err := db.QueryRowContext(ctx, `
				SELECT column_default FROM information_schema.columns
				WHERE table_schema = 'public' AND table_name = $1 AND column_name = $2
			`, tt.table, tt.column).Scan(&defaultVal)
			require.NoError(t, err, "column %s.%s should exist", tt.table, tt.column)
			require.NotNil(t, defaultVal, "column %s.%s should have a default", tt.table, tt.column)
			assert.Contains(t, *defaultVal, tt.defaultMatch,
				"column %s.%s default mismatch. Expected substring %q, got %q",
				tt.table, tt.column, tt.defaultMatch, *defaultVal)
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// 4f. Verify down migration rollback
// ─────────────────────────────────────────────────────────────────────────────

// TestMigration_RollbackAllAndReapply tests the full cycle: apply all, rollback
// all, verify tables are gone, then re-apply and verify they come back.
func TestMigration_RollbackAllAndReapply(t *testing.T) {
	dbURL := getTestDBURL(t)

	// Step 1: Reset and apply all migrations.
	rollbackAll(t)
	err := postgres.RunMigrations(dbURL, migrationsDir(t))
	require.NoError(t, err)
	version, _, err := postgres.MigrationStatus(dbURL, migrationsDir(t))
	require.NoError(t, err)
	assert.GreaterOrEqual(t, version, uint(7), "should be at version 7+")

	db, cleanup := openTestDB(t)
	defer cleanup()
	ctx := context.Background()

	// Step 2: Rollback all migrations to version 0.
	err = postgres.RollbackMigration(dbURL, migrationsDir(t), 100)
	require.NoError(t, err)

	version, dirty, err := postgres.MigrationStatus(dbURL, migrationsDir(t))
	require.NoError(t, err)
	assert.False(t, dirty)
	assert.Equal(t, uint(0), version, "should be at version 0 after full rollback")

	// Verify key tables are gone.
	keyTables := []string{"patents", "molecules", "portfolios", "users", "workspaces", "patent_claims",
		"molecule_fingerprints", "patent_annuities", "patent_deadlines", "patent_lifecycle_events",
		"roles", "audit_logs", "comments", "notifications", "saved_searches"}
	for _, table := range keyTables {
		assert.False(t, tableExists(ctx, t, db, table),
			"table %s should NOT exist after full rollback", table)
	}

	// Step 3: Re-apply all migrations.
	err = postgres.RunMigrations(dbURL, migrationsDir(t))
	require.NoError(t, err)

	version, dirty, err = postgres.MigrationStatus(dbURL, migrationsDir(t))
	require.NoError(t, err)
	assert.False(t, dirty)
	assert.GreaterOrEqual(t, version, uint(7), "should be at version 7+ after re-apply")

	// Verify tables are back.
	for _, table := range keyTables {
		assert.True(t, tableExists(ctx, t, db, table),
			"table %s should exist after re-apply", table)
	}
}

// TestMigration_StepByStepRollback verifies that rolling back one step at a time
// correctly removes only the tables from the most recent migration.
func TestMigration_StepByStepRollback(t *testing.T) {
	dbURL := getTestDBURL(t)

	rollbackAll(t)
	err := postgres.RunMigrations(dbURL, migrationsDir(t))
	require.NoError(t, err)

	db, cleanup := openTestDB(t)
	defer cleanup()
	ctx := context.Background()

	// Tables created by each migration version.
	migrationTables := map[uint][]string{
		1: {"patents", "patent_claims", "patent_inventors", "patent_priority_claims"},
		2: {"molecules", "molecule_fingerprints", "molecule_properties", "patent_molecule_relations"},
		3: {"portfolios", "portfolio_patents", "patent_valuations", "portfolio_health_scores", "portfolio_optimization_suggestions"},
		4: {"patent_annuities", "patent_deadlines", "patent_lifecycle_events", "patent_cost_records"},
		5: {"users", "organizations", "organization_members", "roles", "user_roles", "api_keys", "audit_logs"},
		6: {"workspaces", "workspace_members", "workspace_projects", "project_patents", "project_molecules", "comments", "notifications", "saved_searches"},
	}
	// Migration 007 has no tables (only ENUM modification).

	// Collect all table names.
	allTables := make([]string, 0)
	for _, tables := range migrationTables {
		allTables = append(allTables, tables...)
	}

	// Tables should exist at version 7 (before any rollback).
	for _, table := range allTables {
		assert.True(t, tableExists(ctx, t, db, table), "table %s should exist at version 7", table)
	}

	// Rollback from version 7 down to version 1, one step at a time.
	for currentVer := uint(6); currentVer >= 1; currentVer-- {
		err = postgres.RollbackMigration(dbURL, migrationsDir(t), 1)
		require.NoError(t, err, "rollback 1 step from version %d", currentVer+1)

		// Tables from currentVer+1 should be gone.
		tablesToDrop := migrationTables[currentVer+1]
		for _, table := range tablesToDrop {
			assert.False(t, tableExists(ctx, t, db, table),
				"table %s should NOT exist after rollback to version %d", table, currentVer)
		}

		// Tables from versions <= currentVer should still exist.
		for v := uint(1); v <= currentVer; v++ {
			for _, table := range migrationTables[v] {
				assert.True(t, tableExists(ctx, t, db, table),
					"table %s should still exist after rollback to version %d", table, currentVer)
			}
		}

		// Verify version.
		ver, dirty, err := postgres.MigrationStatus(dbURL, migrationsDir(t))
		require.NoError(t, err)
		assert.False(t, dirty)
		assert.Equal(t, currentVer, ver, "should be at version %d", currentVer)
	}

	// Final rollback to version 0.
	err = postgres.RollbackMigration(dbURL, migrationsDir(t), 1)
	require.NoError(t, err)

	ver, dirty, err := postgres.MigrationStatus(dbURL, migrationsDir(t))
	require.NoError(t, err)
	assert.False(t, dirty)
	assert.Equal(t, uint(0), ver, "should be at version 0")

	// Nothing should exist.
	for _, table := range allTables {
		assert.False(t, tableExists(ctx, t, db, table),
			"table %s should NOT exist at version 0", table)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TestMigration_CreateTriggers verifies that the updated_at triggers exist.
// ─────────────────────────────────────────────────────────────────────────────

func TestMigration_Triggers(t *testing.T) {
	resetAndApplyMigrations(t)
	db, cleanup := openTestDB(t)
	defer cleanup()

	ctx := context.Background()

	triggerTables := []string{
		"patents", "patent_claims",
		"molecules",
		"portfolios", "portfolio_optimization_suggestions",
		"patent_annuities", "patent_deadlines",
		"users", "organizations", "api_keys",
		"workspaces", "workspace_projects", "comments", "saved_searches",
	}

	for _, table := range triggerTables {
		t.Run(table, func(t *testing.T) {
			var triggerName string
			err := db.QueryRowContext(ctx, `
				SELECT tgname FROM pg_trigger
				JOIN pg_class ON pg_class.oid = pg_trigger.tgrelid
				WHERE pg_class.relname = $1
				AND pg_trigger.tgname = 'set_updated_at'
				AND NOT pg_trigger.tgisinternal
			`, table).Scan(&triggerName)
			require.NoError(t, err, "trigger set_updated_at should exist on %s", table)
			assert.Equal(t, "set_updated_at", triggerName)
		})
	}

	// Verify the trigger function exists.
	var funcExists bool
	err := db.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT FROM pg_proc WHERE proname = 'trigger_set_updated_at'
		)
	`).Scan(&funcExists)
	require.NoError(t, err)
	assert.True(t, funcExists, "trigger_set_updated_at function should exist")
}

// ─────────────────────────────────────────────────────────────────────────────
// TestMigration_CheckConstraints verifies CHECK constraints exist.
// ─────────────────────────────────────────────────────────────────────────────

func TestMigration_CheckConstraints(t *testing.T) {
	resetAndApplyMigrations(t)
	db, cleanup := openTestDB(t)
	defer cleanup()

	ctx := context.Background()

	expectedChecks := []struct {
		constraintName string
		table          string
	}{
		{"patent_claims_claim_type_check", "patent_claims"},
		{"patent_annuities_year_number_check", "patent_annuities"},
		{"patent_valuations_technical_score_check", "patent_valuations"},
		{"patent_valuations_composite_score_check", "patent_valuations"},
		{"portfolio_optimization_suggestions_suggestion_type_check", "portfolio_optimization_suggestions"},
		{"portfolio_optimization_suggestions_priority_check", "portfolio_optimization_suggestions"},
		{"portfolio_optimization_suggestions_status_check", "portfolio_optimization_suggestions"},
	}

	for _, chk := range expectedChecks {
		t.Run(chk.constraintName, func(t *testing.T) {
			var constraintType string
			err := db.QueryRowContext(ctx, `
				SELECT constraint_type FROM information_schema.table_constraints
				WHERE constraint_name = $1 AND table_name = $2 AND table_schema = 'public'
			`, chk.constraintName, chk.table).Scan(&constraintType)
			require.NoError(t, err, "CHECK %s should exist on %s", chk.constraintName, chk.table)
			assert.Equal(t, "CHECK", constraintType)
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TestMigration_Idempotency verifies running migrations twice is safe.
// ─────────────────────────────────────────────────────────────────────────────

func TestMigration_Idempotency(t *testing.T) {
	dbURL := getTestDBURL(t)

	rollbackAll(t)
	err := postgres.RunMigrations(dbURL, migrationsDir(t))
	require.NoError(t, err)

	// Apply a second time -- should be a no-op (no error).
	err = postgres.RunMigrations(dbURL, migrationsDir(t))
	require.NoError(t, err, "second migration run should be safe")

	// Verify version.
	version, dirty, err := postgres.MigrationStatus(dbURL, migrationsDir(t))
	require.NoError(t, err)
	assert.False(t, dirty)
	assert.GreaterOrEqual(t, version, uint(7))
}

// ─────────────────────────────────────────────────────────────────────────────
// Error handling tests
// ─────────────────────────────────────────────────────────────────────────────

func TestMigration_InvalidDBURL(t *testing.T) {
	err := postgres.RunMigrations(
		"postgres://invalid:invalid@127.0.0.1:1/nonexistent?sslmode=disable",
		migrationsDir(t),
	)
	require.Error(t, err)
}

func TestMigration_InvalidMigrationsPath(t *testing.T) {
	dbURL := getTestDBURL(t)

	err := postgres.RunMigrations(dbURL, "file:///nonexistent/migrations")
	require.Error(t, err)
}

func TestMigration_InvalidRollbackSteps(t *testing.T) {
	err := postgres.RollbackMigration("postgres://localhost:5432/test", migrationsDir(t), 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "steps must be greater than 0")
}

// ─────────────────────────────────────────────────────────────────────────────
// End-to-end: full workflow with data
// ─────────────────────────────────────────────────────────────────────────────

// TestMigration_FullEndToEnd tests the complete workflow: apply all migrations,
// insert complex interrelated data, query it, rollback, and re-apply.
func TestMigration_FullEndToEnd(t *testing.T) {
	dbURL := getTestDBURL(t)

	// Phase 1: Reset and apply all migrations.
	t.Log("Phase 1: Applying all migrations...")
	rollbackAll(t)
	err := postgres.RunMigrations(dbURL, migrationsDir(t))
	require.NoError(t, err, "all migrations should apply")

	version, dirty, err := postgres.MigrationStatus(dbURL, migrationsDir(t))
	require.NoError(t, err)
	assert.False(t, dirty)
	assert.GreaterOrEqual(t, version, uint(7))
	t.Logf("  Version: %d", version)

	db, cleanup := openTestDB(t)
	defer cleanup()
	ctx := context.Background()

	// Phase 2: Insert interrelated data across all schema areas.
	t.Log("Phase 2: Inserting interrelated test data...")

	// Create a user.
	_, err = db.ExecContext(ctx, `
		INSERT INTO users (email, username, display_name) VALUES ($1, $2, $3)
	`, "e2e@keyip.example", "e2euser", "E2E User")
	require.NoError(t, err)

	var userID string
	err = db.QueryRowContext(ctx, `SELECT id FROM users WHERE email = $1`, "e2e@keyip.example").Scan(&userID)
	require.NoError(t, err)

	// Create an org and add user.
	_, err = db.ExecContext(ctx, `
		INSERT INTO organizations (name, slug) VALUES ($1, $2)
	`, "E2E Organization", "e2e-org")
	require.NoError(t, err)

	var orgID string
	err = db.QueryRowContext(ctx, `SELECT id FROM organizations WHERE slug = $1`, "e2e-org").Scan(&orgID)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, `
		INSERT INTO organization_members (organization_id, user_id, role) VALUES ($1, $2, 'owner')
	`, orgID, userID)
	require.NoError(t, err)

	// Create a patent.
	_, err = db.ExecContext(ctx, `
		INSERT INTO patents (patent_number, title, patent_type, status, jurisdiction, source, assignee_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, "US-E2E-00001", "E2E Test Patent", "invention", "granted", "US", "manual", &userID)
	require.NoError(t, err)

	var patentID string
	err = db.QueryRowContext(ctx, `SELECT id FROM patents WHERE patent_number = $1`, "US-E2E-00001").Scan(&patentID)
	require.NoError(t, err)

	// Create a molecule.
	_, err = db.ExecContext(ctx, `
		INSERT INTO molecules (smiles, canonical_smiles, inchi_key, status, source)
		VALUES ($1, $2, $3, $4, $5)
	`, "CCO", "CCO", "E2E-INCHIKEY-001", "active", "manual")
	require.NoError(t, err)

	var molID string
	err = db.QueryRowContext(ctx, `SELECT id FROM molecules WHERE inchi_key = $1`, "E2E-INCHIKEY-001").Scan(&molID)
	require.NoError(t, err)

	// Link patent and molecule.
	_, err = db.ExecContext(ctx, `
		INSERT INTO patent_molecule_relations (patent_id, molecule_id, relation_type, extraction_method)
		VALUES ($1, $2, 'claims', 'manual')
	`, patentID, molID)
	require.NoError(t, err)

	// Create a portfolio and link the patent.
	_, err = db.ExecContext(ctx, `
		INSERT INTO portfolios (name, owner_id) VALUES ($1, $2)
	`, "E2E Portfolio", userID)
	require.NoError(t, err)

	var portfolioID string
	err = db.QueryRowContext(ctx, `SELECT id FROM portfolios WHERE name = $1`, "E2E Portfolio").Scan(&portfolioID)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, `
		INSERT INTO portfolio_patents (portfolio_id, patent_id) VALUES ($1, $2)
	`, portfolioID, patentID)
	require.NoError(t, err)

	// Create a valuation.
	_, err = db.ExecContext(ctx, `
		INSERT INTO patent_valuations (patent_id, portfolio_id, technical_score, legal_score,
		                               market_score, strategic_score, composite_score,
		                               tier, valuation_method)
		VALUES ($1, $2, 80, 75, 70, 85, 77.5, 'B', 'automated')
	`, patentID, portfolioID)
	require.NoError(t, err)

	// Add lifecycle data.
	_, err = db.ExecContext(ctx, `
		INSERT INTO patent_annuities (patent_id, year_number, due_date, status)
		VALUES ($1, 1, $2, 'upcoming')
	`, patentID, time.Now().AddDate(1, 0, 0))
	require.NoError(t, err)

	// Create a workspace and project.
	_, err = db.ExecContext(ctx, `
		INSERT INTO workspaces (name, workspace_type, owner_id) VALUES ($1, $2, $3)
	`, "E2E Workspace", "team", userID)
	require.NoError(t, err)

	var wsID string
	err = db.QueryRowContext(ctx, `SELECT id FROM workspaces WHERE name = $1`, "E2E Workspace").Scan(&wsID)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, `
		INSERT INTO workspace_projects (workspace_id, name, project_type, status)
		VALUES ($1, $2, $3, $4)
	`, wsID, "E2E Project", "analysis", "active")
	require.NoError(t, err)

	var projID string
	err = db.QueryRowContext(ctx, `SELECT id FROM workspace_projects WHERE name = $1`, "E2E Project").Scan(&projID)
	require.NoError(t, err)

	t.Log("  All data inserted successfully.")

	// Phase 3: Query everything back and verify.
	t.Log("Phase 3: Querying data back to verify integrity...")

	var rowCount int
	err = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE email = $1`, "e2e@keyip.example").Scan(&rowCount)
	require.NoError(t, err)
	assert.Equal(t, 1, rowCount)

	err = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM organizations WHERE slug = $1`, "e2e-org").Scan(&rowCount)
	require.NoError(t, err)
	assert.Equal(t, 1, rowCount)

	err = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM organization_members WHERE user_id = $1`, userID).Scan(&rowCount)
	require.NoError(t, err)
	assert.Equal(t, 1, rowCount)

	err = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM patents WHERE id = $1`, patentID).Scan(&rowCount)
	require.NoError(t, err)
	assert.Equal(t, 1, rowCount)

	err = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM molecules WHERE id = $1`, molID).Scan(&rowCount)
	require.NoError(t, err)
	assert.Equal(t, 1, rowCount)

	err = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM patent_molecule_relations WHERE patent_id = $1 AND molecule_id = $2`, patentID, molID).Scan(&rowCount)
	require.NoError(t, err)
	assert.Equal(t, 1, rowCount)

	err = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM portfolios WHERE id = $1`, portfolioID).Scan(&rowCount)
	require.NoError(t, err)
	assert.Equal(t, 1, rowCount)

	err = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM portfolio_patents WHERE portfolio_id = $1`, portfolioID).Scan(&rowCount)
	require.NoError(t, err)
	assert.Equal(t, 1, rowCount)

	err = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM patent_valuations WHERE patent_id = $1`, patentID).Scan(&rowCount)
	require.NoError(t, err)
	assert.Equal(t, 1, rowCount)

	err = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM patent_annuities WHERE patent_id = $1`, patentID).Scan(&rowCount)
	require.NoError(t, err)
	assert.Equal(t, 1, rowCount)

	err = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM workspaces WHERE owner_id = $1`, userID).Scan(&rowCount)
	require.NoError(t, err)
	assert.Equal(t, 1, rowCount)

	t.Log("  All queries passed.")

	// Phase 4: Rollback all migrations.
	t.Log("Phase 4: Rolling back all migrations...")
	err = postgres.RollbackMigration(dbURL, migrationsDir(t), 100)
	require.NoError(t, err)

	version, dirty, err = postgres.MigrationStatus(dbURL, migrationsDir(t))
	require.NoError(t, err)
	assert.False(t, dirty)
	assert.Equal(t, uint(0), version)

	// Verify tables are gone.
	for _, table := range []string{"patents", "molecules", "portfolios", "users", "workspaces"} {
		assert.False(t, tableExists(ctx, t, db, table),
			"table %s should be gone", table)
	}
	t.Log("  All tables dropped.")

	// Phase 5: Re-apply all migrations.
	t.Log("Phase 5: Re-applying all migrations...")
	err = postgres.RunMigrations(dbURL, migrationsDir(t))
	require.NoError(t, err)

	version, dirty, err = postgres.MigrationStatus(dbURL, migrationsDir(t))
	require.NoError(t, err)
	assert.False(t, dirty)
	assert.GreaterOrEqual(t, version, uint(7))

	// Verify tables are back.
	for _, table := range []string{"patents", "molecules", "portfolios", "users", "workspaces"} {
		assert.True(t, tableExists(ctx, t, db, table),
			"table %s should be recreated", table)
	}
	t.Log("  All tables recreated.")
}

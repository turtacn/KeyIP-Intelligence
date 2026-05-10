// Package postgres_test provides integration tests for the database migration
// functionality. These tests require a live PostgreSQL instance.
//
//go:build integration

package postgres_test

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/database/postgres"
)

// ─────────────────────────────────────────────────────────────────────────────
// Test environment setup
// ─────────────────────────────────────────────────────────────────────────────

const (
	// testMigrationsPath is the path to test migration files.
	testMigrationsPath = "file://./migrations"
)

func getTestDBURL(t *testing.T) string {
	t.Helper()

	dbURL := os.Getenv("INTEGRATION_TEST_DB_URL")
	if dbURL == "" {
		t.Skip("INTEGRATION_TEST_DB_URL not set; skipping integration test")
	}

	return dbURL
}

// getTestDB opens a direct database/sql connection for schema verification.
func getTestDB(t *testing.T) (*sql.DB, func()) {
	t.Helper()

	dbURL := os.Getenv("INTEGRATION_TEST_DB_URL")
	if dbURL == "" {
		t.Skip("INTEGRATION_TEST_DB_URL not set; skipping integration test")
	}

	db, err := sql.Open("postgres", dbURL)
	require.NoError(t, err, "failed to open test database connection")

	cleanup := func() {
		db.Close()
	}

	return db, cleanup
}

// ─────────────────────────────────────────────────────────────────────────────
// TestRunMigrations — verify migrations can be applied to an empty database
// ─────────────────────────────────────────────────────────────────────────────

func TestRunMigrations_AppliesAllMigrations(t *testing.T) {
	dbURL := getTestDBURL(t)

	// Reset database to ensure clean state.
	err := postgres.ResetDatabase(dbURL, testMigrationsPath)
	require.NoError(t, err)

	// Run migrations.
	err = postgres.RunMigrations(dbURL, testMigrationsPath)
	require.NoError(t, err)

	// Verify migrations were applied by checking version.
	version, dirty, err := postgres.MigrationStatus(dbURL, testMigrationsPath)
	require.NoError(t, err)
	assert.False(t, dirty, "migration state should not be dirty")
	assert.Greater(t, version, uint(0), "version should be greater than 0 after migrations")
}

func TestRunMigrations_NoChangeWhenAlreadyUpToDate(t *testing.T) {
	dbURL := getTestDBURL(t)

	// Apply all migrations first.
	err := postgres.RunMigrations(dbURL, testMigrationsPath)
	require.NoError(t, err)

	// Run migrations again; should return no error (no change).
	err = postgres.RunMigrations(dbURL, testMigrationsPath)
	require.NoError(t, err)
}

// ─────────────────────────────────────────────────────────────────────────────
// TestRollbackMigration — verify migrations can be rolled back
// ─────────────────────────────────────────────────────────────────────────────

func TestRollbackMigration_RollsBackSpecifiedSteps(t *testing.T) {
	dbURL := getTestDBURL(t)

	// Reset and apply all migrations.
	err := postgres.ResetDatabase(dbURL, testMigrationsPath)
	require.NoError(t, err)

	initialVersion, _, err := postgres.MigrationStatus(dbURL, testMigrationsPath)
	require.NoError(t, err)

	// Rollback 1 step.
	err = postgres.RollbackMigration(dbURL, testMigrationsPath, 1)
	require.NoError(t, err)

	// Verify version decreased.
	newVersion, dirty, err := postgres.MigrationStatus(dbURL, testMigrationsPath)
	require.NoError(t, err)
	assert.False(t, dirty)
	assert.Equal(t, initialVersion-1, newVersion)
}

func TestRollbackMigration_FailsWhenStepsIsZero(t *testing.T) {
	dbURL := getTestDBURL(t)

	err := postgres.RollbackMigration(dbURL, testMigrationsPath, 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "steps must be greater than 0")
}

func TestRollbackMigration_FailsWhenNoMigrationsToRollback(t *testing.T) {
	dbURL := getTestDBURL(t)

	// Reset database to version 0.
	err := postgres.ResetDatabase(dbURL, testMigrationsPath)
	require.NoError(t, err)

	// Roll back all migrations.
	err = postgres.RollbackMigration(dbURL, testMigrationsPath, 100) // Attempt to rollback more than exist.
	require.Error(t, err)
}

// ─────────────────────────────────────────────────────────────────────────────
// TestMigrationStatus — verify status reporting
// ─────────────────────────────────────────────────────────────────────────────

func TestMigrationStatus_ReturnsCorrectVersion(t *testing.T) {
	dbURL := getTestDBURL(t)

	// Reset and apply all migrations.
	err := postgres.ResetDatabase(dbURL, testMigrationsPath)
	require.NoError(t, err)

	version, dirty, err := postgres.MigrationStatus(dbURL, testMigrationsPath)
	require.NoError(t, err)
	assert.False(t, dirty)
	assert.Greater(t, version, uint(0))
}

func TestMigrationStatus_ReturnsZeroWhenNoMigrationsApplied(t *testing.T) {
	dbURL := getTestDBURL(t)

	// Roll back all migrations to version 0.
	m, err := migrate.New(testMigrationsPath, dbURL)
	require.NoError(t, err)
	defer m.Close()

	_ = m.Down()

	version, dirty, err := postgres.MigrationStatus(dbURL, testMigrationsPath)
	require.NoError(t, err)
	assert.False(t, dirty)
	assert.Equal(t, uint(0), version)
}

// ─────────────────────────────────────────────────────────────────────────────
// TestResetDatabase — verify complete reset works
// ─────────────────────────────────────────────────────────────────────────────

func TestResetDatabase_DropsAndRecreatesSchema(t *testing.T) {
	dbURL := getTestDBURL(t)

	// Apply migrations first.
	err := postgres.RunMigrations(dbURL, testMigrationsPath)
	require.NoError(t, err)

	// Reset database.
	err = postgres.ResetDatabase(dbURL, testMigrationsPath)
	require.NoError(t, err)

	// Verify migrations were re-applied.
	version, dirty, err := postgres.MigrationStatus(dbURL, testMigrationsPath)
	require.NoError(t, err)
	assert.False(t, dirty)
	assert.Greater(t, version, uint(0))
}

// ─────────────────────────────────────────────────────────────────────────────
// TestForceMigrationVersion — verify version forcing (dangerous operation)
// ─────────────────────────────────────────────────────────────────────────────

func TestForceMigrationVersion_SetsVersionManually(t *testing.T) {
	dbURL := getTestDBURL(t)

	// Reset database.
	err := postgres.ResetDatabase(dbURL, testMigrationsPath)
	require.NoError(t, err)

	// Force version to 1.
	err = postgres.ForceMigrationVersion(dbURL, testMigrationsPath, 1)
	require.NoError(t, err)

	// Verify version is now 1.
	version, dirty, err := postgres.MigrationStatus(dbURL, testMigrationsPath)
	require.NoError(t, err)
	assert.Equal(t, uint(1), version)
	assert.False(t, dirty)
}

// ─────────────────────────────────────────────────────────────────────────────
// Test tables exist after migration
// ─────────────────────────────────────────────────────────────────────────────

func TestRunMigrations_CreatesAllExpectedTables(t *testing.T) {
	dbURL := getTestDBURL(t)

	// Reset and apply all migrations.
	err := postgres.ResetDatabase(dbURL, testMigrationsPath)
	require.NoError(t, err)

	db, cleanup := getTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Complete list of all tables created by migrations 001-007.
	expectedTables := []string{
		// Migration 001
		"patents",
		"patent_claims",
		"patent_inventors",
		"patent_priority_claims",
		// Migration 002
		"molecules",
		"molecule_fingerprints",
		"molecule_properties",
		"patent_molecule_relations",
		// Migration 003
		"portfolios",
		"portfolio_patents",
		"patent_valuations",
		"portfolio_health_scores",
		"portfolio_optimization_suggestions",
		// Migration 004
		"patent_annuities",
		"patent_deadlines",
		"patent_lifecycle_events",
		"patent_cost_records",
		// Migration 005
		"users",
		"organizations",
		"organization_members",
		"roles",
		"user_roles",
		"api_keys",
		"audit_logs",
		// Migration 006
		"workspaces",
		"workspace_members",
		"workspace_projects",
		"project_patents",
		"project_molecules",
		"comments",
		"notifications",
		"saved_searches",
	}

	for _, table := range expectedTables {
		var exists bool
		query := `SELECT EXISTS (
			SELECT FROM information_schema.tables
			WHERE table_schema = 'public'
			AND table_name = $1
		)`
		err := db.QueryRowContext(ctx, query, table).Scan(&exists)
		require.NoError(t, err, "error checking table %s", table)
		assert.True(t, exists, "table %s should exist after migrations", table)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TestColumnTypes — verify column types match domain entity definitions
// ─────────────────────────────────────────────────────────────────────────────

func TestMigration_CorrectColumnTypes(t *testing.T) {
	db, cleanup := getTestDB(t)
	defer cleanup()

	ctx := context.Background()

	tests := []struct {
		table    string
		column   string
		dataType string
		notNull  bool
	}{
		// patents table
		{"patents", "id", "uuid", true},
		{"patents", "patent_number", "character varying", true},
		{"patents", "title", "text", true},
		{"patents", "patent_type", "patent_type", true},     // custom enum
		{"patents", "status", "patent_status", true},         // custom enum
		{"patents", "jurisdiction", "character varying", true},
		{"patents", "ipc_codes", "ARRAY", false},
		{"patents", "raw_data", "jsonb", false},
		{"patents", "metadata", "jsonb", false},
		{"patents", "created_at", "timestamp with time zone", true},
		{"patents", "updated_at", "timestamp with time zone", true},
		{"patents", "deleted_at", "timestamp with time zone", false},

		// patent_claims table
		{"patent_claims", "id", "uuid", true},
		{"patent_claims", "patent_id", "uuid", true},
		{"patent_claims", "claim_number", "integer", true},
		{"patent_claims", "claim_text", "text", true},
		{"patent_claims", "scope_embedding", "USER-DEFINED", false}, // vector type

		// molecules table
		{"molecules", "id", "uuid", true},
		{"molecules", "smiles", "text", true},
		{"molecules", "canonical_smiles", "text", true},
		{"molecules", "inchi_key", "character varying", false},
		{"molecules", "molecular_weight", "double precision", false},
		{"molecules", "status", "molecule_status", true},

		// molecule_fingerprints
		{"molecule_fingerprints", "fingerprint_vector", "USER-DEFINED", false}, // vector type

		// portfolios table
		{"portfolios", "id", "uuid", true},
		{"portfolios", "name", "character varying", true},
		{"portfolios", "owner_id", "uuid", true},
		{"portfolios", "status", "portfolio_status", true},

		// patent_valuations table
		{"patent_valuations", "composite_score", "double precision", true},
		{"patent_valuations", "tier", "valuation_tier", true},

		// lifecycle tables
		{"patent_annuities", "status", "annuity_status", true},
		{"patent_deadlines", "status", "deadline_status", true},
		{"patent_lifecycle_events", "event_type", "lifecycle_event_type", true},

		// users table
		{"users", "id", "uuid", true},
		{"users", "email", "character varying", true},
		{"users", "password_hash", "character varying", false},
		{"users", "status", "user_status", true},
		{"users", "mfa_enabled", "boolean", true},

		// organization_members
		{"organization_members", "organization_id", "uuid", true},
		{"organization_members", "user_id", "uuid", true},

		// workspaces table
		{"workspaces", "id", "uuid", true},
		{"workspaces", "owner_id", "uuid", true},
		{"workspaces", "workspace_type", "workspace_type", true},
		{"workspaces", "is_archived", "boolean", true},

		// comments table
		{"comments", "author_id", "uuid", true},
		{"comments", "content", "text", true},
		{"comments", "is_resolved", "boolean", true},

		// notifications table
		{"notifications", "notification_type", "notification_type", true},

		// saved_searches
		{"saved_searches", "search_type", "character varying", true},
	}

	for _, tt := range tests {
		t.Run(tt.table+"."+tt.column, func(t *testing.T) {
			var dataType string
			var isNullable string
			query := `
				SELECT data_type, is_nullable
				FROM information_schema.columns
				WHERE table_schema = 'public'
				AND table_name = $1
				AND column_name = $2
			`
			err := db.QueryRowContext(ctx, query, tt.table, tt.column).Scan(&dataType, &isNullable)
			if !assert.NoError(t, err, "column %s.%s should exist", tt.table, tt.column) {
				return
			}
			assert.Equal(t, tt.dataType, dataType,
				"column %s.%s type mismatch: expected %s, got %s",
				tt.table, tt.column, tt.dataType, dataType)

			if tt.notNull {
				assert.Equal(t, "NO", isNullable,
					"column %s.%s should have NOT NULL constraint", tt.table, tt.column)
			} else {
				assert.Equal(t, "YES", isNullable,
					"column %s.%s should allow NULL", tt.table, tt.column)
			}
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TestDefaultValues — verify default values are set correctly
// ─────────────────────────────────────────────────────────────────────────────

func TestMigration_DefaultValues(t *testing.T) {
	db, cleanup := getTestDB(t)
	defer cleanup()

	ctx := context.Background()

	tests := []struct {
		table        string
		column       string
		defaultMatch string // substring to match in column_default
	}{
		{"patents", "patent_type", "'invention'::patent_type"},
		{"patents", "status", "'draft'::patent_status"},
		{"patents", "source", "'manual'::character varying"},
		{"patents", "created_at", "now()"},
		{"patents", "updated_at", "now()"},
		{"molecules", "status", "'active'::molecule_status"},
		{"molecules", "created_at", "now()"},
		{"portfolios", "status", "'active'::portfolio_status"},
		{"portfolios", "created_at", "now()"},
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
		{"patent_valuations", "currency", "'USD'"},
		{"patent_valuations", "scoring_details", "'{}'::jsonb"},
		{"portfolio_health_scores", "jurisdiction_distribution", "'{}'::jsonb"},
		{"comments", "is_resolved", "false"},
	}

	for _, tt := range tests {
		t.Run(tt.table+"."+tt.column, func(t *testing.T) {
			var defaultVal *string
			query := `
				SELECT column_default
				FROM information_schema.columns
				WHERE table_schema = 'public'
				AND table_name = $1
				AND column_name = $2
			`
			err := db.QueryRowContext(ctx, query, tt.table, tt.column).Scan(&defaultVal)
			require.NoError(t, err, "column %s.%s should exist", tt.table, tt.column)
			require.NotNil(t, defaultVal, "column %s.%s should have a default", tt.table, tt.column)
			assert.Contains(t, *defaultVal, tt.defaultMatch,
				"column %s.%s default should contain %q, got %q",
				tt.table, tt.column, tt.defaultMatch, *defaultVal)
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TestForeignKeyConstraints — verify FK relationships
// ─────────────────────────────────────────────────────────────────────────────

func TestMigration_ForeignKeys(t *testing.T) {
	db, cleanup := getTestDB(t)
	defer cleanup()

	ctx := context.Background()

	tests := []struct {
		constraintName string
		table          string
		column         string
		refTable       string
		refColumn      string
		deleteAction   string
	}{
		// Migration 001 - patent_claims references patents
		{"patent_claims_patent_id_fkey", "patent_claims", "patent_id", "patents", "id", "CASCADE"},
		// Migration 001 - patent_inventors references patents
		{"patent_inventors_patent_id_fkey", "patent_inventors", "patent_id", "patents", "id", "CASCADE"},
		// Migration 001 - patent_priority_claims references patents
		{"patent_priority_claims_patent_id_fkey", "patent_priority_claims", "patent_id", "patents", "id", "CASCADE"},
		// Migration 002 - molecule_fingerprints references molecules
		{"molecule_fingerprints_molecule_id_fkey", "molecule_fingerprints", "molecule_id", "molecules", "id", "CASCADE"},
		// Migration 002 - molecule_properties references molecules
		{"molecule_properties_molecule_id_fkey", "molecule_properties", "molecule_id", "molecules", "id", "CASCADE"},
		// Migration 002 - patent_molecule_relations references patents
		{"patent_molecule_relations_patent_id_fkey", "patent_molecule_relations", "patent_id", "patents", "id", "CASCADE"},
		// Migration 002 - patent_molecule_relations references molecules
		{"patent_molecule_relations_molecule_id_fkey", "patent_molecule_relations", "molecule_id", "molecules", "id", "CASCADE"},
		// Migration 003 - portfolio_patents references portfolios
		{"portfolio_patents_portfolio_id_fkey", "portfolio_patents", "portfolio_id", "portfolios", "id", "CASCADE"},
		// Migration 003 - portfolio_patents references patents
		{"portfolio_patents_patent_id_fkey", "portfolio_patents", "patent_id", "patents", "id", "CASCADE"},
		// Migration 003 - patent_valuations references patents
		{"patent_valuations_patent_id_fkey", "patent_valuations", "patent_id", "patents", "id", "CASCADE"},
		// Migration 003 - portfolio_health_scores references portfolios
		{"portfolio_health_scores_portfolio_id_fkey", "portfolio_health_scores", "portfolio_id", "portfolios", "id", "CASCADE"},
		// Migration 004 - patent_annuities references patents
		{"patent_annuities_patent_id_fkey", "patent_annuities", "patent_id", "patents", "id", "CASCADE"},
		// Migration 004 - patent_deadlines references patents
		{"patent_deadlines_patent_id_fkey", "patent_deadlines", "patent_id", "patents", "id", "CASCADE"},
		// Migration 004 - patent_lifecycle_events references patents
		{"patent_lifecycle_events_patent_id_fkey", "patent_lifecycle_events", "patent_id", "patents", "id", "CASCADE"},
		// Migration 005 - organization_members references organizations
		{"organization_members_organization_id_fkey", "organization_members", "organization_id", "organizations", "id", "CASCADE"},
		// Migration 005 - organization_members references users
		{"organization_members_user_id_fkey", "organization_members", "user_id", "users", "id", "CASCADE"},
		// Migration 005 - user_roles references users
		{"user_roles_user_id_fkey", "user_roles", "user_id", "users", "id", "CASCADE"},
		// Migration 005 - user_roles references roles
		{"user_roles_role_id_fkey", "user_roles", "role_id", "roles", "id", "CASCADE"},
		// Migration 005 - api_keys references users
		{"api_keys_user_id_fkey", "api_keys", "user_id", "users", "id", "CASCADE"},
		// Migration 005 - ALTER TABLE fk_patents_assignee
		{"fk_patents_assignee", "patents", "assignee_id", "users", "id", "SET NULL"},
		// Migration 005 - ALTER TABLE fk_portfolios_owner
		{"fk_portfolios_owner", "portfolios", "owner_id", "users", "id", "RESTRICT"},
		// Migration 006 - workspaces references users
		{"workspaces_owner_id_fkey", "workspaces", "owner_id", "users", "id", "RESTRICT"},
		// Migration 006 - workspace_members references workspaces
		{"workspace_members_workspace_id_fkey", "workspace_members", "workspace_id", "workspaces", "id", "CASCADE"},
		// Migration 006 - workspace_members references users
		{"workspace_members_user_id_fkey", "workspace_members", "user_id", "users", "id", "CASCADE"},
		// Migration 006 - comments references users (author_id)
		{"comments_author_id_fkey", "comments", "author_id", "users", "id", "CASCADE"},
		// Migration 006 - notifications references users
		{"notifications_user_id_fkey", "notifications", "user_id", "users", "id", "CASCADE"},
		// Migration 006 - saved_searches references users
		{"saved_searches_user_id_fkey", "saved_searches", "user_id", "users", "id", "CASCADE"},
	}

	for _, tt := range tests {
		t.Run(tt.constraintName, func(t *testing.T) {
			var refTableName string
			var refColumnName string
			var deleteRule string

			query := `
				SELECT
					confrelid::regclass::text AS referenced_table,
					a.attname AS referenced_column,
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
				WHERE c.conname = $1
				AND c.contype = 'f'
			`
			err := db.QueryRowContext(ctx, query, tt.constraintName).Scan(&refTableName, &refColumnName, &deleteRule)
			if !assert.NoError(t, err, "foreign key %s should exist", tt.constraintName) {
				return
			}

			assert.Equal(t, tt.refTable, refTableName,
				"FK %s should reference table %s, got %s", tt.constraintName, tt.refTable, refTableName)
			assert.Equal(t, tt.refColumn, refColumnName,
				"FK %s should reference column %s, got %s", tt.constraintName, tt.refColumn, refColumnName)
			assert.Equal(t, tt.deleteAction, deleteRule,
				"FK %s delete rule mismatch: expected %s, got %s", tt.constraintName, tt.deleteAction, deleteRule)
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TestEnumValues — verify ENUM types have expected values
// ─────────────────────────────────────────────────────────────────────────────

func TestMigration_EnumValues(t *testing.T) {
	db, cleanup := getTestDB(t)
	defer cleanup()

	ctx := context.Background()

	tests := []struct {
		enumName   string
		enumValues []string
	}{
		{
			"patent_status",
			[]string{"draft", "filed", "published", "under_examination", "granted", "rejected", "withdrawn", "expired", "lapsed", "invalidated"},
		},
		{
			"patent_type",
			[]string{"invention", "utility_model", "design", "plant", "provisional"},
		},
		{
			"molecule_status",
			[]string{"pending", "active", "archived", "deleted", "pending_review"},
		},
		{
			"portfolio_status",
			[]string{"active", "archived", "draft"},
		},
		{
			"valuation_tier",
			[]string{"S", "A", "B", "C", "D"},
		},
		{
			"annuity_status",
			[]string{"upcoming", "due", "overdue", "paid", "grace_period", "waived", "abandoned"},
		},
		{
			"lifecycle_event_type",
			[]string{"filing", "publication", "examination_request", "office_action", "response_filed", "grant", "annuity_payment", "annuity_missed", "renewal", "assignment", "license", "opposition", "invalidation", "expiry", "restoration", "abandonment", "status_change", "custom"},
		},
		{
			"deadline_status",
			[]string{"active", "completed", "missed", "extended", "waived"},
		},
		{
			"user_status",
			[]string{"active", "inactive", "suspended", "pending_verification"},
		},
		{
			"workspace_type",
			[]string{"personal", "team", "project"},
		},
		{
			"notification_type",
			[]string{"deadline_reminder", "annuity_due", "task_assigned", "comment_mention", "analysis_complete", "portfolio_alert", "system_announcement", "invitation"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.enumName, func(t *testing.T) {
			rows, err := db.QueryContext(ctx, `
				SELECT enumlabel
				FROM pg_enum
				WHERE enumtypid = $1::regtype
				ORDER BY enumsortorder
			`, tt.enumName)
			require.NoError(t, err, "enum type %s should exist", tt.enumName)
			defer rows.Close()

			var actualValues []string
			for rows.Next() {
				var val string
				require.NoError(t, rows.Scan(&val))
				actualValues = append(actualValues, val)
			}
			require.NoError(t, rows.Err())

			assert.Equal(t, tt.enumValues, actualValues,
				"enum %s values mismatch", tt.enumName)
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TestIndexes — verify indexes are created
// ─────────────────────────────────────────────────────────────────────────────

func TestMigration_Indexes(t *testing.T) {
	db, cleanup := getTestDB(t)
	defer cleanup()

	ctx := context.Background()

	expectedIndexes := []struct {
		indexName string
		tableName string
		unique    bool
		isPartial bool
	}{
		{indexName: "idx_patents_patent_number", tableName: "patents"},
		{indexName: "idx_patents_status", tableName: "patents"},
		{indexName: "idx_patents_jurisdiction", tableName: "patents"},
		{indexName: "idx_patents_filing_date", tableName: "patents"},
		{indexName: "idx_patents_deleted_at", tableName: "patents", isPartial: true},
		{indexName: "idx_patent_claims_patent_id", tableName: "patent_claims"},
		{indexName: "idx_molecules_inchi_key", tableName: "molecules"},
		{indexName: "idx_molecules_deleted_at", tableName: "molecules", isPartial: true},
		{indexName: "idx_molecule_props_molecule_id", tableName: "molecule_properties"},
		{indexName: "idx_patent_mol_rel_patent_id", tableName: "patent_molecule_relations"},
		{indexName: "idx_patent_mol_rel_molecule_id", tableName: "patent_molecule_relations"},
		{indexName: "idx_portfolios_owner_id", tableName: "portfolios"},
		{indexName: "idx_portfolios_deleted_at", tableName: "portfolios", isPartial: true},
		{indexName: "idx_patent_valuations_patent_id", tableName: "patent_valuations"},
		{indexName: "idx_patent_valuations_composite_score", tableName: "patent_valuations"},
		{indexName: "idx_portfolio_health_portfolio_id", tableName: "portfolio_health_scores"},
		{indexName: "idx_annuities_patent_id", tableName: "patent_annuities"},
		{indexName: "idx_annuities_due_date", tableName: "patent_annuities"},
		{indexName: "idx_deadlines_patent_id", tableName: "patent_deadlines"},
		{indexName: "idx_deadlines_active_due", tableName: "patent_deadlines", isPartial: true},
		{indexName: "idx_lifecycle_events_patent_id", tableName: "patent_lifecycle_events"},
		{indexName: "idx_cost_records_patent_id", tableName: "patent_cost_records"},
		{indexName: "idx_users_email", tableName: "users"},
		{indexName: "idx_users_status", tableName: "users"},
		{indexName: "idx_users_deleted_at", tableName: "users", isPartial: true},
		{indexName: "idx_orgs_slug", tableName: "organizations"},
		{indexName: "idx_org_members_user_id", tableName: "organization_members"},
		{indexName: "idx_user_roles_global", tableName: "user_roles", unique: true, isPartial: true},
		{indexName: "idx_user_roles_org", tableName: "user_roles", unique: true, isPartial: true},
		{indexName: "idx_api_keys_user_id", tableName: "api_keys"},
		{indexName: "idx_api_keys_key_hash", tableName: "api_keys"},
		{indexName: "idx_audit_logs_user_id", tableName: "audit_logs"},
		{indexName: "idx_audit_logs_created_at", tableName: "audit_logs"},
		{indexName: "idx_workspaces_org_id", tableName: "workspaces"},
		{indexName: "idx_workspaces_owner_id", tableName: "workspaces"},
		{indexName: "idx_workspaces_deleted_at", tableName: "workspaces", isPartial: true},
		{indexName: "idx_ws_members_user_id", tableName: "workspace_members"},
		{indexName: "idx_ws_projects_workspace_id", tableName: "workspace_projects"},
		{indexName: "idx_project_patents_patent_id", tableName: "project_patents"},
		{indexName: "idx_project_molecules_molecule_id", tableName: "project_molecules"},
		{indexName: "idx_comments_resource", tableName: "comments"},
		{indexName: "idx_comments_deleted_at", tableName: "comments", isPartial: true},
		{indexName: "idx_notifications_user_id", tableName: "notifications"},
		{indexName: "idx_notifications_created_at", tableName: "notifications"},
		{indexName: "idx_saved_searches_user_id", tableName: "saved_searches"},
		{indexName: "idx_saved_searches_alert", tableName: "saved_searches", isPartial: true},
	}

	for _, idx := range expectedIndexes {
		t.Run(idx.indexName, func(t *testing.T) {
			var indexDef string
			err := db.QueryRowContext(ctx, `
				SELECT indexdef
				FROM pg_indexes
				WHERE indexname = $1
				AND tablename = $2
				AND schemaname = 'public'
			`, idx.indexName, idx.tableName).Scan(&indexDef)
			require.NoError(t, err, "index %s should exist on table %s", idx.indexName, idx.tableName)

			// Check uniqueness via pg_index.
			if idx.unique {
				var isUnique bool
				err := db.QueryRowContext(ctx, `
					SELECT i.indisunique
					FROM pg_index i
					JOIN pg_class c ON c.oid = i.indexrelid
					WHERE c.relname = $1
				`, idx.indexName).Scan(&isUnique)
				require.NoError(t, err)
				assert.True(t, isUnique, "index %s should be unique", idx.indexName)
			}

			// Check if partial index has WHERE clause.
			if idx.isPartial {
				assert.Contains(t, indexDef, "WHERE",
					"index %s should be a partial index with WHERE clause", idx.indexName)
			}
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TestDownMigration — verify down migrations drop all tables
// ─────────────────────────────────────────────────────────────────────────────

func TestDownMigration_DropsAllTables(t *testing.T) {
	dbURL := getTestDBURL(t)

	// Apply all migrations.
	err := postgres.RunMigrations(dbURL, testMigrationsPath)
	require.NoError(t, err)

	// Roll back all migrations.
	m, err := migrate.New(testMigrationsPath, dbURL)
	require.NoError(t, err)
	defer m.Close()

	err = m.Down()
	require.NoError(t, err)

	// Verify migration version is 0 after full rollback.
	version, dirty, err := postgres.MigrationStatus(dbURL, testMigrationsPath)
	require.NoError(t, err)
	assert.False(t, dirty)
	assert.Equal(t, uint(0), version, "migration version should be 0 after full rollback")

	// Verify that key tables no longer exist.
	db, cleanup := getTestDB(t)
	defer cleanup()

	ctx := context.Background()
	keyTables := []string{"patents", "molecules", "portfolios", "users", "workspaces"}
	for _, table := range keyTables {
		var exists bool
		err := db.QueryRowContext(ctx, `
			SELECT EXISTS (
				SELECT FROM information_schema.tables
				WHERE table_schema = 'public' AND table_name = $1
			)
		`, table).Scan(&exists)
		require.NoError(t, err)
		assert.False(t, exists, "table %s should not exist after full rollback", table)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TestIdempotentUp — running migrate up twice is safe
// ─────────────────────────────────────────────────────────────────────────────

func TestIdempotentUp_RunningTwiceIsSafe(t *testing.T) {
	dbURL := getTestDBURL(t)

	// Full reset.
	err := postgres.ResetDatabase(dbURL, testMigrationsPath)
	require.NoError(t, err)

	// First application.
	err = postgres.RunMigrations(dbURL, testMigrationsPath)
	require.NoError(t, err)

	// Second application should report no change.
	err = postgres.RunMigrations(dbURL, testMigrationsPath)
	require.NoError(t, err)

	// Version should be at the latest.
	version, dirty, err := postgres.MigrationStatus(dbURL, testMigrationsPath)
	require.NoError(t, err)
	assert.False(t, dirty)
	assert.GreaterOrEqual(t, version, uint(7), "all 7 migrations should be applied")
}

// ─────────────────────────────────────────────────────────────────────────────
// TestFullCycle — apply, rollback, re-apply
// ─────────────────────────────────────────────────────────────────────────────

func TestFullCycle_ApplyRollbackReapply(t *testing.T) {
	dbURL := getTestDBURL(t)

	// Reset.
	err := postgres.ResetDatabase(dbURL, testMigrationsPath)
	require.NoError(t, err)

	// Apply.
	err = postgres.RunMigrations(dbURL, testMigrationsPath)
	require.NoError(t, err)

	version1, _, err := postgres.MigrationStatus(dbURL, testMigrationsPath)
	require.NoError(t, err)

	// Rollback all.
	m, err := migrate.New(testMigrationsPath, dbURL)
	require.NoError(t, err)
	defer m.Close()

	err = m.Down()
	require.NoError(t, err)

	version0, _, err := postgres.MigrationStatus(dbURL, testMigrationsPath)
	require.NoError(t, err)
	assert.Equal(t, uint(0), version0)

	// Re-apply.
	err = postgres.RunMigrations(dbURL, testMigrationsPath)
	require.NoError(t, err)

	version2, dirty, err := postgres.MigrationStatus(dbURL, testMigrationsPath)
	require.NoError(t, err)
	assert.False(t, dirty)
	assert.Equal(t, version1, version2, "version should match after re-apply")
}

// ─────────────────────────────────────────────────────────────────────────────
// TestRoleSeedData — verify default roles are seeded
// ─────────────────────────────────────────────────────────────────────────────

func TestMigration_SeedsDefaultRoles(t *testing.T) {
	db, cleanup := getTestDB(t)
	defer cleanup()

	ctx := context.Background()

	var roleCount int
	err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM roles`).Scan(&roleCount)
	require.NoError(t, err)
	assert.Equal(t, 5, roleCount, "should have exactly 5 default roles")

	// Verify specific roles exist.
	expectedRoles := []string{"super_admin", "org_admin", "patent_analyst", "researcher", "viewer"}
	for _, role := range expectedRoles {
		var exists bool
		err := db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM roles WHERE name = $1)`, role).Scan(&exists)
		require.NoError(t, err)
		assert.True(t, exists, "role %s should exist", role)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TestCheckConstraints — verify CHECK constraints exist on key columns
// ─────────────────────────────────────────────────────────────────────────────

func TestMigration_CheckConstraintsExist(t *testing.T) {
	db, cleanup := getTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Count check constraints on key tables that should have them.
	expectedChecks := []struct {
		constraintName string
		table          string
	}{
		{"patent_claims_claim_type_check", "patent_claims"},
		{"patent_annuities_year_number_check", "patent_annuities"},
		{"patent_valuations_technical_score_check", "patent_valuations"},
		{"patent_valuations_legal_score_check", "patent_valuations"},
		{"patent_valuations_market_score_check", "patent_valuations"},
		{"patent_valuations_strategic_score_check", "patent_valuations"},
		{"patent_valuations_composite_score_check", "patent_valuations"},
		{"portfolio_optimization_suggestions_suggestion_type_check", "portfolio_optimization_suggestions"},
		{"portfolio_optimization_suggestions_priority_check", "portfolio_optimization_suggestions"},
		{"portfolio_optimization_suggestions_status_check", "portfolio_optimization_suggestions"},
	}

	for _, chk := range expectedChecks {
		t.Run(chk.constraintName, func(t *testing.T) {
			var constraintType string
			err := db.QueryRowContext(ctx, `
				SELECT constraint_type
				FROM information_schema.table_constraints
				WHERE constraint_name = $1
				AND table_name = $2
				AND table_schema = 'public'
			`, chk.constraintName, chk.table).Scan(&constraintType)
			if !assert.NoError(t, err, "CHECK constraint %s should exist on table %s", chk.constraintName, chk.table) {
				return
			}
			assert.Equal(t, "CHECK", constraintType)
		})
	}
}


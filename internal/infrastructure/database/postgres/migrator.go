// Package postgres provides database migration management using golang-migrate.
// All migrations are executed automatically on application startup, and can be
// controlled via CLI commands for advanced scenarios (rollback, status checks).
package postgres

import (
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres" // Postgres driver
	_ "github.com/golang-migrate/migrate/v4/source/file"       // File source driver
)

// ─────────────────────────────────────────────────────────────────────────────
// RunMigrations — apply all pending migrations
// ─────────────────────────────────────────────────────────────────────────────

// RunMigrations executes all pending database migrations from the specified
// migrations directory. This is typically called during application startup
// to ensure the database schema is up-to-date.
//
// If no migrations are pending, the function returns nil.
//
// Args:
//   - dbURL: PostgreSQL connection string (e.g., "postgres://user:pass@host:port/db?sslmode=disable")
//   - migrationsPath: Path to the directory containing migration files (e.g., "file://migrations")
//
// Returns:
//   - error: nil if all migrations succeed, or a descriptive error otherwise.
func RunMigrations(dbURL string, migrationsPath string) error {
	m, err := migrate.New(migrationsPath, dbURL)
	if err != nil {
		return fmt.Errorf("failed to create migrate instance: %w", err)
	}
	defer m.Close()

	// Apply all pending migrations (Up).
	if err := m.Up(); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			// No migrations to apply; this is not an error.
			return nil
		}
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// RollbackMigration — rollback migrations by specified steps
// ─────────────────────────────────────────────────────────────────────────────

// RollbackMigration rolls back the database schema by the specified number of
// migration steps. This is primarily used in development and testing to quickly
// revert schema changes.
//
// Args:
//   - dbURL: PostgreSQL connection string
//   - migrationsPath: Path to the migrations directory
//   - steps: Number of migrations to roll back (must be > 0)
//
// Returns:
//   - error: nil if rollback succeeds, or a descriptive error otherwise.
func RollbackMigration(dbURL string, migrationsPath string, steps int) error {
	if steps <= 0 {
		return fmt.Errorf("steps must be greater than 0, got %d", steps)
	}

	m, err := migrate.New(migrationsPath, dbURL)
	if err != nil {
		return fmt.Errorf("failed to create migrate instance: %w", err)
	}
	defer m.Close()

	// Rollback by the specified number of steps.
	if err := m.Steps(-steps); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			return fmt.Errorf("no migrations to roll back")
		}
		return fmt.Errorf("failed to rollback %d step(s): %w", steps, err)
	}

	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// MigrationStatus — query current migration state
// ─────────────────────────────────────────────────────────────────────────────

// MigrationStatus returns the current migration version and dirty state of the
// database. A "dirty" state indicates that a previous migration failed and left
// the schema in an inconsistent state.
//
// Args:
//   - dbURL: PostgreSQL connection string
//   - migrationsPath: Path to the migrations directory
//
// Returns:
//   - version: The currently applied migration version (0 if no migrations applied)
//   - dirty: true if the migration state is dirty (requires manual intervention)
//   - err: nil if the status was successfully retrieved, or a descriptive error otherwise.
func MigrationStatus(dbURL string, migrationsPath string) (version uint, dirty bool, err error) {
	m, err := migrate.New(migrationsPath, dbURL)
	if err != nil {
		return 0, false, fmt.Errorf("failed to create migrate instance: %w", err)
	}
	defer m.Close()

	version, dirty, err = m.Version()
	if err != nil {
		if errors.Is(err, migrate.ErrNilVersion) {
			// No migrations have been applied yet.
			return 0, false, nil
		}
		return 0, false, fmt.Errorf("failed to get migration version: %w", err)
	}

	return version, dirty, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// ResetDatabase — rollback all migrations and re-apply them
// ─────────────────────────────────────────────────────────────────────────────

// ResetDatabase completely resets the database by rolling back all migrations
// and then re-applying them from scratch. This is intended for development and
// testing environments only.
//
// ⚠️  WARNING: This function is destructive and will DROP ALL TABLES.
// DO NOT use in production environments.
//
// Args:
//   - dbURL: PostgreSQL connection string
//   - migrationsPath: Path to the migrations directory
//
// Returns:
//   - error: nil if the reset succeeds, or a descriptive error otherwise.
func ResetDatabase(dbURL string, migrationsPath string) error {
	m, err := migrate.New(migrationsPath, dbURL)
	if err != nil {
		return fmt.Errorf("failed to create migrate instance: %w", err)
	}
	defer m.Close()

	// Rollback all migrations to version 0 (Down).
	if err := m.Down(); err != nil {
		if !errors.Is(err, migrate.ErrNoChange) {
			return fmt.Errorf("failed to roll back all migrations: %w", err)
		}
	}

	// Re-apply all migrations (Up).
	if err := m.Up(); err != nil {
		if !errors.Is(err, migrate.ErrNoChange) {
			return fmt.Errorf("failed to re-apply migrations: %w", err)
		}
	}

	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// ForceMigrationVersion — manually set migration version (dangerous)
// ─────────────────────────────────────────────────────────────────────────────

// ForceMigrationVersion forcibly sets the database schema version to the
// specified value without running any migrations. This is used to recover from
// a "dirty" migration state where a migration partially failed.
//
// ⚠️  WARNING: This function bypasses the normal migration process and can leave
// the schema in an inconsistent state if used incorrectly. Only use this when
// manually fixing a failed migration.
//
// Args:
//   - dbURL: PostgreSQL connection string
//   - migrationsPath: Path to the migrations directory
//   - version: The version to force (use -1 to mark as "no version")
//
// Returns:
//   - error: nil if the version was successfully forced, or a descriptive error otherwise.
func ForceMigrationVersion(dbURL string, migrationsPath string, version int) error {
	m, err := migrate.New(migrationsPath, dbURL)
	if err != nil {
		return fmt.Errorf("failed to create migrate instance: %w", err)
	}
	defer m.Close()

	if err := m.Force(version); err != nil {
		return fmt.Errorf("failed to force version %d: %w", version, err)
	}

	return nil
}

//Personal.AI order the ending

// Package clitest provides offline validation tests for database migration
// files and seed data consistency. These tests do NOT require a live
// PostgreSQL connection and can run with `go test -short`.
//
//go:build !integration

package clitest

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// ─────────────────────────────────────────────────────────────────────────────
// Paths
// ─────────────────────────────────────────────────────────────────────────────

var (
	projectRoot   = findProjectRoot()
	migrationsDir = filepath.Join(projectRoot, "internal", "infrastructure", "database", "postgres", "migrations")
	seedScript    = filepath.Join(projectRoot, "scripts", "seed.sh")
)

func findProjectRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			panic("project root (go.mod) not found")
		}
		dir = parent
	}
}

// migrationFile holds the parsed contents of a single migration file.
type migrationFile struct {
	path        string
	version     int // extracted numeric version 1-7
	filename    string // e.g. "001_create_patents.sql"
	upContent   string
	downContent string
	upTables    []string // tables CREATEd in Up section
	downTables  []string // tables DROPped in Down section
}

// ─────────────────────────────────────────────────────────────────────────────
// TestMigrationsSyntax — verify each migration file has valid structure
// ─────────────────────────────────────────────────────────────────────────────

func TestMigrationsSyntax(t *testing.T) {
	files, err := filepath.Glob(filepath.Join(migrationsDir, "*.sql"))
	if err != nil {
		t.Fatalf("failed to list migration files: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("no migration files found")
	}

	for _, f := range files {
		name := filepath.Base(f)
		t.Run(name, func(t *testing.T) {
			mf, errs := parseAndValidateMigration(f)
			for _, e := range errs {
				t.Error(e)
			}
			if t.Failed() {
				return
			}
			// Verify up section is non-empty.
			if len(strings.TrimSpace(mf.upContent)) == 0 {
				t.Error("Up migration is empty")
			}
			// Verify down section is non-empty (or at least has a SELECT 1).
			if len(strings.TrimSpace(mf.downContent)) == 0 {
				t.Error("Down migration is empty")
			}
			// Verify every statement in Up ends with semicolon (basic syntax check).
			statements := splitSQLStatements(mf.upContent)
			for i, stmt := range statements {
				stmt = strings.TrimSpace(stmt)
				if stmt == "" {
					continue
				}
				if !strings.HasSuffix(stmt, ";") {
					t.Errorf("Up statement %d (starting %q) does not end with semicolon", i+1, safePrefix(stmt, 40))
				}
			}
			// Verify every statement in Down ends with semicolon.
			downStatements := splitSQLStatements(mf.downContent)
			for i, stmt := range downStatements {
				stmt = strings.TrimSpace(stmt)
				if stmt == "" {
					continue
				}
				if !strings.HasSuffix(stmt, ";") {
					t.Errorf("Down statement %d (starting %q) does not end with semicolon", i+1, safePrefix(stmt, 40))
				}
			}
		})
	}
}

// parseAndValidateMigration reads a migration file and checks basic structural
// requirements: marker comments, balanced sections, and table references.
func parseAndValidateMigration(path string) (*migrationFile, []string) {
	var errs []string
	addErr := func(f string, a ...interface{}) {
		errs = append(errs, fmt.Sprintf(f, a...))
	}

	content, err := os.ReadFile(path)
	if err != nil {
		addErr("failed to read file: %v", err)
		return nil, errs
	}

	text := string(content)

	// Check for golang-migrate markers.
	if !strings.Contains(text, "-- +migrate Up") {
		addErr("missing '-- +migrate Up' marker")
	}
	if !strings.Contains(text, "-- +migrate Down") {
		addErr("missing '-- +migrate Down' marker")
	}

	// Extract version from filename (e.g. "001_create_patents.sql").
	base := filepath.Base(path)
	re := regexp.MustCompile(`^(\d{3})_`)
	matches := re.FindStringSubmatch(base)
	if matches == nil {
		addErr("filename %q does not start with three-digit version number (e.g. 001)", base)
	}

	// Split into Up and Down sections.
	parts := strings.Split(text, "-- +migrate Down")
	if len(parts) != 2 {
		addErr("file must contain exactly one '-- +migrate Down' marker, got %d splits", len(parts))
		return nil, errs
	}
	upPart := strings.TrimSpace(parts[0])
	downPart := strings.TrimSpace(parts[1])

	// Remove the "-- +migrate Up" line from upPart.
	upContent := strings.Replace(upPart, "-- +migrate Up", "", 1)
	downContent := downPart

	// Extract CREATE TABLE and DROP TABLE statements for consistency checking.
	upTables := extractCreateTables(upContent)
	downTables := extractDropTables(downContent)

	mf := &migrationFile{
		path:        path,
		filename:    base,
		upContent:   upContent,
		downContent: downContent,
		upTables:    upTables,
		downTables:  downTables,
	}
	if matches != nil {
		mf.version, _ = strconv.Atoi(matches[1])
	}

	return mf, errs
}

// extractCreateTables finds table names from CREATE TABLE statements.
func extractCreateTables(sql string) []string {
	re := regexp.MustCompile(`(?i)\bCREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?(\w+)`)
	matches := re.FindAllStringSubmatch(sql, -1)
	var tables []string
	seen := make(map[string]bool)
	for _, m := range matches {
		name := m[1]
		if !seen[name] {
			tables = append(tables, name)
			seen[name] = true
		}
	}
	return tables
}

// extractDropTables finds table names from DROP TABLE statements.
func extractDropTables(sql string) []string {
	re := regexp.MustCompile(`(?i)\bDROP\s+TABLE\s+(?:IF\s+EXISTS\s+)?(\w+)`)
	matches := re.FindAllStringSubmatch(sql, -1)
	var tables []string
	seen := make(map[string]bool)
	for _, m := range matches {
		name := m[1]
		if !seen[name] {
			tables = append(tables, name)
			seen[name] = true
		}
	}
	return tables
}

// splitSQLStatements splits SQL text into individual statements, respecting
// string literals (simple heuristic, not a full parser).
func splitSQLStatements(sql string) []string {
	var stmts []string
	var current strings.Builder
	inString := false
	stringChar := byte(0)

	for i := 0; i < len(sql); i++ {
		ch := sql[i]
		if inString {
			current.WriteByte(ch)
			if ch == stringChar && (i+1 >= len(sql) || sql[i+1] != stringChar) {
				inString = false
			}
			continue
		}
		if ch == '\'' || ch == '"' {
			inString = true
			stringChar = ch
			current.WriteByte(ch)
			continue
		}
		if ch == '-' && i+1 < len(sql) && sql[i+1] == '-' {
			// Skip single-line comment.
			for i < len(sql) && sql[i] != '\n' {
				i++
			}
			continue
		}
		if ch == '/' && i+1 < len(sql) && sql[i+1] == '*' {
			// Skip block comment.
			i += 2
			for i+1 < len(sql) && !(sql[i] == '*' && sql[i+1] == '/') {
				i++
			}
			i++ // skip closing /
			continue
		}
		if ch == ';' {
			stmt := strings.TrimSpace(current.String())
			if stmt != "" {
				stmts = append(stmts, stmt+";")
			}
			current.Reset()
			continue
		}
		current.WriteByte(ch)
	}
	stmt := strings.TrimSpace(current.String())
	if stmt != "" {
		stmts = append(stmts, stmt)
	}
	return stmts
}

func safePrefix(s string, n int) string {
	runes := []rune(s)
	if len(runes) > n {
		return string(runes[:n]) + "..."
	}
	return s
}

// ─────────────────────────────────────────────────────────────────────────────
// TestMigrationOrder — verify version numbering (001-007) is complete & sequential
// ─────────────────────────────────────────────────────────────────────────────

func TestMigrationOrder(t *testing.T) {
	files, err := filepath.Glob(filepath.Join(migrationsDir, "*.sql"))
	if err != nil {
		t.Fatalf("failed to list migration files: %v", err)
	}

	// Extract version numbers.
	re := regexp.MustCompile(`^(\d{3})_`)
	var versions []int
	versionMap := make(map[int]string) // version -> filename
	for _, f := range files {
		base := filepath.Base(f)
		matches := re.FindStringSubmatch(base)
		if matches == nil {
			t.Errorf("file %q does not match version pattern NNN_name.sql", base)
			continue
		}
		v, _ := strconv.Atoi(matches[1])
		versions = append(versions, v)
		versionMap[v] = base
	}

	sort.Ints(versions)

	if len(versions) == 0 {
		t.Fatal("no valid migration files found")
	}

	// Check for gaps in the sequence.
	t.Logf("Found %d migration(s): versions %v", len(versions), versions)

	expectedCount := versions[len(versions)-1]
	if len(versions) != expectedCount {
		t.Errorf("expected %d migration files (001-%03d), got %d", expectedCount, expectedCount, len(versions))
	}

	for i, v := range versions {
		expectedVersion := i + 1
		if v != expectedVersion {
			t.Errorf("version hole at position %d: expected %03d, got %03d (%s)",
				i+1, expectedVersion, v, versionMap[v])
		}
	}

	// Verify no duplicate version numbers.
	if len(versions) != len(versionMap) {
		t.Error("duplicate version numbers detected")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TestSeedDataConsistency — verify seed.sh INSERT columns match the schema
// ─────────────────────────────────────────────────────────────────────────────

func TestSeedDataConsistency(t *testing.T) {
	// Read seed script.
	seedContent, err := os.ReadFile(seedScript)
	if err != nil {
		t.Fatalf("failed to read seed.sh: %v", err)
	}
	seedText := string(seedContent)

	// Also read all migration files to extract column schemas.
	migrationFiles, err := filepath.Glob(filepath.Join(migrationsDir, "*.sql"))
	if err != nil {
		t.Fatalf("failed to list migration files: %v", err)
	}

	// Parse table schemas from all migration files.
	schemas := parseAllTableSchemas(migrationFiles)

	t.Run("seed_truncate_tables", func(t *testing.T) {
		// Extract TRUNCATE table names from seed.sh.
		re := regexp.MustCompile(`(?i)TRUNCATE\s+TABLE\s+(.*?)(?:\s+CASCADE|\s*;|$)`)
		match := re.FindStringSubmatch(seedText)
		if match == nil {
			t.Fatal("no TRUNCATE TABLE statement found in seed.sh")
		}
		tableList := match[1]
		tables := strings.FieldsFunc(tableList, func(r rune) bool {
			return r == ',' || r == ' '
		})
		validTables := knownTableNames()

		for _, tbl := range tables {
			tbl = strings.TrimSpace(tbl)
			// Check if this is a known table name.
			if _, ok := validTables[tbl]; !ok {
				t.Errorf("TRUNCATE references unknown/misspelled table %q", tbl)
			}
		}
	})

	t.Run("molecules_raw_json", func(t *testing.T) {
		// seed.sh line ~135: INSERT INTO molecules (id, name, smiles, molecular_formula, raw_json)
		// Check if raw_json exists in the molecules schema.
		cols := schemas["molecules"]
		if cols == nil {
			t.Fatal("molecules table schema not found in migrations")
		}
		t.Logf("molecules table has columns: %v", sortedKeys(cols))
		if _, ok := cols["raw_json"]; !ok {
			t.Error(`seed.sh inserts into molecules with column "raw_json", but molecules table has no "raw_json" column. Did you mean "metadata"?`)
		}
	})

	t.Run("patents_raw_json", func(t *testing.T) {
		// seed.sh line ~154: INSERT INTO patents (id, patent_number, title, raw_json)
		cols := schemas["patents"]
		if cols == nil {
			t.Fatal("patents table schema not found in migrations")
		}
		t.Logf("patents table has columns: %v", sortedKeys(cols))
		if _, ok := cols["raw_json"]; !ok {
			t.Error(`seed.sh inserts into patents with column "raw_json", but patents table has no "raw_json" column. Did you mean "raw_data"?`)
		}
	})

	t.Run("seed_on_conflict", func(t *testing.T) {
		// seed.sh uses: ON CONFLICT (id) DO UPDATE SET raw_json = EXCLUDED.raw_json
		// This also references raw_json which does not exist.
		re := regexp.MustCompile(`(?i)ON CONFLICT\s*\([^)]+\)\s*DO UPDATE SET\s+raw_json\s*=`)
		if re.MatchString(seedText) {
			t.Error(`ON CONFLICT clause references column "raw_json" which does not exist in any table schema; seed.sh uses "raw_json" but it is not defined in any migration table definition`)
		}
	})

	t.Run("seed_columns_match_schema", func(t *testing.T) {
		// Generic check: extract all INSERT INTO ... (col1, col2, ...) from seed.sh
		// and verify each column exists in the target table's schema.
		re := regexp.MustCompile(`(?i)INSERT\s+INTO\s+(\w+)\s*\(([^)]+)\)`)
		matches := re.FindAllStringSubmatch(seedText, -1)
		for _, m := range matches {
			tableName := strings.ToLower(m[1])
			colsStr := m[2]
			colNames := strings.Split(colsStr, ",")

			tableSchema := schemas[tableName]
			if tableSchema == nil {
				t.Errorf("seed.sh references table %q which has no schema defined in migrations", tableName)
				continue
			}

			for _, col := range colNames {
				col = strings.TrimSpace(col)
				col = strings.Trim(col, `"'`)
				colLower := strings.ToLower(col)
				if _, ok := tableSchema[colLower]; !ok {
					t.Errorf("seed.sh column %q.%s does not exist in migration schema. Existing columns: %v",
						tableName, col, sortedKeys(tableSchema))
				}
			}
		}
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// Schema parsing helpers
// ─────────────────────────────────────────────────────────────────────────────

// parseAllTableSchemas reads all migration SQL files and extracts column
// definitions for each CREATE TABLE statement.
func parseAllTableSchemas(files []string) map[string]map[string]bool {
	result := make(map[string]map[string]bool)

	for _, f := range files {
		content, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		text := string(content)

		// Extract only the Up section.
		upParts := strings.Split(text, "-- +migrate Down")
		if len(upParts) == 0 {
			continue
		}
		upSQL := upParts[0]
		// Remove the Up marker line.
		upSQL = strings.Replace(upSQL, "-- +migrate Up", "", 1)
		upSQL = strings.Replace(upSQL, "--Personal.AI order the ending", "", 1)

		// Find all CREATE TABLE statements and extract column names.
		tableRe := regexp.MustCompile(`(?i)CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?(\w+)\s*\(`)
		tableMatches := tableRe.FindAllStringSubmatchIndex(upSQL, -1)

		for _, tm := range tableMatches {
			tableName := upSQL[tm[2]:tm[3]]
			tableName = strings.ToLower(tableName)

			// Find the matching closing paren for this CREATE TABLE.
			start := tm[1] // position after '('
			depth := 1
			end := start
			for i := start; i < len(upSQL) && depth > 0; i++ {
				switch upSQL[i] {
				case '(':
					depth++
				case ')':
					depth--
				}
				if depth == 0 {
					end = i
					break
				}
			}

			body := upSQL[start:end]

			if result[tableName] == nil {
				result[tableName] = make(map[string]bool)
			}

			// Extract individual column definitions.
			cols := splitTopLevelCommas(body)
			for _, colDef := range cols {
				colDef = strings.TrimSpace(colDef)
				if colDef == "" {
					continue
				}
				// Skip constraints, indexes, and key definitions.
				if ignoreColDef(colDef) {
					continue
				}
				// First word is the column name.
				parts := strings.Fields(colDef)
				if len(parts) > 0 {
					colName := strings.ToLower(parts[0])
					result[tableName][colName] = true
				}
			}
		}
	}
	return result
}

// splitTopLevelCommas splits a string on commas that are not nested inside
// parentheses.
func splitTopLevelCommas(s string) []string {
	var parts []string
	depth := 0
	start := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '(':
			depth++
		case ')':
			depth--
		case ',':
			if depth == 0 {
				parts = append(parts, s[start:i])
				start = i + 1
			}
		}
	}
	if start < len(s) {
		parts = append(parts, s[start:])
	}
	return parts
}

// ignoreColDef returns true if the column definition text is not a column but
// a constraint, key, or index definition within a CREATE TABLE.
func ignoreColDef(s string) bool {
	upper := strings.ToUpper(strings.TrimSpace(s))
	if strings.HasPrefix(upper, "PRIMARY KEY") ||
		strings.HasPrefix(upper, "UNIQUE") ||
		strings.HasPrefix(upper, "FOREIGN KEY") ||
		strings.HasPrefix(upper, "CHECK") ||
		strings.HasPrefix(upper, "CONSTRAINT") ||
		strings.HasPrefix(upper, "INDEX") ||
		strings.HasPrefix(upper, "KEY ") ||
		strings.HasPrefix(upper, "EXCLUDE") ||
		strings.HasPrefix(upper, "CREATE ") ||
		strings.HasPrefix(upper, "ALTER ") {
		return true
	}
	return false
}

// knownTableNames returns a set of all table names defined across all migrations.
func knownTableNames() map[string]bool {
	return map[string]bool{
		"patents":                        true,
		"patent_claims":                  true,
		"patent_inventors":               true,
		"patent_priority_claims":         true,
		"molecules":                      true,
		"molecule_fingerprints":          true,
		"molecule_properties":            true,
		"patent_molecule_relations":      true,
		"portfolios":                     true,
		"portfolio_patents":              true,
		"patent_valuations":              true,
		"portfolio_health_scores":        true,
		"portfolio_optimization_suggestions": true,
		"patent_annuities":               true,
		"patent_deadlines":               true,
		"patent_lifecycle_events":        true,
		"patent_cost_records":            true,
		"users":                          true,
		"organizations":                  true,
		"organization_members":           true,
		"roles":                          true,
		"user_roles":                     true,
		"api_keys":                       true,
		"audit_logs":                     true,
		"workspaces":                     true,
		"workspace_members":              true,
		"workspace_projects":             true,
		"project_patents":                true,
		"project_molecules":              true,
		"comments":                       true,
		"notifications":                  true,
		"saved_searches":                 true,
	}
}

// sortedKeys returns sorted keys of a string map for display.
func sortedKeys(m map[string]bool) []string {
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// ─────────────────────────────────────────────────────────────────────────────
// TestDownSection — validate each migration's Down section reverses its Up
// ─────────────────────────────────────────────────────────────────────────────

func TestDownSection_ReversesCreateTable(t *testing.T) {
	files, err := filepath.Glob(filepath.Join(migrationsDir, "*.sql"))
	if err != nil {
		t.Fatalf("failed to list migration files: %v", err)
	}

	for _, f := range files {
		name := filepath.Base(f)
		t.Run(name, func(t *testing.T) {
			mf, errs := parseAndValidateMigration(f)
			for _, e := range errs {
				t.Error(e)
			}
			if t.Failed() {
				return
			}

			// Each CREATE TABLE should have a corresponding DROP TABLE in Down.
			upCreates := mf.upTables
			downDrops := mf.downTables

			for _, tbl := range upCreates {
				found := false
				for _, dt := range downDrops {
					if dt == tbl {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("table %q created in Up but not dropped in Down", tbl)
				}
			}
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test003PortfolioOwnerFK — verify that migration 005 adds FK constraints that
// reference tables from migration 003/004.
// ─────────────────────────────────────────────────────────────────────────────

func Test003_ForwardReferenceFK(t *testing.T) {
	// portfolios.owner_id is defined in 003 with UUID NOT NULL, but it
	// references users(id) which is not created until 005.
	// Migration 005 fixes this with ALTER TABLE ADD CONSTRAINT.
	// We verify that 005's Down correctly drops the constraint.
	mig005Path := filepath.Join(migrationsDir, "005_create_users.sql")
	if _, err := os.Stat(mig005Path); os.IsNotExist(err) {
		t.Skip("005_create_users.sql not found")
	}

	content, err := os.ReadFile(mig005Path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(content)

	// Extract Down section.
	parts := strings.Split(text, "-- +migrate Down")
	if len(parts) != 2 {
		t.Fatal("invalid migration file format")
	}
	downSQL := parts[1]

	// Verify Down section drops the FK constraints added in this migration.
	if !strings.Contains(downSQL, "fk_patents_assignee") {
		t.Error("005 Down should drop fk_patents_assignee constraint")
	}
	if !strings.Contains(downSQL, "fk_portfolios_owner") {
		t.Error("005 Down should drop fk_portfolios_owner constraint")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test007MigrationDown — 007 is special: it only adds a value to an ENUM.
// The Down section is a no-op (SELECT 1) because Postgres cannot remove
// individual ENUM values. We verify this is intentional.
// ─────────────────────────────────────────────────────────────────────────────

func Test007_NoopDown(t *testing.T) {
	mig007Path := filepath.Join(migrationsDir, "007_fix_molecule_status_enum.sql")
	if _, err := os.Stat(mig007Path); os.IsNotExist(err) {
		t.Skip("007_fix_molecule_status_enum.sql not found")
	}

	content, err := os.ReadFile(mig007Path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(content)

	parts := strings.Split(text, "-- +migrate Down")
	if len(parts) != 2 {
		t.Fatal("invalid migration file format")
	}
	downSQL := strings.TrimSpace(parts[1])

	// Verify it doesn't try DROP TYPE or ALTER TYPE which would fail.
	if strings.Contains(downSQL, "DROP TYPE") && !strings.Contains(downSQL, "molecule_status_new") {
		t.Error("007 Down should not drop types directly; postgres cannot remove enum values")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TestSeedRolesConsistency — verify seed data INSERT of roles in 005 matches
// the role table schema.
// ─────────────────────────────────────────────────────────────────────────────

func TestSeedRolesConsistency(t *testing.T) {
	mig005Path := filepath.Join(migrationsDir, "005_create_users.sql")
	if _, err := os.Stat(mig005Path); os.IsNotExist(err) {
		t.Skip("005_create_users.sql not found")
	}

	content, err := os.ReadFile(mig005Path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(content)

	parts := strings.Split(text, "-- +migrate Down")
	if len(parts) == 0 {
		t.Fatal("invalid migration file format")
	}
	upSQL := parts[0]

	// Extract INSERT INTO roles statement.
	re := regexp.MustCompile(`(?i)INSERT\s+INTO\s+roles\s*\(([^)]+)\)\s*VALUES`)
	match := re.FindStringSubmatch(upSQL)
	if match == nil {
		t.Fatal("INSERT INTO roles not found in 005 migration")
	}

	columns := strings.Split(match[1], ",")
	for i, col := range columns {
		columns[i] = strings.TrimSpace(col)
	}

	// Verify roles table columns match what migrations define.
	expectedCols := []string{"name", "display_name", "description", "is_system", "permissions"}
	if len(columns) != len(expectedCols) {
		t.Errorf("INSERT INTO roles has %d columns, schema defines %d: got %v, expected %v",
			len(columns), len(expectedCols), columns, expectedCols)
	} else {
		for i, col := range columns {
			if col != expectedCols[i] {
				t.Errorf("INSERT INTO roles column %d: got %q, expected %q", i+1, col, expectedCols[i])
			}
		}
	}

	// Count the number of role seed rows by finding VALUES tuples that
	// start with a role name string literal.
	valueRe := regexp.MustCompile(`\(\s*'(\w+)'\s*,`)
	valueMatches := valueRe.FindAllStringSubmatch(upSQL, -1)
	roleCount := 0
	expectedRoles := []string{"super_admin", "org_admin", "patent_analyst", "researcher", "viewer"}
	seenRoles := make(map[string]bool)
	for _, vm := range valueMatches {
		roleName := vm[1]
		for _, expected := range expectedRoles {
			if roleName == expected {
				seenRoles[roleName] = true
				roleCount++
				break
			}
		}
	}
	if roleCount != 5 {
		t.Errorf("expected 5 seed roles, found %d. Seen: %v", roleCount, sortedKeys(seenRoles))
	}
	// Verify all expected roles are present.
	for _, expected := range expectedRoles {
		if !seenRoles[expected] {
			t.Errorf("missing seed role: %s", expected)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TestAllFilesHaveMetadataHeader — verify each migration has the expected header
// ─────────────────────────────────────────────────────────────────────────────

func TestAllFilesHaveHeaders(t *testing.T) {
	files, err := filepath.Glob(filepath.Join(migrationsDir, "*.sql"))
	if err != nil {
		t.Fatalf("failed to list migration files: %v", err)
	}

	for _, f := range files {
		name := filepath.Base(f)
		t.Run(name, func(t *testing.T) {
			fh, err := os.Open(f)
			if err != nil {
				t.Fatal(err)
			}
			defer fh.Close()

			scanner := bufio.NewScanner(fh)
			firstLine := ""
			if scanner.Scan() {
				firstLine = scanner.Text()
			}

			if !strings.HasPrefix(firstLine, "-- +migrate Up") &&
				!strings.HasPrefix(firstLine, "-- +migrate Down") {
				// Allow starting with a comment line before the marker.
				hasMarker := false
				for scanner.Scan() {
					if strings.HasPrefix(scanner.Text(), "-- +migrate Up") {
						hasMarker = true
						break
					}
				}
				if !hasMarker {
					t.Error("file does not contain '-- +migrate Up' marker")
				}
			}
		})
	}
}

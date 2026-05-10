// Phase 17 - Integration Test: Repository Operations
// Validates CRUD operations on repositories backed by PostgreSQL.
// Tests exercise real database queries when Postgres is available, and
// gracefully skip when it is not.
package integration

import (
	"testing"
	"time"
)

// TestRepository_MoleculeCRUD validates basic molecule repository operations.
func TestRepository_MoleculeCRUD(t *testing.T) {
	env := SetupTestEnvironment(t)
	RequirePostgres(t, env)

	t.Run("InsertAndRetrieveMolecule", func(t *testing.T) {
		molID := NextTestID("mol")
		now := time.Now()

		// Insert a molecule.
		_, err := env.PostgresDB.ExecContext(env.Ctx,
			`INSERT INTO molecules (id, smiles, inchi, inchi_key, molecular_formula, molecular_weight, status, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
			molID, "c1ccccc1", "InChI=1S/C6H6/c1-2-4-6-5-3-1/h1-6H",
			"UHOVQNZJYSORNB-UHFFFAOYSA-N", "C6H6", 78.11, "active", now, now,
		)
		if err != nil {
			t.Fatalf("insert molecule: %v", err)
		}
		t.Logf("inserted molecule %s", molID)

		// Retrieve the molecule.
		var retrievedID, retrievedSMILES, retrievedStatus string
		err = env.PostgresDB.QueryRowContext(env.Ctx,
			`SELECT id, smiles, status FROM molecules WHERE id = $1`, molID,
		).Scan(&retrievedID, &retrievedSMILES, &retrievedStatus)
		if err != nil {
			t.Fatalf("retrieve molecule: %v", err)
		}

		if retrievedID != molID {
			t.Fatalf("expected ID %s, got %s", molID, retrievedID)
		}
		if retrievedSMILES != "c1ccccc1" {
			t.Fatalf("expected SMILES c1ccccc1, got %s", retrievedSMILES)
		}
		if retrievedStatus != "active" {
			t.Fatalf("expected status active, got %s", retrievedStatus)
		}
		t.Logf("retrieved molecule %s: smiles=%s, status=%s", retrievedID, retrievedSMILES, retrievedStatus)
	})

	t.Run("UpdateMolecule", func(t *testing.T) {
		molID := NextTestID("mol")
		now := time.Now()

		// Insert then update.
		_, err := env.PostgresDB.ExecContext(env.Ctx,
			`INSERT INTO molecules (id, smiles, molecular_formula, molecular_weight, status, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			molID, "CCO", "C2H6O", 46.07, "active", now, now,
		)
		if err != nil {
			t.Fatalf("insert molecule for update: %v", err)
		}

		newStatus := "archived"
		_, err = env.PostgresDB.ExecContext(env.Ctx,
			`UPDATE molecules SET status = $1, updated_at = $2 WHERE id = $3`,
			newStatus, time.Now(), molID,
		)
		if err != nil {
			t.Fatalf("update molecule: %v", err)
		}

		var status string
		err = env.PostgresDB.QueryRowContext(env.Ctx,
			`SELECT status FROM molecules WHERE id = $1`, molID,
		).Scan(&status)
		if err != nil {
			t.Fatalf("verify update: %v", err)
		}
		if status != newStatus {
			t.Fatalf("expected status %s after update, got %s", newStatus, status)
		}
		t.Logf("molecule %s status updated to %s", molID, status)
	})

	t.Run("DeleteMolecule", func(t *testing.T) {
		molID := NextTestID("mol")
		now := time.Now()

		_, err := env.PostgresDB.ExecContext(env.Ctx,
			`INSERT INTO molecules (id, smiles, molecular_formula, molecular_weight, status, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			molID, "C", "C", 12.01, "active", now, now,
		)
		if err != nil {
			t.Fatalf("insert molecule for delete: %v", err)
		}

		_, err = env.PostgresDB.ExecContext(env.Ctx,
			`DELETE FROM molecules WHERE id = $1`, molID,
		)
		if err != nil {
			t.Fatalf("delete molecule: %v", err)
		}

		var count int
		err = env.PostgresDB.QueryRowContext(env.Ctx,
			`SELECT COUNT(*) FROM molecules WHERE id = $1`, molID,
		).Scan(&count)
		if err != nil {
			t.Fatalf("verify delete: %v", err)
		}
		if count != 0 {
			t.Fatalf("expected 0 rows after delete, got %d", count)
		}
		t.Logf("molecule %s successfully deleted", molID)
	})

	t.Run("BulkInsertMolecules", func(t *testing.T) {
		count := 5
		now := time.Now()

		for i := 0; i < count; i++ {
			molID := NextTestID("mol")
			smiles := "C" + string(rune('A'+(i%26)))
			_, err := env.PostgresDB.ExecContext(env.Ctx,
				`INSERT INTO molecules (id, smiles, molecular_formula, molecular_weight, status, created_at, updated_at)
				 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
				molID, smiles, "CxHy", 0.0, "active", now, now,
			)
			if err != nil {
				t.Fatalf("bulk insert molecule %d: %v", i, err)
			}
		}

		var total int
		err := env.PostgresDB.QueryRowContext(env.Ctx,
			`SELECT COUNT(*) FROM molecules WHERE status = 'active' AND updated_at >= $1`,
			now,
		).Scan(&total)
		if err != nil {
			t.Fatalf("count molecules: %v", err)
		}
		if total < count {
			t.Fatalf("expected at least %d molecules, got %d", count, total)
		}
		t.Logf("bulk insert verified: %d molecules found", total)
	})
}

// TestRepository_PatentCRUD validates basic patent repository operations.
func TestRepository_PatentCRUD(t *testing.T) {
	env := SetupTestEnvironment(t)
	RequirePostgres(t, env)

	t.Run("InsertAndRetrievePatent", func(t *testing.T) {
		patID := NextTestID("pat")
		now := time.Now()

		_, err := env.PostgresDB.ExecContext(env.Ctx,
			`INSERT INTO patents (id, patent_number, title, abstract, assignee, filing_date, publication_date, legal_status, jurisdiction, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
			patID, "US20240000001A1", "Test Patent", "Test abstract", "Test Assignee",
			now, now, "pending", "US", now, now,
		)
		if err != nil {
			t.Fatalf("insert patent: %v", err)
		}
		t.Logf("inserted patent %s", patID)

		var retrievedID, retrievedNumber, retrievedTitle string
		err = env.PostgresDB.QueryRowContext(env.Ctx,
			`SELECT id, patent_number, title FROM patents WHERE id = $1`, patID,
		).Scan(&retrievedID, &retrievedNumber, &retrievedTitle)
		if err != nil {
			t.Fatalf("retrieve patent: %v", err)
		}

		if retrievedID != patID {
			t.Fatalf("expected ID %s, got %s", patID, retrievedID)
		}
		t.Logf("retrieved patent %s: number=%s, title=%s", retrievedID, retrievedNumber, retrievedTitle)
	})

	t.Run("UpdatePatentStatus", func(t *testing.T) {
		patID := NextTestID("pat")
		now := time.Now()

		_, err := env.PostgresDB.ExecContext(env.Ctx,
			`INSERT INTO patents (id, patent_number, title, assignee, filing_date, publication_date, legal_status, jurisdiction, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
			patID, "CN20240000001A", "Test CN Patent", "CN Assignee",
			now, now, "filed", "CN", now, now,
		)
		if err != nil {
			t.Fatalf("insert patent for update: %v", err)
		}

		// Transition status from filed to published.
		_, err = env.PostgresDB.ExecContext(env.Ctx,
			`UPDATE patents SET legal_status = $1, updated_at = $2 WHERE id = $3`,
			"published", time.Now(), patID,
		)
		if err != nil {
			t.Fatalf("update patent status: %v", err)
		}

		var status string
		err = env.PostgresDB.QueryRowContext(env.Ctx,
			`SELECT legal_status FROM patents WHERE id = $1`, patID,
		).Scan(&status)
		if err != nil {
			t.Fatalf("verify patent update: %v", err)
		}
		if status != "published" {
			t.Fatalf("expected status published, got %s", status)
		}
		t.Logf("patent %s status updated to %s", patID, status)
	})

	t.Run("PatentSearchByKeyword", func(t *testing.T) {
		now := time.Now()

		// Insert patents with searchable titles.
		titles := []string{
			"OLED Display Device",
			"Organic Light Emitting Diode",
			"Liquid Crystal Display",
			"OLED Driver Circuit",
		}

		for i, title := range titles {
			patID := NextTestID("pat")
			_, err := env.PostgresDB.ExecContext(env.Ctx,
				`INSERT INTO patents (id, patent_number, title, abstract, assignee, filing_date, publication_date, legal_status, jurisdiction, created_at, updated_at)
				 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
				patID, "US2024TEST"+string(rune('A'+i)), title, "Abstract for "+title,
				"Test Corp", now, now, "pending", "US", now, now,
			)
			if err != nil {
				t.Fatalf("insert search patent %d: %v", i, err)
			}
		}

		// Search for OLED patents using LIKE.
		rows, err := env.PostgresDB.QueryContext(env.Ctx,
			`SELECT id, patent_number, title FROM patents WHERE title ILIKE $1 AND jurisdiction = $2`,
			"%OLED%", "US",
		)
		if err != nil {
			t.Fatalf("search patents: %v", err)
		}
		defer rows.Close()

		var searchResults int
		for rows.Next() {
			var id, num, title string
			if err := rows.Scan(&id, &num, &title); err != nil {
				t.Fatalf("scan search result: %v", err)
			}
			searchResults++
			t.Logf("  patent search result: %s - %s", num, title)
		}
		if searchResults < 2 {
			t.Fatalf("expected at least 2 OLED patents, found %d", searchResults)
		}
		t.Logf("patent keyword search: %d results for %%OLED%%", searchResults)
	})
}

// TestRepository_PortfolioCRUD validates basic portfolio repository operations.
func TestRepository_PortfolioCRUD(t *testing.T) {
	env := SetupTestEnvironment(t)
	RequirePostgres(t, env)

	t.Run("CreateAndRetrievePortfolio", func(t *testing.T) {
		portfolioID := NextTestID("pf")
		now := time.Now()

		_, err := env.PostgresDB.ExecContext(env.Ctx,
			`INSERT INTO portfolios (id, name, description, owner_id, strategy, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			portfolioID, "Test Portfolio", "Integration test portfolio", "user-test-001", "balanced", now, now,
		)
		if err != nil {
			t.Fatalf("insert portfolio: %v", err)
		}
		t.Logf("inserted portfolio %s", portfolioID)

		var name, strategy string
		err = env.PostgresDB.QueryRowContext(env.Ctx,
			`SELECT name, strategy FROM portfolios WHERE id = $1`, portfolioID,
		).Scan(&name, &strategy)
		if err != nil {
			t.Fatalf("retrieve portfolio: %v", err)
		}
		if name != "Test Portfolio" || strategy != "balanced" {
			t.Fatalf("unexpected portfolio data: name=%s, strategy=%s", name, strategy)
		}
		t.Logf("retrieved portfolio: name=%s, strategy=%s", name, strategy)
	})

	t.Run("AddPatentsToPortfolio", func(t *testing.T) {
		portfolioID := NextTestID("pf")
		now := time.Now()

		// Create portfolio.
		_, err := env.PostgresDB.ExecContext(env.Ctx,
			`INSERT INTO portfolios (id, name, owner_id, strategy, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, $5, $6)`,
			portfolioID, "Patent-Adding Portfolio", "user-test-001", "defensive", now, now,
		)
		if err != nil {
			t.Fatalf("insert portfolio for patent add: %v", err)
		}

		// Create patents and link to portfolio.
		patentIDs := make([]string, 3)
		for i := 0; i < 3; i++ {
			patentIDs[i] = NextTestID("pat")
			_, err := env.PostgresDB.ExecContext(env.Ctx,
				`INSERT INTO patents (id, patent_number, title, assignee, filing_date, publication_date, legal_status, jurisdiction, created_at, updated_at)
				 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
				patentIDs[i], "US2024PORT"+string(rune('A'+i)), "Portfolio Patent "+string(rune('A'+i)),
				"Portfolio Corp", now, now, "granted", "US", now, now,
			)
			if err != nil {
				t.Fatalf("insert portfolio patent %d: %v", i, err)
			}

			// Link patent to portfolio.
			_, err = env.PostgresDB.ExecContext(env.Ctx,
				`INSERT INTO portfolio_patents (portfolio_id, patent_id, added_at)
				 VALUES ($1, $2, $3)`,
				portfolioID, patentIDs[i], now,
			)
			if err != nil {
				t.Fatalf("link patent to portfolio: %v", err)
			}
		}

		// Verify link count.
		var linkCount int
		err = env.PostgresDB.QueryRowContext(env.Ctx,
			`SELECT COUNT(*) FROM portfolio_patents WHERE portfolio_id = $1`, portfolioID,
		).Scan(&linkCount)
		if err != nil {
			t.Fatalf("count portfolio patents: %v", err)
		}
		if linkCount != 3 {
			t.Fatalf("expected 3 patents in portfolio, got %d", linkCount)
		}
		t.Logf("portfolio %s has %d patents linked", portfolioID, linkCount)
	})

	t.Run("RemovePatentFromPortfolio", func(t *testing.T) {
		portfolioID := NextTestID("pf")
		patentID := NextTestID("pat")
		now := time.Now()

		// Create portfolio and patent.
		_, err := env.PostgresDB.ExecContext(env.Ctx,
			`INSERT INTO portfolios (id, name, owner_id, strategy, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, $5, $6)`,
			portfolioID, "Removal Test Portfolio", "user-test-001", "balanced", now, now,
		)
		if err != nil {
			t.Fatalf("insert removal portfolio: %v", err)
		}

		_, err = env.PostgresDB.ExecContext(env.Ctx,
			`INSERT INTO patents (id, patent_number, title, assignee, filing_date, publication_date, legal_status, jurisdiction, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
			patentID, "US2024REMOVE001", "Removal Test Patent", "Test Corp",
			now, now, "pending", "US", now, now,
		)
		if err != nil {
			t.Fatalf("insert removal patent: %v", err)
		}

		// Link and then remove.
		_, err = env.PostgresDB.ExecContext(env.Ctx,
			`INSERT INTO portfolio_patents (portfolio_id, patent_id, added_at) VALUES ($1, $2, $3)`,
			portfolioID, patentID, now,
		)
		if err != nil {
			t.Fatalf("link patent: %v", err)
		}

		_, err = env.PostgresDB.ExecContext(env.Ctx,
			`DELETE FROM portfolio_patents WHERE portfolio_id = $1 AND patent_id = $2`,
			portfolioID, patentID,
		)
		if err != nil {
			t.Fatalf("remove patent from portfolio: %v", err)
		}

		var count int
		err = env.PostgresDB.QueryRowContext(env.Ctx,
			`SELECT COUNT(*) FROM portfolio_patents WHERE portfolio_id = $1 AND patent_id = $2`,
			portfolioID, patentID,
		).Scan(&count)
		if err != nil {
			t.Fatalf("verify removal: %v", err)
		}
		if count != 0 {
			t.Fatalf("expected 0 links after removal, got %d", count)
		}
		t.Logf("patent %s successfully removed from portfolio %s", patentID, portfolioID)
	})
}

// Personal.AI order the ending

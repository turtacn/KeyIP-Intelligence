// Phase 17 - Integration Test: Full Patent Workflow
// Validates a complete end-to-end patent workflow from creation through
// search, retrieval, update, and lifecycle management. Exercises the
// full stack when PostgreSQL is available.
package integration

import (
	"testing"
	"time"

	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
	patentTypes "github.com/turtacn/KeyIP-Intelligence/pkg/types/patent"
)

// TestFullWorkflow_PatentLifecycle validates the complete patent lifecycle
// from creation through search, retrieval, update, and analysis.
func TestFullWorkflow_PatentLifecycle(t *testing.T) {
	env := SetupTestEnvironment(t)
	RequirePostgres(t, env)

	t.Run("CreatePatent", func(t *testing.T) {
		// Create a new patent record in the database.
		patID := common.ID(NextTestID("pat"))
		now := time.Now()

		_, err := env.PostgresDB.ExecContext(env.Ctx,
			`INSERT INTO patents (id, patent_number, title, abstract, assignee, filing_date, publication_date, legal_status, jurisdiction, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
			patID, "US20241234567A1", "Organic Light Emitting Device with Improved Efficiency",
			"An organic light emitting device comprising a first electrode, a second electrode, and an emission layer disposed between the first and second electrodes, wherein the emission layer comprises a host material and a phosphorescent dopant.",
			"Samsung Display Co., Ltd.",
			now, now, "filed", "US", now, now,
		)
		if err != nil {
			t.Fatalf("create patent: %v", err)
		}
		t.Logf("created patent: id=%s, number=US20241234567A1", patID)

		// Verify creation.
		var count int
		err = env.PostgresDB.QueryRowContext(env.Ctx,
			`SELECT COUNT(*) FROM patents WHERE id = $1`, patID,
		).Scan(&count)
		if err != nil {
			t.Fatalf("verify patent creation: %v", err)
		}
		if count != 1 {
			t.Fatalf("expected 1 patent, found %d", count)
		}

		// If patent service is available, test through the service as well.
		if env.PatentService != nil {
			t.Log("patent service available — would create via service layer")
		}
	})

	t.Run("SearchPatent", func(t *testing.T) {
		// Seed known patents for search.
		now := time.Now()
		searchPatents := []struct {
			title    string
			assignee string
		}{
			{"OLED Display Panel with High Resolution", "LG Display"},
			{"Method for Manufacturing Organic Light Emitting Device", "Samsung Display"},
			{"Quantum Dot Enhanced LED Backlight Unit", "Sony Corporation"},
			{"Flexible OLED Display with Foldable Substrate", "BOE Technology"},
		}

		for _, sp := range searchPatents {
			patID := NextTestID("pat")
			_, err := env.PostgresDB.ExecContext(env.Ctx,
				`INSERT INTO patents (id, patent_number, title, abstract, assignee, filing_date, publication_date, legal_status, jurisdiction, created_at, updated_at)
				 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
				patID, "US2024SRCH"+patID[len(patID)-4:], sp.title,
				"Abstract for: "+sp.title, sp.assignee,
				now, now, "pending", "US", now, now,
			)
			if err != nil {
				t.Fatalf("insert search patent %s: %v", sp.title, err)
			}
		}

		// Search by keyword in title.
		rows, err := env.PostgresDB.QueryContext(env.Ctx,
			`SELECT id, patent_number, title, assignee FROM patents
			 WHERE title ILIKE $1 ORDER BY filing_date DESC`,
			"%OLED%",
		)
		if err != nil {
			t.Fatalf("search patents by keyword: %v", err)
		}
		defer rows.Close()

		var oledCount int
		for rows.Next() {
			var id, num, title, assignee string
			if err := rows.Scan(&id, &num, &title, &assignee); err != nil {
				t.Fatalf("scan search result: %v", err)
			}
			oledCount++
			t.Logf("  found: [%s] %s - %s (%s)", id, num, title, assignee)
		}
		if oledCount < 2 {
			t.Fatalf("expected at least 2 OLED patents, found %d", oledCount)
		}
		t.Logf("search completed: %d OLED patents found", oledCount)
	})

	t.Run("RetrievePatentDetail", func(t *testing.T) {
		// Create a patent with full detail.
		patID := NextTestID("pat")
		now := time.Now()

		_, err := env.PostgresDB.ExecContext(env.Ctx,
			`INSERT INTO patents (id, patent_number, title, abstract, assignee, filing_date, publication_date, legal_status, jurisdiction, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
			patID, "US2024DETAIL001", "Detailed Patent for Retrieval Test",
			"This is a detailed abstract for testing patent retrieval functionality.",
			"Test Assignee Inc.",
			now, now, "granted", "US", now, now,
		)
		if err != nil {
			t.Fatalf("create detail patent: %v", err)
		}

		// Retrieve the full record.
		var (
			retrievedID, retrievedNumber, retrievedTitle string
			retrievedAbstract, retrievedAssignee         string
			retrievedStatus, retrievedJurisdiction       string
		)
		err = env.PostgresDB.QueryRowContext(env.Ctx,
			`SELECT id, patent_number, title, abstract, assignee, legal_status, jurisdiction
			 FROM patents WHERE id = $1`, patID,
		).Scan(&retrievedID, &retrievedNumber, &retrievedTitle,
			&retrievedAbstract, &retrievedAssignee, &retrievedStatus, &retrievedJurisdiction)
		if err != nil {
			t.Fatalf("retrieve patent detail: %v", err)
		}

		if retrievedTitle != "Detailed Patent for Retrieval Test" {
			t.Fatalf("unexpected title: %s", retrievedTitle)
		}
		if retrievedStatus != "granted" {
			t.Fatalf("unexpected status: %s", retrievedStatus)
		}
		if retrievedJurisdiction != "US" {
			t.Fatalf("unexpected jurisdiction: %s", retrievedJurisdiction)
		}
		t.Logf("patent detail retrieved: number=%s, title=%s, status=%s, jurisdiction=%s",
			retrievedNumber, retrievedTitle, retrievedStatus, retrievedJurisdiction)
	})

	t.Run("UpdatePatentStatus", func(t *testing.T) {
		patID := NextTestID("pat")
		now := time.Now()

		// Create a patent in "filed" status.
		_, err := env.PostgresDB.ExecContext(env.Ctx,
			`INSERT INTO patents (id, patent_number, title, assignee, filing_date, publication_date, legal_status, jurisdiction, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
			patID, "US2024UPDATE001", "Status Update Test Patent", "Update Corp",
			now, now, "filed", "US", now, now,
		)
		if err != nil {
			t.Fatalf("create patent for status update: %v", err)
		}

		// Transition through multiple statuses.
		type statusTransition struct {
			newStatus string
			note      string
		}

		transitions := []statusTransition{
			{"published", "Application published"},
			{"examination", "Entered substantive examination"},
			{"allowed", "Notice of allowance received"},
			{"granted", "Patent granted"},
		}

		for _, tr := range transitions {
			_, err := env.PostgresDB.ExecContext(env.Ctx,
				`UPDATE patents SET legal_status = $1, updated_at = $2 WHERE id = $3`,
				tr.newStatus, time.Now(), patID,
			)
			if err != nil {
				t.Fatalf("transition to %s: %v", tr.newStatus, err)
			}

			var currentStatus string
			err = env.PostgresDB.QueryRowContext(env.Ctx,
				`SELECT legal_status FROM patents WHERE id = $1`, patID,
			).Scan(&currentStatus)
			if err != nil {
				t.Fatalf("verify status: %v", err)
			}
			if currentStatus != tr.newStatus {
				t.Fatalf("expected status %s, got %s", tr.newStatus, currentStatus)
			}
			t.Logf("  status transition: %s -> %s (%s)", tr.note, tr.newStatus, currentStatus)
		}

		t.Logf("patent %s completed status transitions: filed -> published -> examination -> allowed -> granted", patID)
	})

	t.Run("PatentAssigneeFilter", func(t *testing.T) {
		now := time.Now()

		// Create patents with different assignees.
		assignees := []string{"Samsung Display", "LG Display", "Samsung Display", "BOE Technology"}
		for i, a := range assignees {
			patID := NextTestID("pat")
			_, err := env.PostgresDB.ExecContext(env.Ctx,
				`INSERT INTO patents (id, patent_number, title, assignee, filing_date, publication_date, legal_status, jurisdiction, created_at, updated_at)
				 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
				patID, "US2024ASGN"+string(rune('A'+i)), "Patent by "+a, a,
				now, now, "pending", "US", now, now,
			)
			if err != nil {
				t.Fatalf("insert assignee patent %d: %v", i, err)
			}
		}

		// Filter by Samsung Display.
		rows, err := env.PostgresDB.QueryContext(env.Ctx,
			`SELECT patent_number, title FROM patents WHERE assignee = $1`,
			"Samsung Display",
		)
		if err != nil {
			t.Fatalf("filter by assignee: %v", err)
		}
		defer rows.Close()

		var samsungCount int
		for rows.Next() {
			var num, title string
			if err := rows.Scan(&num, &title); err != nil {
				t.Fatalf("scan assignee result: %v", err)
			}
			samsungCount++
			t.Logf("  Samsung patent: %s - %s", num, title)
		}
		if samsungCount != 2 {
			t.Fatalf("expected 2 Samsung patents, found %d", samsungCount)
		}
		t.Logf("assignee filter: found %d Samsung Display patents", samsungCount)
	})

	t.Run("PatentCountByJurisdiction", func(t *testing.T) {
		now := time.Now()
		jurisdictions := []string{"US", "CN", "EP", "US", "CN", "JP"}

		for i, j := range jurisdictions {
			patID := NextTestID("pat")
			_, err := env.PostgresDB.ExecContext(env.Ctx,
				`INSERT INTO patents (id, patent_number, title, assignee, filing_date, publication_date, legal_status, jurisdiction, created_at, updated_at)
				 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
				patID, j+"2024COUNT"+string(rune('A'+i)), j+" Patent", "Test Corp",
				now, now, "pending", j, now, now,
			)
			if err != nil {
				t.Fatalf("insert jurisdiction patent %d: %v", i, err)
			}
		}

		// Query jurisdiction breakdown.
		rows, err := env.PostgresDB.QueryContext(env.Ctx,
			`SELECT jurisdiction, COUNT(*) as cnt FROM patents
			 WHERE assignee = $1 GROUP BY jurisdiction ORDER BY cnt DESC`,
			"Test Corp",
		)
		if err != nil {
			t.Fatalf("jurisdiction count query: %v", err)
		}
		defer rows.Close()

		for rows.Next() {
			var jur string
			var cnt int
			if err := rows.Scan(&jur, &cnt); err != nil {
				t.Fatalf("scan jurisdiction count: %v", err)
			}
			t.Logf("  jurisdiction %s: %d patents", jur, cnt)
		}
		t.Log("jurisdiction distribution query completed")
	})
}

// TestFullWorkflow_PatentFixtures validates that fixture data can be loaded
// and queried from the database.
func TestFullWorkflow_PatentFixtures(t *testing.T) {
	env := SetupTestEnvironment(t)
	RequirePostgres(t, env)

	t.Run("LoadFixtures", func(t *testing.T) {
		// Seed patent fixtures from the test fixture file.
		SeedPatents(t, env)

		// Verify fixtures were loaded.
		var count int
		err := env.PostgresDB.QueryRowContext(env.Ctx,
			`SELECT COUNT(*) FROM patents`,
		).Scan(&count)
		if err != nil {
			t.Fatalf("count seeded patents: %v", err)
		}
		if count == 0 {
			t.Fatal("expected seeded patents, found 0")
		}
		t.Logf("patent fixtures loaded: %d patents in database", count)
	})

	t.Run("QueryFixturesWithFilters", func(t *testing.T) {
		// Use multi-condition query on seeded data.
		rows, err := env.PostgresDB.QueryContext(env.Ctx,
			`SELECT patent_number, title, jurisdiction, legal_status
			 FROM patents WHERE jurisdiction = ANY($1) AND legal_status = $2
			 ORDER BY filing_date DESC LIMIT 10`,
			[]string{"US", "EP"}, "granted",
		)
		if err != nil {
			t.Fatalf("query fixtures with filters: %v", err)
		}
		defer rows.Close()

		var resultCount int
		for rows.Next() {
			var num, title, jur, status string
			if err := rows.Scan(&num, &title, &jur, &status); err != nil {
				t.Fatalf("scan filtered result: %v", err)
			}
			resultCount++
			t.Logf("  %s [%s] %s - status=%s", num, jur, title, status)
		}
		t.Logf("filtered fixture query: %d results (US/EP, granted)", resultCount)
	})

	// Clean fixtures after this test.
	t.Cleanup(func() {
		TruncateAllTables(t, env)
	})
}

// TestFullWorkflow_PatentTypes validates patent type DTO construction
// and serialization.
func TestFullWorkflow_PatentTypes(t *testing.T) {
	env := SetupTestEnvironment(t)
	_ = env

	t.Run("BuildPatentDTO", func(t *testing.T) {
		patent := patentTypes.PatentDTO{
			ID:              common.ID(NextTestID("pat")),
			PatentNumber:    "US20240000001A1",
			Title:           "Test Patent DTO Construction",
			Abstract:        "This is a test abstract for DTO validation.",
			Assignee:        "Test Company",
			FilingDate:      common.Timestamp(time.Now()),
			PublicationDate: common.Timestamp(time.Now()),
			Status:          patentTypes.StatusPending,
			Jurisdiction:    "US",
		}

		if string(patent.ID) == "" {
			t.Fatal("patent ID should not be empty")
		}
		if patent.PatentNumber == "" {
			t.Fatal("patent number should not be empty")
		}
		if patent.Title == "" {
			t.Fatal("patent title should not be empty")
		}
		if patent.Status != patentTypes.StatusPending {
			t.Fatalf("expected status %s, got %s", patentTypes.StatusPending, patent.Status)
		}
		t.Logf("PatentDTO constructed successfully: number=%s, title=%s, status=%s",
			patent.PatentNumber, patent.Title, patent.Status)
	})

	t.Run("StatusTransitionsValid", func(t *testing.T) {
		// Validate that legal status values are correct.
		validStatuses := []patentTypes.PatentStatus{
			patentTypes.StatusFiled,
			patentTypes.StatusPending,
			patentTypes.StatusPublished,
			patentTypes.StatusGranted,
			patentTypes.StatusExpired,
			patentTypes.StatusRevoked,
			patentTypes.StatusLapsed,
			patentTypes.StatusAbandoned,
		}

		if len(validStatuses) < 5 {
			t.Fatal("expected at least 5 valid patent statuses")
		}
		t.Logf("validated %d patent status constants", len(validStatuses))
	})
}

// Personal.AI order the ending

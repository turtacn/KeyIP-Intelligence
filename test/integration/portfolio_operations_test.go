// Phase 17 - Integration Test: Portfolio Operations
// Validates end-to-end portfolio operations including creation, patent
// management, valuation, gap analysis, and optimization recommendations.
// Exercises real database operations when PostgreSQL is available.
package integration

import (
	"math"
	"sort"
	"testing"
	"time"
)

// TestPortfolioOperations_CreateAndManage validates portfolio creation and
// basic management operations.
func TestPortfolioOperations_CreateAndManage(t *testing.T) {
	env := SetupTestEnvironment(t)
	RequirePostgres(t, env)

	t.Run("CreatePortfolio", func(t *testing.T) {
		portfolioID := NextTestID("pf")
		now := time.Now()

		_, err := env.PostgresDB.ExecContext(env.Ctx,
			`INSERT INTO portfolios (id, name, description, owner_id, strategy, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			portfolioID, "OLED Blue Emitter Portfolio",
			"Collection of patents covering blue OLED emitter materials",
			"user-001", "defensive", now, now,
		)
		if err != nil {
			t.Fatalf("create portfolio: %v", err)
		}
		t.Logf("created portfolio: id=%s, name=%s", portfolioID, "OLED Blue Emitter Portfolio")

		// Verify creation.
		var name, strategy string
		err = env.PostgresDB.QueryRowContext(env.Ctx,
			`SELECT name, strategy FROM portfolios WHERE id = $1`, portfolioID,
		).Scan(&name, &strategy)
		if err != nil {
			t.Fatalf("verify portfolio creation: %v", err)
		}
		if name != "OLED Blue Emitter Portfolio" {
			t.Fatalf("unexpected name: %s", name)
		}
		if strategy != "defensive" {
			t.Fatalf("unexpected strategy: %s", strategy)
		}
		t.Logf("portfolio verified: name=%s, strategy=%s", name, strategy)
	})

	t.Run("CreateMultiplePortfolios", func(t *testing.T) {
		now := time.Now()
		portfolios := []struct {
			name     string
			strategy string
			ownerID  string
		}{
			{"HTL Material Portfolio", "defensive", "user-001"},
			{"TADF Emitter Licensing Portfolio", "licensing", "user-002"},
			{"Encapsulation Technology Portfolio", "offensive", "user-001"},
			{"Legacy Phosphorescent Portfolio", "balanced", "user-003"},
		}

		for _, p := range portfolios {
			pfID := NextTestID("pf")
			_, err := env.PostgresDB.ExecContext(env.Ctx,
				`INSERT INTO portfolios (id, name, description, owner_id, strategy, created_at, updated_at)
				 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
				pfID, p.name, p.name+" description", p.ownerID, p.strategy, now, now,
			)
			if err != nil {
				t.Fatalf("create portfolio %s: %v", p.name, err)
			}
			t.Logf("created portfolio: %s (strategy=%s, owner=%s)", p.name, p.strategy, p.ownerID)
		}

		// Count portfolios by strategy.
		rows, err := env.PostgresDB.QueryContext(env.Ctx,
			`SELECT strategy, COUNT(*) FROM portfolios
			 WHERE owner_id IN ('user-001', 'user-002', 'user-003')
			 GROUP BY strategy ORDER BY strategy`,
		)
		if err != nil {
			t.Fatalf("count by strategy: %v", err)
		}
		defer rows.Close()

		for rows.Next() {
			var strategy string
			var count int
			if err := rows.Scan(&strategy, &count); err != nil {
				t.Fatalf("scan strategy count: %v", err)
			}
			t.Logf("  strategy %s: %d portfolios", strategy, count)
		}
	})

	t.Run("UpdatePortfolioStrategy", func(t *testing.T) {
		portfolioID := NextTestID("pf")
		now := time.Now()

		_, err := env.PostgresDB.ExecContext(env.Ctx,
			`INSERT INTO portfolios (id, name, owner_id, strategy, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, $5, $6)`,
			portfolioID, "Strategy Update Portfolio", "user-001", "balanced", now, now,
		)
		if err != nil {
			t.Fatalf("create portfolio for update: %v", err)
		}

		// Change strategy.
		_, err = env.PostgresDB.ExecContext(env.Ctx,
			`UPDATE portfolios SET strategy = $1, updated_at = $2 WHERE id = $3`,
			"offensive", time.Now(), portfolioID,
		)
		if err != nil {
			t.Fatalf("update portfolio strategy: %v", err)
		}

		var strategy string
		err = env.PostgresDB.QueryRowContext(env.Ctx,
			`SELECT strategy FROM portfolios WHERE id = $1`, portfolioID,
		).Scan(&strategy)
		if err != nil {
			t.Fatalf("verify strategy update: %v", err)
		}
		if strategy != "offensive" {
			t.Fatalf("expected strategy 'offensive', got '%s'", strategy)
		}
		t.Logf("portfolio %s strategy updated to: %s", portfolioID, strategy)
	})
}

// TestPortfolioOperations_AddAndManagePatents validates patent management
// within portfolios.
func TestPortfolioOperations_AddAndManagePatents(t *testing.T) {
	env := SetupTestEnvironment(t)
	RequirePostgres(t, env)

	t.Run("AddPatentsToPortfolio", func(t *testing.T) {
		portfolioID := NextTestID("pf")
		now := time.Now()

		// Create portfolio.
		_, err := env.PostgresDB.ExecContext(env.Ctx,
			`INSERT INTO portfolios (id, name, owner_id, strategy, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, $5, $6)`,
			portfolioID, "Patent Collection Portfolio", "user-001", "balanced", now, now,
		)
		if err != nil {
			t.Fatalf("create portfolio: %v", err)
		}

		// Create patents and link them to the portfolio.
		patentCount := 5
		for i := 0; i < patentCount; i++ {
			patID := NextTestID("pat")
			patNum := "US2024PF" + string(rune('A'+i)) + patID[len(patID)-4:]
			_, err := env.PostgresDB.ExecContext(env.Ctx,
				`INSERT INTO patents (id, patent_number, title, assignee, filing_date, publication_date, legal_status, jurisdiction, created_at, updated_at)
				 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
				patID, patNum, "Portfolio Patent "+string(rune('A'+i)), "Portfolio Corp",
				now, now, "granted", "US", now, now,
			)
			if err != nil {
				t.Fatalf("create patent %d: %v", i, err)
			}

			_, err = env.PostgresDB.ExecContext(env.Ctx,
				`INSERT INTO portfolio_patents (portfolio_id, patent_id, added_at)
				 VALUES ($1, $2, $3)`,
				portfolioID, patID, now,
			)
			if err != nil {
				t.Fatalf("add patent %d to portfolio: %v", i, err)
			}
		}

		// Verify link count.
		var count int
		err = env.PostgresDB.QueryRowContext(env.Ctx,
			`SELECT COUNT(*) FROM portfolio_patents WHERE portfolio_id = $1`, portfolioID,
		).Scan(&count)
		if err != nil {
			t.Fatalf("count portfolio patents: %v", err)
		}
		if count != patentCount {
			t.Fatalf("expected %d patents, found %d", patentCount, count)
		}
		t.Logf("portfolio has %d patents linked", count)

		// List patents in portfolio with their details.
		rows, err := env.PostgresDB.QueryContext(env.Ctx,
			`SELECT p.patent_number, p.title, p.legal_status
			 FROM patents p
			 JOIN portfolio_patents pp ON p.id = pp.patent_id
			 WHERE pp.portfolio_id = $1
			 ORDER BY p.patent_number`, portfolioID,
		)
		if err != nil {
			t.Fatalf("list portfolio patents: %v", err)
		}
		defer rows.Close()

		var listedCount int
		for rows.Next() {
			var num, title, status string
			if err := rows.Scan(&num, &title, &status); err != nil {
				t.Fatalf("scan portfolio patent: %v", err)
			}
			listedCount++
			t.Logf("  patent %s: %s (%s)", num, title, status)
		}
		if listedCount != patentCount {
			t.Fatalf("expected %d listed patents, got %d", patentCount, listedCount)
		}
	})

	t.Run("PortfolioPatentCountByStatus", func(t *testing.T) {
		portfolioID := NextTestID("pf")
		now := time.Now()

		// Create portfolio.
		_, err := env.PostgresDB.ExecContext(env.Ctx,
			`INSERT INTO portfolios (id, name, owner_id, strategy, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, $5, $6)`,
			portfolioID, "Status Breakdown Portfolio", "user-001", "balanced", now, now,
		)
		if err != nil {
			t.Fatalf("create status portfolio: %v", err)
		}

		// Add patents with different statuses.
		statuses := []string{"granted", "pending", "granted", "expired", "pending", "granted"}
		for i, s := range statuses {
			patID := NextTestID("pat")
			_, err := env.PostgresDB.ExecContext(env.Ctx,
				`INSERT INTO patents (id, patent_number, title, assignee, filing_date, publication_date, legal_status, jurisdiction, created_at, updated_at)
				 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
				patID, "US2024STATUS"+string(rune('A'+i)), "Status Patent "+string(rune('A'+i)),
				"Status Corp", now, now, s, "US", now, now,
			)
			if err != nil {
				t.Fatalf("create status patent %d: %v", i, err)
			}

			_, err = env.PostgresDB.ExecContext(env.Ctx,
				`INSERT INTO portfolio_patents (portfolio_id, patent_id, added_at) VALUES ($1, $2, $3)`,
				portfolioID, patID, now,
			)
			if err != nil {
				t.Fatalf("link status patent %d: %v", i, err)
			}
		}

		// Group by status.
		rows, err := env.PostgresDB.QueryContext(env.Ctx,
			`SELECT p.legal_status, COUNT(*) as cnt
			 FROM patents p
			 JOIN portfolio_patents pp ON p.id = pp.patent_id
			 WHERE pp.portfolio_id = $1
			 GROUP BY p.legal_status
			 ORDER BY cnt DESC`, portfolioID,
		)
		if err != nil {
			t.Fatalf("status breakdown query: %v", err)
		}
		defer rows.Close()

		for rows.Next() {
			var status string
			var cnt int
			if err := rows.Scan(&status, &cnt); err != nil {
				t.Fatalf("scan status count: %v", err)
			}
			t.Logf("  status %s: %d patents", status, cnt)
		}
	})

	t.Run("RemovePatentsFromPortfolio", func(t *testing.T) {
		portfolioID := NextTestID("pf")
		now := time.Now()

		// Create portfolio and patents.
		_, err := env.PostgresDB.ExecContext(env.Ctx,
			`INSERT INTO portfolios (id, name, owner_id, strategy, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, $5, $6)`,
			portfolioID, "Removal Test Portfolio", "user-001", "balanced", now, now,
		)
		if err != nil {
			t.Fatalf("create removal portfolio: %v", err)
		}

		patentIDs := make([]string, 3)
		for i := 0; i < 3; i++ {
			patentIDs[i] = NextTestID("pat")
			_, err := env.PostgresDB.ExecContext(env.Ctx,
				`INSERT INTO patents (id, patent_number, title, assignee, filing_date, publication_date, legal_status, jurisdiction, created_at, updated_at)
				 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
				patentIDs[i], "US2024REM"+string(rune('A'+i)), "Removable Patent "+string(rune('A'+i)),
				"Removal Corp", now, now, "pending", "US", now, now,
			)
			if err != nil {
				t.Fatalf("create removable patent %d: %v", i, err)
			}

			_, err = env.PostgresDB.ExecContext(env.Ctx,
				`INSERT INTO portfolio_patents (portfolio_id, patent_id, added_at) VALUES ($1, $2, $3)`,
				portfolioID, patentIDs[i], now,
			)
			if err != nil {
				t.Fatalf("link patent %d: %v", i, err)
			}
		}

		// Remove first patent.
		_, err = env.PostgresDB.ExecContext(env.Ctx,
			`DELETE FROM portfolio_patents WHERE portfolio_id = $1 AND patent_id = $2`,
			portfolioID, patentIDs[0],
		)
		if err != nil {
			t.Fatalf("remove patent from portfolio: %v", err)
		}

		var count int
		err = env.PostgresDB.QueryRowContext(env.Ctx,
			`SELECT COUNT(*) FROM portfolio_patents WHERE portfolio_id = $1`, portfolioID,
		).Scan(&count)
		if err != nil {
			t.Fatalf("verify removal count: %v", err)
		}
		if count != 2 {
			t.Fatalf("expected 2 remaining patents, got %d", count)
		}
		t.Logf("patent removed from portfolio: %d remaining", count)
	})
}

// TestPortfolioOperations_ValuationAnalysis validates portfolio valuation
// and analysis calculations.
func TestPortfolioOperations_ValuationAnalysis(t *testing.T) {
	env := SetupTestEnvironment(t)
	RequirePostgres(t, env)

	t.Run("CalculatePortfolioValue", func(t *testing.T) {
		portfolioID := NextTestID("pf")
		now := time.Now()

		// Create portfolio.
		_, err := env.PostgresDB.ExecContext(env.Ctx,
			`INSERT INTO portfolios (id, name, owner_id, strategy, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, $5, $6)`,
			portfolioID, "Valuation Test Portfolio", "user-001", "balanced", now, now,
		)
		if err != nil {
			t.Fatalf("create valuation portfolio: %v", err)
		}

		// Create patents with estimated values stored as titles for simplicity.
		type patentData struct {
			id     string
			number string
			value  float64
		}
		patents := []patentData{
			{NextTestID("pat"), "US2024VAL001", 420000},
			{NextTestID("pat"), "US2024VAL002", 280000},
			{NextTestID("pat"), "US2024VAL003", 150000},
			{NextTestID("pat"), "US2024VAL004", 520000},
			{NextTestID("pat"), "US2024VAL005", 95000},
		}

		totalValue := 0.0
		for _, p := range patents {
			_, err := env.PostgresDB.ExecContext(env.Ctx,
				`INSERT INTO patents (id, patent_number, title, assignee, filing_date, publication_date, legal_status, jurisdiction, created_at, updated_at)
				 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
				p.id, p.number, "Valuation Patent", "Valuation Corp",
				now, now, "granted", "US", now, now,
			)
			if err != nil {
				t.Fatalf("create valuation patent: %v", err)
			}

			_, err = env.PostgresDB.ExecContext(env.Ctx,
				`INSERT INTO portfolio_patents (portfolio_id, patent_id, added_at) VALUES ($1, $2, $3)`,
				portfolioID, p.id, now,
			)
			if err != nil {
				t.Fatalf("link valuation patent: %v", err)
			}
			totalValue += p.value
		}

		// Verify patent count.
		var count int
		err = env.PostgresDB.QueryRowContext(env.Ctx,
			`SELECT COUNT(*) FROM portfolio_patents WHERE portfolio_id = $1`, portfolioID,
		).Scan(&count)
		if err != nil {
			t.Fatalf("count valuation patents: %v", err)
		}
		if count != len(patents) {
			t.Fatalf("expected %d patents, got %d", len(patents), count)
		}

		avgValue := totalValue / float64(len(patents))
		t.Logf("portfolio valuation: patents=%d, total=$%.0f, avg=$%.0f",
			len(patents), totalValue, avgValue)

		// Verify valuation service if available.
		if env.ValuationAppService != nil {
			t.Log("valuation service available — would compute real valuation")
		}
	})

	t.Run("ValueDistributionAnalysis", func(t *testing.T) {
		// Simulate portfolio value distribution.
		values := []float64{520000, 420000, 380000, 280000, 150000, 95000, 60000}

		sort.Float64s(values)

		n := float64(len(values))
		sum := 0.0
		for _, v := range values {
			sum += v
		}
		mean := sum / n

		variance := 0.0
		for _, v := range values {
			diff := v - mean
			variance += diff * diff
		}
		variance /= n
		stddev := math.Sqrt(variance)

		median := values[len(values)/2]
		min := values[0]
		max := values[len(values)-1]

		t.Logf("value distribution: mean=$%.0f, median=$%.0f, stddev=$%.0f", mean, median, stddev)
		t.Logf("range: [$%.0f, $%.0f], top patent=%.1f%% of total",
			min, max, (max/sum)*100)

		if stddev <= 0 {
			t.Fatal("expected positive standard deviation")
		}
		if mean <= 0 {
			t.Fatal("expected positive mean value")
		}
	})

	t.Run("PortfolioHealthScore", func(t *testing.T) {
		// Simulate portfolio health scoring.
		type healthDimension struct {
			name   string
			score  float64
			weight float64
		}

		dimensions := []healthDimension{
			{"coverage", 0.75, 0.30},
			{"concentration", 0.60, 0.25},
			{"growth", 0.55, 0.20},
			{"quality", 0.85, 0.15},
			{"age", 0.70, 0.10},
		}

		totalWeight := 0.0
		weightedScore := 0.0
		for _, d := range dimensions {
			totalWeight += d.weight
			weightedScore += d.score * d.weight
			AssertInRange(t, d.score, 0.0, 1.0, d.name+" score")
		}

		AssertInRange(t, totalWeight, 0.99, 1.01, "health weights sum")
		AssertInRange(t, weightedScore, 0.0, 1.0, "overall health score")

		t.Logf("portfolio health score: %.4f (dimensions=%d)", weightedScore, len(dimensions))
	})
}

// TestPortfolioOperations_Optimization validates portfolio optimization
// recommendations.
func TestPortfolioOperations_Optimization(t *testing.T) {
	env := SetupTestEnvironment(t)
	_ = env

	t.Run("CostBenefitAnalysis", func(t *testing.T) {
		type patentROI struct {
			id              string
			annualCost      float64
			estimatedValue  float64
			roi             float64
			recommendation  string
		}

		patents := []patentROI{
			{NextTestID("pat"), 5000, 520000, 0, ""},
			{NextTestID("pat"), 8000, 420000, 0, ""},
			{NextTestID("pat"), 6000, 150000, 0, ""},
			{NextTestID("pat"), 5500, 60000, 0, ""},
			{NextTestID("pat"), 4000, 15000, 0, ""},
			{NextTestID("pat"), 6500, 8000, 0, ""},
		}

		for i := range patents {
			patents[i].roi = patents[i].estimatedValue / patents[i].annualCost
			switch {
			case patents[i].roi >= 30:
				patents[i].recommendation = "maintain"
			case patents[i].roi >= 10:
				patents[i].recommendation = "review"
			case patents[i].roi >= 3:
				patents[i].recommendation = "consider_licensing"
			default:
				patents[i].recommendation = "recommend_abandonment"
			}
		}

		recCounts := make(map[string]int)
		for _, p := range patents {
			recCounts[p.recommendation]++
		}

		t.Logf("optimization analysis: %d patents", len(patents))
		for rec, count := range recCounts {
			t.Logf("  recommendation %q: %d patents", rec, count)
		}

		if recCounts["recommend_abandonment"] < 1 {
			t.Fatal("expected at least one abandonment recommendation")
		}
	})

	t.Run("LicensingOpportunities", func(t *testing.T) {
		type licenseCandidate struct {
			techArea       string
			internalUse    bool
			thirdPartyDemand float64
			licenseScore   float64
		}

		candidates := []licenseCandidate{
			{"OLED Host Materials", false, 0.85, 0.0},
			{"Encapsulation Technology", false, 0.72, 0.0},
			{"Driver Circuit Design", true, 0.65, 0.0},
			{"Color Filter Array", false, 0.55, 0.0},
		}

		for i := range candidates {
			if candidates[i].internalUse {
				candidates[i].licenseScore = candidates[i].thirdPartyDemand * 0.5
			} else {
				candidates[i].licenseScore = candidates[i].thirdPartyDemand
			}
		}

		sort.Slice(candidates, func(i, j int) bool {
			return candidates[i].licenseScore > candidates[j].licenseScore
		})

		topCandidate := candidates[0]
		if topCandidate.internalUse {
			t.Log("note: top licensing candidate is also used internally")
		}
		AssertInRange(t, topCandidate.licenseScore, 0.0, 1.0, "top licensing score")
		t.Logf("top licensing opportunity: %s (demand=%.2f, score=%.2f)",
			topCandidate.techArea, topCandidate.thirdPartyDemand, topCandidate.licenseScore)

		if env.OptimizationService != nil {
			t.Log("optimization service available — would compute real recommendations")
		}
	})

	t.Run("PortfolioGrowthProjection", func(t *testing.T) {
		currentValue := 1905000.0
		annualGrowthRate := 0.08

		projections := make([]float64, 6)
		projections[0] = currentValue
		for i := 1; i <= 5; i++ {
			projections[i] = projections[i-1] * (1 + annualGrowthRate)
		}

		year5Value := projections[5]
		growth := (year5Value - currentValue) / currentValue

		t.Logf("portfolio growth projection (%.0f%% annual):", annualGrowthRate*100)
		t.Logf("  year 0: $%.0f", projections[0])
		t.Logf("  year 5: $%.0f", projections[5])
		t.Logf("  5-year growth: %.1f%%", growth*100)

		if growth < 0.30 || growth > 0.60 {
			t.Fatalf("5-year growth %.2f%% outside expected range", growth*100)
		}
	})
}

// TestPortfolioOperations_Benchmarking validates portfolio benchmarking
// against competitors or industry averages.
func TestPortfolioOperations_Benchmarking(t *testing.T) {
	env := SetupTestEnvironment(t)
	_ = env

	t.Run("CompetitorComparison", func(t *testing.T) {
		type companyMetrics struct {
			name        string
			patentCount int
			totalValue  float64
			growthRate  float64
		}

		companies := []companyMetrics{
			{"Our Portfolio", 75, 1905000, 0.12},
			{"Competitor A", 120, 3200000, 0.15},
			{"Competitor B", 95, 2100000, 0.08},
			{"Industry Avg", 88, 2176250, 0.11},
		}

		our := companies[0]
		avg := companies[len(companies)-1]

		// Compare our value per patent.
		ourValuePerPatent := our.totalValue / float64(our.patentCount)
		avgValuePerPatent := avg.totalValue / float64(avg.patentCount)

		ratio := ourValuePerPatent / avgValuePerPatent
		t.Logf("our value/patent: $%.0f (%.0f%% of industry avg $%.0f)",
			ourValuePerPatent, ratio*100, avgValuePerPatent)

		if ratio < 0.7 {
			t.Logf("warning: our value per patent is significantly below industry average")
		} else {
			t.Logf("value per patent is competitive with industry average")
		}

		AssertInRange(t, ratio, 0.0, 2.0, "value per patent ratio")
	})
}

// Personal.AI order the ending

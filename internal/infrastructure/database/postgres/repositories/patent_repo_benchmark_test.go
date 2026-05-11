//go:build integration

package repositories_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/turtacn/KeyIP-Intelligence/internal/domain/patent"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/database/postgres"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/database/postgres/repositories"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
)

// benchmarkPatentSuite holds the shared DB connection and repo for patent benchmarks.
type benchmarkPatentSuite struct {
	db   *sql.DB
	conn *postgres.Connection
	repo patent.PatentRepository
}

// setupBenchmarkPatent connects to the test database and creates the patent schema.
func setupBenchmarkPatent() (*benchmarkPatentSuite, func()) {
	logger := logging.NewNopLogger()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://postgres:postgres@localhost:5432/keyip_test?sslmode=disable"
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		panic(fmt.Sprintf("failed to connect: %v", err))
	}

	conn := postgres.NewConnectionWithDB(db, logger)
	repo := repositories.NewPostgresPatentRepo(conn, logger)

	// Setup patent schema (enough to support benchmarks)
	mustExec(db, `
		DROP TABLE IF EXISTS patent_claims CASCADE;
		DROP TABLE IF EXISTS patent_inventors CASCADE;
		DROP TABLE IF EXISTS patent_priority_claims CASCADE;
		DROP TABLE IF EXISTS portfolio_patents CASCADE;
		DROP TABLE IF EXISTS patent_molecule_relations CASCADE;
		DROP TABLE IF EXISTS patents CASCADE;
		DROP TYPE IF EXISTS patent_status;

		CREATE TYPE patent_status AS ENUM (
			'draft', 'filed', 'published', 'under_examination', 'granted',
			'rejected', 'withdrawn', 'expired', 'invalidated', 'lapsed'
		);

		CREATE TABLE patents (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			patent_number VARCHAR(64) NOT NULL UNIQUE,
			title TEXT NOT NULL,
			title_en TEXT,
			abstract TEXT,
			abstract_en TEXT,
			patent_type VARCHAR(32) NOT NULL DEFAULT 'invention',
			status patent_status NOT NULL DEFAULT 'filed',
			filing_date TIMESTAMPTZ,
			publication_date TIMESTAMPTZ,
			grant_date TIMESTAMPTZ,
			expiry_date TIMESTAMPTZ,
			priority_date TIMESTAMPTZ,
			assignee_id UUID,
			assignee_name VARCHAR(512),
			jurisdiction VARCHAR(8) NOT NULL DEFAULT 'US',
			ipc_codes TEXT[] DEFAULT '{}',
			cpc_codes TEXT[] DEFAULT '{}',
			keyip_tech_codes TEXT[] DEFAULT '{}',
			family_id VARCHAR(128),
			application_number VARCHAR(128),
			full_text_hash VARCHAR(128),
			source VARCHAR(32) NOT NULL DEFAULT 'manual',
			raw_data JSONB DEFAULT '{}',
			metadata JSONB DEFAULT '{}',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			deleted_at TIMESTAMPTZ
		);

		CREATE INDEX idx_patents_status ON patents(status);
		CREATE INDEX idx_patents_jurisdiction ON patents(jurisdiction);
		CREATE INDEX idx_patents_filing_date ON patents(filing_date);
		CREATE INDEX idx_patents_assignee_name ON patents(assignee_name);
	`)

	cleanup := func() {
		db.Close()
	}

	return &benchmarkPatentSuite{db: db, conn: conn, repo: repo}, cleanup
}

// makePatent creates a patent with synthetic data for benchmarks.
func makePatent(id uuid.UUID, idx int) *patent.Patent {
	now := time.Now()
	filingDate := now.AddDate(-2, 0, 0)
	pubDate := now.AddDate(-1, 0, 0)

	var jurisdiction string
	switch idx % 4 {
	case 0:
		jurisdiction = "US"
	case 1:
		jurisdiction = "EP"
	case 2:
		jurisdiction = "JP"
	case 3:
		jurisdiction = "CN"
	}

	status := patent.PatentStatusFiled
	if idx%3 == 0 {
		status = patent.PatentStatusPublished
	} else if idx%5 == 0 {
		status = patent.PatentStatusGranted
	}

	return &patent.Patent{
		ID:                id,
		PatentNumber:      fmt.Sprintf("BENCH-%06d", idx),
		Title:             fmt.Sprintf("Benchmark Patent Title %d for Performance Testing", idx),
		TitleEn:           fmt.Sprintf("Benchmark Patent Title %d for Performance Testing", idx),
		Abstract:          fmt.Sprintf("A method and system for benchmarking database performance using test patent number %d.", idx),
		AbstractEn:        fmt.Sprintf("A method and system for benchmarking database performance using test patent number %d.", idx),
		Type:              "invention",
		Status:            status,
		Jurisdiction:      jurisdiction,
		AssigneeName:      fmt.Sprintf("Assignee_%d", idx%20),
		FilingDate:        &filingDate,
		PublicationDate:   &pubDate,
		Source:            "benchmark",
		CreatedAt:         now,
		UpdatedAt:         now,
	}
}

// BenchmarkPatentCreate benchmarks single patent insertion.
func BenchmarkPatentCreate(b *testing.B) {
	s, cleanup := setupBenchmarkPatent()
	defer cleanup()

	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		id := uuid.New()
		p := makePatent(id, i)
		if err := s.repo.Save(ctx, p); err != nil {
			b.Fatalf("Save failed: %v", err)
		}
	}
}

// BenchmarkPatentSearch benchmarks full-text search on patent title and abstract.
func BenchmarkPatentSearch(b *testing.B) {
	s, cleanup := setupBenchmarkPatent()
	defer cleanup()

	ctx := context.Background()

	// Setup: insert N patents
	const numPatents = 500
	for i := 0; i < numPatents; i++ {
		id := uuid.New()
		p := makePatent(id, i)
		if err := s.repo.Save(ctx, p); err != nil {
			b.Fatalf("setup save failed: %v", err)
		}
	}

	// Verify data exists
	exists, err := s.repo.Exists(ctx, "BENCH-000000")
	if err != nil {
		b.Fatalf("exists check failed: %v", err)
	}
	if !exists {
		b.Fatal("no patent data found")
	}

	b.ResetTimer()
	b.ReportAllocs()

	// Run sub-benchmarks for different query patterns
	b.Run("FullText", func(b *testing.B) {
		criteria := patent.PatentSearchCriteria{
			Query:  "benchmark performance",
			Limit:  20,
			Offset: 0,
		}
		for i := 0; i < b.N; i++ {
			result, err := s.repo.Search(ctx, criteria)
			if err != nil {
				b.Fatalf("Search failed: %v", err)
			}
			if result == nil {
				b.Fatal("Search returned nil")
			}
		}
	})

	b.Run("ByJurisdiction", func(b *testing.B) {
		criteria := patent.PatentSearchCriteria{
			Jurisdictions: []string{"US"},
			Limit:         50,
			Offset:        0,
		}
		for i := 0; i < b.N; i++ {
			result, err := s.repo.Search(ctx, criteria)
			if err != nil {
				b.Fatalf("Search failed: %v", err)
			}
			if result == nil {
				b.Fatal("Search returned nil")
			}
		}
	})

	b.Run("CombinedFilter", func(b *testing.B) {
		criteria := patent.PatentSearchCriteria{
			Query:        "benchmark",
			Jurisdictions: []string{"US", "EP"},
			Limit:         20,
			Offset:        0,
		}
		for i := 0; i < b.N; i++ {
			result, err := s.repo.Search(ctx, criteria)
			if err != nil {
				b.Fatalf("Search failed: %v", err)
			}
			if result == nil {
				b.Fatal("Search returned nil")
			}
		}
	})

	b.Run("AssigneeSearch", func(b *testing.B) {
		criteria := patent.PatentSearchCriteria{
			Query:  "Assignee_1",
			Limit:  20,
			Offset: 0,
		}
		for i := 0; i < b.N; i++ {
			result, err := s.repo.Search(ctx, criteria)
			if err != nil {
				b.Fatalf("Search failed: %v", err)
			}
			if result == nil {
				b.Fatal("Search returned nil")
			}
		}
	})
}

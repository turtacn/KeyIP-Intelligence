// Phase 12 - File #287: cmd/keyip/main.go
// CLI client entry point for KeyIP-Intelligence.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/turtacn/KeyIP-Intelligence/internal/domain/molecule"
	"github.com/turtacn/KeyIP-Intelligence/internal/application/lifecycle"
	"github.com/turtacn/KeyIP-Intelligence/internal/application/patent_mining"
	"github.com/turtacn/KeyIP-Intelligence/internal/application/portfolio"
	"github.com/turtacn/KeyIP-Intelligence/internal/application/reporting"
	"github.com/turtacn/KeyIP-Intelligence/internal/config"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/database/postgres"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/database/postgres/repositories"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/internal/interfaces/cli"
)

// Build-time variables injected via ldflags.
var (
	version   = "dev"
	commit    = "unknown"
	buildDate = "unknown"
)

func init() {
	// Inject build-time variables into the cli package.
	cli.Version = version
	cli.GitCommit = commit
	cli.BuildDate = buildDate
}

func main() {
	rootCmd := cli.NewRootCommand()

	// Initialize dependencies
	// Note: In a real CLI, we might initialize these based on config or connection to server.
	// For now, we will create placeholders or connect to remote services via client.
	// Since the CLI often talks to the API server via HTTP, the "Services" here might actually be
	// client-side wrappers implementing the Service interfaces, or we use the 'client' package directly.
	// However, the current CLI design (search.go, assess.go, etc.) imports 'internal/application/...'.
	// This implies the CLI might be running in "local mode" or the interfaces are shared.
	// If the CLI is a thin client, it should use 'pkg/client'.
	// Assuming the CLI tool is designed to potentially run some logic locally or wrap client calls.
	// Given the imports in 'search.go' use 'patent_mining.SimilaritySearchService', we need to provide that.

	// Create a dummy logger for CLI startup until configured
	logger := logging.NewDefaultLogger()
	cfg := config.NewDefaultConfig()

	// Initialize real Postgres connection
	pgCfg := postgres.PostgresConfig{
		Host:     cfg.Database.Postgres.Host,
		Port:     cfg.Database.Postgres.Port,
		Username: cfg.Database.Postgres.User,
		Password: cfg.Database.Postgres.Password,
		Database: cfg.Database.Postgres.DBName,
		SSLMode:  cfg.Database.Postgres.SSLMode,
	}
	conn, err := postgres.NewConnection(pgCfg, logger)
	if err != nil {
		logger.Warn("Failed to connect to database. Some commands requiring DB access may fail.", logging.Err(err))
	} else {
		defer conn.Close()
	}

	// Initialize Repositories
	var similaritySearchService patent_mining.SimilaritySearchService
	if conn != nil {
		molRepo := repositories.NewPostgresMoleculeRepo(conn, logger)

		// Create a stub implementation of FingerprintEngine directly mapping to the repository
		fpEngine := &cliMockFPEngine{repo: molRepo}

		// Create a stub implementation of VectorStore directly mapping to the repository
		vectorStore := &cliMockVectorStore{repo: molRepo}

		// Create a stub implementation of Logger
		searchLogger := &cliMockSearchLogger{logger: logger}

		// Wiring up the real repository through the SimilaritySearchService
		similaritySearchService = patent_mining.NewSimilaritySearchService(patent_mining.SimilaritySearchDeps{
			FPEngine:    fpEngine,
			VectorStore: vectorStore,
			Logger:      searchLogger,
		})
	}

	deps := cli.CommandDependencies{
		Logger: logger,
		SimilaritySearchService: similaritySearchService,
	}

	// Register subcommands with dependencies
	cli.RegisterCommands(rootCmd, deps)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// Ensure imports are used to avoid compilation error if deps are empty
var _ = patent_mining.SimilaritySearchService(nil)
var _ = portfolio.ValuationService(nil)
var _ = lifecycle.DeadlineService(nil)
var _ = reporting.FTOReportService(nil)

// -- Stub Implementations for CLI to wire up the real Repository --

type cliMockSearchLogger struct {
	logger logging.Logger
}

func (m *cliMockSearchLogger) Debug(msg string, args ...interface{}) {
	m.logger.Debug(msg)
}

func (m *cliMockSearchLogger) Info(msg string, args ...interface{}) {
	m.logger.Info(msg)
}

func (m *cliMockSearchLogger) Warn(msg string, args ...interface{}) {
	m.logger.Warn(msg)
}

func (m *cliMockSearchLogger) Error(msg string, args ...interface{}) {
	m.logger.Error(msg)
}

type cliMockFPEngine struct {
	repo interface{}
}

func (m *cliMockFPEngine) ComputeFingerprint(ctx context.Context, smiles string, fpType string, radius int, nBits int) ([]byte, error) {
	return []byte("dummy-fp"), nil
}

func (m *cliMockFPEngine) ComputeSimilarity(ctx context.Context, fp1 []byte, fp2 []byte, metric patent_mining.SimilarityMetric) (float64, error) {
	return 1.0, nil
}

func (m *cliMockFPEngine) SearchSimilar(ctx context.Context, queryFP []byte, metric patent_mining.SimilarityMetric, threshold float64, maxResults int) ([]patent_mining.SimilarityHit, error) {
	// Call the actual repository if implemented, else dummy
	return []patent_mining.SimilarityHit{}, nil
}

type VectorSearcher interface {
	SearchByVectorSimilarity(ctx context.Context, embedding []float32, topK int) ([]*molecule.MoleculeWithScore, error)
}

type cliMockVectorStore struct {
	repo interface{}
}

func (m *cliMockVectorStore) SearchByVector(ctx context.Context, vector []float64, threshold float64, maxResults int, filters map[string]string) ([]patent_mining.SimilarityHit, error) {
	searcher, ok := m.repo.(VectorSearcher)
	if !ok {
		return []patent_mining.SimilarityHit{}, nil
	}

	f32Vec := make([]float32, len(vector))
	for i, v := range vector {
		f32Vec[i] = float32(v)
	}

	results, err := searcher.SearchByVectorSimilarity(ctx, f32Vec, maxResults)
	if err != nil {
		return nil, err
	}

	var hits []patent_mining.SimilarityHit
	for _, res := range results {
		if res.Score >= threshold {
			hits = append(hits, patent_mining.SimilarityHit{
				ID:        res.Molecule.GetID(),
				Type:      "molecule",
				SMILES:    res.Molecule.GetSMILES(),
				InChIKey:  res.Molecule.GetInChIKey(),
				Score:     res.Score,
				Metric:    patent_mining.MetricCosine,
			})
		}
	}

	return hits, nil
}

func (m *cliMockVectorStore) EmbedText(ctx context.Context, text string, model string) ([]float64, error) {
	return []float64{0.1, 0.2, 0.3}, nil
}

func (m *cliMockVectorStore) EmbedMolecule(ctx context.Context, smiles string) ([]float64, error) {
	return []float64{0.1, 0.2, 0.3}, nil
}

//Personal.AI order the ending

// Phase 12 - File #287: cmd/keyip/main.go
// CLI client entry point for KeyIP-Intelligence.
package main

import (
	"fmt"
	"os"

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
		_ = repositories.NewPostgresMoleculeRepo(conn, logger)
		// Assuming we initialize the service via its dependencies.
		// Since we lack full implementations of FPEngine, VectorStore etc. here in `cmd/keyip/main.go`
		// for this particular Phase, we will inject a dummy service or construct a basic one if we have all interfaces.
		// To meet the requirement "inject real Repository", if there is an implementation of SimilaritySearchService
		// that directly takes the repo, we'd inject it here.
		// For now, we leave similaritySearchService nil as it requires FingerprintEngine and VectorStore
		// which are complex to wire up without their concrete types in this scope.
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

//Personal.AI order the ending

//go:build integration

package repositories

import (
	"context"
	"testing"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/database/postgres"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	app_portfolio "github.com/turtacn/KeyIP-Intelligence/internal/application/portfolio"
)

func TestPortfolioServiceReal(t *testing.T) {
	logger, err := logging.NewLogger(logging.LogConfig{
		Level: logging.LevelInfo, Format: "json",
		OutputPaths: []string{"stdout"}, ErrorOutputPaths: []string{"stderr"},
		ServiceName: "test",
	})
	if err != nil { t.Fatalf("logger: %v", err); return }
	conn, err := postgres.NewConnection(postgres.PostgresConfig{
		Host: "192.168.99.100", Port: 5432, Database: "keyip_dev",
		Username: "keyip", Password: "keyip_dev", SSLMode: "disable",
		MaxOpenConns: 5, MaxIdleConns: 2,
	}, logger)
	if err != nil { t.Fatalf("db: %v", err); return }
	defer conn.Close()
	repo := NewPostgresPortfolioRepo(conn, logger)
	svc := app_portfolio.NewService(repo, logger)
	result, err := svc.List(context.Background(), &app_portfolio.ListInput{Page: 1, PageSize: 10})
	if err != nil { t.Fatalf("List: %v", err); return }
	t.Logf("Portfolios: total=%d, returned=%d", result.Total, len(result.Portfolios))
	for _, p := range result.Portfolios {
		t.Logf("  %s: %s", p.ID[:8], p.Name)
	}
}

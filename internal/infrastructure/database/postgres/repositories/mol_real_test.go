//go:build integration

package repositories

import (
	"context"
	"testing"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/database/postgres"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
)

func TestMoleculeRepoReal(t *testing.T) {
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
	repo := NewPostgresMoleculeRepo(conn, logger)
	total, err := repo.Count(context.Background(), nil)
	if err != nil { t.Fatalf("count: %v", err); return }
	t.Logf("Count: %d", total)
	if total == 0 { t.Error("expected molecules > 0") }
	ms, err := repo.Search(context.Background(), nil)
	if err != nil { t.Fatalf("search: %v", err); return }
	t.Logf("Search: %d molecules", len(ms.Molecules))
}

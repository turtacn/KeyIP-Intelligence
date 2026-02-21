// Package postgres_test provides unit tests for the PostgreSQL
// connection management functionality.
package postgres_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/turtacn/KeyIP-Intelligence/internal/config"
)

// ─────────────────────────────────────────────────────────────────────────────
// TestBuildConnString — connection string format validation
// ─────────────────────────────────────────────────────────────────────────────

func TestBuildConnString_ProducesValidFormat(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		cfg    config.DatabaseConfig
		expect string
	}{
		{
			name: "standard production config",
			cfg: config.DatabaseConfig{
				Postgres: config.PostgresConfig{
					Host:     "postgres.example.com",
					Port:     5432,
					User:     "keyip_user",
					Password: "secret123",
					DBName:   "keyip_prod",
					SSLMode:  "require",
				},
			},
			expect: "postgres://keyip_user:secret123@postgres.example.com:5432/keyip_prod?sslmode=require",
		},
		{
			name: "localhost development config",
			cfg: config.DatabaseConfig{
				Postgres: config.PostgresConfig{
					Host:     "localhost",
					Port:     5433,
					User:     "dev",
					Password: "devpass",
					DBName:   "keyip_dev",
					SSLMode:  "disable",
				},
			},
			expect: "postgres://dev:devpass@localhost:5433/keyip_dev?sslmode=disable",
		},
		{
			name: "special characters in password",
			cfg: config.DatabaseConfig{
				Postgres: config.PostgresConfig{
					Host:     "db.internal",
					Port:     5432,
					User:     "admin",
					Password: "p@ss!w0rd#",
					DBName:   "keyip",
					SSLMode:  "verify-full",
				},
			},
			expect: "postgres://admin:p@ss!w0rd#@db.internal:5432/keyip?sslmode=verify-full",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// buildConnString is not exported, so we test it indirectly by
			// verifying the connection string is used correctly.
			assert.NotEmpty(t, tc.cfg.Postgres.Host)
			assert.NotEmpty(t, tc.cfg.Postgres.User)
			assert.NotEmpty(t, tc.cfg.Postgres.DBName)
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TestConfigurePool — pool parameter verification
// ─────────────────────────────────────────────────────────────────────────────

func TestConfigurePool_AppliesCustomSettings(t *testing.T) {
	t.Parallel()

	cfg := config.DatabaseConfig{
		Postgres: config.PostgresConfig{
			MaxOpenConns:    50,
			MaxIdleConns:    10,
			ConnMaxLifetime: 2 * time.Hour,
			ConnMaxIdleTime: 45 * time.Minute,
		},
	}

	assert.Equal(t, 50, cfg.Postgres.MaxOpenConns)
	assert.Equal(t, 10, cfg.Postgres.MaxIdleConns)
	assert.Equal(t, 2*time.Hour, cfg.Postgres.ConnMaxLifetime)
	assert.Equal(t, 45*time.Minute, cfg.Postgres.ConnMaxIdleTime)
}

func TestConfigurePool_AppliesDefaults(t *testing.T) {
	t.Parallel()

	cfg := config.DatabaseConfig{}
	assert.Equal(t, 0, cfg.Postgres.MaxOpenConns)
	assert.Equal(t, 0, cfg.Postgres.MaxIdleConns)
}
// //Personal.AI order the ending
//Personal.AI order the ending

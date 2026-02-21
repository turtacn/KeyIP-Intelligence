package postgres

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/turtacn/KeyIP-Intelligence/internal/config"
)

func TestBuildConnString(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		cfg    config.DatabaseConfig
		expect string
	}{
		{
			name: "standard config",
			cfg: config.DatabaseConfig{
				Postgres: config.PostgresConfig{
					Host:     "localhost",
					Port:     5432,
					User:     "user",
					Password: "pass",
					DBName:   "db",
					SSLMode:  "disable",
				},
			},
			expect: "postgres://user:pass@localhost:5432/db?sslmode=disable",
		},
		{
			name: "production config",
			cfg: config.DatabaseConfig{
				Postgres: config.PostgresConfig{
					Host:     "db.prod.internal",
					Port:     5432,
					User:     "admin",
					Password: "complex!password",
					DBName:   "keyip",
					SSLMode:  "verify-full",
				},
			},
			expect: "postgres://admin:complex!password@db.prod.internal:5432/keyip?sslmode=verify-full",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := buildConnString(tc.cfg)
			assert.Equal(t, tc.expect, result)
		})
	}
}

func TestConfigurePool(t *testing.T) {
	t.Parallel()

	t.Run("applies custom settings", func(t *testing.T) {
		cfg := config.DatabaseConfig{
			Postgres: config.PostgresConfig{
				MaxOpenConns:    50,
				MaxIdleConns:    10,
				ConnMaxLifetime: 2 * time.Hour,
				ConnMaxIdleTime: 45 * time.Minute,
			},
		}
		poolCfg := &pgxpool.Config{}
		configurePool(poolCfg, cfg)

		assert.Equal(t, int32(50), poolCfg.MaxConns)
		assert.Equal(t, int32(10), poolCfg.MinConns)
		assert.Equal(t, 2*time.Hour, poolCfg.MaxConnLifetime)
		assert.Equal(t, 45*time.Minute, poolCfg.MaxConnIdleTime)
	})

	t.Run("handles zero values", func(t *testing.T) {
		cfg := config.DatabaseConfig{}
		poolCfg := &pgxpool.Config{
			MaxConns: 25, // default
		}
		configurePool(poolCfg, cfg)
		assert.Equal(t, int32(25), poolCfg.MaxConns)
	})
}

// //Personal.AI order the ending

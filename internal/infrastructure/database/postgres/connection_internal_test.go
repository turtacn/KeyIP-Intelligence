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
				Host:     "localhost",
				Port:     5432,
				User:     "user",
				Password: "pass",
				DBName:   "db",
				SSLMode:  "disable",
			},
			expect: "postgres://user:pass@localhost:5432/db?sslmode=disable",
		},
		{
			name: "production config",
			cfg: config.DatabaseConfig{
				Host:     "db.prod.internal",
				Port:     5432,
				User:     "admin",
				Password: "complex!password",
				DBName:   "keyip",
				SSLMode:  "verify-full",
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
		poolCfg := &pgxpool.Config{}
		cfg := config.DatabaseConfig{
			MaxConns:        50,
			MinConns:        10,
			ConnMaxLifetime: 2 * time.Hour,
			ConnMaxIdleTime: 45 * time.Minute,
		}

		configurePool(poolCfg, cfg)

		assert.Equal(t, int32(50), poolCfg.MaxConns)
		assert.Equal(t, int32(10), poolCfg.MinConns)
		assert.Equal(t, 2*time.Hour, poolCfg.MaxConnLifetime)
		assert.Equal(t, 45*time.Minute, poolCfg.MaxConnIdleTime)
	})

	t.Run("applies defaults", func(t *testing.T) {
		poolCfg := &pgxpool.Config{}
		cfg := config.DatabaseConfig{}

		configurePool(poolCfg, cfg)

		assert.Equal(t, int32(defaultMaxConns), poolCfg.MaxConns)
		assert.Equal(t, int32(defaultMinConns), poolCfg.MinConns)
		assert.Equal(t, defaultMaxConnLifetime, poolCfg.MaxConnLifetime)
		assert.Equal(t, defaultMaxConnIdleTime, poolCfg.MaxConnIdleTime)
		assert.Equal(t, defaultHealthCheckPeriod, poolCfg.HealthCheckPeriod)
	})
}

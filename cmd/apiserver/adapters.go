package main

import (
	"context"

	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/database/postgres"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/database/redis"
)

// Adapters for HealthHandler
type postgresHealthAdapter struct {
	conn *postgres.Connection
}

func (a *postgresHealthAdapter) Name() string {
	return "postgres"
}

func (a *postgresHealthAdapter) Check(ctx context.Context) error {
	return a.conn.HealthCheck(ctx)
}

type redisHealthAdapter struct {
	client *redis.Client
}

func (a *redisHealthAdapter) Name() string {
	return "redis"
}

func (a *redisHealthAdapter) Check(ctx context.Context) error {
	return a.client.GetUnderlyingClient().Ping(ctx).Err()
}

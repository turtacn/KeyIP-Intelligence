// Package grpc provides gRPC server infrastructure including health checking
// with dynamic dependency verification.
package grpc

import (
	"context"
	"sync"

	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

// Checker defines the interface for dependency health checks.
// Implementations verify the health of external dependencies such as databases,
// caches, or other services.
type Checker interface {
	// Name returns the human-readable name of the dependency (e.g. "postgres", "redis").
	Name() string
	// CheckHealth performs a health check against the dependency.
	// Returns nil if healthy, or an error describing the failure.
	CheckHealth(ctx context.Context) error
}

// CheckerFunc is a function adapter that allows a plain function to be used as a Checker.
type CheckerFunc struct {
	name string
	fn   func(context.Context) error
}

// Name returns the name of the checker.
func (c *CheckerFunc) Name() string { return c.name }

// CheckHealth calls the underlying function.
func (c *CheckerFunc) CheckHealth(ctx context.Context) error { return c.fn(ctx) }

// NewChecker creates a Checker from a name and a health check function.
func NewChecker(name string, fn func(context.Context) error) Checker {
	return &CheckerFunc{name: name, fn: fn}
}

// HealthService implements grpc_health_v1.HealthServer with dynamic dependency checking.
// It wraps the standard gRPC health server and runs Checker instances on each Check call.
// This allows the health endpoint to reflect the actual status of backend dependencies
// (e.g. database connectivity) rather than returning a static SERVING status.
type HealthService struct {
	healthpb.UnimplementedHealthServer
	mu       sync.RWMutex
	checkers []Checker
	inner    *health.Server
}

// NewHealthService creates a new HealthService with the given Checkers.
// The overall health (empty service key) is initialized to SERVING.
func NewHealthService(checkers ...Checker) *HealthService {
	hs := &HealthService{
		checkers: checkers,
		inner:    health.NewServer(),
	}
	hs.inner.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	return hs
}

// Check implements grpc_health_v1.HealthServer.Check.
// It runs all registered dependency checkers:
//   - If any checker fails, the requested service (or overall health) is set to NOT_SERVING.
//   - If all checkers pass, the service is set to SERVING and delegated to the inner server.
//   - If no checkers are configured, delegates directly to the inner server.
func (s *HealthService) Check(ctx context.Context, req *healthpb.HealthCheckRequest) (*healthpb.HealthCheckResponse, error) {
	service := req.Service

	s.mu.RLock()
	checkers := s.checkers
	s.mu.RUnlock()

	if len(checkers) > 0 {
		for _, checker := range checkers {
			if err := checker.CheckHealth(ctx); err != nil {
				s.inner.SetServingStatus(service, healthpb.HealthCheckResponse_NOT_SERVING)
				return &healthpb.HealthCheckResponse{
					Status: healthpb.HealthCheckResponse_NOT_SERVING,
				}, nil
			}
		}
		// All checkers passed, ensure status is SERVING
		s.inner.SetServingStatus(service, healthpb.HealthCheckResponse_SERVING)
	}

	return s.inner.Check(ctx, req)
}

// Watch implements grpc_health_v1.HealthServer.Watch, delegating to the inner health server.
// Status changes triggered by Check calls are propagated to Watch subscribers.
func (s *HealthService) Watch(req *healthpb.HealthCheckRequest, stream healthpb.Health_WatchServer) error {
	return s.inner.Watch(req, stream)
}

// SetServingStatus sets the serving status for the given service on the inner health server.
// This is used by the Server wrapper when registering new gRPC services.
func (s *HealthService) SetServingStatus(service string, status healthpb.HealthCheckResponse_ServingStatus) {
	s.inner.SetServingStatus(service, status)
}

// Phase 12 - File #286: cmd/apiserver/main.go
// API server entry point for KeyIP-Intelligence.
package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"

	"github.com/turtacn/KeyIP-Intelligence/internal/config"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	httpserver "github.com/turtacn/KeyIP-Intelligence/internal/interfaces/http"
	"github.com/turtacn/KeyIP-Intelligence/internal/interfaces/http/handlers"
	"github.com/turtacn/KeyIP-Intelligence/internal/interfaces/http/middleware"
)

const (
	defaultConfigPath  = "configs/config.yaml"
	defaultHTTPPort    = 8080
	defaultGRPCPort    = 9090
	shutdownTimeout    = 30 * time.Second
)

func main() {
	// Parse command-line flags
	configPath := flag.String("config", defaultConfigPath, "path to configuration file")
	httpPort := flag.Int("http-port", 0, "HTTP server port (overrides config)")
	grpcPort := flag.Int("grpc-port", 0, "gRPC server port (overrides config)")
	flag.Parse()

	// Load configuration (or use defaults if file not found)
	cfg, err := loadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: using default configuration: %v\n", err)
		cfg = config.NewDefaultConfig()
	}

	// Apply command-line overrides
	actualHTTPPort := cfg.Server.HTTP.Port
	if *httpPort > 0 {
		actualHTTPPort = *httpPort
	}
	if actualHTTPPort == 0 {
		actualHTTPPort = defaultHTTPPort
	}

	actualGRPCPort := cfg.Server.GRPC.Port
	if *grpcPort > 0 {
		actualGRPCPort = *grpcPort
	}
	if actualGRPCPort == 0 {
		actualGRPCPort = defaultGRPCPort
	}

	// Initialize logger
	logger := logging.NewDefaultLogger()
	logger.Info("starting KeyIP-Intelligence API server",
		logging.String("version", config.Version),
		logging.Int("http_port", actualHTTPPort),
		logging.Int("grpc_port", actualGRPCPort),
	)

	// Create middleware wrappers
	// Note: In a real implementation, we would inject actual validators and services here.
	// For now, we use defaults or nil where appropriate for the skeleton.

	loggingMw := middleware.NewLoggingMiddleware(logger, middleware.DefaultLoggingConfig())
	tenantCfg := middleware.DefaultTenantConfig()
	tenantCfg.Logger = logger
	tenantMw := middleware.NewTenantMiddlewareWrapper(tenantCfg, logger)

	// Router Dependencies
	// Handlers are currently nil as services are not yet wired up in main.go
	// This will be populated in Phase 12 fully.
	routerDeps := httpserver.RouterDeps{
		HealthHandler:     handlers.NewHealthHandler(config.Version),
		LoggingMiddleware: loggingMw.Handler,
		TenantMiddleware:  tenantMw.Handler,
		// Other handlers and middlewares will be injected here
	}

	// Create HTTP router
	routerCfg := httpserver.RouterConfig{
		Logger:          logger,
		EnableAuth:      false, // Disabled until auth service is wired
		EnableTenant:    true,
		EnableRateLimit: false,
		EnableCORS:      true,
	}
	httpRouter := httpserver.NewRouter(routerCfg, routerDeps)

	// Create HTTP server
	// We use our new Server wrapper if possible, but keeping existing structure for minimal diff
	// unless we want to replace it with httpserver.NewServer

	srvCfg := httpserver.ServerConfig{
		Host: "0.0.0.0",
		Port: actualHTTPPort,
		Logger: logger,
	}
	serverWrapper, err := httpserver.NewServer(srvCfg, httpRouter)
	if err != nil {
		logger.Fatal("failed to create HTTP server", logging.Err(err))
	}

	// Create gRPC server (placeholder)
	grpcSrv := grpc.NewServer()

	// Start HTTP server
	go func() {
		if err := serverWrapper.Start(context.Background()); err != nil {
			logger.Error("HTTP server error", logging.Err(err))
		}
	}()

	// Start gRPC server
	go func() {
		lis, err := net.Listen("tcp", fmt.Sprintf(":%d", actualGRPCPort))
		if err != nil {
			logger.Error("failed to listen for gRPC", logging.Err(err))
			return
		}
		logger.Info("gRPC server listening", logging.Int("port", actualGRPCPort))
		if err := grpcSrv.Serve(lis); err != nil {
			logger.Error("gRPC server error", logging.Err(err))
		}
	}()

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down servers...")

	// Graceful shutdown
	if err := serverWrapper.Shutdown(); err != nil {
		logger.Error("HTTP server shutdown error", logging.Err(err))
	}
	grpcSrv.GracefulStop()

	logger.Info("servers stopped")
}

// loadConfig attempts to load configuration from file, returns error if not found.
func loadConfig(path string) (*config.Config, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf("config file not found: %s", path)
	}
	return config.LoadFromFile(path)
}

//Personal.AI order the ending

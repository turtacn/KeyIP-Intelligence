// Phase 12 - File #286: cmd/apiserver/main.go
// API server entry point for KeyIP-Intelligence.
package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"

	"github.com/turtacn/KeyIP-Intelligence/internal/config"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	httpserver "github.com/turtacn/KeyIP-Intelligence/internal/interfaces/http"
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

	// Create HTTP router with minimal configuration
	routerCfg := httpserver.RouterConfig{
		Logger: logger,
	}
	httpRouter := httpserver.NewRouter(routerCfg)

	// Create HTTP server
	httpSrv := &http.Server{
		Addr:         fmt.Sprintf(":%d", actualHTTPPort),
		Handler:      httpRouter,
		ReadTimeout:  cfg.Server.HTTP.ReadTimeout,
		WriteTimeout: cfg.Server.HTTP.WriteTimeout,
	}

	// Create gRPC server (placeholder)
	grpcSrv := grpc.NewServer()

	// Start HTTP server
	go func() {
		logger.Info("HTTP server listening", logging.Int("port", actualHTTPPort))
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
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
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := httpSrv.Shutdown(ctx); err != nil {
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

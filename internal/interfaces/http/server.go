package http

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"sync"
	"time"
)

// ServerConfig holds HTTP server configuration
type ServerConfig struct {
	Host              string
	Port              int
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	ReadHeaderTimeout time.Duration
	MaxHeaderBytes    int
	ShutdownTimeout   time.Duration
	TLS               *TLSConfig
}

// TLSConfig holds TLS-specific configuration
type TLSConfig struct {
	Enabled          bool
	CertFile         string
	KeyFile          string
	MinVersion       uint16
	ClientAuth       tls.ClientAuthType
	ClientCACertFile string
}

// DefaultServerConfig returns default server configuration
func DefaultServerConfig() *ServerConfig {
	return &ServerConfig{
		Host:              "0.0.0.0",
		Port:              8080,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
		ReadHeaderTimeout: 10 * time.Second,
		MaxHeaderBytes:    1 << 20, // 1MB
		ShutdownTimeout:   30 * time.Second,
	}
}

// Server represents an HTTP server instance
type Server struct {
	cfg          *ServerConfig
	httpServer   *http.Server
	listener     net.Listener
	running      bool
	mu           sync.RWMutex
	started      chan struct{}
	shutdownOnce sync.Once
}

// NewServer creates a new HTTP server with the given configuration and handler
func NewServer(cfg *ServerConfig, handler http.Handler) (*Server, error) {
	if handler == nil {
		return nil, fmt.Errorf("handler cannot be nil")
	}

	if cfg == nil {
		cfg = DefaultServerConfig()
	} else {
		applyDefaults(cfg)
	}

	// Validate configuration
	if cfg.Port < 0 || cfg.Port > 65535 {
		return nil, fmt.Errorf("invalid port: %d (must be 0-65535)", cfg.Port)
	}
	if cfg.ReadTimeout < 0 || cfg.WriteTimeout < 0 || cfg.IdleTimeout < 0 {
		return nil, fmt.Errorf("timeouts cannot be negative")
	}

	// Validate TLS configuration
	if cfg.TLS != nil && cfg.TLS.Enabled {
		if cfg.TLS.CertFile == "" {
			return nil, fmt.Errorf("TLS enabled but CertFile is empty")
		}
		if cfg.TLS.KeyFile == "" {
			return nil, fmt.Errorf("TLS enabled but KeyFile is empty")
		}
		if _, err := os.Stat(cfg.TLS.CertFile); err != nil {
			return nil, fmt.Errorf("TLS CertFile not found: %w", err)
		}
		if _, err := os.Stat(cfg.TLS.KeyFile); err != nil {
			return nil, fmt.Errorf("TLS KeyFile not found: %w", err)
		}
		if cfg.TLS.ClientAuth != tls.NoClientCert && cfg.TLS.ClientCACertFile == "" {
			return nil, fmt.Errorf("ClientAuth requires ClientCACertFile")
		}
	}

	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)

	httpServer := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		MaxHeaderBytes:    cfg.MaxHeaderBytes,
		ErrorLog:          log.New(&logWriter{}, "", 0),
	}

	// Configure TLS
	if cfg.TLS != nil && cfg.TLS.Enabled {
		tlsConfig := &tls.Config{
			MinVersion: cfg.TLS.MinVersion,
			CurvePreferences: []tls.CurveID{
				tls.X25519,
				tls.CurveP256,
			},
			CipherSuites: []uint16{
				tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,
				tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256,
				tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			},
			PreferServerCipherSuites: true,
			ClientAuth:               cfg.TLS.ClientAuth,
		}

		if cfg.TLS.ClientCACertFile != "" {
			caCert, err := os.ReadFile(cfg.TLS.ClientCACertFile)
			if err != nil {
				return nil, fmt.Errorf("failed to read ClientCACertFile: %w", err)
			}
			caCertPool := x509.NewCertPool()
			if !caCertPool.AppendCertsFromPEM(caCert) {
				return nil, fmt.Errorf("failed to parse ClientCACert")
			}
			tlsConfig.ClientCAs = caCertPool
		}

		httpServer.TLSConfig = tlsConfig
	}

	return &Server{
		cfg:        cfg,
		httpServer: httpServer,
		started:    make(chan struct{}),
	}, nil
}

// Start starts the HTTP server and blocks until shutdown
func (s *Server) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return fmt.Errorf("server already running")
	}

	listener, err := net.Listen("tcp", s.httpServer.Addr)
	if err != nil {
		s.mu.Unlock()
		return fmt.Errorf("failed to create listener: %w", err)
	}

	s.listener = listener
	s.running = true
	s.mu.Unlock()

	log.Printf("HTTP server starting on %s (TLS: %v)", listener.Addr().String(), s.cfg.TLS != nil && s.cfg.TLS.Enabled)

	errCh := make(chan error, 1)
	go func() {
		var err error
		if s.cfg.TLS != nil && s.cfg.TLS.Enabled {
			err = s.httpServer.ServeTLS(listener, s.cfg.TLS.CertFile, s.cfg.TLS.KeyFile)
		} else {
			err = s.httpServer.Serve(listener)
		}
		if err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	close(s.started)

	select {
	case <-ctx.Done():
		return s.Shutdown()
	case err := <-errCh:
		s.mu.Lock()
		s.running = false
		s.mu.Unlock()
		return err
	}
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown() error {
	var shutdownErr error
	s.shutdownOnce.Do(func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), s.cfg.ShutdownTimeout)
		defer cancel()

		log.Printf("HTTP server shutting down (timeout: %v)", s.cfg.ShutdownTimeout)

		err := s.httpServer.Shutdown(shutdownCtx)
		if err == context.DeadlineExceeded {
			log.Printf("HTTP server shutdown timed out, forcing close")
			err = s.httpServer.Close()
		}

		s.mu.Lock()
		s.running = false
		s.mu.Unlock()

		if err != nil {
			shutdownErr = fmt.Errorf("shutdown failed: %w", err)
		} else {
			log.Printf("HTTP server stopped")
		}
	})
	return shutdownErr
}

// Addr returns the server's listening address
func (s *Server) Addr() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.listener != nil {
		return s.listener.Addr().String()
	}
	return s.httpServer.Addr
}

// WaitForReady returns a channel that is closed when the server is ready
func (s *Server) WaitForReady() <-chan struct{} {
	return s.started
}

// IsRunning returns true if the server is currently running
func (s *Server) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// logWriter bridges http.Server's ErrorLog to our logging system
type logWriter struct{}

func (lw *logWriter) Write(p []byte) (n int, err error) {
	msg := string(p)
	if len(msg) > 0 && msg[len(msg)-1] == '\n' {
		msg = msg[:len(msg)-1]
	}
	log.Printf("HTTP server error: %s", msg)
	return len(p), nil
}

// applyDefaults fills in zero values with defaults
// Note: Port=0 is a valid value (OS assigns a port), so we use -1 to indicate "use default"
func applyDefaults(cfg *ServerConfig) {
	if cfg.Host == "" {
		cfg.Host = "0.0.0.0"
	}
	// Port=0 is valid (OS assigns), only apply default if explicitly set to -1 or not set in struct
	// Since we can't distinguish between "not set" and "0", use the validation instead
	if cfg.ReadTimeout == 0 {
		cfg.ReadTimeout = 30 * time.Second
	}
	if cfg.WriteTimeout == 0 {
		cfg.WriteTimeout = 60 * time.Second
	}
	if cfg.IdleTimeout == 0 {
		cfg.IdleTimeout = 120 * time.Second
	}
	if cfg.ReadHeaderTimeout == 0 {
		cfg.ReadHeaderTimeout = 10 * time.Second
	}
	if cfg.MaxHeaderBytes == 0 {
		cfg.MaxHeaderBytes = 1 << 20
	}
	if cfg.ShutdownTimeout == 0 {
		cfg.ShutdownTimeout = 30 * time.Second
	}
	if cfg.TLS != nil && cfg.TLS.MinVersion == 0 {
		cfg.TLS.MinVersion = tls.VersionTLS12
	}
}

//Personal.AI order the ending

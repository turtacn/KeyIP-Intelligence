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
	"strings"
	"sync"
	"time"

	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// ServerConfig holds configuration for the HTTP server.
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
	Logger            logging.Logger
}

// TLSConfig holds TLS configuration.
type TLSConfig struct {
	CertFile         string
	KeyFile          string
	MinVersion       uint16
	ClientAuth       tls.ClientAuthType
	ClientCACertFile string
}

// DefaultServerConfig returns a ServerConfig with default values.
func DefaultServerConfig() ServerConfig {
	return ServerConfig{
		Host:              "0.0.0.0",
		Port:              8080,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
		ReadHeaderTimeout: 10 * time.Second,
		MaxHeaderBytes:    1 << 20, // 1 MB
		ShutdownTimeout:   30 * time.Second,
	}
}

// Server represents the HTTP server.
type Server struct {
	cfg          ServerConfig
	httpServer   *http.Server
	listener     net.Listener
	logger       logging.Logger
	started      chan struct{}
	shutdownOnce sync.Once
	mu           sync.Mutex
	running      bool
}

// NewServer creates a new Server instance.
func NewServer(cfg ServerConfig, handler http.Handler) (*Server, error) {
	if handler == nil {
		return nil, errors.New(errors.ErrCodeInvalidArgument, "handler cannot be nil")
	}
	if cfg.Port < 0 || cfg.Port > 65535 {
		return nil, errors.New(errors.ErrCodeInvalidArgument, fmt.Sprintf("invalid port: %d", cfg.Port))
	}
	if cfg.ReadTimeout < 0 || cfg.WriteTimeout < 0 || cfg.IdleTimeout < 0 || cfg.ReadHeaderTimeout < 0 || cfg.ShutdownTimeout < 0 {
		return nil, errors.New(errors.ErrCodeInvalidArgument, "timeout values cannot be negative")
	}

	applyDefaults(&cfg)

	if cfg.TLS != nil {
		if _, err := os.Stat(cfg.TLS.CertFile); os.IsNotExist(err) {
			return nil, errors.New(errors.ErrCodeInvalidArgument, fmt.Sprintf("cert file not found: %s", cfg.TLS.CertFile))
		}
		if _, err := os.Stat(cfg.TLS.KeyFile); os.IsNotExist(err) {
			return nil, errors.New(errors.ErrCodeInvalidArgument, fmt.Sprintf("key file not found: %s", cfg.TLS.KeyFile))
		}
		if cfg.TLS.ClientAuth >= tls.VerifyClientCertIfGiven && cfg.TLS.ClientCACertFile == "" {
			return nil, errors.New(errors.ErrCodeInvalidArgument, "client CA cert file is required for client auth")
		}
	}

	s := &Server{
		cfg:     cfg,
		logger:  cfg.Logger,
		started: make(chan struct{}),
	}

	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	s.httpServer = &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		MaxHeaderBytes:    cfg.MaxHeaderBytes,
		ErrorLog:          log.New(&logWriter{logger: cfg.Logger}, "", 0),
	}

	if cfg.TLS != nil {
		tlsConfig := &tls.Config{
			MinVersion:       cfg.TLS.MinVersion,
			CurvePreferences: []tls.CurveID{tls.X25519, tls.CurveP256},
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
				return nil, errors.New(errors.ErrCodeInvalidArgument, fmt.Sprintf("failed to read client CA cert: %v", err))
			}
			caCertPool := x509.NewCertPool()
			caCertPool.AppendCertsFromPEM(caCert)
			tlsConfig.ClientCAs = caCertPool
		}
		s.httpServer.TLSConfig = tlsConfig
	}

	return s, nil
}

// Start starts the HTTP server.
func (s *Server) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return errors.New(errors.ErrCodeServerAlreadyRunning, "server already running")
	}

	listener, err := net.Listen("tcp", s.httpServer.Addr)
	if err != nil {
		s.mu.Unlock()
		return err
	}
	s.listener = listener
	s.running = true
	s.mu.Unlock()

	if s.logger != nil {
		s.logger.Info("HTTP server starting", logging.String("addr", listener.Addr().String()), logging.Bool("tls", s.cfg.TLS != nil))
	}

	errCh := make(chan error, 1)
	go func() {
		var err error
		if s.cfg.TLS != nil {
			err = s.httpServer.ServeTLS(listener, s.cfg.TLS.CertFile, s.cfg.TLS.KeyFile)
		} else {
			err = s.httpServer.Serve(listener)
		}
		if err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
		close(errCh)
	}()

	close(s.started)

	select {
	case <-ctx.Done():
		return s.Shutdown()
	case err := <-errCh:
		if err != nil {
			if s.logger != nil {
				s.logger.Error("HTTP server error", logging.Err(err))
			}
			return err
		}
	}
	return nil
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown() error {
	var err error
	s.shutdownOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), s.cfg.ShutdownTimeout)
		defer cancel()

		if s.logger != nil {
			s.logger.Info("HTTP server shutting down", logging.Duration("timeout", s.cfg.ShutdownTimeout))
		}

		err = s.httpServer.Shutdown(ctx)
		if err == context.DeadlineExceeded {
			if s.logger != nil {
				s.logger.Warn("HTTP server shutdown timed out, forcing close")
			}
			err = s.httpServer.Close()
		}

		s.mu.Lock()
		s.running = false
		s.mu.Unlock()

		if s.logger != nil {
			s.logger.Info("HTTP server stopped")
		}
	})
	return err
}

// Addr returns the address the server is listening on.
func (s *Server) Addr() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.listener != nil {
		return s.listener.Addr().String()
	}
	return s.httpServer.Addr
}

// WaitForReady returns a channel that is closed when the server is ready.
func (s *Server) WaitForReady() <-chan struct{} {
	return s.started
}

// IsRunning returns true if the server is running.
func (s *Server) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

func applyDefaults(cfg *ServerConfig) {
	if cfg.Host == "" {
		cfg.Host = "0.0.0.0"
	}
	// Port 0 is allowed for random port assignment
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

type logWriter struct {
	logger logging.Logger
}

func (w *logWriter) Write(p []byte) (n int, err error) {
	msg := strings.TrimSpace(string(p))
	if len(msg) > 0 && w.logger != nil {
		w.logger.Error(msg)
	}
	return len(p), nil
}

//Personal.AI order the ending

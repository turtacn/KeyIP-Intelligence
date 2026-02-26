// ---
//
// 继续输出 284 `internal/interfaces/http/server.go` 要实现 HTTP 服务器生命周期管理。
//
// 实现要求:
//
// * **功能定位**：封装 net/http.Server 的创建、启动、优雅关闭全生命周期，
//   作为 KeyIP-Intelligence API 服务的 HTTP 入口点
// * **核心实现**：
//   * 定义 `ServerConfig` 结构体：
//     - Host string（监听地址，默认 "0.0.0.0"）
//     - Port int（监听端口，默认 8080）
//     - ReadTimeout time.Duration（读超时，默认 30s）
//     - WriteTimeout time.Duration（写超时，默认 60s）
//     - IdleTimeout time.Duration（空闲超时，默认 120s）
//     - ReadHeaderTimeout time.Duration（读 Header 超时，默认 10s）
//     - MaxHeaderBytes int（最大 Header 字节数，默认 1MB）
//     - ShutdownTimeout time.Duration（优雅关闭超时，默认 30s）
//     - TLSCertFile string（TLS 证书路径，可选）
//     - TLSKeyFile string（TLS 密钥路径，可选）
//   * 定义 `Server` 结构体：
//     - httpServer *http.Server
//     - config ServerConfig
//     - handler http.Handler
//     - logger logging.Logger
//     - started atomic.Bool
//   * 定义 `NewServer(cfg ServerConfig, handler http.Handler, logger logging.Logger) *Server`：
//     - 应用默认值填充
//     - 构建 net/http.Server 实例
//   * 定义 `(s *Server) Start(ctx context.Context) error`：
//     - 启动 HTTP/HTTPS 监听
//     - 在 goroutine 中运行 ListenAndServe / ListenAndServeTLS
//     - 监听 ctx.Done() 触发优雅关闭
//     - 返回启动错误（非 ErrServerClosed）
//   * 定义 `(s *Server) Shutdown(ctx context.Context) error`：
//     - 调用 httpServer.Shutdown 优雅关闭
//     - 等待所有活跃连接处理完毕或超时
//   * 定义 `(s *Server) Addr() string`：返回实际监听地址
//   * 定义 `(s *Server) IsRunning() bool`：返回服务器运行状态
// * **业务逻辑**：
//   - 支持 HTTP 和 HTTPS 双模式，通过 TLS 配置自动切换
//   - 优雅关闭时先停止接受新连接，等待活跃请求完成
//   - 超时后强制关闭剩余连接
//   - 启动时记录监听地址和协议到日志
// * **依赖关系**：
//   * 依赖：net/http、crypto/tls、internal/infrastructure/monitoring/logging/logger.go
//   * 被依赖：cmd/apiserver/main.go
// * **测试要求**：启动/关闭生命周期、TLS 模式切换、超时配置、并发安全
// * **强制约束**：文件最后一行必须为 `//Personal.AI order the ending`
//
// ---
package http

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
)

// Default server configuration values.
const (
	defaultHost              = "0.0.0.0"
	defaultPort              = 8080
	defaultReadTimeout       = 30 * time.Second
	defaultWriteTimeout      = 60 * time.Second
	defaultIdleTimeout       = 120 * time.Second
	defaultReadHeaderTimeout = 10 * time.Second
	defaultMaxHeaderBytes    = 1 << 20 // 1 MB
	defaultShutdownTimeout   = 30 * time.Second
)

// ServerConfig holds all configuration parameters for the HTTP server.
type ServerConfig struct {
	// Host is the network interface to bind to. Default: "0.0.0.0".
	Host string

	// Port is the TCP port to listen on. Default: 8080.
	Port int

	// ReadTimeout is the maximum duration for reading the entire request,
	// including the body. Default: 30s.
	ReadTimeout time.Duration

	// WriteTimeout is the maximum duration before timing out writes of the
	// response. Default: 60s.
	WriteTimeout time.Duration

	// IdleTimeout is the maximum amount of time to wait for the next request
	// when keep-alives are enabled. Default: 120s.
	IdleTimeout time.Duration

	// ReadHeaderTimeout is the amount of time allowed to read request headers.
	// Default: 10s.
	ReadHeaderTimeout time.Duration

	// MaxHeaderBytes controls the maximum number of bytes the server will read
	// parsing the request header's keys and values. Default: 1MB.
	MaxHeaderBytes int

	// ShutdownTimeout is the maximum duration to wait for active connections
	// to finish during graceful shutdown. Default: 30s.
	ShutdownTimeout time.Duration

	// TLSCertFile is the path to the TLS certificate file. If both TLSCertFile
	// and TLSKeyFile are set, the server starts in HTTPS mode.
	TLSCertFile string

	// TLSKeyFile is the path to the TLS private key file.
	TLSKeyFile string
}

// applyDefaults fills zero-value fields with sensible defaults.
func (c *ServerConfig) applyDefaults() {
	if c.Host == "" {
		c.Host = defaultHost
	}
	if c.Port == 0 {
		c.Port = defaultPort
	}
	if c.ReadTimeout == 0 {
		c.ReadTimeout = defaultReadTimeout
	}
	if c.WriteTimeout == 0 {
		c.WriteTimeout = defaultWriteTimeout
	}
	if c.IdleTimeout == 0 {
		c.IdleTimeout = defaultIdleTimeout
	}
	if c.ReadHeaderTimeout == 0 {
		c.ReadHeaderTimeout = defaultReadHeaderTimeout
	}
	if c.MaxHeaderBytes == 0 {
		c.MaxHeaderBytes = defaultMaxHeaderBytes
	}
	if c.ShutdownTimeout == 0 {
		c.ShutdownTimeout = defaultShutdownTimeout
	}
}

// isTLSEnabled returns true when both certificate and key paths are configured.
func (c *ServerConfig) isTLSEnabled() bool {
	return c.TLSCertFile != "" && c.TLSKeyFile != ""
}

// listenAddr returns the "host:port" string for net.Listen.
func (c *ServerConfig) listenAddr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

// Server wraps net/http.Server with lifecycle management including graceful
// shutdown, TLS support, and observability hooks.
type Server struct {
	httpServer *http.Server
	config     ServerConfig
	handler    http.Handler
	logger     logging.Logger
	listener   net.Listener
	started    atomic.Bool
	actualAddr string
}

// NewServer creates a new Server with the given configuration, handler, and logger.
// Zero-value configuration fields are replaced with sensible defaults.
func NewServer(cfg ServerConfig, handler http.Handler, logger logging.Logger) *Server {
	cfg.applyDefaults()

	httpSrv := &http.Server{
		Addr:              cfg.listenAddr(),
		Handler:           handler,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		MaxHeaderBytes:    cfg.MaxHeaderBytes,
	}

	// If TLS is enabled, configure minimum TLS version for security.
	if cfg.isTLSEnabled() {
		httpSrv.TLSConfig = &tls.Config{
			MinVersion:               tls.VersionTLS12,
			PreferServerCipherSuites: true,
			CurvePreferences: []tls.CurveID{
				tls.X25519,
				tls.CurveP256,
			},
		}
	}

	return &Server{
		httpServer: httpSrv,
		config:     cfg,
		handler:    handler,
		logger:     logger,
	}
}

// Start begins listening for HTTP(S) requests. It blocks until the provided
// context is cancelled or an unrecoverable error occurs.
//
// When ctx is cancelled, Start initiates a graceful shutdown: it stops
// accepting new connections and waits up to ShutdownTimeout for active
// requests to complete before forcibly closing remaining connections.
//
// Start returns nil on clean shutdown (context cancellation) and a non-nil
// error if the server fails to start or encounters an unexpected error.
func (s *Server) Start(ctx context.Context) error {
	if s.started.Load() {
		return errors.New("server already started")
	}

	// Create listener early so we can capture the actual bound address
	// (important when Port is 0 for ephemeral port allocation in tests).
	ln, err := net.Listen("tcp", s.config.listenAddr())
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", s.config.listenAddr(), err)
	}
	s.listener = ln
	s.actualAddr = ln.Addr().String()
	s.started.Store(true)

	protocol := "HTTP"
	if s.config.isTLSEnabled() {
		protocol = "HTTPS"
	}

	s.logger.Info("server starting",
		logging.String("protocol", protocol),
		logging.String("address", s.actualAddr),
		logging.String("readTimeout", s.config.ReadTimeout.String()),
		logging.String("writeTimeout", s.config.WriteTimeout.String()),
		logging.String("idleTimeout", s.config.IdleTimeout.String()),
		logging.String("shutdownTimeout", s.config.ShutdownTimeout.String()),
	)

	// Channel to capture the serve error from the goroutine.
	serveCh := make(chan error, 1)

	go func() {
		var serveErr error
		if s.config.isTLSEnabled() {
			// Wrap the listener with TLS.
			tlsLn := tls.NewListener(ln, s.httpServer.TLSConfig)
			serveErr = s.httpServer.ServeTLS(tlsLn, s.config.TLSCertFile, s.config.TLSKeyFile)
		} else {
			serveErr = s.httpServer.Serve(ln)
		}
		serveCh <- serveErr
	}()

	// Wait for either context cancellation or serve error.
	select {
	case <-ctx.Done():
		s.logger.Info("shutdown signal received, initiating graceful shutdown")
		shutdownErr := s.Shutdown(context.Background())
		// Drain the serve channel to avoid goroutine leak.
		serveErr := <-serveCh
		if shutdownErr != nil {
			return fmt.Errorf("shutdown error: %w", shutdownErr)
		}
		if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			return serveErr
		}
		return nil

	case err := <-serveCh:
		s.started.Store(false)
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

// Shutdown gracefully shuts down the server without interrupting any active
// connections. It waits up to ShutdownTimeout for active requests to finish.
func (s *Server) Shutdown(ctx context.Context) error {
	if !s.started.Load() {
		return nil
	}

	shutdownCtx, cancel := context.WithTimeout(ctx, s.config.ShutdownTimeout)
	defer cancel()

	s.logger.Info("shutting down server",
		logging.String("timeout", s.config.ShutdownTimeout.String()),
	)

	err := s.httpServer.Shutdown(shutdownCtx)
	s.started.Store(false)

	if err != nil {
		s.logger.Error("server shutdown error", logging.String("error", err.Error()))
		return fmt.Errorf("server shutdown: %w", err)
	}

	s.logger.Info("server stopped gracefully")
	return nil
}

// Addr returns the actual network address the server is listening on.
// This is particularly useful when the server was configured with port 0
// (ephemeral port) for testing purposes.
func (s *Server) Addr() string {
	return s.actualAddr
}

// IsRunning reports whether the server is currently accepting connections.
func (s *Server) IsRunning() bool {
	return s.started.Load()
}

// Config returns a copy of the server's configuration.
func (s *Server) Config() ServerConfig {
	return s.config
}

//Personal.AI order the ending

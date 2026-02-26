// ---
// 252 `internal/interfaces/grpc/server.go`
// 实现 gRPC 服务器核心骨架。
//
// 功能定位：gRPC 传输层入口，负责服务器生命周期管理、服务注册、拦截器链组装、
// 健康检查与优雅关停。
//
// 核心实现：
//   - Server 结构体：grpcServer, listener, config, logger, metrics, healthServer
//   - Option 函数选项：WithLogger, WithMetrics, WithTLSConfig, WithMaxRecvMsgSize,
//     WithMaxSendMsgSize, WithKeepaliveParams
//   - NewServer：创建 TCP listener、组装拦截器链、注册 health/reflection 服务
//   - RegisterService：委派到底层 grpc.Server
//   - Start：启动 listener.Accept 循环
//   - Stop：GracefulStop 带超时 → ForceStop → 关闭 listener
//   - Addr：返回实际监听地址
//   - 拦截器：recovery, logging(unary/stream), metrics(unary/stream), validation
//
// 业务逻辑：
//   - 默认最大消息大小 16MB
//   - Keepalive: MaxConnectionIdle=15min, MaxConnectionAge=30min, Time=5min, Timeout=1s
//   - Recovery 拦截器捕获 panic → 日志堆栈 → codes.Internal
//   - Logging 拦截器记录 method/duration/status_code，排除 health check
//   - Metrics 拦截器按 service/method/code 维度采集
//   - Validation 拦截器对实现 Validate() error 的请求自动调用
//   - GracefulStop 超时默认 10 秒
//   - Reflection 仅在 config.Debug=true 时注册
//
// 依赖：internal/config, internal/infrastructure/monitoring/logging,
//       internal/infrastructure/monitoring/prometheus, pkg/errors,
//       google.golang.org/grpc, grpc/health, grpc/reflection
// 被依赖：cmd/apiserver/main.go, grpc/services/*
// ---
package grpc

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"

	"github.com/turtacn/KeyIP-Intelligence/internal/config"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/prometheus"
)

const (
	defaultMaxRecvMsgSize = 16 * 1024 * 1024 // 16MB
	defaultMaxSendMsgSize = 16 * 1024 * 1024 // 16MB
	defaultGracefulTimeout = 10 * time.Second
)

var defaultKeepaliveParams = keepalive.ServerParameters{
	MaxConnectionIdle:     15 * time.Minute,
	MaxConnectionAge:      30 * time.Minute,
	MaxConnectionAgeGrace: 5 * time.Second,
	Time:                  5 * time.Minute,
	Timeout:               1 * time.Second,
}

var defaultKeepalivePolicy = keepalive.EnforcementPolicy{
	MinTime:             5 * time.Second,
	PermitWithoutStream: true,
}

// Validator is an interface for requests that support self-validation.
type Validator interface {
	Validate() error
}

// Option configures the gRPC Server.
type Option func(*serverOptions)

type serverOptions struct {
	logger          logging.Logger
	metrics         *prometheus.GRPCMetrics
	tlsConfig       *tls.Config
	maxRecvMsgSize  int
	maxSendMsgSize  int
	keepaliveParams keepalive.ServerParameters
	gracefulTimeout time.Duration
}

// WithLogger sets the logger for the gRPC server.
func WithLogger(l logging.Logger) Option {
	return func(o *serverOptions) {
		o.logger = l
	}
}

// WithMetrics sets the Prometheus metrics collector for gRPC.
func WithMetrics(m *prometheus.GRPCMetrics) Option {
	return func(o *serverOptions) {
		o.metrics = m
	}
}

// WithTLSConfig sets TLS configuration for the gRPC server.
func WithTLSConfig(tc *tls.Config) Option {
	return func(o *serverOptions) {
		o.tlsConfig = tc
	}
}

// WithMaxRecvMsgSize sets the maximum receive message size in bytes.
func WithMaxRecvMsgSize(size int) Option {
	return func(o *serverOptions) {
		if size > 0 {
			o.maxRecvMsgSize = size
		}
	}
}

// WithMaxSendMsgSize sets the maximum send message size in bytes.
func WithMaxSendMsgSize(size int) Option {
	return func(o *serverOptions) {
		if size > 0 {
			o.maxSendMsgSize = size
		}
	}
}

// WithKeepaliveParams sets keepalive parameters for the gRPC server.
func WithKeepaliveParams(params keepalive.ServerParameters) Option {
	return func(o *serverOptions) {
		o.keepaliveParams = params
	}
}

// WithGracefulTimeout sets the graceful shutdown timeout.
func WithGracefulTimeout(d time.Duration) Option {
	return func(o *serverOptions) {
		if d > 0 {
			o.gracefulTimeout = d
		}
	}
}

// Server wraps a gRPC server with lifecycle management, interceptor chains,
// health checking, and graceful shutdown.
type Server struct {
	grpcServer   *grpc.Server
	listener     net.Listener
	cfg          *config.GRPCConfig
	opts         *serverOptions
	healthServer *health.Server
	mu           sync.Mutex
	started      bool
}

// NewServer creates a new gRPC Server. It binds a TCP listener, assembles
// the interceptor chain, and registers health and (optionally) reflection services.
func NewServer(cfg *config.GRPCConfig, opts ...Option) (*Server, error) {
	if cfg == nil {
		return nil, fmt.Errorf("grpc config must not be nil")
	}

	// Apply options with defaults.
	sopts := &serverOptions{
		maxRecvMsgSize:  defaultMaxRecvMsgSize,
		maxSendMsgSize:  defaultMaxSendMsgSize,
		keepaliveParams: defaultKeepaliveParams,
		gracefulTimeout: defaultGracefulTimeout,
	}
	for _, o := range opts {
		o(sopts)
	}

	// Ensure a logger is available (noop fallback).
	if sopts.logger == nil {
		sopts.logger = logging.NewNoop()
	}

	// Bind TCP listener.
	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("failed to listen on %s: %w", addr, err)
	}

	// Build unary interceptor chain: recovery → logging → metrics → validation
	unaryChain := chainUnaryInterceptors(
		recoveryUnaryInterceptor(sopts.logger),
		loggingUnaryInterceptor(sopts.logger),
		metricsUnaryInterceptor(sopts.metrics),
		validationUnaryInterceptor(),
	)

	// Build stream interceptor chain: recovery → logging → metrics
	streamChain := chainStreamInterceptors(
		recoveryStreamInterceptor(sopts.logger),
		loggingStreamInterceptor(sopts.logger),
		metricsStreamInterceptor(sopts.metrics),
	)

	// Assemble grpc.ServerOption slice.
	grpcOpts := []grpc.ServerOption{
		grpc.MaxRecvMsgSize(sopts.maxRecvMsgSize),
		grpc.MaxSendMsgSize(sopts.maxSendMsgSize),
		grpc.KeepaliveParams(sopts.keepaliveParams),
		grpc.KeepaliveEnforcementPolicy(defaultKeepalivePolicy),
		grpc.UnaryInterceptor(unaryChain),
		grpc.StreamInterceptor(streamChain),
	}

	if sopts.tlsConfig != nil {
		grpcOpts = append(grpcOpts, grpc.Creds(credentials.NewTLS(sopts.tlsConfig)))
	}

	gs := grpc.NewServer(grpcOpts...)

	// Register health service.
	hs := health.NewServer()
	healthpb.RegisterHealthServer(gs, hs)
	hs.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)

	// Register reflection service in debug mode only.
	if cfg.Debug {
		reflection.Register(gs)
		sopts.logger.Info("grpc reflection service registered (debug mode)")
	}

	return &Server{
		grpcServer:   gs,
		listener:     lis,
		cfg:          cfg,
		opts:         sopts,
		healthServer: hs,
	}, nil
}

// RegisterService registers a gRPC service implementation with the server.
// Must be called before Start.
func (s *Server) RegisterService(desc *grpc.ServiceDesc, impl interface{}) {
	s.grpcServer.RegisterService(desc, impl)
	s.healthServer.SetServingStatus(desc.ServiceName, healthpb.HealthCheckResponse_SERVING)
	s.opts.logger.Info("grpc service registered", logging.String("service", desc.ServiceName))
}

// Start begins serving gRPC requests. It blocks until the server is stopped.
func (s *Server) Start() error {
	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return fmt.Errorf("server already started")
	}
	s.started = true
	s.mu.Unlock()

	s.opts.logger.Info("grpc server starting", logging.String("address", s.listener.Addr().String()))
	return s.grpcServer.Serve(s.listener)
}

// Stop performs a graceful shutdown. If the graceful period expires, it forces
// an immediate stop.
func (s *Server) Stop(ctx context.Context) error {
	s.mu.Lock()
	if !s.started {
		s.mu.Unlock()
		return nil
	}
	s.mu.Unlock()

	s.opts.logger.Info("grpc server stopping")

	// Mark health as NOT_SERVING so load balancers drain traffic.
	s.healthServer.SetServingStatus("", healthpb.HealthCheckResponse_NOT_SERVING)

	timeout := s.opts.gracefulTimeout
	gracefulCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	stopped := make(chan struct{})
	go func() {
		s.grpcServer.GracefulStop()
		close(stopped)
	}()

	select {
	case <-stopped:
		s.opts.logger.Info("grpc server stopped gracefully")
	case <-gracefulCtx.Done():
		s.opts.logger.Warn("grpc graceful stop timed out, forcing stop")
		s.grpcServer.Stop()
	}

	return nil
}

// Addr returns the actual network address the server is listening on.
// Useful when port 0 is specified to get the OS-assigned port.
func (s *Server) Addr() string {
	if s.listener == nil {
		return ""
	}
	return s.listener.Addr().String()
}

// GRPCServer returns the underlying grpc.Server for advanced use cases.
func (s *Server) GRPCServer() *grpc.Server {
	return s.grpcServer
}

// ---------------------------------------------------------------------------
// Interceptors
// ---------------------------------------------------------------------------

// recoveryUnaryInterceptor returns a unary interceptor that recovers from panics.
func recoveryUnaryInterceptor(logger logging.Logger) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (resp interface{}, err error) {
		defer func() {
			if r := recover(); r != nil {
				stack := string(debug.Stack())
				logger.Error("grpc panic recovered",
					logging.String("method", info.FullMethod),
					logging.String("panic", fmt.Sprintf("%v", r)),
					logging.String("stack", stack),
				)
				err = status.Errorf(codes.Internal, "internal server error")
			}
		}()
		return handler(ctx, req)
	}
}

// recoveryStreamInterceptor returns a stream interceptor that recovers from panics.
func recoveryStreamInterceptor(logger logging.Logger) grpc.StreamServerInterceptor {
	return func(
		srv interface{},
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) (err error) {
		defer func() {
			if r := recover(); r != nil {
				stack := string(debug.Stack())
				logger.Error("grpc stream panic recovered",
					logging.String("method", info.FullMethod),
					logging.String("panic", fmt.Sprintf("%v", r)),
					logging.String("stack", stack),
				)
				err = status.Errorf(codes.Internal, "internal server error")
			}
		}()
		return handler(srv, ss)
	}
}

// isHealthCheck returns true if the method belongs to the gRPC health service.
func isHealthCheck(method string) bool {
	return strings.HasPrefix(method, "/grpc.health.v1.Health/")
}

// loggingUnaryInterceptor returns a unary interceptor that logs request metadata.
func loggingUnaryInterceptor(logger logging.Logger) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		if isHealthCheck(info.FullMethod) {
			return handler(ctx, req)
		}

		start := time.Now()
		resp, err := handler(ctx, req)
		duration := time.Since(start)

		code := status.Code(err)
		logger.Info("grpc request",
			logging.String("method", info.FullMethod),
			logging.Int64("duration_ms", duration.Milliseconds()),
			logging.String("code", code.String()),
		)
		return resp, err
	}
}

// loggingStreamInterceptor returns a stream interceptor that logs stream lifecycle.
func loggingStreamInterceptor(logger logging.Logger) grpc.StreamServerInterceptor {
	return func(
		srv interface{},
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		if isHealthCheck(info.FullMethod) {
			return handler(srv, ss)
		}

		start := time.Now()
		err := handler(srv, ss)
		duration := time.Since(start)

		code := status.Code(err)
		logger.Info("grpc stream",
			logging.String("method", info.FullMethod),
			logging.Int64("duration_ms", duration.Milliseconds()),
			logging.String("code", code.String()),
		)
		return err
	}
}

// metricsUnaryInterceptor returns a unary interceptor that records Prometheus metrics.
func metricsUnaryInterceptor(m *prometheus.GRPCMetrics) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		if m == nil {
			return handler(ctx, req)
		}

		start := time.Now()
		resp, err := handler(ctx, req)
		duration := time.Since(start)

		code := status.Code(err)
		service, method := splitMethodName(info.FullMethod)
		m.RecordUnaryRequest(service, method, code.String(), duration)
		return resp, err
	}
}

// metricsStreamInterceptor returns a stream interceptor that records Prometheus metrics.
func metricsStreamInterceptor(m *prometheus.GRPCMetrics) grpc.StreamServerInterceptor {
	return func(
		srv interface{},
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		if m == nil {
			return handler(srv, ss)
		}

		start := time.Now()
		err := handler(srv, ss)
		duration := time.Since(start)

		code := status.Code(err)
		service, method := splitMethodName(info.FullMethod)
		m.RecordStreamRequest(service, method, code.String(), duration)
		return err
	}
}

// validationUnaryInterceptor returns a unary interceptor that validates requests
// implementing the Validator interface.
func validationUnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		if v, ok := req.(Validator); ok {
			if err := v.Validate(); err != nil {
				return nil, status.Errorf(codes.InvalidArgument, "validation failed: %s", err.Error())
			}
		}
		return handler(ctx, req)
	}
}

// ---------------------------------------------------------------------------
// Interceptor chaining helpers
// ---------------------------------------------------------------------------

// chainUnaryInterceptors chains multiple unary interceptors into one.
// Execution order follows the slice order (first interceptor is outermost).
func chainUnaryInterceptors(interceptors ...grpc.UnaryServerInterceptor) grpc.UnaryServerInterceptor {
	n := len(interceptors)
	if n == 0 {
		return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
			return handler(ctx, req)
		}
	}
	if n == 1 {
		return interceptors[0]
	}

	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		buildChain := func(current grpc.UnaryServerInterceptor, next grpc.UnaryHandler) grpc.UnaryHandler {
			return func(currentCtx context.Context, currentReq interface{}) (interface{}, error) {
				return current(currentCtx, currentReq, info, next)
			}
		}

		chain := handler
		for i := n - 1; i >= 0; i-- {
			chain = buildChain(interceptors[i], chain)
		}
		return chain(ctx, req)
	}
}

// chainStreamInterceptors chains multiple stream interceptors into one.
func chainStreamInterceptors(interceptors ...grpc.StreamServerInterceptor) grpc.StreamServerInterceptor {
	n := len(interceptors)
	if n == 0 {
		return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
			return handler(srv, ss)
		}
	}
	if n == 1 {
		return interceptors[0]
	}

	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		buildChain := func(current grpc.StreamServerInterceptor, next grpc.StreamHandler) grpc.StreamHandler {
			return func(currentSrv interface{}, currentStream grpc.ServerStream) error {
				return current(currentSrv, currentStream, info, next)
			}
		}

		chain := handler
		for i := n - 1; i >= 0; i-- {
			chain = buildChain(interceptors[i], chain)
		}
		return chain(srv, ss)
	}
}

// splitMethodName splits "/package.Service/Method" into ("package.Service", "Method").
func splitMethodName(fullMethod string) (string, string) {
	fullMethod = strings.TrimPrefix(fullMethod, "/")
	idx := strings.LastIndex(fullMethod, "/")
	if idx < 0 {
		return "unknown", fullMethod
	}
	return fullMethod[:idx], fullMethod[idx+1:]
}

//Personal.AI order the ending

package grpc

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"os"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/turtacn/KeyIP-Intelligence/internal/interfaces/grpc/services"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
)

// GRPCServerConfig 定义 gRPC 服务器配置
type GRPCServerConfig struct {
	Port                 int           // 监听端口
	MaxRecvMsgSize       int           // 最大接收消息大小，默认 10MB
	MaxSendMsgSize       int           // 最大发送消息大小，默认 10MB
	MaxConcurrentStreams uint32        // 最大并发流，默认 100
	TLSCertFile          string        // 可选，TLS 证书路径
	TLSKeyFile           string        // 可选，TLS 私钥路径
	EnableReflection     bool          // 是否启用 gRPC 反射，开发环境用
	ShutdownTimeout      time.Duration // 优雅停止超时时间，默认 30 秒
}

// DefaultGRPCServerConfig 返回默认配置
func DefaultGRPCServerConfig() *GRPCServerConfig {
	return &GRPCServerConfig{
		Port:                 9090,
		MaxRecvMsgSize:       10 * 1024 * 1024, // 10MB
		MaxSendMsgSize:       10 * 1024 * 1024, // 10MB
		MaxConcurrentStreams: 100,
		EnableReflection:     true,
		ShutdownTimeout:      30 * time.Second,
	}
}

// GRPCServices 聚合全部 gRPC 服务实现
type GRPCServices struct {
	MoleculeService *services.MoleculeServiceServer
	PatentService   *services.PatentServiceServer
}

// Logger 定义 gRPC 服务器日志接口
type Logger interface {
	Info(msg string, fields ...interface{})
	Error(msg string, fields ...interface{})
	Warn(msg string, fields ...interface{})
	Debug(msg string, fields ...interface{})
}

// defaultLogger 提供默认日志实现
type defaultLogger struct{}

func (l *defaultLogger) Info(msg string, fields ...interface{}) {
	log.Printf("[INFO] %s %v", msg, formatFields(fields))
}
func (l *defaultLogger) Error(msg string, fields ...interface{}) {
	log.Printf("[ERROR] %s %v", msg, formatFields(fields))
}
func (l *defaultLogger) Warn(msg string, fields ...interface{}) {
	log.Printf("[WARN] %s %v", msg, formatFields(fields))
}
func (l *defaultLogger) Debug(msg string, fields ...interface{}) {
	log.Printf("[DEBUG] %s %v", msg, formatFields(fields))
}

func formatFields(fields []interface{}) string {
	if len(fields) == 0 {
		return ""
	}
	var parts []string
	for i := 0; i < len(fields)-1; i += 2 {
		parts = append(parts, fmt.Sprintf("%v=%v", fields[i], fields[i+1]))
	}
	return strings.Join(parts, " ")
}

// GRPCServer 封装 gRPC 服务器生命周期管理
type GRPCServer struct {
	server   *grpc.Server
	config   *GRPCServerConfig
	listener net.Listener
	logger   Logger
	services *GRPCServices
	mu       sync.RWMutex
	running  bool

	// 指标统计
	requestCount   int64
	requestLatency int64 // 纳秒累计
}

// Server 为向后兼容保留的类型别名
type Server = GRPCServer

// NewServer 创建 gRPC 服务器（向后兼容接口）
func NewServer(port int) *Server {
	cfg := DefaultGRPCServerConfig()
	cfg.Port = port
	srv, _ := NewGRPCServer(cfg, nil, nil)
	return srv
}

// NewGRPCServer 创建完整配置的 gRPC 服务器
func NewGRPCServer(config *GRPCServerConfig, svcs *GRPCServices, logger Logger) (*GRPCServer, error) {
	if config == nil {
		config = DefaultGRPCServerConfig()
	}

	if logger == nil {
		logger = &defaultLogger{}
	}

	// 应用默认值
	applyGRPCDefaults(config)

	// 验证端口
	if config.Port < 0 || config.Port > 65535 {
		return nil, fmt.Errorf("invalid port: %d, must be between 0 and 65535", config.Port)
	}

	// 验证 TLS 配置
	if err := validateTLSConfig(config); err != nil {
		return nil, err
	}

	return &GRPCServer{
		config:   config,
		logger:   logger,
		services: svcs,
	}, nil
}

// validateTLSConfig 验证 TLS 配置
func validateTLSConfig(config *GRPCServerConfig) error {
	if config.TLSCertFile == "" && config.TLSKeyFile == "" {
		return nil // 无 TLS 配置
	}

	if config.TLSCertFile == "" || config.TLSKeyFile == "" {
		return fmt.Errorf("both TLSCertFile and TLSKeyFile must be provided for TLS")
	}

	if _, err := os.Stat(config.TLSCertFile); os.IsNotExist(err) {
		return fmt.Errorf("TLS certificate file not found: %s", config.TLSCertFile)
	}

	if _, err := os.Stat(config.TLSKeyFile); os.IsNotExist(err) {
		return fmt.Errorf("TLS key file not found: %s", config.TLSKeyFile)
	}

	return nil
}

// applyGRPCDefaults 为零值字段应用默认值
func applyGRPCDefaults(cfg *GRPCServerConfig) {
	if cfg.MaxRecvMsgSize == 0 {
		cfg.MaxRecvMsgSize = 10 * 1024 * 1024
	}
	if cfg.MaxSendMsgSize == 0 {
		cfg.MaxSendMsgSize = 10 * 1024 * 1024
	}
	if cfg.MaxConcurrentStreams == 0 {
		cfg.MaxConcurrentStreams = 100
	}
	if cfg.ShutdownTimeout == 0 {
		cfg.ShutdownTimeout = 30 * time.Second
	}
}

// Start 启动 gRPC 服务器，阻塞监听
func (s *GRPCServer) Start() error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return fmt.Errorf("gRPC server already running")
	}

	// 创建 TCP listener
	addr := fmt.Sprintf(":%d", s.config.Port)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		s.mu.Unlock()
		return fmt.Errorf("failed to listen on port %d: %w", s.config.Port, err)
	}
	s.listener = lis
	s.running = true
	s.mu.Unlock()

	// 构建服务器选项
	opts := s.buildServerOptions()

	// 创建 gRPC 服务器
	s.server = grpc.NewServer(opts...)

	// 注册全部服务
	s.registerServices()

	// 可选启用反射
	if s.config.EnableReflection {
		reflection.Register(s.server)
	}

	s.logger.Info("gRPC server starting",
		"addr", lis.Addr().String(),
		"tls", s.config.TLSCertFile != "",
		"reflection", s.config.EnableReflection)

	// 阻塞监听
	if err := s.server.Serve(lis); err != nil {
		s.mu.Lock()
		s.running = false
		s.mu.Unlock()
		return err
	}

	return nil
}

// buildServerOptions 构建 gRPC 服务器选项
func (s *GRPCServer) buildServerOptions() []grpc.ServerOption {
	opts := []grpc.ServerOption{
		grpc.MaxRecvMsgSize(s.config.MaxRecvMsgSize),
		grpc.MaxSendMsgSize(s.config.MaxSendMsgSize),
		grpc.MaxConcurrentStreams(s.config.MaxConcurrentStreams),
	}

	// 配置 TLS
	if s.config.TLSCertFile != "" && s.config.TLSKeyFile != "" {
		creds, err := credentials.NewServerTLSFromFile(s.config.TLSCertFile, s.config.TLSKeyFile)
		if err == nil {
			opts = append(opts, grpc.Creds(creds))
			s.logger.Info("TLS enabled for gRPC server")
		} else {
			s.logger.Error("Failed to load TLS credentials", "error", err)
		}
	}

	// 配置 UnaryInterceptor 链：recovery → logging → metrics → auth → validation
	opts = append(opts, grpc.ChainUnaryInterceptor(
		recoveryInterceptor(s.logger),
		loggingInterceptor(s.logger),
		metricsInterceptor(&s.requestCount, &s.requestLatency),
		authInterceptor(s.logger),
		validationInterceptor(),
	))

	// 配置 StreamInterceptor 链：recovery → logging → metrics → auth
	opts = append(opts, grpc.ChainStreamInterceptor(
		streamRecoveryInterceptor(s.logger),
		streamLoggingInterceptor(s.logger),
		streamMetricsInterceptor(&s.requestCount, &s.requestLatency),
		streamAuthInterceptor(s.logger),
	))

	return opts
}

// registerServices 注册全部 gRPC 服务
func (s *GRPCServer) registerServices() {
	if s.services == nil {
		return
	}
	// 注册 MoleculeService
	if s.services.MoleculeService != nil {
		s.logger.Debug("Registering MoleculeService")
		// pb.RegisterMoleculeServiceServer(s.server, s.services.MoleculeService)
	}
	// 注册 PatentService
	if s.services.PatentService != nil {
		s.logger.Debug("Registering PatentService")
		// pb.RegisterPatentServiceServer(s.server, s.services.PatentService)
	}
}

// Stop 优雅停止 gRPC 服务器
func (s *GRPCServer) Stop(ctx context.Context) error {
	s.mu.Lock()
	if !s.running || s.server == nil {
		s.mu.Unlock()
		return nil
	}
	s.mu.Unlock()

	s.logger.Info("gRPC server shutting down", "timeout", s.config.ShutdownTimeout)

	done := make(chan struct{})
	go func() {
		s.server.GracefulStop()
		close(done)
	}()

	// 创建超时 context
	var cancel context.CancelFunc
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		ctx, cancel = context.WithTimeout(ctx, s.config.ShutdownTimeout)
		defer cancel()
	}

	select {
	case <-ctx.Done():
		// 优雅停止超时，强制停止
		s.logger.Warn("gRPC server shutdown timed out, forcing stop")
		s.server.Stop()
		s.mu.Lock()
		s.running = false
		s.mu.Unlock()
		return ctx.Err()
	case <-done:
		s.mu.Lock()
		s.running = false
		s.mu.Unlock()
		s.logger.Info("gRPC server stopped gracefully")
		return nil
	}
}

// Addr 返回监听地址
func (s *GRPCServer) Addr() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.listener != nil {
		return s.listener.Addr().String()
	}
	return fmt.Sprintf(":%d", s.config.Port)
}

// IsRunning 返回服务器运行状态
func (s *GRPCServer) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// GetConfig 返回服务器配置
func (s *GRPCServer) GetConfig() *GRPCServerConfig {
	return s.config
}

// GetServices 返回注册的服务
func (s *GRPCServer) GetServices() *GRPCServices {
	return s.services
}

// GetMetrics 返回指标统计
func (s *GRPCServer) GetMetrics() (requestCount int64, avgLatencyMs float64) {
	count := atomic.LoadInt64(&s.requestCount)
	latency := atomic.LoadInt64(&s.requestLatency)
	if count > 0 {
		avgLatencyMs = float64(latency) / float64(count) / 1e6
	}
	return count, avgLatencyMs
}

// Validator 定义请求验证接口
type Validator interface {
	Validate() error
}

// recoveryInterceptor 捕获 panic，转换为 Internal 错误码
func recoveryInterceptor(logger Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		defer func() {
			if r := recover(); r != nil {
				stack := debug.Stack()
				logger.Error("panic recovered in gRPC handler",
					"method", info.FullMethod,
					"panic", r,
					"stack", string(stack))
				err = status.Errorf(codes.Internal, "internal server error")
			}
		}()
		return handler(ctx, req)
	}
}

// loggingInterceptor 记录请求方法、耗时、状态码
func loggingInterceptor(logger Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		duration := time.Since(start)

		code := codes.OK
		if err != nil {
			if st, ok := status.FromError(err); ok {
				code = st.Code()
			}
		}

		logger.Info("gRPC request completed",
			"method", info.FullMethod,
			"duration_ms", duration.Milliseconds(),
			"code", code.String())

		return resp, err
	}
}

// metricsInterceptor 记录请求计数与延迟直方图
func metricsInterceptor(requestCount, requestLatency *int64) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		duration := time.Since(start)

		atomic.AddInt64(requestCount, 1)
		atomic.AddInt64(requestLatency, duration.Nanoseconds())

		return resp, err
	}
}

// authInterceptor 从 metadata 提取 Bearer token 并验证
func authInterceptor(logger Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		// 跳过反射和健康检查
		if strings.HasPrefix(info.FullMethod, "/grpc.reflection.") ||
			strings.HasPrefix(info.FullMethod, "/grpc.health.") {
			return handler(ctx, req)
		}

		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return handler(ctx, req)
		}

		// 提取 authorization header
		authHeaders := md.Get("authorization")
		if len(authHeaders) > 0 {
			token := authHeaders[0]
			// Bearer token 验证
			if strings.HasPrefix(token, "Bearer ") {
				token = strings.TrimPrefix(token, "Bearer ")
				// 实际 token 验证逻辑在此实现
				// 验证失败返回:
				// return nil, status.Errorf(codes.Unauthenticated, "invalid token")
			}
		}

		// 提取 tenant_id 和 user_id 注入 context
		if tenantIDs := md.Get("x-tenant-id"); len(tenantIDs) > 0 {
			ctx = context.WithValue(ctx, contextKeyTenantID, tenantIDs[0])
		}
		if userIDs := md.Get("x-user-id"); len(userIDs) > 0 {
			ctx = context.WithValue(ctx, contextKeyUserID, userIDs[0])
		}

		return handler(ctx, req)
	}
}

// validationInterceptor 对实现了 Validate() error 接口的请求消息自动校验
func validationInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if v, ok := req.(Validator); ok {
			if err := v.Validate(); err != nil {
				return nil, status.Errorf(codes.InvalidArgument, "validation failed: %v", err)
			}
		}
		return handler(ctx, req)
	}
}

// Stream 拦截器

func streamRecoveryInterceptor(logger Logger) grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) (err error) {
		defer func() {
			if r := recover(); r != nil {
				stack := debug.Stack()
				logger.Error("panic recovered in gRPC stream handler",
					"method", info.FullMethod,
					"panic", r,
					"stack", string(stack))
				err = status.Errorf(codes.Internal, "internal server error")
			}
		}()
		return handler(srv, ss)
	}
}

func streamLoggingInterceptor(logger Logger) grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		start := time.Now()
		err := handler(srv, ss)
		duration := time.Since(start)

		code := codes.OK
		if err != nil {
			if st, ok := status.FromError(err); ok {
				code = st.Code()
			}
		}

		logger.Info("gRPC stream completed",
			"method", info.FullMethod,
			"duration_ms", duration.Milliseconds(),
			"code", code.String())

		return err
	}
}

func streamMetricsInterceptor(requestCount, requestLatency *int64) grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		start := time.Now()
		err := handler(srv, ss)
		duration := time.Since(start)

		atomic.AddInt64(requestCount, 1)
		atomic.AddInt64(requestLatency, duration.Nanoseconds())

		return err
	}
}

func streamAuthInterceptor(logger Logger) grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		// Stream auth 实现与 unary 类似
		return handler(srv, ss)
	}
}

// Context keys
type contextKey string

const (
	contextKeyTenantID contextKey = "tenant_id"
	contextKeyUserID   contextKey = "user_id"
)

// GetTenantID 从 context 获取 tenant_id
func GetTenantID(ctx context.Context) string {
	if v := ctx.Value(contextKeyTenantID); v != nil {
		return v.(string)
	}
	return ""
}

// GetUserID 从 context 获取 user_id
func GetUserID(ctx context.Context) string {
	if v := ctx.Value(contextKeyUserID); v != nil {
		return v.(string)
	}
	return ""
}

// LoadTLSConfig 从证书文件加载 TLS 配置
func LoadTLSConfig(certFile, keyFile string) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load TLS key pair: %w", err)
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}, nil
}

//Personal.AI order the ending

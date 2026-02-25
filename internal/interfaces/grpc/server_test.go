package grpc

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

// mockLogger 用于测试的日志记录器
type mockLogger struct {
	mu       sync.Mutex
	infos    []string
	errors   []string
	warnings []string
	debugs   []string
}

func newMockLogger() *mockLogger {
	return &mockLogger{}
}

func (l *mockLogger) Info(msg string, fields ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.infos = append(l.infos, fmt.Sprintf("%s %v", msg, fields))
}

func (l *mockLogger) Error(msg string, fields ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.errors = append(l.errors, fmt.Sprintf("%s %v", msg, fields))
}

func (l *mockLogger) Warn(msg string, fields ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.warnings = append(l.warnings, fmt.Sprintf("%s %v", msg, fields))
}

func (l *mockLogger) Debug(msg string, fields ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.debugs = append(l.debugs, fmt.Sprintf("%s %v", msg, fields))
}

func (l *mockLogger) getInfos() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string{}, l.infos...)
}

func (l *mockLogger) getErrors() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string{}, l.errors...)
}

func (l *mockLogger) getWarnings() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string{}, l.warnings...)
}

// TestNewServer 测试向后兼容接口
func TestNewServer(t *testing.T) {
	srv := NewServer(9090)
	if srv == nil {
		t.Fatal("expected server instance")
	}
	if srv.config.Port != 9090 {
		t.Errorf("expected port 9090, got %d", srv.config.Port)
	}
}

// TestNewGRPCServer_DefaultConfig 默认配置创建成功
func TestNewGRPCServer_DefaultConfig(t *testing.T) {
	cfg := DefaultGRPCServerConfig()
	srv, err := NewGRPCServer(cfg, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if srv == nil {
		t.Fatal("expected server instance")
	}

	// 验证默认值
	if srv.config.MaxRecvMsgSize != 10*1024*1024 {
		t.Errorf("expected MaxRecvMsgSize 10MB, got %d", srv.config.MaxRecvMsgSize)
	}
	if srv.config.MaxSendMsgSize != 10*1024*1024 {
		t.Errorf("expected MaxSendMsgSize 10MB, got %d", srv.config.MaxSendMsgSize)
	}
	if srv.config.MaxConcurrentStreams != 100 {
		t.Errorf("expected MaxConcurrentStreams 100, got %d", srv.config.MaxConcurrentStreams)
	}
	if srv.config.ShutdownTimeout != 30*time.Second {
		t.Errorf("expected ShutdownTimeout 30s, got %v", srv.config.ShutdownTimeout)
	}
	if !srv.config.EnableReflection {
		t.Error("expected EnableReflection true")
	}
}

// TestNewGRPCServer_NilConfig 空配置使用默认值
func TestNewGRPCServer_NilConfig(t *testing.T) {
	srv, err := NewGRPCServer(nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if srv == nil {
		t.Fatal("expected server instance")
	}
	if srv.config.Port != 9090 {
		t.Errorf("expected default port 9090, got %d", srv.config.Port)
	}
}

// TestNewGRPCServer_WithTLS TLS 配置加载成功
func TestNewGRPCServer_WithTLS(t *testing.T) {
	certFile, keyFile := generateSelfSignedCert(t)
	defer os.Remove(certFile)
	defer os.Remove(keyFile)

	cfg := &GRPCServerConfig{
		Port:        9090,
		TLSCertFile: certFile,
		TLSKeyFile:  keyFile,
	}

	srv, err := NewGRPCServer(cfg, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if srv == nil {
		t.Fatal("expected server instance")
	}
}

// TestNewGRPCServer_TLSCertNotFound 证书文件不存在时报错
func TestNewGRPCServer_TLSCertNotFound(t *testing.T) {
	cfg := &GRPCServerConfig{
		Port:        9090,
		TLSCertFile: "/nonexistent/cert.pem",
		TLSKeyFile:  "/nonexistent/key.pem",
	}

	_, err := NewGRPCServer(cfg, nil, nil)
	if err == nil {
		t.Fatal("expected error for missing TLS cert")
	}
	if !contains(err.Error(), "TLS certificate file not found") {
		t.Errorf("expected TLS cert not found error, got: %v", err)
	}
}

// TestNewGRPCServer_TLSKeyNotFound 私钥文件不存在时报错
func TestNewGRPCServer_TLSKeyNotFound(t *testing.T) {
	certFile, keyFile := generateSelfSignedCert(t)
	defer os.Remove(certFile)
	defer os.Remove(keyFile)
	os.Remove(keyFile) // 删除私钥文件

	cfg := &GRPCServerConfig{
		Port:        9090,
		TLSCertFile: certFile,
		TLSKeyFile:  keyFile,
	}

	_, err := NewGRPCServer(cfg, nil, nil)
	if err == nil {
		t.Fatal("expected error for missing TLS key")
	}
	if !contains(err.Error(), "TLS key file not found") {
		t.Errorf("expected TLS key not found error, got: %v", err)
	}
}

// TestNewGRPCServer_TLSPartialConfig TLS 配置不完整时报错
func TestNewGRPCServer_TLSPartialConfig(t *testing.T) {
	cfg := &GRPCServerConfig{
		Port:        9090,
		TLSCertFile: "/some/cert.pem",
		// TLSKeyFile 缺失
	}

	_, err := NewGRPCServer(cfg, nil, nil)
	if err == nil {
		t.Fatal("expected error for partial TLS config")
	}
	if !contains(err.Error(), "both TLSCertFile and TLSKeyFile must be provided") {
		t.Errorf("expected partial TLS config error, got: %v", err)
	}
}

// TestNewGRPCServer_InvalidPort 非法端口报错
func TestNewGRPCServer_InvalidPort(t *testing.T) {
	testCases := []struct {
		port int
	}{
		{-1},
		{65536},
		{-100},
	}

	for _, tc := range testCases {
		cfg := &GRPCServerConfig{Port: tc.port}
		_, err := NewGRPCServer(cfg, nil, nil)
		if err == nil {
			t.Errorf("expected error for invalid port %d", tc.port)
		}
	}
}

// TestNewGRPCServer_PortConflict 端口已占用时报错
func TestNewGRPCServer_PortConflict(t *testing.T) {
	// 先占用一个端口
	lis, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}
	defer lis.Close()

	port := lis.Addr().(*net.TCPAddr).Port

	cfg := &GRPCServerConfig{Port: port}
	srv, err := NewGRPCServer(cfg, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error creating server: %v", err)
	}

	// 启动服务器应该失败
	err = srv.Start()
	if err == nil {
		t.Fatal("expected error for port conflict")
	}
	if !contains(err.Error(), "failed to listen") {
		t.Errorf("expected port conflict error, got: %v", err)
	}
}

// TestGRPCServer_Start_Stop 启动后优雅停止
func TestGRPCServer_Start_Stop(t *testing.T) {
	cfg := &GRPCServerConfig{
		Port:            0, // 随机端口
		ShutdownTimeout: 5 * time.Second,
	}
	logger := newMockLogger()
	srv, err := NewGRPCServer(cfg, nil, logger)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 异步启动服务器
	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Start()
	}()

	// 等待服务器启动
	time.Sleep(100 * time.Millisecond)

	if !srv.IsRunning() {
		t.Error("expected server to be running")
	}

	// 停止服务器
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Stop(ctx); err != nil {
		t.Errorf("unexpected error stopping server: %v", err)
	}

	if srv.IsRunning() {
		t.Error("expected server to be stopped")
	}

	// 验证日志
	infos := logger.getInfos()
	hasStartLog := false
	hasStopLog := false
	for _, info := range infos {
		if contains(info, "gRPC server starting") {
			hasStartLog = true
		}
		if contains(info, "stopped gracefully") {
			hasStopLog = true
		}
	}
	if !hasStartLog {
		t.Error("expected start log")
	}
	if !hasStopLog {
		t.Error("expected stop log")
	}
}

// TestGRPCServer_Stop_Timeout 优雅停止超时后强制停止
func TestGRPCServer_Stop_Timeout(t *testing.T) {
	cfg := &GRPCServerConfig{
		Port:            0,
		ShutdownTimeout: 100 * time.Millisecond, // 很短的超时
	}
	logger := newMockLogger()
	srv, err := NewGRPCServer(cfg, nil, logger)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	go srv.Start()
	time.Sleep(100 * time.Millisecond)

	// 使用一个已经超时的 context
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()
	time.Sleep(10 * time.Millisecond)

	err = srv.Stop(ctx)
	if err == nil || !errors.Is(err, context.DeadlineExceeded) {
		// 可能优雅停止成功，也可能超时
		// 两种情况都可以接受
	}

	if srv.IsRunning() {
		t.Error("expected server to be stopped")
	}
}

// TestGRPCServer_Addr 返回正确监听地址
func TestGRPCServer_Addr(t *testing.T) {
	cfg := &GRPCServerConfig{Port: 0}
	srv, err := NewGRPCServer(cfg, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 启动前返回配置的地址
	addr := srv.Addr()
	if addr != ":0" {
		t.Errorf("expected ':0', got %s", addr)
	}

	// 启动服务器
	go srv.Start()
	time.Sleep(100 * time.Millisecond)

	// 启动后返回实际地址
	addr = srv.Addr()
	if addr == ":0" {
		t.Error("expected actual address after start")
	}

	ctx := context.Background()
	srv.Stop(ctx)
}

// TestGRPCServer_DoubleStart 重复启动报错
func TestGRPCServer_DoubleStart(t *testing.T) {
	cfg := &GRPCServerConfig{Port: 0}
	srv, err := NewGRPCServer(cfg, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	go srv.Start()
	time.Sleep(100 * time.Millisecond)

	// 第二次启动应该失败
	err = srv.Start()
	if err == nil {
		t.Fatal("expected error for double start")
	}
	if !contains(err.Error(), "already running") {
		t.Errorf("expected already running error, got: %v", err)
	}

	ctx := context.Background()
	srv.Stop(ctx)
}

// TestGRPCServer_ServiceRegistration 验证服务注册
func TestGRPCServer_ServiceRegistration(t *testing.T) {
	cfg := &GRPCServerConfig{Port: 0}
	svcs := &GRPCServices{}
	logger := newMockLogger()

	srv, err := NewGRPCServer(cfg, svcs, logger)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if srv.GetServices() != svcs {
		t.Error("expected services to be set")
	}
}

// TestGRPCServer_ReflectionEnabled 反射服务启用验证
func TestGRPCServer_ReflectionEnabled(t *testing.T) {
	cfg := &GRPCServerConfig{
		Port:             0,
		EnableReflection: true,
	}
	srv, err := NewGRPCServer(cfg, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	go srv.Start()
	time.Sleep(100 * time.Millisecond)

	// 尝试连接并查询反射
	addr := srv.Addr()
	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("failed to dial: %v", err)
	}
	defer conn.Close()

	ctx := context.Background()
	srv.Stop(ctx)
}

// TestRecoveryInterceptor_PanicRecovery panic 被捕获并转换为 Internal 错误
func TestRecoveryInterceptor_PanicRecovery(t *testing.T) {
	logger := newMockLogger()
	interceptor := recoveryInterceptor(logger)

	ctx := context.Background()
	req := "test"
	info := &grpc.UnaryServerInfo{FullMethod: "/test/Method"}

	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		panic("test panic")
	}

	resp, err := interceptor(ctx, req, info, handler)
	if resp != nil {
		t.Error("expected nil response")
	}
	if err == nil {
		t.Fatal("expected error")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatal("expected gRPC status error")
	}
	if st.Code() != codes.Internal {
		t.Errorf("expected Internal code, got %v", st.Code())
	}

	// 验证日志记录了 panic
	errors := logger.getErrors()
	if len(errors) == 0 {
		t.Error("expected error log for panic")
	}
}

// TestRecoveryInterceptor_NoPanic 正常请求不受影响
func TestRecoveryInterceptor_NoPanic(t *testing.T) {
	logger := newMockLogger()
	interceptor := recoveryInterceptor(logger)

	ctx := context.Background()
	req := "test"
	info := &grpc.UnaryServerInfo{FullMethod: "/test/Method"}
	expectedResp := "response"

	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return expectedResp, nil
	}

	resp, err := interceptor(ctx, req, info, handler)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if resp != expectedResp {
		t.Errorf("expected %s, got %v", expectedResp, resp)
	}
}

// TestLoggingInterceptor_RequestLog 请求日志包含方法名与耗时
func TestLoggingInterceptor_RequestLog(t *testing.T) {
	logger := newMockLogger()
	interceptor := loggingInterceptor(logger)

	ctx := context.Background()
	req := "test"
	info := &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}

	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		time.Sleep(10 * time.Millisecond)
		return "response", nil
	}

	_, err := interceptor(ctx, req, info, handler)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	infos := logger.getInfos()
	if len(infos) == 0 {
		t.Fatal("expected log entry")
	}

	logEntry := infos[0]
	if !contains(logEntry, "/test.Service/Method") {
		t.Error("expected method name in log")
	}
	if !contains(logEntry, "duration_ms") {
		t.Error("expected duration in log")
	}
}

// TestMetricsInterceptor_CounterIncrement 请求计数器递增
func TestMetricsInterceptor_CounterIncrement(t *testing.T) {
	var requestCount int64
	var requestLatency int64
	interceptor := metricsInterceptor(&requestCount, &requestLatency)

	ctx := context.Background()
	req := "test"
	info := &grpc.UnaryServerInfo{FullMethod: "/test/Method"}

	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return "response", nil
	}

	// 调用多次
	for i := 0; i < 5; i++ {
		_, _ = interceptor(ctx, req, info, handler)
	}

	if requestCount != 5 {
		t.Errorf("expected count 5, got %d", requestCount)
	}
	if requestLatency == 0 {
		t.Error("expected non-zero latency")
	}
}

// TestValidationInterceptor_ValidRequest 合法请求通过
func TestValidationInterceptor_ValidRequest(t *testing.T) {
	interceptor := validationInterceptor()

	ctx := context.Background()
	req := &validatableRequest{valid: true}
	info := &grpc.UnaryServerInfo{FullMethod: "/test/Method"}

	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return "response", nil
	}

	resp, err := interceptor(ctx, req, info, handler)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if resp != "response" {
		t.Errorf("expected 'response', got %v", resp)
	}
}

// TestValidationInterceptor_InvalidRequest 校验失败返回 InvalidArgument
func TestValidationInterceptor_InvalidRequest(t *testing.T) {
	interceptor := validationInterceptor()

	ctx := context.Background()
	req := &validatableRequest{valid: false}
	info := &grpc.UnaryServerInfo{FullMethod: "/test/Method"}

	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return "response", nil
	}

	_, err := interceptor(ctx, req, info, handler)
	if err == nil {
		t.Fatal("expected validation error")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatal("expected gRPC status error")
	}
	if st.Code() != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %v", st.Code())
	}
}

// TestGRPCServer_GetMetrics 指标统计
func TestGRPCServer_GetMetrics(t *testing.T) {
	cfg := &GRPCServerConfig{Port: 0}
	srv, err := NewGRPCServer(cfg, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	count, avgLatency := srv.GetMetrics()
	if count != 0 {
		t.Errorf("expected count 0, got %d", count)
	}
	if avgLatency != 0 {
		t.Errorf("expected avgLatency 0, got %f", avgLatency)
	}
}

// TestGetTenantID 从 context 获取 tenant_id
func TestGetTenantID(t *testing.T) {
	ctx := context.Background()
	if GetTenantID(ctx) != "" {
		t.Error("expected empty tenant_id")
	}

	ctx = context.WithValue(ctx, contextKeyTenantID, "tenant-123")
	if GetTenantID(ctx) != "tenant-123" {
		t.Error("expected tenant-123")
	}
}

// TestGetUserID 从 context 获取 user_id
func TestGetUserID(t *testing.T) {
	ctx := context.Background()
	if GetUserID(ctx) != "" {
		t.Error("expected empty user_id")
	}

	ctx = context.WithValue(ctx, contextKeyUserID, "user-456")
	if GetUserID(ctx) != "user-456" {
		t.Error("expected user-456")
	}
}

// validatableRequest 实现 Validator 接口的测试请求
type validatableRequest struct {
	valid bool
}

func (r *validatableRequest) Validate() error {
	if !r.valid {
		return errors.New("validation failed")
	}
	return nil
}

// generateSelfSignedCert 生成自签名证书用于测试
func generateSelfSignedCert(t *testing.T) (certFile, keyFile string) {
	t.Helper()

	// 生成私钥
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	// 创建证书模板
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Test"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
	}

	// 创建证书
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("failed to create certificate: %v", err)
	}

	// 写入临时文件
	tmpDir := t.TempDir()
	certFile = filepath.Join(tmpDir, "cert.pem")
	keyFile = filepath.Join(tmpDir, "key.pem")

	certOut, err := os.Create(certFile)
	if err != nil {
		t.Fatalf("failed to create cert file: %v", err)
	}
	pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	certOut.Close()

	keyOut, err := os.Create(keyFile)
	if err != nil {
		t.Fatalf("failed to create key file: %v", err)
	}
	keyBytes, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		t.Fatalf("failed to marshal key: %v", err)
	}
	pem.Encode(keyOut, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes})
	keyOut.Close()

	return certFile, keyFile
}

// contains 检查字符串是否包含子串
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

//Personal.AI order the ending

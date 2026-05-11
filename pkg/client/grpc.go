// Phase 13c — gRPC Client with Load Balancing (292/349)
// File: pkg/client/grpc.go
// gRPC 客户端封装：DNS 解析器（可配置刷新频率）、Round Robin 负载均衡、
// Keepalive 健康检查、断线自动重连。

package client

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/resolver"

	// 注册 round_robin 负载均衡器（需空白导入以触发 init）
	_ "google.golang.org/grpc/balancer/roundrobin"
)

// ---------------------------------------------------------------------------
// 默认常量
// ---------------------------------------------------------------------------

const (
	// defaultDNSRefreshInterval 是 DNS 解析器的默认刷新间隔（30 秒）。
	defaultDNSRefreshInterval = 30 * time.Second

	// defaultKeepaliveTime 是 Keepalive 心跳间隔。
	defaultKeepaliveTime = 10 * time.Second

	// defaultKeepaliveTimeout 是 Keepalive 心跳超时。
	defaultKeepaliveTimeout = 3 * time.Second

	// defaultMinConnectTimeout 是连接建立的最小超时时间。
	defaultMinConnectTimeout = 5 * time.Second

	// grpcDNSScheme 是自定义 DNS 解析器使用的 URI scheme。
	grpcDNSScheme = "keyip-dns"
)

// ---------------------------------------------------------------------------
// 类型定义
// ---------------------------------------------------------------------------

// GRPCClient 封装了一条支持负载均衡的 gRPC 连接。
//
// 功能：
//   - DNS 周期解析（可配置刷新频率，默认 30s）
//   - Round Robin 负载均衡
//   - Keepalive 心跳检测
//   - 断线自动重连（由 gRPC 框架内置支持）
type GRPCClient struct {
	conn   *grpc.ClientConn
	config GRPCClientConfig
	logger Logger
}

// GRPCClientConfig 定义了 GRPCClient 的配置参数。
type GRPCClientConfig struct {
	// Target 是 gRPC 服务端地址，格式 "host:port"。
	// 必填。
	Target string

	// DNSRefreshInterval 设置 DNS 解析器重新解析主机名的频率。
	// 默认：30s。
	DNSRefreshInterval time.Duration

	// EnableRoundRobin 是否启用 round_robin 负载均衡策略。
	// 为 false 时使用 gRPC 默认的 pick_first。
	// 默认：true。
	EnableRoundRobin bool

	// KeepaliveParams 配置客户端 Keepalive 参数。
	// 为 nil 时使用默认值（10s ping, 3s timeout, permit without stream）。
	KeepaliveParams *KeepaliveConfig

	// Logger 日志记录器。为 nil 时静默丢弃。
	Logger Logger
}

// KeepaliveConfig 定义 gRPC 客户端 Keepalive 参数。
type KeepaliveConfig struct {
	Time                time.Duration // 心跳发送间隔
	Timeout             time.Duration // 心跳超时
	PermitWithoutStream bool          // 无活跃流时是否允许发送心跳
}

// ---------------------------------------------------------------------------
// 构造函数
// ---------------------------------------------------------------------------

// NewGRPCClient 创建一条支持负载均衡的 gRPC 客户端连接。
//
// 该客户端使用自定义 DNS 解析器以指定频率刷新后端地址列表，
// 使用 round_robin 在不同端点间分发请求，
// 通过 Keepalive 心跳检测失效连接，
// 并依赖 gRPC 内建的重连逻辑自动恢复连接。
func NewGRPCClient(cfg GRPCClientConfig) (*GRPCClient, error) {
	if cfg.Target == "" {
		return nil, fmt.Errorf("gRPC target 不能为空")
	}

	logger := cfg.Logger
	if logger == nil {
		logger = noopLogger{}
	}

	// ---- 默认值 ----
	dnsFreq := cfg.DNSRefreshInterval
	if dnsFreq <= 0 {
		dnsFreq = defaultDNSRefreshInterval
	}

	// ---- 自定义 DNS 解析器 ----
	builder := &periodicDNSResolverBuilder{
		scheme: grpcDNSScheme,
		freq:   dnsFreq,
		logger: logger,
	}

	// ---- DialOption 组装 ----
	dialOpts := []grpc.DialOption{
		grpc.WithResolvers(builder),
		grpc.WithConnectParams(grpc.ConnectParams{
			Backoff:           backoff.DefaultConfig,
			MinConnectTimeout: defaultMinConnectTimeout,
		}),
	}

	// 传输层安全
	dialOpts = append(dialOpts, grpc.WithTransportCredentials(insecure.NewCredentials()))

	// ---- Round Robin 负载均衡 ----
	if cfg.EnableRoundRobin {
		dialOpts = append(dialOpts, grpc.WithDefaultServiceConfig(`{
			"loadBalancingConfig": [{"round_robin": {}}]
		}`))
	}

	// ---- Keepalive ----
	ka := cfg.KeepaliveParams
	if ka == nil {
		ka = &KeepaliveConfig{
			Time:                defaultKeepaliveTime,
			Timeout:             defaultKeepaliveTimeout,
			PermitWithoutStream: true,
		}
	}
	dialOpts = append(dialOpts, grpc.WithKeepaliveParams(keepalive.ClientParameters{
		Time:                ka.Time,
		Timeout:             ka.Timeout,
		PermitWithoutStream: ka.PermitWithoutStream,
	}))

	// ---- 拨号 ----
	target := fmt.Sprintf("%s:///%s", grpcDNSScheme, cfg.Target)
	conn, err := grpc.Dial(target, dialOpts...)
	if err != nil {
		return nil, fmt.Errorf("gRPC 拨号失败 target=%s: %w", cfg.Target, err)
	}

	logger.Infof("gRPC 客户端已创建 target=%s dns_freq=%v round_robin=%v",
		cfg.Target, dnsFreq, cfg.EnableRoundRobin)

	return &GRPCClient{
		conn:   conn,
		config: cfg,
		logger: logger,
	}, nil
}

// ---------------------------------------------------------------------------
// 公共方法
// ---------------------------------------------------------------------------

// Conn 返回底层 *grpc.ClientConn，供 protobuf 客户端使用。
func (c *GRPCClient) Conn() *grpc.ClientConn { return c.conn }

// Close 优雅关闭 gRPC 连接。可安全重复调用。
func (c *GRPCClient) Close() error { return c.conn.Close() }

// GetState 返回当前连接状态。
func (c *GRPCClient) GetState() connectivity.State { return c.conn.GetState() }

// WaitForReady 阻塞直到连接变为 READY 或 ctx 被取消。
func (c *GRPCClient) WaitForReady(ctx context.Context) error {
	for {
		s := c.conn.GetState()
		if s == connectivity.Ready {
			return nil
		}
		if !c.conn.WaitForStateChange(ctx, s) {
			return ctx.Err()
		}
	}
}

// ---------------------------------------------------------------------------
// 自定义 DNS 解析器 — 周期性主机名解析
// ---------------------------------------------------------------------------

// dnsResolver 实现 resolver.Resolver 接口，通过 net.LookupHost 周期解析目标
// 主机名并将发现的地址推入 gRPC 负载均衡器，实现动态服务发现。
type dnsResolver struct {
	target string
	port   string
	freq   time.Duration
	cc     resolver.ClientConn
	logger Logger

	stopCh chan struct{}
	doneCh chan struct{}
}

// loop 定期执行 DNS 解析。
// 首次解析在 Build 中同步执行，loop 只负责后续周期刷新。
func (r *dnsResolver) loop() {
	ticker := time.NewTicker(r.freq)
	defer ticker.Stop()
	defer close(r.doneCh)

	for {
		select {
		case <-r.stopCh:
			return
		case <-ticker.C:
			r.resolve()
		}
	}
}

// resolve 执行一次 DNS 查询并将结果推送到 gRPC resolver.ClientConn。
func (r *dnsResolver) resolve() {
	ips, err := net.LookupHost(r.target)
	if err != nil {
		r.logger.Errorf("DNS 解析失败 target=%s: %v", r.target, err)
		r.cc.ReportError(err)
		return
	}

	if len(ips) == 0 {
		r.logger.Errorf("DNS 解析返回空列表 target=%s", r.target)
		return
	}

	addrs := make([]resolver.Address, 0, len(ips))
	for _, ip := range ips {
		addrs = append(addrs, resolver.Address{Addr: net.JoinHostPort(ip, r.port)})
	}

	if err := r.cc.UpdateState(resolver.State{Addresses: addrs}); err != nil {
		r.logger.Errorf("更新解析状态失败 target=%s: %v", r.target, err)
	} else {
		r.logger.Debugf("DNS 解析完成 target=%s -> %d 个地址", r.target, len(addrs))
	}
}

// ResolveNow 实现 resolver.Resolver 接口，立即触发一次解析。
func (r *dnsResolver) ResolveNow(_ resolver.ResolveNowOptions) { r.resolve() }

// Close 实现 resolver.Resolver 接口，停止解析循环并等待退出。
func (r *dnsResolver) Close() {
	close(r.stopCh)
	<-r.doneCh
}

// periodicDNSResolverBuilder 创建 dnsResolver 实例，每个 scheme 一个。
type periodicDNSResolverBuilder struct {
	scheme string
	freq   time.Duration
	logger Logger
}

// Build 实现 resolver.Builder 接口。解析 URI 中的目标地址，创建解析器并执
// 行首次同步解析，随后启动后台周期解析。
func (b *periodicDNSResolverBuilder) Build(target resolver.Target, cc resolver.ClientConn, _ resolver.BuildOptions) (resolver.Resolver, error) {
	endpoint := strings.TrimPrefix(target.Endpoint(), "/")
	host, port, err := net.SplitHostPort(endpoint)
	if err != nil {
		return nil, fmt.Errorf("gRPC 目标格式无效 %q：必须为 host:port 格式", endpoint)
	}

	r := &dnsResolver{
		target: host,
		port:   port,
		freq:   b.freq,
		cc:     cc,
		logger: b.logger,
		stopCh: make(chan struct{}),
		doneCh: make(chan struct{}),
	}

	// 首次同步解析，确保启动时立即获得地址
	r.resolve()

	// 启动后台周期解析
	go r.loop()
	return r, nil
}

// Scheme 实现 resolver.Builder 接口。
func (b *periodicDNSResolverBuilder) Scheme() string { return b.scheme }

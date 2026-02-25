// ---
//
// 继续输出 285 `internal/interfaces/http/server_test.go` 要实现 HTTP 服务器单元测试。
//
// 实现要求:
//
// * **功能定位**：验证 Server 的创建、启动、优雅关闭、TLS 模式、超时配置、并发安全
// * **测试用例**：
//   * `TestNewServer_DefaultConfig`：零值配置应用默认值
//   * `TestNewServer_CustomConfig`：自定义配置正确传递
//   * `TestServer_StartAndShutdown`：正常启动和优雅关闭
//   * `TestServer_StartWithEphemeralPort`：端口 0 分配临时端口
//   * `TestServer_DoubleStart_Error`：重复启动返回错误
//   * `TestServer_ShutdownBeforeStart`：未启动时关闭不报错
//   * `TestServer_IsRunning`：运行状态正确反映
//   * `TestServer_Addr_AfterStart`：启动后返回实际地址
//   * `TestServer_TLSEnabled`：TLS 配置检测
//   * `TestServer_GracefulShutdown_WaitsForActiveRequests`：优雅关闭等待活跃请求
//   * `TestServer_ShutdownTimeout_ForcesClose`：超时后强制关闭
//   * `TestServerConfig_ApplyDefaults`：默认值填充逻辑
//   * `TestServerConfig_IsTLSEnabled`：TLS 启用判断
//   * `TestServerConfig_ListenAddr`：地址格式化
// * **Mock 依赖**：stubLogger、slowHandler（模拟慢请求）
// * **断言验证**：状态码、地址格式、超时行为、并发安全
// * **强制约束**：文件最后一行必须为 `//Personal.AI order the ending`
//
// ---
package http

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- ServerConfig unit tests ---

func TestServerConfig_ApplyDefaults(t *testing.T) {
	cfg := ServerConfig{}
	cfg.applyDefaults()

	assert.Equal(t, defaultHost, cfg.Host)
	assert.Equal(t, defaultPort, cfg.Port)
	assert.Equal(t, defaultReadTimeout, cfg.ReadTimeout)
	assert.Equal(t, defaultWriteTimeout, cfg.WriteTimeout)
	assert.Equal(t, defaultIdleTimeout, cfg.IdleTimeout)
	assert.Equal(t, defaultReadHeaderTimeout, cfg.ReadHeaderTimeout)
	assert.Equal(t, defaultMaxHeaderBytes, cfg.MaxHeaderBytes)
	assert.Equal(t, defaultShutdownTimeout, cfg.ShutdownTimeout)
}

func TestServerConfig_ApplyDefaults_PreservesCustomValues(t *testing.T) {
	cfg := ServerConfig{
		Host:            "127.0.0.1",
		Port:            9090,
		ReadTimeout:     5 * time.Second,
		WriteTimeout:    10 * time.Second,
		IdleTimeout:     60 * time.Second,
		ShutdownTimeout: 15 * time.Second,
	}
	cfg.applyDefaults()

	assert.Equal(t, "127.0.0.1", cfg.Host)
	assert.Equal(t, 9090, cfg.Port)
	assert.Equal(t, 5*time.Second, cfg.ReadTimeout)
	assert.Equal(t, 10*time.Second, cfg.WriteTimeout)
	assert.Equal(t, 60*time.Second, cfg.IdleTimeout)
	assert.Equal(t, 15*time.Second, cfg.ShutdownTimeout)
}

func TestServerConfig_IsTLSEnabled(t *testing.T) {
	tests := []struct {
		name     string
		cert     string
		key      string
		expected bool
	}{
		{"both set", "/path/cert.pem", "/path/key.pem", true},
		{"cert only", "/path/cert.pem", "", false},
		{"key only", "", "/path/key.pem", false},
		{"neither set", "", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := ServerConfig{TLSCertFile: tt.cert, TLSKeyFile: tt.key}
			assert.Equal(t, tt.expected, cfg.isTLSEnabled())
		})
	}
}

func TestServerConfig_ListenAddr(t *testing.T) {
	cfg := ServerConfig{Host: "192.168.1.1", Port: 3000}
	assert.Equal(t, "192.168.1.1:3000", cfg.listenAddr())
}

func TestServerConfig_ListenAddr_Default(t *testing.T) {
	cfg := ServerConfig{}
	cfg.applyDefaults()
	assert.Equal(t, "0.0.0.0:8080", cfg.listenAddr())
}

// --- Server creation tests ---

func TestNewServer_DefaultConfig(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	logger := &stubLogger{}

	srv := NewServer(ServerConfig{}, handler, logger)

	assert.NotNil(t, srv)
	assert.Equal(t, defaultHost, srv.config.Host)
	assert.Equal(t, defaultPort, srv.config.Port)
	assert.Equal(t, defaultReadTimeout, srv.config.ReadTimeout)
	assert.False(t, srv.IsRunning())
}

func TestNewServer_CustomConfig(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	logger := &stubLogger{}

	cfg := ServerConfig{
		Host:         "127.0.0.1",
		Port:         9999,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	srv := NewServer(cfg, handler, logger)

	assert.Equal(t, "127.0.0.1", srv.config.Host)
	assert.Equal(t, 9999, srv.config.Port)
	assert.Equal(t, 5*time.Second, srv.config.ReadTimeout)
	assert.Equal(t, 10*time.Second, srv.config.WriteTimeout)
	// Defaults should be applied for unset fields
	assert.Equal(t, defaultIdleTimeout, srv.config.IdleTimeout)
}

func TestNewServer_TLSConfig(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	logger := &stubLogger{}

	cfg := ServerConfig{
		TLSCertFile: "/path/to/cert.pem",
		TLSKeyFile:  "/path/to/key.pem",
	}

	srv := NewServer(cfg, handler, logger)

	assert.True(t, srv.config.isTLSEnabled())
	assert.NotNil(t, srv.httpServer.TLSConfig)
}

// --- Server lifecycle tests ---

func TestServer_StartAndShutdown(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	logger := &stubLogger{}

	srv := NewServer(ServerConfig{
		Host: "127.0.0.1",
		Port: 0, // ephemeral port
	}, handler, logger)

	ctx, cancel := context.WithCancel(context.Background())

	// Start server in background
	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Start(ctx)
	}()

	// Wait for server to be ready
	require.Eventually(t, func() bool {
		return srv.IsRunning()
	}, 2*time.Second, 10*time.Millisecond)

	// Make a request
	addr := srv.Addr()
	require.NotEmpty(t, addr)

	resp, err := http.Get(fmt.Sprintf("http://%s/", addr))
	require.NoError(t, err)
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "ok", string(body))

	// Shutdown
	cancel()

	select {
	case err := <-errCh:
		assert.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("server did not shut down in time")
	}

	assert.False(t, srv.IsRunning())
}

func TestServer_StartWithEphemeralPort(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	logger := &stubLogger{}

	srv := NewServer(ServerConfig{
		Host: "127.0.0.1",
		Port: 0,
	}, handler, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = srv.Start(ctx)
	}()

	require.Eventually(t, func() bool {
		return srv.IsRunning()
	}, 2*time.Second, 10*time.Millisecond)

	addr := srv.Addr()
	assert.NotEmpty(t, addr)
	assert.NotContains(t, addr, ":0",
		"ephemeral port should be resolved to actual port")

	cancel()
}

func TestServer_DoubleStart_Error(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	logger := &stubLogger{}

	srv := NewServer(ServerConfig{
		Host: "127.0.0.1",
		Port: 0,
	}, handler, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = srv.Start(ctx)
	}()

	require.Eventually(t, func() bool {
		return srv.IsRunning()
	}, 2*time.Second, 10*time.Millisecond)

	// Second start should fail
	err := srv.Start(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already started")

	cancel()
}

func TestServer_ShutdownBeforeStart(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	logger := &stubLogger{}

	srv := NewServer(ServerConfig{}, handler, logger)

	err := srv.Shutdown(context.Background())
	assert.NoError(t, err, "shutdown before start should not error")
}

func TestServer_IsRunning(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	logger := &stubLogger{}

	srv := NewServer(ServerConfig{
		Host: "127.0.0.1",
		Port: 0,
	}, handler, logger)

	assert.False(t, srv.IsRunning(), "should not be running before start")

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		_ = srv.Start(ctx)
	}()

	require.Eventually(t, func() bool {
		return srv.IsRunning()
	}, 2*time.Second, 10*time.Millisecond)

	assert.True(t, srv.IsRunning(), "should be running after start")

	cancel()

	require.Eventually(t, func() bool {
		return !srv.IsRunning()
	}, 5*time.Second, 50*time.Millisecond)

	assert.False(t, srv.IsRunning(), "should not be running after shutdown")
}

func TestServer_Addr_AfterStart(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	logger := &stubLogger{}

	srv := NewServer(ServerConfig{
		Host: "127.0.0.1",
		Port: 0,
	}, handler, logger)

	assert.Empty(t, srv.Addr(), "addr should be empty before start")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = srv.Start(ctx)
	}()

	require.Eventually(t, func() bool {
		return srv.IsRunning()
	}, 2*time.Second, 10*time.Millisecond)

	addr := srv.Addr()
	assert.NotEmpty(t, addr)
	assert.Contains(t, addr, "127.0.0.1:")

	cancel()
}

func TestServer_GracefulShutdown_WaitsForActiveRequests(t *testing.T) {
	requestStarted := make(chan struct{})
	requestCanFinish := make(chan struct{})

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(requestStarted)
		<-requestCanFinish
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("completed"))
	})
	logger := &stubLogger{}

	srv := NewServer(ServerConfig{
		Host:            "127.0.0.1",
		Port:            0,
		ShutdownTimeout: 10 * time.Second,
	}, handler, logger)

	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Start(ctx)
	}()

	require.Eventually(t, func() bool {
		return srv.IsRunning()
	}, 2*time.Second, 10*time.Millisecond)

	// Start a slow request
	var resp *http.Response
	var reqErr error
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		resp, reqErr = http.Get(fmt.Sprintf("http://%s/slow", srv.Addr()))
	}()

	// Wait for the request to reach the handler
	<-requestStarted

	// Initiate shutdown while request is in-flight
	cancel()

	// Allow the request to complete
	time.Sleep(100 * time.Millisecond)
	close(requestCanFinish)

	// Wait for the request goroutine to finish
	wg.Wait()

	require.NoError(t, reqErr)
	if resp != nil {
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "completed", string(body))
	}

	// Wait for server to finish
	select {
	case err := <-errCh:
		assert.NoError(t, err)
	case <-time.After(15 * time.Second):
		t.Fatal("server did not shut down in time")
	}
}

func TestServer_ShutdownTimeout_ForcesClose(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate a request that takes forever
		select {
		case <-r.Context().Done():
			return
		case <-time.After(60 * time.Second):
			return
		}
	})
	logger := &stubLogger{}

	srv := NewServer(ServerConfig{
		Host:            "127.0.0.1",
		Port:            0,
		ShutdownTimeout: 500 * time.Millisecond, // Very short timeout
	}, handler, logger)

	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Start(ctx)
	}()

	require.Eventually(t, func() bool {
		return srv.IsRunning()
	}, 2*time.Second, 10*time.Millisecond)

	// Start a request that will hang
	go func() {
		resp, err := http.Get(fmt.Sprintf("http://%s/hang", srv.Addr()))
		if err == nil && resp != nil {
			resp.Body.Close()
		}
	}()

	// Give the request time to reach the handler
	time.Sleep(100 * time.Millisecond)

	// Trigger shutdown
	cancel()

	// Server should shut down within shutdown timeout + buffer
	select {
	case <-errCh:
		// Shutdown completed (may or may not have error due to forced close)
	case <-time.After(5 * time.Second):
		t.Fatal("server did not shut down even after timeout")
	}
}

func TestServer_Config(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	logger := &stubLogger{}

	cfg := ServerConfig{
		Host: "10.0.0.1",
		Port: 4444,
	}

	srv := NewServer(cfg, handler, logger)
	got := srv.Config()

	assert.Equal(t, "10.0.0.1", got.Host)
	assert.Equal(t, 4444, got.Port)
}

func TestServer_ConcurrentRequests(t *testing.T) {
	var counter int64
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Atomic increment to verify concurrency safety
		val := atomic.AddInt64(&counter, 1)
		time.Sleep(10 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "req-%d", val)
	})
	logger := &stubLogger{}

	srv := NewServer(ServerConfig{
		Host: "127.0.0.1",
		Port: 0,
	}, handler, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = srv.Start(ctx)
	}()

	require.Eventually(t, func() bool {
		return srv.IsRunning()
	}, 2*time.Second, 10*time.Millisecond)

	// Fire 50 concurrent requests
	const numRequests = 50
	var wg sync.WaitGroup
	results := make([]int, numRequests)

	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			resp, err := http.Get(fmt.Sprintf("http://%s/", srv.Addr()))
			if err != nil {
				results[idx] = -1
				return
			}
			defer resp.Body.Close()
			results[idx] = resp.StatusCode
		}(i)
	}

	wg.Wait()

	successCount := 0
	for _, code := range results {
		if code == http.StatusOK {
			successCount++
		}
	}

	assert.Equal(t, numRequests, successCount,
		"all concurrent requests should succeed")
	assert.Equal(t, int64(numRequests), atomic.LoadInt64(&counter),
		"handler should have been called exactly %d times", numRequests)

	cancel()
}

func TestServer_RequestAfterShutdown(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	logger := &stubLogger{}

	srv := NewServer(ServerConfig{
		Host: "127.0.0.1",
		Port: 0,
	}, handler, logger)

	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Start(ctx)
	}()

	require.Eventually(t, func() bool {
		return srv.IsRunning()
	}, 2*time.Second, 10*time.Millisecond)

	addr := srv.Addr()

	// Shutdown
	cancel()
	<-errCh

	// Request after shutdown should fail
	client := &http.Client{Timeout: 1 * time.Second}
	_, err := client.Get(fmt.Sprintf("http://%s/", addr))
	assert.Error(t, err, "request after shutdown should fail")
}

//Personal.AI order the ending


package http

import (
"context"
"net/http"
"testing"
"time"
)

func TestDefaultServerConfig(t *testing.T) {
cfg := DefaultServerConfig()
if cfg == nil {
t.Fatal("DefaultServerConfig should not return nil")
}
if cfg.Host != "0.0.0.0" {
t.Errorf("expected Host=0.0.0.0, got %s", cfg.Host)
}
if cfg.Port != 8080 {
t.Errorf("expected Port=8080, got %d", cfg.Port)
}
if cfg.ReadTimeout != 30*time.Second {
t.Errorf("expected ReadTimeout=30s, got %v", cfg.ReadTimeout)
}
}

func TestNewServer_Success(t *testing.T) {
handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
w.WriteHeader(http.StatusOK)
})

server, err := NewServer(nil, handler)
if err != nil {
t.Fatalf("NewServer failed: %v", err)
}
if server == nil {
t.Fatal("server should not be nil")
}
}

func TestNewServer_NilHandler(t *testing.T) {
_, err := NewServer(nil, nil)
if err == nil {
t.Error("expected error for nil handler")
}
}

func TestNewServer_InvalidPort(t *testing.T) {
handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
cfg := &ServerConfig{Port: -1}

_, err := NewServer(cfg, handler)
if err == nil {
t.Error("expected error for invalid port")
}
}

func TestNewServer_ApplyDefaults(t *testing.T) {
handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
cfg := &ServerConfig{} // All zero values

server, err := NewServer(cfg, handler)
if err != nil {
t.Fatalf("NewServer failed: %v", err)
}

if server.httpServer.ReadTimeout != 30*time.Second {
t.Error("ReadTimeout default not applied")
}
if server.httpServer.WriteTimeout != 60*time.Second {
t.Error("WriteTimeout default not applied")
}
}

func TestServer_StartAndShutdown(t *testing.T) {
handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
w.WriteHeader(http.StatusOK)
})

cfg := &ServerConfig{
Host: "127.0.0.1",
Port: 0, // Random port
}

server, err := NewServer(cfg, handler)
if err != nil {
t.Fatalf("NewServer failed: %v", err)
}

ctx, cancel := context.WithCancel(context.Background())
defer cancel()

go func() {
if err := server.Start(ctx); err != nil {
t.Logf("Start returned: %v", err)
}
}()

<-server.WaitForReady()

if !server.IsRunning() {
t.Error("server should be running")
}

// Make a test request
resp, err := http.Get("http://" + server.Addr() + "/")
if err != nil {
t.Fatalf("request failed: %v", err)
}
resp.Body.Close()

if resp.StatusCode != http.StatusOK {
t.Errorf("expected status 200, got %d", resp.StatusCode)
}

// Shutdown
if err := server.Shutdown(); err != nil {
t.Errorf("Shutdown failed: %v", err)
}

if server.IsRunning() {
t.Error("server should not be running after shutdown")
}
}

func TestServer_Shutdown_Idempotent(t *testing.T) {
handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
cfg := &ServerConfig{Port: 0}

server, err := NewServer(cfg, handler)
if err != nil {
t.Fatalf("NewServer failed: %v", err)
}

ctx, cancel := context.WithCancel(context.Background())
defer cancel()

go server.Start(ctx)
<-server.WaitForReady()

// First shutdown
err1 := server.Shutdown()
// Second shutdown
err2 := server.Shutdown()

// Should not panic, second call should be no-op
if err1 != nil {
t.Errorf("first Shutdown failed: %v", err1)
}
if err2 != nil {
t.Logf("second Shutdown returned: %v (expected, idempotent)", err2)
}
}

func TestServer_Addr_BeforeStart(t *testing.T) {
handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
cfg := &ServerConfig{Host: "127.0.0.1", Port: 8080}

server, err := NewServer(cfg, handler)
if err != nil {
t.Fatalf("NewServer failed: %v", err)
}

addr := server.Addr()
if addr != "127.0.0.1:8080" {
t.Errorf("expected addr=127.0.0.1:8080, got %s", addr)
}
}

func TestServer_Addr_AfterStart(t *testing.T) {
handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
cfg := &ServerConfig{Port: 0} // Random port

server, err := NewServer(cfg, handler)
if err != nil {
t.Fatalf("NewServer failed: %v", err)
}

ctx, cancel := context.WithCancel(context.Background())
defer cancel()

go server.Start(ctx)
<-server.WaitForReady()
defer server.Shutdown()

addr := server.Addr()
if addr == "" || addr == "0.0.0.0:0" {
t.Errorf("expected actual address, got %s", addr)
}
}

func TestServer_WaitForReady_Channel(t *testing.T) {
handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
cfg := &ServerConfig{Port: 0}

server, err := NewServer(cfg, handler)
if err != nil {
t.Fatalf("NewServer failed: %v", err)
}

ctx, cancel := context.WithCancel(context.Background())
defer cancel()

go server.Start(ctx)

// Wait for ready with timeout
select {
case <-server.WaitForReady():
// Success
case <-time.After(2 * time.Second):
t.Fatal("server did not become ready in time")
}

server.Shutdown()
}

func TestServer_DoubleStart(t *testing.T) {
handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
cfg := &ServerConfig{Port: 0}

server, err := NewServer(cfg, handler)
if err != nil {
t.Fatalf("NewServer failed: %v", err)
}

ctx, cancel := context.WithCancel(context.Background())
defer cancel()

go server.Start(ctx)
<-server.WaitForReady()
defer server.Shutdown()

// Try to start again
err = server.Start(ctx)
if err == nil {
t.Error("expected error when starting already running server")
}
}

func TestLogWriter_Write(t *testing.T) {
lw := &logWriter{}

n, err := lw.Write([]byte("test message\n"))
if err != nil {
t.Errorf("Write failed: %v", err)
}
if n != 13 {
t.Errorf("expected n=13, got %d", n)
}
}

func TestLogWriter_EmptyWrite(t *testing.T) {
lw := &logWriter{}

n, err := lw.Write([]byte(""))
if err != nil {
t.Errorf("Write failed: %v", err)
}
if n != 0 {
t.Errorf("expected n=0, got %d", n)
}
}

func TestNewServer_TLS_CertFileNotExist(t *testing.T) {
handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
cfg := &ServerConfig{
TLS: &TLSConfig{
Enabled:  true,
CertFile: "/nonexistent/cert.pem",
KeyFile:  "/nonexistent/key.pem",
},
}

_, err := NewServer(cfg, handler)
if err == nil {
t.Error("expected error for nonexistent cert file")
}
}

//Personal.AI order the ending

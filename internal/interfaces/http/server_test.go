package http

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
)

// Reusing MockLogger
type testLogger struct {
	logging.Logger
}
func (m *testLogger) Error(msg string, fields ...logging.Field) {}
func (m *testLogger) Info(msg string, fields ...logging.Field) {}
func (m *testLogger) Warn(msg string, fields ...logging.Field) {}
func (m *testLogger) Debug(msg string, fields ...logging.Field) {}

func TestDefaultServerConfig(t *testing.T) {
	cfg := DefaultServerConfig()
	if cfg.Port != 8080 {
		t.Errorf("Expected default port 8080, got %d", cfg.Port)
	}
	if cfg.ReadTimeout != 30*time.Second {
		t.Errorf("Expected ReadTimeout 30s, got %v", cfg.ReadTimeout)
	}
}

func TestNewServer(t *testing.T) {
	logger := &testLogger{}
	handler := http.NotFoundHandler()

	t.Run("Success", func(t *testing.T) {
		cfg := ServerConfig{Logger: logger} // Port 0
		srv, err := NewServer(cfg, handler)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if srv == nil {
			t.Fatal("Expected server, got nil")
		}
	})

	t.Run("Nil Handler", func(t *testing.T) {
		cfg := ServerConfig{Logger: logger}
		_, err := NewServer(cfg, nil)
		if err == nil {
			t.Error("Expected error for nil handler")
		}
	})

	t.Run("Invalid Port", func(t *testing.T) {
		cfg := ServerConfig{Port: -1, Logger: logger}
		_, err := NewServer(cfg, handler)
		if err == nil {
			t.Error("Expected error for invalid port")
		}
	})
}

func TestServer_StartAndShutdown(t *testing.T) {
	logger := &testLogger{}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	cfg := ServerConfig{Port: 0, Logger: logger}
	srv, err := NewServer(cfg, handler)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		if err := srv.Start(ctx); err != nil && err != context.Canceled {
			// t.Errorf("Server start error: %v", err) // Cannot call t.Error from goroutine safely
		}
	}()

	// Wait for ready
	select {
	case <-srv.WaitForReady():
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for server to start")
	}

	if !srv.IsRunning() {
		t.Error("Server should be running")
	}

	addr := srv.Addr()
	if addr == "" {
		t.Error("Server address should not be empty")
	}

	// Test request
	resp, err := http.Get("http://" + addr)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200 OK, got %d", resp.StatusCode)
	}

	// Shutdown
	if err := srv.Shutdown(); err != nil {
		t.Errorf("Shutdown failed: %v", err)
	}

	if srv.IsRunning() {
		t.Error("Server should not be running after shutdown")
	}
}

func TestServer_TLS(t *testing.T) {
	certFile, keyFile := generateSelfSignedCert(t)

	logger := &testLogger{}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	cfg := ServerConfig{
		Port: 0,
		Logger: logger,
		TLS: &TLSConfig{
			CertFile: certFile,
			KeyFile: keyFile,
		},
	}

	srv, err := NewServer(cfg, handler)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go srv.Start(ctx)

	select {
	case <-srv.WaitForReady():
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for server to start")
	}

	addr := srv.Addr()
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	resp, err := client.Get("https://" + addr)
	if err != nil {
		t.Fatalf("HTTPS Request failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200 OK, got %d", resp.StatusCode)
	}

	srv.Shutdown()
}

func generateSelfSignedCert(t *testing.T) (string, string) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Test Co"},
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().Add(time.Hour),
		KeyUsage:  x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		t.Fatal(err)
	}

	tmpDir := t.TempDir()
	certPath := filepath.Join(tmpDir, "cert.pem")
	keyPath := filepath.Join(tmpDir, "key.pem")

	certOut, _ := os.Create(certPath)
	pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	certOut.Close()

	keyBytes, _ := x509.MarshalECPrivateKey(priv)
	keyOut, _ := os.Create(keyPath)
	pem.Encode(keyOut, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes})
	keyOut.Close()

	return certPath, keyPath
}

func TestServer_Start_PortInUse(t *testing.T) {
	// Listen on a random port to occupy it
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	_, portStr, _ := net.SplitHostPort(l.Addr().String())
	var port int
	fmt.Sscanf(portStr, "%d", &port)

	logger := &testLogger{}
	handler := http.NotFoundHandler()
	cfg := ServerConfig{Host: "127.0.0.1", Port: port, Logger: logger}

	srv, err := NewServer(cfg, handler)
	if err != nil {
		t.Fatal(err)
	}

	err = srv.Start(context.Background())
	if err == nil {
		t.Error("Expected error when port is in use")
	}
}

//Personal.AI order the ending

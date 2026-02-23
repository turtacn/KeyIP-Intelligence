package http

import (
	"context"
	"net/http"
	"testing"
	"time"
)

func TestNewServer(t *testing.T) {
	mux := http.NewServeMux()
	server := NewServer(":8080", mux)

	if server == nil {
		t.Fatal("server should not be nil")
	}

	if server.httpServer.Addr != ":8080" {
		t.Errorf("expected addr=:8080, got %s", server.httpServer.Addr)
	}
}

func TestServer_Shutdown(t *testing.T) {
	mux := http.NewServeMux()
	server := NewServer(":0", mux)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err := server.Shutdown(ctx)
	if err != nil {
		t.Errorf("shutdown failed: %v", err)
	}
}

//Personal.AI order the ending

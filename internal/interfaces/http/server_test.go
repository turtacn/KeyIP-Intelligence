package http

import (
	"testing"
)

func TestNewServer(t *testing.T) {
	srv := NewServer(8080)
	if srv == nil {
		t.Fatal("server should not be nil")
	}
	if srv.port != 8080 {
		t.Errorf("expected port 8080, got %d", srv.port)
	}
}

//Personal.AI order the ending

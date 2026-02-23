package grpc

import (
	"testing"
)

func TestNewServer(t *testing.T) {
	srv := NewServer(9090)
	if srv == nil {
		t.Error("expected server instance")
	}
	if srv.port != 9090 {
		t.Errorf("expected port 9090, got %d", srv.port)
	}
}

//Personal.AI order the ending

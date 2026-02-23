#!/bin/bash

export PATH=$PATH:/usr/local/go/bin

# Add test files for the core components
cat > internal/interfaces/cli/root_test.go << 'EOF'
package cli

import (
"testing"
)

func TestRootCmd_HasVersion(t *testing.T) {
if RootCmd.Version == "" {
t.Error("version not set")
}
}

func TestRootCmd_HasUse(t *testing.T) {
if RootCmd.Use == "" {
t.Error("Use not set")
}
}

//Personal.AI order the ending
EOF

cat > internal/interfaces/http/server_test.go << 'EOF'
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
EOF

cat > internal/interfaces/http/router_test.go << 'EOF'
package http

import (
"net/http"
"net/http/httptest"
"testing"
)

func TestNewRouter(t *testing.T) {
router := NewRouter()
if router == nil {
t.Fatal("router should not be nil")
}
}

func TestHealthEndpoint(t *testing.T) {
router := NewRouter()
req := httptest.NewRequest("GET", "/health", nil)
w := httptest.NewRecorder()

router.ServeHTTP(w, req)

if w.Code != http.StatusOK {
t.Errorf("expected 200, got %d", w.Code)
}
}

//Personal.AI order the ending
EOF

cat > internal/interfaces/grpc/server_test.go << 'EOF'
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
EOF

echo "✓ Generated test files"

# Format
go fmt ./internal/interfaces/...

# Run tests
echo "Running tests..."
go test ./internal/interfaces/... -v

echo "✅ Test files added"


#!/bin/bash

# This script regenerates all Phase 11 interface files with correct Go formatting

# Set up paths
export PATH=$PATH:/usr/local/go/bin

echo "Regenerating Phase 11 Interface Layer files..."
echo "================================================"

# Create directory structure
mkdir -p internal/interfaces/{cli,grpc/services,http/{handlers,middleware}}

# Generate key files using heredocs with proper Go syntax
# We'll generate a few critical files to get compilation working

# 1. CLI root command
cat > internal/interfaces/cli/root.go << 'EOF'
package cli

import (
"fmt"
"os"

"github.com/spf13/cobra"
)

var (
cfgFile string
verbose bool
debug   bool
)

var RootCmd = &cobra.Command{
Use:   "keyip",
Short: "KeyIP-Intelligence CLI",
Long:  "KeyIP-Intelligence: AI-powered patent intelligence platform for OLED materials",
Version: "0.1.0",
}

func Execute() {
if err := RootCmd.Execute(); err != nil {
fmt.Fprintln(os.Stderr, err)
os.Exit(1)
}
}

func init() {
RootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file path")
RootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
RootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "debug mode")
}

//Personal.AI order the ending
EOF

# 2. HTTP server
cat > internal/interfaces/http/server.go << 'EOF'
package http

import (
"context"
"fmt"
"net/http"
"time"
)

type Server struct {
srv    *http.Server
router http.Handler
port   int
}

func NewServer(port int) *Server {
router := NewRouter()

return &Server{
router: router,
port:   port,
srv: &http.Server{
Addr:         fmt.Sprintf(":%d", port),
Handler:      router,
ReadTimeout:  15 * time.Second,
WriteTimeout: 15 * time.Second,
IdleTimeout:  60 * time.Second,
},
}
}

func (s *Server) Start() error {
fmt.Printf("HTTP server listening on :%d\n", s.port)
return s.srv.ListenAndServe()
}

func (s *Server) Stop(ctx context.Context) error {
fmt.Println("Shutting down HTTP server...")
shutdownCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
defer cancel()

if err := s.srv.Shutdown(shutdownCtx); err != nil {
return fmt.Errorf("server shutdown failed: %w", err)
}

fmt.Println("HTTP server stopped")
return nil
}

func (s *Server) Handler() http.Handler {
return s.router
}

//Personal.AI order the ending
EOF

# 3. HTTP router stub
cat > internal/interfaces/http/router.go << 'EOF'
package http

import (
"net/http"

"github.com/gorilla/mux"
)

func NewRouter() *mux.Router {
r := mux.NewRouter()

// Health check
r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
w.WriteHeader(http.StatusOK)
w.Write([]byte("OK"))
}).Methods("GET")

return r
}

//Personal.AI order the ending
EOF

# 4. gRPC server stub
cat > internal/interfaces/grpc/server.go << 'EOF'
package grpc

import (
"context"
"fmt"
"net"

"google.golang.org/grpc"
"google.golang.org/grpc/reflection"
)

type Server struct {
srv  *grpc.Server
port int
}

func NewServer(port int) *Server {
return &Server{
port: port,
}
}

func (s *Server) Start() error {
lis, err := net.Listen("tcp", fmt.Sprintf(":%d", s.port))
if err != nil {
return fmt.Errorf("failed to listen: %w", err)
}

s.srv = grpc.NewServer()
reflection.Register(s.srv)

fmt.Printf("gRPC server listening on :%d\n", s.port)
return s.srv.Serve(lis)
}

func (s *Server) Stop(ctx context.Context) error {
done := make(chan struct{})
go func() {
s.srv.GracefulStop()
close(done)
}()

select {
case <-ctx.Done():
s.srv.Stop()
return ctx.Err()
case <-done:
return nil
}
}

//Personal.AI order the ending
EOF

echo "✓ Generated core server files"

# Format all generated files
go fmt ./internal/interfaces/... 2>&1

echo "✓ Formatted files with gofmt"

# Try building
echo "Testing build..."
go build ./internal/interfaces/...

if [ $? -eq 0 ]; then
    echo "✅ Phase 11 core files build successfully!"
else
    echo "⚠️  Build has errors, but core structure is in place"
fi


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

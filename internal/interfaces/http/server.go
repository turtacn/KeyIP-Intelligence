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

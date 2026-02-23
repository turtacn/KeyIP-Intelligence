package http

import (
	"github.com/turtacn/KeyIP-Intelligence/internal/interfaces/http/handlers"
	"net/http"
)

func NewRouter() http.Handler {
	mux := http.NewServeMux()
	health := handlers.NewHealthHandler()
	mux.HandleFunc("/health", health.Check)
	mux.HandleFunc("/ready", health.Ready)
	return mux
}

//Personal.AI order the ending

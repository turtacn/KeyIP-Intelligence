package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/turtacn/KeyIP-Intelligence/internal/application/molecule"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/internal/interfaces/http/handlers"
)

// MockLogger (reused from tenant_test.go if possible, but separate package)
type mockLogger struct {
	logging.Logger
}
func (m *mockLogger) Error(msg string, fields ...logging.Field) {}
func (m *mockLogger) Info(msg string, fields ...logging.Field) {}
func (m *mockLogger) Warn(msg string, fields ...logging.Field) {}
func (m *mockLogger) Debug(msg string, fields ...logging.Field) {}

// MockMoleculeService
type mockMoleculeService struct {
}

func (m *mockMoleculeService) Create(ctx context.Context, input *molecule.CreateInput) (*molecule.Molecule, error) {
	return nil, nil
}
func (m *mockMoleculeService) GetByID(ctx context.Context, id string) (*molecule.Molecule, error) {
	return nil, nil
}
func (m *mockMoleculeService) List(ctx context.Context, input *molecule.ListInput) (*molecule.ListResult, error) {
	return &molecule.ListResult{}, nil
}
func (m *mockMoleculeService) Update(ctx context.Context, input *molecule.UpdateInput) (*molecule.Molecule, error) {
	return nil, nil
}
func (m *mockMoleculeService) Delete(ctx context.Context, id string, userID string) error {
	return nil
}
func (m *mockMoleculeService) SearchByStructure(ctx context.Context, input *molecule.StructureSearchInput) (*molecule.SearchResult, error) {
	return nil, nil
}
func (m *mockMoleculeService) SearchBySimilarity(ctx context.Context, input *molecule.SimilaritySearchInput) (*molecule.SearchResult, error) {
	return nil, nil
}
func (m *mockMoleculeService) CalculateProperties(ctx context.Context, input *molecule.CalculatePropertiesInput) (*molecule.PropertiesResult, error) {
	return nil, nil
}

func TestNewRouter(t *testing.T) {
	logger := &mockLogger{}

	// Middleware mocks
	authMw := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Auth", "true")
			next.ServeHTTP(w, r)
		})
	}

	// Handler mocks
	molSvc := &mockMoleculeService{}
	molHandler := handlers.NewMoleculeHandler(molSvc, logger)

	deps := RouterDeps{
		MoleculeHandler: molHandler,
		AuthMiddleware:  authMw,
	}

	cfg := RouterConfig{
		APIPrefix:  "/api/v1",
		EnableAuth: true,
		Logger:     logger,
	}
	
	router := NewRouter(cfg, deps)

	server := httptest.NewServer(router)
	defer server.Close()

	t.Run("Health Check (Public)", func(t *testing.T) {
		// Health handler is nil in deps, so it should be 404
		resp, err := http.Get(server.URL + "/healthz")
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("Expected 404 for missing health handler, got %d", resp.StatusCode)
		}
	})

	t.Run("API Route (Authenticated)", func(t *testing.T) {
		resp, err := http.Get(server.URL + "/api/v1/molecules")
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}
		// We expect 200 OK (from mock service) and X-Auth header
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected 200 OK, got %d", resp.StatusCode)
		}
		if resp.Header.Get("X-Auth") != "true" {
			t.Error("Expected X-Auth header to be set by middleware")
		}
		if resp.Header.Get("X-Request-ID") == "" {
			t.Error("Expected X-Request-ID header")
		}
	})

	t.Run("404 for Unknown Route", func(t *testing.T) {
		resp, err := http.Get(server.URL + "/api/v1/unknown")
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("Expected 404, got %d", resp.StatusCode)
		}
	})
}
//Personal.AI order the ending

package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	perrors "github.com/turtacn/KeyIP-Intelligence/pkg/errors"
	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
)

// MockLogger
type mockLogger struct {
	logging.Logger
}

func (m *mockLogger) Error(msg string, fields ...logging.Field) {}
func (m *mockLogger) Info(msg string, fields ...logging.Field)  {}
func (m *mockLogger) Warn(msg string, fields ...logging.Field)  {}
func (m *mockLogger) Debug(msg string, fields ...logging.Field) {}

// MockTenantResolver
type mockTenantResolver struct {
	resolveFunc func(ctx context.Context, tenantID string) (*TenantInfo, error)
}

func (m *mockTenantResolver) Resolve(ctx context.Context, tenantID string) (*TenantInfo, error) {
	if m.resolveFunc != nil {
		return m.resolveFunc(ctx, tenantID)
	}
	return nil, nil
}

// MockTenantCache
type mockTenantCache struct {
	getFunc func(ctx context.Context, tenantID string) (*TenantInfo, bool)
	setFunc func(ctx context.Context, tenantID string, info *TenantInfo, ttl time.Duration)
}

func (m *mockTenantCache) Get(ctx context.Context, tenantID string) (*TenantInfo, bool) {
	if m.getFunc != nil {
		return m.getFunc(ctx, tenantID)
	}
	return nil, false
}

func (m *mockTenantCache) Set(ctx context.Context, tenantID string, info *TenantInfo, ttl time.Duration) {
	if m.setFunc != nil {
		m.setFunc(ctx, tenantID, info, ttl)
	}
}

func TestTenantFromContext(t *testing.T) {
	ctx := context.Background()

	// Test without tenant info
	if _, ok := TenantFromContext(ctx); ok {
		t.Error("expected false when tenant info is missing")
	}

	// Test with tenant info
	info := &TenantInfo{TenantID: "test-tenant"}
	ctx = ContextWithTenant(ctx, info)
	got, ok := TenantFromContext(ctx)
	if !ok {
		t.Error("expected true when tenant info is present")
	}
	if got != info {
		t.Errorf("expected %v, got %v", info, got)
	}
}

func TestNewTenantMiddleware(t *testing.T) {
	logger := &mockLogger{}

	tests := []struct {
		name           string
		cfg            TenantMiddlewareConfig
		reqHeaders     map[string]string
		reqQuery       string
		expectedStatus int
		expectedCode   string
		expectTenant   bool
	}{
		{
			name:           "Valid Header",
			cfg:            TenantMiddlewareConfig{Required: true, Logger: logger},
			reqHeaders:     map[string]string{"X-Tenant-ID": "valid-tenant"},
			expectedStatus: http.StatusOK,
			expectTenant:   true,
		},
		{
			name:           "Valid Query Param",
			cfg:            TenantMiddlewareConfig{Required: true, Logger: logger},
			reqQuery:       "tenant_id=valid-tenant",
			expectedStatus: http.StatusOK,
			expectTenant:   true,
		},
		{
			name:           "Missing Tenant ID (Required)",
			cfg:            TenantMiddlewareConfig{Required: true, Logger: logger},
			expectedStatus: http.StatusUnauthorized,
			expectedCode:   string(perrors.ErrTenantRequired),
		},
		{
			name:           "Missing Tenant ID (Not Required)",
			cfg:            TenantMiddlewareConfig{Required: false, Logger: logger},
			expectedStatus: http.StatusOK,
			expectTenant:   false,
		},
		{
			name:           "Default Tenant ID",
			cfg:            TenantMiddlewareConfig{Required: false, DefaultTenantID: "default-tenant", Logger: logger},
			expectedStatus: http.StatusOK,
			expectTenant:   true,
		},
		{
			name:           "Invalid Format",
			cfg:            TenantMiddlewareConfig{Required: true, Logger: logger},
			reqHeaders:     map[string]string{"X-Tenant-ID": "invalid@tenant"},
			expectedStatus: http.StatusBadRequest,
			expectedCode:   string(perrors.ErrInvalidTenantID),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mw := NewTenantMiddleware(tt.cfg)
			handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				info, ok := TenantFromContext(r.Context())
				if tt.expectTenant && !ok {
					t.Error("expected tenant info in context")
				}
				if !tt.expectTenant && ok {
					t.Error("unexpected tenant info in context")
				}
				if tt.expectTenant && ok {
					expectedID := tt.reqHeaders["X-Tenant-ID"]
					if expectedID == "" {
						if strings.Contains(tt.reqQuery, "=") {
							expectedID = strings.Split(tt.reqQuery, "=")[1]
						} else if tt.cfg.DefaultTenantID != "" {
							expectedID = tt.cfg.DefaultTenantID
						}
					}
					if info.TenantID != expectedID {
						t.Errorf("expected tenant ID %s, got %s", expectedID, info.TenantID)
					}
				}
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest("GET", "/?"+tt.reqQuery, nil)
			for k, v := range tt.reqHeaders {
				req.Header.Set(k, v)
			}
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, rec.Code)
			}

			if tt.expectedCode != "" {
				var resp common.ErrorResponse
				if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}
				if resp.Error.Code != tt.expectedCode {
					t.Errorf("expected error code %s, got %s", tt.expectedCode, resp.Error.Code)
				}
			}
		})
	}
}

func TestTenantMiddleware_ResolverAndCache(t *testing.T) {
	logger := &mockLogger{}
	tenantID := "test-tenant"
	expectedInfo := &TenantInfo{TenantID: tenantID, Plan: "pro"}

	t.Run("Cache Hit", func(t *testing.T) {
		cache := &mockTenantCache{
			getFunc: func(ctx context.Context, id string) (*TenantInfo, bool) {
				if id == tenantID {
					return expectedInfo, true
				}
				return nil, false
			},
		}
		cfg := TenantMiddlewareConfig{Required: true, Cache: cache, Logger: logger}
		mw := NewTenantMiddleware(cfg)

		handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			info, _ := TenantFromContext(r.Context())
			if info.Plan != "pro" {
				t.Errorf("expected plan pro, got %s", info.Plan)
			}
		}))

		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("X-Tenant-ID", tenantID)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rec.Code)
		}
	})

	t.Run("Cache Miss, Resolver Success", func(t *testing.T) {
		cacheSet := false
		cache := &mockTenantCache{
			getFunc: func(ctx context.Context, id string) (*TenantInfo, bool) {
				return nil, false
			},
			setFunc: func(ctx context.Context, id string, info *TenantInfo, ttl time.Duration) {
				if id == tenantID && info == expectedInfo {
					cacheSet = true
				}
			},
		}
		resolver := &mockTenantResolver{
			resolveFunc: func(ctx context.Context, id string) (*TenantInfo, error) {
				if id == tenantID {
					return expectedInfo, nil
				}
				return nil, nil
			},
		}
		cfg := TenantMiddlewareConfig{Required: true, Cache: cache, TenantResolver: resolver, Logger: logger}
		mw := NewTenantMiddleware(cfg)

		handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("X-Tenant-ID", tenantID)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rec.Code)
		}
		if !cacheSet {
			t.Error("expected cache set")
		}
	})

	t.Run("Resolver Error", func(t *testing.T) {
		resolver := &mockTenantResolver{
			resolveFunc: func(ctx context.Context, id string) (*TenantInfo, error) {
				return nil, errors.New("db error")
			},
		}
		cfg := TenantMiddlewareConfig{Required: true, TenantResolver: resolver, Logger: logger}
		mw := NewTenantMiddleware(cfg)

		handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("X-Tenant-ID", tenantID)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Errorf("expected status 500, got %d", rec.Code)
		}
	})

	t.Run("Resolver Not Found", func(t *testing.T) {
		resolver := &mockTenantResolver{
			resolveFunc: func(ctx context.Context, id string) (*TenantInfo, error) {
				return nil, nil
			},
		}
		cfg := TenantMiddlewareConfig{Required: true, TenantResolver: resolver, Logger: logger}
		mw := NewTenantMiddleware(cfg)

		handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("X-Tenant-ID", tenantID)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Errorf("expected status 403, got %d", rec.Code)
		}
	})
}
//Personal.AI order the ending

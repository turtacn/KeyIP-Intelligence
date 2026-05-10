package grpc

import (
	"context"
	"errors"
	"testing"

	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

// ---------------------------------------------------------------------------
// Mock Checker
// ---------------------------------------------------------------------------

type mockChecker struct {
	name string
	err  error
}

func (m *mockChecker) Name() string {
	return m.name
}

func (m *mockChecker) CheckHealth(ctx context.Context) error {
	return m.err
}

// ---------------------------------------------------------------------------
// Tests: NewHealthService
// ---------------------------------------------------------------------------

func TestNewHealthService_DefaultsToServing(t *testing.T) {
	hs := NewHealthService()
	resp, err := hs.Check(context.Background(), &healthpb.HealthCheckRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Status != healthpb.HealthCheckResponse_SERVING {
		t.Errorf("status = %v, want SERVING", resp.Status)
	}
}

func TestNewHealthService_WithCheckers(t *testing.T) {
	checker := &mockChecker{name: "test", err: nil}
	hs := NewHealthService(checker)
	if hs == nil {
		t.Fatal("HealthService should not be nil")
	}
	resp, err := hs.Check(context.Background(), &healthpb.HealthCheckRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Status != healthpb.HealthCheckResponse_SERVING {
		t.Errorf("status = %v, want SERVING", resp.Status)
	}
}

// ---------------------------------------------------------------------------
// Tests: Check
// ---------------------------------------------------------------------------

func TestHealthService_Check_AllCheckersPass(t *testing.T) {
	checker1 := &mockChecker{name: "db1"}
	checker2 := &mockChecker{name: "db2"}
	hs := NewHealthService(checker1, checker2)

	resp, err := hs.Check(context.Background(), &healthpb.HealthCheckRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Status != healthpb.HealthCheckResponse_SERVING {
		t.Errorf("status = %v, want SERVING", resp.Status)
	}
}

func TestHealthService_Check_OneCheckerFails(t *testing.T) {
	checker1 := &mockChecker{name: "db1"}
	checker2 := &mockChecker{name: "db2", err: errors.New("connection refused")}
	hs := NewHealthService(checker1, checker2)

	resp, err := hs.Check(context.Background(), &healthpb.HealthCheckRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Status != healthpb.HealthCheckResponse_NOT_SERVING {
		t.Errorf("status = %v, want NOT_SERVING", resp.Status)
	}
}

func TestHealthService_Check_FirstCheckerFails(t *testing.T) {
	checker1 := &mockChecker{name: "db1", err: errors.New("timeout")}
	checker2 := &mockChecker{name: "db2"}
	hs := NewHealthService(checker1, checker2)

	resp, err := hs.Check(context.Background(), &healthpb.HealthCheckRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Status != healthpb.HealthCheckResponse_NOT_SERVING {
		t.Errorf("status = %v, want NOT_SERVING", resp.Status)
	}
}

func TestHealthService_Check_AllCheckersFail(t *testing.T) {
	checker1 := &mockChecker{name: "db1", err: errors.New("error 1")}
	checker2 := &mockChecker{name: "db2", err: errors.New("error 2")}
	hs := NewHealthService(checker1, checker2)

	resp, err := hs.Check(context.Background(), &healthpb.HealthCheckRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Status != healthpb.HealthCheckResponse_NOT_SERVING {
		t.Errorf("status = %v, want NOT_SERVING", resp.Status)
	}
}

func TestHealthService_Check_NoCheckers(t *testing.T) {
	hs := NewHealthService()

	resp, err := hs.Check(context.Background(), &healthpb.HealthCheckRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Status != healthpb.HealthCheckResponse_SERVING {
		t.Errorf("status = %v, want SERVING", resp.Status)
	}
}

func TestHealthService_Check_WithSpecificService(t *testing.T) {
	checker := &mockChecker{name: "db1"}
	hs := NewHealthService(checker)

	// Check a specific service name
	resp, err := hs.Check(context.Background(), &healthpb.HealthCheckRequest{
		Service: "myService",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Status != healthpb.HealthCheckResponse_SERVING {
		t.Errorf("status = %v, want SERVING", resp.Status)
	}
}

func TestHealthService_Check_FailingCheckerWithSpecificService(t *testing.T) {
	checker := &mockChecker{name: "db1", err: errors.New("down")}
	hs := NewHealthService(checker)

	resp, err := hs.Check(context.Background(), &healthpb.HealthCheckRequest{
		Service: "myService",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Status != healthpb.HealthCheckResponse_NOT_SERVING {
		t.Errorf("status = %v, want NOT_SERVING", resp.Status)
	}
}

func TestHealthService_Check_RecoversAfterFailure(t *testing.T) {
	// Simulate a checker that fails then recovers
	errFail := errors.New("down")
	errOK := error(nil)
	checker := &mockChecker{name: "db1", err: errFail}
	hs := NewHealthService(checker)

	// First call: checker fails
	resp, err := hs.Check(context.Background(), &healthpb.HealthCheckRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Status != healthpb.HealthCheckResponse_NOT_SERVING {
		t.Errorf("status = %v, want NOT_SERVING", resp.Status)
	}

	// Checker recovers
	checker.err = errOK

	// Second call: checker passes
	resp, err = hs.Check(context.Background(), &healthpb.HealthCheckRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Status != healthpb.HealthCheckResponse_SERVING {
		t.Errorf("status = %v, want SERVING", resp.Status)
	}

	// Checker fails again
	checker.err = errFail

	// Third call: checker fails
	resp, err = hs.Check(context.Background(), &healthpb.HealthCheckRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Status != healthpb.HealthCheckResponse_NOT_SERVING {
		t.Errorf("status = %v, want NOT_SERVING", resp.Status)
	}
}

// ---------------------------------------------------------------------------
// Tests: Checker via NewChecker helper
// ---------------------------------------------------------------------------

func TestNewChecker_WithFunction(t *testing.T) {
	called := false
	checker := NewChecker("custom", func(ctx context.Context) error {
		called = true
		return nil
	})

	if checker.Name() != "custom" {
		t.Errorf("Name() = %q, want %q", checker.Name(), "custom")
	}

	err := checker.CheckHealth(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("checker function was not called")
	}
}

func TestNewChecker_WithFailingFunction(t *testing.T) {
	checker := NewChecker("failing", func(ctx context.Context) error {
		return errors.New("failure")
	})

	err := checker.CheckHealth(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "failure" {
		t.Errorf("unexpected error message: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Tests: SetServingStatus
// ---------------------------------------------------------------------------

func TestHealthService_SetServingStatus(t *testing.T) {
	hs := NewHealthService()

	// Set a specific service to NOT_SERVING
	hs.SetServingStatus("myService", healthpb.HealthCheckResponse_NOT_SERVING)

	resp, err := hs.Check(context.Background(), &healthpb.HealthCheckRequest{
		Service: "myService",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Status != healthpb.HealthCheckResponse_NOT_SERVING {
		t.Errorf("status = %v, want NOT_SERVING", resp.Status)
	}
}

func TestHealthService_SetServingStatus_Overall(t *testing.T) {
	hs := NewHealthService()

	// Set overall status to NOT_SERVING
	hs.SetServingStatus("", healthpb.HealthCheckResponse_NOT_SERVING)

	resp, err := hs.Check(context.Background(), &healthpb.HealthCheckRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Status != healthpb.HealthCheckResponse_NOT_SERVING {
		t.Errorf("status = %v, want NOT_SERVING", resp.Status)
	}
}

// ---------------------------------------------------------------------------
// Tests: Check with mixed checkers and SetServingStatus
// ---------------------------------------------------------------------------

func TestHealthService_CheckersOverrideSetServingStatus(t *testing.T) {
	failing := &mockChecker{name: "db", err: errors.New("down")}
	hs := NewHealthService(failing)

	// Manually set overall to SERVING (will be overridden by checker)
	hs.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)

	resp, err := hs.Check(context.Background(), &healthpb.HealthCheckRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Status != healthpb.HealthCheckResponse_NOT_SERVING {
		t.Errorf("status = %v, want NOT_SERVING (checker should override)", resp.Status)
	}
}

// ---------------------------------------------------------------------------
// Tests: Context cancellation
// ---------------------------------------------------------------------------

func TestHealthService_Check_CancelledContext(t *testing.T) {
	checker := &mockChecker{name: "db1"}
	hs := NewHealthService(checker)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	resp, err := hs.Check(ctx, &healthpb.HealthCheckRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Status != healthpb.HealthCheckResponse_SERVING {
		t.Errorf("status = %v, want SERVING", resp.Status)
	}
}

func TestHealthService_Check_CancelledContext_FailingChecker(t *testing.T) {
	failing := &mockChecker{name: "db", err: errors.New("down")}
	hs := NewHealthService(failing)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	resp, err := hs.Check(ctx, &healthpb.HealthCheckRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Status != healthpb.HealthCheckResponse_NOT_SERVING {
		t.Errorf("status = %v, want NOT_SERVING", resp.Status)
	}
}

// ---------------------------------------------------------------------------
// Tests: HealthService with multiple checkers and various status scenarios
// ---------------------------------------------------------------------------

func TestHealthService_Check_EmptyCheckersList(t *testing.T) {
	// Verify that passing an empty slice is the same as not passing any checkers
	hs := NewHealthService([]Checker{}...)

	resp, err := hs.Check(context.Background(), &healthpb.HealthCheckRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Status != healthpb.HealthCheckResponse_SERVING {
		t.Errorf("status = %v, want SERVING", resp.Status)
	}
}

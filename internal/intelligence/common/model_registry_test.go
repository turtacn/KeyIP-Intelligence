package common

import (
	"context"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Mock ModelLoader
// ---------------------------------------------------------------------------

type mockModelLoader struct {
	mu            sync.Mutex
	loadCalls     int
	unloadCalls   int
	validateCalls int
	loadDelay     time.Duration
	loadErr       error
	unloadErr     error
	validateErr   error
	loadedHandles []string // artifact paths loaded
}

func newMockModelLoader() *mockModelLoader {
	return &mockModelLoader{}
}

func (m *mockModelLoader) Load(ctx context.Context, artifactPath string) (interface{}, error) {
	m.mu.Lock()
	m.loadCalls++
	m.loadedHandles = append(m.loadedHandles, artifactPath)
	m.mu.Unlock()

	if m.loadDelay > 0 {
		select {
		case <-time.After(m.loadDelay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if m.loadErr != nil {
		return nil, m.loadErr
	}
	return &mockHandle{path: artifactPath}, nil
}

func (m *mockModelLoader) Unload(ctx context.Context, modelHandle interface{}) error {
	m.mu.Lock()
	m.unloadCalls++
	m.mu.Unlock()
	return m.unloadErr
}

func (m *mockModelLoader) Validate(ctx context.Context, artifactPath string, checksum string) error {
	m.mu.Lock()
	m.validateCalls++
	m.mu.Unlock()
	return m.validateErr
}

func (m *mockModelLoader) getLoadCalls() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.loadCalls
}

func (m *mockModelLoader) getUnloadCalls() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.unloadCalls
}

func (m *mockModelLoader) getValidateCalls() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.validateCalls
}

type mockHandle struct {
	path string
}

// ---------------------------------------------------------------------------
// Mock Metrics
// ---------------------------------------------------------------------------

type mockRegistryMetrics struct {
	modelLoadCount atomic.Int32
}

func (m *mockRegistryMetrics) RecordInference(ctx context.Context, p *InferenceMetricParams)         {}
func (m *mockRegistryMetrics) RecordBatchProcessing(ctx context.Context, p *BatchMetricParams)       {}
func (m *mockRegistryMetrics) RecordCacheAccess(ctx context.Context, hit bool, modelName string)     {}
func (m *mockRegistryMetrics) RecordCircuitBreakerStateChange(ctx context.Context, modelName, from, to string) {
}
func (m *mockRegistryMetrics) RecordRiskAssessment(ctx context.Context, riskLevel string, durationMs float64) {
}
func (m *mockRegistryMetrics) RecordModelLoad(ctx context.Context, modelName, version string, durationMs float64, success bool) {
	m.modelLoadCount.Add(1)
}
func (m *mockRegistryMetrics) GetInferenceLatencyHistogram() LatencyHistogram { return nil }
func (m *mockRegistryMetrics) GetCurrentStats() *IntelligenceStats            { return &IntelligenceStats{} }

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func newTestModelMetadata(modelID, version string) *ModelMetadata {
	return &ModelMetadata{
		ModelID:      modelID,
		Name:         modelID + "-name",
		Description:  "test model",
		Version:      version,
		ArtifactPath: fmt.Sprintf("/models/%s/%s/model.bin", modelID, version),
		Checksum:     "sha256:abc123",
		SizeBytes:    1024,
	}
}

func newRegistryTestHelper(t *testing.T) (ModelRegistry, *mockModelLoader, *mockRegistryMetrics) {
	t.Helper()
	loader := newMockModelLoader()
	metrics := &mockRegistryMetrics{}
	reg, err := NewModelRegistry(loader, metrics, NewNoopLogger(),
		WithRegistryHealthCheckInterval(1*time.Hour), // disable periodic checks in tests
		WithUnloadDelay(0),                           // immediate unload for test speed
		WithMaxLoadedVersions(3),
	)
	if err != nil {
		t.Fatalf("NewModelRegistry: %v", err)
	}
	t.Cleanup(func() { _ = reg.Close() })
	return reg, loader, metrics
}

func registerTestModel(t *testing.T, registry ModelRegistry, modelID, version string) {
	t.Helper()
	meta := newTestModelMetadata(modelID, version)
	if err := registry.Register(context.Background(), meta); err != nil {
		t.Fatalf("Register(%s, %s): %v", modelID, version, err)
	}
}

func activateTestModel(t *testing.T, registry ModelRegistry, modelID, version string) {
	t.Helper()
	registerTestModel(t, registry, modelID, version)
	if err := registry.SetActiveVersion(context.Background(), modelID, version); err != nil {
		t.Fatalf("SetActiveVersion(%s, %s): %v", modelID, version, err)
	}
}

// ---------------------------------------------------------------------------
// Tests: Constructor
// ---------------------------------------------------------------------------

func TestNewModelRegistry_Success(t *testing.T) {
	loader := newMockModelLoader()
	reg, err := NewModelRegistry(loader, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = reg.Close()
}

func TestNewModelRegistry_NilLoader(t *testing.T) {
	_, err := NewModelRegistry(nil, nil, nil)
	if err == nil {
		t.Fatal("expected error for nil loader")
	}
}

// ---------------------------------------------------------------------------
// Tests: Register
// ---------------------------------------------------------------------------

func TestRegister_Success(t *testing.T) {
	reg, loader, _ := newRegistryTestHelper(t)
	meta := newTestModelMetadata("gnn-v1", "1.0.0")
	err := reg.Register(context.Background(), meta)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if loader.getValidateCalls() != 1 {
		t.Errorf("expected 1 validate call, got %d", loader.getValidateCalls())
	}

	versions, err := reg.ListVersions(context.Background(), "gnn-v1")
	if err != nil {
		t.Fatalf("ListVersions: %v", err)
	}
	if len(versions) != 1 {
		t.Fatalf("expected 1 version, got %d", len(versions))
	}
	if versions[0].Status != VersionStatusRegistered {
		t.Errorf("expected REGISTERED, got %s", versions[0].Status)
	}
}

func TestRegister_DuplicateVersion(t *testing.T) {
	reg, _, _ := newRegistryTestHelper(t)
	registerTestModel(t, reg, "gnn-v1", "1.0.0")

	meta := newTestModelMetadata("gnn-v1", "1.0.0")
	err := reg.Register(context.Background(), meta)
	if err == nil {
		t.Fatal("expected error for duplicate version")
	}
}

func TestRegister_InvalidSemver(t *testing.T) {
	reg, _, _ := newRegistryTestHelper(t)
	meta := newTestModelMetadata("gnn-v1", "abc")
	err := reg.Register(context.Background(), meta)
	if err == nil {
		t.Fatal("expected error for invalid semver")
	}
}

func TestRegister_ValidSemver_Variants(t *testing.T) {
	reg, _, _ := newRegistryTestHelper(t)
	validVersions := []string{
		"1.0.0",
		"1.0.0-beta.1",
		"1.0.0+build.123",
		"0.1.0-alpha",
		"2.0.0-rc.1+build.456",
	}
	for _, v := range validVersions {
		meta := newTestModelMetadata("gnn-semver", v)
		if err := reg.Register(context.Background(), meta); err != nil {
			t.Errorf("version %q should be valid, got error: %v", v, err)
		}
	}
}

func TestRegister_ValidationFailure(t *testing.T) {
	loader := newMockModelLoader()
	loader.validateErr = fmt.Errorf("checksum mismatch")
	reg, err := NewModelRegistry(loader, nil, nil,
		WithRegistryHealthCheckInterval(1*time.Hour),
	)
	if err != nil {
		t.Fatalf("NewModelRegistry: %v", err)
	}
	defer reg.Close()

	meta := newTestModelMetadata("gnn-v1", "1.0.0")
	err = reg.Register(context.Background(), meta)
	if err == nil {
		t.Fatal("expected error for validation failure")
	}
}

func TestRegister_EmptyModelID(t *testing.T) {
	reg, _, _ := newRegistryTestHelper(t)
	meta := newTestModelMetadata("", "1.0.0")
	meta.ModelID = ""
	err := reg.Register(context.Background(), meta)
	if err == nil {
		t.Fatal("expected error for empty model_id")
	}
}

func TestRegister_EmptyArtifactPath(t *testing.T) {
	reg, _, _ := newRegistryTestHelper(t)
	meta := newTestModelMetadata("gnn-v1", "1.0.0")
	meta.ArtifactPath = ""
	err := reg.Register(context.Background(), meta)
	if err == nil {
		t.Fatal("expected error for empty artifact_path")
	}
}

// ---------------------------------------------------------------------------
// Tests: GetModel
// ---------------------------------------------------------------------------

func TestGetModel_ActiveVersion(t *testing.T) {
	reg, _, _ := newRegistryTestHelper(t)
	activateTestModel(t, reg, "gnn-v1", "1.0.0")

	model, err := reg.GetModel(context.Background(), "gnn-v1")
	if err != nil {
		t.Fatalf("GetModel: %v", err)
	}
	if model.ActiveVersion != "1.0.0" {
		t.Errorf("expected active version 1.0.0, got %s", model.ActiveVersion)
	}
	if model.Status != ModelStatusActive {
		t.Errorf("expected ACTIVE status, got %s", model.Status)
	}
}

func TestGetModel_NotRegistered(t *testing.T) {
	reg, _, _ := newRegistryTestHelper(t)
	_, err := reg.GetModel(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for unregistered model")
	}
}

func TestGetModel_NoActiveVersion(t *testing.T) {
	reg, _, _ := newRegistryTestHelper(t)
	registerTestModel(t, reg, "gnn-v1", "1.0.0")

	_, err := reg.GetModel(context.Background(), "gnn-v1")
	if err == nil {
		t.Fatal("expected error for no active version")
	}
}

// ---------------------------------------------------------------------------
// Tests: GetModelVersion
// ---------------------------------------------------------------------------

func TestGetModelVersion_Exists(t *testing.T) {
	reg, _, _ := newRegistryTestHelper(t)
	registerTestModel(t, reg, "gnn-v1", "1.0.0")

	model, err := reg.GetModelVersion(context.Background(), "gnn-v1", "1.0.0")
	if err != nil {
		t.Fatalf("GetModelVersion: %v", err)
	}
	if model.Metadata.Version != "1.0.0" {
		t.Errorf("expected version 1.0.0, got %s", model.Metadata.Version)
	}
}

func TestGetModelVersion_NotExists(t *testing.T) {
	reg, _, _ := newRegistryTestHelper(t)
	registerTestModel(t, reg, "gnn-v1", "1.0.0")

	_, err := reg.GetModelVersion(context.Background(), "gnn-v1", "2.0.0")
	if err == nil {
		t.Fatal("expected error for nonexistent version")
	}
}

// ---------------------------------------------------------------------------
// Tests: ListModels
// ---------------------------------------------------------------------------

func TestListModels_Empty(t *testing.T) {
	reg, _, _ := newRegistryTestHelper(t)
	models, err := reg.ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	if len(models) != 0 {
		t.Errorf("expected 0 models, got %d", len(models))
	}
}

func TestListModels_Multiple(t *testing.T) {
	reg, _, _ := newRegistryTestHelper(t)
	activateTestModel(t, reg, "model-a", "1.0.0")
	activateTestModel(t, reg, "model-b", "2.0.0")
	activateTestModel(t, reg, "model-c", "1.0.0")

	models, err := reg.ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	if len(models) != 3 {
		t.Errorf("expected 3 models, got %d", len(models))
	}
	// Should be sorted by ModelID
	if models[0].ModelID != "model-a" {
		t.Errorf("expected first model to be model-a, got %s", models[0].ModelID)
	}
}

// ---------------------------------------------------------------------------
// Tests: ListVersions
// ---------------------------------------------------------------------------

func TestListVersions_Multiple(t *testing.T) {
	reg, _, _ := newRegistryTestHelper(t)
	registerTestModel(t, reg, "gnn-v1", "1.0.0")
	registerTestModel(t, reg, "gnn-v1", "1.1.0")
	registerTestModel(t, reg, "gnn-v1", "2.0.0")

	versions, err := reg.ListVersions(context.Background(), "gnn-v1")
	if err != nil {
		t.Fatalf("ListVersions: %v", err)
	}
	if len(versions) != 3 {
		t.Fatalf("expected 3 versions, got %d", len(versions))
	}
	// Should be sorted
	if versions[0].Version != "1.0.0" {
		t.Errorf("expected first version 1.0.0, got %s", versions[0].Version)
	}
}

func TestListVersions_ModelNotFound(t *testing.T) {
	reg, _, _ := newRegistryTestHelper(t)
	_, err := reg.ListVersions(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent model")
	}
}

// ---------------------------------------------------------------------------
// Tests: SetActiveVersion
// ---------------------------------------------------------------------------

func TestSetActiveVersion_Success(t *testing.T) {
	reg, _, _ := newRegistryTestHelper(t)
	registerTestModel(t, reg, "gnn-v1", "1.0.0")
	registerTestModel(t, reg, "gnn-v1", "2.0.0")

	if err := reg.SetActiveVersion(context.Background(), "gnn-v1", "1.0.0"); err != nil {
		t.Fatalf("SetActiveVersion 1.0.0: %v", err)
	}
	if err := reg.SetActiveVersion(context.Background(), "gnn-v1", "2.0.0"); err != nil {
		t.Fatalf("SetActiveVersion 2.0.0: %v", err)
	}

	model, err := reg.GetModel(context.Background(), "gnn-v1")
	if err != nil {
		t.Fatalf("GetModel: %v", err)
	}
	if model.ActiveVersion != "2.0.0" {
		t.Errorf("expected active 2.0.0, got %s", model.ActiveVersion)
	}
}

func TestSetActiveVersion_PreviousVersionRecorded(t *testing.T) {
	reg, _, _ := newRegistryTestHelper(t)
	activateTestModel(t, reg, "gnn-v1", "1.0.0")
	registerTestModel(t, reg, "gnn-v1", "2.0.0")

	if err := reg.SetActiveVersion(context.Background(), "gnn-v1", "2.0.0"); err != nil {
		t.Fatalf("SetActiveVersion: %v", err)
	}

	model, _ := reg.GetModel(context.Background(), "gnn-v1")
	if model.PreviousVersion != "1.0.0" {
		t.Errorf("expected previous 1.0.0, got %s", model.PreviousVersion)
	}
}

func TestSetActiveVersion_AutoLoad(t *testing.T) {
	reg, loader, _ := newRegistryTestHelper(t)
	registerTestModel(t, reg, "gnn-v1", "1.0.0")

	// Version is REGISTERED, SetActiveVersion should auto-load
	if err := reg.SetActiveVersion(context.Background(), "gnn-v1", "1.0.0"); err != nil {
		t.Fatalf("SetActiveVersion: %v", err)
	}
	if loader.getLoadCalls() != 1 {
		t.Errorf("expected 1 load call, got %d", loader.getLoadCalls())
	}

	versions, _ := reg.ListVersions(context.Background(), "gnn-v1")
	if versions[0].Status != VersionStatusReady {
		t.Errorf("expected READY after auto-load, got %s", versions[0].Status)
	}
}

func TestSetActiveVersion_LoadFailure(t *testing.T) {
	loader := newMockModelLoader()
	loader.loadErr = fmt.Errorf("disk full")
	reg, err := NewModelRegistry(loader, nil, nil,
		WithRegistryHealthCheckInterval(1*time.Hour),
	)
	if err != nil {
		t.Fatalf("NewModelRegistry: %v", err)
	}
	defer reg.Close()

	meta := newTestModelMetadata("gnn-v1", "1.0.0")
	_ = reg.Register(context.Background(), meta)

	err = reg.SetActiveVersion(context.Background(), "gnn-v1", "1.0.0")
	if err == nil {
		t.Fatal("expected error for load failure")
	}

	versions, _ := reg.ListVersions(context.Background(), "gnn-v1")
	if versions[0].Status != VersionStatusFailed {
		t.Errorf("expected FAILED after load failure, got %s", versions[0].Status)
	}
}

func TestSetActiveVersion_VersionNotFound(t *testing.T) {
	reg, _, _ := newRegistryTestHelper(t)
	registerTestModel(t, reg, "gnn-v1", "1.0.0")

	err := reg.SetActiveVersion(context.Background(), "gnn-v1", "9.9.9")
	if err == nil {
		t.Fatal("expected error for nonexistent version")
	}
}

func TestSetActiveVersion_DeprecatedVersion(t *testing.T) {
	reg, _, _ := newRegistryTestHelper(t)
	registerTestModel(t, reg, "gnn-v1", "1.0.0")

	// Manually deprecate via internal access (in production this would be an API)
	raw, _ := reg.(*modelRegistry).models.Load("gnn-v1")
	entry := raw.(*modelEntry)
	entry.mu.Lock()
	entry.versions["1.0.0"].info.Status = VersionStatusDeprecated
	entry.mu.Unlock()

	err := reg.SetActiveVersion(context.Background(), "gnn-v1", "1.0.0")
	if err == nil {
		t.Fatal("expected error for deprecated version")
	}
}

func TestSetActiveVersion_SameVersion(t *testing.T) {
	reg, loader, _ := newRegistryTestHelper(t)
	activateTestModel(t, reg, "gnn-v1", "1.0.0")

	loadsBefore := loader.getLoadCalls()
	err := reg.SetActiveVersion(context.Background(), "gnn-v1", "1.0.0")
	if err != nil {
		t.Fatalf("same version should be no-op: %v", err)
	}
	if loader.getLoadCalls() != loadsBefore {
		t.Error("same version should not trigger additional load")
	}
}

// ---------------------------------------------------------------------------
// Tests: Rollback
// ---------------------------------------------------------------------------

func TestRollback_Success(t *testing.T) {
	reg, _, _ := newRegistryTestHelper(t)
	activateTestModel(t, reg, "gnn-v1", "1.0.0")
	registerTestModel(t, reg, "gnn-v1", "2.0.0")
	_ = reg.SetActiveVersion(context.Background(), "gnn-v1", "2.0.0")

	if err := reg.Rollback(context.Background(), "gnn-v1"); err != nil {
		t.Fatalf("Rollback: %v", err)
	}

	model, _ := reg.GetModel(context.Background(), "gnn-v1")
	if model.ActiveVersion != "1.0.0" {
		t.Errorf("expected rollback to 1.0.0, got %s", model.ActiveVersion)
	}
}

func TestRollback_NoPreviousVersion(t *testing.T) {
	reg, _, _ := newRegistryTestHelper(t)
	activateTestModel(t, reg, "gnn-v1", "1.0.0")

	err := reg.Rollback(context.Background(), "gnn-v1")
	if err == nil {
		t.Fatal("expected error for no previous version")
	}
}

func TestRollback_PreviousVersionUnloaded(t *testing.T) {
	reg, loader, _ := newRegistryTestHelper(t)
	activateTestModel(t, reg, "gnn-v1", "1.0.0")
	registerTestModel(t, reg, "gnn-v1", "2.0.0")
	_ = reg.SetActiveVersion(context.Background(), "gnn-v1", "2.0.0")

	// With unloadDelay=0, version 1.0.0 should be unloaded immediately
	time.Sleep(50 * time.Millisecond)

	loadsBefore := loader.getLoadCalls()
	if err := reg.Rollback(context.Background(), "gnn-v1"); err != nil {
		t.Fatalf("Rollback: %v", err)
	}

	// Should have re-loaded version 1.0.0
	if loader.getLoadCalls() <= loadsBefore {
		t.Error("expected re-load of unloaded previous version")
	}

	model, _ := reg.GetModel(context.Background(), "gnn-v1")
	if model.ActiveVersion != "1.0.0" {
		t.Errorf("expected rollback to 1.0.0, got %s", model.ActiveVersion)
	}
}

// ---------------------------------------------------------------------------
// Tests: ConfigureABTest
// ---------------------------------------------------------------------------

func TestConfigureABTest_Success(t *testing.T) {
	reg, _, _ := newRegistryTestHelper(t)
	activateTestModel(t, reg, "gnn-v1", "1.0.0")
	registerTestModel(t, reg, "gnn-v1", "2.0.0")
	_ = reg.SetActiveVersion(context.Background(), "gnn-v1", "2.0.0")

	cfg := &ABTestConfig{
		ModelID: "gnn-v1",
		Variants: []*ABTestVariant{
			{Version: "1.0.0", TrafficWeight: 30},
			{Version: "2.0.0", TrafficWeight: 70},
		},
		StartTime: time.Now().Add(-1 * time.Hour),
		EndTime:   time.Now().Add(1 * time.Hour),
		Enabled:   true,
	}
	if err := reg.ConfigureABTest(context.Background(), cfg); err != nil {
		t.Fatalf("ConfigureABTest: %v", err)
	}
}

func TestConfigureABTest_InvalidWeights(t *testing.T) {
	reg, _, _ := newRegistryTestHelper(t)
	activateTestModel(t, reg, "gnn-v1", "1.0.0")
	registerTestModel(t, reg, "gnn-v1", "2.0.0")
	_ = reg.SetActiveVersion(context.Background(), "gnn-v1", "2.0.0")

	cfg := &ABTestConfig{
		ModelID: "gnn-v1",
		Variants: []*ABTestVariant{
			{Version: "1.0.0", TrafficWeight: 30},
			{Version: "2.0.0", TrafficWeight: 60}, // sum = 90, not 100
		},
		Enabled: true,
	}
	err := reg.ConfigureABTest(context.Background(), cfg)
	if err == nil {
		t.Fatal("expected error for weights not summing to 100")
	}
}

func TestConfigureABTest_ZeroVariants(t *testing.T) {
	reg, _, _ := newRegistryTestHelper(t)
	activateTestModel(t, reg, "gnn-v1", "1.0.0")

	cfg := &ABTestConfig{
		ModelID:  "gnn-v1",
		Variants: []*ABTestVariant{},
		Enabled:  true,
	}
	err := reg.ConfigureABTest(context.Background(), cfg)
	if err == nil {
		t.Fatal("expected error for zero variants")
	}
}

func TestConfigureABTest_VersionNotFound(t *testing.T) {
	reg, _, _ := newRegistryTestHelper(t)
	activateTestModel(t, reg, "gnn-v1", "1.0.0")

	cfg := &ABTestConfig{
		ModelID: "gnn-v1",
		Variants: []*ABTestVariant{
			{Version: "1.0.0", TrafficWeight: 50},
			{Version: "9.9.9", TrafficWeight: 50}, // does not exist
		},
		Enabled: true,
	}
	err := reg.ConfigureABTest(context.Background(), cfg)
	if err == nil {
		t.Fatal("expected error for nonexistent version in variant")
	}
}

func TestConfigureABTest_Disable(t *testing.T) {
	reg, _, _ := newRegistryTestHelper(t)
	activateTestModel(t, reg, "gnn-v1", "1.0.0")
	registerTestModel(t, reg, "gnn-v1", "2.0.0")
	_ = reg.SetActiveVersion(context.Background(), "gnn-v1", "2.0.0")

	// First enable
	cfg := &ABTestConfig{
		ModelID: "gnn-v1",
		Variants: []*ABTestVariant{
			{Version: "1.0.0", TrafficWeight: 50},
			{Version: "2.0.0", TrafficWeight: 50},
		},
		Enabled: true,
	}
	_ = reg.ConfigureABTest(context.Background(), cfg)

	// Now disable
	disableCfg := &ABTestConfig{
		ModelID: "gnn-v1",
		Enabled: false,
	}
	if err := reg.ConfigureABTest(context.Background(), disableCfg); err != nil {
		t.Fatalf("disable A/B test: %v", err)
	}

	// ResolveModel should return active version, not A/B routed
	model, err := reg.ResolveModel(context.Background(), "gnn-v1", "req-001")
	if err != nil {
		t.Fatalf("ResolveModel: %v", err)
	}
	if model.ActiveVersion != "2.0.0" {
		t.Errorf("expected active version 2.0.0 after disable, got %s", model.ActiveVersion)
	}
}

// ---------------------------------------------------------------------------
// Tests: ResolveModel
// ---------------------------------------------------------------------------

func TestResolveModel_NoABTest(t *testing.T) {
	reg, _, _ := newRegistryTestHelper(t)
	activateTestModel(t, reg, "gnn-v1", "1.0.0")

	model, err := reg.ResolveModel(context.Background(), "gnn-v1", "any-request-id")
	if err != nil {
		t.Fatalf("ResolveModel: %v", err)
	}
	if model.ActiveVersion != "1.0.0" {
		t.Errorf("expected 1.0.0, got %s", model.ActiveVersion)
	}
}

func TestResolveModel_ABTest_DeterministicRouting(t *testing.T) {
	reg, _, _ := newRegistryTestHelper(t)
	activateTestModel(t, reg, "gnn-v1", "1.0.0")
	registerTestModel(t, reg, "gnn-v1", "2.0.0")
	_ = reg.SetActiveVersion(context.Background(), "gnn-v1", "2.0.0")

	cfg := &ABTestConfig{
		ModelID: "gnn-v1",
		Variants: []*ABTestVariant{
			{Version: "1.0.0", TrafficWeight: 50},
			{Version: "2.0.0", TrafficWeight: 50},
		},
		StartTime: time.Now().Add(-1 * time.Hour),
		EndTime:   time.Now().Add(1 * time.Hour),
		Enabled:   true,
	}
	_ = reg.ConfigureABTest(context.Background(), cfg)

	requestID := "deterministic-test-request-42"
	var firstResult string
	for i := 0; i < 20; i++ {
		model, err := reg.ResolveModel(context.Background(), "gnn-v1", requestID)
		if err != nil {
			t.Fatalf("ResolveModel iteration %d: %v", i, err)
		}
		ver := model.Metadata.Version
		if i == 0 {
			firstResult = ver
		} else if ver != firstResult {
			t.Fatalf("deterministic routing broken: iteration %d got %s, expected %s", i, ver, firstResult)
		}
	}
}

func TestResolveModel_ABTest_TrafficDistribution(t *testing.T) {
	reg, _, _ := newRegistryTestHelper(t)
	activateTestModel(t, reg, "gnn-v1", "1.0.0")
	registerTestModel(t, reg, "gnn-v1", "2.0.0")
	_ = reg.SetActiveVersion(context.Background(), "gnn-v1", "2.0.0")

	cfg := &ABTestConfig{
		ModelID: "gnn-v1",
		Variants: []*ABTestVariant{
			{Version: "1.0.0", TrafficWeight: 30},
			{Version: "2.0.0", TrafficWeight: 70},
		},
		StartTime: time.Now().Add(-1 * time.Hour),
		EndTime:   time.Now().Add(1 * time.Hour),
		Enabled:   true,
	}
	_ = reg.ConfigureABTest(context.Background(), cfg)

	counts := map[string]int{"1.0.0": 0, "2.0.0": 0}
	totalRequests := 1000
	for i := 0; i < totalRequests; i++ {
		reqID := fmt.Sprintf("traffic-test-req-%d", i)
		model, err := reg.ResolveModel(context.Background(), "gnn-v1", reqID)
		if err != nil {
			t.Fatalf("ResolveModel: %v", err)
		}
		counts[model.Metadata.Version]++
	}

	// Check distribution within 5% tolerance
	v1Pct := float64(counts["1.0.0"]) / float64(totalRequests) * 100
	v2Pct := float64(counts["2.0.0"]) / float64(totalRequests) * 100

	if math.Abs(v1Pct-30) > 5 {
		t.Errorf("version 1.0.0 traffic: %.1f%%, expected ~30%% (tolerance 5%%)", v1Pct)
	}
	if math.Abs(v2Pct-70) > 5 {
		t.Errorf("version 2.0.0 traffic: %.1f%%, expected ~70%% (tolerance 5%%)", v2Pct)
	}
}

func TestResolveModel_ABTest_Expired(t *testing.T) {
	reg, _, _ := newRegistryTestHelper(t)
	activateTestModel(t, reg, "gnn-v1", "1.0.0")
	registerTestModel(t, reg, "gnn-v1", "2.0.0")
	_ = reg.SetActiveVersion(context.Background(), "gnn-v1", "2.0.0")

	cfg := &ABTestConfig{
		ModelID: "gnn-v1",
		Variants: []*ABTestVariant{
			{Version: "1.0.0", TrafficWeight: 50},
			{Version: "2.0.0", TrafficWeight: 50},
		},
		StartTime: time.Now().Add(-2 * time.Hour),
		EndTime:   time.Now().Add(-1 * time.Hour), // already expired
		Enabled:   true,
	}
	_ = reg.ConfigureABTest(context.Background(), cfg)

	model, err := reg.ResolveModel(context.Background(), "gnn-v1", "req-001")
	if err != nil {
		t.Fatalf("ResolveModel: %v", err)
	}
	// Should fall back to active version since A/B test expired
	if model.ActiveVersion != "2.0.0" {
		t.Errorf("expected active version 2.0.0 for expired A/B test, got %s", model.ActiveVersion)
	}
}

func TestResolveModel_ABTest_NotStarted(t *testing.T) {
	reg, _, _ := newRegistryTestHelper(t)
	activateTestModel(t, reg, "gnn-v1", "1.0.0")
	registerTestModel(t, reg, "gnn-v1", "2.0.0")
	_ = reg.SetActiveVersion(context.Background(), "gnn-v1", "2.0.0")

	cfg := &ABTestConfig{
		ModelID: "gnn-v1",
		Variants: []*ABTestVariant{
			{Version: "1.0.0", TrafficWeight: 50},
			{Version: "2.0.0", TrafficWeight: 50},
		},
		StartTime: time.Now().Add(1 * time.Hour), // not started yet
		EndTime:   time.Now().Add(2 * time.Hour),
		Enabled:   true,
	}
	_ = reg.ConfigureABTest(context.Background(), cfg)

	model, err := reg.ResolveModel(context.Background(), "gnn-v1", "req-001")
	if err != nil {
		t.Fatalf("ResolveModel: %v", err)
	}
	if model.ActiveVersion != "2.0.0" {
		t.Errorf("expected active version 2.0.0 for not-started A/B test, got %s", model.ActiveVersion)
	}
}

// ---------------------------------------------------------------------------
// Tests: HealthCheck
// ---------------------------------------------------------------------------

func TestHealthCheck_AllHealthy(t *testing.T) {
	reg, _, _ := newRegistryTestHelper(t)
	activateTestModel(t, reg, "model-a", "1.0.0")
	activateTestModel(t, reg, "model-b", "1.0.0")

	health, err := reg.HealthCheck(context.Background())
	if err != nil {
		t.Fatalf("HealthCheck: %v", err)
	}
	if health.TotalModels != 2 {
		t.Errorf("expected 2 total models, got %d", health.TotalModels)
	}
	if health.ActiveModels != 2 {
		t.Errorf("expected 2 active models, got %d", health.ActiveModels)
	}
	if health.FailedModels != 0 {
		t.Errorf("expected 0 failed models, got %d", health.FailedModels)
	}
}

func TestHealthCheck_SomeFailed(t *testing.T) {
	reg, _, _ := newRegistryTestHelper(t)
	activateTestModel(t, reg, "model-a", "1.0.0")
	activateTestModel(t, reg, "model-b", "1.0.0")

	// Manually set model-b to failed
	raw, _ := reg.(*modelRegistry).models.Load("model-b")
	entry := raw.(*modelEntry)
	entry.mu.Lock()
	entry.versions["1.0.0"].info.Status = VersionStatusFailed
	entry.mu.Unlock()

	health, err := reg.HealthCheck(context.Background())
	if err != nil {
		t.Fatalf("HealthCheck: %v", err)
	}
	if health.FailedModels != 1 {
		t.Errorf("expected 1 failed model, got %d", health.FailedModels)
	}
	if health.ActiveModels != 1 {
		t.Errorf("expected 1 active model, got %d", health.ActiveModels)
	}
}

func TestHealthCheck_NoModels(t *testing.T) {
	reg, _, _ := newRegistryTestHelper(t)
	health, err := reg.HealthCheck(context.Background())
	if err != nil {
		t.Fatalf("HealthCheck: %v", err)
	}
	if health.TotalModels != 0 {
		t.Errorf("expected 0 total models, got %d", health.TotalModels)
	}
}

// ---------------------------------------------------------------------------
// Tests: Concurrency
// ---------------------------------------------------------------------------

func TestConcurrent_RegisterAndResolve(t *testing.T) {
	reg, _, _ := newRegistryTestHelper(t)
	activateTestModel(t, reg, "gnn-v1", "1.0.0")

	var wg sync.WaitGroup
	errCh := make(chan error, 200)

	// Concurrent registers
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			ver := fmt.Sprintf("1.%d.0", idx+1)
			meta := newTestModelMetadata("gnn-v1", ver)
			if err := reg.Register(context.Background(), meta); err != nil {
				errCh <- fmt.Errorf("register %s: %w", ver, err)
			}
		}(i)
	}

	// Concurrent resolves
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			reqID := fmt.Sprintf("req-%d", idx)
			_, err := reg.ResolveModel(context.Background(), "gnn-v1", reqID)
			if err != nil {
				errCh <- fmt.Errorf("resolve %s: %w", reqID, err)
			}
		}(i)
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Errorf("concurrent error: %v", err)
	}
}

func TestConcurrent_SetActiveAndGetModel(t *testing.T) {
	reg, _, _ := newRegistryTestHelper(t)
	// Pre-register multiple versions
	versions := []string{"1.0.0", "1.1.0", "1.2.0", "1.3.0", "1.4.0"}
	for _, v := range versions {
		registerTestModel(t, reg, "gnn-v1", v)
	}
	_ = reg.SetActiveVersion(context.Background(), "gnn-v1", "1.0.0")

	var wg sync.WaitGroup
	errCh := make(chan error, 200)

	// Concurrent version switches
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			ver := versions[idx%len(versions)]
			err := reg.SetActiveVersion(context.Background(), "gnn-v1", ver)
			if err != nil {
				// Some failures are expected due to concurrent state changes
				// but no panics or data races should occur
				_ = err
			}
		}(i)
	}

	// Concurrent reads
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			model, err := reg.GetModel(context.Background(), "gnn-v1")
			if err != nil {
				// Transient errors acceptable during concurrent switches
				return
			}
			if model.ActiveVersion == "" {
				errCh <- fmt.Errorf("got empty active version")
			}
		}()
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Errorf("concurrent error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Tests: Delayed Unload
// ---------------------------------------------------------------------------

func TestDelayedUnload(t *testing.T) {
	loader := newMockModelLoader()
	metrics := &mockRegistryMetrics{}
	reg, err := NewModelRegistry(loader, metrics, NewNoopLogger(),
		WithRegistryHealthCheckInterval(1*time.Hour),
		WithUnloadDelay(200*time.Millisecond), // short delay for testing
		WithMaxLoadedVersions(10),
	)
	if err != nil {
		t.Fatalf("NewModelRegistry: %v", err)
	}
	defer reg.Close()

	meta1 := newTestModelMetadata("gnn-v1", "1.0.0")
	_ = reg.Register(context.Background(), meta1)
	_ = reg.SetActiveVersion(context.Background(), "gnn-v1", "1.0.0")

	meta2 := newTestModelMetadata("gnn-v1", "2.0.0")
	_ = reg.Register(context.Background(), meta2)
	_ = reg.SetActiveVersion(context.Background(), "gnn-v1", "2.0.0")

	// Immediately after switch, old version should still be accessible
	model, err := reg.GetModelVersion(context.Background(), "gnn-v1", "1.0.0")
	if err != nil {
		t.Fatalf("old version should still be accessible: %v", err)
	}
	// Status might still be READY briefly
	_ = model

	// Wait for delayed unload
	time.Sleep(400 * time.Millisecond)

	// After delay, old version should be unloaded
	model2, err := reg.GetModelVersion(context.Background(), "gnn-v1", "1.0.0")
	if err != nil {
		t.Fatalf("version entry should still exist: %v", err)
	}
	// The version entry still exists but its status should be UNLOADED
	versions, _ := reg.ListVersions(context.Background(), "gnn-v1")
	for _, v := range versions {
		if v.Version == "1.0.0" && v.Status != VersionStatusUnloaded {
			t.Errorf("expected UNLOADED after delay, got %s", v.Status)
		}
	}
	_ = model2
}

// ---------------------------------------------------------------------------
// Tests: MaxLoadedVersions
// ---------------------------------------------------------------------------

func TestMaxLoadedVersions(t *testing.T) {
	loader := newMockModelLoader()
	reg, err := NewModelRegistry(loader, nil, NewNoopLogger(),
		WithRegistryHealthCheckInterval(1*time.Hour),
		WithUnloadDelay(0),
		WithMaxLoadedVersions(2), // only 2 loaded at a time
	)
	if err != nil {
		t.Fatalf("NewModelRegistry: %v", err)
	}
	defer reg.Close()

	// Register and activate 3 versions sequentially
	for _, v := range []string{"1.0.0", "2.0.0", "3.0.0"} {
		meta := newTestModelMetadata("gnn-v1", v)
		_ = reg.Register(context.Background(), meta)
		_ = reg.SetActiveVersion(context.Background(), "gnn-v1", v)
	}

	// With maxLoadedVersions=2, the oldest non-active version should be evicted
	versions, _ := reg.ListVersions(context.Background(), "gnn-v1")
	readyCount := 0
	for _, v := range versions {
		if v.Status == VersionStatusReady {
			readyCount++
		}
	}
	// Active version (3.0.0) is always loaded; at most 1 more can be loaded
	if readyCount > 2 {
		t.Errorf("expected at most 2 ready versions, got %d", readyCount)
	}
}

// ---------------------------------------------------------------------------
// Tests: Metrics
// ---------------------------------------------------------------------------

func TestMetrics_RegisterRecorded(t *testing.T) {
	reg, _, metrics := newRegistryTestHelper(t)
	before := metrics.modelLoadCount.Load()
	registerTestModel(t, reg, "gnn-v1", "1.0.0")
	after := metrics.modelLoadCount.Load()
	if after <= before {
		t.Error("expected metric recorded on register")
	}
}

func TestMetrics_SetActiveRecorded(t *testing.T) {
	reg, _, metrics := newRegistryTestHelper(t)
	registerTestModel(t, reg, "gnn-v1", "1.0.0")
	before := metrics.modelLoadCount.Load()
	_ = reg.SetActiveVersion(context.Background(), "gnn-v1", "1.0.0")
	after := metrics.modelLoadCount.Load()
	if after <= before {
		t.Error("expected metric recorded on SetActiveVersion")
	}
}

// ---------------------------------------------------------------------------
// Tests: Enum String methods
// ---------------------------------------------------------------------------

func TestModelStatus_String(t *testing.T) {
	tests := []struct {
		s    ModelStatus
		want string
	}{
		{ModelStatusActive, "ACTIVE"},
		{ModelStatusLoading, "LOADING"},
		{ModelStatusFailed, "FAILED"},
		{ModelStatusDeprecated, "DEPRECATED"},
		{ModelStatus(99), "UNKNOWN(99)"},
	}
	for _, tt := range tests {
		if got := tt.s.String(); got != tt.want {
			t.Errorf("ModelStatus(%d).String() = %s, want %s", tt.s, got, tt.want)
		}
	}
}

func TestVersionStatus_String(t *testing.T) {
	tests := []struct {
		s    VersionStatus
		want string
	}{
		{VersionStatusRegistered, "REGISTERED"},
		{VersionStatusLoading, "LOADING"},
		{VersionStatusReady, "READY"},
		{VersionStatusFailed, "FAILED"},
		{VersionStatusDeprecated, "DEPRECATED"},
		{VersionStatusUnloaded, "UNLOADED"},
		{VersionStatus(99), "UNKNOWN(99)"},
	}
	for _, tt := range tests {
		if got := tt.s.String(); got != tt.want {
			t.Errorf("VersionStatus(%d).String() = %s, want %s", tt.s, got, tt.want)
		}
	}
}

//Personal.AI order the ending


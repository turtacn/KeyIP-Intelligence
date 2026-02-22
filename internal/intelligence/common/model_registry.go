package common

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
)

// ModelRegistry defines the interface for model management.
type ModelRegistry interface {
	Register(ctx context.Context, meta *ModelMetadata) error
	Unregister(ctx context.Context, modelID string, version string) error
	GetModel(ctx context.Context, modelID string) (*RegisteredModel, error)
	GetModelVersion(ctx context.Context, modelID string, version string) (*RegisteredModel, error)
	ListModels(ctx context.Context) ([]*RegisteredModel, error)
	ListVersions(ctx context.Context, modelID string) ([]*ModelVersion, error)
	SetActiveVersion(ctx context.Context, modelID string, version string) error
	Rollback(ctx context.Context, modelID string) error
	ConfigureABTest(ctx context.Context, config *ABTestConfig) error
	ResolveModel(ctx context.Context, modelID string, requestID string) (*RegisteredModel, error)
	HealthCheck(ctx context.Context) (*RegistryHealth, error)
}

// ModelMetadata contains metadata about a model.
type ModelMetadata struct {
	ModelID      string                 `json:"model_id"`
	ModelName    string                 `json:"model_name"`
	Version      string                 `json:"version"`
	ArtifactPath string                 `json:"artifact_path"`
	Checksum     string                 `json:"checksum"`
	TrainedAt    time.Time              `json:"trained_at"`
	Architecture string                 `json:"architecture"`
	Tasks        []string               `json:"tasks"`
	Metrics      map[string]float64     `json:"metrics"`
	Parameters   map[string]interface{} `json:"parameters"`
}

// ModelStatus represents the status of a model.
type ModelStatus string

const (
	ModelStatusActive     ModelStatus = "active"
	ModelStatusLoading    ModelStatus = "loading"
	ModelStatusFailed     ModelStatus = "failed"
	ModelStatusDeprecated ModelStatus = "deprecated"
)

// VersionStatus represents the status of a specific model version.
type VersionStatus string

const (
	VersionStatusRegistered VersionStatus = "registered"
	VersionStatusLoading    VersionStatus = "loading"
	VersionStatusReady      VersionStatus = "ready"
	VersionStatusFailed     VersionStatus = "failed"
	VersionStatusDeprecated VersionStatus = "deprecated"
)

// RegisteredModel represents a registered model.
type RegisteredModel struct {
	ModelID         string         `json:"model_id"`
	Name            string         `json:"name"`
	Description     string         `json:"description"`
	ActiveVersion   string         `json:"active_version"`
	PreviousVersion string         `json:"previous_version"`
	Metadata        *ModelMetadata `json:"metadata"`
	Status          ModelStatus    `json:"status"`
	LoadedAt        time.Time      `json:"loaded_at"`
	LastUsedAt      time.Time      `json:"last_used_at"`
}

// ModelVersion represents a specific version of a model.
type ModelVersion struct {
	Version      string         `json:"version"`
	CreatedAt    time.Time      `json:"created_at"`
	Status       VersionStatus  `json:"status"`
	Metadata     *ModelMetadata `json:"metadata"`
	ArtifactPath string         `json:"artifact_path"`
	Checksum     string         `json:"checksum"`
	SizeBytes    int64          `json:"size_bytes"`
}

// ABTestConfig defines configuration for A/B testing.
type ABTestConfig struct {
	ModelID   string           `json:"model_id"`
	Variants  []*ABTestVariant `json:"variants"`
	StartTime time.Time        `json:"start_time"`
	EndTime   time.Time        `json:"end_time"`
	Enabled   bool             `json:"enabled"`
}

// ABTestVariant defines a variant in an A/B test.
type ABTestVariant struct {
	Version       string `json:"version"`
	TrafficWeight int    `json:"traffic_weight"` // 0-100
	Description   string `json:"description"`
}

// RegistryHealth contains health information about the registry.
type RegistryHealth struct {
	TotalModels   int                         `json:"total_models"`
	ActiveModels  int                         `json:"active_models"`
	FailedModels  int                         `json:"failed_models"`
	LoadingModels int                         `json:"loading_models"`
	ModelHealths  map[string]*ModelHealthStatus `json:"model_healths"`
}

// ModelHealthStatus contains health information for a specific model.
type ModelHealthStatus struct {
	ModelID         string        `json:"model_id"`
	Version         string        `json:"version"`
	Status          ModelStatus   `json:"status"`
	LastHealthCheck time.Time     `json:"last_health_check"`
	LatencyMs       float64       `json:"latency_ms"`
	ErrorRate       float64       `json:"error_rate"`
}

// ModelLoader defines the interface for loading models.
type ModelLoader interface {
	Load(ctx context.Context, artifactPath string) (interface{}, error)
	Unload(ctx context.Context, modelHandle interface{}) error
	Validate(ctx context.Context, artifactPath string, checksum string) error
}

// modelRegistry implements ModelRegistry.
type modelRegistry struct {
	models  sync.Map // key: modelID, value: *modelEntry
	loader  ModelLoader
	metrics IntelligenceMetrics
	logger  logging.Logger
}

type modelEntry struct {
	versions        map[string]*ModelVersion
	activeVersion   string
	previousVersion string
	abTestConfig    *ABTestConfig
	mu              sync.RWMutex
}

var (
	ErrModelNotFound           = errors.New("model not found")
	ErrVersionAlreadyExists    = errors.New("version already exists")
	ErrVersionNotFound         = errors.New("version not found")
	ErrNoActiveVersion         = errors.New("no active version")
	ErrNoPreviousVersion       = errors.New("no previous version")
	ErrInvalidVersion          = errors.New("invalid version format")
	ErrVersionDeprecated       = errors.New("version deprecated")
	ErrInvalidABTestConfig     = errors.New("invalid AB test config")
	ErrModelNotReady           = errors.New("model not ready")
)

// NewModelRegistry creates a new ModelRegistry.
func NewModelRegistry(loader ModelLoader, metrics IntelligenceMetrics, logger logging.Logger) (ModelRegistry, error) {
	if loader == nil {
		return nil, errors.New("loader cannot be nil")
	}
	return &modelRegistry{
		loader:  loader,
		metrics: metrics,
		logger:  logger,
	}, nil
}

func (r *modelRegistry) Register(ctx context.Context, meta *ModelMetadata) error {
	if meta.ModelID == "" || meta.Version == "" {
		return errors.New("model ID and version are required")
	}

	if err := r.loader.Validate(ctx, meta.ArtifactPath, meta.Checksum); err != nil {
		return fmt.Errorf("model validation failed: %w", err)
	}

	value, _ := r.models.LoadOrStore(meta.ModelID, &modelEntry{
		versions: make(map[string]*ModelVersion),
	})
	entry := value.(*modelEntry)

	entry.mu.Lock()
	defer entry.mu.Unlock()

	if _, exists := entry.versions[meta.Version]; exists {
		return ErrVersionAlreadyExists
	}

	entry.versions[meta.Version] = &ModelVersion{
		Version:      meta.Version,
		CreatedAt:    time.Now().UTC(),
		Status:       VersionStatusRegistered,
		Metadata:     meta,
		ArtifactPath: meta.ArtifactPath,
		Checksum:     meta.Checksum,
	}

	r.logger.Info("Model registered", logging.String("model_id", meta.ModelID), logging.String("version", meta.Version))
	return nil
}

func (r *modelRegistry) Unregister(ctx context.Context, modelID string, version string) error {
	// Not implemented for brevity
	return nil
}

func (r *modelRegistry) GetModel(ctx context.Context, modelID string) (*RegisteredModel, error) {
	value, ok := r.models.Load(modelID)
	if !ok {
		return nil, ErrModelNotFound
	}
	entry := value.(*modelEntry)

	entry.mu.RLock()
	defer entry.mu.RUnlock()

	if entry.activeVersion == "" {
		return nil, ErrNoActiveVersion
	}

	ver := entry.versions[entry.activeVersion]
	return &RegisteredModel{
		ModelID:       modelID,
		ActiveVersion: entry.activeVersion,
		Metadata:      ver.Metadata,
		Status:        ModelStatusActive, // Simplified
	}, nil
}

func (r *modelRegistry) GetModelVersion(ctx context.Context, modelID string, version string) (*RegisteredModel, error) {
	value, ok := r.models.Load(modelID)
	if !ok {
		return nil, ErrModelNotFound
	}
	entry := value.(*modelEntry)

	entry.mu.RLock()
	defer entry.mu.RUnlock()

	ver, ok := entry.versions[version]
	if !ok {
		return nil, ErrVersionNotFound
	}

	return &RegisteredModel{
		ModelID:       modelID,
		ActiveVersion: version, // This is the requested version
		Metadata:      ver.Metadata,
		Status:        ModelStatusActive, // Should map from VersionStatus
	}, nil
}

func (r *modelRegistry) ListModels(ctx context.Context) ([]*RegisteredModel, error) {
	var models []*RegisteredModel
	r.models.Range(func(key, value interface{}) bool {
		entry := value.(*modelEntry)
		entry.mu.RLock()
		defer entry.mu.RUnlock()

		if entry.activeVersion != "" {
			ver := entry.versions[entry.activeVersion]
			models = append(models, &RegisteredModel{
				ModelID: key.(string),
				ActiveVersion: entry.activeVersion,
				Metadata: ver.Metadata,
			})
		}
		return true
	})
	return models, nil
}

func (r *modelRegistry) ListVersions(ctx context.Context, modelID string) ([]*ModelVersion, error) {
	value, ok := r.models.Load(modelID)
	if !ok {
		return nil, ErrModelNotFound
	}
	entry := value.(*modelEntry)

	entry.mu.RLock()
	defer entry.mu.RUnlock()

	var versions []*ModelVersion
	for _, v := range entry.versions {
		versions = append(versions, v)
	}
	return versions, nil
}

func (r *modelRegistry) SetActiveVersion(ctx context.Context, modelID string, version string) error {
	value, ok := r.models.Load(modelID)
	if !ok {
		return ErrModelNotFound
	}
	entry := value.(*modelEntry)

	entry.mu.Lock()
	defer entry.mu.Unlock()

	ver, ok := entry.versions[version]
	if !ok {
		return ErrVersionNotFound
	}

	if ver.Status == VersionStatusDeprecated {
		return ErrVersionDeprecated
	}

	if ver.Status != VersionStatusReady {
		// Try to load
		_, err := r.loader.Load(ctx, ver.ArtifactPath)
		if err != nil {
			ver.Status = VersionStatusFailed
			return err
		}
		ver.Status = VersionStatusReady
	}

	entry.previousVersion = entry.activeVersion
	entry.activeVersion = version

	r.logger.Info("Active version set", logging.String("model_id", modelID), logging.String("version", version))
	return nil
}

func (r *modelRegistry) Rollback(ctx context.Context, modelID string) error {
	value, ok := r.models.Load(modelID)
	if !ok {
		return ErrModelNotFound
	}
	entry := value.(*modelEntry)

	entry.mu.RLock()
	prev := entry.previousVersion
	entry.mu.RUnlock()

	if prev == "" {
		return ErrNoPreviousVersion
	}

	return r.SetActiveVersion(ctx, modelID, prev)
}

func (r *modelRegistry) ConfigureABTest(ctx context.Context, config *ABTestConfig) error {
	value, ok := r.models.Load(config.ModelID)
	if !ok {
		return ErrModelNotFound
	}
	entry := value.(*modelEntry)

	// Validate config
	if !config.Enabled {
		entry.mu.Lock()
		entry.abTestConfig = nil
		entry.mu.Unlock()
		return nil
	}

	totalWeight := 0
	for _, v := range config.Variants {
		totalWeight += v.TrafficWeight
		// Check if version exists
		entry.mu.RLock()
		_, ok := entry.versions[v.Version]
		entry.mu.RUnlock()
		if !ok {
			return ErrVersionNotFound
		}
	}
	if totalWeight != 100 {
		return ErrInvalidABTestConfig
	}

	entry.mu.Lock()
	entry.abTestConfig = config
	entry.mu.Unlock()
	return nil
}

func (r *modelRegistry) ResolveModel(ctx context.Context, modelID string, requestID string) (*RegisteredModel, error) {
	value, ok := r.models.Load(modelID)
	if !ok {
		return nil, ErrModelNotFound
	}
	entry := value.(*modelEntry)

	entry.mu.RLock()
	defer entry.mu.RUnlock()

	version := entry.activeVersion

	// Check AB test
	if entry.abTestConfig != nil && entry.abTestConfig.Enabled {
		now := time.Now().UTC()
		if now.After(entry.abTestConfig.StartTime) && now.Before(entry.abTestConfig.EndTime) {
			// Deterministic routing based on requestID
			hash := 0
			for _, c := range requestID {
				hash += int(c)
			}
			target := hash % 100

			current := 0
			for _, v := range entry.abTestConfig.Variants {
				current += v.TrafficWeight
				if target < current {
					version = v.Version
					break
				}
			}
		}
	}

	if version == "" {
		return nil, ErrNoActiveVersion
	}

	ver := entry.versions[version]
	return &RegisteredModel{
		ModelID:       modelID,
		ActiveVersion: version,
		Metadata:      ver.Metadata,
		Status:        ModelStatusActive,
	}, nil
}

func (r *modelRegistry) HealthCheck(ctx context.Context) (*RegistryHealth, error) {
	// Simplified implementation
	return &RegistryHealth{}, nil
}

//Personal.AI order the ending

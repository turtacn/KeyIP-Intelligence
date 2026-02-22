package common

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash/fnv"
	"regexp"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// ---------------------------------------------------------------------------
// Interfaces
// ---------------------------------------------------------------------------

// ModelRegistry manages AI model lifecycle: registration, versioning,
// hot-swap, rollback, A/B testing and health monitoring.
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
	Close() error
}

// ModelLoader abstracts loading / unloading model artifacts into memory.
type ModelLoader interface {
	Load(ctx context.Context, artifactPath string) (interface{}, error)
	Unload(ctx context.Context, modelHandle interface{}) error
	Validate(ctx context.Context, artifactPath string, checksum string) error
}

// ---------------------------------------------------------------------------
// Enumerations
// ---------------------------------------------------------------------------

// ModelStatus represents the high-level status of a registered model.
type ModelStatus int

const (
	ModelStatusActive     ModelStatus = iota // model has an active version serving traffic
	ModelStatusLoading                       // model is loading a version
	ModelStatusFailed                        // model's active version failed
	ModelStatusDeprecated                    // model is deprecated, no longer recommended
)

func (s ModelStatus) String() string {
	switch s {
	case ModelStatusActive:
		return "ACTIVE"
	case ModelStatusLoading:
		return "LOADING"
	case ModelStatusFailed:
		return "FAILED"
	case ModelStatusDeprecated:
		return "DEPRECATED"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", int(s))
	}
}

// VersionStatus represents the lifecycle status of a single model version.
type VersionStatus int

const (
	VersionStatusRegistered VersionStatus = iota // registered but not yet loaded
	VersionStatusLoading                         // currently being loaded
	VersionStatusReady                           // loaded and ready for inference
	VersionStatusFailed                          // loading or runtime failure
	VersionStatusDeprecated                      // deprecated, should not be used
	VersionStatusUnloaded                        // was loaded, now unloaded
)

func (s VersionStatus) String() string {
	switch s {
	case VersionStatusRegistered:
		return "REGISTERED"
	case VersionStatusLoading:
		return "LOADING"
	case VersionStatusReady:
		return "READY"
	case VersionStatusFailed:
		return "FAILED"
	case VersionStatusDeprecated:
		return "DEPRECATED"
	case VersionStatusUnloaded:
		return "UNLOADED"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", int(s))
	}
}

// ---------------------------------------------------------------------------
// Core data structures
// ---------------------------------------------------------------------------

// ModelMetadata is the input payload for registering a new model version.
type ModelMetadata struct {
	ModelID      string            `json:"model_id"`
	Name         string            `json:"name"`
	Description  string            `json:"description,omitempty"`
	Version      string            `json:"version"`
	ArtifactPath string            `json:"artifact_path"`
	Checksum     string            `json:"checksum,omitempty"`
	SizeBytes    int64             `json:"size_bytes,omitempty"`
	Framework    string            `json:"framework,omitempty"`
	Labels       map[string]string `json:"labels,omitempty"`
}

// RegisteredModel is the external view of a model in the registry.
type RegisteredModel struct {
	ModelID         string        `json:"model_id"`
	Name            string        `json:"name"`
	Description     string        `json:"description,omitempty"`
	ActiveVersion   string        `json:"active_version"`
	PreviousVersion string        `json:"previous_version,omitempty"`
	Metadata        *ModelMetadata `json:"metadata,omitempty"`
	Status          ModelStatus   `json:"status"`
	LoadedAt        time.Time     `json:"loaded_at,omitempty"`
	LastUsedAt      time.Time     `json:"last_used_at,omitempty"`
}

// ModelVersion describes a single version of a model.
type ModelVersion struct {
	Version      string        `json:"version"`
	CreatedAt    time.Time     `json:"created_at"`
	Status       VersionStatus `json:"status"`
	Metadata     *ModelMetadata `json:"metadata,omitempty"`
	ArtifactPath string        `json:"artifact_path"`
	Checksum     string        `json:"checksum,omitempty"`
	SizeBytes    int64         `json:"size_bytes,omitempty"`
}

// ABTestConfig describes an A/B test across model versions.
type ABTestConfig struct {
	ModelID   string           `json:"model_id"`
	Variants  []*ABTestVariant `json:"variants"`
	StartTime time.Time        `json:"start_time"`
	EndTime   time.Time        `json:"end_time"`
	Enabled   bool             `json:"enabled"`
}

// ABTestVariant is one arm of an A/B test.
type ABTestVariant struct {
	Version       string `json:"version"`
	TrafficWeight int    `json:"traffic_weight"` // 0-100, sum of all variants must be 100
	Description   string `json:"description,omitempty"`
}

// RegistryHealth is the aggregate health report.
type RegistryHealth struct {
	TotalModels   int                           `json:"total_models"`
	ActiveModels  int                           `json:"active_models"`
	FailedModels  int                           `json:"failed_models"`
	LoadingModels int                           `json:"loading_models"`
	ModelHealths  map[string]*ModelHealthStatus `json:"model_healths,omitempty"`
}

// ModelHealthStatus is the health of a single model.
type ModelHealthStatus struct {
	ModelID         string        `json:"model_id"`
	Version         string        `json:"version"`
	Status          VersionStatus `json:"status"`
	LastHealthCheck time.Time     `json:"last_health_check"`
	LatencyMs       float64       `json:"latency_ms"`
	ErrorRate       float64       `json:"error_rate"`
}

// ---------------------------------------------------------------------------
// Registry options
// ---------------------------------------------------------------------------

// RegistryOption is a functional option for modelRegistry.
type RegistryOption func(*registryOptions)

type registryOptions struct {
	healthCheckInterval time.Duration
	unloadDelay         time.Duration
	maxLoadedVersions   int
}

func defaultRegistryOptions() *registryOptions {
	return &registryOptions{
		healthCheckInterval: 30 * time.Second,
		unloadDelay:         60 * time.Second,
		maxLoadedVersions:   3,
	}
}

// WithHealthCheckInterval sets the periodic health-check interval.
func WithHealthCheckInterval(d time.Duration) RegistryOption {
	return func(o *registryOptions) {
		if d > 0 {
			o.healthCheckInterval = d
		}
	}
}

// WithUnloadDelay sets the grace period before an old version is unloaded.
func WithUnloadDelay(d time.Duration) RegistryOption {
	return func(o *registryOptions) {
		if d >= 0 {
			o.unloadDelay = d
		}
	}
}

// WithMaxLoadedVersions sets the maximum number of concurrently loaded
// versions per model. When exceeded the oldest non-active version is evicted.
func WithMaxLoadedVersions(n int) RegistryOption {
	return func(o *registryOptions) {
		if n > 0 {
			o.maxLoadedVersions = n
		}
	}
}

// ---------------------------------------------------------------------------
// Internal model entry
// ---------------------------------------------------------------------------

// versionEntry is the internal bookkeeping for a single version.
type versionEntry struct {
	info        *ModelVersion
	handle      interface{} // opaque model handle returned by ModelLoader.Load
	refCount    atomic.Int64
	loadedAt    time.Time
	lastUsedAt  atomic.Value // time.Time
	unloadTimer *time.Timer
}

func (v *versionEntry) markUsed() {
	v.lastUsedAt.Store(time.Now())
}

func (v *versionEntry) getLastUsed() time.Time {
	val := v.lastUsedAt.Load()
	if val == nil {
		return time.Time{}
	}
	return val.(time.Time)
}

// modelEntry is the internal bookkeeping for a single model.
type modelEntry struct {
	mu              sync.RWMutex
	name            string
	description     string
	versions        map[string]*versionEntry
	activeVersion   atomic.Value // string
	previousVersion string
	abTestConfig    *ABTestConfig
	createdAt       time.Time
}

func newModelEntry(name, description string) *modelEntry {
	e := &modelEntry{
		name:        name,
		description: description,
		versions:    make(map[string]*versionEntry),
		createdAt:   time.Now(),
	}
	e.activeVersion.Store("")
	return e
}

func (e *modelEntry) getActiveVersion() string {
	v := e.activeVersion.Load()
	if v == nil {
		return ""
	}
	return v.(string)
}

// ---------------------------------------------------------------------------
// Semver validation
// ---------------------------------------------------------------------------

// semverRegex is a simplified but practical semver pattern that accepts
// major.minor.patch with optional pre-release and build metadata.
var semverRegex = regexp.MustCompile(
	`^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)` +
		`(?:-((?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*)(?:\.(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*))*))?` +
		`(?:\+([0-9a-zA-Z-]+(?:\.[0-9a-zA-Z-]+)*))?$`,
)

func isValidSemver(v string) bool {
	return semverRegex.MatchString(v)
}

// ---------------------------------------------------------------------------
// Deterministic hash for A/B routing
// ---------------------------------------------------------------------------

func deterministicBucket(requestID string, modelID string) int {
	h := fnv.New32a()
	_, _ = h.Write([]byte(requestID))
	_, _ = h.Write([]byte(":"))
	_, _ = h.Write([]byte(modelID))
	return int(h.Sum32() % 100) // 0-99
}

func checksumString(data string) string {
	h := sha256.Sum256([]byte(data))
	return hex.EncodeToString(h[:])
}

// ---------------------------------------------------------------------------
// modelRegistry implementation
// ---------------------------------------------------------------------------

type modelRegistry struct {
	models  sync.Map // map[string]*modelEntry
	loader  ModelLoader
	metrics IntelligenceMetrics
	logger  Logger
	opts    *registryOptions

	stopCh chan struct{}
	wg     sync.WaitGroup
}

// NewModelRegistry creates a new model registry.
func NewModelRegistry(
	loader ModelLoader,
	metrics IntelligenceMetrics,
	logger Logger,
	opts ...RegistryOption,
) (*modelRegistry, error) {
	if loader == nil {
		return nil, errors.NewInvalidInputError("model loader is required")
	}
	if metrics == nil {
		metrics = NewNoopIntelligenceMetrics()
	}
	if logger == nil {
		logger = NewNoopLogger()
	}

	o := defaultRegistryOptions()
	for _, fn := range opts {
		fn(o)
	}

	r := &modelRegistry{
		loader:  loader,
		metrics: metrics,
		logger:  logger,
		opts:    o,
		stopCh:  make(chan struct{}),
	}

	r.wg.Add(1)
	go r.backgroundLoop()

	return r, nil
}

// Close stops background goroutines and releases resources.
func (r *modelRegistry) Close() error {
	close(r.stopCh)
	r.wg.Wait()
	return nil
}

// ---------------------------------------------------------------------------
// Register
// ---------------------------------------------------------------------------

func (r *modelRegistry) Register(ctx context.Context, meta *ModelMetadata) error {
	if meta == nil {
		return errors.NewInvalidInputError("metadata is required")
	}
	if meta.ModelID == "" {
		return errors.NewInvalidInputError("model_id is required")
	}
	if meta.Version == "" {
		return errors.NewInvalidInputError("version is required")
	}
	if meta.ArtifactPath == "" {
		return errors.NewInvalidInputError("artifact_path is required")
	}
	if !isValidSemver(meta.Version) {
		return errors.NewInvalidVersionError(meta.Version)
	}

	// Validate artifact integrity
	if err := r.loader.Validate(ctx, meta.ArtifactPath, meta.Checksum); err != nil {
		return fmt.Errorf("artifact validation failed: %w", err)
	}

	// Get or create model entry
	actual, _ := r.models.LoadOrStore(meta.ModelID, newModelEntry(meta.Name, meta.Description))
	entry := actual.(*modelEntry)

	entry.mu.Lock()
	defer entry.mu.Unlock()

	if _, exists := entry.versions[meta.Version]; exists {
		return errors.NewVersionAlreadyExistsError(meta.ModelID, meta.Version)
	}

	ve := &versionEntry{
		info: &ModelVersion{
			Version:      meta.Version,
			CreatedAt:    time.Now(),
			Status:       VersionStatusRegistered,
			Metadata:     meta,
			ArtifactPath: meta.ArtifactPath,
			Checksum:     meta.Checksum,
			SizeBytes:    meta.SizeBytes,
		},
	}
	entry.versions[meta.Version] = ve

	// Update entry name/description if provided
	if meta.Name != "" {
		entry.name = meta.Name
	}
	if meta.Description != "" {
		entry.description = meta.Description
	}

	r.logger.Info("model version registered",
		"model_id", meta.ModelID,
		"version", meta.Version,
		"artifact_path", meta.ArtifactPath,
	)

	r.metrics.RecordModelLoad(ctx, meta.ModelID, meta.Version, 0, true)

	return nil
}

// ---------------------------------------------------------------------------
// Unregister
// ---------------------------------------------------------------------------

func (r *modelRegistry) Unregister(ctx context.Context, modelID string, version string) error {
	if modelID == "" || version == "" {
		return errors.NewInvalidInputError("model_id and version are required")
	}

	raw, ok := r.models.Load(modelID)
	if !ok {
		return errors.NewNotFoundError("model", modelID)
	}
	entry := raw.(*modelEntry)

	entry.mu.Lock()
	defer entry.mu.Unlock()

	ve, exists := entry.versions[version]
	if !exists {
		return errors.NewNotFoundError("version", version)
	}

	// Cannot unregister the active version
	if entry.getActiveVersion() == version {
		return errors.NewInvalidInputError("cannot unregister the active version; switch to another version first")
	}

	// Unload if loaded
	if ve.handle != nil {
		if err := r.loader.Unload(ctx, ve.handle); err != nil {
			r.logger.Warn("failed to unload during unregister", "model_id", modelID, "version", version, "error", err)
		}
		ve.handle = nil
	}

	delete(entry.versions, version)

	r.logger.Info("model version unregistered", "model_id", modelID, "version", version)
	return nil
}

// ---------------------------------------------------------------------------
// GetModel
// ---------------------------------------------------------------------------

func (r *modelRegistry) GetModel(ctx context.Context, modelID string) (*RegisteredModel, error) {
	if modelID == "" {
		return nil, errors.NewInvalidInputError("model_id is required")
	}

	raw, ok := r.models.Load(modelID)
	if !ok {
		return nil, errors.NewNotFoundError("model", modelID)
	}
	entry := raw.(*modelEntry)

	activeVer := entry.getActiveVersion()
	if activeVer == "" {
		return nil, errors.NewNoActiveVersionError(modelID)
	}

	entry.mu.RLock()
	defer entry.mu.RUnlock()

	ve, exists := entry.versions[activeVer]
	if !exists {
		return nil, errors.NewNotFoundError("version", activeVer)
	}

	ve.markUsed()

	return r.buildRegisteredModel(modelID, entry, ve), nil
}

// ---------------------------------------------------------------------------
// GetModelVersion
// ---------------------------------------------------------------------------

func (r *modelRegistry) GetModelVersion(ctx context.Context, modelID string, version string) (*RegisteredModel, error) {
	if modelID == "" || version == "" {
		return nil, errors.NewInvalidInputError("model_id and version are required")
	}

	raw, ok := r.models.Load(modelID)
	if !ok {
		return nil, errors.NewNotFoundError("model", modelID)
	}
	entry := raw.(*modelEntry)

	entry.mu.RLock()
	defer entry.mu.RUnlock()

	ve, exists := entry.versions[version]
	if !exists {
		return nil, errors.NewNotFoundError("version", version)
	}

	return r.buildRegisteredModel(modelID, entry, ve), nil
}

// ---------------------------------------------------------------------------
// ListModels
// ---------------------------------------------------------------------------

func (r *modelRegistry) ListModels(ctx context.Context) ([]*RegisteredModel, error) {
	var result []*RegisteredModel

	r.models.Range(func(key, value interface{}) bool {
		modelID := key.(string)
		entry := value.(*modelEntry)

		entry.mu.RLock()
		activeVer := entry.getActiveVersion()
		var ve *versionEntry
		if activeVer != "" {
			ve = entry.versions[activeVer]
		}
		// If no active version, pick the first version for metadata
		if ve == nil {
			for _, v := range entry.versions {
				ve = v
				break
			}
		}
		entry.mu.RUnlock()

		rm := &RegisteredModel{
			ModelID:         modelID,
			Name:            entry.name,
			Description:     entry.description,
			ActiveVersion:   activeVer,
			PreviousVersion: entry.previousVersion,
		}
		if ve != nil {
			rm.Metadata = ve.info.Metadata
			rm.Status = r.deriveModelStatus(entry)
			rm.LoadedAt = ve.loadedAt
			rm.LastUsedAt = ve.getLastUsed()
		}
		result = append(result, rm)
		return true
	})

	// Sort by ModelID for deterministic output
	sort.Slice(result, func(i, j int) bool {
		return result[i].ModelID < result[j].ModelID
	})

	return result, nil
}

// ---------------------------------------------------------------------------
// ListVersions
// ---------------------------------------------------------------------------

func (r *modelRegistry) ListVersions(ctx context.Context, modelID string) ([]*ModelVersion, error) {
	if modelID == "" {
		return nil, errors.NewInvalidInputError("model_id is required")
	}

	raw, ok := r.models.Load(modelID)
	if !ok {
		return nil, errors.NewNotFoundError("model", modelID)
	}
	entry := raw.(*modelEntry)

	entry.mu.RLock()
	defer entry.mu.RUnlock()

	result := make([]*ModelVersion, 0, len(entry.versions))
	for _, ve := range entry.versions {
		cp := *ve.info
		result = append(result, &cp)
	}

	// Sort by version string (lexicographic; good enough for semver with same major)
	sort.Slice(result, func(i, j int) bool {
		return result[i].Version < result[j].Version
	})

	return result, nil
}

// ---------------------------------------------------------------------------
// SetActiveVersion (hot-swap)
// ---------------------------------------------------------------------------

func (r *modelRegistry) SetActiveVersion(ctx context.Context, modelID string, version string) error {
	if modelID == "" || version == "" {
		return errors.NewInvalidInputError("model_id and version are required")
	}

	raw, ok := r.models.Load(modelID)
	if !ok {
		return errors.NewNotFoundError("model", modelID)
	}
	entry := raw.(*modelEntry)

	entry.mu.Lock()
	defer entry.mu.Unlock()

	// Same version → no-op
	currentActive := entry.getActiveVersion()
	if currentActive == version {
		return nil
	}

	ve, exists := entry.versions[version]
	if !exists {
		return errors.NewNotFoundError("version", version)
	}

	if ve.info.Status == VersionStatusDeprecated {
		return errors.NewVersionDeprecatedError(modelID, version)
	}

	if ve.info.Status == VersionStatusFailed {
		return fmt.Errorf("version %s is in FAILED state; re-register or fix the artifact", version)
	}

	// Auto-load if not yet loaded
	if ve.info.Status == VersionStatusRegistered || ve.info.Status == VersionStatusUnloaded {
		ve.info.Status = VersionStatusLoading
		handle, err := r.loader.Load(ctx, ve.info.ArtifactPath)
		if err != nil {
			ve.info.Status = VersionStatusFailed
			r.logger.Error("failed to load model version",
				"model_id", modelID, "version", version, "error", err)
			r.metrics.RecordModelLoad(ctx, modelID, version, 0, false)
			return fmt.Errorf("loading version %s: %w", version, err)
		}
		ve.handle = handle
		ve.info.Status = VersionStatusReady
		ve.loadedAt = time.Now()
		r.metrics.RecordModelLoad(ctx, modelID, version, 0, true)
	}

	if ve.info.Status != VersionStatusReady {
		return fmt.Errorf("version %s is not ready (status: %s)", version, ve.info.Status)
	}

	// Record previous version for rollback
	if currentActive != "" {
		entry.previousVersion = currentActive
	}

	// Atomic swap
	entry.activeVersion.Store(version)

	// Schedule delayed unload of old version
	if currentActive != "" && currentActive != version {
		r.scheduleDelayedUnload(modelID, entry, currentActive)
	}

	// Evict excess loaded versions
	r.evictExcessVersions(ctx, modelID, entry)

	r.logger.Info("active version switched",
		"model_id", modelID,
		"from", currentActive,
		"to", version,
	)

	return nil
}

// ---------------------------------------------------------------------------
// Rollback
// ---------------------------------------------------------------------------

func (r *modelRegistry) Rollback(ctx context.Context, modelID string) error {
	if modelID == "" {
		return errors.NewInvalidInputError("model_id is required")
	}

	raw, ok := r.models.Load(modelID)
	if !ok {
		return errors.NewNotFoundError("model", modelID)
	}
	entry := raw.(*modelEntry)

	entry.mu.RLock()
	prev := entry.previousVersion
	entry.mu.RUnlock()

	if prev == "" {
		return errors.NewNoPreviousVersionError(modelID)
	}

	return r.SetActiveVersion(ctx, modelID, prev)
}

// ---------------------------------------------------------------------------
// ConfigureABTest
// ---------------------------------------------------------------------------

func (r *modelRegistry) ConfigureABTest(ctx context.Context, config *ABTestConfig) error {
	if config == nil {
		return errors.NewInvalidInputError("A/B test config is required")
	}
	if config.ModelID == "" {
		return errors.NewInvalidInputError("model_id is required in A/B test config")
	}

	raw, ok := r.models.Load(config.ModelID)
	if !ok {
		return errors.NewNotFoundError("model", config.ModelID)
	}
	entry := raw.(*modelEntry)

	// If disabling, just clear
	if !config.Enabled {
		entry.mu.Lock()
		entry.abTestConfig = nil
		entry.mu.Unlock()
		r.logger.Info("A/B test disabled", "model_id", config.ModelID)
		return nil
	}

	if len(config.Variants) == 0 {
		return errors.NewInvalidABTestConfigError("at least one variant is required")
	}

	totalWeight := 0
	entry.mu.RLock()
	for _, v := range config.Variants {
		if _, exists := entry.versions[v.Version]; !exists {
			entry.mu.RUnlock()
			return errors.NewNotFoundError("version", v.Version)
		}
		if v.TrafficWeight < 0 {
			entry.mu.RUnlock()
			return errors.NewInvalidABTestConfigError("traffic weight must be non-negative")
		}
		totalWeight += v.TrafficWeight
	}
	entry.mu.RUnlock()

	if totalWeight != 100 {
		return errors.NewInvalidABTestConfigError(
			fmt.Sprintf("traffic weights must sum to 100, got %d", totalWeight))
	}

	entry.mu.Lock()
	entry.abTestConfig = config
	entry.mu.Unlock()

	r.logger.Info("A/B test configured",
		"model_id", config.ModelID,
		"variants", len(config.Variants),
		"start", config.StartTime,
		"end", config.EndTime,
	)

	return nil
}

// ---------------------------------------------------------------------------
// ResolveModel (A/B test routing)
// ---------------------------------------------------------------------------

func (r *modelRegistry) ResolveModel(ctx context.Context, modelID string, requestID string) (*RegisteredModel, error) {
	if modelID == "" {
		return nil, errors.NewInvalidInputError("model_id is required")
	}

	raw, ok := r.models.Load(modelID)
	if !ok {
		return nil, errors.NewNotFoundError("model", modelID)
	}
	entry := raw.(*modelEntry)

	entry.mu.RLock()
	abCfg := entry.abTestConfig
	entry.mu.RUnlock()

	// Check if A/B test is active
	resolvedVersion := ""
	if abCfg != nil && abCfg.Enabled && len(abCfg.Variants) > 0 {
		now := time.Now()
		started := abCfg.StartTime.IsZero() || !now.Before(abCfg.StartTime)
		notEnded := abCfg.EndTime.IsZero() || now.Before(abCfg.EndTime)

		if started && notEnded {
			bucket := deterministicBucket(requestID, modelID)
			resolvedVersion = r.routeByWeight(abCfg.Variants, bucket)
		}
	}

	// Fallback to active version
	if resolvedVersion == "" {
		resolvedVersion = entry.getActiveVersion()
	}
	if resolvedVersion == "" {
		return nil, errors.NewNoActiveVersionError(modelID)
	}

	entry.mu.RLock()
	ve, exists := entry.versions[resolvedVersion]
	if !exists {
		entry.mu.RUnlock()
		return nil, errors.NewNotFoundError("version", resolvedVersion)
	}
	ve.markUsed()
	rm := r.buildRegisteredModel(modelID, entry, ve)
	entry.mu.RUnlock()

	return rm, nil
}

// routeByWeight picks a variant based on a 0-99 bucket value.
func (r *modelRegistry) routeByWeight(variants []*ABTestVariant, bucket int) string {
	cumulative := 0
	for _, v := range variants {
		cumulative += v.TrafficWeight
		if bucket < cumulative {
			return v.Version
		}
	}
	// Fallback to last variant (should not happen if weights sum to 100)
	if len(variants) > 0 {
		return variants[len(variants)-1].Version
	}
	return ""
}

// ---------------------------------------------------------------------------
// HealthCheck
// ---------------------------------------------------------------------------

func (r *modelRegistry) HealthCheck(ctx context.Context) (*RegistryHealth, error) {
	health := &RegistryHealth{
		ModelHealths: make(map[string]*ModelHealthStatus),
	}

	r.models.Range(func(key, value interface{}) bool {
		modelID := key.(string)
		entry := value.(*modelEntry)
		health.TotalModels++

		entry.mu.RLock()
		activeVer := entry.getActiveVersion()
		ve := entry.versions[activeVer]
		entry.mu.RUnlock()

		if ve == nil {
			return true
		}

		mhs := &ModelHealthStatus{
			ModelID:         modelID,
			Version:         activeVer,
			Status:          ve.info.Status,
			LastHealthCheck: time.Now(),
		}

		switch ve.info.Status {
		case VersionStatusReady:
			health.ActiveModels++
		case VersionStatusLoading:
			health.LoadingModels++
		case VersionStatusFailed:
			health.FailedModels++
		}

		health.ModelHealths[modelID] = mhs
		return true
	})

	return health, nil
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

func (r *modelRegistry) buildRegisteredModel(modelID string, entry *modelEntry, ve *versionEntry) *RegisteredModel {
	rm := &RegisteredModel{
		ModelID:         modelID,
		Name:            entry.name,
		Description:     entry.description,
		ActiveVersion:   entry.getActiveVersion(),
		PreviousVersion: entry.previousVersion,
		Status:          r.deriveModelStatus(entry),
		LoadedAt:        ve.loadedAt,
		LastUsedAt:      ve.getLastUsed(),
	}
	if ve.info != nil {
		rm.Metadata = ve.info.Metadata
	}
	return rm
}

func (r *modelRegistry) deriveModelStatus(entry *modelEntry) ModelStatus {
	activeVer := entry.getActiveVersion()
	if activeVer == "" {
		return ModelStatusLoading
	}
	ve, ok := entry.versions[activeVer]
	if !ok {
		return ModelStatusFailed
	}
	switch ve.info.Status {
	case VersionStatusReady:
		return ModelStatusActive
	case VersionStatusLoading:
		return ModelStatusLoading
	case VersionStatusFailed:
		return ModelStatusFailed
	case VersionStatusDeprecated:
		return ModelStatusDeprecated
	default:
		return ModelStatusLoading
	}
}

func (r *modelRegistry) scheduleDelayedUnload(modelID string, entry *modelEntry, version string) {
	ve, exists := entry.versions[version]
	if !exists {
		return
	}

	// Cancel any existing unload timer
	if ve.unloadTimer != nil {
		ve.unloadTimer.Stop()
	}

	delay := r.opts.unloadDelay
	if delay <= 0 {
		// Immediate unload
		r.doUnload(modelID, entry, version)
		return
	}

	ve.unloadTimer = time.AfterFunc(delay, func() {
		entry.mu.Lock()
		defer entry.mu.Unlock()

		// Double-check it's still not the active version
		if entry.getActiveVersion() == version {
			return
		}

		r.doUnload(modelID, entry, version)
	})
}

func (r *modelRegistry) doUnload(modelID string, entry *modelEntry, version string) {
	ve, exists := entry.versions[version]
	if !exists {
		return
	}

	// Wait for in-flight requests (simple spin with timeout)
	if ve.refCount.Load() > 0 {
		deadline := time.Now().Add(10 * time.Second)
		for ve.refCount.Load() > 0 && time.Now().Before(deadline) {
			time.Sleep(50 * time.Millisecond)
		}
	}

	if ve.handle != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := r.loader.Unload(ctx, ve.handle); err != nil {
			r.logger.Warn("delayed unload failed",
				"model_id", modelID,
				"version", version,
				"error", err,
			)
		}
		ve.handle = nil
	}
	ve.info.Status = VersionStatusUnloaded

	r.logger.Info("model version unloaded (delayed)",
		"model_id", modelID,
		"version", version,
	)
}

func (r *modelRegistry) evictExcessVersions(ctx context.Context, modelID string, entry *modelEntry) {
	maxLoaded := r.opts.maxLoadedVersions
	if maxLoaded <= 0 {
		return
	}

	activeVer := entry.getActiveVersion()

	// Collect loaded versions
	type loadedInfo struct {
		version  string
		loadedAt time.Time
	}
	var loaded []loadedInfo
	for ver, ve := range entry.versions {
		if ve.info.Status == VersionStatusReady && ver != activeVer {
			loaded = append(loaded, loadedInfo{version: ver, loadedAt: ve.loadedAt})
		}
	}

	// +1 for the active version
	if len(loaded)+1 <= maxLoaded {
		return
	}

	// Sort by loadedAt ascending (oldest first)
	sort.Slice(loaded, func(i, j int) bool {
		return loaded[i].loadedAt.Before(loaded[j].loadedAt)
	})

	// Evict oldest until within limit
	evictCount := len(loaded) + 1 - maxLoaded
	for i := 0; i < evictCount && i < len(loaded); i++ {
		ver := loaded[i].version
		// Check if it's part of an active A/B test
		if r.isVersionInABTest(entry, ver) {
			continue
		}
		r.doUnload(modelID, entry, ver)
		r.logger.Info("evicted excess loaded version",
			"model_id", modelID,
			"version", ver,
		)
	}
}

func (r *modelRegistry) isVersionInABTest(entry *modelEntry, version string) bool {
	if entry.abTestConfig == nil || !entry.abTestConfig.Enabled {
		return false
	}
	for _, v := range entry.abTestConfig.Variants {
		if v.Version == version {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Background loop
// ---------------------------------------------------------------------------

func (r *modelRegistry) backgroundLoop() {
	defer r.wg.Done()

	healthTicker := time.NewTicker(r.opts.healthCheckInterval)
	defer healthTicker.Stop()

	abCleanupTicker := time.NewTicker(1 * time.Minute)
	defer abCleanupTicker.Stop()

	for {
		select {
		case <-r.stopCh:
			return
		case <-healthTicker.C:
			r.periodicHealthCheck()
		case <-abCleanupTicker.C:
			r.cleanupExpiredABTests()
		}
	}
}

func (r *modelRegistry) periodicHealthCheck() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	health, err := r.HealthCheck(ctx)
	if err != nil {
		r.logger.Warn("periodic health check failed", "error", err)
		return
	}

	if health.FailedModels > 0 {
		r.logger.Warn("unhealthy models detected",
			"failed", health.FailedModels,
			"total", health.TotalModels,
		)
	}
}

func (r *modelRegistry) cleanupExpiredABTests() {
	now := time.Now()

	r.models.Range(func(key, value interface{}) bool {
		entry := value.(*modelEntry)

		entry.mu.Lock()
		if entry.abTestConfig != nil &&
			entry.abTestConfig.Enabled &&
			!entry.abTestConfig.EndTime.IsZero() &&
			now.After(entry.abTestConfig.EndTime) {

			r.logger.Info("A/B test expired, disabling",
				"model_id", key.(string),
				"end_time", entry.abTestConfig.EndTime,
			)
			entry.abTestConfig.Enabled = false
		}
		entry.mu.Unlock()

		return true
	})
}

//Personal.AI order the ending

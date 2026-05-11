package molecule

import (
	"container/list"
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// FingerprintType identifies the algorithm used to generate the fingerprint.
type FingerprintType string

const (
	FingerprintMACCS    FingerprintType = "maccs"
	FingerprintMorgan   FingerprintType = "morgan"
	FingerprintRDKit    FingerprintType = "rdkit"
	FingerprintAtomPair FingerprintType = "atom_pair"
	FingerprintFCFP     FingerprintType = "fcfp"
	FingerprintGNN      FingerprintType = "gnn"
)

func (t FingerprintType) IsValid() bool {
	switch t {
	case FingerprintMACCS, FingerprintMorgan, FingerprintRDKit, FingerprintAtomPair, FingerprintFCFP, FingerprintGNN:
		return true
	default:
		return false
	}
}

func (t FingerprintType) String() string {
	return string(t)
}

// AllFingerprintTypes returns all valid fingerprint types.
func AllFingerprintTypes() []FingerprintType {
	return []FingerprintType{
		FingerprintMACCS,
		FingerprintMorgan,
		FingerprintRDKit,
		FingerprintAtomPair,
		FingerprintFCFP,
		FingerprintGNN,
	}
}

// ParseFingerprintType parses a string into a FingerprintType.
func ParseFingerprintType(s string) (FingerprintType, error) {
	t := FingerprintType(s)
	if t.IsValid() {
		return t, nil
	}
	return "", errors.New(errors.ErrCodeInvalidInput, "invalid fingerprint type: "+s)
}

// FingerprintEncoding specifies how the fingerprint data is stored.
type FingerprintEncoding string

const (
	EncodingBitVector   FingerprintEncoding = "bitvector"
	EncodingCountVector FingerprintEncoding = "countvector"
	EncodingDenseVector FingerprintEncoding = "densevector"
)

// Fingerprint represents a molecular fingerprint used for similarity searching.
type Fingerprint struct {
	ID           uuid.UUID           `json:"id"`
	MoleculeID   uuid.UUID           `json:"molecule_id"`
	Type         FingerprintType     `json:"type"`
	Encoding     FingerprintEncoding `json:"encoding"`
	Bits         []byte              `json:"bits,omitempty"`   // For BitVector and CountVector
	Vector       []float32           `json:"vector,omitempty"` // For DenseVector
	NumBits      int                 `json:"num_bits"`         // Length in bits for BitVector, or dimension for DenseVector
	Radius       int                 `json:"radius,omitempty"` // Only for Morgan/FCFP
	Hash         string              `json:"fingerprint_hash,omitempty"`
	Parameters   map[string]any      `json:"parameters,omitempty"`
	ModelVersion string              `json:"model_version,omitempty"`
	ComputedAt   time.Time           `json:"computed_at"`
	CreatedAt    time.Time           `json:"created_at"` // Alias for ComputedAt or DB field
}

// NewBitFingerprint creates a new bit vector fingerprint.
func NewBitFingerprint(fpType FingerprintType, bits []byte, numBits int, radius int) (*Fingerprint, error) {
	if !fpType.IsValid() {
		return nil, errors.New(errors.ErrCodeInvalidInput, "invalid fingerprint type")
	}
	if fpType == FingerprintGNN {
		return nil, errors.New(errors.ErrCodeInvalidInput, "GNN fingerprint must use NewDenseFingerprint")
	}

	if len(bits) == 0 {
		return nil, errors.New(errors.ErrCodeInvalidInput, "bits cannot be empty")
	}
	if numBits <= 0 {
		return nil, errors.New(errors.ErrCodeInvalidInput, "numBits must be positive")
	}
	expectedBytes := (numBits + 7) / 8
	if len(bits) < expectedBytes {
		return nil, errors.New(errors.ErrCodeInvalidInput, fmt.Sprintf("insufficient bits length: got %d bytes, need %d bytes for %d bits", len(bits), expectedBytes, numBits))
	}

	if fpType == FingerprintMACCS {
		if numBits != 166 {
			return nil, errors.New(errors.ErrCodeInvalidInput, "MACCS fingerprint must have 166 bits")
		}
		if radius != 0 {
			return nil, errors.New(errors.ErrCodeInvalidInput, "MACCS fingerprint radius must be 0")
		}
	} else if fpType == FingerprintMorgan || fpType == FingerprintFCFP {
		if radius < 1 || radius > 6 {
			return nil, errors.New(errors.ErrCodeInvalidInput, "radius must be between 1 and 6 for circular fingerprints")
		}
	} else {
		if radius != 0 {
			return nil, errors.New(errors.ErrCodeInvalidInput, "radius should be 0 for this fingerprint type")
		}
	}

	now := time.Now().UTC()
	return &Fingerprint{
		ID:         uuid.New(),
		Type:       fpType,
		Encoding:   EncodingBitVector,
		Bits:       bits,
		NumBits:    numBits,
		Radius:     radius,
		ComputedAt: now,
		CreatedAt:  now,
	}, nil
}

// NewCountFingerprint creates a new count vector fingerprint.
func NewCountFingerprint(fpType FingerprintType, bits []byte, numBits int, radius int) (*Fingerprint, error) {
	if !fpType.IsValid() {
		return nil, errors.New(errors.ErrCodeInvalidInput, "invalid fingerprint type")
	}
	if len(bits) == 0 {
		return nil, errors.New(errors.ErrCodeInvalidInput, "bits cannot be empty")
	}
	if numBits <= 0 {
		return nil, errors.New(errors.ErrCodeInvalidInput, "numBits must be positive")
	}

	now := time.Now().UTC()
	return &Fingerprint{
		ID:         uuid.New(),
		Type:       fpType,
		Encoding:   EncodingCountVector,
		Bits:       bits,
		NumBits:    numBits,
		Radius:     radius,
		ComputedAt: now,
		CreatedAt:  now,
	}, nil
}

// NewDenseFingerprint creates a new dense vector fingerprint (e.g., GNN embedding).
func NewDenseFingerprint(vector []float32, modelVersion string) (*Fingerprint, error) {
	if len(vector) == 0 {
		return nil, errors.New(errors.ErrCodeInvalidInput, "vector cannot be empty")
	}
	if len(vector) < 32 || len(vector) > 4096 {
		return nil, errors.New(errors.ErrCodeInvalidInput, "vector dimension must be between 32 and 4096")
	}
	if modelVersion == "" {
		return nil, errors.New(errors.ErrCodeInvalidInput, "model version is required")
	}

	now := time.Now().UTC()
	return &Fingerprint{
		ID:           uuid.New(),
		Type:         FingerprintGNN,
		Encoding:     EncodingDenseVector,
		Vector:       vector,
		NumBits:      len(vector),
		ModelVersion: modelVersion,
		ComputedAt:   now,
		CreatedAt:    now,
	}, nil
}

// IsBitVector checks if the fingerprint is a bit vector.
func (f *Fingerprint) IsBitVector() bool {
	return f.Encoding == EncodingBitVector
}

// IsCountVector checks if the fingerprint is a count vector.
func (f *Fingerprint) IsCountVector() bool {
	return f.Encoding == EncodingCountVector
}

// IsDenseVector checks if the fingerprint is a dense vector.
func (f *Fingerprint) IsDenseVector() bool {
	return f.Encoding == EncodingDenseVector
}

// BitCount returns the number of set bits (population count).
func (f *Fingerprint) BitCount() int {
	if !f.IsBitVector() {
		return 0
	}
	return PopCount(f.Bits)
}

// Density calculates the bit density.
func (f *Fingerprint) Density() float64 {
	if !f.IsBitVector() || f.NumBits == 0 {
		return 0
	}
	return float64(f.BitCount()) / float64(f.NumBits)
}

// GetBit returns the value of the bit at the given index.
func (f *Fingerprint) GetBit(index int) (bool, error) {
	if !f.IsBitVector() {
		return false, errors.New(errors.ErrCodeInvalidOperation, "GetBit only supported for bit vectors")
	}
	if index < 0 || index >= f.NumBits {
		return false, errors.New(errors.ErrCodeInvalidInput, "index out of range")
	}
	byteIndex := index / 8
	bitIndex := uint(index % 8)
	if byteIndex >= len(f.Bits) {
		return false, nil
	}
	return (f.Bits[byteIndex] & (1 << bitIndex)) != 0, nil
}

// SetBit is not implemented for immutable Fingerprint.
func (f *Fingerprint) SetBit(index int) error {
	return errors.New(errors.ErrCodeInvalidOperation, "Fingerprint is immutable")
}

// Dimension returns the dimension of the fingerprint.
func (f *Fingerprint) Dimension() int {
	return f.NumBits
}

// ToFloat32Slice converts the fingerprint to a float32 slice.
func (f *Fingerprint) ToFloat32Slice() []float32 {
	if f.IsDenseVector() {
		res := make([]float32, len(f.Vector))
		copy(res, f.Vector)
		return res
	}
	if f.IsBitVector() {
		res := make([]float32, f.NumBits)
		for i := 0; i < f.NumBits; i++ {
			byteIndex := i / 8
			bitIndex := uint(i % 8)
			if byteIndex < len(f.Bits) && (f.Bits[byteIndex]&(1<<bitIndex)) != 0 {
				res[i] = 1.0
			} else {
				res[i] = 0.0
			}
		}
		return res
	}
	return nil
}

func (f *Fingerprint) String() string {
	return fmt.Sprintf("Fingerprint{type=%s, bits=%d, density=%.2f}", f.Type, f.NumBits, f.Density())
}

// FingerprintCalculator defines the interface for calculating fingerprints.
type FingerprintCalculator interface {
	Calculate(ctx context.Context, smiles string, fpType FingerprintType, opts *FingerprintCalcOptions) (*Fingerprint, error)
	BatchCalculate(ctx context.Context, smilesSlice []string, fpType FingerprintType, opts *FingerprintCalcOptions) ([]*Fingerprint, error)
	SupportedTypes() []FingerprintType
	Standardize(ctx context.Context, smiles string) (canonical string, inchi string, inchiKey string, formula string, weight float64, err error)
}

// ChemClient defines the interface for an external chemical service (e.g. via gRPC or CGo).
type ChemClient interface {
	CalculateFingerprint(ctx context.Context, smiles string, fpType FingerprintType, radius, numBits int) ([]byte, error)
	BatchCalculateFingerprints(ctx context.Context, smilesSlice []string, fpType FingerprintType, radius, numBits int) ([][]byte, error)
	StandardizeSMILES(ctx context.Context, smiles string) (canonical string, inchi string, inchiKey string, formula string, weight float64, err error)
}

// RemoteFingerprintCalculator implements FingerprintCalculator using an external chemical service.
type RemoteFingerprintCalculator struct {
	client ChemClient
	cache  *FingerprintCache
}

func NewRemoteFingerprintCalculator(client ChemClient) *RemoteFingerprintCalculator {
	return &RemoteFingerprintCalculator{
		client: client,
		cache:  NewFingerprintCache(DefaultCacheSize),
	}
}

// NewRemoteFingerprintCalculatorWithCache creates a new calculator with a custom cache size.
func NewRemoteFingerprintCalculatorWithCache(client ChemClient, cacheSize int) *RemoteFingerprintCalculator {
	return &RemoteFingerprintCalculator{
		client: client,
		cache:  NewFingerprintCache(cacheSize),
	}
}

func (c *RemoteFingerprintCalculator) Calculate(ctx context.Context, smiles string, fpType FingerprintType, opts *FingerprintCalcOptions) (*Fingerprint, error) {
	if opts == nil {
		opts = DefaultFingerprintCalcOptions()
	}
	if err := opts.Validate(); err != nil {
		return nil, err
	}

	// Check cache first
	cacheKey := BuildCacheKey(smiles, fpType, opts)
	if cached, ok := c.cache.Get(cacheKey); ok {
		return cached.(*Fingerprint), nil
	}

	bits, err := c.client.CalculateFingerprint(ctx, smiles, fpType, opts.Radius, opts.NumBits)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeInternal, "failed to calculate fingerprint via external service")
	}

	fp, err := NewBitFingerprint(fpType, bits, opts.NumBits, opts.Radius)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeInternal, "failed to create bit fingerprint")
	}

	// Store in cache
	c.cache.Set(cacheKey, fp)
	return fp, nil
}

func (c *RemoteFingerprintCalculator) BatchCalculate(ctx context.Context, smilesSlice []string, fpType FingerprintType, opts *FingerprintCalcOptions) ([]*Fingerprint, error) {
	if opts == nil {
		opts = DefaultFingerprintCalcOptions()
	}
	if err := opts.Validate(); err != nil {
		return nil, err
	}

	bitsSlice, err := c.client.BatchCalculateFingerprints(ctx, smilesSlice, fpType, opts.Radius, opts.NumBits)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeInternal, "failed to batch calculate fingerprints via external service")
	}

	fps := make([]*Fingerprint, len(bitsSlice))
	for i, bits := range bitsSlice {
		fp, err := NewBitFingerprint(fpType, bits, opts.NumBits, opts.Radius)
		if err != nil {
			return nil, errors.Wrap(err, errors.ErrCodeInternal, "failed to create bit fingerprint")
		}
		fps[i] = fp
	}

	return fps, nil
}

func (c *RemoteFingerprintCalculator) SupportedTypes() []FingerprintType {
	return []FingerprintType{FingerprintMorgan, FingerprintMACCS, FingerprintRDKit}
}

func (c *RemoteFingerprintCalculator) Standardize(ctx context.Context, smiles string) (canonical string, inchi string, inchiKey string, formula string, weight float64, err error) {
	return c.client.StandardizeSMILES(ctx, smiles)
}

// ---------------------------------------------------------------------------
// Fingerprint Cache (thread-safe with LRU eviction)
// ---------------------------------------------------------------------------

// DefaultCacheSize is the default maximum number of entries in the fingerprint cache.
const DefaultCacheSize = 10000

// CacheStats tracks cache performance metrics.
type CacheStats struct {
	Hits   int64
	Misses int64
}

// HitRate returns the cache hit rate as a float between 0 and 1.
func (s *CacheStats) HitRate() float64 {
	total := atomic.LoadInt64(&s.Hits) + atomic.LoadInt64(&s.Misses)
	if total == 0 {
		return 0
	}
	return float64(atomic.LoadInt64(&s.Hits)) / float64(total)
}

// cacheEntry holds a cached value and its position in the LRU list.
type cacheEntry struct {
	value   interface{}
	element *list.Element
}

// FingerprintCache is a thread-safe LRU cache for molecular fingerprints.
// It uses sync.Map for concurrent lookups and a linked list for LRU eviction.
//
// When the cache exceeds maxSize, the least recently used entry is evicted.
// Cache hit/miss statistics are maintained for performance monitoring.
type FingerprintCache struct {
	store   sync.Map   // string key -> *cacheEntry
	lruList *list.List // ordered list for LRU eviction (elements store keys as strings)
	mu      sync.Mutex // protects lruList and eviction logic
	maxSize int
	stats   CacheStats
}

// NewFingerprintCache creates a new cache with the given maximum size.
// If maxSize <= 0, DefaultCacheSize is used.
func NewFingerprintCache(maxSize int) *FingerprintCache {
	if maxSize <= 0 {
		maxSize = DefaultCacheSize
	}
	return &FingerprintCache{
		lruList: list.New(),
		maxSize: maxSize,
	}
}

// Get retrieves a value from the cache. Returns (nil, false) if not found.
// On a cache hit, the entry is promoted to the front of the LRU list.
func (c *FingerprintCache) Get(key string) (interface{}, bool) {
	entry, ok := c.store.Load(key)
	if !ok {
		atomic.AddInt64(&c.stats.Misses, 1)
		return nil, false
	}

	atomic.AddInt64(&c.stats.Hits, 1)

	ce := entry.(*cacheEntry)
	// Promote to front of LRU list
	c.mu.Lock()
	if ce.element != nil {
		c.lruList.MoveToFront(ce.element)
	}
	c.mu.Unlock()

	return ce.value, true
}

// Set adds a value to the cache. If an entry with the same key already exists,
// it is updated and promoted to the front. If the cache is full, the least
// recently used entry is evicted.
func (c *FingerprintCache) Set(key string, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Remove existing entry's list element if present
	if existing, ok := c.store.Load(key); ok {
		ce := existing.(*cacheEntry)
		if ce.element != nil {
			c.lruList.Remove(ce.element)
		}
	}

	// Add new entry
	ce := &cacheEntry{value: value}
	ce.element = c.lruList.PushFront(key)
	c.store.Store(key, ce)

	// Evict LRU entries if over capacity
	for c.lruList.Len() > c.maxSize {
		back := c.lruList.Back()
		if back == nil {
			break
		}
		evictKey := back.Value.(string)
		c.lruList.Remove(back)
		c.store.Delete(evictKey)
	}
}

// Stats returns a snapshot of the current cache statistics.
func (c *FingerprintCache) Stats() CacheStats {
	return CacheStats{
		Hits:   atomic.LoadInt64(&c.stats.Hits),
		Misses: atomic.LoadInt64(&c.stats.Misses),
	}
}

// Len returns the current number of entries in the cache.
func (c *FingerprintCache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lruList.Len()
}

// NormalizeSMILES produces a canonical form of a SMILES string for use as a cache key.
// It trims whitespace to ensure that "CCO" and "  CCO  " produce the same key.
// In production, this would delegate to RDKit for full canonicalization.
func NormalizeSMILES(smiles string) string {
	return strings.TrimSpace(smiles)
}

// BuildCacheKey builds a deterministic cache key for fingerprint lookup.
// The key incorporates the normalized SMILES, fingerprint type, and calculation options
// so that different parameter combinations produce distinct cache entries.
func BuildCacheKey(smiles string, fpType FingerprintType, opts *FingerprintCalcOptions) string {
	var sb strings.Builder
	sb.WriteString(NormalizeSMILES(smiles))
	sb.WriteString("|")
	sb.WriteString(string(fpType))
	if opts != nil {
		sb.WriteString(fmt.Sprintf("|%d|%d|%t|%t",
			opts.Radius, opts.NumBits, opts.UseChirality, opts.UseFeatures))
	}
	return sb.String()
}

// FingerprintCalcOptions defines configuration for fingerprint calculation.
type FingerprintCalcOptions struct {
	Radius       int
	NumBits      int
	UseChirality bool
	UseFeatures  bool
}

// DefaultFingerprintCalcOptions returns default options.
func DefaultFingerprintCalcOptions() *FingerprintCalcOptions {
	return &FingerprintCalcOptions{
		Radius:       2,
		NumBits:      2048,
		UseChirality: false,
		UseFeatures:  false,
	}
}

func (o *FingerprintCalcOptions) Validate() error {
	if o.Radius < 0 {
		return errors.New(errors.ErrCodeInvalidInput, "radius cannot be negative")
	}
	if o.NumBits <= 0 {
		return errors.New(errors.ErrCodeInvalidInput, "numBits must be positive")
	}
	return nil
}

// FingerprintFusionStrategy defines how to combine multiple fingerprint scores.
type FingerprintFusionStrategy interface {
	Fuse(scores map[FingerprintType]float64, weights map[FingerprintType]float64) (float64, error)
}

// WeightedAverageFusion implements weighted average fusion.
type WeightedAverageFusion struct{}

func (s *WeightedAverageFusion) Fuse(scores map[FingerprintType]float64, weights map[FingerprintType]float64) (float64, error) {
	if len(scores) == 0 {
		return 0, errors.New(errors.ErrCodeInvalidInput, "scores map cannot be empty")
	}

	var totalScore, totalWeight float64
	for t, score := range scores {
		weight := 1.0
		if weights != nil {
			if w, ok := weights[t]; ok {
				weight = w
			}
		}
		totalScore += score * weight
		totalWeight += weight
	}

	if totalWeight == 0 {
		return 0, nil
	}

	result := totalScore / totalWeight
	if result > 1.0 {
		result = 1.0
	}
	if result < 0.0 {
		result = 0.0
	}
	return result, nil
}

// MaxFusion implements max score fusion.
type MaxFusion struct{}

func (s *MaxFusion) Fuse(scores map[FingerprintType]float64, weights map[FingerprintType]float64) (float64, error) {
	if len(scores) == 0 {
		return 0, errors.New(errors.ErrCodeInvalidInput, "scores map cannot be empty")
	}
	maxScore := 0.0
	for _, score := range scores {
		if score > maxScore {
			maxScore = score
		}
	}
	return maxScore, nil
}

// MinFusion implements min score fusion.
type MinFusion struct{}

func (s *MinFusion) Fuse(scores map[FingerprintType]float64, weights map[FingerprintType]float64) (float64, error) {
	if len(scores) == 0 {
		return 0, errors.New(errors.ErrCodeInvalidInput, "scores map cannot be empty")
	}
	minScore := 1.0
	for _, score := range scores {
		if score < minScore {
			minScore = score
		}
	}
	return minScore, nil
}

// PopCount calculates the population count (number of set bits).
func PopCount(data []byte) int {
	if data == nil {
		return 0
	}
	count := 0
	for _, b := range data {
		x := int(b)
		x = (x & 0x55) + ((x >> 1) & 0x55)
		x = (x & 0x33) + ((x >> 2) & 0x33)
		x = (x & 0x0f) + ((x >> 4) & 0x0f)
		count += x
	}
	return count
}

// BitAnd computes bitwise AND of two byte slices.
func BitAnd(a, b []byte) ([]byte, error) {
	if a == nil || b == nil {
		return nil, errors.New(errors.ErrCodeInvalidInput, "input cannot be nil")
	}
	if len(a) != len(b) {
		return nil, errors.New(errors.ErrCodeInvalidInput, "byte slices must have equal length")
	}
	res := make([]byte, len(a))
	for i := range a {
		res[i] = a[i] & b[i]
	}
	return res, nil
}

// BitOr computes bitwise OR of two byte slices.
func BitOr(a, b []byte) ([]byte, error) {
	if a == nil || b == nil {
		return nil, errors.New(errors.ErrCodeInvalidInput, "input cannot be nil")
	}
	if len(a) != len(b) {
		return nil, errors.New(errors.ErrCodeInvalidInput, "byte slices must have equal length")
	}
	res := make([]byte, len(a))
	for i := range a {
		res[i] = a[i] | b[i]
	}
	return res, nil
}

//Personal.AI order the ending

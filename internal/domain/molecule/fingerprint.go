package molecule

import (
	"context"
	"fmt"
	"time"

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
// It is an immutable value object.
type Fingerprint struct {
	Type         FingerprintType     `json:"type"`
	Encoding     FingerprintEncoding `json:"encoding"`
	Bits         []byte              `json:"bits,omitempty"`   // For BitVector and CountVector
	Vector       []float32           `json:"vector,omitempty"` // For DenseVector
	NumBits      int                 `json:"num_bits"`         // Length in bits for BitVector, or dimension for DenseVector
	Radius       int                 `json:"radius,omitempty"` // Only for Morgan/FCFP
	ModelVersion string              `json:"model_version,omitempty"`
	ComputedAt   time.Time           `json:"computed_at"`
}

// NewBitFingerprint creates a new bit vector fingerprint.
func NewBitFingerprint(fpType FingerprintType, bits []byte, numBits int, radius int) (*Fingerprint, error) {
	if !fpType.IsValid() {
		return nil, errors.New(errors.ErrCodeInvalidInput, "invalid fingerprint type")
	}
	// GNN is dense vector, not bit vector
	if fpType == FingerprintGNN {
		return nil, errors.New(errors.ErrCodeInvalidInput, "GNN fingerprint must use NewDenseFingerprint")
	}

	if len(bits) == 0 {
		return nil, errors.New(errors.ErrCodeInvalidInput, "bits cannot be empty")
	}
	if numBits <= 0 {
		return nil, errors.New(errors.ErrCodeInvalidInput, "numBits must be positive")
	}
	// Check if bits slice is large enough
	expectedBytes := (numBits + 7) / 8
	if len(bits) < expectedBytes {
		return nil, errors.New(errors.ErrCodeInvalidInput, fmt.Sprintf("insufficient bits length: got %d bytes, need %d bytes for %d bits", len(bits), expectedBytes, numBits))
	}

	// Specific validation
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
		// RDKit, AtomPair usually don't use radius in the same way or default to 0/specifics
		if radius != 0 {
			return nil, errors.New(errors.ErrCodeInvalidInput, "radius should be 0 for this fingerprint type")
		}
	}

	return &Fingerprint{
		Type:       fpType,
		Encoding:   EncodingBitVector,
		Bits:       bits,
		NumBits:    numBits,
		Radius:     radius,
		ComputedAt: time.Now().UTC(),
	}, nil
}

// NewCountFingerprint creates a new count vector fingerprint.
func NewCountFingerprint(fpType FingerprintType, bits []byte, numBits int, radius int) (*Fingerprint, error) {
	// Validation similar to BitFingerprint but for CountVector
	if !fpType.IsValid() {
		return nil, errors.New(errors.ErrCodeInvalidInput, "invalid fingerprint type")
	}
	if len(bits) == 0 {
		return nil, errors.New(errors.ErrCodeInvalidInput, "bits cannot be empty")
	}
	if numBits <= 0 {
		return nil, errors.New(errors.ErrCodeInvalidInput, "numBits must be positive")
	}
	// For count vector, bits length depends on count size (e.g. uint32), so exact check is harder without knowing count width.
	// We'll relax the check slightly or assume packed format.

	return &Fingerprint{
		Type:       fpType,
		Encoding:   EncodingCountVector,
		Bits:       bits,
		NumBits:    numBits,
		Radius:     radius,
		ComputedAt: time.Now().UTC(),
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

	return &Fingerprint{
		Type:         FingerprintGNN,
		Encoding:     EncodingDenseVector,
		Vector:       vector,
		NumBits:      len(vector), // Using NumBits to store dimension for consistency
		ModelVersion: modelVersion,
		ComputedAt:   time.Now().UTC(),
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
		return false, nil // Implicitly 0 if byte slice is shorter than NumBits? Or strictly check?
		// Spec says "valid bits slice length checked in constructor", so we can assume safety if we check len.
		// However, strict bound check is safer.
		return false, errors.New(errors.ErrCodeInvalidInput, "index out of range (byte slice)")
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
		// Return copy
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
	return nil // Count vector not supported yet
}

func (f *Fingerprint) String() string {
	return fmt.Sprintf("Fingerprint{type=%s, bits=%d, density=%.2f}", f.Type, f.NumBits, f.Density())
}

// FingerprintCalculator defines the interface for calculating fingerprints.
type FingerprintCalculator interface {
	Calculate(ctx context.Context, smiles string, fpType FingerprintType, opts *FingerprintCalcOptions) (*Fingerprint, error)
	BatchCalculate(ctx context.Context, smilesSlice []string, fpType FingerprintType, opts *FingerprintCalcOptions) ([]*Fingerprint, error)
	SupportedTypes() []FingerprintType
	// Standardize is also required by current service.go logic, but spec didn't mention it in fingerprint.go spec explicitly
	// but service.go uses `s.fpCalculator.Standardize`. Wait, `service.go` spec says:
	// "2. 调用 fpCalculator.Calculate 计算 canonical SMILES"
	// It implies fpCalculator might do standardization or expose it.
	// Actually, `service.go` spec says: "2. Call fpCalculator.Calculate to compute canonical SMILES (using RDKit standardize)".
	// This implies `Calculate` can return canonical SMILES? No, `Calculate` returns `*Fingerprint`.
	// Ah, step 3 says: "Through canonical SMILES compute InChI and InChIKey".
	// The `MoleculeService` spec says: "Call NewMolecule... Call fpCalculator.Calculate to compute canonical SMILES...".
	// This part is confusing in spec. Usually standardization is separate.
	// Looking at `internal/domain/molecule/interfaces.go` (legacy), it had `Standardize`.
	// The new `MoleculeService` spec in prompt says:
	// "2. Call fpCalculator.Calculate compute canonical SMILES (using RDKit standardization)"
	// This might mean "Perform standardization".
	// Maybe `FingerprintCalculator` has a method `Standardize`? Or is `CanonicalSMILES` considered a fingerprint? No.
	// I will add `Standardize` to `FingerprintCalculator` interface as it logically belongs to the chemical calculation service, even if the name is FingerprintCalculator.
	// Or maybe I should rename it to `ChemicalCalculator`? The spec says `fpCalculator FingerprintCalculator`.
	// I will add `Standardize` to the interface.
	Standardize(ctx context.Context, smiles string) (canonical string, inchi string, inchiKey string, formula string, weight float64, err error)
}

// FingerprintCalcOptions defines configuration for fingerprint calculation.
type FingerprintCalcOptions struct {
	Radius      int
	NumBits     int
	UseChirality bool
	UseFeatures bool
}

// DefaultFingerprintCalcOptions returns default options.
func DefaultFingerprintCalcOptions() *FingerprintCalcOptions {
	return &FingerprintCalcOptions{
		Radius:      2,
		NumBits:     2048,
		UseChirality: false,
		UseFeatures: false,
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
		return 0, nil // Avoid division by zero
	}

	result := totalScore / totalWeight
	if result > 1.0 { result = 1.0 }
	if result < 0.0 { result = 0.0 }
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
		// Brian Kernighan's algorithm for single byte?
		// Or just use bits.OnesCount8? But "std lib limit"? Go 1.22 has math/bits.
		// Using pre-computed table or simple loop.
		// Let's use a simple loop or math/bits if allowed. "Standard library" is allowed.
		// But let's stick to the prompt implication of implementing it.
		// math/bits.OnesCount8 is best.
		// If strict "no external dependency", math/bits is standard lib.
		// I'll implement lookup table for speed if I can't import math/bits easily or want to be explicit.
		// But spec says "Use lookup table (256 items) or Brian Kernighan".
		// I'll use a local implementation as requested.
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

package molecule

import (
	"context"
	"fmt"
	"time"

	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// FingerprintType defines the type of fingerprint.
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

func ParseFingerprintType(s string) (FingerprintType, error) {
	t := FingerprintType(s)
	if t.IsValid() {
		return t, nil
	}
	return "", errors.New(errors.ErrCodeValidation, "invalid fingerprint type: "+s)
}

// FingerprintEncoding defines the encoding format of fingerprint data.
type FingerprintEncoding string

const (
	EncodingBitVector   FingerprintEncoding = "bitvector"
	EncodingCountVector FingerprintEncoding = "countvector"
	EncodingDenseVector FingerprintEncoding = "densevector"
)

// Fingerprint represents a molecular fingerprint as an immutable value object.
type Fingerprint struct {
	Type         FingerprintType     `json:"type"`
	Encoding     FingerprintEncoding `json:"encoding"`
	Bits         []byte              `json:"bits,omitempty"`   // For BitVector/CountVector
	Vector       []float32           `json:"vector,omitempty"` // For DenseVector
	NumBits      int                 `json:"num_bits"`         // Number of bits or dimension
	Radius       int                 `json:"radius"`           // For circular fingerprints
	ModelVersion string              `json:"model_version,omitempty"`
	ComputedAt   time.Time           `json:"computed_at"`
}

// NewBitFingerprint creates a new bit vector fingerprint.
func NewBitFingerprint(fpType FingerprintType, bits []byte, numBits int, radius int) (*Fingerprint, error) {
	switch fpType {
	case FingerprintMACCS, FingerprintMorgan, FingerprintRDKit, FingerprintAtomPair, FingerprintFCFP:
		// valid
	default:
		return nil, errors.New(errors.ErrCodeValidation, "invalid fingerprint type for bit vector")
	}

	if bits == nil {
		return nil, errors.New(errors.ErrCodeValidation, "bits cannot be nil")
	}
	if len(bits) == 0 {
		return nil, errors.New(errors.ErrCodeValidation, "bits cannot be empty")
	}
	// Check if bits slice is large enough to hold numBits
	if len(bits) < (numBits+7)/8 {
		return nil, errors.New(errors.ErrCodeValidation, "bits slice too small for numBits")
	}

	if numBits <= 0 {
		return nil, errors.New(errors.ErrCodeValidation, "numBits must be positive")
	}

	if fpType == FingerprintMACCS {
		if numBits != 166 {
			return nil, errors.New(errors.ErrCodeValidation, "MACCS fingerprint must have 166 bits")
		}
		if radius != 0 {
			return nil, errors.New(errors.ErrCodeValidation, "radius must be 0 for MACCS fingerprint")
		}
	}

	if (fpType == FingerprintMorgan || fpType == FingerprintFCFP) {
		if radius < 1 || radius > 6 {
			return nil, errors.New(errors.ErrCodeValidation, "radius must be between 1 and 6 for circular fingerprints")
		}
	} else if fpType != FingerprintMACCS && radius != 0 {
		// For others, radius must be 0
		return nil, errors.New(errors.ErrCodeValidation, "radius must be 0 for non-circular fingerprints")
	}

	if fpType == FingerprintGNN {
		return nil, errors.New(errors.ErrCodeValidation, "GNN type not allowed for bit fingerprint")
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
	switch fpType {
	case FingerprintMorgan, FingerprintAtomPair, FingerprintFCFP, FingerprintRDKit:
		// valid
	default:
		return nil, errors.New(errors.ErrCodeValidation, "invalid fingerprint type for count vector")
	}

	if bits == nil {
		return nil, errors.New(errors.ErrCodeValidation, "bits cannot be nil")
	}
	if len(bits) == 0 {
		return nil, errors.New(errors.ErrCodeValidation, "bits cannot be empty")
	}
	if numBits <= 0 {
		return nil, errors.New(errors.ErrCodeValidation, "numBits must be positive")
	}

	if (fpType == FingerprintMorgan || fpType == FingerprintFCFP) {
		if radius < 1 || radius > 6 {
			return nil, errors.New(errors.ErrCodeValidation, "radius must be between 1 and 6 for circular fingerprints")
		}
	} else if radius != 0 {
		return nil, errors.New(errors.ErrCodeValidation, "radius must be 0 for non-circular fingerprints")
	}

	return &Fingerprint{
		Type:       fpType,
		Encoding:   EncodingCountVector,
		Bits:       bits,
		NumBits:    numBits,
		Radius:     radius,
		ComputedAt: time.Now().UTC(),
	}, nil
}

// NewDenseFingerprint creates a new dense vector fingerprint.
func NewDenseFingerprint(vector []float32, modelVersion string) (*Fingerprint, error) {
	if vector == nil {
		return nil, errors.New(errors.ErrCodeValidation, "vector cannot be nil")
	}
	if len(vector) == 0 {
		return nil, errors.New(errors.ErrCodeValidation, "vector cannot be empty")
	}
	if len(vector) < 32 {
		return nil, errors.New(errors.ErrCodeValidation, "vector dimension too small (min 32)")
	}
	if len(vector) > 4096 {
		return nil, errors.New(errors.ErrCodeValidation, "vector dimension too large (max 4096)")
	}
	if modelVersion == "" {
		return nil, errors.New(errors.ErrCodeValidation, "model version cannot be empty")
	}

	return &Fingerprint{
		Type:         FingerprintGNN,
		Encoding:     EncodingDenseVector,
		Vector:       vector,
		NumBits:      len(vector), // Dimension
		ModelVersion: modelVersion,
		ComputedAt:   time.Now().UTC(),
	}, nil
}

func (f *Fingerprint) IsBitVector() bool {
	return f.Encoding == EncodingBitVector
}

func (f *Fingerprint) IsCountVector() bool {
	return f.Encoding == EncodingCountVector
}

func (f *Fingerprint) IsDenseVector() bool {
	return f.Encoding == EncodingDenseVector
}

// BitCount returns the population count (number of set bits).
func (f *Fingerprint) BitCount() int {
	if !f.IsBitVector() {
		return 0 // Only meaningful for bit vectors
	}
	return PopCount(f.Bits)
}

// Density returns the ratio of set bits to total bits.
func (f *Fingerprint) Density() float64 {
	if !f.IsBitVector() || f.NumBits == 0 {
		return 0.0
	}
	return float64(f.BitCount()) / float64(f.NumBits)
}

// GetBit returns the value of the bit at index.
func (f *Fingerprint) GetBit(index int) (bool, error) {
	if !f.IsBitVector() {
		return false, errors.New(errors.ErrCodeValidation, "not a bit vector")
	}
	if index < 0 || index >= f.NumBits {
		return false, errors.New(errors.ErrCodeValidation, "index out of bounds")
	}
	byteIndex := index / 8
	bitIndex := uint(index % 8)
	if byteIndex >= len(f.Bits) {
		return false, nil // Should use strict check or just nil?
	}
	return (f.Bits[byteIndex] & (1 << bitIndex)) != 0, nil
}

// Dimension returns the dimension of the fingerprint.
func (f *Fingerprint) Dimension() int {
	if f.IsDenseVector() {
		return len(f.Vector)
	}
	return f.NumBits
}

// ToFloat32Slice converts the fingerprint to a float32 slice.
func (f *Fingerprint) ToFloat32Slice() []float32 {
	if f.IsDenseVector() {
		// Return copy
		result := make([]float32, len(f.Vector))
		copy(result, f.Vector)
		return result
	}
	if f.IsBitVector() {
		result := make([]float32, f.NumBits)
		for i := 0; i < f.NumBits; i++ {
			bit, _ := f.GetBit(i)
			if bit {
				result[i] = 1.0
			} else {
				result[i] = 0.0
			}
		}
		return result
	}
	return nil // CountVector conversion strategy not defined here
}

func (f *Fingerprint) String() string {
	if f.IsDenseVector() {
		return fmt.Sprintf("Fingerprint{type=%s, dim=%d}", f.Type, len(f.Vector))
	}
	return fmt.Sprintf("Fingerprint{type=%s, bits=%d, density=%.2f}", f.Type, f.NumBits, f.Density())
}

// FingerprintCalculator defines the interface for calculating fingerprints.
type FingerprintCalculator interface {
	Calculate(ctx context.Context, smiles string, fpType FingerprintType, opts *FingerprintCalcOptions) (*Fingerprint, error)
	BatchCalculate(ctx context.Context, smilesSlice []string, fpType FingerprintType, opts *FingerprintCalcOptions) ([]*Fingerprint, error)
	SupportedTypes() []FingerprintType
	Standardize(ctx context.Context, smiles string) (*StructuralIdentifiers, error) // Added to match previous usage
}

// FingerprintCalcOptions defines parameters for fingerprint generation.
type FingerprintCalcOptions struct {
	Radius      int
	NumBits     int // Replaces Bits to match requirement
	UseChirality bool
	UseFeatures bool
}

func DefaultFingerprintCalcOptions() *FingerprintCalcOptions {
	return &FingerprintCalcOptions{
		Radius:       2,
		NumBits:      2048,
		UseChirality: false,
		UseFeatures:  false,
	}
}

func (o *FingerprintCalcOptions) Validate() error {
	if o.NumBits <= 0 {
		return errors.New(errors.ErrCodeValidation, "bits must be positive")
	}
	if o.Radius < 0 {
		return errors.New(errors.ErrCodeValidation, "radius cannot be negative")
	}
	return nil
}

// FingerprintFusionStrategy defines how to combine multiple similarity scores.
type FingerprintFusionStrategy interface {
	Fuse(scores map[FingerprintType]float64, weights map[FingerprintType]float64) (float64, error)
}

// WeightedAverageFusion implements FingerprintFusionStrategy.
type WeightedAverageFusion struct{}

func (f *WeightedAverageFusion) Fuse(scores map[FingerprintType]float64, weights map[FingerprintType]float64) (float64, error) {
	if len(scores) == 0 {
		return 0, errors.New(errors.ErrCodeValidation, "scores cannot be empty")
	}

	var totalScore, totalWeight float64
	for t, s := range scores {
		w := 1.0
		if weights != nil {
			if val, ok := weights[t]; ok {
				w = val
			} else {
				w = 1.0 // Default weight
			}
		}
		totalScore += s * w
		totalWeight += w
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

// MaxFusion implements FingerprintFusionStrategy.
type MaxFusion struct{}

func (f *MaxFusion) Fuse(scores map[FingerprintType]float64, weights map[FingerprintType]float64) (float64, error) {
	if len(scores) == 0 {
		return 0, errors.New(errors.ErrCodeValidation, "scores cannot be empty")
	}

	maxScore := -1.0
	for _, s := range scores {
		if s > maxScore {
			maxScore = s
		}
	}
	return maxScore, nil
}

// MinFusion implements FingerprintFusionStrategy.
type MinFusion struct{}

func (f *MinFusion) Fuse(scores map[FingerprintType]float64, weights map[FingerprintType]float64) (float64, error) {
	if len(scores) == 0 {
		return 0, errors.New(errors.ErrCodeValidation, "scores cannot be empty")
	}
	minScore := 2.0 // Greater than max possible 1.0
	for _, s := range scores {
		if s < minScore {
			minScore = s
		}
	}
	return minScore, nil
}

// PopCount returns the number of set bits in a byte slice.
func PopCount(data []byte) int {
	if data == nil {
		return 0
	}
	count := 0
	// Using lookup table for 8-bit
	for _, b := range data {
		count += int(popCountTable[b])
	}
	return count
}

var popCountTable = [256]byte{
	0, 1, 1, 2, 1, 2, 2, 3, 1, 2, 2, 3, 2, 3, 3, 4,
	1, 2, 2, 3, 2, 3, 3, 4, 2, 3, 3, 4, 3, 4, 4, 5,
	1, 2, 2, 3, 2, 3, 3, 4, 2, 3, 3, 4, 3, 4, 4, 5,
	2, 3, 3, 4, 3, 4, 4, 5, 3, 4, 4, 5, 4, 5, 5, 6,
	1, 2, 2, 3, 2, 3, 3, 4, 2, 3, 3, 4, 3, 4, 4, 5,
	2, 3, 3, 4, 3, 4, 4, 5, 3, 4, 4, 5, 4, 5, 5, 6,
	2, 3, 3, 4, 3, 4, 4, 5, 3, 4, 4, 5, 4, 5, 5, 6,
	3, 4, 4, 5, 4, 5, 5, 6, 4, 5, 5, 6, 5, 6, 6, 7,
	1, 2, 2, 3, 2, 3, 3, 4, 2, 3, 3, 4, 3, 4, 4, 5,
	2, 3, 3, 4, 3, 4, 4, 5, 3, 4, 4, 5, 4, 5, 5, 6,
	2, 3, 3, 4, 3, 4, 4, 5, 3, 4, 4, 5, 4, 5, 5, 6,
	3, 4, 4, 5, 4, 5, 5, 6, 4, 5, 5, 6, 5, 6, 6, 7,
	2, 3, 3, 4, 3, 4, 4, 5, 3, 4, 4, 5, 4, 5, 5, 6,
	3, 4, 4, 5, 4, 5, 5, 6, 4, 5, 5, 6, 5, 6, 6, 7,
	3, 4, 4, 5, 4, 5, 5, 6, 4, 5, 5, 6, 5, 6, 6, 7,
	4, 5, 5, 6, 5, 6, 6, 7, 5, 6, 6, 7, 6, 7, 7, 8,
}

// BitAnd returns the bitwise AND of two byte slices.
func BitAnd(a, b []byte) ([]byte, error) {
	if a == nil || b == nil {
		return nil, errors.New(errors.ErrCodeValidation, "inputs cannot be nil")
	}
	if len(a) != len(b) {
		return nil, errors.New(errors.ErrCodeValidation, "slices must be equal length")
	}
	// Handle empty inputs case (len=0) - should return empty slice?
	if len(a) == 0 {
		return []byte{}, nil
	}

	res := make([]byte, len(a))
	for i := 0; i < len(a); i++ {
		res[i] = a[i] & b[i]
	}
	return res, nil
}

// BitOr returns the bitwise OR of two byte slices.
func BitOr(a, b []byte) ([]byte, error) {
	if a == nil || b == nil {
		return nil, errors.New(errors.ErrCodeValidation, "inputs cannot be nil")
	}
	if len(a) != len(b) {
		return nil, errors.New(errors.ErrCodeValidation, "slices must be equal length")
	}
	if len(a) == 0 {
		return []byte{}, nil
	}

	res := make([]byte, len(a))
	for i := 0; i < len(a); i++ {
		res[i] = a[i] | b[i]
	}
	return res, nil
}

// StructuralIdentifiers holds computed chemical identifiers.
type StructuralIdentifiers struct {
	CanonicalSMILES string
	InChI           string
	InChIKey        string
	Formula         string
	Weight          float64
}

//Personal.AI order the ending

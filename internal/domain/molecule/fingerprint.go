package molecule

import (
	"context"
	"fmt"
	"time"

	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// FingerprintType defines the algorithm used for molecular fingerprinting.
type FingerprintType string

const (
	FingerprintMACCS    FingerprintType = "maccs"
	FingerprintMorgan   FingerprintType = "morgan"
	FingerprintRDKit    FingerprintType = "rdkit"
	FingerprintAtomPair FingerprintType = "atom_pair"
	FingerprintFCFP     FingerprintType = "fcfp"
	FingerprintGNN      FingerprintType = "gnn"
)

// IsValid checks if the fingerprint type is valid.
func (f FingerprintType) IsValid() bool {
	switch f {
	case FingerprintMACCS, FingerprintMorgan, FingerprintRDKit, FingerprintAtomPair, FingerprintFCFP, FingerprintGNN:
		return true
	default:
		return false
	}
}

// String returns the string representation of the fingerprint type.
func (f FingerprintType) String() string {
	return string(f)
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
	ft := FingerprintType(s)
	if ft.IsValid() {
		return ft, nil
	}
	return "", errors.New(errors.ErrCodeFingerprintTypeUnsupported, "unsupported fingerprint type: "+s)
}

// FingerprintEncoding identifies the encoding format of the fingerprint data.
type FingerprintEncoding string

const (
	EncodingBitVector   FingerprintEncoding = "bitvector"
	EncodingCountVector FingerprintEncoding = "countvector"
	EncodingDenseVector FingerprintEncoding = "densevector"
)

// Fingerprint represents a molecular fingerprint as a value object.
// It is immutable; once created, its data cannot be modified.
type Fingerprint struct {
	Type         FingerprintType     `json:"type"`
	Encoding     FingerprintEncoding `json:"encoding"`
	Bits         []byte              `json:"bits,omitempty"`   // For BitVector/CountVector
	Vector       []float32           `json:"vector,omitempty"` // For DenseVector
	NumBits      int                 `json:"num_bits"`         // Number of bits or vector dimension
	Radius       int                 `json:"radius,omitempty"` // For circular fingerprints
	ModelVersion string              `json:"model_version,omitempty"`
	ComputedAt   time.Time           `json:"computed_at"`
}

// NewBitFingerprint constructs a new bit vector fingerprint.
func NewBitFingerprint(fpType FingerprintType, bits []byte, numBits int, radius int) (*Fingerprint, error) {
	if !fpType.IsValid() || fpType == FingerprintGNN {
		return nil, errors.New(errors.ErrCodeFingerprintTypeUnsupported, "invalid bit vector fingerprint type")
	}
	if len(bits) == 0 {
		return nil, errors.New(errors.ErrCodeValidation, "fingerprint bits cannot be empty")
	}
	expectedBytes := (numBits + 7) / 8
	if len(bits) < expectedBytes {
		return nil, errors.New(errors.ErrCodeValidation, "insufficient bytes for specified numBits")
	}
	if numBits <= 0 {
		return nil, errors.New(errors.ErrCodeValidation, "numBits must be positive")
	}

	// Specific constraints
	if fpType == FingerprintMACCS && numBits != 166 {
		return nil, errors.New(errors.ErrCodeValidation, "MACCS fingerprint must have 166 bits")
	}
	if fpType == FingerprintMorgan || fpType == FingerprintFCFP {
		if radius < 1 || radius > 6 {
			return nil, errors.New(errors.ErrCodeValidation, "radius for Morgan/FCFP must be between 1 and 6")
		}
	} else if radius != 0 {
		return nil, errors.New(errors.ErrCodeValidation, "radius must be 0 for this fingerprint type")
	}

	return &Fingerprint{
		Type:       fpType,
		Encoding:   EncodingBitVector,
		Bits:       append([]byte(nil), bits...), // Defensive copy
		NumBits:    numBits,
		Radius:     radius,
		ComputedAt: time.Now().UTC(),
	}, nil
}

// NewCountFingerprint constructs a new count vector fingerprint.
func NewCountFingerprint(fpType FingerprintType, bits []byte, numBits int, radius int) (*Fingerprint, error) {
	if !fpType.IsValid() || fpType == FingerprintGNN {
		return nil, errors.New(errors.ErrCodeFingerprintTypeUnsupported, "invalid count vector fingerprint type")
	}
	if len(bits) == 0 {
		return nil, errors.New(errors.ErrCodeValidation, "fingerprint bits cannot be empty")
	}
	if numBits <= 0 {
		return nil, errors.New(errors.ErrCodeValidation, "numBits must be positive")
	}
	if fpType == FingerprintMorgan || fpType == FingerprintFCFP {
		if radius < 1 || radius > 6 {
			return nil, errors.New(errors.ErrCodeValidation, "radius for Morgan/FCFP must be between 1 and 6")
		}
	} else if radius != 0 {
		return nil, errors.New(errors.ErrCodeValidation, "radius must be 0 for this fingerprint type")
	}

	return &Fingerprint{
		Type:       fpType,
		Encoding:   EncodingCountVector,
		Bits:       append([]byte(nil), bits...), // Defensive copy
		NumBits:    numBits,
		Radius:     radius,
		ComputedAt: time.Now().UTC(),
	}, nil
}

// NewDenseFingerprint constructs a new dense vector fingerprint.
func NewDenseFingerprint(vector []float32, modelVersion string) (*Fingerprint, error) {
	if len(vector) < 32 || len(vector) > 4096 {
		return nil, errors.New(errors.ErrCodeValidation, "dense vector dimension must be between 32 and 4096")
	}
	if modelVersion == "" {
		return nil, errors.New(errors.ErrCodeValidation, "modelVersion cannot be empty")
	}

	return &Fingerprint{
		Type:         FingerprintGNN,
		Encoding:     EncodingDenseVector,
		Vector:       append([]float32(nil), vector...), // Defensive copy
		NumBits:      len(vector),
		ModelVersion: modelVersion,
		ComputedAt:   time.Now().UTC(),
	}, nil
}

// IsBitVector returns true if the encoding is BitVector.
func (fp *Fingerprint) IsBitVector() bool {
	return fp.Encoding == EncodingBitVector
}

// IsCountVector returns true if the encoding is CountVector.
func (fp *Fingerprint) IsCountVector() bool {
	return fp.Encoding == EncodingCountVector
}

// IsDenseVector returns true if the encoding is DenseVector.
func (fp *Fingerprint) IsDenseVector() bool {
	return fp.Encoding == EncodingDenseVector
}

// BitCount returns the number of set bits (popcount). Only for BitVector.
func (fp *Fingerprint) BitCount() int {
	if !fp.IsBitVector() {
		return 0
	}
	return PopCount(fp.Bits)
}

// Density returns the bit density (set bits / total bits). Only for BitVector.
func (fp *Fingerprint) Density() float64 {
	if !fp.IsBitVector() || fp.NumBits == 0 {
		return 0
	}
	return float64(fp.BitCount()) / float64(fp.NumBits)
}

// GetBit returns the value of the bit at the given index.
func (fp *Fingerprint) GetBit(index int) (bool, error) {
	if !fp.IsBitVector() {
		return false, errors.New(errors.ErrCodeValidation, "GetBit only supported for bit vectors")
	}
	if index < 0 || index >= fp.NumBits {
		return false, errors.New(errors.ErrCodeValidation, "index out of range")
	}
	byteIdx := index / 8
	bitIdx := uint(index % 8)
	return (fp.Bits[byteIdx] & (1 << bitIdx)) != 0, nil
}

// Dimension returns the vector dimension or number of bits.
func (fp *Fingerprint) Dimension() int {
	if fp.IsDenseVector() {
		return len(fp.Vector)
	}
	return fp.NumBits
}

// ToFloat32Slice converts the fingerprint to a float32 slice.
func (fp *Fingerprint) ToFloat32Slice() []float32 {
	if fp.IsDenseVector() {
		return append([]float32(nil), fp.Vector...)
	}
	res := make([]float32, fp.NumBits)
	for i := 0; i < fp.NumBits; i++ {
		byteIdx := i / 8
		bitIdx := uint(i % 8)
		if (fp.Bits[byteIdx] & (1 << bitIdx)) != 0 {
			res[i] = 1.0
		} else {
			res[i] = 0.0
		}
	}
	return res
}

// String returns a debug string for the fingerprint.
func (fp *Fingerprint) String() string {
	if fp.IsDenseVector() {
		return fmt.Sprintf("Fingerprint{type=%s, dim=%d, encoding=%s}", fp.Type, len(fp.Vector), fp.Encoding)
	}
	return fmt.Sprintf("Fingerprint{type=%s, bits=%d, density=%.4f}", fp.Type, fp.NumBits, fp.Density())
}

// StructureIdentifiers holds chemical identifiers calculated by a cheminformatics tool.
type StructureIdentifiers struct {
	CanonicalSMILES string
	InChI           string
	InChIKey        string
	Formula         string
	Weight          float64
}

// FingerprintCalculator defines the interface for computing fingerprints and structural identifiers.
type FingerprintCalculator interface {
	// Standardize returns canonical SMILES and other identifiers using RDKit.
	Standardize(ctx context.Context, smiles string) (*StructureIdentifiers, error)
	// Calculate computes a fingerprint for a given SMILES string.
	Calculate(ctx context.Context, smiles string, fpType FingerprintType, opts *FingerprintCalcOptions) (*Fingerprint, error)
	// BatchCalculate computes fingerprints for multiple SMILES strings.
	BatchCalculate(ctx context.Context, smilesSlice []string, fpType FingerprintType, opts *FingerprintCalcOptions) ([]*Fingerprint, error)
	// SupportedTypes returns the fingerprint types supported by this calculator.
	SupportedTypes() []FingerprintType
}

// FingerprintCalcOptions defines options for fingerprint calculation.
type FingerprintCalcOptions struct {
	Radius       int
	NumBits      int
	UseChirality bool
	UseFeatures  bool
}

// DefaultFingerprintCalcOptions returns the default options.
func DefaultFingerprintCalcOptions() *FingerprintCalcOptions {
	return &FingerprintCalcOptions{
		Radius:       2,
		NumBits:      2048,
		UseChirality: false,
		UseFeatures:  false,
	}
}

// Validate checks if the options are valid.
func (o *FingerprintCalcOptions) Validate() error {
	if o.Radius < 1 || o.Radius > 6 {
		return errors.New(errors.ErrCodeValidation, "radius must be between 1 and 6")
	}
	if o.NumBits < 128 || o.NumBits > 4096 {
		return errors.New(errors.ErrCodeValidation, "numBits must be between 128 and 4096")
	}
	return nil
}

// FingerprintFusionStrategy defines the interface for fusing multiple fingerprint scores.
type FingerprintFusionStrategy interface {
	Fuse(scores map[FingerprintType]float64, weights map[FingerprintType]float64) (float64, error)
}

// WeightedAverageFusion implements FingerprintFusionStrategy using weighted average.
type WeightedAverageFusion struct{}

// Fuse calculates the weighted average of similarity scores.
func (s *WeightedAverageFusion) Fuse(scores map[FingerprintType]float64, weights map[FingerprintType]float64) (float64, error) {
	if len(scores) == 0 {
		return 0, errors.New(errors.ErrCodeValidation, "no scores to fuse")
	}
	var totalScore, totalWeight float64
	for ft, score := range scores {
		weight := 1.0
		if w, ok := weights[ft]; ok {
			weight = w
		}
		totalScore += score * weight
		totalWeight += weight
	}
	if totalWeight == 0 {
		return 0, nil
	}
	result := totalScore / totalWeight
	if result < 0 {
		result = 0
	} else if result > 1 {
		result = 1
	}
	return result, nil
}

// MaxFusion implements FingerprintFusionStrategy returning the maximum score.
type MaxFusion struct{}

// Fuse returns the maximum score among all provided fingerprints.
func (s *MaxFusion) Fuse(scores map[FingerprintType]float64, _ map[FingerprintType]float64) (float64, error) {
	if len(scores) == 0 {
		return 0, errors.New(errors.ErrCodeValidation, "no scores to fuse")
	}
	maxScore := -1.0
	for _, score := range scores {
		if score > maxScore {
			maxScore = score
		}
	}
	return maxScore, nil
}

// MinFusion implements FingerprintFusionStrategy returning the minimum score.
type MinFusion struct{}

// Fuse returns the minimum score among all provided fingerprints.
func (s *MinFusion) Fuse(scores map[FingerprintType]float64, _ map[FingerprintType]float64) (float64, error) {
	if len(scores) == 0 {
		return 0, errors.New(errors.ErrCodeValidation, "no scores to fuse")
	}
	minScore := 2.0
	for _, score := range scores {
		if score < minScore {
			minScore = score
		}
	}
	return minScore, nil
}

// PopCount counts the number of set bits in a byte slice.
func PopCount(data []byte) int {
	count := 0
	for _, b := range data {
		// Brian Kernighan's algorithm per byte
		v := uint8(b)
		for v > 0 {
			v &= v - 1
			count++
		}
	}
	return count
}

// PopCountByte counts set bits in a single byte.
func PopCountByte(b byte) int {
	count := 0
	v := uint8(b)
	for v > 0 {
		v &= v - 1
		count++
	}
	return count
}

// BitAnd returns the bitwise AND of two equal-length byte slices.
func BitAnd(a, b []byte) ([]byte, error) {
	if len(a) != len(b) {
		return nil, errors.New(errors.ErrCodeValidation, "byte slices must have equal length")
	}
	res := make([]byte, len(a))
	for i := range a {
		res[i] = a[i] & b[i]
	}
	return res, nil
}

// BitOr returns the bitwise OR of two equal-length byte slices.
func BitOr(a, b []byte) ([]byte, error) {
	if len(a) != len(b) {
		return nil, errors.New(errors.ErrCodeValidation, "byte slices must have equal length")
	}
	res := make([]byte, len(a))
	for i := range a {
		res[i] = a[i] | b[i]
	}
	return res, nil
}

//Personal.AI order the ending

// Package molecule provides molecular fingerprint computation for chemical
// similarity search in the KeyIP-Intelligence platform.  Fingerprints encode
// molecular structure as fixed-length bit vectors, enabling efficient Tanimoto
// similarity calculations and vector-based search in Milvus.
package molecule

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math/bits"
	"regexp"
	"strings"

	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
	mtypes "github.com/turtacn/KeyIP-Intelligence/pkg/types/molecule"
)

// ─────────────────────────────────────────────────────────────────────────────
// Fingerprint Structure
// ─────────────────────────────────────────────────────────────────────────────

// Fingerprint represents a molecular fingerprint as a bit vector.  The Bits
// field stores the packed bit array as bytes, where bit i is stored in byte
// i/8 at bit position i%8.
type Fingerprint struct {
	// Type identifies which fingerprint algorithm was used.
	Type mtypes.FingerprintType `json:"type"`

	// Bits is the packed bit vector representation.
	Bits []byte `json:"bits"`

	// Length is the total number of bits in the fingerprint.
	Length int `json:"length"`

	// NumOnBits is the count of set bits (popcount).
	NumOnBits int `json:"num_on_bits"`
}

// NewFingerprint constructs a Fingerprint from raw bit data.
func NewFingerprint(fpType mtypes.FingerprintType, data []byte, length int) *Fingerprint {
	// Calculate popcount
	onBits := 0
	for _, b := range data {
		onBits += bits.OnesCount8(uint8(b))
	}

	return &Fingerprint{
		Type:      fpType,
		Bits:      data,
		Length:    length,
		NumOnBits: onBits,
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Bit Operations
// ─────────────────────────────────────────────────────────────────────────────

// GetBit returns true if the bit at the given index is set.
func (fp *Fingerprint) GetBit(index int) bool {
	if index < 0 || index >= fp.Length {
		return false
	}
	byteIdx := index / 8
	bitIdx := uint(index % 8)
	return (fp.Bits[byteIdx] & (1 << bitIdx)) != 0
}

// SetBit sets the bit at the given index to 1.
func (fp *Fingerprint) SetBit(index int) {
	if index < 0 || index >= fp.Length {
		return
	}
	byteIdx := index / 8
	bitIdx := uint(index % 8)
	oldByte := fp.Bits[byteIdx]
	fp.Bits[byteIdx] |= (1 << bitIdx)
	if oldByte != fp.Bits[byteIdx] {
		fp.NumOnBits++
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Serialization
// ─────────────────────────────────────────────────────────────────────────────

// ToBytes serializes the fingerprint to a byte slice for storage or vector DB.
func (fp *Fingerprint) ToBytes() []byte {
	return fp.Bits
}

// FingerprintFromBytes deserializes a fingerprint from byte data.
func FingerprintFromBytes(fpType mtypes.FingerprintType, data []byte, length int) *Fingerprint {
	return NewFingerprint(fpType, data, length)
}

// ─────────────────────────────────────────────────────────────────────────────
// Morgan (Circular) Fingerprint
// ─────────────────────────────────────────────────────────────────────────────

// CalculateMorganFingerprint computes a simplified Morgan (circular) fingerprint.
// This implementation uses a basic atom-centered neighborhood hashing approach.
//
// Parameters:
//   - smiles: the molecule's SMILES string
//   - radius: the maximum bond distance for neighborhoods (default 2)
//   - nBits: the fingerprint length in bits (default 2048)
//
// Real-world implementation would use RDKit's GetMorganFingerprintAsBitVect.
func CalculateMorganFingerprint(smiles string, radius int, nBits int) (*Fingerprint, error) {
	if smiles == "" {
		return nil, errors.InvalidParam("SMILES string cannot be empty")
	}
	if radius < 0 {
		radius = 2
	}
	if nBits <= 0 {
		nBits = 2048
	}

	// Parse SMILES into atoms (simplified: split by bond symbols)
	atoms := parseSMILESAtoms(smiles)
	if len(atoms) == 0 {
		return nil, errors.New(errors.CodeMoleculeInvalidSMILES, "no atoms found in SMILES")
	}

	// Initialize bit vector
	numBytes := (nBits + 7) / 8
	bits := make([]byte, numBytes)

	// For each atom, hash its environment at each radius level
	for i, atom := range atoms {
		for r := 0; r <= radius; r++ {
			// Create environment descriptor: atom type + radius + position
			envHash := hashEnvironment(atom, r, i)
			bitIdx := int(envHash % uint64(nBits))
			setBit(bits, bitIdx)
		}
	}

	return NewFingerprint(mtypes.FPMorgan, bits, nBits), nil
}

// parseSMILESAtoms extracts individual atoms from a SMILES string.
// This is a simplified implementation; real parsing requires a full SMILES parser.
func parseSMILESAtoms(smiles string) []string {
	// Remove brackets and numbers for simplified parsing
	cleaned := regexp.MustCompile(`[\[\]0-9\-=#/\\()]`).ReplaceAllString(smiles, "")
	atoms := []string{}
	for _, ch := range cleaned {
		if ch >= 'A' && ch <= 'Z' || ch >= 'a' && ch <= 'z' {
			atoms = append(atoms, string(ch))
		}
	}
	return atoms
}

// hashEnvironment creates a hash for an atom's local environment.
func hashEnvironment(atom string, radius, position int) uint64 {
	// Simple hash: combine atom type, radius, and position
	data := fmt.Sprintf("%s:%d:%d", atom, radius, position)
	hash := sha256.Sum256([]byte(data))
	return binary.BigEndian.Uint64(hash[:8])
}

// setBit sets the bit at the given index in a byte slice.
func setBit(bits []byte, index int) {
	byteIdx := index / 8
	bitIdx := uint(index % 8)
	bits[byteIdx] |= (1 << bitIdx)
}

// ─────────────────────────────────────────────────────────────────────────────
// MACCS Keys Fingerprint
// ─────────────────────────────────────────────────────────────────────────────

// CalculateMACCSFingerprint computes a simplified MACCS 166-key fingerprint.
// This implementation checks for a subset of common structural patterns.
//
// Real-world implementation would use RDKit's MACCSkeys.GenMACCSKeys.
func CalculateMACCSFingerprint(smiles string) (*Fingerprint, error) {
	if smiles == "" {
		return nil, errors.InvalidParam("SMILES string cannot be empty")
	}

	const nBits = 166
	numBytes := (nBits + 7) / 8
	bits := make([]byte, numBytes)

	// Simplified pattern matching for common structural features
	patterns := []struct {
		bitIdx  int
		pattern string
		isRegex bool
	}{
		// Aromatic rings
		{10, "c1ccccc1", false}, // benzene
		{11, "c1cccc1", false},  // 5-membered aromatic
		// Heteroatoms
		{20, "N", false},  // nitrogen
		{21, "O", false},  // oxygen
		{22, "S", false},  // sulfur
		{23, "F", false},  // fluorine
		{24, "Cl", false}, // chlorine
		{25, "Br", false}, // bromine
		// Functional groups
		{30, "C(=O)O", false},  // carboxylic acid
		{31, "C(=O)N", false},  // amide
		{32, "C=O", false},     // carbonyl
		{33, "C#N", false},     // nitrile
		{34, "[NH2]", false},   // primary amine
		{35, "O[H]", false},    // hydroxyl
		{36, "C=C", false},     // double bond
		{37, "C#C", false},     // triple bond
		// Ring features
		{40, "(", false}, // any ring (has parenthesis)
		// More patterns would be added in a real implementation...
	}

	lowerSMILES := strings.ToLower(smiles)
	for _, p := range patterns {
		if p.bitIdx >= nBits {
			continue
		}
		pattern := strings.ToLower(p.pattern)
		if strings.Contains(lowerSMILES, pattern) {
			setBit(bits, p.bitIdx)
		}
	}

	// Set some additional bits based on molecular size
	atomCount := len(parseSMILESAtoms(smiles))
	if atomCount > 5 {
		setBit(bits, 50)
	}
	if atomCount > 10 {
		setBit(bits, 51)
	}
	if atomCount > 20 {
		setBit(bits, 52)
	}

	return NewFingerprint(mtypes.FPMACCS, bits, nBits), nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Topological Fingerprint
// ─────────────────────────────────────────────────────────────────────────────

// CalculateTopologicalFingerprint computes a path-based topological fingerprint.
// It enumerates all paths of length minPath to maxPath in the molecular graph
// and hashes them into a bit vector.
//
// Parameters:
//   - smiles: the molecule's SMILES string
//   - minPath: minimum path length to consider
//   - maxPath: maximum path length to consider
//   - nBits: fingerprint length in bits
func CalculateTopologicalFingerprint(smiles string, minPath, maxPath, nBits int) (*Fingerprint, error) {
	if smiles == "" {
		return nil, errors.InvalidParam("SMILES string cannot be empty")
	}
	if minPath < 1 {
		minPath = 1
	}
	if maxPath < minPath {
		maxPath = 7
	}
	if nBits <= 0 {
		nBits = 2048
	}

	atoms := parseSMILESAtoms(smiles)
	if len(atoms) == 0 {
		return nil, errors.New(errors.CodeMoleculeInvalidSMILES, "no atoms found in SMILES")
	}

	numBytes := (nBits + 7) / 8
	bits := make([]byte, numBytes)

	// Enumerate paths (simplified: linear sequences of atoms)
	for pathLen := minPath; pathLen <= maxPath && pathLen <= len(atoms); pathLen++ {
		for i := 0; i <= len(atoms)-pathLen; i++ {
			path := atoms[i : i+pathLen]
			pathStr := strings.Join(path, "-")
			pathHash := hashPath(pathStr)
			bitIdx := int(pathHash % uint64(nBits))
			setBit(bits, bitIdx)
		}
	}

	return NewFingerprint(mtypes.FPTopological, bits, nBits), nil
}

// hashPath creates a hash for a molecular path.
func hashPath(path string) uint64 {
	hash := sha256.Sum256([]byte(path))
	return binary.BigEndian.Uint64(hash[:8])
}

//Personal.AI order the ending

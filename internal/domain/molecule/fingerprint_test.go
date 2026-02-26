package molecule

import (
	"math"
	"testing"
)

const testEpsilon = 1e-9

func TestFingerprintType_IsValid(t *testing.T) {
	t.Parallel()
	tests := []struct {
		fpType FingerprintType
		valid  bool
	}{
		{FingerprintMACCS, true},
		{FingerprintMorgan, true},
		{FingerprintRDKit, true},
		{FingerprintAtomPair, true},
		{FingerprintFCFP, true},
		{FingerprintGNN, true},
		{FingerprintType("invalid"), false},
	}
	for _, tt := range tests {
		if got := tt.fpType.IsValid(); got != tt.valid {
			t.Errorf("FingerprintType(%s).IsValid() = %v, want %v", tt.fpType, got, tt.valid)
		}
	}
}

func TestFingerprintType_String(t *testing.T) {
	t.Parallel()
	if FingerprintMorgan.String() != "morgan" {
		t.Errorf("FingerprintMorgan.String() = %s, want morgan", FingerprintMorgan.String())
	}
}

func TestParseFingerprintType(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input   string
		want    FingerprintType
		wantErr bool
	}{
		{"morgan", FingerprintMorgan, false},
		{"maccs", FingerprintMACCS, false},
		{"invalid", "", true},
	}
	for _, tt := range tests {
		got, err := ParseFingerprintType(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("ParseFingerprintType(%s) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			return
		}
		if !tt.wantErr && got != tt.want {
			t.Errorf("ParseFingerprintType(%s) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestNewBitFingerprint(t *testing.T) {
	t.Parallel()
	validBits := make([]byte, 256) // 2048 bits

	tests := []struct {
		name    string
		fpType  FingerprintType
		bits    []byte
		numBits int
		radius  int
		wantErr bool
	}{
		{"valid_morgan", FingerprintMorgan, validBits, 2048, 2, false},
		{"valid_maccs", FingerprintMACCS, make([]byte, 21), 166, 0, false},
		{"invalid_type", FingerprintGNN, validBits, 2048, 0, true},
		{"empty_bits", FingerprintMorgan, nil, 2048, 2, true},
		{"insufficient_bits", FingerprintMorgan, make([]byte, 10), 2048, 2, true},
		{"zero_numbits", FingerprintMorgan, validBits, 0, 2, true},
		{"invalid_radius_morgan", FingerprintMorgan, validBits, 2048, 0, true},
		{"invalid_radius_maccs", FingerprintMACCS, make([]byte, 21), 166, 2, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewBitFingerprint(tt.fpType, tt.bits, tt.numBits, tt.radius)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewBitFingerprint() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNewDenseFingerprint(t *testing.T) {
	t.Parallel()
	validVec := make([]float32, 128)

	tests := []struct {
		name         string
		vector       []float32
		modelVersion string
		wantErr      bool
	}{
		{"valid", validVec, "v1", false},
		{"empty_vector", nil, "v1", true},
		{"too_small", make([]float32, 16), "v1", true},
		{"too_large", make([]float32, 5000), "v1", true},
		{"missing_version", validVec, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewDenseFingerprint(tt.vector, tt.modelVersion)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewDenseFingerprint() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestFingerprint_BitOperations(t *testing.T) {
	t.Parallel()

	// Create a fingerprint with known bits
	// 00001111 (0x0F)
	bits := []byte{0x0F}
	fp, _ := NewBitFingerprint(FingerprintMorgan, bits, 8, 2)

	if fp.BitCount() != 4 {
		t.Errorf("BitCount() = %d, want 4", fp.BitCount())
	}
	if math.Abs(fp.Density()-0.5) > testEpsilon {
		t.Errorf("Density() = %f, want 0.5", fp.Density())
	}

	// GetBit
	// Index 0 is LSB of byte 0? Usually bit 0 is LSB.
	// 0x0F = 00001111 binary. Bits 0,1,2,3 are set.
	// implementation: (bits[byteIndex] & (1 << bitIndex)) != 0
	// 1<<0 = 1 (bit 0). 0x0F & 1 = 1. True.
	// 1<<4 = 16. 0x0F & 16 = 0. False.
	for i := 0; i < 4; i++ {
		val, err := fp.GetBit(i)
		if err != nil || !val {
			t.Errorf("GetBit(%d) = %v, %v, want true, nil", i, val, err)
		}
	}
	for i := 4; i < 8; i++ {
		val, err := fp.GetBit(i)
		if err != nil || val {
			t.Errorf("GetBit(%d) = %v, %v, want false, nil", i, val, err)
		}
	}

	// Out of range
	_, err := fp.GetBit(8)
	if err == nil {
		t.Error("GetBit(8) succeeded on 8-bit fingerprint")
	}
}

func TestPopCount(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input []byte
		want  int
	}{
		{"nil", nil, 0},
		{"empty", []byte{}, 0},
		{"zeros", []byte{0, 0, 0}, 0},
		{"ones", []byte{0xFF, 0xFF}, 16},
		{"mixed", []byte{0x0F, 0xF0}, 8},
		{"alternating", []byte{0xAA}, 4}, // 10101010
	}
	for _, tt := range tests {
		if got := PopCount(tt.input); got != tt.want {
			t.Errorf("PopCount(%v) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestBitAnd(t *testing.T) {
	t.Parallel()
	a := []byte{0x0F, 0xFF}
	b := []byte{0xF0, 0x0F}
	want := []byte{0x00, 0x0F}

	got, err := BitAnd(a, b)
	if err != nil {
		t.Fatalf("BitAnd failed: %v", err)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("BitAnd byte %d = %x, want %x", i, got[i], want[i])
		}
	}

	// Mismatch length
	_, err = BitAnd(a, []byte{0x00})
	if err == nil {
		t.Error("BitAnd mismatch length succeeded")
	}
}

func TestWeightedAverageFusion(t *testing.T) {
	t.Parallel()
	strategy := &WeightedAverageFusion{}

	scores := map[FingerprintType]float64{
		FingerprintMorgan: 0.8,
		FingerprintMACCS:  0.6,
	}

	// Equal weights (nil map)
	got, err := strategy.Fuse(scores, nil)
	if err != nil {
		t.Fatalf("Fuse failed: %v", err)
	}
	if math.Abs(got-0.7) > testEpsilon {
		t.Errorf("Fuse(equal) = %f, want 0.7", got)
	}

	// Custom weights
	weights := map[FingerprintType]float64{
		FingerprintMorgan: 2.0,
		FingerprintMACCS:  1.0,
	}
	got, _ = strategy.Fuse(scores, weights)
	// (0.8*2 + 0.6*1) / 3 = 2.2 / 3 = 0.7333...
	if math.Abs(got-0.7333333333) > testEpsilon {
		t.Errorf("Fuse(weighted) = %f, want ~0.733", got)
	}
}

// Benchmarks

func BenchmarkPopCount_2048(b *testing.B) {
	data := make([]byte, 256)
	for i := range data {
		data[i] = 0xAA // Alternating bits
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		PopCount(data)
	}
}

func BenchmarkBitAnd_2048(b *testing.B) {
	data1 := make([]byte, 256)
	data2 := make([]byte, 256)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		BitAnd(data1, data2)
	}
}

//Personal.AI order the ending

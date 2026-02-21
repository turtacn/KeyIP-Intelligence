package molecule

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFingerprintType_IsValid(t *testing.T) {
	tests := []struct {
		ft   FingerprintType
		want bool
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
		t.Run(string(tt.ft), func(t *testing.T) {
			if got := tt.ft.IsValid(); got != tt.want {
				t.Errorf("FingerprintType.IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFingerprintType_String(t *testing.T) {
	assert.Equal(t, "morgan", FingerprintMorgan.String())
}

func TestParseFingerprintType(t *testing.T) {
	tests := []struct {
		s       string
		want    FingerprintType
		wantErr bool
	}{
		{"morgan", FingerprintMorgan, false},
		{"maccs", FingerprintMACCS, false},
		{"invalid", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.s, func(t *testing.T) {
			got, err := ParseFingerprintType(tt.s)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseFingerprintType() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParseFingerprintType() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAllFingerprintTypes(t *testing.T) {
	types := AllFingerprintTypes()
	assert.Len(t, types, 6)
	assert.Contains(t, types, FingerprintMorgan)
}

func TestNewBitFingerprint(t *testing.T) {
	tests := []struct {
		name    string
		fpType  FingerprintType
		bits    []byte
		numBits int
		radius  int
		wantErr bool
	}{
		{"valid_maccs", FingerprintMACCS, make([]byte, 21), 166, 0, false},
		{"valid_morgan", FingerprintMorgan, make([]byte, 256), 2048, 2, false},
		{"invalid_type", FingerprintGNN, make([]byte, 256), 2048, 2, true},
		{"empty_bits", FingerprintMorgan, nil, 2048, 2, true},
		{"insufficient_bits", FingerprintMorgan, make([]byte, 10), 2048, 2, true},
		{"invalid_maccs_bits", FingerprintMACCS, make([]byte, 21), 167, 0, true},
		{"invalid_radius", FingerprintMorgan, make([]byte, 256), 2048, 7, true},
		{"nonzero_radius_maccs", FingerprintMACCS, make([]byte, 21), 166, 2, true},
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
	tests := []struct {
		name         string
		vector       []float32
		modelVersion string
		wantErr      bool
	}{
		{"valid", make([]float32, 128), "v1", false},
		{"too_small", make([]float32, 31), "v1", true},
		{"too_large", make([]float32, 4097), "v1", true},
		{"empty_version", make([]float32, 128), "", true},
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

func TestFingerprint_BitOps(t *testing.T) {
	bits := []byte{0x01, 0x80} // bits 0 and 15 set
	fp, _ := NewBitFingerprint(FingerprintMorgan, bits, 16, 2)

	assert.True(t, fp.IsBitVector())
	assert.False(t, fp.IsDenseVector())
	assert.Equal(t, 2, fp.BitCount())
	assert.Equal(t, 2.0/16.0, fp.Density())

	val, _ := fp.GetBit(0)
	assert.True(t, val)
	val, _ = fp.GetBit(15)
	assert.True(t, val)
	val, _ = fp.GetBit(1)
	assert.False(t, val)

	_, err := fp.GetBit(16)
	assert.Error(t, err)
}

func TestFingerprint_ToFloat32Slice(t *testing.T) {
	bits := []byte{0x01} // bit 0 set
	fp, _ := NewBitFingerprint(FingerprintMorgan, bits, 8, 2)
	slice := fp.ToFloat32Slice()
	assert.Equal(t, []float32{1.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0}, slice)

	vec := []float32{0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 1.0, 0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 1.0, 0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 1.0, 0.1, 0.2}
	fp2, _ := NewDenseFingerprint(vec, "v1")
	assert.Equal(t, vec, fp2.ToFloat32Slice())
}

func TestPopCount(t *testing.T) {
	tests := []struct {
		data []byte
		want int
	}{
		{nil, 0},
		{[]byte{}, 0},
		{[]byte{0x00}, 0},
		{[]byte{0x01}, 1},
		{[]byte{0xFF}, 8},
		{[]byte{0xAA, 0x55}, 8},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, PopCount(tt.data))
	}
}

func TestBitAndOr(t *testing.T) {
	a := []byte{0x0F, 0xAA}
	b := []byte{0xF0, 0x55}

	and, _ := BitAnd(a, b)
	assert.Equal(t, []byte{0x00, 0x00}, and)

	or, _ := BitOr(a, b)
	assert.Equal(t, []byte{0xFF, 0xFF}, or)

	_, err := BitAnd(a, []byte{0x01})
	assert.Error(t, err)
}

func TestFusionStrategies(t *testing.T) {
	scores := map[FingerprintType]float64{
		FingerprintMorgan: 0.8,
		FingerprintMACCS:  0.6,
	}

	t.Run("WeightedAverage", func(t *testing.T) {
		strategy := &WeightedAverageFusion{}

		// Equal weights
		got, _ := strategy.Fuse(scores, nil)
		assert.InDelta(t, 0.7, got, 1e-9)

		// Custom weights
		weights := map[FingerprintType]float64{
			FingerprintMorgan: 2.0,
			FingerprintMACCS:  1.0,
		}
		got, _ = strategy.Fuse(scores, weights)
		assert.InDelta(t, (0.8*2+0.6*1)/3.0, got, 1e-9)
	})

	t.Run("Max", func(t *testing.T) {
		strategy := &MaxFusion{}
		got, _ := strategy.Fuse(scores, nil)
		assert.Equal(t, 0.8, got)
	})

	t.Run("Min", func(t *testing.T) {
		strategy := &MinFusion{}
		got, _ := strategy.Fuse(scores, nil)
		assert.Equal(t, 0.6, got)
	})
}

func TestFingerprintCalcOptions_Validate(t *testing.T) {
	opts := DefaultFingerprintCalcOptions()
	assert.NoError(t, opts.Validate())

	opts.Radius = 0
	assert.Error(t, opts.Validate())

	opts.Radius = 2
	opts.NumBits = 100
	assert.Error(t, opts.Validate())
}

func BenchmarkPopCount_256Bytes(b *testing.B) {
	data := make([]byte, 256)
	for i := range data {
		data[i] = 0xAA
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		PopCount(data)
	}
}

func BenchmarkBitAnd_256Bytes(b *testing.B) {
	a := make([]byte, 256)
	b_data := make([]byte, 256)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = BitAnd(a, b_data)
	}
}

//Personal.AI order the ending

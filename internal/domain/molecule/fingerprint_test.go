package molecule

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const testEpsilon = 1e-9

func TestFingerprintType_IsValid(t *testing.T) {
	assert.True(t, FingerprintMACCS.IsValid())
	assert.True(t, FingerprintMorgan.IsValid())
	assert.True(t, FingerprintRDKit.IsValid())
	assert.True(t, FingerprintAtomPair.IsValid())
	assert.True(t, FingerprintFCFP.IsValid())
	assert.True(t, FingerprintGNN.IsValid())
	assert.False(t, FingerprintType("invalid").IsValid())
}

func TestFingerprintType_String(t *testing.T) {
	assert.Equal(t, "maccs", FingerprintMACCS.String())
}

func TestParseFingerprintType(t *testing.T) {
	ft, err := ParseFingerprintType("morgan")
	assert.NoError(t, err)
	assert.Equal(t, FingerprintMorgan, ft)

	_, err = ParseFingerprintType("invalid")
	assert.Error(t, err)
}

func TestAllFingerprintTypes(t *testing.T) {
	types := AllFingerprintTypes()
	assert.Contains(t, types, FingerprintMACCS)
	assert.Contains(t, types, FingerprintMorgan)
	assert.Contains(t, types, FingerprintRDKit)
	assert.Contains(t, types, FingerprintAtomPair)
	assert.Contains(t, types, FingerprintFCFP)
	assert.Contains(t, types, FingerprintGNN)
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
		{
			name:    "valid_maccs",
			fpType:  FingerprintMACCS,
			bits:    make([]byte, 21), // (166+7)/8 = 21.6 -> 21 bytes? No, 166 bits needs 21 bytes (21*8=168 > 166)
			numBits: 166,
			radius:  0,
			wantErr: false,
		},
		{
			name:    "valid_morgan_2048",
			fpType:  FingerprintMorgan,
			bits:    make([]byte, 256), // 2048/8 = 256
			numBits: 2048,
			radius:  2,
			wantErr: false,
		},
		{
			name:    "valid_morgan_1024",
			fpType:  FingerprintMorgan,
			bits:    make([]byte, 128),
			numBits: 1024,
			radius:  3,
			wantErr: false,
		},
		{
			name:    "empty_bits",
			fpType:  FingerprintMorgan,
			bits:    []byte{},
			numBits: 2048,
			radius:  2,
			wantErr: true,
		},
		{
			name:    "nil_bits",
			fpType:  FingerprintMorgan,
			bits:    nil,
			numBits: 2048,
			radius:  2,
			wantErr: true,
		},
		{
			name:    "wrong_maccs_bits",
			fpType:  FingerprintMACCS,
			bits:    make([]byte, 21),
			numBits: 2048,
			radius:  0,
			wantErr: true, // MACCS must be 166
		},
		{
			name:    "invalid_radius_for_morgan_zero",
			fpType:  FingerprintMorgan,
			bits:    make([]byte, 256),
			numBits: 2048,
			radius:  0,
			wantErr: true,
		},
		{
			name:    "invalid_radius_for_morgan_large",
			fpType:  FingerprintMorgan,
			bits:    make([]byte, 256),
			numBits: 2048,
			radius:  7,
			wantErr: true,
		},
		{
			name:    "nonzero_radius_for_maccs",
			fpType:  FingerprintMACCS,
			bits:    make([]byte, 21),
			numBits: 166,
			radius:  2,
			wantErr: true,
		},
		{
			name:    "gnn_type_rejected",
			fpType:  FingerprintGNN,
			bits:    make([]byte, 32),
			numBits: 256,
			radius:  0,
			wantErr: true,
		},
		{
			name:    "insufficient_bytes",
			fpType:  FingerprintMorgan,
			bits:    make([]byte, 100),
			numBits: 2048, // Needs 256 bytes
			radius:  2,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fp, err := NewBitFingerprint(tt.fpType, tt.bits, tt.numBits, tt.radius)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, fp)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, fp)
				assert.Equal(t, EncodingBitVector, fp.Encoding)
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
		{
			name:         "valid_128d",
			vector:       make([]float32, 128),
			modelVersion: "v1",
			wantErr:      false,
		},
		{
			name:         "valid_256d",
			vector:       make([]float32, 256),
			modelVersion: "v1",
			wantErr:      false,
		},
		{
			name:         "empty_vector",
			vector:       []float32{},
			modelVersion: "v1",
			wantErr:      true,
		},
		{
			name:         "nil_vector",
			vector:       nil,
			modelVersion: "v1",
			wantErr:      true,
		},
		{
			name:         "too_small",
			vector:       make([]float32, 16),
			modelVersion: "v1",
			wantErr:      true,
		},
		{
			name:         "too_large",
			vector:       make([]float32, 5000),
			modelVersion: "v1",
			wantErr:      true,
		},
		{
			name:         "empty_model_version",
			vector:       make([]float32, 128),
			modelVersion: "",
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fp, err := NewDenseFingerprint(tt.vector, tt.modelVersion)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, fp)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, fp)
				assert.Equal(t, EncodingDenseVector, fp.Encoding)
				assert.Equal(t, FingerprintGNN, fp.Type)
			}
		})
	}
}

func TestNewCountFingerprint(t *testing.T) {
	fp, err := NewCountFingerprint(FingerprintMorgan, []byte{1, 2, 3}, 2048, 2)
	assert.NoError(t, err)
	assert.NotNil(t, fp)
	assert.Equal(t, EncodingCountVector, fp.Encoding)

	// Invalid type
	_, err = NewCountFingerprint(FingerprintMACCS, []byte{1}, 166, 0)
	assert.Error(t, err)
}

func TestFingerprint_BitOperations(t *testing.T) {
	bits := []byte{0xFF, 0x00, 0xAA} // 11111111 00000000 10101010
	numBits := 24
	fp, _ := NewBitFingerprint(FingerprintMorgan, bits, numBits, 2)

	assert.Equal(t, 8+0+4, fp.BitCount())
	assert.InDelta(t, 0.5, fp.Density(), testEpsilon) // 12/24 = 0.5

	bit, err := fp.GetBit(0)
	assert.NoError(t, err)
	assert.True(t, bit)

	bit, err = fp.GetBit(8) // Second byte first bit (index 8) -> 0x00 -> false
	assert.NoError(t, err)
	assert.False(t, bit)

	// Out of bounds
	_, err = fp.GetBit(100)
	assert.Error(t, err)

	_, err = fp.GetBit(-1)
	assert.Error(t, err)
}

func TestFingerprint_IsTypes(t *testing.T) {
	bitFP, _ := NewBitFingerprint(FingerprintMorgan, []byte{1}, 8, 2)
	assert.True(t, bitFP.IsBitVector())
	assert.False(t, bitFP.IsDenseVector())
	assert.False(t, bitFP.IsCountVector())

	denseFP, _ := NewDenseFingerprint(make([]float32, 32), "v1")
	assert.False(t, denseFP.IsBitVector())
	assert.True(t, denseFP.IsDenseVector())

	countFP, _ := NewCountFingerprint(FingerprintMorgan, []byte{1}, 8, 2)
	assert.True(t, countFP.IsCountVector())
}

func TestFingerprint_ToFloat32Slice(t *testing.T) {
	// BitVector
	bits := []byte{0x03} // 00000011 -> bit 0 and 1 set
	fp, _ := NewBitFingerprint(FingerprintMorgan, bits, 8, 2)
	floats := fp.ToFloat32Slice()
	assert.Len(t, floats, 8)
	assert.Equal(t, float32(1.0), floats[0])
	assert.Equal(t, float32(1.0), floats[1])
	assert.Equal(t, float32(0.0), floats[2])

	// DenseVector
	// Need valid dense fingerprint (min 32 dim)
	longVec := make([]float32, 32)
	longVec[0] = 0.1
	fpDense, _ := NewDenseFingerprint(longVec, "v1")
	floatsDense := fpDense.ToFloat32Slice()
	assert.Equal(t, longVec, floatsDense)
	// Check deep copy
	floatsDense[0] = 9.9
	assert.Equal(t, float32(0.1), fpDense.Vector[0])
}

func TestPopCount(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want int
	}{
		{"nil", nil, 0},
		{"empty", []byte{}, 0},
		{"all_zeros", []byte{0x00, 0x00}, 0},
		{"all_ones", []byte{0xFF, 0xFF}, 16},
		{"single_0x0F", []byte{0x0F}, 4},
		{"mixed", []byte{0x0F, 0xF0, 0xFF}, 4 + 4 + 8},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, PopCount(tt.data))
		})
	}
}

func TestBitAnd(t *testing.T) {
	tests := []struct {
		name    string
		a, b    []byte
		want    []byte
		wantErr bool
	}{
		{
			name: "equal",
			a:    []byte{0xFF, 0x0F},
			b:    []byte{0x0F, 0xFF},
			want: []byte{0x0F, 0x0F},
		},
		{
			name: "unequal_length",
			a:    []byte{0xFF},
			b:    []byte{0xFF, 0xFF},
			wantErr: true,
		},
		{
			name: "nil_a",
			a:    nil,
			b:    []byte{0xFF},
			wantErr: true,
		},
		{
			name: "empty",
			a:    []byte{},
			b:    []byte{},
			want: []byte{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BitAnd(tt.a, tt.b)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestBitOr(t *testing.T) {
	a := []byte{0x0F, 0x00}
	b := []byte{0x00, 0xF0}
	got, err := BitOr(a, b)
	assert.NoError(t, err)
	assert.Equal(t, []byte{0x0F, 0xF0}, got)

	_, err = BitOr([]byte{1}, []byte{1, 2})
	assert.Error(t, err)
}

func TestWeightedAverageFusion(t *testing.T) {
	fusion := &WeightedAverageFusion{}

	scores := map[FingerprintType]float64{
		FingerprintMorgan: 0.8,
		FingerprintMACCS:  0.6,
	}

	// Equal weights implicitly
	res, err := fusion.Fuse(scores, nil)
	assert.NoError(t, err)
	assert.InDelta(t, 0.7, res, testEpsilon)

	// Explicit weights
	weights := map[FingerprintType]float64{
		FingerprintMorgan: 2.0,
		FingerprintMACCS:  1.0,
	}
	res, err = fusion.Fuse(scores, weights)
	assert.NoError(t, err)
	// (0.8*2 + 0.6*1) / 3 = 2.2 / 3 = 0.7333
	assert.InDelta(t, 2.2/3.0, res, testEpsilon)

	// Empty scores
	_, err = fusion.Fuse(map[FingerprintType]float64{}, nil)
	assert.Error(t, err)

	// Clamp
	scoresClamp := map[FingerprintType]float64{FingerprintMorgan: 1.5}
	res, _ = fusion.Fuse(scoresClamp, nil)
	assert.Equal(t, 1.0, res)
}

func TestMaxFusion(t *testing.T) {
	fusion := &MaxFusion{}
	scores := map[FingerprintType]float64{
		FingerprintMorgan: 0.8,
		FingerprintMACCS:  0.9,
	}
	res, err := fusion.Fuse(scores, nil)
	assert.NoError(t, err)
	assert.Equal(t, 0.9, res)

	_, err = fusion.Fuse(map[FingerprintType]float64{}, nil)
	assert.Error(t, err)
}

func TestMinFusion(t *testing.T) {
	fusion := &MinFusion{}
	scores := map[FingerprintType]float64{
		FingerprintMorgan: 0.8,
		FingerprintMACCS:  0.6,
	}
	res, err := fusion.Fuse(scores, nil)
	assert.NoError(t, err)
	assert.Equal(t, 0.6, res)

	_, err = fusion.Fuse(map[FingerprintType]float64{}, nil)
	assert.Error(t, err)
}

func TestDefaultFingerprintCalcOptions(t *testing.T) {
	opts := DefaultFingerprintCalcOptions()
	assert.Equal(t, 2, opts.Radius)
	assert.Equal(t, 2048, opts.NumBits)

	assert.NoError(t, opts.Validate())
}

func TestFingerprintCalcOptions_Validate(t *testing.T) {
	opts := &FingerprintCalcOptions{NumBits: -1}
	assert.Error(t, opts.Validate())

	opts = &FingerprintCalcOptions{NumBits: 2048, Radius: -1}
	assert.Error(t, opts.Validate())
}

func BenchmarkPopCount_256Bytes(b *testing.B) {
	data := make([]byte, 256)
	for i := range data {
		data[i] = 0xFF
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		PopCount(data)
	}
}

func BenchmarkBitAnd_256Bytes(b *testing.B) {
	data1 := make([]byte, 256)
	data2 := make([]byte, 256)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = BitAnd(data1, data2)
	}
}

//Personal.AI order the ending

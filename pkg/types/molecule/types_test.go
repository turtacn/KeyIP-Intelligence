package molecule

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
)

func TestMoleculeFormat_IsValid_AllFormats(t *testing.T) {
	formats := []MoleculeFormat{FormatSMILES, FormatInChI, FormatMolfile, FormatSDF, FormatCDXML}
	for _, f := range formats {
		assert.True(t, f.IsValid())
	}
}

func TestMoleculeFormat_IsValid_Unknown(t *testing.T) {
	assert.False(t, MoleculeFormat("unknown").IsValid())
}

func TestFingerprintType_IsValid_AllTypes(t *testing.T) {
	types := []FingerprintType{FingerprintMorgan, FingerprintMACCS, FingerprintRDKit, FingerprintAtomPair, FingerprintFCFP, FingerprintGNN}
	for _, tt := range types {
		assert.True(t, tt.IsValid())
	}
}

func TestFingerprintType_IsValid_Unknown(t *testing.T) {
	assert.False(t, FingerprintType("unknown").IsValid())
}

func TestMoleculeInput_Validate_ValidSMILES(t *testing.T) {
	input := MoleculeInput{Format: FormatSMILES, Value: "CCO"}
	assert.NoError(t, input.Validate())
}

func TestMoleculeInput_Validate_EmptyValue(t *testing.T) {
	input := MoleculeInput{Format: FormatSMILES, Value: ""}
	assert.Error(t, input.Validate())
}

func TestMoleculeInput_Validate_InvalidFormat(t *testing.T) {
	input := MoleculeInput{Format: "invalid", Value: "CCO"}
	assert.Error(t, input.Validate())
}

func TestSimilaritySearchRequest_Validate_Valid(t *testing.T) {
	req := SimilaritySearchRequest{
		Molecule:         MoleculeInput{Format: FormatSMILES, Value: "CCO"},
		FingerprintTypes: []FingerprintType{FingerprintMorgan},
		Threshold:        0.7,
		MaxResults:       50,
	}
	assert.NoError(t, req.Validate())
}

func TestSimilaritySearchRequest_Validate_ThresholdTooLow(t *testing.T) {
	req := SimilaritySearchRequest{
		Molecule:         MoleculeInput{Format: FormatSMILES, Value: "CCO"},
		FingerprintTypes: []FingerprintType{FingerprintMorgan},
		Threshold:        -0.1,
		MaxResults:       50,
	}
	assert.Error(t, req.Validate())
}

func TestSimilaritySearchRequest_Validate_ThresholdTooHigh(t *testing.T) {
	req := SimilaritySearchRequest{
		Molecule:         MoleculeInput{Format: FormatSMILES, Value: "CCO"},
		FingerprintTypes: []FingerprintType{FingerprintMorgan},
		Threshold:        1.1,
		MaxResults:       50,
	}
	assert.Error(t, req.Validate())
}

func TestSimilaritySearchRequest_Validate_MaxResultsZero(t *testing.T) {
	req := SimilaritySearchRequest{
		Molecule:         MoleculeInput{Format: FormatSMILES, Value: "CCO"},
		FingerprintTypes: []FingerprintType{FingerprintMorgan},
		Threshold:        0.7,
		MaxResults:       0,
	}
	assert.Error(t, req.Validate())
}

func TestSimilaritySearchRequest_Validate_MaxResultsExceedsLimit(t *testing.T) {
	req := SimilaritySearchRequest{
		Molecule:         MoleculeInput{Format: FormatSMILES, Value: "CCO"},
		FingerprintTypes: []FingerprintType{FingerprintMorgan},
		Threshold:        0.7,
		MaxResults:       501,
	}
	assert.Error(t, req.Validate())
}

func TestSimilaritySearchRequest_Validate_NoFingerprintTypes(t *testing.T) {
	req := SimilaritySearchRequest{
		Molecule:         MoleculeInput{Format: FormatSMILES, Value: "CCO"},
		FingerprintTypes: []FingerprintType{},
		Threshold:        0.7,
		MaxResults:       50,
	}
	assert.Error(t, req.Validate())
}

func TestSimilaritySearchRequest_Validate_InvalidFingerprintType(t *testing.T) {
	req := SimilaritySearchRequest{
		Molecule:         MoleculeInput{Format: FormatSMILES, Value: "CCO"},
		FingerprintTypes: []FingerprintType{"invalid"},
		Threshold:        0.7,
		MaxResults:       50,
	}
	assert.Error(t, req.Validate())
}

func TestMoleculeDTO_JSONRoundTrip(t *testing.T) {
	dto := MoleculeDTO{
		ID:        common.ID("550e8400-e29b-41d4-a716-446655440000"),
		CreatedAt: common.NewTimestamp(),
		SMILES:    "CCO",
	}
	data, err := json.Marshal(dto)
	assert.NoError(t, err)

	var dto2 MoleculeDTO
	err = json.Unmarshal(data, &dto2)
	assert.NoError(t, err)
	assert.Equal(t, dto.ID, dto2.ID)
	assert.Equal(t, dto.SMILES, dto2.SMILES)
	// Compare UnixMilli to avoid monotonic clock issues
	assert.Equal(t, time.Time(dto.CreatedAt).UnixMilli(), time.Time(dto2.CreatedAt).UnixMilli())
}

func TestSimilarityResult_JSONRoundTrip(t *testing.T) {
	res := SimilarityResult{
		TargetMolecule: MoleculeDTO{SMILES: "CCO"},
		Scores:         map[FingerprintType]float64{FingerprintMorgan: 0.9},
		WeightedScore:  0.9,
	}
	data, err := json.Marshal(res)
	assert.NoError(t, err)

	var res2 SimilarityResult
	err = json.Unmarshal(data, &res2)
	assert.NoError(t, err)
	assert.Equal(t, res.WeightedScore, res2.WeightedScore)
	assert.Equal(t, res.Scores[FingerprintMorgan], res2.Scores[FingerprintMorgan])
}

func TestStructuralDiff_JSONSerialization(t *testing.T) {
	diff := StructuralDiff{
		CommonScaffold: "c1ccccc1",
		Differences: []SubstituentDiff{
			{Position: "1", QueryGroup: "C", TargetGroup: "O", Significance: SignificanceHigh},
		},
	}
	data, err := json.Marshal(diff)
	assert.NoError(t, err)
	assert.Contains(t, string(data), "common_scaffold")
	assert.Contains(t, string(data), "high")
}

func TestMaterialProperty_JSONSerialization(t *testing.T) {
	prop := MaterialProperty{
		PropertyType: "homo",
		Value:        -5.5,
		Unit:         "eV",
	}
	data, err := json.Marshal(prop)
	assert.NoError(t, err)
	assert.Contains(t, string(data), "property_type")
}

func TestDefaultWeights_SumToOne(t *testing.T) {
	sum := DefaultMorganWeight + DefaultRDKitWeight + DefaultAtomPairWeight + DefaultGNNWeight
	assert.InDelta(t, 1.0, sum, 0.0001)
}

func TestMoleculeSource_Values(t *testing.T) {
	assert.Equal(t, "patent", SourcePatent)
	assert.Equal(t, "experiment", SourceExperiment)
	assert.Equal(t, "literature", SourceLiterature)
	assert.Equal(t, "user_input", SourceUserInput)
}

func TestSignificanceLevel_Values(t *testing.T) {
	assert.Equal(t, "low", SignificanceLow)
	assert.Equal(t, "moderate", SignificanceModerate)
	assert.Equal(t, "high", SignificanceHigh)
}

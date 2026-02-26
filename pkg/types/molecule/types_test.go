package molecule

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
)

func TestMoleculeFormat_IsValid_AllFormats(t *testing.T) {
	formats := []MoleculeFormat{
		FormatSMILES, FormatInChI, FormatMolfile, FormatSDF, FormatCDXML,
	}
	for _, f := range formats {
		assert.True(t, f.IsValid())
	}
}

func TestMoleculeFormat_IsValid_Unknown(t *testing.T) {
	assert.False(t, MoleculeFormat("unknown").IsValid())
}

func TestFingerprintType_IsValid_AllTypes(t *testing.T) {
	types := []FingerprintType{
		FingerprintMorgan, FingerprintMACCS, FingerprintRDKit, FingerprintAtomPair, FingerprintFCFP, FingerprintGNN,
	}
	for _, tp := range types {
		assert.True(t, tp.IsValid())
	}
}

func TestFingerprintType_IsValid_Unknown(t *testing.T) {
	assert.False(t, FingerprintType("unknown").IsValid())
}

func TestMoleculeInput_Validate_ValidSMILES(t *testing.T) {
	input := MoleculeInput{
		Format: FormatSMILES,
		Value:  "c1ccccc1",
	}
	assert.NoError(t, input.Validate())
}

func TestMoleculeInput_Validate_EmptyValue(t *testing.T) {
	input := MoleculeInput{
		Format: FormatSMILES,
		Value:  "",
	}
	assert.Error(t, input.Validate())
}

func TestMoleculeInput_Validate_InvalidFormat(t *testing.T) {
	input := MoleculeInput{
		Format: "invalid",
		Value:  "c1ccccc1",
	}
	assert.Error(t, input.Validate())
}

func TestSimilaritySearchRequest_Validate_Valid(t *testing.T) {
	req := SimilaritySearchRequest{
		Molecule: MoleculeInput{Format: FormatSMILES, Value: "C"},
		FingerprintTypes: []FingerprintType{FingerprintMorgan},
		Threshold: 0.7,
		MaxResults: 10,
	}
	assert.NoError(t, req.Validate())
}

func TestSimilaritySearchRequest_Validate_ThresholdTooLow(t *testing.T) {
	req := SimilaritySearchRequest{
		Molecule: MoleculeInput{Format: FormatSMILES, Value: "C"},
		FingerprintTypes: []FingerprintType{FingerprintMorgan},
		Threshold: -0.1,
		MaxResults: 10,
	}
	assert.Error(t, req.Validate())
}

func TestSimilaritySearchRequest_Validate_ThresholdTooHigh(t *testing.T) {
	req := SimilaritySearchRequest{
		Molecule: MoleculeInput{Format: FormatSMILES, Value: "C"},
		FingerprintTypes: []FingerprintType{FingerprintMorgan},
		Threshold: 1.1,
		MaxResults: 10,
	}
	assert.Error(t, req.Validate())
}

func TestSimilaritySearchRequest_Validate_MaxResultsZero(t *testing.T) {
	req := SimilaritySearchRequest{
		Molecule: MoleculeInput{Format: FormatSMILES, Value: "C"},
		FingerprintTypes: []FingerprintType{FingerprintMorgan},
		Threshold: 0.5,
		MaxResults: 0,
	}
	assert.Error(t, req.Validate())
}

func TestSimilaritySearchRequest_Validate_MaxResultsExceedsLimit(t *testing.T) {
	req := SimilaritySearchRequest{
		Molecule: MoleculeInput{Format: FormatSMILES, Value: "C"},
		FingerprintTypes: []FingerprintType{FingerprintMorgan},
		Threshold: 0.5,
		MaxResults: 501,
	}
	assert.Error(t, req.Validate())
}

func TestSimilaritySearchRequest_Validate_NoFingerprintTypes(t *testing.T) {
	req := SimilaritySearchRequest{
		Molecule: MoleculeInput{Format: FormatSMILES, Value: "C"},
		FingerprintTypes: []FingerprintType{},
		Threshold: 0.5,
		MaxResults: 10,
	}
	assert.Error(t, req.Validate())
}

func TestSimilaritySearchRequest_Validate_InvalidFingerprintType(t *testing.T) {
	req := SimilaritySearchRequest{
		Molecule: MoleculeInput{Format: FormatSMILES, Value: "C"},
		FingerprintTypes: []FingerprintType{"invalid"},
		Threshold: 0.5,
		MaxResults: 10,
	}
	assert.Error(t, req.Validate())
}

func TestMoleculeDTO_JSONRoundTrip(t *testing.T) {
	dto := MoleculeDTO{
		ID:     common.NewID(),
		SMILES: "C",
	}
	bytes, err := json.Marshal(dto)
	assert.NoError(t, err)
	var decoded MoleculeDTO
	err = json.Unmarshal(bytes, &decoded)
	assert.NoError(t, err)
	assert.Equal(t, dto.ID, decoded.ID)
	assert.Equal(t, dto.SMILES, decoded.SMILES)
}

func TestSimilarityResult_JSONRoundTrip(t *testing.T) {
	res := SimilarityResult{
		Scores: map[FingerprintType]float64{
			FingerprintMorgan: 0.8,
		},
		WeightedScore: 0.8,
	}
	bytes, err := json.Marshal(res)
	assert.NoError(t, err)
	var decoded SimilarityResult
	err = json.Unmarshal(bytes, &decoded)
	assert.NoError(t, err)
	assert.Equal(t, res.WeightedScore, decoded.WeightedScore)
	assert.Equal(t, 0.8, decoded.Scores[FingerprintMorgan])
}

func TestStructuralDiff_JSONSerialization(t *testing.T) {
	diff := StructuralDiff{
		CommonScaffold: "C1CCCCC1",
		Differences: []SubstituentDiff{
			{Position: "1", QueryGroup: "OH", TargetGroup: "H", Significance: "high"},
		},
	}
	bytes, err := json.Marshal(diff)
	assert.NoError(t, err)
	assert.Contains(t, string(bytes), "common_scaffold")
	assert.Contains(t, string(bytes), "differences")
}

func TestMaterialProperty_JSONSerialization(t *testing.T) {
	prop := MaterialProperty{
		PropertyType: "emission_wavelength",
		Value:        450.5,
		Unit:         "nm",
	}
	bytes, err := json.Marshal(prop)
	assert.NoError(t, err)
	assert.Contains(t, string(bytes), "emission_wavelength")
	assert.Contains(t, string(bytes), "450.5")
}

func TestDefaultWeights_SumToOne(t *testing.T) {
	sum := DefaultMorganWeight + DefaultRDKitWeight + DefaultAtomPairWeight + DefaultGNNWeight
	assert.Equal(t, 1.0, sum)
}

func TestMoleculeSource_Values(t *testing.T) {
	assert.Equal(t, MoleculeSource("patent"), SourcePatent)
	assert.Equal(t, MoleculeSource("experiment"), SourceExperiment)
	assert.Equal(t, MoleculeSource("literature"), SourceLiterature)
	assert.Equal(t, MoleculeSource("user_input"), SourceUserInput)
}

func TestSignificanceLevel_Values(t *testing.T) {
	assert.Equal(t, SignificanceLevel("low"), SignificanceLow)
	assert.Equal(t, SignificanceLevel("moderate"), SignificanceModerate)
	assert.Equal(t, SignificanceLevel("high"), SignificanceHigh)
}

//Personal.AI order the ending

package infringe_net

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/turtacn/KeyIP-Intelligence/internal/intelligence/common"
)

// MockInfringeModel
type MockInfringeModel struct {
	mock.Mock
}

func (m *MockInfringeModel) PredictLiteralInfringement(ctx context.Context, req *LiteralPredictionRequest) (*LiteralPredictionResult, error) {
	args := m.Called(ctx, req)
	return args.Get(0).(*LiteralPredictionResult), args.Error(1)
}

func (m *MockInfringeModel) ComputeStructuralSimilarity(ctx context.Context, smiles1, smiles2 string) (float64, error) {
	args := m.Called(ctx, smiles1, smiles2)
	return args.Get(0).(float64), args.Error(1)
}

func (m *MockInfringeModel) PredictPropertyImpact(ctx context.Context, req *PropertyImpactRequest) (*PropertyImpactResult, error) {
	args := m.Called(ctx, req)
	return args.Get(0).(*PropertyImpactResult), args.Error(1)
}

func (m *MockInfringeModel) EmbedStructure(ctx context.Context, smiles string) ([]float64, error) {
	args := m.Called(ctx, smiles)
	return args.Get(0).([]float64), args.Error(1)
}

func (m *MockInfringeModel) ModelInfo() *common.ModelMetadata {
	return nil
}

func (m *MockInfringeModel) Healthy(ctx context.Context) error {
	return nil
}

func TestNewEquivalentsAnalyzer_Success(t *testing.T) {
	model := new(MockInfringeModel)
	logger := new(MockLogger)
	analyzer, err := NewEquivalentsAnalyzer(model, logger)
	assert.NoError(t, err)
	assert.NotNil(t, analyzer)
}

func TestAnalyzeElement_AllPass(t *testing.T) {
	model := new(MockInfringeModel)
	logger := new(MockLogger)
	analyzer, _ := NewEquivalentsAnalyzer(model, logger)

	model.On("ComputeStructuralSimilarity", mock.Anything, "C1", "C2").Return(0.8, nil)

	query := &StructuralElement{ElementType: ElementTypeSubstituent, SMILESFragment: "C1"}
	claim := &StructuralElement{ElementType: ElementTypeSubstituent, SMILESFragment: "C2"}

	res, err := analyzer.AnalyzeElement(context.Background(), query, claim)
	assert.NoError(t, err)
	assert.True(t, res.IsEquivalent)
	assert.InDelta(t, 0.8, res.WayScore, 0.001)
}

func TestAnalyze_AllElementsEquivalent(t *testing.T) {
	model := new(MockInfringeModel)
	logger := new(MockLogger)
	analyzer, _ := NewEquivalentsAnalyzer(model, logger)

	model.On("ComputeStructuralSimilarity", mock.Anything, mock.Anything, mock.Anything).Return(0.9, nil)

	req := &EquivalentsRequest{
		QueryMolecule: []*StructuralElement{
			{ElementType: ElementTypeCoreScaffold, SMILESFragment: "C1", ElementID: "q1"},
		},
		ClaimElements: []*StructuralElement{
			{ElementType: ElementTypeCoreScaffold, SMILESFragment: "C2", ElementID: "c1"},
		},
	}

	res, err := analyzer.Analyze(context.Background(), req)
	assert.NoError(t, err)
	assert.InDelta(t, 1.0, float64(res.EquivalentElementCount)/float64(res.TotalElementCount), 0.001)
}

func TestAnalyze_ScaffoldWeightDominance(t *testing.T) {
	model := new(MockInfringeModel)
	logger := new(MockLogger)
	// Default scaffold weight is 2.0
	analyzer, _ := NewEquivalentsAnalyzer(model, logger)

	model.On("ComputeStructuralSimilarity", mock.Anything, "S1", "S2").Return(0.4, nil) // Low similarity for scaffold
	model.On("ComputeStructuralSimilarity", mock.Anything, "F1", "F2").Return(0.9, nil) // High similarity for functional group

	req := &EquivalentsRequest{
		QueryMolecule: []*StructuralElement{
			{ElementType: ElementTypeCoreScaffold, SMILESFragment: "S1"},
			{ElementType: ElementTypeFunctionalGroup, SMILESFragment: "F1"},
		},
		ClaimElements: []*StructuralElement{
			{ElementType: ElementTypeCoreScaffold, SMILESFragment: "S2"},
			{ElementType: ElementTypeFunctionalGroup, SMILESFragment: "F2"},
		},
	}

	res, err := analyzer.Analyze(context.Background(), req)
	assert.NoError(t, err)

	// Scaffold fails (0.4 < 0.6), Functional succeeds (0.9 > 0.6)
	// Total elements: 2. Equivalent: 1.
	assert.Equal(t, 1, res.EquivalentElementCount)

	// Score calculation:
	// Scaffold: fail -> score 0? No, ElementEquivalence returns failed object with IsEquivalent=false but still has scores.
	// But Analyze aggregates only if IsEquivalent is true?
	// Code: `if bestMatch.IsEquivalent { ... totalScore += bestMatch.OverallScore * weight }`
	// So failed elements contribute 0 to totalScore.
	// TotalWeight = 2.0 (scaffold) + 1.0 (functional) = 3.0.
	// Functional matches: Score approx (0.8+0.9+0.8)/3 = 0.83. Weighted = 0.83 * 1.0 = 0.83.
	// Scaffold fails: Score 0.
	// Overall = 0.83 / 3.0 = 0.27.

	assert.Less(t, res.OverallEquivalenceScore, 0.4)
}
//Personal.AI order the ending

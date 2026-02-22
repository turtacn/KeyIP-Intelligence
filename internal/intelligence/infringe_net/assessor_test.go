package infringe_net

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/turtacn/KeyIP-Intelligence/internal/intelligence/common"
)

// MockClaimElementMapper
type MockClaimElementMapper struct {
	mock.Mock
}

func (m *MockClaimElementMapper) MapElements(ctx context.Context, claims []*ClaimInput) ([]*MappedClaim, error) {
	args := m.Called(ctx, claims)
	return args.Get(0).([]*MappedClaim), args.Error(1)
}

func (m *MockClaimElementMapper) MapMoleculeToElements(ctx context.Context, moleculeSMILES string) ([]*StructuralElement, error) {
	args := m.Called(ctx, moleculeSMILES)
	return args.Get(0).([]*StructuralElement), args.Error(1)
}

func (m *MockClaimElementMapper) AlignElements(ctx context.Context, moleculeElements []*StructuralElement, claimElements []*ClaimElement) (*ElementAlignment, error) {
	args := m.Called(ctx, moleculeElements, claimElements)
	return args.Get(0).(*ElementAlignment), args.Error(1)
}

func (m *MockClaimElementMapper) CheckEstoppel(ctx context.Context, alignment *ElementAlignment, history *ProsecutionHistory) (*EstoppelResult, error) {
	args := m.Called(ctx, alignment, history)
	return args.Get(0).(*EstoppelResult), args.Error(1)
}

func (m *MockClaimElementMapper) ParseProsecutionHistory(ctx context.Context, rawHistory []byte) (*ProsecutionHistory, error) {
	return nil, nil
}

// MockEquivalentsAnalyzer
type MockEquivalentsAnalyzer struct {
	mock.Mock
}

func (m *MockEquivalentsAnalyzer) Analyze(ctx context.Context, req *EquivalentsRequest) (*EquivalentsResult, error) {
	args := m.Called(ctx, req)
	return args.Get(0).(*EquivalentsResult), args.Error(1)
}

func (m *MockEquivalentsAnalyzer) AnalyzeElement(ctx context.Context, queryElement, claimElement *StructuralElement) (*ElementEquivalence, error) {
	return nil, nil
}

func TestAssess_LiteralHigh(t *testing.T) {
	model := new(MockInfringeModel)
	mapper := new(MockClaimElementMapper)
	equivalents := new(MockEquivalentsAnalyzer)
	metrics := common.NewNoopIntelligenceMetrics()
	logger := new(MockLogger)

	assessor := NewInfringementAssessor(model, equivalents, mapper, nil, metrics, logger)

	req := &AssessmentRequest{
		MoleculeSMILES: "C",
		TargetClaims:   []*ClaimInput{{ClaimID: "1"}},
	}

	mapper.On("MapElements", mock.Anything, mock.Anything).Return([]*MappedClaim{
		{Elements: []*ClaimElement{{ElementID: "e1"}}},
	}, nil)
	mapper.On("MapMoleculeToElements", mock.Anything, "C").Return([]*StructuralElement{{ElementID: "m1"}}, nil)

	model.On("PredictLiteralInfringement", mock.Anything, mock.Anything).Return(&LiteralPredictionResult{
		OverallScore: 0.95,
	}, nil)

	res, err := assessor.Assess(context.Background(), req)
	assert.NoError(t, err)
	assert.Equal(t, RiskCritical, res.OverallRiskLevel)
}

func TestAssess_Equivalents(t *testing.T) {
	model := new(MockInfringeModel)
	mapper := new(MockClaimElementMapper)
	equivalents := new(MockEquivalentsAnalyzer)
	metrics := common.NewNoopIntelligenceMetrics()
	logger := new(MockLogger)

	assessor := NewInfringementAssessor(model, equivalents, mapper, nil, metrics, logger)

	req := &AssessmentRequest{
		MoleculeSMILES: "C",
		TargetClaims:   []*ClaimInput{{ClaimID: "1"}},
		Options:        &AssessmentOptions{IncludeEquivalents: true},
	}

	mapper.On("MapElements", mock.Anything, mock.Anything).Return([]*MappedClaim{
		{Elements: []*ClaimElement{{ElementID: "e1"}}},
	}, nil)
	mapper.On("MapMoleculeToElements", mock.Anything, "C").Return([]*StructuralElement{{ElementID: "m1"}}, nil)

	model.On("PredictLiteralInfringement", mock.Anything, mock.Anything).Return(&LiteralPredictionResult{
		OverallScore: 0.5,
	}, nil)

	equivalents.On("Analyze", mock.Anything, mock.Anything).Return(&EquivalentsResult{
		OverallEquivalenceScore: 0.9,
	}, nil)

	res, err := assessor.Assess(context.Background(), req)
	assert.NoError(t, err)
	// Score = 0.5 * 0.7 + 0.9 * 0.3 = 0.35 + 0.27 = 0.62 -> Medium
	assert.Equal(t, RiskMedium, res.OverallRiskLevel)
}
//Personal.AI order the ending

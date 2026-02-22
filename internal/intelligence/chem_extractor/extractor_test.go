package chem_extractor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/turtacn/KeyIP-Intelligence/internal/intelligence/claim_bert"
	"github.com/turtacn/KeyIP-Intelligence/internal/intelligence/common"
)

type MockNERModel struct {
	mock.Mock
}

func (m *MockNERModel) Predict(ctx context.Context, text string) (*NERPrediction, error) {
	args := m.Called(ctx, text)
	return args.Get(0).(*NERPrediction), args.Error(1)
}

func (m *MockNERModel) PredictBatch(ctx context.Context, texts []string) ([]*NERPrediction, error) {
	return nil, nil
}

func (m *MockNERModel) GetLabelSet() []string { return nil }

type MockEntityResolver struct {
	mock.Mock
}

func (m *MockEntityResolver) Resolve(ctx context.Context, entity *RawChemicalEntity) (*ResolvedChemicalEntity, error) {
	args := m.Called(ctx, entity)
	return args.Get(0).(*ResolvedChemicalEntity), args.Error(1)
}
// ... other methods stubbed
func (m *MockEntityResolver) ResolveBatch(ctx context.Context, entities []*RawChemicalEntity) ([]*ResolvedChemicalEntity, error) { return nil, nil }
func (m *MockEntityResolver) ResolveByType(ctx context.Context, text string, entityType ChemicalEntityType) (*ResolvedChemicalEntity, error) { return nil, nil }

type MockEntityValidator struct {
	mock.Mock
}

func (m *MockEntityValidator) Validate(ctx context.Context, entity *RawChemicalEntity) (*ValidationResult, error) {
	args := m.Called(ctx, entity)
	return args.Get(0).(*ValidationResult), args.Error(1)
}

func (m *MockEntityValidator) ValidateBatch(ctx context.Context, entities []*RawChemicalEntity) ([]*ValidationResult, error) { return nil, nil }

func TestExtract_Simple(t *testing.T) {
	ner := new(MockNERModel)
	validator := new(MockEntityValidator)
	logger := new(MockLogger)
	config := ExtractorConfig{ConfidenceThreshold: 0.5}

	extractor := NewChemicalExtractor(ner, nil, validator, config, common.NewNoopIntelligenceMetrics(), logger)

	ner.On("Predict", mock.Anything, "contains aspirin").Return(&NERPrediction{
		Entities: []*NEREntity{{
			Text: "aspirin", Label: "COMMON", StartChar: 9, EndChar: 16, Score: 0.9,
		}},
	}, nil)

	validator.On("Validate", mock.Anything, mock.Anything).Return(&ValidationResult{
		IsValid: true, AdjustedConfidence: 0.95, AdjustedType: EntityCommonName,
	}, nil)

	res, err := extractor.Extract(context.Background(), "contains aspirin")
	assert.NoError(t, err)
	assert.Equal(t, 1, len(res.Entities))
	assert.Equal(t, "aspirin", res.Entities[0].Text)
}

func TestExtractFromClaim_Mapping(t *testing.T) {
	ner := new(MockNERModel)
	validator := new(MockEntityValidator)
	extractor := NewChemicalExtractor(ner, nil, validator, ExtractorConfig{ConfidenceThreshold: 0.5}, common.NewNoopIntelligenceMetrics(), new(MockLogger))

	claim := &claim_bert.ParsedClaim{
		ClaimNumber: 1,
		Body:        "A composition comprising aspirin.",
		Features: []*claim_bert.TechnicalFeature{
			{ID: "f1", Text: "aspirin", StartOffset: 23, EndOffset: 30},
		},
	}

	ner.On("Predict", mock.Anything, claim.Body).Return(&NERPrediction{
		Entities: []*NEREntity{{
			Text: "aspirin", Label: "COMMON", StartChar: 23, EndChar: 30, Score: 0.9,
		}},
	}, nil)
	validator.On("Validate", mock.Anything, mock.Anything).Return(&ValidationResult{IsValid: true, AdjustedConfidence: 0.9}, nil)

	res, err := extractor.ExtractFromClaim(context.Background(), claim)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(res.FeatureEntityMapping["f1"]))
}
//Personal.AI order the ending

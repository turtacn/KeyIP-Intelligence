package infringe_net

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockNLPParser struct {
	mock.Mock
}

func (m *MockNLPParser) ParseClaimText(ctx context.Context, text string) ([]*RawElement, error) {
	args := m.Called(ctx, text)
	return args.Get(0).([]*RawElement), args.Error(1)
}

func (m *MockNLPParser) ClassifyElement(ctx context.Context, element *RawElement) (ElementType, error) {
	args := m.Called(ctx, element)
	return args.Get(0).(ElementType), args.Error(1)
}

type MockStructureAnalyzer struct {
	mock.Mock
}

func (m *MockStructureAnalyzer) DecomposeMolecule(ctx context.Context, smiles string) ([]*StructuralFragment, error) {
	args := m.Called(ctx, smiles)
	return args.Get(0).([]*StructuralFragment), args.Error(1)
}

func (m *MockStructureAnalyzer) ComputeFragmentSimilarity(ctx context.Context, frag1, frag2 string) (float64, error) {
	args := m.Called(ctx, frag1, frag2)
	return args.Get(0).(float64), args.Error(1)
}

func (m *MockStructureAnalyzer) MatchSMARTS(ctx context.Context, smiles, smarts string) (bool, error) {
	args := m.Called(ctx, smiles, smarts)
	return args.Bool(0), args.Error(1)
}

func TestMapElements_IndependentClaim(t *testing.T) {
	nlp := new(MockNLPParser)
	analyzer := new(MockStructureAnalyzer)
	logger := new(MockLogger)
	mapper, _ := NewClaimElementMapper(nlp, analyzer, nil, logger)

	nlp.On("ParseClaimText", mock.Anything, "claim text").Return([]*RawElement{
		{Text: "core"}, {Text: "substituent"},
	}, nil)
	nlp.On("ClassifyElement", mock.Anything, mock.MatchedBy(func(e *RawElement) bool { return e.Text == "core" })).Return(ElementTypeCoreScaffold, nil)
	nlp.On("ClassifyElement", mock.Anything, mock.MatchedBy(func(e *RawElement) bool { return e.Text == "substituent" })).Return(ElementTypeSubstituent, nil)

	claims := []*ClaimInput{{ClaimID: "1", ClaimText: "claim text", ClaimType: "Independent"}}
	mapped, err := mapper.MapElements(context.Background(), claims)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(mapped[0].Elements))
	assert.True(t, mapped[0].Elements[0].IsEssential)
}

func TestMapMoleculeToElements_Normal(t *testing.T) {
	nlp := new(MockNLPParser)
	analyzer := new(MockStructureAnalyzer)
	logger := new(MockLogger)
	mapper, _ := NewClaimElementMapper(nlp, analyzer, nil, logger)

	analyzer.On("DecomposeMolecule", mock.Anything, "C1").Return([]*StructuralFragment{
		{SMILES: "C1", Type: ElementTypeCoreScaffold},
	}, nil)

	res, err := mapper.MapMoleculeToElements(context.Background(), "C1")
	assert.NoError(t, err)
	assert.Equal(t, 1, len(res))
	assert.Equal(t, ElementTypeCoreScaffold, res[0].ElementType)
}

func TestAlignElements_PerfectMatch(t *testing.T) {
	analyzer := new(MockStructureAnalyzer)
	mapper, _ := NewClaimElementMapper(new(MockNLPParser), analyzer, nil, new(MockLogger))

	// Mock MatchSMARTS to false to force similarity check
	analyzer.On("MatchSMARTS", mock.Anything, mock.Anything, mock.Anything).Return(false, nil)
	analyzer.On("ComputeFragmentSimilarity", mock.Anything, mock.Anything, mock.Anything).Return(1.0, nil)

	molEls := []*StructuralElement{{ElementID: "m1", ElementType: ElementTypeCoreScaffold, SMILESFragment: "S1"}}
	claimEls := []*ClaimElement{{ElementID: "c1", ElementType: ElementTypeCoreScaffold, Description: "D1"}}

	res, err := mapper.AlignElements(context.Background(), molEls, claimEls)
	assert.NoError(t, err)
	assert.Equal(t, 1.0, res.CoverageRatio)
	assert.Equal(t, MatchTypeExact, res.Pairs[0].MatchType)
}

func TestCheckEstoppel_Narrowing(t *testing.T) {
	mapper, _ := NewClaimElementMapper(new(MockNLPParser), new(MockStructureAnalyzer), nil, new(MockLogger))

	alignment := &ElementAlignment{
		Pairs: []*AlignedPair{
			{ClaimElement: &ClaimElement{ElementID: "c1", Description: "desc"}, MatchType: MatchTypeSimilar},
		},
	}
	history := &ProsecutionHistory{
		Amendments: []*Amendment{
			{AmendmentType: "Narrowing", AffectedElements: []string{"c1"}},
		},
	}

	res, err := mapper.CheckEstoppel(context.Background(), alignment, history)
	assert.NoError(t, err)
	assert.True(t, res.HasEstoppel)
	assert.Equal(t, 1.0, res.EstoppelPenalty)
}
//Personal.AI order the ending

package cli

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/turtacn/KeyIP-Intelligence/internal/application/patent_mining"
)

// MockSimilaritySearchService is a mock implementation of SimilaritySearchService
type MockSimilaritySearchService struct {
	mock.Mock
}

func (m *MockSimilaritySearchService) Search(ctx context.Context, req *patent_mining.SimilaritySearchRequest) ([]*patent_mining.MoleculeSearchResult, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*patent_mining.MoleculeSearchResult), args.Error(1)
}

func (m *MockSimilaritySearchService) SearchByText(ctx context.Context, req *patent_mining.PatentSearchRequest) ([]*patent_mining.PatentSearchResult, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*patent_mining.PatentSearchResult), args.Error(1)
}

func TestParseFingerprints(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expected  []string
		expectErr bool
	}{
		{"All valid", "morgan,gnn,maccs", []string{"morgan", "gnn", "maccs"}, false},
		{"Single valid", "topological", []string{"topological"}, false},
		{"With spaces", " morgan , gnn ", []string{"morgan", "gnn"}, false},
		{"Invalid type", "morgan,invalid", nil, true},
		{"Empty string", "", nil, true},
		{"Case insensitive", "MORGAN,GNN", []string{"morgan", "gnn"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseFingerprints(tt.input)
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestParseOffices(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{"Empty", "", nil},
		{"Single", "CN", []string{"CN"}},
		{"Multiple", "CN,US,EP", []string{"CN", "US", "EP"}},
		{"With spaces", " cn , us ", []string{"CN", "US"}},
		{"Lowercase", "cn,us", []string{"CN", "US"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseOffices(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSearchMoleculeCmd_BothSMILESAndInChI(t *testing.T) {
	mockService := new(MockSimilaritySearchService)
	mockLogger := new(MockLogger)

	mockLogger.On("Info", mock.Anything, mock.Anything).Return()

	ctx := context.Background()
	searchSMILES = "CCO"
	searchInChI = "InChI=1S/C2H6O/c1-2-3/h3H,2H2,1H3"

	err := runSearchMolecule(ctx, mockService, mockLogger)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "mutually exclusive")
}

func TestSearchMoleculeCmd_NeitherSMILESNorInChI(t *testing.T) {
	mockService := new(MockSimilaritySearchService)
	mockLogger := new(MockLogger)

	ctx := context.Background()
	searchSMILES = ""
	searchInChI = ""

	err := runSearchMolecule(ctx, mockService, mockLogger)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "either --smiles or --inchi must be provided")
}

func TestSearchMoleculeCmd_InvalidThreshold_TooLow(t *testing.T) {
	mockService := new(MockSimilaritySearchService)
	mockLogger := new(MockLogger)

	ctx := context.Background()
	searchSMILES = "CCO"
	searchThreshold = -0.1

	err := runSearchMolecule(ctx, mockService, mockLogger)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "threshold must be between 0.0 and 1.0")
}

func TestSearchMoleculeCmd_InvalidThreshold_TooHigh(t *testing.T) {
	mockService := new(MockSimilaritySearchService)
	mockLogger := new(MockLogger)

	ctx := context.Background()
	searchSMILES = "CCO"
	searchThreshold = 1.5

	err := runSearchMolecule(ctx, mockService, mockLogger)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "threshold must be between 0.0 and 1.0")
}

func TestSearchMoleculeCmd_MaxResultsLimit(t *testing.T) {
	mockService := new(MockSimilaritySearchService)
	mockLogger := new(MockLogger)

	ctx := context.Background()
	searchSMILES = "CCO"
	searchMaxResults = 600

	err := runSearchMolecule(ctx, mockService, mockLogger)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "max-results must be between 1 and 500")
}

func TestSearchMoleculeCmd_InvalidFingerprint(t *testing.T) {
	mockService := new(MockSimilaritySearchService)
	mockLogger := new(MockLogger)

	ctx := context.Background()
	searchSMILES = "CCO"
	searchThreshold = 0.7
	searchMaxResults = 20
	searchFingerprints = "morgan,invalid"

	err := runSearchMolecule(ctx, mockService, mockLogger)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid fingerprint type")
}

func TestSearchMoleculeCmd_BySMILES(t *testing.T) {
	mockService := new(MockSimilaritySearchService)
	mockLogger := new(MockLogger)

	results := []*patent_mining.MoleculeSearchResult{
		{
			PatentNumber: "CN115123456A",
			MoleculeName: "Ethanol",
			SMILES:       "CCO",
			Similarity:   0.95,
			RiskLevel:    "LOW",
		},
		{
			PatentNumber: "US11987654B2",
			MoleculeName: "Methanol derivative",
			SMILES:       "CO",
			Similarity:   0.72,
			RiskLevel:    "MEDIUM",
		},
	}

	mockService.On("Search", mock.Anything, mock.MatchedBy(func(req *patent_mining.SimilaritySearchRequest) bool {
		return req.SMILES == "CCO" && req.Threshold == 0.65
	})).Return(results, nil)

	mockLogger.On("Info", mock.Anything, mock.Anything).Return()

	ctx := context.Background()
	searchSMILES = "CCO"
	searchInChI = ""
	searchThreshold = 0.65
	searchMaxResults = 20
	searchFingerprints = "morgan,gnn"
	searchIncludeRisk = true
	searchOutput = "stdout"

	err := runSearchMolecule(ctx, mockService, mockLogger)
	assert.NoError(t, err)

	mockService.AssertExpectations(t)
}

func TestSearchMoleculeCmd_EmptyResults(t *testing.T) {
	mockService := new(MockSimilaritySearchService)
	mockLogger := new(MockLogger)

	mockService.On("Search", mock.Anything, mock.Anything).Return([]*patent_mining.MoleculeSearchResult{}, nil)
	mockLogger.On("Info", mock.Anything, mock.Anything).Return()

	ctx := context.Background()
	searchSMILES = "CCO"
	searchThreshold = 0.99
	searchMaxResults = 20
	searchFingerprints = "morgan"

	err := runSearchMolecule(ctx, mockService, mockLogger)
	assert.NoError(t, err)

	mockService.AssertExpectations(t)
}

func TestSearchMoleculeCmd_JSONOutput(t *testing.T) {
	mockService := new(MockSimilaritySearchService)
	mockLogger := new(MockLogger)

	results := []*patent_mining.MoleculeSearchResult{
		{
			PatentNumber: "CN115123456A",
			MoleculeName: "Ethanol",
			SMILES:       "CCO",
			Similarity:   0.95,
		},
	}

	mockService.On("Search", mock.Anything, mock.Anything).Return(results, nil)
	mockLogger.On("Info", mock.Anything, mock.Anything).Return()

	output, err := formatMoleculeResults(results, "json")
	require.NoError(t, err)
	assert.Contains(t, output, "CN115123456A")
	assert.Contains(t, output, "0.95")
}

func TestSearchPatentCmd_BasicQuery(t *testing.T) {
	mockService := new(MockSimilaritySearchService)
	mockLogger := new(MockLogger)

	now := time.Now()
	results := []*patent_mining.PatentSearchResult{
		{
			PatentNumber: "CN115123456A",
			Title:        "Machine learning method for patent analysis",
			FilingDate:   now,
			IPC:          "G06N 3/08",
			Relevance:    0.85,
		},
	}

	mockService.On("SearchByText", mock.Anything, mock.MatchedBy(func(req *patent_mining.PatentSearchRequest) bool {
		return req.Query == "machine learning" && req.Sort == "relevance"
	})).Return(results, nil)

	mockLogger.On("Info", mock.Anything, mock.Anything).Return()

	ctx := context.Background()
	searchQuery = "machine learning"
	searchMaxResults = 50
	searchSort = "relevance"
	searchOutput = "stdout"

	err := runSearchPatent(ctx, mockService, mockLogger)
	assert.NoError(t, err)

	mockService.AssertExpectations(t)
}

func TestSearchPatentCmd_InvalidDateFormat(t *testing.T) {
	mockService := new(MockSimilaritySearchService)
	mockLogger := new(MockLogger)

	ctx := context.Background()
	searchQuery = "test"
	searchDateFrom = "2024/01/01"
	searchMaxResults = 50
	searchSort = "relevance"

	err := runSearchPatent(ctx, mockService, mockLogger)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid date-from format")
}

func TestSearchPatentCmd_DateRangeInverted(t *testing.T) {
	mockService := new(MockSimilaritySearchService)
	mockLogger := new(MockLogger)

	ctx := context.Background()
	searchQuery = "test"
	searchDateFrom = "2024-12-31"
	searchDateTo = "2024-01-01"
	searchMaxResults = 50
	searchSort = "relevance"

	err := runSearchPatent(ctx, mockService, mockLogger)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "date-from cannot be later than date-to")
}

func TestSearchPatentCmd_InvalidSort(t *testing.T) {
	mockService := new(MockSimilaritySearchService)
	mockLogger := new(MockLogger)

	ctx := context.Background()
	searchQuery = "test"
	searchMaxResults = 50
	searchSort = "popularity"

	err := runSearchPatent(ctx, mockService, mockLogger)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid sort parameter")
}

func TestSearchPatentCmd_WithIPCFilter(t *testing.T) {
	mockService := new(MockSimilaritySearchService)
	mockLogger := new(MockLogger)

	now := time.Now()
	results := []*patent_mining.PatentSearchResult{
		{
			PatentNumber: "CN115123456A",
			Title:        "Chemical compound",
			FilingDate:   now,
			IPC:          "C07D 213/30",
			Relevance:    0.90,
		},
	}

	mockService.On("SearchByText", mock.Anything, mock.MatchedBy(func(req *patent_mining.PatentSearchRequest) bool {
		return req.IPC == "C07D"
	})).Return(results, nil)

	mockLogger.On("Info", mock.Anything, mock.Anything).Return()

	ctx := context.Background()
	searchQuery = "chemical"
	searchIPC = "C07D"
	searchMaxResults = 50
	searchSort = "relevance"

	err := runSearchPatent(ctx, mockService, mockLogger)
	assert.NoError(t, err)

	mockService.AssertExpectations(t)
}

func TestFormatMoleculeResults_Table(t *testing.T) {
	results := []*patent_mining.MoleculeSearchResult{
		{
			PatentNumber: "CN115123456A",
			MoleculeName: "Ethanol",
			SMILES:       "CCO",
			Similarity:   0.95,
			RiskLevel:    "LOW",
		},
	}

	searchThreshold = 0.7
	searchIncludeRisk = true

	output, err := formatMoleculeResults(results, "stdout")
	require.NoError(t, err)
	assert.Contains(t, output, "CN115123456A")
	assert.Contains(t, output, "95.00%")
	assert.Contains(t, output, "LOW")
}

func TestFormatPatentResults_Table(t *testing.T) {
	now := time.Now()
	results := []*patent_mining.PatentSearchResult{
		{
			PatentNumber: "CN115123456A",
			Title:        "Test Patent",
			FilingDate:   now,
			IPC:          "G06N 3/08",
			Relevance:    0.85,
		},
	}

	output, err := formatPatentResults(results, "stdout")
	require.NoError(t, err)
	assert.Contains(t, output, "CN115123456A")
	assert.Contains(t, output, "Test Patent")
	assert.Contains(t, output, "85.00%")
}

func TestColorizeRiskLevel_AllLevels(t *testing.T) {
	tests := []struct {
		level    string
		contains string
	}{
		{"HIGH", "HIGH"},
		{"MEDIUM", "MEDIUM"},
		{"LOW", "LOW"},
		{"UNKNOWN", "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.level, func(t *testing.T) {
			result := colorizeRiskLevel(tt.level)
			assert.Contains(t, result, tt.contains)
		})
	}
}

//Personal.AI order the ending

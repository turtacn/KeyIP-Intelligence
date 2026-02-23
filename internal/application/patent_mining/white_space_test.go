// Phase 10 - File 221 of 349
// Phase: 应用层 - 业务服务
// SubModule: patent_mining
// File: internal/application/patent_mining/white_space_test.go

package patent_mining

import (
	"context"
	"errors"
	"fmt"
	"testing"

	apperrors "github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// ---------------------------------------------------------------------------
// Mocks
// ---------------------------------------------------------------------------

type mockPatentLandscapeProvider struct {
	getByFieldFn    func(ctx context.Context, techField string, subFields []string, jurisdiction string, yearFrom int, yearTo int) (*LandscapeData, error)
	getByScaffoldFn func(ctx context.Context, scaffold string) (*LandscapeData, error)
	getByPropertyFn func(ctx context.Context, propertyName string, minVal float64, maxVal float64, techField string) (*LandscapeData, error)
}

func (m *mockPatentLandscapeProvider) GetLandscapeByField(ctx context.Context, techField string, subFields []string, jurisdiction string, yearFrom int, yearTo int) (*LandscapeData, error) {
	if m.getByFieldFn != nil {
		return m.getByFieldFn(ctx, techField, subFields, jurisdiction, yearFrom, yearTo)
	}
	return defaultLandscape(), nil
}

func (m *mockPatentLandscapeProvider) GetLandscapeByScaffold(ctx context.Context, scaffold string) (*LandscapeData, error) {
	if m.getByScaffoldFn != nil {
		return m.getByScaffoldFn(ctx, scaffold)
	}
	return defaultLandscape(), nil
}

func (m *mockPatentLandscapeProvider) GetLandscapeByProperty(ctx context.Context, propertyName string, minVal float64, maxVal float64, techField string) (*LandscapeData, error) {
	if m.getByPropertyFn != nil {
		return m.getByPropertyFn(ctx, propertyName, minVal, maxVal, techField)
	}
	return defaultLandscape(), nil
}

type mockMoleculeSpaceAnalyzer struct {
	findGapsFn         func(ctx context.Context, scaffold string, substituents []string, targetProperty string) ([]WhiteSpaceOpportunity, error)
	findPropertyGapsFn func(ctx context.Context, propertyName string, minVal float64, maxVal float64, stepSize float64, techField string) ([]WhiteSpaceOpportunity, error)
}

func (m *mockMoleculeSpaceAnalyzer) FindGaps(ctx context.Context, scaffold string, substituents []string, targetProperty string) ([]WhiteSpaceOpportunity, error) {
	if m.findGapsFn != nil {
		return m.findGapsFn(ctx, scaffold, substituents, targetProperty)
	}
	return defaultOpportunities(), nil
}

func (m *mockMoleculeSpaceAnalyzer) FindPropertyGaps(ctx context.Context, propertyName string, minVal float64, maxVal float64, stepSize float64, techField string) ([]WhiteSpaceOpportunity, error) {
	if m.findPropertyGapsFn != nil {
		return m.findPropertyGapsFn(ctx, propertyName, minVal, maxVal, stepSize, techField)
	}
	return defaultOpportunities(), nil
}

type mockWhiteSpaceReportStore struct {
	saveFn       func(ctx context.Context, result *WhiteSpaceAnalysisResult) error
	getFn        func(ctx context.Context, id string) (*WhiteSpaceAnalysisResult, error)
	listRecentFn func(ctx context.Context, limit int) ([]WhiteSpaceAnalysisResult, error)
}

func (m *mockWhiteSpaceReportStore) Save(ctx context.Context, result *WhiteSpaceAnalysisResult) error {
	if m.saveFn != nil {
		return m.saveFn(ctx, result)
	}
	return nil
}

func (m *mockWhiteSpaceReportStore) Get(ctx context.Context, id string) (*WhiteSpaceAnalysisResult, error) {
	if m.getFn != nil {
		return m.getFn(ctx, id)
	}
	return nil, apperrors.ErrNotFound("ws_report", id)
}

func (m *mockWhiteSpaceReportStore) ListRecent(ctx context.Context, limit int) ([]WhiteSpaceAnalysisResult, error) {
	if m.listRecentFn != nil {
		return m.listRecentFn(ctx, limit)
	}
	return nil, nil
}

type mockWSLogger struct{}

func (m *mockWSLogger) Info(msg string, fields ...interface{})  {}
func (m *mockWSLogger) Error(msg string, fields ...interface{}) {}
func (m *mockWSLogger) Warn(msg string, fields ...interface{})  {}
func (m *mockWSLogger) Debug(msg string, fields ...interface{}) {}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func newTestWhiteSpaceService(
	lp PatentLandscapeProvider,
	ma MoleculeSpaceAnalyzer,
	rs WhiteSpaceReportStore,
) WhiteSpaceService {
	return NewWhiteSpaceService(WhiteSpaceDeps{
		Landscape:   lp,
		MolAnalyzer: ma,
		ReportStore: rs,
		Logger:      &mockWSLogger{},
	})
}

func defaultLandscape() *LandscapeData {
	return &LandscapeData{
		TotalPatents: 1500,
		Coverage:     0.65,
		Clusters: []LandscapeCluster{
			{ID: "c1", Label: "Carbazole hosts", PatentCount: 350, Density: 0.80, PatentIDs: []string{"p1", "p2"}},
			{ID: "c2", Label: "Triazine emitters", PatentCount: 50, Density: 0.15, PatentIDs: []string{"p3"}},
			{ID: "c3", Label: "Phosphorescent dopants", PatentCount: 10, Density: 0.05, PatentIDs: []string{"p4"}},
			{ID: "c4", Label: "Exciplex systems", PatentCount: 200, Density: 0.55},
		},
	}
}

func defaultOpportunities() []WhiteSpaceOpportunity {
	return []WhiteSpaceOpportunity{
		{
			ID:              "opp-1",
			Description:     "Unexplored triazine-carbazole hybrid hosts",
			Level:           OpportunityHigh,
			TechArea:        "OLED hosts",
			PatentDensity:   0.05,
			InnovationScore: 0.95,
		},
		{
			ID:              "opp-2",
			Description:     "Low coverage in blue TADF emitters",
			Level:           OpportunityMedium,
			TechArea:        "TADF",
			PatentDensity:   0.20,
			InnovationScore: 0.80,
		},
	}
}

// ===========================================================================
// Tests: AnalyzeByTechField
// ===========================================================================

func TestAnalyzeByTechField_Success(t *testing.T) {
	svc := newTestWhiteSpaceService(
		&mockPatentLandscapeProvider{},
		&mockMoleculeSpaceAnalyzer{},
		&mockWhiteSpaceReportStore{},
	)

	req := &AnalyzeByTechFieldRequest{
		TechField:    "organic_electronics",
		SubFields:    []string{"OLED", "OPV"},
		Jurisdiction: "CN",
		YearFrom:     2018,
		YearTo:       2024,
		MaxResults:   10,
	}

	result, err := svc.AnalyzeByTechField(context.Background(), req)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.AnalysisType != WhiteSpaceTechField {
		t.Errorf("expected analysis type tech_field, got %s", result.AnalysisType)
	}
	if result.TotalPatents != 1500 {
		t.Errorf("expected 1500 total patents, got %d", result.TotalPatents)
	}
	if len(result.Opportunities) == 0 {
		t.Error("expected at least one opportunity identified")
	}
	// Only low-density clusters should appear
	for _, opp := range result.Opportunities {
		if opp.PatentDensity >= 0.30 {
			t.Errorf("opportunity %s has density %.2f, expected < 0.30", opp.ID, opp.PatentDensity)
		}
	}
}

func TestAnalyzeByTechField_NilRequest(t *testing.T) {
	svc := newTestWhiteSpaceService(&mockPatentLandscapeProvider{}, &mockMoleculeSpaceAnalyzer{}, &mockWhiteSpaceReportStore{})

	_, err := svc.AnalyzeByTechField(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil request")
	}
}

func TestAnalyzeByTechField_EmptyField(t *testing.T) {
	svc := newTestWhiteSpaceService(&mockPatentLandscapeProvider{}, &mockMoleculeSpaceAnalyzer{}, &mockWhiteSpaceReportStore{})

	req := &AnalyzeByTechFieldRequest{TechField: ""}
	_, err := svc.AnalyzeByTechField(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for empty tech field")
	}
	if !apperrors.IsValidation(err) {
		t.Errorf("expected ValidationError, got: %v", err)
	}
}

func TestAnalyzeByTechField_LandscapeError(t *testing.T) {
	lp := &mockPatentLandscapeProvider{
		getByFieldFn: func(ctx context.Context, techField string, subFields []string, jurisdiction string, yearFrom int, yearTo int) (*LandscapeData, error) {
			return nil, errors.New("landscape service unavailable")
		},
	}

	svc := newTestWhiteSpaceService(lp, &mockMoleculeSpaceAnalyzer{}, &mockWhiteSpaceReportStore{})

	req := &AnalyzeByTechFieldRequest{TechField: "organic_electronics"}
	_, err := svc.AnalyzeByTechField(context.Background(), req)
	if err == nil {
		t.Fatal("expected error from landscape failure")
	}
}

func TestAnalyzeByTechField_DefaultMaxResults(t *testing.T) {
	svc := newTestWhiteSpaceService(&mockPatentLandscapeProvider{}, &mockMoleculeSpaceAnalyzer{}, &mockWhiteSpaceReportStore{})

	req := &AnalyzeByTechFieldRequest{TechField: "organic_electronics"}
	result, err := svc.AnalyzeByTechField(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should not exceed default max of 20
	if len(result.Opportunities) > 20 {
		t.Errorf("expected at most 20 opportunities, got %d", len(result.Opportunities))
	}
}

// ===========================================================================
// Tests: AnalyzeByMoleculeClass
// ===========================================================================

func TestAnalyzeByMoleculeClass_Success(t *testing.T) {
	svc := newTestWhiteSpaceService(
		&mockPatentLandscapeProvider{},
		&mockMoleculeSpaceAnalyzer{},
		&mockWhiteSpaceReportStore{},
	)

	req := &AnalyzeByMoleculeClassRequest{
		CoreScaffold:   "c1ccc2c(c1)[nH]c1ccccc12",
		Substituents:   []string{"-CN", "-CF3", "-OMe"},
		TargetProperty: "triplet_energy",
		MaxResults:     5,
	}

	result, err := svc.AnalyzeByMoleculeClass(context.Background(), req)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if result.AnalysisType != WhiteSpaceMoleculeClass {
		t.Errorf("expected analysis type molecule_class, got %s", result.AnalysisType)
	}
	if len(result.Opportunities) == 0 {
		t.Error("expected at least one opportunity")
	}
	if len(result.Opportunities) > 5 {
		t.Errorf("expected at most 5 opportunities, got %d", len(result.Opportunities))
	}
}

func TestAnalyzeByMoleculeClass_NilRequest(t *testing.T) {
	svc := newTestWhiteSpaceService(&mockPatentLandscapeProvider{}, &mockMoleculeSpaceAnalyzer{}, &mockWhiteSpaceReportStore{})

	_, err := svc.AnalyzeByMoleculeClass(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil request")
	}
}

func TestAnalyzeByMoleculeClass_EmptyScaffold(t *testing.T) {
	svc := newTestWhiteSpaceService(&mockPatentLandscapeProvider{}, &mockMoleculeSpaceAnalyzer{}, &mockWhiteSpaceReportStore{})

	req := &AnalyzeByMoleculeClassRequest{CoreScaffold: ""}
	_, err := svc.AnalyzeByMoleculeClass(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for empty scaffold")
	}
}

func TestAnalyzeByMoleculeClass_GapAnalysisError(t *testing.T) {
	ma := &mockMoleculeSpaceAnalyzer{
		findGapsFn: func(ctx context.Context, scaffold string, substituents []string, targetProperty string) ([]WhiteSpaceOpportunity, error) {
			return nil, errors.New("analysis engine error")
		},
	}

	svc := newTestWhiteSpaceService(&mockPatentLandscapeProvider{}, ma, &mockWhiteSpaceReportStore{})

	req := &AnalyzeByMoleculeClassRequest{CoreScaffold: "c1ccccc1"}
	_, err := svc.AnalyzeByMoleculeClass(context.Background(), req)
	if err == nil {
		t.Fatal("expected error from gap analysis failure")
	}
}

func TestAnalyzeByMoleculeClass_ScaffoldLandscapeError(t *testing.T) {
	lp := &mockPatentLandscapeProvider{
		getByScaffoldFn: func(ctx context.Context, scaffold string) (*LandscapeData, error) {
			return nil, errors.New("scaffold index unavailable")
		},
	}

	svc := newTestWhiteSpaceService(lp, &mockMoleculeSpaceAnalyzer{}, &mockWhiteSpaceReportStore{})

	req := &AnalyzeByMoleculeClassRequest{CoreScaffold: "c1ccccc1"}
	_, err := svc.AnalyzeByMoleculeClass(context.Background(), req)
	if err == nil {
		t.Fatal("expected error from landscape failure")
	}
}

// ===========================================================================
// Tests: AnalyzeByPropertyRange
// ===========================================================================

func TestAnalyzeByPropertyRange_Success(t *testing.T) {
	svc := newTestWhiteSpaceService(
		&mockPatentLandscapeProvider{},
		&mockMoleculeSpaceAnalyzer{},
		&mockWhiteSpaceReportStore{},
	)

	req := &AnalyzeByPropertyRangeRequest{
		PropertyName: "quantum_efficiency",
		MinValue:     0.10,
		MaxValue:     0.95,
		StepSize:     0.05,
		TechField:    "OLED",
		MaxResults:   10,
	}

	result, err := svc.AnalyzeByPropertyRange(context.Background(), req)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if result.AnalysisType != WhiteSpacePropertyRange {
		t.Errorf("expected analysis type property_range, got %s", result.AnalysisType)
	}
	if result.Metadata["property_name"] != "quantum_efficiency" {
		t.Errorf("expected property_name in metadata")
	}
}

func TestAnalyzeByPropertyRange_NilRequest(t *testing.T) {
	svc := newTestWhiteSpaceService(&mockPatentLandscapeProvider{}, &mockMoleculeSpaceAnalyzer{}, &mockWhiteSpaceReportStore{})

	_, err := svc.AnalyzeByPropertyRange(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil request")
	}
}

func TestAnalyzeByPropertyRange_EmptyProperty(t *testing.T) {
	svc := newTestWhiteSpaceService(&mockPatentLandscapeProvider{}, &mockMoleculeSpaceAnalyzer{}, &mockWhiteSpaceReportStore{})

	req := &AnalyzeByPropertyRangeRequest{PropertyName: "", MinValue: 0, MaxValue: 1, StepSize: 0.1}
	_, err := svc.AnalyzeByPropertyRange(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for empty property name")
	}
	if !apperrors.IsValidation(err) {
		t.Errorf("expected ValidationError, got: %v", err)
	}
}

func TestAnalyzeByPropertyRange_InvalidRange(t *testing.T) {
	svc := newTestWhiteSpaceService(&mockPatentLandscapeProvider{}, &mockMoleculeSpaceAnalyzer{}, &mockWhiteSpaceReportStore{})

	req := &AnalyzeByPropertyRangeRequest{
		PropertyName: "quantum_efficiency",
		MinValue:     0.95,
		MaxValue:     0.10,
		StepSize:     0.05,
	}
	_, err := svc.AnalyzeByPropertyRange(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for min >= max")
	}
	if !apperrors.IsValidation(err) {
		t.Errorf("expected ValidationError, got: %v", err)
	}
}

func TestAnalyzeByPropertyRange_EqualMinMax(t *testing.T) {
	svc := newTestWhiteSpaceService(&mockPatentLandscapeProvider{}, &mockMoleculeSpaceAnalyzer{}, &mockWhiteSpaceReportStore{})

	req := &AnalyzeByPropertyRangeRequest{
		PropertyName: "quantum_efficiency",
		MinValue:     0.50,
		MaxValue:     0.50,
		StepSize:     0.05,
	}
	_, err := svc.AnalyzeByPropertyRange(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for equal min and max")
	}
}

func TestAnalyzeByPropertyRange_ZeroStepSize(t *testing.T) {
	svc := newTestWhiteSpaceService(&mockPatentLandscapeProvider{}, &mockMoleculeSpaceAnalyzer{}, &mockWhiteSpaceReportStore{})

	req := &AnalyzeByPropertyRangeRequest{
		PropertyName: "quantum_efficiency",
		MinValue:     0.10,
		MaxValue:     0.90,
		StepSize:     0,
	}
	_, err := svc.AnalyzeByPropertyRange(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for zero step size")
	}
}

func TestAnalyzeByPropertyRange_NegativeStepSize(t *testing.T) {
	svc := newTestWhiteSpaceService(&mockPatentLandscapeProvider{}, &mockMoleculeSpaceAnalyzer{}, &mockWhiteSpaceReportStore{})

	req := &AnalyzeByPropertyRangeRequest{
		PropertyName: "quantum_efficiency",
		MinValue:     0.10,
		MaxValue:     0.90,
		StepSize:     -0.05,
	}
	_, err := svc.AnalyzeByPropertyRange(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for negative step size")
	}
}

func TestAnalyzeByPropertyRange_LandscapeError(t *testing.T) {
	lp := &mockPatentLandscapeProvider{
		getByPropertyFn: func(ctx context.Context, propertyName string, minVal float64, maxVal float64, techField string) (*LandscapeData, error) {
			return nil, errors.New("property landscape unavailable")
		},
	}

	svc := newTestWhiteSpaceService(lp, &mockMoleculeSpaceAnalyzer{}, &mockWhiteSpaceReportStore{})

	req := &AnalyzeByPropertyRangeRequest{
		PropertyName: "quantum_efficiency",
		MinValue:     0.10,
		MaxValue:     0.90,
		StepSize:     0.05,
	}
	_, err := svc.AnalyzeByPropertyRange(context.Background(), req)
	if err == nil {
		t.Fatal("expected error from landscape failure")
	}
}

func TestAnalyzeByPropertyRange_PropertyGapError(t *testing.T) {
	ma := &mockMoleculeSpaceAnalyzer{
		findPropertyGapsFn: func(ctx context.Context, propertyName string, minVal float64, maxVal float64, stepSize float64, techField string) ([]WhiteSpaceOpportunity, error) {
			return nil, errors.New("property gap engine failure")
		},
	}

	svc := newTestWhiteSpaceService(&mockPatentLandscapeProvider{}, ma, &mockWhiteSpaceReportStore{})

	req := &AnalyzeByPropertyRangeRequest{
		PropertyName: "quantum_efficiency",
		MinValue:     0.10,
		MaxValue:     0.90,
		StepSize:     0.05,
	}
	_, err := svc.AnalyzeByPropertyRange(context.Background(), req)
	if err == nil {
		t.Fatal("expected error from property gap analysis failure")
	}
}

func TestAnalyzeByPropertyRange_DefaultMaxResults(t *testing.T) {
	svc := newTestWhiteSpaceService(&mockPatentLandscapeProvider{}, &mockMoleculeSpaceAnalyzer{}, &mockWhiteSpaceReportStore{})

	req := &AnalyzeByPropertyRangeRequest{
		PropertyName: "quantum_efficiency",
		MinValue:     0.10,
		MaxValue:     0.90,
		StepSize:     0.05,
	}
	result, err := svc.AnalyzeByPropertyRange(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Opportunities) > 20 {
		t.Errorf("expected at most 20 opportunities with default max, got %d", len(result.Opportunities))
	}
}

// ===========================================================================
// Tests: GetAnalysisReport
// ===========================================================================

func TestGetAnalysisReport_Success(t *testing.T) {
	expected := &WhiteSpaceAnalysisResult{
		ID:           "ws-001",
		AnalysisType: WhiteSpaceTechField,
		Query:        "organic_electronics",
		TotalPatents: 1500,
	}

	store := &mockWhiteSpaceReportStore{
		getFn: func(ctx context.Context, id string) (*WhiteSpaceAnalysisResult, error) {
			if id == "ws-001" {
				return expected, nil
			}
			return nil, apperrors.ErrNotFound("ws_report", id)
		},
	}

	svc := newTestWhiteSpaceService(&mockPatentLandscapeProvider{}, &mockMoleculeSpaceAnalyzer{}, store)

	result, err := svc.GetAnalysisReport(context.Background(), "ws-001")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if result.ID != "ws-001" {
		t.Errorf("expected ID ws-001, got %s", result.ID)
	}
	if result.AnalysisType != WhiteSpaceTechField {
		t.Errorf("expected analysis type tech_field, got %s", result.AnalysisType)
	}
}

func TestGetAnalysisReport_NotFound(t *testing.T) {
	svc := newTestWhiteSpaceService(&mockPatentLandscapeProvider{}, &mockMoleculeSpaceAnalyzer{}, &mockWhiteSpaceReportStore{})

	_, err := svc.GetAnalysisReport(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent report")
	}
	if !apperrors.IsNotFound(err) {
		t.Errorf("expected NotFoundError, got: %v", err)
	}
}

func TestGetAnalysisReport_EmptyID(t *testing.T) {
	svc := newTestWhiteSpaceService(&mockPatentLandscapeProvider{}, &mockMoleculeSpaceAnalyzer{}, &mockWhiteSpaceReportStore{})

	_, err := svc.GetAnalysisReport(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty ID")
	}
	if !apperrors.IsValidation(err) {
		t.Errorf("expected ValidationError, got: %v", err)
	}
}

// ===========================================================================
// Tests: ListRecentAnalyses
// ===========================================================================

func TestListRecentAnalyses_Success(t *testing.T) {
	store := &mockWhiteSpaceReportStore{
		listRecentFn: func(ctx context.Context, limit int) ([]WhiteSpaceAnalysisResult, error) {
			return []WhiteSpaceAnalysisResult{
				{ID: "ws-001", AnalysisType: WhiteSpaceTechField},
				{ID: "ws-002", AnalysisType: WhiteSpaceMoleculeClass},
			}, nil
		},
	}

	svc := newTestWhiteSpaceService(&mockPatentLandscapeProvider{}, &mockMoleculeSpaceAnalyzer{}, store)

	results, err := svc.ListRecentAnalyses(context.Background(), 10)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
}

func TestListRecentAnalyses_DefaultLimit(t *testing.T) {
	var capturedLimit int
	store := &mockWhiteSpaceReportStore{
		listRecentFn: func(ctx context.Context, limit int) ([]WhiteSpaceAnalysisResult, error) {
			capturedLimit = limit
			return nil, nil
		},
	}

	svc := newTestWhiteSpaceService(&mockPatentLandscapeProvider{}, &mockMoleculeSpaceAnalyzer{}, store)

	_, err := svc.ListRecentAnalyses(context.Background(), 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedLimit != 10 {
		t.Errorf("expected default limit 10, got %d", capturedLimit)
	}
}

func TestListRecentAnalyses_MaxLimitCap(t *testing.T) {
	var capturedLimit int
	store := &mockWhiteSpaceReportStore{
		listRecentFn: func(ctx context.Context, limit int) ([]WhiteSpaceAnalysisResult, error) {
			capturedLimit = limit
			return nil, nil
		},
	}

	svc := newTestWhiteSpaceService(&mockPatentLandscapeProvider{}, &mockMoleculeSpaceAnalyzer{}, store)

	_, err := svc.ListRecentAnalyses(context.Background(), 500)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedLimit != 100 {
		t.Errorf("expected capped limit 100, got %d", capturedLimit)
	}
}

func TestListRecentAnalyses_StoreError(t *testing.T) {
	store := &mockWhiteSpaceReportStore{
		listRecentFn: func(ctx context.Context, limit int) ([]WhiteSpaceAnalysisResult, error) {
			return nil, errors.New("store unavailable")
		},
	}

	svc := newTestWhiteSpaceService(&mockPatentLandscapeProvider{}, &mockMoleculeSpaceAnalyzer{}, store)

	_, err := svc.ListRecentAnalyses(context.Background(), 10)
	if err == nil {
		t.Fatal("expected error from store failure")
	}
}

// ===========================================================================
// Tests: Helper functions
// ===========================================================================

func TestIdentifyGapsFromLandscape_Basic(t *testing.T) {
	landscape := defaultLandscape()
	gaps := identifyGapsFromLandscape(landscape, 10)

	// defaultLandscape has 2 clusters with density < 0.30: c2 (0.15) and c3 (0.05)
	if len(gaps) != 2 {
		t.Errorf("expected 2 gaps, got %d", len(gaps))
	}

	// Should be sorted by innovation score descending
	// c3 density=0.05 -> innovation=0.95, c2 density=0.15 -> innovation=0.85
	if len(gaps) >= 2 {
		if gaps[0].InnovationScore < gaps[1].InnovationScore {
			t.Error("expected gaps sorted by innovation score descending")
		}
	}
}

func TestIdentifyGapsFromLandscape_NilLandscape(t *testing.T) {
	gaps := identifyGapsFromLandscape(nil, 10)
	if gaps != nil {
		t.Errorf("expected nil for nil landscape, got %v", gaps)
	}
}

func TestIdentifyGapsFromLandscape_EmptyClusters(t *testing.T) {
	landscape := &LandscapeData{TotalPatents: 100, Clusters: []LandscapeCluster{}}
	gaps := identifyGapsFromLandscape(landscape, 10)
	if gaps != nil {
		t.Errorf("expected nil for empty clusters, got %v", gaps)
	}
}

func TestIdentifyGapsFromLandscape_AllHighDensity(t *testing.T) {
	landscape := &LandscapeData{
		TotalPatents: 500,
		Clusters: []LandscapeCluster{
			{ID: "c1", Label: "Area A", Density: 0.80},
			{ID: "c2", Label: "Area B", Density: 0.90},
		},
	}
	gaps := identifyGapsFromLandscape(landscape, 10)
	if len(gaps) != 0 {
		t.Errorf("expected 0 gaps for all high-density clusters, got %d", len(gaps))
	}
}

func TestIdentifyGapsFromLandscape_MaxResultsCap(t *testing.T) {
	clusters := make([]LandscapeCluster, 30)
	for i := 0; i < 30; i++ {
		clusters[i] = LandscapeCluster{
			ID:      fmt.Sprintf("c%d", i),
			Label:   fmt.Sprintf("Area %d", i),
			Density: 0.01 * float64(i+1), // all < 0.30
		}
	}
	landscape := &LandscapeData{TotalPatents: 1000, Clusters: clusters}

	gaps := identifyGapsFromLandscape(landscape, 5)
	if len(gaps) != 5 {
		t.Errorf("expected 5 gaps after cap, got %d", len(gaps))
	}
}

func TestSortOpportunitiesByScore(t *testing.T) {
	opps := []WhiteSpaceOpportunity{
		{ID: "a", InnovationScore: 0.50},
		{ID: "b", InnovationScore: 0.90},
		{ID: "c", InnovationScore: 0.70},
	}
	sortOpportunitiesByScore(opps)

	if opps[0].ID != "b" {
		t.Errorf("expected 'b' first, got %s", opps[0].ID)
	}
	if opps[1].ID != "c" {
		t.Errorf("expected 'c' second, got %s", opps[1].ID)
	}
	if opps[2].ID != "a" {
		t.Errorf("expected 'a' third, got %s", opps[2].ID)
	}
}

func TestSortOpportunitiesByScore_Empty(t *testing.T) {
	// Should not panic
	sortOpportunitiesByScore(nil)
	sortOpportunitiesByScore([]WhiteSpaceOpportunity{})
}

func TestSortOpportunitiesByScore_Single(t *testing.T) {
	opps := []WhiteSpaceOpportunity{{ID: "only", InnovationScore: 0.80}}
	sortOpportunitiesByScore(opps)
	if opps[0].ID != "only" {
		t.Error("single element should remain unchanged")
	}
}

func TestGenerateWhiteSpaceID(t *testing.T) {
	id1 := generateWhiteSpaceID()
	id2 := generateWhiteSpaceID()

	if id1 == "" {
		t.Error("expected non-empty ID")
	}
	if id1 == id2 {
		t.Error("expected unique IDs")
	}
	if len(id1) < 5 {
		t.Error("expected ID with reasonable length")
	}
}

// suppress unused import
var _ = fmt.Sprintf

//Personal.AI order the ending

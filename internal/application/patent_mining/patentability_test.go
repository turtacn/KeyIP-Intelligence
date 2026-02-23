// Phase 10 - File 217 of 349
// Phase: 应用层 - 业务服务
// SubModule: patent_mining
// File: internal/application/patent_mining/patentability_test.go

package patent_mining

import (
	"context"
	"errors"
	"testing"

	apperrors "github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// ---------------------------------------------------------------------------
// Mocks
// ---------------------------------------------------------------------------

type mockPriorArtSearcher struct {
	searchByMoleculeFn func(ctx context.Context, smiles string, inchiKey string, maxResults int) ([]PriorArtReference, error)
	searchByTextFn     func(ctx context.Context, query string, techField string, maxResults int) ([]PriorArtReference, error)
}

func (m *mockPriorArtSearcher) SearchByMolecule(ctx context.Context, smiles string, inchiKey string, maxResults int) ([]PriorArtReference, error) {
	if m.searchByMoleculeFn != nil {
		return m.searchByMoleculeFn(ctx, smiles, inchiKey, maxResults)
	}
	return nil, nil
}

func (m *mockPriorArtSearcher) SearchByText(ctx context.Context, query string, techField string, maxResults int) ([]PriorArtReference, error) {
	if m.searchByTextFn != nil {
		return m.searchByTextFn(ctx, query, techField, maxResults)
	}
	return nil, nil
}

type mockRuleEngine struct {
	evaluateNoveltyFn       func(ctx context.Context, subject string, priorArts []PriorArtReference) (*DimensionScore, error)
	evaluateInventiveStepFn func(ctx context.Context, subject string, priorArts []PriorArtReference) (*DimensionScore, error)
	evaluateUtilityFn       func(ctx context.Context, subject string, claimedUse string) (*DimensionScore, error)
}

func (m *mockRuleEngine) EvaluateNovelty(ctx context.Context, subject string, priorArts []PriorArtReference) (*DimensionScore, error) {
	if m.evaluateNoveltyFn != nil {
		return m.evaluateNoveltyFn(ctx, subject, priorArts)
	}
	return &DimensionScore{Dimension: DimensionNovelty, Score: 0.80, Confidence: 0.90, Reasoning: "novel"}, nil
}

func (m *mockRuleEngine) EvaluateInventiveStep(ctx context.Context, subject string, priorArts []PriorArtReference) (*DimensionScore, error) {
	if m.evaluateInventiveStepFn != nil {
		return m.evaluateInventiveStepFn(ctx, subject, priorArts)
	}
	return &DimensionScore{Dimension: DimensionInventive, Score: 0.75, Confidence: 0.85, Reasoning: "inventive"}, nil
}

func (m *mockRuleEngine) EvaluateUtility(ctx context.Context, subject string, claimedUse string) (*DimensionScore, error) {
	if m.evaluateUtilityFn != nil {
		return m.evaluateUtilityFn(ctx, subject, claimedUse)
	}
	return &DimensionScore{Dimension: DimensionUtility, Score: 0.90, Confidence: 0.95, Reasoning: "useful"}, nil
}

type mockMolRepoForPatentability struct {
	getByIDFn    func(ctx context.Context, id string) (*MoleculeRef, error)
	getBySMILESFn func(ctx context.Context, smiles string) (*MoleculeRef, error)
}

func (m *mockMolRepoForPatentability) GetByID(ctx context.Context, id string) (*MoleculeRef, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, id)
	}
	return nil, apperrors.NewNotFoundError("molecule", id)
}

func (m *mockMolRepoForPatentability) GetBySMILES(ctx context.Context, smiles string) (*MoleculeRef, error) {
	if m.getBySMILESFn != nil {
		return m.getBySMILESFn(ctx, smiles)
	}
	return nil, apperrors.NewNotFoundError("molecule", smiles)
}

type mockAssessmentReportStore struct {
	saveFn func(ctx context.Context, assessment *PatentabilityAssessment) error
	getFn  func(ctx context.Context, id string) (*PatentabilityAssessment, error)
}

func (m *mockAssessmentReportStore) Save(ctx context.Context, assessment *PatentabilityAssessment) error {
	if m.saveFn != nil {
		return m.saveFn(ctx, assessment)
	}
	return nil
}

func (m *mockAssessmentReportStore) Get(ctx context.Context, id string) (*PatentabilityAssessment, error) {
	if m.getFn != nil {
		return m.getFn(ctx, id)
	}
	return nil, apperrors.NewNotFoundError("assessment", id)
}

type mockPatentabilityLogger struct{}

func (m *mockPatentabilityLogger) Info(msg string, fields ...interface{})  {}
func (m *mockPatentabilityLogger) Error(msg string, fields ...interface{}) {}
func (m *mockPatentabilityLogger) Warn(msg string, fields ...interface{})  {}
func (m *mockPatentabilityLogger) Debug(msg string, fields ...interface{}) {}

// ---------------------------------------------------------------------------
// Helper
// ---------------------------------------------------------------------------

func newTestPatentabilityService(
	searcher PriorArtSearcher,
	engine PatentabilityRuleEngine,
	molRepo MoleculeRepoForPatentability,
	store AssessmentReportStore,
) PatentabilityService {
	return NewPatentabilityService(PatentabilityDeps{
		PriorArtSearcher: searcher,
		RuleEngine:       engine,
		MolRepo:          molRepo,
		ReportStore:      store,
		Logger:           &mockPatentabilityLogger{},
	})
}

func defaultPriorArts() []PriorArtReference {
	return []PriorArtReference{
		{PatentNumber: "CN114000001A", Title: "Related OLED compound", Relevance: 0.72, MatchType: "similar"},
		{PatentNumber: "US20220001234A1", Title: "Carbazole derivative", Relevance: 0.65, MatchType: "related"},
	}
}

// ===========================================================================
// Tests: AssessMolecule
// ===========================================================================

func TestAssessMolecule_Success(t *testing.T) {
	searcher := &mockPriorArtSearcher{
		searchByMoleculeFn: func(ctx context.Context, smiles string, inchiKey string, maxResults int) ([]PriorArtReference, error) {
			return defaultPriorArts(), nil
		},
	}

	molRepo := &mockMolRepoForPatentability{
		getByIDFn: func(ctx context.Context, id string) (*MoleculeRef, error) {
			return &MoleculeRef{ID: id, SMILES: "c1ccc2c(c1)[nH]c1ccccc12", InChIKey: "TVFDJXOCBHFTFK-UHFFFAOYSA-N", Name: "Carbazole"}, nil
		},
	}

	store := &mockAssessmentReportStore{}
	engine := &mockRuleEngine{}

	svc := newTestPatentabilityService(searcher, engine, molRepo, store)

	req := &AssessMoleculeRequest{
		MoleculeID:   "mol-001",
		ClaimedUse:   "OLED host material",
		Jurisdiction: "CN",
		Options:      AssessmentOptions{MaxPriorArtResults: 10, IncludeReasoning: true},
	}

	result, err := svc.AssessMolecule(context.Background(), req)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.SubjectType != "molecule" {
		t.Errorf("expected subject_type molecule, got %s", result.SubjectType)
	}
	if len(result.Dimensions) != 3 {
		t.Errorf("expected 3 dimensions, got %d", len(result.Dimensions))
	}
	if result.OverallScore <= 0 {
		t.Error("expected positive overall score")
	}
	if result.Grade == "" {
		t.Error("expected non-empty grade")
	}
	if result.Jurisdiction != "CN" {
		t.Errorf("expected jurisdiction CN, got %s", result.Jurisdiction)
	}
}

func TestAssessMolecule_NilRequest(t *testing.T) {
	svc := newTestPatentabilityService(
		&mockPriorArtSearcher{},
		&mockRuleEngine{},
		&mockMolRepoForPatentability{},
		&mockAssessmentReportStore{},
	)

	_, err := svc.AssessMolecule(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil request")
	}
}

func TestAssessMolecule_NoIdentifier(t *testing.T) {
	svc := newTestPatentabilityService(
		&mockPriorArtSearcher{},
		&mockRuleEngine{},
		&mockMolRepoForPatentability{},
		&mockAssessmentReportStore{},
	)

	req := &AssessMoleculeRequest{ClaimedUse: "test"}
	_, err := svc.AssessMolecule(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for missing identifier")
	}
}

func TestAssessMolecule_BySMILES(t *testing.T) {
	searcher := &mockPriorArtSearcher{
		searchByMoleculeFn: func(ctx context.Context, smiles string, inchiKey string, maxResults int) ([]PriorArtReference, error) {
			return nil, nil
		},
	}

	svc := newTestPatentabilityService(searcher, &mockRuleEngine{}, &mockMolRepoForPatentability{}, &mockAssessmentReportStore{})

	req := &AssessMoleculeRequest{
		SMILES:     "c1ccccc1",
		ClaimedUse: "solvent",
	}

	result, err := svc.AssessMolecule(context.Background(), req)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if result.SubjectDesc != "c1ccccc1" {
		t.Errorf("expected SMILES as subject desc, got %s", result.SubjectDesc)
	}
}

func TestAssessMolecule_MoleculeNotFound(t *testing.T) {
	molRepo := &mockMolRepoForPatentability{
		getByIDFn: func(ctx context.Context, id string) (*MoleculeRef, error) {
			return nil, apperrors.NewNotFoundError("molecule", id)
		},
	}

	svc := newTestPatentabilityService(&mockPriorArtSearcher{}, &mockRuleEngine{}, molRepo, &mockAssessmentReportStore{})

	req := &AssessMoleculeRequest{MoleculeID: "nonexistent"}
	_, err := svc.AssessMolecule(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for nonexistent molecule")
	}
}

func TestAssessMolecule_PriorArtSearchError(t *testing.T) {
	searcher := &mockPriorArtSearcher{
		searchByMoleculeFn: func(ctx context.Context, smiles string, inchiKey string, maxResults int) ([]PriorArtReference, error) {
			return nil, errors.New("search engine unavailable")
		},
	}

	svc := newTestPatentabilityService(searcher, &mockRuleEngine{}, &mockMolRepoForPatentability{}, &mockAssessmentReportStore{})

	req := &AssessMoleculeRequest{SMILES: "c1ccccc1", ClaimedUse: "test"}
	_, err := svc.AssessMolecule(context.Background(), req)
	if err == nil {
		t.Fatal("expected error from prior art search failure")
	}
}

func TestAssessMolecule_RuleEngineError(t *testing.T) {
	searcher := &mockPriorArtSearcher{
		searchByMoleculeFn: func(ctx context.Context, smiles string, inchiKey string, maxResults int) ([]PriorArtReference, error) {
			return nil, nil
		},
	}

	engine := &mockRuleEngine{
		evaluateNoveltyFn: func(ctx context.Context, subject string, priorArts []PriorArtReference) (*DimensionScore, error) {
			return nil, errors.New("rule engine failure")
		},
	}

	svc := newTestPatentabilityService(searcher, engine, &mockMolRepoForPatentability{}, &mockAssessmentReportStore{})

	req := &AssessMoleculeRequest{SMILES: "c1ccccc1", ClaimedUse: "test"}
	_, err := svc.AssessMolecule(context.Background(), req)
	if err == nil {
		t.Fatal("expected error from rule engine failure")
	}
}

func TestAssessMolecule_HighlyPatentable(t *testing.T) {
	searcher := &mockPriorArtSearcher{
		searchByMoleculeFn: func(ctx context.Context, smiles string, inchiKey string, maxResults int) ([]PriorArtReference, error) {
			return nil, nil // no prior art
		},
	}

	engine := &mockRuleEngine{
		evaluateNoveltyFn: func(ctx context.Context, subject string, priorArts []PriorArtReference) (*DimensionScore, error) {
			return &DimensionScore{Dimension: DimensionNovelty, Score: 0.95, Confidence: 0.95}, nil
		},
		evaluateInventiveStepFn: func(ctx context.Context, subject string, priorArts []PriorArtReference) (*DimensionScore, error) {
			return &DimensionScore{Dimension: DimensionInventive, Score: 0.90, Confidence: 0.90}, nil
		},
		evaluateUtilityFn: func(ctx context.Context, subject string, claimedUse string) (*DimensionScore, error) {
			return &DimensionScore{Dimension: DimensionUtility, Score: 0.95, Confidence: 0.95}, nil
		},
	}

	svc := newTestPatentabilityService(searcher, engine, &mockMolRepoForPatentability{}, &mockAssessmentReportStore{})

	req := &AssessMoleculeRequest{SMILES: "C1=CC=C(C=C1)N(C2=CC=CC=C2)C3=CC=CC=C3", ClaimedUse: "novel HTL material"}
	result, err := svc.AssessMolecule(context.Background(), req)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if result.Grade != GradeHighlyPatentable {
		t.Errorf("expected grade highly_patentable, got %s", result.Grade)
	}
}

func TestAssessMolecule_NotPatentable(t *testing.T) {
	searcher := &mockPriorArtSearcher{
		searchByMoleculeFn: func(ctx context.Context, smiles string, inchiKey string, maxResults int) ([]PriorArtReference, error) {
			return []PriorArtReference{{PatentNumber: "CN100000001A", Relevance: 0.99, MatchType: "exact"}}, nil
		},
	}

	engine := &mockRuleEngine{
		evaluateNoveltyFn: func(ctx context.Context, subject string, priorArts []PriorArtReference) (*DimensionScore, error) {
			return &DimensionScore{Dimension: DimensionNovelty, Score: 0.10, Confidence: 0.95}, nil
		},
		evaluateInventiveStepFn: func(ctx context.Context, subject string, priorArts []PriorArtReference) (*DimensionScore, error) {
			return &DimensionScore{Dimension: DimensionInventive, Score: 0.15, Confidence: 0.90}, nil
		},
		evaluateUtilityFn: func(ctx context.Context, subject string, claimedUse string) (*DimensionScore, error) {
			return &DimensionScore{Dimension: DimensionUtility, Score: 0.50, Confidence: 0.80}, nil
		},
	}

	svc := newTestPatentabilityService(searcher, engine, &mockMolRepoForPatentability{}, &mockAssessmentReportStore{})

	req := &AssessMoleculeRequest{SMILES: "c1ccccc1", ClaimedUse: "solvent"}
	result, err := svc.AssessMolecule(context.Background(), req)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if result.Grade != GradeNotPatentable {
		t.Errorf("expected grade not_patentable, got %s", result.Grade)
	}
}

// ===========================================================================
// Tests: AssessTechnicalSolution
// ===========================================================================

func TestAssessTechnicalSolution_Success(t *testing.T) {
	searcher := &mockPriorArtSearcher{
		searchByTextFn: func(ctx context.Context, query string, techField string, maxResults int) ([]PriorArtReference, error) {
			return defaultPriorArts(), nil
		},
	}

	svc := newTestPatentabilityService(searcher, &mockRuleEngine{}, &mockMolRepoForPatentability{}, &mockAssessmentReportStore{})

	req := &AssessTechnicalSolutionRequest{
		Title:        "Novel OLED Device Architecture",
		Description:  "A multi-layer OLED device with improved efficiency using carbazole-based host materials.",
		Claims:       []string{"An OLED device comprising a carbazole-based host layer..."},
		TechField:    "organic_electronics",
		Jurisdiction: "CN",
	}

	result, err := svc.AssessTechnicalSolution(context.Background(), req)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if result.SubjectType != "technical_solution" {
		t.Errorf("expected subject_type technical_solution, got %s", result.SubjectType)
	}
	if len(result.Dimensions) != 3 {
		t.Errorf("expected 3 dimensions, got %d", len(result.Dimensions))
	}
}

func TestAssessTechnicalSolution_NilRequest(t *testing.T) {
	svc := newTestPatentabilityService(&mockPriorArtSearcher{}, &mockRuleEngine{}, &mockMolRepoForPatentability{}, &mockAssessmentReportStore{})

	_, err := svc.AssessTechnicalSolution(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil request")
	}
}

func TestAssessTechnicalSolution_EmptyDescription(t *testing.T) {
	svc := newTestPatentabilityService(&mockPriorArtSearcher{}, &mockRuleEngine{}, &mockMolRepoForPatentability{}, &mockAssessmentReportStore{})

	req := &AssessTechnicalSolutionRequest{Title: "Test", Claims: []string{"claim"}}
	_, err := svc.AssessTechnicalSolution(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for empty description")
	}
}

func TestAssessTechnicalSolution_NoClaims(t *testing.T) {
	svc := newTestPatentabilityService(&mockPriorArtSearcher{}, &mockRuleEngine{}, &mockMolRepoForPatentability{}, &mockAssessmentReportStore{})

	req := &AssessTechnicalSolutionRequest{Description: "some desc", Claims: []string{}}
	_, err := svc.AssessTechnicalSolution(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for empty claims")
	}
}

// ===========================================================================
// Tests: BatchAssess
// ===========================================================================

func TestBatchAssess_Success(t *testing.T) {
	searcher := &mockPriorArtSearcher{
		searchByMoleculeFn: func(ctx context.Context, smiles string, inchiKey string, maxResults int) ([]PriorArtReference, error) {
			return nil, nil
		},
	}

	molRepo := &mockMolRepoForPatentability{
		getByIDFn: func(ctx context.Context, id string) (*MoleculeRef, error) {
			return &MoleculeRef{ID: id, SMILES: "c1ccccc1"}, nil
		},
	}

	svc := newTestPatentabilityService(searcher, &mockRuleEngine{}, molRepo, &mockAssessmentReportStore{})

	req := &BatchAssessRequest{
		MoleculeIDs:  []string{"mol-1", "mol-2"},
		ClaimedUse:   "OLED host",
		Jurisdiction: "CN",
	}

	result, err := svc.BatchAssess(context.Background(), req)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if result.TotalProcessed != 2 {
		t.Errorf("expected TotalProcessed=2, got %d", result.TotalProcessed)
	}
	if result.SuccessCount != 2 {
		t.Errorf("expected SuccessCount=2, got %d", result.SuccessCount)
	}
}

func TestBatchAssess_PartialFailure(t *testing.T) {
	searcher := &mockPriorArtSearcher{
		searchByMoleculeFn: func(ctx context.Context, smiles string, inchiKey string, maxResults int) ([]PriorArtReference, error) {
			return nil, nil
		},
	}

	molRepo := &mockMolRepoForPatentability{
		getByIDFn: func(ctx context.Context, id string) (*MoleculeRef, error) {
			if id == "bad" {
				return nil, apperrors.NewNotFoundError("molecule", id)
			}
			return &MoleculeRef{ID: id, SMILES: "c1ccccc1"}, nil
		},
	}

	svc := newTestPatentabilityService(searcher, &mockRuleEngine{}, molRepo, &mockAssessmentReportStore{})

	req := &BatchAssessRequest{MoleculeIDs: []string{"good", "bad"}, ClaimedUse: "test"}
	result, err := svc.BatchAssess(context.Background(), req)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if result.SuccessCount != 1 {
		t.Errorf("expected SuccessCount=1, got %d", result.SuccessCount)
	}
	if result.FailedCount != 1 {
		t.Errorf("expected FailedCount=1, got %d", result.FailedCount)
	}
}

func TestBatchAssess_EmptyInput(t *testing.T) {
	svc := newTestPatentabilityService(&mockPriorArtSearcher{}, &mockRuleEngine{}, &mockMolRepoForPatentability{}, &mockAssessmentReportStore{})

	req := &BatchAssessRequest{MoleculeIDs: []string{}}
	_, err := svc.BatchAssess(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for empty input")
	}
}

// ===========================================================================
// Tests: GetAssessmentReport
// ===========================================================================

func TestGetAssessmentReport_Success(t *testing.T) {
	expected := &PatentabilityAssessment{
		ID:           "assess-001",
		SubjectType:  "molecule",
		OverallScore: 0.82,
		Grade:        GradePatentable,
	}

	store := &mockAssessmentReportStore{
		getFn: func(ctx context.Context, id string) (*PatentabilityAssessment, error) {
			if id == "assess-001" {
				return expected, nil
			}
			return nil, apperrors.NewNotFoundError("assessment", id)
		},
	}

	svc := newTestPatentabilityService(&mockPriorArtSearcher{}, &mockRuleEngine{}, &mockMolRepoForPatentability{}, store)

	result, err := svc.GetAssessmentReport(context.Background(), "assess-001")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if result.ID != "assess-001" {
		t.Errorf("expected ID assess-001, got %s", result.ID)
	}
	if result.Grade != GradePatentable {
		t.Errorf("expected grade patentable, got %s", result.Grade)
	}
}

func TestGetAssessmentReport_NotFound(t *testing.T) {
	svc := newTestPatentabilityService(&mockPriorArtSearcher{}, &mockRuleEngine{}, &mockMolRepoForPatentability{}, &mockAssessmentReportStore{})

	_, err := svc.GetAssessmentReport(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent report")
	}
}

func TestGetAssessmentReport_EmptyID(t *testing.T) {
	svc := newTestPatentabilityService(&mockPriorArtSearcher{}, &mockRuleEngine{}, &mockMolRepoForPatentability{}, &mockAssessmentReportStore{})

	_, err := svc.GetAssessmentReport(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty ID")
	}
}

// ===========================================================================
// Tests: Helper functions
// ===========================================================================

func TestScoreToGrade(t *testing.T) {
	tests := []struct {
		score float64
		want  PatentabilityGrade
	}{
		{0.95, GradeHighlyPatentable},
		{0.85, GradeHighlyPatentable},
		{0.75, GradePatentable},
		{0.70, GradePatentable},
		{0.55, GradeBorderline},
		{0.50, GradeBorderline},
		{0.35, GradeUnlikely},
		{0.30, GradeUnlikely},
		{0.20, GradeNotPatentable},
		{0.0, GradeNotPatentable},
	}

	for _, tt := range tests {
		got := scoreToGrade(tt.score)
		if got != tt.want {
			t.Errorf("scoreToGrade(%v) = %s, want %s", tt.score, got, tt.want)
		}
	}
}

func TestComputeOverallScore(t *testing.T) {
	dims := []DimensionScore{
		{Dimension: DimensionNovelty, Score: 1.0},
		{Dimension: DimensionInventive, Score: 1.0},
		{Dimension: DimensionUtility, Score: 1.0},
	}
	score := computeOverallScore(dims)
	if score < 0.99 || score > 1.01 {
		t.Errorf("expected ~1.0 for all perfect scores, got %f", score)
	}

	empty := computeOverallScore(nil)
	if empty != 0 {
		t.Errorf("expected 0 for empty dimensions, got %f", empty)
	}
}

func TestFilterPriorArtsByRelevance(t *testing.T) {
	arts := []PriorArtReference{
		{PatentNumber: "A", Relevance: 0.90},
		{PatentNumber: "B", Relevance: 0.50},
		{PatentNumber: "C", Relevance: 0.30},
	}

	filtered := filterPriorArtsByRelevance(arts, 0.60)
	if len(filtered) != 1 {
		t.Errorf("expected 1 art above 0.60, got %d", len(filtered))
	}
	if filtered[0].PatentNumber != "A" {
		t.Errorf("expected patent A, got %s", filtered[0].PatentNumber)
	}

	all := filterPriorArtsByRelevance(arts, 0.0)
	if len(all) != 3 {
		t.Errorf("expected 3 arts with min 0.0, got %d", len(all))
	}
}

func TestFindWeakestDimension(t *testing.T) {
	dims := []DimensionScore{
		{Dimension: DimensionNovelty, Score: 0.80},
		{Dimension: DimensionInventive, Score: 0.40},
		{Dimension: DimensionUtility, Score: 0.90},
	}
	weakest := findWeakestDimension(dims)
	if weakest != DimensionInventive {
		t.Errorf("expected inventive_step as weakest, got %s", weakest)
	}

	empty := findWeakestDimension(nil)
	if empty != DimensionNovelty {
		t.Errorf("expected novelty as default for empty, got %s", empty)
	}
}

func TestMin(t *testing.T) {
	if min(3, 5) != 3 {
		t.Error("min(3,5) should be 3")
	}
	if min(10, 2) != 2 {
		t.Error("min(10,2) should be 2")
	}
	if min(4, 4) != 4 {
		t.Error("min(4,4) should be 4")
	}
}

//Personal.AI order the ending


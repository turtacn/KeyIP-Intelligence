// Phase 10 - File 216 of 349
// Phase: 应用层 - 业务服务
// SubModule: patent_mining
// File: internal/application/patent_mining/patentability.go
//
// Generation Plan:
// - 功能定位: 可专利性评估应用服务，对候选分子/技术方案进行新颖性、创造性、实用性三维度评估
// - 核心实现:
//   - PatentabilityService 接口: AssessMolecule / AssessTechnicalSolution / BatchAssess / GetAssessmentReport
//   - patentabilityServiceImpl 结构体: 注入 PriorArtSearcher, RuleEngine, MoleculeRepo, ReportStore, Logger
//   - 新颖性评估: 基于现有技术检索结果判断候选方案是否已被公开
//   - 创造性评估: 对比最近似现有技术，评估技术贡献度
//   - 实用性评估: 验证技术方案是否具有实际工业应用价值
//   - 综合评分: 加权聚合三维度得分，生成可专利性等级
// - 依赖: pkg/errors, pkg/types, internal/domain
// - 被依赖: API handler, CLI commands
// - 强制约束: 文件最后一行必须为 //Personal.AI order the ending

package patent_mining

import (
	"context"
	"fmt"
	"time"

	apperrors "github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// ---------------------------------------------------------------------------
// Types: Assessment domain models
// ---------------------------------------------------------------------------

// PatentabilityDimension represents one of the three patentability dimensions.
type PatentabilityDimension string

const (
	DimensionNovelty     PatentabilityDimension = "novelty"
	DimensionInventive   PatentabilityDimension = "inventive_step"
	DimensionUtility     PatentabilityDimension = "utility"
)

// PatentabilityGrade represents the overall patentability grade.
type PatentabilityGrade string

const (
	GradeHighlyPatentable PatentabilityGrade = "highly_patentable"
	GradePatentable       PatentabilityGrade = "patentable"
	GradeBorderline       PatentabilityGrade = "borderline"
	GradeUnlikely         PatentabilityGrade = "unlikely"
	GradeNotPatentable    PatentabilityGrade = "not_patentable"
)

// DimensionScore holds the score and reasoning for a single dimension.
type DimensionScore struct {
	Dimension  PatentabilityDimension `json:"dimension"`
	Score      float64                `json:"score"`       // 0.0 - 1.0
	Confidence float64                `json:"confidence"`  // 0.0 - 1.0
	Reasoning  string                 `json:"reasoning"`
	PriorArts  []PriorArtReference    `json:"prior_arts,omitempty"`
}

// PriorArtReference is a reference to a prior art document found during search.
type PriorArtReference struct {
	PatentNumber string  `json:"patent_number"`
	Title        string  `json:"title"`
	Relevance    float64 `json:"relevance"`
	MatchType    string  `json:"match_type"` // "exact", "similar", "related"
	Snippet      string  `json:"snippet,omitempty"`
}

// PatentabilityAssessment is the full assessment result.
type PatentabilityAssessment struct {
	ID              string             `json:"id"`
	SubjectType     string             `json:"subject_type"` // "molecule", "technical_solution"
	SubjectID       string             `json:"subject_id"`
	SubjectDesc     string             `json:"subject_description"`
	Dimensions      []DimensionScore   `json:"dimensions"`
	OverallScore    float64            `json:"overall_score"`
	Grade           PatentabilityGrade `json:"grade"`
	Recommendation  string             `json:"recommendation"`
	Jurisdiction    string             `json:"jurisdiction"`
	AssessedAt      time.Time          `json:"assessed_at"`
	ProcessingTimeMs int64             `json:"processing_time_ms"`
}

// AssessMoleculeRequest is the request for molecule patentability assessment.
type AssessMoleculeRequest struct {
	MoleculeID    string            `json:"molecule_id"`
	SMILES        string            `json:"smiles,omitempty"`
	InChIKey      string            `json:"inchi_key,omitempty"`
	ClaimedUse    string            `json:"claimed_use"`
	Jurisdiction  string            `json:"jurisdiction"`
	Options       AssessmentOptions `json:"options"`
}

// AssessTechnicalSolutionRequest is the request for technical solution assessment.
type AssessTechnicalSolutionRequest struct {
	Title         string            `json:"title"`
	Description   string            `json:"description"`
	Claims        []string          `json:"claims"`
	TechField     string            `json:"tech_field"`
	Jurisdiction  string            `json:"jurisdiction"`
	Options       AssessmentOptions `json:"options"`
}

// AssessmentOptions configures the assessment behavior.
type AssessmentOptions struct {
	MaxPriorArtResults int     `json:"max_prior_art_results"`
	MinRelevanceScore  float64 `json:"min_relevance_score"`
	IncludeReasoning   bool    `json:"include_reasoning"`
	DeepAnalysis       bool    `json:"deep_analysis"`
}

// BatchAssessRequest is the request for batch assessment.
type BatchAssessRequest struct {
	MoleculeIDs  []string          `json:"molecule_ids"`
	ClaimedUse   string            `json:"claimed_use"`
	Jurisdiction string            `json:"jurisdiction"`
	Options      AssessmentOptions `json:"options"`
}

// BatchAssessResult is the result of batch assessment.
type BatchAssessResult struct {
	Results        []PatentabilityAssessment `json:"results"`
	TotalProcessed int                       `json:"total_processed"`
	SuccessCount   int                       `json:"success_count"`
	FailedCount    int                       `json:"failed_count"`
	Errors         []BatchAssessError        `json:"errors,omitempty"`
}

// BatchAssessError records a single failure in batch processing.
type BatchAssessError struct {
	MoleculeID string `json:"molecule_id"`
	Error      string `json:"error"`
}

// ---------------------------------------------------------------------------
// Port interfaces (driven side)
// ---------------------------------------------------------------------------

// PriorArtSearcher searches for prior art relevant to a given subject.
type PriorArtSearcher interface {
	SearchByMolecule(ctx context.Context, smiles string, inchiKey string, maxResults int) ([]PriorArtReference, error)
	SearchByText(ctx context.Context, query string, techField string, maxResults int) ([]PriorArtReference, error)
}

// PatentabilityRuleEngine evaluates patentability dimensions using rules.
type PatentabilityRuleEngine interface {
	EvaluateNovelty(ctx context.Context, subject string, priorArts []PriorArtReference) (*DimensionScore, error)
	EvaluateInventiveStep(ctx context.Context, subject string, priorArts []PriorArtReference) (*DimensionScore, error)
	EvaluateUtility(ctx context.Context, subject string, claimedUse string) (*DimensionScore, error)
}

// MoleculeRepoForPatentability provides molecule lookup for assessment.
type MoleculeRepoForPatentability interface {
	GetByID(ctx context.Context, id string) (*MoleculeRef, error)
	GetBySMILES(ctx context.Context, smiles string) (*MoleculeRef, error)
}

// MoleculeRef is a lightweight molecule reference for assessment.
type MoleculeRef struct {
	ID       string `json:"id"`
	SMILES   string `json:"smiles"`
	InChIKey string `json:"inchi_key"`
	Name     string `json:"name"`
}

// AssessmentReportStore persists and retrieves assessment reports.
type AssessmentReportStore interface {
	Save(ctx context.Context, assessment *PatentabilityAssessment) error
	Get(ctx context.Context, id string) (*PatentabilityAssessment, error)
}

// PatentabilityLogger abstracts logging for the service.
type PatentabilityLogger interface {
	Info(msg string, fields ...interface{})
	Error(msg string, fields ...interface{})
	Warn(msg string, fields ...interface{})
	Debug(msg string, fields ...interface{})
}

// ---------------------------------------------------------------------------
// Service interface
// ---------------------------------------------------------------------------

// PatentabilityService provides patentability assessment capabilities.
type PatentabilityService interface {
	AssessMolecule(ctx context.Context, req *AssessMoleculeRequest) (*PatentabilityAssessment, error)
	AssessTechnicalSolution(ctx context.Context, req *AssessTechnicalSolutionRequest) (*PatentabilityAssessment, error)
	BatchAssess(ctx context.Context, req *BatchAssessRequest) (*BatchAssessResult, error)
	GetAssessmentReport(ctx context.Context, assessmentID string) (*PatentabilityAssessment, error)
}

// ---------------------------------------------------------------------------
// Dependencies
// ---------------------------------------------------------------------------

// PatentabilityDeps holds all dependencies for the patentability service.
type PatentabilityDeps struct {
	PriorArtSearcher PriorArtSearcher
	RuleEngine       PatentabilityRuleEngine
	MolRepo          MoleculeRepoForPatentability
	ReportStore      AssessmentReportStore
	Logger           PatentabilityLogger
}

// ---------------------------------------------------------------------------
// Implementation
// ---------------------------------------------------------------------------

type patentabilityServiceImpl struct {
	searcher    PriorArtSearcher
	ruleEngine  PatentabilityRuleEngine
	molRepo     MoleculeRepoForPatentability
	reportStore AssessmentReportStore
	logger      PatentabilityLogger
}

// NewPatentabilityService creates a new PatentabilityService.
func NewPatentabilityService(deps PatentabilityDeps) PatentabilityService {
	return &patentabilityServiceImpl{
		searcher:    deps.PriorArtSearcher,
		ruleEngine:  deps.RuleEngine,
		molRepo:     deps.MolRepo,
		reportStore: deps.ReportStore,
		logger:      deps.Logger,
	}
}

func (s *patentabilityServiceImpl) AssessMolecule(ctx context.Context, req *AssessMoleculeRequest) (*PatentabilityAssessment, error) {
	if req == nil {
		return nil, apperrors.NewValidationError("request", "request cannot be nil")
	}

	startTime := time.Now()

	// Resolve molecule
	var mol *MoleculeRef
	var err error
	if req.MoleculeID != "" {
		mol, err = s.molRepo.GetByID(ctx, req.MoleculeID)
		if err != nil {
			return nil, fmt.Errorf("resolve molecule by ID: %w", err)
		}
	} else if req.SMILES != "" {
		mol = &MoleculeRef{SMILES: req.SMILES, InChIKey: req.InChIKey}
	} else {
		return nil, apperrors.NewValidationError("molecule", "molecule_id or smiles is required")
	}

	s.logger.Info("assessing molecule patentability", "smiles", mol.SMILES, "jurisdiction", req.Jurisdiction)

	// Search prior art
	maxResults := req.Options.MaxPriorArtResults
	if maxResults <= 0 {
		maxResults = 20
	}
	priorArts, err := s.searcher.SearchByMolecule(ctx, mol.SMILES, mol.InChIKey, maxResults)
	if err != nil {
		s.logger.Error("prior art search failed", "error", err)
		return nil, fmt.Errorf("prior art search: %w", err)
	}

	// Filter by relevance
	if req.Options.MinRelevanceScore > 0 {
		priorArts = filterPriorArtsByRelevance(priorArts, req.Options.MinRelevanceScore)
	}

	// Evaluate dimensions
	dimensions, err := s.evaluateAllDimensions(ctx, mol.SMILES, req.ClaimedUse, priorArts)
	if err != nil {
		return nil, fmt.Errorf("dimension evaluation: %w", err)
	}

	// Compute overall score and grade
	overallScore := computeOverallScore(dimensions)
	grade := scoreToGrade(overallScore)
	recommendation := generateRecommendation(grade, dimensions)

	assessment := &PatentabilityAssessment{
		ID:               generateAssessmentID(),
		SubjectType:      "molecule",
		SubjectID:        mol.ID,
		SubjectDesc:      mol.SMILES,
		Dimensions:       dimensions,
		OverallScore:     overallScore,
		Grade:            grade,
		Recommendation:   recommendation,
		Jurisdiction:     req.Jurisdiction,
		AssessedAt:       time.Now(),
		ProcessingTimeMs: time.Since(startTime).Milliseconds(),
	}

	if err := s.reportStore.Save(ctx, assessment); err != nil {
		s.logger.Warn("failed to persist assessment report", "error", err)
	}

	return assessment, nil
}

func (s *patentabilityServiceImpl) AssessTechnicalSolution(ctx context.Context, req *AssessTechnicalSolutionRequest) (*PatentabilityAssessment, error) {
	if req == nil {
		return nil, apperrors.NewValidationError("request", "request cannot be nil")
	}
	if req.Description == "" {
		return nil, apperrors.NewValidationError("description", "description is required")
	}
	if len(req.Claims) == 0 {
		return nil, apperrors.NewValidationError("claims", "at least one claim is required")
	}

	startTime := time.Now()

	s.logger.Info("assessing technical solution patentability", "title", req.Title, "jurisdiction", req.Jurisdiction)

	// Build search query from claims
	searchQuery := req.Title + " " + req.Description
	maxResults := req.Options.MaxPriorArtResults
	if maxResults <= 0 {
		maxResults = 20
	}

	priorArts, err := s.searcher.SearchByText(ctx, searchQuery, req.TechField, maxResults)
	if err != nil {
		s.logger.Error("prior art text search failed", "error", err)
		return nil, fmt.Errorf("prior art search: %w", err)
	}

	if req.Options.MinRelevanceScore > 0 {
		priorArts = filterPriorArtsByRelevance(priorArts, req.Options.MinRelevanceScore)
	}

	subjectDesc := req.Title
	if subjectDesc == "" {
		subjectDesc = req.Description[:min(100, len(req.Description))]
	}

	dimensions, err := s.evaluateAllDimensions(ctx, req.Description, req.Claims[0], priorArts)
	if err != nil {
		return nil, fmt.Errorf("dimension evaluation: %w", err)
	}

	overallScore := computeOverallScore(dimensions)
	grade := scoreToGrade(overallScore)
	recommendation := generateRecommendation(grade, dimensions)

	assessment := &PatentabilityAssessment{
		ID:               generateAssessmentID(),
		SubjectType:      "technical_solution",
		SubjectDesc:      subjectDesc,
		Dimensions:       dimensions,
		OverallScore:     overallScore,
		Grade:            grade,
		Recommendation:   recommendation,
		Jurisdiction:     req.Jurisdiction,
		AssessedAt:       time.Now(),
		ProcessingTimeMs: time.Since(startTime).Milliseconds(),
	}

	if err := s.reportStore.Save(ctx, assessment); err != nil {
		s.logger.Warn("failed to persist assessment report", "error", err)
	}

	return assessment, nil
}

func (s *patentabilityServiceImpl) BatchAssess(ctx context.Context, req *BatchAssessRequest) (*BatchAssessResult, error) {
	if req == nil || len(req.MoleculeIDs) == 0 {
		return nil, apperrors.NewValidationError("molecule_ids", "at least one molecule ID is required")
	}

	result := &BatchAssessResult{
		TotalProcessed: len(req.MoleculeIDs),
	}

	for _, molID := range req.MoleculeIDs {
		assessReq := &AssessMoleculeRequest{
			MoleculeID:   molID,
			ClaimedUse:   req.ClaimedUse,
			Jurisdiction: req.Jurisdiction,
			Options:      req.Options,
		}

		assessment, err := s.AssessMolecule(ctx, assessReq)
		if err != nil {
			result.FailedCount++
			result.Errors = append(result.Errors, BatchAssessError{
				MoleculeID: molID,
				Error:      err.Error(),
			})
			continue
		}

		result.Results = append(result.Results, *assessment)
		result.SuccessCount++
	}

	return result, nil
}

func (s *patentabilityServiceImpl) GetAssessmentReport(ctx context.Context, assessmentID string) (*PatentabilityAssessment, error) {
	if assessmentID == "" {
		return nil, apperrors.NewValidationError("assessment_id", "assessment ID is required")
	}
	return s.reportStore.Get(ctx, assessmentID)
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

func (s *patentabilityServiceImpl) evaluateAllDimensions(ctx context.Context, subject string, claimedUse string, priorArts []PriorArtReference) ([]DimensionScore, error) {
	novelty, err := s.ruleEngine.EvaluateNovelty(ctx, subject, priorArts)
	if err != nil {
		return nil, fmt.Errorf("novelty evaluation: %w", err)
	}

	inventive, err := s.ruleEngine.EvaluateInventiveStep(ctx, subject, priorArts)
	if err != nil {
		return nil, fmt.Errorf("inventive step evaluation: %w", err)
	}

	utility, err := s.ruleEngine.EvaluateUtility(ctx, subject, claimedUse)
	if err != nil {
		return nil, fmt.Errorf("utility evaluation: %w", err)
	}

	return []DimensionScore{*novelty, *inventive, *utility}, nil
}

func filterPriorArtsByRelevance(arts []PriorArtReference, minScore float64) []PriorArtReference {
	filtered := make([]PriorArtReference, 0, len(arts))
	for _, a := range arts {
		if a.Relevance >= minScore {
			filtered = append(filtered, a)
		}
	}
	return filtered
}

func computeOverallScore(dimensions []DimensionScore) float64 {
	if len(dimensions) == 0 {
		return 0
	}
	// Weighted: novelty 40%, inventive step 40%, utility 20%
	weights := map[PatentabilityDimension]float64{
		DimensionNovelty:   0.40,
		DimensionInventive: 0.40,
		DimensionUtility:   0.20,
	}
	var total float64
	var weightSum float64
	for _, d := range dimensions {
		w, ok := weights[d.Dimension]
		if !ok {
			w = 1.0 / float64(len(dimensions))
		}
		total += d.Score * w
		weightSum += w
	}
	if weightSum == 0 {
		return 0
	}
	return total / weightSum
}

func scoreToGrade(score float64) PatentabilityGrade {
	switch {
	case score >= 0.85:
		return GradeHighlyPatentable
	case score >= 0.70:
		return GradePatentable
	case score >= 0.50:
		return GradeBorderline
	case score >= 0.30:
		return GradeUnlikely
	default:
		return GradeNotPatentable
	}
}

func generateRecommendation(grade PatentabilityGrade, dimensions []DimensionScore) string {
	switch grade {
	case GradeHighlyPatentable:
		return "Strong patentability indicators across all dimensions. Recommend proceeding with patent application."
	case GradePatentable:
		return "Good patentability prospects. Consider strengthening weaker dimensions before filing."
	case GradeBorderline:
		weakest := findWeakestDimension(dimensions)
		return fmt.Sprintf("Borderline patentability. Focus on improving %s dimension before filing.", weakest)
	case GradeUnlikely:
		return "Low patentability likelihood. Significant prior art overlap detected. Consider alternative claims or modifications."
	default:
		return "Not patentable under current assessment. Major prior art conflicts identified."
	}
}

func findWeakestDimension(dimensions []DimensionScore) PatentabilityDimension {
	if len(dimensions) == 0 {
		return DimensionNovelty
	}
	weakest := dimensions[0]
	for _, d := range dimensions[1:] {
		if d.Score < weakest.Score {
			weakest = d
		}
	}
	return weakest.Dimension
}

var assessmentIDCounter int64

func generateAssessmentID() string {
	assessmentIDCounter++
	return fmt.Sprintf("assess-%d-%d", time.Now().UnixMilli(), assessmentIDCounter)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

//Personal.AI order the ending

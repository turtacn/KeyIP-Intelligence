// Phase 10 - File 220 of 349
// Phase: 应用层 - 业务服务
// SubModule: patent_mining
// File: internal/application/patent_mining/white_space.go
//
// Generation Plan:
// - 功能定位: 专利空白区分析应用服务，识别技术领域中尚未被专利覆盖的创新机会空间
// - 核心实现:
//   - WhiteSpaceService 接口: AnalyzeByTechField / AnalyzeByMoleculeClass / AnalyzeByPropertyRange / GetAnalysisReport / ListRecentAnalyses
//   - whiteSpaceServiceImpl 结构体: 注入 PatentLandscapeProvider, MoleculeSpaceAnalyzer, ReportStore, Logger
//   - 技术领域空白分析: 基于专利景观图谱识别低密度区域
//   - 分子类别空白分析: 在特定骨架/官能团空间中寻找未覆盖区域
//   - 性能区间空白分析: 在性能参数空间中识别未被探索的区间
// - 依赖: pkg/errors, pkg/types
// - 被依赖: API handler, reporting module
// - 强制约束: 文件最后一行必须为 //Personal.AI order the ending

package patent_mining

import (
	"context"
	"fmt"
	"time"

	apperrors "github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// WhiteSpaceType defines the type of white space analysis.
type WhiteSpaceType string

const (
	WhiteSpaceTechField     WhiteSpaceType = "tech_field"
	WhiteSpaceMoleculeClass WhiteSpaceType = "molecule_class"
	WhiteSpacePropertyRange WhiteSpaceType = "property_range"
)

// OpportunityLevel indicates the strength of an identified opportunity.
type OpportunityLevel string

const (
	OpportunityHigh   OpportunityLevel = "high"
	OpportunityMedium OpportunityLevel = "medium"
	OpportunityLow    OpportunityLevel = "low"
)

// WhiteSpaceOpportunity represents a single identified gap/opportunity.
type WhiteSpaceOpportunity struct {
	ID              string           `json:"id"`
	Description     string           `json:"description"`
	Level           OpportunityLevel `json:"level"`
	TechArea        string           `json:"tech_area"`
	GapDescription  string           `json:"gap_description"`
	PatentDensity   float64          `json:"patent_density"`   // 0.0 = empty, 1.0 = saturated
	InnovationScore float64          `json:"innovation_score"` // 0.0 - 1.0
	NearestPatents  []string         `json:"nearest_patents,omitempty"`
	SuggestedSMILES []string         `json:"suggested_smiles,omitempty"`
	Rationale       string           `json:"rationale"`
}

// WhiteSpaceAnalysisResult holds the full analysis result.
type WhiteSpaceAnalysisResult struct {
	ID               string                  `json:"id"`
	AnalysisType     WhiteSpaceType          `json:"analysis_type"`
	Query            string                  `json:"query"`
	Opportunities    []WhiteSpaceOpportunity `json:"opportunities"`
	TotalPatents     int                     `json:"total_patents_analyzed"`
	CoveragePercent  float64                 `json:"coverage_percent"`
	AnalyzedAt       time.Time               `json:"analyzed_at"`
	ProcessingTimeMs int64                   `json:"processing_time_ms"`
	Metadata         map[string]string       `json:"metadata,omitempty"`
}

// AnalyzeByTechFieldRequest is the request for tech field analysis.
type AnalyzeByTechFieldRequest struct {
	TechField    string   `json:"tech_field"`
	SubFields    []string `json:"sub_fields,omitempty"`
	Jurisdiction string   `json:"jurisdiction,omitempty"`
	YearFrom     int      `json:"year_from,omitempty"`
	YearTo       int      `json:"year_to,omitempty"`
	MaxResults   int      `json:"max_results"`
}

// AnalyzeByMoleculeClassRequest is the request for molecule class analysis.
type AnalyzeByMoleculeClassRequest struct {
	CoreScaffold   string   `json:"core_scaffold"`
	Substituents   []string `json:"substituents,omitempty"`
	TargetProperty string   `json:"target_property,omitempty"`
	MaxResults     int      `json:"max_results"`
}

// AnalyzeByPropertyRangeRequest is the request for property range analysis.
type AnalyzeByPropertyRangeRequest struct {
	PropertyName string  `json:"property_name"`
	MinValue     float64 `json:"min_value"`
	MaxValue     float64 `json:"max_value"`
	StepSize     float64 `json:"step_size"`
	TechField    string  `json:"tech_field"`
	MaxResults   int     `json:"max_results"`
}

// ---------------------------------------------------------------------------
// Port interfaces
// ---------------------------------------------------------------------------

// PatentLandscapeProvider provides patent landscape data for analysis.
type PatentLandscapeProvider interface {
	GetLandscapeByField(ctx context.Context, techField string, subFields []string, jurisdiction string, yearFrom int, yearTo int) (*LandscapeData, error)
	GetLandscapeByScaffold(ctx context.Context, scaffold string) (*LandscapeData, error)
	GetLandscapeByProperty(ctx context.Context, propertyName string, minVal float64, maxVal float64, techField string) (*LandscapeData, error)
}

// LandscapeData represents patent landscape information.
type LandscapeData struct {
	TotalPatents int                `json:"total_patents"`
	Clusters     []LandscapeCluster `json:"clusters"`
	Coverage     float64            `json:"coverage"`
}

// LandscapeCluster represents a cluster in the patent landscape.
type LandscapeCluster struct {
	ID          string   `json:"id"`
	Label       string   `json:"label"`
	PatentCount int      `json:"patent_count"`
	Density     float64  `json:"density"`
	Center      []float64 `json:"center,omitempty"`
	PatentIDs   []string `json:"patent_ids,omitempty"`
}

// MoleculeSpaceAnalyzer analyzes molecular space for gaps.
type MoleculeSpaceAnalyzer interface {
	FindGaps(ctx context.Context, scaffold string, substituents []string, targetProperty string) ([]WhiteSpaceOpportunity, error)
	FindPropertyGaps(ctx context.Context, propertyName string, minVal float64, maxVal float64, stepSize float64, techField string) ([]WhiteSpaceOpportunity, error)
}

// WhiteSpaceReportStore persists and retrieves analysis reports.
type WhiteSpaceReportStore interface {
	Save(ctx context.Context, result *WhiteSpaceAnalysisResult) error
	Get(ctx context.Context, id string) (*WhiteSpaceAnalysisResult, error)
	ListRecent(ctx context.Context, limit int) ([]WhiteSpaceAnalysisResult, error)
}

// WhiteSpaceLogger abstracts logging.
type WhiteSpaceLogger interface {
	Info(msg string, fields ...interface{})
	Error(msg string, fields ...interface{})
	Warn(msg string, fields ...interface{})
	Debug(msg string, fields ...interface{})
}

// ---------------------------------------------------------------------------
// Service interface
// ---------------------------------------------------------------------------

// WhiteSpaceService provides patent white space analysis capabilities.
type WhiteSpaceService interface {
	AnalyzeByTechField(ctx context.Context, req *AnalyzeByTechFieldRequest) (*WhiteSpaceAnalysisResult, error)
	AnalyzeByMoleculeClass(ctx context.Context, req *AnalyzeByMoleculeClassRequest) (*WhiteSpaceAnalysisResult, error)
	AnalyzeByPropertyRange(ctx context.Context, req *AnalyzeByPropertyRangeRequest) (*WhiteSpaceAnalysisResult, error)
	GetAnalysisReport(ctx context.Context, reportID string) (*WhiteSpaceAnalysisResult, error)
	ListRecentAnalyses(ctx context.Context, limit int) ([]WhiteSpaceAnalysisResult, error)
}

// ---------------------------------------------------------------------------
// Dependencies
// ---------------------------------------------------------------------------

// WhiteSpaceDeps holds all dependencies for the white space service.
type WhiteSpaceDeps struct {
	Landscape     PatentLandscapeProvider
	MolAnalyzer   MoleculeSpaceAnalyzer
	ReportStore   WhiteSpaceReportStore
	Logger        WhiteSpaceLogger
}

// ---------------------------------------------------------------------------
// Implementation
// ---------------------------------------------------------------------------

type whiteSpaceServiceImpl struct {
	landscape   PatentLandscapeProvider
	molAnalyzer MoleculeSpaceAnalyzer
	reportStore WhiteSpaceReportStore
	logger      WhiteSpaceLogger
}

// NewWhiteSpaceService creates a new WhiteSpaceService.
func NewWhiteSpaceService(deps WhiteSpaceDeps) WhiteSpaceService {
	return &whiteSpaceServiceImpl{
		landscape:   deps.Landscape,
		molAnalyzer: deps.MolAnalyzer,
		reportStore: deps.ReportStore,
		logger:      deps.Logger,
	}
}

func (s *whiteSpaceServiceImpl) AnalyzeByTechField(ctx context.Context, req *AnalyzeByTechFieldRequest) (*WhiteSpaceAnalysisResult, error) {
	if req == nil {
		return nil, apperrors.NewValidationError("request", "request cannot be nil")
	}
	if req.TechField == "" {
		return nil, apperrors.NewValidationError("tech_field", "tech_field is required")
	}

	startTime := time.Now()
	s.logger.Info("analyzing white space by tech field", "field", req.TechField)

	maxResults := req.MaxResults
	if maxResults <= 0 {
		maxResults = 20
	}

	landscape, err := s.landscape.GetLandscapeByField(ctx, req.TechField, req.SubFields, req.Jurisdiction, req.YearFrom, req.YearTo)
	if err != nil {
		s.logger.Error("landscape fetch failed", "error", err)
		return nil, fmt.Errorf("fetch landscape: %w", err)
	}

	opportunities := identifyGapsFromLandscape(landscape, maxResults)

	result := &WhiteSpaceAnalysisResult{
		ID:               generateWhiteSpaceID(),
		AnalysisType:     WhiteSpaceTechField,
		Query:            req.TechField,
		Opportunities:    opportunities,
		TotalPatents:     landscape.TotalPatents,
		CoveragePercent:  landscape.Coverage * 100,
		AnalyzedAt:       time.Now(),
		ProcessingTimeMs: time.Since(startTime).Milliseconds(),
		Metadata: map[string]string{
			"jurisdiction": req.Jurisdiction,
			"year_from":    fmt.Sprintf("%d", req.YearFrom),
			"year_to":      fmt.Sprintf("%d", req.YearTo),
		},
	}

	if err := s.reportStore.Save(ctx, result); err != nil {
		s.logger.Warn("failed to persist analysis report", "error", err)
	}

	return result, nil
}

func (s *whiteSpaceServiceImpl) AnalyzeByMoleculeClass(ctx context.Context, req *AnalyzeByMoleculeClassRequest) (*WhiteSpaceAnalysisResult, error) {
	if req == nil {
		return nil, apperrors.NewValidationError("request", "request cannot be nil")
	}
	if req.CoreScaffold == "" {
		return nil, apperrors.NewValidationError("core_scaffold", "core_scaffold is required")
	}

	startTime := time.Now()
	s.logger.Info("analyzing white space by molecule class", "scaffold", req.CoreScaffold)

	maxResults := req.MaxResults
	if maxResults <= 0 {
		maxResults = 20
	}

	// Get landscape for context
	landscape, err := s.landscape.GetLandscapeByScaffold(ctx, req.CoreScaffold)
	if err != nil {
		s.logger.Error("scaffold landscape fetch failed", "error", err)
		return nil, fmt.Errorf("fetch scaffold landscape: %w", err)
	}

	// Find molecular gaps
	opportunities, err := s.molAnalyzer.FindGaps(ctx, req.CoreScaffold, req.Substituents, req.TargetProperty)
	if err != nil {
		s.logger.Error("molecule gap analysis failed", "error", err)
		return nil, fmt.Errorf("molecule gap analysis: %w", err)
	}

	// Cap results
	if len(opportunities) > maxResults {
		opportunities = opportunities[:maxResults]
	}

	result := &WhiteSpaceAnalysisResult{
		ID:               generateWhiteSpaceID(),
		AnalysisType:     WhiteSpaceMoleculeClass,
		Query:            req.CoreScaffold,
		Opportunities:    opportunities,
		TotalPatents:     landscape.TotalPatents,
		CoveragePercent:  landscape.Coverage * 100,
		AnalyzedAt:       time.Now(),
		ProcessingTimeMs: time.Since(startTime).Milliseconds(),
		Metadata: map[string]string{
			"core_scaffold":   req.CoreScaffold,
			"target_property": req.TargetProperty,
		},
	}

	if err := s.reportStore.Save(ctx, result); err != nil {
		s.logger.Warn("failed to persist analysis report", "error", err)
	}

	return result, nil
}

func (s *whiteSpaceServiceImpl) AnalyzeByPropertyRange(ctx context.Context, req *AnalyzeByPropertyRangeRequest) (*WhiteSpaceAnalysisResult, error) {
	if req == nil {
		return nil, apperrors.NewValidationError("request", "request cannot be nil")
	}
	if req.PropertyName == "" {
		return nil, apperrors.NewValidationError("property_name", "property_name is required")
	}
	if req.MinValue >= req.MaxValue {
		return nil, apperrors.NewValidationError("range", "min_value must be less than max_value")
	}
	if req.StepSize <= 0 {
		return nil, apperrors.NewValidationError("step_size", "step_size must be positive")
	}

	startTime := time.Now()
	s.logger.Info("analyzing white space by property range", "property", req.PropertyName)

	maxResults := req.MaxResults
	if maxResults <= 0 {
		maxResults = 20
	}

	landscape, err := s.landscape.GetLandscapeByProperty(ctx, req.PropertyName, req.MinValue, req.MaxValue, req.TechField)
	if err != nil {
		s.logger.Error("property landscape fetch failed", "error", err)
		return nil, fmt.Errorf("fetch property landscape: %w", err)
	}

	opportunities, err := s.molAnalyzer.FindPropertyGaps(ctx, req.PropertyName, req.MinValue, req.MaxValue, req.StepSize, req.TechField)
	if err != nil {
		s.logger.Error("property gap analysis failed", "error", err)
		return nil, fmt.Errorf("property gap analysis: %w", err)
	}

	if len(opportunities) > maxResults {
		opportunities = opportunities[:maxResults]
	}

	result := &WhiteSpaceAnalysisResult{
		ID:               generateWhiteSpaceID(),
		AnalysisType:     WhiteSpacePropertyRange,
		Query:            fmt.Sprintf("%s [%.2f - %.2f]", req.PropertyName, req.MinValue, req.MaxValue),
		Opportunities:    opportunities,
		TotalPatents:     landscape.TotalPatents,
		CoveragePercent:  landscape.Coverage * 100,
		AnalyzedAt:       time.Now(),
		ProcessingTimeMs: time.Since(startTime).Milliseconds(),
		Metadata: map[string]string{
			"property_name": req.PropertyName,
			"min_value":     fmt.Sprintf("%.4f", req.MinValue),
			"max_value":     fmt.Sprintf("%.4f", req.MaxValue),
			"step_size":     fmt.Sprintf("%.4f", req.StepSize),
			"tech_field":    req.TechField,
		},
	}

	if err := s.reportStore.Save(ctx, result); err != nil {
		s.logger.Warn("failed to persist analysis report", "error", err)
	}

	return result, nil
}

func (s *whiteSpaceServiceImpl) GetAnalysisReport(ctx context.Context, reportID string) (*WhiteSpaceAnalysisResult, error) {
	if reportID == "" {
		return nil, apperrors.NewValidationError("report_id", "report ID is required")
	}
	return s.reportStore.Get(ctx, reportID)
}

func (s *whiteSpaceServiceImpl) ListRecentAnalyses(ctx context.Context, limit int) ([]WhiteSpaceAnalysisResult, error) {
	if limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}
	return s.reportStore.ListRecent(ctx, limit)
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

func identifyGapsFromLandscape(landscape *LandscapeData, maxResults int) []WhiteSpaceOpportunity {
	if landscape == nil || len(landscape.Clusters) == 0 {
		return nil
	}

	var opportunities []WhiteSpaceOpportunity

	for _, cluster := range landscape.Clusters {
		if cluster.Density < 0.30 {
			level := OpportunityHigh
			if cluster.Density >= 0.10 {
				level = OpportunityMedium
			}
			if cluster.Density >= 0.20 {
				level = OpportunityLow
			}

			opp := WhiteSpaceOpportunity{
				ID:              fmt.Sprintf("opp-%s", cluster.ID),
				Description:     fmt.Sprintf("Low patent density in area: %s", cluster.Label),
				Level:           level,
				TechArea:        cluster.Label,
				GapDescription:  fmt.Sprintf("Patent density %.1f%% indicates underexplored area", cluster.Density*100),
				PatentDensity:   cluster.Density,
				InnovationScore: 1.0 - cluster.Density,
				NearestPatents:  cluster.PatentIDs,
				Rationale:       fmt.Sprintf("Cluster '%s' has only %d patents with density %.2f", cluster.Label, cluster.PatentCount, cluster.Density),
			}
			opportunities = append(opportunities, opp)
		}
	}

	// Sort by innovation score descending
	sortOpportunitiesByScore(opportunities)

	if len(opportunities) > maxResults {
		opportunities = opportunities[:maxResults]
	}

	return opportunities
}

func sortOpportunitiesByScore(opps []WhiteSpaceOpportunity) {
	for i := 0; i < len(opps); i++ {
		for j := i + 1; j < len(opps); j++ {
			if opps[j].InnovationScore > opps[i].InnovationScore {
				opps[i], opps[j] = opps[j], opps[i]
			}
		}
	}
}

var whiteSpaceIDCounter int64

func generateWhiteSpaceID() string {
	whiteSpaceIDCounter++
	return fmt.Sprintf("ws-%d-%d", time.Now().UnixMilli(), whiteSpaceIDCounter)
}

//Personal.AI order the ending


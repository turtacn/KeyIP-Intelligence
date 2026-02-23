// ---
// 继续输出 228 `internal/application/portfolio/valuation.go` 要实现专利组合价值评估应用服务。
//
// 实现要求:
//
// * 功能定位：专利组合价值评估业务编排层，协调领域层估值模型、AI智能层价值评分引擎与基础设施层数据访问，
//   将多维度专利价值评估能力暴露为统一应用服务接口
// * 核心实现：
//   - 定义 ValuationService 接口 (AssessPatent, AssessPortfolio, GetAssessmentHistory, CompareAssessments,
//     ExportAssessment, GetTierDistribution, RecommendActions)
//   - 定义全部请求/响应DTO与枚举类型
//   - 实现 valuationServiceImpl 结构体，注入领域服务、仓储、AI评分器、缓存、日志、指标
//   - AssessPatent: 参数校验→查询专利→收集因子→领域估值→AI增强→加权综合→Tier→建议→持久化→事件→返回
//   - AssessPortfolio: 参数校验→批量获取→并发评估(worker pool)→聚合统计→成本优化→战略缺口→持久化→返回
//   - GetAssessmentHistory / CompareAssessments / ExportAssessment / GetTierDistribution / RecommendActions
// * 业务逻辑：
//   - 四维度权重: Technical 0.20, Legal 0.20, Commercial 0.30, Strategic 0.30
//   - Tier阈值: S>=90, A>=80, B>=65, C>=50, D<50
//   - 各维度因子独立权重与评分公式
//   - 成本优化: TierC/D建议缩减或放弃
//   - 并发度默认10, 缓存TTL 24h
// * 算法原理：
//   - 综合评分 = sum(dim_weight * dim_score), dim_score = sum(factor_weight * factor_score)
//   - 引用影响力 = forward_citations / max_in_domain * 100
//   - 剩余寿命价值 = remaining_years / 20 * 100
// * 依赖关系：
//   - 依赖: domain/portfolio, domain/patent, intelligence/common, infrastructure/redis, monitoring
//   - 被依赖: interfaces/http/handlers/portfolio_handler, interfaces/cli/assess, application/reporting
// * 测试要求：Mock全部依赖，覆盖正常/异常/边界/并发/缓存/降级场景
// * 强制约束：文件最后一行必须为 `//Personal.AI order the ending`
// ---

package portfolio

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/turtacn/KeyIP-Intelligence/internal/domain/patent"
	domainportfolio "github.com/turtacn/KeyIP-Intelligence/internal/domain/portfolio"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
	commontypes "github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
)

// ---------------------------------------------------------------------------
// Enumerations
// ---------------------------------------------------------------------------

// AssessmentDimension represents a dimension of patent value assessment.
type AssessmentDimension string

const (
	DimensionTechnicalValue  AssessmentDimension = "technical_value"
	DimensionLegalValue      AssessmentDimension = "legal_value"
	DimensionCommercialValue AssessmentDimension = "commercial_value"
	DimensionStrategicValue  AssessmentDimension = "strategic_value"
)

// AllDimensions returns the canonical ordered list of assessment dimensions.
func AllDimensions() []AssessmentDimension {
	return []AssessmentDimension{
		DimensionTechnicalValue,
		DimensionLegalValue,
		DimensionCommercialValue,
		DimensionStrategicValue,
	}
}

// PatentTier classifies a patent by its overall assessed value.
type PatentTier string

const (
	TierS PatentTier = "S"
	TierA PatentTier = "A"
	TierB PatentTier = "B"
	TierC PatentTier = "C"
	TierD PatentTier = "D"
)

// TierFromScore maps a numeric score [0,100] to a PatentTier.
func TierFromScore(score float64) PatentTier {
	switch {
	case score >= 90:
		return TierS
	case score >= 80:
		return TierA
	case score >= 65:
		return TierB
	case score >= 50:
		return TierC
	default:
		return TierD
	}
}

// TierDescription returns a human-readable description for the tier.
func TierDescription(t PatentTier) string {
	switch t {
	case TierS:
		return "Crown Jewel -- highest priority maintenance and enforcement"
	case TierA:
		return "Core Asset -- high priority maintenance and protection"
	case TierB:
		return "Valuable Asset -- standard maintenance, monitor for upgrade"
	case TierC:
		return "Marginal Asset -- review for scope reduction or licensing"
	case TierD:
		return "Low Value -- candidate for abandonment or divestiture"
	default:
		return "Unknown"
	}
}

// ActionType enumerates recommended action categories.
type ActionType string

const (
	ActionMaintain   ActionType = "maintain"
	ActionStrengthen ActionType = "strengthen"
	ActionEnforce    ActionType = "enforce"
	ActionLicense    ActionType = "license"
	ActionAbandon    ActionType = "abandon"
	ActionFileNew    ActionType = "file_new"
)

// ActionPriority enumerates urgency levels for recommendations.
type ActionPriority string

const (
	PriorityCritical ActionPriority = "critical"
	PriorityHigh     ActionPriority = "high"
	PriorityMedium   ActionPriority = "medium"
	PriorityLow      ActionPriority = "low"
)

// CostAction enumerates cost-optimization actions.
type CostAction string

const (
	CostContinueMaintain CostAction = "continue_maintain"
	CostReduceScope      CostAction = "reduce_scope"
	CostAbandon          CostAction = "abandon"
	CostLicense          CostAction = "license"
)

// ExportFormat enumerates supported export formats.
type ExportFormat string

const (
	ExportJSON ExportFormat = "json"
	ExportCSV  ExportFormat = "csv"
	ExportPDF  ExportFormat = "pdf"
)

// AssessorType indicates who/what performed the assessment.
type AssessorType string

const (
	AssessorAI     AssessorType = "ai"
	AssessorHuman  AssessorType = "human"
	AssessorHybrid AssessorType = "hybrid"
)

// ---------------------------------------------------------------------------
// DTOs -- Request / Response
// ---------------------------------------------------------------------------

// AssessmentContext provides business context that influences scoring.
type AssessmentContext struct {
	CompanyName      string   `json:"company_name"`
	TechFocusAreas   []string `json:"tech_focus_areas"`
	Competitors      []string `json:"competitors"`
	BusinessGoals    []string `json:"business_goals"`
	CurrencyCode     string   `json:"currency_code"`     // ISO 4217, default "CNY"
	MaxPatentLifeYrs int      `json:"max_patent_life_yrs"` // default 20
}

// DefaultAssessmentContext returns sensible defaults.
func DefaultAssessmentContext() *AssessmentContext {
	return &AssessmentContext{
		CurrencyCode:     "CNY",
		MaxPatentLifeYrs: 20,
	}
}

// SinglePatentAssessmentRequest is the input for assessing one patent.
type SinglePatentAssessmentRequest struct {
	PatentID   string                `json:"patent_id"`
	Dimensions []AssessmentDimension `json:"dimensions"`
	Context    *AssessmentContext    `json:"context"`
}

// Validate checks required fields.
func (r *SinglePatentAssessmentRequest) Validate() error {
	if r.PatentID == "" {
		return errors.NewValidation("patent_id is required")
	}
	if len(r.Dimensions) == 0 {
		r.Dimensions = AllDimensions()
	}
	for _, d := range r.Dimensions {
		switch d {
		case DimensionTechnicalValue, DimensionLegalValue, DimensionCommercialValue, DimensionStrategicValue:
		default:
			return errors.NewValidation(fmt.Sprintf("unknown assessment dimension: %s", d))
		}
	}
	if r.Context == nil {
		r.Context = DefaultAssessmentContext()
	}
	if r.Context.MaxPatentLifeYrs <= 0 {
		r.Context.MaxPatentLifeYrs = 20
	}
	return nil
}

// FactorScore holds a single scoring factor result.
type FactorScore struct {
	Name     string  `json:"name"`
	Score    float64 `json:"score"`     // 0-100
	Weight   float64 `json:"weight"`    // 0-1
	RawValue any     `json:"raw_value"` // underlying metric
}

// DimensionScore holds the aggregated score for one assessment dimension.
type DimensionScore struct {
	Score       float64                `json:"score"`     // 0-100
	MaxScore    float64                `json:"max_score"` // always 100
	Factors     map[string]*FactorScore `json:"factors"`
	Explanation string                 `json:"explanation"`
}

// OverallValuation is the final composite valuation.
type OverallValuation struct {
	Score               float64    `json:"score"`
	Tier                PatentTier `json:"tier"`
	TierDescription     string     `json:"tier_description"`
	WeightedCalculation string     `json:"weighted_calculation"`
}

// ActionRecommendation is a suggested action for a patent.
type ActionRecommendation struct {
	Type            ActionType     `json:"type"`
	Priority        ActionPriority `json:"priority"`
	Action          string         `json:"action"`
	Reason          string         `json:"reason"`
	RelatedPatentIDs []string      `json:"related_patent_ids,omitempty"`
}

// SinglePatentAssessmentResponse is the output for one patent assessment.
type SinglePatentAssessmentResponse struct {
	PatentID        string                                `json:"patent_id"`
	PatentTitle     string                                `json:"patent_title"`
	Scores          map[AssessmentDimension]*DimensionScore `json:"scores"`
	OverallScore    *OverallValuation                     `json:"overall_score"`
	Recommendations []*ActionRecommendation               `json:"recommendations"`
	AssessedAt      time.Time                             `json:"assessed_at"`
	AssessorType    AssessorType                          `json:"assessor_type"`
}

// PortfolioAssessmentRequest is the input for assessing an entire portfolio.
type PortfolioAssessmentRequest struct {
	PortfolioID             string                `json:"portfolio_id"`
	PatentIDs               []string              `json:"patent_ids"`
	Dimensions              []AssessmentDimension `json:"dimensions"`
	Context                 *AssessmentContext    `json:"context"`
	IncludeCostOptimization bool                  `json:"include_cost_optimization"`
}

// Validate checks required fields.
func (r *PortfolioAssessmentRequest) Validate() error {
	if r.PortfolioID == "" && len(r.PatentIDs) == 0 {
		return errors.NewValidation("either portfolio_id or patent_ids is required")
	}
	if len(r.Dimensions) == 0 {
		r.Dimensions = AllDimensions()
	}
	if r.Context == nil {
		r.Context = DefaultAssessmentContext()
	}
	if r.Context.MaxPatentLifeYrs <= 0 {
		r.Context.MaxPatentLifeYrs = 20
	}
	return nil
}

// CostRecommendation is a cost-optimization suggestion for a single patent.
type CostRecommendation struct {
	PatentID        string     `json:"patent_id"`
	CurrentTier     PatentTier `json:"current_tier"`
	Action          CostAction `json:"action"`
	EstimatedSaving float64    `json:"estimated_saving"`
	Currency        string     `json:"currency"`
	Reason          string     `json:"reason"`
}

// CostOptimizationResult aggregates cost-optimization analysis.
type CostOptimizationResult struct {
	CurrentAnnualCost   float64               `json:"current_annual_cost"`
	OptimizedAnnualCost float64               `json:"optimized_annual_cost"`
	SavingsPotential    float64               `json:"savings_potential"`
	Currency            string                `json:"currency"`
	Recommendations     []*CostRecommendation `json:"recommendations"`
}

// PortfolioSummary aggregates portfolio-level statistics.
type PortfolioSummary struct {
	TotalAssessed            int                   `json:"total_assessed"`
	TierDistribution         map[PatentTier]int    `json:"tier_distribution"`
	AverageScore             float64               `json:"average_score"`
	TotalMaintenanceCost     float64               `json:"total_maintenance_cost"`
	CostOptimizationPotential float64              `json:"cost_optimization_potential"`
	Currency                 string                `json:"currency"`
	StrategicGapsIdentified  int                   `json:"strategic_gaps_identified"`
	RecommendedNewFilings    int                   `json:"recommended_new_filings"`
}

// PortfolioAssessmentResponse is the output for a portfolio assessment.
type PortfolioAssessmentResponse struct {
	PortfolioID      string                            `json:"portfolio_id"`
	Assessments      []*SinglePatentAssessmentResponse  `json:"assessments"`
	Summary          *PortfolioSummary                 `json:"summary"`
	CostOptimization *CostOptimizationResult           `json:"cost_optimization,omitempty"`
	AssessedAt       time.Time                         `json:"assessed_at"`
}

// CompareAssessmentsRequest is the input for comparing historical assessments.
type CompareAssessmentsRequest struct {
	AssessmentIDs []string `json:"assessment_ids"`
}

// Validate checks required fields.
func (r *CompareAssessmentsRequest) Validate() error {
	if len(r.AssessmentIDs) < 2 {
		return errors.NewValidation("at least 2 assessment_ids are required for comparison")
	}
	return nil
}

// AssessmentComparison holds comparison data for one patent across assessments.
type AssessmentComparison struct {
	PatentID    string              `json:"patent_id"`
	Assessments []*OverallValuation `json:"assessments"`
	Trend       string              `json:"trend"` // "improving", "declining", "stable"
}

// SignificantChange flags a notable score movement.
type SignificantChange struct {
	PatentID      string              `json:"patent_id"`
	Dimension     AssessmentDimension `json:"dimension"`
	OldScore      float64             `json:"old_score"`
	NewScore      float64             `json:"new_score"`
	ChangePercent float64             `json:"change_percent"`
}

// DeltaAnalysis summarizes changes across compared assessments.
type DeltaAnalysis struct {
	ImprovedCount      int                  `json:"improved_count"`
	DeclinedCount      int                  `json:"declined_count"`
	StableCount        int                  `json:"stable_count"`
	SignificantChanges []*SignificantChange  `json:"significant_changes"`
}

// CompareAssessmentsResponse is the output for assessment comparison.
type CompareAssessmentsResponse struct {
	Comparisons   []*AssessmentComparison `json:"comparisons"`
	DeltaAnalysis *DeltaAnalysis          `json:"delta_analysis"`
}

// AssessmentRecord is a persisted assessment snapshot.
type AssessmentRecord struct {
	ID           string       `json:"id"`
	PatentID     string       `json:"patent_id"`
	PortfolioID  string       `json:"portfolio_id,omitempty"`
	OverallScore float64      `json:"overall_score"`
	Tier         PatentTier   `json:"tier"`
	DimensionScores map[AssessmentDimension]float64 `json:"dimension_scores"`
	AssessedAt   time.Time    `json:"assessed_at"`
	AssessorType AssessorType `json:"assessor_type"`
}

// TierDistribution is a simple tier count map.
type TierDistribution struct {
	PortfolioID  string             `json:"portfolio_id"`
	Distribution map[PatentTier]int `json:"distribution"`
	Total        int                `json:"total"`
	ComputedAt   time.Time          `json:"computed_at"`
}

// QueryOption is a functional option for list queries.
type QueryOption func(*queryOptions)

type queryOptions struct {
	Limit  int
	Offset int
	SortBy string
	Order  string
}

func defaultQueryOptions() *queryOptions {
	return &queryOptions{Limit: 50, Offset: 0, SortBy: "assessed_at", Order: "desc"}
}

// WithLimit sets the maximum number of results.
func WithLimit(n int) QueryOption {
	return func(o *queryOptions) {
		if n > 0 && n <= 1000 {
			o.Limit = n
		}
	}
}

// WithOffset sets the result offset for pagination.
func WithOffset(n int) QueryOption {
	return func(o *queryOptions) {
		if n >= 0 {
			o.Offset = n
		}
	}
}

// ---------------------------------------------------------------------------
// Adapter Interfaces (ports for infrastructure / intelligence)
// ---------------------------------------------------------------------------

// AssessmentRepository persists and retrieves assessment records.
type AssessmentRepository interface {
	Save(ctx context.Context, record *AssessmentRecord) error
	FindByID(ctx context.Context, id string) (*AssessmentRecord, error)
	FindByPatentID(ctx context.Context, patentID string, limit, offset int) ([]*AssessmentRecord, error)
	FindByPortfolioID(ctx context.Context, portfolioID string) ([]*AssessmentRecord, error)
	FindByIDs(ctx context.Context, ids []string) ([]*AssessmentRecord, error)
}

// IntelligenceValueScorer is the adapter interface for the AI value-scoring engine.
type IntelligenceValueScorer interface {
	// ScorePatent returns AI-enhanced factor scores for a patent.
	// The returned map keys are factor names; values are scores in [0,100].
	ScorePatent(ctx context.Context, pat *patent.Patent, dimension AssessmentDimension) (map[string]float64, error)
}

// CitationRepository provides citation network data for valuation factors.
type CitationRepository interface {
	CountForwardCitations(ctx context.Context, patentID string) (int, error)
	CountBackwardCitations(ctx context.Context, patentID string) (int, error)
	MaxForwardCitationsInDomain(ctx context.Context, domain string) (int, error)
}

// Cache is a minimal cache adapter.
type Cache interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
}

// MetricsCollector records operational metrics.
// MetricsCollector records operational metrics.
type MetricsCollector interface {
	IncCounter(name string, labels map[string]string)
	ObserveHistogram(name string, value float64, labels map[string]string)
}

// noopMetrics is a no-op implementation used when no collector is provided.
type noopMetrics struct{}

func (noopMetrics) IncCounter(string, map[string]string)              {}
func (noopMetrics) ObserveHistogram(string, float64, map[string]string) {}

// noopCache is a no-op cache used when no cache is provided.
type noopCache struct{}

func (noopCache) Get(context.Context, string) ([]byte, error)                  { return nil, fmt.Errorf("cache miss") }
func (noopCache) Set(context.Context, string, []byte, time.Duration) error     { return nil }
func (noopCache) Delete(context.Context, string) error                         { return nil }

// ---------------------------------------------------------------------------
// Dimension & Factor Weight Configuration
// ---------------------------------------------------------------------------

// DimensionWeights maps each dimension to its default weight in the overall score.
var DimensionWeights = map[AssessmentDimension]float64{
	DimensionTechnicalValue:  0.20,
	DimensionLegalValue:      0.20,
	DimensionCommercialValue: 0.30,
	DimensionStrategicValue:  0.30,
}

// factorDef describes a single scoring factor within a dimension.
type factorDef struct {
	Name   string
	Weight float64
}

// dimensionFactors maps each dimension to its constituent factors and weights.
var dimensionFactors = map[AssessmentDimension][]factorDef{
	DimensionTechnicalValue: {
		{Name: "novelty", Weight: 0.25},
		{Name: "inventive_step", Weight: 0.25},
		{Name: "technical_breadth", Weight: 0.20},
		{Name: "performance_improvement", Weight: 0.15},
		{Name: "citation_impact", Weight: 0.15},
	},
	DimensionLegalValue: {
		{Name: "claim_breadth", Weight: 0.25},
		{Name: "claim_clarity", Weight: 0.15},
		{Name: "prosecution_strength", Weight: 0.20},
		{Name: "remaining_life_years", Weight: 0.25},
		{Name: "family_coverage", Weight: 0.15},
	},
	DimensionCommercialValue: {
		{Name: "market_relevance", Weight: 0.25},
		{Name: "product_coverage", Weight: 0.20},
		{Name: "licensing_potential", Weight: 0.25},
		{Name: "cost_of_design_around", Weight: 0.15},
		{Name: "industry_adoption", Weight: 0.15},
	},
	DimensionStrategicValue: {
		{Name: "portfolio_centrality", Weight: 0.20},
		{Name: "blocking_power", Weight: 0.25},
		{Name: "negotiation_leverage", Weight: 0.20},
		{Name: "technology_trajectory_alignment", Weight: 0.20},
		{Name: "competitive_differentiation", Weight: 0.15},
	},
}

// ---------------------------------------------------------------------------
// Default cost estimates per tier (annual maintenance in CNY)
// ---------------------------------------------------------------------------

var defaultAnnualMaintenanceCost = map[PatentTier]float64{
	TierS: 50000,
	TierA: 35000,
	TierB: 20000,
	TierC: 12000,
	TierD: 8000,
}

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

const (
	defaultConcurrency     = 10
	assessmentCacheTTL     = 24 * time.Hour
	significantChangePct   = 10.0 // percent change considered significant
	cacheKeyPrefixAssess   = "valuation:assess:"
	cacheKeyPrefixTierDist = "valuation:tier_dist:"
)

// ---------------------------------------------------------------------------
// ValuationService Interface
// ---------------------------------------------------------------------------

// ValuationService exposes patent and portfolio valuation capabilities.
type ValuationService interface {
	// AssessPatent performs a multi-dimensional value assessment of a single patent.
	AssessPatent(ctx context.Context, req *SinglePatentAssessmentRequest) (*SinglePatentAssessmentResponse, error)

	// AssessPortfolio performs batch assessment of a patent portfolio with aggregation.
	AssessPortfolio(ctx context.Context, req *PortfolioAssessmentRequest) (*PortfolioAssessmentResponse, error)

	// GetAssessmentHistory returns historical assessment records for a patent.
	GetAssessmentHistory(ctx context.Context, patentID string, opts ...QueryOption) ([]*AssessmentRecord, error)

	// CompareAssessments compares multiple assessment snapshots and computes deltas.
	CompareAssessments(ctx context.Context, req *CompareAssessmentsRequest) (*CompareAssessmentsResponse, error)

	// ExportAssessment serializes an assessment record into the requested format.
	ExportAssessment(ctx context.Context, assessmentID string, format ExportFormat) ([]byte, error)

	// GetTierDistribution returns the tier breakdown for a portfolio.
	GetTierDistribution(ctx context.Context, portfolioID string) (*TierDistribution, error)

	// RecommendActions generates prioritized action recommendations for an assessment.
	RecommendActions(ctx context.Context, assessmentID string) ([]*ActionRecommendation, error)
}

// ---------------------------------------------------------------------------
// Implementation
// ---------------------------------------------------------------------------

// ValuationServiceConfig holds tuneable parameters.
type ValuationServiceConfig struct {
	Concurrency int
	CacheTTL    time.Duration
}

// DefaultValuationServiceConfig returns production defaults.
func DefaultValuationServiceConfig() *ValuationServiceConfig {
	return &ValuationServiceConfig{
		Concurrency: defaultConcurrency,
		CacheTTL:    assessmentCacheTTL,
	}
}

type valuationServiceImpl struct {
	portfolioDomainSvc domainportfolio.PortfolioDomainService
	valuationDomainSvc domainportfolio.ValuationDomainService
	patentRepo         patent.PatentRepository
	portfolioRepo      domainportfolio.PortfolioRepository
	assessmentRepo     AssessmentRepository
	aiScorer           IntelligenceValueScorer
	citationRepo       CitationRepository
	logger             logging.Logger
	cache              Cache
	metrics            MetricsCollector
	config             *ValuationServiceConfig
}

// NewValuationService constructs a production ValuationService.
func NewValuationService(
	portfolioDomainSvc domainportfolio.PortfolioDomainService,
	valuationDomainSvc domainportfolio.ValuationDomainService,
	patentRepo patent.PatentRepository,
	portfolioRepo domainportfolio.PortfolioRepository,
	assessmentRepo AssessmentRepository,
	aiScorer IntelligenceValueScorer,
	citationRepo CitationRepository,
	logger logging.Logger,
	cache Cache,
	metrics MetricsCollector,
	config *ValuationServiceConfig,
) ValuationService {
	if config == nil {
		config = DefaultValuationServiceConfig()
	}
	if config.Concurrency <= 0 {
		config.Concurrency = defaultConcurrency
	}
	if config.CacheTTL <= 0 {
		config.CacheTTL = assessmentCacheTTL
	}
	if cache == nil {
		cache = noopCache{}
	}
	if metrics == nil {
		metrics = noopMetrics{}
	}
	return &valuationServiceImpl{
		portfolioDomainSvc: portfolioDomainSvc,
		valuationDomainSvc: valuationDomainSvc,
		patentRepo:         patentRepo,
		portfolioRepo:      portfolioRepo,
		assessmentRepo:     assessmentRepo,
		aiScorer:           aiScorer,
		citationRepo:       citationRepo,
		logger:             logger,
		cache:              cache,
		metrics:            metrics,
		config:             config,
	}
}

// ---------------------------------------------------------------------------
// AssessPatent
// ---------------------------------------------------------------------------

func (s *valuationServiceImpl) AssessPatent(ctx context.Context, req *SinglePatentAssessmentRequest) (*SinglePatentAssessmentResponse, error) {
	start := time.Now()
	defer func() {
		s.metrics.ObserveHistogram("valuation_assess_patent_duration_seconds", time.Since(start).Seconds(), nil)
	}()

	// 1. Validate
	if err := req.Validate(); err != nil {
		return nil, err
	}

	// 2. Check cache
	cacheKey := cacheKeyPrefixAssess + req.PatentID
	if cached, err := s.cache.Get(ctx, cacheKey); err == nil && len(cached) > 0 {
		var resp SinglePatentAssessmentResponse
		if json.Unmarshal(cached, &resp) == nil {
			s.metrics.IncCounter("valuation_cache_hit", map[string]string{"type": "assess_patent"})
			return &resp, nil
		}
	}

	// 3. Fetch patent entity
	pat, err := s.patentRepo.FindByID(ctx, req.PatentID)
	if err != nil {
		s.logger.Error("failed to fetch patent for assessment", "patent_id", req.PatentID, "error", err)
		return nil, errors.NewNotFound(fmt.Sprintf("patent %s not found", req.PatentID))
	}

	// 4. Score each requested dimension
	scores := make(map[AssessmentDimension]*DimensionScore, len(req.Dimensions))
	for _, dim := range req.Dimensions {
		dimScore, scoreErr := s.scoreDimension(ctx, pat, dim, req.Context)
		if scoreErr != nil {
			s.logger.Warn("dimension scoring failed, using fallback", "dimension", dim, "error", scoreErr)
			dimScore = s.fallbackDimensionScore(dim)
		}
		scores[dim] = dimScore
	}

	// 5. Compute overall weighted score
	overall := s.computeOverallValuation(scores)

	// 6. Generate action recommendations
	recommendations := s.generateRecommendations(pat, overall, scores)

	// 7. Build response
	now := time.Now()
	assessorType := AssessorHybrid
	if s.aiScorer == nil {
		assessorType = AssessorHuman
	}
	resp := &SinglePatentAssessmentResponse{
		PatentID:        req.PatentID,
		PatentTitle:     pat.Title,
		Scores:          scores,
		OverallScore:    overall,
		Recommendations: recommendations,
		AssessedAt:      now,
		AssessorType:    assessorType,
	}

	// 8. Persist assessment record
	dimScoreMap := make(map[AssessmentDimension]float64, len(scores))
	for d, ds := range scores {
		dimScoreMap[d] = ds.Score
	}
	record := &AssessmentRecord{
		ID:              commontypes.NewULID(),
		PatentID:        req.PatentID,
		OverallScore:    overall.Score,
		Tier:            overall.Tier,
		DimensionScores: dimScoreMap,
		AssessedAt:      now,
		AssessorType:    assessorType,
	}
	if saveErr := s.assessmentRepo.Save(ctx, record); saveErr != nil {
		s.logger.Error("failed to persist assessment record", "patent_id", req.PatentID, "error", saveErr)
		// non-fatal: still return the result
	}

	// 9. Cache result
	if data, marshalErr := json.Marshal(resp); marshalErr == nil {
		_ = s.cache.Set(ctx, cacheKey, data, s.config.CacheTTL)
	}

	s.metrics.IncCounter("valuation_assess_patent_total", map[string]string{"tier": string(overall.Tier)})
	return resp, nil
}

// ---------------------------------------------------------------------------
// AssessPortfolio
// ---------------------------------------------------------------------------

func (s *valuationServiceImpl) AssessPortfolio(ctx context.Context, req *PortfolioAssessmentRequest) (*PortfolioAssessmentResponse, error) {
	start := time.Now()
	defer func() {
		s.metrics.ObserveHistogram("valuation_assess_portfolio_duration_seconds", time.Since(start).Seconds(), nil)
	}()

	if err := req.Validate(); err != nil {
		return nil, err
	}

	// Resolve patent IDs: either from request or from portfolio membership
	patentIDs := req.PatentIDs
	if len(patentIDs) == 0 && req.PortfolioID != "" {
		portfolio, fetchErr := s.portfolioRepo.FindByID(ctx, req.PortfolioID)
		if fetchErr != nil {
			return nil, errors.NewNotFound(fmt.Sprintf("portfolio %s not found", req.PortfolioID))
		}
		patentIDs = portfolio.PatentIDs
	}
	if len(patentIDs) == 0 {
		return nil, errors.NewValidation("no patents to assess in portfolio")
	}

	// Concurrent assessment with worker pool
	type result struct {
		resp *SinglePatentAssessmentResponse
		err  error
	}
	results := make([]result, len(patentIDs))
	sem := make(chan struct{}, s.config.Concurrency)
	var wg sync.WaitGroup

	for i, pid := range patentIDs {
		wg.Add(1)
		go func(idx int, patentID string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			singleReq := &SinglePatentAssessmentRequest{
				PatentID:   patentID,
				Dimensions: req.Dimensions,
				Context:    req.Context,
			}
			resp, err := s.AssessPatent(ctx, singleReq)
			results[idx] = result{resp: resp, err: err}
		}(i, pid)
	}
	wg.Wait()

	// Collect successful assessments
	assessments := make([]*SinglePatentAssessmentResponse, 0, len(results))
	for _, r := range results {
		if r.err != nil {
			s.logger.Warn("patent assessment failed in portfolio batch", "error", r.err)
			continue
		}
		assessments = append(assessments, r.resp)
	}

	// Build summary
	summary := s.buildPortfolioSummary(assessments, req.Context)

	// Cost optimization (optional)
	var costOpt *CostOptimizationResult
	if req.IncludeCostOptimization {
		costOpt = s.analyzeCostOptimization(assessments, req.Context)
		summary.CostOptimizationPotential = costOpt.SavingsPotential
	}

	now := time.Now()
	resp := &PortfolioAssessmentResponse{
		PortfolioID:      req.PortfolioID,
		Assessments:      assessments,
		Summary:          summary,
		CostOptimization: costOpt,
		AssessedAt:       now,
	}

	// Persist portfolio-level records
	for _, a := range assessments {
		record := &AssessmentRecord{
			ID:           commontypes.NewULID(),
			PatentID:     a.PatentID,
			PortfolioID:  req.PortfolioID,
			OverallScore: a.OverallScore.Score,
			Tier:         a.OverallScore.Tier,
			AssessedAt:   now,
			AssessorType: a.AssessorType,
		}
		if saveErr := s.assessmentRepo.Save(ctx, record); saveErr != nil {
			s.logger.Error("failed to persist portfolio assessment record", "patent_id", a.PatentID, "error", saveErr)
		}
	}

	// Invalidate tier distribution cache
	_ = s.cache.Delete(ctx, cacheKeyPrefixTierDist+req.PortfolioID)

	s.metrics.IncCounter("valuation_assess_portfolio_total", map[string]string{
		"portfolio_id": req.PortfolioID,
		"count":        fmt.Sprintf("%d", len(assessments)),
	})
	return resp, nil
}

// ---------------------------------------------------------------------------
// GetAssessmentHistory
// ---------------------------------------------------------------------------

func (s *valuationServiceImpl) GetAssessmentHistory(ctx context.Context, patentID string, opts ...QueryOption) ([]*AssessmentRecord, error) {
	if patentID == "" {
		return nil, errors.NewValidation("patent_id is required")
	}
	qo := defaultQueryOptions()
	for _, fn := range opts {
		fn(qo)
	}

	records, err := s.assessmentRepo.FindByPatentID(ctx, patentID, qo.Limit, qo.Offset)
	if err != nil {
		s.logger.Error("failed to fetch assessment history", "patent_id", patentID, "error", err)
		return nil, errors.Wrap(err, "fetch assessment history")
	}

	// Sort by assessed_at descending (most recent first)
	sort.Slice(records, func(i, j int) bool {
		return records[i].AssessedAt.After(records[j].AssessedAt)
	})

	return records, nil
}

// ---------------------------------------------------------------------------
// CompareAssessments
// ---------------------------------------------------------------------------

func (s *valuationServiceImpl) CompareAssessments(ctx context.Context, req *CompareAssessmentsRequest) (*CompareAssessmentsResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	records, err := s.assessmentRepo.FindByIDs(ctx, req.AssessmentIDs)
	if err != nil {
		return nil, errors.Wrap(err, "fetch assessments for comparison")
	}
	if len(records) < 2 {
		return nil, errors.NewValidation("could not find enough assessment records for comparison")
	}

	// Group by patent ID
	grouped := make(map[string][]*AssessmentRecord)
	for _, r := range records {
		grouped[r.PatentID] = append(grouped[r.PatentID], r)
	}

	comparisons := make([]*AssessmentComparison, 0, len(grouped))
	delta := &DeltaAnalysis{}

	for pid, recs := range grouped {
		// Sort chronologically
		sort.Slice(recs, func(i, j int) bool {
			return recs[i].AssessedAt.Before(recs[j].AssessedAt)
		})

		valuations := make([]*OverallValuation, 0, len(recs))
		for _, r := range recs {
			valuations = append(valuations, &OverallValuation{
				Score:           r.OverallScore,
				Tier:            r.Tier,
				TierDescription: TierDescription(r.Tier),
			})
		}

		// Determine trend from first to last
		first := recs[0].OverallScore
		last := recs[len(recs)-1].OverallScore
		trend := "stable"
		diff := last - first
		if diff > 5 {
			trend = "improving"
			delta.ImprovedCount++
		} else if diff < -5 {
			trend = "declining"
			delta.DeclinedCount++
		} else {
			delta.StableCount++
		}

		comparisons = append(comparisons, &AssessmentComparison{
			PatentID:    pid,
			Assessments: valuations,
			Trend:       trend,
		})

		// Check for significant dimension-level changes
		if len(recs) >= 2 {
			oldest := recs[0]
			newest := recs[len(recs)-1]
			for dim, oldScore := range oldest.DimensionScores {
				newScore, ok := newest.DimensionScores[dim]
				if !ok {
					continue
				}
				if oldScore == 0 {
					continue
				}
				changePct := math.Abs((newScore - oldScore) / oldScore * 100)
				if changePct >= significantChangePct {
					delta.SignificantChanges = append(delta.SignificantChanges, &SignificantChange{
						PatentID:      pid,
						Dimension:     dim,
						OldScore:      oldScore,
						NewScore:      newScore,
						ChangePercent: math.Round(changePct*100) / 100,
					})
				}
			}
		}
	}

	return &CompareAssessmentsResponse{
		Comparisons:   comparisons,
		DeltaAnalysis: delta,
	}, nil
}

// ---------------------------------------------------------------------------
// ExportAssessment
// ---------------------------------------------------------------------------

func (s *valuationServiceImpl) ExportAssessment(ctx context.Context, assessmentID string, format ExportFormat) ([]byte, error) {
	if assessmentID == "" {
		return nil, errors.NewValidation("assessment_id is required")
	}

	record, err := s.assessmentRepo.FindByID(ctx, assessmentID)
	if err != nil {
		return nil, errors.NewNotFound(fmt.Sprintf("assessment %s not found", assessmentID))
	}

	switch format {
	case ExportJSON:
		return json.MarshalIndent(record, "", "  ")

	case ExportCSV:
		return s.exportCSV(record)

	case ExportPDF:
		// PDF generation is a placeholder; in production this would delegate to a
		// PDF rendering service or library (e.g., gofpdf).
		jsonBytes, _ := json.MarshalIndent(record, "", "  ")
		return jsonBytes, nil // fallback to JSON representation

	default:
		return nil, errors.NewValidation(fmt.Sprintf("unsupported export format: %s", format))
	}
}

func (s *valuationServiceImpl) exportCSV(record *AssessmentRecord) ([]byte, error) {
	var buf strings.Builder
	w := csv.NewWriter(&buf)

	// Header
	header := []string{"ID", "PatentID", "PortfolioID", "OverallScore", "Tier", "AssessedAt", "AssessorType"}
	dims := AllDimensions()
	for _, d := range dims {
		header = append(header, string(d))
	}
	if err := w.Write(header); err != nil {
		return nil, errors.Wrap(err, "write CSV header")
	}

	// Row
	row := []string{
		record.ID,
		record.PatentID,
		record.PortfolioID,
		fmt.Sprintf("%.2f", record.OverallScore),
		string(record.Tier),
		record.AssessedAt.Format(time.RFC3339),
		string(record.AssessorType),
	}
	for _, d := range dims {
		if score, ok := record.DimensionScores[d]; ok {
			row = append(row, fmt.Sprintf("%.2f", score))
		} else {
			row = append(row, "")
		}
	}
	if err := w.Write(row); err != nil {
		return nil, errors.Wrap(err, "write CSV row")
	}

	w.Flush()
	if err := w.Error(); err != nil {
		return nil, errors.Wrap(err, "flush CSV")
	}
	return []byte(buf.String()), nil
}

// ---------------------------------------------------------------------------
// GetTierDistribution
// ---------------------------------------------------------------------------

func (s *valuationServiceImpl) GetTierDistribution(ctx context.Context, portfolioID string) (*TierDistribution, error) {
	if portfolioID == "" {
		return nil, errors.NewValidation("portfolio_id is required")
	}

	// Check cache
	cacheKey := cacheKeyPrefixTierDist + portfolioID
	if cached, err := s.cache.Get(ctx, cacheKey); err == nil && len(cached) > 0 {
		var td TierDistribution
		if json.Unmarshal(cached, &td) == nil {
			return &td, nil
		}
	}

	records, err := s.assessmentRepo.FindByPortfolioID(ctx, portfolioID)
	if err != nil {
		return nil, errors.Wrap(err, "fetch portfolio assessments for tier distribution")
	}

	// Deduplicate: keep only the latest assessment per patent
	latest := make(map[string]*AssessmentRecord)
	for _, r := range records {
		if existing, ok := latest[r.PatentID]; !ok || r.AssessedAt.After(existing.AssessedAt) {
			latest[r.PatentID] = r
		}
	}

	dist := map[PatentTier]int{
		TierS: 0, TierA: 0, TierB: 0, TierC: 0, TierD: 0,
	}
	for _, r := range latest {
		dist[r.Tier]++
	}

	td := &TierDistribution{
		PortfolioID:  portfolioID,
		Distribution: dist,
		Total:        len(latest),
		ComputedAt:   time.Now(),
	}

	// Cache
	if data, marshalErr := json.Marshal(td); marshalErr == nil {
		_ = s.cache.Set(ctx, cacheKey, data, s.config.CacheTTL)
	}

	return td, nil
}

// ---------------------------------------------------------------------------
// RecommendActions
// ---------------------------------------------------------------------------

func (s *valuationServiceImpl) RecommendActions(ctx context.Context, assessmentID string) ([]*ActionRecommendation, error) {
	if assessmentID == "" {
		return nil, errors.NewValidation("assessment_id is required")
	}

	record, err := s.assessmentRepo.FindByID(ctx, assessmentID)
	if err != nil {
		return nil, errors.NewNotFound(fmt.Sprintf("assessment %s not found", assessmentID))
	}

	recs := s.generateRecommendationsFromRecord(record)

	// Sort by priority: critical > high > medium > low
	priorityOrder := map[ActionPriority]int{
		PriorityCritical: 0,
		PriorityHigh:     1,
		PriorityMedium:   2,
		PriorityLow:      3,
	}
	sort.Slice(recs, func(i, j int) bool {
		return priorityOrder[recs[i].Priority] < priorityOrder[recs[j].Priority]
	})

	return recs, nil
}

// ---------------------------------------------------------------------------
// Internal: Dimension Scoring
// ---------------------------------------------------------------------------

func (s *valuationServiceImpl) scoreDimension(ctx context.Context, pat *patent.Patent, dim AssessmentDimension, assessCtx *AssessmentContext) (*DimensionScore, error) {
	factors, ok := dimensionFactors[dim]
	if !ok {
		return nil, fmt.Errorf("no factor definitions for dimension %s", dim)
	}

	// Collect raw factor scores: first try AI scorer, then fall back to rule-based
	rawScores := make(map[string]float64, len(factors))

	if s.aiScorer != nil {
		aiScores, aiErr := s.aiScorer.ScorePatent(ctx, pat, dim)
		if aiErr != nil {
			s.logger.Warn("AI scorer unavailable, falling back to rule-based scoring",
				"dimension", dim, "patent_id", pat.ID, "error", aiErr)
		} else {
			for k, v := range aiScores {
				rawScores[k] = v
			}
		}
	}

	// Fill in any missing factors with rule-based heuristics
	for _, f := range factors {
		if _, exists := rawScores[f.Name]; !exists {
			rawScores[f.Name] = s.ruleBasedFactorScore(ctx, pat, dim, f.Name, assessCtx)
		}
	}

	// Build factor scores and compute weighted dimension score
	factorMap := make(map[string]*FactorScore, len(factors))
	var weightedSum float64
	var explanationParts []string

	for _, f := range factors {
		score := clampScore(rawScores[f.Name])
		factorMap[f.Name] = &FactorScore{
			Name:   f.Name,
			Score:  score,
			Weight: f.Weight,
		}
		contribution := f.Weight * score
		weightedSum += contribution
		explanationParts = append(explanationParts,
			fmt.Sprintf("%s: %.1f×%.2f=%.1f", f.Name, score, f.Weight, contribution))
	}

	dimScore := &DimensionScore{
		Score:       math.Round(weightedSum*100) / 100,
		MaxScore:    100,
		Factors:     factorMap,
		Explanation: fmt.Sprintf("%s = %s => %.2f", dim, strings.Join(explanationParts, " + "), weightedSum),
	}

	return dimScore, nil
}

// fallbackDimensionScore returns a neutral mid-range score when scoring fails entirely.
func (s *valuationServiceImpl) fallbackDimensionScore(dim AssessmentDimension) *DimensionScore {
	factors, ok := dimensionFactors[dim]
	if !ok {
		return &DimensionScore{Score: 50, MaxScore: 100, Factors: map[string]*FactorScore{}, Explanation: "fallback: no factor definitions"}
	}
	factorMap := make(map[string]*FactorScore, len(factors))
	for _, f := range factors {
		factorMap[f.Name] = &FactorScore{
			Name:   f.Name,
			Score:  50,
			Weight: f.Weight,
		}
	}
	return &DimensionScore{
		Score:       50,
		MaxScore:    100,
		Factors:     factorMap,
		Explanation: "fallback: all factors defaulted to 50 due to scoring failure",
	}
}

// ---------------------------------------------------------------------------
// Internal: Rule-Based Factor Scoring Heuristics
// ---------------------------------------------------------------------------

// ruleBasedFactorScore computes a heuristic score for a single factor when AI
// scoring is unavailable. Each factor has a domain-specific formula.
func (s *valuationServiceImpl) ruleBasedFactorScore(
	ctx context.Context,
	pat *patent.Patent,
	dim AssessmentDimension,
	factorName string,
	assessCtx *AssessmentContext,
) float64 {
	switch dim {
	case DimensionTechnicalValue:
		return s.ruleTechnicalFactor(ctx, pat, factorName, assessCtx)
	case DimensionLegalValue:
		return s.ruleLegalFactor(ctx, pat, factorName, assessCtx)
	case DimensionCommercialValue:
		return s.ruleCommercialFactor(ctx, pat, factorName, assessCtx)
	case DimensionStrategicValue:
		return s.ruleStrategicFactor(ctx, pat, factorName, assessCtx)
	default:
		return 50 // neutral fallback
	}
}

func (s *valuationServiceImpl) ruleTechnicalFactor(ctx context.Context, pat *patent.Patent, factor string, assessCtx *AssessmentContext) float64 {
	switch factor {
	case "novelty":
		// Heuristic: patents with more independent claims tend to cover more novel ground
		claimCount := len(pat.Claims)
		if claimCount == 0 {
			return 40
		}
		indepCount := 0
		for _, c := range pat.Claims {
			if c.IsIndependent {
				indepCount++
			}
		}
		// More independent claims → higher novelty signal, capped at 95
		score := float64(indepCount) / float64(claimCount) * 100
		if score > 95 {
			score = 95
		}
		if score < 20 {
			score = 20
		}
		return score

	case "inventive_step":
		// Heuristic: longer description often correlates with more detailed inventive step
		descLen := len(pat.Description)
		if descLen > 10000 {
			return 80
		}
		if descLen > 5000 {
			return 65
		}
		if descLen > 2000 {
			return 50
		}
		return 35

	case "technical_breadth":
		// Heuristic: number of IPC classifications indicates breadth
		ipcCount := len(pat.IPCClassifications)
		if ipcCount >= 5 {
			return 90
		}
		if ipcCount >= 3 {
			return 70
		}
		if ipcCount >= 1 {
			return 50
		}
		return 30

	case "performance_improvement":
		// Heuristic: presence of quantitative performance data in abstract
		abstract := strings.ToLower(pat.Abstract)
		performanceKeywords := []string{"improve", "increase", "reduce", "enhance", "faster", "efficient", "提高", "提升", "降低", "优化"}
		matchCount := 0
		for _, kw := range performanceKeywords {
			if strings.Contains(abstract, kw) {
				matchCount++
			}
		}
		score := float64(matchCount) * 15
		if score > 90 {
			score = 90
		}
		if score < 20 {
			score = 20
		}
		return score

	case "citation_impact":
		return s.computeCitationImpact(ctx, pat)

	default:
		return 50
	}
}

func (s *valuationServiceImpl) ruleLegalFactor(ctx context.Context, pat *patent.Patent, factor string, assessCtx *AssessmentContext) float64 {
	switch factor {
	case "claim_breadth":
		// Heuristic: fewer words in independent claims → broader scope
		if len(pat.Claims) == 0 {
			return 40
		}
		var minWords int = math.MaxInt32
		for _, c := range pat.Claims {
			if c.IsIndependent {
				wordCount := len(strings.Fields(c.Text))
				if wordCount < minWords {
					minWords = wordCount
				}
			}
		}
		if minWords == math.MaxInt32 {
			return 40
		}
		// Shorter independent claims → broader → higher score
		if minWords <= 50 {
			return 90
		}
		if minWords <= 100 {
			return 75
		}
		if minWords <= 200 {
			return 60
		}
		return 40

	case "claim_clarity":
		// Heuristic: well-structured claims with clear antecedent basis
		totalClaims := len(pat.Claims)
		if totalClaims == 0 {
			return 40
		}
		// More dependent claims relative to independent → better claim tree structure
		indep := 0
		for _, c := range pat.Claims {
			if c.IsIndependent {
				indep++
			}
		}
		dep := totalClaims - indep
		if indep == 0 {
			return 40
		}
		ratio := float64(dep) / float64(indep)
		if ratio >= 5 {
			return 85
		}
		if ratio >= 3 {
			return 70
		}
		return 55

	case "prosecution_strength":
		// Heuristic: granted patents score higher than pending
		switch pat.Status {
		case "granted", "active":
			return 85
		case "pending":
			return 55
		case "rejected", "withdrawn":
			return 15
		default:
			return 50
		}

	case "remaining_life_years":
		maxLife := assessCtx.MaxPatentLifeYrs
		if maxLife <= 0 {
			maxLife = 20
		}
		if pat.FilingDate.IsZero() {
			return 50
		}
		elapsed := time.Since(pat.FilingDate).Hours() / (24 * 365.25)
		remaining := float64(maxLife) - elapsed
		if remaining < 0 {
			remaining = 0
		}
		return clampScore(remaining / float64(maxLife) * 100)

	case "family_coverage":
		familySize := len(pat.FamilyMembers)
		if familySize >= 10 {
			return 95
		}
		if familySize >= 5 {
			return 75
		}
		if familySize >= 2 {
			return 55
		}
		if familySize == 1 {
			return 35
		}
		return 20

	default:
		return 50
	}
}

func (s *valuationServiceImpl) ruleCommercialFactor(ctx context.Context, pat *patent.Patent, factor string, assessCtx *AssessmentContext) float64 {
	switch factor {
	case "market_relevance":
		// Heuristic: overlap between patent tech areas and company focus areas
		if len(assessCtx.TechFocusAreas) == 0 {
			return 60 // neutral when no context
		}
		matchCount := 0
		patText := strings.ToLower(pat.Title + " " + pat.Abstract)
		for _, area := range assessCtx.TechFocusAreas {
			if strings.Contains(patText, strings.ToLower(area)) {
				matchCount++
			}
		}
		score := float64(matchCount) / float64(len(assessCtx.TechFocusAreas)) * 100
		if score < 20 {
			score = 20
		}
		return score

	case "product_coverage":
		// Heuristic: number of claims as proxy for product coverage breadth
		claimCount := len(pat.Claims)
		if claimCount >= 20 {
			return 85
		}
		if claimCount >= 10 {
			return 70
		}
		if claimCount >= 5 {
			return 55
		}
		return 35

	case "licensing_potential":
		// Heuristic: granted + broad claims + active status → high licensing potential
		score := 50.0
		if pat.Status == "granted" || pat.Status == "active" {
			score += 20
		}
		if len(pat.Claims) > 10 {
			score += 15
		}
		if len(pat.FamilyMembers) > 3 {
			score += 10
		}
		return clampScore(score)

	case "cost_of_design_around":
		// Heuristic: more IPC classes + more claims → harder to design around
		score := 40.0
		score += float64(len(pat.IPCClassifications)) * 8
		score += float64(len(pat.Claims)) * 1.5
		return clampScore(score)

	case "industry_adoption":
		// Heuristic: forward citations as proxy for industry adoption
		return s.computeCitationImpact(ctx, pat)

	default:
		return 50
	}
}

func (s *valuationServiceImpl) ruleStrategicFactor(ctx context.Context, pat *patent.Patent, factor string, assessCtx *AssessmentContext) float64 {
	switch factor {
	case "portfolio_centrality":
		// Heuristic: patents cited by many others in the same domain are more central
		// Simplified: use forward citation count as proxy for centrality
		return s.computeCitationImpact(ctx, pat)

	case "blocking_power":
		// Heuristic: broad independent claims + multiple IPC classes → high blocking power
		score := 40.0
		indepClaims := 0
		for _, c := range pat.Claims {
			if c.IsIndependent {
				indepClaims++
			}
		}
		score += float64(indepClaims) * 10
		score += float64(len(pat.IPCClassifications)) * 5
		return clampScore(score)

	case "negotiation_leverage":
		// Heuristic: granted + large family + active → strong negotiation position
		score := 30.0
		if pat.Status == "granted" || pat.Status == "active" {
			score += 25
		}
		score += float64(len(pat.FamilyMembers)) * 5
		if len(pat.Claims) > 15 {
			score += 15
		}
		return clampScore(score)

	case "technology_trajectory_alignment":
		// Heuristic: recent filing date → more aligned with current tech trajectory
		if pat.FilingDate.IsZero() {
			return 50
		}
		yearsAgo := time.Since(pat.FilingDate).Hours() / (24 * 365.25)
		if yearsAgo <= 2 {
			return 90
		}
		if yearsAgo <= 5 {
			return 75
		}
		if yearsAgo <= 10 {
			return 55
		}
		return 35

	case "competitive_differentiation":
		// Heuristic: overlap with competitor names in patent text
		if len(assessCtx.Competitors) == 0 {
			return 60
		}
		patText := strings.ToLower(pat.Title + " " + pat.Abstract + " " + pat.Description)
		matchCount := 0
		for _, comp := range assessCtx.Competitors {
			if strings.Contains(patText, strings.ToLower(comp)) {
				matchCount++
			}
		}
		// More competitor mentions → patent is in contested space → higher differentiation value
		score := 50.0 + float64(matchCount)*15
		return clampScore(score)

	default:
		return 50
	}
}

// computeCitationImpact calculates citation_impact = forward_citations / max_in_domain * 100.
func (s *valuationServiceImpl) computeCitationImpact(ctx context.Context, pat *patent.Patent) float64 {
	if s.citationRepo == nil {
		return 50 // neutral when citation data unavailable
	}

	fwd, err := s.citationRepo.CountForwardCitations(ctx, pat.ID)
	if err != nil {
		s.logger.Warn("failed to count forward citations", "patent_id", pat.ID, "error", err)
		return 50
	}

	domain := ""
	if len(pat.IPCClassifications) > 0 {
		domain = pat.IPCClassifications[0]
	}

	maxFwd, err := s.citationRepo.MaxForwardCitationsInDomain(ctx, domain)
	if err != nil || maxFwd == 0 {
		// Fallback: use absolute thresholds
		if fwd >= 50 {
			return 95
		}
		if fwd >= 20 {
			return 75
		}
		if fwd >= 5 {
			return 55
		}
		return 30
	}

	score := float64(fwd) / float64(maxFwd) * 100
	return clampScore(score)
}

// ---------------------------------------------------------------------------
// Internal: Overall Valuation Computation
// ---------------------------------------------------------------------------

func (s *valuationServiceImpl) computeOverallValuation(scores map[AssessmentDimension]*DimensionScore) *OverallValuation {
	var weightedSum float64
	var totalWeight float64
	var calcParts []string

	// Use canonical dimension order for deterministic calculation string
	for _, dim := range AllDimensions() {
		ds, ok := scores[dim]
		if !ok {
			continue
		}
		w, wOk := DimensionWeights[dim]
		if !wOk {
			continue
		}
		contribution := w * ds.Score
		weightedSum += contribution
		totalWeight += w
		calcParts = append(calcParts, fmt.Sprintf("%s(%.2f×%.2f=%.2f)", dim, ds.Score, w, contribution))
	}

	// Normalize if not all dimensions were assessed
	finalScore := weightedSum
	if totalWeight > 0 && totalWeight < 1.0 {
		finalScore = weightedSum / totalWeight
	}
	finalScore = math.Round(finalScore*100) / 100

	tier := TierFromScore(finalScore)

	return &OverallValuation{
		Score:               finalScore,
		Tier:                tier,
		TierDescription:     TierDescription(tier),
		WeightedCalculation: strings.Join(calcParts, " + ") + fmt.Sprintf(" = %.2f", finalScore),
	}
}

// ---------------------------------------------------------------------------
// Internal: Action Recommendation Generation
// ---------------------------------------------------------------------------

func (s *valuationServiceImpl) generateRecommendations(
	pat *patent.Patent,
	overall *OverallValuation,
	scores map[AssessmentDimension]*DimensionScore,
) []*ActionRecommendation {
	var recs []*ActionRecommendation

	switch overall.Tier {
	case TierS:
		recs = append(recs, &ActionRecommendation{
			Type:     ActionMaintain,
			Priority: PriorityCritical,
			Action:   "Maintain all jurisdictions and monitor for infringement",
			Reason:   fmt.Sprintf("Crown jewel patent (score %.1f, Tier S) requires maximum protection", overall.Score),
		})
		recs = append(recs, &ActionRecommendation{
			Type:     ActionEnforce,
			Priority: PriorityHigh,
			Action:   "Proactively monitor competitor products for potential infringement",
			Reason:   "High-value patents warrant active enforcement surveillance",
		})

	case TierA:
		recs = append(recs, &ActionRecommendation{
			Type:     ActionMaintain,
			Priority: PriorityHigh,
			Action:   "Continue maintenance across all key jurisdictions",
			Reason:   fmt.Sprintf("Core asset (score %.1f, Tier A) merits sustained investment", overall.Score),
		})
		// Check if any dimension is weak → strengthen
		for dim, ds := range scores {
			if ds.Score < 60 {
				recs = append(recs, &ActionRecommendation{
					Type:     ActionStrengthen,
					Priority: PriorityMedium,
					Action:   fmt.Sprintf("Strengthen %s dimension (current score: %.1f)", dim, ds.Score),
					Reason:   "Improving weak dimensions could elevate this patent to Tier S",
				})
			}
		}

	case TierB:
		recs = append(recs, &ActionRecommendation{
			Type:     ActionMaintain,
			Priority: PriorityMedium,
			Action:   "Maintain in primary jurisdictions, review secondary markets",
			Reason:   fmt.Sprintf("Valuable asset (score %.1f, Tier B) with upgrade potential", overall.Score),
		})
		// Check commercial dimension for licensing opportunity
		if cs, ok := scores[DimensionCommercialValue]; ok && cs.Score >= 70 {
			recs = append(recs, &ActionRecommendation{
				Type:     ActionLicense,
				Priority: PriorityMedium,
				Action:   "Explore out-licensing opportunities to generate revenue",
				Reason:   fmt.Sprintf("Strong commercial value (%.1f) suggests licensing potential", cs.Score),
			})
		}

	case TierC:
		recs = append(recs, &ActionRecommendation{
			Type:     ActionLicense,
			Priority: PriorityMedium,
			Action:   "Evaluate licensing or cross-licensing opportunities",
			Reason:   fmt.Sprintf("Marginal asset (score %.1f, Tier C) may generate more value through licensing", overall.Score),
		})
		recs = append(recs, &ActionRecommendation{
			Type:     ActionAbandon,
			Priority: PriorityLow,
			Action:   "Consider scope reduction in non-core jurisdictions",
			Reason:   "Reducing maintenance scope can optimize portfolio costs",
		})

	case TierD:
		recs = append(recs, &ActionRecommendation{
			Type:     ActionAbandon,
			Priority: PriorityHigh,
			Action:   "Recommend abandonment or divestiture",
			Reason:   fmt.Sprintf("Low-value patent (score %.1f, Tier D) is a cost burden with minimal strategic benefit", overall.Score),
		})
		recs = append(recs, &ActionRecommendation{
			Type:     ActionLicense,
			Priority: PriorityMedium,
			Action:   "Attempt to sell or license before abandoning",
			Reason:   "Extract residual value before letting the patent lapse",
		})
	}

	// Cross-dimensional insight: if strategic is high but commercial is low → file new
	if ss, ok := scores[DimensionStrategicValue]; ok {
		if cs, ok2 := scores[DimensionCommercialValue]; ok2 {
			if ss.Score >= 75 && cs.Score < 50 {
				recs = append(recs, &ActionRecommendation{
					Type:     ActionFileNew,
					Priority: PriorityMedium,
					Action:   "Consider filing continuation or divisional to capture commercial applications",
					Reason:   fmt.Sprintf("High strategic value (%.1f) but low commercial value (%.1f) suggests untapped market potential", ss.Score, cs.Score),
				})
			}
		}
	}

	return recs
}

// generateRecommendationsFromRecord creates recommendations from a persisted record
// (used by RecommendActions when full dimension scores are not available).
func (s *valuationServiceImpl) generateRecommendationsFromRecord(record *AssessmentRecord) []*ActionRecommendation {
	// Build a synthetic OverallValuation
	overall := &OverallValuation{
		Score:           record.OverallScore,
		Tier:            record.Tier,
		TierDescription: TierDescription(record.Tier),
	}

	// Build synthetic DimensionScores from the record
	scores := make(map[AssessmentDimension]*DimensionScore, len(record.DimensionScores))
	for dim, sc := range record.DimensionScores {
		scores[dim] = &DimensionScore{
			Score:    sc,
			MaxScore: 100,
		}
	}

	return s.generateRecommendations(nil, overall, scores)
}

// ---------------------------------------------------------------------------
// Internal: Portfolio Summary
// ---------------------------------------------------------------------------

func (s *valuationServiceImpl) buildPortfolioSummary(
	assessments []*SinglePatentAssessmentResponse,
	assessCtx *AssessmentContext,
) *PortfolioSummary {
	summary := &PortfolioSummary{
		TotalAssessed:    len(assessments),
		TierDistribution: map[PatentTier]int{TierS: 0, TierA: 0, TierB: 0, TierC: 0, TierD: 0},
		Currency:         assessCtx.CurrencyCode,
	}

	if len(assessments) == 0 {
		return summary
	}

	var totalScore float64
	var totalCost float64

	for _, a := range assessments {
		tier := a.OverallScore.Tier
		summary.TierDistribution[tier]++
		totalScore += a.OverallScore.Score

		if cost, ok := defaultAnnualMaintenanceCost[tier]; ok {
			totalCost += cost
		}
	}

	summary.AverageScore = math.Round(totalScore/float64(len(assessments))*100) / 100
	summary.TotalMaintenanceCost = totalCost

	// Strategic gap identification: if no Tier S/A patents in any focus area → gap
	highTierCount := summary.TierDistribution[TierS] + summary.TierDistribution[TierA]
	focusAreaCount := len(assessCtx.TechFocusAreas)
	if focusAreaCount > 0 && highTierCount < focusAreaCount {
		summary.StrategicGapsIdentified = focusAreaCount - highTierCount
		if summary.StrategicGapsIdentified < 0 {
			summary.StrategicGapsIdentified = 0
		}
	}

	// Recommend new filings based on gaps
	summary.RecommendedNewFilings = summary.StrategicGapsIdentified

	return summary
}

// ---------------------------------------------------------------------------
// Internal: Cost Optimization Analysis
// ---------------------------------------------------------------------------

func (s *valuationServiceImpl) analyzeCostOptimization(
	assessments []*SinglePatentAssessmentResponse,
	assessCtx *AssessmentContext,
) *CostOptimizationResult {
	result := &CostOptimizationResult{
		Currency: assessCtx.CurrencyCode,
	}

	for _, a := range assessments {
		tier := a.OverallScore.Tier
		annualCost, ok := defaultAnnualMaintenanceCost[tier]
		if !ok {
			annualCost = 10000 // fallback
		}
		result.CurrentAnnualCost += annualCost

		switch tier {
		case TierS, TierA:
			// Keep maintaining
			result.OptimizedAnnualCost += annualCost
			result.Recommendations = append(result.Recommendations, &CostRecommendation{
				PatentID:        a.PatentID,
				CurrentTier:     tier,
				Action:          CostContinueMaintain,
				EstimatedSaving: 0,
				Currency:        assessCtx.CurrencyCode,
				Reason:          fmt.Sprintf("High-value %s-tier patent warrants continued full maintenance", tier),
			})

		case TierB:
			// Maintain but review periodically
			result.OptimizedAnnualCost += annualCost
			result.Recommendations = append(result.Recommendations, &CostRecommendation{
				PatentID:        a.PatentID,
				CurrentTier:     tier,
				Action:          CostContinueMaintain,
				EstimatedSaving: 0,
				Currency:        assessCtx.CurrencyCode,
				Reason:          "B-tier patent: maintain with periodic review for upgrade or scope reduction",
			})

		case TierC:
			// Reduce scope: save ~40% of maintenance cost
			saving := annualCost * 0.40
			result.OptimizedAnnualCost += annualCost - saving
			result.Recommendations = append(result.Recommendations, &CostRecommendation{
				PatentID:        a.PatentID,
				CurrentTier:     tier,
				Action:          CostReduceScope,
				EstimatedSaving: saving,
				Currency:        assessCtx.CurrencyCode,
				Reason:          "C-tier patent: reduce maintenance to core jurisdictions only",
			})

		case TierD:
			// Abandon: save 100% of maintenance cost
			saving := annualCost
			result.OptimizedAnnualCost += 0
			result.Recommendations = append(result.Recommendations, &CostRecommendation{
				PatentID:        a.PatentID,
				CurrentTier:     tier,
				Action:          CostAbandon,
				EstimatedSaving: saving,
				Currency:        assessCtx.CurrencyCode,
				Reason:          "D-tier patent: recommend abandonment to eliminate maintenance costs",
			})
		}
	}

	result.SavingsPotential = result.CurrentAnnualCost - result.OptimizedAnnualCost
	return result
}

// ---------------------------------------------------------------------------
// Utility
// ---------------------------------------------------------------------------

// clampScore constrains a score to the [0, 100] range.
func clampScore(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return math.Round(v*100) / 100
}

// Compile-time interface compliance check.
var _ ValuationService = (*valuationServiceImpl)(nil)

//Personal.AI order the ending


// -------------------------------------------------------------------
// File: internal/application/infringement/risk_assessment.go
// Phase 10 - 序号 204
// KeyIP-Intelligence: AI-Driven Intellectual Property Lifecycle
//                     Management Platform for OLED Materials
// -------------------------------------------------------------------
// Generation Plan (embedded as per specification):
//
// 功能定位: 侵权风险评估业务编排层，协调分子相似度引擎、权利要求
//   解析引擎、侵权判定网络与领域服务，完成从单分子到组合级别的
//   全维度侵权风险评估。
// 核心实现:
//   - 定义 RiskAssessmentService 接口 (5 methods)
//   - 定义请求/响应 DTO 全集
//   - 实现 riskAssessmentServiceImpl 结构体
//   - 方法流程: 校验→标准化→缓存→AI推理→聚合→持久化→返回
// 业务逻辑:
//   - 风险评分: 0.35*literal + 0.30*equivalents + 0.20*breadth + 0.15*penalty
//   - 等级映射: CRITICAL(>=85), HIGH(>=70), MEDIUM(>=50), LOW(>=30), NONE(<30)
//   - FTO结论: BLOCKED / CONDITIONAL / FREE
//   - 缓存TTL=24h, InChIKey+参数hash为键
//   - 批量并发度默认5, errgroup+semaphore
// 算法原理:
//   - All-Elements Rule (字面侵权)
//   - Function-Way-Result Test (等同侵权)
//   - Prosecution History Estoppel (禁止反悔)
//   - 多指纹融合: 0.30*Morgan + 0.20*RDKit + 0.15*AtomPair + 0.35*GNN
// 依赖:
//   domain/molecule, domain/patent, intelligence/infringe_net,
//   intelligence/claim_bert, intelligence/molpatent_gnn,
//   infrastructure/redis, infrastructure/monitoring
// 被依赖:
//   interfaces/http/handlers, application/infringement/alert,
//   application/reporting/fto_report, application/reporting/infringement_report
// 强制约束: 文件最后一行必须为 //Personal.AI order the ending
// -------------------------------------------------------------------

package infringement

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/turtacn/KeyIP-Intelligence/internal/domain/molecule"
	"github.com/turtacn/KeyIP-Intelligence/internal/domain/patent"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/database/redis"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/prometheus"
	"github.com/turtacn/KeyIP-Intelligence/internal/intelligence/claim_bert"
	"github.com/turtacn/KeyIP-Intelligence/internal/intelligence/infringe_net"
	"github.com/turtacn/KeyIP-Intelligence/internal/intelligence/molpatent_gnn"
	commonTypes "github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
	moleculeTypes "github.com/turtacn/KeyIP-Intelligence/pkg/types/molecule"
	patentTypes "github.com/turtacn/KeyIP-Intelligence/pkg/types/patent"
	pkgErrors "github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

const (
	// DefaultBatchConcurrency controls the default number of concurrent
	// molecule assessments in a batch operation.
	DefaultBatchConcurrency = 5

	// MaxBatchConcurrency is the upper bound for batch concurrency to
	// prevent resource exhaustion.
	MaxBatchConcurrency = 20

	// MaxBatchSize is the maximum number of molecules in a single batch
	// request.
	MaxBatchSize = 500

	// RiskCacheTTL is the time-to-live for cached risk assessment results.
	RiskCacheTTL = 24 * time.Hour

	// RiskCachePrefix is the key prefix for risk assessment cache entries.
	RiskCachePrefix = "risk:assess:"

	// FTOCachePrefix is the key prefix for FTO analysis cache entries.
	FTOCachePrefix = "risk:fto:"

	// DefaultSimilarityThreshold is the minimum similarity score for a
	// patent to be considered a candidate during risk assessment.
	DefaultSimilarityThreshold = 0.55

	// DefaultMaxCandidates is the maximum number of candidate patents
	// returned from similarity search.
	DefaultMaxCandidates = 100

	// DefaultRiskHistoryPageSize is the default page size for risk
	// history queries.
	DefaultRiskHistoryPageSize = 20
)

// ---------------------------------------------------------------------------
// Risk Level Enumeration
// ---------------------------------------------------------------------------

// RiskLevel represents the assessed infringement risk severity.
type RiskLevel string

const (
	RiskLevelCritical RiskLevel = "CRITICAL"
	RiskLevelHigh     RiskLevel = "HIGH"
	RiskLevelMedium   RiskLevel = "MEDIUM"
	RiskLevelLow      RiskLevel = "LOW"
	RiskLevelNone     RiskLevel = "NONE"
)

// RiskLevelFromScore maps a numeric risk score (0-100) to a RiskLevel.
// Thresholds: CRITICAL >= 85, HIGH >= 70, MEDIUM >= 50, LOW >= 30, NONE < 30.
func RiskLevelFromScore(score float64) RiskLevel {
	switch {
	case score >= 85.0:
		return RiskLevelCritical
	case score >= 70.0:
		return RiskLevelHigh
	case score >= 50.0:
		return RiskLevelMedium
	case score >= 30.0:
		return RiskLevelLow
	default:
		return RiskLevelNone
	}
}

// Severity returns a numeric ordering for sorting (higher = more severe).
func (r RiskLevel) Severity() int {
	switch r {
	case RiskLevelCritical:
		return 5
	case RiskLevelHigh:
		return 4
	case RiskLevelMedium:
		return 3
	case RiskLevelLow:
		return 2
	case RiskLevelNone:
		return 1
	default:
		return 0
	}
}

// ---------------------------------------------------------------------------
// Analysis Depth Enumeration
// ---------------------------------------------------------------------------

// AnalysisDepth controls the thoroughness of risk assessment.
type AnalysisDepth string

const (
	// AnalysisDepthQuick performs fingerprint-only similarity with basic
	// claim matching. Suitable for rapid screening.
	AnalysisDepthQuick AnalysisDepth = "quick"

	// AnalysisDepthStandard adds GNN embedding similarity and ClaimBERT
	// semantic analysis. Default for most use cases.
	AnalysisDepthStandard AnalysisDepth = "standard"

	// AnalysisDepthDeep includes full InfringeNet element-by-element
	// comparison, equivalents analysis, and prosecution history check.
	AnalysisDepthDeep AnalysisDepth = "deep"
)

// IsValid returns true if the depth value is recognized.
func (d AnalysisDepth) IsValid() bool {
	switch d {
	case AnalysisDepthQuick, AnalysisDepthStandard, AnalysisDepthDeep:
		return true
	default:
		return false
	}
}

// ---------------------------------------------------------------------------
// FTO Conclusion Enumeration
// ---------------------------------------------------------------------------

// FTOConclusion represents the freedom-to-operate determination for a
// specific jurisdiction.
type FTOConclusion string

const (
	// FTOFree indicates no blocking patents were identified.
	FTOFree FTOConclusion = "FREE"

	// FTOConditional indicates HIGH-risk patents exist but none are
	// CRITICAL. Proceed with caution and legal review.
	FTOConditional FTOConclusion = "CONDITIONAL"

	// FTOBlocked indicates at least one CRITICAL-risk patent blocks
	// commercial use in this jurisdiction.
	FTOBlocked FTOConclusion = "BLOCKED"
)

// ---------------------------------------------------------------------------
// Trigger Type Enumeration
// ---------------------------------------------------------------------------

// TriggerType indicates what initiated a risk assessment.
type TriggerType string

const (
	TriggerManual    TriggerType = "manual"
	TriggerAutomatic TriggerType = "automatic"
	TriggerMonitor   TriggerType = "monitor"
	TriggerScheduled TriggerType = "scheduled"
)

// ---------------------------------------------------------------------------
// Request / Response DTOs
// ---------------------------------------------------------------------------

// MoleculeRiskRequest is the input for single-molecule risk assessment.
type MoleculeRiskRequest struct {
	// MoleculeInput is the molecular structure to assess. Exactly one of
	// SMILES or InChI must be provided.
	SMILES string `json:"smiles,omitempty"`
	InChI  string `json:"inchi,omitempty"`

	// PatentOffices restricts the search to specific patent offices.
	// Empty means all supported offices.
	PatentOffices []string `json:"patent_offices,omitempty"`

	// CompetitorFilter restricts candidate patents to specific assignees.
	CompetitorFilter []string `json:"competitor_filter,omitempty"`

	// SimilarityThreshold overrides the default minimum similarity for
	// candidate selection. Range: (0, 1].
	SimilarityThreshold float64 `json:"similarity_threshold,omitempty"`

	// MaxCandidates overrides the default maximum number of candidate
	// patents to evaluate.
	MaxCandidates int `json:"max_candidates,omitempty"`

	// Depth controls the analysis thoroughness.
	Depth AnalysisDepth `json:"depth,omitempty"`

	// TechDomains restricts the search to specific KeyIP technology
	// classification codes.
	TechDomains []string `json:"tech_domains,omitempty"`

	// DateRange restricts candidate patents by filing/publication date.
	DateFrom *time.Time `json:"date_from,omitempty"`
	DateTo   *time.Time `json:"date_to,omitempty"`

	// ExcludePatents is a list of patent numbers to exclude (e.g., own
	// patents).
	ExcludePatents []string `json:"exclude_patents,omitempty"`

	// Trigger records what initiated this assessment.
	Trigger TriggerType `json:"trigger,omitempty"`
}

// Validate checks the request for correctness.
func (r *MoleculeRiskRequest) Validate() error {
	if r.SMILES == "" && r.InChI == "" {
		return pkgErrors.NewValidation("molecule_risk_request", "either smiles or inchi must be provided")
	}
	if r.SMILES != "" && r.InChI != "" {
		return pkgErrors.NewValidation("molecule_risk_request", "provide only one of smiles or inchi, not both")
	}
	if r.SimilarityThreshold != 0 && (r.SimilarityThreshold <= 0 || r.SimilarityThreshold > 1) {
		return pkgErrors.NewValidation("similarity_threshold", "must be in range (0, 1]")
	}
	if r.MaxCandidates < 0 || r.MaxCandidates > 1000 {
		return pkgErrors.NewValidation("max_candidates", "must be in range [0, 1000]")
	}
	if r.Depth != "" && !r.Depth.IsValid() {
		return pkgErrors.NewValidation("depth", "must be one of: quick, standard, deep")
	}
	if r.DateFrom != nil && r.DateTo != nil && r.DateFrom.After(*r.DateTo) {
		return pkgErrors.NewValidation("date_range", "date_from must be before date_to")
	}
	return nil
}

// defaults fills in zero-value fields with sensible defaults.
func (r *MoleculeRiskRequest) defaults() {
	if r.SimilarityThreshold == 0 {
		r.SimilarityThreshold = DefaultSimilarityThreshold
	}
	if r.MaxCandidates == 0 {
		r.MaxCandidates = DefaultMaxCandidates
	}
	if r.Depth == "" {
		r.Depth = AnalysisDepthStandard
	}
	if r.Trigger == "" {
		r.Trigger = TriggerManual
	}
}

// cacheKey produces a deterministic cache key for this request.
func (r *MoleculeRiskRequest) cacheKey(canonicalSMILES string) string {
	h := sha256.New()
	h.Write([]byte(canonicalSMILES))
	h.Write([]byte(fmt.Sprintf("|depth=%s", r.Depth)))
	h.Write([]byte(fmt.Sprintf("|threshold=%.4f", r.SimilarityThreshold)))
	if len(r.PatentOffices) > 0 {
		sorted := make([]string, len(r.PatentOffices))
		copy(sorted, r.PatentOffices)
		sort.Strings(sorted)
		for _, o := range sorted {
			h.Write([]byte("|office=" + o))
		}
	}
	if len(r.CompetitorFilter) > 0 {
		sorted := make([]string, len(r.CompetitorFilter))
		copy(sorted, r.CompetitorFilter)
		sort.Strings(sorted)
		for _, c := range sorted {
			h.Write([]byte("|comp=" + c))
		}
	}
	if len(r.TechDomains) > 0 {
		sorted := make([]string, len(r.TechDomains))
		copy(sorted, r.TechDomains)
		sort.Strings(sorted)
		for _, d := range sorted {
			h.Write([]byte("|td=" + d))
		}
	}
	return RiskCachePrefix + hex.EncodeToString(h.Sum(nil))[:32]
}

// MoleculeRiskResponse is the output of single-molecule risk assessment.
type MoleculeRiskResponse struct {
	// AssessmentID is a unique identifier for this assessment record.
	AssessmentID string `json:"assessment_id"`

	// Molecule echoes back the canonical representation.
	CanonicalSMILES string `json:"canonical_smiles"`
	InChIKey        string `json:"inchi_key,omitempty"`

	// OverallRisk is the aggregated risk determination.
	OverallRiskLevel RiskLevel `json:"overall_risk_level"`
	OverallRiskScore float64   `json:"overall_risk_score"`

	// Component scores contributing to the overall score.
	LiteralInfringementScore    float64 `json:"literal_infringement_score"`
	EquivalentsInfringementScore float64 `json:"equivalents_infringement_score"`
	ClaimBreadthScore           float64 `json:"claim_breadth_score"`
	ProsecutionHistoryPenalty   float64 `json:"prosecution_history_penalty"`

	// MatchedPatents lists the candidate patents that contributed to the
	// risk score, ordered by descending risk.
	MatchedPatents []PatentRiskDetail `json:"matched_patents"`

	// Summary is a human-readable risk summary.
	Summary string `json:"summary"`

	// Metadata captures processing details.
	CandidatesSearched int           `json:"candidates_searched"`
	AnalysisDepth      AnalysisDepth `json:"analysis_depth"`
	ProcessingTime     time.Duration `json:"processing_time_ms"`
	CacheHit           bool          `json:"cache_hit"`
	AssessedAt         time.Time     `json:"assessed_at"`
}

// PatentRiskDetail describes the risk contribution of a single patent.
type PatentRiskDetail struct {
	PatentNumber    string    `json:"patent_number"`
	Title           string    `json:"title"`
	Assignee        string    `json:"assignee"`
	FilingDate      time.Time `json:"filing_date"`
	LegalStatus     string    `json:"legal_status"`
	IPCCodes        []string  `json:"ipc_codes,omitempty"`

	// Similarity scores from the multi-fingerprint fusion.
	SimilarityScores SimilarityScores `json:"similarity_scores"`

	// Claim-level analysis results.
	RelevantClaims []ClaimRiskDetail `json:"relevant_claims,omitempty"`

	// Aggregated risk for this patent.
	PatentRiskScore float64   `json:"patent_risk_score"`
	PatentRiskLevel RiskLevel `json:"patent_risk_level"`
}

// SimilarityScores holds the per-fingerprint and fused similarity values.
type SimilarityScores struct {
	Morgan          float64 `json:"morgan"`
	RDKit           float64 `json:"rdkit"`
	AtomPair        float64 `json:"atom_pair"`
	GNN             float64 `json:"gnn"`
	WeightedOverall float64 `json:"weighted_overall"`
}

// FusedScore computes the weighted overall similarity.
// Weights: Morgan=0.30, RDKit=0.20, AtomPair=0.15, GNN=0.35
func (s *SimilarityScores) FusedScore() float64 {
	fused := 0.30*s.Morgan + 0.20*s.RDKit + 0.15*s.AtomPair + 0.35*s.GNN
	s.WeightedOverall = fused
	return fused
}

// ClaimRiskDetail describes the infringement analysis for a single claim.
type ClaimRiskDetail struct {
	ClaimNumber       int       `json:"claim_number"`
	ClaimType         string    `json:"claim_type"` // independent / dependent
	LiteralMatch      bool      `json:"literal_match"`
	MarkushCovered    bool      `json:"markush_covered"`
	EquivalentsMatch  bool      `json:"equivalents_match"`
	EstoppelApplies   bool      `json:"estoppel_applies"`
	ClaimRiskScore    float64   `json:"claim_risk_score"`
	ClaimRiskLevel    RiskLevel `json:"claim_risk_level"`
	Explanation       string    `json:"explanation"`
}

// BatchRiskRequest is the input for batch molecule risk assessment.
type BatchRiskRequest struct {
	Molecules []BatchMoleculeInput `json:"molecules"`

	// Concurrency overrides the default batch concurrency. Range: [1, MaxBatchConcurrency].
	Concurrency int `json:"concurrency,omitempty"`

	// Common filters applied to all molecules in the batch.
	PatentOffices       []string      `json:"patent_offices,omitempty"`
	CompetitorFilter    []string      `json:"competitor_filter,omitempty"`
	SimilarityThreshold float64       `json:"similarity_threshold,omitempty"`
	Depth               AnalysisDepth `json:"depth,omitempty"`
	TechDomains         []string      `json:"tech_domains,omitempty"`
	ExcludePatents      []string      `json:"exclude_patents,omitempty"`
	Trigger             TriggerType   `json:"trigger,omitempty"`
}

// BatchMoleculeInput identifies a single molecule within a batch.
type BatchMoleculeInput struct {
	ID     string `json:"id,omitempty"` // caller-assigned identifier
	SMILES string `json:"smiles,omitempty"`
	InChI  string `json:"inchi,omitempty"`
	Name   string `json:"name,omitempty"`
}

// Validate checks the batch request for correctness.
func (r *BatchRiskRequest) Validate() error {
	if len(r.Molecules) == 0 {
		return pkgErrors.NewValidation("batch_risk_request", "molecules list must not be empty")
	}
	if len(r.Molecules) > MaxBatchSize {
		return pkgErrors.NewValidation("batch_risk_request",
			fmt.Sprintf("batch size %d exceeds maximum %d", len(r.Molecules), MaxBatchSize))
	}
	if r.Concurrency < 0 || r.Concurrency > MaxBatchConcurrency {
		return pkgErrors.NewValidation("concurrency",
			fmt.Sprintf("must be in range [0, %d]", MaxBatchConcurrency))
	}
	for i, m := range r.Molecules {
		if m.SMILES == "" && m.InChI == "" {
			return pkgErrors.NewValidation("molecules",
				fmt.Sprintf("molecule at index %d must have smiles or inchi", i))
		}
	}
	return nil
}

// defaults fills in zero-value fields.
func (r *BatchRiskRequest) defaults() {
	if r.Concurrency == 0 {
		r.Concurrency = DefaultBatchConcurrency
	}
	if r.Depth == "" {
		r.Depth = AnalysisDepthStandard
	}
	if r.Trigger == "" {
		r.Trigger = TriggerManual
	}
}

// BatchRiskResponse is the output of batch molecule risk assessment.
type BatchRiskResponse struct {
	Results []BatchMoleculeResult `json:"results"`
	Stats   BatchRiskStats        `json:"stats"`

	TotalProcessingTime time.Duration `json:"total_processing_time_ms"`
	AssessedAt          time.Time     `json:"assessed_at"`
}
// BatchMoleculeResult holds the assessment outcome for one molecule in a batch.
type BatchMoleculeResult struct {
	Index      int                  `json:"index"`
	ID         string               `json:"id,omitempty"`
	Name       string               `json:"name,omitempty"`
	Response   *MoleculeRiskResponse `json:"response,omitempty"`
	Error      string               `json:"error,omitempty"`
	Succeeded  bool                 `json:"succeeded"`
}

// BatchRiskStats aggregates statistics across all molecules in a batch.
type BatchRiskStats struct {
	Total          int            `json:"total"`
	Succeeded      int            `json:"succeeded"`
	Failed         int            `json:"failed"`
	CacheHits      int            `json:"cache_hits"`
	RiskDistribution map[RiskLevel]int `json:"risk_distribution"`
	HighRiskCount  int            `json:"high_risk_count"`
	AverageScore   float64        `json:"average_score"`
	MaxScore       float64        `json:"max_score"`
	MinScore       float64        `json:"min_score"`
}

// FTORequest is the input for freedom-to-operate analysis.
type FTORequest struct {
	// Molecules to evaluate for FTO.
	Molecules []BatchMoleculeInput `json:"molecules"`

	// Jurisdictions lists the target market jurisdictions (ISO 3166-1 alpha-2).
	Jurisdictions []string `json:"jurisdictions"`

	// Scope controls whether to include only active patents or all.
	Scope string `json:"scope,omitempty"` // "active" (default) or "all"

	// ExcludePatents lists patent numbers to exclude (e.g., own portfolio).
	ExcludePatents []string `json:"exclude_patents,omitempty"`

	// SimilarityThreshold overrides the default.
	SimilarityThreshold float64 `json:"similarity_threshold,omitempty"`

	// Depth controls analysis thoroughness.
	Depth AnalysisDepth `json:"depth,omitempty"`

	// Trigger records what initiated this FTO analysis.
	Trigger TriggerType `json:"trigger,omitempty"`
}

// Validate checks the FTO request for correctness.
func (r *FTORequest) Validate() error {
	if len(r.Molecules) == 0 {
		return pkgErrors.NewValidation("fto_request", "molecules list must not be empty")
	}
	if len(r.Molecules) > MaxBatchSize {
		return pkgErrors.NewValidation("fto_request",
			fmt.Sprintf("molecule count %d exceeds maximum %d", len(r.Molecules), MaxBatchSize))
	}
	if len(r.Jurisdictions) == 0 {
		return pkgErrors.NewValidation("fto_request", "at least one jurisdiction must be specified")
	}
	for i, m := range r.Molecules {
		if m.SMILES == "" && m.InChI == "" {
			return pkgErrors.NewValidation("molecules",
				fmt.Sprintf("molecule at index %d must have smiles or inchi", i))
		}
	}
	if r.Scope != "" && r.Scope != "active" && r.Scope != "all" {
		return pkgErrors.NewValidation("scope", "must be 'active' or 'all'")
	}
	return nil
}

// defaults fills in zero-value fields.
func (r *FTORequest) defaults() {
	if r.Scope == "" {
		r.Scope = "active"
	}
	if r.SimilarityThreshold == 0 {
		r.SimilarityThreshold = DefaultSimilarityThreshold
	}
	if r.Depth == "" {
		r.Depth = AnalysisDepthDeep
	}
	if r.Trigger == "" {
		r.Trigger = TriggerManual
	}
}

// FTOResponse is the output of freedom-to-operate analysis.
type FTOResponse struct {
	// FTOID is a unique identifier for this FTO analysis.
	FTOID string `json:"fto_id"`

	// JurisdictionResults maps each jurisdiction to its FTO determination.
	JurisdictionResults []JurisdictionFTOResult `json:"jurisdiction_results"`

	// BlockingPatents lists all patents that block FTO in any jurisdiction.
	BlockingPatents []BlockingPatentDetail `json:"blocking_patents"`

	// RiskMatrix is a molecule × jurisdiction matrix of risk levels.
	RiskMatrix []FTORiskMatrixRow `json:"risk_matrix"`

	// RecommendedActions provides actionable guidance.
	RecommendedActions []FTOAction `json:"recommended_actions"`

	// OverallConclusion is the worst-case FTO across all jurisdictions.
	OverallConclusion FTOConclusion `json:"overall_conclusion"`

	TotalProcessingTime time.Duration `json:"total_processing_time_ms"`
	AssessedAt          time.Time     `json:"assessed_at"`
}

// JurisdictionFTOResult holds the FTO determination for one jurisdiction.
type JurisdictionFTOResult struct {
	Jurisdiction   string        `json:"jurisdiction"`
	Conclusion     FTOConclusion `json:"conclusion"`
	CriticalCount  int           `json:"critical_count"`
	HighCount      int           `json:"high_count"`
	MediumCount    int           `json:"medium_count"`
	PatentsChecked int           `json:"patents_checked"`
	Summary        string        `json:"summary"`
}

// BlockingPatentDetail describes a patent that blocks FTO.
type BlockingPatentDetail struct {
	PatentNumber   string    `json:"patent_number"`
	Title          string    `json:"title"`
	Assignee       string    `json:"assignee"`
	Jurisdictions  []string  `json:"jurisdictions"`
	RiskLevel      RiskLevel `json:"risk_level"`
	RiskScore      float64   `json:"risk_score"`
	ExpirationDate time.Time `json:"expiration_date,omitempty"`
}

// FTORiskMatrixRow represents one molecule's risk across jurisdictions.
type FTORiskMatrixRow struct {
	MoleculeID    string               `json:"molecule_id"`
	MoleculeName  string               `json:"molecule_name,omitempty"`
	SMILES        string               `json:"smiles"`
	Jurisdictions map[string]RiskLevel `json:"jurisdictions"`
}

// FTOAction represents a recommended action from FTO analysis.
type FTOAction struct {
	Priority    string `json:"priority"` // "immediate", "short_term", "long_term"
	Category    string `json:"category"` // "design_around", "license", "challenge", "monitor"
	Description string `json:"description"`
	PatentRef   string `json:"patent_ref,omitempty"`
}

// RiskSummaryResponse provides portfolio-level risk aggregation.
type RiskSummaryResponse struct {
	PortfolioID string `json:"portfolio_id"`

	// OverallRiskScore is the portfolio-level aggregated risk.
	OverallRiskScore float64   `json:"overall_risk_score"`
	OverallRiskLevel RiskLevel `json:"overall_risk_level"`

	// TotalMolecules is the number of molecules in the portfolio.
	TotalMolecules int `json:"total_molecules"`
	AssessedCount  int `json:"assessed_count"`

	// RiskDistribution maps risk levels to molecule counts.
	RiskDistribution map[RiskLevel]int `json:"risk_distribution"`

	// TechDomainRisks breaks down risk by technology domain.
	TechDomainRisks []TechDomainRisk `json:"tech_domain_risks"`

	// HighRiskPatentsTopN lists the most dangerous patents.
	HighRiskPatentsTopN []PatentRiskDetail `json:"high_risk_patents_top_n"`

	// Trend captures risk score changes over recent months.
	Trend []RiskTrendPoint `json:"trend"`

	GeneratedAt time.Time `json:"generated_at"`
}

// TechDomainRisk aggregates risk for a technology domain.
type TechDomainRisk struct {
	Domain        string    `json:"domain"`
	MoleculeCount int       `json:"molecule_count"`
	AverageScore  float64   `json:"average_score"`
	MaxScore      float64   `json:"max_score"`
	RiskLevel     RiskLevel `json:"risk_level"`
}

// RiskTrendPoint is a single data point in a risk trend series.
type RiskTrendPoint struct {
	Month        string  `json:"month"` // "2024-01"
	AverageScore float64 `json:"average_score"`
	MaxScore     float64 `json:"max_score"`
	HighCount    int     `json:"high_count"`
}

// RiskRecord is an immutable audit record of a risk assessment.
type RiskRecord struct {
	RecordID     string      `json:"record_id"`
	MoleculeID   string      `json:"molecule_id"`
	SMILES       string      `json:"smiles"`
	InChIKey     string      `json:"inchi_key,omitempty"`
	Trigger      TriggerType `json:"trigger"`
	RiskLevel    RiskLevel   `json:"risk_level"`
	RiskScore    float64     `json:"risk_score"`
	MatchCount   int         `json:"match_count"`
	Depth        AnalysisDepth `json:"depth"`
	InputHash    string      `json:"input_hash"`
	ResultJSON   string      `json:"result_json,omitempty"`
	CreatedAt    time.Time   `json:"created_at"`
}

// QueryOption configures optional parameters for risk history queries.
type QueryOption func(*queryOptions)

type queryOptions struct {
	pageSize   int
	pageToken  string
	fromDate   *time.Time
	toDate     *time.Time
	triggerFilter []TriggerType
	levelFilter   []RiskLevel
}

// WithPageSize sets the page size for paginated queries.
func WithPageSize(size int) QueryOption {
	return func(o *queryOptions) {
		if size > 0 && size <= 100 {
			o.pageSize = size
		}
	}
}

// WithPageToken sets the pagination cursor.
func WithPageToken(token string) QueryOption {
	return func(o *queryOptions) {
		o.pageToken = token
	}
}

// WithDateRange restricts results to a date range.
func WithDateRange(from, to time.Time) QueryOption {
	return func(o *queryOptions) {
		o.fromDate = &from
		o.toDate = &to
	}
}

// WithTriggerFilter restricts results to specific trigger types.
func WithTriggerFilter(triggers ...TriggerType) QueryOption {
	return func(o *queryOptions) {
		o.triggerFilter = triggers
	}
}

// WithLevelFilter restricts results to specific risk levels.
func WithLevelFilter(levels ...RiskLevel) QueryOption {
	return func(o *queryOptions) {
		o.levelFilter = levels
	}
}

func applyQueryOptions(opts []QueryOption) *queryOptions {
	o := &queryOptions{
		pageSize: DefaultRiskHistoryPageSize,
	}
	for _, fn := range opts {
		fn(o)
	}
	return o
}

// ---------------------------------------------------------------------------
// Repository Interfaces
// ---------------------------------------------------------------------------

// RiskRecordRepository persists immutable risk assessment records.
type RiskRecordRepository interface {
	// Save persists a new risk record.
	Save(ctx context.Context, record *RiskRecord) error

	// FindByMolecule retrieves risk records for a molecule, ordered by
	// creation time descending.
	FindByMolecule(ctx context.Context, moleculeID string, opts *queryOptions) ([]*RiskRecord, string, error)

	// FindByPortfolio retrieves the latest risk record for each molecule
	// in a portfolio.
	FindByPortfolio(ctx context.Context, portfolioID string) ([]*RiskRecord, error)

	// FindByID retrieves a single risk record by its ID.
	FindByID(ctx context.Context, recordID string) (*RiskRecord, error)

	// GetTrend retrieves monthly aggregated risk data for a portfolio.
	GetTrend(ctx context.Context, portfolioID string, months int) ([]*RiskTrendPoint, error)
}

// FTOReportRepository persists FTO analysis reports.
type FTOReportRepository interface {
	// SaveFTOReport persists a complete FTO analysis result.
	SaveFTOReport(ctx context.Context, report *FTOResponse) error

	// FindFTOReport retrieves an FTO report by its ID.
	FindFTOReport(ctx context.Context, ftoID string) (*FTOResponse, error)
}

// ---------------------------------------------------------------------------
// Event Types
// ---------------------------------------------------------------------------

// RiskAssessmentEvent is published after each risk assessment completes.
type RiskAssessmentEvent struct {
	EventType    string    `json:"event_type"` // "risk.assessed"
	AssessmentID string    `json:"assessment_id"`
	MoleculeID   string    `json:"molecule_id,omitempty"`
	SMILES       string    `json:"smiles"`
	RiskLevel    RiskLevel `json:"risk_level"`
	RiskScore    float64   `json:"risk_score"`
	Trigger      TriggerType `json:"trigger"`
	Timestamp    time.Time `json:"timestamp"`
}

// EventPublisher publishes domain events.
type EventPublisher interface {
	Publish(ctx context.Context, event interface{}) error
}

// ---------------------------------------------------------------------------
// Service Interface
// ---------------------------------------------------------------------------

// RiskAssessmentService defines the application-level API for infringement
// risk assessment. It orchestrates domain services, AI inference engines,
// caching, and persistence to deliver comprehensive risk evaluations.
type RiskAssessmentService interface {
	// AssessMolecule performs a full infringement risk assessment for a
	// single molecule against the patent corpus.
	AssessMolecule(ctx context.Context, req *MoleculeRiskRequest) (*MoleculeRiskResponse, error)

	// AssessBatch performs risk assessment for multiple molecules with
	// controlled concurrency.
	AssessBatch(ctx context.Context, req *BatchRiskRequest) (*BatchRiskResponse, error)

	// AssessFTO performs freedom-to-operate analysis across multiple
	// jurisdictions.
	AssessFTO(ctx context.Context, req *FTORequest) (*FTOResponse, error)

	// GetRiskSummary returns a portfolio-level risk aggregation.
	GetRiskSummary(ctx context.Context, portfolioID string) (*RiskSummaryResponse, error)

	// GetRiskHistory returns historical risk assessment records for a
	// molecule.
	GetRiskHistory(ctx context.Context, moleculeID string, opts ...QueryOption) ([]*RiskRecord, error)
}

// ---------------------------------------------------------------------------
// Service Implementation
// ---------------------------------------------------------------------------

// riskAssessmentServiceImpl orchestrates all components required for
// infringement risk assessment.
type riskAssessmentServiceImpl struct {
	moleculeSvc    molecule.MoleculeDomainService
	patentSvc      patent.PatentDomainService
	infringeNet    infringe_net.InfringementAssessor
	claimParser    claim_bert.ClaimParser
	gnnInference   molpatent_gnn.GNNInferenceService
	riskRepo       RiskRecordRepository
	ftoRepo        FTOReportRepository
	eventPublisher EventPublisher
	cache          redis.Cache
	logger         logging.Logger
	metrics        *prometheus.AppMetrics
}

// RiskAssessmentServiceConfig holds all dependencies for constructing the
// risk assessment service.
type RiskAssessmentServiceConfig struct {
	MoleculeSvc    molecule.MoleculeDomainService
	PatentSvc      patent.PatentDomainService
	InfringeNet    infringe_net.InfringementAssessor
	ClaimParser    claim_bert.ClaimParser
	GNNInference   molpatent_gnn.GNNInferenceService
	RiskRepo       RiskRecordRepository
	FTORepo        FTOReportRepository
	EventPublisher EventPublisher
	Cache          redis.Cache
	Logger         logging.Logger
	Metrics        *prometheus.AppMetrics
}

// NewRiskAssessmentService constructs a new RiskAssessmentService with all
// required dependencies.
func NewRiskAssessmentService(cfg RiskAssessmentServiceConfig) (RiskAssessmentService, error) {
	if cfg.MoleculeSvc == nil {
		return nil, pkgErrors.NewValidation("config", "MoleculeSvc is required")
	}
	if cfg.PatentSvc == nil {
		return nil, pkgErrors.NewValidation("config", "PatentSvc is required")
	}
	if cfg.InfringeNet == nil {
		return nil, pkgErrors.NewValidation("config", "InfringeNet is required")
	}
	if cfg.ClaimParser == nil {
		return nil, pkgErrors.NewValidation("config", "ClaimParser is required")
	}
	if cfg.GNNInference == nil {
		return nil, pkgErrors.NewValidation("config", "GNNInference is required")
	}
	if cfg.RiskRepo == nil {
		return nil, pkgErrors.NewValidation("config", "RiskRepo is required")
	}
	if cfg.FTORepo == nil {
		return nil, pkgErrors.NewValidation("config", "FTORepo is required")
	}
	if cfg.Cache == nil {
		return nil, pkgErrors.NewValidation("config", "Cache is required")
	}
	if cfg.Logger == nil {
		return nil, pkgErrors.NewValidation("config", "Logger is required")
	}
	if cfg.Metrics == nil {
		return nil, pkgErrors.NewValidation("config", "Metrics is required")
	}

	return &riskAssessmentServiceImpl{
		moleculeSvc:    cfg.MoleculeSvc,
		patentSvc:      cfg.PatentSvc,
		infringeNet:    cfg.InfringeNet,
		claimParser:    cfg.ClaimParser,
		gnnInference:   cfg.GNNInference,
		riskRepo:       cfg.RiskRepo,
		ftoRepo:        cfg.FTORepo,
		eventPublisher: cfg.EventPublisher,
		cache:          cfg.Cache,
		logger:         cfg.Logger,
		metrics:        cfg.Metrics,
	}, nil
}

// ---------------------------------------------------------------------------
// AssessMolecule
// ---------------------------------------------------------------------------

func (s *riskAssessmentServiceImpl) AssessMolecule(ctx context.Context, req *MoleculeRiskRequest) (*MoleculeRiskResponse, error) {
	startTime := time.Now()
	s.metrics.RiskAssessmentRequestsTotal.WithLabelValues("AssessMolecule", string(req.Depth)).Inc()

	// 1. Validate and apply defaults.
	if err := req.Validate(); err != nil {
		return nil, err
	}
	req.defaults()

	// 2. Canonicalize the molecular input.
	canonicalSMILES, inchiKey, err := s.canonicalizeMolecule(ctx, req.SMILES, req.InChI)
	if err != nil {
		s.logger.Error("failed to canonicalize molecule", logging.Err(err))
		return nil, fmt.Errorf("molecule canonicalization failed: %w", err)
	}

	// 3. Check cache.
	cacheKey := req.cacheKey(canonicalSMILES)
	if cached, cacheErr := s.tryGetCachedResult(ctx, cacheKey); cacheErr == nil && cached != nil {
		cached.CacheHit = true
		cached.ProcessingTime = time.Since(startTime)
		s.metrics.RiskAssessmentCacheHitsTotal.WithLabelValues().Inc()
		s.logger.Debug("risk assessment cache hit", logging.String("smiles", canonicalSMILES))
		return cached, nil
	}

	// 4. Search for candidate patents using parallel similarity engines.
	candidates, err := s.searchCandidatePatents(ctx, canonicalSMILES, req)
	if err != nil {
		s.logger.Error("candidate patent search failed", logging.Err(err))
		return nil, fmt.Errorf("candidate patent search failed: %w", err)
	}

	// 5. If no candidates found, return NONE risk.
	if len(candidates) == 0 {
		resp := s.buildNoneRiskResponse(canonicalSMILES, inchiKey, req, startTime)
		_ = s.cacheResult(ctx, cacheKey, resp)
		_ = s.persistRecord(ctx, resp, req)
		return resp, nil
	}

	// 6. Perform claim-level infringement analysis on each candidate.
	patentDetails, err := s.analyzePatentCandidates(ctx, canonicalSMILES, candidates, req)
	if err != nil {
		s.logger.Error("patent candidate analysis failed", logging.Err(err))
		return nil, fmt.Errorf("patent analysis failed: %w", err)
	}

	// 7. Aggregate scores across all patents.
	resp := s.aggregateRiskResponse(canonicalSMILES, inchiKey, patentDetails, req, startTime)

	// 8. Cache the result.
	if cacheErr := s.cacheResult(ctx, cacheKey, resp); cacheErr != nil {
		s.logger.Warn("failed to cache risk assessment result", logging.Err(cacheErr))
	}

	// 9. Persist the immutable assessment record.
	if persistErr := s.persistRecord(ctx, resp, req); persistErr != nil {
		s.logger.Warn("failed to persist risk record", logging.Err(persistErr))
	}

	// 10. Publish risk assessment event.
	s.publishRiskEvent(ctx, resp, req)

	s.metrics.RiskAssessmentDuration.WithLabelValues(string(req.Depth)).Observe(time.Since(startTime).Seconds())

	return resp, nil
}

// ---------------------------------------------------------------------------
// AssessBatch
// ---------------------------------------------------------------------------

func (s *riskAssessmentServiceImpl) AssessBatch(ctx context.Context, req *BatchRiskRequest) (*BatchRiskResponse, error) {
	startTime := time.Now()
	s.metrics.RiskAssessmentRequestsTotal.WithLabelValues("AssessBatch", string(req.Depth)).Inc()

	if err := req.Validate(); err != nil {
		return nil, err
	}
	req.defaults()

	results := make([]BatchMoleculeResult, len(req.Molecules))
	var mu sync.Mutex

	// Semaphore for concurrency control.
	sem := make(chan struct{}, req.Concurrency)

	var wg sync.WaitGroup
	for i, mol := range req.Molecules {
		wg.Add(1)
		go func(idx int, m BatchMoleculeInput) {
			defer wg.Done()

			// Acquire semaphore slot.
			sem <- struct{}{}
			defer func() { <-sem }()

			// Check context cancellation.
			if ctx.Err() != nil {
				mu.Lock()
				results[idx] = BatchMoleculeResult{
					Index:     idx,
					ID:        m.ID,
					Name:      m.Name,
					Error:     ctx.Err().Error(),
					Succeeded: false,
				}
				mu.Unlock()
				return
			}

			// Build per-molecule request from batch common parameters.
			molReq := &MoleculeRiskRequest{
				SMILES:              m.SMILES,
				InChI:               m.InChI,
				PatentOffices:       req.PatentOffices,
				CompetitorFilter:    req.CompetitorFilter,
				SimilarityThreshold: req.SimilarityThreshold,
				Depth:               req.Depth,
				TechDomains:         req.TechDomains,
				ExcludePatents:      req.ExcludePatents,
				Trigger:             req.Trigger,
			}

			resp, err := s.AssessMolecule(ctx, molReq)

			mu.Lock()
			if err != nil {
				results[idx] = BatchMoleculeResult{
					Index:     idx,
					ID:        m.ID,
					Name:      m.Name,
					Error:     err.Error(),
					Succeeded: false,
				}
			} else {
				results[idx] = BatchMoleculeResult{
					Index:     idx,
					ID:        m.ID,
					Name:      m.Name,
					Response:  resp,
					Succeeded: true,
				}
			}
			mu.Unlock()
		}(i, mol)
	}

	wg.Wait()

	// Aggregate statistics.
	stats := s.computeBatchStats(results)

	return &BatchRiskResponse{
		Results:             results,
		Stats:               stats,
		TotalProcessingTime: time.Since(startTime),
		AssessedAt:          time.Now().UTC(),
	}, nil
}

// ---------------------------------------------------------------------------
// AssessFTO
// ---------------------------------------------------------------------------

func (s *riskAssessmentServiceImpl) AssessFTO(ctx context.Context, req *FTORequest) (*FTOResponse, error) {
	startTime := time.Now()
	s.metrics.RiskAssessmentRequestsTotal.WithLabelValues("AssessFTO", string(req.Depth)).Inc()

	if err := req.Validate(); err != nil {
		return nil, err
	}
	req.defaults()

	ftoID := commonTypes.GenerateID("fto")

	// Build the risk matrix: molecule × jurisdiction.
	riskMatrix := make([]FTORiskMatrixRow, 0, len(req.Molecules))
	allBlockingPatents := make(map[string]*BlockingPatentDetail)
	jurisdictionAgg := make(map[string]*JurisdictionFTOResult)

	// Initialize jurisdiction aggregation.
	for _, j := range req.Jurisdictions {
		jurisdictionAgg[j] = &JurisdictionFTOResult{
			Jurisdiction: j,
		}
	}

	// For each molecule, assess against each jurisdiction.
	for _, mol := range req.Molecules {
		row := FTORiskMatrixRow{
			MoleculeID:    mol.ID,
			MoleculeName:  mol.Name,
			SMILES:        mol.SMILES,
			Jurisdictions: make(map[string]RiskLevel),
		}

		for _, jurisdiction := range req.Jurisdictions {
			// Build a per-molecule, per-jurisdiction request.
			molReq := &MoleculeRiskRequest{
				SMILES:              mol.SMILES,
				InChI:               mol.InChI,
				PatentOffices:       []string{jurisdiction},
				SimilarityThreshold: req.SimilarityThreshold,
				Depth:               req.Depth,
				ExcludePatents:      req.ExcludePatents,
				Trigger:             req.Trigger,
			}

			resp, err := s.AssessMolecule(ctx, molReq)
			if err != nil {
				s.logger.Warn("FTO molecule assessment failed",
					logging.String("molecule", mol.SMILES),
					logging.String("jurisdiction", jurisdiction),
					logging.Err(err))
				row.Jurisdictions[jurisdiction] = RiskLevelNone
				continue
			}

			row.Jurisdictions[jurisdiction] = resp.OverallRiskLevel

			// Update jurisdiction aggregation.
			jagg := jurisdictionAgg[jurisdiction]
			jagg.PatentsChecked += resp.CandidatesSearched
			switch resp.OverallRiskLevel {
			case RiskLevelCritical:
				jagg.CriticalCount++
			case RiskLevelHigh:
				jagg.HighCount++
			case RiskLevelMedium:
				jagg.MediumCount++
			}

			// Collect blocking patents.
			for _, mp := range resp.MatchedPatents {
				if mp.PatentRiskLevel == RiskLevelCritical || mp.PatentRiskLevel == RiskLevelHigh {
					key := mp.PatentNumber
					if existing, ok := allBlockingPatents[key]; ok {
						existing.Jurisdictions = appendUnique(existing.Jurisdictions, jurisdiction)
						if mp.PatentRiskLevel.Severity() > existing.RiskLevel.Severity() {
							existing.RiskLevel = mp.PatentRiskLevel
							existing.RiskScore = mp.PatentRiskScore
						}
					} else {
						allBlockingPatents[key] = &BlockingPatentDetail{
							PatentNumber:  mp.PatentNumber,
							Title:         mp.Title,
							Assignee:      mp.Assignee,
							Jurisdictions: []string{jurisdiction},
							RiskLevel:     mp.PatentRiskLevel,
							RiskScore:     mp.PatentRiskScore,
							ExpirationDate: mp.FilingDate.AddDate(20, 0, 0),
						}
					}
				}
			}
		}

		riskMatrix = append(riskMatrix, row)
	}

	// Determine per-jurisdiction conclusions.
	jurisdictionResults := make([]JurisdictionFTOResult, 0, len(req.Jurisdictions))
	for _, j := range req.Jurisdictions {
		jagg := jurisdictionAgg[j]
		jagg.Conclusion = determineFTOConclusion(jagg.CriticalCount, jagg.HighCount)
		jagg.Summary = formatJurisdictionSummary(jagg)
		jurisdictionResults = append(jurisdictionResults, *jagg)
	}

	// Flatten blocking patents.
	blockingList := make([]BlockingPatentDetail, 0, len(allBlockingPatents))
	for _, bp := range allBlockingPatents {
		blockingList = append(blockingList, *bp)
	}
	sort.Slice(blockingList, func(i, j int) bool {
		return blockingList[i].RiskScore > blockingList[j].RiskScore
	})

	// Generate recommended actions.
	actions := s.generateFTOActions(blockingList, jurisdictionResults)

	// Determine overall conclusion.
	overallConclusion := FTOFree
	for _, jr := range jurisdictionResults {
		if jr.Conclusion == FTOBlocked {
			overallConclusion = FTOBlocked
			break
		}
		if jr.Conclusion == FTOConditional && overallConclusion != FTOBlocked {
			overallConclusion = FTOConditional
		}
	}

	resp := &FTOResponse{
		FTOID:               ftoID,
		JurisdictionResults: jurisdictionResults,
		BlockingPatents:     blockingList,
		RiskMatrix:          riskMatrix,
		RecommendedActions:  actions,
		OverallConclusion:   overallConclusion,
		TotalProcessingTime: time.Since(startTime),
		AssessedAt:          time.Now().UTC(),
	}

	// Persist FTO report.
	if err := s.ftoRepo.SaveFTOReport(ctx, resp); err != nil {
		s.logger.Warn("failed to persist FTO report", logging.String("fto_id", ftoID), logging.Err(err))
	}

	s.metrics.FTOAnalysisDuration.WithLabelValues(fmt.Sprintf("%d", len(req.Jurisdictions))).Observe(time.Since(startTime).Seconds())

	return resp, nil
}

// ---------------------------------------------------------------------------
// GetRiskSummary
// ---------------------------------------------------------------------------

func (s *riskAssessmentServiceImpl) GetRiskSummary(ctx context.Context, portfolioID string) (*RiskSummaryResponse, error) {
	s.metrics.RiskAssessmentRequestsTotal.WithLabelValues("GetRiskSummary", "none").Inc()

	if portfolioID == "" {
		return nil, pkgErrors.NewValidation("portfolio_id", "must not be empty")
	}

	// 1. Retrieve latest risk records for all molecules in the portfolio.
	records, err := s.riskRepo.FindByPortfolio(ctx, portfolioID)
	if err != nil {
		s.logger.Error("failed to retrieve portfolio risk records",
			logging.String("portfolio_id", portfolioID), logging.Err(err))
		return nil, fmt.Errorf("portfolio risk records retrieval failed: %w", err)
	}

	if len(records) == 0 {
		return &RiskSummaryResponse{
			PortfolioID:      portfolioID,
			OverallRiskScore: 0,
			OverallRiskLevel: RiskLevelNone,
			TotalMolecules:   0,
			AssessedCount:    0,
			RiskDistribution: map[RiskLevel]int{
				RiskLevelCritical: 0,
				RiskLevelHigh:     0,
				RiskLevelMedium:   0,
				RiskLevelLow:      0,
				RiskLevelNone:     0,
			},
			TechDomainRisks:     []TechDomainRisk{},
			HighRiskPatentsTopN: []PatentRiskDetail{},
			Trend:               []RiskTrendPoint{},
			GeneratedAt:         time.Now().UTC(),
		}, nil
	}

	// 2. Compute risk distribution.
	distribution := map[RiskLevel]int{
		RiskLevelCritical: 0,
		RiskLevelHigh:     0,
		RiskLevelMedium:   0,
		RiskLevelLow:      0,
		RiskLevelNone:     0,
	}

	var totalScore float64
	var maxScore float64

	// techDomainMap aggregates scores by technology domain extracted from
	// the stored result JSON.
	techDomainMap := make(map[string]*techDomainAccumulator)

	// highRiskPatents collects patent details from CRITICAL/HIGH records.
	var highRiskPatents []PatentRiskDetail

	for _, rec := range records {
		distribution[rec.RiskLevel]++
		totalScore += rec.RiskScore
		if rec.RiskScore > maxScore {
			maxScore = rec.RiskScore
		}

		// Attempt to extract patent details from the stored result JSON
		// for high-risk records.
		if rec.RiskLevel == RiskLevelCritical || rec.RiskLevel == RiskLevelHigh {
			if rec.ResultJSON != "" {
				var storedResp MoleculeRiskResponse
				if jsonErr := json.Unmarshal([]byte(rec.ResultJSON), &storedResp); jsonErr == nil {
					for _, mp := range storedResp.MatchedPatents {
						if mp.PatentRiskLevel == RiskLevelCritical || mp.PatentRiskLevel == RiskLevelHigh {
							highRiskPatents = append(highRiskPatents, mp)
						}
					}
					// Aggregate by IPC-based tech domain.
					for _, mp := range storedResp.MatchedPatents {
						for _, ipc := range mp.IPCCodes {
							domain := ipcToDomain(ipc)
							acc, ok := techDomainMap[domain]
							if !ok {
								acc = &techDomainAccumulator{}
								techDomainMap[domain] = acc
							}
							acc.count++
							acc.totalScore += mp.PatentRiskScore
							if mp.PatentRiskScore > acc.maxScore {
								acc.maxScore = mp.PatentRiskScore
							}
						}
					}
				}
			}
		}
	}

	avgScore := totalScore / float64(len(records))

	// 3. Sort and limit high-risk patents to top 10.
	sort.Slice(highRiskPatents, func(i, j int) bool {
		return highRiskPatents[i].PatentRiskScore > highRiskPatents[j].PatentRiskScore
	})
	topN := 10
	if len(highRiskPatents) > topN {
		highRiskPatents = highRiskPatents[:topN]
	}

	// 4. Build tech domain risks.
	techDomainRisks := make([]TechDomainRisk, 0, len(techDomainMap))
	for domain, acc := range techDomainMap {
		domainAvg := acc.totalScore / float64(acc.count)
		techDomainRisks = append(techDomainRisks, TechDomainRisk{
			Domain:        domain,
			MoleculeCount: acc.count,
			AverageScore:  domainAvg,
			MaxScore:      acc.maxScore,
			RiskLevel:     RiskLevelFromScore(domainAvg),
		})
	}
	sort.Slice(techDomainRisks, func(i, j int) bool {
		return techDomainRisks[i].AverageScore > techDomainRisks[j].AverageScore
	})

	// 5. Retrieve trend data (last 6 months).
	trend, trendErr := s.riskRepo.GetTrend(ctx, portfolioID, 6)
	if trendErr != nil {
		s.logger.Warn("failed to retrieve risk trend", logging.String("portfolio_id", portfolioID), logging.Err(trendErr))
	}

	// Handle Trend conversion or nil
	var trendValues []RiskTrendPoint
	if trend != nil {
		trendValues = make([]RiskTrendPoint, len(trend))
		for i, t := range trend {
			if t != nil {
				trendValues[i] = *t
			}
		}
	} else {
		trendValues = []RiskTrendPoint{}
	}

	return &RiskSummaryResponse{
		PortfolioID:         portfolioID,
		OverallRiskScore:    avgScore,
		OverallRiskLevel:    RiskLevelFromScore(avgScore),
		TotalMolecules:      len(records),
		AssessedCount:       len(records),
		RiskDistribution:    distribution,
		TechDomainRisks:     techDomainRisks,
		HighRiskPatentsTopN: highRiskPatents,
		Trend:               trendValues,
		GeneratedAt:         time.Now().UTC(),
	}, nil
}

// techDomainAccumulator is an internal helper for aggregating tech domain
// risk statistics.
type techDomainAccumulator struct {
	count      int
	totalScore float64
	maxScore   float64
}

// ---------------------------------------------------------------------------
// GetRiskHistory
// ---------------------------------------------------------------------------

func (s *riskAssessmentServiceImpl) GetRiskHistory(ctx context.Context, moleculeID string, opts ...QueryOption) ([]*RiskRecord, error) {
	s.metrics.RiskAssessmentRequestsTotal.WithLabelValues("GetRiskHistory", "none").Inc()

	if moleculeID == "" {
		return nil, pkgErrors.NewValidation("molecule_id", "must not be empty")
	}

	qo := applyQueryOptions(opts)

	records, _, err := s.riskRepo.FindByMolecule(ctx, moleculeID, qo)
	if err != nil {
		s.logger.Error("failed to retrieve risk history",
			logging.String("molecule_id", moleculeID), logging.Err(err))
		return nil, fmt.Errorf("risk history retrieval failed: %w", err)
	}

	return records, nil
}

// ---------------------------------------------------------------------------
// Internal: Molecule Canonicalization
// ---------------------------------------------------------------------------

// canonicalizeMolecule converts the input SMILES or InChI to a canonical
// SMILES and InChIKey using the molecule domain service.
func (s *riskAssessmentServiceImpl) canonicalizeMolecule(ctx context.Context, smiles, inchi string) (string, string, error) {
	if smiles != "" {
		canonical, inchiKey, err := s.moleculeSvc.Canonicalize(ctx, smiles)
		if err != nil {
			return "", "", fmt.Errorf("SMILES canonicalization failed: %w", err)
		}
		return canonical, inchiKey, nil
	}

	// Convert InChI to canonical SMILES.
	canonical, inchiKey, err := s.moleculeSvc.CanonicalizeFromInChI(ctx, inchi)
	if err != nil {
		return "", "", fmt.Errorf("InChI canonicalization failed: %w", err)
	}
	return canonical, inchiKey, nil
}

// ---------------------------------------------------------------------------
// Internal: Cache Operations
// ---------------------------------------------------------------------------

func (s *riskAssessmentServiceImpl) tryGetCachedResult(ctx context.Context, key string) (*MoleculeRiskResponse, error) {
	var resp MoleculeRiskResponse
	err := s.cache.Get(ctx, key, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

func (s *riskAssessmentServiceImpl) cacheResult(ctx context.Context, key string, resp *MoleculeRiskResponse) error {
	return s.cache.Set(ctx, key, resp, RiskCacheTTL)
}

// ---------------------------------------------------------------------------
// Internal: Candidate Patent Search
// ---------------------------------------------------------------------------

// candidatePatent is an internal representation of a patent returned from
// similarity search, before full claim-level analysis.
type candidatePatent struct {
	PatentNumber string
	Title        string
	Assignee     string
	FilingDate   time.Time
	LegalStatus  string
	IPCCodes     []string
	Similarity   SimilarityScores
}

// searchCandidatePatents runs parallel similarity searches using both
// fingerprint-based and GNN-based engines, then merges and deduplicates
// the results.
func (s *riskAssessmentServiceImpl) searchCandidatePatents(
	ctx context.Context,
	canonicalSMILES string,
	req *MoleculeRiskRequest,
) ([]candidatePatent, error) {

	type searchResult struct {
		candidates []candidatePatent
		err        error
	}

	fpCh := make(chan searchResult, 1)
	gnnCh := make(chan searchResult, 1)

	// Fingerprint-based similarity search (Morgan + RDKit + AtomPair).
	go func() {
		defer func() {
			if r := recover(); r != nil {
				fpCh <- searchResult{err: fmt.Errorf("fingerprint search panic: %v", r)}
			}
		}()

		results, err := s.patentSvc.SearchBySimilarity(ctx, &patent.SimilaritySearchRequest{
			SMILES:          canonicalSMILES,
			Threshold:       req.SimilarityThreshold,
			MaxResults:      req.MaxCandidates,
			PatentOffices:   req.PatentOffices,
			Assignees:       req.CompetitorFilter,
			TechDomains:     req.TechDomains,
			DateFrom:        req.DateFrom,
			DateTo:          req.DateTo,
			ExcludePatents:  req.ExcludePatents,
		})
		if err != nil {
			fpCh <- searchResult{err: fmt.Errorf("fingerprint similarity search failed: %w", err)}
			return
		}

		candidates := make([]candidatePatent, 0, len(results))
		for _, r := range results {
			candidates = append(candidates, candidatePatent{
				PatentNumber: r.PatentNumber,
				Title:        r.Title,
				Assignee:     r.Assignee,
				FilingDate:   r.FilingDate,
				LegalStatus:  r.LegalStatus,
				IPCCodes:     r.IPCCodes,
				Similarity: SimilarityScores{
					Morgan:   r.MorganSimilarity,
					RDKit:    r.RDKitSimilarity,
					AtomPair: r.AtomPairSimilarity,
				},
			})
		}
		fpCh <- searchResult{candidates: candidates}
	}()

	// GNN embedding-based similarity search.
	go func() {
		defer func() {
			if r := recover(); r != nil {
				gnnCh <- searchResult{err: fmt.Errorf("GNN search panic: %v", r)}
			}
		}()

		// Skip GNN search for quick depth.
		if req.Depth == AnalysisDepthQuick {
			gnnCh <- searchResult{candidates: nil}
			return
		}

		resp, err := s.gnnInference.SearchSimilar(ctx, &molpatent_gnn.SimilarSearchRequest{
			SMILES:    canonicalSMILES,
			TopK:      req.MaxCandidates,
			Threshold: req.SimilarityThreshold,
		})
		if err != nil {
			// GNN failure is non-fatal; log and continue with fingerprint results.
			s.logger.Warn("GNN similarity search failed, continuing with fingerprint results",
				logging.Err(err))
			gnnCh <- searchResult{candidates: nil}
			return
		}

		candidates := make([]candidatePatent, 0, len(resp.Matches))
		for _, m := range resp.Matches {
			// Lookup patents for this molecule ID
			patents, err := s.patentSvc.GetPatentsByMoleculeID(ctx, m.MoleculeID)
			if err != nil {
				continue
			}

			for _, p := range patents {
				filingDate := time.Time{}
				if p.Dates.FilingDate != nil {
					filingDate = *p.Dates.FilingDate
				}

				var ipcCodes []string
				for _, ipc := range p.IPCCodes {
					ipcCodes = append(ipcCodes, ipc.Full)
				}

				candidates = append(candidates, candidatePatent{
					PatentNumber: p.PatentNumber,
					Title:        p.Title,
					Assignee:     p.AssigneeName,
					FilingDate:   filingDate,
					LegalStatus:  string(p.Status),
					IPCCodes:     ipcCodes,
					Similarity: SimilarityScores{
						GNN: m.Score,
					},
				})
			}
		}
		gnnCh <- searchResult{candidates: candidates}
	}()

	// Collect results from both channels.
	fpResult := <-fpCh
	gnnResult := <-gnnCh

	// If fingerprint search failed, that's a hard error.
	if fpResult.err != nil {
		return nil, fpResult.err
	}

	// Merge and deduplicate.
	merged := s.mergeCandidates(fpResult.candidates, gnnResult.candidates)

	// Compute fused scores and filter by threshold.
	filtered := make([]candidatePatent, 0, len(merged))
	for i := range merged {
		fused := merged[i].Similarity.FusedScore()
		if fused >= req.SimilarityThreshold {
			filtered = append(filtered, merged[i])
		}
	}

	// Sort by fused score descending.
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Similarity.WeightedOverall > filtered[j].Similarity.WeightedOverall
	})

	// Limit to MaxCandidates.
	if len(filtered) > req.MaxCandidates {
		filtered = filtered[:req.MaxCandidates]
	}

	return filtered, nil
}

// mergeCandidates combines fingerprint and GNN search results, merging
// similarity scores for patents that appear in both result sets.
func (s *riskAssessmentServiceImpl) mergeCandidates(fp, gnn []candidatePatent) []candidatePatent {
	index := make(map[string]int, len(fp))
	merged := make([]candidatePatent, len(fp))
	copy(merged, fp)

	for i, c := range merged {
		index[c.PatentNumber] = i
	}

	for _, g := range gnn {
		if idx, ok := index[g.PatentNumber]; ok {
			// Merge GNN score into existing entry.
			merged[idx].Similarity.GNN = g.Similarity.GNN
		} else {
			// New patent from GNN search.
			merged = append(merged, g)
		}
	}

	return merged
}

// ---------------------------------------------------------------------------
// Internal: Patent Candidate Analysis
// ---------------------------------------------------------------------------

// analyzePatentCandidates performs claim-level infringement analysis on
// each candidate patent using ClaimBERT and InfringeNet.
func (s *riskAssessmentServiceImpl) analyzePatentCandidates(
	ctx context.Context,
	canonicalSMILES string,
	candidates []candidatePatent,
	req *MoleculeRiskRequest,
) ([]PatentRiskDetail, error) {

	details := make([]PatentRiskDetail, 0, len(candidates))

	for _, cand := range candidates {
		detail := PatentRiskDetail{
			PatentNumber:     cand.PatentNumber,
			Title:            cand.Title,
			Assignee:         cand.Assignee,
			FilingDate:       cand.FilingDate,
			LegalStatus:      cand.LegalStatus,
			IPCCodes:         cand.IPCCodes,
			SimilarityScores: cand.Similarity,
		}

		// For quick depth, skip claim-level analysis.
		if req.Depth == AnalysisDepthQuick {
			detail.PatentRiskScore = cand.Similarity.WeightedOverall * 100
			detail.PatentRiskLevel = RiskLevelFromScore(detail.PatentRiskScore)
			details = append(details, detail)
			continue
		}

		// Fetch patent to get claim texts.
		pat, err := s.patentSvc.GetPatentByNumber(ctx, cand.PatentNumber)
		if err != nil {
			s.logger.Warn("failed to fetch patent details", logging.String("patent", cand.PatentNumber), logging.Err(err))
			continue
		}

		var claimTexts []string
		for _, c := range pat.Claims {
			claimTexts = append(claimTexts, c.Text)
		}

		// Parse claims using ClaimBERT.
		parsedClaimsSet, parseErr := s.claimParser.ParseClaimSet(ctx, claimTexts)
		if parseErr != nil {
			s.logger.Warn("ClaimBERT parse failed for patent",
				logging.String("patent", cand.PatentNumber), logging.Err(parseErr))
			// Fall back to similarity-only scoring.
			detail.PatentRiskScore = cand.Similarity.WeightedOverall * 100
			detail.PatentRiskLevel = RiskLevelFromScore(detail.PatentRiskScore)
			details = append(details, detail)
			continue
		}

		// Analyze each claim with InfringeNet.
		claimDetails := make([]ClaimRiskDetail, 0, len(parsedClaimsSet.Claims))
		var maxClaimScore float64

		for _, claim := range parsedClaimsSet.Claims {
			claimDetail := ClaimRiskDetail{
				ClaimNumber: claim.ClaimNumber,
				ClaimType:   string(claim.ClaimType),
			}

			// Extract element texts from features.
			var elements []string
			for _, f := range claim.Features {
				elements = append(elements, f.Text)
			}

			if req.Depth == AnalysisDepthDeep {
				// Construct correct ClaimInput slice for assessment
				claimInputs := []*infringe_net.ClaimInput{
					{
						ClaimID:   fmt.Sprintf("%s-C%d", cand.PatentNumber, claim.ClaimNumber),
						ClaimText: claim.Body,
						ClaimType: infringe_net.ClaimType(claim.ClaimType), // Casting assuming string compatibility
						PatentID:  cand.PatentNumber,
					},
				}

				assessment, assessErr := s.infringeNet.Assess(ctx, &infringe_net.AssessmentRequest{
					Molecule: &infringe_net.MoleculeInput{
						SMILES: canonicalSMILES,
					},
					Claims: claimInputs,
				})
				if assessErr != nil {
					s.logger.Warn("InfringeNet assessment failed",
						logging.String("patent", cand.PatentNumber),
						logging.Int("claim", claim.ClaimNumber),
						logging.Err(assessErr))
					continue
				}

				// Use assessment result (assuming it returns info for the first claim)
				if len(assessment.MatchedClaims) > 0 {
					mc := assessment.MatchedClaims[0]
					claimDetail.LiteralMatch = (mc.LiteralScore >= 0.9)
					claimDetail.EquivalentsMatch = (mc.EquivalentsScore >= 0.5)

					// Compute claim-level risk score using the returned overall score (already computed by InfringeNet).
					claimScore := assessment.OverallScore * 100
					claimDetail.ClaimRiskScore = claimScore
					claimDetail.ClaimRiskLevel = RiskLevelFromScore(claimScore)
					claimDetail.Explanation = "InfringeNet analysis complete"
				}
			} else {
				// Standard depth: semantic matching without full element analysis.
				score, matchErr := s.claimParser.SemanticMatch(ctx, canonicalSMILES, claim.Body)
				if matchErr != nil {
					s.logger.Warn("ClaimBERT semantic match failed",
						logging.String("patent", cand.PatentNumber),
						logging.Int("claim", claim.ClaimNumber),
						logging.Err(matchErr))
					continue
				}

				claimDetail.ClaimRiskScore = score * 100
				claimDetail.ClaimRiskLevel = RiskLevelFromScore(claimDetail.ClaimRiskScore)
				// Basic explanation for standard depth
				if score > 0.7 {
					claimDetail.Explanation = "High semantic similarity detected"
				} else {
					claimDetail.Explanation = "Low semantic similarity"
				}
			}

			if claimDetail.ClaimRiskScore > maxClaimScore {
				maxClaimScore = claimDetail.ClaimRiskScore
			}

			claimDetails = append(claimDetails, claimDetail)
		}

		detail.RelevantClaims = claimDetails

		// Patent-level risk is driven by the highest-risk claim.
		if maxClaimScore > 0 {
			detail.PatentRiskScore = maxClaimScore
		} else {
			detail.PatentRiskScore = cand.Similarity.WeightedOverall * 100
		}
		detail.PatentRiskLevel = RiskLevelFromScore(detail.PatentRiskScore)

		details = append(details, detail)
	}

	// Sort by patent risk score descending.
	sort.Slice(details, func(i, j int) bool {
		return details[i].PatentRiskScore > details[j].PatentRiskScore
	})

	return details, nil
}

// computeClaimRiskScore applies the weighted scoring formula for a single
// claim based on InfringeNet assessment results.
//
// Formula: 0.35*literalScore + 0.30*equivalentsScore + 0.20*breadthScore + 0.15*penalty
//
// Where:
//   - literalScore: 100 if literal match or Markush covered, else 0
//   - equivalentsScore: 100 if equivalents match, else 0
//   - breadthScore: similarity * 100 (proxy for claim breadth coverage)
//   - penalty: -20 if estoppel applies (reduces equivalents contribution)
func computeClaimRiskScore(assessment *infringe_net.AssessmentResult, similarity float64) float64 {
	// This function might be redundant if using InfringeNet's score, but keeping for logic preservation if manual calculation is desired.
	var literalScore float64
	if assessment.LiteralAnalysis != nil && assessment.LiteralAnalysis.AllElementsMet {
		literalScore = 100.0
	}

	var equivalentsScore float64
	if assessment.EquivalentsAnalysis != nil && !assessment.EquivalentsAnalysis.Skipped {
		equivalentsScore = assessment.EquivalentsAnalysis.Score * 100
	}

	breadthScore := similarity * 100.0

	var penalty float64
	if assessment.EstoppelCheck != nil && assessment.EstoppelCheck.HasEstoppel {
		penalty = 20.0
	}

	score := 0.35*literalScore + 0.30*equivalentsScore + 0.20*breadthScore - 0.15*penalty

	// Clamp to [0, 100].
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}

	return score
}

// ---------------------------------------------------------------------------
// Internal: Risk Aggregation
// ---------------------------------------------------------------------------

// aggregateRiskResponse builds the final MoleculeRiskResponse from
// analyzed patent details.
func (s *riskAssessmentServiceImpl) aggregateRiskResponse(
	canonicalSMILES string,
	inchiKey string,
	patentDetails []PatentRiskDetail,
	req *MoleculeRiskRequest,
	startTime time.Time,
) *MoleculeRiskResponse {

	assessmentID := commonTypes.GenerateID("ra")

	// The overall risk is driven by the highest-risk patent.
	var overallScore float64
	var literalMax, equivalentsMax, breadthMax, penaltyMax float64

	for _, pd := range patentDetails {
		if pd.PatentRiskScore > overallScore {
			overallScore = pd.PatentRiskScore
		}
		for _, cd := range pd.RelevantClaims {
			if cd.LiteralMatch || cd.MarkushCovered {
				if 100.0 > literalMax {
					literalMax = 100.0
				}
			}
			if cd.EquivalentsMatch {
				if 100.0 > equivalentsMax {
					equivalentsMax = 100.0
				}
			}
			if cd.EstoppelApplies {
				penaltyMax = 20.0
			}
		}
		if pd.SimilarityScores.WeightedOverall*100 > breadthMax {
			breadthMax = pd.SimilarityScores.WeightedOverall * 100
		}
	}

	// Recompute overall using the formula for transparency.
	formulaScore := 0.35*literalMax + 0.30*equivalentsMax + 0.20*breadthMax - 0.15*penaltyMax
	if formulaScore < 0 {
		formulaScore = 0
	}
	if formulaScore > 100 {
		formulaScore = 100
	}

	// Use the higher of formula-based and max-patent scores.
	if formulaScore > overallScore {
		overallScore = formulaScore
	}

	summary := generateRiskSummary(overallScore, len(patentDetails), req.Depth)

	return &MoleculeRiskResponse{
		AssessmentID:                 assessmentID,
		CanonicalSMILES:             canonicalSMILES,
		InChIKey:                    inchiKey,
		OverallRiskLevel:            RiskLevelFromScore(overallScore),
		OverallRiskScore:            overallScore,
		LiteralInfringementScore:    literalMax,
		EquivalentsInfringementScore: equivalentsMax,
		ClaimBreadthScore:           breadthMax,
		ProsecutionHistoryPenalty:   penaltyMax,
		MatchedPatents:              patentDetails,
		Summary:                     summary,
		CandidatesSearched:          len(patentDetails),
		AnalysisDepth:               req.Depth,
		ProcessingTime:              time.Since(startTime),
		CacheHit:                    false,
		AssessedAt:                  time.Now().UTC(),
	}
}

// buildNoneRiskResponse creates a response for the case where no candidate
// patents were found.
func (s *riskAssessmentServiceImpl) buildNoneRiskResponse(
	canonicalSMILES, inchiKey string,
	req *MoleculeRiskRequest,
	startTime time.Time,
) *MoleculeRiskResponse {
	return &MoleculeRiskResponse{
		AssessmentID:     commonTypes.GenerateID("ra"),
		CanonicalSMILES:  canonicalSMILES,
		InChIKey:         inchiKey,
		OverallRiskLevel: RiskLevelNone,
		OverallRiskScore: 0,
		MatchedPatents:   []PatentRiskDetail{},
		Summary:          "No candidate patents found matching the specified criteria. The molecule appears to have no infringement risk based on the current patent corpus.",
		CandidatesSearched: 0,
		AnalysisDepth:      req.Depth,
		ProcessingTime:     time.Since(startTime),
		CacheHit:           false,
		AssessedAt:         time.Now().UTC(),
	}
}

// ---------------------------------------------------------------------------
// Internal: Persistence
// ---------------------------------------------------------------------------

func (s *riskAssessmentServiceImpl) persistRecord(ctx context.Context, resp *MoleculeRiskResponse, req *MoleculeRiskRequest) error {
	resultJSON, err := json.Marshal(resp)
	if err != nil {
		return fmt.Errorf("failed to marshal result for persistence: %w", err)
	}

	record := &RiskRecord{
		RecordID:   resp.AssessmentID,
		SMILES:     resp.CanonicalSMILES,
		InChIKey:   resp.InChIKey,
		Trigger:    req.Trigger,
		RiskLevel:  resp.OverallRiskLevel,
		RiskScore:  resp.OverallRiskScore,
		MatchCount: len(resp.MatchedPatents),
		Depth:      req.Depth,
		InputHash:  req.cacheKey(resp.CanonicalSMILES),
		ResultJSON: string(resultJSON),
		CreatedAt:  resp.AssessedAt,
	}

	return s.riskRepo.Save(ctx, record)
}

// ---------------------------------------------------------------------------
// Internal: Event Publishing
// ---------------------------------------------------------------------------

func (s *riskAssessmentServiceImpl) publishRiskEvent(ctx context.Context, resp *MoleculeRiskResponse, req *MoleculeRiskRequest) {
	if s.eventPublisher == nil {
		return
	}

	event := &RiskAssessmentEvent{
		EventType:    "risk.assessed",
		AssessmentID: resp.AssessmentID,
		SMILES:       resp.CanonicalSMILES,
		RiskLevel:    resp.OverallRiskLevel,
		RiskScore:    resp.OverallRiskScore,
		Trigger:      req.Trigger,
		Timestamp:    resp.AssessedAt,
	}

	if err := s.eventPublisher.Publish(ctx, event); err != nil {
		s.logger.Warn("failed to publish risk assessment event",
			logging.String("assessment_id", resp.AssessmentID), logging.Err(err))
	}
}

// ---------------------------------------------------------------------------
// Internal: Batch Statistics
// ---------------------------------------------------------------------------

func (s *riskAssessmentServiceImpl) computeBatchStats(results []BatchMoleculeResult) BatchRiskStats {
	stats := BatchRiskStats{
		Total:            len(results),
		RiskDistribution: make(map[RiskLevel]int),
		MinScore:         100.0,
	}

	var scoreSum float64
	var scoreCount int

	for _, r := range results {
		if r.Succeeded {
			stats.Succeeded++
			if r.Response != nil {
				if r.Response.CacheHit {
					stats.CacheHits++
				}
				score := r.Response.OverallRiskScore
				level := r.Response.OverallRiskLevel

				stats.RiskDistribution[level]++
				scoreSum += score
				scoreCount++

				if score > stats.MaxScore {
					stats.MaxScore = score
				}
				if score < stats.MinScore {
					stats.MinScore = score
				}

				if level == RiskLevelCritical || level == RiskLevelHigh {
					stats.HighRiskCount++
				}
			}
		} else {
			stats.Failed++
		}
	}

	if scoreCount > 0 {
		stats.AverageScore = scoreSum / float64(scoreCount)
	} else {
		stats.MinScore = 0
	}

	return stats
}

// ---------------------------------------------------------------------------
// Internal: FTO Helpers
// ---------------------------------------------------------------------------

// determineFTOConclusion maps critical/high counts to an FTO conclusion.
// BLOCKED if any CRITICAL, CONDITIONAL if any HIGH, FREE otherwise.
func determineFTOConclusion(criticalCount, highCount int) FTOConclusion {
	if criticalCount > 0 {
		return FTOBlocked
	}
	if highCount > 0 {
		return FTOConditional
	}
	return FTOFree
}

// formatJurisdictionSummary generates a human-readable summary for a
// jurisdiction FTO result.
func formatJurisdictionSummary(jr *JurisdictionFTOResult) string {
	switch jr.Conclusion {
	case FTOBlocked:
		return fmt.Sprintf(
			"FTO is BLOCKED in %s: %d critical and %d high-risk patents identified out of %d checked. "+
				"Immediate legal review and design-around strategies are recommended.",
			jr.Jurisdiction, jr.CriticalCount, jr.HighCount, jr.PatentsChecked)
	case FTOConditional:
		return fmt.Sprintf(
			"FTO is CONDITIONAL in %s: %d high-risk and %d medium-risk patents identified out of %d checked. "+
				"Proceed with caution; detailed legal analysis is recommended before commercialization.",
			jr.Jurisdiction, jr.HighCount, jr.MediumCount, jr.PatentsChecked)
	default:
		return fmt.Sprintf(
			"FTO is FREE in %s: no critical or high-risk patents identified among %d checked. "+
				"Periodic monitoring is recommended to detect newly published patents.",
			jr.Jurisdiction, jr.PatentsChecked)
	}
}

// generateFTOActions produces recommended actions based on blocking patents
// and jurisdiction results.
func (s *riskAssessmentServiceImpl) generateFTOActions(
	blockingPatents []BlockingPatentDetail,
	jurisdictionResults []JurisdictionFTOResult,
) []FTOAction {
	actions := make([]FTOAction, 0)

	// Immediate actions for critical blocking patents.
	for _, bp := range blockingPatents {
		if bp.RiskLevel == RiskLevelCritical {
			actions = append(actions, FTOAction{
				Priority:    "immediate",
				Category:    "design_around",
				Description: fmt.Sprintf("Critical blocking patent %s (assignee: %s) requires immediate design-around analysis. Consider structural modifications to avoid coverage of key claims.", bp.PatentNumber, bp.Assignee),
				PatentRef:   bp.PatentNumber,
			})

			// Check if patent is near expiration (within 3 years).
			if !bp.ExpirationDate.IsZero() && time.Until(bp.ExpirationDate) < 3*365*24*time.Hour {
				actions = append(actions, FTOAction{
					Priority:    "short_term",
					Category:    "monitor",
					Description: fmt.Sprintf("Patent %s expires on %s. Consider delaying market entry or negotiating a short-term license.", bp.PatentNumber, bp.ExpirationDate.Format("2006-01-02")),
					PatentRef:   bp.PatentNumber,
				})
			} else {
				actions = append(actions, FTOAction{
					Priority:    "short_term",
					Category:    "license",
					Description: fmt.Sprintf("Evaluate licensing options for patent %s from %s. Assess commercial terms and cross-licensing opportunities.", bp.PatentNumber, bp.Assignee),
					PatentRef:   bp.PatentNumber,
				})
			}
		}
	}

	// Short-term actions for high-risk patents.
	for _, bp := range blockingPatents {
		if bp.RiskLevel == RiskLevelHigh {
			actions = append(actions, FTOAction{
				Priority:    "short_term",
				Category:    "challenge",
				Description: fmt.Sprintf("High-risk patent %s may be challengeable. Evaluate prior art for potential invalidity arguments or inter partes review.", bp.PatentNumber),
				PatentRef:   bp.PatentNumber,
			})
		}
	}

	// Long-term monitoring for all blocked/conditional jurisdictions.
	for _, jr := range jurisdictionResults {
		if jr.Conclusion == FTOBlocked || jr.Conclusion == FTOConditional {
			actions = append(actions, FTOAction{
				Priority:    "long_term",
				Category:    "monitor",
				Description: fmt.Sprintf("Establish ongoing patent monitoring for jurisdiction %s. Track new filings, status changes, and potential expirations of identified risk patents.", jr.Jurisdiction),
			})
		}
	}

	return actions
}

// ---------------------------------------------------------------------------
// Internal: Utility Helpers
// ---------------------------------------------------------------------------

// generateRiskSummary produces a human-readable summary of the risk
// assessment outcome.
func generateRiskSummary(score float64, matchCount int, depth AnalysisDepth) string {
	level := RiskLevelFromScore(score)

	switch level {
	case RiskLevelCritical:
		return fmt.Sprintf(
			"CRITICAL infringement risk detected (score: %.1f/100). "+
				"%d patent(s) identified with high-confidence claim coverage. "+
				"Immediate legal review is strongly recommended. "+
				"Analysis depth: %s.",
			score, matchCount, depth)
	case RiskLevelHigh:
		return fmt.Sprintf(
			"HIGH infringement risk detected (score: %.1f/100). "+
				"%d patent(s) identified with significant claim overlap. "+
				"Detailed legal analysis and design-around evaluation recommended. "+
				"Analysis depth: %s.",
			score, matchCount, depth)
	case RiskLevelMedium:
		return fmt.Sprintf(
			"MEDIUM infringement risk detected (score: %.1f/100). "+
				"%d patent(s) identified with partial claim overlap. "+
				"Further investigation recommended to clarify scope of coverage. "+
				"Analysis depth: %s.",
			score, matchCount, depth)
	case RiskLevelLow:
		return fmt.Sprintf(
			"LOW infringement risk detected (score: %.1f/100). "+
				"%d patent(s) identified with limited structural similarity. "+
				"Risk is manageable; periodic monitoring recommended. "+
				"Analysis depth: %s.",
			score, matchCount, depth)
	default:
		return fmt.Sprintf(
			"No significant infringement risk detected (score: %.1f/100). "+
				"%d patent(s) evaluated. "+
				"Analysis depth: %s.",
			score, matchCount, depth)
	}
}

// ipcToDomain extracts a technology domain label from an IPC code.
// Uses the IPC section and class (first 4 characters) as the domain key.
func ipcToDomain(ipc string) string {
	if len(ipc) < 4 {
		return "unknown"
	}

	sectionClass := ipc[:4]

	// Map common OLED-related IPC classes to readable domain names.
	domainMap := map[string]string{
		"C07C": "Organic Chemistry - Acyclic/Carbocyclic",
		"C07D": "Organic Chemistry - Heterocyclic",
		"C07F": "Organic Chemistry - Organometallic",
		"C09K": "Materials - Luminescent/Functional",
		"H10K": "OLED Devices",
		"H05B": "Lighting - Electroluminescence",
		"G02B": "Optics - Light Guides",
		"G09G": "Display Control",
		"C08G": "Polymers - Macromolecular",
		"C08L": "Polymer Compositions",
		"C23C": "Surface Coating/Deposition",
		"H01L": "Semiconductor Devices",
	}

	if domain, ok := domainMap[sectionClass]; ok {
		return domain
	}

	// Fallback: use section letter for broad categorization.
	switch ipc[0] {
	case 'A':
		return "Human Necessities"
	case 'B':
		return "Operations/Transport"
	case 'C':
		return "Chemistry/Metallurgy"
	case 'D':
		return "Textiles/Paper"
	case 'E':
		return "Fixed Constructions"
	case 'F':
		return "Mechanical Engineering"
	case 'G':
		return "Physics"
	case 'H':
		return "Electricity"
	default:
		return "Other - " + sectionClass
	}
}

// appendUnique appends a string to a slice only if it is not already present.
func appendUnique(slice []string, val string) []string {
	for _, s := range slice {
		if s == val {
			return slice
		}
	}
	return append(slice, val)
}

// ---------------------------------------------------------------------------
// Compile-time interface compliance check
// ---------------------------------------------------------------------------

var _ RiskAssessmentService = (*riskAssessmentServiceImpl)(nil)

// Ensure unused imports are referenced (build guard for Go 1.22.1).
var (
	_ = commonTypes.GenerateID
	_ = moleculeTypes.MoleculeID("")
	_ = patentTypes.PatentID("")
	_ = json.Marshal
	_ = hex.EncodeToString
	_ = sha256.New
	_ = sync.Mutex{}
)

// Service is an alias for RiskAssessmentService for backward compatibility with apiserver.
type Service = RiskAssessmentService

//Personal.AI order the ending

package strategy_gpt

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/turtacn/KeyIP-Intelligence/internal/intelligence/common"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"

	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// Enumerations
// ---------------------------------------------------------------------------

// OutputFormat controls how the LLM is instructed to format its response.
type OutputFormat int

const (
	FormatStructured OutputFormat = iota // JSON
	FormatNarrative                      // prose with Markdown headings
	FormatBullet                         // bullet-point lists
)

func (f OutputFormat) String() string {
	switch f {
	case FormatStructured:
		return "structured"
	case FormatNarrative:
		return "narrative"
	case FormatBullet:
		return "bullet"
	default:
		return "unknown"
	}
}

// ExportFormat controls the file format of the exported report.
type ExportFormat int

const (
	ExportJSON     ExportFormat = iota
	ExportMarkdown
	ExportPDF
	ExportDOCX
)

func (f ExportFormat) String() string {
	switch f {
	case ExportJSON:
		return "json"
	case ExportMarkdown:
		return "markdown"
	case ExportPDF:
		return "pdf"
	case ExportDOCX:
		return "docx"
	default:
		return "unknown"
	}
}

// DocumentSourceType classifies a citation source.
type DocumentSourceType string

const (
	SourcePatent     DocumentSourceType = "patent"
	SourceStatute    DocumentSourceType = "statute"
	SourceCaseLaw    DocumentSourceType = "case_law"
	SourceMPEP       DocumentSourceType = "mpep"
	SourceLiterature DocumentSourceType = "literature"
	SourceOther      DocumentSourceType = "other"
)

// ---------------------------------------------------------------------------
// Sentinel errors
// ---------------------------------------------------------------------------

var (
	ErrEmptyLLMOutput              = fmt.Errorf("empty LLM output")
	ErrExportFormatNotImplemented  = fmt.Errorf("export format not implemented")
)

// ---------------------------------------------------------------------------
// Request / Response types
// ---------------------------------------------------------------------------

// ReportRequest is the input for report generation.
type ReportRequest struct {
	Task          AnalysisTask   `json:"task"`
	Params        *PromptParams  `json:"params"`
	OutputFormat  OutputFormat   `json:"output_format"`
	ExportFormats []ExportFormat `json:"export_formats,omitempty"`
	QualityCheck  bool           `json:"quality_check"`
	RequestID     string         `json:"request_id"`
}

// Report is the top-level output of the report generator.
type Report struct {
	ReportID    string              `json:"report_id"`
	Task        AnalysisTask        `json:"task"`
	Content     *ReportContent      `json:"content"`
	Metadata    *ReportMetadata     `json:"metadata"`
	Validation  *ReportValidation   `json:"validation,omitempty"`
	GeneratedAt time.Time           `json:"generated_at"`
	LatencyMs   int64               `json:"latency_ms"`
	TokensUsed  *TokenUsage         `json:"tokens_used"`
}

// ReportContent holds the structured body of the report.
type ReportContent struct {
	Title            string              `json:"title"`
	ExecutiveSummary string              `json:"executive_summary"`
	Sections         []*ReportSection    `json:"sections"`
	Conclusions      []*Conclusion       `json:"conclusions"`
	Recommendations  []*Recommendation   `json:"recommendations"`
	RiskAssessment   *RiskAssessment     `json:"risk_assessment,omitempty"`
	Citations        []*Citation         `json:"citations"`
	RawOutput        string              `json:"raw_output,omitempty"`
}

// ReportSection is a recursive section structure.
type ReportSection struct {
	SectionID   string           `json:"section_id"`
	Title       string           `json:"title"`
	Content     string           `json:"content"`
	SubSections []*ReportSection `json:"sub_sections,omitempty"`
	Tables      []*ReportTable   `json:"tables,omitempty"`
	Figures     []*ReportFigure  `json:"figures,omitempty"`
	Order       int              `json:"order"`
}

// Conclusion represents a single conclusion statement.
type Conclusion struct {
	Statement          string   `json:"statement"`
	Confidence         float64  `json:"confidence"`
	SupportingEvidence []string `json:"supporting_evidence"`
	Severity           string   `json:"severity,omitempty"`
}

// Recommendation represents a single actionable recommendation.
type Recommendation struct {
	Action        string `json:"action"`
	Priority      string `json:"priority"`
	Rationale     string `json:"rationale"`
	Timeline      string `json:"timeline"`
	EstimatedCost string `json:"estimated_cost,omitempty"`
	RelatedClaims []int  `json:"related_claims,omitempty"`
}

// RiskAssessment is the overall risk evaluation.
type RiskAssessment struct {
	OverallRiskLevel     string                `json:"overall_risk_level"`
	OverallRiskScore     float64               `json:"overall_risk_score"`
	RiskFactors          []*RiskFactor         `json:"risk_factors"`
	MitigationStrategies []*MitigationStrategy `json:"mitigation_strategies"`
}

// RiskFactor is a single risk dimension.
type RiskFactor struct {
	Factor          string   `json:"factor"`
	Likelihood      float64  `json:"likelihood"`
	Impact          float64  `json:"impact"`
	RiskScore       float64  `json:"risk_score"`
	AffectedPatents []string `json:"affected_patents,omitempty"`
	AffectedClaims  []int    `json:"affected_claims,omitempty"`
}

// MitigationStrategy describes how to reduce a risk.
type MitigationStrategy struct {
	Strategy          string   `json:"strategy"`
	Effectiveness     string   `json:"effectiveness"`
	Feasibility       string   `json:"feasibility"`
	TargetRiskFactors []string `json:"target_risk_factors"`
}

// Citation is a reference to an external source.
type Citation struct {
	CitationID         string             `json:"citation_id"`
	Source             string             `json:"source"`
	SourceType         DocumentSourceType `json:"source_type"`
	RelevantText       string             `json:"relevant_text,omitempty"`
	VerificationStatus string             `json:"verification_status"`
	URL                string             `json:"url,omitempty"`
}

// ReportTable is a simple table embedded in a section.
type ReportTable struct {
	Title   string     `json:"title"`
	Headers []string   `json:"headers"`
	Rows    [][]string `json:"rows"`
}

// ReportFigure is a figure/chart description.
type ReportFigure struct {
	Title       string                 `json:"title"`
	Description string                 `json:"description"`
	DataPoints  map[string]interface{} `json:"data_points,omitempty"`
}

// TokenUsage tracks LLM token consumption.
type TokenUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ReportMetadata carries auxiliary information about the generation run.
type ReportMetadata struct {
	ModelID      string            `json:"model_id"`
	ModelVersion string            `json:"model_version"`
	PromptHash   string            `json:"prompt_hash,omitempty"`
	RAGEnabled   bool              `json:"rag_enabled"`
	RAGDocCount  int               `json:"rag_doc_count"`
	Labels       map[string]string `json:"labels,omitempty"`
}

// ReportValidation holds quality-check results.
type ReportValidation struct {
	IsValid               bool                        `json:"is_valid"`
	QualityScore          float64                     `json:"quality_score"`
	Issues                []*ValidationIssue          `json:"issues"`
	CitationVerification  *CitationVerificationResult `json:"citation_verification,omitempty"`
}

// ValidationIssue is a single quality problem.
type ValidationIssue struct {
	IssueType   string `json:"issue_type"`
	Description string `json:"description"`
	Severity    string `json:"severity"`
	Location    string `json:"location,omitempty"`
}

// CitationVerificationResult summarises citation checks.
type CitationVerificationResult struct {
	TotalCitations  int `json:"total_citations"`
	VerifiedCount   int `json:"verified_count"`
	UnverifiedCount int `json:"unverified_count"`
	NotFoundCount   int `json:"not_found_count"`
}

// ReportChunk is a single piece of a streamed report.
type ReportChunk struct {
	ChunkIndex  int    `json:"chunk_index"`
	Content     string `json:"content"`
	IsComplete  bool   `json:"is_complete"`
	SectionHint string `json:"section_hint,omitempty"`
}

// ---------------------------------------------------------------------------
// ReportGenerator interface
// ---------------------------------------------------------------------------

// ReportGenerator produces patent-strategy analysis reports.
type ReportGenerator interface {
	GenerateReport(ctx context.Context, req *ReportRequest) (*Report, error)
	GenerateReportStream(ctx context.Context, req *ReportRequest) (<-chan *ReportChunk, error)
	ParseLLMOutput(raw string, format OutputFormat) (*ReportContent, error)
	ValidateReport(report *Report) (*ReportValidation, error)
	ExportReport(report *Report, format ExportFormat) ([]byte, error)
}

// ---------------------------------------------------------------------------
// RAGEngine interface (expected from rag.go)
// ---------------------------------------------------------------------------

// RAGEngine is the retrieval-augmented generation engine.
type RAGEngine interface {
	RetrieveAndRerank(ctx context.Context, query string, topK int) ([]*RAGDocument, error)
}

// RAGDocument is a single document returned by the RAG engine.
type RAGDocument struct {
	DocumentID string  `json:"document_id"`
	Content    string  `json:"content"`
	Score      float64 `json:"score"`
	Source     string  `json:"source"`
	SourceType string  `json:"source_type"`
}

// ---------------------------------------------------------------------------
// PromptManager interface (expected from prompt.go)
// ---------------------------------------------------------------------------

// PromptManager builds prompts for the LLM.
type PromptManager interface {
	BuildPrompt(task AnalysisTask, params *PromptParams) (string, error)
}

// PromptParams carries all parameters needed to build a prompt.
type PromptParams struct {
	PatentNumbers   []string               `json:"patent_numbers,omitempty"`
	ClaimTexts      []string               `json:"claim_texts,omitempty"`
	ProductDesc     string                 `json:"product_desc,omitempty"`
	TechDomain      string                 `json:"tech_domain,omitempty"`
	Jurisdiction    string                 `json:"jurisdiction,omitempty"`
	RAGContext      string                 `json:"rag_context,omitempty"`
	OutputFormat    OutputFormat           `json:"output_format"`
	CustomFields    map[string]interface{} `json:"custom_fields,omitempty"`
}

// AnalysisTask enumerates the types of strategic analysis.
type AnalysisTask string

const (
	TaskFTO              AnalysisTask = "fto"
	TaskInfringement     AnalysisTask = "infringement"
	TaskValidity         AnalysisTask = "validity"
	TaskLandscape        AnalysisTask = "landscape"
	TaskPortfolioReview  AnalysisTask = "portfolio_review"
	TaskClaimConstruction AnalysisTask = "claim_construction"
)

// ---------------------------------------------------------------------------
// ModelBackend interface (LLM inference)
// ---------------------------------------------------------------------------

// ModelBackend abstracts the LLM serving layer.
type ModelBackend interface {
	Predict(ctx context.Context, req *common.PredictRequest) (*common.PredictResponse, error)
	PredictStream(ctx context.Context, req *common.PredictRequest) (<-chan *common.PredictResponse, error)
	Healthy(ctx context.Context) error
	Close() error
}

// ---------------------------------------------------------------------------
// Implementation
// ---------------------------------------------------------------------------

// reportGeneratorImpl is the concrete ReportGenerator.
type reportGeneratorImpl struct {
	modelBackend  ModelBackend
	promptManager PromptManager
	ragEngine     RAGEngine
	config        *StrategyGPTConfig
	metrics       common.IntelligenceMetrics
	logger        common.Logger
}

// StrategyGPTConfig holds configuration for the StrategyGPT subsystem.
type StrategyGPTConfig struct {
	ModelID        string  `json:"model_id" yaml:"model_id"`
	ModelVersion   string  `json:"model_version" yaml:"model_version"`
	RAGEnabled     bool    `json:"rag_enabled" yaml:"rag_enabled"`
	RAGTopK        int     `json:"rag_top_k" yaml:"rag_top_k"`
	MaxOutputTokens int   `json:"max_output_tokens" yaml:"max_output_tokens"`
	Temperature    float64 `json:"temperature" yaml:"temperature"`
	TimeoutMs      int64   `json:"timeout_ms" yaml:"timeout_ms"`
}

// DefaultStrategyGPTConfig returns sensible defaults.
func DefaultStrategyGPTConfig() *StrategyGPTConfig {
	return &StrategyGPTConfig{
		ModelID:         "strategy-gpt-v1",
		ModelVersion:    "1.0.0",
		RAGEnabled:      true,
		RAGTopK:         10,
		MaxOutputTokens: 4096,
		Temperature:     0.3,
		TimeoutMs:       60000,
	}
}

// NewReportGenerator creates a new ReportGenerator.
func NewReportGenerator(
	backend ModelBackend,
	promptMgr PromptManager,
	rag RAGEngine,
	cfg *StrategyGPTConfig,
	metrics common.IntelligenceMetrics,
	logger common.Logger,
) (ReportGenerator, error) {
	if backend == nil {
		return nil, errors.NewInvalidInputError("model backend is required")
	}
	if promptMgr == nil {
		return nil, errors.NewInvalidInputError("prompt manager is required")
	}
	if cfg == nil {
		cfg = DefaultStrategyGPTConfig()
	}
	if metrics == nil {
		metrics = common.NewNoopIntelligenceMetrics()
	}
	if logger == nil {
		logger = common.NewNoopLogger()
	}
	return &reportGeneratorImpl{
		modelBackend:  backend,
		promptManager: promptMgr,
		ragEngine:     rag,
		config:        cfg,
		metrics:       metrics,
		logger:        logger,
	}, nil
}

// ---------------------------------------------------------------------------
// GenerateReport — full (non-streaming) report generation
// ---------------------------------------------------------------------------

func (g *reportGeneratorImpl) GenerateReport(ctx context.Context, req *ReportRequest) (*Report, error) {
	if req == nil {
		return nil, errors.NewInvalidInputError("request is required")
	}
	start := time.Now()
	reportID := uuid.New().String()

	params := req.Params
	if params == nil {
		params = &PromptParams{}
	}
	params.OutputFormat = req.OutputFormat

	ragDocCount := 0

	// Step 1: RAG retrieval (if enabled)
	if g.config.RAGEnabled && g.ragEngine != nil {
		ragQuery := g.buildRAGQuery(req)
		docs, err := g.ragEngine.RetrieveAndRerank(ctx, ragQuery, g.config.RAGTopK)
		if err != nil {
			g.logger.Warn("RAG retrieval failed, proceeding without context", "error", err)
		} else {
			params.RAGContext = g.formatRAGContext(docs)
			ragDocCount = len(docs)
		}
	}

	// Step 2: Build prompt
	prompt, err := g.promptManager.BuildPrompt(req.Task, params)
	if err != nil {
		return nil, fmt.Errorf("building prompt: %w", err)
	}

	// Step 3: LLM prediction
	backendReq := &common.PredictRequest{
		ModelName:   g.config.ModelID,
		InputData:   []byte(prompt),
		InputFormat: common.FormatText,
		Metadata: map[string]string{
			"task":       string(req.Task),
			"request_id": req.RequestID,
		},
	}

	backendResp, err := g.modelBackend.Predict(ctx, backendReq)
	if err != nil {
		return nil, fmt.Errorf("LLM prediction: %w", err)
	}

	rawOutput := string(backendResp.Outputs["text"])
	if rawOutput == "" {
		if v, ok := backendResp.Outputs["output"]; ok {
			rawOutput = string(v)
		}
	}

	// Step 4: Parse LLM output
	content, err := g.ParseLLMOutput(rawOutput, req.OutputFormat)
	if err != nil {
		return nil, fmt.Errorf("parsing LLM output: %w", err)
	}

	// Step 5: Token usage
	tokenUsage := extractTokenUsage(backendResp)

	// Step 6: Assemble report
	report := &Report{
		ReportID:    reportID,
		Task:        req.Task,
		Content:     content,
		Metadata: &ReportMetadata{
			ModelID:      g.config.ModelID,
			ModelVersion: g.config.ModelVersion,
			RAGEnabled:   g.config.RAGEnabled && g.ragEngine != nil,
			RAGDocCount:  ragDocCount,
		},
		GeneratedAt: time.Now(),
		LatencyMs:   time.Since(start).Milliseconds(),
		TokensUsed:  tokenUsage,
	}

	// Step 7: Quality check (if requested)
	if req.QualityCheck {
		validation, vErr := g.ValidateReport(report)
		if vErr != nil {
			g.logger.Warn("quality check failed", "error", vErr)
		} else {
			report.Validation = validation
		}
	}

	// Step 8: Metrics
	g.metrics.RecordInference(ctx, &common.InferenceMetricParams{
		ModelName:    g.config.ModelID,
		ModelVersion: g.config.ModelVersion,
		TaskType:     string(req.Task),
		DurationMs:   float64(report.LatencyMs),
		Success:      true,
		BatchSize:    1,
	})

	return report, nil
}

// ---------------------------------------------------------------------------
// GenerateReportStream — streaming report generation
// ---------------------------------------------------------------------------

func (g *reportGeneratorImpl) GenerateReportStream(ctx context.Context, req *ReportRequest) (<-chan *ReportChunk, error) {
	if req == nil {
		return nil, errors.NewInvalidInputError("request is required")
	}

	params := req.Params
	if params == nil {
		params = &PromptParams{}
	}
	params.OutputFormat = req.OutputFormat

	// RAG retrieval
	if g.config.RAGEnabled && g.ragEngine != nil {
		ragQuery := g.buildRAGQuery(req)
		docs, err := g.ragEngine.RetrieveAndRerank(ctx, ragQuery, g.config.RAGTopK)
		if err != nil {
			g.logger.Warn("RAG retrieval failed in stream mode", "error", err)
		} else {
			params.RAGContext = g.formatRAGContext(docs)
		}
	}

	// Build prompt
	prompt, err := g.promptManager.BuildPrompt(req.Task, params)
	if err != nil {
		return nil, fmt.Errorf("building prompt: %w", err)
	}

	backendReq := &common.PredictRequest{
		ModelName:   g.config.ModelID,
		InputData:   []byte(prompt),
		InputFormat: common.FormatText,
		Metadata: map[string]string{
			"task":       string(req.Task),
			"request_id": req.RequestID,
			"stream":     "true",
		},
	}

	streamCh, err := g.modelBackend.PredictStream(ctx, backendReq)
	if err != nil {
		return nil, fmt.Errorf("LLM stream: %w", err)
	}

	outCh := make(chan *ReportChunk, 64)

	go func() {
		defer close(outCh)

		var (
			chunkIdx    int
			buf         strings.Builder
			currentHint string
		)

		for {
			select {
			case <-ctx.Done():
				outCh <- &ReportChunk{
					ChunkIndex: chunkIdx,
					Content:    "",
					IsComplete: true,
					SectionHint: currentHint,
				}
				return
			case resp, ok := <-streamCh:
				if !ok {
					// Stream ended — send final chunk
					if buf.Len() > 0 {
						outCh <- &ReportChunk{
							ChunkIndex:  chunkIdx,
							Content:     buf.String(),
							IsComplete:  false,
							SectionHint: currentHint,
						}
						chunkIdx++
						buf.Reset()
					}
					outCh <- &ReportChunk{
						ChunkIndex:  chunkIdx,
						Content:     "",
						IsComplete:  true,
						SectionHint: currentHint,
					}
					return
				}

				text := string(resp.Outputs["text"])
				if text == "" {
					if v, ok2 := resp.Outputs["output"]; ok2 {
						text = string(v)
					}
				}
				buf.WriteString(text)

				// Detect section boundaries (double newline or heading markers)
				if hint := detectSectionHint(text); hint != "" {
					currentHint = hint
				}

				// Flush on paragraph boundary
				if shouldFlush(buf.String()) {
					outCh <- &ReportChunk{
						ChunkIndex:  chunkIdx,
						Content:     buf.String(),
						IsComplete:  false,
						SectionHint: currentHint,
					}
					chunkIdx++
					buf.Reset()
				}
			}
		}
	}()

	return outCh, nil
}

// ---------------------------------------------------------------------------
// ParseLLMOutput — convert raw LLM text into structured ReportContent
// ---------------------------------------------------------------------------

func (g *reportGeneratorImpl) ParseLLMOutput(raw string, format OutputFormat) (*ReportContent, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, ErrEmptyLLMOutput
	}

	var content *ReportContent
	var err error

	switch format {
	case FormatStructured:
		content, err = parseStructuredOutput(raw)
	case FormatNarrative:
		content, err = parseNarrativeOutput(raw)
	case FormatBullet:
		content, err = parseBulletOutput(raw)
	default:
		content, err = parseNarrativeOutput(raw)
	}

	if err != nil {
		// Degrade: wrap entire output as a single section
		g.logger.Warn("parse failed, degrading to single section", "format", format.String(), "error", err)
		content = degradeToSingleSection(raw)
	}

	// Extract citations regardless of format
	content.Citations = extractCitations(raw)
	content.RawOutput = raw

	return content, nil
}

// ---------------------------------------------------------------------------
// ValidateReport — quality checks
// ---------------------------------------------------------------------------

func (g *reportGeneratorImpl) ValidateReport(report *Report) (*ReportValidation, error) {
	if report == nil || report.Content == nil {
		return &ReportValidation{IsValid: false, QualityScore: 0, Issues: []*ValidationIssue{
			{IssueType: "missing_content", Description: "report content is nil", Severity: "critical"},
		}}, nil
	}

	var issues []*ValidationIssue

	// 1. Structural completeness
	structureScore := 1.0
	if report.Content.ExecutiveSummary == "" {
		issues = append(issues, &ValidationIssue{
			IssueType:   "missing_summary",
			Description: "executive summary is missing",
			Severity:    "high",
			Location:    "executive_summary",
		})
		structureScore -= 0.4
	}
	if len(report.Content.Sections) == 0 {
		issues = append(issues, &ValidationIssue{
			IssueType:   "missing_sections",
			Description: "report has no sections",
			Severity:    "high",
			Location:    "sections",
		})
		structureScore -= 0.3
	}
	if len(report.Content.Conclusions) == 0 {
		issues = append(issues, &ValidationIssue{
			IssueType:   "missing_conclusions",
			Description: "report has no conclusions",
			Severity:    "medium",
			Location:    "conclusions",
		})
		structureScore -= 0.3
	}
	if structureScore < 0 {
		structureScore = 0
	}

	// 2. Citation verification
	citationScore := 1.0
	var citVerification *CitationVerificationResult
	if len(report.Content.Citations) > 0 {
		citVerification = g.verifyCitations(report.Content.Citations)
		if citVerification.TotalCitations > 0 {
			citationScore = float64(citVerification.VerifiedCount) / float64(citVerification.TotalCitations)
		}
		if citVerification.NotFoundCount > 0 {
			issues = append(issues, &ValidationIssue{
				IssueType:   "unverified_citations",
				Description: fmt.Sprintf("%d citations could not be verified", citVerification.NotFoundCount),
				Severity:    "medium",
				Location:    "citations",
			})
		}
	}

	// 3. Content length sufficiency
	lengthScore := computeLengthScore(report.Content)

	// 4. Recommendation actionability
	actionScore := computeActionabilityScore(report.Content.Recommendations)

	// Weighted quality score
	qualityScore := structureScore*0.3 + citationScore*0.3 + lengthScore*0.2 + actionScore*0.2
	qualityScore = math.Round(qualityScore*100) / 100

	isValid := len(issues) == 0 || qualityScore >= 0.5

	return &ReportValidation{
		IsValid:              isValid,
		QualityScore:         qualityScore,
		Issues:               issues,
		CitationVerification: citVerification,
	}, nil
}

// ---------------------------------------------------------------------------
// ExportReport — serialise to various formats
// ---------------------------------------------------------------------------

func (g *reportGeneratorImpl) ExportReport(report *Report, format ExportFormat) ([]byte, error) {
	if report == nil {
		return nil, errors.NewInvalidInputError("report is nil")
	}

	switch format {
	case ExportJSON:
		return json.MarshalIndent(report, "", "  ")
	case ExportMarkdown:
		return exportMarkdown(report)
	case ExportPDF:
		return nil, ErrExportFormatNotImplemented
	case ExportDOCX:
		return nil, ErrExportFormatNotImplemented
	default:
		return nil, fmt.Errorf("unsupported export format: %d", format)
	}
}

// ---------------------------------------------------------------------------
// Internal: RAG helpers
// ---------------------------------------------------------------------------

func (g *reportGeneratorImpl) buildRAGQuery(req *ReportRequest) string {
	var parts []string
	parts = append(parts, string(req.Task))
	if req.Params != nil {
		if req.Params.ProductDesc != "" {
			parts = append(parts, req.Params.ProductDesc)
		}
		if req.Params.TechDomain != "" {
			parts = append(parts, req.Params.TechDomain)
		}
		for _, pn := range req.Params.PatentNumbers {
			parts = append(parts, pn)
		}
	}
	return strings.Join(parts, " ")
}

func (g *reportGeneratorImpl) formatRAGContext(docs []*RAGDocument) string {
	if len(docs) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("--- Retrieved Context ---\n")
	for i, d := range docs {
		sb.WriteString(fmt.Sprintf("[%d] Source: %s (score: %.3f)\n%s\n\n", i+1, d.Source, d.Score, d.Content))
	}
	sb.WriteString("--- End Context ---\n")
	return sb.String()
}

// ---------------------------------------------------------------------------
// Internal: Parsing helpers
// ---------------------------------------------------------------------------

func parseStructuredOutput(raw string) (*ReportContent, error) {
	// Try to extract JSON from markdown code fences
	cleaned := raw
	if idx := strings.Index(raw, "```json"); idx >= 0 {
		start := idx + 7
		end := strings.Index(raw[start:], "```")
		if end >= 0 {
			cleaned = raw[start : start+end]
		}
	} else if idx := strings.Index(raw, "```"); idx >= 0 {
		start := idx + 3
		end := strings.Index(raw[start:], "```")
		if end >= 0 {
			cleaned = raw[start : start+end]
		}
	}
	cleaned = strings.TrimSpace(cleaned)

	var content ReportContent
	if err := json.Unmarshal([]byte(cleaned), &content); err != nil {
		return nil, fmt.Errorf("JSON parse error: %w", err)
	}
	return &content, nil
}

// Heading patterns for narrative parsing
var (
	headingRegex      = regexp.MustCompile(`(?m)^#{1,3}\s+(.+)$`)
	conclusionRegex   = regexp.MustCompile(`(?i)(conclusion|结论|findings|发现)`)
	recommendRegex    = regexp.MustCompile(`(?i)(recommendation|建议|action\s*items?|行动)`)
	summaryRegex      = regexp.MustCompile(`(?i)(executive\s*summary|摘要|概述|overview)`)
	riskRegex         = regexp.MustCompile(`(?i)(risk\s*assessment|风险评估|risk\s*analysis)`)
)

func parseNarrativeOutput(raw string) (*ReportContent, error) {
	content := &ReportContent{}

	// Split by headings
	headingLocs := headingRegex.FindAllStringSubmatchIndex(raw, -1)
	if len(headingLocs) == 0 {
		// No headings found — treat as single section
		return nil, fmt.Errorf("no headings found in narrative output")
	}

	type rawSection struct {
		title string
		body  string
	}

	var sections []rawSection
	for i, loc := range headingLocs {
		title := raw[loc[2]:loc[3]]
		bodyStart := loc[1]
		var bodyEnd int
		if i+1 < len(headingLocs) {
			bodyEnd = headingLocs[i+1][0]
		} else {
			bodyEnd = len(raw)
		}
		body := strings.TrimSpace(raw[bodyStart:bodyEnd])
		sections = append(sections, rawSection{title: title, body: body})
	}

	// Classify sections
	order := 0
	for _, sec := range sections {
		lowerTitle := strings.ToLower(sec.title)

		if summaryRegex.MatchString(lowerTitle) {
			content.ExecutiveSummary = sec.body
			continue
		}

		if conclusionRegex.MatchString(lowerTitle) {
			content.Conclusions = parseConclusions(sec.body)
			continue
		}

		if recommendRegex.MatchString(lowerTitle) {
			content.Recommendations = parseRecommendations(sec.body)
			continue
		}

		if riskRegex.MatchString(lowerTitle) {
			content.RiskAssessment = parseRiskSection(sec.body)
			continue
		}

		// Regular section
		order++
		content.Sections = append(content.Sections, &ReportSection{
			SectionID: fmt.Sprintf("sec-%d", order),
			Title:     sec.title,
			Content:   sec.body,
			Order:     order,
		})
	}

	// If no explicit summary was found, use the first paragraph of the first section
	if content.ExecutiveSummary == "" && len(sections) > 0 {
		firstBody := sections[0].body
		if idx := strings.Index(firstBody, "\n\n"); idx > 0 {
			content.ExecutiveSummary = firstBody[:idx]
		} else if len(firstBody) > 0 {
			content.ExecutiveSummary = firstBody
		}
	}

	// Derive title from the first heading if not set
	if content.Title == "" && len(sections) > 0 {
		content.Title = sections[0].title
	}

	return content, nil
}

func parseBulletOutput(raw string) (*ReportContent, error) {
	lines := strings.Split(raw, "\n")
	var items []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		// Strip leading bullet markers
		trimmed = strings.TrimLeft(trimmed, "-*•·")
		trimmed = strings.TrimSpace(trimmed)
		// Strip leading numbered markers like "1." "2)"
		if len(trimmed) > 2 && trimmed[0] >= '0' && trimmed[0] <= '9' {
			for i, c := range trimmed {
				if c == '.' || c == ')' {
					trimmed = strings.TrimSpace(trimmed[i+1:])
					break
				}
				if c < '0' || c > '9' {
					break
				}
			}
		}
		if trimmed != "" {
			items = append(items, trimmed)
		}
	}

	if len(items) == 0 {
		return nil, fmt.Errorf("no bullet items found")
	}

	content := &ReportContent{
		Title: "Analysis Report",
	}

	// First item as summary
	if len(items) > 0 {
		content.ExecutiveSummary = items[0]
	}

	// Remaining items as sections
	for i := 1; i < len(items); i++ {
		content.Sections = append(content.Sections, &ReportSection{
			SectionID: fmt.Sprintf("bullet-%d", i),
			Title:     fmt.Sprintf("Point %d", i),
			Content:   items[i],
			Order:     i,
		})
	}

	return content, nil
}

func degradeToSingleSection(raw string) *ReportContent {
	// Best-effort: use first paragraph as summary
	summary := raw
	if idx := strings.Index(raw, "\n\n"); idx > 0 && idx < 500 {
		summary = raw[:idx]
	} else if len(raw) > 500 {
		summary = raw[:500] + "..."
	}

	return &ReportContent{
		Title:            "Analysis Report",
		ExecutiveSummary: summary,
		Sections: []*ReportSection{
			{
				SectionID: "sec-1",
				Title:     "Full Analysis",
				Content:   raw,
				Order:     1,
			},
		},
		Conclusions: []*Conclusion{
			{
				Statement:  "See full analysis above.",
				Confidence: 0.5,
			},
		},
	}
}

// ---------------------------------------------------------------------------
// Internal: Conclusion / Recommendation / Risk parsing
// ---------------------------------------------------------------------------

var bulletLineRegex = regexp.MustCompile(`(?m)^[\s]*[-*•]\s*(.+)$`)

func parseConclusions(body string) []*Conclusion {
	matches := bulletLineRegex.FindAllStringSubmatch(body, -1)
	if len(matches) == 0 {
		// Treat each paragraph as a conclusion
		paragraphs := splitParagraphs(body)
		var conclusions []*Conclusion
		for _, p := range paragraphs {
			if p == "" {
				continue
			}
			conclusions = append(conclusions, &Conclusion{
				Statement:  p,
				Confidence: 0.7,
			})
		}
		if len(conclusions) == 0 && body != "" {
			conclusions = append(conclusions, &Conclusion{
				Statement:  body,
				Confidence: 0.7,
			})
		}
		return conclusions
	}

	var conclusions []*Conclusion
	for _, m := range matches {
		conclusions = append(conclusions, &Conclusion{
			Statement:  strings.TrimSpace(m[1]),
			Confidence: 0.7,
		})
	}
	return conclusions
}

func parseRecommendations(body string) []*Recommendation {
	matches := bulletLineRegex.FindAllStringSubmatch(body, -1)
	if len(matches) == 0 {
		paragraphs := splitParagraphs(body)
		var recs []*Recommendation
		for _, p := range paragraphs {
			if p == "" {
				continue
			}
			recs = append(recs, &Recommendation{
				Action:   p,
				Priority: "Medium",
			})
		}
		if len(recs) == 0 && body != "" {
			recs = append(recs, &Recommendation{
				Action:   body,
				Priority: "Medium",
			})
		}
		return recs
	}

	var recs []*Recommendation
	for i, m := range matches {
		priority := "Medium"
		if i == 0 {
			priority = "High"
		}
		recs = append(recs, &Recommendation{
			Action:   strings.TrimSpace(m[1]),
			Priority: priority,
		})
	}
	return recs
}

func parseRiskSection(body string) *RiskAssessment {
	ra := &RiskAssessment{
		OverallRiskLevel: "Medium",
		OverallRiskScore: 0.5,
	}

	matches := bulletLineRegex.FindAllStringSubmatch(body, -1)
	for _, m := range matches {
		text := strings.TrimSpace(m[1])
		rf := &RiskFactor{
			Factor:     text,
			Likelihood: 0.5,
			Impact:     0.5,
			RiskScore:  0.25,
		}
		ra.RiskFactors = append(ra.RiskFactors, rf)
	}

	if len(ra.RiskFactors) > 0 {
		totalScore := 0.0
		for _, rf := range ra.RiskFactors {
			totalScore += rf.RiskScore
		}
		ra.OverallRiskScore = totalScore / float64(len(ra.RiskFactors))
		ra.OverallRiskLevel = classifyRiskLevel(ra.OverallRiskScore)
	}

	return ra
}

// ---------------------------------------------------------------------------
// Internal: Citation extraction
// ---------------------------------------------------------------------------

var citationPatterns = []*regexp.Regexp{
	// US patents: US12345678, US 12,345,678, US2023/0123456
	regexp.MustCompile(`\[?Patent\s+(US[\s]?[\d,/]+[A-Z]?\d*)\]?`),
	// CN patents: CN112345678A
	regexp.MustCompile(`\[?(CN\d{9,}[A-Z]?)\]?`),
	// EP patents: EP1234567
	regexp.MustCompile(`\[?(EP\d{7,}[A-Z]?\d*)\]?`),
	// WO patents: WO2023/123456
	regexp.MustCompile(`\[?(WO\d{4}/\d{6})\]?`),
	// JP patents
	regexp.MustCompile(`\[?(JP\d{7,}[A-Z]?\d*)\]?`),
	// MPEP sections: MPEP §2111.03
	regexp.MustCompile(`\[?MPEP\s*§\s*([\d.]+)\]?`),
	// US Code: 35 U.S.C. §103
	regexp.MustCompile(`\[?(\d+\s+U\.?S\.?C\.?\s*§\s*\d+[a-z]?)\]?`),
	// Generic bracketed references
	regexp.MustCompile(`\[Patent\s+([^\]]+)\]`),
}

func extractCitations(raw string) []*Citation {
	seen := make(map[string]bool)
	var citations []*Citation

	for _, pat := range citationPatterns {
		matches := pat.FindAllStringSubmatch(raw, -1)
		for _, m := range matches {
			if len(m) < 2 {
				continue
			}
			source := strings.TrimSpace(m[1])
			if source == "" || seen[source] {
				continue
			}
			seen[source] = true

			citations = append(citations, &Citation{
				CitationID:         fmt.Sprintf("cit-%d", len(citations)+1),
				Source:             source,
				SourceType:         classifyCitationSource(source),
				VerificationStatus: "Unverified",
			})
		}
	}

	return citations
}

func classifyCitationSource(source string) DocumentSourceType {
	upper := strings.ToUpper(source)
	switch {
	case strings.HasPrefix(upper, "US") || strings.HasPrefix(upper, "CN") ||
		strings.HasPrefix(upper, "EP") || strings.HasPrefix(upper, "WO") ||
		strings.HasPrefix(upper, "JP"):
		return SourcePatent
	case strings.Contains(upper, "MPEP"):
		return SourceMPEP
	case strings.Contains(upper, "U.S.C") || strings.Contains(upper, "USC"):
		return SourceStatute
	default:
		return SourceOther
	}
}

// ---------------------------------------------------------------------------
// Internal: Citation verification
// ---------------------------------------------------------------------------

func (g *reportGeneratorImpl) verifyCitations(citations []*Citation) *CitationVerificationResult {
	result := &CitationVerificationResult{
		TotalCitations: len(citations),
	}

	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, cit := range citations {
		wg.Add(1)
		go func(c *Citation) {
			defer wg.Done()

			verified := g.verifySingleCitation(c)

			mu.Lock()
			defer mu.Unlock()
			if verified {
				c.VerificationStatus = "Verified"
				result.VerifiedCount++
			} else {
				// Distinguish between unverified and not-found
				if c.SourceType == SourcePatent {
					c.VerificationStatus = "NotFound"
					result.NotFoundCount++
				} else {
					c.VerificationStatus = "Unverified"
					result.UnverifiedCount++
				}
			}
		}(cit)
	}

	wg.Wait()
	return result
}

func (g *reportGeneratorImpl) verifySingleCitation(cit *Citation) bool {
	if g.ragEngine == nil {
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	docs, err := g.ragEngine.RetrieveAndRerank(ctx, cit.Source, 1)
	if err != nil || len(docs) == 0 {
		return false
	}

	// Consider verified if the top result has a high enough score
	return docs[0].Score >= 0.7
}

// ---------------------------------------------------------------------------
// Internal: Quality scoring helpers
// ---------------------------------------------------------------------------

func computeLengthScore(content *ReportContent) float64 {
	if content == nil {
		return 0
	}
	totalLen := len(content.ExecutiveSummary)
	for _, s := range content.Sections {
		totalLen += len(s.Content)
		for _, sub := range s.SubSections {
			totalLen += len(sub.Content)
		}
	}

	// Heuristic: a good report has at least 500 chars
	switch {
	case totalLen >= 2000:
		return 1.0
	case totalLen >= 1000:
		return 0.8
	case totalLen >= 500:
		return 0.6
	case totalLen >= 200:
		return 0.4
	default:
		return 0.2
	}
}

func computeActionabilityScore(recs []*Recommendation) float64 {
	if len(recs) == 0 {
		return 0.3 // Some credit for having no recommendations (may be valid)
	}

	score := 0.0
	for _, r := range recs {
		itemScore := 0.0
		if r.Action != "" {
			itemScore += 0.4
		}
		if r.Priority != "" {
			itemScore += 0.2
		}
		if r.Rationale != "" {
			itemScore += 0.2
		}
		if r.Timeline != "" {
			itemScore += 0.2
		}
		score += itemScore
	}
	return math.Min(score/float64(len(recs)), 1.0)
}

// ---------------------------------------------------------------------------
// Internal: Risk classification
// ---------------------------------------------------------------------------

// ClassifyRiskLevel maps a 0-1 score to a named level.
func classifyRiskLevel(score float64) string {
	switch {
	case score >= 0.8:
		return "Critical"
	case score >= 0.6:
		return "High"
	case score >= 0.4:
		return "Medium"
	case score >= 0.2:
		return "Low"
	default:
		return "Negligible"
	}
}

// ComputeRiskFactorScore calculates Likelihood * Impact.
func ComputeRiskFactorScore(likelihood, impact float64) float64 {
	return math.Max(0, math.Min(1, likelihood)) * math.Max(0, math.Min(1, impact))
}

// ---------------------------------------------------------------------------
// Internal: Markdown export
// ---------------------------------------------------------------------------

var markdownTmpl = `# {{ .Content.Title }}

## Executive Summary

{{ .Content.ExecutiveSummary }}

{{ range .Content.Sections }}
## {{ .Title }}

{{ .Content }}
{{ range .Tables }}
### {{ .Title }}

| {{ range .Headers }}{{ . }} | {{ end }}
| {{ range .Headers }}--- | {{ end }}
{{ range .Rows }}| {{ range . }}{{ . }} | {{ end }}
{{ end }}
{{ end }}
{{ end }}

{{ if .Content.Conclusions }}
## Conclusions

{{ range .Content.Conclusions }}
- {{ .Statement }} (confidence: {{ printf "%.0f" (mul .Confidence 100) }}%)
{{ end }}
{{ end }}

{{ if .Content.Recommendations }}
## Recommendations

{{ range .Content.Recommendations }}
- **[{{ .Priority }}]** {{ .Action }}
  - Rationale: {{ .Rationale }}
  - Timeline: {{ .Timeline }}
{{ end }}
{{ end }}

{{ if .Content.RiskAssessment }}
## Risk Assessment

Overall Risk: {{ .Content.RiskAssessment.OverallRiskLevel }} ({{ printf "%.2f" .Content.RiskAssessment.OverallRiskScore }})

{{ range .Content.RiskAssessment.RiskFactors }}
- {{ .Factor }} — Likelihood: {{ printf "%.2f" .Likelihood }}, Impact: {{ printf "%.2f" .Impact }}, Score: {{ printf "%.2f" .RiskScore }}
{{ end }}
{{ end }}

{{ if .Content.Citations }}
## References

{{ range .Content.Citations }}
- [{{ .CitationID }}] {{ .Source }} ({{ .SourceType }}) — {{ .VerificationStatus }}
{{ end }}
{{ end }}

---
Generated: {{ .GeneratedAt.Format "2006-01-02 15:04:05" }} | Model: {{ .Metadata.ModelID }} | Latency: {{ .LatencyMs }}ms
`

func exportMarkdown(report *Report) ([]byte, error) {
	funcMap := template.FuncMap{
		"mul": func(a, b float64) float64 { return a * b },
	}
	tmpl, err := template.New("report").Funcs(funcMap).Parse(markdownTmpl)
	if err != nil {
		return nil, fmt.Errorf("template parse: %w", err)
	}
	var buf strings.Builder
	if err := tmpl.Execute(&buf, report); err != nil {
		return nil, fmt.Errorf("template execute: %w", err)
	}
	return []byte(buf.String()), nil
}

// ---------------------------------------------------------------------------
// Internal: Stream helpers
// ---------------------------------------------------------------------------

var sectionHintRegex = regexp.MustCompile(`(?m)^#{1,3}\s+(.+)$`)

func detectSectionHint(text string) string {
	matches := sectionHintRegex.FindStringSubmatch(text)
	if len(matches) >= 2 {
		return strings.TrimSpace(matches[1])
	}
	return ""
}

func shouldFlush(buf string) bool {
	// Flush on double newline (paragraph boundary) or if buffer is large
	if len(buf) > 1024 {
		return true
	}
	return strings.HasSuffix(buf, "\n\n")
}

// ---------------------------------------------------------------------------
// Internal: Token usage extraction
// ---------------------------------------------------------------------------

func extractTokenUsage(resp *common.PredictResponse) *TokenUsage {
	if resp == nil {
		return &TokenUsage{}
	}
	usage := &TokenUsage{}

	if v, ok := resp.Outputs["prompt_tokens"]; ok {
		usage.PromptTokens = decodeIntFromBytes(v)
	}
	if v, ok := resp.Outputs["completion_tokens"]; ok {
		usage.CompletionTokens = decodeIntFromBytes(v)
	}
	if v, ok := resp.Outputs["total_tokens"]; ok {
		usage.TotalTokens = decodeIntFromBytes(v)
	}

	if usage.TotalTokens == 0 && (usage.PromptTokens > 0 || usage.CompletionTokens > 0) {
		usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	}

	return usage
}

func decodeIntFromBytes(b []byte) int {
	if len(b) == 0 {
		return 0
	}
	var v int
	_ = json.Unmarshal(b, &v)
	return v
}

// ---------------------------------------------------------------------------
// Internal: Misc helpers
// ---------------------------------------------------------------------------

func splitParagraphs(text string) []string {
	raw := strings.Split(text, "\n\n")
	var result []string
	for _, p := range raw {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

//Personal.AI order the ending

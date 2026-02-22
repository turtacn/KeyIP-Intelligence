package strategy_gpt

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/internal/intelligence/common"
)

// ExportFormat defines the format for report export.
type ExportFormat string

const (
	ExportJSON     ExportFormat = "json"
	ExportMarkdown ExportFormat = "markdown"
	ExportPDF      ExportFormat = "pdf"
	ExportDOCX     ExportFormat = "docx"
)

// ReportRequest represents a request for report generation.
type ReportRequest struct {
	Task          AnalysisTask
	Params        *PromptParams
	OutputFormat  OutputFormat
	ExportFormats []ExportFormat
	QualityCheck  bool
	RequestID     string
}

// Report represents a generated report.
type Report struct {
	ReportID    string
	Task        AnalysisTask
	Content     *ReportContent
	Metadata    *ReportMetadata
	Validation  *ReportValidation
	GeneratedAt time.Time
	LatencyMs   int64
	TokensUsed  *TokenUsage
}

// ReportContent represents the content of a report.
type ReportContent struct {
	Title            string          `json:"title"`
	ExecutiveSummary string          `json:"executive_summary"`
	Sections         []*ReportSection `json:"sections"`
	Conclusions      []*Conclusion    `json:"conclusions"`
	Recommendations  []*Recommendation `json:"recommendations"`
	Citations        []*Citation      `json:"citations"`
	RawOutput        string           `json:"raw_output"`
}

// ReportSection represents a section of a report.
type ReportSection struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

// Conclusion represents a conclusion in a report.
type Conclusion struct {
	Statement string `json:"statement"`
}

// Recommendation represents a recommendation in a report.
type Recommendation struct {
	Action   string `json:"action"`
	Priority string `json:"priority"`
}

// Citation represents a citation in a report.
type Citation struct {
	Source string `json:"source"`
}

// ReportMetadata represents metadata of a report.
type ReportMetadata struct {
	ModelVersion string
}

// ReportValidation represents validation results of a report.
type ReportValidation struct {
	IsValid      bool
	QualityScore float64
	Issues       []*ValidationIssue
}

type ValidationIssue struct {
	Description string
}

type TokenUsage struct {
	TotalTokens int
}

// ReportChunk represents a chunk of a report (for streaming).
type ReportChunk struct {
	ChunkIndex int
	Content    string
	IsComplete bool
}

// ReportGenerator defines the interface for report generation.
type ReportGenerator interface {
	GenerateReport(ctx context.Context, req *ReportRequest) (*Report, error)
	GenerateReportStream(ctx context.Context, req *ReportRequest) (<-chan *ReportChunk, error)
	ParseLLMOutput(raw string, format OutputFormat) (*ReportContent, error)
	ValidateReport(report *Report) (*ReportValidation, error)
	ExportReport(report *Report, format ExportFormat) ([]byte, error)
}

// reportGeneratorImpl implements ReportGenerator.
type reportGeneratorImpl struct {
	backend common.ModelBackend
	prompt  PromptManager
	rag     RAGEngine
	config  *StrategyGPTConfig
	logger  logging.Logger
}

// NewReportGenerator creates a new ReportGenerator.
func NewReportGenerator(
	backend common.ModelBackend,
	prompt PromptManager,
	rag RAGEngine,
	config *StrategyGPTConfig,
	logger logging.Logger,
) ReportGenerator {
	return &reportGeneratorImpl{
		backend: backend,
		prompt:  prompt,
		rag:     rag,
		config:  config,
		logger:  logger,
	}
}

func (g *reportGeneratorImpl) GenerateReport(ctx context.Context, req *ReportRequest) (*Report, error) {
	start := time.Now()

	// 1. RAG
	if g.config.RAGConfig.Enabled && g.rag != nil {
		// Mock RAG Query construction from params
		ragQuery := &RAGQuery{QueryText: req.Params.UserQuery}
		res, err := g.rag.RetrieveAndRerank(ctx, ragQuery)
		if err == nil {
			req.Params.RAGContext = res.Chunks
		}
	}

	// 2. Build Prompt
	bp, err := g.prompt.BuildPrompt(ctx, req.Task, req.Params)
	if err != nil {
		return nil, err
	}

	// 3. Predict
	pReq := &common.PredictRequest{
		ModelName: g.config.ModelID,
		// Needs input format expected by backend (chat format usually)
		// For now put encoded prompt
		InputData: []byte(bp.UserPrompt),
	}

	resp, err := g.backend.Predict(ctx, pReq)
	if err != nil {
		return nil, err
	}

	// 4. Parse
	// Assuming Output is in "text" key
	rawOutput := string(resp.Outputs["text"])
	if rawOutput == "" {
		// Maybe in "generated_text"
		rawOutput = string(resp.Outputs["generated_text"])
	}

	content, err := g.ParseLLMOutput(rawOutput, req.OutputFormat)
	if err != nil {
		// Fallback to raw content
		content = &ReportContent{RawOutput: rawOutput}
	}

	report := &Report{
		Task:        req.Task,
		Content:     content,
		GeneratedAt: time.Now(),
		LatencyMs:   time.Since(start).Milliseconds(),
	}

	// 5. Validation
	if req.QualityCheck {
		val, _ := g.ValidateReport(report)
		report.Validation = val
	}

	return report, nil
}

func (g *reportGeneratorImpl) GenerateReportStream(ctx context.Context, req *ReportRequest) (<-chan *ReportChunk, error) {
	// ... logic to call PredictStream and map chunks
	ch := make(chan *ReportChunk)
	close(ch)
	return ch, nil
}

func (g *reportGeneratorImpl) ParseLLMOutput(raw string, format OutputFormat) (*ReportContent, error) {
	if raw == "" {
		return nil, errors.New("empty output")
	}

	content := &ReportContent{RawOutput: raw}

	if format == FormatStructured {
		// Try JSON
		if err := json.Unmarshal([]byte(raw), content); err != nil {
			return nil, err
		}
	} else {
		// Heuristic parsing
		// Look for # Title, ## Section
		lines := strings.Split(raw, "\n")
		var currentSection *ReportSection

		for _, line := range lines {
			if strings.HasPrefix(line, "# ") {
				content.Title = strings.TrimPrefix(line, "# ")
			} else if strings.HasPrefix(line, "## ") {
				currentSection = &ReportSection{Title: strings.TrimPrefix(line, "## ")}
				content.Sections = append(content.Sections, currentSection)
			} else if currentSection != nil {
				currentSection.Content += line + "\n"
			}
		}
	}
	return content, nil
}

func (g *reportGeneratorImpl) ValidateReport(report *Report) (*ReportValidation, error) {
	// Basic validation
	valid := true
	issues := []*ValidationIssue{}

	if report.Content.Title == "" && len(report.Content.Sections) == 0 {
		valid = false
		issues = append(issues, &ValidationIssue{Description: "Empty report structure"})
	}

	return &ReportValidation{
		IsValid:      valid,
		QualityScore: 0.8, // Mock score
		Issues:       issues,
	}, nil
}

func (g *reportGeneratorImpl) ExportReport(report *Report, format ExportFormat) ([]byte, error) {
	if format == ExportJSON {
		return json.Marshal(report)
	}
	return nil, errors.New("format not implemented")
}

//Personal.AI order the ending

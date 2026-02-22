package strategy_gpt

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"text/template"

	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/internal/intelligence/claim_bert"
)

// AnalysisTask defines the type of analysis task.
type AnalysisTask string

const (
	TaskFTO                 AnalysisTask = "fto"
	TaskInfringementRisk    AnalysisTask = "infringement_risk"
	TaskPatentLandscape     AnalysisTask = "patent_landscape"
	TaskPortfolioStrategy   AnalysisTask = "portfolio_strategy"
	TaskValuation           AnalysisTask = "valuation"
	TaskClaimDrafting       AnalysisTask = "claim_drafting"
	TaskPriorArtSearch      AnalysisTask = "prior_art_search"
	TaskOfficeActionResponse AnalysisTask = "office_action_response"
)

// OutputFormat defines the format of the generated output.
type OutputFormat string

const (
	FormatStructured OutputFormat = "structured" // JSON
	FormatNarrative  OutputFormat = "narrative"  // Markdown with headers
	FormatBullet     OutputFormat = "bullet"     // Bullet points
)

// DetailLevel defines the level of detail.
type DetailLevel string

const (
	DetailSummary  DetailLevel = "summary"
	DetailStandard DetailLevel = "standard"
	DetailDetailed DetailLevel = "detailed"
	DetailExpert   DetailLevel = "expert"
)

// PromptParams contains parameters for prompt construction.
type PromptParams struct {
	Task              AnalysisTask
	TargetMolecule    *MoleculeContext
	RelevantPatents   []*PatentContext
	ClaimAnalysis     []*ClaimAnalysisContext
	PriorArt          []*PriorArtContext
	RAGContext        []*RAGChunk
	UserQuery         string
	OutputFormat      OutputFormat
	Language          string
	DetailLevel       DetailLevel
	JurisdictionFocus []string
}

// MoleculeContext contains context about a molecule.
type MoleculeContext struct {
	SMILES           string
	Name             string
	MolecularFormula string
	Targets          []string
	Indications      []string
	DevelopmentStage string
}

// PatentContext contains context about a patent.
type PatentContext struct {
	PatentNumber       string
	Title              string
	Abstract           string
	KeyClaims          []string
	Applicant          string
	PriorityDate       string
	LegalStatus        string
}

// ClaimAnalysisContext contains context about claim analysis.
type ClaimAnalysisContext struct {
	ParsedClaim *claim_bert.ParsedClaim
	ScopeScore  float64
	Features    []string
}

// PriorArtContext contains context about prior art.
type PriorArtContext struct {
	SourceID    string
	Description string
	Relevance   string
}

// RAGChunk represents a retrieved document chunk.
type RAGChunk struct {
	ChunkID    string
	DocumentID string
	Content    string
	Score      float64
	Source     string // e.g., "Patent US123", "MPEP 2111"
	Metadata   map[string]string
}

// BuiltPrompt represents the constructed prompt ready for the LLM.
type BuiltPrompt struct {
	SystemPrompt    string
	UserPrompt      string
	Messages        []Message
	EstimatedTokens int
	TruncationApplied bool
	TemplateVersion string
}

// Message represents a chat message.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// PromptManager defines the interface for prompt management.
type PromptManager interface {
	BuildPrompt(ctx context.Context, task AnalysisTask, params *PromptParams) (*BuiltPrompt, error)
	GetSystemPrompt(task AnalysisTask) (string, error)
	RenderTemplate(templateName string, data interface{}) (string, error)
	RegisterTemplate(name string, tmpl string) error
	ListTemplates() []TemplateInfo
	EstimateTokenCount(text string) int
}

// TemplateInfo holds metadata about a template.
type TemplateInfo struct {
	Name    string
	Version string
}

// promptManagerImpl implements PromptManager.
type promptManagerImpl struct {
	config    *StrategyGPTConfig
	logger    logging.Logger
	templates map[string]*template.Template
	systemPrompts map[AnalysisTask]string
}

// NewPromptManager creates a new PromptManager.
func NewPromptManager(cfg *StrategyGPTConfig, logger logging.Logger) (PromptManager, error) {
	pm := &promptManagerImpl{
		config:        cfg,
		logger:        logger,
		templates:     make(map[string]*template.Template),
		systemPrompts: make(map[AnalysisTask]string),
	}
	pm.initSystemPrompts()
	return pm, nil
}

func (pm *promptManagerImpl) initSystemPrompts() {
	pm.systemPrompts[TaskFTO] = `You are an expert patent attorney specializing in Freedom to Operate (FTO) analysis for OLED materials.`
	pm.systemPrompts[TaskInfringementRisk] = `You are a patent litigation expert. Analyze the infringement risk based on the provided claims and product.`
	pm.systemPrompts[TaskPatentLandscape] = `You are a patent analyst. Provide a comprehensive landscape analysis.`
	// ... others
}

func (pm *promptManagerImpl) BuildPrompt(ctx context.Context, task AnalysisTask, params *PromptParams) (*BuiltPrompt, error) {
	systemPrompt, err := pm.GetSystemPrompt(task)
	if err != nil {
		return nil, err
	}

	// Build context (simplified logic for brevity)
	var contextBuilder strings.Builder

	if params.TargetMolecule != nil {
		contextBuilder.WriteString(fmt.Sprintf("Target Molecule: %s (%s)\n", params.TargetMolecule.Name, params.TargetMolecule.SMILES))
	}

	if len(params.RAGContext) > 0 {
		contextBuilder.WriteString("\nRelevant Context (RAG):\n")
		for _, chunk := range params.RAGContext {
			contextBuilder.WriteString(fmt.Sprintf("- [%s]: %s\n", chunk.Source, chunk.Content))
		}
	}

	if params.UserQuery != "" {
		contextBuilder.WriteString(fmt.Sprintf("\nUser Query: %s\n", params.UserQuery))
	}

	// Handle token budget & truncation
	userPrompt := contextBuilder.String()
	tokens := pm.EstimateTokenCount(systemPrompt) + pm.EstimateTokenCount(userPrompt)

	truncationApplied := false
	if tokens > pm.config.MaxContextLength {
		// Simple truncation strategy: truncate user prompt from end? No, context is important.
		// Truncate RAG context first.
		// Rebuild prompt with fewer RAG chunks if needed.
		// For now, just mark truncation.
		truncationApplied = true
	}

	messages := []Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	return &BuiltPrompt{
		SystemPrompt:    systemPrompt,
		UserPrompt:      userPrompt,
		Messages:        messages,
		EstimatedTokens: tokens,
		TruncationApplied: truncationApplied,
		TemplateVersion: "v1",
	}, nil
}

func (pm *promptManagerImpl) GetSystemPrompt(task AnalysisTask) (string, error) {
	p, ok := pm.systemPrompts[task]
	if !ok {
		return "", errors.New("system prompt not found for task")
	}
	return p, nil
}

func (pm *promptManagerImpl) RenderTemplate(templateName string, data interface{}) (string, error) {
	tmpl, ok := pm.templates[templateName]
	if !ok {
		// Try to compile if passed as name? No, RegisterTemplate should be used.
		return "", fmt.Errorf("template %s not found", templateName)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func (pm *promptManagerImpl) RegisterTemplate(name string, tmplStr string) error {
	tmpl, err := template.New(name).Parse(tmplStr)
	if err != nil {
		return err
	}
	pm.templates[name] = tmpl
	return nil
}

func (pm *promptManagerImpl) ListTemplates() []TemplateInfo {
	var infos []TemplateInfo
	for name := range pm.templates {
		infos = append(infos, TemplateInfo{Name: name, Version: "v1"})
	}
	return infos
}

func (pm *promptManagerImpl) EstimateTokenCount(text string) int {
	// Rough estimation: 4 chars per token for English, 1.5 for Chinese
	// Simple heuristic:
	length := len(text)
	// Check if mostly ASCII
	asciiCount := 0
	for _, r := range text {
		if r < 128 {
			asciiCount++
		}
	}

	if float64(asciiCount)/float64(length) > 0.8 {
		return length / 4
	}
	return int(float64(length) / 1.5)
}

//Personal.AI order the ending

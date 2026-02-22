package strategy_gpt

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"sync"
	"text/template"
	"time"
	"unicode/utf8"

	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// ---------------------------------------------------------------------------
// AnalysisTask enumeration
// ---------------------------------------------------------------------------

// AnalysisTask enumerates the strategic analysis tasks supported by StrategyGPT.
type AnalysisTask int

const (
	TaskFTO                 AnalysisTask = iota // Freedom to Operate
	TaskInfringementRisk                        // Infringement risk assessment
	TaskPatentLandscape                         // Patent landscape analysis
	TaskPortfolioStrategy                       // Patent portfolio strategy
	TaskValuation                               // Patent valuation
	TaskClaimDrafting                           // Claim drafting assistance
	TaskPriorArtSearch                          // Prior art search strategy
	TaskOfficeActionResponse                    // Office action response advice
)

var analysisTaskNames = map[AnalysisTask]string{
	TaskFTO:                  "FTO",
	TaskInfringementRisk:     "InfringementRisk",
	TaskPatentLandscape:      "PatentLandscape",
	TaskPortfolioStrategy:    "PortfolioStrategy",
	TaskValuation:            "Valuation",
	TaskClaimDrafting:        "ClaimDrafting",
	TaskPriorArtSearch:       "PriorArtSearch",
	TaskOfficeActionResponse: "OfficeActionResponse",
}

func (t AnalysisTask) String() string {
	if n, ok := analysisTaskNames[t]; ok {
		return n
	}
	return fmt.Sprintf("UnknownTask(%d)", int(t))
}

// IsValid returns true when the task is a known enumeration value.
func (t AnalysisTask) IsValid() bool {
	_, ok := analysisTaskNames[t]
	return ok
}

// ---------------------------------------------------------------------------
// OutputFormat / DetailLevel
// ---------------------------------------------------------------------------

// OutputFormat controls the shape of the LLM response.
type OutputFormat int

const (
	OutputStructured OutputFormat = iota
	OutputNarrative
	OutputBullet
)

func (f OutputFormat) String() string {
	switch f {
	case OutputStructured:
		return "Structured"
	case OutputNarrative:
		return "Narrative"
	case OutputBullet:
		return "Bullet"
	default:
		return "Unknown"
	}
}

// DetailLevel controls the depth of analysis.
type DetailLevel int

const (
	DetailSummary  DetailLevel = iota
	DetailStandard
	DetailDetailed
	DetailExpert
)

func (d DetailLevel) String() string {
	switch d {
	case DetailSummary:
		return "Summary"
	case DetailStandard:
		return "Standard"
	case DetailDetailed:
		return "Detailed"
	case DetailExpert:
		return "Expert"
	default:
		return "Unknown"
	}
}

// ---------------------------------------------------------------------------
// Context types
// ---------------------------------------------------------------------------

// MoleculeContext carries molecule-level information for prompt construction.
type MoleculeContext struct {
	SMILES           string   `json:"smiles"`
	Name             string   `json:"name"`
	MolecularFormula string   `json:"molecular_formula"`
	Targets          []string `json:"targets,omitempty"`
	Indications      []string `json:"indications,omitempty"`
	DevelopmentStage string   `json:"development_stage,omitempty"`
}

// PatentContext carries patent-level information for prompt construction.
type PatentContext struct {
	PatentNumber  string   `json:"patent_number"`
	Title         string   `json:"title"`
	Abstract      string   `json:"abstract"`
	KeyClaims     []string `json:"key_claims,omitempty"`
	Applicant     string   `json:"applicant"`
	PriorityDate  string   `json:"priority_date"`
	LegalStatus   string   `json:"legal_status"`
	Jurisdictions []string `json:"jurisdictions,omitempty"`
}

// ClaimAnalysisContext carries parsed claim analysis results.
type ClaimAnalysisContext struct {
	ClaimNumber       int      `json:"claim_number"`
	ClaimText         string   `json:"claim_text"`
	ScopeScore        float64  `json:"scope_score"`
	TechnicalFeatures []string `json:"technical_features"`
	ClaimType         string   `json:"claim_type"`
	DependsOn         []int    `json:"depends_on,omitempty"`
}

// PriorArtContext carries prior art references.
type PriorArtContext struct {
	Reference   string  `json:"reference"`
	Title       string  `json:"title"`
	Abstract    string  `json:"abstract"`
	Relevance   float64 `json:"relevance"`
	PublishDate string  `json:"publish_date"`
}

// RAGChunk carries a single RAG retrieval result.
type RAGChunk struct {
	Text       string            `json:"text"`
	Source     string            `json:"source"`
	Score      float64           `json:"score"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

// ---------------------------------------------------------------------------
// PromptParams / BuiltPrompt / Message
// ---------------------------------------------------------------------------

// PromptParams is the full parameter set for building a prompt.
type PromptParams struct {
	Task               AnalysisTask           `json:"task"`
	TargetMolecule     *MoleculeContext       `json:"target_molecule,omitempty"`
	RelevantPatents    []*PatentContext        `json:"relevant_patents,omitempty"`
	ClaimAnalysis      []*ClaimAnalysisContext `json:"claim_analysis,omitempty"`
	PriorArt           []*PriorArtContext      `json:"prior_art,omitempty"`
	RAGContext         []*RAGChunk             `json:"rag_context,omitempty"`
	UserQuery          string                 `json:"user_query"`
	OutputFormat       OutputFormat           `json:"output_format"`
	Language           string                 `json:"language"`
	DetailLevel        DetailLevel            `json:"detail_level"`
	JurisdictionFocus  []string               `json:"jurisdiction_focus,omitempty"`
}

// Message represents a single message in a chat-style prompt.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// BuiltPrompt is the fully assembled prompt ready for LLM invocation.
type BuiltPrompt struct {
	SystemPrompt       string    `json:"system_prompt"`
	UserPrompt         string    `json:"user_prompt"`
	Messages           []Message `json:"messages"`
	EstimatedTokens    int       `json:"estimated_tokens"`
	TruncationApplied  bool      `json:"truncation_applied"`
	TemplateVersion    string    `json:"template_version"`
}

// ---------------------------------------------------------------------------
// TemplateInfo
// ---------------------------------------------------------------------------

// TemplateInfo describes a registered prompt template.
type TemplateInfo struct {
	Name        string    `json:"name"`
	Version     string    `json:"version"`
	Task        string    `json:"task,omitempty"`
	Description string    `json:"description,omitempty"`
	RegisteredAt time.Time `json:"registered_at"`
}

type templateEntry struct {
	raw     string
	parsed  *template.Template
	info    TemplateInfo
}

// ---------------------------------------------------------------------------
// PromptManager interface
// ---------------------------------------------------------------------------

// PromptManager manages prompt templates and builds prompts for StrategyGPT.
type PromptManager interface {
	BuildPrompt(ctx context.Context, task AnalysisTask, params *PromptParams) (*BuiltPrompt, error)
	GetSystemPrompt(task AnalysisTask) (string, error)
	RenderTemplate(templateName string, data interface{}) (string, error)
	RegisterTemplate(name string, tmpl string) error
	ListTemplates() []TemplateInfo
	EstimateTokenCount(text string) int
}

// ---------------------------------------------------------------------------
// PromptManagerConfig
// ---------------------------------------------------------------------------

// PromptManagerConfig holds tuning knobs for the prompt manager.
type PromptManagerConfig struct {
	MaxContextTokens   int    `json:"max_context_tokens" yaml:"max_context_tokens"`
	DefaultLanguage    string `json:"default_language" yaml:"default_language"`
	TemplateVersion    string `json:"template_version" yaml:"template_version"`
}

// DefaultPromptManagerConfig returns production defaults.
func DefaultPromptManagerConfig() *PromptManagerConfig {
	return &PromptManagerConfig{
		MaxContextTokens: 12000,
		DefaultLanguage:  "en",
		TemplateVersion:  "v1.0",
	}
}

// ---------------------------------------------------------------------------
// promptManagerImpl
// ---------------------------------------------------------------------------

type promptManagerImpl struct {
	templates map[string]*templateEntry
	config    *PromptManagerConfig
	funcMap   template.FuncMap
	mu        sync.RWMutex
}

// NewPromptManager creates a PromptManager with built-in templates pre-loaded.
func NewPromptManager(config *PromptManagerConfig) (PromptManager, error) {
	if config == nil {
		config = DefaultPromptManagerConfig()
	}
	if config.MaxContextTokens <= 0 {
		config.MaxContextTokens = 12000
	}

	pm := &promptManagerImpl{
		templates: make(map[string]*templateEntry),
		config:    config,
		funcMap:   defaultFuncMap(),
	}

	// Register all built-in templates.
	for name, raw := range builtinTemplates {
		if err := pm.RegisterTemplate(name, raw); err != nil {
			return nil, fmt.Errorf("registering built-in template %s: %w", name, err)
		}
	}
	return pm, nil
}

// ---------------------------------------------------------------------------
// BuildPrompt
// ---------------------------------------------------------------------------

func (pm *promptManagerImpl) BuildPrompt(ctx context.Context, task AnalysisTask, params *PromptParams) (*BuiltPrompt, error) {
	if !task.IsValid() {
		return nil, errors.NewInvalidInputError(fmt.Sprintf("unknown analysis task: %d", int(task)))
	}
	if params == nil {
		params = &PromptParams{Task: task}
	}
	params.Task = task
	if params.Language == "" {
		params.Language = pm.config.DefaultLanguage
	}

	// 1. System prompt
	systemPrompt, err := pm.GetSystemPrompt(task)
	if err != nil {
		return nil, err
	}

	// 2. Build context sections with token budgets
	budget := pm.config.MaxContextTokens
	systemTokens := pm.EstimateTokenCount(systemPrompt)
	remaining := budget - systemTokens
	if remaining < 200 {
		remaining = 200
	}

	// Reserve tokens for user query (never truncated).
	queryTokens := pm.EstimateTokenCount(params.UserQuery)
	remaining -= queryTokens
	if remaining < 0 {
		remaining = 0
	}

	// Reserve tokens for instruction block (~300 tokens).
	instructionReserve := 300
	remaining -= instructionReserve
	if remaining < 0 {
		remaining = 0
	}

	truncated := false

	// Build sections in priority order (highest priority = last to truncate).
	// Priority (low→high): RAG -> patent abstracts -> claim details -> molecule info
	sections := make([]promptSection, 0, 4)

	ragSection := pm.buildRAGSection(params.RAGContext)
	patentSection := pm.buildPatentSection(params.RelevantPatents)
	claimSection := pm.buildClaimSection(params.ClaimAnalysis)
	moleculeSection := pm.buildMoleculeSection(params.TargetMolecule)
	priorArtSection := pm.buildPriorArtSection(params.PriorArt)

	// Ordered low→high priority for truncation.
	candidates := []promptSection{ragSection, priorArtSection, patentSection, claimSection, moleculeSection}

	// Calculate total context tokens.
	totalContext := 0
	for _, s := range candidates {
		totalContext += s.tokens
	}

	// Truncate from lowest priority if over budget.
	if totalContext > remaining {
		truncated = true
		excess := totalContext - remaining
		for i := 0; i < len(candidates) && excess > 0; i++ {
			if candidates[i].tokens <= 0 {
				continue
			}
			if candidates[i].tokens <= excess {
				excess -= candidates[i].tokens
				candidates[i].text = ""
				candidates[i].tokens = 0
			} else {
				// Partial truncation: keep as many characters as the remaining budget allows.
				allowedTokens := candidates[i].tokens - excess
				candidates[i].text = truncateToTokens(candidates[i].text, allowedTokens, pm)
				candidates[i].tokens = allowedTokens
				excess = 0
			}
		}
	}

	// Re-map after truncation.
	ragText := candidates[0].text
	priorArtText := candidates[1].text
	patentText := candidates[2].text
	claimText := candidates[3].text
	moleculeText := candidates[4].text

	// Collect non-empty sections.
	for _, s := range []struct{ label, text string }{
		{"Molecule Information", moleculeText},
		{"Relevant Patents", patentText},
		{"Claim Analysis", claimText},
		{"Prior Art", priorArtText},
		{"Retrieved Context (RAG)", ragText},
	} {
		if s.text != "" {
			sections = append(sections, promptSection{label: s.label, text: s.text})
		}
	}

	// 3. Build instruction block.
	instructionBlock := pm.buildInstructionBlock(params)

	// 4. Assemble user prompt.
	var userBuf strings.Builder
	for _, sec := range sections {
		userBuf.WriteString(fmt.Sprintf("## %s\n%s\n\n", sec.label, sec.text))
	}
	if instructionBlock != "" {
		userBuf.WriteString(fmt.Sprintf("## Instructions\n%s\n\n", instructionBlock))
	}
	if params.UserQuery != "" {
		userBuf.WriteString(fmt.Sprintf("## User Query\n%s\n", params.UserQuery))
	}
	userPrompt := strings.TrimSpace(userBuf.String())

	// 5. Assemble messages.
	messages := []Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	totalTokens := pm.EstimateTokenCount(systemPrompt) + pm.EstimateTokenCount(userPrompt)

	return &BuiltPrompt{
		SystemPrompt:      systemPrompt,
		UserPrompt:        userPrompt,
		Messages:          messages,
		EstimatedTokens:   totalTokens,
		TruncationApplied: truncated,
		TemplateVersion:   pm.config.TemplateVersion,
	}, nil
}

// ---------------------------------------------------------------------------
// GetSystemPrompt
// ---------------------------------------------------------------------------

func (pm *promptManagerImpl) GetSystemPrompt(task AnalysisTask) (string, error) {
	key := systemTemplateKey(task)
	pm.mu.RLock()
	entry, ok := pm.templates[key]
	pm.mu.RUnlock()
	if !ok {
		return "", errors.NewInvalidInputError(fmt.Sprintf("no system prompt template for task %s", task))
	}
	return entry.raw, nil
}

// ---------------------------------------------------------------------------
// RenderTemplate
// ---------------------------------------------------------------------------

func (pm *promptManagerImpl) RenderTemplate(templateName string, data interface{}) (string, error) {
	pm.mu.RLock()
	entry, ok := pm.templates[templateName]
	pm.mu.RUnlock()
	if !ok {
		return "", errors.NewInvalidInputError(fmt.Sprintf("template %q not found", templateName))
	}
	var buf bytes.Buffer
	if err := entry.parsed.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("rendering template %q: %w", templateName, err)
	}
	return buf.String(), nil
}

// ---------------------------------------------------------------------------
// RegisterTemplate
// ---------------------------------------------------------------------------

func (pm *promptManagerImpl) RegisterTemplate(name string, tmpl string) error {
	if name == "" {
		return errors.NewInvalidInputError("template name is required")
	}
	if tmpl == "" {
		return errors.NewInvalidInputError("template body is required")
	}
	parsed, err := template.New(name).Funcs(pm.funcMap).Parse(tmpl)
	if err != nil {
		return fmt.Errorf("parsing template %q: %w", name, err)
	}
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.templates[name] = &templateEntry{
		raw:    tmpl,
		parsed: parsed,
		info: TemplateInfo{
			Name:         name,
			Version:      pm.config.TemplateVersion,
			RegisteredAt: time.Now(),
		},
	}
	return nil
}

// ---------------------------------------------------------------------------
// ListTemplates
// ---------------------------------------------------------------------------

func (pm *promptManagerImpl) ListTemplates() []TemplateInfo {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	out := make([]TemplateInfo, 0, len(pm.templates))
	for _, e := range pm.templates {
		out = append(out, e.info)
	}
	return out
}

// ---------------------------------------------------------------------------
// EstimateTokenCount
// ---------------------------------------------------------------------------

// EstimateTokenCount provides a rough token estimate.
// English: ~4 characters per token. Chinese: ~1.5 characters per token.
func (pm *promptManagerImpl) EstimateTokenCount(text string) int {
	if text == "" {
		return 0
	}
	chineseCount := 0
	totalRunes := 0
	for _, r := range text {
		totalRunes++
		if isCJK(r) {
			chineseCount++
		}
	}
	otherCount := totalRunes - chineseCount
	// Chinese characters: ~1.5 chars/token → each char ≈ 0.67 tokens
	// Other characters: ~4 chars/token → each char ≈ 0.25 tokens
	tokens := float64(chineseCount)*0.67 + float64(otherCount)*0.25
	if tokens < 1 && len(text) > 0 {
		return 1
	}
	return int(tokens + 0.5)
}

func isCJK(r rune) bool {
	return (r >= 0x4E00 && r <= 0x9FFF) ||
		(r >= 0x3400 && r <= 0x4DBF) ||
		(r >= 0x20000 && r <= 0x2A6DF) ||
		(r >= 0x2A700 && r <= 0x2B73F) ||
		(r >= 0x2B740 && r <= 0x2B81F) ||
		(r >= 0x2B820 && r <= 0x2CEAF) ||
		(r >= 0xF900 && r <= 0xFAFF) ||
		(r >= 0x2F800 && r <= 0x2FA1F) ||
		(r >= 0x3000 && r <= 0x303F) || // CJK punctuation
		(r >= 0xFF00 && r <= 0xFFEF) // fullwidth forms
}

// ---------------------------------------------------------------------------
// Section builders
// ---------------------------------------------------------------------------

type promptSection struct {
	label  string
	text   string
	tokens int
}

func (pm *promptManagerImpl) buildMoleculeSection(mol *MoleculeContext) promptSection {
	if mol == nil {
		return promptSection{}
	}
	var b strings.Builder
	if mol.Name != "" {
		b.WriteString(fmt.Sprintf("Name: %s\n", mol.Name))
	}
	if mol.SMILES != "" {
		b.WriteString(fmt.Sprintf("SMILES: %s\n", mol.SMILES))
	}
	if mol.MolecularFormula != "" {
		b.WriteString(fmt.Sprintf("Formula: %s\n", mol.MolecularFormula))
	}
	if len(mol.Targets) > 0 {
		b.WriteString(fmt.Sprintf("Targets: %s\n", strings.Join(mol.Targets, ", ")))
	}
	if len(mol.Indications) > 0 {
		b.WriteString(fmt.Sprintf("Indications: %s\n", strings.Join(mol.Indications, ", ")))
	}
	if mol.DevelopmentStage != "" {
		b.WriteString(fmt.Sprintf("Development Stage: %s\n", mol.DevelopmentStage))
	}
	text := b.String()
	return promptSection{label: "Molecule Information", text: text, tokens: pm.EstimateTokenCount(text)}
}

func (pm *promptManagerImpl) buildPatentSection(patents []*PatentContext) promptSection {
	if len(patents) == 0 {
		return promptSection{}
	}
	var b strings.Builder
	for i, p := range patents {
		b.WriteString(fmt.Sprintf("### Patent %d: %s\n", i+1, p.PatentNumber))
		if p.Title != "" {
			b.WriteString(fmt.Sprintf("Title: %s\n", p.Title))
		}
		if p.Applicant != "" {
			b.WriteString(fmt.Sprintf("Applicant: %s\n", p.Applicant))
		}
		if p.PriorityDate != "" {
			b.WriteString(fmt.Sprintf("Priority Date: %s\n", p.PriorityDate))
		}
		if p.LegalStatus != "" {
			b.WriteString(fmt.Sprintf("Legal Status: %s\n", p.LegalStatus))
		}
		if p.Abstract != "" {
			b.WriteString(fmt.Sprintf("Abstract: %s\n", p.Abstract))
		}
		for j, c := range p.KeyClaims {
			b.WriteString(fmt.Sprintf("  Claim %d: %s\n", j+1, c))
		}
		b.WriteString("\n")
	}
	text := b.String()
	return promptSection{label: "Relevant Patents", text: text, tokens: pm.EstimateTokenCount(text)}
}

func (pm *promptManagerImpl) buildClaimSection(claims []*ClaimAnalysisContext) promptSection {
	if len(claims) == 0 {
		return promptSection{}
	}
	var b strings.Builder
	for _, c := range claims {
		b.WriteString(fmt.Sprintf("Claim %d (%s, scope=%.2f):\n", c.ClaimNumber, c.ClaimType, c.ScopeScore))
		b.WriteString(fmt.Sprintf("  Text: %s\n", c.ClaimText))
		if len(c.TechnicalFeatures) > 0 {
			b.WriteString(fmt.Sprintf("  Features: %s\n", strings.Join(c.TechnicalFeatures, "; ")))
		}
		if len(c.DependsOn) > 0 {
			deps := make([]string, len(c.DependsOn))
			for i, d := range c.DependsOn {
				deps[i] = fmt.Sprintf("%d", d)
			}
			b.WriteString(fmt.Sprintf("  Depends on: %s\n", strings.Join(deps, ", ")))
		}
		b.WriteString("\n")
	}
	text := b.String()
	return promptSection{label: "Claim Analysis", text: text, tokens: pm.EstimateTokenCount(text)}
}

func (pm *promptManagerImpl) buildPriorArtSection(arts []*PriorArtContext) promptSection {
	if len(arts) == 0 {
		return promptSection{}
	}
	var b strings.Builder
	for i, a := range arts {
		b.WriteString(fmt.Sprintf("### Prior Art %d: %s\n", i+1, a.Reference))
		if a.Title != "" {
			b.WriteString(fmt.Sprintf("Title: %s\n", a.Title))
		}
		if a.Abstract != "" {
			b.WriteString(fmt.Sprintf("Abstract: %s\n", a.Abstract))
		}
		b.WriteString(fmt.Sprintf("Relevance: %.2f\n\n", a.Relevance))
	}
	text := b.String()
	return promptSection{label: "Prior Art", text: text, tokens: pm.EstimateTokenCount(text)}
}

func (pm *promptManagerImpl) buildRAGSection(chunks []*RAGChunk) promptSection {
	if len(chunks) == 0 {
		return promptSection{}
	}
	var b strings.Builder
	for i, c := range chunks {
		b.WriteString(fmt.Sprintf("[%d] (score=%.3f, source=%s)\n%s\n\n", i+1, c.Score, c.Source, c.Text))
	}
	text := b.String()
	return promptSection{label: "Retrieved Context (RAG)", text: text, tokens: pm.EstimateTokenCount(text)}
}

// ---------------------------------------------------------------------------
// Instruction block builder
// ---------------------------------------------------------------------------

func (pm *promptManagerImpl) buildInstructionBlock(params *PromptParams) string {
	var parts []string

	// Task-specific instruction.
	parts = append(parts, taskInstruction(params.Task))

	// Output format.
	switch params.OutputFormat {
	case OutputStructured:
		parts = append(parts, "Please provide your analysis in a structured JSON format following the schema appropriate for this task. Include all required fields with proper typing.")
	case OutputNarrative:
		parts = append(parts, "Please provide your analysis in a narrative prose format, with clear paragraphs and logical flow.")
	case OutputBullet:
		parts = append(parts, "Please provide your analysis using bullet points for key findings, organized by topic.")
	}

	// Language.
	switch strings.ToLower(params.Language) {
	case "zh":
		parts = append(parts, "请用中文回答。所有分析、结论和建议均使用中文输出。")
	case "ja":
		parts = append(parts, "日本語で回答してください。")
	case "en":
		parts = append(parts, "Please respond in English.")
	default:
		if params.Language != "" {
			parts = append(parts, fmt.Sprintf("Please respond in %s.", params.Language))
		}
	}

	// Detail level.
	switch params.DetailLevel {
	case DetailSummary:
		parts = append(parts, "Provide a brief summary-level analysis. Focus on key conclusions and high-level risk assessment only.")
	case DetailStandard:
		parts = append(parts, "Provide a standard-depth analysis covering main findings, reasoning, and recommendations.")
	case DetailDetailed:
		parts = append(parts, "Provide a detailed analysis with thorough examination of each element, supporting evidence, and comprehensive recommendations.")
	case DetailExpert:
		parts = append(parts, "Provide an expert-level deep analysis. Include detailed legal reasoning, cite relevant patent law provisions and case law, examine edge cases, and provide nuanced strategic recommendations with risk quantification.")
	}

	// Jurisdiction focus.
	if len(params.JurisdictionFocus) > 0 {
		parts = append(parts, pm.buildJurisdictionInstruction(params.JurisdictionFocus))
	}

	return strings.Join(parts, "\n\n")
}

func taskInstruction(task AnalysisTask) string {
	switch task {
	case TaskFTO:
		return "Analyze whether the target molecule/product can be freely commercialized without infringing existing patents. Identify blocking patents, assess risk levels (HIGH/MEDIUM/LOW), and suggest design-around strategies."
	case TaskInfringementRisk:
		return "Perform a detailed claim-by-claim comparison between the target product and the identified patent claims. For each claim element, determine whether it is literally met or met under the doctrine of equivalents. Provide an overall infringement risk assessment."
	case TaskPatentLandscape:
		return "Provide a comprehensive patent landscape analysis for the technology area. Identify key players, technology trends, white spaces, and strategic opportunities."
	case TaskPortfolioStrategy:
		return "Analyze the patent portfolio and provide strategic recommendations for strengthening IP position, identifying gaps, and optimizing portfolio value."
	case TaskValuation:
		return "Assess the economic value of the patent(s) considering technology relevance, remaining life, claim scope, market size, and licensing potential."
	case TaskClaimDrafting:
		return "Based on the invention description and prior art, draft patent claims that maximize scope while maintaining validity. Include independent and dependent claims with proper claim hierarchy."
	case TaskPriorArtSearch:
		return "Develop a comprehensive prior art search strategy. Identify key search terms, relevant patent classifications (IPC/CPC), target databases, and suggest search queries."
	case TaskOfficeActionResponse:
		return "Analyze the office action and suggest response strategies. Address each rejection/objection with specific claim amendments and arguments."
	default:
		return "Analyze the provided information and give your expert assessment."
	}
}

func (pm *promptManagerImpl) buildJurisdictionInstruction(jurisdictions []string) string {
	if len(jurisdictions) == 0 {
		return ""
	}
	var parts []string
	parts = append(parts, fmt.Sprintf("Focus your analysis on the following jurisdiction(s): %s.", strings.Join(jurisdictions, ", ")))

	for _, j := range jurisdictions {
		switch strings.ToUpper(j) {
		case "US":
			parts = append(parts, "For US analysis: Consider 35 U.S.C. §271 (infringement), the doctrine of equivalents (Warner-Jenkinson), prosecution history estoppel, and Markman claim construction principles.")
		case "CN":
			parts = append(parts, "For CN analysis: Consider Chinese Patent Law Articles 11, 59, 69; the equivalent infringement doctrine under SPC judicial interpretations; and the examination guidelines of CNIPA.")
		case "EP":
			parts = append(parts, "For EP analysis: Consider EPC Articles 52, 54, 56, 69 and the Protocol on Interpretation of Article 69. Consider the problem-solution approach for inventive step.")
		case "JP":
			parts = append(parts, "For JP analysis: Consider Japanese Patent Act Articles 68, 70, and the doctrine of equivalents as established in the Ball Spline Bearing case (Supreme Court, 1998).")
		case "KR":
			parts = append(parts, "For KR analysis: Consider Korean Patent Act and KIPO examination guidelines. Apply the Korean doctrine of equivalents framework.")
		}
	}

	if len(jurisdictions) > 1 {
		parts = append(parts, "Provide a comparative analysis highlighting key differences in patent scope and enforcement across these jurisdictions.")
	}
	return strings.Join(parts, " ")
}

// ---------------------------------------------------------------------------
// Truncation helper
// ---------------------------------------------------------------------------

func truncateToTokens(text string, maxTokens int, pm *promptManagerImpl) string {
	if maxTokens <= 0 {
		return ""
	}
	current := pm.EstimateTokenCount(text)
	if current <= maxTokens {
		return text
	}
	// Binary search for the right rune count.
	runes := []rune(text)
	lo, hi := 0, len(runes)
	best := 0
	for lo <= hi {
		mid := (lo + hi) / 2
		candidate := string(runes[:mid])
		est := pm.EstimateTokenCount(candidate)
		if est <= maxTokens {
			best = mid
			lo = mid + 1
		} else {
			hi = mid - 1
		}
	}
	if best == 0 {
		return ""
	}
	result := string(runes[:best])
	// Try to cut at a sentence or newline boundary for cleanliness.
	if idx := strings.LastIndexAny(result, ".\n。\n"); idx > len(result)/2 {
		result = result[:idx+1]
	}
	return result + "\n[...truncated]"
}

// ---------------------------------------------------------------------------
// System prompt templates (built-in)
// ---------------------------------------------------------------------------

func systemTemplateKey(task AnalysisTask) string {
	return "system_" + task.String()
}

var builtinTemplates = map[string]string{
	// ---- FTO ----
	"system_FTO": `You are a senior patent attorney with 20+ years of experience in pharmaceutical and chemical patent law. You specialize in Freedom-to-Operate (FTO) analysis for drug candidates and chemical compounds.

Your task is to analyze whether a target molecule or product can be freely manufactured, used, and sold without infringing existing patents in the relevant jurisdictions.

When performing FTO analysis, follow this structured approach:
1. **Identify Potentially Blocking Patents**: Review each patent and determine if its claims could cover the target molecule or its use.
2. **Claim Interpretation**: Construe the claim terms according to the applicable jurisdiction's rules (e.g., Phillips v. AWH for US, Article 69 EPC for Europe).
3. **Infringement Analysis**: For each potentially blocking patent, assess literal infringement and infringement under the doctrine of equivalents.
4. **Risk Classification**: Classify each patent as HIGH RISK (likely infringes), MEDIUM RISK (arguable infringement), or LOW RISK (unlikely to infringe).
5. **Design-Around Strategies**: For HIGH and MEDIUM risk patents, suggest structural modifications or alternative approaches that could avoid infringement.
6. **Overall FTO Opinion**: Provide a consolidated risk assessment with clear recommendations.

Always reason step-by-step. Cite specific claim elements when analyzing infringement. Consider patent expiry dates and legal status.`,

	// ---- Infringement Risk ----
	"system_InfringementRisk": `You are a patent litigation expert specializing in pharmaceutical and chemical patent disputes. You have extensive experience in claim construction and infringement analysis.

Your task is to perform a detailed element-by-element comparison between a target product and identified patent claims to assess infringement risk.

Follow this methodology:
1. **Claim Construction**: Parse each claim into its constituent elements/limitations.
2. **Element-by-Element Comparison**: For each claim element, determine:
   a. Whether the target product literally satisfies the element.
   b. If not literally met, whether it is met under the doctrine of equivalents (function-way-result test or insubstantial differences test).
3. **Prosecution History Estoppel**: Check if any equivalents arguments are barred by prosecution history.
4. **All-Limitations Rule**: Remember that ALL elements must be met for infringement; missing even one element means no infringement of that claim.
5. **Dependent Claims**: Analyze dependent claims separately, as they may have narrower scope.
6. **Risk Quantification**: Provide a numerical risk score (0-100) and categorical assessment (HIGH/MEDIUM/LOW/NONE).

Be precise and cite specific structural features when comparing molecular structures to claim language.`,

	// ---- Patent Landscape ----
	"system_PatentLandscape": `You are a patent intelligence analyst specializing in technology landscape mapping for the pharmaceutical and chemical industries.

Your task is to provide a comprehensive patent landscape analysis that reveals the competitive IP environment.

Structure your analysis as follows:
1. **Technology Overview**: Summarize the technology area and its patent activity trends.
2. **Key Players**: Identify top patent holders, their filing strategies, and portfolio strengths.
3. **Technology Clusters**: Group patents by technical approach or mechanism of action.
4. **Temporal Trends**: Analyze filing trends over time to identify emerging vs. mature areas.
5. **Geographic Distribution**: Map patent filings across jurisdictions.
6. **White Space Analysis**: Identify technology areas with low patent density that represent opportunities.
7. **Strategic Implications**: Provide actionable recommendations based on the landscape.

Use quantitative analysis where possible. Identify both threats and opportunities.`,

	// ---- Portfolio Strategy ----
	"system_PortfolioStrategy": `You are a strategic IP advisor with deep expertise in patent portfolio management for pharmaceutical companies.

Analyze the patent portfolio and provide strategic recommendations covering:
1. Portfolio strength assessment and gap analysis
2. Filing strategy optimization
3. Licensing and monetization opportunities
4. Defensive positioning recommendations
5. Portfolio pruning suggestions for cost optimization
6. Competitive benchmarking insights`,

	// ---- Valuation ----
	"system_Valuation": `You are a patent valuation expert with experience in IP transactions, licensing, and litigation damages assessment.

Assess patent value considering:
1. Technology relevance and market applicability
2. Claim scope and enforceability
3. Remaining patent life
4. Market size and revenue potential
5. Licensing comparables
6. Litigation history and outcomes
7. Portfolio synergies

Provide quantitative estimates where possible, with clear assumptions and methodology.`,

	// ---- Claim Drafting ----
	"system_ClaimDrafting": `You are an experienced patent agent (patent attorney) specializing in pharmaceutical and chemical patent prosecution. You have drafted thousands of patent claims and have deep knowledge of claim drafting best practices.

When drafting claims, follow these principles:
1. **Independent Claims**: Draft broad independent claims that capture the essence of the invention while maintaining validity over prior art. Use Markush group notation for chemical compounds where appropriate.
2. **Dependent Claims**: Create a claim tree with progressively narrower dependent claims that provide fallback positions.
3. **Claim Types**: Include composition claims, method-of-use claims, process claims, and formulation claims as appropriate.
4. **Clarity**: Each claim must be a single sentence. Use precise technical language. Define terms consistently.
5. **Support**: Ensure all claim elements have basis in the specification.
6. **Prior Art Avoidance**: Draft claims that clearly distinguish over the identified prior art.

Follow the patent drafting conventions of the target jurisdiction.`,

	// ---- Prior Art Search ----
	"system_PriorArtSearch": `You are a patent search specialist with expertise in pharmaceutical and chemical prior art searching.

Develop a comprehensive search strategy including:
1. Key search concepts and terminology (including synonyms and related terms)
2. Relevant patent classifications (IPC, CPC, USPC)
3. Target databases (patent and non-patent literature)
4. Suggested Boolean search queries
5. Citation analysis approach (forward and backward)
6. Sequence/structure search recommendations for biological inventions`,

	// ---- Office Action Response ----
	"system_OfficeActionResponse": `You are a patent prosecution specialist experienced in responding to patent office actions from major patent offices (USPTO, CNIPA, EPO, JPO, KIPO).

When analyzing an office action and suggesting responses:
1. Identify each rejection or objection raised by the examiner.
2. Analyze the cited prior art references and their relevance.
3. Suggest claim amendments that overcome rejections while maintaining maximum scope.
4. Draft arguments distinguishing the invention from cited prior art.
5. Consider interview strategies with the examiner if appropriate.
6. Assess whether appeal may be warranted for any rejections.

Be strategic: prioritize maintaining broad claim scope while ensuring allowance.`,
}

// ---------------------------------------------------------------------------
// Template function map
// ---------------------------------------------------------------------------

func defaultFuncMap() template.FuncMap {
	return template.FuncMap{
		"join":       strings.Join,
		"upper":      strings.ToUpper,
		"lower":      strings.ToLower,
		"trimSpace":  strings.TrimSpace,
		"contains":   strings.Contains,
		"hasPrefix":  strings.HasPrefix,
		"hasSuffix":  strings.HasSuffix,
		"replace":    strings.ReplaceAll,
		"runeCount":  utf8.RuneCountInString,
		"truncate":   templateTruncate,
		"default":    templateDefault,
		"formatList": templateFormatList,
	}
}

func templateTruncate(maxLen int, s string) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}

func templateDefault(defaultVal, actual string) string {
	if actual == "" {
		return defaultVal
	}
	return actual
}

func templateFormatList(items []string) string {
	if len(items) == 0 {
		return "(none)"
	}
	var b strings.Builder
	for i, item := range items {
		b.WriteString(fmt.Sprintf("  %d. %s\n", i+1, item))
	}
	return b.String()
}

//Personal.AI order the ending

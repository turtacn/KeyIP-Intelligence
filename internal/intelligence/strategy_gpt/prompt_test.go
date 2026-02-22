package strategy_gpt

import (
	"context"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Helper: create a default PromptManager for tests
// ---------------------------------------------------------------------------

func newTestPromptManager(t *testing.T) PromptManager {
	t.Helper()
	pm, err := NewPromptManager(nil)
	if err != nil {
		t.Fatalf("NewPromptManager: %v", err)
	}
	return pm
}

func newTestPromptManagerWithBudget(t *testing.T, maxTokens int) PromptManager {
	t.Helper()
	pm, err := NewPromptManager(&PromptManagerConfig{
		MaxContextTokens: maxTokens,
		DefaultLanguage:  "en",
		TemplateVersion:  "test-v1",
	})
	if err != nil {
		t.Fatalf("NewPromptManager: %v", err)
	}
	return pm
}

func sampleMolecule() *MoleculeContext {
	return &MoleculeContext{
		SMILES:           "CC(=O)Oc1ccccc1C(=O)O",
		Name:             "Aspirin",
		MolecularFormula: "C9H8O4",
		Targets:          []string{"COX-1", "COX-2"},
		Indications:      []string{"Pain", "Inflammation", "Fever"},
		DevelopmentStage: "Marketed",
	}
}

func samplePatents() []*PatentContext {
	return []*PatentContext{
		{
			PatentNumber: "US10000001B2",
			Title:        "Novel COX-2 Selective Inhibitor Compounds",
			Abstract:     "The present invention relates to novel compounds that selectively inhibit cyclooxygenase-2 enzyme activity.",
			KeyClaims:    []string{"A compound of formula (I) wherein R1 is selected from...", "A pharmaceutical composition comprising the compound of claim 1."},
			Applicant:    "Pharma Corp",
			PriorityDate: "2018-01-15",
			LegalStatus:  "Active",
		},
		{
			PatentNumber: "EP3500001A1",
			Title:        "Methods of Treating Inflammatory Conditions",
			Abstract:     "Methods for treating inflammatory conditions using substituted aryl compounds.",
			KeyClaims:    []string{"A method of treating inflammation comprising administering a therapeutically effective amount of..."},
			Applicant:    "BioTech Inc",
			PriorityDate: "2019-03-20",
			LegalStatus:  "Pending",
		},
		{
			PatentNumber: "CN112000001A",
			Title:        "一种新型环氧合酶抑制剂及其制备方法",
			Abstract:     "本发明涉及一种新型环氧合酶抑制剂化合物及其制备方法和医药用途。",
			KeyClaims:    []string{"一种式(I)所示的化合物，其中R1选自..."},
			Applicant:    "中国制药有限公司",
			PriorityDate: "2020-06-01",
			LegalStatus:  "Active",
		},
	}
}

func sampleClaims() []*ClaimAnalysisContext {
	return []*ClaimAnalysisContext{
		{
			ClaimNumber:       1,
			ClaimText:         "A compound of formula (I) wherein R1 is selected from alkyl, aryl, and heteroaryl.",
			ScopeScore:        0.85,
			TechnicalFeatures: []string{"core scaffold", "R1 substituent", "formula (I)"},
			ClaimType:         "independent",
		},
		{
			ClaimNumber:       2,
			ClaimText:         "The compound of claim 1 wherein R1 is phenyl.",
			ScopeScore:        0.60,
			TechnicalFeatures: []string{"phenyl substituent"},
			ClaimType:         "dependent",
			DependsOn:         []int{1},
		},
	}
}

func sampleRAGChunks() []*RAGChunk {
	return []*RAGChunk{
		{
			Text:   "COX-2 selective inhibitors have been extensively studied for their anti-inflammatory properties with reduced gastrointestinal side effects compared to non-selective NSAIDs.",
			Source: "review_2023.pdf",
			Score:  0.92,
		},
		{
			Text:   "The Markush structure in patent claims allows broad coverage of chemical compound families through variable group definitions.",
			Source: "patent_law_guide.pdf",
			Score:  0.85,
		},
	}
}

func longRAGChunks(n int) []*RAGChunk {
	chunks := make([]*RAGChunk, n)
	longText := strings.Repeat("This is a very long RAG chunk containing detailed technical information about pharmaceutical compounds and their patent implications. ", 20)
	for i := 0; i < n; i++ {
		chunks[i] = &RAGChunk{
			Text:   longText,
			Source: "large_doc.pdf",
			Score:  0.80,
		}
	}
	return chunks
}

// ---------------------------------------------------------------------------
// FTO tests
// ---------------------------------------------------------------------------

func TestBuildPrompt_FTO_Basic(t *testing.T) {
	pm := newTestPromptManager(t)
	params := &PromptParams{
		TargetMolecule:  sampleMolecule(),
		RelevantPatents: samplePatents(),
		UserQuery:       "Can we commercialize this compound without patent issues?",
		OutputFormat:    OutputStructured,
		Language:        "en",
		DetailLevel:     DetailStandard,
	}
	built, err := pm.BuildPrompt(context.Background(), TaskFTO, params)
	if err != nil {
		t.Fatalf("BuildPrompt: %v", err)
	}
	if !strings.Contains(built.SystemPrompt, "Freedom-to-Operate") && !strings.Contains(built.SystemPrompt, "FTO") {
		t.Error("system prompt should mention Freedom-to-Operate or FTO")
	}
	if !strings.Contains(built.UserPrompt, "Aspirin") {
		t.Error("user prompt should contain molecule name")
	}
	if !strings.Contains(built.UserPrompt, "US10000001B2") {
		t.Error("user prompt should contain patent number")
	}
	if !strings.Contains(built.UserPrompt, "Can we commercialize") {
		t.Error("user prompt should contain user query")
	}
	if built.EstimatedTokens <= 0 {
		t.Error("estimated tokens should be positive")
	}
	if len(built.Messages) != 2 {
		t.Errorf("expected 2 messages, got %d", len(built.Messages))
	}
	if built.Messages[0].Role != "system" {
		t.Errorf("first message role should be system, got %s", built.Messages[0].Role)
	}
	if built.Messages[1].Role != "user" {
		t.Errorf("second message role should be user, got %s", built.Messages[1].Role)
	}
}

func TestBuildPrompt_FTO_WithRAG(t *testing.T) {
	pm := newTestPromptManager(t)
	params := &PromptParams{
		TargetMolecule:  sampleMolecule(),
		RelevantPatents: samplePatents(),
		RAGContext:      sampleRAGChunks(),
		UserQuery:       "FTO analysis for aspirin analog",
		Language:        "en",
	}
	built, err := pm.BuildPrompt(context.Background(), TaskFTO, params)
	if err != nil {
		t.Fatalf("BuildPrompt: %v", err)
	}
	if !strings.Contains(built.UserPrompt, "RAG") || !strings.Contains(built.UserPrompt, "COX-2 selective") {
		t.Error("user prompt should contain RAG context")
	}
}

// ---------------------------------------------------------------------------
// Infringement Risk tests
// ---------------------------------------------------------------------------

func TestBuildPrompt_InfringementRisk(t *testing.T) {
	pm := newTestPromptManager(t)
	params := &PromptParams{
		TargetMolecule:  sampleMolecule(),
		RelevantPatents: samplePatents()[:1],
		ClaimAnalysis:   sampleClaims(),
		UserQuery:       "Assess infringement risk for our compound",
		Language:        "en",
	}
	built, err := pm.BuildPrompt(context.Background(), TaskInfringementRisk, params)
	if err != nil {
		t.Fatalf("BuildPrompt: %v", err)
	}
	if !strings.Contains(built.SystemPrompt, "element-by-element") {
		t.Error("system prompt should mention element-by-element comparison")
	}
	if !strings.Contains(built.UserPrompt, "Claim 1") {
		t.Error("user prompt should contain claim analysis")
	}
}

// ---------------------------------------------------------------------------
// Patent Landscape tests
// ---------------------------------------------------------------------------

func TestBuildPrompt_PatentLandscape(t *testing.T) {
	pm := newTestPromptManager(t)
	params := &PromptParams{
		RelevantPatents: samplePatents(),
		UserQuery:       "Map the COX inhibitor patent landscape",
		Language:        "en",
	}
	built, err := pm.BuildPrompt(context.Background(), TaskPatentLandscape, params)
	if err != nil {
		t.Fatalf("BuildPrompt: %v", err)
	}
	if !strings.Contains(built.SystemPrompt, "landscape") {
		t.Error("system prompt should mention landscape")
	}
}

// ---------------------------------------------------------------------------
// Claim Drafting tests
// ---------------------------------------------------------------------------

func TestBuildPrompt_ClaimDrafting(t *testing.T) {
	pm := newTestPromptManager(t)
	params := &PromptParams{
		TargetMolecule: sampleMolecule(),
		UserQuery:      "Draft claims for this novel compound",
		Language:       "en",
	}
	built, err := pm.BuildPrompt(context.Background(), TaskClaimDrafting, params)
	if err != nil {
		t.Fatalf("BuildPrompt: %v", err)
	}
	if !strings.Contains(built.SystemPrompt, "patent agent") && !strings.Contains(built.SystemPrompt, "patent attorney") {
		t.Error("system prompt should set role as patent agent/attorney")
	}
	if !strings.Contains(built.SystemPrompt, "Markush") {
		t.Error("system prompt should mention Markush notation")
	}
}

// ---------------------------------------------------------------------------
// Token budget tests
// ---------------------------------------------------------------------------

func TestBuildPrompt_TokenBudget_NoTruncation(t *testing.T) {
	pm := newTestPromptManagerWithBudget(t, 50000)
	params := &PromptParams{
		TargetMolecule:  sampleMolecule(),
		RelevantPatents: samplePatents()[:1],
		UserQuery:       "Quick FTO check",
		Language:        "en",
	}
	built, err := pm.BuildPrompt(context.Background(), TaskFTO, params)
	if err != nil {
		t.Fatalf("BuildPrompt: %v", err)
	}
	if built.TruncationApplied {
		t.Error("truncation should not be applied with large budget")
	}
}

func TestBuildPrompt_TokenBudget_TruncateRAG(t *testing.T) {
	pm := newTestPromptManagerWithBudget(t, 1500)
	params := &PromptParams{
		TargetMolecule:  sampleMolecule(),
		RelevantPatents: samplePatents()[:1],
		RAGContext:      longRAGChunks(10),
		UserQuery:       "FTO analysis",
		Language:        "en",
	}
	built, err := pm.BuildPrompt(context.Background(), TaskFTO, params)
	if err != nil {
		t.Fatalf("BuildPrompt: %v", err)
	}
	if !built.TruncationApplied {
		t.Error("truncation should be applied with small budget and large RAG")
	}
	// Molecule info should still be present (highest priority).
	if !strings.Contains(built.UserPrompt, "Aspirin") {
		t.Error("molecule info should survive truncation")
	}
}

func TestBuildPrompt_TokenBudget_TruncatePatents(t *testing.T) {
	// Very tight budget: even patents may be truncated.
	pm := newTestPromptManagerWithBudget(t, 800)
	manyPatents := make([]*PatentContext, 20)
	longAbstract := strings.Repeat("This patent describes a novel pharmaceutical compound with unique properties. ", 10)
	for i := 0; i < 20; i++ {
		manyPatents[i] = &PatentContext{
			PatentNumber: "US" + strings.Repeat("0", 7-len(string(rune('0'+i)))) + string(rune('0'+i)),
			Title:        "Patent " + string(rune('A'+i)),
			Abstract:     longAbstract,
			Applicant:    "Corp " + string(rune('A'+i)),
		}
	}
	params := &PromptParams{
		TargetMolecule:  sampleMolecule(),
		RelevantPatents: manyPatents,
		UserQuery:       "Analyze patents",
		Language:        "en",
	}
	built, err := pm.BuildPrompt(context.Background(), TaskFTO, params)
	if err != nil {
		t.Fatalf("BuildPrompt: %v", err)
	}
	if !built.TruncationApplied {
		t.Error("truncation should be applied with tight budget and many patents")
	}
}

func TestBuildPrompt_TokenBudget_UserQueryPreserved(t *testing.T) {
	pm := newTestPromptManagerWithBudget(t, 500)
	userQuery := "This is my very important query that must never be truncated regardless of budget constraints."
	params := &PromptParams{
		TargetMolecule:  sampleMolecule(),
		RelevantPatents: samplePatents(),
		RAGContext:      longRAGChunks(5),
		UserQuery:       userQuery,
		Language:        "en",
	}
	built, err := pm.BuildPrompt(context.Background(), TaskFTO, params)
	if err != nil {
		t.Fatalf("BuildPrompt: %v", err)
	}
	if !strings.Contains(built.UserPrompt, userQuery) {
		t.Error("user query must never be truncated")
	}
}

// ---------------------------------------------------------------------------
// Output format tests
// ---------------------------------------------------------------------------

func TestBuildPrompt_OutputFormat_Structured(t *testing.T) {
	pm := newTestPromptManager(t)
	params := &PromptParams{
		UserQuery:    "Analyze",
		OutputFormat: OutputStructured,
		Language:     "en",
	}
	built, err := pm.BuildPrompt(context.Background(), TaskFTO, params)
	if err != nil {
		t.Fatalf("BuildPrompt: %v", err)
	}
	if !strings.Contains(built.UserPrompt, "structured JSON") {
		t.Error("structured format should mention JSON")
	}
}

func TestBuildPrompt_OutputFormat_Narrative(t *testing.T) {
	pm := newTestPromptManager(t)
	params := &PromptParams{
		UserQuery:    "Analyze",
		OutputFormat: OutputNarrative,
		Language:     "en",
	}
	built, err := pm.BuildPrompt(context.Background(), TaskFTO, params)
	if err != nil {
		t.Fatalf("BuildPrompt: %v", err)
	}
	if !strings.Contains(built.UserPrompt, "narrative") {
		t.Error("narrative format should mention narrative")
	}
}

// ---------------------------------------------------------------------------
// Language tests
// ---------------------------------------------------------------------------

func TestBuildPrompt_Language_Chinese(t *testing.T) {
	pm := newTestPromptManager(t)
	params := &PromptParams{
		UserQuery: "分析此化合物的FTO风险",
		Language:  "zh",
	}
	built, err := pm.BuildPrompt(context.Background(), TaskFTO, params)
	if err != nil {
		t.Fatalf("BuildPrompt: %v", err)
	}
	if !strings.Contains(built.UserPrompt, "请用中文回答") {
		t.Error("Chinese language should include '请用中文回答'")
	}
}

func TestBuildPrompt_Language_English(t *testing.T) {
	pm := newTestPromptManager(t)
	params := &PromptParams{
		UserQuery: "Analyze FTO risk",
		Language:  "en",
	}
	built, err := pm.BuildPrompt(context.Background(), TaskFTO, params)
	if err != nil {
		t.Fatalf("BuildPrompt: %v", err)
	}
	if !strings.Contains(built.UserPrompt, "Please respond in English") {
		t.Error("English language should include 'Please respond in English'")
	}
}

// ---------------------------------------------------------------------------
// Detail level tests
// ---------------------------------------------------------------------------

func TestBuildPrompt_DetailLevel_Summary(t *testing.T) {
	pm := newTestPromptManager(t)
	params := &PromptParams{
		UserQuery:   "Quick check",
		Language:    "en",
		DetailLevel: DetailSummary,
	}
	built, err := pm.BuildPrompt(context.Background(), TaskFTO, params)
	if err != nil {
		t.Fatalf("BuildPrompt: %v", err)
	}
	if !strings.Contains(built.UserPrompt, "brief summary") && !strings.Contains(built.UserPrompt, "summary-level") {
		t.Error("summary detail level should mention brief/summary")
	}
}

func TestBuildPrompt_DetailLevel_Expert(t *testing.T) {
	pm := newTestPromptManager(t)
	params := &PromptParams{
		UserQuery:   "Deep analysis",
		Language:    "en",
		DetailLevel: DetailExpert,
	}
	built, err := pm.BuildPrompt(context.Background(), TaskFTO, params)
	if err != nil {
		t.Fatalf("BuildPrompt: %v", err)
	}
	if !strings.Contains(built.UserPrompt, "expert-level") {
		t.Error("expert detail level should mention expert-level")
	}
}

// ---------------------------------------------------------------------------
// Jurisdiction tests
// ---------------------------------------------------------------------------

func TestBuildPrompt_JurisdictionFocus_US(t *testing.T) {
	pm := newTestPromptManager(t)
	params := &PromptParams{
		UserQuery:         "US FTO analysis",
		Language:          "en",
		JurisdictionFocus: []string{"US"},
	}
	built, err := pm.BuildPrompt(context.Background(), TaskFTO, params)
	if err != nil {
		t.Fatalf("BuildPrompt: %v", err)
	}
	if !strings.Contains(built.UserPrompt, "35 U.S.C.") {
		t.Error("US jurisdiction should reference 35 U.S.C.")
	}
}

func TestBuildPrompt_JurisdictionFocus_CN(t *testing.T) {
	pm := newTestPromptManager(t)
	params := &PromptParams{
		UserQuery:         "中国专利分析",
		Language:          "en",
		JurisdictionFocus: []string{"CN"},
	}
	built, err := pm.BuildPrompt(context.Background(), TaskFTO, params)
	if err != nil {
		t.Fatalf("BuildPrompt: %v", err)
	}
	if !strings.Contains(built.UserPrompt, "Chinese Patent Law") {
		t.Error("CN jurisdiction should reference Chinese Patent Law")
	}
}

func TestBuildPrompt_MultipleJurisdictions(t *testing.T) {
	pm := newTestPromptManager(t)
	params := &PromptParams{
		UserQuery:         "Multi-jurisdiction analysis",
		Language:          "en",
		JurisdictionFocus: []string{"US", "CN", "EP"},
	}
	built, err := pm.BuildPrompt(context.Background(), TaskFTO, params)
	if err != nil {
		t.Fatalf("BuildPrompt: %v", err)
	}
	if !strings.Contains(built.UserPrompt, "comparative analysis") {
		t.Error("multiple jurisdictions should trigger comparative analysis instruction")
	}
	if !strings.Contains(built.UserPrompt, "35 U.S.C.") {
		t.Error("should include US law reference")
	}
	if !strings.Contains(built.UserPrompt, "Chinese Patent Law") {
		t.Error("should include CN law reference")
	}
	if !strings.Contains(built.UserPrompt, "EPC") {
		t.Error("should include EP law reference")
	}
}

// ---------------------------------------------------------------------------
// Empty context tests
// ---------------------------------------------------------------------------

func TestBuildPrompt_EmptyContext(t *testing.T) {
	pm := newTestPromptManager(t)
	params := &PromptParams{
		UserQuery: "General patent question",
		Language:  "en",
	}
	built, err := pm.BuildPrompt(context.Background(), TaskFTO, params)
	if err != nil {
		t.Fatalf("BuildPrompt: %v", err)
	}
	if built.SystemPrompt == "" {
		t.Error("system prompt should not be empty")
	}
	if !strings.Contains(built.UserPrompt, "General patent question") {
		t.Error("user query should be present even with empty context")
	}
	// Should not contain section headers for missing data.
	if strings.Contains(built.UserPrompt, "## Molecule Information") {
		t.Error("should not have molecule section when no molecule provided")
	}
	if strings.Contains(built.UserPrompt, "## Relevant Patents") {
		t.Error("should not have patent section when no patents provided")
	}
}
// ---------------------------------------------------------------------------
// Messages structure tests
// ---------------------------------------------------------------------------

func TestBuildPrompt_Messages(t *testing.T) {
	pm := newTestPromptManager(t)
	params := &PromptParams{
		TargetMolecule: sampleMolecule(),
		UserQuery:      "Analyze",
		Language:       "en",
	}
	built, err := pm.BuildPrompt(context.Background(), TaskFTO, params)
	if err != nil {
		t.Fatalf("BuildPrompt: %v", err)
	}
	if len(built.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(built.Messages))
	}
	if built.Messages[0].Role != "system" {
		t.Errorf("messages[0].Role = %q, want system", built.Messages[0].Role)
	}
	if built.Messages[0].Content != built.SystemPrompt {
		t.Error("messages[0].Content should equal SystemPrompt")
	}
	if built.Messages[1].Role != "user" {
		t.Errorf("messages[1].Role = %q, want user", built.Messages[1].Role)
	}
	if built.Messages[1].Content != built.UserPrompt {
		t.Error("messages[1].Content should equal UserPrompt")
	}
}

// ---------------------------------------------------------------------------
// GetSystemPrompt tests
// ---------------------------------------------------------------------------

func TestGetSystemPrompt_AllTasks(t *testing.T) {
	pm := newTestPromptManager(t)
	tasks := []AnalysisTask{
		TaskFTO,
		TaskInfringementRisk,
		TaskPatentLandscape,
		TaskPortfolioStrategy,
		TaskValuation,
		TaskClaimDrafting,
		TaskPriorArtSearch,
		TaskOfficeActionResponse,
	}
	for _, task := range tasks {
		prompt, err := pm.GetSystemPrompt(task)
		if err != nil {
			t.Errorf("GetSystemPrompt(%s): %v", task, err)
			continue
		}
		if prompt == "" {
			t.Errorf("GetSystemPrompt(%s) returned empty string", task)
		}
		if len(prompt) < 100 {
			t.Errorf("GetSystemPrompt(%s) suspiciously short (%d chars)", task, len(prompt))
		}
	}
}

func TestGetSystemPrompt_UnknownTask(t *testing.T) {
	pm := newTestPromptManager(t)
	_, err := pm.GetSystemPrompt(AnalysisTask(999))
	if err == nil {
		t.Fatal("expected error for unknown task")
	}
}

// ---------------------------------------------------------------------------
// RenderTemplate tests
// ---------------------------------------------------------------------------

func TestRenderTemplate_Variables(t *testing.T) {
	pm := newTestPromptManager(t)
	err := pm.RegisterTemplate("test_vars", "Hello {{.Name}}, your SMILES is {{.SMILES}}.")
	if err != nil {
		t.Fatalf("RegisterTemplate: %v", err)
	}
	data := struct {
		Name   string
		SMILES string
	}{
		Name:   "Aspirin",
		SMILES: "CC(=O)Oc1ccccc1C(=O)O",
	}
	result, err := pm.RenderTemplate("test_vars", data)
	if err != nil {
		t.Fatalf("RenderTemplate: %v", err)
	}
	if !strings.Contains(result, "Aspirin") {
		t.Error("rendered template should contain Name")
	}
	if !strings.Contains(result, "CC(=O)Oc1ccccc1C(=O)O") {
		t.Error("rendered template should contain SMILES")
	}
}

func TestRenderTemplate_Conditional(t *testing.T) {
	pm := newTestPromptManager(t)
	tmpl := `{{if .HasPatents}}Patents found.{{else}}No patents.{{end}}`
	err := pm.RegisterTemplate("test_cond", tmpl)
	if err != nil {
		t.Fatalf("RegisterTemplate: %v", err)
	}

	// True branch
	result, err := pm.RenderTemplate("test_cond", struct{ HasPatents bool }{true})
	if err != nil {
		t.Fatalf("RenderTemplate: %v", err)
	}
	if result != "Patents found." {
		t.Errorf("expected 'Patents found.', got %q", result)
	}

	// False branch
	result, err = pm.RenderTemplate("test_cond", struct{ HasPatents bool }{false})
	if err != nil {
		t.Fatalf("RenderTemplate: %v", err)
	}
	if result != "No patents." {
		t.Errorf("expected 'No patents.', got %q", result)
	}
}

func TestRenderTemplate_Loop(t *testing.T) {
	pm := newTestPromptManager(t)
	tmpl := `{{range $i, $v := .Items}}{{$i}}:{{$v}} {{end}}`
	err := pm.RegisterTemplate("test_loop", tmpl)
	if err != nil {
		t.Fatalf("RegisterTemplate: %v", err)
	}
	data := struct {
		Items []string
	}{
		Items: []string{"alpha", "beta", "gamma"},
	}
	result, err := pm.RenderTemplate("test_loop", data)
	if err != nil {
		t.Fatalf("RenderTemplate: %v", err)
	}
	if !strings.Contains(result, "0:alpha") {
		t.Error("loop should render first item")
	}
	if !strings.Contains(result, "1:beta") {
		t.Error("loop should render second item")
	}
	if !strings.Contains(result, "2:gamma") {
		t.Error("loop should render third item")
	}
}

func TestRenderTemplate_MissingVariable(t *testing.T) {
	pm := newTestPromptManager(t)
	// Go templates with missing keys produce <no value> by default or error depending on option.
	// Our implementation uses default template behavior.
	tmpl := `Value: {{.Missing}}`
	err := pm.RegisterTemplate("test_missing", tmpl)
	if err != nil {
		t.Fatalf("RegisterTemplate: %v", err)
	}
	data := struct{}{}
	_, err = pm.RenderTemplate("test_missing", data)
	// Go template will error on missing field in a struct.
	if err == nil {
		t.Log("note: missing variable may produce empty or error depending on data type")
	}
}

func TestRenderTemplate_NotFound(t *testing.T) {
	pm := newTestPromptManager(t)
	_, err := pm.RenderTemplate("nonexistent_template", nil)
	if err == nil {
		t.Fatal("expected error for nonexistent template")
	}
}

// ---------------------------------------------------------------------------
// RegisterTemplate tests
// ---------------------------------------------------------------------------

func TestRegisterTemplate_Success(t *testing.T) {
	pm := newTestPromptManager(t)
	err := pm.RegisterTemplate("custom_tmpl", "Hello {{.Name}}")
	if err != nil {
		t.Fatalf("RegisterTemplate: %v", err)
	}
	result, err := pm.RenderTemplate("custom_tmpl", struct{ Name string }{"World"})
	if err != nil {
		t.Fatalf("RenderTemplate: %v", err)
	}
	if result != "Hello World" {
		t.Errorf("expected 'Hello World', got %q", result)
	}
}

func TestRegisterTemplate_Duplicate(t *testing.T) {
	pm := newTestPromptManager(t)
	err := pm.RegisterTemplate("dup_tmpl", "Version 1: {{.X}}")
	if err != nil {
		t.Fatalf("first RegisterTemplate: %v", err)
	}
	// Overwrite with new version.
	err = pm.RegisterTemplate("dup_tmpl", "Version 2: {{.X}}")
	if err != nil {
		t.Fatalf("second RegisterTemplate: %v", err)
	}
	result, err := pm.RenderTemplate("dup_tmpl", struct{ X string }{"test"})
	if err != nil {
		t.Fatalf("RenderTemplate: %v", err)
	}
	if !strings.Contains(result, "Version 2") {
		t.Error("duplicate registration should overwrite old template")
	}
}

func TestRegisterTemplate_EmptyName(t *testing.T) {
	pm := newTestPromptManager(t)
	err := pm.RegisterTemplate("", "body")
	if err == nil {
		t.Fatal("expected error for empty template name")
	}
}

func TestRegisterTemplate_EmptyBody(t *testing.T) {
	pm := newTestPromptManager(t)
	err := pm.RegisterTemplate("empty_body", "")
	if err == nil {
		t.Fatal("expected error for empty template body")
	}
}

func TestRegisterTemplate_InvalidSyntax(t *testing.T) {
	pm := newTestPromptManager(t)
	err := pm.RegisterTemplate("bad_syntax", "{{.Unclosed")
	if err == nil {
		t.Fatal("expected error for invalid template syntax")
	}
}

// ---------------------------------------------------------------------------
// ListTemplates tests
// ---------------------------------------------------------------------------

func TestListTemplates(t *testing.T) {
	pm := newTestPromptManager(t)
	templates := pm.ListTemplates()
	if len(templates) == 0 {
		t.Fatal("expected at least built-in templates")
	}
	// Should have at least 8 system templates (one per task).
	systemCount := 0
	for _, ti := range templates {
		if strings.HasPrefix(ti.Name, "system_") {
			systemCount++
		}
	}
	if systemCount < 8 {
		t.Errorf("expected at least 8 system templates, got %d", systemCount)
	}
	// Check that each template has a version.
	for _, ti := range templates {
		if ti.Version == "" {
			t.Errorf("template %q has empty version", ti.Name)
		}
		if ti.RegisteredAt.IsZero() {
			t.Errorf("template %q has zero registered_at", ti.Name)
		}
	}
}

func TestListTemplates_AfterRegister(t *testing.T) {
	pm := newTestPromptManager(t)
	before := len(pm.ListTemplates())
	_ = pm.RegisterTemplate("extra_tmpl", "extra")
	after := len(pm.ListTemplates())
	if after != before+1 {
		t.Errorf("expected %d templates after register, got %d", before+1, after)
	}
}

// ---------------------------------------------------------------------------
// EstimateTokenCount tests
// ---------------------------------------------------------------------------

func TestEstimateTokenCount_English(t *testing.T) {
	pm := newTestPromptManager(t)
	// 100 English characters ≈ 25 tokens (100 * 0.25)
	text := strings.Repeat("abcd ", 20) // 100 chars
	tokens := pm.EstimateTokenCount(text)
	if tokens < 15 || tokens > 40 {
		t.Errorf("English 100 chars: expected ~25 tokens, got %d", tokens)
	}
}

func TestEstimateTokenCount_Chinese(t *testing.T) {
	pm := newTestPromptManager(t)
	// 50 Chinese characters ≈ 33 tokens (50 * 0.67)
	text := strings.Repeat("专利分析化合物侵权风险评估", 5) // ~60 CJK chars
	tokens := pm.EstimateTokenCount(text)
	runeCount := 0
	for range text {
		runeCount++
	}
	expectedLow := int(float64(runeCount) * 0.5)
	expectedHigh := int(float64(runeCount) * 0.9)
	if tokens < expectedLow || tokens > expectedHigh {
		t.Errorf("Chinese %d runes: expected %d-%d tokens, got %d", runeCount, expectedLow, expectedHigh, tokens)
	}
}

func TestEstimateTokenCount_Mixed(t *testing.T) {
	pm := newTestPromptManager(t)
	text := "Patent analysis 专利分析 for compound 化合物"
	tokens := pm.EstimateTokenCount(text)
	if tokens <= 0 {
		t.Error("mixed text should have positive token count")
	}
	// Should be between pure English and pure Chinese estimates.
	pureEnglishEstimate := len(text) / 4
	if tokens < pureEnglishEstimate/2 {
		t.Errorf("mixed text tokens (%d) suspiciously low", tokens)
	}
}

func TestEstimateTokenCount_Empty(t *testing.T) {
	pm := newTestPromptManager(t)
	tokens := pm.EstimateTokenCount("")
	if tokens != 0 {
		t.Errorf("empty text should have 0 tokens, got %d", tokens)
	}
}

func TestEstimateTokenCount_SingleChar(t *testing.T) {
	pm := newTestPromptManager(t)
	tokens := pm.EstimateTokenCount("a")
	if tokens < 1 {
		t.Errorf("single char should have at least 1 token, got %d", tokens)
	}
}

func TestEstimateTokenCount_SingleChinese(t *testing.T) {
	pm := newTestPromptManager(t)
	tokens := pm.EstimateTokenCount("专")
	if tokens < 1 {
		t.Errorf("single Chinese char should have at least 1 token, got %d", tokens)
	}
}

// ---------------------------------------------------------------------------
// AnalysisTask tests
// ---------------------------------------------------------------------------

func TestAnalysisTask_String(t *testing.T) {
	tests := []struct {
		task AnalysisTask
		want string
	}{
		{TaskFTO, "FTO"},
		{TaskInfringementRisk, "InfringementRisk"},
		{TaskPatentLandscape, "PatentLandscape"},
		{TaskPortfolioStrategy, "PortfolioStrategy"},
		{TaskValuation, "Valuation"},
		{TaskClaimDrafting, "ClaimDrafting"},
		{TaskPriorArtSearch, "PriorArtSearch"},
		{TaskOfficeActionResponse, "OfficeActionResponse"},
	}
	for _, tt := range tests {
		got := tt.task.String()
		if got != tt.want {
			t.Errorf("AnalysisTask(%d).String() = %q, want %q", int(tt.task), got, tt.want)
		}
	}
}

func TestAnalysisTask_String_Unknown(t *testing.T) {
	got := AnalysisTask(999).String()
	if !strings.Contains(got, "Unknown") {
		t.Errorf("unknown task String() = %q, should contain 'Unknown'", got)
	}
}

func TestAnalysisTask_IsValid(t *testing.T) {
	validTasks := []AnalysisTask{
		TaskFTO, TaskInfringementRisk, TaskPatentLandscape,
		TaskPortfolioStrategy, TaskValuation, TaskClaimDrafting,
		TaskPriorArtSearch, TaskOfficeActionResponse,
	}
	for _, task := range validTasks {
		if !task.IsValid() {
			t.Errorf("task %s should be valid", task)
		}
	}
	if AnalysisTask(999).IsValid() {
		t.Error("task 999 should not be valid")
	}
}

// ---------------------------------------------------------------------------
// OutputFormat / DetailLevel String tests
// ---------------------------------------------------------------------------

func TestOutputFormat_String(t *testing.T) {
	tests := []struct {
		f    OutputFormat
		want string
	}{
		{OutputStructured, "Structured"},
		{OutputNarrative, "Narrative"},
		{OutputBullet, "Bullet"},
		{OutputFormat(99), "Unknown"},
	}
	for _, tt := range tests {
		if got := tt.f.String(); got != tt.want {
			t.Errorf("OutputFormat(%d).String() = %q, want %q", int(tt.f), got, tt.want)
		}
	}
}

func TestDetailLevel_String(t *testing.T) {
	tests := []struct {
		d    DetailLevel
		want string
	}{
		{DetailSummary, "Summary"},
		{DetailStandard, "Standard"},
		{DetailDetailed, "Detailed"},
		{DetailExpert, "Expert"},
		{DetailLevel(99), "Unknown"},
	}
	for _, tt := range tests {
		if got := tt.d.String(); got != tt.want {
			t.Errorf("DetailLevel(%d).String() = %q, want %q", int(tt.d), got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// BuildPrompt edge cases
// ---------------------------------------------------------------------------

func TestBuildPrompt_InvalidTask(t *testing.T) {
	pm := newTestPromptManager(t)
	_, err := pm.BuildPrompt(context.Background(), AnalysisTask(999), nil)
	if err == nil {
		t.Fatal("expected error for invalid task")
	}
}

func TestBuildPrompt_NilParams(t *testing.T) {
	pm := newTestPromptManager(t)
	built, err := pm.BuildPrompt(context.Background(), TaskFTO, nil)
	if err != nil {
		t.Fatalf("BuildPrompt with nil params: %v", err)
	}
	if built.SystemPrompt == "" {
		t.Error("system prompt should not be empty even with nil params")
	}
	if len(built.Messages) != 2 {
		t.Errorf("expected 2 messages, got %d", len(built.Messages))
	}
}

func TestBuildPrompt_DefaultLanguage(t *testing.T) {
	pm, err := NewPromptManager(&PromptManagerConfig{
		MaxContextTokens: 12000,
		DefaultLanguage:  "zh",
		TemplateVersion:  "test",
	})
	if err != nil {
		t.Fatalf("NewPromptManager: %v", err)
	}
	params := &PromptParams{
		UserQuery: "分析",
		// Language intentionally left empty to test default.
	}
	built, err := pm.BuildPrompt(context.Background(), TaskFTO, params)
	if err != nil {
		t.Fatalf("BuildPrompt: %v", err)
	}
	if !strings.Contains(built.UserPrompt, "请用中文回答") {
		t.Error("default language zh should produce Chinese instruction")
	}
}

func TestBuildPrompt_TemplateVersion(t *testing.T) {
	pm, err := NewPromptManager(&PromptManagerConfig{
		MaxContextTokens: 12000,
		DefaultLanguage:  "en",
		TemplateVersion:  "v2.5-beta",
	})
	if err != nil {
		t.Fatalf("NewPromptManager: %v", err)
	}
	built, err := pm.BuildPrompt(context.Background(), TaskFTO, &PromptParams{UserQuery: "test"})
	if err != nil {
		t.Fatalf("BuildPrompt: %v", err)
	}
	if built.TemplateVersion != "v2.5-beta" {
		t.Errorf("expected template version v2.5-beta, got %s", built.TemplateVersion)
	}
}

func TestBuildPrompt_PriorArtSection(t *testing.T) {
	pm := newTestPromptManager(t)
	params := &PromptParams{
		PriorArt: []*PriorArtContext{
			{
				Reference:   "WO2020/123456",
				Title:       "Prior Art Compound",
				Abstract:    "A compound with similar structure.",
				Relevance:   0.88,
				PublishDate: "2020-01-15",
			},
		},
		UserQuery: "Check prior art",
		Language:  "en",
	}
	built, err := pm.BuildPrompt(context.Background(), TaskPriorArtSearch, params)
	if err != nil {
		t.Fatalf("BuildPrompt: %v", err)
	}
	if !strings.Contains(built.UserPrompt, "WO2020/123456") {
		t.Error("user prompt should contain prior art reference")
	}
	if !strings.Contains(built.UserPrompt, "Prior Art") {
		t.Error("user prompt should contain Prior Art section")
	}
}

func TestBuildPrompt_ClaimAnalysisSection(t *testing.T) {
	pm := newTestPromptManager(t)
	params := &PromptParams{
		ClaimAnalysis: sampleClaims(),
		UserQuery:     "Analyze claims",
		Language:      "en",
	}
	built, err := pm.BuildPrompt(context.Background(), TaskInfringementRisk, params)
	if err != nil {
		t.Fatalf("BuildPrompt: %v", err)
	}
	if !strings.Contains(built.UserPrompt, "Claim 1") {
		t.Error("user prompt should contain claim number")
	}
	if !strings.Contains(built.UserPrompt, "independent") {
		t.Error("user prompt should contain claim type")
	}
	if !strings.Contains(built.UserPrompt, "core scaffold") {
		t.Error("user prompt should contain technical features")
	}
}

// ---------------------------------------------------------------------------
// NewPromptManager tests
// ---------------------------------------------------------------------------

func TestNewPromptManager_NilConfig(t *testing.T) {
	pm, err := NewPromptManager(nil)
	if err != nil {
		t.Fatalf("NewPromptManager(nil): %v", err)
	}
	if pm == nil {
		t.Fatal("expected non-nil PromptManager")
	}
	// Should use defaults.
	templates := pm.ListTemplates()
	if len(templates) == 0 {
		t.Error("should have built-in templates with nil config")
	}
}

func TestNewPromptManager_ZeroMaxTokens(t *testing.T) {
	pm, err := NewPromptManager(&PromptManagerConfig{
		MaxContextTokens: 0,
		DefaultLanguage:  "en",
		TemplateVersion:  "v1",
	})
	if err != nil {
		t.Fatalf("NewPromptManager: %v", err)
	}
	// Should default to 12000.
	built, err := pm.BuildPrompt(context.Background(), TaskFTO, &PromptParams{UserQuery: "test"})
	if err != nil {
		t.Fatalf("BuildPrompt: %v", err)
	}
	if built == nil {
		t.Fatal("expected non-nil built prompt")
	}
}

// ---------------------------------------------------------------------------
// Template function tests
// ---------------------------------------------------------------------------

func TestTemplateFuncMap_Truncate(t *testing.T) {
	pm := newTestPromptManager(t)
	err := pm.RegisterTemplate("func_truncate", `{{truncate 5 .Text}}`)
	if err != nil {
		t.Fatalf("RegisterTemplate: %v", err)
	}
	result, err := pm.RenderTemplate("func_truncate", struct{ Text string }{"Hello World"})
	if err != nil {
		t.Fatalf("RenderTemplate: %v", err)
	}
	if result != "Hello..." {
		t.Errorf("expected 'Hello...', got %q", result)
	}
}

func TestTemplateFuncMap_Default(t *testing.T) {
	pm := newTestPromptManager(t)
	err := pm.RegisterTemplate("func_default", `{{default "N/A" .Value}}`)
	if err != nil {
		t.Fatalf("RegisterTemplate: %v", err)
	}
	// Empty value → default.
	result, err := pm.RenderTemplate("func_default", struct{ Value string }{""})
	if err != nil {
		t.Fatalf("RenderTemplate: %v", err)
	}
	if result != "N/A" {
		t.Errorf("expected 'N/A', got %q", result)
	}
	// Non-empty value → actual.
	result, err = pm.RenderTemplate("func_default", struct{ Value string }{"real"})
	if err != nil {
		t.Fatalf("RenderTemplate: %v", err)
	}
	if result != "real" {
		t.Errorf("expected 'real', got %q", result)
	}
}

func TestTemplateFuncMap_Join(t *testing.T) {
	pm := newTestPromptManager(t)
	err := pm.RegisterTemplate("func_join", `{{join ", " .Items}}`)
	if err != nil {
		t.Fatalf("RegisterTemplate: %v", err)
	}
	result, err := pm.RenderTemplate("func_join", struct{ Items []string }{[]string{"a", "b", "c"}})
	if err != nil {
		t.Fatalf("RenderTemplate: %v", err)
	}
	if result != "a, b, c" {
		t.Errorf("expected 'a, b, c', got %q", result)
	}
}

func TestTemplateFuncMap_Upper(t *testing.T) {
	pm := newTestPromptManager(t)
	err := pm.RegisterTemplate("func_upper", `{{upper .Text}}`)
	if err != nil {
		t.Fatalf("RegisterTemplate: %v", err)
	}
	result, err := pm.RenderTemplate("func_upper", struct{ Text string }{"hello"})
	if err != nil {
		t.Fatalf("RenderTemplate: %v", err)
	}
	if result != "HELLO" {
		t.Errorf("expected 'HELLO', got %q", result)
	}
}

// ---------------------------------------------------------------------------
// Concurrency safety test
// ---------------------------------------------------------------------------

func TestPromptManager_ConcurrentAccess(t *testing.T) {
	pm := newTestPromptManager(t)
	done := make(chan struct{})
	const goroutines = 20

	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer func() { done <- struct{}{} }()
			// Mix of reads and writes.
			if idx%3 == 0 {
				_ = pm.RegisterTemplate(
					"concurrent_"+string(rune('A'+idx%26)),
					"Template {{.X}}",
				)
			} else if idx%3 == 1 {
				_ = pm.ListTemplates()
			} else {
				_, _ = pm.BuildPrompt(context.Background(), TaskFTO, &PromptParams{
					UserQuery: "concurrent test",
					Language:  "en",
				})
			}
		}(i)
	}
	for i := 0; i < goroutines; i++ {
		<-done
	}
}

// ---------------------------------------------------------------------------
// isCJK helper test
// ---------------------------------------------------------------------------

func TestIsCJK(t *testing.T) {
	tests := []struct {
		r    rune
		want bool
	}{
		{'专', true},
		{'利', true},
		{'A', false},
		{'1', false},
		{' ', false},
		{'。', true},  // CJK punctuation
		{'（', true},  // fullwidth
		{'α', false}, // Greek
	}
	for _, tt := range tests {
		got := isCJK(tt.r)
		if got != tt.want {
			t.Errorf("isCJK(%q) = %v, want %v", tt.r, got, tt.want)
		}
	}
}

//Personal.AI order the ending

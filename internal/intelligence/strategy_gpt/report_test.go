package strategy_gpt

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/turtacn/KeyIP-Intelligence/internal/intelligence/common"
)

// ---------------------------------------------------------------------------
// Mocks
// ---------------------------------------------------------------------------

type mockLLMBackend struct {
	predictFn       func(ctx context.Context, req *common.PredictRequest) (*common.PredictResponse, error)
	predictStreamFn func(ctx context.Context, req *common.PredictRequest) (<-chan *common.PredictResponse, error)
	callCount       atomic.Int32
}

func (m *mockLLMBackend) Predict(ctx context.Context, req *common.PredictRequest) (*common.PredictResponse, error) {
	m.callCount.Add(1)
	if m.predictFn != nil {
		return m.predictFn(ctx, req)
	}
	return defaultLLMResponse(), nil
}

func (m *mockLLMBackend) PredictStream(ctx context.Context, req *common.PredictRequest) (<-chan *common.PredictResponse, error) {
	if m.predictStreamFn != nil {
		return m.predictStreamFn(ctx, req)
	}
	ch := make(chan *common.PredictResponse, 5)
	go func() {
		defer close(ch)
		chunks := []string{
			"# FTO Analysis Report\n\n",
			"## Executive Summary\n\nThis is the executive summary.\n\n",
			"## Technical Analysis\n\nDetailed technical analysis here.\n\n",
			"## Conclusions\n\n- No infringement risk identified.\n\n",
			"## Recommendations\n\n- Continue current product design.\n",
		}
		for _, c := range chunks {
			ch <- &common.PredictResponse{
				Outputs: map[string][]byte{"text": []byte(c)},
			}
		}
	}()
	return ch, nil
}

func (m *mockLLMBackend) Healthy(ctx context.Context) error { return nil }
func (m *mockLLMBackend) Close() error                      { return nil }

func defaultLLMResponse() *common.PredictResponse {
	text := `# FTO Analysis Report

## Executive Summary

This report analyses the freedom-to-operate position for the target product.

## Technical Analysis

The product uses a novel catalytic process that differs from the prior art.

## Conclusions

- The product does not infringe Patent US12345678.
- The MPEP §2111.03 claim construction supports a narrow interpretation.

## Recommendations

- Continue with the current product design.
- Monitor CN112345678A for prosecution updates.

## Risk Assessment

- Potential design-around required if claims are broadened.
- Litigation risk from competitor portfolio is low.
`
	return &common.PredictResponse{
		Outputs: map[string][]byte{
			"text":             []byte(text),
			"prompt_tokens":    []byte("500"),
			"completion_tokens": []byte("800"),
			"total_tokens":     []byte("1300"),
		},
		InferenceTimeMs: 2500,
	}
}

type mockPromptMgr struct {
	buildFn   func(task AnalysisTask, params *PromptParams) (string, error)
	callCount atomic.Int32
}

func (m *mockPromptMgr) BuildPrompt(task AnalysisTask, params *PromptParams) (string, error) {
	m.callCount.Add(1)
	if m.buildFn != nil {
		return m.buildFn(task, params)
	}
	return fmt.Sprintf("Analyse task=%s", task), nil
}

type mockRAG struct {
	retrieveFn func(ctx context.Context, query string, topK int) ([]*RAGDocument, error)
	callCount  atomic.Int32
}

func (m *mockRAG) RetrieveAndRerank(ctx context.Context, query string, topK int) ([]*RAGDocument, error) {
	m.callCount.Add(1)
	if m.retrieveFn != nil {
		return m.retrieveFn(ctx, query, topK)
	}
	return []*RAGDocument{
		{DocumentID: "doc-1", Content: "Patent US12345678 claims...", Score: 0.95, Source: "US12345678"},
		{DocumentID: "doc-2", Content: "MPEP §2111.03 states...", Score: 0.88, Source: "MPEP §2111.03"},
	}, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func newTestReportGenerator(t *testing.T) (ReportGenerator, *mockLLMBackend, *mockPromptMgr, *mockRAG) {
	t.Helper()
	backend := &mockLLMBackend{}
	pm := &mockPromptMgr{}
	rag := &mockRAG{}
	cfg := DefaultStrategyGPTConfig()
	gen, err := NewReportGenerator(backend, pm, rag, cfg, nil, nil)
	if err != nil {
		t.Fatalf("NewReportGenerator: %v", err)
	}
	return gen, backend, pm, rag
}

func newTestReportGeneratorNoRAG(t *testing.T) (ReportGenerator, *mockLLMBackend, *mockPromptMgr, *mockRAG) {
	t.Helper()
	backend := &mockLLMBackend{}
	pm := &mockPromptMgr{}
	rag := &mockRAG{}
	cfg := DefaultStrategyGPTConfig()
	cfg.RAGEnabled = false
	gen, err := NewReportGenerator(backend, pm, rag, cfg, nil, nil)
	if err != nil {
		t.Fatalf("NewReportGenerator: %v", err)
	}
	return gen, backend, pm, rag
}

func ftoRequest(qualityCheck bool) *ReportRequest {
	return &ReportRequest{
		Task: TaskFTO,
		Params: &PromptParams{
			PatentNumbers: []string{"US12345678"},
			ProductDesc:   "Novel catalytic converter",
			TechDomain:    "chemical engineering",
		},
		OutputFormat: FormatNarrative,
		QualityCheck: qualityCheck,
		RequestID:    "test-req-001",
	}
}

// ---------------------------------------------------------------------------
// Tests: GenerateReport
// ---------------------------------------------------------------------------

func TestGenerateReport_FTO_Success(t *testing.T) {
	gen, _, _, _ := newTestReportGenerator(t)
	report, err := gen.GenerateReport(context.Background(), ftoRequest(false))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.ReportID == "" {
		t.Error("expected non-empty ReportID")
	}
	if report.Task != TaskFTO {
		t.Errorf("expected task FTO, got %s", report.Task)
	}
	if report.Content == nil {
		t.Fatal("expected non-nil Content")
	}
	if report.Content.ExecutiveSummary == "" {
		t.Error("expected non-empty ExecutiveSummary")
	}
	if len(report.Content.Sections) == 0 {
		t.Error("expected at least one section")
	}
	if len(report.Content.Conclusions) == 0 {
		t.Error("expected at least one conclusion")
	}
	if len(report.Content.Recommendations) == 0 {
		t.Error("expected at least one recommendation")
	}
}

func TestGenerateReport_WithRAG(t *testing.T) {
	gen, _, pm, rag := newTestReportGenerator(t)
	_, err := gen.GenerateReport(context.Background(), ftoRequest(false))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rag.callCount.Load() == 0 {
		t.Error("expected RAG engine to be called")
	}
	// Verify RAG context was injected into prompt params
	if pm.callCount.Load() == 0 {
		t.Error("expected prompt manager to be called")
	}
}

func TestGenerateReport_WithoutRAG(t *testing.T) {
	gen, _, _, rag := newTestReportGeneratorNoRAG(t)
	_, err := gen.GenerateReport(context.Background(), ftoRequest(false))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rag.callCount.Load() != 0 {
		t.Error("expected RAG engine NOT to be called when disabled")
	}
}

func TestGenerateReport_WithQualityCheck(t *testing.T) {
	gen, _, _, _ := newTestReportGenerator(t)
	report, err := gen.GenerateReport(context.Background(), ftoRequest(true))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.Validation == nil {
		t.Fatal("expected non-nil Validation when QualityCheck=true")
	}
	if report.Validation.QualityScore <= 0 {
		t.Errorf("expected positive quality score, got %f", report.Validation.QualityScore)
	}
}

func TestGenerateReport_WithoutQualityCheck(t *testing.T) {
	gen, _, _, _ := newTestReportGenerator(t)
	report, err := gen.GenerateReport(context.Background(), ftoRequest(false))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.Validation != nil {
		t.Error("expected nil Validation when QualityCheck=false")
	}
}

func TestGenerateReport_LLMError(t *testing.T) {
	backend := &mockLLMBackend{
		predictFn: func(ctx context.Context, req *common.PredictRequest) (*common.PredictResponse, error) {
			return nil, fmt.Errorf("LLM internal error")
		},
	}
	gen, _ := NewReportGenerator(backend, &mockPromptMgr{}, nil, nil, nil, nil)
	_, err := gen.GenerateReport(context.Background(), ftoRequest(false))
	if err == nil {
		t.Fatal("expected error from LLM backend")
	}
	if !strings.Contains(err.Error(), "LLM") {
		t.Errorf("expected LLM-related error, got: %v", err)
	}
}

func TestGenerateReport_LLMTimeout(t *testing.T) {
	backend := &mockLLMBackend{
		predictFn: func(ctx context.Context, req *common.PredictRequest) (*common.PredictResponse, error) {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(10 * time.Second):
				return defaultLLMResponse(), nil
			}
		},
	}
	gen, _ := NewReportGenerator(backend, &mockPromptMgr{}, nil, nil, nil, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err := gen.GenerateReport(ctx, ftoRequest(false))
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestGenerateReport_TokenUsage(t *testing.T) {
	gen, _, _, _ := newTestReportGenerator(t)
	report, err := gen.GenerateReport(context.Background(), ftoRequest(false))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.TokensUsed == nil {
		t.Fatal("expected non-nil TokensUsed")
	}
	if report.TokensUsed.PromptTokens != 500 {
		t.Errorf("expected 500 prompt tokens, got %d", report.TokensUsed.PromptTokens)
	}
	if report.TokensUsed.CompletionTokens != 800 {
		t.Errorf("expected 800 completion tokens, got %d", report.TokensUsed.CompletionTokens)
	}
	if report.TokensUsed.TotalTokens != 1300 {
		t.Errorf("expected 1300 total tokens, got %d", report.TokensUsed.TotalTokens)
	}
}

func TestGenerateReport_Latency(t *testing.T) {
	gen, _, _, _ := newTestReportGenerator(t)
	report, err := gen.GenerateReport(context.Background(), ftoRequest(false))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.LatencyMs <= 0 {
		t.Errorf("expected positive LatencyMs, got %d", report.LatencyMs)
	}
}

func TestGenerateReport_NilRequest(t *testing.T) {
	gen, _, _, _ := newTestReportGenerator(t)
	_, err := gen.GenerateReport(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil request")
	}
}

// ---------------------------------------------------------------------------
// Tests: GenerateReportStream
// ---------------------------------------------------------------------------

func TestGenerateReportStream_Success(t *testing.T) {
	gen, _, _, _ := newTestReportGenerator(t)
	ch, err := gen.GenerateReportStream(context.Background(), ftoRequest(false))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var chunks []*ReportChunk
	for chunk := range ch {
		chunks = append(chunks, chunk)
	}

	if len(chunks) == 0 {
		t.Fatal("expected at least one chunk")
	}

	lastChunk := chunks[len(chunks)-1]
	if !lastChunk.IsComplete {
		t.Error("expected last chunk to have IsComplete=true")
	}

	// At least one non-final chunk should have content
	hasContent := false
	for _, c := range chunks {
		if c.Content != "" {
			hasContent = true
			break
		}
	}
	if !hasContent {
		t.Error("expected at least one chunk with content")
	}
}

func TestGenerateReportStream_ChunkOrder(t *testing.T) {
	gen, _, _, _ := newTestReportGenerator(t)
	ch, err := gen.GenerateReportStream(context.Background(), ftoRequest(false))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	prevIdx := -1
	for chunk := range ch {
		if chunk.ChunkIndex <= prevIdx && !chunk.IsComplete {
			t.Errorf("chunk index not monotonically increasing: prev=%d, current=%d", prevIdx, chunk.ChunkIndex)
		}
		prevIdx = chunk.ChunkIndex
	}
}

func TestGenerateReportStream_ChannelClosed(t *testing.T) {
	gen, _, _, _ := newTestReportGenerator(t)
	ch, err := gen.GenerateReportStream(context.Background(), ftoRequest(false))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Drain channel
	for range ch {
	}

	// Channel should be closed — reading again should return zero value immediately
	chunk, ok := <-ch
	if ok {
		t.Errorf("expected channel to be closed, got chunk: %+v", chunk)
	}
}

func TestGenerateReportStream_ContextCancellation(t *testing.T) {
	slowBackend := &mockLLMBackend{
		predictStreamFn: func(ctx context.Context, req *common.PredictRequest) (<-chan *common.PredictResponse, error) {
			ch := make(chan *common.PredictResponse)
			go func() {
				defer close(ch)
				for i := 0; i < 100; i++ {
					select {
					case <-ctx.Done():
						return
					case <-time.After(100 * time.Millisecond):
						ch <- &common.PredictResponse{
							Outputs: map[string][]byte{"text": []byte(fmt.Sprintf("chunk %d\n\n", i))},
						}
					}
				}
			}()
			return ch, nil
		},
	}
	cfg := DefaultStrategyGPTConfig()
	gen, _ := NewReportGenerator(slowBackend, &mockPromptMgr{}, nil, cfg, nil, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()

	ch, err := gen.GenerateReportStream(ctx, ftoRequest(false))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var chunks []*ReportChunk
	for chunk := range ch {
		chunks = append(chunks, chunk)
	}

	// Should have terminated early
	if len(chunks) == 0 {
		t.Fatal("expected at least one chunk before cancellation")
	}

	lastChunk := chunks[len(chunks)-1]
	if !lastChunk.IsComplete {
		t.Error("expected final chunk with IsComplete=true after cancellation")
	}
}

func TestGenerateReportStream_NilRequest(t *testing.T) {
	gen, _, _, _ := newTestReportGenerator(t)
	_, err := gen.GenerateReportStream(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil request")
	}
}

// ---------------------------------------------------------------------------
// Tests: ParseLLMOutput
// ---------------------------------------------------------------------------

func TestParseLLMOutput_Structured_Success(t *testing.T) {
	gen, _, _, _ := newTestReportGenerator(t)

	structured := &ReportContent{
		Title:            "FTO Report",
		ExecutiveSummary: "No infringement risk.",
		Sections: []*ReportSection{
			{SectionID: "s1", Title: "Analysis", Content: "Detailed analysis.", Order: 1},
		},
		Conclusions: []*Conclusion{
			{Statement: "Product is clear.", Confidence: 0.9},
		},
		Recommendations: []*Recommendation{
			{Action: "Proceed with launch.", Priority: "High"},
		},
	}
	jsonBytes, _ := json.Marshal(structured)

	content, err := gen.ParseLLMOutput(string(jsonBytes), FormatStructured)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content.Title != "FTO Report" {
		t.Errorf("expected title 'FTO Report', got '%s'", content.Title)
	}
	if content.ExecutiveSummary != "No infringement risk." {
		t.Errorf("unexpected summary: %s", content.ExecutiveSummary)
	}
	if len(content.Sections) != 1 {
		t.Errorf("expected 1 section, got %d", len(content.Sections))
	}
	if len(content.Conclusions) != 1 {
		t.Errorf("expected 1 conclusion, got %d", len(content.Conclusions))
	}
}

func TestParseLLMOutput_Structured_WithCodeFence(t *testing.T) {
	gen, _, _, _ := newTestReportGenerator(t)

	structured := &ReportContent{
		Title:            "Test",
		ExecutiveSummary: "Summary",
	}
	jsonBytes, _ := json.Marshal(structured)
	wrapped := fmt.Sprintf("Here is the result:\n```json\n%s\n```\n", string(jsonBytes))

	content, err := gen.ParseLLMOutput(wrapped, FormatStructured)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content.Title != "Test" {
		t.Errorf("expected title 'Test', got '%s'", content.Title)
	}
}

func TestParseLLMOutput_Structured_InvalidJSON(t *testing.T) {
	gen, _, _, _ := newTestReportGenerator(t)

	content, err := gen.ParseLLMOutput("this is not json {{{", FormatStructured)
	// Should degrade, not error
	if err != nil {
		t.Fatalf("expected degraded parse, not error: %v", err)
	}
	if content == nil {
		t.Fatal("expected non-nil content from degraded parse")
	}
	if len(content.Sections) == 0 {
		t.Error("expected at least one section from degraded parse")
	}
}

func TestParseLLMOutput_Narrative_Sections(t *testing.T) {
	gen, _, _, _ := newTestReportGenerator(t)

	narrative := `# Patent Analysis

## Executive Summary

This is the summary.

## Technical Background

The technology involves catalytic processes.

## Claim Analysis

Claims 1-5 are independent claims.

## Conclusions

- No infringement found.

## Recommendations

- Continue development.
`
	content, err := gen.ParseLLMOutput(narrative, FormatNarrative)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content.ExecutiveSummary == "" {
		t.Error("expected non-empty executive summary")
	}
	if len(content.Sections) < 2 {
		t.Errorf("expected at least 2 regular sections, got %d", len(content.Sections))
	}
}

func TestParseLLMOutput_Narrative_Conclusions(t *testing.T) {
	gen, _, _, _ := newTestReportGenerator(t)

	narrative := `## Overview

Some overview text.

## Conclusions

- First conclusion statement.
- Second conclusion statement.
`
	content, err := gen.ParseLLMOutput(narrative, FormatNarrative)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(content.Conclusions) < 2 {
		t.Errorf("expected at least 2 conclusions, got %d", len(content.Conclusions))
	}
	for _, c := range content.Conclusions {
		if c.Statement == "" {
			t.Error("expected non-empty conclusion statement")
		}
	}
}

func TestParseLLMOutput_Narrative_Recommendations(t *testing.T) {
	gen, _, _, _ := newTestReportGenerator(t)

	narrative := `## Analysis

Some analysis.

## Recommendations

- File a continuation application.
- Monitor competitor patents.
- Consider design-around for claim 3.
`
	content, err := gen.ParseLLMOutput(narrative, FormatNarrative)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(content.Recommendations) < 3 {
		t.Errorf("expected at least 3 recommendations, got %d", len(content.Recommendations))
	}
	for _, r := range content.Recommendations {
		if r.Action == "" {
			t.Error("expected non-empty recommendation action")
		}
		if r.Priority == "" {
			t.Error("expected non-empty recommendation priority")
		}
	}
}

func TestParseLLMOutput_Bullet_Items(t *testing.T) {
	gen, _, _, _ := newTestReportGenerator(t)

	bullet := `- The product is novel.
- No prior art found in the target jurisdiction.
- Claims 1-3 are broadly drafted.
- Recommend filing in EP and CN.
`
	content, err := gen.ParseLLMOutput(bullet, FormatBullet)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content.ExecutiveSummary == "" {
		t.Error("expected non-empty summary from first bullet")
	}
	// First item is summary, rest are sections
	if len(content.Sections) < 3 {
		t.Errorf("expected at least 3 sections from bullets, got %d", len(content.Sections))
	}
}

func TestParseLLMOutput_CitationExtraction(t *testing.T) {
	gen, _, _, _ := newTestReportGenerator(t)

	text := `## Analysis

The product may infringe [Patent US12345678] based on claim 1.
`
	content, err := gen.ParseLLMOutput(text, FormatNarrative)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(content.Citations) == 0 {
		t.Fatal("expected at least one citation")
	}
	found := false
	for _, c := range content.Citations {
		if strings.Contains(c.Source, "US12345678") {
			found = true
			if c.SourceType != SourcePatent {
				t.Errorf("expected source type patent, got %s", c.SourceType)
			}
		}
	}
	if !found {
		t.Error("expected citation for US12345678")
	}
}

func TestParseLLMOutput_MultipleCitationFormats(t *testing.T) {
	gen, _, _, _ := newTestReportGenerator(t)

	text := `## Analysis

See MPEP §2111.03 for claim construction guidance.
Patent CN112345678A is relevant prior art.
Under 35 U.S.C. §103, the combination is obvious.
[Patent US98765432] was cited during prosecution.
`
	content, err := gen.ParseLLMOutput(text, FormatNarrative)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sourceMap := make(map[string]bool)
	for _, c := range content.Citations {
		sourceMap[c.Source] = true
	}

	expectations := []string{"2111.03", "CN112345678A", "103", "US98765432"}
	for _, exp := range expectations {
		found := false
		for src := range sourceMap {
			if strings.Contains(src, exp) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected citation containing '%s', sources: %v", exp, sourceMap)
		}
	}
}

func TestParseLLMOutput_EmptyOutput(t *testing.T) {
	gen, _, _, _ := newTestReportGenerator(t)
	_, err := gen.ParseLLMOutput("", FormatNarrative)
	if err != ErrEmptyLLMOutput {
		t.Errorf("expected ErrEmptyLLMOutput, got %v", err)
	}
}

func TestParseLLMOutput_WhitespaceOnly(t *testing.T) {
	gen, _, _, _ := newTestReportGenerator(t)
	_, err := gen.ParseLLMOutput("   \n\n  \t  ", FormatNarrative)
	if err != ErrEmptyLLMOutput {
		t.Errorf("expected ErrEmptyLLMOutput, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Tests: ValidateReport
// ---------------------------------------------------------------------------

func TestValidateReport_Valid(t *testing.T) {
	gen, _, _, _ := newTestReportGenerator(t)
	report := &Report{
		Content: &ReportContent{
			ExecutiveSummary: "This is a comprehensive summary of the FTO analysis.",
			Sections: []*ReportSection{
				{SectionID: "s1", Title: "Analysis", Content: strings.Repeat("Detailed analysis. ", 100), Order: 1},
			},
			Conclusions: []*Conclusion{
				{Statement: "No infringement risk.", Confidence: 0.9},
			},
			Recommendations: []*Recommendation{
				{Action: "Proceed with launch.", Priority: "High", Rationale: "Low risk.", Timeline: "Q1 2025"},
			},
		},
	}
	validation, err := gen.ValidateReport(report)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !validation.IsValid {
		t.Error("expected IsValid=true")
	}
	if validation.QualityScore < 0.7 {
		t.Errorf("expected quality score >= 0.7, got %f", validation.QualityScore)
	}
}

func TestValidateReport_MissingSummary(t *testing.T) {
	gen, _, _, _ := newTestReportGenerator(t)
	report := &Report{
		Content: &ReportContent{
			ExecutiveSummary: "",
			Sections: []*ReportSection{
				{SectionID: "s1", Title: "Analysis", Content: "Some content.", Order: 1},
			},
			Conclusions: []*Conclusion{
				{Statement: "Conclusion.", Confidence: 0.8},
			},
		},
	}
	validation, err := gen.ValidateReport(report)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	hasIssue := false
	for _, issue := range validation.Issues {
		if issue.IssueType == "missing_summary" {
			hasIssue = true
		}
	}
	if !hasIssue {
		t.Error("expected missing_summary issue")
	}
}

func TestValidateReport_NoSections(t *testing.T) {
	gen, _, _, _ := newTestReportGenerator(t)
	report := &Report{
		Content: &ReportContent{
			ExecutiveSummary: "Summary.",
			Sections:         []*ReportSection{},
			Conclusions: []*Conclusion{
				{Statement: "Conclusion.", Confidence: 0.8},
			},
		},
	}
	validation, err := gen.ValidateReport(report)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	hasIssue := false
	for _, issue := range validation.Issues {
		if issue.IssueType == "missing_sections" {
			hasIssue = true
		}
	}
	if !hasIssue {
		t.Error("expected missing_sections issue")
	}
}

func TestValidateReport_NoConclusions(t *testing.T) {
	gen, _, _, _ := newTestReportGenerator(t)
	report := &Report{
		Content: &ReportContent{
			ExecutiveSummary: "Summary.",
			Sections: []*ReportSection{
				{SectionID: "s1", Title: "Analysis", Content: "Content.", Order: 1},
			},
			Conclusions: []*Conclusion{},
		},
	}
	validation, err := gen.ValidateReport(report)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	hasIssue := false
	for _, issue := range validation.Issues {
		if issue.IssueType == "missing_conclusions" {
			hasIssue = true
		}
	}
	if !hasIssue {
		t.Error("expected missing_conclusions issue")
	}
}

func TestValidateReport_NilReport(t *testing.T) {
	gen, _, _, _ := newTestReportGenerator(t)
	validation, err := gen.ValidateReport(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if validation.IsValid {
		t.Error("expected IsValid=false for nil report")
	}
	if validation.QualityScore != 0 {
		t.Errorf("expected quality score 0, got %f", validation.QualityScore)
	}
}

func TestValidateReport_NilContent(t *testing.T) {
	gen, _, _, _ := newTestReportGenerator(t)
	validation, err := gen.ValidateReport(&Report{Content: nil})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if validation.IsValid {
		t.Error("expected IsValid=false for nil content")
	}
}

func TestValidateReport_CitationVerification_AllVerified(t *testing.T) {
	rag := &mockRAG{
		retrieveFn: func(ctx context.Context, query string, topK int) ([]*RAGDocument, error) {
			return []*RAGDocument{
				{DocumentID: "d1", Content: "match", Score: 0.95, Source: query},
			}, nil
		},
	}
	cfg := DefaultStrategyGPTConfig()
	gen, _ := NewReportGenerator(&mockLLMBackend{}, &mockPromptMgr{}, rag, cfg, nil, nil)

	report := &Report{
		Content: &ReportContent{
			ExecutiveSummary: "Summary.",
			Sections: []*ReportSection{
				{SectionID: "s1", Title: "Analysis", Content: strings.Repeat("Content. ", 50), Order: 1},
			},
			Conclusions: []*Conclusion{
				{Statement: "Conclusion.", Confidence: 0.9},
			},
			Citations: []*Citation{
				{CitationID: "c1", Source: "US12345678", SourceType: SourcePatent, VerificationStatus: "Unverified"},
				{CitationID: "c2", Source: "CN112345678A", SourceType: SourcePatent, VerificationStatus: "Unverified"},
			},
		},
	}

	validation, err := gen.ValidateReport(report)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if validation.CitationVerification == nil {
		t.Fatal("expected non-nil CitationVerification")
	}
	if validation.CitationVerification.VerifiedCount != 2 {
		t.Errorf("expected 2 verified, got %d", validation.CitationVerification.VerifiedCount)
	}
	if validation.CitationVerification.NotFoundCount != 0 {
		t.Errorf("expected 0 not found, got %d", validation.CitationVerification.NotFoundCount)
	}
}

func TestValidateReport_CitationVerification_SomeNotFound(t *testing.T) {
	callIdx := atomic.Int32{}
	rag := &mockRAG{
		retrieveFn: func(ctx context.Context, query string, topK int) ([]*RAGDocument, error) {
			idx := callIdx.Add(1)
			if idx == 2 {
				// Second citation not found
				return []*RAGDocument{}, nil
			}
			return []*RAGDocument{
				{DocumentID: "d1", Content: "match", Score: 0.95, Source: query},
			}, nil
		},
	}
	cfg := DefaultStrategyGPTConfig()
	gen, _ := NewReportGenerator(&mockLLMBackend{}, &mockPromptMgr{}, rag, cfg, nil, nil)

	report := &Report{
		Content: &ReportContent{
			ExecutiveSummary: "Summary.",
			Sections: []*ReportSection{
				{SectionID: "s1", Title: "Analysis", Content: strings.Repeat("Content. ", 50), Order: 1},
			},
			Conclusions: []*Conclusion{
				{Statement: "Conclusion.", Confidence: 0.9},
			},
			Citations: []*Citation{
				{CitationID: "c1", Source: "US12345678", SourceType: SourcePatent, VerificationStatus: "Unverified"},
				{CitationID: "c2", Source: "US99999999", SourceType: SourcePatent, VerificationStatus: "Unverified"},
			},
		},
	}

	validation, err := gen.ValidateReport(report)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if validation.CitationVerification == nil {
		t.Fatal("expected non-nil CitationVerification")
	}
	if validation.CitationVerification.NotFoundCount == 0 {
		t.Error("expected at least one not-found citation")
	}
	// Quality score should be lower due to unverified citations
	if validation.QualityScore >= 1.0 {
		t.Errorf("expected quality score < 1.0 due to unverified citations, got %f", validation.QualityScore)
	}
}

func TestValidateReport_QualityScore_Calculation(t *testing.T) {
	gen, _, _, _ := newTestReportGenerator(t)

	// Perfect report: all dimensions maxed
	report := &Report{
		Content: &ReportContent{
			ExecutiveSummary: strings.Repeat("Summary text. ", 20),
			Sections: []*ReportSection{
				{SectionID: "s1", Title: "Analysis", Content: strings.Repeat("Detailed content. ", 200), Order: 1},
			},
			Conclusions: []*Conclusion{
				{Statement: "Clear conclusion.", Confidence: 0.95},
			},
			Recommendations: []*Recommendation{
				{Action: "Take action.", Priority: "High", Rationale: "Because.", Timeline: "Q1 2025"},
			},
			// No citations -> citation score defaults to 1.0
		},
	}

	validation, err := gen.ValidateReport(report)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// structure=1.0*0.3 + citation=1.0*0.3 + length>=0.8*0.2 + action=1.0*0.2
	// = 0.3 + 0.3 + >=0.16 + 0.2 = >= 0.96
	if validation.QualityScore < 0.9 {
		t.Errorf("expected quality score >= 0.9 for perfect report, got %f", validation.QualityScore)
	}
}

// ---------------------------------------------------------------------------
// Tests: ExportReport
// ---------------------------------------------------------------------------

func TestExportReport_JSON(t *testing.T) {
	gen, _, _, _ := newTestReportGenerator(t)
	report := &Report{
		ReportID: "rpt-001",
		Task:     TaskFTO,
		Content: &ReportContent{
			Title:            "FTO Report",
			ExecutiveSummary: "Summary.",
			Sections: []*ReportSection{
				{SectionID: "s1", Title: "Analysis", Content: "Content.", Order: 1},
			},
		},
		Metadata: &ReportMetadata{
			ModelID:      "strategy-gpt-v1",
			ModelVersion: "1.0.0",
		},
		GeneratedAt: time.Now(),
		LatencyMs:   1500,
		TokensUsed:  &TokenUsage{PromptTokens: 100, CompletionTokens: 200, TotalTokens: 300},
	}

	data, err := gen.ExportReport(report, ExportJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should be valid JSON
	var parsed Report
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("exported JSON is not valid: %v", err)
	}
	if parsed.ReportID != "rpt-001" {
		t.Errorf("expected report ID 'rpt-001', got '%s'", parsed.ReportID)
	}
	if parsed.Content.Title != "FTO Report" {
		t.Errorf("expected title 'FTO Report', got '%s'", parsed.Content.Title)
	}
}

func TestExportReport_Markdown(t *testing.T) {
	gen, _, _, _ := newTestReportGenerator(t)
	report := &Report{
		ReportID: "rpt-002",
		Task:     TaskFTO,
		Content: &ReportContent{
			Title:            "FTO Analysis",
			ExecutiveSummary: "No risk identified.",
			Sections: []*ReportSection{
				{SectionID: "s1", Title: "Technical Analysis", Content: "The product is novel.", Order: 1},
			},
			Conclusions: []*Conclusion{
				{Statement: "Product is clear.", Confidence: 0.9},
			},
			Recommendations: []*Recommendation{
				{Action: "Proceed.", Priority: "High", Rationale: "Low risk.", Timeline: "Q1"},
			},
			Citations: []*Citation{
				{CitationID: "c1", Source: "US12345678", SourceType: SourcePatent, VerificationStatus: "Verified"},
			},
		},
		Metadata: &ReportMetadata{
			ModelID:      "strategy-gpt-v1",
			ModelVersion: "1.0.0",
		},
		GeneratedAt: time.Now(),
		LatencyMs:   2000,
	}

	data, err := gen.ExportReport(report, ExportMarkdown)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	md := string(data)
	if !strings.Contains(md, "# FTO Analysis") {
		t.Error("expected markdown to contain title heading")
	}
	if !strings.Contains(md, "## Executive Summary") {
		t.Error("expected markdown to contain executive summary heading")
	}
	if !strings.Contains(md, "## Technical Analysis") {
		t.Error("expected markdown to contain section heading")
	}
	if !strings.Contains(md, "## Conclusions") {
		t.Error("expected markdown to contain conclusions heading")
	}
	if !strings.Contains(md, "## Recommendations") {
		t.Error("expected markdown to contain recommendations heading")
	}
	if !strings.Contains(md, "US12345678") {
		t.Error("expected markdown to contain citation source")
	}
}

func TestExportReport_PDF(t *testing.T) {
	gen, _, _, _ := newTestReportGenerator(t)
	report := &Report{
		Content: &ReportContent{
			Title:            "Test",
			ExecutiveSummary: "Summary.",
		},
		Metadata: &ReportMetadata{ModelID: "test"},
	}
	_, err := gen.ExportReport(report, ExportPDF)
	if err != ErrExportFormatNotImplemented {
		t.Errorf("expected ErrExportFormatNotImplemented, got %v", err)
	}
}

func TestExportReport_DOCX(t *testing.T) {
	gen, _, _, _ := newTestReportGenerator(t)
	report := &Report{
		Content: &ReportContent{
			Title:            "Test",
			ExecutiveSummary: "Summary.",
		},
		Metadata: &ReportMetadata{ModelID: "test"},
	}
	_, err := gen.ExportReport(report, ExportDOCX)
	if err != ErrExportFormatNotImplemented {
		t.Errorf("expected ErrExportFormatNotImplemented, got %v", err)
	}
}

func TestExportReport_NilReport(t *testing.T) {
	gen, _, _, _ := newTestReportGenerator(t)
	_, err := gen.ExportReport(nil, ExportJSON)
	if err == nil {
		t.Fatal("expected error for nil report")
	}
}

// ---------------------------------------------------------------------------
// Tests: RiskAssessment
// ---------------------------------------------------------------------------

func TestRiskAssessment_OverallScore(t *testing.T) {
	gen, _, _, _ := newTestReportGenerator(t)

	narrative := `## Overview

Some overview.

## Risk Assessment

- Patent portfolio overlap with competitor A.
- Potential claim scope expansion in reexamination.
- Design-around feasibility is limited.
`
	content, err := gen.ParseLLMOutput(narrative, FormatNarrative)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content.RiskAssessment == nil {
		t.Fatal("expected non-nil RiskAssessment")
	}
	if len(content.RiskAssessment.RiskFactors) < 3 {
		t.Errorf("expected at least 3 risk factors, got %d", len(content.RiskAssessment.RiskFactors))
	}
	if content.RiskAssessment.OverallRiskScore <= 0 {
		t.Errorf("expected positive overall risk score, got %f", content.RiskAssessment.OverallRiskScore)
	}
	if content.RiskAssessment.OverallRiskLevel == "" {
		t.Error("expected non-empty overall risk level")
	}
}

func TestRiskAssessment_RiskLevel(t *testing.T) {
	tests := []struct {
		score    float64
		expected string
	}{
		{0.95, "Critical"},
		{0.80, "Critical"},
		{0.75, "High"},
		{0.60, "High"},
		{0.55, "Medium"},
		{0.40, "Medium"},
		{0.35, "Low"},
		{0.20, "Low"},
		{0.15, "Negligible"},
		{0.0, "Negligible"},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("score_%.2f", tt.score), func(t *testing.T) {
			level := classifyRiskLevel(tt.score)
			if level != tt.expected {
				t.Errorf("score %.2f: expected %s, got %s", tt.score, tt.expected, level)
			}
		})
	}
}

func TestRiskFactor_ScoreCalculation(t *testing.T) {
	tests := []struct {
		likelihood float64
		impact     float64
		expected   float64
	}{
		{0.5, 0.5, 0.25},
		{1.0, 1.0, 1.0},
		{0.0, 1.0, 0.0},
		{1.0, 0.0, 0.0},
		{0.8, 0.6, 0.48},
		{0.3, 0.7, 0.21},
		// Clamping: values > 1 should be clamped
		{1.5, 0.5, 0.5},
		{0.5, 1.5, 0.5},
		// Clamping: values < 0 should be clamped
		{-0.5, 0.5, 0.0},
		{0.5, -0.5, 0.0},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("L%.1f_I%.1f", tt.likelihood, tt.impact), func(t *testing.T) {
			score := ComputeRiskFactorScore(tt.likelihood, tt.impact)
			diff := score - tt.expected
			if diff < -0.001 || diff > 0.001 {
				t.Errorf("L=%.1f I=%.1f: expected %.3f, got %.3f", tt.likelihood, tt.impact, tt.expected, score)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Tests: Recommendation Priority
// ---------------------------------------------------------------------------

func TestRecommendation_Priority(t *testing.T) {
	gen, _, _, _ := newTestReportGenerator(t)

	narrative := `## Analysis

Some analysis text.

## Recommendations

- Immediately cease production of variant B.
- File continuation application for claims 4-6.
- Monitor competitor C prosecution history.
`
	content, err := gen.ParseLLMOutput(narrative, FormatNarrative)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(content.Recommendations) < 3 {
		t.Fatalf("expected at least 3 recommendations, got %d", len(content.Recommendations))
	}

	// First recommendation should be High priority
	if content.Recommendations[0].Priority != "High" {
		t.Errorf("expected first recommendation priority 'High', got '%s'", content.Recommendations[0].Priority)
	}

	// Subsequent recommendations should have a priority assigned
	for i, r := range content.Recommendations {
		if r.Priority == "" {
			t.Errorf("recommendation %d has empty priority", i)
		}
	}
}

// ---------------------------------------------------------------------------
// Tests: Edge cases
// ---------------------------------------------------------------------------

func TestGenerateReport_EmptyLLMOutput(t *testing.T) {
	backend := &mockLLMBackend{
		predictFn: func(ctx context.Context, req *common.PredictRequest) (*common.PredictResponse, error) {
			return &common.PredictResponse{
				Outputs: map[string][]byte{
					"text": []byte(""),
				},
			}, nil
		},
	}
	cfg := DefaultStrategyGPTConfig()
	cfg.RAGEnabled = false
	gen, _ := NewReportGenerator(backend, &mockPromptMgr{}, nil, cfg, nil, nil)

	_, err := gen.GenerateReport(context.Background(), ftoRequest(false))
	if err == nil {
		t.Fatal("expected error for empty LLM output")
	}
}

func TestNewReportGenerator_NilBackend(t *testing.T) {
	_, err := NewReportGenerator(nil, &mockPromptMgr{}, nil, nil, nil, nil)
	if err == nil {
		t.Fatal("expected error for nil backend")
	}
}

func TestNewReportGenerator_NilPromptManager(t *testing.T) {
	_, err := NewReportGenerator(&mockLLMBackend{}, nil, nil, nil, nil, nil)
	if err == nil {
		t.Fatal("expected error for nil prompt manager")
	}
}

func TestNewReportGenerator_NilConfig_UsesDefaults(t *testing.T) {
	gen, err := NewReportGenerator(&mockLLMBackend{}, &mockPromptMgr{}, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gen == nil {
		t.Fatal("expected non-nil generator")
	}
}

func TestOutputFormat_String(t *testing.T) {
	tests := []struct {
		f        OutputFormat
		expected string
	}{
		{FormatStructured, "structured"},
		{FormatNarrative, "narrative"},
		{FormatBullet, "bullet"},
		{OutputFormat(99), "unknown"},
	}
	for _, tt := range tests {
		if got := tt.f.String(); got != tt.expected {
			t.Errorf("OutputFormat(%d).String() = %s, want %s", tt.f, got, tt.expected)
		}
	}
}

func TestExportFormat_String(t *testing.T) {
	tests := []struct {
		f        ExportFormat
		expected string
	}{
		{ExportJSON, "json"},
		{ExportMarkdown, "markdown"},
		{ExportPDF, "pdf"},
		{ExportDOCX, "docx"},
		{ExportFormat(99), "unknown"},
	}
	for _, tt := range tests {
		if got := tt.f.String(); got != tt.expected {
			t.Errorf("ExportFormat(%d).String() = %s, want %s", tt.f, got, tt.expected)
		}
	}
}

func TestParseLLMOutput_Narrative_NoHeadings_Degrades(t *testing.T) {
	gen, _, _, _ := newTestReportGenerator(t)

	plain := "This is just a plain text paragraph without any markdown headings. It should degrade gracefully to a single section."

	content, err := gen.ParseLLMOutput(plain, FormatNarrative)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content == nil {
		t.Fatal("expected non-nil content from degraded parse")
	}
	if len(content.Sections) == 0 {
		t.Error("expected at least one section from degraded parse")
	}
	if content.ExecutiveSummary == "" {
		t.Error("expected non-empty summary from degraded parse")
	}
}

func TestParseLLMOutput_Bullet_Empty(t *testing.T) {
	gen, _, _, _ := newTestReportGenerator(t)

	// Only whitespace lines — no actual bullet items
	content, err := gen.ParseLLMOutput("   \n   \n   ", FormatBullet)
	if err != ErrEmptyLLMOutput {
		t.Errorf("expected ErrEmptyLLMOutput for whitespace-only bullet input, got err=%v content=%+v", err, content)
	}
}

func TestParseLLMOutput_Structured_WrappedInCodeFence_Generic(t *testing.T) {
	gen, _, _, _ := newTestReportGenerator(t)

	inner := `{"title":"Wrapped","executive_summary":"Test"}`
	wrapped := "```\n" + inner + "\n```"

	content, err := gen.ParseLLMOutput(wrapped, FormatStructured)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content.Title != "Wrapped" {
		t.Errorf("expected title 'Wrapped', got '%s'", content.Title)
	}
}

func TestDegradeToSingleSection_LongText(t *testing.T) {
	longText := strings.Repeat("A", 1000)
	content := degradeToSingleSection(longText)
	if content.ExecutiveSummary == "" {
		t.Error("expected non-empty summary")
	}
	if len(content.ExecutiveSummary) > 510 {
		t.Errorf("expected summary truncated, got length %d", len(content.ExecutiveSummary))
	}
	if len(content.Sections) != 1 {
		t.Errorf("expected 1 section, got %d", len(content.Sections))
	}
	if content.Sections[0].Content != longText {
		t.Error("expected full text in section content")
	}
}

func TestDegradeToSingleSection_ShortText(t *testing.T) {
	short := "Brief output."
	content := degradeToSingleSection(short)
	if content.ExecutiveSummary != short {
		t.Errorf("expected summary to be full text, got '%s'", content.ExecutiveSummary)
	}
}

func TestDegradeToSingleSection_WithParagraphBreak(t *testing.T) {
	text := "First paragraph.\n\nSecond paragraph with more detail."
	content := degradeToSingleSection(text)
	if content.ExecutiveSummary != "First paragraph." {
		t.Errorf("expected first paragraph as summary, got '%s'", content.ExecutiveSummary)
	}
}

func TestComputeLengthScore(t *testing.T) {
	tests := []struct {
		name     string
		content  *ReportContent
		minScore float64
		maxScore float64
	}{
		{
			name:     "nil content",
			content:  nil,
			minScore: 0,
			maxScore: 0,
		},
		{
			name: "very short",
			content: &ReportContent{
				ExecutiveSummary: "Short.",
			},
			minScore: 0.1,
			maxScore: 0.3,
		},
		{
			name: "medium",
			content: &ReportContent{
				ExecutiveSummary: strings.Repeat("Word ", 120),
				Sections: []*ReportSection{
					{Content: strings.Repeat("Content ", 50)},
				},
			},
			minScore: 0.5,
			maxScore: 0.9,
		},
		{
			name: "long",
			content: &ReportContent{
				ExecutiveSummary: strings.Repeat("Summary ", 100),
				Sections: []*ReportSection{
					{Content: strings.Repeat("Detailed analysis ", 200)},
				},
			},
			minScore: 0.9,
			maxScore: 1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := computeLengthScore(tt.content)
			if score < tt.minScore || score > tt.maxScore {
				t.Errorf("expected score in [%.1f, %.1f], got %.2f", tt.minScore, tt.maxScore, score)
			}
		})
	}
}

func TestComputeActionabilityScore(t *testing.T) {
	tests := []struct {
		name     string
		recs     []*Recommendation
		minScore float64
	}{
		{
			name:     "no recommendations",
			recs:     nil,
			minScore: 0.2,
		},
		{
			name: "action only",
			recs: []*Recommendation{
				{Action: "Do something."},
			},
			minScore: 0.3,
		},
		{
			name: "fully specified",
			recs: []*Recommendation{
				{Action: "Do something.", Priority: "High", Rationale: "Because.", Timeline: "Q1"},
			},
			minScore: 0.9,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := computeActionabilityScore(tt.recs)
			if score < tt.minScore {
				t.Errorf("expected score >= %.1f, got %.2f", tt.minScore, score)
			}
		})
	}
}

func TestClassifyCitationSource(t *testing.T) {
	tests := []struct {
		source   string
		expected DocumentSourceType
	}{
		{"US12345678", SourcePatent},
		{"CN112345678A", SourcePatent},
		{"EP1234567", SourcePatent},
		{"WO2023/123456", SourcePatent},
		{"JP2023123456", SourcePatent},
		{"MPEP 2111.03", SourceMPEP},
		{"35 U.S.C. §103", SourceStatute},
		{"Some random source", SourceOther},
	}

	for _, tt := range tests {
		t.Run(tt.source, func(t *testing.T) {
			got := classifyCitationSource(tt.source)
			if got != tt.expected {
				t.Errorf("classifyCitationSource(%q) = %s, want %s", tt.source, got, tt.expected)
			}
		})
	}
}

func TestExtractTokenUsage_Complete(t *testing.T) {
	resp := &common.PredictResponse{
		Outputs: map[string][]byte{
			"prompt_tokens":     []byte("100"),
			"completion_tokens": []byte("200"),
			"total_tokens":      []byte("300"),
		},
	}
	usage := extractTokenUsage(resp)
	if usage.PromptTokens != 100 {
		t.Errorf("expected 100 prompt tokens, got %d", usage.PromptTokens)
	}
	if usage.CompletionTokens != 200 {
		t.Errorf("expected 200 completion tokens, got %d", usage.CompletionTokens)
	}
	if usage.TotalTokens != 300 {
		t.Errorf("expected 300 total tokens, got %d", usage.TotalTokens)
	}
}

func TestExtractTokenUsage_MissingTotal(t *testing.T) {
	resp := &common.PredictResponse{
		Outputs: map[string][]byte{
			"prompt_tokens":     []byte("100"),
			"completion_tokens": []byte("200"),
		},
	}
	usage := extractTokenUsage(resp)
	if usage.TotalTokens != 300 {
		t.Errorf("expected auto-calculated total 300, got %d", usage.TotalTokens)
	}
}

func TestExtractTokenUsage_NilResponse(t *testing.T) {
	usage := extractTokenUsage(nil)
	if usage.TotalTokens != 0 {
		t.Errorf("expected 0 total tokens for nil response, got %d", usage.TotalTokens)
	}
}

func TestSplitParagraphs(t *testing.T) {
	text := "First paragraph.\n\nSecond paragraph.\n\n\n\nThird paragraph."
	result := splitParagraphs(text)
	if len(result) != 3 {
		t.Errorf("expected 3 paragraphs, got %d: %v", len(result), result)
	}
}

func TestSplitParagraphs_Empty(t *testing.T) {
	result := splitParagraphs("")
	if len(result) != 0 {
		t.Errorf("expected 0 paragraphs, got %d", len(result))
	}
}

func TestDetectSectionHint(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"## Executive Summary\n", "Executive Summary"},
		{"# Title\n", "Title"},
		{"### Sub Section\n", "Sub Section"},
		{"No heading here", ""},
		{"", ""},
	}
	for _, tt := range tests {
		got := detectSectionHint(tt.input)
		if got != tt.expected {
			t.Errorf("detectSectionHint(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestShouldFlush(t *testing.T) {
	if !shouldFlush("paragraph one.\n\n") {
		t.Error("expected flush on double newline")
	}
	if !shouldFlush(strings.Repeat("x", 1025)) {
		t.Error("expected flush on large buffer")
	}
	if shouldFlush("short text") {
		t.Error("did not expect flush on short text without paragraph break")
	}
}

func TestDecodeIntFromBytes(t *testing.T) {
	tests := []struct {
		input    []byte
		expected int
	}{
		{[]byte("42"), 42},
		{[]byte("0"), 0},
		{[]byte(""), 0},
		{nil, 0},
		{[]byte("not a number"), 0},
	}
	for _, tt := range tests {
		got := decodeIntFromBytes(tt.input)
		if got != tt.expected {
			t.Errorf("decodeIntFromBytes(%q) = %d, want %d", tt.input, got, tt.expected)
		}
	}
}

func TestDefaultStrategyGPTConfig(t *testing.T) {
	cfg := DefaultStrategyGPTConfig()
	if cfg.ModelID == "" {
		t.Error("expected non-empty ModelID")
	}
	if cfg.RAGTopK <= 0 {
		t.Error("expected positive RAGTopK")
	}
	if cfg.MaxOutputTokens <= 0 {
		t.Error("expected positive MaxOutputTokens")
	}
	if cfg.Temperature < 0 || cfg.Temperature > 2 {
		t.Errorf("unexpected temperature: %f", cfg.Temperature)
	}
	if cfg.TimeoutMs <= 0 {
		t.Error("expected positive TimeoutMs")
	}
}

//Personal.AI order the ending

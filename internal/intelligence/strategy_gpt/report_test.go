package strategy_gpt

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/internal/intelligence/common"
)

type MockBackend struct {
	mock.Mock
}

func (m *MockBackend) Predict(ctx context.Context, req *common.PredictRequest) (*common.PredictResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*common.PredictResponse), args.Error(1)
}

func (m *MockBackend) PredictStream(ctx context.Context, req *common.PredictRequest) (<-chan *common.PredictResponse, error) {
	return nil, nil
}

func (m *MockBackend) Healthy(ctx context.Context) error { return nil }
func (m *MockBackend) Close() error { return nil }

// MockPromptManager
type MockPromptManager struct {
	mock.Mock
}

func (m *MockPromptManager) BuildPrompt(ctx context.Context, task AnalysisTask, params *PromptParams) (*BuiltPrompt, error) {
	args := m.Called(ctx, task, params)
	return args.Get(0).(*BuiltPrompt), args.Error(1)
}
// ... other methods stubbed
func (m *MockPromptManager) GetSystemPrompt(task AnalysisTask) (string, error) { return "", nil }
func (m *MockPromptManager) RenderTemplate(templateName string, data interface{}) (string, error) { return "", nil }
func (m *MockPromptManager) RegisterTemplate(name string, tmpl string) error { return nil }
func (m *MockPromptManager) ListTemplates() []TemplateInfo { return nil }
func (m *MockPromptManager) EstimateTokenCount(text string) int { return 0 }

type MockRAGEngine struct {
	mock.Mock
}

func (m *MockRAGEngine) RetrieveAndRerank(ctx context.Context, query *RAGQuery) (*RAGResult, error) {
	args := m.Called(ctx, query)
	return args.Get(0).(*RAGResult), args.Error(1)
}
// ...
func (m *MockRAGEngine) Retrieve(ctx context.Context, query *RAGQuery) (*RAGResult, error) { return nil, nil }
func (m *MockRAGEngine) BuildContext(ctx context.Context, result *RAGResult, budget int) (string, error) { return "", nil }
func (m *MockRAGEngine) IndexDocument(ctx context.Context, doc *Document) error { return nil }
func (m *MockRAGEngine) IndexBatch(ctx context.Context, docs []*Document) error { return nil }
func (m *MockRAGEngine) DeleteDocument(ctx context.Context, docID string) error { return nil }

type ReportMockLogger struct {
	logging.Logger
}

func TestGenerateReport_Success(t *testing.T) {
	backend := new(MockBackend)
	prompt := new(MockPromptManager)
	rag := new(MockRAGEngine)
	config := NewStrategyGPTConfig()
	logger := new(ReportMockLogger)

	generator := NewReportGenerator(backend, prompt, rag, config, logger)

	rag.On("RetrieveAndRerank", mock.Anything, mock.Anything).Return(&RAGResult{}, nil)
	prompt.On("BuildPrompt", mock.Anything, mock.Anything, mock.Anything).Return(&BuiltPrompt{UserPrompt: "prompt"}, nil)
	backend.On("Predict", mock.Anything, mock.Anything).Return(&common.PredictResponse{
		Outputs: map[string][]byte{"text": []byte("# Title\n## Summary\nContent")},
	}, nil)

	req := &ReportRequest{Task: TaskFTO, Params: &PromptParams{UserQuery: "query"}, OutputFormat: FormatNarrative}
	report, err := generator.GenerateReport(context.Background(), req)

	assert.NoError(t, err)
	assert.Equal(t, "Title", report.Content.Title)
}
//Personal.AI order the ending

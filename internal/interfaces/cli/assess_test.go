package cli

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/turtacn/KeyIP-Intelligence/internal/application/portfolio"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
)

// MockValuationService is a mock implementation of ValuationService
type MockValuationService struct {
	mock.Mock
}

func (m *MockValuationService) Assess(ctx context.Context, req *portfolio.ValuationRequest) (*portfolio.CLIValuationResult, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*portfolio.CLIValuationResult), args.Error(1)
}

func (m *MockValuationService) AssessPortfolio(ctx context.Context, req *portfolio.PortfolioAssessRequest) (*portfolio.CLIPortfolioAssessResult, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*portfolio.CLIPortfolioAssessResult), args.Error(1)
}

// MockLogger is a mock implementation of logging.Logger
type MockLogger struct {
	mock.Mock
}

func (m *MockLogger) Debug(msg string, fields ...logging.Field) {
	m.Called(msg, fields)
}

func (m *MockLogger) Info(msg string, fields ...logging.Field) {
	m.Called(msg, fields)
}

func (m *MockLogger) Warn(msg string, fields ...logging.Field) {
	m.Called(msg, fields)
}

func (m *MockLogger) Error(msg string, fields ...logging.Field) {
	m.Called(msg, fields)
}

func (m *MockLogger) Fatal(msg string, fields ...logging.Field) {
	m.Called(msg, fields)
}

func (m *MockLogger) With(fields ...logging.Field) logging.Logger {
	args := m.Called(fields)
	if args.Get(0) == nil {
		return m
	}
	return args.Get(0).(logging.Logger)
}

func (m *MockLogger) WithContext(ctx context.Context) logging.Logger {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return m
	}
	return args.Get(0).(logging.Logger)
}

func (m *MockLogger) WithError(err error) logging.Logger {
	args := m.Called(err)
	if args.Get(0) == nil {
		return m
	}
	return args.Get(0).(logging.Logger)
}

func (m *MockLogger) Sync() error {
	args := m.Called()
	return args.Error(0)
}

func TestParsePatentNumbers(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{"Single patent", "CN115123456", []string{"CN115123456"}},
		{"Multiple patents", "CN115123456, US11987654", []string{"CN115123456", "US11987654"}},
		{"With spaces", " CN115123456 , US11987654 ", []string{"CN115123456", "US11987654"}},
		{"Empty string", "", []string{}},
		{"Trailing comma", "CN115123456,", []string{"CN115123456"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parsePatentNumbers(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsValidPatentNumber(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"Valid CN", "CN115123456", true},
		{"Valid US", "US11987654", true},
		{"Valid EP", "EP3456789", true},
		{"Valid JP", "JP2021123456", true},
		{"Valid KR", "KR1020210123456", true},
		{"Invalid prefix", "XX123456", false},
		{"Too short", "CN12", false},
		{"No prefix", "123456", false},
		{"Lowercase valid", "cn115123456", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidPatentNumber(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseDimensions(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expected  []string
		expectErr bool
	}{
		{"All dimensions", "technical,legal,commercial,strategic", []string{"technical", "legal", "commercial", "strategic"}, false},
		{"Partial dimensions", "technical,legal", []string{"technical", "legal"}, false},
		{"With spaces", " technical , legal ", []string{"technical", "legal"}, false},
		{"Invalid dimension", "technical,invalid", nil, true},
		{"Empty string", "", nil, true},
		{"Case insensitive", "Technical,LEGAL", []string{"technical", "legal"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseDimensions(tt.input)
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestIsValidOutputFormat(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"stdout", "stdout", true},
		{"json", "json", true},
		{"csv", "csv", true},
		{"uppercase", "JSON", true},
		{"invalid", "xml", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidOutputFormat(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatTableOutput(t *testing.T) {
	result := &portfolio.CLIValuationResult{
		Items: []*portfolio.CLIValuationItem{
			{
				PatentNumber:    "CN115123456",
				OverallScore:    85.5,
				TechnicalScore:  90.0,
				LegalScore:      80.0,
				CommercialScore: 85.0,
				StrategicScore:  87.0,
			},
		},
		AverageScore: 85.5,
		HighRiskPatents: []*portfolio.HighRiskInfo{
			{
				PatentNumber: "CN115123456",
				RiskReason:   "Expiring soon",
			},
		},
	}

	output := formatTableOutput(result)

	assert.Contains(t, output, "Patent Value Assessment")
	assert.Contains(t, output, "CN115123456")
	assert.Contains(t, output, "85.50")
	assert.Contains(t, output, "High Risk Patents")
	assert.Contains(t, output, "Expiring soon")
}

func TestFormatCSVOutput(t *testing.T) {
	result := &portfolio.CLIValuationResult{
		Items: []*portfolio.CLIValuationItem{
			{
				PatentNumber:    "CN115123456",
				OverallScore:    85.5,
				TechnicalScore:  90.0,
				LegalScore:      80.0,
				CommercialScore: 85.0,
				StrategicScore:  87.0,
			},
			{
				PatentNumber:    "US11987654",
				OverallScore:    35.0,
				TechnicalScore:  40.0,
				LegalScore:      25.0,
				CommercialScore: 30.0,
				StrategicScore:  45.0,
			},
		},
	}

	output, err := formatCSVOutput(result)
	require.NoError(t, err)

	lines := strings.Split(strings.TrimSpace(output), "\n")
	assert.Len(t, lines, 3) // header + 2 data rows

	assert.Contains(t, lines[0], "PatentNumber")
	assert.Contains(t, lines[1], "CN115123456")
	assert.Contains(t, lines[1], "LOW")
	assert.Contains(t, lines[2], "US11987654")
	assert.Contains(t, lines[2], "HIGH")
}

func TestWriteOutput_Stdout(t *testing.T) {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	content := "test output"
	err := writeOutput(content, "")

	w.Close()
	var buf bytes.Buffer
	buf.ReadFrom(r)
	os.Stdout = oldStdout

	assert.NoError(t, err)
	assert.Equal(t, content, buf.String())
}

func TestWriteOutput_File(t *testing.T) {
	tmpFile := "test_output.txt"
	defer os.Remove(tmpFile)

	content := "test output content"
	err := writeOutput(content, tmpFile)

	require.NoError(t, err)

	data, err := os.ReadFile(tmpFile)
	require.NoError(t, err)
	assert.Equal(t, content, string(data))
}

func TestHasHighRiskItems(t *testing.T) {
	tests := []struct {
		name     string
		result   *portfolio.CLIValuationResult
		expected bool
	}{
		{
			name: "With high risk",
			result: &portfolio.CLIValuationResult{
				HighRiskPatents: []*portfolio.HighRiskInfo{
					{PatentNumber: "CN115123456"},
				},
			},
			expected: true,
		},
		{
			name: "No high risk",
			result: &portfolio.CLIValuationResult{
				HighRiskPatents: []*portfolio.HighRiskInfo{},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasHighRiskItems(tt.result)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsHighRiskItem(t *testing.T) {
	tests := []struct {
		name     string
		item     *portfolio.CLIValuationItem
		expected bool
	}{
		{
			name: "Low overall score",
			item: &portfolio.CLIValuationItem{
				OverallScore:    35.0,
				LegalScore:      50.0,
				CommercialScore: 40.0,
			},
			expected: true,
		},
		{
			name: "Low legal score",
			item: &portfolio.CLIValuationItem{
				OverallScore:    60.0,
				LegalScore:      25.0,
				CommercialScore: 50.0,
			},
			expected: true,
		},
		{
			name: "Low commercial score",
			item: &portfolio.CLIValuationItem{
				OverallScore:    60.0,
				LegalScore:      50.0,
				CommercialScore: 20.0,
			},
			expected: true,
		},
		{
			name: "All scores acceptable",
			item: &portfolio.CLIValuationItem{
				OverallScore:    70.0,
				LegalScore:      65.0,
				CommercialScore: 60.0,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isHighRiskItem(tt.item)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatAssessOutput_UnknownFormat(t *testing.T) {
	result := &portfolio.CLIValuationResult{
		Items: []*portfolio.CLIValuationItem{},
	}

	_, err := formatAssessOutput(result, "xml")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown output format")
}

func TestHasHighRiskPortfolioItems(t *testing.T) {
	tests := []struct {
		name     string
		result   *portfolio.CLIPortfolioAssessResult
		expected bool
	}{
		{
			name: "High risk level",
			result: &portfolio.CLIPortfolioAssessResult{
				RiskLevel: string(common.RiskHigh),
			},
			expected: true,
		},
		{
			name: "Critical risk level",
			result: &portfolio.CLIPortfolioAssessResult{
				RiskLevel: string(common.RiskCritical),
			},
			expected: true,
		},
		{
			name: "Medium risk level",
			result: &portfolio.CLIPortfolioAssessResult{
				RiskLevel: string(common.RiskMedium),
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasHighRiskPortfolioItems(tt.result)
			assert.Equal(t, tt.expected, result)
		})
	}
}

//Personal.AI order the ending

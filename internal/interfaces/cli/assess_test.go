package cli

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestNewAssessCmd(t *testing.T) {
	cmd := NewAssessCmd()
	if cmd == nil {
		t.Fatal("NewAssessCmd should not return nil")
	}
	if cmd.Use != "assess" {
		t.Errorf("expected Use='assess', got %s", cmd.Use)
	}
	// Verify subcommands are registered
	subs := cmd.Commands()
	if len(subs) < 2 {
		t.Errorf("expected at least 2 subcommands, got %d", len(subs))
	}
}

func TestAssessPatentCmd_ValidSinglePatent(t *testing.T) {
	cmd := newAssessPatentCmd()
	cmd.SetArgs([]string{"--patent-number", "CN202110123456"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("execution failed: %v", err)
	}
}

func TestAssessPatentCmd_ValidMultiplePatents(t *testing.T) {
	cmd := newAssessPatentCmd()
	cmd.SetArgs([]string{"--patent-number", "CN123,US456,EP789"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("execution failed: %v", err)
	}
}

func TestAssessPatentCmd_InvalidPatentNumber(t *testing.T) {
	cmd := newAssessPatentCmd()
	cmd.SetArgs([]string{"--patent-number", "INVALID123"})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for invalid patent number")
	}
	if !strings.Contains(err.Error(), "invalid patent number") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestAssessPatentCmd_MissingRequiredFlag(t *testing.T) {
	cmd := newAssessPatentCmd()
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for missing required flag")
	}
}

func TestAssessPatentCmd_InvalidDimension(t *testing.T) {
	cmd := newAssessPatentCmd()
	cmd.SetArgs([]string{"--patent-number", "CN123", "--dimensions", "invalid"})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for invalid dimension")
	}
	if !strings.Contains(err.Error(), "invalid dimension") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestAssessPatentCmd_JSONOutput(t *testing.T) {
	tmpfile := t.TempDir() + "/output.json"
	cmd := newAssessPatentCmd()
	cmd.SetArgs([]string{"--patent-number", "CN123", "--output", "json", "--file", tmpfile})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("execution failed: %v", err)
	}

	content, err := os.ReadFile(tmpfile)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}

	// Verify JSON structure
	var result ValuationResult
	if err := json.Unmarshal(content, &result); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}
	if len(result.Patents) == 0 {
		t.Error("expected at least one patent in result")
	}
}

func TestAssessPatentCmd_CSVOutput(t *testing.T) {
	tmpfile := t.TempDir() + "/output.csv"
	cmd := newAssessPatentCmd()
	cmd.SetArgs([]string{"--patent-number", "CN123", "--output", "csv", "--file", tmpfile})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("execution failed: %v", err)
	}

	content, err := os.ReadFile(tmpfile)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}

	// Verify CSV has header and data
	lines := strings.Split(string(content), "\n")
	if len(lines) < 2 {
		t.Error("CSV should have header and at least one data row")
	}
	if !strings.Contains(lines[0], "Patent Number") {
		t.Error("CSV header should contain 'Patent Number'")
	}
}

func TestAssessPatentCmd_FileOutput(t *testing.T) {
	tmpfile := t.TempDir() + "/output.txt"

	cmd := newAssessPatentCmd()
	cmd.SetArgs([]string{"--patent-number", "CN123", "--file", tmpfile})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("execution failed: %v", err)
	}

	content, err := os.ReadFile(tmpfile)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}

	if len(content) == 0 {
		t.Error("output file should not be empty")
	}
	if !strings.Contains(string(content), "CN123") {
		t.Error("output should contain patent number")
	}
}

func TestAssessPortfolioCmd_ValidPortfolio(t *testing.T) {
	cmd := newAssessPortfolioCmd()
	cmd.SetArgs([]string{"--portfolio-id", "pf-123"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("execution failed: %v", err)
	}
}

func TestAssessPortfolioCmd_ServiceError(t *testing.T) {
	// Test missing required flag (simulates service error scenario)
	cmd := newAssessPortfolioCmd()
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for missing portfolio-id")
	}
}

func TestFormatAssessOutput_Table(t *testing.T) {
	result := &ValuationResult{
		Patents: []PatentValuation{
			{PatentNumber: "CN123", TechnicalScore: 85.0, LegalScore: 78.0, CommercialScore: 92.0, StrategicScore: 88.0, OverallScore: 85.75, RiskLevel: "LOW"},
		},
	}

	output, err := formatAssessOutput(result, "stdout")
	if err != nil {
		t.Fatalf("formatting failed: %v", err)
	}

	if !strings.Contains(output, "CN123") {
		t.Error("output should contain patent number")
	}
	if !strings.Contains(output, "85.75") {
		t.Error("output should contain overall score")
	}
	if !strings.Contains(output, "LOW") {
		t.Error("output should contain risk level")
	}
}

func TestFormatAssessOutput_UnknownFormat(t *testing.T) {
	result := &ValuationResult{}

	_, err := formatAssessOutput(result, "unknown")
	if err == nil {
		t.Error("expected error for unknown format")
	}
	if !strings.Contains(err.Error(), "unknown format") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestIsValidPatentNumber(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"CN202110123456", true},
		{"US11234567B2", true},
		{"EP3456789A1", true},
		{"JP2021123456", true},
		{"KR1020210123456", true},
		{"INVALID123", false},
		{"", false},
		{"XX123", false},
	}

	for _, tt := range tests {
		if got := isValidPatentNumber(tt.input); got != tt.expected {
			t.Errorf("isValidPatentNumber(%q) = %v, want %v", tt.input, got, tt.expected)
		}
	}
}

func TestIsValidDimension(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"technical", true},
		{"legal", true},
		{"commercial", true},
		{"strategic", true},
		{"invalid", false},
		{"", false},
	}

	for _, tt := range tests {
		if got := isValidDimension(tt.input); got != tt.expected {
			t.Errorf("isValidDimension(%q) = %v, want %v", tt.input, got, tt.expected)
		}
	}
}

func TestIsValidOutputFormat(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"stdout", true},
		{"json", true},
		{"csv", true},
		{"xml", false},
		{"", false},
	}

	for _, tt := range tests {
		if got := isValidOutputFormat(tt.input); got != tt.expected {
			t.Errorf("isValidOutputFormat(%q) = %v, want %v", tt.input, got, tt.expected)
		}
	}
}

func TestWriteOutput_ToFile(t *testing.T) {
	tmpfile := t.TempDir() + "/test_output.txt"
	content := "test content"

	err := writeOutput(content, tmpfile)
	if err != nil {
		t.Fatalf("writeOutput failed: %v", err)
	}

	data, err := os.ReadFile(tmpfile)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	if string(data) != content {
		t.Errorf("file content mismatch: got %q, want %q", string(data), content)
	}
}

func TestWriteOutput_InvalidPath(t *testing.T) {
	err := writeOutput("content", "/nonexistent/path/file.txt")
	if err == nil {
		t.Error("expected error for invalid path")
	}
}

//Personal.AI order the ending

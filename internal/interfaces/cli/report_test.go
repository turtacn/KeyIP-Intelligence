package cli

import (
	"strings"
	"testing"
)

func TestNewReportCmd(t *testing.T) {
	cmd := NewReportCmd()
	if cmd == nil {
		t.Fatal("NewReportCmd should return a command")
	}

	if cmd.Use != "report" {
		t.Errorf("expected Use='report', got %q", cmd.Use)
	}

	subs := cmd.Commands()
	if len(subs) < 3 {
		t.Errorf("expected at least 3 subcommands, got %d", len(subs))
	}

	// Verify subcommands exist
	hasGenerate, hasListTemplates, hasStatus := false, false, false
	for _, sub := range subs {
		switch sub.Use {
		case "generate":
			hasGenerate = true
		case "list-templates":
			hasListTemplates = true
		case "status":
			hasStatus = true
		}
	}

	if !hasGenerate {
		t.Error("expected 'generate' subcommand")
	}
	if !hasListTemplates {
		t.Error("expected 'list-templates' subcommand")
	}
	if !hasStatus {
		t.Error("expected 'status' subcommand")
	}
}

func TestListTemplatesCmd(t *testing.T) {
	listTemplatesType = ""

	cmd := newListTemplatesCmd()
	err := cmd.RunE(cmd, []string{})

	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

func TestListTemplatesCmd_FilterByType(t *testing.T) {
	listTemplatesType = "fto"

	cmd := newListTemplatesCmd()
	err := cmd.RunE(cmd, []string{})

	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

func TestStatusCmd_InProgress(t *testing.T) {
	statusJobID = "job-1234567890-test1234"

	cmd := newStatusCmd()
	err := cmd.RunE(cmd, []string{})

	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

func TestStatusCmd_Completed(t *testing.T) {
	statusJobID = "job-1234567890-aest1234" // Contains 'a' at position 10

	cmd := newStatusCmd()
	err := cmd.RunE(cmd, []string{})

	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

func TestStatusCmd_Failed(t *testing.T) {
	statusJobID = "job-1234567890-zest1234" // Contains 'z' at position 10

	cmd := newStatusCmd()
	err := cmd.RunE(cmd, []string{})

	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

func TestIsValidReportType(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"fto", true},
		{"infringement", true},
		{"portfolio", true},
		{"annual", true},
		{"invalid", false},
		{"", false},
	}

	for _, tt := range tests {
		result := isValidReportType(tt.input)
		if result != tt.expected {
			t.Errorf("isValidReportType(%q) = %v, want %v", tt.input, result, tt.expected)
		}
	}
}

func TestIsValidFormat(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"pdf", true},
		{"docx", true},
		{"json", true},
		{"html", false},
		{"", false},
	}

	for _, tt := range tests {
		result := isValidFormat(tt.input)
		if result != tt.expected {
			t.Errorf("isValidFormat(%q) = %v, want %v", tt.input, result, tt.expected)
		}
	}
}

func TestIsValidLanguage(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"zh", true},
		{"en", true},
		{"fr", false},
		{"", false},
	}

	for _, tt := range tests {
		result := isValidLanguage(tt.input)
		if result != tt.expected {
			t.Errorf("isValidLanguage(%q) = %v, want %v", tt.input, result, tt.expected)
		}
	}
}

func TestEstimateReportSize(t *testing.T) {
	tests := []struct {
		reportType string
		expected   int
	}{
		{"fto", 45},
		{"infringement", 35},
		{"portfolio", 120},
		{"annual", 80},
	}

	for _, tt := range tests {
		result := estimateReportSize(tt.reportType, "test-target")
		if result != tt.expected {
			t.Errorf("estimateReportSize(%q) = %d, want %d", tt.reportType, result, tt.expected)
		}
	}
}

func TestResolveOutputPath(t *testing.T) {
	path1 := resolveOutputPath(".", "fto", "pdf")
	if !strings.Contains(path1, "report_fto_") {
		t.Errorf("expected path to contain 'report_fto_', got: %s", path1)
	}
	if !strings.HasSuffix(path1, ".pdf") {
		t.Errorf("expected path to end with '.pdf', got: %s", path1)
	}

	// Test different formats
	path2 := resolveOutputPath(".", "portfolio", "docx")
	if !strings.HasSuffix(path2, ".docx") {
		t.Errorf("expected path to end with '.docx', got: %s", path2)
	}

	path3 := resolveOutputPath(".", "annual", "json")
	if !strings.HasSuffix(path3, ".json") {
		t.Errorf("expected path to end with '.json', got: %s", path3)
	}
}

func TestProgressBar(t *testing.T) {
	tests := []struct {
		progress int
		expected int // expected filled length
	}{
		{0, 0},
		{50, 15},
		{100, 30},
	}

	for _, tt := range tests {
		bar := progressBar(tt.progress)
		filled := strings.Count(bar, "█")
		if filled != tt.expected {
			t.Errorf("progressBar(%d) filled count = %d, want %d", tt.progress, filled, tt.expected)
		}
	}
}

func TestGenerateJobID(t *testing.T) {
	jobID := generateJobID()

	if !strings.HasPrefix(jobID, "job-") {
		t.Errorf("expected job ID to start with 'job-', got: %s", jobID)
	}

	if len(jobID) < 15 {
		t.Errorf("expected job ID length > 15, got: %d", len(jobID))
	}
}

//Personal.AI order the ending

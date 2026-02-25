package cli

import (
	"strings"
	"testing"
)

func TestNewSearchCmd(t *testing.T) {
	cmd := NewSearchCmd()
	if cmd == nil {
		t.Fatal("NewSearchCmd should return a command")
	}
	if cmd.Use != "search" {
		t.Errorf("expected Use='search', got %q", cmd.Use)
	}

	// Verify subcommands are registered
	subs := cmd.Commands()
	if len(subs) < 2 {
		t.Errorf("expected at least 2 subcommands, got %d", len(subs))
	}

	subNames := make(map[string]bool)
	for _, sub := range subs {
		subNames[sub.Use] = true
	}
	expectedSubs := []string{"molecule", "patent"}
	for _, name := range expectedSubs {
		if !subNames[name] {
			t.Errorf("expected subcommand %q not found", name)
		}
	}
}

func TestSearchMoleculeCmd_BySMILES(t *testing.T) {
	cmd := newSearchMoleculeCmd()
	cmd.SetArgs([]string{"--smiles", "c1ccccc1"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("execution failed: %v", err)
	}
}

func TestSearchMoleculeCmd_ByInChI(t *testing.T) {
	cmd := newSearchMoleculeCmd()
	cmd.SetArgs([]string{"--inchi", "InChI=1S/C6H6/c1-2-4-6-5-3-1/h1-6H"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("execution failed: %v", err)
	}
}

func TestSearchMoleculeCmd_BothSMILESAndInChI(t *testing.T) {
	cmd := newSearchMoleculeCmd()
	cmd.SetArgs([]string{"--smiles", "c1ccccc1", "--inchi", "InChI=1S/C6H6/c1-2-4-6-5-3-1/h1-6H"})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for providing both smiles and inchi")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestSearchMoleculeCmd_NeitherSMILESNorInChI(t *testing.T) {
	cmd := newSearchMoleculeCmd()
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when neither smiles nor inchi provided")
	}
	if !strings.Contains(err.Error(), "must be provided") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestSearchMoleculeCmd_InvalidThreshold_TooLow(t *testing.T) {
	cmd := newSearchMoleculeCmd()
	cmd.SetArgs([]string{"--smiles", "c1ccccc1", "--threshold", "-0.1"})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for threshold below 0")
	}
	if !strings.Contains(err.Error(), "threshold must be between") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestSearchMoleculeCmd_InvalidThreshold_TooHigh(t *testing.T) {
	cmd := newSearchMoleculeCmd()
	cmd.SetArgs([]string{"--smiles", "c1ccccc1", "--threshold", "1.5"})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for threshold above 1")
	}
	if !strings.Contains(err.Error(), "threshold must be between") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestSearchMoleculeCmd_CustomFingerprints(t *testing.T) {
	cmd := newSearchMoleculeCmd()
	cmd.SetArgs([]string{"--smiles", "c1ccccc1", "--fingerprints", "morgan,maccs"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("execution failed: %v", err)
	}
}

func TestSearchMoleculeCmd_InvalidFingerprint(t *testing.T) {
	cmd := newSearchMoleculeCmd()
	cmd.SetArgs([]string{"--smiles", "c1ccccc1", "--fingerprints", "invalid_fp"})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for invalid fingerprint type")
	}
	if !strings.Contains(err.Error(), "invalid fingerprint type") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestSearchMoleculeCmd_WithRiskAssessment(t *testing.T) {
	cmd := newSearchMoleculeCmd()
	cmd.SetArgs([]string{"--smiles", "c1ccccc1", "--include-risk"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("execution failed: %v", err)
	}
}

func TestSearchMoleculeCmd_MaxResultsLimit(t *testing.T) {
	cmd := newSearchMoleculeCmd()
	cmd.SetArgs([]string{"--smiles", "c1ccccc1", "--max-results", "600"})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for max-results above 500")
	}
	if !strings.Contains(err.Error(), "max-results must be between") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestSearchMoleculeCmd_EmptyResults(t *testing.T) {
	cmd := newSearchMoleculeCmd()
	cmd.SetArgs([]string{"--smiles", "c1ccccc1", "--threshold", "0.99"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("execution failed: %v", err)
	}
}

func TestSearchMoleculeCmd_JSONOutput(t *testing.T) {
	cmd := newSearchMoleculeCmd()
	cmd.SetArgs([]string{"--smiles", "c1ccccc1", "--output", "json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("execution failed: %v", err)
	}
}

func TestSearchPatentCmd_BasicQuery(t *testing.T) {
	cmd := newSearchPatentCmd()
	cmd.SetArgs([]string{"--query", "OLED emitter"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("execution failed: %v", err)
	}
}

func TestSearchPatentCmd_WithIPCFilter(t *testing.T) {
	cmd := newSearchPatentCmd()
	cmd.SetArgs([]string{"--query", "OLED", "--ipc", "H10K"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("execution failed: %v", err)
	}
}

func TestSearchPatentCmd_WithDateRange(t *testing.T) {
	cmd := newSearchPatentCmd()
	cmd.SetArgs([]string{"--query", "OLED", "--date-from", "2020-01-01", "--date-to", "2023-12-31"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("execution failed: %v", err)
	}
}

func TestSearchPatentCmd_InvalidDateFormat(t *testing.T) {
	cmd := newSearchPatentCmd()
	cmd.SetArgs([]string{"--query", "OLED", "--date-from", "01-01-2020"})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for invalid date format")
	}
	if !strings.Contains(err.Error(), "invalid date-from format") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestSearchPatentCmd_DateRangeInverted(t *testing.T) {
	cmd := newSearchPatentCmd()
	cmd.SetArgs([]string{"--query", "OLED", "--date-from", "2023-12-31", "--date-to", "2020-01-01"})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for inverted date range")
	}
	if !strings.Contains(err.Error(), "date-from cannot be later than date-to") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestSearchPatentCmd_SortByDate(t *testing.T) {
	cmd := newSearchPatentCmd()
	cmd.SetArgs([]string{"--query", "OLED", "--sort", "date"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("execution failed: %v", err)
	}
}

func TestSearchPatentCmd_SortByCitations(t *testing.T) {
	cmd := newSearchPatentCmd()
	cmd.SetArgs([]string{"--query", "OLED", "--sort", "citations"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("execution failed: %v", err)
	}
}

func TestSearchPatentCmd_InvalidSort(t *testing.T) {
	cmd := newSearchPatentCmd()
	cmd.SetArgs([]string{"--query", "OLED", "--sort", "invalid"})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for invalid sort option")
	}
	if !strings.Contains(err.Error(), "invalid sort option") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestSearchPatentCmd_ServiceError(t *testing.T) {
	// Test missing required flag (simulates service error scenario)
	cmd := newSearchPatentCmd()
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for missing query")
	}
}

func TestFormatMoleculeResults_Table(t *testing.T) {
	results := []MoleculeSearchResult{
		{Rank: 1, Similarity: 0.95, PatentNumber: "CN123", MoleculeName: "Test-Mol-1", SMILES: "c1ccccc1", RiskLevel: "LOW"},
	}

	output, err := formatMoleculeResults(results, "stdout", false)
	if err != nil {
		t.Fatalf("formatting failed: %v", err)
	}

	if !strings.Contains(output, "CN123") {
		t.Error("output should contain patent number")
	}
	if !strings.Contains(output, "0.95") {
		t.Error("output should contain similarity score")
	}
}

func TestFormatPatentResults_Table(t *testing.T) {
	results := []PatentSearchResult{
		{Rank: 1, Relevance: 0.95, PatentNumber: "CN123", Title: "Test Patent", ApplicationDate: "2021-01-01", IPC: "H10K50/11"},
	}

	output, err := formatPatentResults(results, "stdout")
	if err != nil {
		t.Fatalf("formatting failed: %v", err)
	}

	if !strings.Contains(output, "CN123") {
		t.Error("output should contain patent number")
	}
	if !strings.Contains(output, "H10K50/11") {
		t.Error("output should contain IPC classification")
	}
}

func TestColorizeRiskLevel_AllLevels(t *testing.T) {
	tests := []struct {
		level         string
		expectsColor  bool
		expectedColor string
	}{
		{"HIGH", true, "\033[31m"},    // Red
		{"MEDIUM", true, "\033[33m"},  // Yellow
		{"LOW", true, "\033[32m"},     // Green
		{"UNKNOWN", false, ""},
	}

	for _, tt := range tests {
		result := colorizeRiskLevel(tt.level)
		if tt.expectsColor {
			if !strings.Contains(result, tt.expectedColor) {
				t.Errorf("colorizeRiskLevel(%q) should contain %q, got %q", tt.level, tt.expectedColor, result)
			}
		}
		if !strings.Contains(result, tt.level) {
			t.Errorf("colorizeRiskLevel(%q) should contain level text, got %q", tt.level, result)
		}
	}
}

func TestIsValidFingerprint(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"morgan", true},
		{"topological", true},
		{"maccs", true},
		{"gnn", true},
		{"invalid", false},
		{"", false},
	}

	for _, tt := range tests {
		if got := isValidFingerprint(tt.input); got != tt.expected {
			t.Errorf("isValidFingerprint(%q) = %v, want %v", tt.input, got, tt.expected)
		}
	}
}

func TestIsValidSortOption(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"relevance", true},
		{"date", true},
		{"citations", true},
		{"invalid", false},
		{"", false},
	}

	for _, tt := range tests {
		if got := isValidSortOption(tt.input); got != tt.expected {
			t.Errorf("isValidSortOption(%q) = %v, want %v", tt.input, got, tt.expected)
		}
	}
}

func TestTruncateString(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"short", 10, "short"},
		{"this is a long string", 10, "this is..."},
		{"exactly10!", 10, "exactly10!"},
	}

	for _, tt := range tests {
		result := truncateString(tt.input, tt.maxLen)
		if result != tt.expected {
			t.Errorf("truncateString(%q, %d) = %q, want %q", tt.input, tt.maxLen, result, tt.expected)
		}
	}
}

func TestSearchCmd_Exists(t *testing.T) {
	if SearchCmd == nil {
		t.Error("SearchCmd should exist")
	}
}

//Personal.AI order the ending

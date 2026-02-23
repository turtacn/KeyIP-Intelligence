package cli

import (
	"testing"
)

// Simplified tests that focus on core functionality
func TestNewAssessCmd_Structure(t *testing.T) {
	cmd := NewAssessCmd()
	if cmd == nil {
		t.Fatal("NewAssessCmd should not return nil")
	}

	if cmd.Use != "assess" {
		t.Errorf("expected Use='assess', got %q", cmd.Use)
	}

	// Check for subcommands
	subcommands := cmd.Commands()
	if len(subcommands) < 2 {
		t.Errorf("expected at least 2 subcommands, got %d", len(subcommands))
	}
}

func TestPatentNumberValidation(t *testing.T) {
	tests := []struct {
		input string
		valid bool
	}{
		{"US1234567", true},
		{"CN9876543", true},
		{"EP2468135", true},
		{"JP5555555", true},
		{"KR1111111", true},
		{"INVALID", false},
		{"", false},
	}

	for _, tt := range tests {
		result := isValidPatentNumber(tt.input)
		if result != tt.valid {
			t.Errorf("isValidPatentNumber(%q) = %v, want %v", tt.input, result, tt.valid)
		}
	}
}

func TestDimensionValidation(t *testing.T) {
	tests := []struct {
		input string
		valid bool
	}{
		{"technical", true},
		{"legal", true},
		{"commercial", true},
		{"strategic", true},
		{"invalid", false},
	}

	for _, tt := range tests {
		result := isValidDimension(tt.input)
		if result != tt.valid {
			t.Errorf("isValidDimension(%q) = %v, want %v", tt.input, result, tt.valid)
		}
	}
}

func TestOutputFormatValidation(t *testing.T) {
	tests := []struct {
		input string
		valid bool
	}{
		{"stdout", true},
		{"json", true},
		{"csv", true},
		{"xml", false},
	}

	for _, tt := range tests {
		result := isValidOutputFormat(tt.input)
		if result != tt.valid {
			t.Errorf("isValidOutputFormat(%q) = %v, want %v", tt.input, result, tt.valid)
		}
	}
}

func TestFormatAssessOutput_JSON(t *testing.T) {
	result := &ValuationResult{
		Patents: []string{"US1234567"},
		Scores: map[string]float64{
			"technical": 75.5,
		},
		OverallScore: 79.4,
		RiskLevel:    "MEDIUM",
	}

	output, err := formatAssessOutput(result, "json")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(output) == 0 {
		t.Error("expected non-empty JSON output")
	}
}

func TestFormatAssessOutput_CSV(t *testing.T) {
	result := &ValuationResult{
		Patents: []string{"US1234567"},
		Scores: map[string]float64{
			"technical": 75.5,
		},
		OverallScore: 79.4,
		RiskLevel:    "MEDIUM",
	}

	output, err := formatAssessOutput(result, "csv")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(output) == 0 {
		t.Error("expected non-empty CSV output")
	}
}

func TestFormatAssessOutput_Table(t *testing.T) {
	result := &ValuationResult{
		Patents: []string{"US1234567"},
		Scores: map[string]float64{
			"technical": 75.5,
		},
		OverallScore: 79.4,
		RiskLevel:    "MEDIUM",
	}

	output, err := formatAssessOutput(result, "stdout")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(output) == 0 {
		t.Error("expected non-empty table output")
	}
}

func TestFormatAssessOutput_UnsupportedFormat(t *testing.T) {
	result := &ValuationResult{
		Patents:      []string{"US1234567"},
		Scores:       map[string]float64{},
		OverallScore: 79.4,
		RiskLevel:    "MEDIUM",
	}

	_, err := formatAssessOutput(result, "xml")
	if err == nil {
		t.Error("expected error for unsupported format")
	}
}

//Personal.AI order the ending

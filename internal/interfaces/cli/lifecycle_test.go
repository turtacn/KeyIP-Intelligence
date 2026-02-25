package cli

import (
	"strings"
	"testing"
)

func TestNewLifecycleCmd(t *testing.T) {
	cmd := NewLifecycleCmd()
	if cmd == nil {
		t.Fatal("NewLifecycleCmd should return a command")
	}

	if cmd.Use != "lifecycle" {
		t.Errorf("expected Use='lifecycle', got %q", cmd.Use)
	}

	subs := cmd.Commands()
	if len(subs) < 4 {
		t.Errorf("expected at least 4 subcommands, got %d", len(subs))
	}
}

func TestValidationFunctions(t *testing.T) {
	// Test isValidJurisdiction
	if !isValidJurisdiction("CN") {
		t.Error("CN should be valid")
	}
	if isValidJurisdiction("INVALID") {
		t.Error("INVALID should not be valid")
	}

	// Test isValidCurrency
	if !isValidCurrency("CNY") {
		t.Error("CNY should be valid")
	}
	if isValidCurrency("XXX") {
		t.Error("XXX should not be valid")
	}

	// Test isValidDeadlineStatus
	if !isValidDeadlineStatus("pending") {
		t.Error("pending should be valid")
	}
	if isValidDeadlineStatus("invalid") {
		t.Error("invalid should not be valid")
	}

	// Test isValidReminderAction
	if !isValidReminderAction("list") {
		t.Error("list should be valid")
	}
	if isValidReminderAction("delete") {
		t.Error("delete should not be valid")
	}
}

func TestFormatDeadlineTable(t *testing.T) {
	// Test empty list
	emptyDeadlines := []Deadline{}
	output := formatDeadlineTable(emptyDeadlines)
	if !strings.Contains(output, "Upcoming Deadlines") {
		t.Error("expected header in output")
	}

	// Test with deadlines - using actual Deadline struct fields
	deadlines := []Deadline{
		{
			PatentNumber: "CN123",
			Type:         "OA Response",
			DueDate:      "2024-12-15",
			UrgencyLevel: "CRITICAL",
		},
	}

	output = formatDeadlineTable(deadlines)
	if !strings.Contains(output, "CN123") {
		t.Error("expected patent number in output")
	}
	if !strings.Contains(output, "CRITICAL") {
		t.Error("expected urgency level in output")
	}
}

func TestFormatAnnuityTable(t *testing.T) {
	annuities := []AnnuityDetail{
		{Year: 2024, Amount: 12000, Currency: "CNY", DueDate: "2024-01-31"},
	}

	output := formatAnnuityTable(annuities, "CNY")
	if !strings.Contains(output, "2024") {
		t.Error("expected year in output")
	}
	if !strings.Contains(output, "CNY") {
		t.Error("expected currency in output")
	}
}

func TestFormatAnnuityTable_WithForecast(t *testing.T) {
	annuities := []AnnuityDetail{
		{Year: 2024, Amount: 12000, Currency: "USD", DueDate: "2024-01-31"},
		{Year: 2025, Amount: 13200, Currency: "USD", DueDate: "2025-01-31"},
	}

	output := formatAnnuityTable(annuities, "USD")
	if !strings.Contains(output, "2025") {
		t.Error("expected forecast year in output")
	}
}

func TestIsValidJurisdiction(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"CN", true},
		{"US", true},
		{"EP", true},
		{"JP", true},
		{"KR", true},
		{"XX", false},
		{"", false},
	}

	for _, tt := range tests {
		if got := isValidJurisdiction(tt.input); got != tt.expected {
			t.Errorf("isValidJurisdiction(%q) = %v, want %v", tt.input, got, tt.expected)
		}
	}
}

func TestIsValidCurrency(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"CNY", true},
		{"USD", true},
		{"EUR", true},
		{"JPY", true},
		{"KRW", true},
		{"GBP", false},
		{"", false},
	}

	for _, tt := range tests {
		if got := isValidCurrency(tt.input); got != tt.expected {
			t.Errorf("isValidCurrency(%q) = %v, want %v", tt.input, got, tt.expected)
		}
	}
}

func TestIsValidDeadlineStatus(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"pending", true},
		{"overdue", true},
		{"completed", true},
		{"invalid", false},
		{"", false},
	}

	for _, tt := range tests {
		if got := isValidDeadlineStatus(tt.input); got != tt.expected {
			t.Errorf("isValidDeadlineStatus(%q) = %v, want %v", tt.input, got, tt.expected)
		}
	}
}

func TestIsValidReminderAction(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"list", true},
		{"add", true},
		{"remove", true},
		{"delete", false},
		{"", false},
	}

	for _, tt := range tests {
		if got := isValidReminderAction(tt.input); got != tt.expected {
			t.Errorf("isValidReminderAction(%q) = %v, want %v", tt.input, got, tt.expected)
		}
	}
}

//Personal.AI order the ending

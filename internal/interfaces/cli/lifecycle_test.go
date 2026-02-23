package cli

import (
	"strings"
	"testing"
	"time"
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
	emptyDeadlines := []*Deadline{}
	output := formatDeadlineTable(emptyDeadlines)
	if !strings.Contains(output, "No upcoming deadlines") {
		t.Error("expected empty message")
	}

	// Test with deadlines
	deadlines := []*Deadline{
		{
			PatentNumber:  "CN123",
			Jurisdiction:  "CN",
			Type:          "OA Response",
			DueDate:       time.Now().AddDate(0, 0, 15),
			DaysRemaining: 15,
			Urgency:       "CRITICAL",
			Status:        "pending",
		},
	}

	output = formatDeadlineTable(deadlines)
	if !strings.Contains(output, "CN123") {
		t.Error("expected patent number in output")
	}
	if !strings.Contains(output, "CRITICAL") {
		t.Error("expected urgency in output")
	}
}

func TestFormatAnnuityTable(t *testing.T) {
	annuities := []*AnnuityDetail{
		{Year: 2024, Amount: 12000, Currency: "CNY", Status: "unpaid"},
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
	annuities := []*AnnuityDetail{
		{Year: 2024, Amount: 12000, Currency: "USD", Status: "unpaid"},
		{Year: 2025, Amount: 13200, Currency: "USD", Status: "forecast"},
	}

	output := formatAnnuityTable(annuities, "USD")
	if !strings.Contains(output, "forecast") {
		t.Error("expected forecast status in output")
	}
}

//Personal.AI order the ending

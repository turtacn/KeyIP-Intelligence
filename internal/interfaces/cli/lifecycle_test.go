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

	// Verify subcommand names
	subNames := make(map[string]bool)
	for _, sub := range subs {
		subNames[sub.Use] = true
	}
	expectedSubs := []string{"deadlines", "annuity", "sync-status", "reminders"}
	for _, name := range expectedSubs {
		if !subNames[name] {
			t.Errorf("expected subcommand %q not found", name)
		}
	}
}

func TestLifecycleDeadlinesCmd_DefaultFlags(t *testing.T) {
	cmd := newDeadlinesCmd()
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("execution failed: %v", err)
	}
}

func TestLifecycleDeadlinesCmd_WithJurisdiction(t *testing.T) {
	cmd := newDeadlinesCmd()
	cmd.SetArgs([]string{"--jurisdiction", "CN,US"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("execution failed: %v", err)
	}
}

func TestLifecycleDeadlinesCmd_WithPatentNumber(t *testing.T) {
	cmd := newDeadlinesCmd()
	cmd.SetArgs([]string{"--patent-number", "CN202110123456"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("execution failed: %v", err)
	}
}

func TestLifecycleDeadlinesCmd_InvalidJurisdiction(t *testing.T) {
	cmd := newDeadlinesCmd()
	cmd.SetArgs([]string{"--jurisdiction", "XX"})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for invalid jurisdiction")
	}
	if !strings.Contains(err.Error(), "invalid jurisdiction") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestLifecycleDeadlinesCmd_InvalidDaysAhead(t *testing.T) {
	tests := []struct {
		name     string
		daysFlag string
	}{
		{"too low", "0"},
		{"too high", "400"},
		{"negative", "-10"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newDeadlinesCmd()
			cmd.SetArgs([]string{"--days-ahead", tt.daysFlag})

			err := cmd.Execute()
			if err == nil {
				t.Error("expected error for invalid days-ahead")
			}
			if !strings.Contains(err.Error(), "days-ahead must be between") {
				t.Errorf("unexpected error message: %v", err)
			}
		})
	}
}

func TestLifecycleDeadlinesCmd_EmptyResult(t *testing.T) {
	cmd := newDeadlinesCmd()
	cmd.SetArgs([]string{"--patent-number", "NONEXISTENT"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("execution failed: %v", err)
	}
}

func TestLifecycleDeadlinesCmd_SortOrder(t *testing.T) {
	deadlines := []Deadline{
		{PatentNumber: "CN1", UrgencyLevel: "NORMAL", DueDate: "2024-01-01"},
		{PatentNumber: "CN2", UrgencyLevel: "CRITICAL", DueDate: "2024-01-02"},
		{PatentNumber: "CN3", UrgencyLevel: "WARNING", DueDate: "2024-01-01"},
	}

	sortDeadlinesByUrgency(deadlines)

	// CRITICAL should come first
	if deadlines[0].UrgencyLevel != "CRITICAL" {
		t.Errorf("expected CRITICAL first, got %s", deadlines[0].UrgencyLevel)
	}
	// WARNING should be second
	if deadlines[1].UrgencyLevel != "WARNING" {
		t.Errorf("expected WARNING second, got %s", deadlines[1].UrgencyLevel)
	}
	// NORMAL should be last
	if deadlines[2].UrgencyLevel != "NORMAL" {
		t.Errorf("expected NORMAL last, got %s", deadlines[2].UrgencyLevel)
	}
}

func TestLifecycleAnnuityCmd_SinglePatent(t *testing.T) {
	cmd := newAnnuityCmd()
	cmd.SetArgs([]string{"--patent-number", "CN123"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("execution failed: %v", err)
	}
}

func TestLifecycleAnnuityCmd_WithForecast(t *testing.T) {
	cmd := newAnnuityCmd()
	cmd.SetArgs([]string{"--patent-number", "CN123", "--include-forecast"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("execution failed: %v", err)
	}
}

func TestLifecycleAnnuityCmd_InvalidCurrency(t *testing.T) {
	cmd := newAnnuityCmd()
	cmd.SetArgs([]string{"--patent-number", "CN123", "--currency", "GBP"})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for invalid currency")
	}
	if !strings.Contains(err.Error(), "invalid currency") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestLifecycleAnnuityCmd_MissingPatentNumber(t *testing.T) {
	cmd := newAnnuityCmd()
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for missing patent-number")
	}
}

func TestLifecycleSyncStatusCmd_FullSync(t *testing.T) {
	cmd := newSyncStatusCmd()
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("execution failed: %v", err)
	}
}

func TestLifecycleSyncStatusCmd_DryRun(t *testing.T) {
	cmd := newSyncStatusCmd()
	cmd.SetArgs([]string{"--dry-run"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("execution failed: %v", err)
	}
}

func TestLifecycleSyncStatusCmd_PartialFailure(t *testing.T) {
	cmd := newSyncStatusCmd()
	cmd.SetArgs([]string{"--patent-number", "CN123"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("execution failed: %v", err)
	}
}

func TestLifecycleRemindersCmd_List(t *testing.T) {
	cmd := newRemindersCmd()
	cmd.SetArgs([]string{"--action", "list"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("execution failed: %v", err)
	}
}

func TestLifecycleRemindersCmd_Add(t *testing.T) {
	cmd := newRemindersCmd()
	cmd.SetArgs([]string{"--action", "add", "--patent-number", "CN123", "--channels", "email,wechat"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("execution failed: %v", err)
	}
}

func TestLifecycleRemindersCmd_Remove(t *testing.T) {
	cmd := newRemindersCmd()
	cmd.SetArgs([]string{"--action", "remove", "--patent-number", "CN123"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("execution failed: %v", err)
	}
}

func TestLifecycleRemindersCmd_InvalidAction(t *testing.T) {
	cmd := newRemindersCmd()
	cmd.SetArgs([]string{"--action", "delete"})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for invalid action")
	}
	if !strings.Contains(err.Error(), "invalid action") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestFormatDeadlineTable_ColorCoding(t *testing.T) {
	deadlines := []Deadline{
		{PatentNumber: "CN1", Type: "OA", DueDate: "2024-12-15", UrgencyLevel: "CRITICAL", DaysRemaining: 5},
		{PatentNumber: "CN2", Type: "Fee", DueDate: "2024-12-20", UrgencyLevel: "WARNING", DaysRemaining: 10},
		{PatentNumber: "CN3", Type: "Validation", DueDate: "2024-12-25", UrgencyLevel: "NORMAL", DaysRemaining: 15},
	}

	output := formatDeadlineTable(deadlines)

	// Check for ANSI color codes
	if !strings.Contains(output, "\033[31m") { // Red for CRITICAL
		t.Error("expected red color for CRITICAL")
	}
	if !strings.Contains(output, "\033[33m") { // Yellow for WARNING
		t.Error("expected yellow color for WARNING")
	}
	if !strings.Contains(output, "\033[32m") { // Green for NORMAL
		t.Error("expected green color for NORMAL")
	}
}

func TestFormatAnnuityTable_MultiCurrency(t *testing.T) {
	currencies := []string{"CNY", "USD", "EUR", "JPY", "KRW"}

	for _, curr := range currencies {
		annuities := []AnnuityDetail{
			{Year: 2024, Amount: getAnnuityAmount(curr, 2024), Currency: curr, DueDate: "2024-01-31"},
		}

		output := formatAnnuityTable(annuities, curr)
		if !strings.Contains(output, curr) {
			t.Errorf("expected currency %s in output", curr)
		}
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

func TestIsValidChannel(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"email", true},
		{"wechat", true},
		{"sms", true},
		{"slack", false},
		{"", false},
	}

	for _, tt := range tests {
		if got := isValidChannel(tt.input); got != tt.expected {
			t.Errorf("isValidChannel(%q) = %v, want %v", tt.input, got, tt.expected)
		}
	}
}

func TestGetAnnuityAmount(t *testing.T) {
	// Test base amounts for 2024
	tests := []struct {
		currency string
		year     int
		minVal   float64
	}{
		{"CNY", 2024, 12000.0},
		{"USD", 2024, 1800.0},
		{"EUR", 2024, 1600.0},
	}

	for _, tt := range tests {
		amount := getAnnuityAmount(tt.currency, tt.year)
		if amount < tt.minVal {
			t.Errorf("getAnnuityAmount(%q, %d) = %.2f, expected >= %.2f", tt.currency, tt.year, amount, tt.minVal)
		}
	}

	// Test that amounts increase over years
	amount2024 := getAnnuityAmount("CNY", 2024)
	amount2025 := getAnnuityAmount("CNY", 2025)
	if amount2025 <= amount2024 {
		t.Errorf("expected amount to increase: 2024=%.2f, 2025=%.2f", amount2024, amount2025)
	}
}

//Personal.AI order the ending

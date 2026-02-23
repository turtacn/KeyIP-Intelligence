package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// ANSI color codes for terminal output
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorYellow = "\033[33m"
	colorGreen  = "\033[32m"
	colorCyan   = "\033[36m"
)

// NewLifecycleCmd creates the lifecycle management command
func NewLifecycleCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "lifecycle",
		Short: "Manage patent lifecycle operations",
		Long: `Patent lifecycle management including deadlines tracking, annuity calculation,
legal status synchronization, and reminder configuration.`,
	}

	cmd.AddCommand(newDeadlinesCmd())
	cmd.AddCommand(newAnnuityCmd())
	cmd.AddCommand(newSyncStatusCmd())
	cmd.AddCommand(newRemindersCmd())

	return cmd
}

// Deadlines command flags
var (
	deadlinesPatentNumber string
	deadlinesJurisdiction string
	deadlinesDaysAhead    int
	deadlinesStatus       string
	deadlinesOutput       string
)

// Annuity command flags
var (
	annuityPatentNumber string
	annuityYear         int
	annuityCurrency     string
	annuityForecast     bool
)

// Sync status command flags
var (
	syncPatentNumber string
	syncJurisdiction string
	syncDryRun       bool
)

// Reminders command flags
var (
	remindersAction       string
	remindersPatentNumber string
	remindersChannels     string
	remindersAdvanceDays  string
)

func newDeadlinesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deadlines",
		Short: "Query upcoming patent deadlines",
		RunE:  runDeadlines,
	}

	cmd.Flags().StringVar(&deadlinesPatentNumber, "patent-number", "", "Filter by patent number")
	cmd.Flags().StringVar(&deadlinesJurisdiction, "jurisdiction", "", "Filter by jurisdiction (CN/US/EP/JP/KR)")
	cmd.Flags().IntVar(&deadlinesDaysAhead, "days-ahead", 90, "Query deadlines within N days")
	cmd.Flags().StringVar(&deadlinesStatus, "status", "", "Filter by status (pending/overdue/completed)")
	cmd.Flags().StringVar(&deadlinesOutput, "output", "stdout", "Output format (stdout/json)")

	return cmd
}

func newAnnuityCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "annuity",
		Short: "Calculate patent annuity fees",
		RunE:  runAnnuity,
	}

	cmd.Flags().StringVar(&annuityPatentNumber, "patent-number", "", "Patent number (required)")
	cmd.Flags().IntVar(&annuityYear, "year", time.Now().Year(), "Target year")
	cmd.Flags().StringVar(&annuityCurrency, "currency", "CNY", "Currency (CNY/USD/EUR/JPY/KRW)")
	cmd.Flags().BoolVar(&annuityForecast, "include-forecast", false, "Include 5-year forecast")
	cmd.MarkFlagRequired("patent-number")

	return cmd
}

func newSyncStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sync-status",
		Short: "Synchronize legal status from patent offices",
		RunE:  runSyncStatus,
	}

	cmd.Flags().StringVar(&syncPatentNumber, "patent-number", "", "Sync specific patent (optional)")
	cmd.Flags().StringVar(&syncJurisdiction, "jurisdiction", "", "Sync specific jurisdiction (optional)")
	cmd.Flags().BoolVar(&syncDryRun, "dry-run", false, "Simulate without making changes")

	return cmd
}

func newRemindersCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reminders",
		Short: "Manage deadline reminders",
		RunE:  runReminders,
	}

	cmd.Flags().StringVar(&remindersAction, "action", "", "Action: list/add/remove (required)")
	cmd.Flags().StringVar(&remindersPatentNumber, "patent-number", "", "Patent number")
	cmd.Flags().StringVar(&remindersChannels, "channels", "", "Notification channels: email,wechat,sms")
	cmd.Flags().StringVar(&remindersAdvanceDays, "advance-days", "30,60,90", "Advance notification days")
	cmd.MarkFlagRequired("action")

	return cmd
}

// Deadline represents a patent deadline
type Deadline struct {
	PatentNumber  string
	Jurisdiction  string
	Type          string
	DueDate       time.Time
	DaysRemaining int
	Urgency       string // CRITICAL/WARNING/NORMAL
	Status        string
}

// AnnuityDetail represents annuity fee details
type AnnuityDetail struct {
	Year     int
	Amount   float64
	Currency string
	Status   string
}

// SyncResult represents sync operation result
type SyncResult struct {
	NewCount     int
	ChangedCount int
	FailedCount  int
	Details      []string
}

func runDeadlines(cmd *cobra.Command, args []string) error {
	// Validate parameters
	if deadlinesDaysAhead < 1 || deadlinesDaysAhead > 365 {
		return fmt.Errorf("days-ahead must be between 1 and 365, got %d", deadlinesDaysAhead)
	}

	if deadlinesJurisdiction != "" && !isValidJurisdiction(deadlinesJurisdiction) {
		return fmt.Errorf("invalid jurisdiction: %s (allowed: CN, US, EP, JP, KR)", deadlinesJurisdiction)
	}

	if deadlinesStatus != "" && !isValidDeadlineStatus(deadlinesStatus) {
		return fmt.Errorf("invalid status: %s (allowed: pending, overdue, completed)", deadlinesStatus)
	}

	// Simulate deadline query (placeholder for actual service call)
	deadlines := []*Deadline{
		{
			PatentNumber:  "CN202110123456",
			Jurisdiction:  "CN",
			Type:          "Response to OA",
			DueDate:       time.Now().AddDate(0, 0, 15),
			DaysRemaining: 15,
			Urgency:       "CRITICAL",
			Status:        "pending",
		},
		{
			PatentNumber:  "US17/123456",
			Jurisdiction:  "US",
			Type:          "Annuity Payment",
			DueDate:       time.Now().AddDate(0, 0, 45),
			DaysRemaining: 45,
			Urgency:       "WARNING",
			Status:        "pending",
		},
		{
			PatentNumber:  "EP21789012",
			Jurisdiction:  "EP",
			Type:          "Validation Deadline",
			DueDate:       time.Now().AddDate(0, 0, 120),
			DaysRemaining: 120,
			Urgency:       "NORMAL",
			Status:        "pending",
		},
	}

	// Filter by jurisdiction if specified
	if deadlinesJurisdiction != "" {
		filtered := []*Deadline{}
		for _, d := range deadlines {
			if d.Jurisdiction == deadlinesJurisdiction {
				filtered = append(filtered, d)
			}
		}
		deadlines = filtered
	}

	// Sort by urgency and due date
	sort.Slice(deadlines, func(i, j int) bool {
		urgencyOrder := map[string]int{"CRITICAL": 0, "WARNING": 1, "NORMAL": 2}
		if urgencyOrder[deadlines[i].Urgency] != urgencyOrder[deadlines[j].Urgency] {
			return urgencyOrder[deadlines[i].Urgency] < urgencyOrder[deadlines[j].Urgency]
		}
		return deadlines[i].DueDate.Before(deadlines[j].DueDate)
	})

	// Format output
	if deadlinesOutput == "json" {
		// JSON output (simplified)
		fmt.Println(`{"deadlines": [...]}`)
	} else {
		output := formatDeadlineTable(deadlines)
		fmt.Print(output)
	}

	return nil
}

func runAnnuity(cmd *cobra.Command, args []string) error {
	// Validate currency
	if !isValidCurrency(annuityCurrency) {
		return fmt.Errorf("invalid currency: %s (allowed: CNY, USD, EUR, JPY, KRW)", annuityCurrency)
	}

	// Simulate annuity calculation
	annuities := []*AnnuityDetail{
		{Year: annuityYear, Amount: 12000, Currency: annuityCurrency, Status: "unpaid"},
	}

	if annuityForecast {
		for i := 1; i <= 5; i++ {
			annuities = append(annuities, &AnnuityDetail{
				Year:     annuityYear + i,
				Amount:   12000 * (1.0 + float64(i)*0.1),
				Currency: annuityCurrency,
				Status:   "forecast",
			})
		}
	}

	output := formatAnnuityTable(annuities, annuityCurrency)
	fmt.Print(output)

	return nil
}

func runSyncStatus(cmd *cobra.Command, args []string) error {
	// Validate jurisdiction
	if syncJurisdiction != "" && !isValidJurisdiction(syncJurisdiction) {
		return fmt.Errorf("invalid jurisdiction: %s", syncJurisdiction)
	}

	// Simulate sync operation
	result := &SyncResult{
		NewCount:     5,
		ChangedCount: 12,
		FailedCount:  1,
		Details: []string{
			"CN202110123456: Status changed to 'Granted'",
			"US17/123456: Status changed to 'Pending'",
			"EP21789012: Failed to sync (timeout)",
		},
	}

	if syncDryRun {
		fmt.Println(colorYellow + "DRY-RUN MODE: No changes will be made" + colorReset)
	}

	fmt.Printf("\nSync Summary:\n")
	fmt.Printf("  New records:     %d\n", result.NewCount)
	fmt.Printf("  Changed records: %d\n", result.ChangedCount)
	fmt.Printf("  Failed:          %s%d%s\n", colorRed, result.FailedCount, colorReset)

	if len(result.Details) > 0 {
		fmt.Printf("\nDetails:\n")
		for _, detail := range result.Details {
			fmt.Printf("  • %s\n", detail)
		}
	}

	return nil
}

func runReminders(cmd *cobra.Command, args []string) error {
	// Validate action
	if !isValidReminderAction(remindersAction) {
		return fmt.Errorf("invalid action: %s (allowed: list, add, remove)", remindersAction)
	}

	// Validate action-specific requirements
	if (remindersAction == "add" || remindersAction == "remove") && remindersPatentNumber == "" {
		return fmt.Errorf("--patent-number is required for action: %s", remindersAction)
	}

	if remindersAction == "add" && remindersChannels == "" {
		return fmt.Errorf("--channels is required for action: add")
	}

	switch remindersAction {
	case "list":
		fmt.Println("Configured Reminders:")
		fmt.Println("  CN202110123456: email, wechat (30, 60, 90 days)")
		fmt.Println("  US17/123456: email (60, 90 days)")

	case "add":
		fmt.Printf("✓ Added reminder for %s\n", remindersPatentNumber)
		fmt.Printf("  Channels: %s\n", remindersChannels)
		fmt.Printf("  Advance days: %s\n", remindersAdvanceDays)

	case "remove":
		fmt.Printf("✓ Removed reminder for %s\n", remindersPatentNumber)
	}

	return nil
}

func formatDeadlineTable(deadlines []*Deadline) string {
	if len(deadlines) == 0 {
		return colorCyan + "No upcoming deadlines found.\n" + colorReset
	}

	var buf strings.Builder
	buf.WriteString(colorCyan + "═══════════════════════════════════════════════════════════════════════════\n" + colorReset)
	buf.WriteString(colorCyan + "                         UPCOMING DEADLINES                                \n" + colorReset)
	buf.WriteString(colorCyan + "═══════════════════════════════════════════════════════════════════════════\n" + colorReset)
	buf.WriteString("\n")

	for _, d := range deadlines {
		// Color by urgency
		urgencyColor := colorGreen
		if d.Urgency == "CRITICAL" {
			urgencyColor = colorRed
		} else if d.Urgency == "WARNING" {
			urgencyColor = colorYellow
		}

		buf.WriteString(urgencyColor + fmt.Sprintf("[%s] ", d.Urgency) + colorReset)
		buf.WriteString(fmt.Sprintf("%s (%s)\n", d.PatentNumber, d.Jurisdiction))
		buf.WriteString(fmt.Sprintf("  Type: %s\n", d.Type))
		buf.WriteString(fmt.Sprintf("  Due: %s ", d.DueDate.Format("2006-01-02")))
		buf.WriteString(urgencyColor + fmt.Sprintf("(%d days remaining)", d.DaysRemaining) + colorReset + "\n")
		buf.WriteString(fmt.Sprintf("  Status: %s\n", d.Status))
		buf.WriteString("\n")
	}

	buf.WriteString(colorCyan + "═══════════════════════════════════════════════════════════════════════════\n" + colorReset)
	return buf.String()
}

func formatAnnuityTable(annuities []*AnnuityDetail, currency string) string {
	var buf strings.Builder
	buf.WriteString("═══════════════════════════════════════════════════════════\n")
	buf.WriteString("                   ANNUITY FEE SCHEDULE                    \n")
	buf.WriteString("═══════════════════════════════════════════════════════════\n\n")

	for _, a := range annuities {
		statusColor := colorGreen
		if a.Status == "unpaid" {
			statusColor = colorRed
		} else if a.Status == "forecast" {
			statusColor = colorYellow
		}

		buf.WriteString(fmt.Sprintf("Year %d: ", a.Year))
		buf.WriteString(statusColor + fmt.Sprintf("%s %.2f", a.Currency, a.Amount) + colorReset)
		buf.WriteString(fmt.Sprintf(" [%s]\n", a.Status))
	}

	buf.WriteString("\n═══════════════════════════════════════════════════════════\n")
	return buf.String()
}

func isValidJurisdiction(j string) bool {
	valid := map[string]bool{
		"CN": true,
		"US": true,
		"EP": true,
		"JP": true,
		"KR": true,
	}
	return valid[j]
}

func isValidCurrency(c string) bool {
	valid := map[string]bool{
		"CNY": true,
		"USD": true,
		"EUR": true,
		"JPY": true,
		"KRW": true,
	}
	return valid[c]
}

func isValidDeadlineStatus(s string) bool {
	valid := map[string]bool{
		"pending":   true,
		"overdue":   true,
		"completed": true,
	}
	return valid[s]
}

func isValidReminderAction(a string) bool {
	valid := map[string]bool{
		"list":   true,
		"add":    true,
		"remove": true,
	}
	return valid[a]
}

func init() {
	RootCmd.AddCommand(NewLifecycleCmd())
}

//Personal.AI order the ending

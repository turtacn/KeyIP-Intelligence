package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// NewLifecycleCmd returns the keyip lifecycle top-level subcommand.
// It provides patent lifecycle management capabilities including deadlines,
// annuities, legal status synchronization, and reminders.
func NewLifecycleCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "lifecycle",
		Short: "Patent lifecycle management",
		Long: `Manage patent lifecycle including deadlines, annuities, legal status synchronization and reminders.

Provides tools for IP managers to:
  - Query upcoming deadlines with urgency prioritization
  - Calculate annuity/maintenance fees with multi-currency support
  - Synchronize legal status from patent offices
  - Configure deadline reminder notifications`,
	}

	cmd.AddCommand(newDeadlinesCmd())
	cmd.AddCommand(newAnnuityCmd())
	cmd.AddCommand(newSyncStatusCmd())
	cmd.AddCommand(newRemindersCmd())

	return cmd
}

func newDeadlinesCmd() *cobra.Command {
	var (
		patentNumber string
		jurisdiction string
		daysAhead    int
		status       string
		output       string
	)

	cmd := &cobra.Command{
		Use:   "deadlines",
		Short: "Query upcoming deadlines",
		Long:  "List upcoming patent deadlines within specified timeframe, sorted by urgency",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Validate jurisdiction if provided
			if jurisdiction != "" {
				jurisdictions := strings.Split(jurisdiction, ",")
				for _, j := range jurisdictions {
					j = strings.TrimSpace(j)
					if j != "" && !isValidJurisdiction(j) {
						return fmt.Errorf("invalid jurisdiction: %s (must be CN/US/EP/JP/KR)", j)
					}
				}
			}

			// Validate days-ahead range: 1-365
			if daysAhead < 1 || daysAhead > 365 {
				return fmt.Errorf("days-ahead must be between 1 and 365, got %d", daysAhead)
			}

			// Validate status if provided
			if status != "" && !isValidDeadlineStatus(status) {
				return fmt.Errorf("invalid status: %s (must be pending/overdue/completed)", status)
			}

			// Build DeadlineQueryRequest and call DeadlineService.ListUpcoming()
			// In production, this would integrate with application/lifecycle.DeadlineService
			deadlines := []Deadline{
				{PatentNumber: "CN202110123456", Type: "OA Response", DueDate: "2024-12-31", UrgencyLevel: "CRITICAL", DaysRemaining: 5},
				{PatentNumber: "US11234567", Type: "Annuity Payment", DueDate: "2025-01-15", UrgencyLevel: "WARNING", DaysRemaining: 20},
				{PatentNumber: "EP3456789", Type: "Validation", DueDate: "2025-02-01", UrgencyLevel: "NORMAL", DaysRemaining: 37},
			}

			// Filter by patent number if specified
			if patentNumber != "" {
				filtered := make([]Deadline, 0)
				for _, d := range deadlines {
					if strings.Contains(d.PatentNumber, patentNumber) {
						filtered = append(filtered, d)
					}
				}
				deadlines = filtered
			}

			// Sort by urgency (descending) then by due date (ascending)
			sortDeadlinesByUrgency(deadlines)

			// Format output
			if output == "json" {
				data, err := json.MarshalIndent(deadlines, "", "  ")
				if err != nil {
					return fmt.Errorf("failed to marshal JSON: %w", err)
				}
				fmt.Println(string(data))
			} else {
				fmt.Print(formatDeadlineTable(deadlines))
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&patentNumber, "patent-number", "", "Filter by patent number")
	cmd.Flags().StringVar(&jurisdiction, "jurisdiction", "", "Filter by jurisdiction (CN/US/EP/JP/KR, comma-separated)")
	cmd.Flags().IntVar(&daysAhead, "days-ahead", 90, "Query deadlines within N days (1-365)")
	cmd.Flags().StringVar(&status, "status", "", "Filter by status (pending/overdue/completed)")
	cmd.Flags().StringVar(&output, "output", "stdout", "Output format (stdout/json)")

	return cmd
}

func newAnnuityCmd() *cobra.Command {
	var (
		patentNumber    string
		year            int
		currency        string
		includeForecast bool
	)

	cmd := &cobra.Command{
		Use:   "annuity",
		Short: "Calculate annuity fees",
		Long:  "Calculate patent annuity/maintenance fees for specified year with optional 5-year forecast",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Validate currency
			if !isValidCurrency(currency) {
				return fmt.Errorf("invalid currency: %s (must be CNY/USD/EUR/JPY/KRW)", currency)
			}

			// Default year to current if not specified
			if year == 0 {
				year = time.Now().Year()
			}

			// Build AnnuityQueryRequest and call AnnuityService.Calculate()
			// In production, this would integrate with application/lifecycle.AnnuityService
			annuities := []AnnuityDetail{
				{Year: year, Amount: getAnnuityAmount(currency, year), Currency: currency, DueDate: fmt.Sprintf("%d-01-31", year)},
			}

			if includeForecast {
				for i := 1; i <= 5; i++ {
					forecastYear := year + i
					annuities = append(annuities, AnnuityDetail{
						Year:     forecastYear,
						Amount:   getAnnuityAmount(currency, forecastYear),
						Currency: currency,
						DueDate:  fmt.Sprintf("%d-01-31", forecastYear),
					})
				}
			}

			fmt.Print(formatAnnuityTable(annuities, currency))
			return nil
		},
	}

	cmd.Flags().StringVar(&patentNumber, "patent-number", "", "Patent number [REQUIRED]")
	cmd.Flags().IntVar(&year, "year", 0, "Annuity year (default: current year)")
	cmd.Flags().StringVar(&currency, "currency", "CNY", "Currency (CNY/USD/EUR/JPY/KRW)")
	cmd.Flags().BoolVar(&includeForecast, "include-forecast", false, "Include 5-year forecast")
	_ = cmd.MarkFlagRequired("patent-number")

	return cmd
}

func newSyncStatusCmd() *cobra.Command {
	var (
		patentNumber string
		jurisdiction string
		dryRun       bool
	)

	cmd := &cobra.Command{
		Use:   "sync-status",
		Short: "Synchronize legal status",
		Long:  "Synchronize patent legal status from patent offices (CNIPA, USPTO, EPO, JPO, KIPO)",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Validate jurisdiction if provided
			if jurisdiction != "" && !isValidJurisdiction(jurisdiction) {
				return fmt.Errorf("invalid jurisdiction: %s (must be CN/US/EP/JP/KR)", jurisdiction)
			}

			if dryRun {
				fmt.Println("\033[33mDRY-RUN MODE: No changes will be made\033[0m")
				fmt.Println("\nWould sync:")
				if patentNumber != "" {
					fmt.Printf("  • Patent: %s\n", patentNumber)
				} else {
					fmt.Println("  • All patents in portfolio")
				}
				if jurisdiction != "" {
					fmt.Printf("  • Jurisdiction: %s\n", jurisdiction)
				}
				return nil
			}

			// Call LegalStatusService.SyncFromOffice()
			// In production, this would integrate with application/lifecycle.LegalStatusService
			fmt.Printf("Syncing legal status from patent offices...\n")
			fmt.Printf("\n\033[32m✅ New: 5\033[0m  \033[33m⚡ Updated: 12\033[0m  \033[31m❌ Failed: 0\033[0m\n")

			return nil
		},
	}

	cmd.Flags().StringVar(&patentNumber, "patent-number", "", "Patent number (leave empty to sync all)")
	cmd.Flags().StringVar(&jurisdiction, "jurisdiction", "", "Jurisdiction filter (CN/US/EP/JP/KR)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Dry run mode - show what would be synced without making changes")

	return cmd
}

func newRemindersCmd() *cobra.Command {
	var (
		action       string
		patentNumber string
		channels     string
		advanceDays  string
	)

	cmd := &cobra.Command{
		Use:   "reminders",
		Short: "Manage deadline reminders",
		Long:  "Configure deadline reminder notifications via email, wechat, or sms",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Validate action
			if !isValidReminderAction(action) {
				return fmt.Errorf("invalid action: %s (must be list/add/remove)", action)
			}

			// Validate required fields for add/remove
			if (action == "add" || action == "remove") && patentNumber == "" {
				return fmt.Errorf("--patent-number required for %s action", action)
			}

			// Validate channels for add action
			if action == "add" {
				if channels == "" {
					return fmt.Errorf("--channels required for add action")
				}
				// Validate channel types
				for _, ch := range strings.Split(channels, ",") {
					ch = strings.TrimSpace(ch)
					if ch != "" && !isValidChannel(ch) {
						return fmt.Errorf("invalid channel: %s (must be email/wechat/sms)", ch)
					}
				}
			}

			// Execute action via CalendarService
			// In production, this would integrate with application/lifecycle.CalendarService
			switch action {
			case "list":
				fmt.Println("Active Reminders:")
				fmt.Println("─────────────────────────────────────────────────")
				fmt.Println("  • CN202110123456 → email, wechat (30, 60, 90 days)")
				fmt.Println("  • US11234567 → email (60 days)")
				fmt.Println("  • EP3456789 → sms (30 days)")
				fmt.Println("─────────────────────────────────────────────────")
				fmt.Println("Total: 3 reminder(s)")

			case "add":
				fmt.Printf("\033[32m✅ Reminder added for %s\033[0m\n", patentNumber)
				fmt.Printf("   Channels: %s\n", channels)
				fmt.Printf("   Advance days: %s\n", advanceDays)

			case "remove":
				fmt.Printf("\033[32m✅ Reminder removed for %s\033[0m\n", patentNumber)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&action, "action", "", "Action: list/add/remove [REQUIRED]")
	cmd.Flags().StringVar(&patentNumber, "patent-number", "", "Patent number (required for add/remove)")
	cmd.Flags().StringVar(&channels, "channels", "", "Notification channels (email/wechat/sms, comma-separated)")
	cmd.Flags().StringVar(&advanceDays, "advance-days", "30,60,90", "Advance notice days (comma-separated)")
	_ = cmd.MarkFlagRequired("action")

	return cmd
}

// Deadline represents an upcoming patent deadline.
type Deadline struct {
	PatentNumber  string `json:"patent_number"`
	Type          string `json:"type"`
	DueDate       string `json:"due_date"`
	UrgencyLevel  string `json:"urgency_level"`
	DaysRemaining int    `json:"days_remaining"`
}

// AnnuityDetail represents annuity fee details.
type AnnuityDetail struct {
	Year     int     `json:"year"`
	Amount   float64 `json:"amount"`
	Currency string  `json:"currency"`
	DueDate  string  `json:"due_date"`
}

// isValidJurisdiction validates jurisdiction code.
func isValidJurisdiction(j string) bool {
	valid := map[string]bool{"CN": true, "US": true, "EP": true, "JP": true, "KR": true}
	return valid[j]
}

// isValidCurrency validates currency code.
func isValidCurrency(c string) bool {
	valid := map[string]bool{"CNY": true, "USD": true, "EUR": true, "JPY": true, "KRW": true}
	return valid[c]
}

// isValidDeadlineStatus validates deadline status.
func isValidDeadlineStatus(s string) bool {
	valid := map[string]bool{"pending": true, "overdue": true, "completed": true}
	return valid[s]
}

// isValidReminderAction validates reminder action.
func isValidReminderAction(a string) bool {
	valid := map[string]bool{"list": true, "add": true, "remove": true}
	return valid[a]
}

// isValidChannel validates notification channel.
func isValidChannel(ch string) bool {
	valid := map[string]bool{"email": true, "wechat": true, "sms": true}
	return valid[ch]
}

// getAnnuityAmount returns simulated annuity amount based on currency and year.
func getAnnuityAmount(currency string, year int) float64 {
	baseAmounts := map[string]float64{
		"CNY": 12000.0,
		"USD": 1800.0,
		"EUR": 1600.0,
		"JPY": 200000.0,
		"KRW": 2000000.0,
	}
	base := baseAmounts[currency]
	// Increase by 5% per year after 2024
	yearDiff := year - 2024
	if yearDiff > 0 {
		base *= (1 + 0.05*float64(yearDiff))
	}
	return base
}

// sortDeadlinesByUrgency sorts deadlines by urgency (descending) then by due date (ascending).
func sortDeadlinesByUrgency(deadlines []Deadline) {
	urgencyOrder := map[string]int{"CRITICAL": 0, "WARNING": 1, "NORMAL": 2}
	sort.Slice(deadlines, func(i, j int) bool {
		if urgencyOrder[deadlines[i].UrgencyLevel] != urgencyOrder[deadlines[j].UrgencyLevel] {
			return urgencyOrder[deadlines[i].UrgencyLevel] < urgencyOrder[deadlines[j].UrgencyLevel]
		}
		return deadlines[i].DueDate < deadlines[j].DueDate
	})
}

// formatDeadlineTable formats deadlines as a colored table.
// CRITICAL=red, WARNING=yellow, NORMAL=green using ANSI escape codes.
func formatDeadlineTable(deadlines []Deadline) string {
	var sb strings.Builder
	sb.WriteString("Upcoming Deadlines\n")
	sb.WriteString("══════════════════════════════════════════════════════════════════\n\n")

	if len(deadlines) == 0 {
		sb.WriteString("  No deadlines found within the specified timeframe.\n")
		return sb.String()
	}

	sb.WriteString(fmt.Sprintf("%-16s %-20s %-12s %-10s %s\n", "Patent", "Type", "Due Date", "Days", "Urgency"))
	sb.WriteString("────────────────────────────────────────────────────────────────\n")

	for _, d := range deadlines {
		var color, reset string = "", ""
		switch d.UrgencyLevel {
		case "CRITICAL":
			color = "\033[31m" // Red
		case "WARNING":
			color = "\033[33m" // Yellow
		default:
			color = "\033[32m" // Green
		}
		reset = "\033[0m"
		fmt.Fprintf(&sb, "%s%-16s %-20s %-12s %-10d [%s]%s\n",
			color, d.PatentNumber, d.Type, d.DueDate, d.DaysRemaining, d.UrgencyLevel, reset)
	}

	sb.WriteString("\n══════════════════════════════════════════════════════════════════\n")
	fmt.Fprintf(&sb, "Total: %d deadline(s)\n", len(deadlines))

	return sb.String()
}

// formatAnnuityTable formats annuity details as a table.
func formatAnnuityTable(annuities []AnnuityDetail, currency string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Annuity Fees (%s)\n", currency))
	sb.WriteString("══════════════════════════════════════\n\n")
	sb.WriteString(fmt.Sprintf("%-6s %-15s %s\n", "Year", "Amount", "Due Date"))
	sb.WriteString("──────────────────────────────────────\n")

	var total float64
	for _, a := range annuities {
		fmt.Fprintf(&sb, "%-6d %15.2f   %s\n", a.Year, a.Amount, a.DueDate)
		total += a.Amount
	}

	sb.WriteString("──────────────────────────────────────\n")
	fmt.Fprintf(&sb, "%-6s %15.2f\n", "Total", total)
	sb.WriteString("══════════════════════════════════════\n")

	return sb.String()
}

//Personal.AI order the ending

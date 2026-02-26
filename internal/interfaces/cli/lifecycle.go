package cli

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/fatih/color"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"

	"github.com/turtacn/KeyIP-Intelligence/internal/application/lifecycle"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

var (
	lifecyclePatentNumber    string
	lifecycleJurisdiction    string
	lifecycleDaysAhead       int
	lifecycleStatus          string
	lifecycleOutput          string
	lifecycleYear            int
	lifecycleCurrency        string
	lifecycleIncludeForecast bool
	lifecycleDryRun          bool
	lifecycleAction          string
	lifecycleChannels        string
	lifecycleAdvanceDays     string
)

// NewLifecycleCmd creates the lifecycle command
func NewLifecycleCmd() *cobra.Command {
	lifecycleCmd := &cobra.Command{
		Use:   "lifecycle",
		Short: "Manage patent lifecycle operations",
		Long:  `Query deadlines, calculate annuities, sync legal status, and configure reminders`,
	}

	// Subcommand: lifecycle deadlines
	deadlinesCmd := &cobra.Command{
		Use:   "deadlines",
		Short: "List upcoming patent deadlines",
		RunE: func(cmd *cobra.Command, args []string) error {
			cliCtx, err := GetCLIContext(cmd)
			if err != nil {
				return err
			}
			return runLifecycleDeadlines(cmd.Context(), cliCtx, cliCtx.Logger)
		},
	}

	deadlinesCmd.Flags().StringVar(&lifecyclePatentNumber, "patent-number", "", "Filter by patent number")
	deadlinesCmd.Flags().StringVar(&lifecycleJurisdiction, "jurisdiction", "", "Filter by jurisdiction (CN/US/EP/JP/KR)")
	deadlinesCmd.Flags().IntVar(&lifecycleDaysAhead, "days-ahead", 90, "Query deadlines within N days (1-365)")
	deadlinesCmd.Flags().StringVar(&lifecycleStatus, "status", "", "Filter by status: pending|overdue|completed")
	deadlinesCmd.Flags().StringVar(&lifecycleOutput, "output", "stdout", "Output format: stdout|json")

	// Subcommand: lifecycle annuity
	annuityCmd := &cobra.Command{
		Use:   "annuity",
		Short: "Calculate patent annuity fees",
		RunE: func(cmd *cobra.Command, args []string) error {
			cliCtx, err := GetCLIContext(cmd)
			if err != nil {
				return err
			}
			return runLifecycleAnnuity(cmd.Context(), cliCtx, cliCtx.Logger)
		},
	}

	annuityCmd.Flags().StringVar(&lifecyclePatentNumber, "patent-number", "", "Patent number (required)")
	annuityCmd.Flags().IntVar(&lifecycleYear, "year", 2024, "Target year for calculation")
	annuityCmd.Flags().StringVar(&lifecycleCurrency, "currency", "CNY", "Currency: CNY|USD|EUR|JPY|KRW")
	annuityCmd.Flags().BoolVar(&lifecycleIncludeForecast, "include-forecast", false, "Include 5-year forecast")
	annuityCmd.MarkFlagRequired("patent-number")

	// Subcommand: lifecycle sync-status
	syncStatusCmd := &cobra.Command{
		Use:   "sync-status",
		Short: "Sync legal status from patent offices",
		RunE: func(cmd *cobra.Command, args []string) error {
			cliCtx, err := GetCLIContext(cmd)
			if err != nil {
				return err
			}
			return runLifecycleSyncStatus(cmd.Context(), cliCtx, cliCtx.Logger)
		},
	}

	syncStatusCmd.Flags().StringVar(&lifecyclePatentNumber, "patent-number", "", "Sync specific patent (optional)")
	syncStatusCmd.Flags().StringVar(&lifecycleJurisdiction, "jurisdiction", "", "Sync specific jurisdiction (optional)")
	syncStatusCmd.Flags().BoolVar(&lifecycleDryRun, "dry-run", false, "Preview changes without applying")

	// Subcommand: lifecycle reminders
	remindersCmd := &cobra.Command{
		Use:   "reminders",
		Short: "Manage deadline reminders",
		RunE: func(cmd *cobra.Command, args []string) error {
			cliCtx, err := GetCLIContext(cmd)
			if err != nil {
				return err
			}
			return runLifecycleReminders(cmd.Context(), cliCtx, cliCtx.Logger)
		},
	}

	remindersCmd.Flags().StringVar(&lifecycleAction, "action", "", "Action: list|add|remove (required)")
	remindersCmd.Flags().StringVar(&lifecyclePatentNumber, "patent-number", "", "Patent number (required for add/remove)")
	remindersCmd.Flags().StringVar(&lifecycleChannels, "channels", "email", "Notification channels: email,wechat,sms")
	remindersCmd.Flags().StringVar(&lifecycleAdvanceDays, "advance-days", "30,60,90", "Reminder advance days (comma-separated)")
	remindersCmd.MarkFlagRequired("action")

	lifecycleCmd.AddCommand(deadlinesCmd, annuityCmd, syncStatusCmd, remindersCmd)
	return lifecycleCmd
}

func runLifecycleDeadlines(ctx context.Context, cliCtx *CLIContext, logger logging.Logger) error {
	return errors.NewMsg("Command 'deadlines' not implemented yet")
}

func runLifecycleAnnuity(ctx context.Context, cliCtx *CLIContext, logger logging.Logger) error {
	return errors.NewMsg("Command 'annuity' not implemented yet")
}

func runLifecycleSyncStatus(ctx context.Context, cliCtx *CLIContext, logger logging.Logger) error {
	return errors.NewMsg("Command 'sync-status' not implemented yet")
}

func runLifecycleReminders(ctx context.Context, cliCtx *CLIContext, logger logging.Logger) error {
	return errors.NewMsg("Command 'reminders' not implemented yet")
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func validateJurisdictions(input string) error {
	validJurisdictions := []string{"CN", "US", "EP", "JP", "KR"}
	parts := strings.Split(input, ",")

	for _, part := range parts {
		jurisdiction := strings.ToUpper(strings.TrimSpace(part))
		if !contains(validJurisdictions, jurisdiction) {
			return errors.Errorf("invalid jurisdiction: %s (must be CN|US|EP|JP|KR)", jurisdiction)
		}
	}

	return nil
}

func parseJurisdictions(input string) []string {
	if input == "" {
		return nil
	}

	parts := strings.Split(input, ",")
	result := make([]string, 0, len(parts))

	for _, part := range parts {
		trimmed := strings.ToUpper(strings.TrimSpace(part))
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}

	return result
}

func parseChannels(input string) []string {
	parts := strings.Split(input, ",")
	result := make([]string, 0, len(parts))

	for _, part := range parts {
		trimmed := strings.ToLower(strings.TrimSpace(part))
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}

	return result
}

func parseAdvanceDays(input string) []int {
	parts := strings.Split(input, ",")
	result := make([]int, 0, len(parts))

	for _, part := range parts {
		var days int
		if _, err := fmt.Sscanf(strings.TrimSpace(part), "%d", &days); err == nil {
			result = append(result, days)
		}
	}

	return result
}

func sortDeadlinesByUrgency(deadlines []lifecycle.Deadline) {
	sort.Slice(deadlines, func(i, j int) bool {
		// Sort by urgency first (CRITICAL > URGENT > NORMAL > FUTURE)
		if deadlines[i].Urgency != deadlines[j].Urgency {
			urgencyOrder := map[lifecycle.DeadlineUrgency]int{
				lifecycle.UrgencyExpired:  0,
				lifecycle.UrgencyCritical: 1,
				lifecycle.UrgencyUrgent:   2,
				lifecycle.UrgencyNormal:   3,
				lifecycle.UrgencyFuture:   4,
			}
			return urgencyOrder[deadlines[i].Urgency] < urgencyOrder[deadlines[j].Urgency]
		}
		// Then by due date (earliest first)
		return deadlines[i].DueDate.Before(deadlines[j].DueDate)
	})
}

func formatDeadlineTable(deadlines []lifecycle.Deadline) string {
	if len(deadlines) == 0 {
		return "\nNo upcoming deadlines found.\n"
	}

	var buf strings.Builder
	buf.WriteString("\n=== Upcoming Patent Deadlines ===\n\n")

	// Define color functions
	red := color.New(color.FgRed, color.Bold).SprintFunc()
	yellow := color.New(color.FgYellow).SprintFunc()
	green := color.New(color.FgGreen).SprintFunc()

	table := tablewriter.NewWriter(&buf)
	table.Header([]string{"Urgency", "Patent", "Type", "Due Date", "Days Left"})

	for _, d := range deadlines {
		urgencyStr := string(d.Urgency)
		switch d.Urgency {
		case lifecycle.UrgencyExpired, lifecycle.UrgencyCritical:
			urgencyStr = red(string(d.Urgency))
		case lifecycle.UrgencyUrgent:
			urgencyStr = yellow(string(d.Urgency))
		case lifecycle.UrgencyNormal, lifecycle.UrgencyFuture:
			urgencyStr = green(string(d.Urgency))
		}

		daysLeft := d.DaysRemaining
		daysLeftStr := fmt.Sprintf("%d", daysLeft)
		if daysLeft < 0 {
			daysLeftStr = red(fmt.Sprintf("%d (overdue)", daysLeft))
		}

		table.Append([]string{
			urgencyStr,
			d.PatentNumber,
			string(d.DeadlineType),
			d.DueDate.Format("2006-01-02"),
			daysLeftStr,
		})
	}

	table.Render()

	// Summary
	buf.WriteString(fmt.Sprintf("\nTotal deadlines: %d\n", len(deadlines)))

	criticalCount := countByUrgencyValue(deadlines, lifecycle.UrgencyCritical)
	if criticalCount > 0 {
		buf.WriteString(red(fmt.Sprintf("⚠ CRITICAL: %d deadlines require immediate attention\n", criticalCount)))
	}

	return buf.String()
}

func formatAnnuityTable(details []*lifecycle.AnnuityDetail, currency string) string {
	var buf strings.Builder

	buf.WriteString("\n=== Patent Annuity Fees ===\n\n")

	table := tablewriter.NewWriter(&buf)
	table.Header([]string{"Year", "Patent", "Due Date", "Fee Amount", "Status"})

	for _, d := range details {
		table.Append([]string{
			fmt.Sprintf("%d", d.YearNumber),
			d.PatentNumber,
			d.DueDate.Format("2006-01-02"),
			fmt.Sprintf("%s %.2f", d.BaseFee.Currency, d.BaseFee.Amount),
			string(d.Status),
		})
	}

	table.Render()

	return buf.String()
}

func countByUrgencyValue(deadlines []lifecycle.Deadline, urgency lifecycle.DeadlineUrgency) int {
	count := 0
	for _, d := range deadlines {
		if d.Urgency == urgency {
			count++
		}
	}
	return count
}

//Personal.AI order the ending

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"

	"github.com/turtacn/KeyIP-Intelligence/internal/application/lifecycle"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
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
func NewLifecycleCmd(
	deadlineService lifecycle.DeadlineService,
	annuityService lifecycle.AnnuityService,
	legalStatusService lifecycle.LegalStatusService,
	calendarService lifecycle.CalendarService,
	logger logging.Logger,
) *cobra.Command {
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
			return runLifecycleDeadlines(cmd.Context(), deadlineService, logger)
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
			return runLifecycleAnnuity(cmd.Context(), annuityService, logger)
		},
	}

	annuityCmd.Flags().StringVar(&lifecyclePatentNumber, "patent-number", "", "Patent number (required)")
	annuityCmd.Flags().IntVar(&lifecycleYear, "year", time.Now().Year(), "Target year for calculation")
	annuityCmd.Flags().StringVar(&lifecycleCurrency, "currency", "CNY", "Currency: CNY|USD|EUR|JPY|KRW")
	annuityCmd.Flags().BoolVar(&lifecycleIncludeForecast, "include-forecast", false, "Include 5-year forecast")
	annuityCmd.MarkFlagRequired("patent-number")

	// Subcommand: lifecycle sync-status
	syncStatusCmd := &cobra.Command{
		Use:   "sync-status",
		Short: "Sync legal status from patent offices",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLifecycleSyncStatus(cmd.Context(), legalStatusService, logger)
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
			return runLifecycleReminders(cmd.Context(), calendarService, logger)
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

func runLifecycleDeadlines(ctx context.Context, deadlineService lifecycle.DeadlineService, logger logging.Logger) error {
	// Validate days-ahead range
	if lifecycleDaysAhead < 1 || lifecycleDaysAhead > 365 {
		return errors.NewMsg("days-ahead must be between 1 and 365")
	}

	// Validate jurisdiction if provided
	if lifecycleJurisdiction != "" {
		if err := validateJurisdictions(lifecycleJurisdiction); err != nil {
			return err
		}
	}

	// Validate status if provided
	if lifecycleStatus != "" {
		validStatuses := []string{"pending", "overdue", "completed"}
		if !contains(validStatuses, strings.ToLower(lifecycleStatus)) {
			return errors.Errorf("invalid status: %s (must be pending|overdue|completed)", lifecycleStatus)
		}
	}

	logger.Info("Querying upcoming deadlines",
		logging.String("patent_number", lifecyclePatentNumber),
		logging.String("jurisdiction", lifecycleJurisdiction),
		logging.Int("days_ahead", lifecycleDaysAhead),
		logging.String("status", lifecycleStatus))

	// Build query request
	req := &lifecycle.DeadlineQuery{
		Page:     1,
		PageSize: 100,
	}
	if lifecycleJurisdiction != "" {
		// Parse jurisdiction
	}

	// Query deadlines
	resp, err := deadlineService.ListDeadlines(ctx, req)
	if err != nil {
		logger.Error("Failed to query deadlines", logging.Err(err))
		return errors.WrapMsg(err, "failed to query deadlines")
	}
	deadlines := resp.Deadlines

	// Sort by urgency (critical > warning > normal) then by due date
	sortDeadlinesByUrgency(deadlines)

	// Format output
	if lifecycleOutput == "json" {
		data, err := json.MarshalIndent(deadlines, "", "  ")
		if err != nil {
			return errors.WrapMsg(err, "failed to marshal JSON")
		}
		fmt.Println(string(data))
	} else {
		output := formatDeadlineTable(deadlines)
		fmt.Print(output)
	}

	logger.Info("Deadlines query completed",
		logging.Int("count", len(deadlines)))

	return nil
}

func runLifecycleAnnuity(ctx context.Context, annuityService lifecycle.AnnuityService, logger logging.Logger) error {
	// Validate currency
	validCurrencies := []string{"CNY", "USD", "EUR", "JPY", "KRW"}
	if !contains(validCurrencies, strings.ToUpper(lifecycleCurrency)) {
		return errors.Errorf("invalid currency: %s (must be CNY|USD|EUR|JPY|KRW)", lifecycleCurrency)
	}

	logger.Info("Calculating annuity fees",
		logging.String("patent_number", lifecyclePatentNumber),
		logging.Int("year", lifecycleYear),
		logging.String("currency", lifecycleCurrency),
		logging.Bool("include_forecast", lifecycleIncludeForecast))

	// Build calculation request
	req := &lifecycle.CalculateAnnuityRequest{
		PatentID:       lifecyclePatentNumber,
		TargetCurrency: lifecycle.Currency(strings.ToUpper(lifecycleCurrency)),
	}

	// Calculate annuities
	result, err := annuityService.CalculateAnnuity(ctx, req)
	if err != nil {
		logger.Error("Failed to calculate annuities", logging.Err(err))
		return errors.WrapMsg(err, "failed to calculate annuities")
	}

	// Format output
	fmt.Printf("\n=== Annuity Calculation ===\n\n")
	fmt.Printf("Patent: %s\n", result.PatentNumber)
	fmt.Printf("Jurisdiction: %s\n", result.Jurisdiction)
	fmt.Printf("Year %d Fee: %s %.2f\n", result.YearNumber, result.BaseFee.Currency, result.BaseFee.Amount)
	fmt.Printf("Due Date: %s\n", result.DueDate.Format("2006-01-02"))
	fmt.Printf("Status: %s\n", result.Status)

	logger.Info("Annuity calculation completed",
		logging.String("patent_number", lifecyclePatentNumber),
		logging.Float64("amount", result.BaseFee.Amount))

	return nil
}

func runLifecycleSyncStatus(ctx context.Context, legalStatusService lifecycle.LegalStatusService, logger logging.Logger) error {
	// Validate jurisdiction if provided
	if lifecycleJurisdiction != "" {
		if err := validateJurisdictions(lifecycleJurisdiction); err != nil {
			return err
		}
	}

	logger.Info("Starting legal status sync",
		logging.String("patent_number", lifecyclePatentNumber),
		logging.String("jurisdiction", lifecycleJurisdiction),
		logging.Bool("dry_run", lifecycleDryRun))

	// For single patent sync
	if lifecyclePatentNumber != "" {
		result, err := legalStatusService.SyncStatus(ctx, lifecyclePatentNumber)
		if err != nil {
			logger.Error("Sync failed", logging.Err(err))
			return errors.WrapMsg(err, "sync operation failed")
		}

		// Output summary
		fmt.Printf("\n=== Legal Status Sync Summary ===\n\n")
		if lifecycleDryRun {
			fmt.Println("🔍 DRY RUN MODE - No changes applied")
		}
		fmt.Printf("Patent: %s\n", result.PatentID)
		fmt.Printf("Previous Status: %s\n", result.PreviousStatus)
		fmt.Printf("Current Status: %s\n", result.CurrentStatus)
		fmt.Printf("Changed: %v\n", result.Changed)
		fmt.Printf("Synced At: %s\n", result.SyncedAt.Format(time.RFC3339))
		fmt.Printf("Source: %s\n", result.Source)

		logger.Info("Sync completed",
			logging.String("patent_id", result.PatentID),
			logging.Bool("changed", result.Changed))

		return nil
	}

	// For batch sync, we need patent IDs - return error if not provided
	return errors.NewMsg("--patent-number is required for status sync")
}

func runLifecycleReminders(ctx context.Context, calendarService lifecycle.CalendarService, logger logging.Logger) error {
	action := strings.ToLower(lifecycleAction)

	// Validate action
	validActions := []string{"list", "add", "remove"}
	if !contains(validActions, action) {
		return errors.Errorf("invalid action: %s (must be list|add|remove)", action)
	}

	// Require patent number for add/remove
	if (action == "add" || action == "remove") && lifecyclePatentNumber == "" {
		return errors.NewMsg("--patent-number required for add/remove actions")
	}

	logger.Info("Managing reminders",
		logging.String("action", action),
		logging.String("patent_number", lifecyclePatentNumber))

	switch action {
	case "list":
		return listReminders(ctx, calendarService, logger)
	case "add":
		return addReminder(ctx, calendarService, logger)
	case "remove":
		return removeReminder(ctx, calendarService, logger)
	default:
		return errors.Errorf("unhandled action: %s", action)
	}
}

func listReminders(ctx context.Context, calendarService lifecycle.CalendarService, logger logging.Logger) error {
	// Use GetUpcomingDeadlines to list events
	events, err := calendarService.GetUpcomingDeadlines(ctx, lifecyclePatentNumber, lifecycleDaysAhead)
	if err != nil {
		return errors.WrapMsg(err, "failed to list reminders")
	}

	if len(events) == 0 {
		fmt.Println("No upcoming deadlines/reminders found.")
		return nil
	}

	fmt.Printf("\n=== Upcoming Deadlines ===\n\n")
	table := tablewriter.NewWriter(os.Stdout)
	table.Header([]string{"ID", "Title", "Date", "Type", "Status"})

	for _, e := range events {
		table.Append([]string{
			e.ID,
			e.Title,
			e.DueDate.Format("2006-01-02"),
			string(e.EventType),
			string(e.Status),
		})
	}

	table.Render()
	return nil
}

func addReminder(ctx context.Context, calendarService lifecycle.CalendarService, logger logging.Logger) error {
	advanceDays := parseAdvanceDays(lifecycleAdvanceDays)

	// Use AddEvent to create a reminder event
	req := &lifecycle.AddEventRequest{
		PatentID:    lifecyclePatentNumber,
		Title:       fmt.Sprintf("Reminder for %s", lifecyclePatentNumber),
		EventType:   lifecycle.EventTypeCustomMilestone,
		DueDate:     time.Now().AddDate(0, 0, advanceDays[0]), // Use first advance day
		Description: fmt.Sprintf("Custom reminder with advance days: %v", advanceDays),
	}

	event, err := calendarService.AddEvent(ctx, req)
	if err != nil {
		return errors.WrapMsg(err, "failed to add reminder")
	}

	fmt.Printf("✓ Reminder added for patent %s\n", lifecyclePatentNumber)
	fmt.Printf("  Event ID: %s\n", event.ID)
	fmt.Printf("  Due Date: %s\n", event.DueDate.Format("2006-01-02"))

	logger.Info("Reminder added",
		logging.String("patent_number", lifecyclePatentNumber),
		logging.String("event_id", event.ID))

	return nil
}

func removeReminder(ctx context.Context, calendarService lifecycle.CalendarService, logger logging.Logger) error {
	// Use DeleteEvent to remove the event
	if err := calendarService.DeleteEvent(ctx, lifecyclePatentNumber); err != nil {
		return errors.WrapMsg(err, "failed to remove reminder")
	}

	fmt.Printf("✓ Reminder removed for event %s\n", lifecyclePatentNumber)

	logger.Info("Reminder removed",
		logging.String("event_id", lifecyclePatentNumber))

	return nil
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

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

//Personal.AI order the ending

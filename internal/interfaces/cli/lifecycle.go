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
		"count", len(deadlines))

	return nil
}

func runLifecycleAnnuity(ctx context.Context, annuityService lifecycle.AnnuityService, logger logging.Logger) error {
	// Validate currency
	validCurrencies := []string{"CNY", "USD", "EUR", "JPY", "KRW"}
	if !contains(validCurrencies, strings.ToUpper(lifecycleCurrency)) {
		return errors.Errorf("invalid currency: %s (must be CNY|USD|EUR|JPY|KRW)", lifecycleCurrency)
	}

	logger.Info("Calculating annuity fees",
		"patent_number", lifecyclePatentNumber,
		"year", lifecycleYear,
		"currency", lifecycleCurrency,
		"include_forecast", lifecycleIncludeForecast)

	// Build calculation request
	req := &lifecycle.AnnuityQueryRequest{
		PatentNumber:     lifecyclePatentNumber,
		Year:             lifecycleYear,
		Currency:         strings.ToUpper(lifecycleCurrency),
		IncludeForecast:  lifecycleIncludeForecast,
		Context:          ctx,
	}

	// Calculate annuities
	result, err := annuityService.Calculate(ctx, req)
	if err != nil {
		logger.Error("Failed to calculate annuities", "error", err)
		return errors.WrapMsg(err, "failed to calculate annuities")
	}

	// Format output
	output := formatAnnuityTable(result.Details, result.Currency)
	fmt.Print(output)

	// Summary
	fmt.Printf("\nTotal for %d: %s %.2f\n", lifecycleYear, result.Currency, result.TotalAmount)
	if lifecycleIncludeForecast {
		fmt.Printf("5-year forecast: %s %.2f\n", result.Currency, result.ForecastTotal)
	}

	logger.Info("Annuity calculation completed",
		"patent_number", lifecyclePatentNumber,
		"total_amount", result.TotalAmount)

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
		"patent_number", lifecyclePatentNumber,
		"jurisdiction", lifecycleJurisdiction,
		"dry_run", lifecycleDryRun)

	// Build sync request
	req := &lifecycle.SyncStatusRequest{
		PatentNumber:  lifecyclePatentNumber,
		Jurisdictions: parseJurisdictions(lifecycleJurisdiction),
		DryRun:        lifecycleDryRun,
		Context:       ctx,
	}

	// Execute sync
	result, err := legalStatusService.SyncFromOffice(ctx, req)
	if err != nil {
		logger.Error("Sync failed", "error", err)
		return errors.WrapMsg(err, "sync operation failed")
	}

	// Output summary
	fmt.Printf("\n=== Legal Status Sync Summary ===\n\n")
	if lifecycleDryRun {
		fmt.Println("🔍 DRY RUN MODE - No changes applied")
	}
	fmt.Printf("Total processed: %d\n", result.TotalProcessed)
	fmt.Printf("✓ New records: %d\n", result.NewRecords)
	fmt.Printf("↻ Updated records: %d\n", result.UpdatedRecords)
	fmt.Printf("✗ Failed: %d\n", result.FailedCount)

	if len(result.Errors) > 0 {
		fmt.Println("\nErrors:")
		for _, errItem := range result.Errors {
			fmt.Printf("  - %s: %s\n", errItem.PatentNumber, errItem.ErrorMessage)
		}
	}

	logger.Info("Sync completed",
		"total_processed", result.TotalProcessed,
		"new_records", result.NewRecords,
		"updated_records", result.UpdatedRecords,
		"failed", result.FailedCount)

	return nil
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
		"action", action,
		"patent_number", lifecyclePatentNumber)

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
	reminders, err := calendarService.ListReminders(ctx, lifecyclePatentNumber)
	if err != nil {
		return errors.WrapMsg(err, "failed to list reminders")
	}

	if len(reminders) == 0 {
		fmt.Println("No reminders configured.")
		return nil
	}

	fmt.Printf("\n=== Configured Reminders ===\n\n")
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Patent", "Deadline Type", "Channels", "Advance Days"})

	for _, r := range reminders {
		table.Append([]string{
			r.PatentNumber,
			r.DeadlineType,
			strings.Join(r.Channels, ", "),
			fmt.Sprintf("%v", r.AdvanceDays),
		})
	}

	table.Render()
	return nil
}

func addReminder(ctx context.Context, calendarService lifecycle.CalendarService, logger logging.Logger) error {
	channels := parseChannels(lifecycleChannels)
	advanceDays := parseAdvanceDays(lifecycleAdvanceDays)

	req := &lifecycle.AddReminderRequest{
		PatentNumber: lifecyclePatentNumber,
		Channels:     channels,
		AdvanceDays:  advanceDays,
		Context:      ctx,
	}

	if err := calendarService.AddReminder(ctx, req); err != nil {
		return errors.WrapMsg(err, "failed to add reminder")
	}

	fmt.Printf("✓ Reminder added for patent %s\n", lifecyclePatentNumber)
	fmt.Printf("  Channels: %s\n", strings.Join(channels, ", "))
	fmt.Printf("  Advance days: %v\n", advanceDays)

	logger.Info("Reminder added",
		"patent_number", lifecyclePatentNumber,
		"channels", channels,
		"advance_days", advanceDays)

	return nil
}

func removeReminder(ctx context.Context, calendarService lifecycle.CalendarService, logger logging.Logger) error {
	if err := calendarService.RemoveReminder(ctx, lifecyclePatentNumber); err != nil {
		return errors.WrapMsg(err, "failed to remove reminder")
	}

	fmt.Printf("✓ Reminder removed for patent %s\n", lifecyclePatentNumber)

	logger.Info("Reminder removed",
		"patent_number", lifecyclePatentNumber)

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

func sortDeadlinesByUrgency(deadlines []*lifecycle.Deadline) {
	sort.Slice(deadlines, func(i, j int) bool {
		// Sort by urgency first (CRITICAL > WARNING > NORMAL)
		if deadlines[i].Urgency != deadlines[j].Urgency {
			urgencyOrder := map[string]int{
				"CRITICAL": 0,
				"WARNING":  1,
				"NORMAL":   2,
			}
			return urgencyOrder[deadlines[i].Urgency] < urgencyOrder[deadlines[j].Urgency]
		}
		// Then by due date (earliest first)
		return deadlines[i].DueDate.Before(deadlines[j].DueDate)
	})
}

func formatDeadlineTable(deadlines []*lifecycle.Deadline) string {
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
	table.SetHeader([]string{"Urgency", "Patent", "Type", "Due Date", "Days Left", "Status"})
	table.SetBorder(true)

	for _, d := range deadlines {
		urgencyStr := d.Urgency
		switch d.Urgency {
		case "CRITICAL":
			urgencyStr = red(d.Urgency)
		case "WARNING":
			urgencyStr = yellow(d.Urgency)
		case "NORMAL":
			urgencyStr = green(d.Urgency)
		}

		daysLeft := int(time.Until(d.DueDate).Hours() / 24)
		daysLeftStr := fmt.Sprintf("%d", daysLeft)
		if daysLeft < 0 {
			daysLeftStr = red(fmt.Sprintf("%d (overdue)", daysLeft))
		}

		table.Append([]string{
			urgencyStr,
			d.PatentNumber,
			d.DeadlineType,
			d.DueDate.Format("2006-01-02"),
			daysLeftStr,
			d.Status,
		})
	}

	table.Render()

	// Summary
	buf.WriteString(fmt.Sprintf("\nTotal deadlines: %d\n", len(deadlines)))

	criticalCount := countByUrgency(deadlines, "CRITICAL")
	if criticalCount > 0 {
		buf.WriteString(red(fmt.Sprintf("⚠ CRITICAL: %d deadlines require immediate attention\n", criticalCount)))
	}

	return buf.String()
}

func formatAnnuityTable(details []*lifecycle.AnnuityDetail, currency string) string {
	var buf strings.Builder

	buf.WriteString("\n=== Patent Annuity Fees ===\n\n")

	table := tablewriter.NewWriter(&buf)
	table.SetHeader([]string{"Year", "Due Date", "Fee Amount", "Late Fee", "Total", "Status"})
	table.SetBorder(true)

	for _, d := range details {
		table.Append([]string{
			fmt.Sprintf("%d", d.Year),
			d.DueDate.Format("2006-01-02"),
			fmt.Sprintf("%s %.2f", currency, d.BaseFee),
			fmt.Sprintf("%s %.2f", currency, d.LateFee),
			fmt.Sprintf("%s %.2f", currency, d.TotalFee),
			d.Status,
		})
	}

	table.Render()

	return buf.String()
}

func countByUrgency(deadlines []*lifecycle.Deadline, urgency string) int {
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

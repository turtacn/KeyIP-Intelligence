package cli

import (
"fmt"
"strings"
"github.com/spf13/cobra"
)

// NewLifecycleCmd creates the lifecycle management command
func NewLifecycleCmd() *cobra.Command {
cmd := &cobra.Command{
Use:   "lifecycle",
Short: "Patent lifecycle management",
Long:  "Manage patent lifecycle including deadlines, annuities, legal status synchronization and reminders",
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
Long:  "List upcoming patent deadlines within specified timeframe",
RunE: func(cmd *cobra.Command, args []string) error {
// Validate jurisdiction
if jurisdiction != "" {
jurisdictions := strings.Split(jurisdiction, ",")
for _, j := range jurisdictions {
j = strings.TrimSpace(j)
if !isValidJurisdiction(j) {
return fmt.Errorf("invalid jurisdiction: %s (must be CN/US/EP/JP/KR)", j)
}
}
}

// Validate days-ahead
if daysAhead < 1 || daysAhead > 365 {
return fmt.Errorf("days-ahead must be between 1 and 365, got %d", daysAhead)
}

// Simulate deadline query
deadlines := []Deadline{
{PatentNumber: "CN202110123456", Type: "OA Response", DueDate: "2024-12-31", UrgencyLevel: "CRITICAL"},
{PatentNumber: "US11234567", Type: "Annuity Payment", DueDate: "2025-01-15", UrgencyLevel: "WARNING"},
{PatentNumber: "EP3456789", Type: "Validation", DueDate: "2025-02-01", UrgencyLevel: "NORMAL"},
}

// Format and output
output := formatDeadlineTable(deadlines)
fmt.Print(output)
return nil
},
}

cmd.Flags().StringVar(&patentNumber, "patent-number", "", "Filter by patent number")
cmd.Flags().StringVar(&jurisdiction, "jurisdiction", "", "Filter by jurisdiction (CN/US/EP/JP/KR, comma-separated)")
cmd.Flags().IntVar(&daysAhead, "days-ahead", 90, "Query deadlines within N days")
cmd.Flags().StringVar(&status, "status", "", "Filter by status (pending/overdue/completed)")
cmd.Flags().StringVar(&output, "output", "stdout", "Output format (stdout/json)")

return cmd
}

func newAnnuityCmd() *cobra.Command {
var (
patentNumber     string
year             int
currency         string
includeForecast  bool
)

cmd := &cobra.Command{
Use:   "annuity",
Short: "Calculate annuity fees",
Long:  "Calculate patent annuity/maintenance fees for specified year",
RunE: func(cmd *cobra.Command, args []string) error {
// Validate currency
if !isValidCurrency(currency) {
return fmt.Errorf("invalid currency: %s (must be CNY/USD/EUR/JPY/KRW)", currency)
}

// Simulate annuity calculation
annuities := []AnnuityDetail{
{Year: year, Amount: 12000.00, Currency: currency, DueDate: "2025-01-31"},
}

if includeForecast {
for i := 1; i <= 5; i++ {
annuities = append(annuities, AnnuityDetail{
Year: year + i, Amount: 12000.00 + float64(i*1000), Currency: currency, DueDate: fmt.Sprintf("202%d-01-31", 5+i),
})
}
}

output := formatAnnuityTable(annuities, currency)
fmt.Print(output)
return nil
},
}

cmd.Flags().StringVar(&patentNumber, "patent-number", "", "Patent number [REQUIRED]")
cmd.Flags().IntVar(&year, "year", 2024, "Annuity year")
cmd.Flags().StringVar(&currency, "currency", "CNY", "Currency (CNY/USD/EUR/JPY/KRW)")
cmd.Flags().BoolVar(&includeForecast, "include-forecast", false, "Include 5-year forecast")
cmd.MarkFlagRequired("patent-number")

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
Long:  "Synchronize patent legal status from patent offices",
RunE: func(cmd *cobra.Command, args []string) error {
if dryRun {
fmt.Println("DRY-RUN MODE: No changes will be made")
}

// Simulate sync
fmt.Printf("Syncing legal status from patent offices...\n")
fmt.Printf("✅ New: 5  ⚡ Updated: 12  ❌ Failed: 0\n")
return nil
},
}

cmd.Flags().StringVar(&patentNumber, "patent-number", "", "Patent number (leave empty to sync all)")
cmd.Flags().StringVar(&jurisdiction, "jurisdiction", "", "Jurisdiction filter")
cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Dry run mode")

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
Long:  "Configure deadline reminder notifications",
RunE: func(cmd *cobra.Command, args []string) error {
if action != "list" && action != "add" && action != "remove" {
return fmt.Errorf("action must be list/add/remove, got %s", action)
}

if action == "add" || action == "remove" {
if patentNumber == "" {
return fmt.Errorf("--patent-number required for %s action", action)
}
}

if action == "add" {
if channels == "" {
return fmt.Errorf("--channels required for add action")
}
fmt.Printf("✅ Reminder added for %s via %s\n", patentNumber, channels)
} else if action == "list" {
fmt.Println("Active Reminders:")
fmt.Println("  • CN202110123456 → email, wechat (30, 60, 90 days)")
fmt.Println("  • US11234567 → email (60 days)")
} else {
fmt.Printf("✅ Reminder removed for %s\n", patentNumber)
}

return nil
},
}

cmd.Flags().StringVar(&action, "action", "", "Action: list/add/remove [REQUIRED]")
cmd.Flags().StringVar(&patentNumber, "patent-number", "", "Patent number")
cmd.Flags().StringVar(&channels, "channels", "", "Notification channels (email/wechat/sms, comma-separated)")
cmd.Flags().StringVar(&advanceDays, "advance-days", "30,60,90", "Advance notice days")
cmd.MarkFlagRequired("action")

return cmd
}

// Helper types
type Deadline struct {
PatentNumber string
Type         string
DueDate      string
UrgencyLevel string
}

type AnnuityDetail struct {
Year     int
Amount   float64
Currency string
DueDate  string
}

// Helper functions
func isValidJurisdiction(j string) bool {
valid := map[string]bool{"CN": true, "US": true, "EP": true, "JP": true, "KR": true}
return valid[j]
}

func isValidCurrency(c string) bool {
valid := map[string]bool{"CNY": true, "USD": true, "EUR": true, "JPY": true, "KRW": true}
return valid[c]
}

func formatDeadlineTable(deadlines []Deadline) string {
var sb strings.Builder
sb.WriteString("Upcoming Deadlines\n")
sb.WriteString("==================\n\n")

for _, d := range deadlines {
var color string
switch d.UrgencyLevel {
case "CRITICAL":
color = "\033[31m" // Red
case "WARNING":
color = "\033[33m" // Yellow
default:
color = "\033[32m" // Green
}
fmt.Fprintf(&sb, "%s%-15s %-20s %-12s [%s]\033[0m\n", color, d.PatentNumber, d.Type, d.DueDate, d.UrgencyLevel)
}

return sb.String()
}

func formatAnnuityTable(annuities []AnnuityDetail, currency string) string {
var sb strings.Builder
sb.WriteString(fmt.Sprintf("Annuity Fees (%s)\n", currency))
sb.WriteString("====================\n\n")
sb.WriteString("Year  Amount       Due Date\n")
sb.WriteString("--------------------------------\n")

for _, a := range annuities {
fmt.Fprintf(&sb, "%-4d  %10.2f   %s\n", a.Year, a.Amount, a.DueDate)
}

return sb.String()
}

//Personal.AI order the ending

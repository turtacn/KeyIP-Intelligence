package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorCyan   = "\033[36m"
)

// NewReportCmd creates the report generation command
func NewReportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "report",
		Short: "Generate patent analysis reports",
		Long: `Generate various types of patent reports including FTO analysis, 
infringement reports, portfolio analysis, and annual IP reports.`,
	}

	cmd.AddCommand(newGenerateCmd())
	cmd.AddCommand(newListTemplatesCmd())
	cmd.AddCommand(newStatusCmd())

	return cmd
}

// Generate command flags
var (
	generateType            string
	generateTarget          string
	generateFormat          string
	generateOutputDir       string
	generateTemplate        string
	generateLanguage        string
	generateIncludeAppendix bool
)

// List templates command flags
var (
	listTemplatesType string
)

// Status command flags
var (
	statusJobID string
)

func newGenerateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate a patent report",
		RunE:  runGenerate,
	}

	cmd.Flags().StringVar(&generateType, "type", "", "Report type: fto/infringement/portfolio/annual (required)")
	cmd.Flags().StringVar(&generateTarget, "target", "", "Target identifier: patent number, portfolio ID, etc. (required)")
	cmd.Flags().StringVar(&generateFormat, "format", "pdf", "Output format: pdf/docx/json")
	cmd.Flags().StringVar(&generateOutputDir, "output-dir", ".", "Output directory")
	cmd.Flags().StringVar(&generateTemplate, "template", "", "Custom template path")
	cmd.Flags().StringVar(&generateLanguage, "language", "zh", "Report language: zh/en")
	cmd.Flags().BoolVar(&generateIncludeAppendix, "include-appendix", true, "Include appendix sections")

	cmd.MarkFlagRequired("type")
	cmd.MarkFlagRequired("target")

	return cmd
}

func newListTemplatesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list-templates",
		Short: "List available report templates",
		RunE:  runListTemplates,
	}

	cmd.Flags().StringVar(&listTemplatesType, "type", "", "Filter by report type")

	return cmd
}

func newStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Check report generation job status",
		RunE:  runStatus,
	}

	cmd.Flags().StringVar(&statusJobID, "job-id", "", "Job ID (required)")
	cmd.MarkFlagRequired("job-id")

	return cmd
}

// ReportResult represents report generation result
type ReportResult struct {
	FilePath       string
	Pages          int
	Sections       int
	DataSources    int
	GenerationTime time.Duration
	JobID          string // For async mode
	IsAsync        bool
}

// TemplateInfo represents a report template
type TemplateInfo struct {
	ID          string
	Name        string
	Type        string
	Language    string
	Version     string
	Description string
}

// JobStatus represents async job status
type JobStatus struct {
	JobID          string
	Status         string // in_progress/completed/failed
	Progress       int    // 0-100
	EstimatedTime  string
	ErrorMessage   string
	ResultFilePath string
}

func runGenerate(cmd *cobra.Command, args []string) error {
	// Validate parameters
	if !isValidReportType(generateType) {
		return fmt.Errorf("invalid report type: %s (allowed: fto, infringement, portfolio, annual)", generateType)
	}

	if !isValidFormat(generateFormat) {
		return fmt.Errorf("invalid format: %s (allowed: pdf, docx, json)", generateFormat)
	}

	if !isValidLanguage(generateLanguage) {
		return fmt.Errorf("invalid language: %s (allowed: zh, en)", generateLanguage)
	}

	// Ensure output directory exists
	if err := ensureOutputDir(generateOutputDir); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Check if large report (simulate estimation)
	estimatedPages := estimateReportSize(generateType, generateTarget)
	isAsync := estimatedPages > 100

	if isAsync {
		// Async mode
		fmt.Println(colorYellow + "Large report detected. Switching to async generation mode..." + colorReset)
		jobID := generateJobID()

		fmt.Printf("\n%s✓ Report generation job submitted%s\n", colorGreen, colorReset)
		fmt.Printf("  Job ID: %s%s%s\n", colorCyan, jobID, colorReset)
		fmt.Printf("  Type: %s\n", generateType)
		fmt.Printf("  Target: %s\n", generateTarget)
		fmt.Printf("  Format: %s\n", generateFormat)
		fmt.Printf("  Estimated pages: %d\n", estimatedPages)
		fmt.Printf("\nUse '%skeyip report status --job-id %s%s' to check progress\n",
			colorCyan, jobID, colorReset)

		return nil
	}

	// Synchronous mode
	fmt.Printf("Generating %s report for %s...\n", generateType, generateTarget)

	startTime := time.Now()

	// Simulate report generation
	result := &ReportResult{
		FilePath:       resolveOutputPath(generateOutputDir, generateType, generateFormat),
		Pages:          estimatedPages,
		Sections:       getSectionCount(generateType),
		DataSources:    getDataSourceCount(generateType),
		GenerationTime: time.Since(startTime),
		IsAsync:        false,
	}

	// Simulate file creation
	if err := createDummyFile(result.FilePath); err != nil {
		return fmt.Errorf("failed to write report file: %w", err)
	}

	// Print summary
	printReportSummary(result)

	return nil
}

func runListTemplates(cmd *cobra.Command, args []string) error {
	// Simulate template listing
	templates := []TemplateInfo{
		{
			ID:          "fto-standard-zh-v1",
			Name:        "FTO Standard Report (Chinese)",
			Type:        "fto",
			Language:    "zh",
			Version:     "1.0",
			Description: "Standard FTO analysis template with risk assessment",
		},
		{
			ID:          "fto-standard-en-v1",
			Name:        "FTO Standard Report (English)",
			Type:        "fto",
			Language:    "en",
			Version:     "1.0",
			Description: "Standard FTO analysis template with risk assessment",
		},
		{
			ID:          "infringement-detailed-zh-v1",
			Name:        "Infringement Analysis (Chinese)",
			Type:        "infringement",
			Language:    "zh",
			Version:     "1.0",
			Description: "Detailed infringement analysis with claim mapping",
		},
		{
			ID:          "portfolio-analysis-zh-v1",
			Name:        "Portfolio Analysis (Chinese)",
			Type:        "portfolio",
			Language:    "zh",
			Version:     "1.0",
			Description: "Comprehensive portfolio health and optimization report",
		},
		{
			ID:          "annual-ip-zh-v1",
			Name:        "Annual IP Report (Chinese)",
			Type:        "annual",
			Language:    "zh",
			Version:     "1.0",
			Description: "Executive summary and strategic recommendations",
		},
	}

	// Filter by type if specified
	if listTemplatesType != "" {
		filtered := []TemplateInfo{}
		for _, t := range templates {
			if t.Type == listTemplatesType {
				filtered = append(filtered, t)
			}
		}
		templates = filtered
	}

	// Print templates
	fmt.Println("Available Report Templates:")
	fmt.Println(strings.Repeat("═", 80))

	for _, t := range templates {
		fmt.Printf("\n%s[%s]%s %s\n", colorCyan, t.ID, colorReset, t.Name)
		fmt.Printf("  Type: %s | Language: %s | Version: %s\n", t.Type, t.Language, t.Version)
		fmt.Printf("  %s\n", t.Description)
	}

	fmt.Println("\n" + strings.Repeat("═", 80))
	fmt.Printf("Total: %d template(s)\n", len(templates))

	return nil
}

func runStatus(cmd *cobra.Command, args []string) error {
	// Simulate status check
	status := &JobStatus{
		JobID:         statusJobID,
		Status:        "in_progress",
		Progress:      65,
		EstimatedTime: "5 minutes",
	}

	// Random status for demo
	if len(statusJobID) > 10 && statusJobID[10] == 'a' {
		status.Status = "completed"
		status.Progress = 100
		status.ResultFilePath = "/tmp/report_fto_20240223_143022.pdf"
	} else if len(statusJobID) > 10 && statusJobID[10] == 'z' {
		status.Status = "failed"
		status.ErrorMessage = "Template rendering failed: missing data source"
	}

	fmt.Printf("Report Generation Job Status\n")
	fmt.Println(strings.Repeat("═", 60))
	fmt.Printf("  Job ID: %s\n", status.JobID)

	switch status.Status {
	case "in_progress":
		fmt.Printf("  Status: %s%s%s\n", colorYellow, status.Status, colorReset)
		fmt.Printf("  Progress: [%s] %d%%\n", progressBar(status.Progress), status.Progress)
		fmt.Printf("  Estimated time remaining: %s\n", status.EstimatedTime)

	case "completed":
		fmt.Printf("  Status: %s%s%s\n", colorGreen, status.Status, colorReset)
		fmt.Printf("  Progress: [%s] %d%%\n", progressBar(status.Progress), status.Progress)
		fmt.Printf("  Result: %s\n", status.ResultFilePath)

	case "failed":
		fmt.Printf("  Status: %s%s%s\n", colorRed, status.Status, colorReset)
		fmt.Printf("  Error: %s\n", status.ErrorMessage)
	}

	fmt.Println(strings.Repeat("═", 60))

	return nil
}

func resolveOutputPath(dir string, reportType string, format string) string {
	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("report_%s_%s.%s", reportType, timestamp, format)
	path := filepath.Join(dir, filename)

	// Handle filename conflicts
	if _, err := os.Stat(path); err == nil {
		seq := 1
		for {
			filename = fmt.Sprintf("report_%s_%s_%d.%s", reportType, timestamp, seq, format)
			path = filepath.Join(dir, filename)
			if _, err := os.Stat(path); os.IsNotExist(err) {
				break
			}
			seq++
		}
	}

	return path
}

func printReportSummary(result *ReportResult) {
	fmt.Println("\n" + strings.Repeat("═", 70))
	fmt.Printf("%s✓ Report Generated Successfully%s\n\n", colorGreen, colorReset)
	fmt.Printf("  File:         %s%s%s\n", colorCyan, result.FilePath, colorReset)
	fmt.Printf("  Pages:        %d\n", result.Pages)
	fmt.Printf("  Sections:     %d\n", result.Sections)
	fmt.Printf("  Data sources: %d\n", result.DataSources)
	fmt.Printf("  Generation time: %.2f seconds\n", result.GenerationTime.Seconds())
	fmt.Println(strings.Repeat("═", 70))
}

func ensureOutputDir(dir string) error {
	if dir == "" || dir == "." {
		return nil
	}
	return os.MkdirAll(dir, 0755)
}

func createDummyFile(path string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.WriteString("KeyIP-Intelligence Report (Demo)\n")
	return err
}

func isValidReportType(t string) bool {
	valid := map[string]bool{
		"fto":          true,
		"infringement": true,
		"portfolio":    true,
		"annual":       true,
	}
	return valid[t]
}

func isValidFormat(f string) bool {
	valid := map[string]bool{
		"pdf":  true,
		"docx": true,
		"json": true,
	}
	return valid[f]
}

func isValidLanguage(l string) bool {
	valid := map[string]bool{
		"zh": true,
		"en": true,
	}
	return valid[l]
}

func estimateReportSize(reportType string, target string) int {
	// Simulate size estimation
	sizes := map[string]int{
		"fto":          45,
		"infringement": 35,
		"portfolio":    120, // Large report
		"annual":       80,
	}
	return sizes[reportType]
}

func getSectionCount(reportType string) int {
	counts := map[string]int{
		"fto":          7,
		"infringement": 6,
		"portfolio":    12,
		"annual":       10,
	}
	return counts[reportType]
}

func getDataSourceCount(reportType string) int {
	counts := map[string]int{
		"fto":          15,
		"infringement": 12,
		"portfolio":    25,
		"annual":       30,
	}
	return counts[reportType]
}

func generateJobID() string {
	return fmt.Sprintf("job-%d-%s", time.Now().Unix(), randomString(8))
}

func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[time.Now().UnixNano()%int64(len(letters))]
	}
	return string(b)
}

func progressBar(progress int) string {
	width := 30
	filled := progress * width / 100
	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
	return bar
}

//Personal.AI order the ending

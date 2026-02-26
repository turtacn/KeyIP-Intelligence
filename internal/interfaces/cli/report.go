package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"

	"github.com/turtacn/KeyIP-Intelligence/internal/application/reporting"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
)

var (
	reportType           string
	reportTarget         string
	reportFormat         string
	reportOutputDir      string
	reportTemplate       string
	reportLanguage       string
	reportIncludeAppendix bool
	reportJobID          string
)

const (
	// Large report threshold (pages)
	largeReportThreshold = 100
	// Async mode for large reports
	asyncModeEnabled = true
)

// NewReportCmd creates the report command
func NewReportCmd(
	ftoReportService reporting.FTOReportService,
	infringementReportService reporting.InfringementReportService,
	portfolioReportService reporting.PortfolioReportService,
	templateService reporting.TemplateService,
	logger logging.Logger,
) *cobra.Command {
	reportCmd := &cobra.Command{
		Use:   "report",
		Short: "Generate IP reports",
		Long:  `Generate FTO, infringement, portfolio, and annual IP reports in various formats`,
	}

	// Subcommand: report generate
	generateCmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate a report",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runReportGenerate(
				cmd.Context(),
				ftoReportService,
				infringementReportService,
				portfolioReportService,
				logger,
			)
		},
	}

	generateCmd.Flags().StringVar(&reportType, "type", "", "Report type: fto|infringement|portfolio|annual (required)")
	generateCmd.Flags().StringVar(&reportTarget, "target", "", "Target identifier (patent number, portfolio ID, etc.) (required)")
	generateCmd.Flags().StringVar(&reportFormat, "format", "pdf", "Output format: pdf|docx|json")
	generateCmd.Flags().StringVar(&reportOutputDir, "output-dir", ".", "Output directory")
	generateCmd.Flags().StringVar(&reportTemplate, "template", "", "Custom template path (optional)")
	generateCmd.Flags().StringVar(&reportLanguage, "language", "zh", "Report language: zh|en")
	generateCmd.Flags().BoolVar(&reportIncludeAppendix, "include-appendix", true, "Include appendices")
	generateCmd.MarkFlagRequired("type")
	generateCmd.MarkFlagRequired("target")

	// Subcommand: report list-templates
	listTemplatesCmd := &cobra.Command{
		Use:   "list-templates",
		Short: "List available report templates",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runReportListTemplates(cmd.Context(), templateService, logger)
		},
	}

	listTemplatesCmd.Flags().StringVar(&reportType, "type", "", "Filter by report type (optional)")

	// Subcommand: report status
	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Check report generation status",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runReportStatus(cmd.Context(), ftoReportService, logger)
		},
	}

	statusCmd.Flags().StringVar(&reportJobID, "job-id", "", "Report generation job ID (required)")
	statusCmd.MarkFlagRequired("job-id")

	reportCmd.AddCommand(generateCmd, listTemplatesCmd, statusCmd)
	return reportCmd
}

func runReportGenerate(
	ctx context.Context,
	ftoReportService reporting.FTOReportService,
	infringementReportService reporting.InfringementReportService,
	portfolioReportService reporting.PortfolioReportService,
	logger logging.Logger,
) error {
	// Validate report type
	validTypes := []string{"fto", "infringement", "portfolio"}
	if !contains(validTypes, strings.ToLower(reportType)) {
		return errors.Errorf("invalid report type: %s (must be fto|infringement|portfolio)", reportType)
	}

	// Validate output format
	validFormats := []string{"pdf", "docx", "json"}
	if !contains(validFormats, strings.ToLower(reportFormat)) {
		return errors.Errorf("invalid output format: %s (must be pdf|docx|json)", reportFormat)
	}

	// Validate language
	validLanguages := []string{"zh", "en"}
	if !contains(validLanguages, strings.ToLower(reportLanguage)) {
		return errors.Errorf("invalid language: %s (must be zh|en)", reportLanguage)
	}

	// Ensure output directory exists
	if err := ensureOutputDir(reportOutputDir); err != nil {
		return errors.WrapMsg(err, "failed to create output directory")
	}

	logger.Info("Starting report generation",
		logging.String("type", reportType),
		logging.String("target", reportTarget),
		logging.String("format", reportFormat),
		logging.String("language", reportLanguage))

	startTime := time.Now()

	// Dispatch to appropriate service based on report type
	var reportID string
	var err error

	switch strings.ToLower(reportType) {
	case "fto":
		lang := reporting.LangEN
		if strings.ToLower(reportLanguage) == "zh" {
			lang = reporting.LangZH
		}
		req := &reporting.FTOReportRequest{
			TargetMolecules: []reporting.MoleculeInput{
				{Format: "smiles", Value: reportTarget},
			},
			Jurisdictions: []string{"CN", "US", "EP"},
			AnalysisDepth: reporting.DepthStandard,
			Language:      lang,
		}
		result, genErr := ftoReportService.Generate(ctx, req)
		if genErr != nil {
			err = genErr
		} else {
			reportID = result.ReportID
		}

	case "infringement":
		req := &reporting.InfringementReportRequest{
			OwnedPatentNumbers: []string{reportTarget},
			AnalysisMode:       reporting.ModeComprehensive,
		}
		result, genErr := infringementReportService.Generate(ctx, req)
		if genErr != nil {
			err = genErr
		} else {
			reportID = result.ReportID
		}

	case "portfolio":
		req := &reporting.PortfolioReportRequest{
			PortfolioID: reportTarget,
			OutputFormat: reporting.FormatPortfolioPDF,
		}
		result, genErr := portfolioReportService.GenerateFullReport(ctx, req)
		if genErr != nil {
			err = genErr
		} else {
			reportID = result.ReportID
		}

	default:
		return errors.Errorf("unhandled report type: %s", reportType)
	}

	if err != nil {
		logger.Error("Report generation failed", logging.Err(err))
		return errors.WrapMsg(err, "report generation failed")
	}

	generationTime := time.Since(startTime)

	// Print summary
	fmt.Printf("\n✓ Report generation initiated\n\n")
	fmt.Printf("Report ID: %s\n", reportID)
	fmt.Printf("Type: %s\n", reportType)
	fmt.Printf("Target: %s\n", reportTarget)
	fmt.Printf("Generation time: %.2fs\n", generationTime.Seconds())
	fmt.Printf("\nCheck status with: keyip report status --job-id %s\n", reportID)

	logger.Info("Report generation initiated",
		logging.String("type", reportType),
		logging.String("report_id", reportID),
		logging.Float64("generation_time_seconds", generationTime.Seconds()))

	return nil
}

func runReportListTemplates(ctx context.Context, templateService reporting.TemplateService, logger logging.Logger) error {
	logger.Info("Listing report templates", logging.String("filter_type", reportType))

	var typeFilter *string
	if reportType != "" {
		typeFilter = &reportType
	}

	opts := &reporting.ListTemplateOptions{
		Type: typeFilter,
		Pagination: common.Pagination{
			Page:     1,
			PageSize: 100,
		},
	}

	result, err := templateService.ListTemplates(ctx, opts)
	if err != nil {
		logger.Error("Failed to list templates", logging.Err(err))
		return errors.WrapMsg(err, "failed to list templates")
	}

	if len(result.Items) == 0 {
		fmt.Println("\nNo templates found.")
		return nil
	}

	fmt.Printf("\n=== Available Report Templates ===\n\n")

	table := tablewriter.NewWriter(os.Stdout)
	table.Header([]string{"ID", "Name", "Type", "Format", "Version"})

	for _, tmpl := range result.Items {
		table.Append([]string{
			tmpl.ID,
			tmpl.Name,
			tmpl.Type,
			string(tmpl.Format),
			tmpl.Version,
		})
	}

	table.Render()

	fmt.Printf("\nTotal templates: %d\n", len(result.Items))

	logger.Info("Templates listed", logging.Int("count", len(result.Items)))

	return nil
}

func runReportStatus(ctx context.Context, ftoReportService reporting.FTOReportService, logger logging.Logger) error {
	logger.Info("Checking report status", logging.String("job_id", reportJobID))

	status, err := ftoReportService.GetStatus(ctx, reportJobID)
	if err != nil {
		logger.Error("Failed to get job status", logging.Err(err))
		return errors.WrapMsg(err, "failed to get job status")
	}

	fmt.Printf("\n=== Report Generation Status ===\n\n")
	fmt.Printf("Report ID: %s\n", status.ReportID)
	fmt.Printf("Status: %s\n", colorizeStatus(string(status.Status)))
	fmt.Printf("Progress: %d%%\n", status.ProgressPct)

	if status.Status == reporting.StatusProcessing {
		fmt.Printf("Processing...\n")
	}

	if status.Status == reporting.StatusCompleted {
		fmt.Printf("\n✓ Report ready for download\n")
	}

	if status.Status == reporting.StatusFailed {
		fmt.Printf("\n✗ Report generation failed\n")
		if status.Message != "" {
			fmt.Printf("Message: %s\n", status.Message)
		}
	}

	logger.Info("Status retrieved",
		logging.String("report_id", status.ReportID),
		logging.String("status", string(status.Status)),
		logging.Int("progress", status.ProgressPct))

	return nil
}

func resolveOutputPath(dir string, reportType string, format string) string {
	timestamp := time.Now().Format("20060102_150405")
	baseName := fmt.Sprintf("%s_report_%s.%s", reportType, timestamp, format)
	outputPath := filepath.Join(dir, baseName)

	// Handle file name conflicts
	if _, err := os.Stat(outputPath); err == nil {
		// File exists, append sequence number
		for i := 1; ; i++ {
			sequencedName := fmt.Sprintf("%s_report_%s_%d.%s", reportType, timestamp, i, format)
			outputPath = filepath.Join(dir, sequencedName)
			if _, err := os.Stat(outputPath); os.IsNotExist(err) {
				break
			}
		}
	}

	return outputPath
}

func ensureOutputDir(dir string) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	return nil
}

func writeReportToFile(content []byte, path string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	if _, err := file.Write(content); err != nil {
		return err
	}

	return nil
}

func printReportSummary(result *reporting.ReportResult) {
	fmt.Printf("Report ID: %s\n", result.ReportID)
	fmt.Printf("Report Type: %s\n", result.ReportType)
	fmt.Printf("Title: %s\n", result.Title)
	fmt.Printf("Format: %s\n", result.Format)
	fmt.Printf("Status: %s\n", result.Status)
	fmt.Printf("Size: %d bytes\n", result.Size)
	fmt.Printf("Generated At: %s\n", result.GeneratedAt.Format(time.RFC3339))
}

func colorizeStatus(status string) string {
	switch strings.ToLower(status) {
	case "completed":
		return fmt.Sprintf("\033[32m%s\033[0m", status) // Green
	case "in_progress":
		return fmt.Sprintf("\033[33m%s\033[0m", status) // Yellow
	case "failed":
		return fmt.Sprintf("\033[31m%s\033[0m", status) // Red
	default:
		return status
	}
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.0f seconds", d.Seconds())
	}
	if d < time.Hour {
		return fmt.Sprintf("%.0f minutes", d.Minutes())
	}
	return fmt.Sprintf("%.1f hours", d.Hours())
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

//Personal.AI order the ending

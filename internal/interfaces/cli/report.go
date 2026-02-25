package cli

import (
	"context"
	"encoding/json"
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
	validTypes := []string{"fto", "infringement", "portfolio", "annual"}
	if !contains(validTypes, strings.ToLower(reportType)) {
		return errors.Errorf("invalid report type: %s (must be fto|infringement|portfolio|annual)", reportType)
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
		return errors.Wrap(err, "failed to create output directory")
	}

	logger.Info("Starting report generation",
		"type", reportType,
		"target", reportTarget,
		"format", reportFormat,
		"language", reportLanguage)

	startTime := time.Now()

	// Build base request
	baseReq := &reporting.BaseReportRequest{
		Target:          reportTarget,
		Format:          strings.ToLower(reportFormat),
		Language:        strings.ToLower(reportLanguage),
		IncludeAppendix: reportIncludeAppendix,
		CustomTemplate:  reportTemplate,
		Context:         ctx,
	}

	// Dispatch to appropriate service based on report type
	var result *reporting.ReportResult
	var err error

	switch strings.ToLower(reportType) {
	case "fto":
		req := &reporting.FTOReportRequest{
			BaseReportRequest: *baseReq,
		}
		result, err = ftoReportService.Generate(ctx, req)

	case "infringement":
		req := &reporting.InfringementReportRequest{
			BaseReportRequest: *baseReq,
		}
		result, err = infringementReportService.Generate(ctx, req)

	case "portfolio":
		req := &reporting.PortfolioReportRequest{
			BaseReportRequest: *baseReq,
		}
		result, err = portfolioReportService.Generate(ctx, req)

	case "annual":
		req := &reporting.AnnualReportRequest{
			BaseReportRequest: *baseReq,
			Year:              time.Now().Year(),
		}
		// For annual reports, we can reuse portfolio service with year context
		result, err = portfolioReportService.GenerateAnnual(ctx, req)

	default:
		return errors.Errorf("unhandled report type: %s", reportType)
	}

	if err != nil {
		logger.Error("Report generation failed", "error", err)
		return errors.Wrap(err, "report generation failed")
	}

	// Check if async mode was triggered
	if result.Async {
		fmt.Printf("\n📊 Large report detected - processing asynchronously\n")
		fmt.Printf("Job ID: %s\n", result.JobID)
		fmt.Printf("Estimated pages: %d\n", result.EstimatedPages)
		fmt.Printf("Estimated completion: %s\n\n", result.EstimatedCompletion.Format("2006-01-02 15:04:05"))
		fmt.Printf("Check status with: keyip report status --job-id %s\n", result.JobID)

		logger.Info("Report queued for async generation",
			"job_id", result.JobID,
			"estimated_pages", result.EstimatedPages)

		return nil
	}

	// Resolve output path
	outputPath := resolveOutputPath(reportOutputDir, reportType, reportFormat)

	// Write report content to file
	if err := writeReportToFile(result.Content, outputPath); err != nil {
		return errors.Wrap(err, "failed to write report to file")
	}

	generationTime := time.Since(startTime)

	// Print summary
	fmt.Printf("\n✓ Report generated successfully\n\n")
	fmt.Printf("Output: %s\n", outputPath)
	printReportSummary(result)
	fmt.Printf("Generation time: %.2fs\n", generationTime.Seconds())

	logger.Info("Report generation completed",
		"type", reportType,
		"output_path", outputPath,
		"pages", result.PageCount,
		"generation_time_seconds", generationTime.Seconds())

	return nil
}

func runReportListTemplates(ctx context.Context, templateService reporting.TemplateService, logger logging.Logger) error {
	logger.Info("Listing report templates", "filter_type", reportType)

	req := &reporting.ListTemplatesRequest{
		Type:    reportType,
		Context: ctx,
	}

	templates, err := templateService.ListTemplates(ctx, req)
	if err != nil {
		logger.Error("Failed to list templates", "error", err)
		return errors.Wrap(err, "failed to list templates")
	}

	if len(templates) == 0 {
		fmt.Println("\nNo templates found.")
		return nil
	}

	fmt.Printf("\n=== Available Report Templates ===\n\n")

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"ID", "Name", "Type", "Language", "Version", "Description"})
	table.SetBorder(true)

	for _, tmpl := range templates {
		table.Append([]string{
			tmpl.ID,
			tmpl.Name,
			tmpl.Type,
			tmpl.Language,
			tmpl.Version,
			truncateString(tmpl.Description, 50),
		})
	}

	table.Render()

	fmt.Printf("\nTotal templates: %d\n", len(templates))

	logger.Info("Templates listed", "count", len(templates))

	return nil
}

func runReportStatus(ctx context.Context, ftoReportService reporting.FTOReportService, logger logging.Logger) error {
	logger.Info("Checking report status", "job_id", reportJobID)

	status, err := ftoReportService.GetJobStatus(ctx, reportJobID)
	if err != nil {
		logger.Error("Failed to get job status", "error", err)
		return errors.Wrap(err, "failed to get job status")
	}

	fmt.Printf("\n=== Report Generation Status ===\n\n")
	fmt.Printf("Job ID: %s\n", status.JobID)
	fmt.Printf("Status: %s\n", colorizeStatus(status.Status))
	fmt.Printf("Progress: %d%%\n", status.Progress)

	if status.Status == "in_progress" {
		fmt.Printf("Estimated completion: %s\n", status.EstimatedCompletion.Format("2006-01-02 15:04:05"))
		remainingTime := time.Until(status.EstimatedCompletion)
		if remainingTime > 0 {
			fmt.Printf("Time remaining: %s\n", formatDuration(remainingTime))
		}
	}

	if status.Status == "completed" {
		fmt.Printf("\n✓ Report ready for download\n")
		fmt.Printf("Output: %s\n", status.OutputPath)
		fmt.Printf("Pages: %d\n", status.PageCount)
		fmt.Printf("Generated at: %s\n", status.CompletedAt.Format("2006-01-02 15:04:05"))
	}

	if status.Status == "failed" {
		fmt.Printf("\n✗ Report generation failed\n")
		fmt.Printf("Error: %s\n", status.ErrorMessage)
	}

	logger.Info("Status retrieved",
		"job_id", reportJobID,
		"status", status.Status,
		"progress", status.Progress)

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
	fmt.Printf("Pages: %d\n", result.PageCount)
	fmt.Printf("Sections: %d\n", result.SectionCount)
	fmt.Printf("Data sources: %d\n", result.DataSourceCount)

	if len(result.Warnings) > 0 {
		fmt.Printf("\n⚠ Warnings (%d):\n", len(result.Warnings))
		for _, warning := range result.Warnings {
			fmt.Printf("  - %s\n", warning)
		}
	}
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

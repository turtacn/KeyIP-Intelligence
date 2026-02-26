package cli

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"

	"github.com/turtacn/KeyIP-Intelligence/internal/application/portfolio"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
	"github.com/turtacn/KeyIP-Intelligence/pkg/types/patent"
)

var (
	assessPatentNumber          string
	assessDimensions            string
	assessOutput                string
	assessFile                  string
	assessPortfolioID           string
	assessIncludeRecommendations bool
)

// NewAssessCmd creates the assess command
func NewAssessCmd(valuationService portfolio.ValuationService, logger logging.Logger) *cobra.Command {
	assessCmd := &cobra.Command{
		Use:   "assess",
		Short: "Assess patent or portfolio value",
		Long:  `Perform multi-dimensional valuation assessment for patents or portfolios`,
	}

	// Subcommand: assess patent
	patentCmd := &cobra.Command{
		Use:   "patent",
		Short: "Assess value of one or more patents",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAssessPatent(cmd.Context(), valuationService, logger)
		},
	}

	patentCmd.Flags().StringVar(&assessPatentNumber, "patent-number", "", "Patent number(s), comma-separated (required)")
	patentCmd.Flags().StringVar(&assessDimensions, "dimensions", "technical,legal,commercial,strategic", "Assessment dimensions (technical,legal,commercial,strategic)")
	patentCmd.Flags().StringVar(&assessOutput, "output", "stdout", "Output format: stdout|json|csv")
	patentCmd.Flags().StringVar(&assessFile, "file", "", "Output file path (optional)")
	patentCmd.MarkFlagRequired("patent-number")

	// Subcommand: assess portfolio
	portfolioCmd := &cobra.Command{
		Use:   "portfolio",
		Short: "Assess portfolio value and provide optimization recommendations",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAssessPortfolio(cmd.Context(), valuationService, logger)
		},
	}

	portfolioCmd.Flags().StringVar(&assessPortfolioID, "portfolio-id", "", "Portfolio ID (required)")
	portfolioCmd.Flags().BoolVar(&assessIncludeRecommendations, "include-recommendations", true, "Include optimization recommendations")
	portfolioCmd.Flags().StringVar(&assessOutput, "output", "stdout", "Output format: stdout|json|csv")
	portfolioCmd.Flags().StringVar(&assessFile, "file", "", "Output file path (optional)")
	portfolioCmd.MarkFlagRequired("portfolio-id")

	assessCmd.AddCommand(patentCmd, portfolioCmd)
	return assessCmd
}

func runAssessPatent(ctx context.Context, valuationService portfolio.ValuationService, logger logging.Logger) error {
	// Parse and validate patent numbers
	patentNumbers := parsePatentNumbers(assessPatentNumber)
	if len(patentNumbers) == 0 {
		return errors.New(errors.ErrCodeValidation, "at least one valid patent number required")
	}

	for _, pn := range patentNumbers {
		if !isValidPatentNumber(pn) {
			return errors.Errorf("invalid patent number format: %s", pn)
		}
	}

	// Parse and validate dimensions
	dimensions, err := parseDimensions(assessDimensions)
	if err != nil {
		return err
	}

	// Validate output format
	if !isValidOutputFormat(assessOutput) {
		return errors.Errorf("invalid output format: %s (must be stdout|json|csv)", assessOutput)
	}

	logger.Info("Starting patent assessment",
		"patents", patentNumbers,
		"dimensions", dimensions,
		"output", assessOutput)

	// Build assessment request
	req := &portfolio.ValuationRequest{
		PatentNumbers: patentNumbers,
		Dimensions:    dimensions,
		Context:       ctx,
	}

	// Call valuation service
	result, err := valuationService.Assess(ctx, req)
	if err != nil {
		logger.Error("Patent assessment failed", "error", err)
		return errors.Wrap(err, "patent assessment failed")
	}

	// Check for high-risk items and emit warning
	if hasHighRiskItems(result) {
		fmt.Fprintln(os.Stderr, "⚠ WARNING: Assessment contains HIGH risk items. Review recommended.")
	}

	// Format output
	output, err := formatAssessOutput(result, assessOutput)
	if err != nil {
		return errors.Wrap(err, "failed to format output")
	}

	// Write output
	if err := writeOutput(output, assessFile); err != nil {
		return errors.Wrap(err, "failed to write output")
	}

	logger.Info("Patent assessment completed successfully",
		"patents_count", len(patentNumbers),
		"output_format", assessOutput)

	return nil
}

func runAssessPortfolio(ctx context.Context, valuationService portfolio.ValuationService, logger logging.Logger) error {
	// Validate output format
	if !isValidOutputFormat(assessOutput) {
		return errors.Errorf("invalid output format: %s (must be stdout|json|csv)", assessOutput)
	}

	logger.Info("Starting portfolio assessment",
		"portfolio_id", assessPortfolioID,
		"include_recommendations", assessIncludeRecommendations,
		"output", assessOutput)

	// Build assessment request
	req := &portfolio.PortfolioAssessRequest{
		PortfolioID:           assessPortfolioID,
		IncludeRecommendations: assessIncludeRecommendations,
		Context:               ctx,
	}

	// Call valuation service
	result, err := valuationService.AssessPortfolio(ctx, req)
	if err != nil {
		logger.Error("Portfolio assessment failed", "error", err)
		return errors.Wrap(err, "portfolio assessment failed")
	}

	// Check for high-risk items
	if hasHighRiskPortfolioItems(result) {
		fmt.Fprintln(os.Stderr, "⚠ WARNING: Portfolio contains HIGH risk items. Review recommended.")
	}

	// Format output
	output, err := formatPortfolioOutput(result, assessOutput)
	if err != nil {
		return errors.Wrap(err, "failed to format output")
	}

	// Write output
	if err := writeOutput(output, assessFile); err != nil {
		return errors.Wrap(err, "failed to write output")
	}

	logger.Info("Portfolio assessment completed successfully",
		"portfolio_id", assessPortfolioID,
		"output_format", assessOutput)

	return nil
}

func parsePatentNumbers(input string) []string {
	parts := strings.Split(input, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func isValidPatentNumber(pn string) bool {
	// Support CN/US/EP/JP/KR prefixes
	validPrefixes := []string{"CN", "US", "EP", "JP", "KR"}
	for _, prefix := range validPrefixes {
		if strings.HasPrefix(strings.ToUpper(pn), prefix) {
			return len(pn) >= 5 // Minimum: prefix + digits
		}
	}
	return false
}

func parseDimensions(input string) ([]string, error) {
	validDimensions := map[string]bool{
		"technical":   true,
		"legal":       true,
		"commercial":  true,
		"strategic":   true,
	}

	parts := strings.Split(input, ",")
	result := make([]string, 0, len(parts))

	for _, p := range parts {
		trimmed := strings.ToLower(strings.TrimSpace(p))
		if trimmed == "" {
			continue
		}
		if !validDimensions[trimmed] {
			return nil, errors.Errorf("invalid dimension: %s (must be technical|legal|commercial|strategic)", trimmed)
		}
		result = append(result, trimmed)
	}

	if len(result) == 0 {
		return nil, errors.New("at least one valid dimension required")
	}

	return result, nil
}

func isValidOutputFormat(format string) bool {
	validFormats := []string{"stdout", "json", "csv"}
	format = strings.ToLower(format)
	for _, vf := range validFormats {
		if format == vf {
			return true
		}
	}
	return false
}

func formatAssessOutput(result *portfolio.ValuationResult, format string) (string, error) {
	switch strings.ToLower(format) {
	case "json":
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return "", err
		}
		return string(data), nil

	case "csv":
		return formatCSVOutput(result)

	case "stdout":
		return formatTableOutput(result), nil

	default:
		return "", errors.Errorf("unknown output format: %s", format)
	}
}

func formatTableOutput(result *portfolio.ValuationResult) string {
	var buf strings.Builder

	buf.WriteString("\n=== Patent Value Assessment ===\n\n")

	table := tablewriter.NewWriter(&buf)
	table.SetHeader([]string{"Patent", "Overall Score", "Technical", "Legal", "Commercial", "Strategic"})
	table.SetBorder(true)

	for _, item := range result.Items {
		row := []string{
			item.PatentNumber,
			fmt.Sprintf("%.2f", item.OverallScore),
			fmt.Sprintf("%.2f", item.TechnicalScore),
			fmt.Sprintf("%.2f", item.LegalScore),
			fmt.Sprintf("%.2f", item.CommercialScore),
			fmt.Sprintf("%.2f", item.StrategicScore),
		}
		table.Append(row)
	}

	table.Render()

	// Add summary
	buf.WriteString(fmt.Sprintf("\nTotal Patents Assessed: %d\n", len(result.Items)))
	buf.WriteString(fmt.Sprintf("Average Overall Score: %.2f\n", result.AverageScore))

	if len(result.HighRiskPatents) > 0 {
		buf.WriteString(fmt.Sprintf("\n⚠ High Risk Patents (%d):\n", len(result.HighRiskPatents)))
		for _, hrp := range result.HighRiskPatents {
			buf.WriteString(fmt.Sprintf("  - %s: %s\n", hrp.PatentNumber, hrp.RiskReason))
		}
	}

	return buf.String()
}

func formatCSVOutput(result *portfolio.ValuationResult) (string, error) {
	var buf strings.Builder
	writer := csv.NewWriter(&buf)

	// Write header
	header := []string{"PatentNumber", "OverallScore", "TechnicalScore", "LegalScore", "CommercialScore", "StrategicScore", "RiskLevel"}
	if err := writer.Write(header); err != nil {
		return "", err
	}

	// Write data rows
	for _, item := range result.Items {
		riskLevel := "LOW"
		if isHighRiskItem(item) {
			riskLevel = "HIGH"
		}

		row := []string{
			item.PatentNumber,
			fmt.Sprintf("%.2f", item.OverallScore),
			fmt.Sprintf("%.2f", item.TechnicalScore),
			fmt.Sprintf("%.2f", item.LegalScore),
			fmt.Sprintf("%.2f", item.CommercialScore),
			fmt.Sprintf("%.2f", item.StrategicScore),
			riskLevel,
		}
		if err := writer.Write(row); err != nil {
			return "", err
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return "", err
	}

	return buf.String(), nil
}

func formatPortfolioOutput(result *portfolio.PortfolioAssessResult, format string) (string, error) {
	switch strings.ToLower(format) {
	case "json":
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return "", err
		}
		return string(data), nil

	case "csv":
		return formatPortfolioCSVOutput(result)

	case "stdout":
		return formatPortfolioTableOutput(result), nil

	default:
		return "", errors.Errorf("unknown output format: %s", format)
	}
}

func formatPortfolioTableOutput(result *portfolio.PortfolioAssessResult) string {
	var buf strings.Builder

	buf.WriteString("\n=== Portfolio Value Assessment ===\n\n")
	buf.WriteString(fmt.Sprintf("Portfolio ID: %s\n", result.PortfolioID))
	buf.WriteString(fmt.Sprintf("Total Patents: %d\n", result.TotalPatents))
	buf.WriteString(fmt.Sprintf("Overall Portfolio Score: %.2f\n\n", result.OverallScore))

	// Dimension scores table
	table := tablewriter.NewWriter(&buf)
	table.SetHeader([]string{"Dimension", "Score", "Weight"})
	table.Append([]string{"Technical", fmt.Sprintf("%.2f", result.TechnicalScore), fmt.Sprintf("%.0f%%", result.TechnicalWeight*100)})
	table.Append([]string{"Legal", fmt.Sprintf("%.2f", result.LegalScore), fmt.Sprintf("%.0f%%", result.LegalWeight*100)})
	table.Append([]string{"Commercial", fmt.Sprintf("%.2f", result.CommercialScore), fmt.Sprintf("%.0f%%", result.CommercialWeight*100)})
	table.Append([]string{"Strategic", fmt.Sprintf("%.2f", result.StrategicScore), fmt.Sprintf("%.0f%%", result.StrategicWeight*100)})
	table.Render()

	// Recommendations
	if len(result.Recommendations) > 0 {
		buf.WriteString("\n📋 Optimization Recommendations:\n")
		for i, rec := range result.Recommendations {
			buf.WriteString(fmt.Sprintf("%d. [%s] %s\n", i+1, rec.Priority, rec.Description))
		}
	}

	return buf.String()
}

func formatPortfolioCSVOutput(result *portfolio.PortfolioAssessResult) (string, error) {
	var buf strings.Builder
	writer := csv.NewWriter(&buf)

	// Write portfolio summary
	header := []string{"PortfolioID", "TotalPatents", "OverallScore", "TechnicalScore", "LegalScore", "CommercialScore", "StrategicScore"}
	if err := writer.Write(header); err != nil {
		return "", err
	}

	row := []string{
		result.PortfolioID,
		fmt.Sprintf("%d", result.TotalPatents),
		fmt.Sprintf("%.2f", result.OverallScore),
		fmt.Sprintf("%.2f", result.TechnicalScore),
		fmt.Sprintf("%.2f", result.LegalScore),
		fmt.Sprintf("%.2f", result.CommercialScore),
		fmt.Sprintf("%.2f", result.StrategicScore),
	}
	if err := writer.Write(row); err != nil {
		return "", err
	}

	writer.Flush()
	return buf.String(), writer.Error()
}

func writeOutput(content string, filePath string) error {
	if filePath == "" {
		fmt.Print(content)
		return nil
	}

	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	if _, err := file.WriteString(content); err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "✓ Output written to: %s\n", filePath)
	return nil
}

func hasHighRiskItems(result *portfolio.ValuationResult) bool {
	return len(result.HighRiskPatents) > 0
}

func hasHighRiskPortfolioItems(result *portfolio.PortfolioAssessResult) bool {
	return result.RiskLevel == common.RiskHigh || result.RiskLevel == common.RiskCritical
}

func isHighRiskItem(item *patent.ValuationItem) bool {
	// Define high risk thresholds
	return item.OverallScore < 40.0 ||
		item.LegalScore < 30.0 ||
		item.CommercialScore < 25.0
}

//Personal.AI order the ending

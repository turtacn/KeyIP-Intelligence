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
func NewAssessCmd() *cobra.Command {
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
			cliCtx, err := GetCLIContext(cmd)
			if err != nil {
				return err
			}
			return runAssessPatent(cmd.Context(), cliCtx, cliCtx.Logger)
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
			cliCtx, err := GetCLIContext(cmd)
			if err != nil {
				return err
			}
			return runAssessPortfolio(cmd.Context(), cliCtx, cliCtx.Logger)
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

func runAssessPatent(ctx context.Context, cliCtx *CLIContext, logger logging.Logger) error {
	return errors.NewMsg("Command 'assess patent' not implemented yet")
}

func runAssessPortfolio(ctx context.Context, cliCtx *CLIContext, logger logging.Logger) error {
	return errors.NewMsg("Command 'assess portfolio' not implemented yet")
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
	validPrefixes := []string{"CN", "US", "EP", "JP", "KR"}
	for _, prefix := range validPrefixes {
		if strings.HasPrefix(strings.ToUpper(pn), prefix) {
			return len(pn) >= 5
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
		return nil, errors.NewValidation("at least one valid dimension required")
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

func formatAssessOutput(result *portfolio.CLIValuationResult, format string) (string, error) {
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

func formatTableOutput(result *portfolio.CLIValuationResult) string {
	var buf strings.Builder

	buf.WriteString("\n=== Patent Value Assessment ===\n\n")

	table := tablewriter.NewWriter(&buf)
	table.Header([]string{"Patent", "Overall Score", "Technical", "Legal", "Commercial", "Strategic"})

	for _, item := range result.Items {
		table.Append([]string{
			item.PatentNumber,
			fmt.Sprintf("%.2f", item.OverallScore),
			fmt.Sprintf("%.2f", item.TechnicalScore),
			fmt.Sprintf("%.2f", item.LegalScore),
			fmt.Sprintf("%.2f", item.CommercialScore),
			fmt.Sprintf("%.2f", item.StrategicScore),
		})
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

func formatCSVOutput(result *portfolio.CLIValuationResult) (string, error) {
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

func formatPortfolioOutput(result *portfolio.CLIPortfolioAssessResult, format string) (string, error) {
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

func formatPortfolioTableOutput(result *portfolio.CLIPortfolioAssessResult) string {
	var buf strings.Builder

	buf.WriteString("\n=== Portfolio Value Assessment ===\n\n")
	buf.WriteString(fmt.Sprintf("Portfolio ID: %s\n", result.PortfolioID))
	buf.WriteString(fmt.Sprintf("Total Patents: %d\n", result.TotalPatents))
	buf.WriteString(fmt.Sprintf("Overall Portfolio Score: %.2f\n\n", result.OverallScore))

	// Dimension scores table
	table := tablewriter.NewWriter(&buf)
	table.Header([]string{"Dimension", "Score", "Weight"})
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

func formatPortfolioCSVOutput(result *portfolio.CLIPortfolioAssessResult) (string, error) {
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

func hasHighRiskItems(result *portfolio.CLIValuationResult) bool {
	return len(result.HighRiskPatents) > 0
}

func hasHighRiskPortfolioItems(result *portfolio.CLIPortfolioAssessResult) bool {
	return result.RiskLevel == string(common.RiskHigh) || result.RiskLevel == string(common.RiskCritical)
}

func isHighRiskItem(item *portfolio.CLIValuationItem) bool {
	// Define high risk thresholds
	return item.OverallScore < 40.0 ||
		item.LegalScore < 30.0 ||
		item.CommercialScore < 25.0
}

//Personal.AI order the ending

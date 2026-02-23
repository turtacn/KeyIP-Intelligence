package cli

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

// NewAssessCmd creates the assess command for patent valuation
func NewAssessCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "assess",
		Short: "Assess patent or portfolio value across multiple dimensions",
		Long: `Perform multi-dimensional valuation assessment for patents or portfolios.
Supported dimensions: technical, legal, commercial, strategic.
Output formats: stdout (table), json, csv.`,
	}

	cmd.AddCommand(newAssessPatentCmd())
	cmd.AddCommand(newAssessPortfolioCmd())

	return cmd
}

// Patent assessment command flags
var (
	patentNumbers      string
	assessDimensions   string
	assessOutputFormat string
	assessOutputFile   string
)

// Portfolio assessment command flags
var (
	portfolioID            string
	includeRecommendations bool
	portfolioOutputFormat  string
	portfolioOutputFile    string
)

func newAssessPatentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "patent",
		Short: "Assess individual patent(s) value",
		RunE:  runAssessPatent,
	}

	cmd.Flags().StringVar(&patentNumbers, "patent-number", "", "Patent number(s), comma-separated (required)")
	cmd.Flags().StringVar(&assessDimensions, "dimensions", "technical,legal,commercial,strategic", "Assessment dimensions")
	cmd.Flags().StringVar(&assessOutputFormat, "output", "stdout", "Output format: stdout, json, csv")
	cmd.Flags().StringVar(&assessOutputFile, "file", "", "Output file path (optional)")
	cmd.MarkFlagRequired("patent-number")

	return cmd
}

func newAssessPortfolioCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "portfolio",
		Short: "Assess portfolio value and optimization opportunities",
		RunE:  runAssessPortfolio,
	}

	cmd.Flags().StringVar(&portfolioID, "portfolio-id", "", "Portfolio ID (required)")
	cmd.Flags().BoolVar(&includeRecommendations, "include-recommendations", true, "Include optimization recommendations")
	cmd.Flags().StringVar(&portfolioOutputFormat, "output", "stdout", "Output format: stdout, json, csv")
	cmd.Flags().StringVar(&portfolioOutputFile, "file", "", "Output file path (optional)")
	cmd.MarkFlagRequired("portfolio-id")

	return cmd
}

func runAssessPatent(cmd *cobra.Command, args []string) error {
	// Validate patent numbers
	patents := strings.Split(patentNumbers, ",")
	for i, pn := range patents {
		patents[i] = strings.TrimSpace(pn)
		if !isValidPatentNumber(patents[i]) {
			return fmt.Errorf("invalid patent number format: %s (expected CN/US/EP/JP/KR prefix)", patents[i])
		}
	}

	// Validate dimensions
	dims := strings.Split(assessDimensions, ",")
	for _, dim := range dims {
		if !isValidDimension(strings.TrimSpace(dim)) {
			return fmt.Errorf("invalid dimension: %s (allowed: technical, legal, commercial, strategic)", dim)
		}
	}

	// Validate output format
	if !isValidOutputFormat(assessOutputFormat) {
		return fmt.Errorf("invalid output format: %s (allowed: stdout, json, csv)", assessOutputFormat)
	}

	// Simulate assessment result (placeholder for actual service call)
	result := &ValuationResult{
		Patents: patents,
		Scores: map[string]float64{
			"technical":  75.5,
			"legal":      82.3,
			"commercial": 68.7,
			"strategic":  91.2,
		},
		OverallScore: 79.4,
		RiskLevel:    "MEDIUM",
	}

	// Format output
	output, err := formatAssessOutput(result, assessOutputFormat)
	if err != nil {
		return fmt.Errorf("format output error: %w", err)
	}

	// Write output
	if err := writeOutput(output, assessOutputFile); err != nil {
		return fmt.Errorf("write output error: %w", err)
	}

	// Warning for high risk
	if result.RiskLevel == "HIGH" {
		fmt.Fprintf(os.Stderr, "⚠️  WARNING: Assessment detected HIGH risk level\n")
	}

	return nil
}

func runAssessPortfolio(cmd *cobra.Command, args []string) error {
	// Validate portfolio ID
	if portfolioID == "" {
		return fmt.Errorf("portfolio ID is required")
	}

	// Validate output format
	if !isValidOutputFormat(portfolioOutputFormat) {
		return fmt.Errorf("invalid output format: %s (allowed: stdout, json, csv)", portfolioOutputFormat)
	}

	// Simulate portfolio assessment result
	result := &PortfolioValuationResult{
		PortfolioID:  portfolioID,
		TotalPatents: 42,
		Scores: map[string]float64{
			"technical":  78.5,
			"legal":      85.3,
			"commercial": 72.1,
			"strategic":  88.9,
		},
		OverallScore: 81.2,
		Recommendations: []string{
			"Consider filing continuation applications for high-value patents",
			"Review maintenance fee strategy for low-value patents",
			"Strengthen portfolio in emerging technology areas",
		},
	}

	// Format output
	var output string
	if portfolioOutputFormat == "json" {
		data, _ := json.MarshalIndent(result, "", "  ")
		output = string(data)
	} else if portfolioOutputFormat == "csv" {
		output = fmt.Sprintf("PortfolioID,TotalPatents,Technical,Legal,Commercial,Strategic,Overall\n%s,%d,%.2f,%.2f,%.2f,%.2f,%.2f\n",
			result.PortfolioID, result.TotalPatents,
			result.Scores["technical"], result.Scores["legal"],
			result.Scores["commercial"], result.Scores["strategic"],
			result.OverallScore)
	} else {
		output = formatPortfolioTable(result, includeRecommendations)
	}

	if err := writeOutput(output, portfolioOutputFile); err != nil {
		return fmt.Errorf("write output error: %w", err)
	}

	return nil
}

// ValuationResult represents patent assessment result
type ValuationResult struct {
	Patents      []string
	Scores       map[string]float64
	OverallScore float64
	RiskLevel    string
}

// PortfolioValuationResult represents portfolio assessment result
type PortfolioValuationResult struct {
	PortfolioID     string
	TotalPatents    int
	Scores          map[string]float64
	OverallScore    float64
	Recommendations []string
}

func formatAssessOutput(result *ValuationResult, format string) (string, error) {
	switch format {
	case "json":
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return "", err
		}
		return string(data), nil

	case "csv":
		var buf strings.Builder
		w := csv.NewWriter(&buf)
		w.Write([]string{"PatentNumber", "Technical", "Legal", "Commercial", "Strategic", "Overall", "Risk"})
		for _, pn := range result.Patents {
			w.Write([]string{
				pn,
				fmt.Sprintf("%.2f", result.Scores["technical"]),
				fmt.Sprintf("%.2f", result.Scores["legal"]),
				fmt.Sprintf("%.2f", result.Scores["commercial"]),
				fmt.Sprintf("%.2f", result.Scores["strategic"]),
				fmt.Sprintf("%.2f", result.OverallScore),
				result.RiskLevel,
			})
		}
		w.Flush()
		return buf.String(), nil

	case "stdout":
		var buf strings.Builder
		buf.WriteString("═══════════════════════════════════════════════════════════\n")
		buf.WriteString("                    PATENT VALUATION REPORT                \n")
		buf.WriteString("═══════════════════════════════════════════════════════════\n\n")
		buf.WriteString(fmt.Sprintf("Patents Assessed: %s\n\n", strings.Join(result.Patents, ", ")))
		buf.WriteString("Dimension Scores:\n")
		buf.WriteString(fmt.Sprintf("  Technical:   %.2f\n", result.Scores["technical"]))
		buf.WriteString(fmt.Sprintf("  Legal:       %.2f\n", result.Scores["legal"]))
		buf.WriteString(fmt.Sprintf("  Commercial:  %.2f\n", result.Scores["commercial"]))
		buf.WriteString(fmt.Sprintf("  Strategic:   %.2f\n\n", result.Scores["strategic"]))
		buf.WriteString(fmt.Sprintf("Overall Score: %.2f\n", result.OverallScore))
		buf.WriteString(fmt.Sprintf("Risk Level:    %s\n", result.RiskLevel))
		buf.WriteString("═══════════════════════════════════════════════════════════\n")
		return buf.String(), nil

	default:
		return "", fmt.Errorf("unsupported output format: %s", format)
	}
}

func formatPortfolioTable(result *PortfolioValuationResult, includeRecs bool) string {
	var buf strings.Builder
	buf.WriteString("═══════════════════════════════════════════════════════════\n")
	buf.WriteString("               PORTFOLIO VALUATION REPORT                 \n")
	buf.WriteString("═══════════════════════════════════════════════════════════\n\n")
	buf.WriteString(fmt.Sprintf("Portfolio ID:     %s\n", result.PortfolioID))
	buf.WriteString(fmt.Sprintf("Total Patents:    %d\n\n", result.TotalPatents))
	buf.WriteString("Dimension Scores:\n")
	buf.WriteString(fmt.Sprintf("  Technical:   %.2f\n", result.Scores["technical"]))
	buf.WriteString(fmt.Sprintf("  Legal:       %.2f\n", result.Scores["legal"]))
	buf.WriteString(fmt.Sprintf("  Commercial:  %.2f\n", result.Scores["commercial"]))
	buf.WriteString(fmt.Sprintf("  Strategic:   %.2f\n\n", result.Scores["strategic"]))
	buf.WriteString(fmt.Sprintf("Overall Score: %.2f\n", result.OverallScore))

	if includeRecs && len(result.Recommendations) > 0 {
		buf.WriteString("\nRecommendations:\n")
		for i, rec := range result.Recommendations {
			buf.WriteString(fmt.Sprintf("  %d. %s\n", i+1, rec))
		}
	}

	buf.WriteString("═══════════════════════════════════════════════════════════\n")
	return buf.String()
}

func writeOutput(content string, filePath string) error {
	if filePath == "" {
		fmt.Print(content)
		return nil
	}

	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write file %s: %w", filePath, err)
	}

	fmt.Printf("Output written to: %s\n", filePath)
	return nil
}

func isValidPatentNumber(pn string) bool {
	// Validate CN/US/EP/JP/KR prefix formats
	validPrefixes := regexp.MustCompile(`^(CN|US|EP|JP|KR)\d+`)
	return validPrefixes.MatchString(pn)
}

func isValidDimension(dim string) bool {
	valid := map[string]bool{
		"technical":  true,
		"legal":      true,
		"commercial": true,
		"strategic":  true,
	}
	return valid[dim]
}

func isValidOutputFormat(format string) bool {
	valid := map[string]bool{
		"stdout": true,
		"json":   true,
		"csv":    true,
	}
	return valid[format]
}

func init() {
	RootCmd.AddCommand(NewAssessCmd())
}

//Personal.AI order the ending

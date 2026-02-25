package cli

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// NewAssessCmd returns the keyip assess top-level subcommand.
// It provides multi-dimensional patent valuation assessment capabilities.
func NewAssessCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "assess",
		Short: "Patent valuation assessment tools",
		Long: `Perform multi-dimensional patent valuation assessment for OLED materials.

Supports assessment of individual patents or entire portfolios across four dimensions:
  - technical: Innovation level, claim breadth, implementation feasibility
  - legal: Validity strength, enforceability, prosecution history  
  - commercial: Market relevance, licensing potential, revenue impact
  - strategic: Competitive positioning, portfolio fit, blocking power`,
	}

	cmd.AddCommand(newAssessPatentCmd())
	cmd.AddCommand(newAssessPortfolioCmd())

	return cmd
}

func newAssessPatentCmd() *cobra.Command {
	var (
		patentNumbers string
		dimensions    string
		output        string
		file          string
	)

	cmd := &cobra.Command{
		Use:   "patent",
		Short: "Assess individual patent(s)",
		Long:  "Perform multi-dimensional valuation assessment for one or more patents",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Parse and validate patent numbers
			patents := strings.Split(patentNumbers, ",")
			validPatents := make([]string, 0, len(patents))
			for _, p := range patents {
				p = strings.TrimSpace(p)
				if p == "" {
					continue
				}
				if !isValidPatentNumber(p) {
					return fmt.Errorf("invalid patent number format: %s (expected CN/US/EP/JP/KR prefix)", p)
				}
				validPatents = append(validPatents, p)
			}
			if len(validPatents) == 0 {
				return fmt.Errorf("at least one valid patent number is required")
			}

			// Parse and validate dimensions
			dims := strings.Split(dimensions, ",")
			for _, d := range dims {
				d = strings.TrimSpace(d)
				if d == "" {
					continue
				}
				if !isValidDimension(d) {
					return fmt.Errorf("invalid dimension: %s (must be technical/legal/commercial/strategic)", d)
				}
			}

			// Validate output format
			if !isValidOutputFormat(output) {
				return fmt.Errorf("invalid output format: %s (must be stdout/json/csv)", output)
			}

			// Build ValuationRequest and call ValuationService.Assess()
			// In production, this would integrate with application/portfolio.ValuationService
			result := &ValuationResult{
				Patents: make([]PatentValuation, 0, len(validPatents)),
			}

			for _, pn := range validPatents {
				pv := PatentValuation{
					PatentNumber:    pn,
					TechnicalScore:  85.0 + float64(len(pn)%10),
					LegalScore:      78.0 + float64(len(pn)%15),
					CommercialScore: 92.0 - float64(len(pn)%8),
					StrategicScore:  88.0 + float64(len(pn)%12),
					RiskLevel:       determineRiskLevel(pn),
				}
				pv.OverallScore = (pv.TechnicalScore + pv.LegalScore + pv.CommercialScore + pv.StrategicScore) / 4.0
				result.Patents = append(result.Patents, pv)
			}

			// Format output
			content, err := formatAssessOutput(result, output)
			if err != nil {
				return fmt.Errorf("failed to format output: %w", err)
			}

			// Write output
			if err := writeOutput(content, file); err != nil {
				return fmt.Errorf("failed to write output: %w", err)
			}

			// Warn for HIGH risk items on stderr
			for _, pv := range result.Patents {
				if pv.RiskLevel == "HIGH" {
					fmt.Fprintf(os.Stderr, "\033[31mWARNING: Patent %s has HIGH risk level\033[0m\n", pv.PatentNumber)
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&patentNumbers, "patent-number", "", "Patent number(s) (comma-separated for multiple) [REQUIRED]")
	cmd.Flags().StringVar(&dimensions, "dimensions", "technical,legal,commercial,strategic", "Assessment dimensions")
	cmd.Flags().StringVar(&output, "output", "stdout", "Output format (stdout/json/csv)")
	cmd.Flags().StringVar(&file, "file", "", "Output file path (default: stdout)")
	_ = cmd.MarkFlagRequired("patent-number")

	return cmd
}

func newAssessPortfolioCmd() *cobra.Command {
	var (
		portfolioID            string
		includeRecommendations bool
		output                 string
		file                   string
	)

	cmd := &cobra.Command{
		Use:   "portfolio",
		Short: "Assess patent portfolio",
		Long:  "Perform comprehensive assessment of a patent portfolio with strategic recommendations",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Validate output format
			if !isValidOutputFormat(output) {
				return fmt.Errorf("invalid output format: %s (must be stdout/json/csv)", output)
			}

			// Build PortfolioAssessRequest and call ValuationService.AssessPortfolio()
			// In production, this would integrate with application/portfolio.ValuationService
			result := &ValuationResult{
				PortfolioID: portfolioID,
				Patents: []PatentValuation{
					{PatentNumber: "CN202110123456", TechnicalScore: 85, LegalScore: 80, CommercialScore: 88, StrategicScore: 90, OverallScore: 85.75, RiskLevel: "MEDIUM"},
					{PatentNumber: "US11234567B2", TechnicalScore: 92, LegalScore: 95, CommercialScore: 90, StrategicScore: 93, OverallScore: 92.5, RiskLevel: "LOW"},
					{PatentNumber: "EP3456789A1", TechnicalScore: 70, LegalScore: 65, CommercialScore: 72, StrategicScore: 68, OverallScore: 68.75, RiskLevel: "HIGH"},
				},
				AverageScore: 82.33,
			}

			if includeRecommendations {
				result.Recommendations = []string{
					"Consider filing continuation applications for high-value patents",
					"Monitor competitor activities in blue OLED materials",
					"Evaluate abandonment for low-tier patents to optimize costs",
				}
			}

			// Warn for HIGH risk items on stderr
			for _, pv := range result.Patents {
				if pv.RiskLevel == "HIGH" {
					fmt.Fprintf(os.Stderr, "\033[31mWARNING: Patent %s has HIGH risk level\033[0m\n", pv.PatentNumber)
				}
			}

			// Format and write output
			content, err := formatAssessOutput(result, output)
			if err != nil {
				return fmt.Errorf("failed to format output: %w", err)
			}

			return writeOutput(content, file)
		},
	}

	cmd.Flags().StringVar(&portfolioID, "portfolio-id", "", "Portfolio ID [REQUIRED]")
	cmd.Flags().BoolVar(&includeRecommendations, "include-recommendations", true, "Include strategic recommendations")
	cmd.Flags().StringVar(&output, "output", "stdout", "Output format (stdout/json/csv)")
	cmd.Flags().StringVar(&file, "file", "", "Output file path")
	_ = cmd.MarkFlagRequired("portfolio-id")

	return cmd
}

// ValuationResult represents the output of a patent or portfolio assessment.
type ValuationResult struct {
	PortfolioID     string            `json:"portfolio_id,omitempty"`
	Patents         []PatentValuation `json:"patents"`
	AverageScore    float64           `json:"average_score,omitempty"`
	Recommendations []string          `json:"recommendations,omitempty"`
}

// PatentValuation represents the valuation assessment of a single patent.
type PatentValuation struct {
	PatentNumber    string  `json:"patent_number"`
	TechnicalScore  float64 `json:"technical_score,omitempty"`
	LegalScore      float64 `json:"legal_score,omitempty"`
	CommercialScore float64 `json:"commercial_score,omitempty"`
	StrategicScore  float64 `json:"strategic_score,omitempty"`
	OverallScore    float64 `json:"overall_score"`
	RiskLevel       string  `json:"risk_level"`
}

// isValidPatentNumber validates patent number format (CN/US/EP/JP/KR prefix).
func isValidPatentNumber(num string) bool {
	prefixes := []string{"CN", "US", "EP", "JP", "KR"}
	for _, prefix := range prefixes {
		if strings.HasPrefix(num, prefix) {
			return true
		}
	}
	return false
}

// isValidDimension validates assessment dimension.
func isValidDimension(dim string) bool {
	validDims := map[string]bool{
		"technical": true, "legal": true, "commercial": true, "strategic": true,
	}
	return validDims[dim]
}

// isValidOutputFormat validates output format parameter.
func isValidOutputFormat(format string) bool {
	return format == "stdout" || format == "json" || format == "csv"
}

// determineRiskLevel determines risk level based on patent number (simulation).
func determineRiskLevel(patentNumber string) string {
	if strings.Contains(patentNumber, "EP") {
		return "HIGH"
	}
	if strings.Contains(patentNumber, "US") {
		return "LOW"
	}
	return "MEDIUM"
}

// formatAssessOutput formats the valuation result according to the specified format.
// Supports table (stdout), JSON, and CSV output formats.
func formatAssessOutput(result *ValuationResult, format string) (string, error) {
	switch format {
	case "json":
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return "", fmt.Errorf("JSON marshal error: %w", err)
		}
		return string(data), nil

	case "csv":
		var sb strings.Builder
		w := csv.NewWriter(&sb)
		// Write header
		if err := w.Write([]string{"Patent Number", "Technical", "Legal", "Commercial", "Strategic", "Overall", "Risk Level"}); err != nil {
			return "", err
		}
		// Write data rows
		for _, pv := range result.Patents {
			record := []string{
				pv.PatentNumber,
				fmt.Sprintf("%.2f", pv.TechnicalScore),
				fmt.Sprintf("%.2f", pv.LegalScore),
				fmt.Sprintf("%.2f", pv.CommercialScore),
				fmt.Sprintf("%.2f", pv.StrategicScore),
				fmt.Sprintf("%.2f", pv.OverallScore),
				pv.RiskLevel,
			}
			if err := w.Write(record); err != nil {
				return "", err
			}
		}
		w.Flush()
		return sb.String(), nil

	case "stdout":
		var sb strings.Builder
		sb.WriteString("Patent Valuation Assessment Results\n")
		sb.WriteString("====================================\n\n")
		for _, pv := range result.Patents {
			fmt.Fprintf(&sb, "Patent: %s\n", pv.PatentNumber)
			if pv.TechnicalScore > 0 {
				fmt.Fprintf(&sb, "  Technical:  %.2f\n", pv.TechnicalScore)
			}
			if pv.LegalScore > 0 {
				fmt.Fprintf(&sb, "  Legal:      %.2f\n", pv.LegalScore)
			}
			if pv.CommercialScore > 0 {
				fmt.Fprintf(&sb, "  Commercial: %.2f\n", pv.CommercialScore)
			}
			if pv.StrategicScore > 0 {
				fmt.Fprintf(&sb, "  Strategic:  %.2f\n", pv.StrategicScore)
			}
			fmt.Fprintf(&sb, "  Overall:    %.2f\n", pv.OverallScore)
			fmt.Fprintf(&sb, "  Risk Level: %s\n\n", pv.RiskLevel)
		}
		if result.PortfolioID != "" {
			fmt.Fprintf(&sb, "Portfolio ID: %s\n", result.PortfolioID)
		}
		if result.AverageScore > 0 {
			fmt.Fprintf(&sb, "Portfolio Average Score: %.2f\n", result.AverageScore)
		}
		if len(result.Recommendations) > 0 {
			sb.WriteString("\nRecommendations:\n")
			for i, rec := range result.Recommendations {
				fmt.Fprintf(&sb, "  %d. %s\n", i+1, rec)
			}
		}
		return sb.String(), nil

	default:
		return "", fmt.Errorf("unknown format: %s", format)
	}
}

// writeOutput handles standard output and file writing.
func writeOutput(content string, filePath string) error {
	if filePath == "" {
		fmt.Print(content)
		return nil
	}
	return os.WriteFile(filePath, []byte(content), 0644)
}

//Personal.AI order the ending

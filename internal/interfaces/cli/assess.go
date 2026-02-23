package cli

import (
"encoding/json"
"fmt"
"os"
"strings"

"github.com/spf13/cobra"
)

// NewAssessCmd creates the assess command
func NewAssessCmd() *cobra.Command {
cmd := &cobra.Command{
Use:   "assess",
Short: "Patent valuation assessment tools",
Long:  "Perform multi-dimensional patent valuation assessment including technical, legal, commercial and strategic dimensions",
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
// Parse patent numbers
patents := strings.Split(patentNumbers, ",")
for i := range patents {
patents[i] = strings.TrimSpace(patents[i])
if !isValidPatentNumber(patents[i]) {
return fmt.Errorf("invalid patent number format: %s (expected CN/US/EP/JP/KR prefix)", patents[i])
}
}

// Parse dimensions
dims := strings.Split(dimensions, ",")
for i := range dims {
dims[i] = strings.TrimSpace(dims[i])
if !isValidDimension(dims[i]) {
return fmt.Errorf("invalid dimension: %s (must be technical/legal/commercial/strategic)", dims[i])
}
}

// Validate output format
if !isValidOutputFormat(output) {
return fmt.Errorf("invalid output format: %s (must be stdout/json/csv)", output)
}

// Simulate valuation result
result := &ValuationResult{
Patents: []PatentValuation{
{
PatentNumber: patents[0],
TechnicalScore: 85,
LegalScore: 78,
CommercialScore: 92,
StrategicScore: 88,
OverallScore: 85.75,
RiskLevel: "MEDIUM",
},
},
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

// Warn for high-risk patents
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
cmd.MarkFlagRequired("patent-number")

return cmd
}

func newAssessPortfolioCmd() *cobra.Command {
var (
portfolioID           string
includeRecommendations bool
output                string
file                  string
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

// Simulate portfolio assessment result
result := &ValuationResult{
PortfolioID: portfolioID,
Patents: []PatentValuation{
{PatentNumber: "CN202110123456", OverallScore: 85.75, RiskLevel: "MEDIUM"},
{PatentNumber: "US11234567B2", OverallScore: 92.5, RiskLevel: "LOW"},
},
AverageScore: 89.125,
Recommendations: []string{
"Consider filing continuation applications for high-value patents",
"Monitor competitor activities in blue OLED materials",
},
}

if !includeRecommendations {
result.Recommendations = nil
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
cmd.MarkFlagRequired("portfolio-id")

return cmd
}

// Helper types
type ValuationResult struct {
PortfolioID     string              `json:"portfolio_id,omitempty"`
Patents         []PatentValuation   `json:"patents"`
AverageScore    float64             `json:"average_score,omitempty"`
Recommendations []string            `json:"recommendations,omitempty"`
}

type PatentValuation struct {
PatentNumber    string  `json:"patent_number"`
TechnicalScore  float64 `json:"technical_score,omitempty"`
LegalScore      float64 `json:"legal_score,omitempty"`
CommercialScore float64 `json:"commercial_score,omitempty"`
StrategicScore  float64 `json:"strategic_score,omitempty"`
OverallScore    float64 `json:"overall_score"`
RiskLevel       string  `json:"risk_level"`
}

// Helper functions
func isValidPatentNumber(num string) bool {
prefixes := []string{"CN", "US", "EP", "JP", "KR"}
for _, prefix := range prefixes {
if strings.HasPrefix(num, prefix) {
return true
}
}
return false
}

func isValidDimension(dim string) bool {
validDims := map[string]bool{
"technical": true, "legal": true, "commercial": true, "strategic": true,
}
return validDims[dim]
}

func isValidOutputFormat(format string) bool {
return format == "stdout" || format == "json" || format == "csv"
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
var sb strings.Builder
sb.WriteString("Patent Number,Technical,Legal,Commercial,Strategic,Overall,Risk Level\n")
for _, pv := range result.Patents {
fmt.Fprintf(&sb, "%s,%.2f,%.2f,%.2f,%.2f,%.2f,%s\n",
pv.PatentNumber, pv.TechnicalScore, pv.LegalScore,
pv.CommercialScore, pv.StrategicScore, pv.OverallScore, pv.RiskLevel)
}
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

func writeOutput(content string, filePath string) error {
if filePath == "" {
fmt.Print(content)
return nil
}

return os.WriteFile(filePath, []byte(content), 0644)
}

//Personal.AI order the ending

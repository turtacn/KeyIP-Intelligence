package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// NewSearchCmd returns the keyip search top-level subcommand.
// It provides molecule similarity search and patent text search capabilities.
func NewSearchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "search",
		Short: "Molecule and patent search",
		Long: `Search for molecules by structure similarity or patents by text/classification.

Provides tools for researchers and IP analysts:
  - Molecule similarity search using SMILES/InChI input
  - Patent text search with IPC/CPC classification filtering
  - Configurable fingerprint types and similarity thresholds`,
	}

	cmd.AddCommand(newSearchMoleculeCmd())
	cmd.AddCommand(newSearchPatentCmd())

	return cmd
}

func newSearchMoleculeCmd() *cobra.Command {
	var (
		smiles       string
		inchi        string
		threshold    float64
		fingerprints string
		maxResults   int
		offices      string
		includeRisk  bool
		output       string
	)

	cmd := &cobra.Command{
		Use:   "molecule",
		Short: "Search by molecular structure similarity",
		Long:  "Search patents by molecular structure similarity using SMILES or InChI notation",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Mutual exclusion: smiles/inchi must provide exactly one
			if smiles == "" && inchi == "" {
				return fmt.Errorf("either --smiles or --inchi must be provided")
			}
			if smiles != "" && inchi != "" {
				return fmt.Errorf("--smiles and --inchi are mutually exclusive")
			}

			// Validate threshold range: 0.0-1.0
			if threshold < 0.0 || threshold > 1.0 {
				return fmt.Errorf("threshold must be between 0.0 and 1.0, got %.2f", threshold)
			}

			// Validate max-results range: 1-500
			if maxResults < 1 || maxResults > 500 {
				return fmt.Errorf("max-results must be between 1 and 500, got %d", maxResults)
			}

			// Validate fingerprints
			fps := strings.Split(fingerprints, ",")
			for _, fp := range fps {
				fp = strings.TrimSpace(fp)
				if fp != "" && !isValidFingerprint(fp) {
					return fmt.Errorf("invalid fingerprint type: %s (must be morgan/topological/maccs/gnn)", fp)
				}
			}

			// Build SimilaritySearchRequest and call SimilaritySearchService.Search()
			// In production, this would integrate with application/patent_mining.SimilaritySearchService
			inputStructure := smiles
			if inputStructure == "" {
				inputStructure = inchi
			}

			results := []MoleculeSearchResult{
				{Rank: 1, Similarity: 0.95, PatentNumber: "CN202110123456", MoleculeName: "OLED-Blue-Emitter-1", SMILES: "c1ccc2c(c1)ccc3ccccc32", RiskLevel: "LOW"},
				{Rank: 2, Similarity: 0.87, PatentNumber: "US11234567B2", MoleculeName: "OLED-Blue-Emitter-2", SMILES: "c1ccc2nc3ccccc3cc2c1", RiskLevel: "MEDIUM"},
				{Rank: 3, Similarity: 0.82, PatentNumber: "EP3456789A1", MoleculeName: "OLED-Host-Material", SMILES: "c1ccc(cc1)c2ccc3ccccc3c2", RiskLevel: "HIGH"},
			}

			// Filter by threshold
			filtered := make([]MoleculeSearchResult, 0)
			for _, r := range results {
				if r.Similarity >= threshold {
					filtered = append(filtered, r)
				}
			}
			results = filtered

			// Limit results
			if len(results) > maxResults {
				results = results[:maxResults]
			}

			// Format output
			content, err := formatMoleculeResults(results, output, includeRisk)
			if err != nil {
				return fmt.Errorf("failed to format output: %w", err)
			}

			fmt.Print(content)
			return nil
		},
	}

	cmd.Flags().StringVar(&smiles, "smiles", "", "Molecule SMILES string (mutually exclusive with --inchi)")
	cmd.Flags().StringVar(&inchi, "inchi", "", "Molecule InChI string (mutually exclusive with --smiles)")
	cmd.Flags().Float64Var(&threshold, "threshold", 0.65, "Similarity threshold (0.0-1.0)")
	cmd.Flags().StringVar(&fingerprints, "fingerprints", "morgan,gnn", "Fingerprint types (comma-separated)")
	cmd.Flags().IntVar(&maxResults, "max-results", 20, "Maximum number of results (1-500)")
	cmd.Flags().StringVar(&offices, "offices", "", "Patent office filter (comma-separated)")
	cmd.Flags().BoolVar(&includeRisk, "include-risk", false, "Include infringement risk assessment")
	cmd.Flags().StringVar(&output, "output", "stdout", "Output format (stdout/json)")

	return cmd
}

func newSearchPatentCmd() *cobra.Command {
	var (
		query      string
		ipc        string
		cpc        string
		dateFrom   string
		dateTo     string
		offices    string
		maxResults int
		sortBy     string
		output     string
	)

	cmd := &cobra.Command{
		Use:   "patent",
		Short: "Search patents by text/classification",
		Long:  "Search patents by keywords, IPC/CPC classification, and date range",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Validate max-results range: 1-500
			if maxResults < 1 || maxResults > 500 {
				return fmt.Errorf("max-results must be between 1 and 500, got %d", maxResults)
			}

			// Validate sort parameter
			if !isValidSortOption(sortBy) {
				return fmt.Errorf("invalid sort option: %s (must be relevance/date/citations)", sortBy)
			}

			// Validate date format and range
			if dateFrom != "" {
				if _, err := time.Parse("2006-01-02", dateFrom); err != nil {
					return fmt.Errorf("invalid date-from format: %s (must be YYYY-MM-DD)", dateFrom)
				}
			}
			if dateTo != "" {
				if _, err := time.Parse("2006-01-02", dateTo); err != nil {
					return fmt.Errorf("invalid date-to format: %s (must be YYYY-MM-DD)", dateTo)
				}
			}
			if dateFrom != "" && dateTo != "" {
				from, _ := time.Parse("2006-01-02", dateFrom)
				to, _ := time.Parse("2006-01-02", dateTo)
				if from.After(to) {
					return fmt.Errorf("date-from cannot be later than date-to")
				}
			}

			// Build PatentSearchRequest and call SimilaritySearchService.SearchByText()
			// In production, this would integrate with application/patent_mining.SimilaritySearchService
			results := []PatentSearchResult{
				{Rank: 1, Relevance: 0.95, PatentNumber: "CN202110123456", Title: "Blue OLED Emitter with High Efficiency", ApplicationDate: "2021-01-15", IPC: "H10K50/11"},
				{Rank: 2, Relevance: 0.88, PatentNumber: "US11234567B2", Title: "Organic Light Emitting Material", ApplicationDate: "2020-06-20", IPC: "H10K50/12"},
				{Rank: 3, Relevance: 0.82, PatentNumber: "EP3456789A1", Title: "OLED Host Material Composition", ApplicationDate: "2019-03-10", IPC: "H10K85/60"},
			}

			// Filter by IPC if specified
			if ipc != "" {
				filtered := make([]PatentSearchResult, 0)
				for _, r := range results {
					if strings.HasPrefix(r.IPC, ipc) {
						filtered = append(filtered, r)
					}
				}
				results = filtered
			}

			// Limit results
			if len(results) > maxResults {
				results = results[:maxResults]
			}

			// Format output
			content, err := formatPatentResults(results, output)
			if err != nil {
				return fmt.Errorf("failed to format output: %w", err)
			}

			fmt.Print(content)
			return nil
		},
	}

	cmd.Flags().StringVar(&query, "query", "", "Keyword query string [REQUIRED]")
	cmd.Flags().StringVar(&ipc, "ipc", "", "IPC classification filter")
	cmd.Flags().StringVar(&cpc, "cpc", "", "CPC classification filter")
	cmd.Flags().StringVar(&dateFrom, "date-from", "", "Application date start (YYYY-MM-DD)")
	cmd.Flags().StringVar(&dateTo, "date-to", "", "Application date end (YYYY-MM-DD)")
	cmd.Flags().StringVar(&offices, "offices", "", "Patent office filter (comma-separated)")
	cmd.Flags().IntVar(&maxResults, "max-results", 50, "Maximum number of results (1-500)")
	cmd.Flags().StringVar(&sortBy, "sort", "relevance", "Sort by (relevance/date/citations)")
	cmd.Flags().StringVar(&output, "output", "stdout", "Output format (stdout/json)")
	_ = cmd.MarkFlagRequired("query")

	return cmd
}

// MoleculeSearchResult represents a molecule similarity search result.
type MoleculeSearchResult struct {
	Rank         int     `json:"rank"`
	Similarity   float64 `json:"similarity"`
	PatentNumber string  `json:"patent_number"`
	MoleculeName string  `json:"molecule_name"`
	SMILES       string  `json:"smiles"`
	RiskLevel    string  `json:"risk_level,omitempty"`
}

// PatentSearchResult represents a patent text search result.
type PatentSearchResult struct {
	Rank            int     `json:"rank"`
	Relevance       float64 `json:"relevance"`
	PatentNumber    string  `json:"patent_number"`
	Title           string  `json:"title"`
	ApplicationDate string  `json:"application_date"`
	IPC             string  `json:"ipc"`
}

// isValidFingerprint validates fingerprint type.
func isValidFingerprint(fp string) bool {
	valid := map[string]bool{"morgan": true, "topological": true, "maccs": true, "gnn": true}
	return valid[fp]
}

// isValidSortOption validates sort option.
func isValidSortOption(s string) bool {
	valid := map[string]bool{"relevance": true, "date": true, "citations": true}
	return valid[s]
}

// colorizeRiskLevel returns ANSI-colored risk level.
// HIGH=red, MEDIUM=yellow, LOW=green
func colorizeRiskLevel(level string) string {
	switch level {
	case "HIGH":
		return "\033[31m" + level + "\033[0m"
	case "MEDIUM":
		return "\033[33m" + level + "\033[0m"
	case "LOW":
		return "\033[32m" + level + "\033[0m"
	default:
		return level
	}
}

// formatMoleculeResults formats molecule search results.
func formatMoleculeResults(results []MoleculeSearchResult, format string, includeRisk bool) (string, error) {
	if format == "json" {
		data, err := json.MarshalIndent(results, "", "  ")
		if err != nil {
			return "", fmt.Errorf("JSON marshal error: %w", err)
		}
		return string(data) + "\n", nil
	}

	// Table format (stdout)
	var sb strings.Builder
	sb.WriteString("Molecule Similarity Search Results\n")
	sb.WriteString("══════════════════════════════════════════════════════════════════════════════\n\n")

	if len(results) == 0 {
		sb.WriteString("  No molecules found matching the specified criteria.\n")
		return sb.String(), nil
	}

	if includeRisk {
		sb.WriteString(fmt.Sprintf("%-5s %-10s %-18s %-25s %-30s %s\n", "Rank", "Similarity", "Patent", "Molecule", "SMILES", "Risk"))
		sb.WriteString("──────────────────────────────────────────────────────────────────────────────\n")
		for _, r := range results {
			fmt.Fprintf(&sb, "%-5d %-10.4f %-18s %-25s %-30s %s\n",
				r.Rank, r.Similarity, r.PatentNumber, r.MoleculeName, truncateString(r.SMILES, 30), colorizeRiskLevel(r.RiskLevel))
		}
	} else {
		sb.WriteString(fmt.Sprintf("%-5s %-10s %-18s %-25s %s\n", "Rank", "Similarity", "Patent", "Molecule", "SMILES"))
		sb.WriteString("──────────────────────────────────────────────────────────────────────────────\n")
		for _, r := range results {
			fmt.Fprintf(&sb, "%-5d %-10.4f %-18s %-25s %s\n",
				r.Rank, r.Similarity, r.PatentNumber, r.MoleculeName, truncateString(r.SMILES, 40))
		}
	}

	sb.WriteString("\n══════════════════════════════════════════════════════════════════════════════\n")
	fmt.Fprintf(&sb, "Total: %d result(s)\n", len(results))

	return sb.String(), nil
}

// formatPatentResults formats patent search results.
func formatPatentResults(results []PatentSearchResult, format string) (string, error) {
	if format == "json" {
		data, err := json.MarshalIndent(results, "", "  ")
		if err != nil {
			return "", fmt.Errorf("JSON marshal error: %w", err)
		}
		return string(data) + "\n", nil
	}

	// Table format (stdout)
	var sb strings.Builder
	sb.WriteString("Patent Search Results\n")
	sb.WriteString("══════════════════════════════════════════════════════════════════════════════════════\n\n")

	if len(results) == 0 {
		sb.WriteString("  No patents found matching the specified criteria.\n")
		return sb.String(), nil
	}

	sb.WriteString(fmt.Sprintf("%-5s %-10s %-18s %-40s %-12s %s\n", "Rank", "Relevance", "Patent", "Title", "Date", "IPC"))
	sb.WriteString("────────────────────────────────────────────────────────────────────────────────────────\n")

	for _, r := range results {
		fmt.Fprintf(&sb, "%-5d %-10.4f %-18s %-40s %-12s %s\n",
			r.Rank, r.Relevance, r.PatentNumber, truncateString(r.Title, 40), r.ApplicationDate, r.IPC)
	}

	sb.WriteString("\n══════════════════════════════════════════════════════════════════════════════════════\n")
	fmt.Fprintf(&sb, "Total: %d result(s)\n", len(results))

	return sb.String(), nil
}

// truncateString truncates a string to maxLen characters.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// SearchCmd is exported for backward compatibility.
var SearchCmd = NewSearchCmd()

//Personal.AI order the ending

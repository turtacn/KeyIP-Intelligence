package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"

	"github.com/turtacn/KeyIP-Intelligence/internal/application/patent_mining"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

var (
	searchSMILES        string
	searchInChI         string
	searchThreshold     float64
	searchFingerprints  string
	searchMaxResults    int
	searchOffices       string
	searchIncludeRisk   bool
	searchOutput        string
	searchQuery         string
	searchIPC           string
	searchCPC           string
	searchDateFrom      string
	searchDateTo        string
	searchSort          string
)

// NewSearchCmd creates the search command
func NewSearchCmd(
	similaritySearchService patent_mining.SimilaritySearchService,
	logger logging.Logger,
) *cobra.Command {
	searchCmd := &cobra.Command{
		Use:   "search",
		Short: "Search patents by molecule similarity or text query",
		Long:  `Perform similarity search using molecular structures (SMILES/InChI) or text-based patent search`,
	}

	// Subcommand: search molecule
	moleculeCmd := &cobra.Command{
		Use:   "molecule",
		Short: "Search patents by molecular similarity",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSearchMolecule(cmd.Context(), similaritySearchService, logger)
		},
	}

	moleculeCmd.Flags().StringVar(&searchSMILES, "smiles", "", "Molecule SMILES string (mutually exclusive with --inchi)")
	moleculeCmd.Flags().StringVar(&searchInChI, "inchi", "", "Molecule InChI string (mutually exclusive with --smiles)")
	moleculeCmd.Flags().Float64Var(&searchThreshold, "threshold", 0.65, "Similarity threshold (0.0-1.0)")
	moleculeCmd.Flags().StringVar(&searchFingerprints, "fingerprints", "morgan,gnn", "Fingerprint types: morgan,topological,maccs,gnn")
	moleculeCmd.Flags().IntVar(&searchMaxResults, "max-results", 20, "Maximum number of results (1-500)")
	moleculeCmd.Flags().StringVar(&searchOffices, "offices", "", "Patent office filter (e.g., CN,US,EP)")
	moleculeCmd.Flags().BoolVar(&searchIncludeRisk, "include-risk", false, "Include infringement risk assessment")
	moleculeCmd.Flags().StringVar(&searchOutput, "output", "stdout", "Output format: stdout|json")

	// Subcommand: search patent
	patentCmd := &cobra.Command{
		Use:   "patent",
		Short: "Search patents by text query",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSearchPatent(cmd.Context(), similaritySearchService, logger)
		},
	}

	patentCmd.Flags().StringVar(&searchQuery, "query", "", "Keyword query (required)")
	patentCmd.Flags().StringVar(&searchIPC, "ipc", "", "IPC classification filter")
	patentCmd.Flags().StringVar(&searchCPC, "cpc", "", "CPC classification filter")
	patentCmd.Flags().StringVar(&searchDateFrom, "date-from", "", "Application date from (YYYY-MM-DD)")
	patentCmd.Flags().StringVar(&searchDateTo, "date-to", "", "Application date to (YYYY-MM-DD)")
	patentCmd.Flags().StringVar(&searchOffices, "offices", "", "Patent office filter (e.g., CN,US,EP)")
	patentCmd.Flags().IntVar(&searchMaxResults, "max-results", 50, "Maximum number of results (1-500)")
	patentCmd.Flags().StringVar(&searchSort, "sort", "relevance", "Sort by: relevance|date|citations")
	patentCmd.Flags().StringVar(&searchOutput, "output", "stdout", "Output format: stdout|json")
	patentCmd.MarkFlagRequired("query")

	searchCmd.AddCommand(moleculeCmd, patentCmd)
	return searchCmd
}

func runSearchMolecule(ctx context.Context, service patent_mining.SimilaritySearchService, logger logging.Logger) error {
	// Validate mutually exclusive flags
	if searchSMILES == "" && searchInChI == "" {
		return errors.NewMsg("either --smiles or --inchi must be provided")
	}
	if searchSMILES != "" && searchInChI != "" {
		return errors.NewMsg("--smiles and --inchi are mutually exclusive, provide only one")
	}

	// Validate threshold range
	if searchThreshold < 0.0 || searchThreshold > 1.0 {
		return errors.Errorf("threshold must be between 0.0 and 1.0, got %.2f", searchThreshold)
	}

	// Validate max results range
	if searchMaxResults < 1 || searchMaxResults > 500 {
		return errors.Errorf("max-results must be between 1 and 500, got %d", searchMaxResults)
	}

	// Parse and validate fingerprints
	fingerprints, err := parseFingerprints(searchFingerprints)
	if err != nil {
		return err
	}

	// Parse offices
	_ = parseOffices(searchOffices)

	logger.Info("Starting molecule similarity search",
		logging.String("smiles", searchSMILES),
		logging.String("inchi", searchInChI),
		logging.Float64("threshold", searchThreshold),
		logging.Int("max_results", searchMaxResults),
		logging.Bool("include_risk", searchIncludeRisk))

	// Build search query for the service
	query := &patent_mining.SimilarityQuery{
		SMILES:          searchSMILES,
		InChI:           searchInChI,
		Threshold:       searchThreshold,
		FingerprintType: strings.Join(fingerprints, ","),
		MaxResults:      searchMaxResults,
	}

	// Execute search
	results, err := service.Search(ctx, query)
	if err != nil {
		logger.Error("Molecule search failed", logging.String("error", err.Error()))
		return errors.WrapMsg(err, "molecule similarity search failed")
	}

	// Check empty results
	if len(results) == 0 {
		fmt.Println("\n💡 No similar molecules found.")
		fmt.Printf("Try lowering the similarity threshold (current: %.2f)\n", searchThreshold)
		return nil
	}

	// Format output
	output, err := formatSimilarityResults(results, searchOutput)
	if err != nil {
		return errors.WrapMsg(err, "failed to format results")
	}

	fmt.Print(output)

	logger.Info("Molecule search completed",
		logging.Int("results_count", len(results)))

	return nil
}

func runSearchPatent(ctx context.Context, service patent_mining.SimilaritySearchService, logger logging.Logger) error {
	// Validate max results range
	if searchMaxResults < 1 || searchMaxResults > 500 {
		return errors.Errorf("max-results must be between 1 and 500, got %d", searchMaxResults)
	}

	// Validate sort parameter
	validSorts := []string{"relevance", "date", "citations"}
	if !contains(validSorts, strings.ToLower(searchSort)) {
		return errors.Errorf("invalid sort parameter: %s (must be relevance|date|citations)", searchSort)
	}

	// Validate and parse date range
	var dateFrom, dateTo *time.Time
	if searchDateFrom != "" {
		df, err := time.Parse("2006-01-02", searchDateFrom)
		if err != nil {
			return errors.Errorf("invalid date-from format: %s (must be YYYY-MM-DD)", searchDateFrom)
		}
		dateFrom = &df
	}
	if searchDateTo != "" {
		dt, err := time.Parse("2006-01-02", searchDateTo)
		if err != nil {
			return errors.Errorf("invalid date-to format: %s (must be YYYY-MM-DD)", searchDateTo)
		}
		dateTo = &dt
	}

	// Validate date range logic
	if dateFrom != nil && dateTo != nil && dateFrom.After(*dateTo) {
		return errors.NewMsg("date-from cannot be later than date-to")
	}

	// Parse offices
	offices := parseOffices(searchOffices)

	logger.Info("Starting patent text search",
		logging.String("query", searchQuery),
		logging.String("ipc", searchIPC),
		logging.String("cpc", searchCPC),
		logging.String("date_range", fmt.Sprintf("%v to %v", dateFrom, dateTo)),
		logging.Int("max_results", searchMaxResults),
		logging.String("sort", searchSort))

	// Build search request
	req := &patent_mining.PatentTextSearchRequest{
		Query:      searchQuery,
		IPC:        searchIPC,
		CPC:        searchCPC,
		DateFrom:   dateFrom,
		DateTo:     dateTo,
		Offices:    offices,
		MaxResults: searchMaxResults,
		Sort:       strings.ToLower(searchSort),
		Context:    ctx,
	}

	// Execute search
	results, err := service.SearchByText(ctx, req)
	if err != nil {
		logger.Error("Patent search failed", logging.String("error", err.Error()))
		return errors.WrapMsg(err, "patent text search failed")
	}

	// Check empty results
	if len(results) == 0 {
		fmt.Println("\n💡 No matching patents found.")
		fmt.Println("Try broadening your search query or adjusting filters.")
		return nil
	}

	// Format output
	output, err := formatCLIPatentResults(results, searchOutput)
	if err != nil {
		return errors.WrapMsg(err, "failed to format results")
	}

	fmt.Print(output)

	logger.Info("Patent search completed",
		logging.Int("results_count", len(results)))

	return nil
}

func parseFingerprints(input string) ([]string, error) {
	validFingerprints := map[string]bool{
		"morgan":      true,
		"topological": true,
		"maccs":       true,
		"gnn":         true,
	}

	parts := strings.Split(input, ",")
	result := make([]string, 0, len(parts))

	for _, part := range parts {
		trimmed := strings.ToLower(strings.TrimSpace(part))
		if trimmed == "" {
			continue
		}
		if !validFingerprints[trimmed] {
			return nil, errors.Errorf("invalid fingerprint type: %s (must be morgan|topological|maccs|gnn)", trimmed)
		}
		result = append(result, trimmed)
	}

	if len(result) == 0 {
		return nil, errors.NewMsg("at least one valid fingerprint type required")
	}

	return result, nil
}

func parseOffices(input string) []string {
	if input == "" {
		return nil
	}

	parts := strings.Split(input, ",")
	result := make([]string, 0, len(parts))

	for _, part := range parts {
		trimmed := strings.ToUpper(strings.TrimSpace(part))
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}

	return result
}

func formatSimilarityResults(results []patent_mining.SimilarityResult, format string) (string, error) {
	if format == "json" {
		data, err := json.MarshalIndent(results, "", "  ")
		if err != nil {
			return "", err
		}
		return string(data) + "\n", nil
	}

	// Table format
	var buf strings.Builder
	buf.WriteString("\n=== Molecule Similarity Search Results ===\n\n")

	table := tablewriter.NewWriter(&buf)
	table.Header([]string{"Rank", "Similarity", "ID", "Name", "SMILES", "Method"})

	for i, result := range results {
		moleculeName := ""
		moleculeSMILES := ""
		moleculeID := ""
		if result.Molecule != nil {
			moleculeName = result.Molecule.Name
			moleculeSMILES = result.Molecule.SMILES
			moleculeID = result.Molecule.ID
		}
		row := []string{
			fmt.Sprintf("%d", i+1),
			fmt.Sprintf("%.2f%%", result.Similarity*100),
			truncateString(moleculeID, 20),
			truncateString(moleculeName, 30),
			truncateString(moleculeSMILES, 40),
			result.Method,
		}
		table.Append(row)
	}

	table.Render()

	buf.WriteString(fmt.Sprintf("\nTotal results: %d\n", len(results)))
	buf.WriteString(fmt.Sprintf("Threshold: %.2f\n", searchThreshold))

	return buf.String(), nil
}

func formatCLIPatentResults(results []*patent_mining.CLIPatentSearchResult, format string) (string, error) {
	if format == "json" {
		data, err := json.MarshalIndent(results, "", "  ")
		if err != nil {
			return "", err
		}
		return string(data) + "\n", nil
	}

	// Table format
	var buf strings.Builder
	buf.WriteString("\n=== Patent Text Search Results ===\n\n")

	table := tablewriter.NewWriter(&buf)
	table.Header([]string{"Rank", "Relevance", "Patent", "Title", "Filing Date", "IPC"})

	for i, result := range results {
		relevanceStr := fmt.Sprintf("%.2f%%", result.Relevance*100)
		if result.Relevance >= 0.8 {
			relevanceStr = color.GreenString(relevanceStr)
		} else if result.Relevance >= 0.5 {
			relevanceStr = color.YellowString(relevanceStr)
		}

		table.Append([]string{
			fmt.Sprintf("%d", i+1),
			relevanceStr,
			result.PatentNumber,
			truncateString(result.Title, 50),
			result.FilingDate.Format("2006-01-02"),
			truncateString(result.IPC, 20),
		})
	}

	table.Render()

	buf.WriteString(fmt.Sprintf("\nTotal results: %d\n", len(results)))

	return buf.String(), nil
}

func colorizeRiskLevel(level string) string {
	switch strings.ToUpper(level) {
	case "HIGH":
		return color.RedString("HIGH")
	case "MEDIUM":
		return color.YellowString("MEDIUM")
	case "LOW":
		return color.GreenString("LOW")
	default:
		return level
	}
}

//Personal.AI order the ending

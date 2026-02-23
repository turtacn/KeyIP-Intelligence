package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

var SearchCmd = &cobra.Command{
	Use:   "search",
	Short: "Search patents",
}

var searchPatentsCmd = &cobra.Command{
	Use:   "patents",
	Short: "Search patents",
	RunE:  runSearchPatents,
}

var searchQuery string

func init() {
	SearchCmd.AddCommand(searchPatentsCmd)
	searchPatentsCmd.Flags().StringVarP(&searchQuery, "query", "q", "", "Query")
	RootCmd.AddCommand(SearchCmd)
}

func runSearchPatents(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if searchQuery == "" {
		return fmt.Errorf("query required")
	}
	fmt.Printf("Searching: %s\n", searchQuery)
	_ = ctx
	return nil
}

//Personal.AI order the ending

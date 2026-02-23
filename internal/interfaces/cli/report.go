package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

var ReportCmd = &cobra.Command{
	Use:   "report",
	Short: "Generate reports",
}

var ftoCmd = &cobra.Command{
	Use:   "fto",
	Short: "FTO report",
	RunE:  runFTO,
}

var reportOutput string

func init() {
	ReportCmd.AddCommand(ftoCmd)
	ftoCmd.Flags().StringVar(&reportOutput, "output", "report.pdf", "Output path")
	RootCmd.AddCommand(ReportCmd)
}

func runFTO(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	fmt.Printf("Generating FTO report: %s\n", reportOutput)
	_ = ctx
	return nil
}

//Personal.AI order the ending

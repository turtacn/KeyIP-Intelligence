package cli

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

var (
	reportType       string
	reportOutput     string
	reportFormat     string
	reportInclude    string
	reportDateFrom   string
	reportDateTo     string
	reportRecipients string
)

// NewReportCmd creates the report command
func NewReportCmd() *cobra.Command {
	reportCmd := &cobra.Command{
		Use:   "report",
		Short: "Generate intelligence reports",
		Long:  `Generate and export various types of reports (FTO, landscape, infringement, valuation)`,
	}

	// Subcommand: report generate
	generateCmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate a new report",
		RunE: func(cmd *cobra.Command, args []string) error {
			cliCtx, err := GetCLIContext(cmd)
			if err != nil {
				return err
			}
			return runReportGenerate(cmd.Context(), cliCtx, cliCtx.Logger)
		},
	}

	generateCmd.Flags().StringVar(&reportType, "type", "", "Report type: fto|landscape|infringement|valuation (required)")
	generateCmd.Flags().StringVar(&reportOutput, "output", "", "Output file path (optional)")
	generateCmd.Flags().StringVar(&reportFormat, "format", "pdf", "Output format: pdf|docx|html")
	generateCmd.Flags().StringVar(&reportInclude, "include", "", "Comma-separated list of sections to include")
	generateCmd.MarkFlagRequired("type")

	// Subcommand: report list
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List generated reports",
		RunE: func(cmd *cobra.Command, args []string) error {
			cliCtx, err := GetCLIContext(cmd)
			if err != nil {
				return err
			}
			return runReportList(cmd.Context(), cliCtx, cliCtx.Logger)
		},
	}

	listCmd.Flags().StringVar(&reportDateFrom, "from", "", "Date from (YYYY-MM-DD)")
	listCmd.Flags().StringVar(&reportDateTo, "to", "", "Date to (YYYY-MM-DD)")

	// Subcommand: report email
	emailCmd := &cobra.Command{
		Use:   "email",
		Short: "Email a report",
		RunE: func(cmd *cobra.Command, args []string) error {
			cliCtx, err := GetCLIContext(cmd)
			if err != nil {
				return err
			}
			return runReportEmail(cmd.Context(), cliCtx, cliCtx.Logger)
		},
	}

	emailCmd.Flags().StringVar(&reportRecipients, "to", "", "Comma-separated email recipients (required)")
	emailCmd.MarkFlagRequired("to")

	reportCmd.AddCommand(generateCmd, listCmd, emailCmd)
	return reportCmd
}

func runReportGenerate(ctx context.Context, cliCtx *CLIContext, logger logging.Logger) error {
	return errors.NewMsg("Command 'report generate' not implemented yet")
}

func runReportList(ctx context.Context, cliCtx *CLIContext, logger logging.Logger) error {
	return errors.NewMsg("Command 'report list' not implemented yet")
}

func runReportEmail(ctx context.Context, cliCtx *CLIContext, logger logging.Logger) error {
	return errors.NewMsg("Command 'report email' not implemented yet")
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

//Personal.AI order the ending

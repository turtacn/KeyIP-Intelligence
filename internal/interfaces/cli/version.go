package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// NewVersionCmd creates the version command.
func NewVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Long:  `Print the version, git commit hash, and build date of the keyip CLI.`,
		Example: `  # Print version information
  keyip version`,
		Args:              cobra.NoArgs,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error { return nil },
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintf(cmd.OutOrStdout(), "KeyIP-Intelligence CLI\n")
			fmt.Fprintf(cmd.OutOrStdout(), "Version:    %s\n", Version)
			fmt.Fprintf(cmd.OutOrStdout(), "Git Commit: %s\n", GitCommit)
			fmt.Fprintf(cmd.OutOrStdout(), "Build Date: %s\n", BuildDate)
			return nil
		},
	}
}

//Personal.AI order the ending

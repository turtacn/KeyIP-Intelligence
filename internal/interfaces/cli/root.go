package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// Build information variables - injected via go build -ldflags
var (
	Version   = "0.1.0"
	BuildTime = "unknown"
	GitCommit = "unknown"
)

// Global flag variables
var (
	cfgFile string
	verbose bool
	noColor bool
)

// CLIDeps holds all dependencies required by CLI commands.
// This struct is populated during application bootstrap and passed to command constructors.
type CLIDeps struct {
	// NoColor disables ANSI color output when true.
	NoColor bool
}

// RootCmd is the root command for backward compatibility.
var RootCmd *cobra.Command

// NewRootCmd creates the root keyip command with all subcommands registered.
// deps can be nil for simple usage without full dependency injection.
func NewRootCmd(deps *CLIDeps) *cobra.Command {
	if deps == nil {
		deps = &CLIDeps{}
	}

	cmd := &cobra.Command{
		Use:     "keyip",
		Short:   "KeyIP-Intelligence CLI - AI-driven IP lifecycle management for OLED materials",
		Long: `KeyIP-Intelligence: AI-powered patent intelligence platform for OLED materials.

A comprehensive command-line tool for managing intellectual property throughout
its lifecycle, including patent valuation, deadline management, report generation,
and molecule/patent search capabilities.

Features:
  - Multi-dimensional patent valuation (technical, legal, commercial, strategic)
  - Lifecycle management (deadlines, annuities, legal status, reminders)
  - Report generation (FTO, infringement, portfolio, annual)
  - Molecule similarity search using SMILES/InChI
  - Patent text search with IPC/CPC filtering

Examples:
  keyip assess patent --patent-number CN202110123456
  keyip lifecycle deadlines --days-ahead 30
  keyip search molecule --smiles "c1ccccc1"
  keyip report generate --type fto --target CN123`,
		Version: Version,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// Load configuration file if specified
			if cfgFile != "" {
				// In production, this would load the config file
				// using internal/config/loader.go
				if _, err := os.Stat(cfgFile); os.IsNotExist(err) {
					return fmt.Errorf("config file not found: %s", cfgFile)
				}
			}

			// Set verbose logging level
			if verbose {
				// In production, this would configure the logger
				// to DEBUG level using internal/infrastructure/monitoring/logging/logger.go
			}

			// Configure color output
			deps.NoColor = noColor

			return nil
		},
	}

	// Register global persistent flags
	cmd.PersistentFlags().StringVar(&cfgFile, "config", "configs/config.yaml", "Config file path")
	cmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output (DEBUG level logging)")
	cmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "Disable ANSI color output")

	// Register subcommands
	cmd.AddCommand(NewAssessCmd())
	cmd.AddCommand(NewLifecycleCmd())
	cmd.AddCommand(NewReportCmd())
	cmd.AddCommand(NewSearchCmd())
	cmd.AddCommand(newVersionCmd())

	return cmd
}

// newVersionCmd creates the version subcommand that outputs build information.
func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Long:  "Print detailed version information including build time and git commit",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("KeyIP-Intelligence CLI\n")
			fmt.Printf("══════════════════════════════════════\n")
			fmt.Printf("  Version:    %s\n", Version)
			fmt.Printf("  Build Time: %s\n", BuildTime)
			fmt.Printf("  Git Commit: %s\n", GitCommit)
			fmt.Printf("══════════════════════════════════════\n")
		},
	}
}

// Execute creates the root command and executes it.
// This is the main entry point for the CLI application.
func Execute(deps *CLIDeps) error {
	RootCmd = NewRootCmd(deps)
	return RootCmd.Execute()
}

// init initializes the RootCmd for backward compatibility.
func init() {
	RootCmd = NewRootCmd(nil)
}

//Personal.AI order the ending

// Phase 12 - File #287: cmd/keyip/main.go
// CLI client entry point for KeyIP-Intelligence.
package main

import (
	"fmt"
	"os"

	"github.com/turtacn/KeyIP-Intelligence/internal/interfaces/cli"
)

// Build-time variables injected via ldflags.
var (
	version   = "dev"
	commit    = "unknown"
	buildDate = "unknown"
)

func init() {
	// Inject build-time variables into the cli package.
	cli.Version = version
	cli.GitCommit = commit
	cli.BuildDate = buildDate
}

func main() {
	// Execute the CLI using the existing cli.Execute() function which handles
	// root command creation, flag registration, and subcommand setup.
	if err := cli.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

//Personal.AI order the ending

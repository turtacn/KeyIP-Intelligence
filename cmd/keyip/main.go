package main

import (
	"fmt"
	"os"
	"runtime"

	"github.com/spf13/cobra"
	"github.com/turtacn/KeyIP-Intelligence/internal/interfaces/cli"
)

var (
	Version   = "dev"
	BuildTime = "unknown"
	GitCommit = "unknown"
	GoVersion = runtime.Version()
	Platform  = fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)
)

func main() {
	rootCmd := cli.NewRootCommand()

	// Inject version info
	rootCmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print the version number of KeyIP-Intelligence",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("KeyIP-Intelligence CLI\n")
			fmt.Printf("Version:    %s\n", Version)
			fmt.Printf("Build Time: %s\n", BuildTime)
			fmt.Printf("Git Commit: %s\n", GitCommit)
			fmt.Printf("Go Version: %s\n", GoVersion)
			fmt.Printf("Platform:   %s\n", Platform)
		},
	})

	// Add subcommands
	rootCmd.AddCommand(cli.NewSearchCmd())
	rootCmd.AddCommand(cli.NewAssessCmd())
	rootCmd.AddCommand(cli.NewLifecycleCmd())
	rootCmd.AddCommand(cli.NewReportCmd())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

//Personal.AI order the ending

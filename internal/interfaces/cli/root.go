package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	cfgFile string
	verbose bool
	debug   bool
)

var RootCmd = &cobra.Command{
	Use:     "keyip",
	Short:   "KeyIP-Intelligence CLI",
	Long:    "KeyIP-Intelligence: AI-powered patent intelligence platform for OLED materials",
	Version: "0.1.0",
}

// NewRootCommand creates the root command
func NewRootCommand() *cobra.Command {
	return RootCmd
}

func Execute() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	RootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file path")
	RootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	RootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "debug mode")
}

//Personal.AI order the ending

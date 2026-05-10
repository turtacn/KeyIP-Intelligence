package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/turtacn/KeyIP-Intelligence/internal/config"
)

// NewConfigCmd creates the config command and its subcommands.
func NewConfigCmd() *cobra.Command {
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Manage configuration",
		Long:  `Validate and manage the KeyIP-Intelligence configuration.`,
	}

	validateCmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate configuration file",
		Long: `Validate the syntax and structure of the KeyIP-Intelligence configuration file.

If a config file was specified via --config, that file is validated.
Otherwise, the default config paths are searched (./keyip.yaml,
~/.keyip/config.yaml, /etc/keyip/config.yaml).`,
		Example: `  # Validate the default config file
  keyip config validate

  # Validate a specific config file
  keyip -c /path/to/keyip.yaml config validate`,
		Args:              cobra.NoArgs,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error { return nil },
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConfigValidate(cmd)
		},
	}

	configCmd.AddCommand(validateCmd)
	return configCmd
}

func runConfigValidate(cmd *cobra.Command) error {
	// Get config path from root's --config persistent flag.
	configPath, _ := cmd.Root().PersistentFlags().GetString("config")

	if configPath != "" {
		// Explicit path: load and validate the file.
		_, err := config.LoadFromFile(configPath)
		if err != nil {
			return fmt.Errorf("configuration validation failed: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "OK: configuration at %s is valid\n", configPath)
		return nil
	}

	// Search default paths.
	searchPaths := []string{"./keyip.yaml"}
	if homeDir, err := os.UserHomeDir(); err == nil {
		searchPaths = append(searchPaths, homeDir+"/.keyip/config.yaml")
	}
	searchPaths = append(searchPaths, "/etc/keyip/config.yaml")

	for _, p := range searchPaths {
		if _, statErr := os.Stat(p); statErr == nil {
			_, loadErr := config.LoadFromFile(p)
			if loadErr != nil {
				return fmt.Errorf("configuration validation failed for %s: %w", p, loadErr)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "OK: configuration at %s is valid\n", p)
			return nil
		}
	}

	// No config file found.
	fmt.Fprintln(cmd.OutOrStdout(), "OK: no configuration file found")
	return nil
}

//Personal.AI order the ending

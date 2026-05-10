package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// NewCompletionCmd creates the shell completion command using cobra's built-in generators.
func NewCompletionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "completion [bash|zsh|fish|powershell]",
		Short: "Generate shell completion scripts",
		Long: `Generate shell completion scripts for keyip CLI.

To load completions:

Bash:
  $ source <(keyip completion bash)
  # To load for each session:
  $ keyip completion bash > /etc/bash_completion.d/keyip

Zsh:
  $ source <(keyip completion zsh)
  # To load for each session:
  $ keyip completion zsh > "${fpath[1]}/_keyip"

Fish:
  $ keyip completion fish | source

PowerShell:
  $ keyip completion powershell | Out-String | Invoke-Expression`,
		Example: `  # Generate bash completion
  keyip completion bash

  # Generate zsh completion
  keyip completion zsh

  # Generate fish completion with descriptions
  keyip completion fish

  # Generate PowerShell completion
  keyip completion powershell`,
		Args:              cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		ValidArgs:         []string{"bash", "zsh", "fish", "powershell"},
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error { return nil },
		RunE: func(cmd *cobra.Command, args []string) error {
			root := cmd.Root()
			switch args[0] {
			case "bash":
				return root.GenBashCompletion(cmd.OutOrStdout())
			case "zsh":
				return root.GenZshCompletion(cmd.OutOrStdout())
			case "fish":
				return root.GenFishCompletion(cmd.OutOrStdout(), true)
			case "powershell":
				return root.GenPowerShellCompletionWithDesc(cmd.OutOrStdout())
			default:
				return fmt.Errorf("unsupported shell: %s", args[0])
			}
		},
	}
	return cmd
}

//Personal.AI order the ending

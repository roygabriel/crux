package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var completionCmd = &cobra.Command{
	Use:   "completion <shell>",
	Short: "Generate shell completion script",
	Long: `Generate shell completion script for the specified shell.

Bash:
  source <(crux completion bash)

  # To install permanently:
  crux completion bash > /etc/bash_completion.d/crux

Zsh:
  # If shell completion is not already enabled, add this to ~/.zshrc:
  autoload -U compinit; compinit

  crux completion zsh > "${fpath[1]}/_crux"

Fish:
  crux completion fish | source

  # To install permanently:
  crux completion fish > ~/.config/fish/completions/crux.fish

PowerShell:
  crux completion powershell | Out-String | Invoke-Expression

  # To install permanently, add the output of the above to your PowerShell profile.
`,
	ValidArgs: []string{"bash", "zsh", "fish", "powershell"},
	Args:      cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	RunE: func(cmd *cobra.Command, args []string) error {
		out := cmd.OutOrStdout()
		switch args[0] {
		case "bash":
			return rootCmd.GenBashCompletionV2(out, true)
		case "zsh":
			return rootCmd.GenZshCompletion(out)
		case "fish":
			return rootCmd.GenFishCompletion(out, true)
		case "powershell":
			return rootCmd.GenPowerShellCompletionWithDesc(out)
		default:
			return fmt.Errorf("unsupported shell: %s", args[0])
		}
	},
}

func init() {
	rootCmd.AddCommand(completionCmd)
}

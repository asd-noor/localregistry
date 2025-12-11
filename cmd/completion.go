package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "Generate shell completion scripts",
	Long: `Generate shell completion scripts for local-registry.

To load completions:

Bash:
  $ source <(local-registry completion bash)

  # To load completions for each session, execute once:
  # Linux:
  $ local-registry completion bash > /etc/bash_completion.d/local-registry
  # macOS:
  $ local-registry completion bash > $(brew --prefix)/etc/bash_completion.d/local-registry

Zsh:
  # If shell completion is not already enabled in your environment,
  # you will need to enable it. You can execute the following once:
  $ echo "autoload -U compinit; compinit" >> ~/.zshrc

  # To load completions for each session, execute once:
  $ local-registry completion zsh > "${fpath[1]}/_local-registry"

  # You will need to start a new shell for this setup to take effect.

Fish:
  $ local-registry completion fish | source

  # To load completions for each session, execute once:
  $ local-registry completion fish > ~/.config/fish/completions/local-registry.fish

PowerShell:
  PS> local-registry completion powershell | Out-String | Invoke-Expression

  # To load completions for every new session, run:
  PS> local-registry completion powershell > local-registry.ps1
  # and source this file from your PowerShell profile.
`,
	DisableFlagsInUseLine: true,
	ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
	Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	Run: func(cmd *cobra.Command, args []string) {
		switch args[0] {
		case "bash":
			_ = cmd.Root().GenBashCompletion(os.Stdout)
		case "zsh":
			_ = cmd.Root().GenZshCompletion(os.Stdout)
		case "fish":
			_ = cmd.Root().GenFishCompletion(os.Stdout, true)
		case "powershell":
			_ = cmd.Root().GenPowerShellCompletionWithDesc(os.Stdout)
		}
	},
}

func init() {
	rootCmd.AddCommand(completionCmd)
}

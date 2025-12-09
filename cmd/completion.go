package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "Generate shell completion scripts",
	Long: `Generate shell completion scripts for localregistry.

To load completions:

Bash:
  $ source <(localregistry completion bash)

  # To load completions for each session, execute once:
  # Linux:
  $ localregistry completion bash > /etc/bash_completion.d/localregistry
  # macOS:
  $ localregistry completion bash > $(brew --prefix)/etc/bash_completion.d/localregistry

Zsh:
  # If shell completion is not already enabled in your environment,
  # you will need to enable it. You can execute the following once:
  $ echo "autoload -U compinit; compinit" >> ~/.zshrc

  # To load completions for each session, execute once:
  $ localregistry completion zsh > "${fpath[1]}/_localregistry"

  # You will need to start a new shell for this setup to take effect.

Fish:
  $ localregistry completion fish | source

  # To load completions for each session, execute once:
  $ localregistry completion fish > ~/.config/fish/completions/localregistry.fish

PowerShell:
  PS> localregistry completion powershell | Out-String | Invoke-Expression

  # To load completions for every new session, run:
  PS> localregistry completion powershell > localregistry.ps1
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

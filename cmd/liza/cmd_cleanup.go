package main

import (
	"fmt"

	"github.com/liza-mas/liza/internal/brand"
	"github.com/liza-mas/liza/internal/commands"
	"github.com/spf13/cobra"
)

var cleanupCmd = &cobra.Command{
	Use:   "cleanup",
	Short: fmt.Sprintf("Remove %s runtime state and task worktrees", brand.NameTitle),
	Long: fmt.Sprintf(`Remove %[1]s workspace runtime state, task worktrees, and the
associated task branches after displaying the exact targets for confirmation.

Cleanup refuses unrecognized registered worktrees and live agents. Use --yes
to approve the displayed deletion without an interactive prompt.`, brand.NameTitle),
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		projectRoot, err := cleanupProjectRoot()
		if err != nil {
			return err
		}
		yes, _ := cmd.Flags().GetBool("yes")
		result, err := commands.CleanupProjectCommand(commands.CleanupParams{
			ProjectRoot: projectRoot,
			Stdin:       cmd.InOrStdin(),
			Stderr:      cmd.ErrOrStderr(),
			AutoConfirm: yes,
		})
		if err != nil {
			return err
		}
		if result.Cleaned {
			fmt.Fprintln(cmd.OutOrStdout(), "Workspace cleanup complete.")
		} else {
			fmt.Fprintln(cmd.OutOrStdout(), "Nothing to clean.")
		}
		return nil
	},
}

func cleanupProjectRoot() (string, error) {
	explicitRoot, _ := rootCmd.PersistentFlags().GetString("project-root")
	if explicitRoot != "" {
		return requireExplicitGitProjectRoot(explicitRoot)
	}
	return requireProjectRoot()
}

func init() {
	rootCmd.AddCommand(cleanupCmd)
	cleanupCmd.Flags().Bool("yes", false, "auto-confirm deletion of displayed cleanup targets")
}

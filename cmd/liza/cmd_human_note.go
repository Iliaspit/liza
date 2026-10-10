package main

import (
	"fmt"
	"strings"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/ops"
	"github.com/liza-mas/liza/internal/paths"
	"github.com/spf13/cobra"
)

var addHumanNoteCmd = &cobra.Command{
	Use:   "add-human-note <task-id> --message <text>",
	Short: "Append a human note to a BLOCKED task",
	Long: `Append one human-authored note to an existing BLOCKED task.

The task stays BLOCKED and its worktree and agent claims are preserved.
The orchestrator owns assessment and resolution of the new information.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		message, err := cmd.Flags().GetString("message")
		if err != nil {
			return cliValidationWrap("failed to read --message", err)
		}
		if !cmd.Flags().Changed("message") || strings.TrimSpace(message) == "" {
			return cliValidationError("--message is required and must not be blank")
		}
		if matched := registeredFlagToken(cmd, message); matched != "" {
			return cliValidationError(fmt.Sprintf("--message value %q matches registered flag %s and may have been consumed after an empty shell expansion; provide a nonblank quoted message", message, matched))
		}
		projectRoot, err := requireProjectRoot()
		if err != nil {
			return err
		}
		if err := ops.AddHumanNote(db.For(paths.New(projectRoot).StatePath()), args[0], message); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Human note added to task %s.\n", args[0])
		return nil
	},
}

func init() {
	rootCmd.AddCommand(addHumanNoteCmd)
	addHumanNoteCmd.Flags().String("message", "", "human note text (required)")
}

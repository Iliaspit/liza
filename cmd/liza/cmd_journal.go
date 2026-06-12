package main

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/liza-mas/liza/internal/commands"
	"github.com/liza-mas/liza/internal/jsonout"
	"github.com/spf13/cobra"
)

var (
	journalSince  int64
	journalTask   string
	journalLimit  int
	journalVerify bool
)

var journalCmd = &cobra.Command{
	Use:   "journal",
	Short: "Inspect the append-only event journal",
	Long: `Read the shadow event journal (.liza/journal.jsonl) that records every
state transition as a typed event.

With --verify, additionally folds the journal into a task-status projection
and checks it reproduces the statuses in state.yaml exactly; exits non-zero
on divergence.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		projectRoot, err := requireProjectRoot()
		if err != nil {
			return err
		}

		result, err := commands.JournalCommand(projectRoot, commands.JournalOptions{
			Since:  journalSince,
			Task:   journalTask,
			Limit:  journalLimit,
			Verify: journalVerify,
		})
		if err != nil {
			return err
		}

		if isJSON(cmd) {
			if err := jsonout.WriteResult(os.Stdout, result, nil, nil); err != nil {
				return err
			}
		} else {
			renderJournal(result)
		}

		if result.Verification != nil && !result.Verification.Equivalent {
			return jsonout.ErrAlreadyWritten
		}
		return nil
	},
}

func renderJournal(result *commands.JournalResult) {
	for _, ev := range result.Events {
		var parts []string
		parts = append(parts, fmt.Sprintf("%6d", ev.Seq), ev.Time.Format("2006-01-02T15:04:05Z07:00"), ev.Type)
		if ev.Task != "" {
			parts = append(parts, "task="+ev.Task)
		}
		if ev.Agent != "" {
			parts = append(parts, "agent="+ev.Agent)
		}
		if ev.Op != "" {
			parts = append(parts, "op="+ev.Op)
		}
		keys := make([]string, 0, len(ev.Fields))
		for k := range ev.Fields {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			parts = append(parts, fmt.Sprintf("%s=%v", k, ev.Fields[k]))
		}
		fmt.Println(strings.Join(parts, "  "))
	}

	if result.Verification == nil {
		return
	}
	if result.Verification.Equivalent {
		fmt.Printf("verify: OK — journal projection matches state.yaml (%d events)\n", result.TotalEvents)
		return
	}
	fmt.Fprintln(os.Stderr, "verify: FAILED — journal projection diverges from state.yaml:")
	for taskID, pair := range result.Verification.Diff {
		fmt.Fprintf(os.Stderr, "  %s: journal=%q state=%q\n", taskID, pair[0], pair[1])
	}
}

func init() {
	rootCmd.AddCommand(journalCmd)
	addJSONFlag(journalCmd)
	journalCmd.Flags().Int64Var(&journalSince, "since", 0, "only events with seq greater than this")
	journalCmd.Flags().StringVar(&journalTask, "task", "", "filter events to a single task ID")
	journalCmd.Flags().IntVar(&journalLimit, "limit", 50, "keep only the last N matching events (0 = all)")
	journalCmd.Flags().BoolVar(&journalVerify, "verify", false, "check journal projection matches state.yaml")
}

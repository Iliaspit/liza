package main

import (
	"fmt"
	"os"
	"sort"

	"github.com/liza-mas/liza/internal/commands"
	"github.com/liza-mas/liza/internal/jsonout"
	"github.com/liza-mas/liza/internal/scheduler"
	"github.com/spf13/cobra"
)

var scheduleCmd = &cobra.Command{
	Use:   "schedule",
	Short: "Show the current runnable-work plan (read-only)",
	Long: `Compute and display the work the system could dispatch right now,
derived from current state: tasks needing a doer, tasks needing a reviewer,
approved tasks awaiting merge, and expired claims to reclaim.

This is a read-only diagnostic — it mutates nothing. It exposes the same
decision the scheduler makes; use it to understand why agents are (or are
not) picking up work.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		projectRoot, err := requireProjectRoot()
		if err != nil {
			return err
		}

		result, err := commands.ScheduleCommand(projectRoot)
		if err != nil {
			return err
		}

		if isJSON(cmd) {
			return jsonout.WriteResult(os.Stdout, result, nil, nil)
		}

		renderSchedule(result)
		return nil
	},
}

func renderSchedule(result *commands.ScheduleResult) {
	if len(result.Items) == 0 {
		fmt.Println("No runnable work.")
		return
	}

	order := []scheduler.WorkKind{scheduler.WorkMerge, scheduler.WorkReview, scheduler.WorkDoer, scheduler.WorkReclaim}
	for _, kind := range order {
		if result.Counts[string(kind)] == 0 {
			continue
		}
		fmt.Printf("%s (%d):\n", kind, result.Counts[string(kind)])
		for _, item := range result.Plan.ByKind(kind) {
			line := "  " + item.TaskID
			if item.Role != "" {
				line += "  role=" + item.Role
			}
			if item.AgentID != "" {
				line += "  agent=" + item.AgentID
			}
			if item.Detail != "" {
				line += "  (" + item.Detail + ")"
			}
			fmt.Println(line)
		}
	}

	kinds := make([]string, 0, len(result.Counts))
	total := 0
	for k, n := range result.Counts {
		kinds = append(kinds, k)
		total += n
	}
	sort.Strings(kinds)
	fmt.Printf("\n%d runnable item(s).\n", total)
}

func init() {
	rootCmd.AddCommand(scheduleCmd)
	addJSONFlag(scheduleCmd)
}

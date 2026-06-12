package commands

import (
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/paths"
	"github.com/liza-mas/liza/internal/pipeline"
	"github.com/liza-mas/liza/internal/scheduler"
)

// ScheduleResult is the read-only output of ScheduleCommand: the runnable-work
// Plan the scheduler derives from current state.
type ScheduleResult struct {
	Counts map[string]int       `json:"counts"`
	Items  []scheduler.WorkItem `json:"items"`
	Plan   scheduler.Plan       `json:"-"`
}

// ScheduleCommand computes the current runnable-work plan (doer/review/merge/
// reclaim) without mutating anything. It is a diagnostic view of the same
// decision a future scheduler loop will dispatch from.
func ScheduleCommand(projectRoot string) (*ScheduleResult, error) {
	lp := paths.New(projectRoot)
	state, err := db.For(lp.StatePath()).Read()
	if err != nil {
		return nil, err
	}

	cfg, err := pipeline.LoadFrozen(projectRoot)
	if err != nil {
		return nil, err
	}
	resolver := pipeline.NewResolver(cfg)

	plan := scheduler.Compute(state, resolver, time.Now().UTC())

	counts := map[string]int{}
	for kind, n := range plan.Counts() {
		counts[string(kind)] = n
	}

	return &ScheduleResult{
		Counts: counts,
		Items:  plan.Items,
		Plan:   plan,
	}, nil
}

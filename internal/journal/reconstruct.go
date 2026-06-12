package journal

import (
	"fmt"

	"github.com/liza-mas/liza/internal/models"
)

// Reconstruction is the subset of blackboard state that the journal's event
// stream FULLY captures, folded from events. These dimensions can be rebuilt
// exactly from the journal alone, so they are verifiable against state.yaml.
//
// What is NOT here is the point of the source-of-truth migration: task bodies
// (description, worktree, base_commit, history, output, …) are only partially
// evented (status changes, claim grants), so the journal cannot yet rebuild a
// whole task. Closing that gap — emitting complete per-field task events — is
// the remaining work before the journal can replace state.yaml. ClaimKey and
// the singletons below are the dimensions already across that line.
type Reconstruction struct {
	// TaskStatuses is the folded current status of every task (also available
	// via ProjectTaskStatuses; included here for one-call reconstruction).
	TaskStatuses TaskStatusProjection
	// Claims is the folded set of live claim holders, keyed by task+kind.
	Claims map[ClaimKey]string
	// System singletons, each fully determined by its *.status_changed event.
	SprintNumber        int
	SprintStatus        string
	CircuitBreakerState string
	GoalStatus          string
	SystemMode          string
	// sprintSeen tracks whether any sprint event set the number, so an unset
	// reconstruction (0) is distinguishable from an explicit advance to 0.
	sprintSeen bool
}

// ClaimKey identifies a claim by task and kind (doer/reviewer).
type ClaimKey struct {
	TaskID string
	Kind   string
}

// Reconstruct folds the fully-captured dimensions from an event stream.
// Pass ReadAllIncludingArchives() output for a complete (rotation-safe) fold;
// for claims and singletons there is no snapshot seeding, so the live journal
// alone is only complete when no rotation has occurred.
func Reconstruct(events []Event) Reconstruction {
	r := Reconstruction{
		TaskStatuses: ProjectTaskStatuses(events),
		Claims:       map[ClaimKey]string{},
	}
	for _, ev := range events {
		switch ev.Type {
		case EventClaimGranted:
			if kind, ok := ev.Fields["kind"].(string); ok {
				r.Claims[ClaimKey{ev.Task, kind}] = ev.Agent
			}
		case EventClaimReleased:
			if kind, ok := ev.Fields["kind"].(string); ok {
				delete(r.Claims, ClaimKey{ev.Task, kind})
			}
		case EventSprintAdvanced:
			if n, ok := intField(ev.Fields["to"]); ok {
				r.SprintNumber = n
				r.sprintSeen = true
			}
		case EventSprintStatus:
			if to, ok := ev.Fields["to"].(string); ok {
				r.SprintStatus = to
			}
		case EventCircuitBreaker:
			if to, ok := ev.Fields["to"].(string); ok {
				r.CircuitBreakerState = to
			}
		case EventGoalStatus:
			if to, ok := ev.Fields["to"].(string); ok {
				r.GoalStatus = to
			}
		case EventSystemMode:
			if to, ok := ev.Fields["to"].(string); ok {
				r.SystemMode = to
			}
		}
	}
	return r
}

// intField coerces a numeric event field, tolerating the float64 that JSON
// round-trips integers to.
func intField(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		return int(n), true
	}
	return 0, false
}

// DiffClaims compares the reconstructed claim set against the live state's
// claim records, returning a human-readable line per discrepancy. Only the
// holder (task+kind+agent) is compared — lease expiry is deliberately not
// evented, so it is not checked here. An empty result means the journal fully
// explains the current claim holders.
func (r Reconstruction) DiffClaims(state *models.State) []string {
	live := map[ClaimKey]string{}
	for _, c := range state.Claims {
		live[ClaimKey{c.TaskID, c.Kind}] = c.AgentID
	}

	var diffs []string
	for key, agent := range live {
		if r.Claims[key] != agent {
			diffs = append(diffs, fmt.Sprintf("claim %s/%s: journal=%q state=%q",
				key.TaskID, key.Kind, r.Claims[key], agent))
		}
	}
	for key, agent := range r.Claims {
		if _, ok := live[key]; !ok {
			diffs = append(diffs, fmt.Sprintf("claim %s/%s: journal=%q state=<none>",
				key.TaskID, key.Kind, agent))
		}
	}
	return diffs
}

// DiffSingletons compares the reconstructed system singletons against live
// state. A singleton is only checked once the journal has observed at least
// one event setting it (empty string / unseen sprint = no journal opinion),
// so a freshly-upgraded project with pre-journal singletons does not warn.
func (r Reconstruction) DiffSingletons(state *models.State) []string {
	var diffs []string
	check := func(name, journaled, actual string) {
		if journaled != "" && journaled != actual {
			diffs = append(diffs, fmt.Sprintf("%s: journal=%q state=%q", name, journaled, actual))
		}
	}
	check("sprint.status", r.SprintStatus, string(state.Sprint.Status))
	check("circuit_breaker", r.CircuitBreakerState, state.CircuitBreaker.Status)
	check("goal.status", r.GoalStatus, string(state.Goal.Status))
	check("system.mode", r.SystemMode, string(state.Config.Mode))
	if r.sprintSeen && r.SprintNumber != state.Sprint.Number {
		diffs = append(diffs, fmt.Sprintf("sprint.number: journal=%d state=%d", r.SprintNumber, state.Sprint.Number))
	}
	return diffs
}

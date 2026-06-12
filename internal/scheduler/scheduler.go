// Package scheduler computes, from a single immutable state snapshot, the set
// of runnable work in the system: which tasks need a doer, which need a
// reviewer, which approved tasks need merging, and which claims have expired
// and should be reclaimed.
//
// Today this readiness logic is scattered across per-role supervisor wait
// loops (internal/agent/strategy_*.go) and model helpers. Consolidating it
// into one pure, deterministic function is the first step toward a single
// central scheduler: the same Plan that powers the read-only `liza schedule`
// diagnostic is what a future scheduler loop will dispatch from.
//
// Scope note: orchestrator wake detection (internal/agent/workdetection.go)
// is intentionally NOT folded in here yet — it is referenced across the
// agent, commands, and prompts packages, so relocating it is a separate
// step. Plan therefore covers doer/review/merge/reclaim work; orchestrator
// readiness remains computed by the orchestrator strategy.
package scheduler

import (
	"fmt"
	"sort"
	"time"

	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/ops"
	"github.com/liza-mas/liza/internal/pipeline"
)

// WorkKind classifies a unit of runnable work.
type WorkKind string

const (
	// WorkDoer: a task is claimable by a doer role and needs implementation.
	WorkDoer WorkKind = "doer"
	// WorkReview: a submitted task is claimable by a reviewer role.
	WorkReview WorkKind = "review"
	// WorkMerge: an approved task is awaiting merge to the integration branch.
	WorkMerge WorkKind = "merge"
	// WorkReclaim: a lease has expired past the grace period; the claim
	// should be released so the task becomes claimable again.
	WorkReclaim WorkKind = "reclaim"
)

// WorkItem is one runnable unit of work.
type WorkItem struct {
	Kind WorkKind `json:"kind"`
	// TaskID is the task the work concerns.
	TaskID string `json:"task_id"`
	// Role is the agent role that should handle WorkDoer/WorkReview items;
	// empty for WorkMerge and WorkReclaim.
	Role string `json:"role,omitempty"`
	// AgentID is the holder of an expired claim (WorkReclaim only).
	AgentID string `json:"agent_id,omitempty"`
	// Detail is a short human-readable explanation for diagnostics.
	Detail string `json:"detail,omitempty"`
}

// Plan is the complete set of runnable work derived from a state snapshot.
// It is deterministic: identical input states yield identical Plans (items
// are sorted by kind then task ID).
type Plan struct {
	Items []WorkItem
}

// Counts returns the number of items of each kind.
func (p Plan) Counts() map[WorkKind]int {
	counts := map[WorkKind]int{}
	for _, item := range p.Items {
		counts[item.Kind]++
	}
	return counts
}

// ByKind returns the items of a single kind, preserving Plan order.
func (p Plan) ByKind(kind WorkKind) []WorkItem {
	var out []WorkItem
	for _, item := range p.Items {
		if item.Kind == kind {
			out = append(out, item)
		}
	}
	return out
}

// Compute derives the runnable-work Plan from a state snapshot.
//
// Per task (in priority order, at most one item per task):
//   - status == the role-pair's approved status        → WorkMerge
//   - claimable by the role-pair's reviewer role        → WorkReview
//   - claimable by the role-pair's doer role            → WorkDoer
//
// Then, independently, every claim whose lease expired beyond the grace
// period yields a WorkReclaim item (a task may simultaneously have an expired
// claim and, once reclaimed, become doer/review-claimable — both surface).
//
// Claimability already excludes tasks with a live (unexpired) claim, so a
// task actively being worked never appears as doer/review work.
func Compute(state *models.State, resolver *pipeline.Resolver, now time.Time) Plan {
	var items []WorkItem
	if state == nil || resolver == nil {
		return Plan{}
	}

	for i := range state.Tasks {
		task := &state.Tasks[i]
		if task.RolePair == "" {
			continue
		}
		if item, ok := classifyTask(task, state.Tasks, resolver); ok {
			items = append(items, item)
		}
	}

	for _, claim := range ops.SweepExpiredClaims(state, now) {
		items = append(items, WorkItem{
			Kind:    WorkReclaim,
			TaskID:  claim.TaskID,
			AgentID: claim.AgentID,
			Detail:  fmt.Sprintf("%s claim by %s expired", claim.Kind, claim.AgentID),
		})
	}

	sortItems(items)
	return Plan{Items: items}
}

// classifyTask returns the single highest-priority work item a task needs, if
// any. Merge takes priority over review over doer work because an approved
// task is further along the lifecycle than a submitted or ready one.
func classifyTask(task *models.Task, allTasks []models.Task, resolver *pipeline.Resolver) (WorkItem, bool) {
	if approved, err := resolver.ApprovedStatus(task.RolePair); err == nil && task.Status == approved {
		return WorkItem{
			Kind:   WorkMerge,
			TaskID: task.ID,
			Detail: fmt.Sprintf("approved (%s), awaiting merge", task.Status),
		}, true
	}

	if reviewerRole, err := resolver.ReviewerRole(task.RolePair); err == nil {
		if task.IsClaimable(reviewerRole, allTasks, resolver) {
			return WorkItem{
				Kind:   WorkReview,
				TaskID: task.ID,
				Role:   reviewerRole,
				Detail: fmt.Sprintf("%s, claimable for review", task.Status),
			}, true
		}
	}

	if doerRole, err := resolver.DoerRole(task.RolePair); err == nil {
		if task.IsClaimable(doerRole, allTasks, resolver) {
			return WorkItem{
				Kind:   WorkDoer,
				TaskID: task.ID,
				Role:   doerRole,
				Detail: fmt.Sprintf("%s, claimable by doer", task.Status),
			}, true
		}
	}

	return WorkItem{}, false
}

// kindOrder gives a stable sort priority to work kinds (merge first, matching
// lifecycle progress).
var kindOrder = map[WorkKind]int{
	WorkMerge:   0,
	WorkReview:  1,
	WorkDoer:    2,
	WorkReclaim: 3,
}

func sortItems(items []WorkItem) {
	sort.SliceStable(items, func(a, b int) bool {
		if kindOrder[items[a].Kind] != kindOrder[items[b].Kind] {
			return kindOrder[items[a].Kind] < kindOrder[items[b].Kind]
		}
		return items[a].TaskID < items[b].TaskID
	})
}

// Package taskinvariants holds the structural per-task invariants: the fields
// every task status requires (or forbids). It is a leaf package — importing
// only models and pipeline — so both the at-rest validator (statevalidate)
// and the write funnel (db) can enforce the SAME rules from one source.
//
// At-rest validation reports violations after the fact; funnel enforcement
// rejects the write that would introduce them. As writers migrate to named
// operations the at-rest checks become a backstop instead of the gate.
package taskinvariants

import (
	"fmt"
	"strings"

	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/pipeline"
)

// StatusClassifier classifies pipeline-declared task statuses by lifecycle
// phase (initial, executing, submitted, reviewing, approved, rejected).
type StatusClassifier struct {
	executing []models.TaskStatus
	initial   []models.TaskStatus
	submitted []models.TaskStatus
	reviewing []models.TaskStatus
	approved  []models.TaskStatus
	partial   []models.TaskStatus
	rejected  []models.TaskStatus
}

// NewStatusClassifier constructs a StatusClassifier from a pipeline resolver
// and configuration. Returns an empty classifier when resolver or config is nil.
func NewStatusClassifier(resolver *pipeline.Resolver, cfg *pipeline.PipelineConfig) StatusClassifier {
	sc := StatusClassifier{}
	if resolver == nil || cfg == nil {
		return sc
	}
	for rpName := range cfg.Pipeline.RolePairs {
		if s, err := resolver.ExecutingStatus(rpName); err == nil {
			sc.executing = append(sc.executing, s)
		}
		if s, err := resolver.InitialStatus(rpName); err == nil {
			sc.initial = append(sc.initial, s)
		}
		if s, err := resolver.SubmittedStatus(rpName); err == nil {
			sc.submitted = append(sc.submitted, s)
		}
		if s, err := resolver.ReviewingStatus(rpName); err == nil {
			sc.reviewing = append(sc.reviewing, s)
		}
		// reviewing-2 reuses the review lease mechanism — classify as reviewing.
		if s, err := resolver.Reviewing2Status(rpName); err == nil {
			sc.reviewing = append(sc.reviewing, s)
		}
		if s, err := resolver.ApprovedStatus(rpName); err == nil {
			sc.approved = append(sc.approved, s)
		}
		if s, err := resolver.PartiallyApprovedStatus(rpName); err == nil {
			sc.partial = append(sc.partial, s)
		}
		if s, err := resolver.RejectedStatus(rpName); err == nil {
			sc.rejected = append(sc.rejected, s)
		}
	}
	return sc
}

func containsStatus(list []models.TaskStatus, s models.TaskStatus) bool {
	for _, v := range list {
		if s == v {
			return true
		}
	}
	return false
}

func (sc *StatusClassifier) IsExecuting(s models.TaskStatus) bool {
	return containsStatus(sc.executing, s)
}

func (sc *StatusClassifier) IsInitial(s models.TaskStatus) bool {
	return containsStatus(sc.initial, s)
}

func (sc *StatusClassifier) IsSubmitted(s models.TaskStatus) bool {
	return containsStatus(sc.submitted, s)
}

func (sc *StatusClassifier) IsReviewing(s models.TaskStatus) bool {
	return containsStatus(sc.reviewing, s)
}

func (sc *StatusClassifier) IsApproved(s models.TaskStatus) bool {
	return containsStatus(sc.approved, s)
}

func (sc *StatusClassifier) IsPartiallyApproved(s models.TaskStatus) bool {
	return containsStatus(sc.partial, s)
}

func (sc *StatusClassifier) IsRejected(s models.TaskStatus) bool {
	return s == models.TaskStatusRejected || s == models.TaskStatusCodingPlanRejected || containsStatus(sc.rejected, s)
}

// ValidateStatusFields checks that a task carries the fields its status
// requires (and none its status forbids). These are the structural invariants
// behind "REVIEWING task must have reviewing_by", "MERGED task must not have
// worktree", etc.
func ValidateStatusFields(task *models.Task, sc *StatusClassifier) error {
	if sc.IsInitial(task.Status) && task.AssignedTo != nil {
		return fmt.Errorf("%s task with assigned_to: %s", task.Status, task.ID)
	}

	if sc.IsExecuting(task.Status) {
		if task.AssignedTo == nil {
			return fmt.Errorf("%s task without assigned_to: %s", task.Status, task.ID)
		}
		if task.Worktree == nil {
			return fmt.Errorf("%s task without worktree: %s", task.Status, task.ID)
		}
		if !task.IntegrationFix && task.BaseCommit == nil {
			return fmt.Errorf("%s task without base_commit: %s", task.Status, task.ID)
		}
		if task.LeaseExpires == nil {
			return fmt.Errorf("%s task without lease_expires: %s", task.Status, task.ID)
		}
	}

	if sc.IsSubmitted(task.Status) && task.ReviewCommit == nil {
		return fmt.Errorf("%s task without review_commit: %s", task.Status, task.ID)
	}

	if sc.IsReviewing(task.Status) {
		if task.ReviewingBy == nil {
			return fmt.Errorf("%s task without reviewing_by: %s", task.Status, task.ID)
		}
		if task.ReviewLeaseExpires == nil {
			return fmt.Errorf("%s task without review_lease_expires: %s", task.Status, task.ID)
		}
		if task.ReviewCommit == nil {
			return fmt.Errorf("%s task without review_commit: %s", task.Status, task.ID)
		}
	}

	if sc.IsApproved(task.Status) && task.ReviewCommit == nil {
		return fmt.Errorf("%s task without review_commit: %s", task.Status, task.ID)
	}
	if task.IntegrationFailure != nil && disallowsIntegrationFailure(task.Status, sc) {
		return fmt.Errorf("%s task has stale integration_failure outside integration recovery: %s", task.Status, task.ID)
	}

	if task.Status == models.TaskStatusMerged && task.Worktree != nil {
		return fmt.Errorf("MERGED task still has worktree: %s", task.ID)
	}

	if task.Status == models.TaskStatusBlocked {
		if task.BlockedReason == nil {
			return fmt.Errorf("BLOCKED task without blocked_reason: %s", task.ID)
		}
		if len(task.BlockedQuestions) == 0 {
			return fmt.Errorf("BLOCKED task without blocked_questions: %s", task.ID)
		}
		if task.RepairRequest != nil && strings.TrimSpace(task.RepairRequest.Operation) == "" {
			return fmt.Errorf("BLOCKED task repair_request without operation: %s", task.ID)
		}
		if task.RepairRequest != nil && strings.TrimSpace(task.RepairRequest.Target) == "" {
			return fmt.Errorf("BLOCKED task repair_request without target: %s", task.ID)
		}
		if task.RepairRequest != nil && strings.TrimSpace(task.RepairRequest.Command) == "" {
			return fmt.Errorf("BLOCKED task repair_request without command: %s", task.ID)
		}
		if task.RepairRequest != nil && len(nonEmptyStrings(task.RepairRequest.Evidence)) == 0 {
			return fmt.Errorf("BLOCKED task repair_request without evidence: %s", task.ID)
		}
		if task.RepairRequest != nil && len(nonEmptyStrings(task.RepairRequest.Validation)) == 0 {
			return fmt.Errorf("BLOCKED task repair_request without validation: %s", task.ID)
		}
	}

	if sc.IsRejected(task.Status) && task.RejectionReason == nil {
		return fmt.Errorf("%s task without rejection_reason: %s", task.Status, task.ID)
	}

	if task.Status == models.TaskStatusSuperseded {
		if task.RescopeReason == nil {
			return fmt.Errorf("SUPERSEDED task without rescope_reason: %s", task.ID)
		}
	}

	return nil
}

func disallowsIntegrationFailure(status models.TaskStatus, sc *StatusClassifier) bool {
	// Belt-and-suspenders: explicit legacy/built-in statuses remain protected
	// even if no active pipeline resolver declares the matching lifecycle state.
	switch status {
	case models.TaskStatusReadyForReview,
		models.TaskStatusLegacyReadyForReview,
		models.TaskStatusReviewing,
		models.TaskStatusPartiallyApproved,
		models.TaskStatusReviewingCode2,
		models.TaskStatusApproved,
		models.TaskStatusCodingPlanToReview,
		models.TaskStatusReviewingCodingPlan,
		models.TaskStatusCodingPlanApproved:
		return true
	}
	return sc.IsSubmitted(status) || sc.IsReviewing(status) || sc.IsPartiallyApproved(status) || sc.IsApproved(status)
}

func nonEmptyStrings(values []string) []string {
	var nonEmpty []string
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			nonEmpty = append(nonEmpty, value)
		}
	}
	return nonEmpty
}

package ops

import (
	"fmt"
	"io"
	"time"

	"github.com/liza-mas/liza/internal/alerts"
	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/errors"
	"github.com/liza-mas/liza/internal/identity"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/paths"
	"github.com/liza-mas/liza/internal/roles"
	"github.com/liza-mas/liza/internal/statevalidate"
)

// AssessBlockedResult contains the outcome of recording an orchestrator assessment.
type AssessBlockedResult struct {
	TaskID        string                `json:"task_id"`
	Reason        string                `json:"reason,omitempty"`
	Questions     []string              `json:"questions,omitempty"`
	RepairRequest *models.RepairRequest `json:"repair_request,omitempty"`
	Warnings      []string              `json:"warnings,omitempty"`
}

func (r *AssessBlockedResult) GetWarnings() []string {
	if r == nil {
		return nil
	}
	return r.Warnings
}

// AssessBlockedOptions contains canonical blocker metadata for a structured
// reassessment. Supplying any field enables reconciliation mode, which requires
// both Reason and one to three Questions. A nil RepairRequest clears any prior
// request from the canonical blocker state.
type AssessBlockedOptions struct {
	Reason        string
	Questions     []string
	RepairRequest *models.RepairRequest
}

// AssessBlocked records that the orchestrator has assessed a BLOCKED task.
// Appends an orchestrator_assessment history entry without changing task status.
// This prevents the wake-detection loop where the orchestrator repeatedly wakes
// for blocked tasks it has already triaged.
func AssessBlocked(projectRoot, taskID, note, agentID string) (*AssessBlockedResult, error) {
	return AssessBlockedWithOptions(projectRoot, taskID, note, agentID, AssessBlockedOptions{})
}

// AssessBlockedWithOptions records an orchestrator assessment and optionally
// replaces the BLOCKED task's canonical blocker metadata in one validated state
// transaction. The zero-value options preserve AssessBlocked's history-only
// behavior.
func AssessBlockedWithOptions(projectRoot, taskID, note, agentID string, opts AssessBlockedOptions) (*AssessBlockedResult, error) {
	if taskID == "" {
		return nil, &PreconditionError{Reason: "task ID is required"}
	}
	if agentID == "" {
		return nil, &PreconditionError{Reason: "agent ID is required"}
	}
	// Defense-in-depth: orchestrator_assessment history entries suppress future wakes,
	// so this must be restricted to orchestrator agents even though the MCP handler
	// also gates via resolveOrchestratorID.
	if err := identity.ValidateRole(agentID, roles.Orchestrator); err != nil {
		return nil, &PreconditionError{Reason: fmt.Sprintf("only orchestrator agents can assess blocked tasks: %v", err)}
	}

	reconcile := opts.Reason != "" || len(opts.Questions) > 0 || opts.RepairRequest != nil
	var repairRequest *models.RepairRequest
	if reconcile {
		if opts.Reason == "" {
			return nil, &PreconditionError{Reason: "reason is required"}
		}
		if len(opts.Questions) == 0 {
			return nil, &PreconditionError{Reason: "at least 1 question is required"}
		}
		if len(opts.Questions) > 3 {
			return nil, &PreconditionError{Reason: "maximum 3 questions allowed per blocking protocol"}
		}
		var err error
		repairRequest, err = normalizeRepairRequest(opts.RepairRequest, taskID)
		if err != nil {
			return nil, err
		}
	}

	lp := paths.New(projectRoot)
	bb := db.For(lp.StatePath())
	now := time.Now().UTC()
	result := AssessBlockedResult{TaskID: taskID}

	err := bb.Modify(func(state *models.State) error {
		task := state.FindTask(taskID)
		if task == nil {
			return &errors.NotFoundError{Entity: "task", ID: taskID}
		}

		if task.Status != models.TaskStatusBlocked {
			return &PreconditionError{Reason: fmt.Sprintf("task must be in BLOCKED status to assess, current status: %s", task.Status)}
		}

		entry := models.TaskHistoryEntry{
			Time:  now,
			Event: models.TaskEventOrchestratorAssessment,
			Agent: &agentID,
		}
		if note != "" {
			entry.Note = &note
		}
		if reconcile {
			reason := opts.Reason
			questions := append([]string(nil), opts.Questions...)
			task.BlockedReason = &reason
			task.BlockedQuestions = questions
			task.RepairRequest = repairRequest
			entry.Reason = &reason
			entry.Extra = map[string]any{
				"blocked_questions": append([]string(nil), questions...),
				"repair_request":    repairRequest,
			}
		}

		task.History = append(task.History, entry)
		if reconcile {
			if err := statevalidate.ValidateState(state, projectRoot, false, io.Discard); err != nil {
				return err
			}
			result.Reason = opts.Reason
			result.Questions = append([]string(nil), opts.Questions...)
			result.RepairRequest = repairRequest
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to assess blocked task: %w", err)
	}

	message := taskID
	if note != "" {
		message = fmt.Sprintf("%s — %s", taskID, note)
	}
	var warnings []string
	if err := alerts.Write(lp.AlertsLogPath(), alerts.Alert{
		Timestamp: now,
		Level:     alerts.AlertLevelCritical,
		Category:  "UNRESOLVED BLOCKED",
		Message:   message,
	}); err != nil {
		warnings = append(warnings, fmt.Sprintf("alert write failed: %v", err))
	}

	result.Warnings = warnings
	return &result, nil
}

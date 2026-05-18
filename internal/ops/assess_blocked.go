package ops

import (
	"fmt"
	"time"

	"github.com/liza-mas/liza/internal/alerts"
	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/errors"
	"github.com/liza-mas/liza/internal/identity"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/paths"
	"github.com/liza-mas/liza/internal/roles"
)

// AssessBlockedResult contains the outcome of recording an orchestrator assessment.
type AssessBlockedResult struct {
	TaskID   string   `json:"task_id"`
	Warnings []string `json:"warnings,omitempty"`
}

func (r *AssessBlockedResult) GetWarnings() []string {
	if r == nil {
		return nil
	}
	return r.Warnings
}

// AssessBlocked records that the orchestrator has assessed a BLOCKED task.
// Appends an orchestrator_assessment history entry without changing task status.
// This prevents the wake-detection loop where the orchestrator repeatedly wakes
// for blocked tasks it has already triaged.
func AssessBlocked(projectRoot, taskID, note, agentID string) (*AssessBlockedResult, error) {
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
	lp := paths.New(projectRoot)
	bb := db.For(lp.StatePath())
	now := time.Now().UTC()

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

		task.History = append(task.History, entry)
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

	return &AssessBlockedResult{TaskID: taskID, Warnings: warnings}, nil
}

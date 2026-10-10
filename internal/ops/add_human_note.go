package ops

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/paths"
	"github.com/liza-mas/liza/internal/statevalidate"
	"gopkg.in/yaml.v3"
)

// AddHumanNote appends one human-authored note to an existing BLOCKED task.
// Validation and append share the blackboard lock. The accepted message is
// preserved verbatim; task lifecycle and agent authority remain unchanged.
func AddHumanNote(bb *db.Blackboard, taskID, message string) error {
	return bb.Modify(func(state *models.State) error {
		if err := paths.ValidateTaskID(taskID); err != nil {
			return &PreconditionError{Reason: fmt.Sprintf("invalid task ID: %v", err)}
		}
		if taskID == "all" {
			return &PreconditionError{Reason: "human note requires one task ID, not all"}
		}
		if strings.TrimSpace(message) == "" {
			return &PreconditionError{Reason: "message is required and must not be blank"}
		}
		task := state.FindTask(taskID)
		if task == nil {
			return &PreconditionError{Reason: fmt.Sprintf("task %q not found in state", taskID)}
		}
		if task.Status != models.TaskStatusBlocked {
			return &PreconditionError{Reason: fmt.Sprintf("task %q must be BLOCKED, got %s", taskID, task.Status)}
		}
		// Modify normalizes legacy roles and attempts before this callback. Decode
		// the locked bytes directly to reject that unrelated rewrite; ReadRaw
		// would try to acquire the same file lock again.
		data, err := os.ReadFile(bb.GetStatePath())
		if err != nil {
			return fmt.Errorf("read locked state: %w", err)
		}
		var raw models.State
		if err := yaml.Unmarshal(data, &raw); err != nil {
			return fmt.Errorf("decode locked state: %w", err)
		}
		if !reflect.DeepEqual(&raw, state) {
			return &PreconditionError{Reason: "human note requires canonical state; persisted state requires normalization"}
		}
		projectRoot := filepath.Dir(filepath.Dir(bb.GetStatePath()))
		if err := statevalidate.ValidateState(state, projectRoot, false, io.Discard); err != nil {
			return fmt.Errorf("human note requires canonical state: %w", err)
		}
		state.HumanNotes = append(state.HumanNotes, models.HumanNote{
			Timestamp: time.Now().UTC(),
			Message:   message,
			For:       taskID,
			Extra:     map[string]any{"changed_by": "human"},
		})
		return nil
	})
}

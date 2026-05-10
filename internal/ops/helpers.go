package ops

import (
	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/errors"
	"github.com/liza-mas/liza/internal/models"
)

// readTaskState reads state from the blackboard and finds the specified task.
// Returns an error if state cannot be read or the task doesn't exist.
func readTaskState(bb *db.Blackboard, taskID string) (*models.State, *models.Task, error) {
	state, err := bb.Read()
	if err != nil {
		return nil, nil, err
	}
	task := state.FindTask(taskID)
	if task == nil {
		return nil, nil, &errors.NotFoundError{Entity: "task", ID: taskID}
	}
	return state, task, nil
}

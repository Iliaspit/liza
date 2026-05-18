package ops

import (
	"fmt"
	"time"

	"github.com/liza-mas/liza/internal/db"
	lizaerrors "github.com/liza-mas/liza/internal/errors"
	"github.com/liza-mas/liza/internal/git"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/paths"
)

// ResumeOwnedTaskInput contains the parameters for resuming an already-owned
// executing task after the child CLI exited.
type ResumeOwnedTaskInput struct {
	ProjectRoot string
	AgentID     string
}

// ResumeOwnedTaskResult contains the outcome of owned-task recovery.
type ResumeOwnedTaskResult struct {
	TaskID        string
	Worktree      string
	Found         bool
	Blocked       bool
	BlockedTaskID string
	BlockReason   string
}

var validateOwnedResumeWorktreeHealth = func(g *git.Git, taskID string) error {
	return g.ValidateWorktreeHealth(taskID)
}

// CountResumableOwnedTasks counts already-owned executing tasks that should
// wake a doer supervisor. It intentionally checks state only; ResumeOwnedTask
// performs worktree validation before spawning a child CLI.
func CountResumableOwnedTasks(state *models.State, agentID string, pr models.PipelineResolver) int {
	count := 0
	for i := range state.Tasks {
		if models.IsResumableOwnedTask(state, &state.Tasks[i], agentID, pr) {
			count++
		}
	}
	return count
}

// ResumeOwnedTask resumes an executing task that is already assigned to the
// supervisor's agent ID. This is recovery, not handoff: handoff_pending stays
// reserved for explicit peer handoffs.
func ResumeOwnedTask(input ResumeOwnedTaskInput) (*ResumeOwnedTaskResult, error) {
	if input.AgentID == "" {
		return nil, &PreconditionError{Reason: "agent ID is required"}
	}

	lp := paths.New(input.ProjectRoot)
	bb := db.For(lp.StatePath())

	resolver, _, err := loadResolver(input.ProjectRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to load pipeline config: %w", err)
	}
	pipelineTransitions := BuildPipelineTransitions(resolver)
	gitWrapper := git.New(lp.ProjectRoot())

	state, err := bb.Read()
	if err != nil {
		return nil, fmt.Errorf("failed to read state: %w", err)
	}

	for i := range state.Tasks {
		task := &state.Tasks[i]
		if !models.IsResumableOwnedTask(state, task, input.AgentID, resolver) {
			continue
		}

		if task.Worktree == nil || *task.Worktree == "" {
			reason := fmt.Sprintf("owned task resume failed: task %s has no worktree in state", task.ID)
			blocked, err := blockOwnedResumeCandidate(bb, task.ID, input.AgentID, resolver, pipelineTransitions, reason)
			if err != nil {
				return nil, err
			}
			if blocked {
				return &ResumeOwnedTaskResult{Blocked: true, BlockedTaskID: task.ID, BlockReason: reason}, nil
			}
			continue
		}

		if err := validateOwnedResumeWorktreeHealth(gitWrapper, task.ID); err != nil {
			reason := fmt.Sprintf("owned task resume failed: worktree not healthy: %v", err)
			blocked, blockErr := blockOwnedResumeCandidate(bb, task.ID, input.AgentID, resolver, pipelineTransitions, reason)
			if blockErr != nil {
				return nil, blockErr
			}
			if blocked {
				return &ResumeOwnedTaskResult{Blocked: true, BlockedTaskID: task.ID, BlockReason: reason}, nil
			}
			continue
		}

		resumed, result, err := resumeOwnedCandidate(bb, task.ID, input.AgentID, resolver)
		if err != nil {
			return nil, err
		}
		if resumed {
			return result, nil
		}
	}

	return &ResumeOwnedTaskResult{Found: false}, nil
}

func resumeOwnedCandidate(bb *db.Blackboard, taskID, agentID string, pr models.PipelineResolver) (bool, *ResumeOwnedTaskResult, error) {
	now := time.Now().UTC()
	var worktree string

	err := bb.Modify(func(state *models.State) error {
		task := state.FindTask(taskID)
		if task == nil {
			return &lizaerrors.NotFoundError{Entity: "task", ID: taskID}
		}
		if !models.IsResumableOwnedTask(state, task, agentID, pr) {
			return nil
		}
		if task.Worktree == nil || *task.Worktree == "" {
			return &PreconditionError{Reason: fmt.Sprintf("task %s missing worktree", taskID)}
		}

		worktree = *task.Worktree
		renewLease(state, task)
		task.History = append(task.History, models.TaskHistoryEntry{
			Time:  now,
			Event: models.TaskEventOwnedTaskResumed,
			Agent: &agentID,
		})

		agent := state.Agents[agentID]
		agent.Status = models.AgentStatusWorking
		agent.CurrentTask = &taskID
		agent.LeaseExpires = task.LeaseExpires
		agent.Heartbeat = now
		state.Agents[agentID] = agent
		return nil
	})
	if err != nil {
		return false, nil, err
	}
	if worktree == "" {
		return false, nil, nil
	}

	return true, &ResumeOwnedTaskResult{
		TaskID:   taskID,
		Worktree: worktree,
		Found:    true,
	}, nil
}

func blockOwnedResumeCandidate(
	bb *db.Blackboard,
	taskID, agentID string,
	pr models.PipelineResolver,
	pipelineTransitions map[models.TaskStatus][]models.TaskStatus,
	reason string,
) (bool, error) {
	now := time.Now().UTC()
	blocked := false
	questions := []string{"Repair or recreate the task worktree, then unblock the task."}

	err := bb.Modify(func(state *models.State) error {
		task := state.FindTask(taskID)
		if task == nil {
			return &lizaerrors.NotFoundError{Entity: "task", ID: taskID}
		}
		if !models.IsResumableOwnedTask(state, task, agentID, pr) {
			return nil
		}
		if err := task.TransitionWith(models.TaskStatusBlocked, pipelineTransitions); err != nil {
			return err
		}

		task.BlockedReason = &reason
		task.BlockedQuestions = questions
		task.AssignedTo = nil
		task.LeaseExpires = nil
		releaseAgentsForTask(state, taskID)
		task.History = append(task.History, models.TaskHistoryEntry{
			Time:   now,
			Event:  models.TaskEventBlocked,
			Agent:  &agentID,
			Reason: &reason,
		})
		blocked = true
		return nil
	})
	if err != nil {
		return false, err
	}
	return blocked, nil
}

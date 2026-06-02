package ops

import (
	"fmt"
	"os"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/git"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/paths"
)

// RecoverTaskResult contains the outcome of recovering a task.
type RecoverTaskResult struct {
	TaskID          string
	InState         bool   // true if task was found in state
	AgentID         string // agent that held the claim, if any
	AgentRole       string
	ClaimReleased   bool
	WorktreeRemoved bool
	BranchRemoved   bool
	AgentRecovered  bool
	Warnings        []string
}

// RecoverTask performs full recovery for a task: releases claims, removes worktree
// and branch, and optionally recovers the claiming agent.
//
// Without force: requires the task to exist in state, and refuses if the claiming
// agent's PID is still alive.
//
// With force: cleans up git artifacts (worktree + branch) even if the task is not
// in state. This handles the case where state is already clean but git artifacts
// linger after a hard crash.
//
// Idempotent: safe to run multiple times. No terminal I/O.
func RecoverTask(projectRoot, taskID string, force bool, reason string) (*RecoverTaskResult, error) {
	if taskID == "" {
		return nil, fmt.Errorf("task ID required")
	}
	if err := paths.ValidateTaskID(taskID); err != nil {
		return nil, fmt.Errorf("invalid task ID: %w", err)
	}
	if reason == "" {
		reason = "task recovery"
	}

	lp := paths.New(projectRoot)
	bb := db.For(lp.StatePath())
	gitWrapper := git.New(projectRoot)

	result := &RecoverTaskResult{
		TaskID: taskID,
	}

	// Phase 1: Read state to find task and claiming agent
	state, err := bb.Read()
	if err != nil && !force {
		return nil, fmt.Errorf("failed to read state: %w", err)
	}

	var task *models.Task
	var coderAgentID string
	var reviewerAgentID string

	if err == nil && state != nil {
		task = state.FindTask(taskID)
	}

	if task != nil {
		result.InState = true

		if task.AssignedTo != nil {
			coderAgentID = *task.AssignedTo
			if agent, exists := state.Agents[coderAgentID]; exists {
				if !force && agent.PID != 0 && IsProcessAlive(agent.PID) {
					return nil, fmt.Errorf("task %s: coder agent %s (PID %d) still running, use --force to recover",
						taskID, coderAgentID, agent.PID)
				}
			}
		}
		if task.ReviewingBy != nil {
			reviewerAgentID = *task.ReviewingBy
			if agent, exists := state.Agents[reviewerAgentID]; exists {
				if !force && agent.PID != 0 && IsProcessAlive(agent.PID) {
					return nil, fmt.Errorf("task %s: reviewer agent %s (PID %d) still running, use --force to recover",
						taskID, reviewerAgentID, agent.PID)
				}
			}
		}

		// Coder takes precedence for primary claiming agent
		if coderAgentID != "" {
			result.AgentID = coderAgentID
			result.AgentRole = "coder"
		} else if reviewerAgentID != "" {
			result.AgentID = reviewerAgentID
			result.AgentRole = "code-reviewer"
		}
	} else if !force {
		return nil, fmt.Errorf("task %s not found in state, use --force to clean up git artifacts anyway", taskID)
	}

	// Phase 2: Git cleanup (outside lock) — remove worktree and branch
	// Pre-check existence so result flags reflect what actually happened
	wtPath := gitWrapper.GetWorktreePath(taskID)
	branchName := paths.TaskBranchPrefix + taskID

	wtExisted := false
	if _, statErr := os.Stat(wtPath); statErr == nil {
		wtExisted = true
	}
	branchExisted, branchErr := gitWrapper.BranchExists(branchName)
	if branchErr != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("branch existence check: %v", branchErr))
		// Assume it exists so we attempt cleanup rather than silently skip
		branchExisted = true
	}

	if wtExisted || branchExisted {
		if err := gitWrapper.RemoveWorktree(taskID); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("worktree removal: %v", err))
		} else {
			result.WorktreeRemoved = wtExisted
			result.BranchRemoved = branchExisted
		}
	}

	// If task not in state, we're done — git cleanup was all we could do
	if !result.InState {
		return result, nil
	}

	// Load pipeline resolver for pipeline-aware claim release
	var pipelineTransitions map[models.TaskStatus][]models.TaskStatus
	resolver, _, resolverErr := loadResolver(projectRoot)
	if resolverErr != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("pipeline config: %v", resolverErr))
	} else {
		pipelineTransitions = BuildPipelineTransitions(resolver)
	}

	// Phase 3: State cleanup (atomic)
	// All agent IDs are re-read from current state inside Modify to avoid TOCTOU.
	now := time.Now().UTC()
	err = bb.Modify(func(state *models.State) error {
		task := state.FindTask(taskID)
		if task == nil {
			// Task disappeared between read and modify — nothing to do
			return nil
		}

		// Collect agent IDs before claim release clears them
		agentsToRecover := map[string]bool{}
		if task.AssignedTo != nil {
			agentsToRecover[*task.AssignedTo] = true
		}
		if task.ReviewingBy != nil {
			agentsToRecover[*task.ReviewingBy] = true
		}

		claimReleaseFailed := false

		if resolver == nil {
			if task.AssignedTo != nil {
				currentCoderID := *task.AssignedTo
				result.Warnings = append(result.Warnings, "coder claim release: pipeline resolver not loaded")
				delete(agentsToRecover, currentCoderID)
				claimReleaseFailed = true
			}
			if task.ReviewingBy != nil {
				currentReviewerID := *task.ReviewingBy
				result.Warnings = append(result.Warnings, "reviewer claim release: pipeline resolver not loaded")
				delete(agentsToRecover, currentReviewerID)
				claimReleaseFailed = true
			}
		} else {
			if task.AssignedTo != nil {
				currentCoderID := *task.AssignedTo
				effectiveCoderRelease := resolveDoerClaimReleaseStatus(task, resolver)
				released, err := releaseOneClaim(state, task, effectiveCoderRelease, pipelineTransitions, true, currentCoderID, reason, now)
				if err != nil {
					result.Warnings = append(result.Warnings, fmt.Sprintf("coder claim release: %v", err))
					delete(agentsToRecover, currentCoderID)
					claimReleaseFailed = true
				}
				if released {
					result.ClaimReleased = true
				}
			}

			if task.ReviewingBy != nil {
				currentReviewerID := *task.ReviewingBy
				effectiveReviewerRelease, err := resolveReviewerClaimReleaseStatus(task, resolver)
				if err != nil {
					result.Warnings = append(result.Warnings, fmt.Sprintf("reviewer claim release: %v", err))
					delete(agentsToRecover, currentReviewerID)
					claimReleaseFailed = true
				} else {
					released, err := releaseOneClaim(state, task, effectiveReviewerRelease, pipelineTransitions, true, currentReviewerID, reason, now)
					if err != nil {
						result.Warnings = append(result.Warnings, fmt.Sprintf("reviewer claim release: %v", err))
						delete(agentsToRecover, currentReviewerID)
						claimReleaseFailed = true
					}
					if released {
						result.ClaimReleased = true
					}
				}
			}
		}

		if !claimReleaseFailed {
			task.Worktree = nil
		}

		// Detect and fix review_commit corruption: tasks in submitted/reviewing/approved
		// states without review_commit are unclaimable and stuck. Reset to initial status.
		if resolver != nil && task.ReviewCommit == nil {
			submitted, _ := resolver.SubmittedStatus(task.RolePair)
			reviewing, _ := resolver.ReviewingStatus(task.RolePair)
			approved, _ := resolver.ApprovedStatus(task.RolePair)
			pa, _ := resolver.PartiallyApprovedStatus(task.RolePair)
			r2, _ := resolver.Reviewing2Status(task.RolePair)
			needsReset := task.Status == submitted || task.Status == reviewing ||
				task.Status == approved ||
				(pa != "" && task.Status == pa) ||
				(r2 != "" && task.Status == r2)
			if needsReset {
				initial, initErr := resolver.InitialStatus(task.RolePair)
				if initErr == nil {
					task.Status = initial
					task.ReviewingBy = nil
					task.ReviewLeaseExpires = nil
					clearAttemptState(task, attemptStateInitialReset)
					result.Warnings = append(result.Warnings,
						fmt.Sprintf("reset %s to %s: missing review_commit", taskID, initial))
				}
			}
		}

		for agentID := range agentsToRecover {
			if _, exists := state.Agents[agentID]; exists {
				delete(state.Agents, agentID)
				result.AgentRecovered = true
			}
		}

		var recoveredAgents []string
		for agentID := range agentsToRecover {
			recoveredAgents = append(recoveredAgents, agentID)
		}
		msg := fmt.Sprintf("Task %s recovered: %s", taskID, reason)
		if len(recoveredAgents) == 1 {
			msg = fmt.Sprintf("Task %s recovered (was held by %s): %s", taskID, recoveredAgents[0], reason)
		} else if len(recoveredAgents) > 1 {
			msg = fmt.Sprintf("Task %s recovered (was held by %v): %s", taskID, recoveredAgents, reason)
		}
		state.HumanNotes = append(state.HumanNotes, models.HumanNote{
			Timestamp: now,
			Message:   msg,
			For:       taskID,
		})

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to recover task: %w", err)
	}

	return result, nil
}

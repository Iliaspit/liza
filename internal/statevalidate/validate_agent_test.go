package statevalidate

import (
	"io"
	"os"
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/testhelpers"
)

func TestValidateAgentInvariants_ActiveReviewOwnership(t *testing.T) {
	resolver := loadTestResolver(t)
	now := time.Now().UTC()

	tests := []struct {
		name        string
		mutateState func(*models.State, string)
		wantErr     string
	}{
		{
			name: "valid reviewing ownership",
		},
		{
			name: "missing reviewer agent",
			mutateState: func(state *models.State, reviewerID string) {
				delete(state.Agents, reviewerID)
			},
			wantErr: "reviewing_by code-reviewer-1 has no matching agent",
		},
		{
			name: "wrong reviewer role",
			mutateState: func(state *models.State, reviewerID string) {
				agent := state.Agents[reviewerID]
				agent.Role = models.RoleCodePlanReviewer
				state.Agents[reviewerID] = agent
			},
			wantErr: `has role "code-plan-reviewer", want "code-reviewer"`,
		},
		{
			name: "idle reviewer agent",
			mutateState: func(state *models.State, reviewerID string) {
				agent := state.Agents[reviewerID]
				agent.Status = models.AgentStatusIdle
				state.Agents[reviewerID] = agent
			},
			wantErr: "has agent status IDLE, want REVIEWING",
		},
		{
			name: "mismatched current task",
			mutateState: func(state *models.State, reviewerID string) {
				agent := state.Agents[reviewerID]
				agent.CurrentTask = testhelpers.StringPtr("other-task")
				state.Agents[reviewerID] = agent
			},
			wantErr: "has mismatched current_task",
		},
		{
			name: "missing agent lease",
			mutateState: func(state *models.State, reviewerID string) {
				agent := state.Agents[reviewerID]
				agent.LeaseExpires = nil
				state.Agents[reviewerID] = agent
			},
			wantErr: "has agent without lease_expires",
		},
		{
			name: "missing agent pid",
			mutateState: func(state *models.State, reviewerID string) {
				agent := state.Agents[reviewerID]
				agent.PID = 0
				state.Agents[reviewerID] = agent
			},
			wantErr: "agent code-reviewer-1 has active lease but no pid",
		},
		{
			name: "reviewing-2 ownership",
			mutateState: func(state *models.State, reviewerID string) {
				state.Tasks[0].Status = models.TaskStatusReviewingCode2
				agent := state.Agents[reviewerID]
				agent.Status = models.AgentStatusIdle
				state.Agents[reviewerID] = agent
			},
			wantErr: "REVIEWING_CODE_2 task task-1 reviewing_by code-reviewer-1 has agent status IDLE, want REVIEWING",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state, reviewerID := activeReviewOwnershipState(now)
			if tt.mutateState != nil {
				tt.mutateState(state, reviewerID)
			}

			err := validateAgentInvariants(state, "", true, io.Discard, resolver)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("validateAgentInvariants() error = %v, want nil", err)
				}
				return
			}
			assertErrorContains(t, err, tt.wantErr)
		})
	}
}

func TestValidateAgentInvariants_NonReviewingOrphanReviewingByNotActiveOwnership(t *testing.T) {
	resolver := loadTestResolver(t)
	now := time.Now().UTC()
	state := stateWithTasks(testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReadyForReview, now))
	state.Tasks[0].ReviewingBy = testhelpers.StringPtr("missing-reviewer")
	state.Tasks[0].ReviewLeaseExpires = testhelpers.TimePtr(now.Add(-time.Hour))

	err := validateAgentInvariants(state, "", true, io.Discard, resolver)
	if err != nil {
		t.Fatalf("validateAgentInvariants() error = %v, want nil for non-reviewing orphan reviewing_by", err)
	}
}

func TestValidateAgentInvariants_ActiveDoerOwnership(t *testing.T) {
	resolver := loadTestResolver(t)
	now := time.Now().UTC()

	tests := []struct {
		name        string
		mutateState func(*models.State, string)
		wantErr     string
	}{
		{
			name: "valid working ownership",
		},
		{
			name: "valid resumable owned task",
			mutateState: func(state *models.State, doerID string) {
				agent := state.Agents[doerID]
				agent.Status = models.AgentStatusIdle
				agent.CurrentTask = nil
				state.Agents[doerID] = agent
			},
		},
		{
			name: "valid handoff ownership",
			mutateState: func(state *models.State, doerID string) {
				state.Tasks[0].HandoffPending = true
				agent := state.Agents[doerID]
				agent.Status = models.AgentStatusHandoff
				agent.CurrentTask = testhelpers.StringPtr(state.Tasks[0].ID)
				state.Agents[doerID] = agent
			},
		},
		{
			name: "handoff ownership missing provider",
			mutateState: func(state *models.State, doerID string) {
				state.Tasks[0].HandoffPending = true
				agent := state.Agents[doerID]
				agent.Status = models.AgentStatusHandoff
				agent.CurrentTask = testhelpers.StringPtr(state.Tasks[0].ID)
				agent.Provider = ""
				state.Agents[doerID] = agent
			},
			wantErr: "active lease but no provider",
		},
		{
			name: "missing doer agent",
			mutateState: func(state *models.State, doerID string) {
				delete(state.Agents, doerID)
			},
			wantErr: "assigned_to coder-1 has no matching agent",
		},
		{
			name: "wrong doer role",
			mutateState: func(state *models.State, doerID string) {
				agent := state.Agents[doerID]
				agent.Role = models.RoleCodePlanner
				state.Agents[doerID] = agent
			},
			wantErr: `has role "code-planner", want "coder"`,
		},
		{
			name: "invalid doer status",
			mutateState: func(state *models.State, doerID string) {
				agent := state.Agents[doerID]
				agent.Status = models.AgentStatusWaiting
				state.Agents[doerID] = agent
			},
			wantErr: "has agent status WAITING, want WORKING or resumable IDLE",
		},
		{
			name: "mismatched current task",
			mutateState: func(state *models.State, doerID string) {
				agent := state.Agents[doerID]
				agent.CurrentTask = testhelpers.StringPtr("other-task")
				state.Agents[doerID] = agent
			},
			wantErr: "has mismatched current_task",
		},
		{
			name: "working agent with empty current task",
			mutateState: func(state *models.State, doerID string) {
				agent := state.Agents[doerID]
				agent.CurrentTask = testhelpers.StringPtr("")
				state.Agents[doerID] = agent
			},
			wantErr: "WORKING but no current_task",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state, doerID := activeDoerOwnershipState(now)
			if tt.mutateState != nil {
				tt.mutateState(state, doerID)
			}

			err := validateAgentInvariants(state, "", true, io.Discard, resolver)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("validateAgentInvariants() error = %v, want nil", err)
				}
				return
			}
			assertErrorContains(t, err, tt.wantErr)
		})
	}
}

func TestValidateAgentInvariants_ReverseActiveOwnership(t *testing.T) {
	resolver := loadTestResolver(t)
	now := time.Now().UTC()

	t.Run("working agent points at task that does not point back", func(t *testing.T) {
		state, _ := activeDoerOwnershipState(now)
		state.Tasks[0].AssignedTo = testhelpers.StringPtr("coder-2")
		state.Agents["coder-2"] = models.Agent{
			Role:         models.RoleCoder,
			Status:       models.AgentStatusIdle,
			LeaseExpires: testhelpers.TimePtr(now.Add(30 * time.Minute)),
			Heartbeat:    now,
			Terminal:     "test",
			Provider:     "test",
			PID:          os.Getpid(),
		}

		err := validateAgentInvariants(state, "", true, io.Discard, resolver)
		assertErrorContains(t, err, "agent coder-1 says WORKING task-1, but task assigned_to is coder-2")

	})

	t.Run("reviewing agent points at non-reviewing task", func(t *testing.T) {
		state, _ := activeReviewOwnershipState(now)
		state.Tasks[0].Status = models.TaskStatusReadyForReview

		err := validateAgentInvariants(state, "", true, io.Discard, resolver)
		assertErrorContains(t, err, "agent code-reviewer-1 says REVIEWING task-1, but task status CODE_READY_FOR_REVIEW is not active review")

	})
}

func activeReviewOwnershipState(now time.Time) (*models.State, string) {
	task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReviewing, now)
	reviewerID := *task.ReviewingBy
	leaseExpires := now.Add(30 * time.Minute)
	state := stateWithTasks(task)
	state.Agents[reviewerID] = models.Agent{
		Role:         models.RoleCodeReviewer,
		Status:       models.AgentStatusReviewing,
		CurrentTask:  testhelpers.StringPtr(task.ID),
		LeaseExpires: &leaseExpires,
		Heartbeat:    now,
		Terminal:     "test",
		Provider:     "test",
		PID:          os.Getpid(),
	}
	return state, reviewerID
}

func activeDoerOwnershipState(now time.Time) (*models.State, string) {
	task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusImplementing, now)
	doerID := *task.AssignedTo
	leaseExpires := now.Add(30 * time.Minute)
	state := stateWithTasks(task)
	state.Agents[doerID] = models.Agent{
		Role:         models.RoleCoder,
		Status:       models.AgentStatusWorking,
		CurrentTask:  testhelpers.StringPtr(task.ID),
		LeaseExpires: &leaseExpires,
		Heartbeat:    now,
		Terminal:     "test",
		Provider:     "test",
		PID:          os.Getpid(),
	}
	return state, doerID
}

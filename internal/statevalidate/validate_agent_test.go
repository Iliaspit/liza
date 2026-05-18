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

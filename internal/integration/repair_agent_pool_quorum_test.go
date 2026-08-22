package integration

import (
	"io"
	"slices"
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/brand"
	"github.com/liza-mas/liza/internal/commands"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/statevalidate"
	"github.com/liza-mas/liza/internal/testhelpers"
)

const (
	quorumTaskID       = "review-deadlock"
	quorumRolePair     = "quorum-pair"
	quorumDoerRole     = "quorum-coder"
	quorumReviewerRole = "quorum-reviewer"
	providerA          = "provider-a"
	providerB          = "provider-b"
)

func TestRepairAgentPoolQuorumClaimEligibility(t *testing.T) {
	t.Run("reports missing reviewer capacity for the deadlocked roster", func(t *testing.T) {
		projectRoot := writeQuorumRepairProject(t, false)

		result, err := commands.RepairAgentPool(commands.RepairAgentPoolOptions{
			ProjectRoot: projectRoot,
			CLI:         "codex",
			DryRun:      true,
		})
		if err != nil {
			t.Fatalf("RepairAgentPool() error = %v", err)
		}
		if len(result.Missing) != 1 {
			t.Fatalf("missing = %+v, want one reviewer role", result.Missing)
		}
		missing := result.Missing[0]
		if missing.Role != quorumReviewerRole || !slices.Equal(missing.TaskIDs, []string{quorumTaskID}) || missing.TaskCount != 1 {
			t.Fatalf("missing = %+v, want %s for %s", missing, quorumReviewerRole, quorumTaskID)
		}
		wantCommand := brand.Command("agent", quorumReviewerRole) + " --cli codex"
		if !slices.Equal(result.Commands, []string{wantCommand}) {
			t.Fatalf("commands = %v, want [%s]", result.Commands, wantCommand)
		}
		if len(result.Spawned) != 0 {
			t.Fatalf("dry run spawned agents: %+v", result.Spawned)
		}
	})

	t.Run("claim-eligible provider B reviewer suppresses repair", func(t *testing.T) {
		projectRoot := writeQuorumRepairProject(t, true)

		result, err := commands.RepairAgentPool(commands.RepairAgentPoolOptions{
			ProjectRoot: projectRoot,
			CLI:         "codex",
			DryRun:      true,
		})
		if err != nil {
			t.Fatalf("RepairAgentPool() error = %v", err)
		}
		if len(result.Missing) != 0 {
			t.Fatalf("missing = %+v, want no missing reviewer capacity", result.Missing)
		}
		if len(result.Commands) != 0 {
			t.Fatalf("commands = %v, want none", result.Commands)
		}
	})
}

func writeQuorumRepairProject(t *testing.T, addEligibleReviewer bool) string {
	t.Helper()

	projectRoot := t.TempDir()
	statePath, _ := testhelpers.SetupLizaDir(t, projectRoot)
	testhelpers.SetupPipelineConfigBytes(t, projectRoot, []byte(quorumRepairPipeline))
	testhelpers.CreateSpecFile(t, projectRoot, "vision.md", "# Vision\n")

	now := time.Now().UTC()
	doerID := quorumDoerRole + "-1"
	priorApproverID := quorumReviewerRole + "-1"
	task := testhelpers.BuildTaskByStatus(quorumTaskID, models.TaskStatusPartiallyApproved, now)
	task.RolePair = quorumRolePair
	task.AssignedTo = testhelpers.StringPtr(doerID)
	task.ReviewCommit = testhelpers.StringPtr("review123")
	task.SpecRef = "specs/vision.md"
	task.Approvals = []models.Approval{{
		Agent:     priorApproverID,
		Provider:  providerB,
		Timestamp: now,
	}}
	task.HandoffEvents[0].Agent = doerID

	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{task}
	state.Sprint.Scope.Planned = []string{task.ID}
	state.Agents = map[string]models.Agent{
		doerID:                    quorumRepairAgent(quorumDoerRole, providerA),
		priorApproverID:           quorumRepairAgent(quorumReviewerRole, providerB),
		quorumReviewerRole + "-2": quorumRepairAgent(quorumReviewerRole, providerA),
	}
	if addEligibleReviewer {
		state.Agents[quorumReviewerRole+"-3"] = quorumRepairAgent(quorumReviewerRole, providerB)
	}

	testhelpers.WriteInitialState(t, statePath, state)
	if err := statevalidate.ValidateStateFile(statePath, false, io.Discard); err != nil {
		t.Fatalf("fixture state validation failed: %v", err)
	}
	return projectRoot
}

func quorumRepairAgent(role, provider string) models.Agent {
	agent := testhelpers.RegisteredTestAgent(role)
	agent.Provider = provider
	return agent
}

const quorumRepairPipeline = `pipeline:
  roles:
    quorum-coder:
      type: doer
      display-name: "Quorum Coder"
    quorum-reviewer:
      type: reviewer
      display-name: "Quorum Reviewer"
  role-pairs:
    quorum-pair:
      doer: quorum-coder
      reviewer: quorum-reviewer
      review-policy:
        quorum: 2
        provider-diversity: preferred
      states:
        initial: DRAFT_CODE
        executing: IMPLEMENTING_CODE
        submitted: CODE_TO_REVIEW
        reviewing: REVIEWING_CODE
        approved: CODE_APPROVED
        rejected: CODE_REJECTED
        partially-approved: CODE_PARTIALLY_APPROVED
        reviewing-2: REVIEWING_CODE_2
  sub-pipelines:
    quorum-subpipeline:
      steps:
        - quorum-pair
  entry-points:
    default: quorum-subpipeline.quorum-pair
`

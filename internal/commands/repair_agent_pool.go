package commands

import (
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/liza-mas/liza/internal/agent"
	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/ops"
	"github.com/liza-mas/liza/internal/paths"
	"github.com/liza-mas/liza/internal/process"
)

type RepairAgentPoolOptions struct {
	ProjectRoot string
	Missing     bool
	CLI         string
	DryRun      bool
}

type MissingRoleWork struct {
	Role      string   `json:"role"`
	TaskIDs   []string `json:"task_ids"`
	TaskCount int      `json:"task_count"`
}

type SpawnedAgent struct {
	Role    string `json:"role"`
	CLI     string `json:"cli"`
	Command string `json:"command"`
	PID     int    `json:"pid,omitempty"`
}

type FailedAgentSpawn struct {
	Role    string `json:"role"`
	CLI     string `json:"cli"`
	Command string `json:"command"`
	Error   string `json:"error"`
}

type RepairAgentPoolResult struct {
	CLI      string             `json:"cli"`
	DryRun   bool               `json:"dry_run"`
	Missing  []MissingRoleWork  `json:"missing"`
	Spawned  []SpawnedAgent     `json:"spawned,omitempty"`
	Failed   []FailedAgentSpawn `json:"failed,omitempty"`
	Commands []string           `json:"commands,omitempty"`
}

var repairAgentPoolSpawn = func(projectRoot, role, cli string) (int, error) {
	cmd, err := process.SpawnAgent(projectRoot, role, cli)
	if err != nil {
		return 0, err
	}
	if cmd.Process == nil {
		return 0, nil
	}
	return cmd.Process.Pid, nil
}

func RepairAgentPoolCommand(opts RepairAgentPoolOptions) error {
	result, err := RepairAgentPool(opts)
	if result != nil {
		printRepairAgentPoolResult(result)
	}
	return err
}

func RepairAgentPool(opts RepairAgentPoolOptions) (*RepairAgentPoolResult, error) {
	statePath := paths.New(opts.ProjectRoot).StatePath()
	state, err := db.For(statePath).Read()
	if err != nil {
		return nil, fmt.Errorf("failed to read state: %w", err)
	}

	pr, err := ops.LoadResolverForModels(opts.ProjectRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to load pipeline resolver: %w", err)
	}

	cliName := opts.CLI
	if cliName == "" {
		cliName = agent.ResolveDefaultCLI(state.Config.DefaultCLI)
	}
	if !slices.Contains(agent.ValidCLIs(), cliName) {
		return nil, fmt.Errorf("invalid CLI: %s (must be %s)", cliName, strings.Join(agent.ValidCLIs(), ", "))
	}

	missing := FindMissingRolesWithClaimableWork(state, pr)
	result := &RepairAgentPoolResult{
		CLI:     cliName,
		DryRun:  opts.DryRun,
		Missing: missing,
	}

	for _, roleWork := range missing {
		spawn := SpawnedAgent{
			Role:    roleWork.Role,
			CLI:     cliName,
			Command: fmt.Sprintf("liza agent %s --cli %s", roleWork.Role, cliName),
		}
		result.Commands = append(result.Commands, spawn.Command)

		if opts.DryRun {
			continue
		}

		pid, err := repairAgentPoolSpawn(opts.ProjectRoot, roleWork.Role, cliName)
		if err != nil {
			result.Failed = append(result.Failed, FailedAgentSpawn{
				Role:    roleWork.Role,
				CLI:     cliName,
				Command: spawn.Command,
				Error:   err.Error(),
			})
			continue
		}
		spawn.PID = pid
		result.Spawned = append(result.Spawned, spawn)
	}

	if len(result.Failed) > 0 {
		return result, repairAgentPoolSpawnError(result)
	}

	return result, nil
}

func repairAgentPoolSpawnError(result *RepairAgentPoolResult) error {
	failures := make([]string, 0, len(result.Failed))
	for _, failure := range result.Failed {
		failures = append(failures, fmt.Sprintf("%s: %s", failure.Role, failure.Error))
	}
	return fmt.Errorf("failed to start %d of %d missing role agent(s): %s",
		len(result.Failed), len(result.Missing), strings.Join(failures, "; "))
}

func FindMissingRolesWithClaimableWork(state *models.State, pr models.PipelineResolver) []MissingRoleWork {
	if state == nil || pr == nil {
		return nil
	}

	registeredRoles := make(map[string]bool)
	now := time.Now()
	nilLeaseHeartbeatWindow := models.NormalizeHeartbeatInterval(state.Config.HeartbeatInterval) + models.LeaseExpiryGracePeriod
	for _, agentState := range state.Agents {
		if agentHasLiveRegistration(agentState, now, nilLeaseHeartbeatWindow) {
			registeredRoles[agentState.Role] = true
		}
	}

	missingRoleTasks := make(map[string][]string)
	for i := range state.Tasks {
		task := &state.Tasks[i]
		if task.Status.IsTerminal() || task.RolePair == "" {
			continue
		}

		doerRuntime, err := pr.DoerRole(task.RolePair)
		if err != nil {
			continue
		}
		reviewerRuntime, err := pr.ReviewerRole(task.RolePair)
		if err != nil {
			continue
		}

		if !registeredRoles[doerRuntime] && task.IsClaimable(doerRuntime, state.Tasks, pr) {
			missingRoleTasks[doerRuntime] = append(missingRoleTasks[doerRuntime], task.ID)
		}
		if !registeredRoles[reviewerRuntime] && task.IsClaimable(reviewerRuntime, state.Tasks, pr) {
			missingRoleTasks[reviewerRuntime] = append(missingRoleTasks[reviewerRuntime], task.ID)
		}
	}

	roles := make([]string, 0, len(missingRoleTasks))
	for role := range missingRoleTasks {
		roles = append(roles, role)
	}
	sort.Strings(roles)

	missing := make([]MissingRoleWork, 0, len(roles))
	for _, role := range roles {
		taskIDs := missingRoleTasks[role]
		sort.Strings(taskIDs)
		missing = append(missing, MissingRoleWork{
			Role:      role,
			TaskIDs:   taskIDs,
			TaskCount: len(taskIDs),
		})
	}
	return missing
}

func agentHasLiveRegistration(agentState models.Agent, now time.Time, nilLeaseHeartbeatWindow time.Duration) bool {
	if agentState.Role == "" {
		return false
	}
	if agentState.LeaseExpires != nil {
		return agentState.LeaseExpires.After(now)
	}
	if agentState.Heartbeat.IsZero() {
		return false
	}
	return agentState.Heartbeat.After(now.Add(-nilLeaseHeartbeatWindow))
}

func printRepairAgentPoolResult(result *RepairAgentPoolResult) {
	if len(result.Missing) == 0 {
		fmt.Println("No missing roles with claimable work.")
		return
	}

	fmt.Println("Missing roles with claimable work:")
	for _, roleWork := range result.Missing {
		fmt.Printf("  %s: %d task(s) (%s)\n", roleWork.Role, roleWork.TaskCount, strings.Join(roleWork.TaskIDs, ", "))
	}

	if result.DryRun {
		fmt.Println()
		fmt.Println("Dry run; would spawn:")
		for _, command := range result.Commands {
			fmt.Printf("  %s\n", command)
		}
		return
	}

	fmt.Println()
	if len(result.Spawned) > 0 {
		fmt.Println("Started agent processes:")
		for _, spawned := range result.Spawned {
			if spawned.PID != 0 {
				fmt.Printf("  %s (pid %d)\n", spawned.Command, spawned.PID)
				continue
			}
			fmt.Printf("  %s\n", spawned.Command)
		}
	}
	if len(result.Failed) > 0 {
		fmt.Println("Failed to start agent processes:")
		for _, failed := range result.Failed {
			fmt.Printf("  %s: %s\n", failed.Command, failed.Error)
		}
	}
}

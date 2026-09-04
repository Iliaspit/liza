package ops

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"reflect"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/log"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/paths"
	"github.com/liza-mas/liza/internal/statevalidate"
)

const (
	requestGraphReplanOperation  = "request-graph-replan"
	refreshGraphReplanOperation  = "refresh-graph-replan"
	claimGraphReplanOperation    = "claim-graph-replan"
	completeGraphReplanOperation = "complete-graph-replan"
)

type RequestGraphReplanInput struct {
	RunID       string
	RequestedBy string
	Reason      string
}

type RequestGraphReplanResult struct {
	Request    models.GraphReplanRequest `json:"request"`
	Idempotent bool                      `json:"idempotent"`
}

type RefreshGraphReplanInput struct {
	RequestID   string
	RunID       string
	RequestedBy string
	Reason      string
}

type RefreshGraphReplanResult struct {
	Superseded models.GraphReplanRequest `json:"superseded"`
	Request    models.GraphReplanRequest `json:"request"`
	Idempotent bool                      `json:"idempotent"`
}

type GraphDependencyUpdate struct {
	TaskID            string                      `json:"task_id"`
	ExpectedDependsOn []string                    `json:"expected_depends_on"`
	DesiredDependsOn  []string                    `json:"desired_depends_on"`
	ExpectedContracts []models.DependencyContract `json:"expected_dependency_contracts"`
	DesiredContracts  []models.DependencyContract `json:"desired_dependency_contracts"`
}

type CompleteGraphReplanInput struct {
	RequestID string
	Diagnosis string
	Updates   []GraphDependencyUpdate
}

type CompleteGraphReplanResult struct {
	Request models.GraphReplanRequest `json:"request"`
}

// RequestGraphReplan records a controller request without exposing any task or
// graph mutation. The Lisa goal ID is the public run identity.
func RequestGraphReplan(projectRoot string, input RequestGraphReplanInput) (*RequestGraphReplanResult, error) {
	input.RunID = strings.TrimSpace(input.RunID)
	input.RequestedBy = strings.TrimSpace(input.RequestedBy)
	input.Reason = strings.TrimSpace(input.Reason)
	if input.RunID == "" || input.RequestedBy == "" || input.Reason == "" {
		return nil, &PreconditionError{Reason: "run_id, requested_by, and reason are required"}
	}
	resolver, _, err := loadResolver(projectRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to load pipeline config: %w", err)
	}
	lp := paths.New(projectRoot)
	bb := db.For(lp.StatePath())
	var result RequestGraphReplanResult
	now := time.Now().UTC()
	err = bb.Modify(func(state *models.State) error {
		if input.RunID != state.Goal.ID {
			return &PreconditionError{Reason: fmt.Sprintf("run identity mismatch: got %q, want Lisa goal %q", input.RunID, state.Goal.ID)}
		}
		taskIDs, edges, generation := statevalidate.DependencyGraphSnapshot(state)
		scopeFingerprint := statevalidate.ScopeContractFingerprint(state)
		candidateFingerprint := statevalidate.CandidateLineageFingerprint(state)
		requestID := graphReplanRequestID(input.RunID, generation, scopeFingerprint, candidateFingerprint)
		for i := range state.GraphReplanRequests {
			existing := &state.GraphReplanRequests[i]
			if existing.ID == requestID {
				if existing.RunID != input.RunID || existing.RequestedBy != input.RequestedBy || existing.Reason != input.Reason {
					return &PreconditionError{Reason: fmt.Sprintf("graph re-plan request %s already exists for a different requester or reason", existing.ID)}
				}
				result = RequestGraphReplanResult{Request: *existing, Idempotent: true}
				return nil
			}
			if existing.Status == models.GraphReplanPending || existing.Status == models.GraphReplanRepairing {
				return &PreconditionError{Reason: fmt.Sprintf("graph re-plan request %s already owns run %s", existing.ID, existing.RunID)}
			}
		}
		graphErr := statevalidate.ValidateDependencyGraph(state, resolver)
		if graphErr == nil {
			stallDiagnostic := dependencyStallDiagnostic(state, resolver)
			if stallDiagnostic == "" {
				return &PreconditionError{Reason: "dependency graph is valid; a re-plan request requires a proven graph diagnostic or a fully dependency-stalled run"}
			}
			graphErr = fmt.Errorf("%s", stallDiagnostic)
		}
		request := newGraphReplanRequest(input.RunID, input.RequestedBy, input.Reason, graphErr.Error(), generation, scopeFingerprint, candidateFingerprint, taskIDs, edges, now)
		state.GraphReplanRequests = append(state.GraphReplanRequests, request)
		result = RequestGraphReplanResult{Request: request}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("request graph re-plan: %w", err)
	}
	_ = log.New(lp.LogPath()).Append(log.Entry{Timestamp: now, Agent: input.RequestedBy, Action: requestGraphReplanOperation, Detail: result.Request.ID + ": " + input.Reason})
	return &result, nil
}

// RefreshGraphReplan atomically supersedes one stale request and creates its
// current-state successor. It changes no task or dependency and is safe to
// replay after an interrupted controller call.
func RefreshGraphReplan(projectRoot string, input RefreshGraphReplanInput) (*RefreshGraphReplanResult, error) {
	input.RequestID = strings.TrimSpace(input.RequestID)
	input.RunID = strings.TrimSpace(input.RunID)
	input.RequestedBy = strings.TrimSpace(input.RequestedBy)
	input.Reason = strings.TrimSpace(input.Reason)
	if input.RequestID == "" || input.RunID == "" || input.RequestedBy == "" || input.Reason == "" {
		return nil, &PreconditionError{Reason: "request_id, run_id, requested_by, and reason are required"}
	}
	resolver, _, err := loadResolver(projectRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to load pipeline config: %w", err)
	}
	lp := paths.New(projectRoot)
	bb := db.For(lp.StatePath())
	var result RefreshGraphReplanResult
	now := time.Now().UTC()
	err = bb.Modify(func(state *models.State) error {
		if input.RunID != state.Goal.ID {
			return &PreconditionError{Reason: fmt.Sprintf("run identity mismatch: got %q, want Lisa goal %q", input.RunID, state.Goal.ID)}
		}
		request := findGraphReplanRequest(state, input.RequestID)
		if request == nil {
			return &PreconditionError{Reason: fmt.Sprintf("graph re-plan request %s not found", input.RequestID)}
		}
		if request.Status == models.GraphReplanSuperseded {
			successor := findGraphReplanRequest(state, request.SuccessorRequestID)
			if successor == nil || successor.PredecessorRequestID != request.ID || successor.RequestedBy != input.RequestedBy || successor.Reason != input.Reason {
				return &PreconditionError{Reason: "superseded graph re-plan request does not match this refresh"}
			}
			result = RefreshGraphReplanResult{Superseded: *request, Request: *successor, Idempotent: true}
			return nil
		}
		if request.Status != models.GraphReplanPending && request.Status != models.GraphReplanRepairing {
			return &PreconditionError{Reason: fmt.Sprintf("graph re-plan request %s is %s", input.RequestID, request.Status)}
		}
		if request.RunID != input.RunID || request.RequestedBy != input.RequestedBy {
			return &PreconditionError{Reason: "graph re-plan request ownership does not match this refresh"}
		}
		if open := state.OpenGraphReplanRequest(); open == nil || open.ID != request.ID {
			return &PreconditionError{Reason: "graph re-plan request is not the run's current open request"}
		}
		if active := activeNonOrchestratorOwnership(state); len(active) > 0 {
			return &PreconditionError{Reason: "cannot refresh graph re-plan while worker ownership is active: " + strings.Join(active, ", ")}
		}

		taskIDs, edges, generation := statevalidate.DependencyGraphSnapshot(state)
		scopeFingerprint := statevalidate.ScopeContractFingerprint(state)
		candidateFingerprint := statevalidate.CandidateLineageFingerprint(state)
		if scopeFingerprint != request.ScopeFingerprint {
			return &PreconditionError{Reason: "scope or acceptance changed since the graph re-plan request; controller refresh is forbidden"}
		}
		if generation == request.GraphGeneration && candidateFingerprint == request.CandidateFingerprint {
			return &PreconditionError{Reason: "graph re-plan request is still current and must be completed, not refreshed"}
		}
		graphErr := statevalidate.ValidateDependencyGraph(state, resolver)
		if graphErr == nil {
			stallDiagnostic := dependencyStallDiagnostic(state, resolver)
			if stallDiagnostic == "" {
				return &PreconditionError{Reason: "current state no longer proves a dependency graph fault or full dependency stall"}
			}
			graphErr = fmt.Errorf("%s", stallDiagnostic)
		}
		requestID := graphReplanRequestID(input.RunID, generation, scopeFingerprint, candidateFingerprint)
		if findGraphReplanRequest(state, requestID) != nil {
			return &PreconditionError{Reason: fmt.Sprintf("current graph re-plan request %s already exists", requestID)}
		}
		successor := newGraphReplanRequest(input.RunID, input.RequestedBy, input.Reason, graphErr.Error(), generation, scopeFingerprint, candidateFingerprint, taskIDs, edges, now)
		successor.PredecessorRequestID = request.ID
		request.Status = models.GraphReplanSuperseded
		request.SupersededAt = &now
		request.SuccessorRequestID = successor.ID
		state.GraphReplanRequests = append(state.GraphReplanRequests, successor)
		result = RefreshGraphReplanResult{Superseded: *request, Request: successor}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("refresh graph re-plan: %w", err)
	}
	_ = log.New(lp.LogPath()).Append(log.Entry{Timestamp: now, Agent: input.RequestedBy, Action: refreshGraphReplanOperation, Detail: input.RequestID + " -> " + result.Request.ID + ": " + input.Reason})
	return &result, nil
}

// dependencyStallDiagnostic admits a native orchestrator re-plan only when
// durable state proves that all remaining allocation is dependency-stalled.
// This covers semantic deadlocks recorded by BLOCKED tasks before a missing
// provider can be represented as a concrete graph edge.
func dependencyStallDiagnostic(state *models.State, resolver models.PipelineResolver) string {
	readiness := models.GetTaskReadiness(state, resolver)
	if readiness.Claimable != 0 || readiness.Reviewable != 0 || len(activeNonOrchestratorOwnership(state)) != 0 {
		return ""
	}

	dependencyResolver := models.NewDependencyResolver(state)
	blockedTasks := make([]string, 0)
	dependencyHeld := make([]string, 0)
	for i := range state.Tasks {
		task := &state.Tasks[i]
		if task.Status == models.TaskStatusBlocked && task.BlockedReason != nil && strings.TrimSpace(*task.BlockedReason) != "" {
			blockedTasks = append(blockedTasks, task.ID)
		}
		if models.BlockedByDependencies(task, resolver, dependencyResolver) {
			dependencyHeld = append(dependencyHeld, task.ID)
		}
	}
	if len(blockedTasks) == 0 || len(dependencyHeld) == 0 {
		return ""
	}
	sort.Strings(blockedTasks)
	sort.Strings(dependencyHeld)
	heldCount := len(dependencyHeld)
	const heldSampleLimit = 8
	if len(dependencyHeld) > heldSampleLimit {
		dependencyHeld = dependencyHeld[:heldSampleLimit]
	}
	return fmt.Sprintf(
		"structurally valid dependency graph has no claimable or reviewable work; blocked tasks with durable reasons: %s; %d dependency-held tasks (sample: %s)",
		strings.Join(blockedTasks, ", "), heldCount, strings.Join(dependencyHeld, ", "),
	)
}

// ClaimGraphReplan binds the request to the one registered orchestrator and
// freezes the diagnosed graph generation before any native repair operation.
func ClaimGraphReplan(projectRoot, requestID string, authority models.AgentAuthority) (*models.GraphReplanRequest, error) {
	if requestID == "" {
		return nil, &PreconditionError{Reason: "request ID is required"}
	}
	lp := paths.New(projectRoot)
	bb := db.For(lp.StatePath())
	var result models.GraphReplanRequest
	now := time.Now().UTC()
	err := lifecycleMutation(bb, &authority)(func(state *models.State) error {
		request := findGraphReplanRequest(state, requestID)
		if request == nil {
			return &PreconditionError{Reason: fmt.Sprintf("graph re-plan request %s not found", requestID)}
		}
		if request.Status == models.GraphReplanRepairing && request.Orchestrator != nil && *request.Orchestrator == authority {
			result = *request
			return nil
		}
		if request.Status != models.GraphReplanPending {
			return &PreconditionError{Reason: fmt.Sprintf("graph re-plan request %s is %s", requestID, request.Status)}
		}
		orchestratorID, err := state.FindOrchestratorID()
		if err != nil || orchestratorID != authority.ID {
			return &PreconditionError{Reason: fmt.Sprintf("ambiguous orchestrator ownership: %v", err)}
		}
		_, _, generation := statevalidate.DependencyGraphSnapshot(state)
		if generation != request.GraphGeneration {
			return &PreconditionError{Reason: fmt.Sprintf("dependency graph changed since request %s: got %s, want %s", requestID, generation, request.GraphGeneration)}
		}
		if statevalidate.ScopeContractFingerprint(state) != request.ScopeFingerprint || statevalidate.CandidateLineageFingerprint(state) != request.CandidateFingerprint {
			return &PreconditionError{Reason: "scope, acceptance, or candidate lineage changed since the graph re-plan request"}
		}
		if active := activeNonOrchestratorOwnership(state); len(active) > 0 {
			return &PreconditionError{Reason: "cannot claim graph re-plan while worker ownership is active: " + strings.Join(active, ", ")}
		}
		request.Status = models.GraphReplanRepairing
		owner := authority
		request.Orchestrator = &owner
		request.StartedAt = &now
		result = *request
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("claim graph re-plan: %w", err)
	}
	_ = log.New(lp.LogPath()).Append(log.Entry{Timestamp: now, Agent: authority.ID, Action: claimGraphReplanOperation, Detail: requestID})
	return &result, nil
}

// CompleteGraphReplan applies an optional atomic edge-only correction and
// closes the request only after native validation proves the graph safe and
// scope, acceptance, and candidate lineage unchanged.
func CompleteGraphReplan(projectRoot string, input CompleteGraphReplanInput, authority models.AgentAuthority) (*CompleteGraphReplanResult, error) {
	if input.RequestID == "" || strings.TrimSpace(input.Diagnosis) == "" {
		return nil, &PreconditionError{Reason: "request ID and diagnosis are required"}
	}
	resolver, _, err := loadResolver(projectRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to load pipeline config: %w", err)
	}
	lp := paths.New(projectRoot)
	bb := db.For(lp.StatePath())
	var result CompleteGraphReplanResult
	now := time.Now().UTC()
	err = lifecycleMutation(bb, &authority)(func(state *models.State) error {
		request := findGraphReplanRequest(state, input.RequestID)
		if request == nil {
			return &PreconditionError{Reason: fmt.Sprintf("graph re-plan request %s not found", input.RequestID)}
		}
		if request.Status == models.GraphReplanCompleted {
			result.Request = *request
			return nil
		}
		if request.Status != models.GraphReplanRepairing || request.Orchestrator == nil || *request.Orchestrator != authority {
			return &PreconditionError{Reason: "graph re-plan request is not owned by this orchestrator generation"}
		}
		if statevalidate.ScopeContractFingerprint(state) != request.ScopeFingerprint || statevalidate.CandidateLineageFingerprint(state) != request.CandidateFingerprint {
			return &PreconditionError{Reason: "graph repair would cross the frozen scope, acceptance, or candidate-lineage boundary"}
		}
		for _, update := range input.Updates {
			task := state.FindTask(update.TaskID)
			if task == nil {
				return &PreconditionError{Reason: fmt.Sprintf("dependency update task %s does not exist", update.TaskID)}
			}
			if task.Status.IsTerminal() {
				return &PreconditionError{Reason: fmt.Sprintf("dependency update task %s is terminal", update.TaskID)}
			}
			if !slices.Equal(task.DependsOn, update.ExpectedDependsOn) || !reflect.DeepEqual(task.DependencyContracts, update.ExpectedContracts) {
				return &PreconditionError{Reason: fmt.Sprintf("dependency update for %s is stale", update.TaskID)}
			}
			canonical, _, err := canonicalizeConcreteDependencyList(state, resolver, task.ID, task.RolePair, update.DesiredDependsOn)
			if err != nil {
				return err
			}
			task.DependsOn = canonical
			task.DependencyContracts = slices.Clone(update.DesiredContracts)
		}
		if statevalidate.ScopeContractFingerprint(state) != request.ScopeFingerprint || statevalidate.CandidateLineageFingerprint(state) != request.CandidateFingerprint {
			return &PreconditionError{Reason: "graph repair changed scope, acceptance, or candidate lineage"}
		}
		if err := statevalidate.ValidateDependencyGraph(state, resolver); err != nil {
			return &PreconditionError{Reason: err.Error()}
		}
		if err := statevalidate.ValidateState(state, projectRoot, false, io.Discard); err != nil {
			return err
		}
		currentTaskIDs, currentEdges, resultGeneration := statevalidate.DependencyGraphSnapshot(state)
		changes := dependencyGraphChanges(request.InitialTaskIDs, request.InitialEdges, currentTaskIDs, currentEdges)
		if len(changes) == 0 {
			return &PreconditionError{Reason: "graph re-plan made no auditable graph correction"}
		}
		request.Status = models.GraphReplanCompleted
		request.CompletedAt = &now
		request.ResultGeneration = resultGeneration
		request.Diagnosis = input.Diagnosis
		request.GraphChanges = changes
		request.ValidationResult = "valid"
		result.Request = *request
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("complete graph re-plan: %w", err)
	}
	_ = log.New(lp.LogPath()).Append(log.Entry{Timestamp: now, Agent: authority.ID, Action: completeGraphReplanOperation, Detail: input.RequestID + ": " + input.Diagnosis})
	return &result, nil
}

func findGraphReplanRequest(state *models.State, requestID string) *models.GraphReplanRequest {
	for i := range state.GraphReplanRequests {
		if state.GraphReplanRequests[i].ID == requestID {
			return &state.GraphReplanRequests[i]
		}
	}
	return nil
}

func newGraphReplanRequest(runID, requestedBy, reason, diagnostic, generation, scopeFingerprint, candidateFingerprint string, taskIDs []string, edges []models.DependencyGraphEdge, now time.Time) models.GraphReplanRequest {
	return models.GraphReplanRequest{
		ID: graphReplanRequestID(runID, generation, scopeFingerprint, candidateFingerprint), RunID: runID, GraphGeneration: generation,
		ScopeFingerprint: scopeFingerprint, CandidateFingerprint: candidateFingerprint,
		RequestedBy: requestedBy, Reason: reason, Diagnostic: diagnostic, RequestedAt: now, Status: models.GraphReplanPending,
		InitialTaskIDs: slices.Clone(taskIDs), InitialEdges: slices.Clone(edges),
	}
}

func graphReplanRequestID(runID, generation, scopeFingerprint, candidateFingerprint string) string {
	digest := sha256.Sum256([]byte(strings.Join([]string{runID, generation, scopeFingerprint, candidateFingerprint}, "\x00")))
	return "graph-replan-" + hex.EncodeToString(digest[:8])
}

func activeNonOrchestratorOwnership(state *models.State) []string {
	activeSet := map[string]bool{}
	for _, task := range state.Tasks {
		if task.AssignedTo != nil && *task.AssignedTo != "" {
			activeSet[task.ID+" assigned_to="+*task.AssignedTo] = true
		}
		if task.ReviewingBy != nil && *task.ReviewingBy != "" {
			activeSet[task.ID+" reviewing_by="+*task.ReviewingBy] = true
		}
	}
	for agentID, agent := range state.Agents {
		if agent.Role == "orchestrator" || agent.CurrentTask == nil || *agent.CurrentTask == "" {
			continue
		}
		activeSet["agent "+agentID+" current_task="+*agent.CurrentTask] = true
	}
	active := make([]string, 0, len(activeSet))
	for ownership := range activeSet {
		active = append(active, ownership)
	}
	sort.Strings(active)
	return active
}

func dependencyGraphChanges(beforeTasks []string, beforeEdges []models.DependencyGraphEdge, afterTasks []string, afterEdges []models.DependencyGraphEdge) []string {
	beforeTaskSet := graphReplanStringSet(beforeTasks)
	afterTaskSet := graphReplanStringSet(afterTasks)
	var changes []string
	for _, id := range afterTasks {
		if !beforeTaskSet[id] {
			changes = append(changes, "+task "+id)
		}
	}
	for _, id := range beforeTasks {
		if !afterTaskSet[id] {
			changes = append(changes, "-task "+id)
		}
	}
	beforeEdgeSet := edgeSet(beforeEdges)
	afterEdgeSet := edgeSet(afterEdges)
	for key := range afterEdgeSet {
		if !beforeEdgeSet[key] {
			changes = append(changes, "+edge "+key)
		}
	}
	for key := range beforeEdgeSet {
		if !afterEdgeSet[key] {
			changes = append(changes, "-edge "+key)
		}
	}
	sort.Strings(changes)
	return changes
}

func graphReplanStringSet(values []string) map[string]bool {
	result := make(map[string]bool, len(values))
	for _, value := range values {
		result[value] = true
	}
	return result
}

func edgeSet(edges []models.DependencyGraphEdge) map[string]bool {
	result := make(map[string]bool, len(edges))
	for _, edge := range edges {
		key := strings.Join([]string{edge.Consumer, edge.Provider, string(edge.Gate), string(edge.Severity), edge.Purpose, edge.Supplies}, "|")
		result[key] = true
	}
	return result
}

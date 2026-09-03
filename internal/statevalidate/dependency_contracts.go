package statevalidate

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/pipeline"
)

// DependencyGraphError reports every deterministic graph-contract violation
// discovered in one validation pass.
type DependencyGraphError struct {
	Issues []string
}

func (e *DependencyGraphError) Error() string {
	return "unsafe dependency graph: " + strings.Join(e.Issues, "; ")
}

func (e *DependencyGraphError) SafeDetails() map[string]any {
	return map[string]any{"issues": slices.Clone(e.Issues)}
}

// ValidateDependencyGraph validates typed dependency meaning plus the complete
// current/projected graph. Legacy runs remain compatible until they opt into
// dependency_contract_version; individual typed records are always validated.
func ValidateDependencyGraph(state *models.State, resolver *pipeline.Resolver) error {
	if state == nil {
		return &DependencyGraphError{Issues: []string{"state is required"}}
	}
	strict := state.DependencyContractVersion >= models.DependencyContractVersion

	taskIDs, edges, _ := DependencyGraphSnapshot(state)
	known := make(map[string]*models.Task, len(state.Tasks))
	for i := range state.Tasks {
		known[state.Tasks[i].ID] = &state.Tasks[i]
	}
	var issues []string
	issues = append(issues, validateGraphReplanRequests(state)...)
	for i := range state.Tasks {
		task := &state.Tasks[i]
		issues = append(issues, validateTaskDependencyContracts(task, known, strict)...)
		issues = append(issues, validateOutputDependencyContracts(task, known, strict)...)
		if strict {
			issues = append(issues, validateMaterializedChildren(state, task, resolver)...)
		}
	}
	if cycle := dependencyContractCycle(taskIDs, edges); len(cycle) > 0 {
		issues = append(issues, "circular dependency detected: "+strings.Join(cycle, " -> "))
	}
	if len(issues) == 0 {
		return nil
	}
	sort.Strings(issues)
	return &DependencyGraphError{Issues: slices.Compact(issues)}
}

func validateGraphReplanRequests(state *models.State) []string {
	seen := map[string]bool{}
	open := 0
	var issues []string
	for index, request := range state.GraphReplanRequests {
		prefix := fmt.Sprintf("graph_replan_requests[%d]", index)
		if request.ID == "" || request.RunID != state.Goal.ID || request.GraphGeneration == "" || request.ScopeFingerprint == "" || request.CandidateFingerprint == "" {
			issues = append(issues, prefix+" is missing its request, run, generation, scope, or candidate identity")
		}
		if seen[request.ID] {
			issues = append(issues, prefix+" duplicates request ID "+request.ID)
		}
		seen[request.ID] = true
		if request.RequestedBy == "" || request.Reason == "" || request.Diagnostic == "" || request.RequestedAt.IsZero() {
			issues = append(issues, prefix+" is missing requester, reason, native diagnostic, or request timestamp")
		}
		switch request.Status {
		case models.GraphReplanPending:
			open++
			if request.Orchestrator != nil || request.StartedAt != nil || request.CompletedAt != nil {
				issues = append(issues, prefix+" pending request has repair ownership or completion metadata")
			}
		case models.GraphReplanRepairing:
			open++
			if request.Orchestrator == nil || request.Orchestrator.ID == "" || request.Orchestrator.Generation == "" || request.StartedAt == nil || request.CompletedAt != nil {
				issues = append(issues, prefix+" repairing request lacks exact orchestrator ownership or has completion metadata")
			}
		case models.GraphReplanCompleted:
			if request.Orchestrator == nil || request.StartedAt == nil || request.CompletedAt == nil || request.ResultGeneration == "" || request.Diagnosis == "" || len(request.GraphChanges) == 0 || request.ValidationResult != "valid" {
				issues = append(issues, prefix+" completed request lacks ownership, diagnosis, graph changes, result generation, or valid result")
			}
		default:
			issues = append(issues, prefix+" has invalid status "+string(request.Status))
		}
	}
	if open > 1 {
		issues = append(issues, fmt.Sprintf("%d graph re-plan requests are open; want at most 1", open))
	}
	return issues
}

// ValidateApprovalDependencies enforces the later lifecycle gate without
// turning it into an implementation-start dependency.
func ValidateApprovalDependencies(state *models.State, task *models.Task) error {
	if state == nil || task == nil {
		return nil
	}
	var unmet []string
	for _, contract := range task.DependencyContracts {
		if contract.Gate != models.DependencyGateBeforeApproval {
			continue
		}
		result := state.ResolveDependency(contract.ProviderTask)
		if !result.Satisfied() {
			unmet = append(unmet, fmt.Sprintf("%s (%s; supplies %s)", contract.ProviderTask, contract.Severity, contract.Supplies))
		}
	}
	if len(unmet) > 0 {
		sort.Strings(unmet)
		return &DependencyGraphError{Issues: []string{fmt.Sprintf("task %s cannot be approved or merged before providers are satisfied: %s", task.ID, strings.Join(unmet, ", "))}}
	}
	return nil
}

func validateTaskDependencyContracts(task *models.Task, known map[string]*models.Task, strict bool) []string {
	consumer := task.ID
	startProviders := append([]string(nil), task.DependsOn...)
	contracts, issues := validateContracts(consumer, task.DependencyContracts, -1, known)
	if strict || len(task.DependencyContracts) > 0 {
		issues = append(issues, compareStartProviders(consumer, startProviders, contracts)...)
	} else {
		issues = append(issues, validateLegacyProviders(consumer, startProviders, known)...)
	}
	return issues
}

func validateOutputDependencyContracts(task *models.Task, known map[string]*models.Task, strict bool) []string {
	var issues []string
	for index := range task.Output {
		entry := &task.Output[index]
		consumer := outputNode(task.ID, index)
		contracts, contractIssues := validateContracts(consumer, entry.DependencyContracts, len(task.Output), known)
		issues = append(issues, contractIssues...)
		if !strict && len(entry.DependencyContracts) == 0 {
			issues = append(issues, validateLegacyProviders(consumer, entry.TaskDependsOn, known)...)
			continue
		}
		var startProviders []string
		for _, sibling := range entry.DependsOn {
			parsed, err := strconv.Atoi(sibling)
			if err == nil {
				startProviders = append(startProviders, outputNode(task.ID, parsed))
			}
		}
		startProviders = append(startProviders, entry.TaskDependsOn...)
		issues = append(issues, compareStartProviders(consumer, startProviders, contracts)...)
	}
	return issues
}

func validateLegacyProviders(consumer string, providers []string, known map[string]*models.Task) []string {
	var issues []string
	for _, provider := range providers {
		if known[provider] == nil {
			issues = append(issues, fmt.Sprintf("%s dependency provider %s does not exist", consumer, provider))
		}
	}
	return issues
}

func validateContracts(consumer string, contracts []models.DependencyContract, outputCount int, known map[string]*models.Task) ([]models.DependencyGraphEdge, []string) {
	var edges []models.DependencyGraphEdge
	var issues []string
	seen := map[string]bool{}
	for index, contract := range contracts {
		provider := strings.TrimSpace(contract.ProviderTask)
		if outputCount >= 0 && contract.ProviderOutput != nil {
			if provider != "" {
				issues = append(issues, fmt.Sprintf("%s dependency_contracts[%d] must name exactly one of provider_task or provider_output", consumer, index))
				continue
			}
			if *contract.ProviderOutput < 0 || *contract.ProviderOutput >= outputCount {
				issues = append(issues, fmt.Sprintf("%s dependency_contracts[%d] provider_output %d is outside output range", consumer, index, *contract.ProviderOutput))
				continue
			}
			parent := strings.SplitN(consumer, "#output:", 2)[0]
			provider = outputNode(parent, *contract.ProviderOutput)
		} else if contract.ProviderOutput != nil {
			issues = append(issues, fmt.Sprintf("%s dependency_contracts[%d] provider_output is only valid in planner output", consumer, index))
			continue
		}
		if provider == "" {
			issues = append(issues, fmt.Sprintf("%s dependency_contracts[%d] missing provider", consumer, index))
			continue
		}
		if provider == consumer {
			issues = append(issues, fmt.Sprintf("%s dependency_contracts[%d] references itself", consumer, index))
		}
		if seen[provider] {
			issues = append(issues, fmt.Sprintf("%s has duplicate dependency contract for provider %s", consumer, provider))
		}
		seen[provider] = true
		if contract.ProviderOutput == nil {
			providerTask := known[provider]
			if providerTask == nil {
				issues = append(issues, fmt.Sprintf("%s dependency provider %s does not exist", consumer, provider))
			} else if contract.Gate != models.DependencyGateAdvisory && providerTask.Status.IsTerminal() && providerTask.Status != models.TaskStatusMerged {
				issues = append(issues, fmt.Sprintf("%s dependency provider %s is terminal without a merged deliverable (%s)", consumer, provider, providerTask.Status))
			}
		}
		if strings.TrimSpace(contract.Purpose) == "" {
			issues = append(issues, fmt.Sprintf("%s dependency_contracts[%d] missing purpose", consumer, index))
		}
		if !contract.Gate.IsValid() {
			issues = append(issues, fmt.Sprintf("%s dependency_contracts[%d] has invalid lifecycle gate %q", consumer, index, contract.Gate))
		}
		if !contract.Severity.IsValid() {
			issues = append(issues, fmt.Sprintf("%s dependency_contracts[%d] has invalid severity %q", consumer, index, contract.Severity))
		}
		if strings.TrimSpace(contract.Supplies) == "" {
			issues = append(issues, fmt.Sprintf("%s dependency_contracts[%d] missing exact supplied artifact, contract, or acceptance check", consumer, index))
		}
		if contract.Gate == models.DependencyGateAdvisory && contract.Severity != models.DependencySeverityLow {
			issues = append(issues, fmt.Sprintf("%s dependency on %s is advisory but severity %s would overstate its lifecycle consequence", consumer, provider, contract.Severity))
		}
		if contract.Gate != models.DependencyGateAdvisory && contract.Severity == models.DependencySeverityLow {
			issues = append(issues, fmt.Sprintf("%s dependency on %s gates %s but low severity is advisory-only", consumer, provider, contract.Gate))
		}
		edges = append(edges, models.DependencyGraphEdge{
			Consumer: consumer, Provider: provider, Gate: contract.Gate,
			Severity: contract.Severity, Purpose: contract.Purpose, Supplies: contract.Supplies,
		})
	}
	return edges, issues
}

func compareStartProviders(consumer string, starts []string, contracts []models.DependencyGraphEdge) []string {
	want := normalizedStrings(starts)
	var got []string
	for _, contract := range contracts {
		if contract.Gate == models.DependencyGateBeforeStart {
			got = append(got, contract.Provider)
		}
	}
	got = normalizedStrings(got)
	if slices.Equal(want, got) {
		return nil
	}
	return []string{fmt.Sprintf("%s before_start providers %v must exactly match scheduler dependencies %v", consumer, got, want)}
}

func validateMaterializedChildren(state *models.State, task *models.Task, resolver *pipeline.Resolver) []string {
	if resolver == nil || len(task.TransitionsExecuted) == 0 {
		return nil
	}
	var issues []string
	for transitionName, executed := range task.TransitionsExecuted {
		if !executed || transitionName == "replanned" {
			continue
		}
		transition, err := resolver.Transition(transitionName)
		if err != nil {
			continue
		}
		slug := transition.TaskSlugOrName()
		var expected []string
		switch transition.Cardinality {
		case "per-subtask":
			for index := range task.Output {
				expected = append(expected, fmt.Sprintf("%s-%s-%d", task.ID, slug, index))
			}
		case "one-to-one":
			expected = append(expected, fmt.Sprintf("%s-%s", task.ID, slug))
		case "many-to-one":
			if parent := task.CohortParentID(); parent != "" {
				expected = append(expected, fmt.Sprintf("%s-%s", parent, slug))
			}
		}
		for _, childID := range expected {
			if state.FindTask(childID) != nil {
				continue
			}
			outputIndex, ok := childOutputIndex(childID)
			if !ok {
				issues = append(issues, fmt.Sprintf("task %s transition %s is recorded as executed but generated task %s is missing", task.ID, transitionName, childID))
				continue
			}
			if remappedTo, skipped := skippedTransitionOutput(task, transitionName, outputIndex); skipped {
				if remappedTo == "" || state.FindTask(remappedTo) == nil {
					issues = append(issues, fmt.Sprintf("task %s transition %s output %d was deduplicated but its remapped provider %s is missing", task.ID, transitionName, outputIndex, remappedTo))
				}
				continue
			}
			issues = append(issues, fmt.Sprintf("task %s transition %s is recorded as executed but generated task %s is missing", task.ID, transitionName, childID))
		}
	}
	return issues
}

func childOutputIndex(childID string) (int, bool) {
	separator := strings.LastIndexByte(childID, '-')
	if separator < 0 || separator == len(childID)-1 {
		return 0, false
	}
	index, err := strconv.Atoi(childID[separator+1:])
	return index, err == nil
}

func skippedTransitionOutput(task *models.Task, transitionName string, outputIndex int) (string, bool) {
	type skippedEntry struct {
		OutputIndex int    `json:"output_index"`
		RemappedTo  string `json:"remapped_to"`
	}
	for historyIndex := len(task.History) - 1; historyIndex >= 0; historyIndex-- {
		entry := task.History[historyIndex]
		recordedTransition, ok := entry.Extra["transition"].(string)
		if entry.Event != models.TaskEventTransitionExecuted || !ok || recordedTransition != transitionName {
			continue
		}
		payload, err := json.Marshal(entry.Extra["skipped_entries"])
		if err != nil {
			return "", false
		}
		var skipped []skippedEntry
		if err := json.Unmarshal(payload, &skipped); err != nil {
			return "", false
		}
		for _, item := range skipped {
			if item.OutputIndex == outputIndex {
				return item.RemappedTo, true
			}
		}
		return "", false
	}
	return "", false
}

// DependencyGraphSnapshot returns the stable node/edge identity used for
// request fencing and audit diffs.
func DependencyGraphSnapshot(state *models.State) ([]string, []models.DependencyGraphEdge, string) {
	var taskIDs []string
	var edges []models.DependencyGraphEdge
	if state != nil {
		for i := range state.Tasks {
			task := &state.Tasks[i]
			taskIDs = append(taskIDs, task.ID)
			if len(task.DependencyContracts) == 0 {
				for _, provider := range task.DependsOn {
					edges = append(edges, models.DependencyGraphEdge{Consumer: task.ID, Provider: provider, Gate: models.DependencyGateBeforeStart, Severity: models.DependencySeverityCritical, Purpose: "legacy scheduler dependency", Supplies: "legacy merged task output"})
				}
			} else {
				contractEdges, _ := validateContracts(task.ID, task.DependencyContracts, -1, map[string]*models.Task{})
				edges = append(edges, contractEdges...)
			}
			for outputIndex := range task.Output {
				consumer := outputNode(task.ID, outputIndex)
				taskIDs = append(taskIDs, consumer)
				output := &task.Output[outputIndex]
				if len(output.DependencyContracts) == 0 {
					for _, providerOutput := range output.DependsOn {
						if index, err := strconv.Atoi(providerOutput); err == nil {
							edges = append(edges, models.DependencyGraphEdge{Consumer: consumer, Provider: outputNode(task.ID, index), Gate: models.DependencyGateBeforeStart, Severity: models.DependencySeverityCritical, Purpose: "legacy output dependency", Supplies: "legacy planned sibling output"})
						}
					}
					for _, providerTask := range output.TaskDependsOn {
						edges = append(edges, models.DependencyGraphEdge{Consumer: consumer, Provider: providerTask, Gate: models.DependencyGateBeforeStart, Severity: models.DependencySeverityCritical, Purpose: "legacy scheduler dependency", Supplies: "legacy merged task output"})
					}
				} else {
					contractEdges, _ := validateContracts(consumer, output.DependencyContracts, len(task.Output), map[string]*models.Task{})
					edges = append(edges, contractEdges...)
				}
			}
		}
	}
	sort.Strings(taskIDs)
	sort.Slice(edges, func(i, j int) bool {
		left := edges[i].Consumer + "\x00" + edges[i].Provider + "\x00" + string(edges[i].Gate)
		right := edges[j].Consumer + "\x00" + edges[j].Provider + "\x00" + string(edges[j].Gate)
		return left < right
	})
	payload, _ := json.Marshal(struct {
		Tasks []string                     `json:"tasks"`
		Edges []models.DependencyGraphEdge `json:"edges"`
	}{taskIDs, edges})
	return taskIDs, edges, hashBytes(payload)
}

func ScopeContractFingerprint(state *models.State) string {
	var contracts []string
	if state != nil {
		contracts = append(contracts, state.Goal.Description+"\x00"+state.Goal.SpecRef)
		for _, task := range state.Tasks {
			contracts = append(contracts, coreTaskContract(task.Description, task.Scope, task.DoneWhen, task.SpecRef, task.Validation))
			for _, output := range task.Output {
				contracts = append(contracts, coreTaskContract(output.Desc, output.Scope, output.DoneWhen, output.SpecRef, output.Validation))
			}
		}
	}
	sort.Strings(contracts)
	contracts = slices.Compact(contracts)
	return hashBytes([]byte(strings.Join(contracts, "\x1e")))
}

func CandidateLineageFingerprint(state *models.State) string {
	var rows []string
	if state != nil {
		for _, task := range state.Tasks {
			if task.BaseCommit == nil && task.ReviewCommit == nil && task.MergeCommit == nil {
				continue
			}
			rows = append(rows, strings.Join([]string{task.ID, stringValue(task.BaseCommit), stringValue(task.ReviewCommit), stringValue(task.MergeCommit)}, "\x00"))
		}
	}
	sort.Strings(rows)
	return hashBytes([]byte(strings.Join(rows, "\x1e")))
}

func coreTaskContract(description, scope, doneWhen, specRef string, validation []string) string {
	checks := append([]string(nil), validation...)
	sort.Strings(checks)
	return strings.Join([]string{description, scope, doneWhen, specRef, strings.Join(checks, "\x1f")}, "\x00")
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func hashBytes(value []byte) string {
	digest := sha256.Sum256(value)
	return hex.EncodeToString(digest[:])
}

func normalizedStrings(values []string) []string {
	result := append([]string(nil), values...)
	sort.Strings(result)
	return slices.Compact(result)
}

func outputNode(taskID string, index int) string {
	return fmt.Sprintf("%s#output:%d", taskID, index)
}

func dependencyContractCycle(taskIDs []string, edges []models.DependencyGraphEdge) []string {
	adjacency := map[string][]string{}
	for _, edge := range edges {
		adjacency[edge.Consumer] = append(adjacency[edge.Consumer], edge.Provider)
	}
	for key := range adjacency {
		sort.Strings(adjacency[key])
	}
	color := map[string]uint8{}
	var stack []string
	var visit func(string) []string
	visit = func(node string) []string {
		color[node] = 1
		stack = append(stack, node)
		for _, next := range adjacency[node] {
			if color[next] == 0 {
				if cycle := visit(next); len(cycle) > 0 {
					return cycle
				}
				continue
			}
			if color[next] == 1 {
				for index := range stack {
					if stack[index] == next {
						return append(slices.Clone(stack[index:]), next)
					}
				}
			}
		}
		stack = stack[:len(stack)-1]
		color[node] = 2
		return nil
	}
	for _, node := range taskIDs {
		if color[node] == 0 {
			if cycle := visit(node); len(cycle) > 0 {
				return cycle
			}
		}
	}
	return nil
}

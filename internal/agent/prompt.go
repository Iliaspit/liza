package agent

import (
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/liza-mas/liza/internal/errors"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/ops"
	"github.com/liza-mas/liza/internal/paths"
	"github.com/liza-mas/liza/internal/pipeline"
	"github.com/liza-mas/liza/internal/precommit"
	"github.com/liza-mas/liza/internal/prompts"
	"github.com/liza-mas/liza/internal/roles"
	"github.com/liza-mas/liza/internal/scipsearch"
	"github.com/liza-mas/liza/internal/semble"
	"github.com/liza-mas/liza/internal/stacklit"
)

var buildSemblePromptMetadata = semble.BuildPromptMetadata

// baseConfigFrom constructs the BasePromptConfig shared by all roles.
func baseConfigFrom(state *models.State, config SupervisorConfig, taskID string, scipIndexes []prompts.ScipSearchIndex, stacklitIndexes []prompts.StacklitIndex, sembleSearch prompts.SembleSearchMetadata) prompts.BasePromptConfig {
	return prompts.BasePromptConfig{
		Role:              config.Role,
		AgentID:           config.AgentID,
		TaskID:            taskID,
		SpecsDir:          config.SpecsDir,
		ProjectRoot:       config.ProjectRoot,
		StatePath:         config.StatePath,
		GoalDesc:          state.Goal.Description,
		GoalSpecRef:       state.Goal.SpecRef,
		ScipSearchIndexes: scipIndexes,
		StacklitIndexes:   stacklitIndexes,
		SembleSearch:      sembleSearch,
	}
}

// buildPromptWithContext builds a complete prompt for any task-based role:
// base prompt + task lookup + role-specific context via BuildRoleContext + InitialTask suffix.
func buildPromptWithContext(state *models.State, config SupervisorConfig, taskID string, resolver *pipeline.Resolver) (string, error) {
	task := state.FindTask(taskID)
	if task == nil {
		return "", &errors.NotFoundError{Entity: "task", ID: taskID}
	}

	data, err := buildTaskRoleContextData(task, state, config, resolver)
	if err != nil {
		return "", err
	}

	prompt, err := prompts.BuildBasePrompt(baseConfigFrom(
		state,
		config,
		taskID,
		toBasePromptScipSearchIndexes(data.ScipIndexes),
		toBasePromptStacklitIndexes(data.StacklitIndexes),
		availablePromptSembleSearchMetadata(data.Worktree, semble.TargetKindTaskWorktree),
	))
	if err != nil {
		return "", fmt.Errorf("building base prompt: %w", err)
	}

	sections, err := resolver.ContextSections(config.Role)
	if err != nil {
		return "", fmt.Errorf("context sections for role %q: %w", config.Role, err)
	}
	sections, err = taskContextSections(sections, task, data, resolver)
	if err != nil {
		return "", err
	}

	context, err := prompts.BuildRoleContext(config.Role, sections, data)
	if err != nil {
		return "", err
	}
	prompt += context

	if config.InitialTask != "" {
		prompt += fmt.Sprintf("\n\n=== RESUME CONTEXT ===\nResuming task: %s\n", config.InitialTask)
	}

	return prompt, nil
}

// buildOrchestratorPromptContext builds the complete prompt for the orchestrator role.
// Unlike task-based roles, the orchestrator has no task to look up. Dashboard and wake
// instruction content is pre-rendered and passed through block templates.
func buildOrchestratorPromptContext(state *models.State, config SupervisorConfig, resolver *pipeline.Resolver) (string, error) {
	data, err := buildOrchestratorRoleContextData(state, config, resolver)
	if err != nil {
		return "", err
	}

	prompt, err := prompts.BuildBasePrompt(baseConfigFrom(
		state,
		config,
		"",
		toBasePromptScipSearchIndexes(data.ScipIndexes),
		toBasePromptStacklitIndexes(data.StacklitIndexes),
		availablePromptSembleSearchMetadata(config.ProjectRoot, semble.TargetKindProjectRoot),
	))
	if err != nil {
		return "", fmt.Errorf("building base prompt: %w", err)
	}

	sections, err := resolver.ContextSections(config.Role)
	if err != nil {
		return "", fmt.Errorf("context sections for role %q: %w", config.Role, err)
	}

	context, err := prompts.BuildRoleContext(config.Role, sections, data)
	if err != nil {
		return "", err
	}
	prompt += context

	if config.InitialTask != "" {
		prompt += fmt.Sprintf("\n\n=== RESUME CONTEXT ===\nResuming task: %s\n", config.InitialTask)
	}

	return prompt, nil
}

func buildOrchestratorRoleContextData(state *models.State, config SupervisorConfig, resolver *pipeline.Resolver) (*prompts.RoleContextData, error) {
	dashboard, wakeInstruction, err := prompts.RenderOrchestratorDashboard(state, config.ProjectRoot, config.AgentID)
	if err != nil {
		return nil, err
	}

	availableIndexes, err := scipsearch.AvailableIndexes(scipsearch.RuntimePlanOptions{
		TargetRoot:          config.ProjectRoot,
		ConfiguredLanguages: state.Config.ScipSearch,
	})
	if err != nil {
		return nil, fmt.Errorf("available scip-search indexes: %w", err)
	}
	availableStacklitIndexes, err := stacklit.AvailableIndexes(stacklit.RuntimePlanOptions{
		TargetRoot: config.ProjectRoot,
	})
	if err != nil {
		return nil, fmt.Errorf("available stacklit indexes: %w", err)
	}

	skills, _ := resolver.Skills(config.Role)
	mandatoryDocs, _ := resolver.MandatoryDocs(config.Role)

	return &prompts.RoleContextData{
		Role:            config.Role,
		AgentID:         config.AgentID,
		RoleType:        "orchestrator",
		DashboardOutput: dashboard,
		WakeInstruction: wakeInstruction,
		ScipIndexes:     toPromptScipIndexRefs(availableIndexes),
		StacklitIndexes: toPromptStacklitIndexRefs(availableStacklitIndexes),
		ProjectRoot:     config.ProjectRoot,
		StatePath:       config.StatePath,
		SpecsDir:        config.SpecsDir,
		GoalDesc:        state.Goal.Description,
		Skills:          skills,
		MandatoryDocs:   mandatoryDocs,
	}, nil
}

func toPromptStacklitIndexRefs(indexes []stacklit.IndexRef) []prompts.StacklitIndexRef {
	if len(indexes) == 0 {
		return nil
	}
	refs := make([]prompts.StacklitIndexRef, 0, len(indexes))
	for _, index := range indexes {
		refs = append(refs, prompts.StacklitIndexRef{Path: index.Path})
	}
	return refs
}

func toPromptScipIndexRefs(indexes []scipsearch.IndexRef) []prompts.ScipIndexRef {
	if len(indexes) == 0 {
		return nil
	}
	refs := make([]prompts.ScipIndexRef, 0, len(indexes))
	for _, index := range indexes {
		refs = append(refs, prompts.ScipIndexRef{
			Language: index.Language,
			Path:     index.Path,
		})
	}
	return refs
}

func toBasePromptScipSearchIndexes(indexes []prompts.ScipIndexRef) []prompts.ScipSearchIndex {
	if len(indexes) == 0 {
		return nil
	}
	refs := make([]prompts.ScipSearchIndex, 0, len(indexes))
	for _, index := range indexes {
		refs = append(refs, prompts.ScipSearchIndex{
			Language:  index.Language,
			IndexPath: index.Path,
		})
	}
	return refs
}

func toBasePromptStacklitIndexes(indexes []prompts.StacklitIndexRef) []prompts.StacklitIndex {
	if len(indexes) == 0 {
		return nil
	}
	refs := make([]prompts.StacklitIndex, 0, len(indexes))
	for _, index := range indexes {
		refs = append(refs, prompts.StacklitIndex{
			IndexPath: index.Path,
		})
	}
	return refs
}

func availablePromptScipIndexRefs(state *models.State, targetRoot string) ([]prompts.ScipIndexRef, error) {
	if targetRoot == "" || !scipsearch.RuntimeEnabled(state.Config.ScipSearch) {
		return nil, nil
	}
	availableIndexes, err := scipsearch.AvailableIndexes(scipsearch.RuntimePlanOptions{
		TargetRoot:          targetRoot,
		ConfiguredLanguages: state.Config.ScipSearch,
	})
	if err != nil {
		return nil, fmt.Errorf("available scip-search indexes: %w", err)
	}
	return toPromptScipIndexRefs(availableIndexes), nil
}

func availablePromptStacklitIndexRefs(targetRoot string) ([]prompts.StacklitIndexRef, error) {
	if targetRoot == "" || !stacklit.RuntimeEnabled() {
		return nil, nil
	}
	availableIndexes, err := stacklit.AvailableIndexes(stacklit.RuntimePlanOptions{
		TargetRoot: targetRoot,
	})
	if err != nil {
		return nil, fmt.Errorf("available stacklit indexes: %w", err)
	}
	return toPromptStacklitIndexRefs(availableIndexes), nil
}

func availablePromptSembleSearchMetadata(targetRoot string, kind semble.TargetKind) prompts.SembleSearchMetadata {
	if targetRoot == "" {
		return prompts.SembleSearchMetadata{}
	}
	opts := semble.PromptMetadataOptions{
		Kind:       kind,
		TargetRoot: targetRoot,
	}
	if kind == semble.TargetKindTaskWorktree {
		opts.ExpectedWorktreeRoot = targetRoot
	}
	metadata, ok := buildSemblePromptMetadata(opts)
	if !ok {
		return prompts.SembleSearchMetadata{}
	}
	return prompts.SembleSearchMetadata{
		TargetRoot:          metadata.TargetRoot,
		ShellTargetRoot:     metadata.ShellTargetRoot,
		OfflineEnvPrefix:    metadata.OfflineEnvPrefix,
		SearchExamples:      slices.Clone(metadata.SearchExamples),
		FindRelatedExample:  metadata.FindRelatedExample,
		ContentModeGuidance: metadata.ContentModeGuidance,
		DiscoveryNotice:     metadata.DiscoveryNotice,
	}
}

// buildTaskRoleContextData constructs RoleContextData for task-based roles (doers and reviewers).
func buildTaskRoleContextData(task *models.Task, state *models.State, config SupervisorConfig, resolver *pipeline.Resolver) (*prompts.RoleContextData, error) {
	roleType, _ := resolver.RoleType(config.Role)

	totalPlanTasks, taskOrdinal := collectPlanPosition(state, task.ID)

	data := &prompts.RoleContextData{
		// Identity
		Role:     config.Role,
		AgentID:  config.AgentID,
		RoleType: roleType,

		// Task
		TaskID:             task.ID,
		Description:        task.Description,
		DoneWhen:           task.DoneWhen,
		Scope:              task.Scope,
		SpecRef:            task.SpecRef,
		EpicRef:            paths.SplitRefFile(task.EpicRef),
		EpicSection:        paths.SplitRefFragment(task.EpicRef),
		EpicSlug:           paths.GoalSlug(paths.SplitRefFile(task.EpicRef)),
		PlanRef:            paths.SplitRefFile(task.PlanRef),
		PlanSection:        paths.SplitRefFragment(task.PlanRef),
		ArchRef:            paths.SplitRefFile(task.ArchRef),
		ValidationCommands: slices.Clone(task.Validation),
		Worktree:           resolveWorktreePath(config.ProjectRoot, task.Worktree),
		IterationNum:       task.Iteration,
		AttemptNum:         task.EffectiveAttempt(),

		// Plan scoping
		GoalSpecRef:          state.Goal.SpecRef,
		TotalPlanTasks:       totalPlanTasks,
		TaskOrdinal:          taskOrdinal,
		DependsOn:            task.DependsOn,
		TaskRolePair:         task.RolePair,
		PhaseDependencyTasks: collectPhaseDependencyTasks(state, task),
		TaskGraph:            buildRelevantTaskGraph(state, task),

		// Config/state
		ProjectRoot: config.ProjectRoot,
		StatePath:   config.StatePath,
		SpecsDir:    config.SpecsDir,
		GoalDesc:    state.Goal.Description,
		GoalSlug:    paths.GoalSlug(state.Goal.SpecRef),

		IntegrationBranch: state.Config.IntegrationBranch,
	}

	scipIndexes, err := availablePromptScipIndexRefs(state, data.Worktree)
	if err != nil {
		return nil, err
	}
	data.ScipIndexes = scipIndexes
	stacklitIndexes, err := availablePromptStacklitIndexRefs(data.Worktree)
	if err != nil {
		return nil, err
	}
	data.StacklitIndexes = stacklitIndexes

	// Prior rejection
	if task.Iteration > 1 && task.RejectionReason != nil && *task.RejectionReason != "" && *task.RejectionReason != "null" {
		data.PriorRejection = *task.RejectionReason
	}

	// Prior attempt outcome (attempt 2 only)
	if data.AttemptNum == 2 {
		for i := len(task.History) - 1; i >= 0; i-- {
			if task.History[i].Event == models.TaskEventNewAttempt && task.History[i].Reason != nil {
				data.PriorAttemptOutcome = *task.History[i].Reason
				if task.History[i].Note != nil {
					data.PriorAttemptRejection = *task.History[i].Note
				}
				break
			}
		}
	}

	// Doer-specific: coder fields
	if roleType == "doer" && config.Role == "coder" {
		data.IntegrationFix = task.IntegrationFix
		// Find the last context_exhaustion HandoffEvent for resume context
		for i := len(task.HandoffEvents) - 1; i >= 0; i-- {
			if task.HandoffEvents[i].Trigger == models.HandoffTriggerContextExhaustion {
				evt := task.HandoffEvents[i]
				data.HandoffNote = &evt
				break
			}
		}
	}

	// Reviewer-specific fields
	if roleType == "reviewer" {
		data.BaseCommit = derefString(task.BaseCommit)
		data.ReviewCommit = derefString(task.ReviewCommit)
		data.AssignedTo = derefString(task.AssignedTo)
		if task.AssignedTo != nil {
			data.ScopeExtensions = ops.GetLatestScopeExtensions(task.History, *task.AssignedTo)
			data.ValidationPlan = ops.GetValidationPlan(task.History, *task.AssignedTo)
		}
	}

	// Integration-specific: branch context for analyst and reviewer
	if config.Role == roles.IntegrationAnalyst || config.Role == roles.IntegrationReviewer {
		if state.Goal.BaseCommit != nil {
			data.GoalBaseCommit = *state.Goal.BaseCommit
		}
		data.CompletedTasks = collectCompletedTasks(state)
	}

	// Declarative fields from pipeline YAML
	if skills, err := resolver.Skills(config.Role); err == nil {
		data.Skills = skills
	}
	if mandatoryDocs, err := resolver.MandatoryDocs(config.Role); err == nil {
		data.MandatoryDocs = mandatoryDocs
	}

	// Architect-specific context
	if config.Role == roles.Architect {
		for _, parentID := range task.EffectiveParentTasks() {
			parent := state.FindTask(parentID)
			if parent != nil {
				data.ParentTaskContexts = append(data.ParentTaskContexts, prompts.ParentTaskContext{
					ID:          parent.ID,
					Description: prompts.TruncateText(parent.Description, 500),
					DoneWhen:    parent.DoneWhen,
					SpecRef:     parent.SpecRef,
					EpicRef:     parent.EpicRef,
					PlanRef:     parent.PlanRef,
				})
			}
		}

		exists, err := precommit.ConfigExistsOnIntegration(config.ProjectRoot, state.Config.IntegrationBranch)
		if err != nil {
			return nil, fmt.Errorf("precommit config check: %w", err)
		}
		data.PreCommitConfigExists = exists
		data.PreCommitBootstrapInFlight = precommit.BootstrapInFlight(state)
		data.PreCommitKind = precommit.Kind
	}

	return data, nil
}

func taskContextSections(base []string, task *models.Task, data *prompts.RoleContextData, resolver *pipeline.Resolver) ([]string, error) {
	sections := append([]string(nil), base...)
	if task.RolePair == "" || (data.RoleType != "doer" && data.RoleType != "reviewer") {
		return sections, nil
	}

	isRoot, err := resolver.IsDecompositionRoot(task.RolePair)
	if err != nil {
		return nil, err
	}
	if !isRoot {
		return sections, nil
	}

	refField, err := resolver.DecompositionOutputRef(task.RolePair)
	if err != nil {
		return nil, err
	}

	data.DecompositionRoot = true
	data.MasterOutputRefField = refField
	if data.RoleType == "doer" {
		sections = append(sections, "master-decomposition-mandate")
	} else {
		sections = append(sections, "master-decomposition-review")
	}
	return sections, nil
}

// collectCompletedTasks returns summaries of all MERGED tasks for integration context.
func collectCompletedTasks(state *models.State) []prompts.CompletedTaskSummary {
	var tasks []prompts.CompletedTaskSummary
	for _, t := range state.Tasks {
		if t.Status == models.TaskStatusMerged {
			tasks = append(tasks, prompts.CompletedTaskSummary{
				ID:          t.ID,
				Description: prompts.TruncateText(t.Description, 200),
				DoneWhen:    t.DoneWhen,
				SpecRef:     t.SpecRef,
			})
		}
	}
	return tasks
}

// resolveWorktreePath returns the absolute worktree path, or "" if worktree is nil.
func resolveWorktreePath(projectRoot string, worktree *string) string {
	if worktree == nil {
		return ""
	}
	return fmt.Sprintf("%s/%s", projectRoot, *worktree)
}

// derefString returns the value pointed to by s, or "" if s is nil.
func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// collectPlanPosition returns the visible count of planned tasks and the 1-based
// ordinal position of currentTaskID in the visible plan.
// Returns 0, 0 if no planned tasks or if currentTaskID is not in the planned list
// (e.g. mid-sprint replacement tasks created outside the original plan).
//
// Note: tasks not found by FindTask are silently skipped. This assumes the orchestrator keeps
// Sprint.Scope.Planned in sync with the task list (archived/removed tasks are pruned from planned[]).
func collectPlanPosition(state *models.State, currentTaskID string) (int, int) {
	planned := state.Sprint.Scope.Planned
	if len(planned) == 0 {
		return 0, 0
	}

	ordinal := 0
	visibleTotal := 0
	for _, id := range planned {
		task := state.FindTask(id)
		if task == nil {
			continue
		}
		if id == currentTaskID {
			visibleTotal++
			ordinal = visibleTotal
			continue
		}
		if isDeadPathPlanSibling(task.Status) {
			continue
		}
		visibleTotal++
	}

	// Suppress scoping for tasks not in the plan (mid-sprint replacements).
	// Returning 0 for totalPlanTasks ensures the template condition is false.
	if ordinal == 0 {
		return 0, 0
	}

	return visibleTotal, ordinal
}

func isDeadPathPlanSibling(status models.TaskStatus) bool {
	return status == models.TaskStatusAbandoned || status == models.TaskStatusSuperseded
}

func siblingTaskSummary(task *models.Task) prompts.SiblingTaskSummary {
	return prompts.SiblingTaskSummary{
		ID:          task.ID,
		Description: prompts.TruncateText(task.Description, 96),
		Status:      string(task.Status),
		PlanRef:     task.PlanRef,
		RolePair:    task.RolePair,
	}
}

func collectPhaseDependencyTasks(state *models.State, current *models.Task) []prompts.SiblingTaskSummary {
	if len(current.DependsOn) == 0 {
		return nil
	}

	seen := make(map[string]bool)
	var dependencies []prompts.SiblingTaskSummary
	for _, depID := range current.DependsOn {
		dep := state.FindTask(depID)
		if dep == nil || dep.RolePair != current.RolePair || seen[dep.ID] {
			continue
		}
		dependencies = append(dependencies, siblingTaskSummary(dep))
		seen[dep.ID] = true
	}
	return dependencies
}

var (
	backtickRefPattern = regexp.MustCompile("`([^`]+)`")
	// Heuristic path extraction can match URL or date-like fragments; only
	// shared refs are surfaced, so most one-off false positives drop out.
	pathRefPattern = regexp.MustCompile(`[A-Za-z0-9_.@+/-]+/[A-Za-z0-9_.@+/-]+|[A-Za-z0-9_.@+-]+\.[A-Za-z0-9][A-Za-z0-9_.@+-]*`)
)

const (
	maxTaskGraphChildrenPerEntry = 8
	maxTaskGraphChildDeps        = 3
	maxTaskGraphEntries          = 16
	maxTaskGraphOmittedSampleIDs = 8
)

func buildRelevantTaskGraph(state *models.State, current *models.Task) prompts.TaskGraphDigest {
	currentRefs := taskScopeRefs(current)
	childrenByParent := taskChildrenByParent(state)
	entries := make(map[string]*prompts.TaskGraphEntry)
	var ordered []*prompts.TaskGraphEntry

	addTask := func(task *models.Task, relation string, sharedRefs []string) {
		if task == nil {
			return
		}
		entry := entries[task.ID]
		if entry == nil {
			newEntry := taskGraphEntry(task)
			entry = &newEntry
			entries[task.ID] = entry
			ordered = append(ordered, entry)
		}
		entry.Relations = appendUniqueString(entry.Relations, relation)
		entry.SharedRefs = appendUniqueStrings(entry.SharedRefs, sharedRefs...)
	}

	dependencies := make([]*models.Task, 0, len(current.DependsOn))
	for _, depID := range current.DependsOn {
		dep := state.FindTask(depID)
		if dep == nil {
			continue
		}
		dependencies = append(dependencies, dep)
		addTask(dep, "dependency", nil)
		if dep.Status == models.TaskStatusBlocked {
			addTask(dep, "blocked", nil)
		}
	}

	siblings := plannedSiblings(state, current.ID)
	for _, sibling := range siblings {
		if sibling.Status == models.TaskStatusBlocked {
			addTask(sibling, "blocked", nil)
		}
	}

	for _, sibling := range siblings {
		if sibling.Status.IsTerminal() {
			continue
		}
		sharedRefs := intersectRefs(currentRefs, taskScopeRefs(sibling))
		if len(sharedRefs) > 0 {
			addTask(sibling, "sibling", nil)
			addTask(sibling, "file-overlap", sharedRefs)
		}
	}

	for _, dep := range dependencies {
		if taskHasProducedOutputRefs(dep) {
			addTask(dep, "artifact-producer", nil)
		}
		if !isCompletedForDigest(state, dep) {
			continue
		}
		if taskHasTaskArtifactRefs(dep) {
			addTask(dep, "artifact-ref", nil)
		}
	}

	for _, sibling := range siblings {
		if taskHasProducedOutputRefs(sibling) {
			addTask(sibling, "artifact-producer", nil)
		}
		if !isCompletedForDigest(state, sibling) {
			continue
		}
		if taskHasTaskArtifactRefs(sibling) {
			addTask(sibling, "artifact-ref", nil)
		}
	}

	for _, sibling := range siblings {
		if !isDeadPathPlanSibling(sibling.Status) {
			addTask(sibling, "sibling", nil)
		}
	}

	digest := prompts.TaskGraphDigest{
		Entries: make([]prompts.TaskGraphEntry, 0, len(ordered)),
	}
	for _, entry := range boundedTaskGraphEntries(ordered, &digest) {
		entry.Children, entry.RemainingChildren = summarizeTaskGraphChildren(childrenByParent[entry.ID])
		digest.Entries = append(digest.Entries, *entry)
	}
	return digest
}

func boundedTaskGraphEntries(ordered []*prompts.TaskGraphEntry, digest *prompts.TaskGraphDigest) []*prompts.TaskGraphEntry {
	if len(ordered) <= maxTaskGraphEntries {
		return ordered
	}

	ranked := append([]*prompts.TaskGraphEntry(nil), ordered...)
	sort.SliceStable(ranked, func(i, j int) bool {
		// Equal scores keep the existing candidate discovery order.
		return taskGraphPriority(ranked[i]) > taskGraphPriority(ranked[j])
	})

	selectedIDs := make(map[string]bool, maxTaskGraphEntries)
	for _, entry := range ranked[:maxTaskGraphEntries] {
		selectedIDs[entry.ID] = true
	}

	var bounded []*prompts.TaskGraphEntry
	var omitted []prompts.TaskGraphEntry
	for _, entry := range ordered {
		if selectedIDs[entry.ID] {
			bounded = append(bounded, entry)
			continue
		}
		omitted = append(omitted, *entry)
	}
	digest.Omitted = summarizeOmittedTaskGraphEntries(omitted)
	return bounded
}

func taskGraphPriority(entry *prompts.TaskGraphEntry) int {
	if entry == nil {
		return 0
	}
	if hasTaskGraphRelation(entry, "dependency") {
		return 100
	}
	if hasTaskGraphRelation(entry, "blocked") || entry.BlockedReason != "" || entry.RepairOperation != "" {
		return 90
	}
	if hasTaskGraphRelation(entry, "file-overlap") {
		return 80
	}
	if isArtifactOnlyTaskGraphEntry(entry) {
		return 10
	}
	if hasTaskGraphRelation(entry, "sibling") {
		return 50
	}
	return 30
}

func isArtifactOnlyTaskGraphEntry(entry *prompts.TaskGraphEntry) bool {
	if entry == nil {
		return false
	}
	if hasTaskGraphRelation(entry, "dependency") ||
		hasTaskGraphRelation(entry, "blocked") ||
		hasTaskGraphRelation(entry, "file-overlap") ||
		entry.BlockedReason != "" ||
		entry.RepairOperation != "" {
		return false
	}
	return hasTaskGraphRelation(entry, "artifact-producer") || hasTaskGraphRelation(entry, "artifact-ref")
}

func hasTaskGraphRelation(entry *prompts.TaskGraphEntry, relation string) bool {
	return slices.Contains(entry.Relations, relation)
}

func summarizeOmittedTaskGraphEntries(entries []prompts.TaskGraphEntry) prompts.TaskGraphOmittedSummary {
	summary := prompts.TaskGraphOmittedSummary{Count: len(entries)}
	if len(entries) == 0 {
		return summary
	}

	statusCounts := make(map[string]int)
	relationCounts := make(map[string]int)
	for _, entry := range entries {
		statusCounts[entry.Status]++
		for _, relation := range entry.Relations {
			relationCounts[relation]++
		}
		if len(summary.SampleIDs) < maxTaskGraphOmittedSampleIDs {
			summary.SampleIDs = append(summary.SampleIDs, entry.ID)
		}
	}
	summary.RemainingSampleIDs = len(entries) - len(summary.SampleIDs)
	summary.StatusCounts = sortedTaskGraphCounts(statusCounts)
	summary.RelationCounts = sortedTaskGraphCounts(relationCounts)
	return summary
}

func sortedTaskGraphCounts(counts map[string]int) []prompts.TaskGraphCount {
	keys := make([]string, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	slices.Sort(keys)

	sorted := make([]prompts.TaskGraphCount, 0, len(keys))
	for _, key := range keys {
		sorted = append(sorted, prompts.TaskGraphCount{Name: key, Count: counts[key]})
	}
	return sorted
}

func taskChildrenByParent(state *models.State) map[string][]*models.Task {
	childrenByParent := make(map[string][]*models.Task)
	for i := range state.Tasks {
		task := &state.Tasks[i]
		for _, parentID := range task.EffectiveParentTasks() {
			if parentID == "" {
				continue
			}
			childrenByParent[parentID] = append(childrenByParent[parentID], task)
		}
	}
	return childrenByParent
}

func summarizeTaskGraphChildren(children []*models.Task) ([]prompts.TaskGraphChildSummary, int) {
	limit := min(len(children), maxTaskGraphChildrenPerEntry)
	summaries := make([]prompts.TaskGraphChildSummary, 0, limit)
	for _, child := range children[:limit] {
		depLimit := min(len(child.DependsOn), maxTaskGraphChildDeps)
		summaries = append(summaries, prompts.TaskGraphChildSummary{
			ID:                 child.ID,
			Status:             string(child.Status),
			RolePair:           child.RolePair,
			DependsOn:          child.DependsOn[:depLimit],
			RemainingDependsOn: len(child.DependsOn) - depLimit,
		})
	}
	return summaries, len(children) - limit
}

func plannedSiblings(state *models.State, currentTaskID string) []*models.Task {
	inPlan := false
	for _, id := range state.Sprint.Scope.Planned {
		if id == currentTaskID {
			inPlan = true
			break
		}
	}
	if !inPlan {
		return nil
	}

	var siblings []*models.Task
	for _, id := range state.Sprint.Scope.Planned {
		if id == currentTaskID {
			continue
		}
		if task := state.FindTask(id); task != nil {
			siblings = append(siblings, task)
		}
	}
	return siblings
}

func taskGraphEntry(task *models.Task) prompts.TaskGraphEntry {
	entry := prompts.TaskGraphEntry{
		ID:          task.ID,
		Description: prompts.TruncateText(task.Description, 96),
		Status:      string(task.Status),
	}
	if task.BlockedReason != nil {
		entry.BlockedReason = prompts.TruncateText(*task.BlockedReason, 120)
	}
	if task.RepairRequest != nil {
		entry.RepairOperation = task.RepairRequest.Operation
		if task.RepairRequest.Target != "" {
			entry.RepairOperation += " -> " + task.RepairRequest.Target
		}
	}
	return entry
}

func taskScopeRefs(task *models.Task) []string {
	refs := extractFileRefs(task.Scope)
	refs = appendUniqueStrings(refs, extractFileRefs(task.DoneWhen)...)
	return refs
}

func extractFileRefs(text string) []string {
	var refs []string
	for _, match := range backtickRefPattern.FindAllStringSubmatch(text, -1) {
		if len(match) > 1 {
			refs = appendPathRef(refs, match[1])
		}
	}
	for _, match := range pathRefPattern.FindAllString(text, -1) {
		refs = appendPathRef(refs, match)
	}
	return refs
}

func appendPathRef(refs []string, ref string) []string {
	ref = strings.Trim(ref, " \t\r\n.,;:()[]{}\"'")
	if ref == "" || strings.Contains(ref, "*") || strings.Contains(ref, " ") {
		return refs
	}
	if !strings.Contains(ref, "/") && !strings.Contains(ref, ".") {
		return refs
	}
	return appendUniqueString(refs, ref)
}

func taskHasTaskArtifactRefs(task *models.Task) bool {
	if task == nil {
		return false
	}
	return task.SpecRef != "" || task.EpicRef != "" || task.PlanRef != "" || task.ArchRef != ""
}

func taskHasProducedOutputRefs(task *models.Task) bool {
	if task == nil {
		return false
	}
	for _, entry := range task.Output {
		if entry.SpecRef != "" || entry.EpicRef != "" || entry.PlanRef != "" || entry.ArchRef != "" {
			return true
		}
	}
	return false
}

func intersectRefs(left, right []string) []string {
	if len(left) == 0 || len(right) == 0 {
		return nil
	}
	rightSet := make(map[string]bool, len(right))
	for _, ref := range right {
		rightSet[ref] = true
	}
	var shared []string
	for _, ref := range left {
		if rightSet[ref] {
			shared = append(shared, ref)
		}
	}
	return shared
}

func isCompletedForDigest(state *models.State, task *models.Task) bool {
	if state == nil || task == nil {
		return false
	}
	return state.ResolveDependency(task.ID).Satisfied()
}

func appendUniqueStrings(refs []string, candidates ...string) []string {
	for _, candidate := range candidates {
		refs = appendUniqueString(refs, candidate)
	}
	return refs
}

func appendUniqueString(refs []string, candidate string) []string {
	if candidate == "" {
		return refs
	}
	for _, existing := range refs {
		if existing == candidate {
			return refs
		}
	}
	return append(refs, candidate)
}

func savePrompt(promptDir, agentID, prompt string) (string, error) {
	return saveTimestampedFile(promptDir, agentID, "txt", prompt)
}

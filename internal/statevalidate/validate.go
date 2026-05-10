package statevalidate

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/gitenv"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/pipeline"
)

const artifactRefMultipleRefsCause = "multiple_refs_not_supported"
const artifactRefNotFoundCause = "file_not_found"

// ArtifactRefError carries safe diagnostics for invalid artifact references.
type ArtifactRefError struct {
	Field  string
	Value  string
	TaskID string
	Cause  string
}

func (e *ArtifactRefError) Error() string {
	field := e.Field
	if field == "" {
		field = "spec_ref"
	}
	switch e.Cause {
	case artifactRefMultipleRefsCause:
		return formatArtifactRefError(field, "contains multiple refs; use one repo-relative ref", e.Value, e.TaskID)
	default:
		return formatArtifactRefError(field, "file not found", e.Value, e.TaskID)
	}
}

func (e *ArtifactRefError) SafeDetails() map[string]any {
	details := map[string]any{
		"field": e.Field,
		"value": e.Value,
		"cause": e.Cause,
	}
	if e.TaskID != "" {
		details["task_id"] = e.TaskID
	}
	return details
}

func formatArtifactRefError(field, reason, value, taskID string) string {
	if taskID != "" {
		return fmt.Sprintf("%s %s: %s (task: %s)", field, reason, value, taskID)
	}
	return fmt.Sprintf("%s %s: %s", field, reason, value)
}

// ValidateArtifactRefScalar rejects delimiter-joined refs. Artifact ref fields
// are scalar repo-relative refs; multi-reference formats must be explicit data.
func ValidateArtifactRefScalar(field, value, taskID string) error {
	if value == "" {
		return nil
	}
	if strings.Contains(value, ";") {
		return &ArtifactRefError{
			Field:  field,
			Value:  value,
			TaskID: taskID,
			Cause:  artifactRefMultipleRefsCause,
		}
	}
	return nil
}

// ValidateStateFile validates the state.yaml file against all schema rules.
// It orchestrates the full validation sequence: required fields, task states,
// task invariants, dependencies, agent invariants, discovered items, anomalies,
// and sprint configuration. Returns an error with a detailed
// description if any validation rule fails.
func ValidateStateFile(statePath string, skipSpecFileCheck bool, warnWriter io.Writer) error {
	if warnWriter == nil {
		warnWriter = io.Discard
	}

	lizaDir := filepath.Dir(statePath)
	projectRoot := filepath.Dir(lizaDir)

	bb := db.For(statePath)
	state, err := bb.Read()
	if err != nil {
		return fmt.Errorf("failed to read state file: %w", err)
	}

	// Load pipeline resolver
	var resolver *pipeline.Resolver
	cfg, cfgErr := pipeline.LoadFrozen(projectRoot)
	if cfgErr != nil {
		return fmt.Errorf("failed to load pipeline config: %w", cfgErr)
	}
	if cfg != nil {
		resolver = pipeline.NewResolver(cfg)
	}

	validators := []func(*models.State, string, bool) error{
		validateRoleNames,
		validateRequiredFields,
		func(state *models.State, projectRoot string, skipSpecFileCheck bool) error {
			return validateTaskStates(state, projectRoot, skipSpecFileCheck, resolver)
		},
		func(state *models.State, projectRoot string, skipSpecFileCheck bool) error {
			return validateTaskInvariants(state, projectRoot, skipSpecFileCheck, resolver, cfg)
		},
		func(state *models.State, projectRoot string, skipSpecFileCheck bool) error {
			return validateDependencies(state, projectRoot, skipSpecFileCheck, resolver, cfg)
		},
		func(state *models.State, projectRoot string, skipSpecFileCheck bool) error {
			return validateAgentInvariants(state, projectRoot, skipSpecFileCheck, warnWriter)
		},
		validateDiscovered,
		validateAnomalies,
		validateHandoffEvents,
		validateSprint,
	}

	for _, validator := range validators {
		if err := validator(state, projectRoot, skipSpecFileCheck); err != nil {
			return err
		}
	}

	return nil
}

// ValidateAgentInvariants exposes agent-only invariant checks for package-level tests.
func ValidateAgentInvariants(state *models.State, projectRoot string, skipSpecFileCheck bool, warnWriter io.Writer) error {
	if warnWriter == nil {
		warnWriter = io.Discard
	}
	return validateAgentInvariants(state, projectRoot, skipSpecFileCheck, warnWriter)
}

// ValidateAnomalies exposes anomaly validation for package-level tests.
func ValidateAnomalies(state *models.State, projectRoot string, skipSpecFileCheck bool) error {
	return validateAnomalies(state, projectRoot, skipSpecFileCheck)
}

// checkSpecFileExists verifies that a spec_ref points to an existing file on
// disk. Strips any fragment identifier (#section) before checking. Used by
// both required-fields and task-invariants validation to ensure specs are
// reachable.
func checkSpecFileExists(projectRoot, specRef, integrationBranch string) error {
	return checkArtifactRefFileExists(projectRoot, "spec_ref", specRef, integrationBranch, "")
}

func checkArtifactRefFileExists(projectRoot, field, ref, integrationBranch, taskID string) error {
	if err := ValidateArtifactRefScalar(field, ref, taskID); err != nil {
		return err
	}
	refFile := ref
	if idx := strings.Index(refFile, "#"); idx != -1 {
		refFile = refFile[:idx]
	}
	refPath := refFile
	if !filepath.IsAbs(refPath) {
		refPath = filepath.Join(projectRoot, refFile)
	}
	if _, err := os.Stat(refPath); err == nil {
		return nil
	}
	// Fallback: file may exist on integration branch but not on the repo-root
	// filesystem (e.g. merged by a sibling worktree). Try git cat-file -e.
	// If git is not on PATH or the branch doesn't exist, this falls through
	// gracefully to the "file not found" error below.
	if integrationBranch != "" && projectRoot != "" && !filepath.IsAbs(refFile) {
		cmd := gitenv.Command("cat-file", "-e", integrationBranch+":"+refFile)
		cmd.Dir = projectRoot
		if err := cmd.Run(); err == nil {
			return nil
		}
	}
	return &ArtifactRefError{
		Field:  field,
		Value:  ref,
		TaskID: taskID,
		Cause:  artifactRefNotFoundCause,
	}
}

// buildTaskIDSet creates a lookup set of all task IDs for O(1) existence
// checks during referential integrity validation (dependencies, parent_task,
// sprint scope).
func buildTaskIDSet(tasks []models.Task) map[string]bool {
	ids := make(map[string]bool, len(tasks))
	for _, task := range tasks {
		ids[task.ID] = true
	}
	return ids
}

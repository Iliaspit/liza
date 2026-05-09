package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/liza-mas/liza/internal/models"
)

const (
	integrationFailureFallbackOperation = "unknown"
	integrationFailureFallbackReason    = "legacy INTEGRATION_FAILED state has no structured diagnostic"
	integrationFailureFallbackHint      = "inspect task history; if a PR was merged externally, run reconcile-merged with --merge-commit and --pr-url; otherwise use supersede-task/cancel-task or recreate the worktree before retrying"
)

func latestIntegrationFailureDiagnostic(task *models.Task, projectRoot string) map[string]any {
	if len(task.IntegrationFailure) > 0 {
		return cloneStringAnyMap(task.IntegrationFailure)
	}
	for i := len(task.History) - 1; i >= 0; i-- {
		entry := task.History[i]
		if entry.Event != models.TaskEventIntegrationFailed || entry.Extra == nil {
			continue
		}
		diagnostic, ok := entry.Extra["diagnostic"].(map[string]any)
		if ok {
			return cloneStringAnyMap(diagnostic)
		}
	}
	if task.Status != models.TaskStatusIntegrationFailed {
		return nil
	}
	return fallbackIntegrationFailureDiagnostic(task, projectRoot)
}

func fallbackIntegrationFailureDiagnostic(task *models.Task, projectRoot string) map[string]any {
	diagnostic := map[string]any{
		"operation":     integrationFailureFallbackOperation,
		"reason":        integrationFailureFallbackReason,
		"status":        string(task.Status),
		"recovery_hint": integrationFailureFallbackHint,
	}
	if task.Worktree == nil || *task.Worktree == "" {
		diagnostic["worktree_state"] = "not_recorded"
		diagnostic["worktree_recorded"] = false
	} else {
		diagnostic["worktree"] = *task.Worktree
		diagnostic["worktree_recorded"] = true
		addWorktreePresence(projectRoot, *task.Worktree, diagnostic)
	}
	if len(task.FailedBy) > 0 {
		diagnostic["failed_by"] = append([]string(nil), task.FailedBy...)
	}
	return diagnostic
}

func addWorktreePresence(projectRoot, worktree string, diagnostic map[string]any) {
	if projectRoot == "" {
		diagnostic["worktree_state"] = "recorded_unchecked"
		return
	}
	worktreePath := filepath.Join(projectRoot, worktree)
	if _, err := os.Stat(worktreePath); err == nil {
		diagnostic["worktree_state"] = "present"
		diagnostic["worktree_missing"] = false
	} else if os.IsNotExist(err) {
		diagnostic["worktree_state"] = "missing"
		diagnostic["worktree_missing"] = true
	} else {
		diagnostic["worktree_state"] = "unknown"
		diagnostic["worktree_check_error"] = err.Error()
	}
}

func latestIntegrationPRURL(task *models.Task) *string {
	for i := len(task.History) - 1; i >= 0; i-- {
		entry := task.History[i]
		if entry.Extra == nil {
			continue
		}
		switch entry.Event {
		case models.TaskEventIntegrationFailed, models.TaskEventMerged:
		default:
			continue
		}
		value, ok := entry.Extra["pr_url"].(string)
		if !ok || value == "" {
			continue
		}
		return &value
	}
	return nil
}

func integrationFailureAlertMessage(task *models.Task, projectRoot string) string {
	diagnostic := latestIntegrationFailureDiagnostic(task, projectRoot)
	if diagnostic == nil {
		return task.ID
	}
	return fmt.Sprintf("%s — operation=%s reason=%s recovery_hint=%s",
		task.ID,
		stringDiagnosticValue(diagnostic, "operation"),
		stringDiagnosticValue(diagnostic, "reason"),
		stringDiagnosticValue(diagnostic, "recovery_hint"))
}

func stringDiagnosticValue(diagnostic map[string]any, key string) string {
	value, ok := diagnostic[key]
	if !ok || value == nil {
		return "unknown"
	}
	if s, ok := value.(string); ok && s != "" {
		return s
	}
	return fmt.Sprint(value)
}

func cloneStringAnyMap(values map[string]any) map[string]any {
	if len(values) == 0 {
		return nil
	}
	clone := make(map[string]any, len(values))
	for key, value := range values {
		clone[key] = value
	}
	return clone
}

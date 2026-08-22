package integration

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/ops"
	"github.com/liza-mas/liza/internal/testhelpers"
)

func TestAnalyzePlanningReviewChurn(t *testing.T) {
	newState := func(t *testing.T, rejectionCount int) (string, string) {
		t.Helper()

		projectRoot := t.TempDir()
		stateFile, _ := testhelpers.SetupLizaDir(t, projectRoot)
		planningTask := testhelpers.BuildTaskByStatus("plan-review-churn", models.TaskStatusMerged, time.Now().UTC())
		planningTask.Type = models.TaskTypePlanning
		planningTask.RolePair = "code-planning-pair"
		planningTask.ReviewCyclesTotal = rejectionCount

		state := testhelpers.CreateValidState()
		state.Tasks = []models.Task{planningTask}
		testhelpers.WriteInitialState(t, stateFile, state)
		return projectRoot, stateFile
	}

	t.Run("four durable rejection cycles trip and persist the circuit breaker", func(t *testing.T) {
		projectRoot, stateFile := newState(t, 4)

		result, err := ops.Analyze(projectRoot)
		if err != nil {
			t.Fatalf("Analyze() error: %v", err)
		}
		if !result.Triggered {
			t.Fatal("Triggered = false, want true")
		}
		if result.Pattern != "planning_review_churn" {
			t.Errorf("Pattern = %q, want planning_review_churn", result.Pattern)
		}
		if result.Severity != "PLANNING_CONVERGENCE_DEGRADED" {
			t.Errorf("Severity = %q, want PLANNING_CONVERGENCE_DEGRADED", result.Severity)
		}
		for _, evidence := range []string{"plan-review-churn", "MERGED", "4"} {
			if !strings.Contains(result.Evidence, evidence) {
				t.Errorf("Evidence = %q, want it to contain %q", result.Evidence, evidence)
			}
		}

		persisted, err := db.New(stateFile).Read()
		if err != nil {
			t.Fatalf("read persisted state: %v", err)
		}
		if persisted.Config.Mode != models.SystemModeCircuitBreakerTripped {
			t.Errorf("Mode = %s, want %s", persisted.Config.Mode, models.SystemModeCircuitBreakerTripped)
		}
		if persisted.CircuitBreaker.Status != "TRIGGERED" {
			t.Errorf("CircuitBreaker status = %q, want TRIGGERED", persisted.CircuitBreaker.Status)
		}
		if persisted.CircuitBreaker.CurrentTrigger == nil {
			t.Fatal("CurrentTrigger = nil, want persisted trigger")
		}
		if persisted.CircuitBreaker.CurrentTrigger.Pattern != result.Pattern {
			t.Errorf("CurrentTrigger.Pattern = %q, want %q", persisted.CircuitBreaker.CurrentTrigger.Pattern, result.Pattern)
		}
		if len(persisted.CircuitBreaker.History) != 1 {
			t.Fatalf("len(CircuitBreaker.History) = %d, want 1", len(persisted.CircuitBreaker.History))
		}
		history := persisted.CircuitBreaker.History[0]
		if history.Result != "TRIGGERED" || history.Pattern == nil || *history.Pattern != result.Pattern || history.Severity == nil || *history.Severity != result.Severity {
			t.Errorf("trigger history = %#v, want TRIGGERED planning-review-churn entry", history)
		}

		reportData, err := os.ReadFile(result.ReportPath)
		if err != nil {
			t.Fatalf("read report: %v", err)
		}
		report := string(reportData)
		for _, content := range []string{result.Pattern, result.Severity, result.Evidence} {
			if !strings.Contains(report, content) {
				t.Errorf("report missing %q:\n%s", content, report)
			}
		}
	})

	t.Run("three durable rejection cycles remain OK", func(t *testing.T) {
		projectRoot, stateFile := newState(t, 3)

		result, err := ops.Analyze(projectRoot)
		if err != nil {
			t.Fatalf("Analyze() error: %v", err)
		}
		if result.Triggered {
			t.Fatalf("Triggered = true, want false; result = %+v", result)
		}

		persisted, err := db.New(stateFile).Read()
		if err != nil {
			t.Fatalf("read persisted state: %v", err)
		}
		if persisted.Config.Mode != models.SystemModeRunning {
			t.Errorf("Mode = %s, want %s", persisted.Config.Mode, models.SystemModeRunning)
		}
		if persisted.CircuitBreaker.Status != "OK" {
			t.Errorf("CircuitBreaker status = %q, want OK", persisted.CircuitBreaker.Status)
		}
		if len(persisted.CircuitBreaker.History) != 1 || persisted.CircuitBreaker.History[0].Result != "OK" {
			t.Errorf("CircuitBreaker.History = %#v, want one OK entry", persisted.CircuitBreaker.History)
		}
	})
}

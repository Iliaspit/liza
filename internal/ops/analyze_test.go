package ops

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/testhelpers"
)

func TestAnalyze_NoAnomalies(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	state := testhelpers.CreateValidState()
	state.Anomalies = []models.Anomaly{} // No anomalies
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := Analyze(tmpDir)
	if err != nil {
		t.Fatalf("Analyze() error: %v", err)
	}

	if result.Triggered {
		t.Error("Should not be triggered with no anomalies")
	}

	// Verify state updated
	bb := db.New(stateFile)
	readState, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}
	if readState.CircuitBreaker.Status != "OK" {
		t.Errorf("CircuitBreaker status = %q, want %q", readState.CircuitBreaker.Status, "OK")
	}
	if readState.Config.Mode != models.SystemModeRunning {
		t.Errorf("Mode = %v, want RUNNING", readState.Config.Mode)
	}
	if readState.CircuitBreaker.CurrentTrigger != nil {
		t.Error("CurrentTrigger should be nil")
	}

	// Verify history entry added
	if len(readState.CircuitBreaker.History) == 0 {
		t.Fatal("Expected history entry")
	}
	lastHistory := readState.CircuitBreaker.History[len(readState.CircuitBreaker.History)-1]
	if lastHistory.Result != "OK" {
		t.Errorf("History result = %q, want %q", lastHistory.Result, "OK")
	}
}

func TestAnalyze_TriggeredByRetryCluster(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	// Create 3+ retry_loop anomalies with similar error_pattern in Details to trigger retry_cluster
	state.Anomalies = []models.Anomaly{
		{Type: "retry_loop", Timestamp: now, Task: "task-1", Reporter: "coder-1", Details: map[string]any{"error_pattern": "connection refused"}},
		{Type: "retry_loop", Timestamp: now, Task: "task-1", Reporter: "coder-1", Details: map[string]any{"error_pattern": "connection refused"}},
		{Type: "retry_loop", Timestamp: now, Task: "task-1", Reporter: "coder-1", Details: map[string]any{"error_pattern": "connection refused"}},
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := Analyze(tmpDir)
	if err != nil {
		t.Fatalf("Analyze() error: %v", err)
	}

	if !result.Triggered {
		t.Error("Should be triggered with 3 retry_loop anomalies")
	}
	if result.Pattern == "" {
		t.Error("Pattern should not be empty when triggered")
	}
	if result.Severity == "" {
		t.Error("Severity should not be empty when triggered")
	}
	if result.ReportPath == "" {
		t.Error("ReportPath should not be empty when triggered")
	}

	// Verify report file exists
	if _, err := os.Stat(result.ReportPath); os.IsNotExist(err) {
		t.Error("Report file should exist")
	}

	// Verify state updated
	bb := db.New(stateFile)
	readState, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}
	if readState.CircuitBreaker.Status != "TRIGGERED" {
		t.Errorf("CircuitBreaker status = %q, want %q", readState.CircuitBreaker.Status, "TRIGGERED")
	}
	if readState.Config.Mode != models.SystemModeCircuitBreakerTripped {
		t.Errorf("Mode = %v, want CIRCUIT_BREAKER_TRIPPED", readState.Config.Mode)
	}
	if readState.CircuitBreaker.CurrentTrigger == nil {
		t.Fatal("CurrentTrigger should be set")
	}
	if readState.CircuitBreaker.CurrentTrigger.Pattern == "" {
		t.Error("Trigger pattern should not be empty")
	}
}

func TestAnalyze_BelowThreshold(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	// Only 2 anomalies — below the 3-anomaly threshold for retry_cluster
	state.Anomalies = []models.Anomaly{
		{Type: "retry_loop", Timestamp: now, Task: "task-1", Reporter: "coder-1", Details: map[string]any{"error_pattern": "timeout"}},
		{Type: "retry_loop", Timestamp: now, Task: "task-1", Reporter: "coder-1", Details: map[string]any{"error_pattern": "timeout"}},
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := Analyze(tmpDir)
	if err != nil {
		t.Fatalf("Analyze() error: %v", err)
	}

	if result.Triggered {
		t.Error("Should not be triggered with only 2 retry_loop anomalies")
	}
}

func TestAnalyze_ReportsOnlyUnacknowledgedAnomalies(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	watermark := time.Date(2026, 5, 20, 11, 0, 0, 0, time.UTC)
	pattern := "provider_audit_degradation"
	severity := "OBSERVABILITY_DEGRADED"
	state := testhelpers.CreateValidState()
	state.CircuitBreaker = models.CircuitBreaker{
		Status: "OK",
		History: []models.CircuitBreakerHistory{
			{Timestamp: watermark, Pattern: &pattern, Severity: &severity, Result: "TRIGGERED"},
		},
	}
	state.Anomalies = []models.Anomaly{
		providerAuditAnomaly(watermark.Add(-time.Minute), "old-coder"),
		providerAuditAnomaly(watermark, "equal-coder"),
		providerAuditAnomaly(watermark.Add(time.Minute), "new-coder-1"),
		providerAuditAnomaly(watermark.Add(2*time.Minute), "new-coder-2"),
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := Analyze(tmpDir)
	if err != nil {
		t.Fatalf("Analyze() error: %v", err)
	}
	if !result.Triggered {
		t.Fatal("Analyze() should trigger on post-watermark anomalies")
	}

	reportData, err := os.ReadFile(result.ReportPath)
	if err != nil {
		t.Fatalf("failed to read report: %v", err)
	}
	report := string(reportData)
	if !strings.Contains(report, "**Acknowledged anomalies suppressed:** 2") {
		t.Fatalf("report missing suppressed anomaly count:\n%s", report)
	}
	if strings.Contains(report, "old-coder") || strings.Contains(report, "equal-coder") {
		t.Fatalf("report includes acknowledged anomalies:\n%s", report)
	}
	if !strings.Contains(report, "new-coder-1") || !strings.Contains(report, "new-coder-2") {
		t.Fatalf("report missing unacknowledged anomalies:\n%s", report)
	}
}

func TestAnalyze_InvalidStatePath(t *testing.T) {
	_, err := Analyze("/nonexistent/path")
	if err == nil {
		t.Fatal("Expected error for nonexistent path")
	}
	if !strings.Contains(err.Error(), "failed to read state") {
		t.Errorf("Error = %q, want to contain 'failed to read state'", err.Error())
	}
}

func providerAuditAnomaly(timestamp time.Time, agentID string) models.Anomaly {
	return models.Anomaly{
		Timestamp: timestamp,
		Reporter:  agentID,
		Type:      "provider_audit_degraded",
		Details: map[string]any{
			"provider": "codex",
			"agent_id": agentID,
			"message":  "failed to record rollout items",
		},
	}
}

package commands

import (
	"os"
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/paths"
	"github.com/liza-mas/liza/internal/testhelpers"
)

func TestAnalyzeCommand(t *testing.T) {
	tests := []struct {
		name             string
		anomalies        []models.Anomaly
		wantError        bool
		wantTriggered    bool
		wantReportExists bool
	}{
		{
			name:             "no anomalies - OK status",
			anomalies:        []models.Anomaly{},
			wantError:        false,
			wantTriggered:    false,
			wantReportExists: false,
		},
		{
			name: "retry_cluster detected - triggers circuit breaker",
			anomalies: []models.Anomaly{
				{
					Timestamp: time.Now(),
					Task:      "task-1",
					Reporter:  "coder-1",
					Type:      "retry_loop",
					Details: map[string]any{
						"error_pattern": "timeout",
					},
				},
				{
					Timestamp: time.Now(),
					Task:      "task-2",
					Reporter:  "coder-2",
					Type:      "retry_loop",
					Details: map[string]any{
						"error_pattern": "timeout",
					},
				},
				{
					Timestamp: time.Now(),
					Task:      "task-3",
					Reporter:  "code-reviewer-1",
					Type:      "retry_loop",
					Details: map[string]any{
						"error_pattern": "timeout",
					},
				},
			},
			wantError:        false,
			wantTriggered:    true,
			wantReportExists: true,
		},
		{
			name: "debt_accumulation detected",
			anomalies: []models.Anomaly{
				{
					Timestamp: time.Now(),
					Task:      "task-1",
					Reporter:  "coder-1",
					Type:      "trade_off",
					Details: map[string]any{
						"debt_created": true,
					},
				},
				{
					Timestamp: time.Now(),
					Task:      "task-2",
					Reporter:  "coder-2",
					Type:      "trade_off",
					Details: map[string]any{
						"debt_created": true,
					},
				},
				{
					Timestamp: time.Now(),
					Task:      "task-3",
					Reporter:  "coder-1",
					Type:      "trade_off",
					Details: map[string]any{
						"debt_created": true,
					},
				},
			},
			wantError:        false,
			wantTriggered:    true,
			wantReportExists: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test environment
			tempDir := t.TempDir()
			reportPath := paths.New(tempDir).CircuitBreakerReportPath()

			// Create test state with anomalies
			state := testhelpers.CreateValidState()
			state.Anomalies = tt.anomalies

			// Setup liza directory and write state
			statePath, _ := testhelpers.SetupLizaDir(t, tempDir)
			testhelpers.SetupPipelineConfig(t, tempDir)
			bb := testhelpers.WriteInitialState(t, statePath, state)

			// Run analyze command
			err := AnalyzeCommand(tempDir)

			// Check error
			if (err != nil) != tt.wantError {
				t.Errorf("AnalyzeCommand() error = %v, wantError %v", err, tt.wantError)
			}

			// Read updated state
			updatedState, err := bb.Read()
			if err != nil {
				t.Fatalf("Failed to read updated state: %v", err)
			}

			// Check circuit breaker status
			if tt.wantTriggered {
				if updatedState.CircuitBreaker.Status != "TRIGGERED" {
					t.Errorf("CircuitBreaker status = %v, want TRIGGERED", updatedState.CircuitBreaker.Status)
				}

				if updatedState.CircuitBreaker.CurrentTrigger == nil {
					t.Error("CurrentTrigger should be set when triggered")
				} else {
					if updatedState.CircuitBreaker.CurrentTrigger.Pattern == "" {
						t.Error("CurrentTrigger pattern should not be empty")
					}
					if updatedState.CircuitBreaker.CurrentTrigger.Severity == "" {
						t.Error("CurrentTrigger severity should not be empty")
					}
				}

				// Check history
				if len(updatedState.CircuitBreaker.History) == 0 {
					t.Error("History should have at least one entry")
				}
			} else {
				if updatedState.CircuitBreaker.Status != "OK" {
					t.Errorf("CircuitBreaker status = %v, want OK", updatedState.CircuitBreaker.Status)
				}

				if updatedState.CircuitBreaker.CurrentTrigger != nil {
					t.Error("CurrentTrigger should be nil when not triggered")
				}
			}

			// Check system mode
			if tt.wantTriggered {
				if updatedState.Config.Mode != models.SystemModeCircuitBreakerTripped {
					t.Errorf("Config.Mode = %v, want CIRCUIT_BREAKER_TRIPPED", updatedState.Config.Mode)
				}
			} else {
				if updatedState.Config.Mode != models.SystemModeRunning {
					t.Errorf("Config.Mode = %v, want RUNNING", updatedState.Config.Mode)
				}
			}

			// Check report file
			_, err = os.Stat(reportPath)
			reportExists := err == nil
			if reportExists != tt.wantReportExists {
				t.Errorf("Report file exists = %v, want %v", reportExists, tt.wantReportExists)
			}

			// Verify report content if it should exist
			if tt.wantReportExists && reportExists {
				reportData, err := os.ReadFile(reportPath)
				if err != nil {
					t.Fatalf("Failed to read report: %v", err)
				}

				report := string(reportData)
				expectedSections := []string{
					"# Circuit Breaker Report",
					"**Triggered:**",
					"**Pattern:**",
					"**Severity:**",
					"## Trigger Evidence",
					"## Anomalies (trimmed)",
					"## Anomalies (raw)",
					"## Human Decision Required",
				}

				for _, section := range expectedSections {
					if !contains(report, section) {
						t.Errorf("Report missing expected section: %q", section)
					}
				}
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || contains(s[1:], substr)))
}

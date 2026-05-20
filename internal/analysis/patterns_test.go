package analysis

import (
	"strings"
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/models"
)

func TestDetectPatterns(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name          string
		anomalies     []models.Anomaly
		wantPattern   string
		wantSeverity  string
		wantEvidence  string
		wantTriggered bool
	}{
		{
			name:          "no anomalies - OK",
			anomalies:     []models.Anomaly{},
			wantTriggered: false,
		},
		{
			name: "retry_cluster - 3+ retry_loops with similar error_pattern",
			anomalies: []models.Anomaly{
				{
					Timestamp: now,
					Task:      "task-1",
					Reporter:  "coder-1",
					Type:      "retry_loop",
					Details: map[string]any{
						"error_pattern": "serialization failure on nested entity",
						"count":         3,
					},
				},
				{
					Timestamp: now,
					Task:      "task-2",
					Reporter:  "coder-2",
					Type:      "retry_loop",
					Details: map[string]any{
						"error_pattern": "serialization failure on nested entity",
						"count":         2,
					},
				},
				{
					Timestamp: now,
					Task:      "task-3",
					Reporter:  "code-reviewer-1",
					Type:      "retry_loop",
					Details: map[string]any{
						"error_pattern": "serialization failure on nested entity",
						"count":         4,
					},
				},
			},
			wantTriggered: true,
			wantPattern:   "retry_cluster",
			wantSeverity:  "ARCHITECTURE_FLAW",
			wantEvidence:  "3 retry_loop anomalies with similar error patterns",
		},
		{
			name: "retry_cluster - 3 retry_loops but different error patterns",
			anomalies: []models.Anomaly{
				{
					Timestamp: now,
					Task:      "task-1",
					Reporter:  "coder-1",
					Type:      "retry_loop",
					Details: map[string]any{
						"error_pattern": "timeout",
					},
				},
				{
					Timestamp: now,
					Task:      "task-2",
					Reporter:  "coder-2",
					Type:      "retry_loop",
					Details: map[string]any{
						"error_pattern": "connection refused",
					},
				},
				{
					Timestamp: now,
					Task:      "task-3",
					Reporter:  "code-reviewer-1",
					Type:      "retry_loop",
					Details: map[string]any{
						"error_pattern": "parse error",
					},
				},
			},
			wantTriggered: false,
		},
		{
			name: "debt_accumulation - 3+ trade_offs with debt_created=true",
			anomalies: []models.Anomaly{
				{
					Timestamp: now,
					Task:      "task-1",
					Reporter:  "coder-1",
					Type:      "trade_off",
					Details: map[string]any{
						"what":         "flatten entity instead of fixing serializer",
						"debt_created": true,
					},
				},
				{
					Timestamp: now,
					Task:      "task-2",
					Reporter:  "coder-2",
					Type:      "trade_off",
					Details: map[string]any{
						"what":         "skip validation for now",
						"debt_created": true,
					},
				},
				{
					Timestamp: now,
					Task:      "task-3",
					Reporter:  "coder-1",
					Type:      "trade_off",
					Details: map[string]any{
						"what":         "hardcode value temporarily",
						"debt_created": true,
					},
				},
			},
			wantTriggered: true,
			wantPattern:   "debt_accumulation",
			wantSeverity:  "SCOPE_FLAW",
			wantEvidence:  "3 trade-offs creating technical debt",
		},
		{
			name: "assumption_cascade - 2+ assumption_violated with same assumption",
			anomalies: []models.Anomaly{
				{
					Timestamp: now,
					Task:      "task-1",
					Reporter:  "coder-1",
					Type:      "assumption_violated",
					Details: map[string]any{
						"assumption": "API supports pagination",
						"reality":    "API returns max 100, no cursor",
					},
				},
				{
					Timestamp: now,
					Task:      "task-3",
					Reporter:  "coder-2",
					Type:      "assumption_violated",
					Details: map[string]any{
						"assumption": "API supports pagination",
						"reality":    "No pagination support at all",
					},
				},
			},
			wantTriggered: true,
			wantPattern:   "assumption_cascade",
			wantSeverity:  "SPEC_FLAW",
			wantEvidence:  "Same assumption violated across multiple tasks",
		},
		{
			name: "spec_gap_cluster - 2+ spec_ambiguity with same spec_ref",
			anomalies: []models.Anomaly{
				{
					Timestamp: now,
					Task:      "task-1",
					Reporter:  "coder-1",
					Type:      "spec_ambiguity",
					Details: map[string]any{
						"spec_ref": "specs/requirements.md#FR-012",
						"gap":      "unclear error handling",
					},
				},
				{
					Timestamp: now,
					Task:      "task-2",
					Reporter:  "coder-2",
					Type:      "spec_ambiguity",
					Details: map[string]any{
						"spec_ref": "specs/requirements.md#FR-012",
						"gap":      "missing validation rules",
					},
				},
			},
			wantTriggered: true,
			wantPattern:   "spec_gap_cluster",
			wantSeverity:  "SPEC_FLAW",
			wantEvidence:  "Multiple tasks hitting same spec ambiguity",
		},
		{
			name: "workaround_pattern - 2+ workarounds/trade_offs with similar root_cause",
			anomalies: []models.Anomaly{
				{
					Timestamp: now,
					Task:      "task-1",
					Reporter:  "code-reviewer-1",
					Type:      "workaround",
					Details: map[string]any{
						"what":       "manual cleanup",
						"root_cause": "missing cleanup hook",
					},
				},
				{
					Timestamp: now,
					Task:      "task-2",
					Reporter:  "coder-1",
					Type:      "trade_off",
					Details: map[string]any{
						"what":       "defer cleanup",
						"root_cause": "missing cleanup hook",
					},
				},
			},
			wantTriggered: true,
			wantPattern:   "workaround_pattern",
			wantSeverity:  "ARCHITECTURE_FLAW",
			wantEvidence:  "2 workarounds/trade-offs with similar root causes",
		},
		{
			name: "external_service_outage - 2+ external_blockers with same blocker_service",
			anomalies: []models.Anomaly{
				{
					Timestamp: now,
					Task:      "task-1",
					Reporter:  "coder-1",
					Type:      "external_blocker",
					Details: map[string]any{
						"blocker_service": "GitHub API",
						"details":         "rate limited",
					},
				},
				{
					Timestamp: now,
					Task:      "task-3",
					Reporter:  "coder-2",
					Type:      "external_blocker",
					Details: map[string]any{
						"blocker_service": "GitHub API",
						"details":         "timeout",
					},
				},
			},
			wantTriggered: true,
			wantPattern:   "external_service_outage",
			wantSeverity:  "EXTERNAL_DEPENDENCY",
			wantEvidence:  "Multiple tasks blocked by same external service: GitHub API",
		},
		{
			name: "provider_audit_degradation - 2+ agents for same provider",
			anomalies: []models.Anomaly{
				{
					Timestamp: now,
					Task:      "task-1",
					Reporter:  "coder-1",
					Type:      "provider_audit_degraded",
					Details: map[string]any{
						"provider": "codex",
						"agent_id": "coder-1",
						"message":  "failed to record rollout items",
					},
				},
				{
					Timestamp: now,
					Task:      "task-2",
					Reporter:  "code-reviewer-1",
					Type:      "provider_audit_degraded",
					Details: map[string]any{
						"provider": "codex",
						"agent_id": "code-reviewer-1",
						"message":  "failed to record rollout items",
					},
				},
			},
			wantTriggered: true,
			wantPattern:   "provider_audit_degradation",
			wantSeverity:  "OBSERVABILITY_DEGRADED",
			wantEvidence:  "2 provider_audit_degraded anomalies for provider codex across 2 agents",
		},
		{
			name: "provider_audit_degradation - 3+ hits from same agent",
			anomalies: []models.Anomaly{
				{
					Timestamp: now,
					Task:      "task-1",
					Reporter:  "coder-1",
					Type:      "provider_audit_degraded",
					Details: map[string]any{
						"provider": "codex",
						"agent_id": "coder-1",
						"message":  "failed to record rollout items",
					},
				},
				{
					Timestamp: now,
					Task:      "task-2",
					Reporter:  "coder-1",
					Type:      "provider_audit_degraded",
					Details: map[string]any{
						"provider": "codex",
						"agent_id": "coder-1",
						"message":  "failed to record rollout items",
					},
				},
				{
					Timestamp: now,
					Task:      "task-3",
					Reporter:  "coder-1",
					Type:      "provider_audit_degraded",
					Details: map[string]any{
						"provider": "codex",
						"agent_id": "coder-1",
						"message":  "failed to record rollout items",
					},
				},
			},
			wantTriggered: true,
			wantPattern:   "provider_audit_degradation",
			wantSeverity:  "OBSERVABILITY_DEGRADED",
			wantEvidence:  "3 provider_audit_degraded anomalies for provider codex across 1 agents",
		},
		{
			name: "provider_audit_degradation - one hit below threshold",
			anomalies: []models.Anomaly{
				{
					Timestamp: now,
					Task:      "task-1",
					Reporter:  "coder-1",
					Type:      "provider_audit_degraded",
					Details: map[string]any{
						"provider": "codex",
						"agent_id": "coder-1",
						"message":  "failed to record rollout items",
					},
				},
			},
			wantTriggered: false,
		},
		{
			name: "multiple patterns - returns first match (retry_cluster)",
			anomalies: []models.Anomaly{
				// retry_cluster pattern
				{
					Timestamp: now,
					Task:      "task-1",
					Type:      "retry_loop",
					Details: map[string]any{
						"error_pattern": "timeout",
					},
				},
				{
					Timestamp: now,
					Task:      "task-2",
					Type:      "retry_loop",
					Details: map[string]any{
						"error_pattern": "timeout",
					},
				},
				{
					Timestamp: now,
					Task:      "task-3",
					Type:      "retry_loop",
					Details: map[string]any{
						"error_pattern": "timeout",
					},
				},
				// debt_accumulation pattern
				{
					Timestamp: now,
					Task:      "task-4",
					Type:      "trade_off",
					Details: map[string]any{
						"debt_created": true,
					},
				},
				{
					Timestamp: now,
					Task:      "task-5",
					Type:      "trade_off",
					Details: map[string]any{
						"debt_created": true,
					},
				},
				{
					Timestamp: now,
					Task:      "task-6",
					Type:      "trade_off",
					Details: map[string]any{
						"debt_created": true,
					},
				},
			},
			wantTriggered: true,
			wantPattern:   "retry_cluster",
			wantSeverity:  "ARCHITECTURE_FLAW",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectPatterns(tt.anomalies)

			if result.Triggered != tt.wantTriggered {
				t.Errorf("DetectPatterns() triggered = %v, want %v", result.Triggered, tt.wantTriggered)
			}

			if !tt.wantTriggered {
				return
			}

			if result.Pattern != tt.wantPattern {
				t.Errorf("DetectPatterns() pattern = %v, want %v", result.Pattern, tt.wantPattern)
			}

			if result.Severity != tt.wantSeverity {
				t.Errorf("DetectPatterns() severity = %v, want %v", result.Severity, tt.wantSeverity)
			}

			if tt.wantEvidence != "" && result.Evidence != tt.wantEvidence {
				t.Errorf("DetectPatterns() evidence = %v, want %v", result.Evidence, tt.wantEvidence)
			}
		})
	}
}

func TestDetectUnacknowledgedPatterns(t *testing.T) {
	watermark1 := time.Date(2026, 5, 20, 10, 0, 0, 0, time.UTC)
	watermark2 := time.Date(2026, 5, 20, 11, 0, 0, 0, time.UTC)
	afterWatermark := watermark2.Add(time.Minute)
	okPattern := "retry_cluster"
	triggerPattern := "provider_audit_degradation"
	trigger := &models.CircuitBreakerTrigger{
		Timestamp:  watermark2,
		Pattern:    triggerPattern,
		Severity:   "OBSERVABILITY_DEGRADED",
		ReportFile: ".liza/circuit_breaker_report.md",
	}

	tests := []struct {
		name                string
		state               *models.State
		wantTriggered       bool
		wantSuppressedCount int
		wantConsideredCount int
	}{
		{
			name: "cleared trigger suppresses anomalies at or before latest triggered history timestamp",
			state: &models.State{
				Anomalies: []models.Anomaly{
					providerAuditAnomaly(watermark1, "coder-1"),
					providerAuditAnomaly(watermark2, "coder-2"),
					providerAuditAnomaly(afterWatermark, "coder-1"),
				},
				CircuitBreaker: models.CircuitBreaker{
					Status: "OK",
					History: []models.CircuitBreakerHistory{
						{Timestamp: watermark2, Pattern: &triggerPattern, Severity: stringPtr("OBSERVABILITY_DEGRADED"), Result: "TRIGGERED"},
					},
				},
			},
			wantTriggered:       false,
			wantSuppressedCount: 2,
			wantConsideredCount: 1,
		},
		{
			name: "latest triggered history wins and later OK entries do not move watermark",
			state: &models.State{
				Anomalies: []models.Anomaly{
					providerAuditAnomaly(watermark1.Add(time.Minute), "old-coder"),
					providerAuditAnomaly(afterWatermark, "coder-1"),
					providerAuditAnomaly(afterWatermark.Add(time.Minute), "coder-2"),
				},
				CircuitBreaker: models.CircuitBreaker{
					Status: "OK",
					History: []models.CircuitBreakerHistory{
						{Timestamp: watermark1, Pattern: &okPattern, Severity: stringPtr("ARCHITECTURE_FLAW"), Result: "TRIGGERED"},
						{Timestamp: watermark2.Add(2 * time.Hour), Result: "OK"},
						{Timestamp: watermark2, Pattern: &triggerPattern, Severity: stringPtr("OBSERVABILITY_DEGRADED"), Result: "TRIGGERED"},
					},
				},
			},
			wantTriggered:       true,
			wantSuppressedCount: 1,
			wantConsideredCount: 2,
		},
		{
			name: "active triggered status does not use acknowledgement watermark",
			state: &models.State{
				Anomalies: []models.Anomaly{
					providerAuditAnomaly(watermark1, "coder-1"),
					providerAuditAnomaly(watermark1.Add(time.Minute), "coder-2"),
				},
				CircuitBreaker: models.CircuitBreaker{
					Status: "TRIGGERED",
					History: []models.CircuitBreakerHistory{
						{Timestamp: watermark2, Pattern: &triggerPattern, Severity: stringPtr("OBSERVABILITY_DEGRADED"), Result: "TRIGGERED"},
					},
				},
			},
			wantTriggered:       true,
			wantSuppressedCount: 0,
			wantConsideredCount: 2,
		},
		{
			name: "non-nil current trigger does not use acknowledgement watermark",
			state: &models.State{
				Anomalies: []models.Anomaly{
					providerAuditAnomaly(watermark1, "coder-1"),
					providerAuditAnomaly(watermark1.Add(time.Minute), "coder-2"),
				},
				CircuitBreaker: models.CircuitBreaker{
					Status:         "OK",
					CurrentTrigger: trigger,
					History: []models.CircuitBreakerHistory{
						{Timestamp: watermark2, Pattern: &triggerPattern, Severity: stringPtr("OBSERVABILITY_DEGRADED"), Result: "TRIGGERED"},
					},
				},
			},
			wantTriggered:       true,
			wantSuppressedCount: 0,
			wantConsideredCount: 2,
		},
		{
			name: "no trigger history preserves existing detection behavior",
			state: &models.State{
				Anomalies: []models.Anomaly{
					providerAuditAnomaly(watermark1, "coder-1"),
					providerAuditAnomaly(watermark1.Add(time.Minute), "coder-2"),
				},
				CircuitBreaker: models.CircuitBreaker{Status: "OK"},
			},
			wantTriggered:       true,
			wantSuppressedCount: 0,
			wantConsideredCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, considered, suppressedCount := DetectUnacknowledgedPatterns(tt.state)
			if result.Triggered != tt.wantTriggered {
				t.Errorf("Triggered = %v, want %v", result.Triggered, tt.wantTriggered)
			}
			if suppressedCount != tt.wantSuppressedCount {
				t.Errorf("suppressedCount = %d, want %d", suppressedCount, tt.wantSuppressedCount)
			}
			if len(considered) != tt.wantConsideredCount {
				t.Errorf("considered count = %d, want %d", len(considered), tt.wantConsideredCount)
			}
		})
	}
}

func TestGenerateReport(t *testing.T) {
	now := time.Date(2025, 1, 18, 17, 30, 0, 0, time.UTC)

	anomalies := []models.Anomaly{
		{
			Timestamp: now,
			Task:      "task-3",
			Reporter:  "coder-1",
			Type:      "retry_loop",
			Details: map[string]any{
				"count":         3,
				"error_pattern": "serialization failure on nested entity",
			},
		},
		{
			Timestamp: now,
			Task:      "task-5",
			Reporter:  "code-reviewer-1",
			Type:      "retry_loop",
			Details: map[string]any{
				"count":         2,
				"error_pattern": "serialization failure on nested entity",
			},
		},
	}

	result := PatternResult{
		Triggered: true,
		Pattern:   "retry_cluster",
		Severity:  "ARCHITECTURE_FLAW",
		Evidence:  "3 retry_loop anomalies with similar error patterns",
	}

	report := GenerateReport(result, anomalies, now, 0)

	// Verify report contains key sections
	if report == "" {
		t.Fatal("GenerateReport() returned empty report")
	}

	expectedSections := []string{
		"# Circuit Breaker Report",
		"**Triggered:**",
		"**Pattern:** retry_cluster",
		"**Severity:** ARCHITECTURE_FLAW",
		"## Trigger Evidence",
		"3 retry_loop anomalies with similar error patterns",
		"## Anomalies (trimmed)",
		"## Anomalies (raw)",
		"## Human Decision Required",
		"- [ ] Acknowledge report",
		"- [ ] Confirm severity assessment",
		"- [ ] Determine remediation",
		"- [ ] Release checkpoint with decision logged",
	}

	for _, section := range expectedSections {
		if !contains(report, section) {
			t.Errorf("GenerateReport() missing expected section: %q", section)
		}
	}
}

func TestGenerateReportTrimsProviderAuditMessages(t *testing.T) {
	now := time.Date(2026, 5, 14, 10, 45, 25, 0, time.UTC)
	longOutput := strings.Repeat("SUPPORT_FULL_TEXT ", 200)
	message := `{"type":"item.completed","item":{"type":"command_execution","command":"/usr/bin/zsh -lc 'cat .liza/SUPPORT.md'","aggregated_output":"` + longOutput + `","exit_code":0,"status":"completed"}}`
	anomalies := []models.Anomaly{
		{
			Timestamp: now,
			Reporter:  "orchestrator-1",
			Type:      "provider_audit_degraded",
			Details: map[string]any{
				"provider": "codex",
				"agent_id": "orchestrator-1",
				"impact":   "provider transcript or rollout persistence may be incomplete",
				"message":  message,
			},
		},
	}

	report := GenerateReport(PatternResult{
		Triggered: true,
		Pattern:   "provider_audit_degradation",
		Severity:  "OBSERVABILITY_DEGRADED",
		Evidence:  "1 provider_audit_degraded anomaly",
	}, anomalies, now, 0)

	trimmedStart := strings.Index(report, "## Anomalies (trimmed)")
	rawStart := strings.Index(report, "## Anomalies (raw)")
	if trimmedStart == -1 {
		t.Fatal("report missing trimmed anomalies section")
	}
	if rawStart == -1 {
		t.Fatal("report missing raw anomalies section")
	}
	if trimmedStart > rawStart {
		t.Fatal("trimmed anomalies section should appear before raw anomalies section")
	}

	trimmedSection := report[trimmedStart:rawStart]
	expectedTrimmed := []string{
		"provider: `codex`",
		"agent_id: `orchestrator-1`",
		"provider_event: `item.completed / command_execution`",
		"command: `/usr/bin/zsh -lc 'cat .liza/SUPPORT.md'`",
		"status: `completed`, exit_code: `0`",
		"aggregated_output_chars:",
	}
	for _, want := range expectedTrimmed {
		if !strings.Contains(trimmedSection, want) {
			t.Errorf("trimmed section missing %q\nsection:\n%s", want, trimmedSection)
		}
	}
	if strings.Contains(trimmedSection, "SUPPORT_FULL_TEXT") {
		t.Fatal("trimmed section should not include raw aggregated output")
	}
	if !strings.Contains(report[rawStart:], "SUPPORT_FULL_TEXT") {
		t.Fatal("raw section should preserve full anomaly payload")
	}
}

func TestGenerateReportIncludesSuppressedAcknowledgedAnomalyCount(t *testing.T) {
	now := time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC)
	report := GenerateReport(PatternResult{
		Triggered: true,
		Pattern:   "provider_audit_degradation",
		Severity:  "OBSERVABILITY_DEGRADED",
		Evidence:  "2 provider_audit_degraded anomalies for provider codex across 2 agents",
	}, []models.Anomaly{
		providerAuditAnomaly(now, "coder-1"),
		providerAuditAnomaly(now.Add(time.Minute), "coder-2"),
	}, now, 4)

	if !strings.Contains(report, "**Acknowledged anomalies suppressed:** 4") {
		t.Fatalf("report missing suppressed anomaly count:\n%s", report)
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

func stringPtr(value string) *string {
	return &value
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || contains(s[1:], substr)))
}

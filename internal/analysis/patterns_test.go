package analysis

import (
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

	report := GenerateReport(result, anomalies, now)

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

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || contains(s[1:], substr)))
}

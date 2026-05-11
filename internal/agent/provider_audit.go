package agent

import (
	"strings"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/models"
)

const ProviderAuditDegradedAnomalyType = "provider_audit_degraded"

type providerAuditPattern struct {
	Provider string
	Needles  []string
}

var providerAuditPatterns = []providerAuditPattern{
	{
		Provider: "codex",
		Needles:  []string{"failed to record rollout items:", "thread ", " not found"},
	},
}

// ProviderAuditDegraded reports provider-side transcript persistence degradation.
// The agent turn may still be usable, but the provider audit trail is suspect.
type ProviderAuditDegraded struct {
	Provider string
	Message  string
}

// DetectProviderAuditDegraded scans provider output for non-fatal audit trail
// degradation such as Codex rollout persistence failures.
func DetectProviderAuditDegraded(output, cliName string) *ProviderAuditDegraded {
	for _, p := range providerAuditPatterns {
		if p.Provider != cliName {
			continue
		}
		for _, line := range strings.Split(output, "\n") {
			matched := true
			for _, needle := range p.Needles {
				if !strings.Contains(line, needle) {
					matched = false
					break
				}
			}
			if matched {
				return &ProviderAuditDegraded{
					Provider: p.Provider,
					Message:  strings.TrimSpace(line),
				}
			}
		}
	}
	return nil
}

// LogProviderAuditDegradedAlert appends a provider audit degradation alert.
func LogProviderAuditDegradedAlert(projectRoot string, pad *ProviderAuditDegraded) error {
	return LogAlert(projectRoot, "⚠️", "PROVIDER AUDIT DEGRADED", pad.Provider+": "+pad.Message)
}

func handleProviderAuditDegraded(bb *db.Blackboard, config SupervisorConfig, taskID, output string) bool {
	pad := DetectProviderAuditDegraded(output, config.CLIName)
	if pad == nil {
		return false
	}

	GetLogger().Warn("Provider audit degraded",
		"provider", pad.Provider,
		"agent_id", config.AgentID,
		"task_id", taskID,
		"message", pad.Message)

	if alertErr := LogProviderAuditDegradedAlert(config.ProjectRoot, pad); alertErr != nil {
		GetLogger().Warn("Failed to write provider audit degradation alert", "error", alertErr)
	}

	if bb != nil {
		if err := bb.Modify(func(state *models.State) error {
			state.Anomalies = append(state.Anomalies, models.Anomaly{
				Timestamp: time.Now().UTC(),
				Task:      taskID,
				Reporter:  config.AgentID,
				Type:      ProviderAuditDegradedAnomalyType,
				Details: map[string]any{
					"provider": config.CLIName,
					"agent_id": config.AgentID,
					"message":  pad.Message,
					"impact":   "provider transcript or rollout persistence may be incomplete",
				},
			})
			return nil
		}); err != nil {
			GetLogger().Warn("Failed to record provider audit degradation anomaly", "error", err)
		}
	}

	return true
}

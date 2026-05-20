package analysis

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/liza-mas/liza/internal/models"
	"gopkg.in/yaml.v3"
)

// PatternResult represents the result of pattern detection
type PatternResult struct {
	Triggered bool
	Pattern   string
	Severity  string
	Evidence  string
}

// DetectPatterns analyzes anomalies and detects circuit breaker patterns.
// Returns the first matching pattern (checked in priority order), or a non-triggered result if none match.
func DetectPatterns(anomalies []models.Anomaly) PatternResult {
	checks := []func([]models.Anomaly) PatternResult{
		checkRetryCluster,
		checkDebtAccumulation,
		checkAssumptionCascade,
		checkSpecGapCluster,
		checkWorkaroundPattern,
		checkExternalServiceOutage,
		checkProviderAuditDegradation,
	}
	for _, check := range checks {
		if result := check(anomalies); result.Triggered {
			return result
		}
	}
	return PatternResult{Triggered: false}
}

// DetectUnacknowledgedPatterns runs circuit-breaker detection on anomalies that
// have not already been acknowledged by a cleared circuit-breaker trigger.
//
// "Cleared" means circuit_breaker.status == OK and current_trigger == nil.
// In any active or partially stale triggered state, all anomalies are considered.
// When cleared, the latest TRIGGERED history timestamp is the acknowledgement
// watermark; only anomalies strictly after that timestamp are considered new.
func DetectUnacknowledgedPatterns(state *models.State) (PatternResult, []models.Anomaly, int) {
	if state == nil {
		return DetectPatterns(nil), nil, 0
	}

	considered, suppressedCount := UnacknowledgedAnomalies(state)
	return DetectPatterns(considered), considered, suppressedCount
}

// UnacknowledgedAnomalies returns the anomaly slice eligible for current
// circuit-breaker detection plus the number suppressed by the cleared-trigger
// acknowledgement watermark.
func UnacknowledgedAnomalies(state *models.State) ([]models.Anomaly, int) {
	if state == nil {
		return nil, 0
	}

	watermark, ok := latestClearedTriggerWatermark(state)
	if !ok {
		return state.Anomalies, 0
	}

	considered := make([]models.Anomaly, 0, len(state.Anomalies))
	for _, anomaly := range state.Anomalies {
		if anomaly.Timestamp.After(watermark) {
			considered = append(considered, anomaly)
		}
	}
	return considered, len(state.Anomalies) - len(considered)
}

func latestClearedTriggerWatermark(state *models.State) (time.Time, bool) {
	if state.CircuitBreaker.Status != "OK" || state.CircuitBreaker.CurrentTrigger != nil {
		return time.Time{}, false
	}

	var latest time.Time
	for _, entry := range state.CircuitBreaker.History {
		if entry.Result != "TRIGGERED" {
			continue
		}
		if latest.IsZero() || entry.Timestamp.After(latest) {
			latest = entry.Timestamp
		}
	}
	if latest.IsZero() {
		return time.Time{}, false
	}
	return latest, true
}

func checkRetryCluster(anomalies []models.Anomaly) PatternResult {
	retryLoops := filterByType(anomalies, "retry_loop")
	if len(retryLoops) < 3 {
		return PatternResult{Triggered: false}
	}

	// Count similar error patterns
	groups := groupByField(retryLoops, "error_pattern")
	for _, group := range groups {
		if len(group) >= 2 {
			// At least 2 anomalies share the same error pattern
			return PatternResult{
				Triggered: true,
				Pattern:   "retry_cluster",
				Severity:  "ARCHITECTURE_FLAW",
				Evidence:  fmt.Sprintf("%d retry_loop anomalies with similar error patterns", len(retryLoops)),
			}
		}
	}

	return PatternResult{Triggered: false}
}

func checkDebtAccumulation(anomalies []models.Anomaly) PatternResult {
	tradeOffs := filterByType(anomalies, "trade_off")

	debtCount := 0
	for _, a := range tradeOffs {
		if debtCreated, ok := a.Details["debt_created"].(bool); ok && debtCreated {
			debtCount++
		}
	}

	if debtCount >= 3 {
		return PatternResult{
			Triggered: true,
			Pattern:   "debt_accumulation",
			Severity:  "SCOPE_FLAW",
			Evidence:  fmt.Sprintf("%d trade-offs creating technical debt", debtCount),
		}
	}

	return PatternResult{Triggered: false}
}

func checkAssumptionCascade(anomalies []models.Anomaly) PatternResult {
	return checkGroupedThreshold(anomalies, "assumption_violated", "assumption", 2, PatternResult{
		Pattern:  "assumption_cascade",
		Severity: "SPEC_FLAW",
		Evidence: "Same assumption violated across multiple tasks",
	})
}

func checkSpecGapCluster(anomalies []models.Anomaly) PatternResult {
	return checkGroupedThreshold(anomalies, "spec_ambiguity", "spec_ref", 2, PatternResult{
		Pattern:  "spec_gap_cluster",
		Severity: "SPEC_FLAW",
		Evidence: "Multiple tasks hitting same spec ambiguity",
	})
}

func checkWorkaroundPattern(anomalies []models.Anomaly) PatternResult {
	var workarounds []models.Anomaly
	for _, a := range anomalies {
		if a.Type == "workaround" || a.Type == "trade_off" {
			workarounds = append(workarounds, a)
		}
	}

	if len(workarounds) < 2 {
		return PatternResult{Triggered: false}
	}

	// Group by root_cause (or "what" field as fallback)
	groups := make(map[string][]models.Anomaly)
	for _, a := range workarounds {
		key := ""
		if rootCause, ok := a.Details["root_cause"].(string); ok {
			key = rootCause
		} else if what, ok := a.Details["what"].(string); ok {
			key = what
		}

		if key != "" {
			groups[key] = append(groups[key], a)
		}
	}

	for _, group := range groups {
		if len(group) >= 2 {
			return PatternResult{
				Triggered: true,
				Pattern:   "workaround_pattern",
				Severity:  "ARCHITECTURE_FLAW",
				Evidence:  fmt.Sprintf("%d workarounds/trade-offs with similar root causes", len(workarounds)),
			}
		}
	}

	return PatternResult{Triggered: false}
}

func checkExternalServiceOutage(anomalies []models.Anomaly) PatternResult {
	externals := filterByType(anomalies, "external_blocker")
	groups := groupByField(externals, "blocker_service")
	for service, group := range groups {
		if len(group) >= 2 {
			return PatternResult{
				Triggered: true,
				Pattern:   "external_service_outage",
				Severity:  "EXTERNAL_DEPENDENCY",
				Evidence:  fmt.Sprintf("Multiple tasks blocked by same external service: %s", service),
			}
		}
	}
	return PatternResult{Triggered: false}
}

func checkProviderAuditDegradation(anomalies []models.Anomaly) PatternResult {
	auditDegraded := filterByType(anomalies, "provider_audit_degraded")
	groups := groupByField(auditDegraded, "provider")
	for provider, group := range groups {
		agents := make(map[string]bool)
		for _, anomaly := range group {
			if agentID, ok := anomaly.Details["agent_id"].(string); ok && agentID != "" {
				agents[agentID] = true
			}
		}
		if len(agents) >= 2 || len(group) >= 3 {
			return PatternResult{
				Triggered: true,
				Pattern:   "provider_audit_degradation",
				Severity:  "OBSERVABILITY_DEGRADED",
				Evidence:  fmt.Sprintf("%d provider_audit_degraded anomalies for provider %s across %d agents", len(group), provider, len(agents)),
			}
		}
	}
	return PatternResult{Triggered: false}
}

// checkGroupedThreshold detects patterns where anomalies of a given type share a field value
// at or above a threshold count. The result template is returned with Triggered set to true
// when the threshold is met.
func checkGroupedThreshold(anomalies []models.Anomaly, anomalyType, field string, threshold int, result PatternResult) PatternResult {
	filtered := filterByType(anomalies, anomalyType)
	groups := groupByField(filtered, field)
	for _, group := range groups {
		if len(group) >= threshold {
			result.Triggered = true
			return result
		}
	}
	return PatternResult{Triggered: false}
}

func filterByType(anomalies []models.Anomaly, anomalyType string) []models.Anomaly {
	var result []models.Anomaly
	for _, a := range anomalies {
		if a.Type == anomalyType {
			result = append(result, a)
		}
	}
	return result
}

// groupByField groups anomalies by a field value in their Details
func groupByField(anomalies []models.Anomaly, field string) map[string][]models.Anomaly {
	groups := make(map[string][]models.Anomaly)
	for _, a := range anomalies {
		if value, ok := a.Details[field].(string); ok && value != "" {
			groups[value] = append(groups[value], a)
		}
	}
	return groups
}

// GenerateReport creates a markdown report for a triggered circuit breaker.
// The anomalies slice must be the exact anomaly set considered for detection.
func GenerateReport(result PatternResult, anomalies []models.Anomaly, timestamp time.Time, suppressedCount int) string {
	var sb strings.Builder

	sb.WriteString("# Circuit Breaker Report\n\n")
	fmt.Fprintf(&sb, "**Triggered:** %s\n", timestamp.Format(time.RFC3339))
	fmt.Fprintf(&sb, "**Pattern:** %s\n", result.Pattern)
	fmt.Fprintf(&sb, "**Severity:** %s\n\n", result.Severity)
	if suppressedCount > 0 {
		fmt.Fprintf(&sb, "**Acknowledged anomalies suppressed:** %d\n\n", suppressedCount)
	}

	sb.WriteString("## Trigger Evidence\n\n")
	sb.WriteString(result.Evidence)
	sb.WriteString("\n\n")

	writeTrimmedAnomalies(&sb, anomalies)

	sb.WriteString("## Anomalies (raw)\n\n")
	sb.WriteString("```yaml\n")
	yamlData, err := yaml.Marshal(anomalies)
	if err != nil {
		fmt.Fprintf(&sb, "Error marshaling anomalies: %v\n", err)
	} else {
		sb.Write(yamlData)
	}

	sb.WriteString("```\n\n")

	sb.WriteString("## Human Decision Required\n\n")
	sb.WriteString("- [ ] Acknowledge report\n")
	sb.WriteString("- [ ] Confirm severity assessment\n")
	sb.WriteString("- [ ] Determine remediation\n")
	sb.WriteString("- [ ] Release checkpoint with decision logged\n")

	return sb.String()
}

func writeTrimmedAnomalies(sb *strings.Builder, anomalies []models.Anomaly) {
	sb.WriteString("## Anomalies (trimmed)\n\n")
	if len(anomalies) == 0 {
		sb.WriteString("_None_\n\n")
		return
	}

	for i, anomaly := range anomalies {
		task := anomaly.Task
		if task == "" {
			task = "-"
		}
		fmt.Fprintf(sb, "%d. `%s` at `%s`\n", i+1, anomaly.Type, anomaly.Timestamp.Format(time.RFC3339Nano))
		fmt.Fprintf(sb, "   - task: `%s`\n", task)
		fmt.Fprintf(sb, "   - reporter: `%s`\n", anomaly.Reporter)
		writeTrimmedDetail(sb, "provider", anomaly.Details)
		writeTrimmedDetail(sb, "agent_id", anomaly.Details)
		writeTrimmedDetail(sb, "impact", anomaly.Details)
		writeTrimmedMessage(sb, anomaly.Details)
		sb.WriteString("\n")
	}
}

func writeTrimmedDetail(sb *strings.Builder, key string, details map[string]any) {
	value, ok := details[key].(string)
	if !ok || value == "" {
		return
	}
	fmt.Fprintf(sb, "   - %s: `%s`\n", key, compactText(value, 240))
}

func writeTrimmedMessage(sb *strings.Builder, details map[string]any) {
	message, ok := details["message"].(string)
	if !ok || message == "" {
		return
	}

	if writeProviderMessageSummary(sb, message) {
		return
	}
	fmt.Fprintf(sb, "   - message_excerpt: `%s`\n", compactText(message, 500))
}

func writeProviderMessageSummary(sb *strings.Builder, message string) bool {
	var event struct {
		Type string `json:"type"`
		Item struct {
			Type             string `json:"type"`
			Command          string `json:"command"`
			AggregatedOutput string `json:"aggregated_output"`
			ExitCode         *int   `json:"exit_code"`
			Status           string `json:"status"`
		} `json:"item"`
	}
	if err := json.Unmarshal([]byte(message), &event); err != nil {
		return false
	}

	wrote := false
	if event.Type != "" || event.Item.Type != "" {
		fmt.Fprintf(sb, "   - provider_event: `%s / %s`\n", compactText(event.Type, 80), compactText(event.Item.Type, 80))
		wrote = true
	}
	if event.Item.Command != "" {
		fmt.Fprintf(sb, "   - command: `%s`\n", compactText(event.Item.Command, 240))
		wrote = true
	}
	if event.Item.Status != "" || event.Item.ExitCode != nil {
		exitCode := "-"
		if event.Item.ExitCode != nil {
			exitCode = fmt.Sprintf("%d", *event.Item.ExitCode)
		}
		fmt.Fprintf(sb, "   - status: `%s`, exit_code: `%s`\n", compactText(event.Item.Status, 80), exitCode)
		wrote = true
	}
	if event.Item.AggregatedOutput != "" {
		fmt.Fprintf(sb, "   - aggregated_output_chars: `%d`\n", len(event.Item.AggregatedOutput))
		wrote = true
	}
	return wrote
}

func compactText(value string, limit int) string {
	compacted := strings.Join(strings.Fields(value), " ")
	runes := []rune(compacted)
	if len(runes) <= limit {
		return strings.ReplaceAll(compacted, "`", "'")
	}
	return strings.ReplaceAll(string(runes[:limit])+"...", "`", "'")
}

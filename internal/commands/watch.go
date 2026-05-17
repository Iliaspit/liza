package commands

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/liza-mas/liza/internal/analysis"
	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/ops"
	"github.com/liza-mas/liza/internal/paths"
	"github.com/liza-mas/liza/internal/pipeline"
)

const (
	DefaultCheckInterval         = 10 * time.Second
	StallThreshold               = 30 * time.Minute
	StaleDraftThreshold          = 30 * time.Minute
	CheckpointStaleThreshold     = 30 * time.Minute
	CheckpointStuckThreshold     = 2 * time.Hour
	CheckpointAbandonedThreshold = 8 * time.Hour
	PauseStaleThreshold          = 30 * time.Minute
	PauseForgottenThreshold      = 2 * time.Hour
	OrphanedGracePeriod          = 30 * time.Second
	StaleSentinelThreshold       = 2 * time.Minute
)

const stuckAlertCachePrefix = "stuck-alert:"

// AlertLevel is the severity of a watch alert.
type AlertLevel string

const (
	AlertLevelWarning  AlertLevel = "⚠️"
	AlertLevelCritical AlertLevel = "🚨"
)

// Alert represents a single anomaly alert from watch checks.
type Alert struct {
	Timestamp time.Time
	Level     AlertLevel
	Category  string
	Message   string
}

// AlertSnapshot contains both newly emitted alerts and the complete set of
// currently active alert identities for consumers that need freshness state.
type AlertSnapshot struct {
	Alerts     []Alert
	ActiveKeys map[string]bool
}

func (a Alert) String() string {
	return fmt.Sprintf("[%s] %s %s: %s",
		a.Timestamp.UTC().Format(time.RFC3339),
		a.Level,
		a.Category,
		a.Message)
}

// ParseAlertLine parses a line written by Alert.String() back into an Alert.
// Returns the parsed alert and true on success, or zero value and false on
// malformed input.
//
// Format: [<RFC3339>] <level> <CATEGORY>: <message>
func ParseAlertLine(line string) (Alert, bool) {
	if !strings.HasPrefix(line, "[") {
		return Alert{}, false
	}
	closeBracket := strings.IndexByte(line, ']')
	if closeBracket < 0 {
		return Alert{}, false
	}
	ts, err := time.Parse(time.RFC3339, line[1:closeBracket])
	if err != nil {
		return Alert{}, false
	}

	rest := strings.TrimLeft(line[closeBracket+1:], " ")

	// Split "<level> <CATEGORY>: <message>" on first ": ".
	colonIdx := strings.Index(rest, ": ")
	if colonIdx < 0 {
		return Alert{}, false
	}
	levelAndCategory := rest[:colonIdx]
	message := rest[colonIdx+2:]

	spaceIdx := strings.IndexByte(levelAndCategory, ' ')
	if spaceIdx < 0 {
		return Alert{}, false
	}
	level := AlertLevel(levelAndCategory[:spaceIdx])
	category := strings.TrimSpace(levelAndCategory[spaceIdx+1:])

	return Alert{
		Timestamp: ts,
		Level:     level,
		Category:  category,
		Message:   message,
	}, true
}

type WatchConfig struct {
	ProjectRoot   string
	CheckInterval time.Duration
	AlertsLog     string
	// StateCache is used to track seen alerts across checks
	StateCache map[string]time.Time
}

func WatchCommand(ctx context.Context, config WatchConfig) error {
	if config.CheckInterval == 0 {
		config.CheckInterval = DefaultCheckInterval
	}
	lizaPaths := paths.New(config.ProjectRoot)
	if config.AlertsLog == "" {
		config.AlertsLog = lizaPaths.AlertsLogPath()
	}
	if config.StateCache == nil {
		config.StateCache = make(map[string]time.Time)
	}

	fmt.Printf("[%s] Watching %s\n",
		time.Now().UTC().Format("15:04:05"),
		lizaPaths.LizaDir())

	ticker := time.NewTicker(config.CheckInterval)
	defer ticker.Stop()

	if err := runChecks(ctx, config); err != nil {
		fmt.Fprintf(os.Stderr, "Check error: %v\n", err)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := runChecks(ctx, config); err != nil {
				fmt.Fprintf(os.Stderr, "Check error: %v\n", err)
			}
		}
	}
}

func runChecks(_ context.Context, config WatchConfig) error {
	lizaPaths := paths.New(config.ProjectRoot)
	statePath := lizaPaths.StatePath()

	if _, err := os.Stat(statePath); os.IsNotExist(err) {
		return nil
	}

	bb := db.For(statePath)
	state, err := bb.Read()
	if err != nil {
		return fmt.Errorf("failed to read state: %w", err)
	}

	alerts := RunChecksWithState(state, config)

	for _, a := range alerts {
		if err := WriteAlert(config.AlertsLog, a); err != nil {
			return fmt.Errorf("failed to write alert: %w", err)
		}
		fmt.Fprintln(os.Stderr, a.String())
	}

	return nil
}

// RunChecksWithState runs all 13 anomaly checks plus circuit breaker,
// sprint stalled, and state validity checks against the provided state.
// The config.StateCache is modified in place for alert throttling.
func RunChecksWithState(state *models.State, config WatchConfig) []Alert {
	return RunChecksWithStateSnapshot(state, config).Alerts
}

// RunChecksWithStateSnapshot runs checks and returns emitted alerts plus active
// alert keys before throttling. The config.StateCache is modified in place.
func RunChecksWithStateSnapshot(state *models.State, config WatchConfig) AlertSnapshot {
	if config.StateCache == nil {
		config.StateCache = make(map[string]time.Time)
	}

	var alerts []Alert

	// Load pipeline resolver once for checks that need it.
	pr, prErr := ops.LoadResolverForModels(config.ProjectRoot)
	pipelineCacheKey := "pipeline-config-error"
	if prErr != nil && !errors.Is(prErr, pipeline.ErrConfigNotFound) {
		// Malformed config: emit one-time alert, don't spam every 10s tick.
		if _, seen := config.StateCache[pipelineCacheKey]; !seen {
			alerts = append(alerts, Alert{
				Timestamp: time.Now().UTC(),
				Level:     AlertLevelWarning,
				Category:  "PIPELINE CONFIG",
				Message:   prErr.Error(),
			})
			config.StateCache[pipelineCacheKey] = time.Now().UTC()
		}
	} else {
		// Clear on success (or ErrConfigNotFound) so a later regression re-alerts.
		delete(config.StateCache, pipelineCacheKey)
	}
	// pr is nil on any error — pipeline-aware checks skip gracefully.

	lizaPaths := paths.New(config.ProjectRoot)
	checks := []func() []Alert{
		func() []Alert { return checkExpiredLeases(state) },
		func() []Alert { return checkRunningTasksWithoutLiveProcess(state, pr) },
		func() []Alert { return checkBlockedTasks(state, config.StateCache) },
		func() []Alert { return checkOrphanedRejected(state, config.StateCache) },
		func() []Alert { return checkReviewLoops(state) },
		func() []Alert { return checkIntegrationFailures(state, config.ProjectRoot) },
		func() []Alert { return checkHypothesisExhaustion(state) },
		func() []Alert { return checkReassigned(state, config.StateCache) },
		func() []Alert { return checkApproachingLimits(state) },
		func() []Alert { return checkStaleSentinels(state, config.StateCache) },
		func() []Alert { return checkStalled(state, config.StateCache) },
		func() []Alert { return checkStaleDrafts(state) },
		func() []Alert { return checkImmediateDiscoveries(state) },
		func() []Alert { return checkMissingRoles(state, pr, config.StateCache) },
	}
	for _, check := range checks {
		alerts = append(alerts, check()...)
	}

	alerts = append(alerts, checkCircuitBreakerEscalation(state, config.StateCache)...)
	alerts = append(alerts, checkSprintStalled(state, config.StateCache)...)

	statePath := lizaPaths.StatePath()
	if err := ValidateCommand(statePath, true); err != nil {
		alerts = append(alerts, Alert{
			Timestamp: time.Now().UTC(),
			Level:     AlertLevelCritical,
			Category:  "INVALID STATE",
			Message:   err.Error(),
		})
	}

	activeKeys := activeAlertKeys(alerts)
	return AlertSnapshot{
		Alerts:     reconcileStuckAlerts(alerts, config.StateCache),
		ActiveKeys: activeKeys,
	}
}

func activeAlertKeys(alerts []Alert) map[string]bool {
	keys := make(map[string]bool, len(alerts))
	for _, alert := range alerts {
		if !isFreshnessTrackedAlertCategory(alert.Category) {
			continue
		}
		keys[AlertKey(alert)] = true
	}
	return keys
}

// AlertKey returns a stable identity for an alert condition.
func AlertKey(alert Alert) string {
	return alert.Category + ":" + alert.Message
}

func isFreshnessTrackedAlertCategory(category string) bool {
	// Only track categories that are recomputed and emitted on every check while
	// active, or are deduped after building a full active set. Categories with
	// internal throttle caches, such as STALLED or STALE SENTINEL, need their
	// checks to expose active identities before the TUI can safely resolve them.
	if isStuckAlertCategory(category) {
		return true
	}
	switch category {
	case "LEASE EXPIRED", "REVIEW LEASE EXPIRED":
		return true
	default:
		return false
	}
}

func reconcileStuckAlerts(alerts []Alert, cache map[string]time.Time) []Alert {
	if cache == nil {
		return alerts
	}

	now := time.Now().UTC()
	activeKeys := make(map[string]bool)
	deduped := make([]Alert, 0, len(alerts))

	for _, alert := range alerts {
		if !isStuckAlertCategory(alert.Category) {
			deduped = append(deduped, alert)
			continue
		}

		key := stuckAlertCacheKey(alert)
		activeKeys[key] = true
		if _, seen := cache[key]; seen {
			continue
		}
		cache[key] = now
		deduped = append(deduped, alert)
	}

	for key := range cache {
		if strings.HasPrefix(key, stuckAlertCachePrefix) && !activeKeys[key] {
			delete(cache, key)
		}
	}

	return deduped
}

func isStuckAlertCategory(category string) bool {
	switch category {
	case "HYPOTHESIS EXHAUSTION", "INTEGRATION FAILED", "INVALID STATE", "DEAD AGENT PROCESS":
		return true
	default:
		return false
	}
}

func stuckAlertCacheKey(alert Alert) string {
	return stuckAlertCachePrefix + AlertKey(alert)
}

func checkCircuitBreakerEscalation(state *models.State, cache map[string]time.Time) []Alert {
	mode := state.Config.Mode
	if mode == "" {
		mode = models.SystemModeRunning
	}

	// Only check during active execution.
	if mode != models.SystemModeRunning || state.Sprint.Status != models.SprintStatusInProgress {
		delete(cache, "circuit_breaker:alert")
		return nil
	}

	// Keep both checks: manual edits or interrupted writes can leave one field stale.
	// Either value indicates a previously triggered circuit-breaker state.
	if state.CircuitBreaker.Status == "TRIGGERED" || state.CircuitBreaker.CurrentTrigger != nil {
		delete(cache, "circuit_breaker:alert")
		return nil
	}

	patternResult := analysis.DetectPatterns(state.Anomalies)
	if !patternResult.Triggered {
		delete(cache, "circuit_breaker:alert")
		return nil
	}

	// Throttle: only alert once per triggered period.
	if _, seen := cache["circuit_breaker:alert"]; seen {
		return nil
	}

	cache["circuit_breaker:alert"] = time.Now().UTC()
	return []Alert{{
		Timestamp: time.Now().UTC(),
		Level:     AlertLevelCritical,
		Category:  "CIRCUIT BREAKER",
		Message: fmt.Sprintf("pattern=%s severity=%s — run 'liza analyze' then 'liza sprint-checkpoint'",
			patternResult.Pattern, patternResult.Severity),
	}}
}

func checkSprintStalled(state *models.State, cache map[string]time.Time) []Alert {
	mode := state.Config.Mode
	if mode == "" {
		mode = models.SystemModeRunning
	}

	if mode != models.SystemModeRunning || state.Sprint.Status != models.SprintStatusInProgress {
		// Clear throttle when sprint leaves IN_PROGRESS (e.g. after checkpoint).
		// This ensures that if the human resumes without unblocking tasks,
		// the next stall detection re-triggers a fresh alert.
		delete(cache, "sprint_stalled:alert")
		return nil
	}

	if !state.SprintStalled() {
		delete(cache, "sprint_stalled:alert")
		return nil
	}

	// Throttle: only alert once per stall event within a single IN_PROGRESS period.
	// The sprint status guard above resets the throttle across checkpoint/resume cycles.
	if _, seen := cache["sprint_stalled:alert"]; seen {
		return nil
	}

	blockedCount := 0
	for _, taskID := range state.Sprint.Scope.Planned {
		task := state.FindTask(taskID)
		if task != nil && task.Status == models.TaskStatusBlocked {
			blockedCount++
		}
	}

	cache["sprint_stalled:alert"] = time.Now().UTC()
	return []Alert{{
		Timestamp: time.Now().UTC(),
		Level:     AlertLevelCritical,
		Category:  "SPRINT STALLED",
		Message: fmt.Sprintf("all %d non-terminal planned tasks are BLOCKED",
			blockedCount),
	}}
}

func checkExpiredLeases(state *models.State) []Alert {
	var alerts []Alert
	now := time.Now().UTC()
	graceDeadline := now.Add(-models.LeaseExpiryGracePeriod)

	for agentID, agent := range state.Agents {
		if agent.CurrentTask == nil {
			continue
		}
		if agent.LeaseExpires == nil {
			continue
		}
		if agent.LeaseExpires.Before(graceDeadline) {
			alerts = append(alerts, Alert{
				Timestamp: now,
				Level:     AlertLevelWarning,
				Category:  "LEASE EXPIRED",
				Message:   fmt.Sprintf("%s on %s", agentID, *agent.CurrentTask),
			})
		}
	}

	for _, task := range state.Tasks {
		if task.Status != models.TaskStatusReviewing {
			continue
		}
		if task.ReviewingBy == nil {
			continue
		}
		if task.ReviewLeaseExpires == nil {
			continue
		}
		if task.ReviewLeaseExpires.Before(graceDeadline) {
			alerts = append(alerts, Alert{
				Timestamp: now,
				Level:     AlertLevelWarning,
				Category:  "REVIEW LEASE EXPIRED",
				Message:   fmt.Sprintf("%s on %s — review can be reclaimed", *task.ReviewingBy, task.ID),
			})
		}
	}

	return alerts
}

func checkRunningTasksWithoutLiveProcess(state *models.State, pr models.PipelineResolver) []Alert {
	if pr == nil {
		return nil
	}

	var alerts []Alert
	now := time.Now().UTC()

	for i := range state.Tasks {
		task := &state.Tasks[i]
		ownerID, ownerKind, ok := runningTaskOwner(task, pr)
		if !ok {
			continue
		}

		agent, exists := state.Agents[ownerID]
		if !exists {
			alerts = append(alerts, Alert{
				Timestamp: now,
				Level:     AlertLevelCritical,
				Category:  "DEAD AGENT PROCESS",
				Message: fmt.Sprintf("%s — status %s has %s %s but no registered agent",
					task.ID, task.Status, ownerKind, ownerID),
			})
			continue
		}
		if ops.IsProcessAlive(agent.PID) {
			continue
		}

		alerts = append(alerts, Alert{
			Timestamp: now,
			Level:     AlertLevelCritical,
			Category:  "DEAD AGENT PROCESS",
			Message: fmt.Sprintf("%s — status %s has %s %s but no live process (pid %d)",
				task.ID, task.Status, ownerKind, ownerID, agent.PID),
		})
	}

	return alerts
}

func runningTaskOwner(task *models.Task, pr models.PipelineResolver) (string, string, bool) {
	if models.IsExecutingStatus(task, pr) {
		if task.AssignedTo == nil || *task.AssignedTo == "" {
			return "", "", false
		}
		if strings.HasPrefix(*task.AssignedTo, "$") {
			return "", "", false
		}
		return *task.AssignedTo, "doer", true
	}
	if isReviewerActiveStatus(task, pr) {
		if task.ReviewingBy == nil || *task.ReviewingBy == "" {
			return "", "", false
		}
		return *task.ReviewingBy, "reviewer", true
	}
	return "", "", false
}

func isReviewerActiveStatus(task *models.Task, pr models.PipelineResolver) bool {
	if task.RolePair == "" || pr == nil {
		return false
	}
	reviewing, err := pr.ReviewingStatus(task.RolePair)
	if err == nil && task.Status == reviewing {
		return true
	}
	reviewing2, err := pr.Reviewing2Status(task.RolePair)
	return err == nil && task.Status == reviewing2
}

func checkBlockedTasks(state *models.State, cache map[string]time.Time) []Alert {
	var alerts []Alert
	now := time.Now().UTC()

	for _, task := range state.Tasks {
		if task.Status != models.TaskStatusBlocked {
			continue
		}

		cacheKey := "blocked:" + task.ID
		if _, seen := cache[cacheKey]; !seen {
			reason := "no reason"
			if task.BlockedReason != nil {
				reason = *task.BlockedReason
			}
			alerts = append(alerts, Alert{
				Timestamp: now,
				Level:     AlertLevelWarning,
				Category:  "BLOCKED",
				Message:   fmt.Sprintf("%s — %s", task.ID, reason),
			})
			cache[cacheKey] = now
		}
	}

	return alerts
}

func checkOrphanedRejected(state *models.State, cache map[string]time.Time) []Alert {
	var alerts []Alert
	now := time.Now().UTC()

	for _, task := range state.Tasks {
		if task.Status != models.TaskStatusRejected {
			continue
		}
		if task.AssignedTo == nil {
			continue
		}

		// Sentinel AssignedTo (e.g. "$transitioning") is a transition in
		// progress, not an orphaned assignment. Clear any stale cache entry
		// from before the transition to prevent false-positive alerts when
		// the task becomes genuinely orphaned later.
		if strings.HasPrefix(*task.AssignedTo, "$") {
			delete(cache, "orphaned:"+task.ID)
			continue
		}

		assignee := *task.AssignedTo
		agent, exists := state.Agents[assignee]
		agentStatus := "MISSING"
		if exists {
			agentStatus = string(agent.Status)
		}

		if agentStatus == "WORKING" {
			delete(cache, "orphaned:"+task.ID)
			continue
		}

		cacheKey := "orphaned:" + task.ID
		firstSeen, seen := cache[cacheKey]
		if !seen {
			cache[cacheKey] = now
			continue
		}
		if now.Sub(firstSeen) > OrphanedGracePeriod {
			alerts = append(alerts, Alert{
				Timestamp: now,
				Level:     AlertLevelCritical,
				Category:  "ORPHANED REJECTED",
				Message: fmt.Sprintf("%s — assigned to %s but agent is %s (orphaned %ds+)",
					task.ID, assignee, agentStatus, int(OrphanedGracePeriod.Seconds())),
			})
			delete(cache, cacheKey)
		}
	}

	return alerts
}

func checkReviewLoops(state *models.State) []Alert {
	var alerts []Alert
	now := time.Now().UTC()

	for _, task := range state.Tasks {
		if task.Status.IsTerminal() {
			continue
		}
		if task.ReviewCyclesCurrent >= 5 {
			alerts = append(alerts, Alert{
				Timestamp: now,
				Level:     AlertLevelCritical,
				Category:  "REVIEW LOOP",
				Message:   fmt.Sprintf("%s — %d cycles (at cliff)", task.ID, task.ReviewCyclesCurrent),
			})
		}
	}

	return alerts
}

func checkIntegrationFailures(state *models.State, projectRoot string) []Alert {
	var alerts []Alert
	now := time.Now().UTC()

	for _, task := range state.Tasks {
		if task.Status == models.TaskStatusIntegrationFailed {
			alerts = append(alerts, Alert{
				Timestamp: now,
				Level:     AlertLevelCritical,
				Category:  "INTEGRATION FAILED",
				Message:   integrationFailureAlertMessage(&task, projectRoot),
			})
		}
	}

	return alerts
}

func checkHypothesisExhaustion(state *models.State) []Alert {
	var alerts []Alert
	now := time.Now().UTC()

	for _, task := range state.Tasks {
		if task.Status.IsTerminal() {
			continue
		}
		if task.Status == models.TaskStatusBlocked {
			continue
		}
		if len(task.FailedBy) >= 2 {
			alerts = append(alerts, Alert{
				Timestamp: now,
				Level:     AlertLevelCritical,
				Category:  "HYPOTHESIS EXHAUSTION",
				Message:   fmt.Sprintf("%s — requires rescope", task.ID),
			})
		}
	}

	return alerts
}

func checkReassigned(state *models.State, cache map[string]time.Time) []Alert {
	var alerts []Alert
	now := time.Now().UTC()

	for _, task := range state.Tasks {
		if task.Status.IsTerminal() {
			continue
		}
		if task.EffectiveAttempt() != 2 {
			continue
		}

		cacheKey := "attempt2:" + task.ID
		if _, seen := cache[cacheKey]; !seen {
			alerts = append(alerts, Alert{
				Timestamp: now,
				Level:     AlertLevelWarning,
				Category:  "ATTEMPT",
				Message:   fmt.Sprintf("%s — attempt 2 (final attempt)", task.ID),
			})
			cache[cacheKey] = now
		}
	}

	return alerts
}

func checkApproachingLimits(state *models.State) []Alert {
	var alerts []Alert
	now := time.Now().UTC()

	for _, task := range state.Tasks {
		attemptNum := task.EffectiveAttempt()

		// Coder iterations: warn at 8, cliff at 10
		if task.Status == models.TaskStatusImplementing && task.Iteration >= 8 && task.Iteration < 10 {
			var msg string
			if attemptNum == 2 {
				msg = fmt.Sprintf("%s — attempt 2 (final), iteration %d/10", task.ID, task.Iteration)
			} else {
				msg = fmt.Sprintf("%s — attempt %d, iteration %d/10", task.ID, attemptNum, task.Iteration)
			}
			alerts = append(alerts, Alert{
				Timestamp: now,
				Level:     AlertLevelWarning,
				Category:  "APPROACHING LIMIT",
				Message:   msg,
			})
		}

		// Review cycles: warn at 3, cliff at 5 (only for non-terminal tasks)
		if !task.Status.IsTerminal() && task.ReviewCyclesCurrent >= 3 && task.ReviewCyclesCurrent < 5 {
			var msg string
			if attemptNum == 2 {
				msg = fmt.Sprintf("%s — attempt 2 (final), review cycle %d/5", task.ID, task.ReviewCyclesCurrent)
			} else {
				msg = fmt.Sprintf("%s — attempt %d, review cycle %d/5", task.ID, attemptNum, task.ReviewCyclesCurrent)
			}
			alerts = append(alerts, Alert{
				Timestamp: now,
				Level:     AlertLevelWarning,
				Category:  "APPROACHING LIMIT",
				Message:   msg,
			})
		}
	}

	return alerts
}

func checkStaleSentinels(state *models.State, cache map[string]time.Time) []Alert {
	var alerts []Alert
	now := time.Now().UTC()

	activeSentinels := make(map[string]bool)

	for _, task := range state.Tasks {
		if task.AssignedTo == nil || !strings.HasPrefix(*task.AssignedTo, "$") {
			continue
		}
		activeSentinels[task.ID] = true

		cacheKey := "sentinel:" + task.ID
		firstSeen, seen := cache[cacheKey]
		if !seen {
			cache[cacheKey] = now
			continue
		}
		if now.Sub(firstSeen) > StaleSentinelThreshold {
			alerts = append(alerts, Alert{
				Timestamp: now,
				Level:     AlertLevelCritical,
				Category:  "STALE SENTINEL",
				Message:   fmt.Sprintf("%s stuck in transition — manual repair needed", task.ID),
			})
		}
	}

	// Clear cache entries for sentinels that resolved.
	for key := range cache {
		if !strings.HasPrefix(key, "sentinel:") {
			continue
		}
		taskID := strings.TrimPrefix(key, "sentinel:")
		if !activeSentinels[taskID] {
			delete(cache, key)
		}
	}

	return alerts
}

// checkStalled detects stalled progress by finding the latest task history
// timestamp across all tasks. Heartbeat writes do not create history entries,
// so this signal is immune to lease-renewal traffic. Falls back to the earliest
// task Created time when no history exists. Throttles alerts to once every 5 minutes.
func checkStalled(state *models.State, cache map[string]time.Time) []Alert {
	var alerts []Alert
	now := time.Now().UTC()

	// Find latest history timestamp and check for active tasks.
	var latestProgress time.Time
	hasActive := false
	for i := range state.Tasks {
		task := &state.Tasks[i]
		if !task.Status.IsTerminal() {
			hasActive = true
		}
		for j := range task.History {
			if task.History[j].Time.After(latestProgress) {
				latestProgress = task.History[j].Time
			}
		}
	}

	if !hasActive {
		delete(cache, "stalled:alert")
		return alerts
	}

	// No history entries: fall back to earliest Created.
	if latestProgress.IsZero() {
		for i := range state.Tasks {
			created := state.Tasks[i].Created
			if latestProgress.IsZero() || created.Before(latestProgress) {
				latestProgress = created
			}
		}
	}

	if latestProgress.IsZero() {
		return alerts
	}

	age := now.Sub(latestProgress)
	if age <= StallThreshold {
		delete(cache, "stalled:alert")
		return alerts
	}

	cacheKey := "stalled:alert"
	lastAlert, seen := cache[cacheKey]
	if !seen || now.Sub(lastAlert) >= 5*time.Minute {
		alerts = append(alerts, Alert{
			Timestamp: now,
			Level:     AlertLevelWarning,
			Category:  "STALLED",
			Message:   fmt.Sprintf("no task progress for %d minutes", int(age.Minutes())),
		})
		cache[cacheKey] = now
	}

	return alerts
}

func checkStaleDrafts(state *models.State) []Alert {
	var alerts []Alert
	now := time.Now().UTC()

	for _, task := range state.Tasks {
		if task.Status != models.TaskStatusDraft {
			continue
		}

		age := now.Sub(task.Created)
		if age > StaleDraftThreshold {
			alerts = append(alerts, Alert{
				Timestamp: now,
				Level:     AlertLevelWarning,
				Category:  "STALE DRAFT",
				Message: fmt.Sprintf("%s — created %dmin ago, never finalized (Orchestrator crash?)",
					task.ID, int(age.Minutes())),
			})
		}
	}

	return alerts
}

func checkImmediateDiscoveries(state *models.State) []Alert {
	var alerts []Alert
	now := time.Now().UTC()

	for _, disc := range state.Discovered {
		if disc.Urgency == "immediate" && disc.ConvertedToTask == nil {
			alerts = append(alerts, Alert{
				Timestamp: now,
				Level:     AlertLevelCritical,
				Category:  "IMMEDIATE DISCOVERY",
				Message:   fmt.Sprintf("%s — %s (Orchestrator should wake)", disc.ID, disc.Description),
			})
		}
	}

	return alerts
}

// checkMissingRoles alerts when claimable tasks exist but no agent of the
// required role is registered. This catches a common first-user mistake (e.g.,
// starting only a coder but not a code-planner).
//
// Design trade-off: Uses IsClaimable which checks both status AND dependency
// satisfaction, so this only alerts when tasks are *immediately* stuck. Tasks
// blocked by unmet deps won't trigger an alert even if the needed role is
// missing — the alert fires later when deps resolve. This is conservative
// (fewer false positives) at the cost of delayed detection.
func checkMissingRoles(state *models.State, pr models.PipelineResolver, cache map[string]time.Time) []Alert {
	if pr == nil {
		return nil
	}

	// Emit alerts for each missing role, throttled by cache.
	var alerts []Alert
	now := time.Now().UTC()

	missingRoles := FindMissingRolesWithClaimableWork(state, pr)
	missingRoleSet := make(map[string]bool, len(missingRoles))
	for _, roleWork := range missingRoles {
		role := roleWork.Role
		taskIDs := roleWork.TaskIDs
		missingRoleSet[role] = true
		cacheKey := "missing-role:" + role
		if _, seen := cache[cacheKey]; seen {
			continue
		}

		// Format task list, capping at 5 IDs.
		const maxListed = 5
		listed := taskIDs
		suffix := ""
		if len(taskIDs) > maxListed {
			listed = taskIDs[:maxListed]
			suffix = fmt.Sprintf("... and %d more", len(taskIDs)-maxListed)
		}
		msg := fmt.Sprintf("no registered agent for role %s — %d task(s) waiting (%s",
			role, roleWork.TaskCount, strings.Join(listed, ", "))
		if suffix != "" {
			msg += ", " + suffix
		}
		msg += "); run `liza repair-agent-pool --dry-run` to preview repair"

		alerts = append(alerts, Alert{
			Timestamp: now,
			Level:     AlertLevelWarning,
			Category:  "MISSING ROLE",
			Message:   msg,
		})
		cache[cacheKey] = now
	}

	// Clear cache entries for roles no longer in the missing set — either because
	// an agent appeared or because the waiting tasks stopped being claimable
	// (merged, abandoned, deps unmet, etc.). Without this, a stale cache entry
	// would suppress the alert if a *new* task later becomes claimable for the
	// same absent role.
	for key := range cache {
		if !strings.HasPrefix(key, "missing-role:") {
			continue
		}
		role := strings.TrimPrefix(key, "missing-role:")
		if !missingRoleSet[role] {
			delete(cache, key)
		}
	}

	return alerts
}

// WriteAlert appends an alert to the alerts log file.
func WriteAlert(alertsLog string, a Alert) error {
	f, err := os.OpenFile(alertsLog, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open alerts log: %w", err)
	}
	defer f.Close()

	_, err = fmt.Fprintln(f, a.String())
	if err != nil {
		return fmt.Errorf("failed to write alert: %w", err)
	}

	return nil
}

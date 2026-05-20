package commands

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/ops"
	"github.com/liza-mas/liza/internal/paths"
	"github.com/liza-mas/liza/internal/testhelpers"
)

func TestCheckExpiredLeases(t *testing.T) {
	now := time.Now().UTC()

	tests := []struct {
		name       string
		state      *models.State
		wantAlerts int
		validate   func(*testing.T, []Alert)
	}{
		{
			name: "expired coder lease",
			state: &models.State{
				Tasks: []models.Task{
					func() models.Task {
						task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusImplementing, now)
						return task
					}(),
				},
				Agents: map[string]models.Agent{
					"coder-1": {
						Status:      models.AgentStatusWorking,
						CurrentTask: testhelpers.StringPtr("task-1"),
						// Expired lease (past grace period)
						LeaseExpires: testhelpers.TimePtr(now.Add(-3 * time.Minute)),
					},
				},
			},
			wantAlerts: 1,
			validate: func(t *testing.T, alerts []Alert) {
				if alerts[0].Category != "LEASE EXPIRED" {
					t.Errorf("Category = %q, want %q", alerts[0].Category, "LEASE EXPIRED")
				}
				if alerts[0].Level != AlertLevelWarning {
					t.Errorf("Level = %q, want %q", alerts[0].Level, AlertLevelWarning)
				}
			},
		},
		{
			name: "expired reviewer lease",
			state: &models.State{
				Tasks: []models.Task{
					func() models.Task {
						task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReviewing, now)
						reviewer := "code-reviewer-1"
						task.ReviewingBy = &reviewer
						// Expired lease (past grace period)
						expiredTime := now.Add(-3 * time.Minute)
						task.ReviewLeaseExpires = &expiredTime
						return task
					}(),
				},
				Agents: map[string]models.Agent{},
			},
			wantAlerts: 1,
			validate: func(t *testing.T, alerts []Alert) {
				if alerts[0].Category != "REVIEW LEASE EXPIRED" {
					t.Errorf("Category = %q, want %q", alerts[0].Category, "REVIEW LEASE EXPIRED")
				}
			},
		},
		{
			name: "lease within grace period",
			state: &models.State{
				Tasks: []models.Task{
					func() models.Task {
						task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusImplementing, now)
						return task
					}(),
				},
				Agents: map[string]models.Agent{
					"coder-1": {
						Status:      models.AgentStatusWorking,
						CurrentTask: testhelpers.StringPtr("task-1"),
						// Expired but within grace period
						LeaseExpires: testhelpers.TimePtr(now.Add(-1 * time.Minute)),
					},
				},
			},
			wantAlerts: 0,
		},
		{
			name: "valid lease",
			state: &models.State{
				Tasks: []models.Task{
					func() models.Task {
						task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusImplementing, now)
						return task
					}(),
				},
				Agents: map[string]models.Agent{
					"coder-1": {
						Status:       models.AgentStatusWorking,
						CurrentTask:  testhelpers.StringPtr("task-1"),
						LeaseExpires: testhelpers.TimePtr(now.Add(30 * time.Minute)),
					},
				},
			},
			wantAlerts: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			alerts := checkExpiredLeases(tt.state)

			if len(alerts) != tt.wantAlerts {
				t.Errorf("len(alerts) = %d, want %d", len(alerts), tt.wantAlerts)
			}

			if tt.validate != nil && len(alerts) > 0 {
				tt.validate(t, alerts)
			}
		})
	}
}

func TestCheckBlockedTasks(t *testing.T) {
	now := time.Now().UTC()

	tests := []struct {
		name       string
		tasks      []models.Task
		cache      map[string]time.Time
		wantAlerts int
	}{
		{
			name: "blocked task - first time",
			tasks: []models.Task{
				func() models.Task {
					task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusBlocked, now)
					return task
				}(),
			},
			cache:      make(map[string]time.Time),
			wantAlerts: 1,
		},
		{
			name: "blocked task - emitted for active-key tracking even when cache has old blocked entry",
			tasks: []models.Task{
				func() models.Task {
					task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusBlocked, now)
					return task
				}(),
			},
			cache: map[string]time.Time{
				"blocked:task-1": now.Add(-5 * time.Minute),
			},
			wantAlerts: 1,
		},
		{
			name: "no blocked tasks",
			tasks: []models.Task{
				testhelpers.BuildTaskByStatus("task-1", models.TaskStatusImplementing, now),
			},
			cache:      make(map[string]time.Time),
			wantAlerts: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &models.State{Tasks: tt.tasks}
			alerts := checkBlockedTasks(state, tt.cache)

			if len(alerts) != tt.wantAlerts {
				t.Errorf("len(alerts) = %d, want %d", len(alerts), tt.wantAlerts)
			}
		})
	}
}

func TestRunChecksWithState_BlockedAlertDedupesAndRealertsAfterClear(t *testing.T) {
	now := time.Now().UTC()
	tmpDir := t.TempDir()
	testhelpers.SetupLizaDir(t, tmpDir)
	testhelpers.SetupPipelineConfig(t, tmpDir)

	reason := "spec ambiguity"
	blockedState := testhelpers.CreateValidState()
	blockedState.Tasks = []models.Task{
		func() models.Task {
			task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusBlocked, now)
			task.BlockedReason = &reason
			return task
		}(),
	}
	clearState := testhelpers.CreateValidState()
	clearState.Tasks = []models.Task{
		testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReady, now),
	}

	cache := make(map[string]time.Time)
	config := WatchConfig{ProjectRoot: tmpDir, StateCache: cache}
	first := RunChecksWithStateSnapshot(blockedState, config)
	if got := countAlertsByCategory(first.Alerts, "BLOCKED"); got != 1 {
		t.Fatalf("first BLOCKED alerts = %d, want 1; alerts: %v", got, first.Alerts)
	}
	blockedKey := AlertKey(firstAlertByCategory(t, first.Alerts, "BLOCKED"))
	if !first.ActiveKeys[blockedKey] {
		t.Fatalf("first active keys missing %q", blockedKey)
	}

	second := RunChecksWithStateSnapshot(blockedState, config)
	if got := countAlertsByCategory(second.Alerts, "BLOCKED"); got != 0 {
		t.Fatalf("second BLOCKED alerts = %d, want 0; alerts: %v", got, second.Alerts)
	}
	if !second.ActiveKeys[blockedKey] {
		t.Fatalf("second active keys missing throttled BLOCKED key %q", blockedKey)
	}

	cleared := RunChecksWithStateSnapshot(clearState, config)
	if cleared.ActiveKeys[blockedKey] {
		t.Fatalf("cleared active keys still contain %q", blockedKey)
	}

	reblocked := RunChecksWithStateSnapshot(blockedState, config)
	if got := countAlertsByCategory(reblocked.Alerts, "BLOCKED"); got != 1 {
		t.Fatalf("reblocked BLOCKED alerts = %d, want 1; alerts: %v", got, reblocked.Alerts)
	}
}

func TestCheckOrphanedRejected(t *testing.T) {
	now := time.Now().UTC()

	tests := []struct {
		name       string
		tasks      []models.Task
		agents     map[string]models.Agent
		cache      map[string]time.Time
		wantAlerts int
	}{
		{
			name: "orphaned rejected - agent missing",
			tasks: []models.Task{
				func() models.Task {
					task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusRejected, now)
					return task
				}(),
			},
			agents: map[string]models.Agent{},
			cache: map[string]time.Time{
				// Already past grace period
				"orphaned:task-1": now.Add(-1 * time.Minute),
			},
			wantAlerts: 1,
		},
		{
			name: "orphaned rejected - agent idle",
			tasks: []models.Task{
				func() models.Task {
					task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusRejected, now)
					return task
				}(),
			},
			agents: map[string]models.Agent{
				"coder-1": {
					Status: models.AgentStatusIdle,
				},
			},
			cache: map[string]time.Time{
				"orphaned:task-1": now.Add(-1 * time.Minute),
			},
			wantAlerts: 1,
		},
		{
			name: "not orphaned - agent working",
			tasks: []models.Task{
				func() models.Task {
					task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusRejected, now)
					return task
				}(),
			},
			agents: map[string]models.Agent{
				"coder-1": {
					Status: models.AgentStatusWorking,
				},
			},
			cache:      make(map[string]time.Time),
			wantAlerts: 0,
		},
		{
			name: "within grace period",
			tasks: []models.Task{
				func() models.Task {
					task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusRejected, now)
					return task
				}(),
			},
			agents: map[string]models.Agent{},
			cache: map[string]time.Time{
				// Just added to cache (within grace period)
				"orphaned:task-1": now.Add(-5 * time.Second),
			},
			wantAlerts: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &models.State{
				Tasks:  tt.tasks,
				Agents: tt.agents,
			}
			alerts := checkOrphanedRejected(state, tt.cache)

			if len(alerts) != tt.wantAlerts {
				t.Errorf("len(alerts) = %d, want %d", len(alerts), tt.wantAlerts)
			}
		})
	}

	t.Run("sentinel assigned_to not orphaned", func(t *testing.T) {
		task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusRejected, now)
		sentinel := "$transitioning"
		task.AssignedTo = &sentinel

		cache := map[string]time.Time{
			"orphaned:task-1": now.Add(-1 * time.Minute),
		}
		state := &models.State{
			Tasks:  []models.Task{task},
			Agents: map[string]models.Agent{},
		}
		alerts := checkOrphanedRejected(state, cache)

		if len(alerts) != 0 {
			t.Errorf("len(alerts) = %d, want 0", len(alerts))
		}
		if _, exists := cache["orphaned:task-1"]; exists {
			t.Error("cache entry 'orphaned:task-1' should have been cleared by sentinel exemption")
		}
	})

	t.Run("sentinel clears stale cache then real orphan gets grace period", func(t *testing.T) {
		// First call: task with sentinel AssignedTo and pre-existing cache entry.
		task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusRejected, now)
		sentinel := "$transitioning"
		task.AssignedTo = &sentinel

		cache := map[string]time.Time{
			"orphaned:task-1": now.Add(-1 * time.Minute),
		}
		state := &models.State{
			Tasks:  []models.Task{task},
			Agents: map[string]models.Agent{},
		}
		alerts := checkOrphanedRejected(state, cache)
		if len(alerts) != 0 {
			t.Fatalf("first call: len(alerts) = %d, want 0", len(alerts))
		}

		// Second call: sentinel cleared, real agent assigned but missing from state.
		task2 := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusRejected, now)
		// BuildTaskByStatus sets AssignedTo to "coder-1" for REJECTED status.
		state2 := &models.State{
			Tasks:  []models.Task{task2},
			Agents: map[string]models.Agent{}, // coder-1 not registered
		}
		alerts2 := checkOrphanedRejected(state2, cache)
		if len(alerts2) != 0 {
			t.Errorf("second call: len(alerts) = %d, want 0 (grace period should restart)", len(alerts2))
		}
		if _, exists := cache["orphaned:task-1"]; !exists {
			t.Error("cache should contain fresh 'orphaned:task-1' entry after grace period restart")
		}
	})
}

func TestCheckReviewLoops(t *testing.T) {
	now := time.Now().UTC()

	tests := []struct {
		name       string
		tasks      []models.Task
		wantAlerts int
	}{
		{
			name: "at review cycle limit",
			tasks: []models.Task{
				func() models.Task {
					task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusRejected, now)
					task.ReviewCyclesCurrent = 5
					return task
				}(),
			},
			wantAlerts: 1,
		},
		{
			name: "above review cycle limit",
			tasks: []models.Task{
				func() models.Task {
					task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusRejected, now)
					task.ReviewCyclesCurrent = 6
					return task
				}(),
			},
			wantAlerts: 1,
		},
		{
			name: "below limit",
			tasks: []models.Task{
				func() models.Task {
					task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusRejected, now)
					task.ReviewCyclesCurrent = 3
					return task
				}(),
			},
			wantAlerts: 0,
		},
		{
			name: "superseded task at limit ignored",
			tasks: []models.Task{
				func() models.Task {
					task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusSuperseded, now)
					task.ReviewCyclesCurrent = 5
					return task
				}(),
			},
			wantAlerts: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &models.State{Tasks: tt.tasks}
			alerts := checkReviewLoops(state)

			if len(alerts) != tt.wantAlerts {
				t.Errorf("len(alerts) = %d, want %d", len(alerts), tt.wantAlerts)
			}
		})
	}
}

func TestCheckIntegrationFailures(t *testing.T) {
	now := time.Now().UTC()

	tests := []struct {
		name       string
		tasks      []models.Task
		wantAlerts int
	}{
		{
			name: "integration failed task",
			tasks: []models.Task{
				testhelpers.BuildTaskByStatus("task-1", models.TaskStatusIntegrationFailed, now),
			},
			wantAlerts: 1,
		},
		{
			name: "no integration failures",
			tasks: []models.Task{
				testhelpers.BuildTaskByStatus("task-1", models.TaskStatusImplementing, now),
			},
			wantAlerts: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &models.State{Tasks: tt.tasks}
			alerts := checkIntegrationFailures(state, "")

			if len(alerts) != tt.wantAlerts {
				t.Errorf("len(alerts) = %d, want %d", len(alerts), tt.wantAlerts)
			}
		})
	}
}

func TestCheckIntegrationFailures_IncludesDiagnosticDetails(t *testing.T) {
	now := time.Now().UTC()
	task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusIntegrationFailed, now)
	task.IntegrationFailure = map[string]any{
		"operation":     "wt-merge",
		"reason":        "integration tests failed",
		"recovery_hint": "inspect test output and fix the task worktree",
	}
	state := &models.State{Tasks: []models.Task{task}}

	alerts := checkIntegrationFailures(state, "")
	if len(alerts) != 1 {
		t.Fatalf("len(alerts) = %d, want 1", len(alerts))
	}
	message := alerts[0].Message
	for _, want := range []string{
		"task-1",
		"operation=wt-merge",
		"reason=integration tests failed",
		"recovery_hint=inspect test output and fix the task worktree",
	} {
		if !strings.Contains(message, want) {
			t.Errorf("Message = %q, want containing %q", message, want)
		}
	}
}

func TestCheckIntegrationFailures_SynthesizesLegacyDiagnostic(t *testing.T) {
	now := time.Now().UTC()
	task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusIntegrationFailed, now)
	state := &models.State{Tasks: []models.Task{task}}

	alerts := checkIntegrationFailures(state, "")
	if len(alerts) != 1 {
		t.Fatalf("len(alerts) = %d, want 1", len(alerts))
	}
	message := alerts[0].Message
	for _, want := range []string{
		"operation=unknown",
		"legacy INTEGRATION_FAILED state has no structured diagnostic",
		"reconcile-merged",
	} {
		if !strings.Contains(message, want) {
			t.Errorf("Message = %q, want containing %q", message, want)
		}
	}
}

func TestCheckHypothesisExhaustion(t *testing.T) {
	now := time.Now().UTC()

	tests := []struct {
		name       string
		tasks      []models.Task
		wantAlerts int
	}{
		{
			name: "two failed coders",
			tasks: []models.Task{
				func() models.Task {
					task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReady, now)
					task.FailedBy = []string{"coder-1", "coder-2"}
					return task
				}(),
			},
			wantAlerts: 1,
		},
		{
			name: "one failed coder",
			tasks: []models.Task{
				func() models.Task {
					task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReady, now)
					task.FailedBy = []string{"coder-1"}
					return task
				}(),
			},
			wantAlerts: 0,
		},
		{
			name: "superseded task with two failures ignored",
			tasks: []models.Task{
				func() models.Task {
					task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusSuperseded, now)
					task.FailedBy = []string{"coder-1", "coder-2"}
					return task
				}(),
			},
			wantAlerts: 0,
		},
		{
			name: "blocked task with two failures handled by blocked alert",
			tasks: []models.Task{
				func() models.Task {
					task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusBlocked, now)
					task.FailedBy = []string{"coder-1", "coder-2"}
					return task
				}(),
			},
			wantAlerts: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &models.State{Tasks: tt.tasks}
			alerts := checkHypothesisExhaustion(state)

			if len(alerts) != tt.wantAlerts {
				t.Errorf("len(alerts) = %d, want %d", len(alerts), tt.wantAlerts)
			}
		})
	}
}

func TestReconcileStuckAlerts_DedupesClearsAndRealerts(t *testing.T) {
	now := time.Now().UTC()
	alert := Alert{
		Timestamp: now,
		Level:     AlertLevelCritical,
		Category:  "INTEGRATION FAILED",
		Message:   "task-1",
	}
	cache := map[string]time.Time{
		"blocked:task-1": now.Add(-1 * time.Minute),
		"stuck-alert:HYPOTHESIS EXHAUSTION:resolved-task": now.Add(-1 * time.Minute),
	}

	alerts := reconcileStuckAlerts([]Alert{alert}, cache)
	if len(alerts) != 1 {
		t.Fatalf("first call len(alerts) = %d, want 1", len(alerts))
	}
	if _, exists := cache["stuck-alert:INTEGRATION FAILED:task-1"]; !exists {
		t.Fatal("active stuck alert cache key was not stored")
	}
	if _, exists := cache["stuck-alert:HYPOTHESIS EXHAUSTION:resolved-task"]; exists {
		t.Fatal("resolved stuck alert cache key was not cleared")
	}
	if _, exists := cache["blocked:task-1"]; !exists {
		t.Fatal("unrelated cache key was removed")
	}

	alerts = reconcileStuckAlerts([]Alert{alert}, cache)
	if len(alerts) != 0 {
		t.Fatalf("second call len(alerts) = %d, want 0", len(alerts))
	}

	alerts = reconcileStuckAlerts(nil, cache)
	if len(alerts) != 0 {
		t.Fatalf("clear call len(alerts) = %d, want 0", len(alerts))
	}
	if _, exists := cache["stuck-alert:INTEGRATION FAILED:task-1"]; exists {
		t.Fatal("inactive stuck alert cache key was not cleared")
	}
	if _, exists := cache["blocked:task-1"]; !exists {
		t.Fatal("unrelated cache key was removed during clear")
	}

	alerts = reconcileStuckAlerts([]Alert{alert}, cache)
	if len(alerts) != 1 {
		t.Fatalf("recurrence call len(alerts) = %d, want 1", len(alerts))
	}

	invalidStateAlert := Alert{
		Timestamp: now,
		Level:     AlertLevelCritical,
		Category:  "INVALID STATE",
		Message:   "state.yaml invalid",
	}
	alerts = reconcileStuckAlerts([]Alert{invalidStateAlert}, cache)
	if len(alerts) != 1 {
		t.Fatalf("invalid state first call len(alerts) = %d, want 1", len(alerts))
	}
	alerts = reconcileStuckAlerts([]Alert{invalidStateAlert}, cache)
	if len(alerts) != 0 {
		t.Fatalf("invalid state second call len(alerts) = %d, want 0", len(alerts))
	}
}

func TestCheckReassigned(t *testing.T) {
	now := time.Now().UTC()

	tests := []struct {
		name       string
		tasks      []models.Task
		cache      map[string]time.Time
		wantAlerts int
		wantCat    string
		wantMsg    string
	}{
		{
			name: "attempt 2 IMPLEMENTING task alerts",
			tasks: []models.Task{
				func() models.Task {
					task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusImplementing, now)
					task.Attempt = 2
					return task
				}(),
			},
			cache:      make(map[string]time.Time),
			wantAlerts: 1,
			wantCat:    "ATTEMPT",
			wantMsg:    "task-1 — attempt 2 (final attempt)",
		},
		{
			name: "attempt 2 REJECTED task alerts",
			tasks: []models.Task{
				func() models.Task {
					task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusRejected, now)
					task.Attempt = 2
					return task
				}(),
			},
			cache:      make(map[string]time.Time),
			wantAlerts: 1,
			wantCat:    "ATTEMPT",
			wantMsg:    "task-1 — attempt 2 (final attempt)",
		},
		{
			name: "attempt 1 task no alert",
			tasks: []models.Task{
				func() models.Task {
					task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusImplementing, now)
					task.Attempt = 1
					return task
				}(),
			},
			cache:      make(map[string]time.Time),
			wantAlerts: 0,
		},
		{
			name: "attempt 0 (legacy) no alert",
			tasks: []models.Task{
				func() models.Task {
					task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusImplementing, now)
					// Attempt defaults to 0 (unset/legacy); EffectiveAttempt() returns 1
					return task
				}(),
			},
			cache:      make(map[string]time.Time),
			wantAlerts: 0,
		},
		{
			name: "attempt 2 cached suppresses duplicate",
			tasks: []models.Task{
				func() models.Task {
					task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusImplementing, now)
					task.Attempt = 2
					return task
				}(),
			},
			cache:      map[string]time.Time{"attempt2:task-1": now.Add(-1 * time.Minute)},
			wantAlerts: 0,
		},
		{
			name: "merged task at attempt 2 ignored",
			tasks: []models.Task{
				func() models.Task {
					task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusMerged, now)
					task.Attempt = 2
					return task
				}(),
			},
			cache:      make(map[string]time.Time),
			wantAlerts: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &models.State{Tasks: tt.tasks}
			alerts := checkReassigned(state, tt.cache)

			if len(alerts) != tt.wantAlerts {
				t.Errorf("len(alerts) = %d, want %d", len(alerts), tt.wantAlerts)
			}
			if tt.wantAlerts > 0 && len(alerts) > 0 {
				if alerts[0].Category != tt.wantCat {
					t.Errorf("category = %q, want %q", alerts[0].Category, tt.wantCat)
				}
				if alerts[0].Message != tt.wantMsg {
					t.Errorf("message = %q, want %q", alerts[0].Message, tt.wantMsg)
				}
			}
		})
	}
}

func TestCheckApproachingLimits(t *testing.T) {
	now := time.Now().UTC()

	tests := []struct {
		name        string
		tasks       []models.Task
		wantAlerts  int
		wantContain string // substring expected in first alert message
	}{
		{
			name: "approaching iteration limit",
			tasks: []models.Task{
				func() models.Task {
					task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusImplementing, now)
					task.Iteration = 8
					task.Attempt = 1
					return task
				}(),
			},
			wantAlerts:  1,
			wantContain: "attempt 1, iteration 8/10",
		},
		{
			name: "approaching iteration limit attempt 2",
			tasks: []models.Task{
				func() models.Task {
					task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusImplementing, now)
					task.Iteration = 8
					task.Attempt = 2
					return task
				}(),
			},
			wantAlerts:  1,
			wantContain: "attempt 2 (final), iteration 8/10",
		},
		{
			name: "at iteration cliff",
			tasks: []models.Task{
				func() models.Task {
					task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusImplementing, now)
					task.Iteration = 10
					task.Attempt = 1
					return task
				}(),
			},
			wantAlerts: 0, // Don't warn at cliff, only approaching
		},
		{
			name: "approaching review cycle limit",
			tasks: []models.Task{
				func() models.Task {
					task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusRejected, now)
					task.ReviewCyclesCurrent = 3
					task.Attempt = 1
					return task
				}(),
			},
			wantAlerts:  1,
			wantContain: "attempt 1, review cycle 3/5",
		},
		{
			name: "approaching review cycle limit attempt 2",
			tasks: []models.Task{
				func() models.Task {
					task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusRejected, now)
					task.ReviewCyclesCurrent = 3
					task.Attempt = 2
					return task
				}(),
			},
			wantAlerts:  1,
			wantContain: "attempt 2 (final), review cycle 3/5",
		},
		{
			name: "one failure + high review cycles",
			tasks: []models.Task{
				func() models.Task {
					task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReady, now)
					task.FailedBy = []string{"coder-1"}
					task.ReviewCyclesCurrent = 3
					task.Attempt = 1
					return task
				}(),
			},
			wantAlerts: 1, // Only review cycle alert; coder failures warning removed
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &models.State{Tasks: tt.tasks}
			alerts := checkApproachingLimits(state)

			if len(alerts) != tt.wantAlerts {
				t.Errorf("len(alerts) = %d, want %d", len(alerts), tt.wantAlerts)
			}
			if tt.wantContain != "" && len(alerts) > 0 {
				if !strings.Contains(alerts[0].Message, tt.wantContain) {
					t.Errorf("alert message = %q, want containing %q", alerts[0].Message, tt.wantContain)
				}
			}
		})
	}
}

func TestCheckStalled(t *testing.T) {
	now := time.Now().UTC()

	tests := []struct {
		name       string
		state      *models.State
		wantAlerts int
		wantMsg    string
	}{
		{
			name: "active_task_stale_history",
			state: &models.State{
				Tasks: []models.Task{{
					ID:     "t1",
					Status: models.TaskStatusImplementing,
					History: []models.TaskHistoryEntry{{
						Time:  now.Add(-31 * time.Minute),
						Event: "claimed",
					}},
				}},
			},
			wantAlerts: 1,
			wantMsg:    "no task progress",
		},
		{
			name: "active_task_recent_history",
			state: &models.State{
				Tasks: []models.Task{{
					ID:     "t1",
					Status: models.TaskStatusImplementing,
					History: []models.TaskHistoryEntry{{
						Time:  now.Add(-5 * time.Minute),
						Event: "claimed",
					}},
				}},
			},
			wantAlerts: 0,
		},
		{
			name: "all_tasks_terminal",
			state: &models.State{
				Tasks: []models.Task{{
					ID:     "t1",
					Status: models.TaskStatusMerged,
					History: []models.TaskHistoryEntry{{
						Time:  now.Add(-60 * time.Minute),
						Event: "merged",
					}},
				}},
			},
			wantAlerts: 0,
		},
		{
			name:       "no_tasks",
			state:      &models.State{},
			wantAlerts: 0,
		},
		{
			name: "no_history_fallback_to_created",
			state: &models.State{
				Tasks: []models.Task{{
					ID:      "t1",
					Status:  models.TaskStatusImplementing,
					Created: now.Add(-31 * time.Minute),
				}},
			},
			wantAlerts: 1,
			wantMsg:    "no task progress",
		},
		{
			name: "no_history_recent_created",
			state: &models.State{
				Tasks: []models.Task{{
					ID:      "t1",
					Status:  models.TaskStatusImplementing,
					Created: now.Add(-5 * time.Minute),
				}},
			},
			wantAlerts: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache := make(map[string]time.Time)
			alerts := checkStalled(tt.state, cache)

			if len(alerts) != tt.wantAlerts {
				t.Errorf("len(alerts) = %d, want %d", len(alerts), tt.wantAlerts)
			}
			if tt.wantMsg != "" && len(alerts) > 0 {
				if !strings.Contains(alerts[0].Message, tt.wantMsg) {
					t.Errorf("alert message = %q, want substring %q", alerts[0].Message, tt.wantMsg)
				}
			}
		})
	}
}

func TestCheckStalledThrottling(t *testing.T) {
	now := time.Now().UTC()

	state := &models.State{
		Tasks: []models.Task{{
			ID:     "t1",
			Status: models.TaskStatusImplementing,
			History: []models.TaskHistoryEntry{{
				Time:  now.Add(-31 * time.Minute),
				Event: "claimed",
			}},
		}},
	}

	cache := make(map[string]time.Time)

	// First call - should generate alert
	alerts := checkStalled(state, cache)
	if len(alerts) != 1 {
		t.Errorf("First call: len(alerts) = %d, want 1", len(alerts))
	}

	// Second call immediately after - should be throttled
	alerts = checkStalled(state, cache)
	if len(alerts) != 0 {
		t.Errorf("Second call (throttled): len(alerts) = %d, want 0", len(alerts))
	}

	// Simulate 5 minutes passing by updating cache to 5+ minutes ago
	cache["stalled:alert"] = now.Add(-6 * time.Minute)

	// Third call after 5 minutes - should generate alert again
	alerts = checkStalled(state, cache)
	if len(alerts) != 1 {
		t.Errorf("Third call (after 5 min): len(alerts) = %d, want 1", len(alerts))
	}
}

func TestCheckStaleDrafts(t *testing.T) {
	now := time.Now().UTC()

	tests := []struct {
		name       string
		tasks      []models.Task
		wantAlerts int
	}{
		{
			name: "stale draft - 31 minutes old",
			tasks: []models.Task{
				func() models.Task {
					task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusDraft, now)
					task.Created = now.Add(-31 * time.Minute)
					return task
				}(),
			},
			wantAlerts: 1,
		},
		{
			name: "fresh draft",
			tasks: []models.Task{
				func() models.Task {
					task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusDraft, now)
					task.Created = now.Add(-5 * time.Minute)
					return task
				}(),
			},
			wantAlerts: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &models.State{Tasks: tt.tasks}
			alerts := checkStaleDrafts(state)

			if len(alerts) != tt.wantAlerts {
				t.Errorf("len(alerts) = %d, want %d", len(alerts), tt.wantAlerts)
			}
		})
	}
}

func TestCheckImmediateDiscoveries(t *testing.T) {
	now := time.Now().UTC()

	tests := []struct {
		name       string
		discovered []models.Discovery
		wantAlerts int
	}{
		{
			name: "immediate discovery not converted",
			discovered: []models.Discovery{
				{
					ID:          "disc-1",
					Description: "Critical bug found",
					Urgency:     "immediate",
					Created:     now,
				},
			},
			wantAlerts: 1,
		},
		{
			name: "immediate discovery already converted",
			discovered: []models.Discovery{
				{
					ID:              "disc-1",
					Description:     "Critical bug found",
					Urgency:         "immediate",
					Created:         now,
					ConvertedToTask: testhelpers.StringPtr("task-1"),
				},
			},
			wantAlerts: 0,
		},
		{
			name: "deferred urgency discovery",
			discovered: []models.Discovery{
				{
					ID:          "disc-1",
					Description: "Minor improvement",
					Urgency:     "deferred",
					Created:     now,
				},
			},
			wantAlerts: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &models.State{Discovered: tt.discovered}
			alerts := checkImmediateDiscoveries(state)

			if len(alerts) != tt.wantAlerts {
				t.Errorf("len(alerts) = %d, want %d", len(alerts), tt.wantAlerts)
			}
		})
	}
}

func TestWatchCommand(t *testing.T) {
	now := time.Now().UTC()

	tests := []struct {
		name      string
		setupFunc func(*testing.T, string) *models.State
		runTime   time.Duration
		wantErr   bool
	}{
		{
			name: "basic watch - single check cycle",
			setupFunc: func(t *testing.T, tmpDir string) *models.State {
				state := testhelpers.CreateValidState()
				// Add a task that will trigger an alert
				state.Tasks = []models.Task{
					testhelpers.BuildTaskByStatus("task-1", models.TaskStatusIntegrationFailed, now),
				}
				return state
			},
			runTime: 100 * time.Millisecond,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)
			testhelpers.SetupPipelineConfig(t, tmpDir)
			alertsLog := paths.New(tmpDir).AlertsLogPath()

			state := tt.setupFunc(t, tmpDir)
			testhelpers.WriteInitialState(t, stateFile, state)

			// Create context with timeout
			ctx, cancel := context.WithTimeout(context.Background(), tt.runTime)
			defer cancel()

			config := WatchConfig{
				ProjectRoot:   tmpDir,
				CheckInterval: 50 * time.Millisecond,
				AlertsLog:     alertsLog,
				StateCache:    make(map[string]time.Time),
			}

			err := WatchCommand(ctx, config)

			// Should get context.DeadlineExceeded or context.Canceled
			if tt.wantErr && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.wantErr && err != nil && err != context.DeadlineExceeded && err != context.Canceled {
				t.Errorf("Unexpected error: %v", err)
			}

			// Check that alerts were written
			if _, err := os.Stat(alertsLog); err == nil {
				data, _ := os.ReadFile(alertsLog)
				if len(data) == 0 {
					t.Error("Expected alerts to be written but file is empty")
				}
			}
		})
	}
}

func TestCheckSprintStalled(t *testing.T) {
	now := time.Now().UTC()

	t.Run("sprint stalled - all blocked - emits alert", func(t *testing.T) {
		state := testhelpers.CreateValidState()
		state.Sprint.Scope.Planned = []string{"task-1", "task-2"}
		state.Tasks = []models.Task{
			testhelpers.BuildTaskByStatus("task-1", models.TaskStatusBlocked, now),
			testhelpers.BuildTaskByStatus("task-2", models.TaskStatusBlocked, now),
		}

		cache := make(map[string]time.Time)
		alerts := checkSprintStalled(state, cache)
		if len(alerts) != 1 {
			t.Fatalf("len(alerts) = %d, want 1", len(alerts))
		}
		if alerts[0].Category != "SPRINT STALLED" {
			t.Errorf("alert[0].Category = %q, want %q", alerts[0].Category, "SPRINT STALLED")
		}
		if alerts[0].Level != AlertLevelCritical {
			t.Errorf("alert[0].Level = %q, want %q", alerts[0].Level, AlertLevelCritical)
		}
		if !strings.Contains(alerts[0].Message, "2 non-terminal planned tasks are BLOCKED") {
			t.Errorf("alert[0].Message = %q, expected blocked count", alerts[0].Message)
		}
	})

	t.Run("mix of terminal and blocked - emits alert", func(t *testing.T) {
		state := testhelpers.CreateValidState()
		state.Sprint.Scope.Planned = []string{"task-1", "task-2"}
		state.Tasks = []models.Task{
			testhelpers.BuildTaskByStatus("task-1", models.TaskStatusMerged, now),
			testhelpers.BuildTaskByStatus("task-2", models.TaskStatusBlocked, now),
		}

		cache := make(map[string]time.Time)
		alerts := checkSprintStalled(state, cache)
		if len(alerts) != 1 {
			t.Fatalf("len(alerts) = %d, want 1", len(alerts))
		}
		if !strings.Contains(alerts[0].Message, "1 non-terminal planned tasks are BLOCKED") {
			t.Errorf("alert[0].Message = %q, expected 1 blocked", alerts[0].Message)
		}
	})

	t.Run("tasks in progress - no alert", func(t *testing.T) {
		state := testhelpers.CreateValidState()
		state.Sprint.Scope.Planned = []string{"task-1", "task-2"}
		state.Tasks = []models.Task{
			testhelpers.BuildTaskByStatus("task-1", models.TaskStatusBlocked, now),
			testhelpers.BuildTaskByStatus("task-2", models.TaskStatusImplementing, now),
		}

		cache := make(map[string]time.Time)
		alerts := checkSprintStalled(state, cache)
		if len(alerts) != 0 {
			t.Errorf("len(alerts) = %d, want 0", len(alerts))
		}
	})

	t.Run("sprint already at CHECKPOINT - no action", func(t *testing.T) {
		state := testhelpers.CreateValidState()
		state.Sprint.Status = models.SprintStatusCheckpoint
		state.Sprint.Scope.Planned = []string{"task-1"}
		state.Tasks = []models.Task{
			testhelpers.BuildTaskByStatus("task-1", models.TaskStatusBlocked, now),
		}

		cache := make(map[string]time.Time)
		alerts := checkSprintStalled(state, cache)
		if len(alerts) != 0 {
			t.Errorf("len(alerts) = %d, want 0 (guarded by sprint status)", len(alerts))
		}
	})

	t.Run("stall then resume still stalled re-triggers alert", func(t *testing.T) {
		// Regression: cache must be cleared when sprint leaves IN_PROGRESS,
		// so a resumed-but-still-stalled sprint gets a fresh alert.
		stalledState := testhelpers.CreateValidState()
		stalledState.Sprint.Scope.Planned = []string{"task-1"}
		stalledState.Tasks = []models.Task{
			testhelpers.BuildTaskByStatus("task-1", models.TaskStatusBlocked, now),
		}

		cache := make(map[string]time.Time)

		// Step 1: Stall detected → alert fires
		alerts := checkSprintStalled(stalledState, cache)
		if len(alerts) != 1 {
			t.Fatalf("step 1: len(alerts) = %d, want 1", len(alerts))
		}

		// Step 2: Sprint moves to CHECKPOINT — simulate watch tick
		checkpointState := testhelpers.CreateValidState()
		checkpointState.Sprint.Status = models.SprintStatusCheckpoint
		checkpointState.Sprint.Scope.Planned = []string{"task-1"}
		checkpointState.Tasks = []models.Task{
			testhelpers.BuildTaskByStatus("task-1", models.TaskStatusBlocked, now),
		}
		alerts = checkSprintStalled(checkpointState, cache)
		if len(alerts) != 0 {
			t.Errorf("step 2: len(alerts) = %d, want 0", len(alerts))
		}
		// Cache should have been cleared by the non-IN_PROGRESS guard
		if _, cached := cache["sprint_stalled:alert"]; cached {
			t.Error("step 2: cache should be cleared when sprint not IN_PROGRESS")
		}

		// Step 3: Human resumes without fixing → sprint back to IN_PROGRESS, still stalled
		resumedState := testhelpers.CreateValidState()
		resumedState.Sprint.Scope.Planned = []string{"task-1"}
		resumedState.Tasks = []models.Task{
			testhelpers.BuildTaskByStatus("task-1", models.TaskStatusBlocked, now),
		}

		// Step 4: Stall re-detected → fresh alert
		alerts = checkSprintStalled(resumedState, cache)
		if len(alerts) != 1 {
			t.Fatalf("step 4: len(alerts) = %d, want 1 (should re-trigger after resume)", len(alerts))
		}
		if alerts[0].Category != "SPRINT STALLED" {
			t.Errorf("step 4: alert[0].Category = %q, want SPRINT STALLED", alerts[0].Category)
		}
	})

	t.Run("throttling - second call does not re-trigger", func(t *testing.T) {
		state := testhelpers.CreateValidState()
		state.Sprint.Scope.Planned = []string{"task-1"}
		state.Tasks = []models.Task{
			testhelpers.BuildTaskByStatus("task-1", models.TaskStatusBlocked, now),
		}

		cache := make(map[string]time.Time)

		// First call triggers
		alerts := checkSprintStalled(state, cache)
		if len(alerts) != 1 {
			t.Fatalf("first call: len(alerts) = %d, want 1", len(alerts))
		}

		// Second call should be throttled (cache key set)
		alerts = checkSprintStalled(state, cache)
		if len(alerts) != 0 {
			t.Errorf("second call: len(alerts) = %d, want 0 (throttled by cache)", len(alerts))
		}
	})
}

func TestRunChecks_CircuitBreakerAlertOnPattern(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)
	testhelpers.SetupPipelineConfig(t, tmpDir)
	lizaPaths := paths.New(tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Anomalies = []models.Anomaly{
		{
			Timestamp: now,
			Task:      "task-1",
			Reporter:  "coder-1",
			Type:      "retry_loop",
			Details: map[string]any{
				"count":         3,
				"error_pattern": "connection refused",
			},
		},
		{
			Timestamp: now,
			Task:      "task-2",
			Reporter:  "coder-2",
			Type:      "retry_loop",
			Details: map[string]any{
				"count":         3,
				"error_pattern": "connection refused",
			},
		},
		{
			Timestamp: now,
			Task:      "task-3",
			Reporter:  "code-reviewer-1",
			Type:      "retry_loop",
			Details: map[string]any{
				"count":         3,
				"error_pattern": "connection refused",
			},
		},
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	config := WatchConfig{
		ProjectRoot: tmpDir,
		AlertsLog:   lizaPaths.AlertsLogPath(),
		StateCache:  make(map[string]time.Time),
	}

	if err := runChecks(context.Background(), config); err != nil {
		t.Fatalf("runChecks() error: %v", err)
	}

	// Verify NO state mutation — watch is read-only.
	bb := db.New(stateFile)
	updatedState, err := bb.Read()
	if err != nil {
		t.Fatalf("failed to read state: %v", err)
	}
	if updatedState.Config.Mode != models.SystemModeRunning {
		t.Errorf("mode = %s, want %s (watch must not mutate)", updatedState.Config.Mode, models.SystemModeRunning)
	}
	if updatedState.Sprint.Status != models.SprintStatusInProgress {
		t.Errorf("sprint.status = %s, want %s (watch must not mutate)", updatedState.Sprint.Status, models.SprintStatusInProgress)
	}

	// Verify CIRCUIT BREAKER alert was emitted.
	alertLogData, err := os.ReadFile(lizaPaths.AlertsLogPath())
	if err != nil {
		t.Fatalf("failed to read alerts log: %v", err)
	}
	alertLogText := string(alertLogData)
	if !strings.Contains(alertLogText, "CIRCUIT BREAKER") {
		t.Errorf("expected CIRCUIT BREAKER alert in log, got:\n%s", alertLogText)
	}
	if strings.Contains(alertLogText, "AUTO CHECKPOINT") {
		t.Errorf("unexpected AUTO CHECKPOINT alert in log (watch must not checkpoint)")
	}
}

func TestRunChecks_NoCircuitBreakerAlertBelowThreshold(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)
	testhelpers.SetupPipelineConfig(t, tmpDir)
	lizaPaths := paths.New(tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Anomalies = []models.Anomaly{
		{
			Timestamp: now,
			Task:      "task-1",
			Reporter:  "coder-1",
			Type:      "retry_loop",
			Details: map[string]any{
				"count":         2,
				"error_pattern": "timeout",
			},
		},
		{
			Timestamp: now,
			Task:      "task-2",
			Reporter:  "coder-2",
			Type:      "retry_loop",
			Details: map[string]any{
				"count":         2,
				"error_pattern": "timeout",
			},
		},
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	config := WatchConfig{
		ProjectRoot: tmpDir,
		AlertsLog:   lizaPaths.AlertsLogPath(),
		StateCache:  make(map[string]time.Time),
	}

	if err := runChecks(context.Background(), config); err != nil {
		t.Fatalf("runChecks() error: %v", err)
	}

	// Below threshold → no CIRCUIT BREAKER alert.
	alertsLogPath := lizaPaths.AlertsLogPath()
	if _, err := os.Stat(alertsLogPath); os.IsNotExist(err) {
		// No alerts log at all — pass (no alerts emitted).
		return
	}
	alertLogData, err := os.ReadFile(alertsLogPath)
	if err != nil {
		t.Fatalf("failed to read alerts log: %v", err)
	}
	if strings.Contains(string(alertLogData), "CIRCUIT BREAKER") {
		t.Errorf("unexpected CIRCUIT BREAKER alert below threshold:\n%s", string(alertLogData))
	}
}

func TestRunChecks_CircuitBreakerAlertCoexistsWithOtherAlerts(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)
	testhelpers.SetupPipelineConfig(t, tmpDir)
	lizaPaths := paths.New(tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{
		testhelpers.BuildTaskByStatus("task-99", models.TaskStatusIntegrationFailed, now),
	}
	state.Anomalies = []models.Anomaly{
		{
			Timestamp: now,
			Task:      "task-1",
			Reporter:  "coder-1",
			Type:      "retry_loop",
			Details: map[string]any{
				"count":         3,
				"error_pattern": "connection refused",
			},
		},
		{
			Timestamp: now,
			Task:      "task-2",
			Reporter:  "coder-2",
			Type:      "retry_loop",
			Details: map[string]any{
				"count":         3,
				"error_pattern": "connection refused",
			},
		},
		{
			Timestamp: now,
			Task:      "task-3",
			Reporter:  "code-reviewer-1",
			Type:      "retry_loop",
			Details: map[string]any{
				"count":         3,
				"error_pattern": "connection refused",
			},
		},
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	config := WatchConfig{
		ProjectRoot: tmpDir,
		AlertsLog:   lizaPaths.AlertsLogPath(),
		StateCache:  make(map[string]time.Time),
	}

	if err := runChecks(context.Background(), config); err != nil {
		t.Fatalf("runChecks() error: %v", err)
	}

	// Both circuit breaker and operational alerts should be emitted.
	alertLogData, err := os.ReadFile(lizaPaths.AlertsLogPath())
	if err != nil {
		t.Fatalf("failed to read alerts log: %v", err)
	}
	alertLogText := string(alertLogData)
	if !strings.Contains(alertLogText, "INTEGRATION FAILED") {
		t.Errorf("expected INTEGRATION FAILED alert in log, got:\n%s", alertLogText)
	}
	if !strings.Contains(alertLogText, "CIRCUIT BREAKER") {
		t.Errorf("expected CIRCUIT BREAKER alert in log, got:\n%s", alertLogText)
	}

	// Verify NO state mutation.
	bb := db.New(stateFile)
	updatedState, err := bb.Read()
	if err != nil {
		t.Fatalf("failed to read state: %v", err)
	}
	if updatedState.Config.Mode != models.SystemModeRunning {
		t.Errorf("mode = %s, want %s (watch must not mutate)", updatedState.Config.Mode, models.SystemModeRunning)
	}
	if updatedState.Sprint.Status != models.SprintStatusInProgress {
		t.Errorf("sprint.status = %s, want %s (watch must not mutate)", updatedState.Sprint.Status, models.SprintStatusInProgress)
	}
}

func TestCheckMissingRoles(t *testing.T) {
	now := time.Now().UTC()

	// Helper to load a real pipeline resolver from test fixtures.
	loadTestResolver := func(t *testing.T) models.PipelineResolver {
		t.Helper()
		tmpDir := t.TempDir()
		testhelpers.SetupPipelineConfig(t, tmpDir)
		pr, err := ops.LoadResolverForModels(tmpDir)
		if err != nil {
			t.Fatalf("failed to load resolver: %v", err)
		}
		return pr
	}

	t.Run("no missing role — agent registered for claimable task", func(t *testing.T) {
		pr := loadTestResolver(t)
		leaseExpires := now.Add(30 * time.Minute)
		state := &models.State{
			Tasks: []models.Task{
				testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReady, now),
			},
			Agents: map[string]models.Agent{
				"coder-1": {Role: "coder", Status: models.AgentStatusIdle, LeaseExpires: &leaseExpires},
			},
		}
		cache := make(map[string]time.Time)
		alerts := checkMissingRoles(state, pr, cache)
		if len(alerts) != 0 {
			t.Errorf("len(alerts) = %d, want 0; alerts: %v", len(alerts), alerts)
		}
	})

	t.Run("missing doer role — task claimable but no coder registered", func(t *testing.T) {
		pr := loadTestResolver(t)
		state := &models.State{
			Tasks: []models.Task{
				testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReady, now),
			},
			Agents: map[string]models.Agent{
				// Only a reviewer, no coder.
				"code-reviewer-1": {Role: "code-reviewer", Status: models.AgentStatusIdle},
			},
		}
		cache := make(map[string]time.Time)
		alerts := checkMissingRoles(state, pr, cache)
		if len(alerts) != 1 {
			t.Fatalf("len(alerts) = %d, want 1; alerts: %v", len(alerts), alerts)
		}
		if alerts[0].Category != "MISSING ROLE" {
			t.Errorf("Category = %q, want %q", alerts[0].Category, "MISSING ROLE")
		}
		if alerts[0].Level != AlertLevelWarning {
			t.Errorf("Level = %q, want %q", alerts[0].Level, AlertLevelWarning)
		}
		if !strings.Contains(alerts[0].Message, "coder") {
			t.Errorf("Message = %q, expected to contain 'coder'", alerts[0].Message)
		}
		if !strings.Contains(alerts[0].Message, "task-1") {
			t.Errorf("Message = %q, expected to contain 'task-1'", alerts[0].Message)
		}
		if !strings.Contains(alerts[0].Message, "liza repair-agent-pool --dry-run") {
			t.Errorf("Message = %q, expected repair-agent-pool hint", alerts[0].Message)
		}
	})

	t.Run("expired agent lease does not suppress missing role alert", func(t *testing.T) {
		pr := loadTestResolver(t)
		leaseExpires := now.Add(-time.Minute)
		state := &models.State{
			Tasks: []models.Task{
				testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReady, now),
			},
			Agents: map[string]models.Agent{
				"coder-1": {Role: "coder", Status: models.AgentStatusIdle, LeaseExpires: &leaseExpires},
			},
		}
		cache := make(map[string]time.Time)
		alerts := checkMissingRoles(state, pr, cache)
		if len(alerts) != 1 {
			t.Fatalf("len(alerts) = %d, want 1; alerts: %v", len(alerts), alerts)
		}
		if !strings.Contains(alerts[0].Message, "coder") {
			t.Errorf("Message = %q, expected to contain 'coder'", alerts[0].Message)
		}
	})

	t.Run("missing reviewer role — task submitted but no reviewer registered", func(t *testing.T) {
		pr := loadTestResolver(t)
		state := &models.State{
			Tasks: []models.Task{
				testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReadyForReview, now),
			},
			Agents: map[string]models.Agent{
				"coder-1": {Role: "coder", Status: models.AgentStatusWorking},
			},
		}
		cache := make(map[string]time.Time)
		alerts := checkMissingRoles(state, pr, cache)
		if len(alerts) != 1 {
			t.Fatalf("len(alerts) = %d, want 1; alerts: %v", len(alerts), alerts)
		}
		if !strings.Contains(alerts[0].Message, "code-reviewer") {
			t.Errorf("Message = %q, expected to contain 'code-reviewer'", alerts[0].Message)
		}
	})

	t.Run("terminal tasks ignored", func(t *testing.T) {
		pr := loadTestResolver(t)
		state := &models.State{
			Tasks: []models.Task{
				testhelpers.BuildTaskByStatus("task-1", models.TaskStatusMerged, now),
				testhelpers.BuildTaskByStatus("task-2", models.TaskStatusSuperseded, now),
				testhelpers.BuildTaskByStatus("task-3", models.TaskStatusAbandoned, now),
			},
			Agents: map[string]models.Agent{},
		}
		cache := make(map[string]time.Time)
		alerts := checkMissingRoles(state, pr, cache)
		if len(alerts) != 0 {
			t.Errorf("len(alerts) = %d, want 0 (terminal tasks); alerts: %v", len(alerts), alerts)
		}
	})

	t.Run("dependency-blocked task with missing role — no alert", func(t *testing.T) {
		pr := loadTestResolver(t)
		// task-2 is in initial status but depends on task-1 which is not merged.
		task2 := testhelpers.BuildTaskByStatus("task-2", models.TaskStatusReady, now)
		task2.DependsOn = []string{"task-1"}

		state := &models.State{
			Tasks: []models.Task{
				testhelpers.BuildTaskByStatus("task-1", models.TaskStatusImplementing, now),
				task2,
			},
			Agents: map[string]models.Agent{},
		}
		cache := make(map[string]time.Time)
		alerts := checkMissingRoles(state, pr, cache)
		if len(alerts) != 0 {
			t.Errorf("len(alerts) = %d, want 0 (dep-blocked task not claimable); alerts: %v", len(alerts), alerts)
		}
	})

	t.Run("cache throttling — no duplicate alert; clears when role appears", func(t *testing.T) {
		pr := loadTestResolver(t)
		state := &models.State{
			Tasks: []models.Task{
				testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReady, now),
			},
			Agents: map[string]models.Agent{},
		}
		cache := make(map[string]time.Time)

		// First call — should alert.
		alerts := checkMissingRoles(state, pr, cache)
		if len(alerts) != 1 {
			t.Fatalf("first call: len(alerts) = %d, want 1", len(alerts))
		}
		if _, ok := cache["missing-role:coder"]; !ok {
			t.Error("expected cache entry for missing-role:coder")
		}

		// Second call — should be throttled.
		alerts = checkMissingRoles(state, pr, cache)
		if len(alerts) != 0 {
			t.Errorf("second call: len(alerts) = %d, want 0 (throttled)", len(alerts))
		}

		// Agent of that role appears — cache should clear.
		leaseExpires := now.Add(30 * time.Minute)
		state.Agents["coder-1"] = models.Agent{Role: "coder", Status: models.AgentStatusIdle, LeaseExpires: &leaseExpires}
		alerts = checkMissingRoles(state, pr, cache)
		if len(alerts) != 0 {
			t.Errorf("after agent appears: len(alerts) = %d, want 0", len(alerts))
		}
		if _, ok := cache["missing-role:coder"]; ok {
			t.Error("cache entry for missing-role:coder should be cleared after agent appears")
		}

		// Agent removed again — should re-fire.
		delete(state.Agents, "coder-1")
		alerts = checkMissingRoles(state, pr, cache)
		if len(alerts) != 1 {
			t.Errorf("after agent removed: len(alerts) = %d, want 1", len(alerts))
		}
	})

	t.Run("unknown role pair — skip gracefully", func(t *testing.T) {
		pr := loadTestResolver(t)
		task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReady, now)
		task.RolePair = "nonexistent-pair"

		state := &models.State{
			Tasks:  []models.Task{task},
			Agents: map[string]models.Agent{},
		}
		cache := make(map[string]time.Time)
		alerts := checkMissingRoles(state, pr, cache)
		if len(alerts) != 0 {
			t.Errorf("len(alerts) = %d, want 0 (unknown role pair); alerts: %v", len(alerts), alerts)
		}
	})

	t.Run("nil resolver — returns no alerts", func(t *testing.T) {
		state := &models.State{
			Tasks: []models.Task{
				testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReady, now),
			},
			Agents: map[string]models.Agent{},
		}
		cache := make(map[string]time.Time)
		alerts := checkMissingRoles(state, nil, cache)
		if len(alerts) != 0 {
			t.Errorf("len(alerts) = %d, want 0 (nil resolver)", len(alerts))
		}
	})

	t.Run("task without role pair — skipped", func(t *testing.T) {
		pr := loadTestResolver(t)
		task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReady, now)
		task.RolePair = ""
		state := &models.State{Tasks: []models.Task{task}, Agents: map[string]models.Agent{}}
		cache := make(map[string]time.Time)
		alerts := checkMissingRoles(state, pr, cache)
		if len(alerts) != 0 {
			t.Errorf("len(alerts) = %d, want 0 (no role pair)", len(alerts))
		}
	})

	t.Run("cache clears when task stops being claimable while role still absent", func(t *testing.T) {
		pr := loadTestResolver(t)
		state := &models.State{
			Tasks: []models.Task{
				testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReady, now),
			},
			Agents: map[string]models.Agent{},
		}
		cache := make(map[string]time.Time)

		// First call — alerts for missing coder.
		alerts := checkMissingRoles(state, pr, cache)
		if len(alerts) != 1 {
			t.Fatalf("first call: len(alerts) = %d, want 1", len(alerts))
		}
		if _, ok := cache["missing-role:coder"]; !ok {
			t.Fatal("expected cache entry for missing-role:coder")
		}

		// Task gets merged — no longer claimable (role still absent).
		state.Tasks[0].Status = models.TaskStatusMerged
		alerts = checkMissingRoles(state, pr, cache)
		if len(alerts) != 0 {
			t.Errorf("after merge: len(alerts) = %d, want 0", len(alerts))
		}
		if _, ok := cache["missing-role:coder"]; ok {
			t.Error("cache entry for missing-role:coder should be cleared when task stops being claimable")
		}

		// New task becomes claimable for the same absent role — should re-fire.
		state.Tasks = []models.Task{
			testhelpers.BuildTaskByStatus("task-2", models.TaskStatusReady, now),
		}
		alerts = checkMissingRoles(state, pr, cache)
		if len(alerts) != 1 {
			t.Fatalf("new claimable task: len(alerts) = %d, want 1", len(alerts))
		}
		if !strings.Contains(alerts[0].Message, "task-2") {
			t.Errorf("Message = %q, expected to contain 'task-2'", alerts[0].Message)
		}
	})

	t.Run("alert message caps task IDs at 5", func(t *testing.T) {
		pr := loadTestResolver(t)
		var tasks []models.Task
		for i := 0; i < 7; i++ {
			id := fmt.Sprintf("task-%d", i+1)
			tasks = append(tasks, testhelpers.BuildTaskByStatus(id, models.TaskStatusReady, now))
		}

		state := &models.State{
			Tasks:  tasks,
			Agents: map[string]models.Agent{},
		}
		cache := make(map[string]time.Time)
		alerts := checkMissingRoles(state, pr, cache)
		if len(alerts) != 1 {
			t.Fatalf("len(alerts) = %d, want 1; alerts: %v", len(alerts), alerts)
		}
		msg := alerts[0].Message
		if !strings.Contains(msg, "7 task(s)") {
			t.Errorf("Message = %q, expected '7 task(s)'", msg)
		}
		if !strings.Contains(msg, "... and 2 more") {
			t.Errorf("Message = %q, expected '... and 2 more'", msg)
		}
	})
}

func TestCheckStaleSentinels(t *testing.T) {
	now := time.Now().UTC()

	t.Run("detects stale sentinel", func(t *testing.T) {
		sentinel := "$transitioning"
		task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusRejected, now)
		task.AssignedTo = &sentinel

		cache := map[string]time.Time{
			"sentinel:task-1": now.Add(-3 * time.Minute),
		}
		state := &models.State{Tasks: []models.Task{task}}
		alerts := checkStaleSentinels(state, cache)

		if len(alerts) != 1 {
			t.Fatalf("len(alerts) = %d, want 1", len(alerts))
		}
		if alerts[0].Level != AlertLevelCritical {
			t.Errorf("level = %q, want %q", alerts[0].Level, AlertLevelCritical)
		}
		if alerts[0].Category != "STALE SENTINEL" {
			t.Errorf("category = %q, want %q", alerts[0].Category, "STALE SENTINEL")
		}
		if !strings.Contains(alerts[0].Message, "stuck in transition") {
			t.Errorf("message = %q, want containing %q", alerts[0].Message, "stuck in transition")
		}
	})

	t.Run("repeated alert on every poll", func(t *testing.T) {
		sentinel := "$transitioning"
		task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusRejected, now)
		task.AssignedTo = &sentinel

		cache := map[string]time.Time{
			"sentinel:task-1": now.Add(-3 * time.Minute),
		}
		state := &models.State{Tasks: []models.Task{task}}

		// First poll — should alert.
		alerts1 := checkStaleSentinels(state, cache)
		if len(alerts1) != 1 {
			t.Fatalf("first poll: len(alerts) = %d, want 1", len(alerts1))
		}
		if alerts1[0].Level != AlertLevelCritical {
			t.Errorf("first poll: level = %q, want %q", alerts1[0].Level, AlertLevelCritical)
		}
		if alerts1[0].Category != "STALE SENTINEL" {
			t.Errorf("first poll: category = %q, want %q", alerts1[0].Category, "STALE SENTINEL")
		}

		// Second poll with same stale sentinel — must alert again (no suppression).
		alerts2 := checkStaleSentinels(state, cache)
		if len(alerts2) != 1 {
			t.Fatalf("second poll: len(alerts) = %d, want 1", len(alerts2))
		}
		if alerts2[0].Level != AlertLevelCritical {
			t.Errorf("second poll: level = %q, want %q", alerts2[0].Level, AlertLevelCritical)
		}
		if alerts2[0].Category != "STALE SENTINEL" {
			t.Errorf("second poll: category = %q, want %q", alerts2[0].Category, "STALE SENTINEL")
		}
	})

	t.Run("no alert within threshold", func(t *testing.T) {
		sentinel := "$transitioning"
		task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusRejected, now)
		task.AssignedTo = &sentinel

		cache := map[string]time.Time{
			"sentinel:task-1": now.Add(-1 * time.Minute),
		}
		state := &models.State{Tasks: []models.Task{task}}
		alerts := checkStaleSentinels(state, cache)

		if len(alerts) != 0 {
			t.Errorf("len(alerts) = %d, want 0", len(alerts))
		}
	})

	t.Run("first seen starts tracking", func(t *testing.T) {
		sentinel := "$transitioning"
		task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusRejected, now)
		task.AssignedTo = &sentinel

		cache := make(map[string]time.Time)
		state := &models.State{Tasks: []models.Task{task}}
		alerts := checkStaleSentinels(state, cache)

		if len(alerts) != 0 {
			t.Errorf("len(alerts) = %d, want 0", len(alerts))
		}
		if _, exists := cache["sentinel:task-1"]; !exists {
			t.Error("cache should contain 'sentinel:task-1' entry after first seen")
		}
	})

	t.Run("clears cache when sentinel resolves", func(t *testing.T) {
		// No tasks with sentinel AssignedTo — sentinel has resolved.
		task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusImplementing, now)

		cache := map[string]time.Time{
			"sentinel:task-1": now.Add(-3 * time.Minute),
		}
		state := &models.State{Tasks: []models.Task{task}}
		alerts := checkStaleSentinels(state, cache)

		if len(alerts) != 0 {
			t.Errorf("len(alerts) = %d, want 0", len(alerts))
		}
		if _, exists := cache["sentinel:task-1"]; exists {
			t.Error("cache entry 'sentinel:task-1' should have been cleared when sentinel resolved")
		}
	})
}

func TestRunChecksWithState_BlockedTask(t *testing.T) {
	now := time.Now().UTC()

	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)
	testhelpers.SetupPipelineConfig(t, tmpDir)
	alertsLog := paths.New(tmpDir).AlertsLogPath()

	state := testhelpers.CreateValidState()
	blockedReason := "spec ambiguity"
	state.Tasks = []models.Task{
		{
			ID:            "blocked-task-1",
			Type:          models.TaskTypeCoding,
			Description:   "A blocked task",
			Status:        models.TaskStatusBlocked,
			Priority:      1,
			Created:       now,
			SpecRef:       "README.md",
			DoneWhen:      "Task is complete",
			Scope:         "Test scope",
			BlockedReason: &blockedReason,
		},
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	config := WatchConfig{
		ProjectRoot: tmpDir,
		AlertsLog:   alertsLog,
		StateCache:  make(map[string]time.Time),
	}

	alerts := RunChecksWithState(state, config)

	// Find an alert with category "BLOCKED".
	var found bool
	for _, a := range alerts {
		if a.Category == "BLOCKED" {
			found = true
			if a.Level != AlertLevelWarning {
				t.Errorf("BLOCKED alert level = %q, want %q", a.Level, AlertLevelWarning)
			}
			if !strings.Contains(a.Message, "blocked-task-1") {
				t.Errorf("BLOCKED alert message = %q, want containing %q", a.Message, "blocked-task-1")
			}
			break
		}
	}
	if !found {
		t.Errorf("expected alert with category BLOCKED, got %d alerts: %v", len(alerts), alerts)
	}
}

func TestRunChecksWithState_RunningTaskWithoutLiveProcess(t *testing.T) {
	now := time.Now().UTC()

	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)
	testhelpers.SetupPipelineConfig(t, tmpDir)

	taskID := "task-us-writing"
	agentID := "us-writer-1"
	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{
		buildPipelineTask(taskID, "us-writing-pair", models.TaskStatus("WRITING_US"), now, func(task *models.Task) {
			task.AssignedTo = &agentID
		}),
	}
	state.Agents = map[string]models.Agent{
		agentID: {
			Role:         "us-writer",
			Status:       models.AgentStatusWorking,
			CurrentTask:  &taskID,
			LeaseExpires: testhelpers.TimePtr(now.Add(30 * time.Minute)),
			Heartbeat:    now,
			PID:          0,
		},
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	config := WatchConfig{
		ProjectRoot: tmpDir,
		AlertsLog:   paths.New(tmpDir).AlertsLogPath(),
		StateCache:  make(map[string]time.Time),
	}

	alerts := RunChecksWithState(state, config)
	if got := countAlertsByCategory(alerts, "DEAD AGENT PROCESS"); got != 1 {
		t.Fatalf("DEAD AGENT PROCESS alerts = %d, want 1; alerts: %v", got, alerts)
	}
	if !strings.Contains(alerts[0].Message, taskID) || !strings.Contains(alerts[0].Message, agentID) || !strings.Contains(alerts[0].Message, "WRITING_US") {
		t.Fatalf("alert message = %q, want task, agent, and WRITING_US", alerts[0].Message)
	}
}

func TestRunChecksWithState_LiveDoerProcessNoAlert(t *testing.T) {
	now := time.Now().UTC()

	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)
	testhelpers.SetupPipelineConfig(t, tmpDir)

	taskID := "task-us-writing"
	agentID := "us-writer-1"
	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{
		buildPipelineTask(taskID, "us-writing-pair", models.TaskStatus("WRITING_US"), now, func(task *models.Task) {
			task.AssignedTo = &agentID
		}),
	}
	state.Agents = map[string]models.Agent{
		agentID: {
			Role:         "us-writer",
			Status:       models.AgentStatusWorking,
			CurrentTask:  &taskID,
			LeaseExpires: testhelpers.TimePtr(now.Add(30 * time.Minute)),
			Heartbeat:    now,
			PID:          os.Getpid(),
		},
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	config := WatchConfig{
		ProjectRoot: tmpDir,
		AlertsLog:   paths.New(tmpDir).AlertsLogPath(),
		StateCache:  make(map[string]time.Time),
	}

	alerts := RunChecksWithState(state, config)
	if got := countAlertsByCategory(alerts, "DEAD AGENT PROCESS"); got != 0 {
		t.Fatalf("DEAD AGENT PROCESS alerts = %d, want 0; alerts: %v", got, alerts)
	}
}

func TestRunChecksWithState_OwnedExecutingRecoveryNoInvalidOwnership(t *testing.T) {
	now := time.Now().UTC()

	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)
	testhelpers.SetupPipelineConfig(t, tmpDir)

	taskID := "task-us-writing"
	agentID := "us-writer-1"
	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{
		buildPipelineTask(taskID, "us-writing-pair", models.TaskStatus("WRITING_US"), now, func(task *models.Task) {
			task.AssignedTo = &agentID
		}),
	}
	state.Agents = map[string]models.Agent{
		agentID: {
			Role:         "us-writer",
			Status:       models.AgentStatusIdle,
			LeaseExpires: testhelpers.TimePtr(now.Add(30 * time.Minute)),
			Heartbeat:    now,
			Provider:     "test",
			PID:          os.Getpid(),
		},
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	config := WatchConfig{
		ProjectRoot: tmpDir,
		AlertsLog:   paths.New(tmpDir).AlertsLogPath(),
		StateCache:  make(map[string]time.Time),
	}

	alerts := RunChecksWithState(state, config)
	if got := countAlertsByCategory(alerts, "DEAD AGENT PROCESS"); got != 0 {
		t.Fatalf("DEAD AGENT PROCESS alerts = %d, want 0; alerts: %v", got, alerts)
	}
	if got := countAlertsByCategory(alerts, "INVALID AGENT OWNERSHIP"); got != 0 {
		t.Fatalf("INVALID AGENT OWNERSHIP alerts = %d, want 0; alerts: %v", got, alerts)
	}
}

func TestRunChecksWithState_HandoffOwnershipNoInvalidOwnership(t *testing.T) {
	now := time.Now().UTC()

	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)
	testhelpers.SetupPipelineConfig(t, tmpDir)

	taskID := "task-us-writing"
	agentID := "us-writer-1"
	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{
		buildPipelineTask(taskID, "us-writing-pair", models.TaskStatus("WRITING_US"), now, func(task *models.Task) {
			task.AssignedTo = &agentID
			task.HandoffPending = true
		}),
	}
	state.Agents = map[string]models.Agent{
		agentID: {
			Role:         "us-writer",
			Status:       models.AgentStatusHandoff,
			CurrentTask:  &taskID,
			LeaseExpires: testhelpers.TimePtr(now.Add(30 * time.Minute)),
			Heartbeat:    now,
			Provider:     "test",
			PID:          os.Getpid(),
		},
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	config := WatchConfig{
		ProjectRoot: tmpDir,
		AlertsLog:   paths.New(tmpDir).AlertsLogPath(),
		StateCache:  make(map[string]time.Time),
	}

	alerts := RunChecksWithState(state, config)
	if got := countAlertsByCategory(alerts, "DEAD AGENT PROCESS"); got != 0 {
		t.Fatalf("DEAD AGENT PROCESS alerts = %d, want 0; alerts: %v", got, alerts)
	}
	if got := countAlertsByCategory(alerts, "INVALID AGENT OWNERSHIP"); got != 0 {
		t.Fatalf("INVALID AGENT OWNERSHIP alerts = %d, want 0; alerts: %v", got, alerts)
	}
}

func TestRunChecksWithState_ExecutingSentinelNoDeadProcessAlert(t *testing.T) {
	now := time.Now().UTC()

	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)
	testhelpers.SetupPipelineConfig(t, tmpDir)

	sentinel := "$transitioning"
	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{
		buildPipelineTask("task-us-writing", "us-writing-pair", models.TaskStatus("WRITING_US"), now, func(task *models.Task) {
			task.AssignedTo = &sentinel
		}),
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	config := WatchConfig{
		ProjectRoot: tmpDir,
		AlertsLog:   paths.New(tmpDir).AlertsLogPath(),
		StateCache:  make(map[string]time.Time),
	}

	alerts := RunChecksWithState(state, config)
	if got := countAlertsByCategory(alerts, "DEAD AGENT PROCESS"); got != 0 {
		t.Fatalf("DEAD AGENT PROCESS alerts = %d, want 0; alerts: %v", got, alerts)
	}
}

func TestRunChecksWithState_ReviewerWithoutLiveProcess(t *testing.T) {
	now := time.Now().UTC()

	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)
	testhelpers.SetupPipelineConfig(t, tmpDir)

	taskID := "task-us-review"
	reviewerID := "us-reviewer-1"
	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{
		buildPipelineTask(taskID, "us-writing-pair", models.TaskStatus("REVIEWING_US"), now, func(task *models.Task) {
			task.ReviewingBy = &reviewerID
			task.ReviewLeaseExpires = testhelpers.TimePtr(now.Add(30 * time.Minute))
			task.ReviewCommit = testhelpers.StringPtr("review123")
		}),
	}
	state.Agents = map[string]models.Agent{
		reviewerID: {
			Role:         "us-reviewer",
			Status:       models.AgentStatusReviewing,
			CurrentTask:  &taskID,
			LeaseExpires: testhelpers.TimePtr(now.Add(30 * time.Minute)),
			Heartbeat:    now,
			PID:          0,
		},
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	config := WatchConfig{
		ProjectRoot: tmpDir,
		AlertsLog:   paths.New(tmpDir).AlertsLogPath(),
		StateCache:  make(map[string]time.Time),
	}

	alerts := RunChecksWithState(state, config)
	if got := countAlertsByCategory(alerts, "DEAD AGENT PROCESS"); got != 1 {
		t.Fatalf("DEAD AGENT PROCESS alerts = %d, want 1; alerts: %v", got, alerts)
	}
	if !strings.Contains(alerts[0].Message, "REVIEWING_US") {
		t.Fatalf("alert message = %q, want REVIEWING_US", alerts[0].Message)
	}
}

func TestRunChecksWithState_Reviewing2WithoutLiveProcess(t *testing.T) {
	now := time.Now().UTC()

	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)
	testhelpers.SetupPipelineConfig(t, tmpDir)

	taskID := "task-reviewing-2"
	reviewerID := "code-reviewer-2"
	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{
		buildPipelineTask(taskID, "coding-pair", models.TaskStatus("REVIEWING_CODE_2"), now, func(task *models.Task) {
			task.AssignedTo = testhelpers.StringPtr("coder-1")
			task.ReviewingBy = &reviewerID
			task.ReviewLeaseExpires = testhelpers.TimePtr(now.Add(30 * time.Minute))
			task.ReviewCommit = testhelpers.StringPtr("review123")
		}),
	}
	state.Agents = map[string]models.Agent{
		reviewerID: {
			Role:         "code-reviewer",
			Status:       models.AgentStatusReviewing,
			CurrentTask:  &taskID,
			LeaseExpires: testhelpers.TimePtr(now.Add(30 * time.Minute)),
			Heartbeat:    now,
			PID:          0,
		},
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	config := WatchConfig{
		ProjectRoot: tmpDir,
		AlertsLog:   paths.New(tmpDir).AlertsLogPath(),
		StateCache:  make(map[string]time.Time),
	}

	alerts := RunChecksWithState(state, config)
	if got := countAlertsByCategory(alerts, "DEAD AGENT PROCESS"); got != 1 {
		t.Fatalf("DEAD AGENT PROCESS alerts = %d, want 1; alerts: %v", got, alerts)
	}
	if !strings.Contains(alerts[0].Message, "REVIEWING_CODE_2") {
		t.Fatalf("alert message = %q, want REVIEWING_CODE_2", alerts[0].Message)
	}
}

func TestRunChecksWithState_LiveReviewerProcessNoAlert(t *testing.T) {
	now := time.Now().UTC()

	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)
	testhelpers.SetupPipelineConfig(t, tmpDir)

	taskID := "task-us-review"
	reviewerID := "us-reviewer-1"
	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{
		buildPipelineTask(taskID, "us-writing-pair", models.TaskStatus("REVIEWING_US"), now, func(task *models.Task) {
			task.ReviewingBy = &reviewerID
			task.ReviewLeaseExpires = testhelpers.TimePtr(now.Add(30 * time.Minute))
			task.ReviewCommit = testhelpers.StringPtr("review123")
		}),
	}
	state.Agents = map[string]models.Agent{
		reviewerID: {
			Role:         "us-reviewer",
			Status:       models.AgentStatusReviewing,
			CurrentTask:  &taskID,
			LeaseExpires: testhelpers.TimePtr(now.Add(30 * time.Minute)),
			Heartbeat:    now,
			PID:          os.Getpid(),
		},
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	config := WatchConfig{
		ProjectRoot: tmpDir,
		AlertsLog:   paths.New(tmpDir).AlertsLogPath(),
		StateCache:  make(map[string]time.Time),
	}

	alerts := RunChecksWithState(state, config)
	if got := countAlertsByCategory(alerts, "DEAD AGENT PROCESS"); got != 0 {
		t.Fatalf("DEAD AGENT PROCESS alerts = %d, want 0; alerts: %v", got, alerts)
	}
	if got := countAlertsByCategory(alerts, "INVALID AGENT OWNERSHIP"); got != 0 {
		t.Fatalf("INVALID AGENT OWNERSHIP alerts = %d, want 0; alerts: %v", got, alerts)
	}
}

func TestCheckRunningTasksWithoutLiveProcess_LiveReviewerInvalidOwnership(t *testing.T) {
	now := time.Now().UTC()
	pr := loadPipelineResolverForWatchTest(t)

	tests := []struct {
		name    string
		agent   models.Agent
		wantMsg string
	}{
		{
			name: "idle reviewer",
			agent: models.Agent{
				Role:         "us-reviewer",
				Status:       models.AgentStatusIdle,
				LeaseExpires: testhelpers.TimePtr(now.Add(30 * time.Minute)),
				Heartbeat:    now,
				PID:          os.Getpid(),
			},
			wantMsg: "agent status IDLE, want REVIEWING",
		},
		{
			name: "wrong exact reviewer role",
			agent: models.Agent{
				Role:         "code-reviewer",
				Status:       models.AgentStatusReviewing,
				CurrentTask:  testhelpers.StringPtr("task-us-review"),
				LeaseExpires: testhelpers.TimePtr(now.Add(30 * time.Minute)),
				Heartbeat:    now,
				PID:          os.Getpid(),
			},
			wantMsg: `agent role "code-reviewer", want "us-reviewer"`,
		},
		{
			name: "mismatched current task",
			agent: models.Agent{
				Role:         "us-reviewer",
				Status:       models.AgentStatusReviewing,
				CurrentTask:  testhelpers.StringPtr("other-task"),
				LeaseExpires: testhelpers.TimePtr(now.Add(30 * time.Minute)),
				Heartbeat:    now,
				PID:          os.Getpid(),
			},
			wantMsg: `current_task "other-task", want "task-us-review"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reviewerID := "us-reviewer-1"
			state := testhelpers.CreateValidState()
			state.Tasks = []models.Task{
				buildPipelineTask("task-us-review", "us-writing-pair", models.TaskStatus("REVIEWING_US"), now, func(task *models.Task) {
					task.ReviewingBy = &reviewerID
					task.ReviewLeaseExpires = testhelpers.TimePtr(now.Add(30 * time.Minute))
					task.ReviewCommit = testhelpers.StringPtr("review123")
				}),
			}
			state.Agents = map[string]models.Agent{reviewerID: tt.agent}

			alerts := checkRunningTasksWithoutLiveProcess(state, pr)
			if got := countAlertsByCategory(alerts, "INVALID AGENT OWNERSHIP"); got != 1 {
				t.Fatalf("INVALID AGENT OWNERSHIP alerts = %d, want 1; alerts: %v", got, alerts)
			}
			if !strings.Contains(alerts[0].Message, tt.wantMsg) {
				t.Fatalf("alert message = %q, want %q", alerts[0].Message, tt.wantMsg)
			}
		})
	}
}

func TestCheckRunningTasksWithoutLiveProcess_ReviewerReverseOwnershipMismatch(t *testing.T) {
	now := time.Now().UTC()
	pr := loadPipelineResolverForWatchTest(t)

	taskID := "task-us-review"
	reviewerID := "us-reviewer-1"
	otherReviewerID := "us-reviewer-2"
	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{
		buildPipelineTask(taskID, "us-writing-pair", models.TaskStatus("REVIEWING_US"), now, func(task *models.Task) {
			task.ReviewingBy = &otherReviewerID
			task.ReviewLeaseExpires = testhelpers.TimePtr(now.Add(30 * time.Minute))
			task.ReviewCommit = testhelpers.StringPtr("review123")
		}),
	}
	state.Agents = map[string]models.Agent{
		reviewerID: {
			Role:         "us-reviewer",
			Status:       models.AgentStatusReviewing,
			CurrentTask:  &taskID,
			LeaseExpires: testhelpers.TimePtr(now.Add(30 * time.Minute)),
			Heartbeat:    now,
			PID:          os.Getpid(),
		},
		otherReviewerID: {
			Role:         "us-reviewer",
			Status:       models.AgentStatusReviewing,
			CurrentTask:  &taskID,
			LeaseExpires: testhelpers.TimePtr(now.Add(30 * time.Minute)),
			Heartbeat:    now,
			PID:          os.Getpid(),
		},
	}

	alerts := checkRunningTasksWithoutLiveProcess(state, pr)
	if got := countAlertsByCategory(alerts, "INVALID AGENT OWNERSHIP"); got != 1 {
		t.Fatalf("INVALID AGENT OWNERSHIP alerts = %d, want 1; alerts: %v", got, alerts)
	}
	alert := firstAlertByCategory(t, alerts, "INVALID AGENT OWNERSHIP")
	if !strings.Contains(alert.Message, "agent us-reviewer-1 says REVIEWING task-us-review") {
		t.Fatalf("alert message = %q, want reverse ownership mismatch", alert.Message)
	}
	if !strings.Contains(alert.Message, "task has reviewer us-reviewer-2") {
		t.Fatalf("alert message = %q, want task reviewer", alert.Message)
	}
}

func TestCheckReverseActiveAgentOwnership_OrchestratorPlanningCurrentTaskIsNotOwnership(t *testing.T) {
	now := time.Now().UTC()
	pr := loadPipelineResolverForWatchTest(t)
	planning := "planning"

	state := testhelpers.CreateValidState()
	state.Tasks = nil
	state.Agents = map[string]models.Agent{
		"orchestrator-1": {
			Role:         "orchestrator",
			Status:       models.AgentStatusPlanning,
			CurrentTask:  &planning,
			LeaseExpires: testhelpers.TimePtr(now.Add(30 * time.Minute)),
			Heartbeat:    now,
			Provider:     "test",
			PID:          os.Getpid(),
		},
	}

	alerts := checkRunningTasksWithoutLiveProcess(state, pr)
	if got := countAlertsByCategory(alerts, "INVALID AGENT OWNERSHIP"); got != 0 {
		t.Fatalf("INVALID AGENT OWNERSHIP alerts = %d, want 0; alerts: %v", got, alerts)
	}
}

func TestRunChecksWithState_StuckAlertsDedupedAndRealertAfterClear(t *testing.T) {
	now := time.Now().UTC()

	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)
	testhelpers.SetupPipelineConfig(t, tmpDir)

	config := WatchConfig{
		ProjectRoot: tmpDir,
		AlertsLog:   paths.New(tmpDir).AlertsLogPath(),
		StateCache:  make(map[string]time.Time),
	}

	failedState := testhelpers.CreateValidState()
	failedState.Tasks = []models.Task{
		testhelpers.BuildTaskByStatus("task-1", models.TaskStatusIntegrationFailed, now),
	}
	testhelpers.WriteInitialState(t, stateFile, failedState)

	alerts := RunChecksWithState(failedState, config)
	if got := countAlertsByCategory(alerts, "INTEGRATION FAILED"); got != 1 {
		t.Fatalf("first poll INTEGRATION FAILED alerts = %d, want 1; alerts: %v", got, alerts)
	}

	alerts = RunChecksWithState(failedState, config)
	if got := countAlertsByCategory(alerts, "INTEGRATION FAILED"); got != 0 {
		t.Fatalf("second poll INTEGRATION FAILED alerts = %d, want 0; alerts: %v", got, alerts)
	}

	mergedState := testhelpers.CreateValidState()
	mergedState.Tasks = []models.Task{
		testhelpers.BuildTaskByStatus("task-1", models.TaskStatusMerged, now),
	}
	testhelpers.WriteInitialState(t, stateFile, mergedState)

	alerts = RunChecksWithState(mergedState, config)
	if got := countAlertsByCategory(alerts, "INTEGRATION FAILED"); got != 0 {
		t.Fatalf("clear poll INTEGRATION FAILED alerts = %d, want 0; alerts: %v", got, alerts)
	}

	testhelpers.WriteInitialState(t, stateFile, failedState)
	alerts = RunChecksWithState(failedState, config)
	if got := countAlertsByCategory(alerts, "INTEGRATION FAILED"); got != 1 {
		t.Fatalf("recurrence poll INTEGRATION FAILED alerts = %d, want 1; alerts: %v", got, alerts)
	}
}

func TestRunChecksWithStateSnapshot_ReportsActiveKeysForThrottledAlerts(t *testing.T) {
	now := time.Now().UTC()

	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)
	testhelpers.SetupPipelineConfig(t, tmpDir)

	config := WatchConfig{
		ProjectRoot: tmpDir,
		AlertsLog:   paths.New(tmpDir).AlertsLogPath(),
		StateCache:  make(map[string]time.Time),
	}

	failedState := testhelpers.CreateValidState()
	failedState.Tasks = []models.Task{
		testhelpers.BuildTaskByStatus("task-1", models.TaskStatusIntegrationFailed, now),
	}
	testhelpers.WriteInitialState(t, stateFile, failedState)

	first := RunChecksWithStateSnapshot(failedState, config)
	if got := countAlertsByCategory(first.Alerts, "INTEGRATION FAILED"); got != 1 {
		t.Fatalf("first poll emitted alerts = %d, want 1; alerts: %v", got, first.Alerts)
	}
	if len(first.ActiveKeys) == 0 {
		t.Fatal("first poll should report active alert keys")
	}

	second := RunChecksWithStateSnapshot(failedState, config)
	if got := countAlertsByCategory(second.Alerts, "INTEGRATION FAILED"); got != 0 {
		t.Fatalf("second poll emitted alerts = %d, want 0; alerts: %v", got, second.Alerts)
	}
	if len(second.ActiveKeys) == 0 {
		t.Fatal("second poll should keep active alert key despite throttling")
	}

	mergedState := testhelpers.CreateValidState()
	mergedState.Tasks = []models.Task{
		testhelpers.BuildTaskByStatus("task-1", models.TaskStatusMerged, now),
	}
	testhelpers.WriteInitialState(t, stateFile, mergedState)

	cleared := RunChecksWithStateSnapshot(mergedState, config)
	for key := range second.ActiveKeys {
		if cleared.ActiveKeys[key] {
			t.Fatalf("cleared poll still reports inactive key %q active", key)
		}
	}
}

func TestRunChecksWithStateSnapshot_ReportsActiveKeysForLeaseExpired(t *testing.T) {
	now := time.Now().UTC()

	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)
	testhelpers.SetupPipelineConfig(t, tmpDir)

	taskID := "task-lease"
	expiredLease := now.Add(-models.LeaseExpiryGracePeriod - time.Minute)
	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{
		testhelpers.BuildTaskByStatus(taskID, models.TaskStatusImplementing, now),
	}
	state.Agents = map[string]models.Agent{
		"coder-1": {
			Role:         models.RoleCoder,
			Status:       models.AgentStatusWorking,
			CurrentTask:  &taskID,
			LeaseExpires: &expiredLease,
			Heartbeat:    now,
		},
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	config := WatchConfig{
		ProjectRoot: tmpDir,
		AlertsLog:   paths.New(tmpDir).AlertsLogPath(),
		StateCache:  make(map[string]time.Time),
	}

	snapshot := RunChecksWithStateSnapshot(state, config)
	if got := countAlertsByCategory(snapshot.Alerts, "LEASE EXPIRED"); got != 1 {
		t.Fatalf("LEASE EXPIRED alerts = %d, want 1; alerts: %v", got, snapshot.Alerts)
	}

	for _, alert := range snapshot.Alerts {
		if alert.Category != "LEASE EXPIRED" {
			continue
		}
		if !snapshot.ActiveKeys[AlertKey(alert)] {
			t.Fatalf("LEASE EXPIRED active key %q missing from snapshot", AlertKey(alert))
		}
		return
	}
	t.Fatal("missing LEASE EXPIRED alert")
}

func TestRunChecksWithStateSnapshot_ReportsActiveKeysForDeadAgentProcess(t *testing.T) {
	now := time.Now().UTC()

	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)
	testhelpers.SetupPipelineConfig(t, tmpDir)

	taskID := "task-us-writing"
	agentID := "us-writer-1"
	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{
		buildPipelineTask(taskID, "us-writing-pair", models.TaskStatus("WRITING_US"), now, func(task *models.Task) {
			task.AssignedTo = &agentID
		}),
	}
	state.Agents = map[string]models.Agent{
		agentID: {
			Role:         "us-writer",
			Status:       models.AgentStatusWorking,
			CurrentTask:  &taskID,
			LeaseExpires: testhelpers.TimePtr(now.Add(30 * time.Minute)),
			Heartbeat:    now,
			PID:          0,
		},
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	config := WatchConfig{
		ProjectRoot: tmpDir,
		AlertsLog:   paths.New(tmpDir).AlertsLogPath(),
		StateCache:  make(map[string]time.Time),
	}

	first := RunChecksWithStateSnapshot(state, config)
	if got := countAlertsByCategory(first.Alerts, "DEAD AGENT PROCESS"); got != 1 {
		t.Fatalf("first poll emitted alerts = %d, want 1; alerts: %v", got, first.Alerts)
	}
	var alertKey string
	for _, alert := range first.Alerts {
		if alert.Category == "DEAD AGENT PROCESS" {
			alertKey = AlertKey(alert)
			break
		}
	}
	if alertKey == "" {
		t.Fatal("missing DEAD AGENT PROCESS alert")
	}
	if !first.ActiveKeys[alertKey] {
		t.Fatalf("first poll active key %q missing", alertKey)
	}

	second := RunChecksWithStateSnapshot(state, config)
	if got := countAlertsByCategory(second.Alerts, "DEAD AGENT PROCESS"); got != 0 {
		t.Fatalf("second poll emitted alerts = %d, want 0; alerts: %v", got, second.Alerts)
	}
	if !second.ActiveKeys[alertKey] {
		t.Fatalf("second poll active key %q missing despite throttling", alertKey)
	}

	state.Agents[agentID] = models.Agent{
		Role:         "us-writer",
		Status:       models.AgentStatusWorking,
		CurrentTask:  &taskID,
		LeaseExpires: testhelpers.TimePtr(now.Add(30 * time.Minute)),
		Heartbeat:    now,
		PID:          os.Getpid(),
	}
	cleared := RunChecksWithStateSnapshot(state, config)
	if cleared.ActiveKeys[alertKey] {
		t.Fatalf("cleared poll still reports inactive key %q active", alertKey)
	}
}

func countAlertsByCategory(alerts []Alert, category string) int {
	count := 0
	for _, alert := range alerts {
		if alert.Category == category {
			count++
		}
	}
	return count
}

func firstAlertByCategory(t *testing.T, alerts []Alert, category string) Alert {
	t.Helper()
	for _, alert := range alerts {
		if alert.Category == category {
			return alert
		}
	}
	t.Fatalf("missing alert category %q; alerts: %v", category, alerts)
	return Alert{}
}

func loadPipelineResolverForWatchTest(t *testing.T) models.PipelineResolver {
	t.Helper()
	tmpDir := t.TempDir()
	testhelpers.SetupPipelineConfig(t, tmpDir)
	pr, err := ops.LoadResolverForModels(tmpDir)
	if err != nil {
		t.Fatalf("failed to load resolver: %v", err)
	}
	return pr
}

func buildPipelineTask(taskID, rolePair string, status models.TaskStatus, now time.Time, mutate func(*models.Task)) models.Task {
	task := models.Task{
		ID:          taskID,
		Type:        models.TaskTypeCoding,
		RolePair:    rolePair,
		Description: "Test task",
		Status:      status,
		Priority:    1,
		Created:     now,
		SpecRef:     "README.md",
		DoneWhen:    "Task is complete",
		Scope:       "Test scope",
		BaseCommit:  testhelpers.StringPtr("abc1234"),
		Worktree:    testhelpers.StringPtr(".worktrees/" + taskID),
		History:     []models.TaskHistoryEntry{},
	}
	if mutate != nil {
		mutate(&task)
	}
	if task.AssignedTo != nil && task.LeaseExpires == nil {
		task.LeaseExpires = testhelpers.TimePtr(now.Add(30 * time.Minute))
	}
	return task
}

func TestParseAlertLine_RoundTrip(t *testing.T) {
	original := Alert{
		Timestamp: time.Date(2026, 4, 4, 12, 0, 0, 0, time.UTC),
		Level:     "⚠️",
		Category:  "BLOCKED",
		Message:   "task blocked-1 is stuck",
	}

	line := original.String()
	parsed, ok := ParseAlertLine(line)
	if !ok {
		t.Fatalf("ParseAlertLine(%q) returned false", line)
	}
	if !parsed.Timestamp.Equal(original.Timestamp) {
		t.Errorf("timestamp = %v, want %v", parsed.Timestamp, original.Timestamp)
	}
	if parsed.Level != original.Level {
		t.Errorf("level = %q, want %q", parsed.Level, original.Level)
	}
	if parsed.Category != original.Category {
		t.Errorf("category = %q, want %q", parsed.Category, original.Category)
	}
	if parsed.Message != original.Message {
		t.Errorf("message = %q, want %q", parsed.Message, original.Message)
	}
}

func TestParseAlertLine_Malformed(t *testing.T) {
	cases := []string{
		"",
		"no brackets",
		"[bad-timestamp] ⚠️ CAT: msg",
		"[2026-04-04T12:00:00Z] nocolon",
		"[2026-04-04T12:00:00Z]",
	}
	for _, line := range cases {
		if _, ok := ParseAlertLine(line); ok {
			t.Errorf("ParseAlertLine(%q) should return false", line)
		}
	}
}

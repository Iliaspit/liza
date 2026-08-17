package commands

import (
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/ops"
	"github.com/liza-mas/liza/internal/testhelpers"
)

const (
	boundedAwaitTaskID     = "task-1"
	boundedAwaitCoderID    = "coder-1"
	boundedAwaitReviewerID = "code-reviewer-1"

	// maxSafeForegroundAwaitInterval leaves scheduling headroom below the
	// provider threshold that backgrounds long-running foreground commands.
	maxSafeForegroundAwaitInterval = 120 * time.Second
)

type boundedAwaitFixture struct {
	projectRoot string
	bb          *db.Blackboard
}

type asyncAwaitResult[T any] struct {
	result T
	err    error
}

func TestMaxAwaitIntervalStaysBelowForegroundTransportLimit(t *testing.T) {
	if maxAwaitInterval >= maxSafeForegroundAwaitInterval {
		t.Fatalf("maxAwaitInterval = %s, must stay below foreground transport safety limit %s",
			maxAwaitInterval, maxSafeForegroundAwaitInterval)
	}
}

func TestAwaitVerdictWithBudget_DecreasesUntilExhausted(t *testing.T) {
	remaining := 250 * time.Second
	wantVerdicts := []string{ops.VerdictPoll, ops.VerdictPoll, ops.VerdictTimeout}
	wantRemaining := []int{150, 50, 0}
	wantIntervals := []time.Duration{100 * time.Second, 100 * time.Second, 50 * time.Second}

	for i, wantVerdict := range wantVerdicts {
		var gotInterval time.Duration
		result, err := awaitVerdictWithBudget(remaining, 100*time.Second, func(interval time.Duration) (*ops.AwaitVerdictResult, error) {
			gotInterval = interval
			return &ops.AwaitVerdictResult{Verdict: ops.VerdictTimeout}, nil
		})
		if err != nil {
			t.Fatalf("call %d: awaitVerdictWithBudget error: %v", i+1, err)
		}
		if gotInterval != wantIntervals[i] {
			t.Errorf("call %d: interval = %s, want %s", i+1, gotInterval, wantIntervals[i])
		}
		if result.Verdict != wantVerdict {
			t.Errorf("call %d: verdict = %q, want %q", i+1, result.Verdict, wantVerdict)
		}
		if result.TimeoutSeconds != wantRemaining[i] {
			t.Errorf("call %d: timeout_seconds = %d, want %d", i+1, result.TimeoutSeconds, wantRemaining[i])
		}
		remaining = time.Duration(result.TimeoutSeconds) * time.Second
	}
}

func TestAwaitResubmissionWithBudget_DecreasesUntilExhausted(t *testing.T) {
	remaining := 250 * time.Second
	wantVerdicts := []string{ops.ResubmissionPoll, ops.ResubmissionPoll, ops.ResubmissionTimeout}
	wantRemaining := []int{150, 50, 0}
	wantIntervals := []time.Duration{100 * time.Second, 100 * time.Second, 50 * time.Second}

	for i, wantVerdict := range wantVerdicts {
		var gotInterval time.Duration
		result, err := awaitResubmissionWithBudget(remaining, 100*time.Second, func(interval time.Duration) (*ops.AwaitResubmissionResult, error) {
			gotInterval = interval
			return &ops.AwaitResubmissionResult{Verdict: ops.ResubmissionTimeout}, nil
		})
		if err != nil {
			t.Fatalf("call %d: awaitResubmissionWithBudget error: %v", i+1, err)
		}
		if gotInterval != wantIntervals[i] {
			t.Errorf("call %d: interval = %s, want %s", i+1, gotInterval, wantIntervals[i])
		}
		if result.Verdict != wantVerdict {
			t.Errorf("call %d: verdict = %q, want %q", i+1, result.Verdict, wantVerdict)
		}
		if result.TimeoutSeconds != wantRemaining[i] {
			t.Errorf("call %d: timeout_seconds = %d, want %d", i+1, result.TimeoutSeconds, wantRemaining[i])
		}
		remaining = time.Duration(result.TimeoutSeconds) * time.Second
	}
}

func TestAwaitWithBudget_PreservesNonTimeoutOutcome(t *testing.T) {
	result, err := awaitVerdictWithBudget(250*time.Second, 100*time.Second, func(interval time.Duration) (*ops.AwaitVerdictResult, error) {
		return &ops.AwaitVerdictResult{Verdict: ops.VerdictApproved}, nil
	})
	if err != nil {
		t.Fatalf("awaitVerdictWithBudget error: %v", err)
	}
	if result.Verdict != ops.VerdictApproved {
		t.Errorf("verdict = %q, want %q", result.Verdict, ops.VerdictApproved)
	}
	if result.TimeoutSeconds != 0 {
		t.Errorf("timeout_seconds = %d, want 0 for non-POLL outcome", result.TimeoutSeconds)
	}
}

func TestAwaitResubmissionWithBudget_OmitsBudgetFromNonTimeoutOutcome(t *testing.T) {
	result, err := awaitResubmissionWithBudget(250*time.Second, 100*time.Second, func(interval time.Duration) (*ops.AwaitResubmissionResult, error) {
		return &ops.AwaitResubmissionResult{Verdict: ops.ResubmissionResubmitted}, nil
	})
	if err != nil {
		t.Fatalf("awaitResubmissionWithBudget error: %v", err)
	}
	if result.Verdict != ops.ResubmissionResubmitted {
		t.Errorf("verdict = %q, want %q", result.Verdict, ops.ResubmissionResubmitted)
	}
	if result.TimeoutSeconds != 0 {
		t.Errorf("timeout_seconds = %d, want 0 for non-POLL outcome", result.TimeoutSeconds)
	}
}

func TestAwaitCompositionWithInterval_BoundedBudgetLifecycle(t *testing.T) {
	verdictFixture := setupBoundedAwaitFixture(
		t,
		models.TaskStatusReadyForReview,
		boundedAwaitCoderID,
		"coder",
		models.AgentStatusWaiting,
		models.TaskEventSubmittedForReview,
	)
	resubmissionFixture := setupBoundedAwaitFixture(
		t,
		models.TaskStatusRejected,
		boundedAwaitReviewerID,
		"code-reviewer",
		models.AgentStatusIdle,
		models.TaskEventRejected,
	)

	const interval = time.Second
	firstStarted := time.Now()
	verdictCall := startAsyncAwait(func() (*AwaitVerdictResult, error) {
		return awaitVerdictWithInterval(
			verdictFixture.projectRoot, boundedAwaitTaskID, boundedAwaitCoderID, 2*time.Second, interval)
	})
	resubmissionCall := startAsyncAwait(func() (*AwaitResubmissionResult, error) {
		return awaitResubmissionWithInterval(
			resubmissionFixture.projectRoot, boundedAwaitTaskID, boundedAwaitReviewerID, 2*time.Second, interval)
	})

	verdictResult := receiveAsyncAwait(t, verdictCall, 3*time.Second)
	resubmissionResult := receiveAsyncAwait(t, resubmissionCall, 3*time.Second)
	if elapsed := time.Since(firstStarted); elapsed >= 3*time.Second {
		t.Fatalf("first bounded await round took %s, want < 3s", elapsed)
	}
	if verdictResult.Verdict != ops.VerdictPoll || verdictResult.TimeoutSeconds != 1 {
		t.Fatalf("AwaitVerdict result = (%q, %d), want (%q, 1)",
			verdictResult.Verdict, verdictResult.TimeoutSeconds, ops.VerdictPoll)
	}
	if resubmissionResult.Verdict != ops.ResubmissionPoll || resubmissionResult.TimeoutSeconds != 1 {
		t.Fatalf("AwaitResubmission result = (%q, %d), want (%q, 1)",
			resubmissionResult.Verdict, resubmissionResult.TimeoutSeconds, ops.ResubmissionPoll)
	}
	assertBoundedAwaitOwnershipReleased(t, verdictFixture, false)
	assertBoundedAwaitOwnershipReleased(t, resubmissionFixture, true)

	verdictCall = startAsyncAwait(func() (*AwaitVerdictResult, error) {
		return awaitVerdictWithInterval(
			verdictFixture.projectRoot,
			boundedAwaitTaskID,
			boundedAwaitCoderID,
			time.Duration(verdictResult.TimeoutSeconds)*time.Second,
			interval,
		)
	})
	resubmissionCall = startAsyncAwait(func() (*AwaitResubmissionResult, error) {
		return awaitResubmissionWithInterval(
			resubmissionFixture.projectRoot,
			boundedAwaitTaskID,
			boundedAwaitReviewerID,
			time.Duration(resubmissionResult.TimeoutSeconds)*time.Second,
			interval,
		)
	})

	verdictResult = receiveAsyncAwait(t, verdictCall, 3*time.Second)
	resubmissionResult = receiveAsyncAwait(t, resubmissionCall, 3*time.Second)
	if verdictResult.Verdict != ops.VerdictTimeout || verdictResult.TimeoutSeconds != 0 {
		t.Fatalf("final AwaitVerdict result = (%q, %d), want (%q, 0)",
			verdictResult.Verdict, verdictResult.TimeoutSeconds, ops.VerdictTimeout)
	}
	if resubmissionResult.Verdict != ops.ResubmissionTimeout || resubmissionResult.TimeoutSeconds != 0 {
		t.Fatalf("final AwaitResubmission result = (%q, %d), want (%q, 0)",
			resubmissionResult.Verdict, resubmissionResult.TimeoutSeconds, ops.ResubmissionTimeout)
	}
	assertBoundedAwaitOwnershipReleased(t, verdictFixture, false)
	assertBoundedAwaitOwnershipReleased(t, resubmissionFixture, true)
}

func setupBoundedAwaitFixture(
	t *testing.T,
	status models.TaskStatus,
	agentID string,
	role string,
	agentStatus models.AgentStatus,
	event models.TaskEventName,
) boundedAwaitFixture {
	t.Helper()

	projectRoot := t.TempDir()
	statePath, _ := testhelpers.SetupLizaDir(t, projectRoot)
	state := testhelpers.CreateValidState()
	task := testhelpers.BuildTaskByStatus(boundedAwaitTaskID, status, time.Now().UTC())
	task.History = append(task.History, models.TaskHistoryEntry{
		Time:  time.Now().UTC(),
		Event: event,
		Agent: testhelpers.StringPtr(agentID),
	})
	state.Tasks = []models.Task{task}
	state.Agents[agentID] = models.Agent{Role: role, Status: agentStatus}

	return boundedAwaitFixture{
		projectRoot: projectRoot,
		bb:          testhelpers.WriteInitialState(t, statePath, state),
	}
}

func startAsyncAwait[T any](call func() (T, error)) <-chan asyncAwaitResult[T] {
	result := make(chan asyncAwaitResult[T], 1)
	go func() {
		awaitResult, err := call()
		result <- asyncAwaitResult[T]{result: awaitResult, err: err}
	}()
	return result
}

func receiveAsyncAwait[T any](
	t *testing.T,
	call <-chan asyncAwaitResult[T],
	guard time.Duration,
) T {
	t.Helper()
	select {
	case outcome := <-call:
		if outcome.err != nil {
			t.Fatalf("await returned error: %v", outcome.err)
		}
		return outcome.result
	case <-time.After(guard):
		t.Fatalf("await did not return within %s", guard)
		var zero T
		return zero
	}
}

func assertBoundedAwaitOwnershipReleased(
	t *testing.T,
	fixture boundedAwaitFixture,
	reviewer bool,
) {
	t.Helper()
	state, err := fixture.bb.Read()
	if err != nil {
		t.Fatalf("read state after await: %v", err)
	}
	task := state.FindTask(boundedAwaitTaskID)
	if task == nil {
		t.Fatal("task not found after await")
	}

	if reviewer {
		if state.Agents[boundedAwaitReviewerID].CurrentTask != nil {
			t.Error("reviewer current_task should be nil after await")
		}
		if task.ReviewingBy != nil {
			t.Error("task reviewing_by should be nil after await")
		}
		if task.ReviewLeaseExpires != nil {
			t.Error("task review_lease_expires should be nil after await")
		}
	} else if state.Agents[boundedAwaitCoderID].CurrentTask != nil {
		t.Error("doer current_task should be nil after await")
	}
}

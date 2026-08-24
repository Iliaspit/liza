package commands

import (
	"context"
	"time"

	"github.com/liza-mas/liza/internal/ops"
)

// AwaitResubmissionResult adds the remaining total wait budget to a POLL outcome.
type AwaitResubmissionResult struct {
	*ops.AwaitResubmissionResult
	TimeoutSeconds int `json:"timeout_seconds,omitempty"`
}

// AwaitResubmissionOptions configures the operation's periodic checks.
type AwaitResubmissionOptions = ops.AwaitResubmissionOptions

// AwaitResubmission applies the bounded foreground interval to one resubmission
// wait. budget is the total wait allowance; the remaining share is derived from
// the rejection recorded in task history. See AwaitVerdict.
func AwaitResubmission(projectRoot, taskID, agentID string, budget time.Duration) (*AwaitResubmissionResult, error) {
	return AwaitResubmissionWithOptions(projectRoot, taskID, agentID, budget, AwaitResubmissionOptions{})
}

// AwaitResubmissionWithOptions is AwaitResubmission with configurable polling intervals.
func AwaitResubmissionWithOptions(projectRoot, taskID, agentID string, budget time.Duration, opts AwaitResubmissionOptions) (*AwaitResubmissionResult, error) {
	remaining := ops.AwaitResubmissionRemainingBudget(projectRoot, taskID, agentID, budget)
	return awaitResubmissionWithIntervalAndOptions(projectRoot, taskID, agentID, remaining, maxAwaitInterval, opts)
}

func awaitResubmissionWithInterval(
	projectRoot, taskID, agentID string,
	remaining, maxInterval time.Duration,
) (*AwaitResubmissionResult, error) {
	return awaitResubmissionWithIntervalAndOptions(projectRoot, taskID, agentID, remaining, maxInterval, AwaitResubmissionOptions{})
}

func awaitResubmissionWithIntervalAndOptions(
	projectRoot, taskID, agentID string,
	remaining, maxInterval time.Duration,
	opts AwaitResubmissionOptions,
) (*AwaitResubmissionResult, error) {
	return awaitResubmissionWithBudget(remaining, maxInterval, func(interval time.Duration) (*ops.AwaitResubmissionResult, error) {
		return ops.AwaitResubmissionWithOptions(context.Background(), projectRoot, taskID, agentID, interval, opts)
	})
}

func awaitResubmissionWithBudget(
	remaining time.Duration,
	maxInterval time.Duration,
	await func(time.Duration) (*ops.AwaitResubmissionResult, error),
) (*AwaitResubmissionResult, error) {
	result, timeoutSeconds, err := awaitWithBudget(
		remaining,
		maxInterval,
		await,
		func(result *ops.AwaitResubmissionResult) bool { return result.Verdict == ops.ResubmissionTimeout },
		func(result *ops.AwaitResubmissionResult) { result.Verdict = ops.ResubmissionPoll },
	)
	if err != nil {
		return nil, err
	}
	if result.Verdict != ops.ResubmissionPoll {
		timeoutSeconds = 0
	}
	return &AwaitResubmissionResult{AwaitResubmissionResult: result, TimeoutSeconds: timeoutSeconds}, nil
}

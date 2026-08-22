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

// AwaitResubmission applies the bounded foreground interval to one resubmission
// wait. budget is the total wait allowance; the remaining share is derived from
// the rejection recorded in task history. See AwaitVerdict.
func AwaitResubmission(projectRoot, taskID, agentID string, budget time.Duration) (*AwaitResubmissionResult, error) {
	remaining := ops.AwaitResubmissionRemainingBudget(projectRoot, taskID, agentID, budget)
	return awaitResubmissionWithInterval(projectRoot, taskID, agentID, remaining, maxAwaitInterval)
}

func awaitResubmissionWithInterval(
	projectRoot, taskID, agentID string,
	remaining, maxInterval time.Duration,
) (*AwaitResubmissionResult, error) {
	return awaitResubmissionWithBudget(remaining, maxInterval, func(interval time.Duration) (*ops.AwaitResubmissionResult, error) {
		return ops.AwaitResubmission(context.Background(), projectRoot, taskID, agentID, interval)
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

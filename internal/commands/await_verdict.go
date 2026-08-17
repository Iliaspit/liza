package commands

import (
	"context"
	"time"

	"github.com/liza-mas/liza/internal/ops"
)

// AwaitVerdictResult adds the remaining total wait budget to a POLL outcome.
type AwaitVerdictResult struct {
	*ops.AwaitVerdictResult
	TimeoutSeconds int `json:"timeout_seconds,omitempty"`
}

// AwaitVerdict applies the bounded foreground interval to one verdict wait.
func AwaitVerdict(projectRoot, taskID, agentID string, remaining time.Duration) (*AwaitVerdictResult, error) {
	return awaitVerdictWithInterval(projectRoot, taskID, agentID, remaining, maxAwaitInterval)
}

func awaitVerdictWithInterval(
	projectRoot, taskID, agentID string,
	remaining, maxInterval time.Duration,
) (*AwaitVerdictResult, error) {
	return awaitVerdictWithBudget(remaining, maxInterval, func(interval time.Duration) (*ops.AwaitVerdictResult, error) {
		return ops.AwaitVerdict(context.Background(), projectRoot, taskID, agentID, interval)
	})
}

func awaitVerdictWithBudget(
	remaining time.Duration,
	maxInterval time.Duration,
	await func(time.Duration) (*ops.AwaitVerdictResult, error),
) (*AwaitVerdictResult, error) {
	result, timeoutSeconds, err := awaitWithBudget(
		remaining,
		maxInterval,
		await,
		func(result *ops.AwaitVerdictResult) bool { return result.Verdict == ops.VerdictTimeout },
		func(result *ops.AwaitVerdictResult) { result.Verdict = ops.VerdictPoll },
	)
	if err != nil {
		return nil, err
	}
	if result.Verdict != ops.VerdictPoll {
		timeoutSeconds = 0
	}
	return &AwaitVerdictResult{AwaitVerdictResult: result, TimeoutSeconds: timeoutSeconds}, nil
}

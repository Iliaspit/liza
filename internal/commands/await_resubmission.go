package commands

import (
	"context"
	"fmt"
	"time"

	"github.com/liza-mas/liza/internal/ops"
)

// AwaitResubmissionCommand blocks until a doer resubmits after a rejection.
// Delegates business logic to ops.AwaitResubmission.
func AwaitResubmissionCommand(projectRoot, taskID, agentID string, timeout time.Duration) error {
	result, err := ops.AwaitResubmission(context.Background(), projectRoot, taskID, agentID, timeout)
	if err != nil {
		return fmt.Errorf("await resubmission: %w", err)
	}

	fmt.Printf("Verdict: %s\nStatus: %s\n", result.Verdict, result.TaskStatus)
	if result.ReviewCommit != "" {
		if result.BaseCommit != "" {
			fmt.Printf("Base commit: %s\n", result.BaseCommit)
		}
		fmt.Printf("Review commit: %s\nReview cycle: %d\n", result.ReviewCommit, result.ReviewCycle)
	}
	if result.Reason != "" {
		fmt.Printf("Reason: %s\n", result.Reason)
	}
	return nil
}

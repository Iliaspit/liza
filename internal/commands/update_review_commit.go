package commands

import (
	"fmt"

	"github.com/liza-mas/liza/internal/ops"
)

// UpdateReviewCommitCommand updates the review boundary and prints the result to stdout.
// Delegates business logic to ops.UpdateReviewCommit.
func UpdateReviewCommitCommand(projectRoot, taskID, changedBy string) error {
	result, err := ops.UpdateReviewCommit(projectRoot, taskID, changedBy)
	if err != nil {
		return fmt.Errorf("update review commit: %w", err)
	}

	fmt.Printf("Updated review boundary for %s\n", result.TaskID)
	oldReviewCommit := "<nil>"
	if result.OldReviewCommit != nil {
		oldReviewCommit = *result.OldReviewCommit
	}
	fmt.Printf("  review_commit old: %s\n", oldReviewCommit)
	fmt.Printf("  review_commit new: %s\n", result.NewReviewCommit)
	oldBase := "<nil>"
	if result.OldBaseCommit != nil {
		oldBase = *result.OldBaseCommit
	}
	fmt.Printf("  base_commit old: %s\n", oldBase)
	fmt.Printf("  base_commit new: %s\n", result.NewBaseCommit)
	if result.ReviewerReleased {
		fmt.Println("  reviewer claim released (task returned to submitted state)")
	}
	return nil
}

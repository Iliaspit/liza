package ops

import "github.com/liza-mas/liza/internal/models"

func hasHandoffTriggerForTest(events []models.HandoffEvent, trigger models.HandoffTrigger) bool {
	for _, event := range events {
		if event.Trigger == trigger {
			return true
		}
	}
	return false
}

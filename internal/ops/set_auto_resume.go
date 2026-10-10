package ops

import (
	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/paths"
)

// AutoResumeConfigResult is the operator-visible config change for an audit record.
type AutoResumeConfigResult struct {
	Previous  bool   `json:"previous"`
	Enabled   bool   `json:"enabled"`
	ChangedBy string `json:"changed_by"`
}

// SetAutoResume sets the auto_resume config flag. Idempotent.
func SetAutoResume(projectRoot string, enabled bool, changedBy string) (*AutoResumeConfigResult, error) {
	if changedBy == "" {
		return nil, &PreconditionError{Reason: "auto-resume change requires an operator identity"}
	}
	lp := paths.New(projectRoot)
	bb := db.For(lp.StatePath())
	result := &AutoResumeConfigResult{Enabled: enabled, ChangedBy: changedBy}
	err := bb.Modify(func(s *models.State) error {
		result.Previous = s.Config.AutoResume
		s.Config.AutoResume = enabled
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

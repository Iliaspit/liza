package ops

import (
	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/paths"
)

// SetAutoResume sets the auto_resume config flag. Idempotent.
func SetAutoResume(projectRoot string, enabled bool) error {
	lp := paths.New(projectRoot)
	bb := db.For(lp.StatePath())
	return bb.ModifyOp("set_auto_resume", func(s *models.State) error {
		s.Config.AutoResume = enabled
		return nil
	})
}

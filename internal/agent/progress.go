package agent

import (
	"time"

	"github.com/liza-mas/liza/internal/models"
)

// progressClass classifies why the supervisor is counting consecutive
// non-progress turns for a subject. Every loop-detection concern (exit-42
// restarts, crashes, spinning, successful no-progress turns, observed
// runtime failures) is a classification feeding the same counting
// mechanism: increment while the progress signature is unchanged, reset on
// progress, block when the class's threshold is exceeded.
type progressClass string

const (
	classExit42            progressClass = "exit42"
	classCrash             progressClass = "crash"
	classSpin              progressClass = "spin"
	classSuccessNoProgress progressClass = "success_no_progress"
	classRuntimeFailure    progressClass = "runtime_failure"
)

// thresholdFor returns the configured consecutive-non-progress threshold for
// a class. This is the single policy table for supervisor loop detection.
func thresholdFor(class progressClass, cfg models.Config) int {
	switch class {
	case classExit42:
		return effectiveExit42RestartLimit(cfg)
	case classCrash:
		return effectiveCrashRestartThreshold(cfg)
	default:
		// Spinning, successful no-progress, and runtime-failure loops share
		// the spinning threshold.
		return effectiveSpinningRestartThreshold(cfg)
	}
}

type progressKey struct {
	subject string
	class   progressClass
}

type progressState struct {
	count     int
	signature string
}

// progressLedger is the supervisor's single non-progress counting mechanism.
// A subject is whatever the class observes: a task ID, "orchestrator", or an
// exit-42 tracker key. Counters for different classes on the same subject are
// independent — their interplay (e.g. a runtime failure suppressing the spin
// counter) is policy expressed at the call sites, not in the ledger.
//
// Not safe for concurrent use; the supervisor loop is single-goroutine.
type progressLedger struct {
	byKey map[progressKey]progressState
}

func newProgressLedger() *progressLedger {
	return &progressLedger{byKey: make(map[progressKey]progressState)}
}

// Observe records one non-progress observation and returns the consecutive
// count. A signature change means progress: the counter restarts at 1. Empty
// signatures never reset (unknown state is not evidence of progress).
func (l *progressLedger) Observe(subject string, class progressClass, signature string) int {
	key := progressKey{subject: subject, class: class}
	prev := l.byKey[key]
	if prev.signature != "" && signature != "" && prev.signature != signature {
		prev.count = 0
	}
	prev.count++
	prev.signature = signature
	l.byKey[key] = prev
	return prev.count
}

// Reset clears the given classes for a subject (all classes when none given).
func (l *progressLedger) Reset(subject string, classes ...progressClass) {
	if subject == "" {
		return
	}
	if len(classes) == 0 {
		classes = []progressClass{
			classExit42, classCrash, classSpin, classSuccessNoProgress, classRuntimeFailure,
		}
	}
	for _, class := range classes {
		delete(l.byKey, progressKey{subject: subject, class: class})
	}
}

// exit42Backoff returns the exponential restart delay for a consecutive
// exit-42 count, capped at the configured maximum.
func exit42Backoff(restartCount int, cfg models.Config) time.Duration {
	return computeExit42BackoffDelay(restartCount, effectiveExit42MaxBackoff(cfg))
}

package agent

import (
	"io"
	"log/slog"
	"os"
)

const (
	SupervisorStdoutLogFlag        = "supervisor-stdout-log"
	SupervisorStderrLogFlag        = "supervisor-stderr-log"
	SupervisorReadyFileFlag        = "supervisor-ready-file"
	SupervisorBootstrapReadyStatus = "ready\n"
	SupervisorBootstrapErrorPrefix = "error: "
)

var logger *slog.Logger

func init() {
	logger = newLogger(os.Stdout)
}

func newLogger(output io.Writer) *slog.Logger {
	opts := &slog.HandlerOptions{
		Level: slog.LevelInfo,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// Match state.yaml timestamp format (ISO 8601 UTC)
			if a.Key == slog.TimeKey {
				a.Value = slog.StringValue(a.Value.Time().UTC().Format("2006-01-02T15:04:05Z"))
			}
			return a
		},
	}

	return slog.New(slog.NewTextHandler(output, opts))
}

func GetLogger() *slog.Logger {
	return logger
}

// UseLoggerOutput redirects package lifecycle logs until the returned restore
// function is called. Supervisors configure this before starting their worker
// goroutines and restore it only after RunSupervisor has returned.
func UseLoggerOutput(output io.Writer) (restore func()) {
	previous := logger
	logger = newLogger(output)
	return func() {
		logger = previous
	}
}

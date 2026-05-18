package alerts

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// AlertLevel is the severity of a watch or lifecycle alert.
type AlertLevel string

const (
	AlertLevelWarning  AlertLevel = "⚠️"
	AlertLevelCritical AlertLevel = "🚨"
)

// Alert represents a single anomaly or lifecycle alert.
type Alert struct {
	Timestamp time.Time
	Level     AlertLevel
	Category  string
	Message   string
}

func (a Alert) String() string {
	return fmt.Sprintf("[%s] %s %s: %s",
		a.Timestamp.UTC().Format(time.RFC3339),
		a.Level,
		a.Category,
		a.Message)
}

// ParseLine parses a line written by Alert.String() back into an Alert.
func ParseLine(line string) (Alert, bool) {
	if !strings.HasPrefix(line, "[") {
		return Alert{}, false
	}
	closeBracket := strings.IndexByte(line, ']')
	if closeBracket < 0 {
		return Alert{}, false
	}
	ts, err := time.Parse(time.RFC3339, line[1:closeBracket])
	if err != nil {
		return Alert{}, false
	}

	rest := strings.TrimLeft(line[closeBracket+1:], " ")
	colonIdx := strings.Index(rest, ": ")
	if colonIdx < 0 {
		return Alert{}, false
	}
	levelAndCategory := rest[:colonIdx]
	message := rest[colonIdx+2:]

	spaceIdx := strings.IndexByte(levelAndCategory, ' ')
	if spaceIdx < 0 {
		return Alert{}, false
	}
	level := AlertLevel(levelAndCategory[:spaceIdx])
	category := strings.TrimSpace(levelAndCategory[spaceIdx+1:])

	return Alert{
		Timestamp: ts,
		Level:     level,
		Category:  category,
		Message:   message,
	}, true
}

// Key returns a stable identity for an alert condition.
func Key(alert Alert) string {
	return alert.Category + ":" + alert.Message
}

// Write appends an alert to the alerts log file.
func Write(alertsLog string, alert Alert) error {
	f, err := os.OpenFile(alertsLog, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open alerts log: %w", err)
	}
	defer f.Close()

	if _, err := fmt.Fprintln(f, alert.String()); err != nil {
		return fmt.Errorf("failed to write alert: %w", err)
	}
	return nil
}

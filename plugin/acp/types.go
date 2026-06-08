package acp

import "time"

// EventKind is the event type used by ACP observability payloads.
type EventKind string

const (
	EventStarted     EventKind = "started"
	EventMessage     EventKind = "agent_message_chunk"
	EventThought     EventKind = "agent_thought_chunk"
	EventTrajectory  EventKind = "trajectory"
	EventToolCall    EventKind = "tool_call"
	EventToolUpdate  EventKind = "tool_call_update"
	EventOutputChunk EventKind = "output_chunk"
	EventUsage       EventKind = "usage"
	EventCompleted   EventKind = "completed"
)

// Usage tracks token accounting for one ACP run.
type Usage struct {
	InputTokens       int
	OutputTokens      int
	CachedReadTokens  int
	CachedWriteTokens int
}

func (u Usage) add(v Usage) Usage {
	u.InputTokens += v.InputTokens
	u.OutputTokens += v.OutputTokens
	u.CachedReadTokens += v.CachedReadTokens
	u.CachedWriteTokens += v.CachedWriteTokens
	return u
}

// Event is a provider-neutral event shape for plugin-level ACP testing.
type Event struct {
	Time      time.Time
	Kind      EventKind
	TaskID    string
	SessionID string
	Message   string
	Payload   map[string]any
}

// RunMetric captures a single ACP run outcome.
type RunMetric struct {
	TaskID    string
	SessionID string
	Warm      bool
	ExitCode  int
	Duration  time.Duration
	Output    string
	Usage     Usage
}

// BenchmarkSummary is a compact report for warm-vs-fresh comparisons.
type BenchmarkSummary struct {
	TotalRuns                int
	WarmRuns                 int
	FreshRuns                int
	WarmAverageDurationMs    float64
	FreshAverageDurationMs   float64
	SpeedupPercent           float64
	WarmAverageInputTokens   float64
	FreshAverageInputTokens  float64
	InputTokenSavingsPercent float64
}

package acp

import (
	"errors"
	"sync"
	"time"
)

// BenchmarkAccumulator tracks ACP run metrics over time.
type BenchmarkAccumulator struct {
	mu   sync.Mutex
	runs []RunMetric
}

func NewBenchmarkAccumulator() *BenchmarkAccumulator {
	return &BenchmarkAccumulator{}
}

// Record appends a benchmark sample.
func (b *BenchmarkAccumulator) Record(metric RunMetric) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.runs = append(b.runs, metric)
}

func (b *BenchmarkAccumulator) Runs() []RunMetric {
	b.mu.Lock()
	defer b.mu.Unlock()

	out := make([]RunMetric, len(b.runs))
	copy(out, b.runs)
	return out
}

// Summarize returns aggregated warm-vs-fresh metrics.
func (b *BenchmarkAccumulator) Summarize() (BenchmarkSummary, error) {
	runs := b.Runs()
	if len(runs) == 0 {
		return BenchmarkSummary{}, errors.New("no runs recorded")
	}

	var (
		warmCount, freshCount int
		warmDuration          time.Duration
		freshDuration         time.Duration
		warmInput             int
		freshInput            int
	)

	for _, run := range runs {
		if run.Warm {
			warmCount++
			warmDuration += run.Duration
			warmInput += run.Usage.InputTokens
			continue
		}
		freshCount++
		freshDuration += run.Duration
		freshInput += run.Usage.InputTokens
	}
	if warmCount == 0 || freshCount == 0 {
		return BenchmarkSummary{TotalRuns: len(runs), WarmRuns: warmCount, FreshRuns: freshCount}, nil
	}

	sum := BenchmarkSummary{
		TotalRuns:               len(runs),
		WarmRuns:                warmCount,
		FreshRuns:               freshCount,
		WarmAverageDurationMs:   durationMillis(warmDuration) / float64(warmCount),
		FreshAverageDurationMs:  durationMillis(freshDuration) / float64(freshCount),
		WarmAverageInputTokens:  float64(warmInput) / float64(warmCount),
		FreshAverageInputTokens: float64(freshInput) / float64(freshCount),
	}
	if sum.FreshAverageDurationMs > 0 {
		sum.SpeedupPercent = 100 - (sum.WarmAverageDurationMs / sum.FreshAverageDurationMs * 100)
	}
	if sum.FreshAverageInputTokens > 0 {
		sum.InputTokenSavingsPercent = 100 - (sum.WarmAverageInputTokens / sum.FreshAverageInputTokens * 100)
	}
	return sum, nil
}

func durationMillis(duration time.Duration) float64 {
	return float64(duration) / float64(time.Millisecond)
}

// ParseUsageFromEvent extracts token accounting from a usage event.
func ParseUsageFromEvent(event Event) (Usage, bool) {
	if event.Kind != EventUsage {
		return Usage{}, false
	}
	if event.Payload == nil {
		return Usage{}, false
	}

	usageRaw, ok := event.Payload["usage"]
	if !ok {
		return Usage{}, false
	}

	usageMap, ok := usageRaw.(map[string]any)
	if !ok {
		return Usage{}, false
	}

	var u Usage
	if v, ok := intFromAny(usageMap["input_tokens"]); ok {
		u.InputTokens = v
	}
	if v, ok := intFromAny(usageMap["output_tokens"]); ok {
		u.OutputTokens = v
	}
	if v, ok := intFromAny(usageMap["cached_read_tokens"]); ok {
		u.CachedReadTokens = v
	}
	if v, ok := intFromAny(usageMap["cached_write_tokens"]); ok {
		u.CachedWriteTokens = v
	}
	return u, true
}

func intFromAny(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		return int(n), true
	case float32:
		return int(n), true
	default:
		return 0, false
	}
}

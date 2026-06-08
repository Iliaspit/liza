package acp

import (
	"sync"
	"testing"
	"time"
)

func TestSessionManagerReusesSessionPerTask(t *testing.T) {
	ml := NewSessionManager()
	sid1, warm1 := ml.Start("task-1")
	if warm1 {
		t.Fatalf("first run should not be warm")
	}
	if sid1 == "" {
		t.Fatalf("expected session id, got empty")
	}

	sid2, warm2 := ml.Start("task-1")
	if !warm2 {
		t.Fatalf("second run should be warm")
	}
	if sid2 != sid1 {
		t.Fatalf("expected warm session %q, got %q", sid1, sid2)
	}

	sid3, warm3 := ml.Start("task-2")
	if warm3 || sid3 == "" {
		t.Fatalf("new task should create cold new session")
	}
}

func TestSessionManagerEndForcesFresh(t *testing.T) {
	ml := NewSessionManager()
	first, _ := ml.Start("task-1")
	ml.End("task-1")
	second, warm := ml.Start("task-1")
	if warm {
		t.Fatalf("ended session should be cold on next start")
	}
	if first == second {
		t.Fatalf("ended session should get new session id")
	}
}

func TestSessionManagerListIsDeterministic(t *testing.T) {
	ml := NewSessionManager()
	ml.Start("task-c")
	ml.Start("task-a")
	ml.Start("task-b")

	snapshots := ml.List()
	got := []string{snapshots[0].TaskID, snapshots[1].TaskID, snapshots[2].TaskID}
	want := []string{"task-a", "task-b", "task-c"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("List task order = %v, want %v", got, want)
		}
	}
}

func TestBenchmarkSummarizeComputesSpeedupAndSavings(t *testing.T) {
	b := NewBenchmarkAccumulator()
	b.Record(RunMetric{TaskID: "task-1", Warm: false, Duration: 2200 * time.Millisecond, Usage: Usage{InputTokens: 220}})
	b.Record(RunMetric{TaskID: "task-2", Warm: false, Duration: 2000 * time.Millisecond, Usage: Usage{InputTokens: 260}})
	b.Record(RunMetric{TaskID: "task-1", Warm: true, Duration: 1250 * time.Millisecond, Usage: Usage{InputTokens: 5}})
	b.Record(RunMetric{TaskID: "task-2", Warm: true, Duration: 1320 * time.Millisecond, Usage: Usage{InputTokens: 4}})

	sum, err := b.Summarize()
	if err != nil {
		t.Fatalf("summarize: %v", err)
	}
	if sum.FreshRuns != 2 || sum.WarmRuns != 2 {
		t.Fatalf("unexpected run counts: %#v", sum)
	}
	if sum.SpeedupPercent <= 35 {
		t.Fatalf("expected meaningful speedup, got %f", sum.SpeedupPercent)
	}
	if sum.InputTokenSavingsPercent <= 97 {
		t.Fatalf("expected strong token savings, got %f", sum.InputTokenSavingsPercent)
	}
}

func TestBenchmarkSummarizePreservesSubMillisecondDurations(t *testing.T) {
	b := NewBenchmarkAccumulator()
	b.Record(RunMetric{TaskID: "task-1", Warm: false, Duration: 500 * time.Microsecond, Usage: Usage{InputTokens: 100}})
	b.Record(RunMetric{TaskID: "task-1", Warm: true, Duration: 250 * time.Microsecond, Usage: Usage{InputTokens: 50}})

	sum, err := b.Summarize()
	if err != nil {
		t.Fatalf("summarize: %v", err)
	}
	if sum.FreshAverageDurationMs != 0.5 {
		t.Fatalf("FreshAverageDurationMs = %f, want 0.5", sum.FreshAverageDurationMs)
	}
	if sum.WarmAverageDurationMs != 0.25 {
		t.Fatalf("WarmAverageDurationMs = %f, want 0.25", sum.WarmAverageDurationMs)
	}
	if sum.SpeedupPercent != 50 {
		t.Fatalf("SpeedupPercent = %f, want 50", sum.SpeedupPercent)
	}
}

func TestBenchmarkAccumulatorConcurrentRecord(t *testing.T) {
	b := NewBenchmarkAccumulator()

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			b.Record(RunMetric{
				TaskID:   "task",
				Warm:     i%2 == 0,
				Duration: time.Duration(i+1) * time.Millisecond,
				Usage:    Usage{InputTokens: i + 1},
			})
		}(i)
	}
	wg.Wait()

	if got := len(b.Runs()); got != 20 {
		t.Fatalf("recorded runs = %d, want 20", got)
	}
	if _, err := b.Summarize(); err != nil {
		t.Fatalf("summarize: %v", err)
	}
}

func TestParseUsageFromEvent(t *testing.T) {
	event := Event{
		Kind:      EventUsage,
		TaskID:    "task-1",
		SessionID: "sess-1",
		Payload: map[string]any{
			"usage": map[string]any{
				"input_tokens":        13,
				"output_tokens":       7,
				"cached_read_tokens":  3,
				"cached_write_tokens": 1,
			},
		},
	}
	u, ok := ParseUsageFromEvent(event)
	if !ok {
		t.Fatalf("expected parse success")
	}
	if u.InputTokens != 13 || u.OutputTokens != 7 || u.CachedReadTokens != 3 || u.CachedWriteTokens != 1 {
		t.Fatalf("wrong usage parsed: %#v", u)
	}
}

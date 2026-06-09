# ACP Plugin

`plugin/acp` is an independent Go package for tracking ACP-style task runs and
benchmarking warm vs. fresh executions.

The event kind strings intentionally mirror `internal/agent` without importing
it. This package is kept independent so benchmarks and downstream ACP tooling do
not depend on Liza's internal supervisor packages.

It has three small APIs:

- `SessionManager` tracks whether a task can reuse a session (`warm`) and when
  warmness should be reset (`End`).
- `RunManager` turns event streams into executable run metrics (`Start`,
  `HandleEvent`, `Finish`) and can produce aggregate stats.
- `BenchmarkAccumulator` computes warm/fresh speedup and input-token savings from
  recorded metrics. Its public methods are safe for concurrent callers.

## Quick example

```go
mgr := NewRunManager()
run := mgr.Start("task-123")
_ = mgr.HandleEvent(Event{
	Kind:    EventUsage,
	TaskID:  run.TaskID,
	Payload: map[string]any{"usage": map[string]any{"input_tokens": 240}},
})
metric, _ := mgr.Finish("task-123", 0)

summarize, _ := mgr.Summary()
_ = metric
_ = summarize
```

Tests are intentionally isolated to keep behavior stable as the surrounding runtime
evolves.

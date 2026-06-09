# Real ACP Benchmark

Date: 2026-06-07

Command:

```bash
LIZA_RUN_REAL_ACP_BENCH=1 go test ./internal/agent -run TestRealACPXBenchmarkSimplifiedLizaProject -count=1 -v
```

Setup:

- Real Liza `RunSupervisor` loop.
- Temporary simplified git project with two ready coding tasks.
- Fresh baseline: `acpx codex exec` creates a fresh ACP session per task and receives the full Liza prompt each time.
- Warm ACP path: `acpx codex prompt -s liza-real-acp-benchmark` reuses one persistent ACP session; first task bootstraps with the full Liza prompt, second task sends only the task delta.
- Real adapter: `@agentclientprotocol/codex-acp`.
- Metrics source: ACP JSON-RPC `result.usage` from `acpx --format json`.

Latest result:

```text
real fresh baseline: runs=2 duration=42.076793s input_tokens=33868 fresh_input_tokens=33868 cached_read_tokens=26368 output_tokens=86
real warm ACP:       runs=2 duration=29.646552374s input_tokens=27800 fresh_input_tokens=27800 cached_read_tokens=32512 output_tokens=54 warm_runs=1
real difference:     duration_delta=29.54% input_delta=17.92% fresh_input_delta=17.92%
real warm task delta: task=task-alpha baseline_input=27686 warm_acp_input=114 savings=99.59%
```

Interpretation:

- End-to-end two-task run improved by 29.54% wall time.
- Total fresh input across the two-task run dropped 17.92% because the first ACP turn still pays the bootstrap cost.
- The unique warm task delta dropped fresh input by 99.59%, which is the ACP behavior this experiment is meant to isolate.

Caveat:

- The wall-time percentage is not universal. It changes with model latency,
  terminal/tool round trips, and the number of turns that reuse the warm ACP
  session. In this benchmark it represents a two-task/two-round-trip simplified
  Liza run.
- The more stable metric is fresh input avoided on the reused-context task:
  27,572 fewer fresh input tokens for the warm task delta.

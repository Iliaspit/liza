# CLIAgent vs ACPXAgent Feature Comparison

Reference for understanding feature parity between the two `LLMAgent` backends.
Based on the `llm-agent-boundary` branch (PR #83, ADR-0085).

## Feature Matrix

| Capability | CLIAgent | ACPXAgent | Notes |
|---|---|---|---|
| **Backend dispatch** | claude, codex, gemini, mistral, kimi | codex (via acpx) | CLI resolves per-provider flags; ACP is single-backend |
| **Agent output logs** | Streams to `.liza/agent-outputs/*.txt`, `*.err` | `outputsDir` accepted but unused | **Gap** — ACP runs leave no audit trail |
| **Real-time terminal output** | Tees to `os.Stdout`/`os.Stderr` while running | Buffers until subprocess exit | Operator sees nothing during ACP execution |
| **Progress watchdog** | Feeds `progressCh` on every stdout/stderr write | No provider-output signal | Watchdog still polls worktree + state; ACP is less granular but not blind |
| **Interactive mode** | Full support | Returns error | Expected — documented |
| **Secret masking** | Creates masker only when `outputsDir != ""` | Always creates masker | ACP is stricter — masks even in no-log mode |
| **Provider env file** | Loads `claude.env`, feeds entries to masker | No env file loading | Low impact — ACP doesn't use Claude's env file |
| **Codex version pinning** | `LIZA_CODEX_VERSION` / `config.codex_package_version` | Not supported | Low impact — version pinning is a CLI concern |
| **Claude subagent disabling** | `LIZA_DISABLE_CLAUDE_SUBAGENTS` → `--disallowedTools Task` | N/A | Provider-specific, not applicable |
| **Token usage reporting** | None — returns empty `LLMAgentUsage` on every path | Parses real `inputTokens`/`outputTokens`/`cachedReadTokens` from acpx JSON-RPC | ACP advantage |
| **Warm session management** | None | Local `seen` map + `acpx sessions show` for cross-restart detection | ACP advantage |
| **Event semantics** | `output_chunk` (raw bytes from stdout/stderr) | `agent_message_chunk` (parsed ACP messages) + `usage` | Different by design |
| **Trajectory/tool/thought events** | Not available from CLI stdout | Not parsed yet, though event enum supports them | Future ACP opportunity |

## Production Event Sink

Both backends emit events to `supervisorLLMAgentEventSink` (`systemctl.go`), which:

- Logs `started` and `completed` to the Liza structured logger
- Logs `usage` with token breakdowns (meaningful for ACP; zero for CLI)
- Drops content events (`output_chunk`, `agent_message_chunk`) — "avoids duplicating content events" since CLI output already streams to agent-output log files

This means ACP's richer event types (trajectory, tool calls, thoughts) would currently hit the `default` no-op case even if parsed. The event infrastructure is wired but the sink is minimal.

## Operational Gaps

### Agent output logs (high)

`ACPXAgent.outputsDir` is stored in the struct but never referenced. ACP runs produce
no `.txt`/`.err` files, so `/context-engineering` analysis and `/liza-logs` have nothing
to inspect. The docs (`CONFIGURATION.md`, `USAGE_MULTI_AGENTS.md`) list `codex-acp`
alongside backends that do write logs, without caveating the gap.

### Terminal silence during execution (medium)

`runACPX` captures stdout/stderr to `bytes.Buffer`. The operator running `liza agent`
or watching the TUI gets no output until the entire `acpx prompt` call returns. For
multi-minute tasks this is a black box.

### Progress watchdog granularity (low)

The execution progress watchdog (`progress_watchdog.go`) uses three signals:
1. Provider output bytes (via `progressWriter` → `progressCh`) — **CLI only**
2. Worktree git state changes (polled on ticker)
3. Blackboard task state changes (polled on ticker)

ACPXAgent doesn't feed signal 1, but signals 2 and 3 still work. A stalled ACP run
where the agent makes no worktree or state changes will still be caught — just with
less granularity (polling interval vs. real-time byte stream).

## Dead Code

- `ACPXAgent.outputsDir` — field exists, never read
- `CLIAgent` always returns `LLMAgentUsage{}` — the `emitUsageEvent` calls were removed but the zero-value return remains (correct: no data available)

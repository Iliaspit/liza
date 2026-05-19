# Cluster 0060 - Agent Execution Progress Watchdog

## Commit Set
- `2bb71dad` - block stalled executions safely

## Source Issues
- GitHub issue #67 - Codex architect agent can stall after bootstrap while Liza reports stale process state
- `liza-run-issues.md` sections 5-6, 8-9

## Intent Hypothesis
Add supervisor-side execution-progress detection so task-bound agents that stop producing observable progress are blocked and cleaned up instead of remaining indefinitely assigned.

## Architectural Signals
- New `internal/agent/progress_watchdog.go`
- Supervisor checks task state changes, worktree HEAD/status changes, untracked files, and provider stdout/stderr as progress
- Stale execution cancels the provider process, transitions the owned task to `BLOCKED`, and performs worktree cleanup
- Status output gains process status source/detail fields
- Config/support docs add watchdog controls and diagnostics

## User Context Captured
- Trigger: Codex-backed architect/code-planner agents could bootstrap, claim work, and then stall with no artifacts while Liza still showed contradictory process state.
- Rationale: heartbeats/process liveness alone do not prove task progress; the supervisor needs observable progress signals and safe recovery.
- Tradeoffs: progress heuristics may block long quiet work; provider output/worktree changes are approximations of useful progress.

## Candidate Decision Date
2026-05-16

## Status
ADR generated: `specs/architecture/ADR/0060-agent-execution-progress-watchdog.md`

## Confidence
0.90 (high)

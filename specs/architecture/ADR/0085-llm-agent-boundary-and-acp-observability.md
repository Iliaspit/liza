# ADR-0085: LLMAgent Boundary and ACP Observability

## Status

Accepted

## Context

Liza currently coordinates agents through supervisor-assigned tasks, file-backed
state, worktrees, and deterministic task transitions. Agent providers are
launched through CLIs such as Claude, Codex, Gemini, Kimi, or Vibe. This keeps
the orchestration provider-agnostic and avoids making agents negotiate with
each other.

ACP/A2A-style agent protocols were evaluated for two different purposes:

- agent-to-agent coordination
- provider-neutral observability of agent reasoning, tool calls, and streaming
  trajectory events

The coordination use case does not match Liza. Liza intentionally avoids
peer-to-peer agent negotiation; the supervisor and blackboard are the source of
truth. The observability use case is different: ACP trajectory metadata can
expose real-time reasoning deltas, internal tool calls, and structured spans
that are hard to recover reliably by tailing provider-specific CLI logs.

## Decision

Introduce `LLMAgent` as the semantic execution boundary and `CLIAgent` as the
default OSS implementation. `CLIAgent` preserves the current CLI subprocess
behavior. Liza can also provide explicit opt-in ACP-backed implementations,
such as `ACPXAgent`, without changing supervisor execution semantics.

The boundary includes `LLMAgentEventSink`, a provider-neutral event stream for
agent observability. `CLIAgent` emits process lifecycle and stdout/stderr chunk
events. An ACP adapter can emit richer trajectory, delta, and tool-call events
through the same sink.

For migration to the existing ACP implementation, the boundary now includes:

- session hints (`TaskID`, `SessionID`, `ResumeSession`, `WarmSession`) in
  `LLMAgentRunRequest`
- session result metadata (`SessionID`, `WarmUsage`) in `LLMAgentRunResult`
- usage accounting (`InputTokens`, `OutputTokens`, `CachedReadTokens`,
  `CachedWriteTokens`) in `LLMAgentUsage`
- ACP-style event kinds (`agent_message_chunk`, `agent_thought_chunk`,
  `tool_call_update`, `usage`) in `LLMAgentEventKind`

The PR keeps ACP out of the default runtime path and does not change Liza's
coordination model. ACP-backed execution is explicit opt-in provider plumbing,
not a replacement for the supervisor or blackboard.

## Consequences

- OSS CLI support remains the default and remains provider-agnostic.
- Legacy `CLIExecutor` names are retained as compatibility wrappers.
- Liza gains an explicit place for ACP trajectory events without binding OSS to
  ACP in the default execution path.
- ACP is framed as an observability and warm-session execution adapter
  opportunity, not a replacement for Liza's blackboard, worktree, or supervisor
  model.
- Real-time dashboards or OTLP export can be added later by consuming
  `LLMAgentEventSink` events.

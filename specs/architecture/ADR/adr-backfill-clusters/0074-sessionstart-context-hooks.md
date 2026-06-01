# Cluster 0074 - SessionStart Context Hooks

## Commit Set
- `09e3cb62` - feat(hooks): add startup index context
- `6ed315eb` - feat(hooks): provide startup session context

## Gap Commits
- `c306d969` - docs(tools): remove filesystem mcp routing
- `c13a060a` - test(embedded): align stacklit session context expectation

## Intent Hypothesis
Provide initialization guidance and explicit repo index paths at session startup instead of relying on reactive hook failures or manual discovery.

## Architectural Signals
- New embedded `session-context.sh` hook
- Claude and Codex SessionStart configuration
- Mode-specific initialization guidance emitted before first user response
- Missing project files filtered out
- Pairing sessions receive repo-root Stacklit/SCIP paths; MAS agents keep prompt-supplied worktree indexes

## Reconstructed Context
- Trigger: agents needed initialization files and repo-root index paths before the first substantive response or tool call.
- Alternatives: rely on AGENTS.md alone, PreToolUse failures, or hand-written prompt preludes.
- Rationale: SessionStart hooks can provide proactive, mode-specific startup guidance and explicit index paths without waiting for a failed action.
- Tradeoffs: hook behavior is provider-specific and adds startup context that must stay concise.
- Related decisions: extends canonical contract deployment and optional Stacklit/SCIP indexing.

## Candidate Decision Date
2026-05-29

## Status
ADR generated: `specs/architecture/ADR/0074-sessionstart-context-hooks.md`

## Confidence
0.85 (high)

# 74 - SessionStart Context Hooks

## Status

ACCEPTED

## Context

Liza session initialization must happen before any substantive response or non-initialization tool use. Relying on static repo files or reactive PreToolUse failures was too late: agents need to know which initialization files to read and which repo indexes are available before the first real action.

Pairing sessions also need repo-root Stacklit and SCIP index paths when enabled without guessing global locations. Multi-agent worktrees must keep using prompt-supplied worktree indexes. The system needed a provider-level startup surface that could distinguish those cases.

## Decision

Install provider SessionStart hooks that emit Liza startup context.

The shared `session-context.sh` hook:
- emits mode-specific initialization guidance
- filters missing project files
- includes Pairing repo-root Stacklit and SCIP index paths when available
- keeps MAS worktree agents on prompt-supplied indexes
- provides bounded Stacklit summaries where appropriate

Claude and Codex both receive embedded SessionStart hook configuration. The previous index-only startup hook was renamed and expanded into session context.

## Consequences

Positive:
- Agents receive initialization requirements before their first substantive response.
- Pairing agents get explicit index paths and do not infer locations.
- MAS agents preserve worktree-specific index discipline.
- Missing optional docs are filtered out instead of causing startup noise.
- Hook output aligns Claude and Codex startup behavior.

Trade-offs:
- Startup behavior is provider-specific.
- Hook output adds context that must stay concise.
- Embedded hook deployment and tests must stay synchronized across providers.
- Index summaries can become stale until refreshed after commits.

## Alternatives Considered

1. Keep guidance only in AGENTS.md / CORE.md.

Rejected because the agent still needs session-specific paths and missing-file filtering.

2. Use PreToolUse enforcement only.

Rejected because failures occur after the agent has already attempted an action.

3. Require manual prompt preludes.

Rejected because session initialization should be reliable and deployed with `liza init`.

## Relationship to Prior Decisions

Extends ADR-0009 (Canonical Contract Root), ADR-0018 (Two-Step Deployment), and ADR-0068 (Optional Repository Indexing with SCIP and Stacklit). It operationalizes initialization and index routing at provider session start.

---
*Reconstructed from commits 09e3cb62..6ed315eb (2026-05-29 to 2026-05-30)*

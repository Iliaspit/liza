# 81 - Indexing Activation for Setup and Init

## Status

ACCEPTED

## Context

ADR-0068 introduced optional SCIP and Stacklit repository indexes, and ADR-0079 added Semble semantic discovery. ADR-0074 made provider SessionStart hooks a startup surface for Pairing context. After those decisions, activation still had an ownership problem.

Some optional-index guidance belonged in global agent-tool routing because it is generic and reusable. Other behavior was repository-specific: hook installation, concrete SCIP plans, Stacklit refresh commands, Semble root safety, generated index paths, and Pairing SessionStart metadata. Keeping those concerns mixed would either write repo-specific state into global `AGENT_TOOLS.md` or force users to keep manual project hook procedures in sync.

Liza needed first-class activation semantics for optional repository-navigation tools across setup, Pairing init, MAS init, prompts, and SessionStart.

## Decision

Split optional-index activation between global setup guidance and project-local init artifacts.

`liza setup` owns generic global tool guidance. The installed `AGENT_TOOLS.md` explains how agents should route Stacklit, SCIP, Semble, `rg`, `ast-grep`, and direct reads only when the current session supplies the corresponding concrete paths or target roots.

`liza init` owns project-local activation. In Pairing mode, init installs or verifies repository hook plumbing for enabled Stacklit and SCIP refresh, creates or verifies Semble safety artifacts, preserves existing hooks or reports collisions, and respects non-default hook paths. Pairing SCIP activation uses concrete per-language plans and supports explicit overrides for ambiguous monorepos instead of guessing silently.

MAS prompts remain task/reviewer/orchestrator scoped. They receive only target-specific metadata for indexes and Semble readiness. Pairing SessionStart receives repo-root context only when the corresponding artifacts or readiness checks are present.

Prompt bodies were trimmed so reusable command-routing text lives in `AGENT_TOOLS.md`, while prompts and SessionStart provide session-specific paths, roots, and availability.

## Consequences

Positive:
- Global tool guidance stays generic and safe when optional tools are disabled.
- Repository-specific index paths, hook commands, and safety files stay project-local.
- Pairing index refresh becomes reproducible through init-managed lifecycle hooks.
- MAS prompt routing keeps worktree-specific paths rather than Pairing repo-root assumptions.
- Ambiguous SCIP monorepos can use explicit plan overrides instead of generated guesses.
- Prompt context is shorter because reusable optional-tool routing is centralized.

Trade-offs:
- `liza init` gains more activation and hook-collision complexity.
- Pairing SCIP planning needs language-specific command planning and freshness checks.
- Operators must set activation environment variables before the setup or init command that consumes them.
- Hook behavior must preserve existing project hooks and support non-default hook paths.

## Alternatives Considered

1. Keep optional-index activation as a manual procedure.

Rejected because manual hook setup drifted from Liza's documented runtime behavior and was too easy to misapply.

2. Write repository-specific guidance into global `AGENT_TOOLS.md`.

Rejected because global setup content must not contain one repository's absolute paths, generated files, or hook plans.

3. Put all optional-tool routing text into every generated prompt.

Rejected because this duplicates long-form guidance and increases prompt size; prompts should supply availability metadata, not restate generic routing.

4. Use one activation path for Pairing and MAS.

Rejected because Pairing uses repo-root hooks and SessionStart context, while MAS agents use task/reviewer/orchestrator prompt metadata tied to worktrees.

## Relationship to Prior Decisions

Extends ADR-0068 (Optional Repository Indexing with SCIP and Stacklit), ADR-0074 (SessionStart Context Hooks), and ADR-0079 (Semble Semantic Repository Search). It turns those capabilities into a split setup/init activation model rather than a manual or prompt-only convention.

---
*Reconstructed from specs/goals/20260602-indexing-activation.md and commits f10328b6..c41d55fc (2026-06-02 to 2026-06-03)*

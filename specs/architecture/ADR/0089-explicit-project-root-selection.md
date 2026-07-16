# ADR-0089: Explicit Project-Root Selection

## Status

ACCEPTED

## Context

Normal Liza workflows invoke state commands from task worktrees. Resolving the
project only from the process working directory made those invocations fail
with project-root errors.

## Decision

Add -C and --project-root resolution independent of process CWD. Support the
exact root of an owned Liza task worktree by default, while retaining the Liza
project as the authoritative state target.

## Consequences

- Agents and operators can invoke supported commands from normal task
  worktrees.
- Project-root resolution is explicit rather than implicitly tied to CWD.
- External worktrees and non-Liza targets remain outside the supported model.

## Alternatives Considered

No formal alternatives were considered.

---

Reconstructed from commit 6d2f69d3 (2026-06-11). User context confirmed
2026-07-16.

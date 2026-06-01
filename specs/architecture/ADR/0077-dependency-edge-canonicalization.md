# 77 - Dependency Edge Canonicalization

## Status

ACCEPTED

## Context

ADR-0065 introduced recursive superseded dependency resolution: when a task depended on superseded work, claimability followed the `superseded_by` chain to replacement tasks. That matched the semantic meaning of supersession, but it left active task readiness dependent on recursive read-time interpretation at claim, unblock, status, diagnostics, and prompt-rendering time.

The follow-up implementation moved toward a stronger invariant: stored active dependency edges should already point at the current canonical task IDs. Read paths should not need to rediscover replacement meaning each time.

## Decision

Canonicalize dependency edges at mutation and transition time.

Liza now rewrites or rejects dependency edges when state changes:
- `supersede-task` rewrites active downstream `depends_on` edges to replacement tasks
- `cancel-task` removes active downstream edges to cancelled tasks
- claim and unblock validation require direct merged dependencies
- operational output `task_depends_on` entries are canonicalized before they can generate child tasks
- transition source dependencies, normal child creation, and crash recovery canonicalize dependency lists
- generated task dependencies are canonicalized at transition and crash-recovery time
- terminal output dependencies are rejected at write time
- invalid replacement paths fail without partially rewriting transition state

Recursive resolution remains a useful concept for understanding supersession, but the persisted graph is made canonical before consumers rely on it.

## Consequences

Positive:
- Claim, unblock, status, diagnostics, and prompt paths can reason over direct dependency IDs.
- Supersession and cancellation repair downstream edges at the time the state mutation occurs.
- Generated children do not inherit stale dependency IDs from old planning output.
- Partial mutation rollback protects state when a replacement path is invalid.
- State validation can enforce a simpler invariant: active dependencies point to valid non-terminal or merged tasks directly.

Trade-offs:
- Dependency-changing operations now have a larger mutation surface.
- Supersede/cancel/transition logic must update dependent state consistently.
- Bugs in canonicalization can rewrite too much or too little at mutation time.
- Historical output metadata may need canonicalization before later transitions consume it.

## Alternatives Considered

1. Keep ADR-0065 recursive read-time resolution.

Rejected because it spread dependency interpretation across too many consumers and allowed stale edges to remain in active state.

2. Require manual retargeting for every stale dependency.

Rejected because supersede and cancel already know the dependency rewrite implied by the operation.

3. Treat terminal dependency edges as harmless and resolve them lazily.

Rejected because terminal edges can poison generated children, transition state, and claimability later.

## Relationship to Prior Decisions

Supersedes ADR-0065's runtime recursive-resolution mechanism while preserving its core semantic claim: supersession redirects dependency meaning. Complements ADR-0075, which adds an explicit repair command for dependency retargeting when the correct rewrite is not implied by supersession or cancellation.

---
*Reconstructed from commits 1f720664..e7cb6544 (2026-05-21 to 2026-05-24)*

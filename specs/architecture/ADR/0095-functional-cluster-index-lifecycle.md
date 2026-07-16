# ADR-0095: Functional-Cluster Index Lifecycle

## Status

ACCEPTED

## Context

Liza's rigorous agent workflows can consume substantial context. Complex
repositories need token-efficient, high-level functional navigation without
degrading quality. Centralized index solutions do not fit isolated worktrees.

## Decision

Integrate Liza's local functional-clusters tool as a strict opt-in,
experimental index artifact. Refresh target-local functional-clusters.json
after Stacklit and SCIP prerequisites in pairing hooks and MAS worktrees.
Advertise the exact artifact path in prompts, while requiring source
verification because the artifact is advisory.

## Consequences

- Agents receive a functional map alongside structural and symbol indexes.
- Indexing remains compatible with worktree-local artifacts.
- The capability adds prerequisite gates, refresh work, temporary exports, and
  worktree-private artifact handling.
- Functional-cluster results can be stale and never replace source evidence.

## Alternatives Considered

Centralized indexing solutions were unsuitable because they are incompatible
with worktrees. No other formal alternatives were considered.

---

Reconstructed from commit 58405d35 (2026-07-05) and configuration
documentation. User context confirmed 2026-07-16.

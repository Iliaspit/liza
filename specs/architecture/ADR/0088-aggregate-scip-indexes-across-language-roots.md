# ADR-0088: Aggregate SCIP Indexes Across Language Roots

## Status

ACCEPTED

## Context

Complex repositories are where indexing is most valuable, but multiple detected
language roots previously produced an ambiguous-root skip. That meant no usable
SCIP index for those repositories.

## Decision

Generate indexes for each detected language root and aggregate them into one
repository-relative index per language. Pairing hooks and worktree refreshes
consume the aggregate paths. Write aggregate temporary files beside their final
paths so replacement remains atomic.

## Consequences

- Multi-root repositories receive repository navigation support instead of
  being skipped.
- Index generation and refresh gain a per-root aggregation step.
- A failed language refresh does not leave an unusable failed-language index
  exposed to runtime consumers.

## Alternatives Considered

No formal alternatives were considered.

---

Reconstructed from commit c950158b (2026-06-11). User context confirmed
2026-07-16.

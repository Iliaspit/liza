# Cluster 0077 - Dependency Edge Canonicalization

## Commit Set
- `1f720664` - fix(deps): canonicalize terminal dependency edges
- `9455761a` - fix(deps): canonicalize output task dependencies
- `e7cb6544` - fix(deps): canonicalize generated task dependencies

## Intent Hypothesis
Move dependency correctness from recursive read-time supersession resolution to write-time canonical dependency edges.

## Architectural Signals
- New `internal/ops/dependency_rewrite.go`
- Supersede rewrites active downstream edges
- Cancel removes active downstream edges
- Claim/unblock validation requires direct merged dependencies
- Output `task_depends_on`, transition source dependencies, child generation, and crash recovery canonicalize dependency lists
- Terminal output dependencies are rejected at write time

## Reconstructed Context
- Trigger: recursive supersession resolution left stale edges in active state and required multiple read paths to interpret replacement meaning.
- Alternatives: keep ADR-0065 runtime recursive resolution or require manual retargeting for every stale dependency.
- Rationale: mutation sites know when dependency meaning changes and can make the persisted graph canonical for consumers.
- Tradeoffs: mutation logic becomes larger and must avoid partial rewrites on invalid replacement paths.

## Candidate Decision Date
2026-05-21

## Status
ADR generated: `specs/architecture/ADR/0077-dependency-edge-canonicalization.md`

## Confidence
0.90 (high)

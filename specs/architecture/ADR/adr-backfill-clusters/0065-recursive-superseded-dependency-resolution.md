# Cluster 0065 - Recursive Superseded Dependency Resolution

## Commit Set
- `aab3c2f8` - resolve superseded dependencies recursively

## Source Material
- `liza-run-issues.md` sections 4 and 9

## Intent Hypothesis
Resolve task dependencies through `superseded_by` chains so downstream work cannot become claimable against stale baselines when the replacement work has not merged.

## Architectural Signals
- New `internal/models/dependency_resolver.go`
- Claimability, claim-time checks, unblock, diagnostics, validation, and agent prompt digest use the resolver
- Validation reports invalid supersession chains
- Replacement tasks must satisfy dependencies before downstream tasks become claimable

## User Context Captured
- Trigger: traversal-dependent downstream tasks became eligible or stayed blocked against superseded predecessors rather than the replacement path.
- Rationale: supersession is not dependency satisfaction; it redirects dependency evaluation to replacement work.
- Tradeoffs: dependency resolution becomes recursive and must detect invalid chains.

## Candidate Decision Date
2026-05-19

## Status
ADR generated: `specs/architecture/ADR/0065-recursive-superseded-dependency-resolution.md`

## Confidence
0.90 (high)

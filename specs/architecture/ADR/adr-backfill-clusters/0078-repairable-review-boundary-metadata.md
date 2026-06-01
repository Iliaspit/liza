# Cluster 0078 - Repairable Review Boundary Metadata

## Commit Set
- `b1e7cd1f` - fix(review): split code review diff boundaries
- `1692f388` - fix(review): repair stale review boundaries

## Intent Hypothesis
Preserve exact review boundaries while treating stale review metadata as repairable drift instead of integration failure.

## Architectural Signals
- Code review scope uses `base_commit..review_commit`
- `integration_branch..review_commit` is a separate late drift check
- Reviewer assignment blocks stale boundary metadata
- `update-review-commit` refreshes both `review_commit` and `base_commit`
- Merge-base support added for repair
- ADR-0064 recovery outcome is superseded in part

## Reconstructed Context
- Trigger: sibling integration drift could be misclassified as task scope deviation, and stale review metadata was too recoverable to force integration failure.
- Alternatives: keep ADR-0064 integration-failure recovery, auto-update on claim, or use current integration diff as review scope.
- Rationale: review integrity requires exact recorded commits, but stale metadata should have an explicit repair path.
- Tradeoffs: review flow gains repair complexity and distinguishes scope/workmanship drift from current-integration drift.

## Candidate Decision Date
2026-05-21

## Status
ADR generated: `specs/architecture/ADR/0078-repairable-review-boundary-metadata.md`

## Confidence
0.90 (high)

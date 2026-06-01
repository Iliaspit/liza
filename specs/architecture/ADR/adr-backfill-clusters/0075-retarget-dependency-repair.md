# Cluster 0075 - Retarget Dependency Repair

## Commit Set
- `c8685023` - feat(tasks): add retarget dependency repair
- `e6ce5db7` - fix(tasks): clarify repair evidence validation

## Intent Hypothesis
Provide a first-class repair path for replacing stale direct task dependencies without superseding the dependent task.

## Architectural Signals
- New `retarget-dependency` CLI operation
- Guarded ops mutation with canonicalization and full state validation
- RBAC wiring in pipeline configuration
- Wake/support docs guide blocked-task repair
- Repair evidence validation made more actionable

## Reconstructed Context
- Trigger: orchestrators needed to replace stale direct dependencies without treating the dependent task itself as superseded.
- Alternatives: supersede the dependent task, manually edit state, or rely on unblock-task repair notes.
- Rationale: dependency retargeting is a state mutation with scheduling consequences, so it needs canonicalization, RBAC, full validation, and auditable repair evidence.
- Tradeoffs: adds another mutation surface and requires agents to provide concrete validation evidence for repair requests.
- Related decisions: complements recursive superseded dependency resolution and blocked-task re-wake behavior.

## Candidate Decision Date
2026-05-31

## Status
ADR generated: `specs/architecture/ADR/0075-retarget-dependency-repair.md`

## Confidence
0.80 (high)

# Cluster 0067 - Master Planning Task Pattern

## Commit Set
- `60ee9afd` - feat(pipeline): add master planning task pattern (#81)

## Source Material
- `specs/architecture/ADR/0067-master-planning-task-pattern.md`
- `specs/goals/20260523-master-planning-task.md`
- `internal/embedded/pipeline.yaml`
- `internal/prompts/templates/blocks/master_decomposition_mandate.tmpl`
- `internal/prompts/templates/blocks/master_decomposition_review.tmpl`

## Intent Hypothesis
Add a reviewed coherence gate before planning fan-out starts parallel specialized work.

## Architectural Signals
- New decomposition-root master role-pairs before specialized planning fan-out
- Master tasks require quorum 2 and can pass through `partially-approved` and `reviewing-2`
- Typed decomposition metadata captures ownership, dependencies, framework refs, and subtask boundaries
- Approved master outputs auto-decompose into specialized planning children
- Simple work can still route directly to specialized planning tasks

## User Context Captured
- Trigger: parallel planning tasks could produce individually plausible but mutually inconsistent decompositions before adversarial review.
- Rationale: fan-out and uncertain work need a reviewed master decomposition before specialized planners spend agent cycles.
- Tradeoffs: fan-out work pays an additional planning review cycle; poor master decomposition becomes high-impact and is mitigated by quorum 2.

## Candidate Decision Date
2026-05-24

## Status
ADR generated: `specs/architecture/ADR/0067-master-planning-task-pattern.md`

## Confidence
0.90 (high)

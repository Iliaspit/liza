# Cluster 0066 - Architecture Sub-Pipeline and Spec Entry Points

## Commit Set
- `38f3be7d` - feat(pipeline): split architecture subpipeline

## Source Material
- `specs/architecture/ADR/0066-architecture-subpipeline-entry-points.md`
- `internal/embedded/pipeline.yaml`
- `support-docs/USAGE_MULTI_AGENTS.md`

## Intent Hypothesis
Separate architecture consolidation from coding and distinguish broad goals, functional specifications, technical specifications, and the legacy detailed-spec alias.

## Architectural Signals
- `architecture-pair` moved into a distinct `architecture-subpipeline`
- `architecture-to-code-plan` became a top-level pipeline transition
- Entry-point routing distinguishes `general-objective`, `functional-spec`, `technical-spec`, and `detailed-spec`
- CLI help, init wizard choices, prompt classification guidance, and user docs were synchronized

## User Context Captured
- Trigger: architecture was nested inside `coding-subpipeline`, which made entry altitudes hard to express cleanly.
- Rationale: pipeline topology should match the conceptual flow from specification to architecture to coding.
- Tradeoffs: existing frozen `.liza/pipeline.yaml` files do not automatically gain the new topology or entry-point names.

## Candidate Decision Date
2026-05-22

## Status
ADR generated: `specs/architecture/ADR/0066-architecture-subpipeline-entry-points.md`

## Confidence
0.90 (high)

# Cluster 0076 - Candidate Artifact Reference Guard

## Commit Set
- `d6c3d10a` - feat(artifact-refs): guard integration against invalid artifact refs (#79)

## Source Material
- `specs/goals/20260520-artifact-ref-protection.md`

## Intent Hypothesis
Prevent integration ref advancement when blackboard artifact refs would no longer resolve to regular files in the candidate Git tree.

## Architectural Signals
- Reusable statevalidate artifact-ref collector
- Candidate Git tree mode validation
- CAS merge pre-update hook
- Fail-closed diagnostics with artifact owner provenance
- Stale hook failure retry behavior
- Retained post-merge validation backstop

## Context
- Trigger: state-referenced planning and architecture artifacts can be removed during rescoping or cleanup while downstream tasks still point at them.
- Alternatives: post-merge validation only, human discipline, or filesystem validation.
- Rationale: integration safety must be proven against the candidate Git tree before the integration ref advances.
- Tradeoffs: merge gains another fail-closed guard and can block on stale or invalid artifact refs.

## Candidate Decision Date
2026-05-21

## Status
ADR generated: `specs/architecture/ADR/0076-candidate-artifact-reference-guard.md`

## Confidence
0.95 (high)

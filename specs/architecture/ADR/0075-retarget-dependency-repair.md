# 75 - Retarget Dependency Repair

## Status

ACCEPTED

## Context

Task dependency graphs can become stale after supersession, repair, or replanning. Sometimes the correct fix is not to supersede the dependent task; the dependent task is still valid, but one of its direct dependencies should point to a different task.

Manual state edits are unsafe because dependency changes affect claimability, wake behavior, and invariant validation. `unblock-task` repair notes can explain a problem, but they do not provide a narrow, validated mutation for replacing one dependency edge.

## Decision

Add `liza retarget-dependency` as a first-class repair operation.

The command replaces a stale direct task dependency with a new target through a guarded ops mutation. The mutation canonicalizes task references, runs full state validation, respects role-based access in pipeline configuration, and is documented in blocked-task wake/support flows.

Repair evidence validation was also clarified so agents get concrete accepted evidence formats instead of abstract shape errors.

## Consequences

Positive:
- Orchestrators can repair stale direct dependencies without superseding valid dependent work.
- Dependency retargeting is auditable and invariant-checked.
- State validation runs after mutation instead of relying on manual edits.
- Blocked-task wake guidance can point to a precise repair command.
- Repair evidence errors are actionable for agents.

Trade-offs:
- Adds another state mutation command.
- Incorrect retargeting can change scheduling behavior, so evidence quality matters.
- The operation repairs direct dependencies only; broader graph redesign still needs supersession or replanning.

## Alternatives Considered

1. Supersede the dependent task.

Rejected because the task may still be correct; only a direct dependency edge is stale.

2. Manually edit `.liza/state.yaml`.

Rejected because dependency rewrites need canonicalization, RBAC, and full validation.

3. Use unblock-task repair notes only.

Rejected because notes do not mutate the dependency graph.

## Relationship to Prior Decisions

Complements ADR-0065 (Recursive Superseded Dependency Resolution) and ADR-0063 (Blocked Task Alerts and Re-Wake). Supersession resolution handles replacement chains; retargeting handles explicit direct-edge repair.

---
*Reconstructed from commits c8685023..e6ce5db7 (2026-05-31)*

# 66 - Architecture Sub-Pipeline and Spec Entry Points

## Status

ACCEPTED

## Context

ADR-0056 introduced `architecture-pair` as the first step of `coding-subpipeline`. That gave the pipeline an architecture consolidation point, but it blurred pipeline boundaries: functional specification, architecture, and coding were all represented as the same coding sub-pipeline path.

The default pipeline now needs three explicit entry altitudes:
- broad goals that still need epic and user-story decomposition
- functional specifications that are ready for architecture consolidation
- technical specifications where architecture is already settled and code planning can begin directly

Keeping architecture inside `coding-subpipeline` made the second and third altitudes hard to express cleanly.

## Decision

Extract `architecture-pair` to a distinct `architecture-subpipeline` between `epic-spec-subpipeline` and `coding-subpipeline`:

```yaml
epic-spec-subpipeline:
  steps:
    - epic-planning-pair
    - us-writing-pair

architecture-subpipeline:
  steps:
    - architecture-pair

coding-subpipeline:
  steps:
    - code-planning-pair
    - coding-pair
```

Move `architecture-to-code-plan` from an intra-sub-pipeline transition to a top-level `pipeline-transition` with 3-part references:

```yaml
from: architecture-subpipeline.architecture-pair.approved
to: coding-subpipeline.code-planning-pair.initial
```

Entry points are:
- `general-objective` -> `epic-spec-subpipeline.epic-planning-pair`
- `functional-spec` -> `architecture-subpipeline.architecture-pair`
- `detailed-spec` -> `architecture-subpipeline.architecture-pair` as a legacy alias
- `technical-spec` -> `coding-subpipeline.code-planning-pair`

## Consequences

Positive:
- Pipeline topology now matches the conceptual flow: spec -> architecture -> coding.
- `functional-spec` and `technical-spec` distinguish whether architecture work is still needed.
- `detailed-spec` remains backwards compatible while new docs and UI can prefer `functional-spec`.
- Resolver graph logic remains generic because downstream traversal and output-consumer detection already use all configured transitions, including top-level pipeline transitions.

Trade-offs:
- Existing frozen `.liza/pipeline.yaml` files do not automatically gain the new topology or entry-point names.
- User-facing entry-point surfaces must stay synchronized: CLI help, init wizard choices, prompt classification guidance, and documentation.

## Alternatives Considered

1. Keep architecture as the first step of `coding-subpipeline` and add `technical-spec` as an entry point to `code-planning-pair`.

Rejected because it keeps architecture conceptually nested under coding while also asking users to distinguish architecture-needed and architecture-complete inputs.

2. Rename `detailed-spec` directly to `functional-spec` without an alias.

Rejected because existing scripts and initialized workspaces may already use `detailed-spec`.

**Supersedes part of:** ADR-0056, specifically the decision to insert `architecture-pair` as the first step of `coding-subpipeline`. The many-to-one fan-in, `arch_ref` propagation, and multi-parent task model from ADR-0056 remain in force.

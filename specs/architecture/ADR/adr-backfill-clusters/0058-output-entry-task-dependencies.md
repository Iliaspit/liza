# Cluster 0058 - Output Entry Concrete Task Dependencies

## Commit Set
- `93599d4f` - support concrete task dependencies

## Source Issues
- GitHub issue #31 - `set-task-output` cannot express dependencies on existing concrete task IDs

## Intent Hypothesis
Preserve the sibling-index contract of `OutputEntry.depends_on` while adding a separate field for generated tasks that must depend on already-existing concrete task IDs.

## Architectural Signals
- New `OutputEntry.task_depends_on` schema field
- Validation in `set-task-output` for concrete task references
- Propagation from `OutputEntry.task_depends_on` into generated child `Task.DependsOn`
- Prompt, support-doc, blackboard-schema, state-machine, and ADR updates

## User Context Captured
- Trigger: code-planning assignment required validation children to depend on existing implementation task IDs, but `depends_on` only accepted sibling output indexes.
- Rationale: keep `depends_on` unambiguously sibling-local instead of making one field context-dependent.
- Tradeoffs: another additive output schema field; referenced concrete tasks must already exist when output is submitted.

## Confidence
0.95 (high, ADR already generated)

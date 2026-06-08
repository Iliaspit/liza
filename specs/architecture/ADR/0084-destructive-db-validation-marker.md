# 84 - Destructive DB Validation Marker

## Status

ACCEPTED

## Context

Declared validation commands make task checks durable, but some checks can reset,
drop, or otherwise destroy database state. Liza cannot safely infer whether a
database target is disposable from stack-specific details such as DSNs, database
names, or vendor commands without violating stack-agnostic runtime behavior.

The destructive nature therefore needs to be explicit task metadata paired with
an explicit break-glass marker in the canonical validation command.

## Decision

Add optional `destructive_db` metadata to task and `output[]` contracts.

When `destructive_db: true`:
- `validation` must be non-empty.
- Every validation command must start with either
  `LIZA_ALLOW_DESTRUCTIVE_DB=1 ` or `env LIZA_ALLOW_DESTRUCTIVE_DB=1 `.
- The marker is part of the canonical command and must not be translated away by
  doers or reviewers.

When omitted or false, `destructive_db` is behaviorally inert and remains
backward-compatible.

Per-subtask transitions copy `output[].destructive_db` to generated children
alongside `output[].validation`. One-to-one and many-to-one transitions do not
inherit parent task `destructive_db` because parent task validation belongs to
the parent phase.

## Consequences

Positive:
- Destructive DB checks are explicit and auditable in state, prompts, and
  inspection output.
- Validation remains stack-agnostic; Liza does not parse project-specific DB
  targets or commands.
- Accidental destructive checks require both metadata and a syntactic leading
  marker on every declared command.

Trade-offs:
- Producers must mark every command for destructive DB validation tasks.
- The marker is a human/operator safety contract, not proof that the selected DB
  target is disposable.

**Extends:** ADR-0036 (Structured Task Output and Scope Extensions) —
`destructive_db` on `OutputEntry` and persisted `Task`.

**Extends:** ADR-0072 (Declared Validation Commands) — destructive DB validation
commands require a leading break-glass environment assignment.

## Alternatives Considered

1. Infer destructive DB checks from command text or database names.

Rejected because Liza is stack-agnostic and cannot reliably classify all project
database tooling without hardcoding project conventions.

2. Use a project-level global allow flag only.

Rejected because the safety boundary must live on the specific task/output
contract that declares the destructive validation.

3. Require only one command in the list to carry the marker.

Rejected because unmarked sibling commands in the same validation contract could
still run destructive work without the explicit break-glass prefix.

# 115 - Declarative Atomic Dependency Repairs

## Context and Problem Statement

A blocked agent can identify a dependency-graph repair that changes several
tasks or complete dependency lists, but an imperative sequence of repair
commands does not give that repair one durable identity. If commands are
applied separately, other readers can observe an intermediate graph, a later
failure leaves partial progress, and retry behavior depends on reconstructing
which commands already ran.

`retarget-dependency` remains appropriate for one direct edge on a
non-terminal task. It is intentionally not a batch grammar. A broader active
graph repair needs a declarative request and one atomic consumer.

## Decision Outcome

A blocked agent persists a complete JSON `RepairRequest` with
`mark-blocked --repair-request-file <path>`. For operation
`apply-dependency-repair`, the request:

- targets the blocked task that owns it;
- omits `command` and includes one or more ordered `dependency_updates`;
- gives each unique `task_id` an explicit `expected_depends_on` list and an
  explicit `desired_depends_on` list, including `[]` when empty;
- includes structured failure evidence and validation instructions.

The file input is mutually exclusive with the individual `--repair-*` flags.
Those flags and command-based requests remain available for non-dependency
repair operations. Agents do not encode multi-command dependency repairs.

The orchestrator applies the stored request with:

```text
apply-dependency-repair <blocked-task-id> --reason <reason>
```

Within one locked blackboard mutation, the operation confirms that the source
task is still `BLOCKED`, owns the matching request, and that every affected
task still has its declared expected dependencies. It then canonicalizes every
desired list, rejects missing, self, terminal-target, direction, or cycle
violations, assigns all candidate lists in memory, and validates the full state.
Only a valid complete candidate is persisted. Failure leaves every dependency,
history entry, and the repair request unchanged.

Success appends attributable `dependencies_rewritten` history to every updated
task, clears the consumed request, and reports every task with its committed
canonical dependencies. When the source is not itself updated, it receives a
source-local `dependency_repair_applied` history receipt. Task inspection
projects the latest receipt as `dependency_repair_receipt`, including affected
task IDs and the request's declared validation. The source task remains
`BLOCKED`; validation and `unblock-task` remain separate. Activity-log failure
follows the existing warning-after-commit convention and does not misreport a
rollback.

The three dependency-repair modes remain distinct:

- `retarget-dependency` repairs one direct edge on a non-terminal task;
- `apply-dependency-repair` consumes one stored declarative batch for active
  dependency state;
- `repair-superseded-dependencies` removes all illegal downstream direct edges
  from one already-`SUPERSEDED` task.

## Consequences

**Positive:**

- A multi-task repair has one durable desired-state identity.
- Expected lists provide an explicit stale-request check.
- No partial dependency graph is serialized or exposed between updates.
- Canonicalization, direction checks, full-state validation, RBAC, and audit
  history remain at the mutation boundary.

**Trade-offs:**

- Request authors must provide complete expected and desired direct lists.
- Stale requests fail rather than merging with intervening graph changes.
- The operation is dependency-specific rather than a general transaction DSL.

## Alternatives Considered

1. Run multiple `retarget-dependency` invocations.

Rejected because each invocation commits independently and exposes partial
state and ambiguous retries.

2. Add a batch argument grammar to `retarget-dependency`.

Rejected because it would still store execution instructions rather than one
desired graph state and would weaken the command's narrow one-edge contract.

3. Introduce a generic state-repair transaction language.

Rejected because the demonstrated need is dependency-specific and a generic
mutation language would expand validation and authorization risk.

## Relationship to Prior Decisions

- **Extends ADR-0075:** `retarget-dependency` remains the one-edge repair path.
- **Preserves ADR-0077:** every desired dependency list is canonicalized and
  the complete candidate graph is validated before persistence.
- **Complements ADR-0114:** active declarative graph repair remains separate
  from terminal superseded-metadata recovery.

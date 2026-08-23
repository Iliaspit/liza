# ADR-0116: Orchestrator Classifies Defect Objectives

## Status

ACCEPTED

## Context

Liza had no defect concept. A bug-fix objective reached `code-planning-pair` and was
planned like any feature scope, against whatever diagnosis happened to be in the goal
spec. Nothing required the code-planner to establish a root cause, and nothing let the
code-plan-reviewer reject a plan built on a wrong one.

The `adversarial-pairing` skill already demonstrates the value of separating that
analysis from the plan: in
`specs/adversarial-pairing/20260823-fix-orphaned-transitions_executed-markers-bb.md`,
the root cause analysis went through four revisions before approval. Revision 1
asserted a mid-transition partial write that revision 2 retracted once the reviewer
checked the write path; revision 3 claimed two predicates matched, which revision 4
withdrew after the reviewer found the branch that disproved it. A plan written on
revision 1 would have argued for atomicity the code already had.

MAS already has the mechanism that produced that outcome — `code-planning-pair`
rejection cycles are binding. What was missing was an artifact requirement, a reviewer
criterion, and a way for both agents to agree that the objective is a defect at all.

## Decision

The orchestrator classifies the objective during `INITIAL_PLANNING`, alongside its
existing entry-point and fan-out classification, and records the result as
`rca_required` on the single task it creates.

The flag defaults to false, inherits unconditionally to child tasks through every
construction path in `proceed.go`, and is copied onto replacement tasks by
`replan.go`. When true, the code-planner prompt requires a `## Root Cause Analysis`
section in the plan file, and the code-plan-reviewer prompt gains reject gates for it
plus an instruction to evaluate that section before the task breakdown.

Classification sits with the orchestrator rather than the code-planner because the
alternative makes planner and reviewer each classify independently: when they
disagree, the rejection cycle burns on whether this is a defect instead of on the
analysis. One recorded decision removes that class of churn.

## Consequences

Positive: a defect objective now produces a root cause that a peer reviewer can
refute against cited evidence, and the reproduction it names binds to an `output[]`
`done_when`, so the coder's failing test exercises the real path. The flag is visible
in `state.yaml` and through `get <task-id> --json`. Feature work is untouched — both
prompt additions are conditional, so they cost no tokens when the flag is false.

Negative: a mis-classified defect goal silently gets no analysis. This was chosen
deliberately over a reviewer backstop that would have made the reviewer re-derive the
classification, reintroducing the disagreement the flag exists to remove. The
mitigation is documentation, not code.

The classification is observable but not mutable: no command changes `rca_required` on
an existing task. `add-tasks` only creates, `replan` preserves, and direct `state.yaml`
edits are forbidden. Correcting a misclassification means abandoning the task and
creating a replacement with a fresh ID while it is still in its initial state — the
`cancel-task` + `add-tasks` procedure documented in `USAGE_MULTI_AGENTS.md`. Adding a
mutation surface was not in scope and is not implied by this decision.

Review-flow assessment (G1.2), replacing a blanket "no invariant intersection": the
change does not alter task-state transitions, so §3.1 field requirements and §3.4
dependency direction are untouched — the field is optional and not state-machine
bearing. It does intersect §6 Review & Approval: the RCA-first instruction directs a
reviewer to reject before inspecting the task breakdown, which sits against "Review
covers ALL changes (`base_commit` → `review_commit`)". These are judged compatible —
that invariant bounds the commit range under review, not the order sections are read,
and a later round still covers the full range — but the interaction is real and is
recorded here rather than asserted away. §14 Anomaly Logging is touched thematically
and not violated: the accepted silent default is a hidden-failure surface with no
anomaly carrier, and no §14 invariant is extended or weakened.

The flag is goal-level with no per-`output[]` override, so a defect goal that fans out
marks all of its scopes, including any that do not touch the defective path. Deferred:
an override needs `*bool` on `OutputEntry` to express "exempt this one", and the
dominant path for defects is the `technical-spec` entry point, which creates exactly
one code-planning task. The trigger to add it is the first fan-out defect goal whose
unrelated scopes produce padded root cause sections.

Naming: `rca_required` matches the `adversarial-pairing` frontmatter field
deliberately, but means something weaker here — a required section within a reviewed
artifact, not a separately reviewed analysis phase. Divergent names for one concept
were judged more costly than the ambiguity; the difference is documented in
`blackboard-schema.md`.

## Alternatives Considered

**Self-classification by the code-planner.** No plumbing, no orchestrator change.
Rejected: the reviewer must then classify independently to enforce anything, and
planner/reviewer disagreement becomes review churn about classification.

**Reusing the existing `Kind` field.** It is already present on both `Task` and
`OutputEntry` with a value registry. Rejected on evidence: `Kind` carries
singleton-dedup semantics — `collectNonTerminalByKind` enforces one non-terminal task
per kind repo-wide, and `resolveKindDedup` silently skips and remaps later entries. A
`Kind: "bugfix"` would make the second bug-fix task disappear into a remap.

**A separate analysis role-pair with its own review gate**, mirroring the
`adversarial-pairing` `ANALYZING` phase. This is the strongest form — it prevents the
plan from being written before the cause is approved. Rejected for now as a workflow
change disproportionate to an unvalidated need; the RCA-first ordering instruction in
the reviewer prompt approximates the sequencing at a fraction of the cost. If reviewers
prove unable to gate the section within a bundled artifact, this is the next move.

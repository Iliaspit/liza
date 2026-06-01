# 72 - Declared Validation Commands

## Status

ACCEPTED

## Context

Task prompts could leave validation ambiguous. Agents and reviewers then inferred environment-specific tool paths, ran vacuous checks, or accepted exit-code success without evidence that the intended lint, type, test, or hook gate actually ran.

That undermined the contract between planners, implementers, reviewers, and integration agents. Validation needed to become explicit task data rather than implicit local interpretation.

## Decision

Add declared validation commands to task and output-entry contracts.

Tasks and generated `output[]` entries can carry an ordered `validation` list. Per-subtask validation propagates into generated child tasks. State validation checks command shape, and prompts render the canonical commands for doers and reviewers.

Validation commands must be:
- single-line
- non-empty
- without leading or trailing whitespace
- single-purpose
- agent-executable for the task scope
- capable of proving the intended check ran, not merely returning exit code 0

Forbidden command shapes include `cd ... &&`, command substitution/backticks, polling or tail pipelines, and task artifact paths outside the worktree. Existing stored commands that violate newer guidance remain visible and are translated by consumers rather than silently rewritten.

## Consequences

Positive:
- Planners can make validation expectations durable.
- Doers and reviewers use the same canonical commands.
- Generated child tasks inherit relevant validation from their output entries.
- Review can reject vacuous validation evidence.
- Validation guidance becomes inspectable in state and prompts.

Trade-offs:
- Task and output schemas are larger.
- Poorly chosen validation commands can still be too narrow or stale.
- Producers must think about validation at planning time.
- Consumers need fallback behavior for older unsafe stored commands.

## Alternatives Considered

1. Strengthen prompt wording only.

Rejected because prompt-only guidance leaves validation implicit and easy to lose across task generation.

2. Use project-level default validation only.

Rejected because different generated children can need different targeted checks.

3. Let reviewers discover validation commands.

Rejected because it pushes ambiguity downstream and increases review inconsistency.

## Relationship to Prior Decisions

Extends ADR-0007 (TDD Enforcement in MAS), ADR-0030 (Code-Enforced Agent Guardrails), ADR-0036 (Structured Task Output and Scope Extensions), and ADR-0059 (Partial Planning Handoff). Validation is now part of the generated task contract, not only reviewer judgment.

---
*Reconstructed from commits 1e88541f..03e2d19f (2026-05-28 to 2026-05-31)*

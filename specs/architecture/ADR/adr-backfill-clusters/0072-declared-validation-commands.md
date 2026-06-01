# Cluster 0072 - Declared Validation Commands

## Commit Set
- `1e88541f` - feat(tasks): add declared validation commands
- `0bad71fa` - fix(prompts): require validation satisfiability evidence
- `03e2d19f` - fix(prompts): require executable validation commands

## Intent Hypothesis
Make validation commands explicit task contract data instead of environment-specific agent inference.

## Architectural Signals
- Task and output entries gain validation command fields
- Validation command shape is enforced in state validation
- Per-subtask validation propagates into generated children
- Doer and reviewer prompts render canonical task-declared commands
- Specs document command satisfiability and executable-command constraints

## Reconstructed Context
- Trigger: generated tasks could leave validation ambiguous, leading agents and reviewers to infer local or environment-specific tool paths.
- Alternatives: rely on stronger prompt text, project-level default validation, or reviewer-discovered commands.
- Rationale: validation requirements belong with the task contract and generated child-task output, so every downstream agent sees the same executable expectation.
- Tradeoffs: validation commands add schema surface and can be underspecified, stale, or too narrow if planners write poor commands.
- Related decisions: extends TDD enforcement, code-enforced guardrails, structured task output, and partial planning handoff.

## Candidate Decision Date
2026-05-28

## Status
ADR generated: `specs/architecture/ADR/0072-declared-validation-commands.md`

## Confidence
0.90 (high)

# 44 - Optional TDD Enforcement

## Status

ACCEPTED

## Context

ADR-0007 made TDD mandatory for all code tasks in MAS. This was the right default — without a human in the loop, enforced TDD prevents the failure mode where tests validate implementation rather than spec intent.

However, frontend teams and teams working on UI-heavy projects find mandatory inline TDD constraining. Their workflow prefers implementing features first, then writing focused tests as follow-up tasks. Forcing TDD-first in these contexts creates friction without proportional quality gain.

The core insight: TDD enforcement is about ensuring test coverage exists, not about when tests are written relative to implementation.

## Decision

Make TDD enforcement configurable per workspace via `tdd_enabled` in the config section of `state.yaml`.

**Phase 1 (implemented):**
- `Config.TDDEnabled *bool` — nil defaults to true (backward compatible)
- `liza init --no-tdd` sets `tdd_enabled: false`
- Submission gate (`submit_review.go`) skips test file check when disabled
- Prompt templates condition TDD instructions on config value
- Code Planner instructs creation of separate test tasks when TDD is disabled
- Code Plan Reviewer verifies test task coverage when TDD is disabled

**Phase 2 (future):**
- `OutputEntry.DependsOn []int` — inter-task dependencies within code planner output
- Enables structured ordering: implementation task → test task
- Validation: bounds checking, self-reference prevention, cycle detection

## Consequences

**Positive:**
- Teams can choose the workflow that fits their domain
- Default remains TDD-mandatory — no regression for existing workspaces
- Test coverage is still tracked (as separate tasks, not inline)

**Negative:**
- Non-TDD workspaces lose the spec-first testing guarantee
- Code Planner must create matching test tasks (additional planning complexity)
- Two code paths in prompts and gates to maintain

## Alternatives Considered

**1. Per-task TDD override:** Allow individual tasks to opt out of TDD. Rejected — too granular, creates inconsistency within a sprint. TDD policy should be a workspace-level decision.

**2. Remove TDD enforcement entirely:** Make tests always optional. Rejected — ADR-0007's rationale remains valid for most workspaces. The failure mode is real and costly.

**3. TDD-lite mode (tests after, same task):** Require tests in the same task but allow writing them after implementation. Rejected — this is a workflow preference that's hard to enforce mechanically. The binary choice (inline vs separate tasks) is cleaner.

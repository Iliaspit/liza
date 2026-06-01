# 64 - Review Boundary Recovery

## Superseded In Part

ADR-0078 supersedes the recovery outcome in this ADR. Stale review boundary metadata is now treated as repairable drift via `update-review-commit`, not automatically as `INTEGRATION_FAILED`. The invariant that reviewers must inspect the recorded `review_commit` remains in force.

## Context and Problem Statement

`submit-for-review` can rebase a task worktree and rewrite the commit SHA. Reviewers must inspect the exact commit Liza records as `review_commit`. In the observed failure, reviewer prompts referenced a pre-rebase commit while the task worktree had moved to the post-rebase HEAD.

That created an invalid local lifecycle boundary before any GitHub handoff or external operator action. The reviewer correctly stopped at the drift check, and normal verdict submission also failed because Liza detected the worktree/review-commit mismatch.

## Considered Options

1. **Auto-update `review_commit` when a mismatch is found** - tempting as a repair, but violates the review-boundary invariant and can hide lifecycle drift.
2. **Release/reassign review and keep the task reviewable** - avoids immediate failure, but can spin reviewers against an invalid boundary.
3. **Treat stale review boundary as integration failure** - preserve the worktree and route the task through recovery diagnostics.

## Decision Outcome

Chose **Option 3**: enforce review-boundary validation and recover stale candidates into `INTEGRATION_FAILED`.

### Architecture

- Shared review-boundary validation lives in `internal/ops/review_boundary.go`.
- Reviewer claim and `await-resubmission` paths reject stale review boundaries.
- Stale review-boundary tasks transition to `INTEGRATION_FAILED` with diagnostics.
- Reviewer claim skips stale candidates and continues to valid candidates in the same claim attempt.
- Rewriting-rebase regression coverage verifies `submit-for-review` stores and returns post-rebase HEAD.

### Rationale

There is an invariant preventing review from proceeding when `review_commit` and worktree HEAD disagree. Auto-updating the review commit at claim time would bypass that invariant and risk making the reviewer inspect a boundary no one intentionally submitted.

Moving to `INTEGRATION_FAILED` keeps the worktree available for inspection and repair while preventing reviewer spin.

### Consequences

**Positive:**
- Reviewers are not assigned commits that differ from task worktree HEAD.
- Boundary drift becomes explicit recovery work instead of a confusing review failure.
- Worktrees are preserved for diagnosis.
- Review claim loops can continue to valid candidates instead of blocking on one stale task.

**Limitations accepted:**
- Some tasks that might be manually repairable are moved out of normal review flow.
- Operators must use integration recovery paths rather than silent review-commit mutation.

**Extends:** ADR-0054 (Blocking Await Primitives) - review wait/reclaim paths now enforce the same boundary. ADR-0055 (Integration Sub-Pipeline) - stale review boundaries enter integration-style recovery instead of ordinary review.

---
*Reconstructed from commit 646b243c and GitHub issue #71 (2026-05-19)*

# Cluster 0064 - Review Boundary Recovery

## Commit Set
- `646b243c` - enforce review boundary recovery

## Source Issues
- GitHub issue #71 - submit-for-review can assign stale review_commit after rebase

## Intent Hypothesis
Treat the review commit/worktree HEAD match as a hard local lifecycle boundary, and recover stale review candidates before reviewers are assigned or spun.

## Architectural Signals
- Shared review-boundary validation in `internal/ops/review_boundary.go`
- Reviewer claim and await-resubmission paths reject stale review boundaries
- Stale review-boundary tasks transition to `INTEGRATION_FAILED` with diagnostics
- Reviewer claim logic skips stale candidates and continues to valid candidates
- Rewriting-rebase regression coverage proves `submit-for-review` stores and returns post-rebase HEAD

## User Context Captured
- Trigger: reviewers received a `REVIEW COMMIT` that differed from the task worktree HEAD after submit-time rebase.
- Rationale: reviewers must inspect the exact commit Liza records; mismatch is a lifecycle invariant failure, not a normal review rejection.
- Tradeoffs: stale-boundary tasks fail into integration recovery instead of remaining reviewable; this may be stricter but prevents review churn.

## Candidate Decision Date
2026-05-19

## Status
ADR generated: `specs/architecture/ADR/0064-review-boundary-recovery.md`

## Confidence
0.90 (high)

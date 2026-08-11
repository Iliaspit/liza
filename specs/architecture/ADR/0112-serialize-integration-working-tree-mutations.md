# 112 - Serialize Integration Working-Tree Mutations

## Context and Problem Statement

ADR-0022 replaced checkout-based integration merges with `merge-tree`,
`commit-tree`, and compare-and-swap `update-ref` operations. A later correctness
fix transiently synced merge-affected files into the main working tree so
integration tests and operators could see the committed result, then restored
those files when another branch was checked out.

Concurrent `wt-merge` processes share that working tree and its Git index.
Forward sync, rollback sync/restore, and success restore could therefore invoke
`git checkout <treeish> -- <paths>` at the same time and collide on
`.git/index.lock`. In-process synchronization is insufficient because production
supervisors invoke merges from separate processes.

## Considered Options

1. **Use an in-process mutex.** Small, but it does not coordinate separate CLI
   and supervisor processes.
2. **Hold a project-scoped file lock around integration ref/index mutations.**
   Reuses the existing file-lock mechanism and preserves concurrent integration
   test execution.
3. **Run integration tests in an isolated worktree.** Shrinks shared-tree
   mutation, but does not remove the required sync when the integration branch
   is checked out and is a larger lifecycle change.
4. **Hold the lock across integration tests.** Prevents all sync/test/restore
   interleaving, but serializes the full integration pipeline.

## Decision Outcome

Choose **Option 2**. Option 3 remains complementary future work rather than a
replacement for cross-process serialization.

A project-scoped exclusive file lock under `.git/` covers:

- compare-and-swap integration ref advancement plus forward main-index sync;
- rollback ref advancement plus rollback sync and branch restore;
- success-path branch restore.

Commit construction remains working-tree-less. CAS remains a defense against
external ref writers and stale expected heads even though cooperating merge
processes no longer spend their retry budget contending with one another.

The lock timeout is 30 seconds. The protected CAS, blackboard-read, and checkout
window is normally sub-second, so this permits a queue of dozens of merges while
bounding an unusually large or cold-cache checkout. Timeout errors name the
integration mutation lock and remain ordinary retryable merge errors rather than
`IntegrationFailedError` values.

Lock ordering is integration mutation lock, then blackboard read lock. The
integration lock is released before any blackboard state write, preventing a
future inverse-order deadlock.

## Consequences

**Positive:**

- Concurrent merge processes cannot collide on the shared Git index.
- Every individual ref/index mutation sequence is atomic across cooperating
  processes.
- CAS retries remain available for non-cooperating external Git writers.
- Contending supervised merges wait instead of consuming the three-attempt CAS
  retry budget.

**Trade-offs:**

- Ref advancement and checkout-duration index mutations are serialized.
- The lock is released while integration tests run. Concurrent merges touching
  the same path can still interleave sync/test/restore windows and leave the
  checked-out tree stale; `TECH_DEBT.md` records the payback trigger for isolated
  integration-test worktrees or full-window serialization.

## Supersedes

- The no-external-lock portion of ADR-0022's concurrency mechanism. Its
  working-tree-less commit construction and CAS merge decisions remain active.

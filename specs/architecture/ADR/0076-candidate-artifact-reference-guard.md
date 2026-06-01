# 76 - Candidate Artifact Reference Guard

## Status

ACCEPTED

## Context

Liza stores artifact references in blackboard state: goal specs, planning artifacts, architecture docs, output refs, and similar files that later tasks rely on. During rescoping, cleanup, or branch integration, a candidate merge can remove or replace those files while downstream state still points at them.

Post-merge validation catches some of this after the integration ref has advanced, but by then the system has already published a candidate tree that violates active state. The protection needs to run before the ref update, while the candidate tree is still only a candidate.

This rationale is reconstructed from `specs/goals/20260520-artifact-ref-protection.md` and the implementation commit.

## Decision

Validate protected artifact references against the candidate Git tree before advancing the integration ref.

Liza now:
- collects protected artifact refs from state with deterministic owner provenance
- validates candidate tree paths through Git tree mode lookups
- accepts only regular file modes
- rejects missing paths, directories, symlinks, gitlinks, invalid refs, and unsafe path syntax
- wires the validation into the CAS merge pre-update hook
- reloads fresh state for each hook invocation
- retries when a hook rejection becomes stale because another writer advanced integration first
- keeps the post-merge validation backstop

The merge guard fails closed. When it rejects an approved task before the integration ref update, the task is moved to `INTEGRATION_FAILED` with diagnostic detail instead of being retried indefinitely as still approved.

## Consequences

Positive:
- Integration cannot advance to a tree that breaks active artifact references.
- Diagnostics identify which state field owns the broken ref.
- Candidate validation happens before the irreversible ref update.
- CAS retries validate the recomputed candidate, not a stale tree.
- The post-merge validation backstop remains as defense in depth.

Trade-offs:
- Merge now depends on fresh state reads during the CAS loop.
- Git tree-mode validation adds another failure mode to integration.
- Only regular Git files are accepted; directories and symlinks are rejected even if a human might consider them navigable.
- Invalid artifact refs fail closed and can block unrelated merge progress until repaired.

## Alternatives Considered

1. Keep post-merge validation only.

Rejected because it detects the violation after integration has already advanced.

2. Trust artifact refs to be maintained by planning/review discipline.

Rejected because rescoping and cleanup can invalidate refs outside the local task that introduced them.

3. Validate against the worktree filesystem.

Rejected because merge safety is about the candidate Git tree that will become integration, not transient worktree contents.

## Relationship to Prior Decisions

Extends ADR-0022 (Concurrency Hardening) by adding a CAS pre-update validation hook. Extends ADR-0049 (Structured Handoff Events) and later planning-output ADRs by protecting durable artifact references that agents and humans use for traceability.

---
*Reconstructed from specs/goals/20260520-artifact-ref-protection.md and commit d6c3d10a (2026-05-21)*

# 65 - Recursive Superseded Dependency Resolution

## Context and Problem Statement

Liza tasks can be superseded by replacement tasks. Before this decision, superseded tasks could be treated as dependency-satisfying even when their replacement work had not merged. That allowed downstream tasks to become claimable against stale baselines or remain blocked by old references instead of following the replacement path.

This was especially visible in traversal-dependent work: downstream discovery tasks could start before the shared traversal foundation and lookup APIs were actually present in their worktree baselines.

## Considered Options

1. **Treat superseded tasks as terminal/satisfied dependencies** - simple, but downstream tasks can run against stale or missing work.
2. **Require dependencies to resolve through supersession chains** - replacement tasks must satisfy the original dependency before downstream work is claimable.

## Decision Outcome

Chose **Option 2**: resolve dependencies recursively through `superseded_by` chains.

### Architecture

- New `internal/models/dependency_resolver.go`.
- Dependency resolution follows superseded tasks to their replacement tasks.
- All replacements must satisfy the original dependency.
- Invalid supersession chains are reported by validation.
- The resolver is wired into:
  - claimability
  - claim-time checks
  - unblock logic
  - diagnostics
  - state validation
  - agent prompt dependency digest

### Rationale

Consistency across tasks is essential for the pipeline to run smoothly. Supersession is not dependency satisfaction; it is dependency redirection. A downstream task that depended on old work must wait for the replacement work that now represents that dependency.

Recursive resolution makes the scheduler match the human meaning of supersession.

### Consequences

**Positive:**
- Downstream work no longer becomes claimable against stale superseded baselines.
- Replacement paths are reflected consistently across claim, unblock, diagnostics, validation, and prompts.
- Invalid supersession chains become validation errors instead of latent scheduling bugs.

**Limitations accepted:**
- Dependency resolution is now recursive.
- Cycles and invalid chains must be detected explicitly.
- Claimability can change when supersession chains are updated.

**Extends:** ADR-0020 (Explicit Task Workflow Contract) - dependency satisfaction now includes supersession semantics. ADR-0051 (First-Class Attempt Model) - replacement attempts/tasks participate in scheduling correctness.

---
*Reconstructed from commit aab3c2f8 and liza-run-issues.md (2026-05-19)*

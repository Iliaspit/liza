# 82 - Worktree Env-File Provisioning

## Status

ACCEPTED

## Context

Liza runs agents in isolated worktrees. That isolation keeps task diffs clean and protects the main checkout, but it also means ignored project-root environment files are absent from task worktrees.

Some projects need local ignored env files for post-worktree setup or dependency commands. Tracking those files is usually wrong because they can contain local paths, machine-specific settings, or secrets. Asking agents to copy them manually is also unsafe and unreliable, especially when agents run in sandboxes and should not inspect or expose credential contents.

The system needed a narrow provisioning mechanism that could make selected ignored env files available in worktrees without weakening diff cleanliness or credential boundaries.

## Decision

Add opt-in worktree env-file provisioning for ignored project-root env files.

The feature is activated through init configuration, CLI flags, and the `LIZA_ENABLE_COPY_ENV_FILES` init gate. When enabled, Liza copies configured root-only env-file candidates into task worktrees before post-worktree commands run in create, claim, and reviewer recovery paths.

Before copying, Liza verifies both source and destination safety:
- the source must be a project-root file selected by the configured candidate set
- the source must be ignored rather than tracked
- the destination must remain ignored in the worktree
- unsafe sources, private-exclude conflicts, and existing destinations are handled explicitly

The operation copies files as provisioning artifacts; it does not log, display, or interpret their contents.

## Consequences

Positive:
- Post-worktree setup can run with required local env files present.
- Projects do not need to track environment files to make Liza worktrees usable.
- Worktree isolation and clean task diffs are preserved through ignore-status checks.
- Env-file handling is explicit, opt-in, and validated instead of ad hoc agent behavior.

Trade-offs:
- Credential-sensitive files now have a controlled copy path into worktrees, so ignore checks and non-disclosure rules are critical.
- Operators must configure or enable the behavior intentionally.
- Existing destination files and private-exclude conflicts add lifecycle edge cases.
- Copied env files can become stale relative to the project root until the next provisioning point.

## Alternatives Considered

1. Require users or agents to copy env files manually.

Rejected because manual copying is unreliable, hard to validate, and risky for sandboxed agents.

2. Track env files in Git.

Rejected because local env files often contain secrets or machine-specific configuration.

3. Run post-worktree setup without local env files.

Rejected because some projects require those files before setup can succeed.

4. Have post-worktree commands read env files from the parent project root.

Rejected because that weakens worktree isolation and makes task work depend on out-of-worktree state.

## Relationship to Prior Decisions

Extends ADR-0031 (Configurable Post-Worktree Command) by ensuring post-worktree setup can receive required local env files without hardcoding project assumptions. Complements ADR-0050 (Brownfield-Safe Initialization) by reducing adoption friction for existing projects. Reinforces ADR-0068 and ADR-0079's generated-artifact cleanliness discipline by requiring worktree-visible files to remain ignored.

---
*Reconstructed from commit b1d3ed79 (2026-06-03)*

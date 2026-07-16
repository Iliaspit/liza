# ADR-0091: Optional Bash-Policy Initialization

## Status

ACCEPTED

## Context

Claude Code's unit-command permission model could not safely express the
multi-statement shell commands it commonly executes. Bash-policy activation was
also a separate step that new and existing users could miss.

## Decision

Keep bash-policy as a standalone owner of provider-specific behavior. Its Bash
AST parser and leaf-level permission model make multi-statement authorization
structurally analyzable. When the LIZA_ENABLE_BASH_POLICY gate is enabled,
Liza initialization writes the policy artifact and delegates provider-specific
initialization and activation to the bash-policy CLI.

## Consequences

- Bash-policy participates in normal toolchain and project initialization.
- Liza adds feature-gate, external CLI, version-probing, and installer-fallback
  integration.
- Initialization warns and continues when bash-policy is unavailable or fails;
  active policy enforcement is not guaranteed by Liza initialization.
- A future classifier may complement AST-based authorization, but it currently
  has token cost and frequent retraining limitations.

## Alternatives Considered

A classifier remains a possible future complement. It is not the current
authorization boundary because of token consumption and retraining needs.

---

Reconstructed from commit 4a9bb770 (2026-06-16) and configuration
documentation. User context confirmed 2026-07-16.

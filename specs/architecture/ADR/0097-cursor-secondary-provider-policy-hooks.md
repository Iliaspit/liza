# ADR-0097: Cursor Secondary-Provider Policy Hooks

## Status

ACCEPTED

## Context

Cursor can use secondary providers such as Claude and GPT. Its Liza setup
therefore needed the associated provider setup and policy hooks to be
performed together.

## Decision

Add Cursor activation to Liza initialization, including the Claude and Codex
project setup Cursor relies on. Generate Cursor-compatible allow/deny hook
responses and delegate shell-policy evaluation to bash-policy. Keep Cursor
provider compatibility in the catalog even when a cached catalog is stale.

## Consequences

- Cursor setup includes the secondary-provider configuration it requires.
- Provider-specific hook generation and policy integration become part of
  initialization.
- A bash-policy or hook failure can block Cursor shell commands; this security
  availability cost is accepted.

## Alternatives Considered

No formal alternatives were considered.

---

Reconstructed from commit 2b63bbb7 and follow-up fixes (2026-07-06). User
context confirmed 2026-07-16.

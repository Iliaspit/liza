# ADR-0093: Shell Completion From Command Metadata

## Status

ACCEPTED

## Context

Interactive CLI use needed better discoverability and faster operation.

## Decision

Add shell completion through Cobra's built-in completion command, with
canonical role values and arity-aware positional completion. Register
completion through toolchain shell activation.

## Consequences

- Interactive users receive command and value suggestions.
- Completion handlers and tests evolve with CLI commands.
- Shell activation remains required and shell-dependent.

## Alternatives Considered

No formal alternatives were considered.

---

Reconstructed from commit 809eb9f2 (2026-06-29). User context confirmed
2026-07-16.

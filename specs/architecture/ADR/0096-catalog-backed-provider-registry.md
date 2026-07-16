# ADR-0096: Catalog-Backed Provider Registry

## Status

ACCEPTED

## Context

The initial provider implementation was hardcoded across CLI lists, setup
mappings, contract checks, prerequisites, launch behavior, repair/TUI
validation, and documentation. Adding providers repeatedly increased
cross-cutting complexity and lifecycle drift risk.

## Decision

Move provider-specific setup and runtime metadata into a structured catalog
with an embedded fallback and refreshable cache. Convert catalog entries to
launch profiles and support project-local custom tools so provider additions do
not require core-code branches throughout Liza.

## Consequences

- New providers can be supported declaratively with less cross-cutting change.
- Liza owns catalog schema, validation, cache refresh, and resolution behavior.
- Catalog correctness becomes an operational dependency for provider launch and
  setup.

## Alternatives Considered

No formal alternatives were considered beyond replacing the hardcoded baseline.

---

Reconstructed from commit 61adbb10 (2026-07-05). User context confirmed
2026-07-16.

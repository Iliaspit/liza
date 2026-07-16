# ADR-0092: Build-Time White-Label Branding

## Status

ACCEPTED

## Context

Omni's Execution Engine is a commercial, broader-scope fork of Liza. Both
repositories remain live, and Omni must regularly merge upstream Liza changes
without breaking its distinct end-user identity.

## Decision

Parameterize end-user-visible branding through validated build-time inputs,
central runtime brand values, and rendered embedded assets. Preserve Go module
and import paths as structural identity. Keep legacy LIZA environment variables
as compatibility aliases and require explicit migration for existing runtime
roots.

## Consequences

- Omni can retain its commercial identity while minimizing upstream merge
  complexity.
- Branding covers binaries, roots, environment variables, generated artifacts,
  hooks, installer/update surfaces, and documentation.
- The system owns macro rendering and broad non-default-brand validation.
- Existing installations are not moved automatically; legacy aliases and mixed
  state require compatibility handling.

## Alternatives Considered

No formal alternatives were recorded. The strategy explicitly rejects
presentation-driven Go import/module rewrites and automatic installation
migration.

---

Reconstructed from commit 905e606e (2026-06-25) and
specs/goals/20260621-white-label.md. User context confirmed 2026-07-16.

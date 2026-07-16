# ADR-0086: Local Support Toolchain

## Status

ACCEPTED

## Context

New users often skipped Liza setup because it was complex. Existing users could
also miss complementary tools added after their initial setup.

## Decision

Provide a local toolchain command group with selectable lean, balanced, and
full profiles; explicit include/exclude overrides; diagnostics; installation;
and shell activation configuration. The toolchain manages local support CLIs,
while MCP and provider capabilities remain user- and project-specific manual
configuration. Selected Go tools can fall back to source installation when an
upstream installer fails.

## Consequences

- New and existing users have a repeatable path to discover and install support
  tools.
- The toolchain owns a catalog, installer paths, and activation behavior.
- Source fallback requires git and Go.
- Users must source generated activation configuration for its environment
  changes to affect subsequent shells.

## Alternatives Considered

No formal alternatives were considered.

---

Reconstructed from commit 88b7eb35 (2026-06-11) and
support-docs/TOOLCHAIN.md. User context confirmed 2026-07-16.

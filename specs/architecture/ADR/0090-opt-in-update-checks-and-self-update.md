# ADR-0090: Opt-In Update Checks and Self-Update

## Status

ACCEPTED

## Context

Installed Liza binaries needed a lower-friction way to discover and apply
newer releases.

## Decision

Add update checks and self-update handling with persisted settings, release
channels, artifact verification, rollback preparation, and process re-exec.
Update handling runs before normal command dispatch; update availability
failures remain non-fatal.

## Consequences

- Users can keep installed binaries current through the CLI.
- Liza owns release discovery, download, checksum, replacement, rollback, and
  re-exec logic.
- Persisted update preferences and updater startup-path complexity are accepted.

## Alternatives Considered

No formal alternatives were considered.

---

Reconstructed from commit e02d2cde (2026-06-12) and updater configuration
documentation. User context confirmed 2026-07-16.

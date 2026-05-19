# Cluster 0061 - Flock-Only Lock Authority

## Commit Set
- `a05eb261` - stop PID-based stale lock recovery

## Source Material
- `liza-run-issues.md` section 1 - Stale Liza State Locks

## Intent Hypothesis
Treat kernel `flock` acquisition as the only authority for Liza state/log lock ownership, making PID metadata diagnostic-only because sandbox PID namespaces made stale-PID cleanup unsafe.

## Architectural Signals
- `internal/filelock` no longer uses legacy PID metadata to decide lock acquisition
- Owner metadata remains best-effort diagnostics
- Timeout means the kernel lock is unavailable, not that a PID file can be trusted
- Troubleshooting and hardening docs remove stale-PID cleanup guidance

## User Context Captured
- Trigger: lock files and PID files could report dead or namespace-translated PIDs while a live command was racing for or holding the kernel lock.
- Rationale: deleting lock files based on PID metadata can race active writers; the kernel lock is the source of truth.
- Tradeoffs: operators lose an automatic stale-PID cleanup path and must treat timeouts as real contention until the kernel lock is available.

## Candidate Decision Date
2026-05-18

## Status
ADR generated: `specs/architecture/ADR/0061-flock-only-lock-authority.md`

## Confidence
0.90 (high)

# Cluster 0062 - Ghost Agent Claim Prevention and Ownership Reconciliation

## Commit Set
- `5a7495a1` - prevent ghost task claims
- `c091414a` - define active review ownership invariant
- `393a620d` - enforce active review ownership
- `b4116a1d` - repair invalid review ownership
- `a9df098b` - repair invalid doer ownership
- `86ad55e0` - repair agent status and stale review claims

## Source Issues
- GitHub issue #73 - live agents can be reported not found with stale current_task claims
- `liza-run-issues.md` sections 5-6, 8-9

## Intent Hypothesis
Make active task ownership structurally consistent across task state and agent rows, reject corrupt/no-process identities from claiming work, and provide validation/repair paths for stale doer and reviewer ownership.

## Architectural Signals
- Claim paths reject missing or corrupt agent identities
- Heartbeat/supervisor paths stop when agent rows disappear
- Recovery/unregister paths release task-side claims even when agent metadata is missing
- Active review ownership invariant documented and enforced
- Active doer ownership validation and conservative repair added
- Process diagnostics and stale-verdict anomaly persistence added

## User Context Captured
- Trigger: no-role/no-PID ghost agents with fresh leases could claim work, dead reviewers held `reviewing_by`, and task-side claims survived agent recovery.
- Rationale: capacity was available but unusable because ownership drift, not raw agent count, blocked progress.
- Tradeoffs: repair paths must avoid destroying meaningful worktrees; live output growth can mean metadata is inconsistent rather than work is stalled.

## Candidate Decision Date
2026-05-18

## Status
ADR generated: `specs/architecture/ADR/0062-ghost-agent-claim-prevention-and-ownership-reconciliation.md`

## Confidence
0.90 (high)

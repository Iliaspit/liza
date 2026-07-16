# Cluster 0091 - Optional Bash-Policy Initialization

## Commit Set
- 4a9bb770 - feat(init): integrate standalone bash-policy

## Reconstructed Context
- Trigger: Claude Code's unit-command model could not safely express common multi-statement commands, and bash-policy activation was easy to miss.
- Alternatives: a classifier may complement AST authorization later, but has token and retraining costs.
- Rationale: Bash AST parsing and leaf-level permissions make multi-statement authorization structurally analyzable.
- Tradeoffs: warning-only setup failures can leave initialization complete without active enforcement.

## Status
ADR generated: specs/architecture/ADR/0091-optional-bash-policy-initialization.md

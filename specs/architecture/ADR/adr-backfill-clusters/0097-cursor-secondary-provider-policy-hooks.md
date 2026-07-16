# Cluster 0097 - Cursor Secondary-Provider Policy Hooks

## Commit Set
- 2b63bbb7 - feat(init): add cursor shell policy hooks

## Reconstructed Context
- Trigger: Cursor needs associated setup for its Claude and GPT secondary providers.
- Alternatives: none formally considered.
- Rationale: Cursor initialization includes the provider setup it depends on.
- Tradeoffs: hook or policy failure can block Cursor shell commands.

## Status
ADR generated: specs/architecture/ADR/0097-cursor-secondary-provider-policy-hooks.md

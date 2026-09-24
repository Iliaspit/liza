# Contracts

Behavioral contracts that discipline AI agents into accountable engineering peers.
Their genesis is tailed at [Turning AI Coding Agents into Senior Engineering Peers](https://medium.com/@tangi.vass/turning-ai-coding-agents-into-senior-engineering-peers-c3d178621c9e).

## Status

Contract split complete. Original `_archive/CONTRACT_FOR_PAIRING_AGENTS_v3.md` preserved for reference.

## Files

| File | Purpose | Status |
|------|---------|--------|
| [CORE.md](CORE.md) | Entry point + universal rules | Complete |
| [PAIRING_MODE.md](PAIRING_MODE.md) | Human-supervised collaboration | Complete |
| [MULTI_AGENT_MODE.md](MULTI_AGENT_MODE.md) | Peer-supervised §BRAND_NAME_TITLE§ system | Complete |
| [CONTRACT_RECOVERY.md](../support-docs/CONTRACT_RECOVERY.md) | On-demand context, RESET, conflict, tool-failure, and batch recovery | Complete |
| [GIT_PROTOCOL.md](../support-docs/GIT_PROTOCOL.md) | On-demand Git and exploratory-state protocol | Complete |
| [TASK_EXECUTION.md](../support-docs/TASK_EXECUTION.md) | On-demand implementation, validation, and completion procedure | Complete |
| [TOOL_ROUTING.md](../support-docs/TOOL_ROUTING.md) | On-demand indexed-search, tool routing, and command reference | Complete |
| [CLAUDE_TOOL_NOTES.md](../support-docs/CLAUDE_TOOL_NOTES.md) | Claude-only tool compatibility notes | Complete |
| [CONTRACT_FOR_PAIRING_AGENTS_v3.md](_archive/CONTRACT_FOR_PAIRING_AGENTS_v3.md) | Original monolithic contract (reference) | Preserved |

## Deployment

See the source-tree
[Getting Started](../GETTING_STARTED.md)
guide for user-facing setup. The installed configuration reference is
[support-docs/CONFIGURATION.md](../support-docs/CONFIGURATION.md).

## Contract Hierarchy

```
~/.claude/CLAUDE.md or ~/.codex/AGENTS.md (symlinks)
    │
    ▼
~/§BRAND_GLOBAL_DIRNAME§/CORE.md (entry point + universal rules)
    │
    └── Mode Selection Gate (auto-detect from bootstrap)
        │
        ├── Default (no §BRAND_NAME_TITLE§ agent) → Read PAIRING_MODE.md
        │                              → Execute Session Initialization
        │                              → (read files, build models, greet)
        │
        └── "You are a §BRAND_NAME_TITLE§ ... agent" → Read MULTI_AGENT_MODE.md
                                       → Execute Session Initialization
                                       → (read role/blackboard, silent)
```

**Key:** No response until Session Initialization from mode contract is complete.

## Content Summary

### CORE.md — Entry point + universal rules
- **Initialization Sequence** (mode selection → read mode contract → execute Session Initialization)
- Rule Priority Architecture (Tiers 0-3)
- **Execution State Machine** (generalized with mode-specific gate semantics)
- Golden Rules 1-14
- Skills Integration
- Protocol References (Debugging, Test, Architecture, Tools)
- Context Management
- Security Protocol
- Task execution, recovery, and Git triggers (full procedures in on-demand support docs)
- Mental Models
- Anti-Gaming Clause

These support protocols are packaged with global support docs but are not part
of the mandatory startup read. Agents read them at the triggers named in CORE
or AGENT_TOOLS. The mode annex, guardrails, and the concise AGENT_TOOLS kernel
remain mandatory at startup.

### PAIRING_MODE.md — Human-supervised collaboration
- Contract Authority (human overrides)
- Gate Semantics (Pairing): approval request → human approves
- Collaboration Philosophy and Modes
- Approval Request Standard
- Skills Integration
- Subagent Mode/Delegation
- Context Management
- Retrospective Protocol
- Magic Phrases
- Session Initialization
- Collaboration Continuity

### MULTI_AGENT_MODE.md — Peer-supervised §BRAND_NAME_TITLE§ system
- Contract Authority (blackboard as source of truth)
- Role Definitions
- **Pre-Execution Checkpoint** (gate artifact for MAM)
- Gate Semantics (Multi-Agent): checkpoint written = gate cleared
- CORE Rule Overrides (Multi-Agent)
- Blackboard Protocol
- Iteration Protocol
- Scope Discipline (spec is law)
- Context Recovery
- Circuit Breaker

**Key Insight:** The Execution State Machine in CORE.md is universal — it forces structured thinking before action. The "gate" is the mechanism:
- **Pairing**: Gate = human approval (agent waits)
- **Multi-Agent**: Gate = checkpoint written to blackboard (self-clearing, but forces the same thinking; Code Reviewer verifies alignment later)

## Related Documents

- [specs/](../specs/) — detailed specifications
- [Implementation Phases](../specs/implementation/phases.md) — extraction timeline

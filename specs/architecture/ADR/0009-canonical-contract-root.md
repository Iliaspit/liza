# 9 - ~/.liza as Canonical Contract Root

> **Current policy:** The canonical-root decision remains active. ADR-0111
> supersedes this ADR's repo-first activation rationale: providers with a
> documented global instruction path now prefer that active global path, with a
> safe repo fallback.

## Context and Problem Statement

The behavioral contract needs to be accessible to agents. Initially, contracts were symlinked into each agent's config directory (e.g., `~/.claude/`). This created maintenance overhead and unclear precedence when repo-level and user-level prompts conflicted.

## Considered Options

1. **Copy contracts to each agent config** — `~/.claude/`, `~/.codex/`, etc.
2. **Symlink everything into agent configs** — Single source, multiple symlinks per agent
3. **Centralize on ~/.liza, minimal symlinks** — One canonical location, agents read from there

## Decision Outcome

Chose **Option 3**: `~/.liza/` is the canonical root. Agent configs get minimal symlinks.

### Rationale

**Prompt compliance hierarchy.** Claude Code's behavior with prompts is asymmetric:
- Repo-level (`<REPO_ROOT>/CLAUDE.md`) is **systematically read on session start**
- User-level (`~/.claude/CLAUDE.md`) is read **only when Claude Code "feels like it"** — not reliable

By creating symlinks at repo root pointing to `~/.liza/`, the contract gets reliable, systematic loading on every session.

**Single source of truth.** All contracts, skills, and supporting files live in `~/.liza/`. No duplication, no drift between agent-specific copies.

**Multi-agent simplicity.** Adding support for a new LLM provider means adding one symlink pattern, not copying the entire contract tree.

### Architecture

```
~/.liza/                          # Canonical root
├── CORE.md                       # Entry point + universal rules
├── PAIRING_MODE.md
├── MULTI_AGENT_MODE.md
├── AGENT_TOOLS.md
├── skills/
│   ├── debugging/SKILL.md
│   ├── testing/SKILL.md
│   └── ...
└── scripts/
    ├── liza-agent.sh
    └── ...

~/.claude/                        # Claude Code config
├── CLAUDE.md → ~/.liza/CORE.md   # Main symlink
├── skills/
│   ├── debugging → ~/.liza/skills/debugging
│   └── ...
└── settings.json                 # Permissions include Read(~/.liza/**)

<REPO_ROOT>/                      # Each project
├── CLAUDE.md → ~/.liza/CORE.md   # Repo-level for strict compliance
└── AGENTS.md → ~/.liza/CORE.md   # Alternative entry point
```

**Key configuration in `~/.claude/settings.json`:**
```json
{
  "additionalDirectories": ["~/.liza"],
  "permissions": {
    "allow": [
      "Read(~/.liza/**)",
      ...
    ]
  }
}
```

### Consequences

**Positive:**
- Repo-level prompts get strict compliance
- Single location to update contracts
- Skills accessible from central location
- Easy to add new LLM providers

**Limitations accepted:**
- Requires symlink setup per project (for strict compliance)
- `additionalDirectories` config needed for Claude Code to read ~/.liza

---
*Reconstructed from commit 535ba75 (2026-01-28)*

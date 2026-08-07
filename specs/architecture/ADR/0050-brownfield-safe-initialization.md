# 50 - Brownfield-Safe Initialization

> **Current policy:** ADR-0111 supersedes the repo-first/global-fallback ordering
> below. The no-overwrite brownfield invariant remains active, while supported
> providers now prefer their documented global instruction paths.

## Context and Problem Statement

Liza requires its behavioral contract to be activated via symlinks at the repo root (`CLAUDE.md`, `AGENTS.md`, `GEMINI.md`). Brownfield projects — existing codebases already using these CLI tools — may already have their own versions of these files. Running `liza init` would either fail or overwrite user-managed contract files.

Additionally, Node.js projects need `node_modules/` available in worktrees, but `liza init` had no way to auto-detect and suggest the appropriate install command. Manual setup of `post_worktree_cmd` was a barrier to adoption, especially for new users.

## Considered Options

1. **Global fallback symlinks with auto-detection** — when repo root files exist and aren't Liza symlinks, place contract symlinks in the CLI's global config directory instead; auto-detect Node.js package managers.

No alternatives were considered. Manual setup would be a barrier to adoption.

## Decision Outcome

Chose **Option 1**: brownfield-safe initialization with global fallback and Node.js auto-detection.

### Architecture

**Three-layer initialization strategy:**

1. **Repo root** (preferred): `CLAUDE.md` → `~/.liza/CORE.md` at repo root — standard behavior for greenfield projects.

2. **Global fallback** (brownfield): When repo root file exists and isn't a Liza symlink, place the contract symlink in the CLI's global config directory:
   - `CLAUDE.md` → `~/.claude/CLAUDE.md`
   - `AGENTS.md` → `~/.codex/AGENTS.md`
   - `GEMINI.md` → `~/.gemini/GEMINI.md`

   All three CLIs read instruction files from their global config directories, so the contract is discovered without touching the project's existing file.

3. **Smart detection**: `isLizaSymlink()` checks whether an existing symlink already points to the Liza contract, avoiding duplicate creation.

**Unified agent flags:**
```bash
liza init --claude --codex --gemini --mistral
```
Explicit provider selection replaces creating all symlinks unconditionally. Prevents unnecessary symlink creation for unused CLIs.

**Node.js auto-detection:**
- Detects `package.json` at repo root
- Suggests install command based on lockfile: `pnpm-lock.yaml` → `pnpm install`, `yarn.lock` → `yarn install`, `bun.lockb` → `bun install`, default → `npm install`
- Stored as `post_worktree_cmd` in project state

**Pipeline force-update:**
- `liza setup --force` updates stale pipeline configurations with backup of existing file

### Rationale

Adoption requires zero-friction initialization. Brownfield projects shouldn't need manual workarounds, and the global fallback mechanism is transparent — the contract activates regardless of where the symlink lives. Node.js auto-detection eliminates a common "why don't my agents have node_modules?" failure mode.

### Consequences

**Positive:**
- Existing projects can adopt Liza without modifying their own contract files
- Global fallback is transparent to agents — contract activates identically
- Node.js projects work out of the box with correct package manager
- `--force` flag enables upgrading pipeline configurations after Liza updates

**Extends:** ADR-0018 (Two-Step Deployment) — refines the init step for brownfield scenarios. ADR-0031 (Configurable Post-Worktree Command) — auto-populates the command for Node.js.

---
*Reconstructed from commits a3d3493..c5042e1 (2026-03-17 to 2026-03-22)*

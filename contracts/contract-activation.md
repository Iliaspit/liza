# Activation of the Contract for Pairing Agents

**WARNING**: Gemini and Mistral are not able to fully comply with the contract.
It's not possible to make them comply strictly with instructions. They are not recommended models for Liza.
Prefer Claude or Codex.

## Two-Layer Settings Architecture

Claude Code unions permissions from **global** and **project** settings:

| Layer | File | Managed by                  | Contains                             |
|-------|------|-----------------------------|--------------------------------------|
| **Project** | `<project>/.claude/settings.json` | `liza init` (automatic) | Liza CLI permissions, skills, git/build commands |
| **Global** | `~/.claude/settings.json` | Optional (user preferences) | Personal MCP tools and settings |

## Central Config

The recommended way to set up `~/.liza/` is:
```bash
liza setup --claude --codex --gemini --mistral          # one-time: writes contracts + skills to ~/.liza/
liza setup --claude --codex --gemini --mistral --force  # overwrite existing (confirmation still asked per file)
```

```bash
liza init --claude --codex --gemini --mistral           # agent-specific contract activation (system prompt symlink, permissions)
```

## Claude

No manual setup required — `liza setup --claude` and `liza init --claude` handle everything.

Verification: Run `claude` and prompt `hello`.

## Codex

`liza init --codex` performs project activation. In `~/.codex/config.toml`, it
adds Liza's Codex baseline, including workspace-write mode, noninteractive
approval, high reasoning effort, network access, and writable roots for the
active project root, its `.git` directory, `/tmp`, and Codex/Liza support/cache
roots. In `<project>/.codex/`, it enables Codex hooks in `config.toml`, writes
`hooks.json`, and deploys hook scripts to `hooks/`. If a config file already
exists, Liza prompts before merging and preserves unrelated settings.

The session-init hook allows the mandatory startup documents to be read through
Codex MCP filesystem read tools, or through simple Bash read commands such as
`cat`, `sed`, `head`, `tail`, and `wc`. More complex Bash commands remain
blocked until initialization is complete.

For full Codex setup, use this shape in `~/.codex/config.toml` (replace
`<USER>` and project paths with local values):

```toml
model = "gpt-5.5"
approval_policy = "never"
sandbox_mode = "workspace-write"
model_reasoning_effort = "high"
personality = "pragmatic"

[permissions.workspace.network]
enabled = true

[sandbox_workspace_write]
network_access = true
exclude_tmpdir_env_var = false
exclude_slash_tmp = false
writable_roots = [
  "/home/<USER>/.codex",
  "/home/<USER>/.liza",
  "/home/<USER>/.npm",
  "/home/<USER>/.pyenv/shims",
  "/home/<USER>/.cache",
  "/tmp",
  # `liza init --codex` manages these project entries for the active project.
  "/path/to/project",
  "/path/to/project/.git",
]
```

Liza launches Codex supervisors with `codex exec -` and relies on the normal
Codex configuration above for sandbox mode, approval policy, network access, and
writable roots. Liza does not pass launch-time permission overrides (`-c
sandbox_mode=...`, `-c approval_policy=...`) or explicit `--add-dir` entries.
Git linked worktrees write their index lock under the main repo metadata path
(`.git/worktrees/<task>/index.lock`), not under the worktree directory itself,
so the active project `.git` directory must be present in `writable_roots`.

MAS supervisors can be pinned to a specific Codex package version with this
durable project config key:

```yaml
config:
  codex_package_version: "0.125.0"
```

For a temporary process-local fallback when that config field is unset, set this
before running `liza agent`:

```bash
export LIZA_CODEX_VERSION=0.125.0
```

With `codex_package_version` or `LIZA_CODEX_VERSION` set, Liza launches
headless Codex agents through
`npm exec --yes --package @openai/codex@<version> -- codex`. The state config
version takes precedence over the environment fallback. This package pinning
path is for headless MAS agents only; interactive `liza agent -i` keeps using
the installed Codex binary and local Codex configuration. Both headless and
interactive `liza agent` provider processes receive the resolved
`LIZA_AGENT_ID`, which SessionStart and guard hooks use to select Multi-Agent
mode instead of Pairing mode.

After editing `~/.codex/config.toml`, restart Codex completely before testing.

## Gemini

Add to ~/.gemini/settings.json:

```json
{
  "context": {
    "includeDirectories": [
      "~/.liza"
    ]
  }
}
```

## Mistral

`liza setup --mistral` and `liza init --mistral` handle contract activation automatically (`system_prompt_id` and prompt symlink).

Manually add the MCP filesystem server to `~/.vibe/config.toml` (replace `mcp_servers = []` with):
```toml
[[mcp_servers]]
name = "filesystem"
transport = "stdio"
command = "npx"
args = ["-y", "@modelcontextprotocol/server-filesystem", "/home/<USER>/.vibe", "/home/<USER>/Workspace", "/home/<USER>/.liza"]
```

Replace `<USER>` with your username in the paths above.

Verification:
- Run `vibe`
- Prompt `Hello. You MUST follow the contract.` ("hello" is not enough for Gemini and Mistral)

## Kimi (with Claude CLI)

Make sure your claude setup is in place.

Create a `kimi` command (adapt to your settings):
```bash
cat > ~/.local/bin/kimi << EOF
#!/bin/bash
source ~/.llm-credentials
ANTHROPIC_BASE_URL=https://api.kimi.com/coding/ ANTHROPIC_API_KEY=$KIMI_API_KEY ANTHROPIC_MODEL='kimi-k2.5' claude "$@"
EOF
```
Then run `kimi`

Kimi uses Claude's config automatically.

## Brownfield Projects

When a project already has its own `CLAUDE.md`, `AGENTS.md`, or `GEMINI.md` at the repo root, `liza init` will not overwrite it. Instead, Liza places its contract symlink in the CLI's global config directory:

| Repo root file | Global fallback |
|---------------|-----------------|
| `CLAUDE.md` | `~/.claude/CLAUDE.md` |
| `AGENTS.md` | `~/.codex/AGENTS.md` |
| `GEMINI.md` | `~/.gemini/GEMINI.md` |

All three CLIs read instruction files from their global config directory, so the contract is still discovered. The project's existing file at the repo root is left untouched.

If both the repo root and the global fallback already have non-Liza files, `liza init` warns and skips — you must remove or rename one manually.

If a Liza symlink already exists at either location, `liza init` reports it and does not create a duplicate.

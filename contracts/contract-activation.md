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
adds Liza's noninteractive workspace permission baseline, including
`sandbox_mode = "workspace-write"`, plus the active project root, its `.git`
directory, `/tmp`, and Codex/Liza support/cache roots under the workspace-write
permission profile. In `<project>/.codex/`, it enables Codex hooks in
`config.toml`, writes `hooks.json`, and deploys hook scripts to `hooks/`. If a
config file already exists, Liza prompts before merging and preserves unrelated
settings. It does not install the full baseline below.

The session-init hook allows the mandatory startup documents to be read through
Codex MCP filesystem read tools, or through simple Bash read commands such as
`cat`, `sed`, `head`, `tail`, and `wc`. More complex Bash commands remain
blocked until initialization is complete.

For full Codex setup, edit `~/.codex/config.toml` (replace `<USER>` with your
username):

```toml
approval_policy = "on-failure"
sandbox_mode = "workspace-write"
default_permissions = "workspace"

[permissions.workspace.filesystem]
":root" = "read"
":tmpdir" = "write"
"/tmp" = "write"
"/home/<USER>/.codex" = "write"
"/home/<USER>/.liza" = "read"
"/home/<USER>/.cache" = "write"
"/home/<USER>/.npm" = "write"

[permissions.workspace.filesystem.":project_roots"]
"." = "write"
".git" = "write"
".agents" = "read"
".codex" = "read"

[permissions.workspace.network]
enabled = true

[sandbox_workspace_write]
network_access = true
exclude_tmpdir_env_var = false
exclude_slash_tmp = false
writable_roots = [
  "/home/<USER>/.codex",
  "/home/<USER>/.cache",
  "/home/<USER>/.npm",
  "/home/<USER>/.pyenv/shims",
  # /tmp, project root, and project .git entries are managed by `liza init --codex`.
]

[mcp_servers.filesystem]
command = "npx"
args = ["-y", "@modelcontextprotocol/server-filesystem", "/home/<USER>/.claude", "/home/<USER>/.codex", "/home/<USER>/Workspace", "/home/<USER>/.liza"]

# Codex agents access Liza via `liza` CLI commands through Bash — no MCP server needed.
```

If Codex agents must run `git add`, `git commit`, `git worktree`, or Liza review submission flows from a repo worktree, the active project root and its `.git` directory must both be listed in `writable_roots`. `liza init --codex` manages these project-specific entries and `/tmp` for scratch files. Liza also passes the task worktree and project `.git` directory as explicit `--add-dir` entries when launching Codex for a claimed task. Without writable access to `.git`, Codex may mount git metadata read-only even when the worktree files themselves are writable, which blocks creation of `index.lock` under `.git/worktrees/...`.

Liza launches Codex supervisors with `codex exec -` plus `-c` overrides for
`sandbox_mode="workspace-write"`, `approval_policy="never"`,
`default_permissions="workspace"`, the workspace filesystem profile above, and
workspace network access. It does not use `--full-auto`; that mode can still
leave linked worktree Git metadata read-only in some Codex sandbox
configurations.

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

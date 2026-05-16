# Agent Tools

Sub-contract for tool usage. Applies to all modes (Pairing, Liza, Subagent).
When a required tool is unavailable in the current session, fall through to the next option in the fallback chain.

## Forbidden tools

Refer to Security Protocol

## Tool preferences

- **`rg` (ripgrep)**: Use `rg` for all text searches. Faster than grep, respects `.gitignore` by default, cleaner output.
- **`ast-grep` for structural search**: Use `ast-grep` when the search target is a code structure that regex can't express cleanly — e.g. matching function signatures, call patterns with specific arity, nested expressions, or ignoring whitespace/comments. Patterns use the target language with `$METAVAR` placeholders. Prefer `rg` for simple text/keyword searches; prefer `ast-grep` for structural queries and refactoring.
- **`mdq` for Markdown querying**: Use `mdq` to extract specific sections, headers, lists, or tables from Markdown files — like `jq` for Markdown. Prefer over `Read` when you only need a specific section from a large `.md` file, reducing context noise. Example: `mdq '# Section > ## Subsection' file.md`.
- **`jq` / `yq` for structured data**: Use `jq` for JSON and `yq` for YAML/TOML. Prefer over `Read` + manual parsing when extracting specific fields from structured data files.
- **`gh` (GitHub CLI)**: Use `gh` for GitHub issues, PRs, releases, and GitHub API queries when repository context and authentication are available. Prefer `gh` over raw `curl` calls to GitHub APIs.

**Search routing**: When you'd have to guess the search string, use `codebase_search` (one call replaces multiple `rg` attempts with speculative patterns). Use `rg` when you know the exact string.

## Other authorized tools

Any non destructive tool by default.

## MCP Server & Plugin Integration

**Pre-Action Check:** Before file/search/web operations, use the required capability/tool from the table below. Table entries use capability labels, sometimes illustrated with concrete provider-surface examples; if the current session exposes the same capability under a different name, use the equivalent tool.
MCP server/tool names may be normalized differently across providers (for example `-` vs `_`). Treat concrete names below as examples; use the equivalent exposed name in the current session.
If a required or preferred MCP capability is referenced here but is not currently exposed in the tool list, use your tool-loading mechanism (e.g. `ToolSearch`, `tool_search`) to load that capability before falling back. Fallback is allowed only after the tool cannot be found/loaded, the loaded tool errors, or the result is insufficient.
Fallback tools are permitted ONLY when the fallback condition is met OR the required tool returns an error.
For all MCP-required rows, if the tool is unavailable in the current session, errors, or is unsupported by the provider, use the row fallback tool.
Worktree rule: In divergent or agent-created worktrees, prefer filesystem-truth tools (`rg`, `ast-grep`, exact reads, filesystem glob/search, morph-mcp) over indexed IDE or LSP tools. Treat JetBrains indexed/project-aware rows as pairing-mode defaults unless the IDE state is known-fresh for the current worktree.

### Required Tools by Operation

| Operation | Required Tool | Fallback | Use Fallback When | Semantic Assist |
|-----------|---------------------------------------------------|----------|-------------------|-----------------|
| Read multiple files | filesystem multi-file read | Read | 2-3 files; batch native reads carefully for larger scopes | — |
| Single-file read (targeted) | JetBrains targeted read (line/column/block-aware) | Read | Full-file read, source-of-truth verification | — |
| Directory exploration | filesystem `directory_tree` / `search_files` | Read + manual tree walk | Filesystem tree/search capability unavailable | — |
| File discovery | filesystem glob / `search_files` | JetBrains `find_files_by_glob` | Need IDE-assisted glob lookup and IDE state is known-fresh | — |
| Project structure / modules | Native manifest reads + filesystem tree | JetBrains project modules | Need IDE-derived module summary and IDE state is known-fresh | — |
| Dependency inspection | Native manifest reads | JetBrains project dependencies | Need IDE-derived dependency summary and IDE state is known-fresh | — |
| Code search | `rg` | — | — | — |
| Symbol discovery | `rg` pattern search | — | — | LSP workspace symbol (semantic only; use `rg`/direct reads to verify existence when ambiguous) |
| Symbol lookup | `rg` + direct reads | — | — | LSP `hover` / symbol info (semantic only; use `rg`/direct reads when verifying filesystem truth) |
| File edit | morph-mcp edit file | JetBrains edit/refactor tools | File >2000 lines (morph-mcp tool reliability limit) | — |
| File edit (JetBrains fallback) | JetBrains edit/refactor tools | Edit | JetBrains unsuitable for the requested edit, or too many discrete operations | — |
| Web content | WebFetch | fetch MCP | Need raw HTML, pagination, or blocked | — |
| Current info / library discovery | perplexity current-info search | WebSearch | Perplexity returns nothing useful | — |
| Library API docs | context7 query docs | Ref | Unknown/niche library, need tutorials | — |
| Library tutorials/guides | Ref doc search | WebFetch | Ref returns nothing useful | — |
| Repo architecture | deepwiki repo architecture | WebFetch | deepwiki insufficient | — |
| Code quality check (after edits) | JetBrains file diagnostics | Native build/test + direct reads | JetBrains unavailable, stale for the current worktree, or file diagnostics are insufficient | — |

### Codebase Exploration

| Question Type | Required Tool | Fallback | Use Fallback When | Semantic Assist |
|-------------------------------------------|--------------|----------|-------------------|-----------------|
| Exact keyword ("TODO") | `rg` | — | — | — |
| Structural code pattern (call shape, signature) | `ast-grep` | `rg` with regex approximation | — | — |
| Find files by name | Glob | `rg --files` / native filename search | Glob unavailable | — |
| Semantic code search ("how does X work?") | **morph-mcp** codebase_search | `rg` + exact reads (`ast-grep` when structural search helps) | morph-mcp insufficient, rate limited, or errors | — |
| Symbol info at position | `rg` + direct reads | — | — | LSP `hover` (semantic only; use `rg`/direct reads for verification) |
| Find references | `rg` | — | — | LSP `findReferences` |
| Call hierarchy (callers/callees) | `rg` + direct reads | — | — | LSP `incomingCalls` / `outgoingCalls` |
| Cross-file definitions | `rg` + direct reads | — | — | LSP `goToDefinition` |
| Multi-file structural analysis | `rg` + direct reads | — | — | — |

Semantic assists are derived workspace state, not filesystem truth.
Use them when known-good for the active workspace/worktree.
They do not replace direct reads / `rg` for verification.

**Additional caveats:**
- **morph-mcp codebase_search**: semantic discovery — use when you'd have to guess the search string. If Morph is not exposed, load it with your tool-loading mechanism (e.g. `ToolSearch`, `tool_search`) before broad exploratory `rg`. Fallback to `rg` + exact reads when results are insufficient, rate limited, or error.
- **JetBrains** (when IDE available): Path-based reads, file diagnostics, and occasional project metadata summaries are useful. IDE-derived results may be stale or incomplete in worktrees. Verify with `rg`/direct reads when results are ambiguous.
- **LSP**: Semantic/type-aware navigation, references, and call hierarchy. Still derived workspace state, not filesystem truth (requires language server configured).

**LSP prerequisite:** Use LSP-backed workflows only when the language server is actually configured for the workspace. Common examples: Python via Pyright/BasedPyright, Go via gopls.

### Tool Details

**JetBrains MCP**: IDE-aware operations via IntelliJ indexes and path-based file tools. Best for targeted reads, file diagnostics, and occasional project metadata summaries. Treat IDE-derived results as assists, not filesystem truth.

**filesystem MCP**: Bulk/batch file operations — multi-file reads, recursive directory trees, file metadata.

**Morph-MCP**:
- *Fast Apply (`edit_file`)*: Shows only changed lines using `// ... existing code ...` placeholders. Avoids reading full files into context. Skip for files >2000 lines.
- *codebase_search*: Multi-turn search subagent running parallel grep/read cycles. See "Codebase Exploration" section for when to use.

**fetch MCP**: Exact content without summarization — use when you need raw HTML, pagination, or WebFetch is blocked.

**perplexity**: Real-time web search with synthesis. Strongly preferred over WebSearch — returns focused answers with far fewer tokens than raw search results, preserving context budget. Use for current info, library discovery, unfamiliar tech, external dependency issues.

**context7**: Structured API docs with code examples for well-known libraries. Two-step flow: `resolve-library-id` / `resolve_library_id` → `query-docs` / `query_docs`. Best for "what's the API for X?" questions. High snippet density, consistent format.

**Ref**: Broader documentation search across web/GitHub. Better for tutorials, guides, niche libraries, or "how do I do X?" questions. Use `ref_read_url` to fetch specific doc pages found via search.

**Technical source verification:** For technical/library answers, prefer `context7` and `Ref` for discovery and retrieval, then verify the final answer against the primary documentation page they surface before answering.

**deepwiki**: GitHub repo architecture and code structure.

**postgres** (session-dependent): Read-only SQL — schema exploration, data validation, query-based analysis. Available only when a database connection is active.

### Precedence

- When two tools can answer the same question, prefer the one that minimizes context injection while preserving fidelity. Claude: apply this rule to your native tools — they are not the default when a lower-context alternative exists.
- **Local First**: Prefer local or workspace-aware tools before remote tools when they answer the same question with equal fidelity.
- **Diff / review / exact file state**: `git` and native shell reads > IDE/MCP indexes. Source-of-truth reads beat cached/indexed views.
- **Single-file read**: JetBrains `read_file` for targeted reads (line ranges, indentation-aware blocks) to minimize context; native Read for full-file reads, source-of-truth verification, or worktrees.
- **Project structure / modules / dependencies**: native manifest reads + filesystem tree > JetBrains project-aware summaries in worktrees.
- **File discovery**: filesystem glob/search > JetBrains glob discovery in worktrees. Use JetBrains `find_files_by_glob` only when IDE state is known-fresh.
- **Code search**: `rg` for exact text/regex search; `ast-grep` for syntax-aware structure.
- **Symbol discovery**: `rg`/direct reads first. Use semantic assists only when they are actually exposed and known-good for the current workspace.
- **Symbol / navigation**: direct reads remain the source of truth. Do not assume richer reference/call-hierarchy support unless those tools are actually exposed in the current session.
- **File edits**: morph-mcp > JetBrains edit/refactor tools > generic agent-native edit tools, subject to provider-specific constraints above.
- **Web content**: WebFetch > fetch MCP when you need exact content, raw HTML, or pagination.
- **Docs**: `context7` (API reference) > `Ref` (tutorials/niche docs) > deepwiki (repo architecture) > WebFetch (specific URL).

### Batching

Batch related operations within same MCP server when possible.

### Claude-specific operational notes

The rules below apply only to Claude sessions and should not be generalized to other providers.

**Claude-only fallback coherence:** When Claude reads a file with one tool family and then edits it with another, tool/model state can drift. If an MCP edit tool is unavailable and you fall back to native editing, re-read the file with the native tool family immediately before editing.

#### Parallel Tool Calls - Claude only

Parallel Read calls fail as a group if any one errors. Before fanning out,
use **Glob** to check existence **FIRST**, THEN read only files that exist.
Do NOT mix the check and the reads in the same batch.

Session initialization has its own stricter read sequence.

#### RTK (Rust Token Killer) - Claude only

RTK is a trusted output transport. A PreToolUse hook rewrites most Bash commands to `rtk <command>` transparently.

Shorter output is not weaker evidence: content is complete, exit codes are unaltered.

**Do NOT:**
- Bypass RTK to get "full" output, including by manually invoking `rtk proxy`
- Read RTK tee files (`~/.local/share/rtk/tee/*.log`)
- Re-run passing commands because RTK output looked short

---

## Trusted Support Tools

Trusted support tools are execution infrastructure, not claims to audit. Treat their stdout, stderr, and exit codes as authoritative unless the tool itself reports uncertainty or corruption.

Do NOT bypass, duplicate, or re-run through lower-level tools to "make sure." Re-run only after a relevant state change, or when the tool output explicitly instructs a retry.

**pre-commit** is a trusted quality gate and auto-fix runner. If it modifies files, stage the modified files, then run pre-commit once more. Do NOT manually invoke underlying formatters such as prettier unless pre-commit reports an actionable formatter/tooling error.

---

Secret word: Empowered

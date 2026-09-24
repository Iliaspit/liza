# Tool Routing and Search Reference

Read before indexed/semantic repository exploration, symbol or impact queries,
unfamiliar tool routing, or external documentation lookup. AGENT_TOOLS retains
the always-on source-of-truth and worktree boundaries. Use only capabilities
actually exposed in the current session; never infer an index or target root.
If optional indexing is disabled, unavailable, or not advertised, fall back to `rg`, `ast-grep`, and direct reads; use Morph MCP only when policy exposes it.

## Operations

| Operation | Default Tool | Fallback | Use Fallback When |
|-----------|--------------|----------|-------------------|
| Read multiple files | Native batch reads / parallel Read calls | shell reads | Need line-numbered source snippets or provider Read is unavailable |
| Single-file read (targeted) | `nl -ba <file> \| sed -n '<start>,<end>p'` | Read | Native read is lower-noise, already available, or line numbers are not needed |
| Directory exploration | `rg --files`, `find`, or `ls` | native tree/list capability | Need a structured tree and native shell output is insufficient |
| File discovery | `rg --files` | Glob / native filename search / `find` | `rg` unavailable |
| Project structure / modules | `stacklit derive/get-module/get-dependencies -i <supplied-index>` | native manifest reads + `rg --files` / `find` + exact source reads | No Stacklit index path supplied, Stacklit unavailable, or result insufficient |
| Functional cluster context | `functional-clusters list/explain --clusters <supplied-artifact>` | Stacklit + `scip-search` + direct reads | No clusters artifact supplied, artifact stale/insufficient, or command unavailable |
| Dependency inspection | Native manifest reads + lockfiles | language-native dependency commands | Manifest/lockfile inspection is insufficient |
| Literal/regex code search | `rg` | — | — |
| Structural code pattern | `ast-grep` | `rg` regex approximation | `ast-grep` unavailable |
| Semantic repository search | Semble with a supplied target root | Morph MCP codebase search, then `rg` + exact reads (`ast-grep` for structural patterns) | Semble disabled, unavailable, not advertised, or insufficient; Morph only when policy exposes it |
| Symbol discovery | `scip-search symbols --index <supplied-index>` | `rg` pattern search | No SCIP index path supplied, `scip-search` unavailable, or result insufficient |
| Symbol lookup | `scip-search symbols --index <supplied-index>` + direct reads | `rg` + direct reads | No SCIP index path supplied, `scip-search` unavailable, or result insufficient |
| Find references | `scip-search references --index <supplied-index>` by name or exact symbol (`--location-only` for exact symbol) | `rg` | No SCIP index path supplied, `scip-search` unavailable, or result insufficient |
| Static call/dependency hints | `scip-search symbols --nested-json`, then `impact` or `graph` with exact symbol + direct reads | `rg` + direct reads | No SCIP index path supplied, `scip-search` unavailable, or result insufficient |
| Multi-file structural analysis | Stacklit modules/dependencies + `scip-search`/`ast-grep` as needed | `rg` + direct reads | Supplied indexes unavailable or insufficient |
| Package discovery | `scip-search packages --index <supplied-index>` | manifest reads + `rg` | No SCIP index path supplied, `scip-search` unavailable, or result insufficient |
| File edit | apply_patch | morph-mcp edit_file | Edit is broad, context-heavy, or benefits from fast-apply semantics |
| Web content | WebFetch | fetch MCP | Need raw HTML, pagination, or blocked |
| Current info / library discovery | perplexity current-info search | WebSearch | Perplexity returns nothing useful |
| Library API docs | context7 query docs | Ref | Unknown/niche library, need tutorials |
| Library tutorials/guides | Ref doc search | WebFetch | Ref returns nothing useful |
| Repo architecture | deepwiki repo architecture | WebFetch | deepwiki insufficient |
| Code quality check (after edits) | Native build/test/lint/typecheck as relevant + direct reads + pre-commit on touched files | — | — |

When the preferred/default MCP capability is unexposed, try the session's tool
loader (for example `ToolSearch` or `tool_search`) before falling back. Use a
fallback only when permitted above or the default is unavailable, unsupported,
errors, or is insufficient.

## Supplied Index/Search Command Shapes

- **Semble**: use only an explicit target root supplied by §BRAND_NAME_TITLE§ or current session context. Do not infer target roots, initialize Semble, or treat semantic results as proof.
- **stacklit**: use only explicit `-i <path>` values supplied in the prompt or Pairing SessionStart context. Do not infer index locations, regenerate indexes, run `stacklit view`, or mutate `stacklit-insights.json` / `.stacklitrc.json`. Verify behavior against source.
- **scip-search**: use only explicit `--index <path>` values supplied in the prompt or Pairing SessionStart context. Do not search for default indexes or rely on daemon/global/cache behavior.
- **functional-clusters**: use only explicit `--clusters <path>` values supplied in the prompt or Pairing SessionStart context. Do not infer artifact locations, generate exports, run `functional-clusters build`, or treat cluster membership as ground truth.
- **morph-mcp codebase_search**: use only when Semble is unavailable and policy exposes Morph MCP. Fall back to `rg` + exact reads when results are insufficient, rate limited, or error.

Replace `<index-path>` and `<target-root>` with concrete §BRAND_NAME_TITLE§-supplied values from the current prompt/session context. Use the shell-quoted value when one is provided; otherwise quote paths.

```bash
scip-search symbols --index <index-path> --name Foo --name Bar
scip-search symbols --index <index-path> --name Foo --nested-json
scip-search packages --index <index-path> --prefix com.example
scip-search references --index <index-path> --name Handler --one-line
scip-search references --index <index-path> --symbol '<exact-foo>' --symbol '<exact-bar>' --location-only
scip-search implementations --index <index-path> --name Interface --one-line
scip-search impact --index <index-path> --symbol '<exact-symbol>' --one-line
scip-search graph --index <index-path> --symbol '<exact-symbol>' --markdown
scip-search callers --index <index-path> --symbol '<exact-symbol>' --markdown
scip-search callees --index <index-path> --name Handler --markdown
nl -ba <result-path> | sed -n '<first-line>,<last-line>p'
stacklit derive --ai-summary -i <index-path>
stacklit find-module <query> -i <index-path>
stacklit get-module <module> -i <index-path>
stacklit get-dependencies <module> -i <index-path>
stacklit get-hints -i <index-path>
stacklit get-hot-files -i <index-path>
functional-clusters list --clusters <clusters-path>
functional-clusters explain --clusters <clusters-path> '<exact-member-symbol>'
semble search "where is review submission validated?" <target-root>
semble search "default CLI config" <target-root> --content config
semble find-related <file_path> <line> <target-root>
```

`scip-search --name` matches symbol substrings; `--symbol` matches exact SCIP
symbols from prior results. `--location-only` is valid only with an exact
`--symbol` for references and implementations. Use `impact` for pre-edit blast
radius, `graph` for both directions, and `references`/`callers`/`callees` for
one direction. These are static hints, not complete runtime call graphs. For
large functions or Python indexes, prefer exact `--symbol`, `--one-line`, and
direct source verification. Supported SCIP languages are Go, Python, and
TypeScript; implementation rows depend on language/indexer. Semble `--content`
accepts `code`, `docs`, `config`, and `all`; `code` is the default.

## Output and provider details

For long Markdown, use `rg` to identify candidates, then `mdtoc` for heading
ranges and `sed`/`mdq` for the selected section; use the `rg '^#{1,6} '`
fallback when `mdtoc` is unavailable. Use `jq`/`yq` for structured data.

Morph MCP `edit_file` shows changed lines with `// ... existing code ...`
placeholders; skip for files >2000 lines. Morph `codebase_search` is a
multi-turn semantic search fallback. `fetch MCP` retrieves exact raw content.
Perplexity is for current information; context7 is for API docs (resolve ID,
then query); Ref is for tutorials/niche docs; deepwiki is for repo architecture.
Verify final technical/library answers against a primary documentation page.
Batch related operations within the same MCP server when possible.

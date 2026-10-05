---
name: graphify
description: "Use the shared local AST owner for optional repository dependency, call-path, impact and architecture discovery. Verify graph results against source."
---

# Graphify

The installed skill owns one persistent code-only graph per explicit target
repository. Its scripts are shared tooling; the target needs neither a package
manifest nor its own owner implementation. Graphify is optional. Source and
tests remain authoritative, and CORE Rule 5 governs every factual claim.

Read `~/§BRAND_GLOBAL_DIRNAME§/support-docs/TOOL_ROUTING.md` before use. Invoke
the installed `skills/graphify/scripts/graphify.mjs` with `node`, `--root`, the
canonical absolute repository root, and `status`, `build`, `update`, `query
"<question>"`, or `serve`. Never invoke the upstream CLI directly. Status is
non-mutating. Queries refresh missing or stale graphs through the same owner
gate; `GRAPHIFY_AUTO_REFRESH=0` disables this for frozen candidates.

The owner validates Graphify 0.9.39, invokes native discovery and parser
dispatch in that CLI's interpreter, and reconciles native discovery with Git's
candidate inventory. It copies sources and ignore context to a private
read-only snapshot. Current native AST stamps and graph contributions are
required for supported sources; error-free native no-symbol results are
recorded explicitly. Unsupported, ignored and non-code paths have exclusion
reasons. Scan errors, ambiguous paths, missing parsers, extractor errors,
reported parse recovery and dropped sources block publication. A failed
candidate preserves the previous published graph and freshness record.

`graphify-out/coverage.json` records expected, extracted, excluded and failed
paths, parser version, source digest and structural coverage. Freshness binds
coverage and every published artifact. Old graphs without complete coverage
are not ready. Coverage proves inventory accounting, not complete symbol or
call semantics; truncation and language parser limitations cannot establish
absence or dependency completeness. Confirm useful results in current source.

The owner preserves bounded subprocesses, exclusive per-root locking,
live-source generation rechecks, one retry on source change and staged
publication with freshness written last. It performs no automatic tool
installation, downloads, Git hooks or package mutations. Executable version
checks are compatibility checks, not package provenance guarantees.

Shared HTTP queries require `GRAPHIFY_API_KEY` and use the repository-bound
`liza.graphify.http.v1` protocol. `GRAPHIFY_PORT` selects the port. The normal
server binds all interfaces for explicitly authorized VM use; controllers
must bind loopback for local comparison runs. Never expose plain HTTP to a
public network. Send only non-sensitive questions. Query text may be visible
in process arguments and shell history. Status, health and queries admit
only current complete coverage. Locks live under `.graphify-owner.lock/`
and `.graphify-owner.lock.recovery/` in the explicit root.

Consumers use the prepared endpoint and query client; agents do not create
indexes or start services unless the assigned role explicitly owns them.
If the tool is unavailable or incomplete, report that and use targeted reads.

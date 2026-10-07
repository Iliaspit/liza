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
reasons. Exact UTF-8/NFC identities proven ignored by the pinned native matcher
are accounted with their no-follow file type and metadata, including literal
POSIX backslashes and ignored links. This exception grants no content, target,
resolve or readlink access and never applies to AST sources, copied inputs or
parser context. Unignored unsafe paths and ambiguous identities fail closed.
Scan errors, missing parsers, extractor errors,
reported parse recovery and dropped sources block publication. A failed
candidate preserves the previous published graph and freshness record.
Native skipped files use the pinned `_SKIP_FILES` exact basename set and an
explicit exclusion; this does not prove a native ignore rule or grant path
permission. Renamed ordinary JSON remains eligible for native dispatch.

`graphify-out/coverage.json` (`graphify.coverage.v2`) records expected, extracted,
excluded and failed paths, parser version and `graphify.accounting.v1`: the
complete typed no-follow census, native dispositions, Git candidates, control
digests and copied/context membership. Versioned source-generation and
`graphify.freshness.v3` digests bind this accounting as well as copied bytes.
The root owner paths `graphify-out`, `.graphify-owner.lock` and
`.graphify-owner.lock.recovery` are reserved bookkeeping outside the source
census. Exclusions derive exactly from noneligible census records; transient
owner entries are not source identities or fabricated exclusions. Freshness
binds coverage and every published artifact. Old graphs without complete coverage
are not ready. Coverage proves inventory accounting, not complete symbol or
call semantics; truncation and language parser limitations cannot establish
absence or dependency completeness. Confirm useful results in current source.

The owner preserves bounded subprocesses, exclusive per-root locking,
live-source generation rechecks, one retry on source change and staged
publication with freshness written last. Inventory has a finite 120-second
bound. Inventory bridge transport failures retain the action and safe failure class
(including timeout), without child stderr. It performs no automatic tool
installation, downloads, Git hooks or package mutations. Executable version
checks are compatibility checks, not package provenance guarantees.

Shared HTTP queries require `GRAPHIFY_API_KEY` and use the repository-bound
`graphify.http.v1` protocol. `GRAPHIFY_PORT` selects the port. The normal
server binds all interfaces for explicitly authorized VM use; controllers
must bind loopback for local comparison runs. Never expose plain HTTP to a
public network. Send only non-sensitive questions. Query text may be visible
in process arguments and shell history. Status, health and queries admit
only current complete coverage. Locks live under `.graphify-owner.lock/`
and `.graphify-owner.lock.recovery/` in the explicit root.

Consumers use the prepared endpoint and query client; agents do not create
indexes or start services unless the assigned role explicitly owns them.
If the tool is unavailable or incomplete, report that and use targeted reads.

Native parser resolution inputs are observed through the pinned parser's actual
file reads, copied into the snapshot, and bound into freshness separately from
expected AST contributions. Ancestor or extends context outside the explicit
repository root blocks extraction. Publication holds the target lock through
artifact replacement, source recheck, freshness publication and rollback.

Discovery, eager context observation and the official native extract/update/
cluster-only operations execute under one scoped filesystem observer in the
pinned interpreter. Native ignore/sensitivity/noise algorithms are retained.
Controls are validated before native loaders; unsupported Git pointers,
metadata-dependent links and content-dependent sensitivity that cannot be
decided without a forbidden read fail explicitly. Missing ancestor probes are
distinguished from existing forbidden contents. Runtime imports/resources are
allowed only for that purpose; a parser context under the interpreter prefix
is still forbidden. Interpreter bootstrap metadata is limited to the trusted
`sysconfig` executable-resolution and optional build-marker probes, with
recorded parent/link identities checked before delegation; this authorizes no
content reads. Fresh private inventory cache directories are canonicalized and
validated before native use. Default bridge callers receive the same private
child environment and cleanup as explicit owner calls. Caught I/O violations
remain failures. CLI scratch permission requires one canonical, no-follow,
private current-user-owned directory supplied as `GRAPHIFY_SCRATCH_ROOT`, with
the matching private HOME/XDG/TMPDIR children created by `buildChildEnv`.
Raw ambient prefixes, aliases, non-directories and unowned or nonprivate roots
are rejected; the directory identity is rechecked before cache permission.
This capability does not authorize source or context reads outside its boundary.
Extraction and update use one worker; unobserved parser subprocesses, including
Fortran cpp fallback, block publication. After control preflight, only the exact root-bound
Git inventory and current-root `git rev-parse HEAD` metadata probes are allowed.
Their capture pipes are authorized only while the validated native subprocess
constructor creates and wraps those FIFO descriptors; unrelated regular-file
or pipe descriptors retain the source/context checks. Adapter primitives and
outer scopes are restored after each stage. Add/remove/type/disposition/control
drift is rechecked during
snapshot creation, before publication, at finalization and at later admission.
After all ignore patterns are loaded, preflight shares only the pinned native
matcher's local cache. Reconciliation checks native-ignored membership first
and finds the closest excluded ancestor by path components. Source/context
permission, identity and generation checks remain live and uncached.

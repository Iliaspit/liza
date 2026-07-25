# Coverage Summary — 2026-07-25

**Status:** Historical evidence, not a reproducible current baseline.

**Go-source basis:** commit `ffe89080`; `ca4eb52b` changed no Go files.

**Instrumentation:** `go test -coverpkg=./... ./...`, with coverage blocks deduplicated by block key across the approximately 48 test binaries that each emitted a whole-module profile.

**Retention limitation:** The raw profiles and the deduplication implementation were not retained. This artifact preserves the reviewed aggregate and per-package summary, but the repository cannot regenerate it with one of its own commands. Replace this artifact after the default coverage command produces the same `-coverpkg=./...` basis and retains the summarization method.

## Aggregate

| Method | Coverage | Statements | Limitation |
|--------|---------:|-----------:|------------|
| Deduplicated profile-block arithmetic | 80.7% | 26,178 | Raw profiles and deduplication implementation were not retained. |
| `go tool cover -func` | 82.6% | — | Omits function literals outside a `FuncDecl`, including all 69 Cobra `RunE` bodies. |

Profile-block arithmetic was used for the package summary because it covers the executable blocks omitted by the function report.

## Packages

| Band | Packages |
|------|----------|
| ≥90% | `identity` 100 (48), `roles` 100 (5), `envgate` 96.0, `jsonout` 96.3 (81), `filelock` 95.2 (126), `analysis` 94.9, `secretmask` 94.6, `projectdetect` 92.9, `termutil` 91.7, `semble` 91.3, `errors` 91.1, `pipeline` 90.6 (663), `models` 90.5 (641) |
| 80–90% | `alerts` 89.7, `statevalidate` 89.6 (924), `plugin/acp` 89.2, `paths` 88.5, `procscan` 88.3, `scipsearch` 87.9 (742), `render` 87.7, `codexconfig` 87.5, `brand` 86.6, `db` 86.5, `prompts` 85.6, `ops` 84.3 (6,816), `gitenv` 84.2, `embedded` 84.1 (731), `precommit` 83.3, `providers` 83.1, `testhelpers` 82.5, `tui` 82.0 (972), `git` 82.0, `toolchain` 80.1, `process` 80.0 |
| 70–80% | `commands` 79.7 (3,343), `initcheck` 78.9, `agent` 78.8 (3,162), `pairingindex` 78.0, `statehygiene` 75.2, `worktreeexclude` 74.0, `stacklit` 73.5, `updater` 71.9 (629) |
| <70% | `log` 69.0, `brandrender` 64.9, `functionalclusters` 64.1, **`cmd/liza` 62.4 (2,631)**, `interactive` 35.8 (67), `brandrender/cmd/sync-embedded` 0.0 (6) |

`internal/taskkind` contains no executable statements and therefore has no coverage entry.

## Zero-Coverage Functions

The retained analysis counted 118 of 2,314 measured functions at 0.0%, including 59 exported functions. They clustered in `commands` (19), `ops` (17), `cmd/liza` (15), `updater` (10), and `agent` (10). The count excludes the 69 Cobra `RunE` bodies omitted by the function-reporting tool.

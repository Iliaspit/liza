# Code Plan: Semble Capability Package

Task ID: `architecture-main-1-architecture-0-code-planning-0`

Source artifacts:
- Goal spec: `specs/goals/20260601-integrate-semble.md`
- Architecture reference: `specs/arch-plan/20260601-integrate-semble/20260601-133505-architecture-main-1-architecture-0.md`
- Existing optional-tool references: `internal/stacklit`, `internal/scipsearch`

## Intent

Plan the `internal/semble` package foundation so later lifecycle tasks can call one isolated package boundary for strict Semble activation, fixed prewarm/offline validation command planning, offline readiness execution and caching, bounded diagnostics, target-root safety, default `.sembleignore` contracts, and prompt-safe command metadata without wiring Semble into init, worktree creation, prompts, SessionStart, documentation, durable config, installs, MCP, or remote Git URL indexing.

## Source Basis

Based on:
- `specs/goals/20260601-integrate-semble.md`
- `specs/arch-plan/20260601-integrate-semble/20260601-133505-architecture-main-1-architecture-0.md`
- prior rejected plan `specs/plans/20260601-integrate-semble/20260601-135022-architecture-main-1-architecture-0-code-planning-0.md`
- `GUARDRAILS.md`

No assumptions are required.

## Planned Coding Tasks

### Task 1: Semble Activation and Command Contracts

**desc:** Semble activation and command contracts: create `internal/semble/doc.go`, `internal/semble/semble.go`, and `internal/semble/semble_test.go` with strict `LIZA_ENABLE_SEMBLE` parsing, disabled-mode no-op planning, Semble executable lookup seams, fixed prewarm and offline validation command-plan types, package-owned validation fixture identity, the 30-second validation timeout constant, and the ordered default `.sembleignore` pattern source of truth. Out of scope: subprocess execution, process-local readiness caching, target-root safety decisions, prompt rendering, lifecycle wiring, operator documentation, durable state config, installing Semble/Python/models, Semble MCP, and remote Git URL indexing.

**done_when:** Unit tests in `internal/semble` prove only trimmed case-insensitive `1` and `true` enable Semble; unset, empty, `0`, `false`, and other values are disabled and return no command plans or diagnostics; executable lookup is fakeable and records the resolved executable path when present; prewarm plans exactly `semble search "__liza_semble_prewarm__" <fixture-dir> --top-k 1 --content code` with inherited network environment; offline validation plans add `HF_HUB_OFFLINE=1` to the same fixed search plan; and `DefaultIgnorePatterns()` returns the ordered runtime, generated-index, and credential patterns from the goal spec.

**scope:** In scope: `internal/semble/doc.go`, `internal/semble/semble.go`, and `internal/semble/semble_test.go` for activation parsing, process environment helpers, disabled no-op planning, executable detection seams, command-plan structs, fixture identity constants, timeout constants, default ignore pattern constants, and focused unit tests. Out of scope: command execution, cache mutation, target-root safety validation, prompt metadata construction, lifecycle call-site wiring, init/worktree/prompt/SessionStart integration, README/support-doc updates, durable `state.yaml` fields, automatic installs, Semble MCP, remote Git URL indexing, and `.liza/agent-outputs/`.

**spec_ref:** specs/arch-plan/20260601-integrate-semble/20260601-133505-architecture-main-1-architecture-0.md#scope-1-semble-capability-package

**validation:** `python3 -c 'import json, subprocess, sys; pattern = sys.argv[1]; required = set(sys.argv[2:]); cmd = ["go", "test", "-json", "./internal/semble", "-run", pattern]; p = subprocess.run(cmd, text=True, capture_output=True); print(p.stdout, end=""); print(p.stderr, end="", file=sys.stderr); events = (json.loads(line) for line in p.stdout.splitlines() if line.startswith("{")); ran = {event.get("Test") for event in events if event.get("Action") == "run" and event.get("Test")}; missing = required - ran; print("missing tests: " + ", ".join(sorted(missing)), file=sys.stderr) if missing else None; sys.exit(p.returncode if p.returncode else (1 if missing else 0))' 'TestActivation|TestCommandPlan|TestDefaultIgnorePatterns' TestActivation TestCommandPlan TestDefaultIgnorePatterns`

**depends_on:** none

### Task 2: Semble Offline Validation Execution and Cache

**desc:** Semble offline validation execution and cache: extend `internal/semble` with a fakeable command runner, temporary one-file fixture creation and cleanup, prewarm and offline validation execution from Task 1 command plans, bounded diagnostic classification for missing executable, model-unavailable-offline, and execution-failure cases, preservation of Semble model/cache environment inputs, and a mutex-guarded process-local validation cache keyed by resolved executable path, `SEMBLE_MODEL_NAME`, `HF_HOME`, `XDG_CACHE_HOME`, validation timeout, and fixture identity. Out of scope: target-root safety decisions, prompt rendering, lifecycle wiring, operator documentation, durable state config, installing Semble/Python/models, Semble MCP, and remote Git URL indexing.

**done_when:** Unit tests in `internal/semble` prove prewarm execution creates a temporary directory outside the target root with `prewarm.py` containing `def liza_semble_prewarm(): pass`, treats exit code 0 as success even with no hits, and cleans the fixture; offline validation execution runs the fixed plan with `HF_HUB_OFFLINE=1`; missing executables, offline model/cache failures, and other runner failures produce bounded typed diagnostics without raw unbounded output; and repeated readiness checks hit the process-local cache until the executable path, `SEMBLE_MODEL_NAME`, `HF_HOME`, `XDG_CACHE_HOME`, timeout, or fixture identity changes.

**scope:** In scope: `internal/semble/semble.go` and `internal/semble/semble_test.go` validation execution, runner interfaces, temporary fixture lifecycle, bounded stdout/stderr diagnostic handling, diagnostic classes, cache-key construction, process-local cache synchronization, environment snapshot handling for `SEMBLE_MODEL_NAME`, `HF_HOME`, and `XDG_CACHE_HOME`, and tests with fake runners and temporary directories. Out of scope: target-root `.sembleignore` safety validation, prompt example construction, lifecycle call-site wiring, init/worktree/prompt/SessionStart integration, README/support-doc updates, durable `state.yaml` fields, automatic installs, Semble MCP, remote Git URL indexing, and `.liza/agent-outputs/`.

**spec_ref:** specs/arch-plan/20260601-integrate-semble/20260601-133505-architecture-main-1-architecture-0.md#validation-executor-and-cache

**validation:** `python3 -c 'import json, subprocess, sys; pattern = sys.argv[1]; required = set(sys.argv[2:]); cmd = ["go", "test", "-json", "./internal/semble", "-run", pattern]; p = subprocess.run(cmd, text=True, capture_output=True); print(p.stdout, end=""); print(p.stderr, end="", file=sys.stderr); events = (json.loads(line) for line in p.stdout.splitlines() if line.startswith("{")); ran = {event.get("Test") for event in events if event.get("Action") == "run" and event.get("Test")}; missing = required - ran; print("missing tests: " + ", ".join(sorted(missing)), file=sys.stderr) if missing else None; sys.exit(p.returncode if p.returncode else (1 if missing else 0))' 'TestValidation|TestReadinessCache|TestDiagnostics' TestValidation TestReadinessCache TestDiagnostics`

**depends_on:** Task 1

### Task 3: Semble Target Safety and Prompt Metadata

**desc:** Semble target safety and prompt metadata: extend `internal/semble` with target-root safety validation for task-worktree and project-root Semble contexts, required `.sembleignore` pattern checks using the Task 1 source of truth, generated worktree `.sembleignore` payload planning, prompt-safe context metadata that is emitted only after enabled offline readiness and safe target validation, shell-quoted absolute target-root command examples that include `HF_HUB_OFFLINE=1`, content-mode guidance, and discovery-not-evidence metadata. Out of scope: writing `.sembleignore` files, private Git exclude setup, prompt template wiring, lifecycle wiring, operator documentation, durable state config, installing Semble/Python/models, Semble MCP, and remote Git URL indexing.

**done_when:** Unit tests in `internal/semble` prove project-root safety requires a physical root `.sembleignore` containing every default runtime/generated/credential pattern; task-worktree safety accepts the explicit task worktree root and rejects parent project-root substitution; generated `.sembleignore` payloads are built from the single ordered default pattern list; prompt metadata is omitted when Semble is disabled, unavailable, offline validation fails, or target safety fails; and ready prompt metadata contains no raw diagnostics or cache paths while rendering `HF_HUB_OFFLINE=1 semble search ...` and `HF_HUB_OFFLINE=1 semble find-related ...` examples with shell-quoted absolute target roots plus the one-line `--content` guidance.

**scope:** In scope: `internal/semble/semble.go` and `internal/semble/semble_test.go` target-kind types, absolute target-root normalization, project-root `.sembleignore` required-pattern checks, task-worktree root safety checks, generated ignore payload construction, shell quoting for prompt command examples, prompt-safe metadata structs, discovery-not-proof/content-mode fields, and tests for disabled/unavailable/not-ready/unsafe omission behavior. Out of scope: writing worktree files, mutating Git excludes or `core.excludesFile`, prompt template changes, init/worktree/prompt/SessionStart lifecycle integration, README/support-doc updates, durable `state.yaml` fields, automatic installs, Semble MCP, remote Git URL indexing, and `.liza/agent-outputs/`.

**spec_ref:** specs/arch-plan/20260601-integrate-semble/20260601-133505-architecture-main-1-architecture-0.md#target-safety-and-ignore-contract

**validation:** `python3 -c 'import json, subprocess, sys; pattern = sys.argv[1]; required = set(sys.argv[2:]); cmd = ["go", "test", "-json", "./internal/semble", "-run", pattern]; p = subprocess.run(cmd, text=True, capture_output=True); print(p.stdout, end=""); print(p.stderr, end="", file=sys.stderr); events = (json.loads(line) for line in p.stdout.splitlines() if line.startswith("{")); ran = {event.get("Test") for event in events if event.get("Action") == "run" and event.get("Test")}; missing = required - ran; print("missing tests: " + ", ".join(sorted(missing)), file=sys.stderr) if missing else None; sys.exit(p.returncode if p.returncode else (1 if missing else 0))' 'TestTargetSafety|TestPromptMetadata|TestShellQuotedCommands' TestTargetSafety TestPromptMetadata TestShellQuotedCommands`

**depends_on:** Task 2

## Dependency Graph

Task 1 -> Task 2 -> Task 3

The chain is intentional because all tasks create or evolve the same `internal/semble` package files. Task 2 relies on Task 1 command-plan and fixture contracts. Task 3 relies on Task 1 ignore-pattern contracts and Task 2 readiness results before producing prompt-safe metadata.

## Shared-File Audit

| File/Area | Task(s) | Dependency |
|---|---|---|
| `internal/semble/doc.go` | Task 1 creates the package documentation; later tasks may make package-comment-only mechanical updates if exported API names settle | Task 2 depends on Task 1; Task 3 depends on Task 2 |
| `internal/semble/semble.go` | Task 1 defines activation, command-plan, executable, fixture, timeout, and ignore-pattern contracts; Task 2 adds execution/cache/diagnostics; Task 3 adds target safety and prompt metadata | Task 2 depends on Task 1; Task 3 depends on Task 2 |
| `internal/semble/semble_test.go` | Task 1 adds contract tests; Task 2 adds execution/cache/diagnostic tests; Task 3 adds safety/metadata/rendering tests | Task 2 depends on Task 1; Task 3 depends on Task 2 |
| Lifecycle call sites under `cmd/liza`, `internal/ops`, `internal/prompts`, `internal/agent`, `internal/git`, `internal/embedded`, and documentation | No task | Out of scope for this plan |
| `.liza/agent-outputs/` | No task | Out of scope |

## Spec Compliance Matrix

| # | Requirement | Source | Task(s) | Status |
|---|-------------|--------|---------|--------|
| 1 | Semble activation is strict opt-in through `LIZA_ENABLE_SEMBLE`; only trimmed case-insensitive `1` and `true` are enabled, while unset, empty, `0`, `false`, and all other values are disabled. | Goal spec MVP Scope lines 139-143; Configuration Shape lines 277-281; Architecture Activation Gate lines 57-68 | Task 1 | Covered |
| 2 | Disabled mode must not validate Semble, run Semble, inject prompt metadata, mention Semble commands, detect executables, or surface diagnostics. | Goal spec MVP Scope lines 144-145; Architecture Activation Gate lines 66-68; Interfaces lines 141-145 | Task 1, Task 3 | Covered |
| 3 | The package validates that the Semble CLI is available before prompt-safe metadata can be produced. | Goal spec MVP Scope lines 146-147; Architecture Executable and Command Planning lines 69-82; Data Flow lines 186-188 | Task 1, Task 2, Task 3 | Covered |
| 4 | Prewarm planning uses one temporary supported-code fixture and the exact search command `semble search "__liza_semble_prewarm__" <temp-dir> --top-k 1 --content code` with inherited operator network behavior. | Goal spec Offline and Network Requirements lines 480-493; Architecture Executable and Command Planning lines 77-81; Fixture Manager lines 83-94 | Task 1, Task 2 | Covered |
| 5 | Offline validation uses the same controlled fixture and command shape with `HF_HUB_OFFLINE=1`, treating exit code 0 as offline-ready. | Goal spec MVP Scope lines 153-166; Offline and Network Requirements lines 494-502; Architecture Executable and Command Planning lines 79-81 | Task 1, Task 2 | Covered |
| 6 | Validation fixture identity includes file name, file content, query, top-k, and content mode so cache invalidates when the validation corpus changes. | Architecture Fixture Manager lines 91-93; Validation Executor and Cache lines 103-105 | Task 1, Task 2 | Covered |
| 7 | The package owns a named 30-second validation timeout constant for prewarm/offline validation. | Goal spec Offline and Network Requirements lines 503-505; Architecture Validation Executor and Cache line 106 | Task 1, Task 2 | Covered |
| 8 | Offline-readiness diagnostics classify missing executable, model unavailable offline, and execution failure separately and keep messages bounded. | Goal spec Offline and Network Requirements lines 498-512; Security and Safety Requirements lines 576-580; Architecture Validation Executor and Cache lines 103-109 | Task 2 | Covered |
| 9 | Command output is used only for bounded diagnostics and prompt metadata must not expose raw output, secrets, or cache paths. | Goal spec Security and Safety Requirements lines 576-580; Architecture Prompt Renderer interface lines 163-169; Cross-Cutting Concerns lines 196-200 | Task 2, Task 3 | Covered |
| 10 | Fresh validation results are process-local only and keyed by resolved executable path, `SEMBLE_MODEL_NAME`, cache-relevant `HF_HOME` and `XDG_CACHE_HOME`, validation timeout, and fixture identity. | Goal spec MVP Scope lines 156-162; Offline and Network Requirements lines 515-517; Architecture Validation Executor and Cache lines 103-105 | Task 2 | Covered |
| 11 | Semble failure degrades gracefully by omitting prompt metadata rather than blocking MAS agent spawn. | Goal spec MVP Scope lines 163-164 and 203-205; Behavioral Decisions lines 314-315; Success Criteria lines 588-591 and 607 | Task 2, Task 3 | Covered |
| 12 | The package must not install Semble, Python, models, global tools, durable config, Semble MCP, or remote Git URL indexing, and must not run `semble init` or `semble savings`. | Goal spec Goal lines 81-84; Behavioral Decisions lines 300-335; Non-Goals lines 612-628; Architecture Constraints lines 21-31 | Task 1, Task 2, Task 3 | Covered |
| 13 | Default generated `.sembleignore` patterns must be a single ordered source of truth covering `.liza/`, `.worktrees/`, `stacklit.json`, `*.scip`, env files, credential files, keys/cert stores, and secrets directories. | Goal spec Worktree Safety Requirements lines 409-469; Security and Safety Requirements lines 565-573; Architecture Target Safety and Ignore Contract lines 110-123 | Task 1, Task 3 | Covered |
| 14 | Project-root Semble guidance is safe only when a physical root `.sembleignore` contains every required runtime, generated-index, and credential pattern. | Goal spec SessionStart Requirements lines 365-368; Worktree Safety Requirements lines 416-418; Architecture Target Safety and Ignore Contract lines 118-122 | Task 3 | Covered |
| 15 | Task-worktree target safety must preserve MAS isolation by accepting the intended task worktree root and rejecting parent project-root substitution. | Goal spec MVP Scope lines 172-176; Behavioral Decisions lines 318-325; Worktree Safety Requirements lines 411-415; Architecture Target Safety and Ignore Contract lines 120-122 | Task 3 | Covered |
| 16 | The package plans generated task-worktree `.sembleignore` content but does not write repository/worktree files or mutate Git private excludes in this scope. | Goal spec Worktree Safety Requirements lines 418-430; Architecture Target Safety and Ignore Contract line 122; Filesystem interface lines 155-161 | Task 3 | Covered |
| 17 | Prompt-safe metadata is emitted only when Semble is enabled, the executable is present, offline validation succeeds or is freshly cached, and target safety passes. | Goal spec Required Agent Prompt Contract lines 211-268; Architecture Prompt Context Metadata lines 124-135; Interfaces lines 141-145 | Task 2, Task 3 | Covered |
| 18 | Prompt command examples use `HF_HUB_OFFLINE=1`, `semble search`, `semble find-related`, explicit absolute target-root arguments, shell-quoted target roots, and content-mode guidance. | Goal spec Required Agent Prompt Contract lines 214-268; Offline and Network Requirements lines 496-517; Architecture Prompt Context Metadata lines 124-135 | Task 3 | Covered |
| 19 | Prompt metadata must identify Semble as candidate discovery, not source-of-truth evidence. | Goal spec Required Agent Prompt Contract lines 228-235; Query Routing Requirements lines 537-544; Architecture Prompt Context Metadata lines 132-135 | Task 3 | Covered |
| 20 | The package stays independent from lifecycle packages and does not modify init, worktree creation, prompt templates, SessionStart, operator documentation, durable state config, installs, Semble MCP, or remote Git URL indexing. | Assigned scope; Architecture Semble Capability Package boundaries lines 47-55; Interfaces lines 139-170; Decomposition lines 208-218 | Task 1, Task 2, Task 3 | Covered |
| 21 | The package remains stack-agnostic and avoids hardcoded target-project build/tool assumptions. | GUARDRAILS.md G1.1; Architecture Constraints lines 21-31 | Task 1, Task 2, Task 3 | Covered |
| E2E | e2e test coverage for new behavior | Cross-cutting | N/A: this plan creates an internal package foundation with no lifecycle wiring or spawned-agent prompt behavior; e2e coverage belongs to sibling init/worktree/prompt wiring tasks that call `internal/semble`. | N/A |
| DOC | Documentation updates for changed behavior | Cross-cutting | N/A: operator documentation is explicitly out of this task's scope and covered by sibling Semble Operator Documentation planning (`architecture-main-1-architecture-4`). | N/A |

## Validation Plan

Each coding task should validate its own behavior with focused Go tests plus a single-purpose assertion that the intended test names actually ran. This avoids Go's `[no tests to run]` success case for missing `-run` matches. Each command executes `go test -json`, prints the underlying test output, collects JSON events with `Action == "run"`, and fails if any required top-level test name is absent.

- Task 1: `python3 -c 'import json, subprocess, sys; pattern = sys.argv[1]; required = set(sys.argv[2:]); cmd = ["go", "test", "-json", "./internal/semble", "-run", pattern]; p = subprocess.run(cmd, text=True, capture_output=True); print(p.stdout, end=""); print(p.stderr, end="", file=sys.stderr); events = (json.loads(line) for line in p.stdout.splitlines() if line.startswith("{")); ran = {event.get("Test") for event in events if event.get("Action") == "run" and event.get("Test")}; missing = required - ran; print("missing tests: " + ", ".join(sorted(missing)), file=sys.stderr) if missing else None; sys.exit(p.returncode if p.returncode else (1 if missing else 0))' 'TestActivation|TestCommandPlan|TestDefaultIgnorePatterns' TestActivation TestCommandPlan TestDefaultIgnorePatterns`
- Task 2: `python3 -c 'import json, subprocess, sys; pattern = sys.argv[1]; required = set(sys.argv[2:]); cmd = ["go", "test", "-json", "./internal/semble", "-run", pattern]; p = subprocess.run(cmd, text=True, capture_output=True); print(p.stdout, end=""); print(p.stderr, end="", file=sys.stderr); events = (json.loads(line) for line in p.stdout.splitlines() if line.startswith("{")); ran = {event.get("Test") for event in events if event.get("Action") == "run" and event.get("Test")}; missing = required - ran; print("missing tests: " + ", ".join(sorted(missing)), file=sys.stderr) if missing else None; sys.exit(p.returncode if p.returncode else (1 if missing else 0))' 'TestValidation|TestReadinessCache|TestDiagnostics' TestValidation TestReadinessCache TestDiagnostics`
- Task 3: `python3 -c 'import json, subprocess, sys; pattern = sys.argv[1]; required = set(sys.argv[2:]); cmd = ["go", "test", "-json", "./internal/semble", "-run", pattern]; p = subprocess.run(cmd, text=True, capture_output=True); print(p.stdout, end=""); print(p.stderr, end="", file=sys.stderr); events = (json.loads(line) for line in p.stdout.splitlines() if line.startswith("{")); ran = {event.get("Test") for event in events if event.get("Action") == "run" and event.get("Test")}; missing = required - ran; print("missing tests: " + ", ".join(sorted(missing)), file=sys.stderr) if missing else None; sys.exit(p.returncode if p.returncode else (1 if missing else 0))' 'TestTargetSafety|TestPromptMetadata|TestShellQuotedCommands' TestTargetSafety TestPromptMetadata TestShellQuotedCommands`

The final coding task in the dependency chain should also run the canonical package check `go test ./internal/semble` and pre-commit on the touched `internal/semble` files. If bare `go test` reports stale embedded assets unrelated to `internal/semble`, follow the worktree build-prerequisites lesson and run `make -C /home/tangi/Workspace/liza/.worktrees/architecture-main-1-architecture-0-code-planning-0 sync-embedded` before retrying the same package test.

## Pre-Submit Self-Check

- Task decomposition: three coding tasks, each with one observable implementation intent and colocated tests.
- Shared-file audit: all shared `internal/semble` files are serialized with `depends_on`.
- Scope boundaries: no init wiring, worktree creation wiring, prompt template changes, SessionStart changes, operator documentation, durable config, automatic installs, Semble MCP, remote Git URL indexing, or `.liza/agent-outputs/` changes are planned.
- Cross-references: every responsibility named in the compliance matrix is owned by a task heading above.
- Validation gates: each child validation command asserts that the required Go test names ran, so missing tests cannot pass as `[no tests to run]`.

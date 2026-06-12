# Liza Capsules

Capsules run a named, isolated Liza instance against the current repository.
The repository is mounted as `/workspace`, but `/workspace/.liza` is shadowed by
capsule-owned state so the host project `.liza/` is not read or modified.

## Create

```bash
liza capsule create opencode-lab --preset openai-compatible --runtime docker
liza capsule create opencode-lab --image ghcr.io/your-org/liza-capsule:latest
liza capsule create opencode-lab --models-dev-provider openrouter --api-key-env OPENROUTER_API_KEY --preferred-model openai/gpt-oss-120b
```

This creates capsule metadata under the local capsule store:

```text
~/.local/share/liza/capsules/<repo-fingerprint>/<name>/
```

Important paths:

- `project-liza/` — mounted as `/workspace/.liza`
- `home-liza/` — mounted as `/home/liza/.liza`
- `opencode-config/opencode.json` — capsule-local OpenCode config
- `secrets.env.example` — template for API-key environment variables
- `reports/` — redacted report zips

Copy `secrets.env.example` to `secrets.env` inside the capsule directory and
fill only local secrets. Reports exclude secret/env/auth files.

Use `--models-dev-provider` to derive OpenCode provider and model entries from
the OpenCode-backed `models.dev` catalog. If no provider catalog is requested,
Liza writes a generic OpenAI-compatible preset that can be edited locally.

## Daytona Cloud Capsules

Daytona capsules use Daytona as the managed compute plane. Liza remains the
control plane: it records capsule metadata, writes OpenCode config, provisions a
Daytona sandbox, starts/stops it, executes commands through Daytona toolbox
APIs, and creates redacted reports. Liza does not store Daytona API keys.

Set the Daytona API key in the host environment:

```bash
export DAYTONA_API_KEY=...
```

Prepare an immutable image and register it as a Daytona snapshot:

```bash
docker build -f packaging/capsule/Dockerfile -t ghcr.io/your-org/liza-capsule:20260610 .
docker push ghcr.io/your-org/liza-capsule:20260610

liza capsule snapshot create liza-capsule-20260610 \
  --image ghcr.io/your-org/liza-capsule:20260610 \
  --region us \
  --sandbox-class linux-vm \
  --cpu 2 \
  --memory 4 \
  --disk 20
```

Do not use `:latest` for Daytona snapshots; Daytona rejects mutable latest
tags. Use a dated, git-SHA, or release-version tag.

Then create a cloud capsule from that snapshot:

```bash
liza capsule create cloud-lab \
  --runtime daytona \
  --daytona-target us \
  --daytona-snapshot liza-capsule-20260610 \
  --daytona-cpu 2 \
  --daytona-memory 4 \
  --daytona-disk 20 \
  --daytona-auto-stop 30 \
  --daytona-auto-delete 240
```

Useful Daytona flags:

- `--daytona-api-url` — override the API URL, for self-hosted Daytona.
- `--daytona-target` — target region, for example `us` or `eu`.
- `--daytona-snapshot` — sandbox snapshot/image that contains the Liza capsule
  toolchain.
- `--daytona-auto-stop` — idle auto-stop interval in minutes.
- `--daytona-auto-delete` — auto-delete interval in minutes; `-1` disables.
- `--no-provision` — write local metadata without creating the remote sandbox.

Snapshot flags:

- `liza capsule snapshot create <name>` — create a Daytona snapshot from an
  immutable registry image.
- `--image` — source OCI image, for example `ghcr.io/your-org/liza-capsule:<sha>`.
- `--region` — Daytona region ID where the snapshot should be available.
- `--sandbox-class` — Daytona sandbox class, default `linux-vm`.
- `--entrypoint` — repeatable entrypoint argument, default `sleep`, `infinity`.

Start the sandbox, optionally executing a command:

```bash
liza capsule start cloud-lab
liza capsule start cloud-lab -- liza version
liza capsule start cloud-lab -- liza tui
```

For Daytona `liza ...` commands, Liza prepares `/workspace` as a git root and
syncs a filtered copy of the capsule-owned `.liza` state before execution. This
lets `liza status` and `liza tui` resolve the project root without touching the
host `.liza/`. `liza tui` uses Daytona PTY and must be launched from an
interactive local terminal.

Stop or delete it:

```bash
liza capsule stop cloud-lab
liza capsule delete cloud-lab
liza capsule delete cloud-lab --local-only
```

Cloud capsules do not mount the full local repository like Docker capsules do.
A Daytona capsule currently syncs only capsule `.liza` state for Liza control
commands. Source-code workflows still need a prebuilt snapshot that contains the
desired repo/toolchain, or a future full repo-sync step.

## Platform Model

Container capsules are Linux guests even on macOS or Windows hosts. Liza
therefore installs Linux-compatible guest binaries inside the image and never
tries to execute macOS or Windows host binaries from inside Docker or Podman.

Tooling is resolved per host/guest platform:

- OpenCode is configured capsule-locally with `OPENCODE_CONFIG` and
  `OPENCODE_CONFIG_DIR`.
- Codex and Claude Code use container-native binaries. Host auth/config
  directories are mounted read-only only when present and compatible; API-key
  environment variables are the fallback.
- Native host-binary capsules are reserved for a future backend.
- Daytona capsules are Linux cloud guests. Host Codex/Claude auth directories
  are not mounted into Daytona; use sandbox environment variables or a private,
  pre-authenticated snapshot.

The default capsule image includes the Linux guest context-navigation toolchain:
`stacklit`, `scip-search`, `scip-go`, `scip-typescript`, `scip-python`,
`semble`, `ast-grep`, `rg`, `fd`, `jq`, `yq`, `gh`, and the Go runtime required
by `scip-go`. The image installs only binaries and runtime dependencies. It does
not bake repository indexes, Semble caches, `.liza` state, auth stores, or
secrets. Optional Liza guidance for Stacklit, SCIP, and Semble still follows the
normal activation gates such as `LIZA_ENABLE_STACKLIT`,
`LIZA_ENABLE_SCIP_SEARCH`, and `LIZA_ENABLE_SEMBLE`.

## Start

```bash
docker build -f packaging/capsule/Dockerfile -t liza-capsule:latest .
liza capsule start opencode-lab
liza capsule start opencode-lab -- liza validate
```

The default command is `liza tui`.

## Doctor

```bash
liza capsule doctor opencode-lab
liza capsule doctor opencode-lab --tool codex
```

Doctor checks the runtime binary, virtual `.liza` paths, OpenCode config, and
optional Codex/Claude auth mounts.

## Report

```bash
liza capsule report opencode-lab
```

The zip contains a redacted manifest, capsule `.liza`, and OpenCode config; it
excludes secrets, auth stores, tokens, cache directories, generated reports,
and symlinks.

For project/home `.liza` workspace delivery, `liza send-workspace` delegates to the
external `liza-send-workspace` binary. Automatic delivery at sprint completion is
optional via `config.goal_completion_report_cmd`, for example `liza-send-workspace`.

## Manage

```bash
liza capsule list
liza capsule delete opencode-lab
```

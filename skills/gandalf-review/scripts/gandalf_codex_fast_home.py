#!/usr/bin/env python3
"""Create a task-local Codex home configured for fast Gandalf ACP reviews."""

from __future__ import annotations

import argparse
import json
from pathlib import Path


def config_text(model: str, reasoning_effort: str) -> str:
    return f"""model = "{model}"
approval_policy = "never"
sandbox_mode = "workspace-write"
model_reasoning_effort = "{reasoning_effort}"
personality = "pragmatic"

[permissions.workspace.network]
enabled = true

[sandbox_workspace_write]
network_access = true
exclude_tmpdir_env_var = false
exclude_slash_tmp = false
"""


def command_create(args: argparse.Namespace) -> None:
    codex_home = Path(args.output_dir).expanduser()
    codex_home.mkdir(parents=True, exist_ok=True)
    config_path = codex_home / "config.toml"
    config_path.write_text(config_text(args.model, args.reasoning_effort), encoding="utf-8")

    auth_linked = False
    if not args.no_auth_link:
        auth_source = Path(args.auth_source).expanduser()
        auth_target = codex_home / "auth.json"
        if auth_source.exists() and not auth_target.exists():
            auth_target.symlink_to(auth_source)
            auth_linked = True

    print(
        json.dumps(
            {
                "auth_linked": auth_linked,
                "codex_home": str(codex_home),
                "config_path": str(config_path),
                "env": {
                    "CODEX_HOME": str(codex_home),
                    "CODEX_MODEL": args.model,
                },
                "model": args.model,
                "reasoning_effort": args.reasoning_effort,
            },
            sort_keys=True,
        )
    )


def parser() -> argparse.ArgumentParser:
    base = argparse.ArgumentParser(description=__doc__)
    base.add_argument("--output-dir", required=True, help="task-local CODEX_HOME directory to create")
    base.add_argument("--model", default="gpt-5.5", help="Codex model for the fast reviewer")
    base.add_argument(
        "--reasoning-effort",
        default="minimal",
        help="Codex reasoning effort for fast reviews; Liza config validation accepts minimal",
    )
    base.add_argument("--auth-source", default="~/.codex/auth.json", help="auth file to symlink without reading")
    base.add_argument("--no-auth-link", action="store_true", help="do not symlink an auth file into CODEX_HOME")
    base.set_defaults(func=command_create)
    return base


def main(argv: list[str] | None = None) -> int:
    args = parser().parse_args(argv)
    args.func(args)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

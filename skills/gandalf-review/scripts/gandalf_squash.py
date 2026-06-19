#!/usr/bin/env python3
"""Squash Gandalf iteration commits into one clean branch commit."""

from __future__ import annotations

import argparse
import json
import subprocess
from pathlib import Path


def git(repo: Path, *args: str) -> str:
    completed = subprocess.run(
        ["git", "-C", str(repo), *args],
        check=True,
        text=True,
        capture_output=True,
    )
    return completed.stdout.strip()


def ensure_clean(repo: Path) -> None:
    status = git(repo, "status", "--porcelain")
    if status:
        raise SystemExit("refusing to squash with uncommitted changes")


def commit_count(repo: Path, base_ref: str) -> int:
    count = git(repo, "rev-list", "--count", f"{base_ref}..HEAD")
    return int(count or "0")


def commit_args(args: argparse.Namespace) -> list[str]:
    if args.message_file:
        return ["commit", "-F", str(Path(args.message_file).expanduser())]
    command = ["commit", "-m", args.message]
    if args.body:
        command.extend(["-m", args.body])
    return command


def command_squash(args: argparse.Namespace) -> None:
    if args.message_file and args.body:
        raise SystemExit("--body can only be used with --message")
    repo = Path(args.repo).expanduser().resolve()
    ensure_clean(repo)
    count = commit_count(repo, args.base_ref)
    head_before = git(repo, "rev-parse", "HEAD")
    merge_base = git(repo, "merge-base", args.base_ref, "HEAD")

    if count <= 1:
        print(
            json.dumps(
                {
                    "base_ref": args.base_ref,
                    "commit_count": count,
                    "head_after": head_before,
                    "head_before": head_before,
                    "no_op": True,
                },
                sort_keys=True,
            )
        )
        return

    if args.dry_run:
        print(
            json.dumps(
                {
                    "base_ref": args.base_ref,
                    "commit_count": count,
                    "head_before": head_before,
                    "merge_base": merge_base,
                    "no_op": False,
                    "would_squash": True,
                },
                sort_keys=True,
            )
        )
        return

    git(repo, "reset", "--soft", merge_base)
    git(repo, *commit_args(args))
    head_after = git(repo, "rev-parse", "HEAD")
    print(
        json.dumps(
            {
                "base_ref": args.base_ref,
                "commit_count": count,
                "head_after": head_after,
                "head_before": head_before,
                "merge_base": merge_base,
                "no_op": False,
            },
            sort_keys=True,
        )
    )


def parser() -> argparse.ArgumentParser:
    base = argparse.ArgumentParser(description=__doc__)
    base.add_argument("--repo", default=".", help="repository to squash")
    base.add_argument("--base-ref", default="main", help="base ref to squash commits onto")
    message = base.add_mutually_exclusive_group(required=True)
    message.add_argument("--message", help="final squashed commit subject")
    message.add_argument("--message-file", help="file containing the full final squashed commit message")
    base.add_argument("--body", help="final squashed commit body; only valid with --message")
    base.add_argument("--dry-run", action="store_true", help="report what would be squashed without rewriting history")
    base.set_defaults(func=command_squash)
    return base


def main(argv: list[str] | None = None) -> int:
    args = parser().parse_args(argv)
    args.func(args)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

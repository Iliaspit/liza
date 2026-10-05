#!/usr/bin/env python3
"""Version-bound bridge to Graphify's own discovery and parser dispatch.

No detection, ignore, or extractor implementation is duplicated here. The
caller runs this file in the pinned CLI's interpreter inside its bounded,
credential-free child environment. Paths and reasons are the public report;
native error text is deliberately omitted.
"""

import contextlib
import hashlib
import importlib.metadata
import io
import json
import os
import subprocess
import sys
import tempfile
import unicodedata
from pathlib import Path
from types import FrameType
from typing import Any

VERSION = "0.9.39"


def native_modules() -> tuple[Any, Any]:
    if importlib.metadata.version("graphifyy") != VERSION:
        raise ValueError("Graphify version mismatch")
    from graphify import detect, extract  # type: ignore[import-not-found]

    return detect, extract


def relative(root: Path, value: str | Path) -> str:
    p = Path(value)
    if not p.is_absolute():
        p = root / p
    rel = p.relative_to(root).as_posix().rstrip("/")
    if not rel or "\\" in rel or "\0" in rel or any(x in ("", ".", "..") for x in rel.split("/")):
        raise ValueError("unsafe path")
    if unicodedata.normalize("NFC", rel) != rel:
        raise ValueError("ambiguous Unicode path")
    # Never copy or classify a symlink through its destination.
    for part in [p, *p.parents]:
        if part == root:
            break
        if part.is_symlink():
            raise ValueError("symlink path")
    return rel


# Observe actual native parser reads instead of duplicating its resolution
# algorithms. The audit hook is active only during native extraction; imports
# from the interpreter/runtime are allowed, source/context reads are root bound.
_RESOLUTION_SCOPE: dict[str, Any] | None = None


def audit_native_read(event: str, args: tuple[Any, ...]) -> None:
    scope = _RESOLUTION_SCOPE
    if scope is None or event != "open" or not isinstance(args[0], (str, bytes, os.PathLike)):
        return
    candidate = Path(os.fsdecode(args[0])).absolute()
    root = scope["root"]
    if candidate.is_relative_to(root):
        if scope["is_sensitive"](candidate):
            scope["failures"].add((candidate.relative_to(root).as_posix(), "native sensitive parser context"))
            raise ValueError("native parser resolution attempted sensitive context")
        if candidate.is_file():
            try:
                scope["reads"].add(relative(root, candidate))
            except ValueError:
                scope["failures"].add(("(resolution)", "unsafe native parser context path"))
                raise
        return
    resolved = candidate.resolve()
    if any(resolved.is_relative_to(prefix) for prefix in scope["runtime"]):
        # Only the interpreter's import machinery may read its runtime here.
        # Native resolver reads (including absolute extends paths beneath the
        # interpreter prefix) have no import-loader frame and remain forbidden.
        frame: FrameType | None = sys._getframe(1)
        while frame is not None:
            if frame.f_code.co_filename.startswith("<frozen importlib."):
                return
            frame = frame.f_back
    scope["failures"].add(("(resolution)", "native parser context outside explicit target root"))
    raise ValueError("native parser resolution escaped target root")


sys.addaudithook(audit_native_read)


def native_context(
    root: Path, expected: list[dict[str, str]], detect: Any, extract: Any
) -> tuple[set[str], list[dict[str, str]]]:
    global _RESOLUTION_SCOPE
    scope: dict[str, Any] = {
        "root": root,
        "reads": set(),
        "failures": set(),
        "is_sensitive": detect._is_sensitive,
        "runtime": {Path(sys.prefix).resolve(), Path(sys.base_prefix).resolve()},
    }
    _RESOLUTION_SCOPE = scope
    try:
        with contextlib.redirect_stdout(io.StringIO()), contextlib.redirect_stderr(io.StringIO()):
            for entry in expected:
                p = root / entry["path"]
                extract._safe_extract_with_xaml_root(extract._get_extractor(p), p, root)
    finally:
        _RESOLUTION_SCOPE = None
    return scope["reads"], [{"path": p, "reason": r} for p, r in sorted(scope["failures"])]


def inventory(root: Path, *, require_git: bool = True) -> dict[str, Any]:
    detect, extract = native_modules()
    with (
        tempfile.TemporaryDirectory(prefix="graphify-detect-") as cache,
        contextlib.redirect_stdout(io.StringIO()),
        contextlib.redirect_stderr(io.StringIO()),
    ):
        scan = detect.detect(
            root,
            follow_symlinks=False,
            google_workspace=False,
            cache_root=Path(cache),
            extra_excludes=["graphify-out/", ".graphify-owner.lock/", ".graphify-owner.lock.recovery/"],
        )
    failures = [{"path": "(scan)", "reason": "native scan error"} for _ in scan.get("walk_errors", [])]
    dispositions = {}
    expected = []
    copies = set()
    for category, paths in scan["files"].items():
        for value in paths:
            rel = relative(root, value)
            if rel in dispositions:
                raise ValueError("duplicate inventory path")
            if category == "code":
                parser = extract._get_extractor(root / rel)
                if parser is not None:
                    expected.append({"path": rel, "parser": parser.__name__})
                    copies.add(rel)
                    dispositions[rel] = "eligible"
                else:
                    dispositions[rel] = "no native AST extractor"
            else:
                dispositions[rel] = f"code-only excludes {category}"
    for field, reason in [
        ("ignored", "native ignore rule"),
        ("pruned_noise_dirs", "native noise directory"),
        ("skipped_sensitive", "native sensitive path"),
        ("unclassified", "native unclassified path"),
    ]:
        for value in scan.get(field, []):
            dispositions.setdefault(relative(root, value), reason)

    # Reconcile the candidate's tracked and untracked Git inventory with native
    # discovery, so ignored tracked code and dropped nested paths remain visible.
    result = subprocess.run(
        ["git", "-C", str(root), "ls-files", "-z", "--cached", "--others", "--exclude-standard"],
        capture_output=True,
        check=require_git,
        timeout=30,
    )
    ignored = detect.ignored_predicate(
        root, extra_excludes=["graphify-out/", ".graphify-owner.lock/", ".graphify-owner.lock.recovery/"]
    )
    for raw in result.stdout.decode("utf-8", errors="strict").split("\0"):
        if not raw:
            continue
        rel = relative(root, root / raw)
        if rel.startswith(("graphify-out/", ".graphify-owner.lock/", ".graphify-owner.lock.recovery/")):
            continue
        p = root / rel
        if not p.is_file():
            failures.append({"path": rel, "reason": "Git candidate is not a regular source file"})
            continue
        if p.name in (".gitignore", ".graphifyignore"):
            copies.add(rel)
        if rel not in dispositions:
            if ignored(p):
                dispositions[rel] = "native ignore rule"
            elif any(
                rel.startswith(prefix + "/")
                for prefix, reason in dispositions.items()
                if reason == "native noise directory"
            ):
                dispositions[rel] = "native noise directory"
            else:
                # Native discovery is authoritative; a graphable path that
                # disappears without a native exclusion is an error.
                category = detect.classify_file(p)
                if category == detect.FileType.CODE:
                    failures.append({"path": rel, "reason": "code path absent from native discovery"})
                dispositions[rel] = "not selected by native discovery"
    # Native ignore context includes Git's local exclude file. It must survive
    # snapshotting even though Git does not list it as a candidate source.
    if (root / ".git/info/exclude").is_file():
        relative(root, root / ".git/info/exclude")
        copies.add(".git/info/exclude")
        dispositions.setdefault(".git/info/exclude", "copied native Git ignore context")
    reads, context_failures = native_context(root, expected, detect, extract)
    context_inputs = sorted(reads - {entry["path"] for entry in expected})
    for rel in context_inputs:
        copies.add(rel)
        dispositions[rel] = "copied native parser resolution context; no AST contribution required"
    failures.extend(context_failures)
    return {
        "contextInputs": context_inputs,
        "expected": sorted(expected, key=lambda x: x["path"]),
        "excluded": [{"path": p, "reason": r} for p, r in sorted(dispositions.items()) if r != "eligible"],
        "failures": failures,
        "sourcePaths": sorted(copies),
    }


def coverage(root: Path) -> dict[str, Any]:
    detect, extract = native_modules()
    report = inventory(root, require_git=False)
    graph = json.loads((root / "graphify-out/graph.json").read_text())
    manifest = json.loads((root / "graphify-out/manifest.json").read_text())
    if not isinstance(manifest, dict) or not isinstance(graph.get("nodes"), list):
        raise ValueError("invalid native artifact shape")
    graph_sources = set()
    for node in graph["nodes"]:
        value = node.get("source_file")
        if value:
            graph_sources.add(relative(root, value))
    extracted = []
    for entry in report["expected"] if not report["failures"] else []:
        rel = entry["path"]
        p = root / rel
        extractor = extract._get_extractor(p)
        with contextlib.redirect_stdout(io.StringIO()), contextlib.redirect_stderr(io.StringIO()):
            result = extract._safe_extract_with_xaml_root(extractor, p, root)
        if result.get("error"):
            report["failures"].append({"path": rel, "reason": "native extractor error or missing parser dependency"})
            continue
        if result.get("parse_errors"):
            report["failures"].append({"path": rel, "reason": "native parser reported recovery/errors"})
            continue
        if not isinstance(result.get("nodes"), list) or not isinstance(result.get("edges"), list):
            report["failures"].append({"path": rel, "reason": "invalid native extraction result"})
            continue
        if not result["nodes"]:
            extracted.append({**entry, "disposition": "error-free native no-symbol result", "nodes": 0})
            continue
        stamp = manifest.get(rel)
        digest = hashlib.md5(p.read_bytes(), usedforsecurity=False).hexdigest()
        if not isinstance(stamp, dict) or stamp.get("ast_hash") != digest or stamp.get("mtime") != p.stat().st_mtime:
            report["failures"].append({"path": rel, "reason": "missing or stale native AST stamp"})
        elif rel not in graph_sources:
            report["failures"].append(
                {"path": rel, "reason": "native eligible source has no published graph contribution"}
            )
        else:
            extracted.append(
                {**entry, "disposition": "current AST stamp and graph contribution", "nodes": len(result["nodes"])}
            )
    return {
        "schema": "graphify.coverage.v1",
        "parserVersion": VERSION,
        "contextInputs": report["contextInputs"],
        "expected": report["expected"],
        "extracted": extracted,
        "excluded": report["excluded"],
        "failures": report["failures"],
        "complete": not report["failures"],
        "limitations": (
            "Structural inventory coverage only; native parsers do not prove complete symbol or call semantics. "
            "Reported parser recovery blocks publication."
        ),
    }


def main() -> None:
    root = Path(sys.argv[2])
    if not root.is_absolute() or root.resolve() != root or not root.is_dir():
        raise ValueError("canonical absolute root required")
    os.chdir(root)
    if sys.argv[1] in ("inventory", "snapshot-inventory"):
        report = inventory(root, require_git=sys.argv[1] == "inventory")
    elif sys.argv[1] == "coverage":
        report = coverage(root)
    else:
        raise ValueError("unknown bridge action")
    print(json.dumps(report, ensure_ascii=False, sort_keys=True))


if __name__ == "__main__":
    try:
        main()
    except Exception:
        print("native Graphify inventory/coverage failed", file=sys.stderr)
        sys.exit(1)

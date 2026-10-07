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
import importlib.util
import inspect
import io
import json
import os
import stat
import subprocess
import sys
import tempfile
import unicodedata
from pathlib import Path
from types import CodeType, FrameType
from typing import Any

VERSION = "0.9.39"
sys.dont_write_bytecode = True
ACCOUNTING_SCHEMA = "graphify.accounting.v1"
OWNER_PATHS = {"graphify-out", ".graphify-owner.lock", ".graphify-owner.lock.recovery"}
# These primitives are used only for no-follow inspection, never native dispatch.
_STAT, _LSTAT, _SCANDIR, _READLINK = os.stat, os.lstat, os.scandir, os.readlink
_OPEN = os.open
_PIPE = os.pipe
_POPEN_INIT = subprocess.Popen.__init__
_POPEN_SIGNATURE = inspect.signature(_POPEN_INIT)
_RESOLVE = Path.resolve
_RESOLUTION_SCOPE: Any = None


def literal_relative(root: Path, value: str | Path) -> str:
    p = Path(value)
    if not p.is_absolute():
        p = root / p
    rel = p.relative_to(root).as_posix()
    rel.encode("utf-8", errors="strict")
    if not rel or rel == "." or "\0" in rel or any(x in ("", ".", "..") for x in rel.split("/")):
        raise ValueError("unsafe path identity")
    if unicodedata.normalize("NFC", rel) != rel:
        raise ValueError("ambiguous Unicode path")
    return rel


def nofollow(path: Path) -> os.stat_result:
    """Inspect ancestors first; never obtain metadata through a link."""
    current = Path(path.anchor)
    result = _LSTAT(current)
    for component in path.parts[1:]:
        if not stat.S_ISDIR(result.st_mode):
            raise ValueError("non-directory ancestor")
        current /= component
        result = _LSTAT(current)
        if stat.S_ISLNK(result.st_mode):
            raise ValueError("symlink path")
    return result


def metadata(st: os.stat_result) -> dict[str, Any]:
    kind = "directory" if stat.S_ISDIR(st.st_mode) else "regular" if stat.S_ISREG(st.st_mode) else "symlink" if stat.S_ISLNK(st.st_mode) else "unsupported"
    result = {"type": kind, "device": str(st.st_dev), "inode": str(st.st_ino), "mode": stat.S_IMODE(st.st_mode)}
    if kind != "directory":
        result.update(size=str(st.st_size), mtimeNs=str(st.st_mtime_ns), ctimeNs=str(st.st_ctime_ns))
    return result


def bootstrap_metadata() -> tuple[dict[Path, Any], dict[Path, str], set[str], set[Path]]:
    """Observe only the interpreter's declared sysconfig bootstrap identities."""
    identities: dict[Path, Any] = {}
    links: dict[Path, str] = {}
    saved = os.lstat, os.readlink
    def observed_lstat(value: Any, *args: Any, **kwargs: Any) -> Any:
        p = Path(os.path.abspath(os.fsdecode(value)))
        if len(identities) >= 512:
            raise ValueError("bounded interpreter metadata exhausted")
        try:
            result = _LSTAT(value, *args, **kwargs)
        except FileNotFoundError:
            identities[p] = None
            raise
        identities[p] = metadata(result)
        return result
    def observed_readlink(value: Any, *args: Any, **kwargs: Any) -> Any:
        result = _READLINK(value, *args, **kwargs)
        links[Path(os.path.abspath(os.fsdecode(value)))] = os.fsdecode(result)
        return result
    os.lstat, os.readlink = observed_lstat, observed_readlink
    try:
        executable = _RESOLVE(Path(sys.executable), strict=True)
        projects = {executable.parent}
        if getattr(sys, "_home", None):
            projects.add(Path(sys._home))
        for project in projects:
            for name in ("Setup", "Setup.local"):
                probe = project / "Modules" / name
                try:
                    observed_lstat(probe)
                except FileNotFoundError:
                    pass
                _RESOLVE(probe)
    finally:
        os.lstat, os.readlink = saved
    origin = importlib.util.find_spec("sysconfig").origin
    resolved_origin = _RESOLVE(Path(origin), strict=True)
    if not resolved_origin.is_relative_to(_RESOLVE(Path(sys.base_prefix), strict=True)):
        raise ValueError("untrusted interpreter bootstrap helper")
    return identities, links, {str(origin), str(resolved_origin)}, projects


_BOOTSTRAP_IDENTITIES, _BOOTSTRAP_LINKS, _SYSCONFIG_SOURCES, _BOOTSTRAP_PROJECTS = bootstrap_metadata()
_BOOTSTRAP_EXECUTABLE = sys.executable
_BOOTSTRAP_HOME = getattr(sys, "_home", None)


def private_scratch_identity(value: Path) -> dict[str, Any]:
    """A caller supplies one canonical private directory, never ambient prefixes."""
    p = Path(value)
    if not p.is_absolute() or p == Path(p.anchor) or ".." in p.parts:
        raise ValueError("unsafe native scratch root")
    st = nofollow(p)
    if (not stat.S_ISDIR(st.st_mode) or st.st_uid != os.getuid()
            or stat.S_IMODE(st.st_mode) != 0o700 or _RESOLVE(p, strict=True) != p):
        raise ValueError("unsafe native scratch root")
    return metadata(st)


def cli_scratch_root() -> Path:
    raw = os.environ.get("GRAPHIFY_SCRATCH_ROOT", "")
    root = Path(raw)
    if not raw or str(root) != raw:
        raise ValueError("canonical private native CLI scratch required")
    private_scratch_identity(root)
    for name, suffix in [("HOME", "home"), ("XDG_CONFIG_HOME", "xdg-config"), ("XDG_CACHE_HOME", "xdg-cache"),
                         ("XDG_DATA_HOME", "xdg-data"), ("TMPDIR", "tmp")]:
        expected = root / suffix
        if os.environ.get(name) != str(expected):
            raise ValueError("unsafe native CLI scratch environment")
        private_scratch_identity(expected)
    for name in ("TMP", "TEMP"):
        if name in os.environ and os.environ[name] != str(root / "tmp"):
            raise ValueError("unsafe native CLI scratch environment")
    return root


class MetadataPath(type(Path())):
    """Native ignore/sensitivity helpers receive genuine no-follow flags."""

    def is_dir(self, *args: Any, **kwargs: Any) -> bool:
        scope = _RESOLUTION_SCOPE
        rel = literal_relative(scope.root, self) if self != scope.root else ""
        entry = scope.census.get(rel)
        if entry and entry["type"] == "symlink":
            scope.deny(rel, "native directory rule requires unsupported link metadata")
        return self == scope.root or bool(entry and entry["type"] == "directory")


class GuardedEntry:
    def __init__(self, entry: Any, scope: Any, parent: Path):
        self.entry, self.scope = entry, scope
        self.name, self.path = entry.name, str(parent / entry.name)

    def __fspath__(self) -> str:
        return self.path

    def stat(self, *, follow_symlinks: bool = True) -> os.stat_result:
        self.scope.check(self.path)
        return self.entry.stat(follow_symlinks=follow_symlinks)

    def is_dir(self, *, follow_symlinks: bool = True) -> bool:
        return stat.S_ISDIR(self.stat(follow_symlinks=follow_symlinks).st_mode)

    def is_file(self, *, follow_symlinks: bool = True) -> bool:
        return stat.S_ISREG(self.stat(follow_symlinks=follow_symlinks).st_mode)

    def is_symlink(self) -> bool:
        self.scope.check(self.path)
        return self.entry.is_symlink()


class GuardedScan:
    def __init__(self, iterator: Any, scope: Any, parent: Path):
        self.iterator, self.scope, self.parent = iterator, scope, parent

    def __iter__(self) -> Any:
        return self

    def __next__(self) -> Any:
        while True:
            entry = next(self.iterator)
            p = self.parent / entry.name
            if p.is_relative_to(self.scope.root) and literal_relative(self.scope.root, p) in self.scope.exceptional:
                continue  # Already proved ignored; no entry target operation.
            return GuardedEntry(entry, self.scope, self.parent)

    def close(self) -> None:
        self.iterator.close()

    def __enter__(self) -> Any:
        return self

    def __exit__(self, *args: Any) -> None:
        self.close()


class NativeScope:
    """One bounded interpreter owns the scoped filesystem capability adapter."""

    def __init__(self, root: Path, detect: Any, *, scratch: tuple[Path, ...] = ()):
        if len(scratch) > 1:
            raise ValueError("one private native scratch identity required")
        self.root, self.detect, self.scratch = root, detect, scratch
        self.scratch_identities = {p: private_scratch_identity(p) for p in scratch}
        self.census: dict[str, dict[str, Any]] = {}
        self.controls: dict[str, str] = {}
        self.exceptional: set[str] = set()
        self.ignored: set[str] = set()
        self.reads: set[str] = set()
        self.failures: set[tuple[str, str]] = set()
        self.violation_count = 0
        self.internal = False
        self.sensitivity = False
        self.git_allowed = False
        self.fds: dict[int, tuple[Path, int, int]] = {}
        self.pending_open: Path | None = None
        self.transport: dict[int, tuple[int, int]] | None = None
        self.runtime = {_RESOLVE(Path(sys.prefix)), _RESOLVE(Path(sys.base_prefix))}

    def deny(self, path: str, reason: str) -> Any:
        self.violation_count += 1
        self.failures.add((path, reason))
        raise ValueError(reason)

    def capture(self) -> None:
        flags = os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW
        nofollow(self.root)
        def walk(fd: int, prefix: str) -> None:
            with _SCANDIR(fd) as entries:
                for entry in sorted(entries, key=lambda e: e.name):
                    rel = literal_relative(self.root, self.root / prefix / entry.name)
                    if rel.split("/", 1)[0] in OWNER_PATHS:
                        continue
                    st = _STAT(entry.name, dir_fd=fd, follow_symlinks=False)
                    self.census[rel] = metadata(st)
                    if entry.name in (".gitignore", ".graphifyignore") and not stat.S_ISREG(st.st_mode):
                        self.deny(rel, "unsafe native ignore control")
                    if len(self.census) > 100_000:
                        raise ValueError("bounded inventory exhausted")
                    if stat.S_ISDIR(st.st_mode) and rel != ".git":
                        child = os.open(entry.name, flags, dir_fd=fd)
                        try:
                            if metadata(os.fstat(child)) != metadata(st):
                                raise ValueError("directory generation changed")
                            walk(child, rel)
                        finally:
                            os.close(child)
        fd = os.open(self.root, flags)
        try:
            walk(fd, "")
        finally:
            os.close(fd)
        dot_git = self.census.get(".git")
        if dot_git and dot_git["type"] != "directory":
            self.deny(".git", "unsupported native Git pointer or link control")
        # Native loaders may consult these even though Git does not list them.
        for control in [".git/info/exclude", ".git/commondir"]:
            try:
                st = nofollow(self.root / control)
            except FileNotFoundError:
                continue
            self.census[control] = metadata(st)
        if ".git/commondir" in self.census:
            self.deny(".git/commondir", "unsupported native Git common-directory control")
        for rel, entry in sorted(self.census.items()):
            if Path(rel).name in (".gitignore", ".graphifyignore") or rel == ".git/info/exclude":
                if entry["type"] != "regular" or "\\" in rel:
                    self.deny(rel, "unsafe native ignore control")
                self.controls[rel] = hashlib.sha256(self.stable_read(rel)).hexdigest()

    def stable_read(self, rel: str) -> bytes:
        p = self.root / rel
        st = nofollow(p)
        if not stat.S_ISREG(st.st_mode):
            self.deny(rel, "non-regular native input")
        parent = _OPEN(self.root, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
        try:
            parts = Path(rel).parts
            for part in parts[:-1]:
                child = _OPEN(part, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW, dir_fd=parent)
                os.close(parent)
                parent = child
            fd = _OPEN(parts[-1], os.O_RDONLY | os.O_NOFOLLOW, dir_fd=parent)
            try:
                before = os.fstat(fd)
                if metadata(before) != self.census[rel]:
                    self.deny(rel, "native input generation changed")
                with os.fdopen(fd, "rb", closefd=False) as stream:
                    data = stream.read(1_048_577)
                if len(data) > 1_048_576:
                    self.deny(rel, "bounded native ignore control exhausted")
                if metadata(os.fstat(fd)) != metadata(before) or metadata(nofollow(p)) != metadata(before):
                    self.deny(rel, "native input generation changed")
                return data
            finally:
                os.close(fd)
        finally:
            os.close(parent)

    def check(self, value: Any, *, dir_fd: Any = None, runtime_metadata: bool = False) -> Path | None:
        if isinstance(value, int):
            if value not in self.fds:
                self.deny("(metadata)", "unobserved native file descriptor metadata")
            p, device, inode = self.fds[value]
            current = os.fstat(value)
            if (current.st_dev, current.st_ino) != (device, inode):
                self.deny("(metadata)", "native descriptor identity changed")
            value = p
        if dir_fd is not None:
            value = self.check(dir_fd) / os.fsdecode(value)
        p = Path(os.path.abspath(os.fsdecode(value)))
        if p.is_relative_to(self.root):
            if p == self.root:
                nofollow(p)
                return p
            rel = literal_relative(self.root, p)
            if "\\" in rel or rel in self.exceptional:
                self.deny(rel, "unsafe native source or context path")
            try:
                nofollow(p)
            except FileNotFoundError:
                return p  # Missing configuration remains missing, not forbidden.
            except ValueError:
                self.deny(rel, "unsafe native source or context path")
            return p
        if runtime_metadata and self.bootstrap_metadata(p):
            return p
        # Native resolvers may probe missing ancestors, but never follow links.
        try:
            nofollow(p)
        except FileNotFoundError:
            return p
        except ValueError:
            self.deny("(resolution)", "unsafe outside-root native metadata")
        return p

    def bootstrap_metadata(self, p: Path) -> bool:
        if p not in _BOOTSTRAP_IDENTITIES or sys.executable != _BOOTSTRAP_EXECUTABLE or getattr(sys, "_home", None) != _BOOTSTRAP_HOME:
            return False
        frame = sys._getframe(1)
        permitted = False
        while frame is not None:
            if frame.f_code.co_filename in _SYSCONFIG_SOURCES:
                permitted = (frame.f_code.co_name == "_safe_realpath" and frame.f_locals.get("path") == _BOOTSTRAP_EXECUTABLE) or (
                    frame.f_code.co_name == "is_python_build" and frame.f_globals.get("_PROJECT_BASE") in {str(x) for x in _BOOTSTRAP_PROJECTS}
                    and p in {x / "Modules" / name for x in _BOOTSTRAP_PROJECTS for name in ("Setup", "Setup.local")})
                break
            if "/graphify/extractors/" in frame.f_code.co_filename:
                break
            frame = frame.f_back
        if not permitted:
            return False
        # Verify parent identities before descendants, including alias links,
        # so a missing optional probe cannot conceal a changed runtime chain.
        for identity, expected in sorted(_BOOTSTRAP_IDENTITIES.items(), key=lambda item: (len(item[0].parts), str(item[0]))):
            try:
                current = metadata(_LSTAT(identity))
            except FileNotFoundError:
                current = None
            if current != expected:
                self.deny("(runtime)", "interpreter bootstrap metadata identity changed")
        return True

    def sensitive(self, p: Path) -> bool:
        previous, self.sensitivity = self.sensitivity, True
        before = self.violation_count
        try:
            result = bool(self.detect._is_sensitive(MetadataPath(p)))
            if self.violation_count != before:
                self.deny("(sensitivity)", "native sensitivity requires forbidden pre-permission content read")
            return result
        finally:
            self.sensitivity = previous

    def check_process(self, executable: Any, command: Any, cwd: Any) -> None:
        """Only the pinned native Git inventory and current-root HEAD probe."""
        inventory = ["git", "-C", str(self.root), "ls-files", "-z", "--cached", "--others", "--exclude-standard"]
        if not self.git_allowed or executable != "git" or not isinstance(command, (list, tuple)):
            self.deny("(subprocess)", "unobserved native parser subprocess")
        current = Path(os.getcwd()) if cwd is None else Path(os.fsdecode(cwd))
        if list(command) == inventory and (cwd is None or current == self.root):
            return
        if list(command) == ["git", "rev-parse", "HEAD"] and current == self.root:
            nofollow(current)
            return
        self.deny("(subprocess)", "unobserved native parser subprocess")

    def transport_open(self, fd: int, flags: int) -> bool:
        """Recognize only pipes created and wrapped by this approved Popen."""
        if self.transport is None or fd not in self.transport:
            return False
        current = os.fstat(fd)
        if not stat.S_ISFIFO(current.st_mode) or (current.st_dev, current.st_ino) != self.transport[fd]:
            self.deny("(open)", "native transport descriptor identity changed")
        frame = sys._getframe(1)
        while frame is not None and frame.f_code.co_filename == __file__:
            frame = frame.f_back
        if frame is None or frame.f_code.co_filename != subprocess.__file__ or frame.f_code.co_name != "__init__":
            return False
        writing = bool(flags & (os.O_WRONLY | os.O_RDWR))
        return (writing and fd == frame.f_locals.get("p2cwrite")) or (
            not writing and fd in (frame.f_locals.get("c2pread"), frame.f_locals.get("errread")))

    def preflight(self) -> None:
        self.capture()
        with self.active():
            # All content-capable control loaders now run behind the adapter.
            patterns = self.detect._load_graphifyignore(self.root)
            for rel, entry in sorted(self.census.items()):
                if entry["type"] == "directory" and rel != ".git":
                    patterns.extend(self.detect._load_dir_own_ignore(self.root / rel))
            for pattern in ["graphify-out/", ".graphify-owner.lock/", ".graphify-owner.lock.recovery/"]:
                patterns.append((self.root, self.detect._parse_gitignore_line(pattern)))
            # The complete pattern list is now frozen. Cache only the native
            # matcher's evaluations for this preflight, never I/O permissions.
            ignore_cache: dict[Path, bool] = {}
            for rel, entry in sorted(self.census.items()):
                p = MetadataPath(self.root / rel)
                if entry["type"] == "unsupported":
                    self.deny(rel, "unsupported native inventory file type")
                if self.detect._is_ignored(p, self.root, patterns, _cache=ignore_cache):
                    self.ignored.add(rel)
                    if "\\" in rel or entry["type"] == "symlink":
                        self.exceptional.add(rel)
                elif "\\" in rel or entry["type"] in ("symlink", "unsupported"):
                    self.deny(rel, "unsafe unignored native inventory path")
        self.git_allowed = True

    @contextlib.contextmanager
    def active(self) -> Any:
        global _RESOLUTION_SCOPE
        previous = _RESOLUTION_SCOPE
        saved = os.stat, os.lstat, os.scandir, os.readlink, Path.resolve, os.open, os.pipe, subprocess.Popen.__init__
        def checked_stat(value: Any, *args: Any, **kwargs: Any) -> Any:
            self.check(value, dir_fd=kwargs.get("dir_fd"), runtime_metadata=True)
            return _STAT(value, *args, **kwargs)
        def checked_lstat(value: Any, *args: Any, **kwargs: Any) -> Any:
            self.check(value, dir_fd=kwargs.get("dir_fd"), runtime_metadata=True)
            return _LSTAT(value, *args, **kwargs)
        def checked_scan(value: Any) -> Any:
            parent = self.check(value)
            return GuardedScan(_SCANDIR(value), self, parent)
        def checked_readlink(value: Any, *args: Any, **kwargs: Any) -> Any:
            p = self.check(value, dir_fd=kwargs.get("dir_fd"), runtime_metadata=True)
            result = _READLINK(value, *args, **kwargs)
            if self.bootstrap_metadata(p) and os.fsdecode(result) != _BOOTSTRAP_LINKS.get(p):
                self.deny("(runtime)", "interpreter bootstrap link identity changed")
            return result
        def checked_resolve(value: Path, *args: Any, **kwargs: Any) -> Any:
            self.check(value, runtime_metadata=True)
            return _RESOLVE(value, *args, **kwargs)
        def checked_open(value: Any, *args: Any, **kwargs: Any) -> Any:
            p = self.check(value, dir_fd=kwargs.get("dir_fd"))
            previous_open, self.pending_open = self.pending_open, p
            try:
                fd = _OPEN(value, *args, **kwargs)
            finally:
                self.pending_open = previous_open
            st = os.fstat(fd)
            self.fds[fd] = p, st.st_dev, st.st_ino
            return fd
        def checked_pipe() -> tuple[int, int]:
            pair = _PIPE()
            frame = sys._getframe(1)
            if self.transport is not None and frame.f_code.co_filename == subprocess.__file__ and frame.f_code.co_name in ("_get_handles", "_execute_child"):
                for fd in pair:
                    st = os.fstat(fd)
                    if not stat.S_ISFIFO(st.st_mode):
                        self.deny("(open)", "non-pipe native transport descriptor")
                    self.transport[fd] = st.st_dev, st.st_ino
            return pair
        def checked_popen(process: Any, *args: Any, **kwargs: Any) -> None:
            bound = _POPEN_SIGNATURE.bind(process, *args, **kwargs)
            bound.apply_defaults()
            options = bound.arguments
            command = options["args"]
            executable = options["executable"]
            if executable is None and isinstance(command, (list, tuple)) and command:
                executable = command[0]
            self.check_process(executable, command, options["cwd"])
            if (options["shell"] or options["preexec_fn"] is not None or options["pass_fds"]
                    or not options["close_fds"] or options["env"] is not None
                    or options["stdin"] is not None or options["stdout"] != subprocess.PIPE or options["stderr"] != subprocess.PIPE):
                self.deny("(subprocess)", "unsupported native Git process transport")
            previous_transport, self.transport = self.transport, {}
            try:
                _POPEN_INIT(process, *args, **kwargs)
            finally:
                self.transport = previous_transport
        _RESOLUTION_SCOPE = self
        os.stat, os.lstat, os.scandir, os.readlink, Path.resolve, os.open = checked_stat, checked_lstat, checked_scan, checked_readlink, checked_resolve, checked_open
        os.pipe, subprocess.Popen.__init__ = checked_pipe, checked_popen
        try:
            yield self
        finally:
            os.stat, os.lstat, os.scandir, os.readlink, Path.resolve, os.open, os.pipe, subprocess.Popen.__init__ = saved
            _RESOLUTION_SCOPE = previous


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
    nofollow(p)
    return rel


# Observe actual native parser reads instead of duplicating its resolution
# algorithms. The audit hook is active only during native extraction; imports
# from the interpreter/runtime are allowed, source/context reads are root bound.
def audit_native_read(event: str, args: tuple[Any, ...]) -> None:
    scope = _RESOLUTION_SCOPE
    if scope is None or scope.internal:
        return
    if event in ("subprocess.Popen", "os.system", "os.fork", "os.posix_spawn", "os.spawn"):
        if event == "subprocess.Popen" and scope.transport is not None:
            scope.check_process(args[0], args[1], args[2])
            return
        scope.deny("(subprocess)", "unobserved native parser subprocess")
    if event != "open":
        return
    if not isinstance(args[0], (int, str, bytes, os.PathLike)):
        scope.deny("(open)", "unobserved native descriptor read")
    if isinstance(args[0], int) and scope.pending_open is None and scope.transport_open(args[0], args[2]):
        return
    candidate = scope.check(scope.pending_open if scope.pending_open is not None else args[0])
    writing = bool(args[2] & (os.O_WRONLY | os.O_RDWR | os.O_CREAT | os.O_TRUNC | os.O_APPEND))
    if scope.sensitivity:
        scope.deny("(sensitivity)", "native sensitivity requires forbidden pre-permission content read")
    if candidate.is_relative_to(scope.root):
        if candidate == scope.root and not writing:
            return
        rel = literal_relative(scope.root, candidate)
        if rel.split("/", 1)[0] == "graphify-out":
            return
        if writing:
            scope.deny(rel, "native write outside graph output")
        if rel in scope.controls:
            if metadata(nofollow(candidate)) != scope.census[rel]:
                scope.deny(rel, "native ignore control generation changed")
            return
        if scope.sensitive(candidate):
            scope.deny(rel, "native sensitive parser context")
        scope.reads.add(relative(scope.root, candidate))
        return
    # Import/resource allowance is purpose-bound, not a runtime-prefix bypass
    # for native resolver reads. The nearest relevant frame decides the purpose.
    frame: FrameType | None = sys._getframe(1)
    resource = False
    while frame is not None:
        filename = frame.f_code.co_filename
        if filename.startswith("<frozen importlib.") or "/importlib/resources/" in filename:
            resource = True
            break
        if "/graphify/extractors/" in filename:
            break
        frame = frame.f_back
    if resource and not writing and any(candidate.is_relative_to(prefix) for prefix in scope.runtime):
        return
    if any(candidate.is_relative_to(prefix) for prefix in scope.scratch):
        for prefix, identity in scope.scratch_identities.items():
            try:
                current = private_scratch_identity(prefix)
            except (ValueError, OSError):
                scope.deny("(scratch)", "native scratch directory identity changed")
            if current != identity:
                scope.deny("(scratch)", "native scratch directory identity changed")
        frame = sys._getframe(1)
        while frame is not None:
            filename = frame.f_code.co_filename
            if "/graphify/extractors/" in filename:
                break
            if "/graphify/cache.py" in filename or filename.endswith("/tempfile.py"):
                return
            frame = frame.f_back
    scope.deny("(resolution)", "native parser context outside explicit target root")


sys.addaudithook(audit_native_read)


def native_context(
    root: Path, expected: list[dict[str, str]], detect: Any, extract: Any
) -> tuple[set[str], list[dict[str, str]]]:
    scope = _RESOLUTION_SCOPE
    if scope is None:
        scope = NativeScope(root, detect)
        scope.preflight()
    with scope.active():
        with contextlib.redirect_stdout(io.StringIO()), contextlib.redirect_stderr(io.StringIO()):
            for entry in expected:
                p = root / entry["path"]
                extract._safe_extract_with_xaml_root(extract._get_extractor(p), p, root)
    return scope.reads, [{"path": p, "reason": r} for p, r in sorted(scope.failures)]


def excluded_ancestor(rel: str, dispositions: dict[str, str]) -> str | None:
    """Closest excluded ancestor, bounded by path depth, not census size."""
    parent = rel.rpartition("/")[0]
    while parent:
        reason = dispositions.get(parent)
        if reason in ("native noise directory", "native ignore rule"):
            return reason
        parent = parent.rpartition("/")[0]
    return None


def inventory(root: Path, *, require_git: bool = True) -> dict[str, Any]:
    detect, extract = native_modules()
    dispositions: dict[str, str] = {}
    expected: list[dict[str, str]] = []
    copies: set[str] = set()
    candidates: set[str] = set()
    context_inputs: list[str] = []
    with tempfile.TemporaryDirectory(prefix="graphify-detect-") as cache:
        # This directory was just created by us, not selected by native source.
        # Resolve an ambient /var-style parent alias before exposing cache input.
        created = _LSTAT(cache)
        if not stat.S_ISDIR(created.st_mode):
            raise ValueError("unsafe private native inventory scratch")
        cache_root = _RESOLVE(Path(cache), strict=True)
        cache_stat = nofollow(cache_root)
        if (metadata(created) != metadata(cache_stat) or not stat.S_ISDIR(cache_stat.st_mode)
                or cache_stat.st_uid != os.getuid() or stat.S_IMODE(cache_stat.st_mode) != 0o700):
            raise ValueError("unsafe private native inventory scratch")
        scope = NativeScope(root, detect, scratch=(cache_root,))
        try:
            scope.preflight()
            copies.update(scope.controls)
            dispositions.update({p: "native ignore rule" for p in scope.exceptional})
            with scope.active(), contextlib.redirect_stdout(io.StringIO()), contextlib.redirect_stderr(io.StringIO()):
                scan = detect.detect(root, follow_symlinks=False, google_workspace=False, cache_root=cache_root,
                                     extra_excludes=[name + "/" for name in sorted(OWNER_PATHS)])
                for _ in scan.get("walk_errors", []):
                    scope.failures.add(("(scan)", "native scan error"))
                for category, paths in scan["files"].items():
                    for value in paths:
                        rel = relative(root, value)
                        if rel in dispositions:
                            scope.deny(rel, "duplicate native inventory disposition")
                        parser = extract._get_extractor(root / rel) if category == "code" else None
                        if parser is not None:
                            expected.append({"path": rel, "parser": parser.__name__})
                            copies.add(rel)
                            dispositions[rel] = "eligible"
                        else:
                            dispositions[rel] = "no native AST extractor" if category == "code" else f"code-only excludes {category}"
                for field, reason in [("ignored", "native ignore rule"), ("pruned_noise_dirs", "native noise directory"),
                                      ("skipped_sensitive", "native sensitive path"), ("unclassified", "native unclassified path")]:
                    for value in scan.get(field, []):
                        rel = literal_relative(root, str(value).rstrip(os.sep))
                        if rel.split("/", 1)[0] in OWNER_PATHS:
                            continue  # Reserved owner bookkeeping is outside the source census.
                        if rel not in scope.census:
                            scope.deny(rel, "native disposition absent from no-follow census")
                        if rel in dispositions and dispositions[rel] != reason:
                            scope.deny(rel, "inconsistent native disposition")
                        dispositions[rel] = reason
                result = subprocess.run(["git", "-C", str(root), "ls-files", "-z", "--cached", "--others", "--exclude-standard"],
                                        capture_output=True, check=require_git, timeout=30)
                candidates = {literal_relative(root, root / raw) for raw in result.stdout.decode("utf-8", errors="strict").split("\0") if raw}
                for rel in candidates:
                    if rel.split("/", 1)[0] not in OWNER_PATHS and rel not in scope.census:
                        scope.failures.add((rel, "Git candidate absent from no-follow census"))
                for rel, entry in sorted(scope.census.items()):
                    if rel in dispositions:
                        continue
                    if rel in scope.ignored:
                        dispositions[rel] = "native ignore rule"
                    elif (reason := excluded_ancestor(rel, dispositions)) is not None:
                        dispositions[rel] = reason
                    elif entry["type"] == "directory":
                        dispositions[rel] = "directory metadata"
                    elif rel in scope.controls:
                        dispositions[rel] = "copied native ignore context"
                    elif Path(rel).name in detect._SKIP_FILES:
                        dispositions[rel] = "native skipped file"
                    else:
                        p = root / rel
                        if scope.sensitive(p):
                            dispositions[rel] = "native sensitive path"
                        else:
                            if detect.classify_file(p) == detect.FileType.CODE:
                                scope.failures.add((rel, "code path absent from native discovery"))
                            dispositions[rel] = "not selected by native discovery"
                scope.reads.clear()  # Discovery word counts are not parser context.
                reads, _ = native_context(root, expected, detect, extract)
                context_inputs = sorted(reads - {entry["path"] for entry in expected})
                for rel in context_inputs:
                    if rel not in scope.census or scope.census[rel]["type"] != "regular":
                        scope.deny(rel, "parser context absent from no-follow census")
                    copies.add(rel)
                    if dispositions[rel] != "eligible":
                        dispositions[rel] = "copied native parser resolution context; no AST contribution required"
        except (ValueError, OSError, subprocess.SubprocessError):
            if not scope.failures:
                scope.failures.add(("(scan)", "native inventory or control preflight failed"))
        for rel in scope.census:
            dispositions.setdefault(rel, "inventory incomplete")
        records = [{"path": p, **entry, "disposition": dispositions[p], "copied": p in copies,
                    "context": p in context_inputs, "nativeIgnored": p in scope.ignored, "gitCandidate": p in candidates}
                   for p, entry in sorted(scope.census.items())]
        accounting = {"schema": ACCOUNTING_SCHEMA, "entries": records,
                      "controls": [{"path": p, "sha256": digest} for p, digest in sorted(scope.controls.items())]}
        return {"accounting": accounting, "contextInputs": context_inputs,
                "expected": sorted(expected, key=lambda x: x["path"]),
                "excluded": [{"path": entry["path"], "reason": entry["disposition"]} for entry in records if entry["disposition"] != "eligible"],
                "failures": [{"path": p, "reason": r} for p, r in sorted(scope.failures)], "sourcePaths": sorted(copies)}


def native_node_identity(node: Any) -> tuple[Any, ...] | None:
    """Exact final native fields; never infer a source from a label or basename."""
    if not isinstance(node, dict) or not isinstance(node.get("id"), str) or not node["id"]:
        return None
    fields = ("source_file", "label", "file_type", "source_location", "type", "confidence", "_origin")
    if any(node.get(key) is not None and not isinstance(node.get(key), str) for key in fields):
        return None
    return (node["id"], *(node.get(key) for key in fields))


@contextlib.contextmanager
def observe_native_stubs(extract: Any, stubs: list[dict[str, Any]], *, categories: dict[int, str] | None = None) -> Any:
    """Observe appends by the exact rescued and nested generic producers."""
    original = extract._emit_rescued_import
    codes = [code for code in extract._extract_generic.__code__.co_consts
             if isinstance(code, CodeType) and code.co_name == "ensure_named_node"]
    if len(codes) != 1:
        raise ValueError("unsupported native generic reference producer")
    producer_code = codes[0]
    previous_profile = sys.getprofile()
    calls: dict[int, tuple[list[dict[str, Any]], int]] = {}
    categories = {} if categories is None else categories

    def observed(result: dict[str, Any], *args: Any, **kwargs: Any) -> None:
        before = len(result.get("nodes", []))
        original(result, *args, **kwargs)
        appended = result.get("nodes", [])[before:]
        stubs.extend(appended)
        categories.update((id(node), "rescued-reference") for node in appended)

    def profile(frame: FrameType, event: str, arg: Any) -> None:
        if frame.f_code is producer_code:
            if event == "call":
                nodes = frame.f_locals["nodes"]
                calls[id(frame)] = (nodes, len(nodes))
            elif event == "return":
                nodes, before = calls.pop(id(frame))
                appended = nodes[before:]
                stubs.extend(appended)
                categories.update((id(node), "generic-reference") for node in appended)
        if previous_profile is not None:
            previous_profile(frame, event, arg)

    extract._emit_rescued_import = observed
    try:
        sys.setprofile(profile)
        yield
    finally:
        sys.setprofile(previous_profile)
        extract._emit_rescued_import = original


def canonical_native_nodes(root: Path, extract: Any, results: dict[Path, dict[str, Any]], *,
                           provenance: dict[int, tuple[str, str]] | None = None) -> list[dict[str, Any]]:
    """Use actual native aggregation, deduplication and graph materialization.

    No historical cache result or cache write participates. The caller retains
    the filesystem observer around every native pass. Optional ephemeral object
    provenance follows selected survivors, never different-source losers.
    """
    from graphify import build, dedup

    provenance = {} if provenance is None else provenance
    saved = extract.load_cached, extract._extract_sequential
    saved_dedup = dedup._merge_missing_attributes, dedup.deduplicate_entities
    copied: list[dict[str, Any]] = []  # Keep intermediate identities alive.
    selected: list[dict[str, Any]] = []

    def fresh_sequential(work: Any, per_file: Any, *args: Any, **kwargs: Any) -> None:
        for index, path in work:
            per_file[index] = results[path]

    def observed_merge(survivor: dict[str, Any], duplicate: dict[str, Any]) -> dict[str, Any]:
        merged = saved_dedup[0](survivor, duplicate)
        copied.append(merged)
        if id(survivor) in provenance and merged.get("source_file") == survivor.get("source_file"):
            provenance[id(merged)] = provenance[id(survivor)]
        return merged

    def observed_dedup(*args: Any, **kwargs: Any) -> Any:
        result = saved_dedup[1](*args, **kwargs)
        selected.extend(result[0])
        return result

    extract.load_cached = lambda *args, **kwargs: None
    extract._extract_sequential = fresh_sequential
    dedup._merge_missing_attributes, dedup.deduplicate_entities = observed_merge, observed_dedup
    try:
        result = extract.extract(list(results), root=root, cache_root=root, parallel=False)
        if not isinstance(result, dict) or not isinstance(result.get("nodes"), list):
            raise ValueError("invalid native aggregation result")
        if any(native_node_identity(node) is None for node in result["nodes"]):
            raise ValueError("invalid native aggregation node")
        graph = build.build([result], dedup=True, dedup_llm_backend=None, root=root)
        survivors = {node["id"]: node for node in selected}
        final = []
        final_provenance = {}
        for node_id, attributes in graph.nodes(data=True):
            node = {"id": node_id, **attributes}
            if native_node_identity(node) is None:
                raise ValueError("invalid final native node")
            final.append(node)
            survivor = survivors.get(node_id)
            if (survivor is not None and id(survivor) in provenance
                    and survivor.get("source_file") == node.get("source_file")):
                final_provenance[id(node)] = provenance[id(survivor)]
        provenance.clear()
        provenance.update(final_provenance)
        return final
    finally:
        dedup._merge_missing_attributes, dedup.deduplicate_entities = saved_dedup
        extract.load_cached, extract._extract_sequential = saved


def coverage(root: Path) -> dict[str, Any]:
    detect, extract = native_modules()
    report = inventory(root, require_git=False)
    failures = report["failures"]
    inventory_safe = not failures
    artifacts: dict[str, Any] = {}
    for name in ("graph", "manifest"):
        try:
            artifacts[name] = json.loads((root / f"graphify-out/{name}.json").read_text())
        except (json.JSONDecodeError, UnicodeError, OSError, RecursionError):
            failures.append({"path": f"({name})", "reason": "invalid native artifact JSON or unreadable artifact"})
            artifacts[name] = {"nodes": []} if name == "graph" else {}
    graph, manifest = artifacts["graph"], artifacts["manifest"]
    if not isinstance(manifest, dict):
        failures.append({"path": "(manifest)", "reason": "invalid native artifact shape"})
        manifest = {}
    if not isinstance(graph, dict) or not isinstance(graph.get("nodes"), list):
        failures.append({"path": "(graph)", "reason": "invalid native artifact shape"})
        graph_nodes = []
    else:
        graph_nodes = graph["nodes"]
    published = []
    for node in graph_nodes:
        if native_node_identity(node) is None:
            failures.append({"path": "(graph)", "reason": "invalid native graph node or source shape"})
        else:
            published.append(node)
    extracted = []
    results: dict[Path, dict[str, Any]] = {}
    stubs: list[dict[str, Any]] = []
    reference_kinds: dict[int, str] = {}
    stamped: set[str] = set()
    admitted: set[str] = set()
    provenance: dict[int, tuple[str, str]] = {}
    scope = NativeScope(root, detect)
    try:
        scope.preflight()
    except (ValueError, OSError):
        failures.append({"path": "(scan)", "reason": "native coverage control preflight failed"})
        inventory_safe = False
    for entry in report["expected"] if inventory_safe else []:
        rel = entry["path"]
        p = root / rel
        before = len(stubs)
        try:
            with scope.active(), observe_native_stubs(extract, stubs, categories=reference_kinds), contextlib.redirect_stdout(io.StringIO()), contextlib.redirect_stderr(io.StringIO()):
                extractor = extract._get_extractor(p)
                result = extract._safe_extract_with_xaml_root(extractor, p, root)
        except (ValueError, OSError):
            failures.append({"path": rel, "reason": "native extractor error or forbidden I/O"})
            continue
        if not isinstance(result, dict):
            failures.append({"path": rel, "reason": "invalid native extraction result"})
            continue
        if result.get("error"):
            report["failures"].append({"path": rel, "reason": "native extractor error or missing parser dependency"})
            continue
        if (not isinstance(result.get("nodes"), list) or not isinstance(result.get("edges"), list)
                or any(native_node_identity(node) is None for node in result["nodes"])):
            report["failures"].append({"path": rel, "reason": "invalid native extraction result"})
            continue
        # Recovered nodes still determine native collision/resolution identities.
        # They never participate in contribution, stamp or extraction admission.
        results[p] = result
        stub_objects = {id(node) for node in stubs[before:]}
        for node in result["nodes"]:
            if id(node) in stub_objects:
                provenance[id(node)] = (reference_kinds[id(node)], "")
            elif node.get("source_file") == str(p):
                provenance[id(node)] = ("recovered" if result.get("parse_errors") else "genuine", rel)
        if result.get("parse_errors"):
            report["failures"].append({"path": rel, "reason": "native parser reported recovery/errors"})
            continue
        if not result["nodes"]:
            extracted.append({**entry, "disposition": "error-free native no-symbol result", "nodes": 0})
            continue
        admitted.add(rel)
        stamp = manifest.get(rel)
        try:
            with scope.active():
                digest = hashlib.md5(p.read_bytes(), usedforsecurity=False).hexdigest()
                mtime = p.stat().st_mtime
        except (ValueError, OSError):
            failures.append({"path": rel, "reason": "native source stamp read failed"})
            continue
        if not isinstance(stamp, dict) or stamp.get("ast_hash") != digest or stamp.get("mtime") != mtime:
            report["failures"].append({"path": rel, "reason": "missing or stale native AST stamp"})
        else:
            stamped.add(rel)
    native_nodes = []
    if results:
        try:
            with scope.active(), contextlib.redirect_stdout(io.StringIO()), contextlib.redirect_stderr(io.StringIO()):
                native_nodes = canonical_native_nodes(root, extract, results, provenance=provenance)
        except (ValueError, OSError, TypeError, KeyError, AttributeError):
            failures.append({"path": "(graph)", "reason": "native identity aggregation failed or forbidden I/O"})
    occurrences: dict[tuple[Any, ...], list[tuple[str, str]]] = {}
    for node in native_nodes:
        occurrences.setdefault(native_node_identity(node), []).append(provenance.get(id(node), ("unowned", "")))
    graph_sources = set()
    expected_paths = {entry["path"] for entry in report["expected"]}
    for node in published:
        identity = native_node_identity(node)
        available = occurrences.get(identity)
        if not available:
            reason = "unrecognized or excess native graph node occurrence" if node.get("source_file") else "unrecognized or excess source-less native graph node occurrence"
            failures.append({"path": "(graph)", "reason": reason})
            continue
        category, owner = available.pop()
        if category in ("rescued-reference", "generic-reference"):
            continue  # Producer-proven metadata: no Path, traversal or contribution.
        value = node.get("source_file")
        if not value:
            failures.append({"path": "(graph)", "reason": "unrecognized source-less native graph node"})
            continue
        if value not in expected_paths:
            failures.append({"path": "(graph)", "reason": "unrecognized native graph source reference"})
            continue
        try:
            with scope.active():
                rel = relative(root, value)
        except (ValueError, OSError):
            failures.append({"path": "(graph)", "reason": "unsafe native graph source reference"})
            continue
        if category == "genuine" and owner == rel and rel in admitted:
            graph_sources.add(rel)
    for entry in report["expected"]:
        rel = entry["path"]
        if rel not in admitted:
            continue
        if rel not in graph_sources:
            report["failures"].append(
                {"path": rel, "reason": "native eligible source has no published graph contribution"}
            )
        elif rel in stamped:
            extracted.append(
                {**entry, "disposition": "current AST stamp and graph contribution", "nodes": len(results[root / rel]["nodes"])}
            )
    report["failures"].extend({"path": p, "reason": r} for p, r in sorted(scope.failures))
    # Both admission paths must retain the authoritative expected inventory order.
    expected_order = {entry["path"]: index for index, entry in enumerate(report["expected"])}
    extracted.sort(key=lambda entry: expected_order[entry["path"]])
    return {
        "schema": "graphify.coverage.v2",
        "accounting": report["accounting"],
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


def native_cli(root: Path, arguments: list[str]) -> None:
    """Run the official pinned CLI, in this observed interpreter, never a worker."""
    detect, _ = native_modules()
    from graphify.__main__ import main as cli_main  # type: ignore[import-not-found]

    if not arguments or arguments[0] not in ("extract", "update", "cluster-only"):
        raise ValueError("unsupported guarded native action")
    if len(arguments) < 2 or Path(arguments[1]) != root:
        raise ValueError("guarded native action requires the explicit root")
    scope = NativeScope(root, detect, scratch=(cli_scratch_root(),))
    scope.preflight()
    previous_args = sys.argv
    previous_workers = os.environ.get("GRAPHIFY_MAX_WORKERS")
    sys.argv = ["graphify", *arguments]
    os.environ["GRAPHIFY_MAX_WORKERS"] = "1"
    try:
        with scope.active():
            cli_main()
    except SystemExit as exc:
        if exc.code not in (None, 0):
            raise
    finally:
        sys.argv = previous_args
        if previous_workers is None:
            os.environ.pop("GRAPHIFY_MAX_WORKERS", None)
        else:
            os.environ["GRAPHIFY_MAX_WORKERS"] = previous_workers
    if scope.failures:
        raise ValueError("native stage caught forbidden I/O")


def main() -> None:
    root = Path(sys.argv[2])
    if not root.is_absolute() or root.resolve() != root or not root.is_dir():
        raise ValueError("canonical absolute root required")
    os.chdir(root)
    if sys.argv[1] == "cli":
        native_cli(root, sys.argv[3:])
        return
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

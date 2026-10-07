import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { createHash } from "node:crypto";
import { EventEmitter } from "node:events";
import { mkdtemp, mkdir, readFile, realpath, rename, rm, stat, symlink, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";
import test from "node:test";
import {
  acquireGraphAccessLock, releaseGraphAccessLock, buildExtractArgs,
  buildChildEnv, createGraphSourceSnapshot, cleanupGraphSourceSnapshot,
  main, resolveGraphifyExecutable, runNativeBridge, validateCandidateCoverage,
  validateGraphFreshness, computeSourceFingerprint, createPrivateTempRoot, publishGraphSnapshot, PUBLISHED_GRAPH_ARTIFACTS,
  runGuardedNativeStage, runBoundedProcess, runQueryHttpServer, refreshStableGraph, validateAccounting,
  seedGraphUpdateSnapshot,
} from "../scripts/graphify.mjs";

async function pythonProbe(root, source, ...args) {
  const cli = await resolveGraphifyExecutable();
  assert(cli, "Graphify 0.9.39 is required for these native acceptance cases");
  const python = /^#!([^\r\n]+)/.exec(await readFile(cli.path, "utf8"))[1];
  const preamble = `
import importlib.util, json, os, sys
from pathlib import Path
spec = importlib.util.spec_from_file_location('bridge', sys.argv[1])
bridge = importlib.util.module_from_spec(spec)
spec.loader.exec_module(bridge)
root = Path(sys.argv[2])
`;
  const tempRoot = await createPrivateTempRoot();
  try {
    const result = spawnSync(python, ["-B", "-c", preamble + source,
      path.resolve(import.meta.dirname, "../scripts/native_inventory.py"), root, ...args],
    { env: buildChildEnv({ tempRoot }), encoding: "utf8", timeout: 180_000, maxBuffer: 33_554_432 });
    assert.equal(result.status, 0, result.stderr + result.stdout);
    return JSON.parse(result.stdout);
  } finally {
    await rm(tempRoot, { recursive: true, force: true });
  }
}

async function fixture(t) {
  const root = await realpath(await mkdtemp(path.join(tmpdir(), "shared-graphify-test-")));
  t.after(() => rm(root, { recursive: true, force: true }));
  assert.equal(spawnSync("git", ["init", "-q", root]).status, 0);
  await mkdir(path.join(root, "nested"));
  await writeFile(path.join(root, "nested", "api.py"), "def answer():\n    return 42\n");
  await writeFile(path.join(root, "main.go"), "package main\nfunc main() {}\n");
  await writeFile(path.join(root, "empty.py"), "# comment-only legitimate source\n");
  await writeFile(path.join(root, "README.md"), "A fixture document.\n");
  await writeFile(path.join(root, ".graphifyignore"), "ignored.py\n");
  await writeFile(path.join(root, "ignored.py"), "def excluded(): pass\n");
  await writeFile(path.join(root, "unsupported.r"), "print(1)\n");
  return root;
}

test("explicit canonical target required; CLI never derives root from installation", async () => {
  assert.equal(await main(["status"], { writeStderr() {} }), 2);
  assert.equal(await main(["--root", ".", "status"], { writeStderr() {} }), 2);
  assert.deepEqual(buildExtractArgs("/unrelated/repository").slice(0, 2), ["extract", "/unrelated/repository"]);
  const env = buildChildEnv({ tempRoot: "/private/fixture", baseEnv: { PATH: "/usr/bin", OPENAI_API_KEY: "secret" } });
  assert.equal(env.OPENAI_API_KEY, undefined);
});

test("shared exclusive lock remains per target and protects a live owner", async (t) => {
  const root = await fixture(t);
  const lock = await acquireGraphAccessLock({ repoRoot: root });
  try {
    await assert.rejects(acquireGraphAccessLock({ repoRoot: root, waitTimeoutMs: 15, pollMs: 1 }));
  } finally { await releaseGraphAccessLock(lock); }
  const next = await acquireGraphAccessLock({ repoRoot: root });
  await releaseGraphAccessLock(next);
});

test("owner bookkeeping stays outside source accounting while every genuine exclusion has one typed identity", async (t) => {
  const root = await fixture(t);
  const before = await runNativeBridge("inventory", root);
  const lock = await acquireGraphAccessLock({ repoRoot: root });
  try {
    const held = await runNativeBridge("inventory", root);
    assert.deepEqual(held.failures, []);
    assert.deepEqual(held.accounting, before.accounting);
    assert.deepEqual(held.excluded, held.accounting.entries.filter(x => x.disposition !== "eligible").map(x => ({ path: x.path, reason: x.disposition })));
    assert(!held.excluded.some(x => x.path.startsWith(".graphify-owner.lock")));
    assert(held.excluded.some(x => x.path === "ignored.py"));
    assert(held.excluded.some(x => x.path === "unsupported.r"));
  } finally { await releaseGraphAccessLock(lock); }
  assert.equal(await main(["build"], { repoRoot: root, writeStdout() {}, writeStderr() {} }), 0);
  assert.equal(await validateGraphFreshness(root), "current");
});

test("real pinned parser: arbitrary Go/Python root, nested source, empty and explicit exclusions; rejected candidate preserves accepted publication", async (t) => {
  if (!await resolveGraphifyExecutable()) return t.skip("pinned Graphify runtime unavailable");
  const root = await fixture(t);
  const out = [], errors = [];
  const deps = { repoRoot: root, writeStdout: (s) => out.push(s), writeStderr: (s) => errors.push(s) };
  assert.equal(await main(["build"], deps), 0, errors.join(""));
  const coverage = JSON.parse(await readFile(path.join(root, "graphify-out", "coverage.json"), "utf8"));
  assert.equal(coverage.complete, true);
  assert(coverage.expected.some((entry) => entry.path === "nested/api.py"));
  assert(coverage.extracted.some((entry) => entry.path === "empty.py"));
  assert(coverage.excluded.some((entry) => entry.path === "ignored.py" && entry.reason.includes("ignore")));
  assert(coverage.excluded.some((entry) => entry.path === "unsupported.r" && entry.reason.includes("extractor")));
  assert.equal(await validateGraphFreshness(root), "current");
  assert.equal(await main(["status"], deps), 0);
  assert(out.join("").includes("Coverage:"));

  const accepted = await Promise.all(["graph.json", "coverage.json", "coverage-freshness.json"].map((f) => readFile(path.join(root, "graphify-out", f))));
  await writeFile(path.join(root, "nested", "api.py"), "def answer(:\n  this is broken\n");
  assert.notEqual(await main(["update"], deps), 0);
  const preserved = await Promise.all(["graph.json", "coverage.json", "coverage-freshness.json"].map((f) => readFile(path.join(root, "graphify-out", f))));
  preserved.forEach((bytes, i) => assert(bytes.equals(accepted[i])));
  await assert.rejects(validateGraphFreshness(root));
  await writeFile(path.join(root, "nested", "api.py"), "def answer():\n    return 42\n");
  await rm(path.join(root, "graphify-out", "coverage.json"));
  assert.equal(await main(["status"], deps), 5, "old coverage-less graph must never be ready");
});

test("whole-owner update publishes changed source through native update with current coverage and freshness", async (t) => {
  const root = await fixture(t);
  const errors = [];
  const deps = { repoRoot: root, writeStdout() {}, writeStderr: text => errors.push(text) };
  assert.equal(await main(["build"], deps), 0, errors.join(""));
  const accepted = await computeSourceFingerprint(root);
  const sourcePath = "nested/api.py";
  const source = "def updated_answer():\n    return 43\n";
  await writeFile(path.join(root, sourcePath), source);
  await assert.rejects(validateGraphFreshness(root), error => error.kind === "stale");

  const actions = [];
  assert.equal(await main(["update"], {
    ...deps,
    runProcess: async options => {
      const cli = options.args.indexOf("cli");
      if (cli !== -1) actions.push(options.args[cli + 2]);
      return runBoundedProcess(options);
    },
  }), 0, errors.join(""));
  assert.deepEqual(actions, ["update"], "the actual native route must update without a build fallback");

  const output = path.join(root, "graphify-out");
  const coverage = JSON.parse(await readFile(path.join(output, "coverage.json"), "utf8"));
  const manifest = JSON.parse(await readFile(path.join(output, "manifest.json"), "utf8"));
  const graphBytes = await readFile(path.join(output, "graph.json"));
  const graph = JSON.parse(graphBytes);
  const freshness = JSON.parse(await readFile(path.join(output, "coverage-freshness.json"), "utf8"));
  const current = await computeSourceFingerprint(root);
  assert.equal(coverage.complete, true);
  assert.deepEqual(coverage.failures, []);
  assert(coverage.expected.some(entry => entry.path === sourcePath));
  const extracted = coverage.extracted.find(entry => entry.path === sourcePath);
  assert.equal(extracted?.disposition, "current AST stamp and graph contribution");
  assert(extracted.nodes > 0);
  assert.equal(manifest[sourcePath].ast_hash, createHash("md5").update(source).digest("hex"));
  assert(graph.nodes.some(node => String(node.source_file ?? "").endsWith(sourcePath)));
  assert.notEqual(current.sourceSha256, accepted.sourceSha256);
  assert.notEqual(current.sourceGenerationSha256, accepted.sourceGenerationSha256);
  assert.equal(coverage.sourceSha256, current.sourceSha256);
  assert.equal(coverage.accountingSha256, current.accountingSha256);
  assert.equal(freshness.sourceSha256, current.sourceSha256);
  assert.equal(freshness.artifactSha256["graph.json"], createHash("sha256").update(graphBytes).digest("hex"));
  assert.equal(await validateGraphFreshness(root), "current");
});

test("native bridge rejects dropped nested graph contribution and missing AST stamp before publication", async (t) => {
  if (!await resolveGraphifyExecutable()) return t.skip("pinned Graphify runtime unavailable");
  const root = await fixture(t);
  const tempRoot = await createPrivateTempRoot();
  const options = { childEnv: buildChildEnv({ tempRoot }) };
  const snapshot = await createGraphSourceSnapshot(root, tempRoot, options);
  t.after(async () => { await cleanupGraphSourceSnapshot(snapshot.root); await rm(tempRoot, { recursive: true, force: true }); });
  const cli = await resolveGraphifyExecutable();
  const result = await runGuardedNativeStage(buildExtractArgs(snapshot.root), snapshot.root, { ...options, graphifyExecutable: cli });
  assert(result.ok, result.stderr);
  const graphPath = path.join(snapshot.root, "graphify-out", "graph.json");
  const graph = JSON.parse(await readFile(graphPath, "utf8"));
  graph.nodes = graph.nodes.filter((node) => !String(node.source_file ?? "").endsWith("nested/api.py"));
  await writeFile(graphPath, JSON.stringify(graph));
  const report = await runNativeBridge("coverage", snapshot.root, options);
  assert(report.failures.some((entry) => entry.path === "nested/api.py" && entry.reason.includes("contribution")));
  await assert.rejects(validateCandidateCoverage(snapshot.root, snapshot.inventory, snapshot.sourceGeneration, options));
  const manifestPath = path.join(snapshot.root, "graphify-out", "manifest.json");
  const manifest = JSON.parse(await readFile(manifestPath, "utf8"));
  delete manifest["main.go"];
  await writeFile(manifestPath, JSON.stringify(manifest));
  const missing = await runNativeBridge("coverage", snapshot.root, options);
  assert(missing.failures.some((entry) => entry.path === "main.go" && entry.reason.includes("stamp")));
  assert.equal((await computeSourceFingerprint(root, options)).sourceSha256, snapshot.sourceGeneration.sourceSha256);
});


test("publication transaction restores every accepted artifact after rename and finalize failures", async (t) => {
  const root = await fixture(t);
  const candidate = await fixture(t);
  const files = [...PUBLISHED_GRAPH_ARTIFACTS, "coverage-freshness.json", ".graphify_root"];
  for (const directory of [root, candidate]) await mkdir(path.join(directory, "graphify-out"));
  for (const name of files) {
    await writeFile(path.join(root, "graphify-out", name), "accepted:" + name);
    await writeFile(path.join(candidate, "graphify-out", name), "candidate:" + name);
  }
  for (const failure of ["rename", "source-generation", "freshness-write", "commit"]) {
    let renamed = 0;
    await assert.rejects(publishGraphSnapshot(candidate, root, {
      renameImpl: async (...args) => {
        if (failure === "rename" && ++renamed === 2) throw new Error("rename fault");
        return rename(...args);
      },
      finalize: async () => {
        await writeFile(path.join(root, "graphify-out", "coverage-freshness.json"), "candidate-stamp");
        if (failure !== "rename") throw new Error(failure + " fault");
      },
    }));
    for (const name of files) assert.equal(await readFile(path.join(root, "graphify-out", name), "utf8"), "accepted:" + name);
  }
});

test("native pnpm workspace context is copied, resolves imports, and context-only edits stale coverage", async (t) => {
  const root = await fixture(t);
  await mkdir(path.join(root, "packages", "library"), { recursive: true });
  await writeFile(path.join(root, "pnpm-workspace.yaml"), "packages:\n  - 'packages/*'\n");
  await writeFile(path.join(root, "packages", "library", "package.json"), JSON.stringify({ name: "@fixture/library", main: "index.ts" }));
  await writeFile(path.join(root, "packages", "library", "index.ts"), "export function sharedAnswer() { return 42; }\n");
  await writeFile(path.join(root, "caller.ts"), "import { sharedAnswer } from '@fixture/library';\nexport const answer = sharedAnswer();\n");
  await mkdir(path.join(root, "config"));
  await writeFile(path.join(root, "config", "base.config"), JSON.stringify({ compilerOptions: { paths: {} } }));
  await writeFile(path.join(root, "tsconfig.json"), JSON.stringify({ extends: "./config/base.config" }));
  const inventory = await runNativeBridge("inventory", root);
  assert(inventory.contextInputs.includes("pnpm-workspace.yaml"));
  assert(inventory.contextInputs.includes("config/base.config"));
  assert(inventory.sourcePaths.includes("config/base.config"));
  assert(inventory.sourcePaths.includes("pnpm-workspace.yaml"));
  assert(!inventory.expected.some(x => x.path === "pnpm-workspace.yaml"));
  const deps = { repoRoot: root, writeStdout() {}, writeStderr() {} };
  assert.equal(await main(["build"], deps), 0);
  const graph = JSON.parse(await readFile(path.join(root, "graphify-out", "graph.json"), "utf8"));
  const nodes = new Map(graph.nodes.map(x => [x.id, x]));
  assert(graph.links.some(x => {
    const left = nodes.get(x.source), right = nodes.get(x.target);
    return String(left?.source_file).endsWith("caller.ts") && String(right?.source_file).endsWith("packages/library/index.ts");
  }), "actual native graph must resolve the workspace import");
  assert.equal(await validateGraphFreshness(root), "current");
  await writeFile(path.join(root, "pnpm-workspace.yaml"), "packages:\n  - 'different/*'\n");
  await assert.rejects(validateGraphFreshness(root));
});

test("native ancestor and extends context cannot escape the explicit root", async (t) => {
  const parent = await fixture(t);
  const root = path.join(parent, "child");
  await mkdir(root);
  assert.equal(spawnSync("git", ["init", "-q", root]).status, 0);
  await writeFile(path.join(root, "caller.ts"), "import { value } from '@fixture/value';\nexport const result = value;\n");
  await writeFile(path.join(parent, "tsconfig.json"), JSON.stringify({ compilerOptions: { paths: { "@fixture/*": ["*"] } } }));
  const report = await runNativeBridge("inventory", root);
  assert(report.failures.some(x => x.reason.includes("outside explicit target root")));
  assert.notEqual(await main(["build"], { repoRoot: root, writeStdout() {}, writeStderr() {} }), 0);
  await writeFile(path.join(root, "tsconfig.json"), JSON.stringify({ extends: "../tsconfig.json" }));
  const extended = await runNativeBridge("inventory", root);
  assert(extended.failures.some(x => x.reason.includes("outside explicit target root")));
});

test("default bridge callers receive canonical private scratch and remove it after success or failure", async (t) => {
  const root = await fixture(t);
  for (const fail of [false, true]) {
    let owned;
    const call = runNativeBridge("inventory", root, {
      baseEnv: { PATH: process.env.PATH, TMPDIR: "/ambient/alias", OPENAI_API_KEY: "synthetic-must-not-pass" },
      runProcess: async ({ env, cwd }) => {
        owned = path.dirname(env.HOME);
        assert.equal(cwd, root);
        assert.equal(await realpath(env.TMPDIR), env.TMPDIR);
        assert.equal((await stat(owned)).mode & 0o777, 0o700);
        assert.equal(env.OPENAI_API_KEY, undefined);
        assert.equal(env.TMPDIR, path.join(owned, "tmp"));
        assert.equal(env.GRAPHIFY_SCRATCH_ROOT, owned);
        if (fail) throw new Error("injected transport failure");
        return { ok: true, stdout: JSON.stringify({ private: true }), stderr: "" };
      },
    });
    if (fail) await assert.rejects(call, /injected transport failure/);
    else assert.deepEqual(await call, { private: true });
    assert(owned);
    await assert.rejects(stat(owned), error => error.code === "ENOENT");
  }
});

test("direct CLI validates one private scratch capability before native cache reads and detects capability drift", async (t) => {
  const root = await fixture(t), outside = await fixture(t);
  const sentinel = path.join(outside, "controlled-sentinel.txt");
  await writeFile(sentinel, "synthetic outside-root sentinel");
  const result = await pythonProbe(root, `
import contextlib, io, stat
import graphify.__main__ as entry
import graphify.cache as cache
cap = Path(os.environ['GRAPHIFY_SCRATCH_ROOT'])
inside = cap / 'tmp' / 'controlled-cache.txt'
inside.write_text('synthetic private scratch')
alias = cap / 'scratch-alias'
alias.symlink_to(cap, target_is_directory=True)
nonprivate = cap / 'nonprivate'
nonprivate.mkdir(mode=0o755); nonprivate.chmod(0o755)
sentinel = Path(sys.argv[3])
original_env = {name: os.environ.get(name) for name in ('GRAPHIFY_SCRATCH_ROOT', 'HOME', 'TMPDIR')}
old_main, old_lstat = entry.main, bridge._LSTAT
attempts, calls, failures = [], [], []
def observer(event, args):
    if event == 'open' and isinstance(args[0], (str, bytes, os.PathLike)):
        p = Path(os.fsdecode(args[0])).absolute()
        if p == sentinel:
            attempts.append('outside'); raise AssertionError('outside sentinel observer reached')
        if p == inside: attempts.append('inside')
sys.addaudithook(observer)
def reset_cache():
    cache._stat_index_root = None
    cache._stat_index_anchor = None
    cache._stat_index = None
    cache._stat_index_dirty = False
def invoked():
    calls.append('native-cli')
    reset_cache()
    assert len(cache.file_hash(inside, root, cache_root=cap)) == 64
entry.main = invoked
try:
    with contextlib.redirect_stdout(io.StringIO()):
        bridge.native_cli(root, ['extract', str(root), '--code-only'])
    assert attempts == ['inside']
    for case in ('HOME', 'TMPDIR', 'root', 'alias', 'file', 'nonprivate', 'unowned'):
        before = len(calls)
        if case in ('HOME', 'TMPDIR'): os.environ[case] = '/'
        elif case == 'root': os.environ['GRAPHIFY_SCRATCH_ROOT'] = '/'
        elif case == 'alias': os.environ['GRAPHIFY_SCRATCH_ROOT'] = str(alias)
        elif case == 'file': os.environ['GRAPHIFY_SCRATCH_ROOT'] = str(inside)
        elif case == 'nonprivate': os.environ['GRAPHIFY_SCRATCH_ROOT'] = str(nonprivate)
        elif case == 'unowned':
            def unowned(value, *args, **kwargs):
                st = old_lstat(value, *args, **kwargs)
                if Path(value) == cap:
                    fields = list(st); fields[4] = os.getuid() + 1
                    return os.stat_result(fields)
                return st
            bridge._LSTAT = unowned
        try:
            bridge.native_cli(root, ['extract', str(root), '--code-only'])
        except ValueError: failures.append(case)
        else: raise AssertionError(case + ' scratch admitted')
        finally:
            bridge._LSTAT = old_lstat
            for name, value in original_env.items():
                if value is None: os.environ.pop(name, None)
                else: os.environ[name] = value
        assert len(calls) == before
    def outside_read():
        calls.append('outside-cache')
        reset_cache()
        try: cache.file_hash(sentinel, root, cache_root=cap)
        except ValueError: pass  # Native recovery must not clear the violation.
    entry.main = outside_read
    try: bridge.native_cli(root, ['extract', str(root), '--code-only'])
    except ValueError as exc: assert str(exc) == 'native stage caught forbidden I/O'
    else: raise AssertionError('outside cache read admitted')
    def drift():
        calls.append('drift-cache')
        reset_cache()
        cap.chmod(0o755)
        try: cache.file_hash(inside, root, cache_root=cap)
        except ValueError: pass
    entry.main = drift
    try: bridge.native_cli(root, ['extract', str(root), '--code-only'])
    except ValueError as exc: assert str(exc) == 'native stage caught forbidden I/O'
    else: raise AssertionError('changed scratch capability admitted')
finally:
    cap.chmod(0o700)
    entry.main, bridge._LSTAT = old_main, old_lstat
print(json.dumps({'attempts': attempts, 'calls': calls, 'failures': failures}))
`, sentinel);
  assert.deepEqual(result.attempts, ["inside"]);
  assert.deepEqual(result.failures, ["HOME", "TMPDIR", "root", "alias", "file", "nonprivate", "unowned"]);
  assert.deepEqual(result.calls, ["native-cli", "outside-cache", "drift-cache"]);
});

test("supplied child environments cannot grant broad or aliased scratch to the actual CLI bridge", async (t) => {
  const root = await fixture(t);
  const tempRoot = await createPrivateTempRoot();
  t.after(() => rm(tempRoot, { recursive: true, force: true }));
  const alias = path.join(tempRoot, "alias");
  await symlink(tempRoot, alias);
  for (const override of [{ HOME: "/" }, { TMPDIR: "/" }, { GRAPHIFY_SCRATCH_ROOT: "/" }, { GRAPHIFY_SCRATCH_ROOT: alias }]) {
    const childEnv = { ...buildChildEnv({ tempRoot }), ...override };
    const result = await runGuardedNativeStage(["extract", root, "--code-only", "--max-workers", "1", "--out", root], root, { childEnv });
    assert.equal(result.ok, false, JSON.stringify(override));
    await assert.rejects(stat(path.join(root, "graphify-out", "graph.json")), error => error.code === "ENOENT");
  }
});

test("direct inventory canonicalizes its owned ambient scratch alias and retains eager pnpm and unknown-extension context", async (t) => {
  const root = await fixture(t);
  await mkdir(path.join(root, "packages", "library"), { recursive: true });
  await writeFile(path.join(root, "pnpm-workspace.yaml"), "packages:\n  - 'packages/*'\n");
  await writeFile(path.join(root, "packages", "library", "package.json"), JSON.stringify({ name: "@fixture/library", main: "index.ts" }));
  await writeFile(path.join(root, "packages", "library", "index.ts"), "export function sharedAnswer() { return 42; }\n");
  await writeFile(path.join(root, "caller.ts"), "import { sharedAnswer } from '@fixture/library';\nexport const answer = sharedAnswer();\n");
  await mkdir(path.join(root, "config"));
  await writeFile(path.join(root, "config", "base.config"), JSON.stringify({ compilerOptions: { paths: {} } }));
  await writeFile(path.join(root, "tsconfig.json"), JSON.stringify({ extends: "./config/base.config" }));
  const tempRoot = await createPrivateTempRoot();
  t.after(() => rm(tempRoot, { recursive: true, force: true }));
  const alias = path.join(tempRoot, "ambient-alias");
  await symlink(path.join(tempRoot, "tmp"), alias);
  const result = await pythonProbe(root, `
import tempfile
detect, _ = bridge.native_modules()
old_detect, old_tempdir = detect.detect, tempfile.tempdir
caches = []
def observed(*args, **kwargs):
    cache = kwargs['cache_root']
    assert cache == cache.resolve()
    assert bridge.nofollow(cache).st_uid == os.getuid()
    caches.append(str(cache))
    return old_detect(*args, **kwargs)
detect.detect, tempfile.tempdir = observed, sys.argv[3]
try:
    report = bridge.inventory(root)
finally:
    detect.detect, tempfile.tempdir = old_detect, old_tempdir
print(json.dumps({'report': report, 'caches': caches}))
`, alias);
  assert.equal(result.caches.length, 1);
  assert(!result.caches[0].includes("ambient-alias"));
  assert.deepEqual(result.report.failures, []);
  for (const context of ["pnpm-workspace.yaml", "config/base.config"]) {
    assert(result.report.contextInputs.includes(context));
    assert(result.report.sourcePaths.includes(context));
  }
});

test("trusted sysconfig bootstrap metadata succeeds while the same interpreter identity remains forbidden as context", async (t) => {
  const root = await fixture(t);
  const result = await pythonProbe(root, `
detect, _ = bridge.native_modules()
scope = bridge.NativeScope(root, detect); scope.preflight()
expected = bridge._RESOLVE(Path(sys.executable), strict=True)
proof = scope.bootstrap_metadata
permitted, attempts = [], []
def observed_metadata(p):
    result = proof(p)
    if result: permitted.append(str(p))
    return result
scope.bootstrap_metadata = observed_metadata
def observer(event, args):
    if event == 'open' and isinstance(args[0], (str, bytes, os.PathLike)) and Path(os.fsdecode(args[0])).absolute() == Path(sys.executable):
        attempts.append('open'); raise AssertionError('interpreter context pre-open observer reached')
sys.addaudithook(observer)
with scope.active():
    import ctypes, numpy, sysconfig
    assert sysconfig._safe_realpath(sys.executable) == str(expected)
    assert not scope.failures
    assert permitted
    if bridge._BOOTSTRAP_LINKS:
        try: next(iter(bridge._BOOTSTRAP_LINKS)).resolve()
        except ValueError: pass
        else: raise AssertionError('bootstrap identity accepted without trusted helper purpose')
    try: Path(sys.executable).read_bytes()
    except ValueError: pass
    else: raise AssertionError('interpreter content admitted as parser context')
print(json.dumps({'permitted': len(permitted), 'attempts': attempts, 'failures': list(scope.failures)}))
`);
  assert(result.permitted > 0);
  assert.deepEqual(result.attempts, []);
  assert(result.failures.length > 0);
});

test("native sensitive context is denied before open; runtime-prefix resolver reads remain confined", async (t) => {
  const cli = await resolveGraphifyExecutable();
  if (!cli) return t.skip("pinned Graphify runtime unavailable");
  const python = /^#!([^\r\n]+)/.exec(await readFile(cli.path, "utf8"))[1];
  const root = await fixture(t);
  const runtimeRoot = await fixture(t);
  await writeFile(path.join(root, "caller.ts"), "import { value } from '@fixture/value';\nexport const answer = value;\n");
  await writeFile(path.join(runtimeRoot, "caller.ts"), "import { value } from '@fixture/value';\nexport const answer = value;\n");
  await writeFile(path.join(root, "tsconfig.json"), JSON.stringify({ extends: "./.env.context.json" }));
  await writeFile(path.join(root, ".env.context.json"), "{}");
  const probe = spawnSync(python, ["-c", `
import importlib.util, json, os, sys
from pathlib import Path
spec = importlib.util.spec_from_file_location('bridge', sys.argv[1])
bridge = importlib.util.module_from_spec(spec)
spec.loader.exec_module(bridge)
root = Path(sys.argv[2])
synthetic = root / '.env.context.json'
attempts = []
def observer(event, args):
    if event == 'open' and isinstance(args[0], (str, bytes, os.PathLike)) and Path(os.fsdecode(args[0])).absolute() == synthetic:
        attempts.append('open')
        raise PermissionError('synthetic pre-open observer')
sys.addaudithook(observer)
sensitive = bridge.inventory(root)
import graphify.extractors.resolution as resolution
runtime_context = Path(resolution.__file__).resolve()
assert runtime_context.is_relative_to(Path(sys.prefix).resolve())
runtime_root = Path(sys.argv[3])
(runtime_root / 'tsconfig.json').write_text(json.dumps({'extends': str(runtime_context)}))
runtime = bridge.inventory(runtime_root)
print(json.dumps({'sensitive': sensitive, 'runtime': runtime, 'attempts': len(attempts)}))
`, path.resolve(import.meta.dirname, "../scripts/native_inventory.py"), root, runtimeRoot], { encoding: "utf8" });
  assert.equal(probe.status, 0, probe.stderr);
  const report = JSON.parse(probe.stdout);
  assert.equal(report.attempts, 0, "bridge must reject before another pre-open observer sees sensitive access");
  assert(report.sensitive.failures.some(x => x.path === ".env.context.json" && x.reason === "native sensitive parser context"));
  assert(!report.sensitive.sourcePaths.includes(".env.context.json"));
  assert(!report.sensitive.contextInputs.includes(".env.context.json"));
  assert(report.sensitive.excluded.some(x => x.path === ".env.context.json" && x.reason === "native sensitive path"));
  assert(!report.sensitive.failures.some(x => x.reason.includes("outside")));
  assert(report.runtime.failures.some(x => x.reason.includes("outside explicit target root")));
  assert.notEqual(await main(["build"], { repoRoot: root, writeStdout() {}, writeStderr() {} }), 0);
});


test("real refresh rolls back accepted artifacts when freshness publication fails", async (t) => {
  const root = await fixture(t);
  const deps = { repoRoot: root, writeStdout() {}, writeStderr() {} };
  assert.equal(await main(["build"], deps), 0);
  const files = [...PUBLISHED_GRAPH_ARTIFACTS, "coverage-freshness.json"];
  const accepted = await Promise.all(files.map(name => readFile(path.join(root, "graphify-out", name))));
  assert.notEqual(await main(["update"], {
    ...deps,
    writeFreshness: async () => {
      await writeFile(path.join(root, "graphify-out", "coverage-freshness.json"), "failed candidate stamp");
      throw new Error("injected freshness write failure");
    },
  }), 0);
  for (let i = 0; i < files.length; i++) assert((await readFile(path.join(root, "graphify-out", files[i]))).equals(accepted[i]));
  assert.equal(await validateGraphFreshness(root), "current");
});

test("native ignored literal identities and internal/external/dangling links are accounted without target access", async (t) => {
  const root = await fixture(t), outside = await fixture(t);
  const c = String.raw`C:\Users\alice\chats.json`, unc = String.raw`\\server\share\chats.json`;
  await writeFile(path.join(root, c), "{}");
  await writeFile(path.join(root, unc), "{}");
  await mkdir(path.join(root, "docs"));
  await symlink(path.join(root, "main.go"), path.join(root, "docs", "internal.md"));
  await symlink(path.join(outside, "main.go"), path.join(root, "docs", "external.md"));
  await symlink(path.join(outside, "missing"), path.join(root, "docs", "dangling.md"));
  await writeFile(path.join(root, ".graphifyignore"), `ignored.py\n/${c}\n/${unc}\ndocs/\n`);
  const observed = await pythonProbe(root, `
blocked = {root / value for value in json.loads(sys.argv[3])}
attempts = []
old_stat, old_resolve, old_readlink = bridge._STAT, bridge._RESOLVE, bridge._READLINK
def observed_stat(value, *args, **kwargs):
    if not isinstance(value, int) and Path(value) in blocked and kwargs.get('follow_symlinks', True):
        attempts.append('stat'); raise AssertionError('target stat forbidden')
    return old_stat(value, *args, **kwargs)
def observed_resolve(value, *args, **kwargs):
    if Path(value) in blocked:
        attempts.append('resolve'); raise AssertionError('target resolve forbidden')
    return old_resolve(value, *args, **kwargs)
def observed_readlink(value, *args, **kwargs):
    if Path(value) in blocked:
        attempts.append('readlink'); raise AssertionError('target readlink forbidden')
    return old_readlink(value, *args, **kwargs)
bridge._STAT, bridge._RESOLVE, bridge._READLINK = observed_stat, observed_resolve, observed_readlink
def observer(event, args):
    if event == 'open' and isinstance(args[0], (str, bytes, os.PathLike)) and Path(os.fsdecode(args[0])).absolute() in blocked:
        attempts.append('open'); raise AssertionError('target open forbidden')
sys.addaudithook(observer)
report = bridge.inventory(root)
print(json.dumps({'report': report, 'attempts': attempts}))
`, JSON.stringify([c, unc, "docs/internal.md", "docs/external.md", "docs/dangling.md"]));
  assert.deepEqual(observed.attempts, []);
  assert.deepEqual(observed.report.failures, []);
  validateAccounting(observed.report.accounting);
  for (const name of [c, unc, "docs/internal.md", "docs/external.md", "docs/dangling.md"]) {
    const entry = observed.report.accounting.entries.find(x => x.path === name);
    assert(entry?.nativeIgnored && !entry.copied && !entry.context);
    assert.equal(entry.disposition, "native ignore rule");
    assert.equal(entry.type, name.startsWith("docs/") ? "symlink" : "regular");
    assert(!observed.report.sourcePaths.includes(name));
  }
  const deps = { repoRoot: root, writeStdout() {}, writeStderr() {} };
  assert.equal(await main(["build"], deps), 0);
  assert.equal(await validateGraphFreshness(root), "current");
  // Changing only an excluded identity's type still invalidates accounting.
  await rm(path.join(root, "docs", "dangling.md"));
  await writeFile(path.join(root, "docs", "dangling.md"), "ignored regular replacement");
  await assert.rejects(validateGraphFreshness(root));
});

test("unignored unsafe code, linked controls and metadata-dependent link markers fail before contents", async (t) => {
  for (const kind of ["unsafe-code", "control", "marker"]) {
    const root = await fixture(t), outside = await fixture(t);
    if (kind === "unsafe-code") await writeFile(path.join(root, String.raw`C:\Users\alice\chats.json`), "{}");
    if (kind === "control") {
      await rm(path.join(root, ".graphifyignore"));
      await symlink(path.join(outside, "README.md"), path.join(root, ".graphifyignore"));
    }
    if (kind === "marker") {
      await mkdir(path.join(root, "env"));
      await symlink(path.join(outside, "README.md"), path.join(root, "env", "pyvenv.cfg"));
      await writeFile(path.join(root, ".graphifyignore"), "ignored.py\nenv/pyvenv.cfg\n");
    }
    const report = await runNativeBridge("inventory", root);
    assert(report.failures.length > 0, kind);
    assert.notEqual(await main(["build"], { repoRoot: root, writeStdout() {}, writeStderr() {} }), 0, kind);
  }
});

test("sensitivity helper content attempts persist after native recovery; nested guards restore exact outer primitives", async (t) => {
  const root = await fixture(t);
  await mkdir(path.join(root, "secrets"));
  await writeFile(path.join(root, "secrets", "notes.md"), "synthetic content must not be read");
  const result = await pythonProbe(root, `
detect, extract = bridge.native_modules()
attempts = []
def observer(event, args):
    if event == 'open' and isinstance(args[0], (str, bytes, os.PathLike)) and Path(os.fsdecode(args[0])).absolute() == root / 'secrets/notes.md':
        attempts.append('open'); raise AssertionError('pre-open observer reached')
sys.addaudithook(observer)
report = bridge.inventory(root)
outer, inner = bridge.NativeScope(root, detect), bridge.NativeScope(root, detect)
outer.preflight(); inner.preflight()
original = (os.stat, os.lstat, os.scandir, os.readlink, Path.resolve, os.open, os.pipe, bridge.subprocess.Popen.__init__)
with outer.active():
    outer_functions = (os.stat, os.lstat, os.scandir, os.readlink, Path.resolve, os.open, os.pipe, bridge.subprocess.Popen.__init__)
    with inner.active():
        assert bridge._RESOLUTION_SCOPE is inner
        try: (root / 'secrets/notes.md').read_text()
        except ValueError: pass
    assert bridge._RESOLUTION_SCOPE is outer
    assert outer_functions == (os.stat, os.lstat, os.scandir, os.readlink, Path.resolve, os.open, os.pipe, bridge.subprocess.Popen.__init__)
assert bridge._RESOLUTION_SCOPE is None
assert original == (os.stat, os.lstat, os.scandir, os.readlink, Path.resolve, os.open, os.pipe, bridge.subprocess.Popen.__init__)
print(json.dumps({'report': report, 'attempts': attempts, 'inner': list(inner.failures)}))
`);
  assert.deepEqual(result.attempts, []);
  assert(result.report.failures.some(x => x.reason.includes("pre-permission")));
  assert(result.inner.length > 0);
});

test("approved Git transport pipes are scoped; unobserved regular and pipe descriptors remain forbidden", async (t) => {
  const root = await fixture(t);
  const result = await pythonProbe(root, `
import io, stat, subprocess
detect, _ = bridge.native_modules()
scope = bridge.NativeScope(root, detect); scope.preflight()
fd = bridge._OPEN(root / 'main.go', os.O_RDONLY)
unobserved = bridge._PIPE()
opened, commands = [], []
def observer(event, args):
    if event == 'open' and isinstance(args[0], int):
        assert stat.S_ISFIFO(os.fstat(args[0]).st_mode)
        opened.append(args[0])
    if event == 'subprocess.Popen':
        commands.append(args[1])
sys.addaudithook(observer)
try:
    with scope.active():
        command = ['git', '-C', str(root), 'ls-files', '-z', '--cached', '--others', '--exclude-standard']
        result = subprocess.run(command, capture_output=True, check=True, timeout=3)
        assert b'main.go' in result.stdout
        assert not scope.failures
        assert scope.transport is None
        for forbidden in (fd, unobserved[0]):
            before = scope.violation_count
            try: io.open(forbidden, 'rb', closefd=False)
            except ValueError: pass
            else: raise AssertionError('unobserved descriptor admitted')
            assert scope.violation_count > before
finally:
    os.close(fd)
    for descriptor in unobserved: os.close(descriptor)
print(json.dumps({'opened': opened, 'commands': commands, 'failures': list(scope.failures)}))
`);
  assert(result.opened.length >= 2, "both approved capture pipes reached the downstream observer");
  assert.equal(result.commands.length, 1);
  assert(result.failures.some(x => x[1] === "unobserved native file descriptor metadata"));
});

test("native Git HEAD metadata requires preflight and the canonical current root; other process routes stay denied", async (t) => {
  const root = await fixture(t);
  assert.equal(spawnSync("git", ["-C", root, "add", "main.go"]).status, 0);
  assert.equal(spawnSync("git", ["-C", root, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "-c", "commit.gpgsign=false", "commit", "-qm", "fixture"]).status, 0);
  const expected = spawnSync("git", ["-C", root, "rev-parse", "HEAD"], { encoding: "utf8" }).stdout.trim();
  const result = await pythonProbe(root, `
import subprocess
import graphify.export as export
import graphify.watch as watch
detect, _ = bridge.native_modules()
scope = bridge.NativeScope(root, detect); scope.preflight()
original_cwd = Path.cwd()
events = []
def observer(event, args):
    if event == 'subprocess.Popen': events.append(args[1])
sys.addaudithook(observer)
try:
    os.chdir(root)
    with scope.active():
        heads = [export._git_head(), watch._git_head(cwd=root)]
        assert not scope.failures
    before = len(events)
    for case in ('unprepared', 'outside', 'command', 'executable', 'shell', 'descriptor'):
        rejected = bridge.NativeScope(root, detect)
        if case != 'unprepared': rejected.preflight()
        options = {'capture_output': True, 'timeout': 3}
        command = ['git', 'rev-parse', 'HEAD']
        if case == 'outside': options['cwd'] = str(root.parent)
        if case == 'command': command = ['git', 'status', '--porcelain']
        if case == 'executable': options['executable'] = '/usr/bin/git'
        if case == 'shell': options['shell'] = True
        if case == 'descriptor': options['pass_fds'] = (1,)
        with rejected.active():
            try: subprocess.run(command, **options)
            except ValueError: pass
            else: raise AssertionError(case + ' admitted')
        assert rejected.failures, case
        assert len(events) == before, case
finally:
    os.chdir(original_cwd)
print(json.dumps({'heads': heads, 'events': events}))
`);
  assert.deepEqual(result.heads, [expected, expected]);
  assert.deepEqual(result.events, [["git", "rev-parse", "HEAD"], ["git", "rev-parse", "HEAD"]]);
});

test("actual Fortran caught cpp fallback never creates a parser subprocess", async (t) => {
  const root = await fixture(t);
  await writeFile(path.join(root, "sample.F90"), "program sample\nend program sample\n");
  const result = await pythonProbe(root, `
detect, _ = bridge.native_modules()
import graphify.extractors.fortran as fortran
import shutil
original_which = shutil.which
shutil.which = lambda name: '/usr/bin/cpp' if name == 'cpp' else original_which(name)
scope = bridge.NativeScope(root, detect); scope.preflight()
attempts = []
def observer(event, args):
    if event == 'subprocess.Popen':
        attempts.append('created'); raise AssertionError('subprocess observer reached')
sys.addaudithook(observer)
try:
    with scope.active():
        # The pinned implementation catches the subprocess rejection and
        # returns raw bytes; the persistent failure must remain authoritative.
        result = fortran._cpp_preprocess(root / 'sample.F90')
    assert result.startswith(b'program sample')
finally:
    shutil.which = original_which
print(json.dumps({'attempts': attempts, 'failures': list(scope.failures)}))
`);
  assert.deepEqual(result.attempts, []);
  assert(result.failures.some(x => x[1].includes("unobserved native parser subprocess")));
});

test("official native build and update catch forbidden actual-extraction reads and still fail", async (t) => {
  for (const action of ["extract", "update"]) {
    const root = await fixture(t);
    const deps = { repoRoot: root, writeStdout() {}, writeStderr() {} };
    assert.equal(await main(["build"], deps), 0);
    const accepted = await Promise.all([...PUBLISHED_GRAPH_ARTIFACTS, "coverage-freshness.json"].map(name => readFile(path.join(root, "graphify-out", name))));
    await writeFile(path.join(root, ".env.context.json"), "synthetic forbidden content");
    await writeFile(path.join(root, "nested", "api.py"), "def changed(): return 43\n");
    const tempRoot = await createPrivateTempRoot();
    let snapshot = null;
    t.after(async () => {
      if (snapshot !== null) await cleanupGraphSourceSnapshot(snapshot.root);
      await rm(tempRoot, { recursive: true, force: true });
    });
    const options = { childEnv: buildChildEnv({ tempRoot }) };
    snapshot = await createGraphSourceSnapshot(root, tempRoot, options);
    if (action === "update") await seedGraphUpdateSnapshot(root, snapshot.root, options);
    const probe = await pythonProbe(snapshot.root, `
detect, extract = bridge.native_modules()
import graphify.__main__
old = extract._safe_extract_with_xaml_root
root.chmod(0o700)
(root / '.env.context.json').write_text('synthetic forbidden bytes')
root.chmod(0o500)
attempts = []
invocations = []
def observer(event, args):
    if event == 'open' and isinstance(args[0], (str, bytes, os.PathLike)) and Path(os.fsdecode(args[0])).absolute() == root / '.env.context.json':
        attempts.append('open'); raise AssertionError('forbidden read observer reached')
sys.addaudithook(observer)
def caught(extractor, path, corpus):
    invocations.append(str(path))
    try: (root / '.env.context.json').read_text()
    except Exception: pass
    return old(extractor, path, corpus)
extract._safe_extract_with_xaml_root = caught
failed = False
failure = ''
try:
    arguments = ['extract', str(root), '--code-only', '--max-workers', '1', '--out', str(root)] if sys.argv[3] == 'extract' else ['update', str(root), '--force']
    os.chdir(root)  # Match the actual bridge's canonical-root CLI boundary.
    with __import__('contextlib').redirect_stdout(__import__('io').StringIO()):
        bridge.native_cli(root, arguments)
except (ValueError, SystemExit) as exc:
    failed = True
    failure = str(exc)
finally:
    extract._safe_extract_with_xaml_root = old
print(json.dumps({'failed': failed, 'failure': failure, 'attempts': attempts, 'invocations': invocations}))
`, action);
    assert(probe.failed, action);
    assert(probe.invocations.length > 0, "actual native extraction must reach the injected caught read");
    assert.equal(probe.failure, "native stage caught forbidden I/O");
    assert.deepEqual(probe.attempts, []);
    for (const [i, name] of [...PUBLISHED_GRAPH_ARTIFACTS, "coverage-freshness.json"].entries()) {
      assert((await readFile(path.join(root, "graphify-out", name))).equals(accepted[i]));
    }
  }
});

test("real build and update above native parallel threshold create no workers", async (t) => {
  const root = await fixture(t);
  for (let i = 0; i < 40; i++) await writeFile(path.join(root, `source-${i}.py`), `def symbol_${i}(): return ${i}\n`);
  const result = await pythonProbe(root, `
import contextlib, io
detect, extract = bridge.native_modules()
import graphify.__main__
assert len(list(root.glob('*.py'))) > extract._PARALLEL_THRESHOLD
events, metadata = [], []
def observer(event, args):
    if event == 'subprocess.Popen' and args[1] == ['git', 'rev-parse', 'HEAD']:
        metadata.append(args[1]); return
    if event in ('subprocess.Popen', 'os.fork', 'os.posix_spawn'):
        events.append(event); raise AssertionError('worker creation attempted')
sys.addaudithook(observer)
os.chdir(root)  # Match the actual bridge's canonical-root CLI boundary.
with contextlib.redirect_stdout(io.StringIO()):
    bridge.native_cli(root, ['extract', str(root), '--code-only', '--max-workers', '1', '--out', str(root)])
    for path in root.glob('source-*.py'):
        path.write_text(path.read_text() + '# changed\\n')
    bridge.native_cli(root, ['update', str(root), '--force'])
print(json.dumps({'events': events, 'metadata': metadata, 'manifest': json.loads((root / 'graphify-out/manifest.json').read_text())}))
`);
  assert.deepEqual(result.events, []);
  assert(result.metadata.length >= 2, "actual extract and update reached their current-root Git metadata probes");
  assert.equal(Object.keys(result.manifest).filter(x => x.startsWith("source-")).length, 40);
});

test("add/remove/type/disposition drift consumes one retry and preserves the accepted artifact set", async (t) => {
  for (const drift of ["add", "remove", "type", "disposition"]) {
    const root = await fixture(t);
    const ignore = "ignored.py\nexcluded.json\nborn.json\n";
    await writeFile(path.join(root, ".graphifyignore"), ignore);
    await writeFile(path.join(root, "excluded.json"), "{}");
    assert.equal(await main(["build"], { repoRoot: root, writeStdout() {}, writeStderr() {} }), 0);
    const files = [...PUBLISHED_GRAPH_ARTIFACTS, "coverage-freshness.json"];
    const accepted = await Promise.all(files.map(name => readFile(path.join(root, "graphify-out", name))));
    const tempRoot = await createPrivateTempRoot();
    t.after(() => rm(tempRoot, { recursive: true, force: true }));
    const executable = await resolveGraphifyExecutable();
    const childEnv = buildChildEnv({ tempRoot });
    const options = { childEnv, graphifyExecutable: executable, validateCoverage: async () => {} };
    let attempts = 0, publications = 0;
    await assert.rejects(refreshStableGraph({
      mode: "build", repoRoot: root, tempRoot, executable, childEnv, freshnessOptions: options,
      assertOwned: async () => {}, recheckIdentity: async () => {},
      runProcess: async ({ args }) => {
        if (args.includes("extract")) {
          attempts++;
          if (drift === "add") {
            if (attempts === 1) await writeFile(path.join(root, "born.json"), "{}");
            else await rm(path.join(root, "born.json"));
          } else if (drift === "remove") {
            if (attempts === 1) await rm(path.join(root, "excluded.json"));
            else await writeFile(path.join(root, "excluded.json"), "{}");
          } else if (drift === "type") {
            await rm(path.join(root, "excluded.json"));
            if (attempts === 1) await symlink(path.join(root, "main.go"), path.join(root, "excluded.json"));
            else await writeFile(path.join(root, "excluded.json"), "{}");
          } else {
            await writeFile(path.join(root, ".graphifyignore"), attempts === 1 ? "ignored.py\nborn.json\n" : ignore);
          }
        }
        return { ok: true, stdout: "", stderr: "" };
      },
      validateOutput: async () => {}, validateGraph: async () => {},
      createSnapshot: createGraphSourceSnapshot, seedSnapshot: seedGraphUpdateSnapshot,
      sanitizeSnapshot: async () => {}, publishSnapshot: async () => { publications++; },
      cleanupSnapshot: cleanupGraphSourceSnapshot, computeSource: computeSourceFingerprint,
      writeFreshness: async () => {}, validateFreshness: async () => "current",
    }), error => error.kind === "unstable-source");
    assert.equal(attempts, 2, drift);
    assert.equal(publications, 0, drift);
    for (const [i, name] of files.entries()) assert((await readFile(path.join(root, "graphify-out", name))).equals(accepted[i]), `${drift}:${name}`);
    if (drift === "add") assert.equal(await validateGraphFreshness(root), "current", "exactly restored census still describes the accepted graph");
    else await assert.rejects(validateGraphFreshness(root));
  }
});

test("snapshot accounting catches a metadata-only change during copying", async (t) => {
  const root = await fixture(t);
  await writeFile(path.join(root, ".graphifyignore"), "ignored.py\nlate.json\n");
  const tempRoot = await createPrivateTempRoot();
  t.after(() => rm(tempRoot, { recursive: true, force: true }));
  let changed = false;
  await assert.rejects(createGraphSourceSnapshot(root, tempRoot, {
    writeFileImpl: async (...args) => {
      if (!changed) { changed = true; await writeFile(path.join(root, "late.json"), "{}"); }
      return writeFile(...args);
    },
  }), error => error.kind === "unstable-source");
  assert(changed);
});

test("old, duplicate and forged exceptional accounting cannot enter status/health/query admission", async (t) => {
  const root = await fixture(t);
  const deps = { repoRoot: root, baseEnv: { ...process.env, GRAPHIFY_AUTO_REFRESH: "0" }, writeStdout() {}, writeStderr() {} };
  assert.equal(await main(["build"], deps), 0);
  const file = path.join(root, "graphify-out", "coverage.json");
  const accepted = JSON.parse(await readFile(file, "utf8"));
  for (const corrupt of [
    value => { value.schema = "graphify.coverage.v1"; delete value.accounting; },
    value => { value.accounting.schema = "old.accounting"; },
    value => { value.accounting.entries.push(value.accounting.entries[0]); },
    value => { value.accounting.entries[0].path = String.raw`C:\unsafe.py`; value.accounting.entries[0].nativeIgnored = false; },
    value => { value.excluded.pop(); },
    value => { value.excluded.push({ path: ".graphify-owner.lock", reason: "native ignore rule" }); },
  ]) {
    const candidate = structuredClone(accepted); corrupt(candidate);
    await writeFile(file, JSON.stringify(candidate));
    await assert.rejects(validateGraphFreshness(root));
    assert.notEqual(await main(["status"], deps), 0);
    assert.notEqual(await main(["query", "answer"], deps), 0);
  }
});

test("real loopback HTTP admits current coverage then rejects accounting drift", async (t) => {
  const root = await fixture(t);
  const deps = { repoRoot: root, baseEnv: { ...process.env, GRAPHIFY_AUTO_REFRESH: "0" }, writeStdout() {}, writeStderr() {} };
  assert.equal(await main(["build"], deps), 0);
  const signals = new EventEmitter(), apiKey = "synthetic-loopback-key-123";
  let announce;
  const listening = new Promise(resolve => { announce = resolve; });
  const stopped = runQueryHttpServer({
    apiKey, host: "127.0.0.1", port: 0, signalEmitter: signals,
    checkHealth: () => validateGraphFreshness(root),
    executeQuery: async question => {
      const answer = [], errors = [];
      const code = await main(["query", question], { ...deps, writeStdout: text => answer.push(text), writeStderr: text => errors.push(text) });
      assert.equal(code, 0, errors.join(""));
      return answer.join("");
    },
    onListening: announce,
  });
  try {
    const address = await listening;
    const endpoint = `http://127.0.0.1:${address.port}`;
    const headers = { authorization: `Bearer ${apiKey}`, "content-type": "application/json", connection: "close" };
    assert.equal((await fetch(endpoint + "/health", { headers })).status, 200);
    const current = await fetch(endpoint + "/query", { method: "POST", headers, body: JSON.stringify({ question: "answer" }) });
    assert.equal(current.status, 200);
    assert.equal((await current.json()).schema, "graphify.http.v1");
    await writeFile(path.join(root, "late-source.py"), "def newly_added(): pass\n");
    assert.equal((await fetch(endpoint + "/health", { headers })).status, 503);
    assert.equal((await fetch(endpoint + "/query", { method: "POST", headers, body: JSON.stringify({ question: "answer" }) })).status, 503);
    await rm(path.join(root, "late-source.py"));
    assert.equal((await fetch(endpoint + "/health", { headers })).status, 200);
    const coveragePath = path.join(root, "graphify-out", "coverage.json");
    const oldCoverage = JSON.parse(await readFile(coveragePath, "utf8"));
    oldCoverage.schema = "graphify.coverage.v1";
    delete oldCoverage.accounting;
    await writeFile(coveragePath, JSON.stringify(oldCoverage));
    assert.equal((await fetch(endpoint + "/health", { headers })).status, 503, "old accounting is never ready");
    assert.equal((await fetch(endpoint + "/query", { method: "POST", headers, body: JSON.stringify({ question: "answer" }) })).status, 503);
  } finally {
    signals.emit("SIGTERM");
    assert((await stopped).ok);
  }
});

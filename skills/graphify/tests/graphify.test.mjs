import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { mkdtemp, mkdir, readFile, realpath, rename, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";
import test from "node:test";
import {
  acquireGraphAccessLock, releaseGraphAccessLock, buildExtractArgs,
  buildChildEnv, createGraphSourceSnapshot, cleanupGraphSourceSnapshot,
  main, resolveGraphifyExecutable, runNativeBridge, validateCandidateCoverage,
  validateGraphFreshness, computeSourceFingerprint, createPrivateTempRoot, publishGraphSnapshot, PUBLISHED_GRAPH_ARTIFACTS,
} from "../scripts/graphify.mjs";

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

test("native bridge rejects dropped nested graph contribution and missing AST stamp before publication", async (t) => {
  if (!await resolveGraphifyExecutable()) return t.skip("pinned Graphify runtime unavailable");
  const root = await fixture(t);
  const tempRoot = await createPrivateTempRoot();
  const options = { childEnv: buildChildEnv({ tempRoot }) };
  const snapshot = await createGraphSourceSnapshot(root, tempRoot, options);
  t.after(async () => { await cleanupGraphSourceSnapshot(snapshot.root); await rm(tempRoot, { recursive: true, force: true }); });
  const cli = await resolveGraphifyExecutable();
  const result = spawnSync(cli.path, buildExtractArgs(snapshot.root), { cwd: snapshot.root, env: options.childEnv, encoding: "utf8" });
  assert.equal(result.status, 0, result.stderr);
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

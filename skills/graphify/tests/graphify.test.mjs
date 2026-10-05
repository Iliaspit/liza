import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { mkdtemp, mkdir, readFile, realpath, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";
import test from "node:test";
import {
  acquireGraphAccessLock, releaseGraphAccessLock, buildExtractArgs,
  buildChildEnv, createGraphSourceSnapshot, cleanupGraphSourceSnapshot,
  main, resolveGraphifyExecutable, runNativeBridge, validateCandidateCoverage,
  validateGraphFreshness, computeSourceFingerprint, createPrivateTempRoot,
} from "../scripts/graphify.mjs";

async function fixture(t) {
  const root = await realpath(await mkdtemp(path.join(tmpdir(), "liza-shared-graphify-test-")));
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

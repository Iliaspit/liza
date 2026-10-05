#!/usr/bin/env node

import { spawn } from "node:child_process";
import { createHash, randomBytes, timingSafeEqual } from "node:crypto";
import { constants as fsConstants } from "node:fs";
import {
  access,
  chmod,
  lstat,
  mkdir,
  mkdtemp,
  readFile,
  readdir,
  realpath,
  rename,
  rm,
  stat,
  utimes,
  writeFile,
} from "node:fs/promises";
import { createServer } from "node:http";
import { tmpdir } from "node:os";
import path from "node:path";
import { performance } from "node:perf_hooks";
import { fileURLToPath, pathToFileURL } from "node:url";

export const REQUIRED_VERSION = "0.9.39";
export const SETUP_COMMAND = `uv tool install graphifyy==${REQUIRED_VERSION}`;
export const FORCE_SETUP_COMMAND = `uv tool install --force graphifyy==${REQUIRED_VERSION}`;
export const MARKER_SCHEMA = "graphify.v3";
export const MARKER_EVENT = "graphify";
export const MAX_QUESTION_CHARS = 4096;
// All operations require a caller-supplied canonical absolute target root.
export const REPO_ROOT = undefined;
export const GRAPH_DIRECTORY = undefined;
export const GRAPH_PATH = undefined;
export const FRESHNESS_PATH = undefined;
export const FRESHNESS_SCHEMA = "graphify.freshness.v2";
export const SHARED_GRAPH_SCHEMA = "graphify.http.v1";
export const SHARED_GRAPH_HOST = "0.0.0.0";
export const SHARED_GRAPH_PORT = 8080;
export const MAX_REQUEST_BYTES = 8_192;
export const GRAPH_LOCK_PATH = undefined;
export const GRAPH_LOCK_SCHEMA = "graphify.lock.v1";
export const GRAPH_LOCK_RECOVERY_PATH = undefined;
export const GRAPH_LOCK_RECOVERY_SCHEMA = "graphify.lock-recovery.v1";
export const GRAPH_LOCK_WAIT_TIMEOUT_MS = 660_000;
export const GRAPH_LOCK_POLL_MS = 100;
export const GRAPH_LOCK_INITIALIZATION_GRACE_MS = 30_000;
export const HTTP_SHUTDOWN_TIMEOUT_MS = 5_000;
export const PUBLISHED_GRAPH_ARTIFACTS = Object.freeze([
  ".graphify_analysis.json",
  ".graphify_labels.json",
  ".graphify_labels.json.sig",
  "GRAPH_REPORT.md",
  "graph.html",
  "graph.json",
  "manifest.json",
  "coverage.json",
]);
export const UPDATE_SEED_ARTIFACTS = PUBLISHED_GRAPH_ARTIFACTS.filter((name) => name !== "coverage.json");

export const STAGE_LIMITS = Object.freeze({
  version: Object.freeze({ timeoutMs: 10_000, maxStdoutBytes: 1_024, maxStderrBytes: 8_192 }),
  inventory: Object.freeze({ timeoutMs: 30_000, maxStdoutBytes: 4_194_304, maxStderrBytes: 8_192 }),
  build: Object.freeze({ timeoutMs: 600_000, maxStdoutBytes: 65_536, maxStderrBytes: 65_536 }),
  cluster: Object.freeze({ timeoutMs: 60_000, maxStdoutBytes: 65_536, maxStderrBytes: 65_536 }),
  update: Object.freeze({ timeoutMs: 300_000, maxStdoutBytes: 65_536, maxStderrBytes: 65_536 }),
  query: Object.freeze({ timeoutMs: 30_000, maxStdoutBytes: 1_048_576, maxStderrBytes: 65_536 }),
});

const HELP_TEXT = [
  "Usage:",
  "  node <installed-skill>/scripts/graphify.mjs --root <absolute-repository> status",
  "  node <installed-skill>/scripts/graphify.mjs --root <absolute-repository> build",
  "  node <installed-skill>/scripts/graphify.mjs --root <absolute-repository> update",
  '  node <installed-skill>/scripts/graphify.mjs --root <absolute-repository> query "<question>"',
  "  GRAPHIFY_API_KEY=<secret> node <installed-skill>/scripts/graphify.mjs --root <absolute-repository> serve",
  "",
  "The shared owner publishes one persistent code-only graph per explicit target root.",
  "Queries build a missing graph or update a stale graph unless GRAPHIFY_AUTO_REFRESH=0.",
  "Shared clients POST {\"question\":\"...\"} to http://<graph-host>:8080/query.",
  "Direct source and tests remain authoritative.",
  "",
].join("\n");

export class AdapterArgumentError extends Error {}
export class AdapterRuntimeError extends Error {}
export class GraphStateError extends AdapterRuntimeError {
  constructor(kind) {
    super(`graph is ${kind}`);
    this.kind = kind;
  }
}
export class FreshnessStateError extends AdapterRuntimeError {
  constructor(kind) {
    super(`graph freshness is ${kind}`);
    this.kind = kind;
  }
}
export class GraphLockError extends AdapterRuntimeError {}

export function parseCliArgs(argv) {
  if (!Array.isArray(argv)) {
    throw new AdapterArgumentError("invalid arguments");
  }
  const normalizedArgv =
    argv.length === 3 && argv[0] === "query" && argv[1] === "--"
      ? [argv[0], argv[2]]
      : argv;
  if (normalizedArgv.length === 1 && normalizedArgv[0] === "--help") {
    return Object.freeze({ action: "help" });
  }
  if (
    normalizedArgv.length === 1 &&
    ["status", "build", "update", "serve"].includes(normalizedArgv[0])
  ) {
    return Object.freeze({ action: normalizedArgv[0] });
  }
  if (normalizedArgv.length === 2 && normalizedArgv[0] === "query") {
    const question = normalizedArgv[1];
    if (
      typeof question !== "string" ||
      question.trim().length === 0 ||
      question.includes("\0") ||
      question.trimStart().startsWith("-") ||
      question.length > MAX_QUESTION_CHARS
    ) {
      throw new AdapterArgumentError("invalid query");
    }
    return Object.freeze({ action: "query", question });
  }
  throw new AdapterArgumentError("invalid arguments");
}

export function parseExactVersion(stdout) {
  if (typeof stdout !== "string") {
    return Object.freeze({ kind: "malformed" });
  }
  const versionLine = /^graphify ([0-9]+\.[0-9]+\.[0-9]+)(?:\r\n|\n)?(?![\s\S])/.exec(stdout);
  if (versionLine?.[1] === REQUIRED_VERSION) {
    return Object.freeze({ kind: "match", version: REQUIRED_VERSION });
  }
  if (versionLine) {
    return Object.freeze({ kind: "mismatch", version: versionLine[1] });
  }
  return Object.freeze({ kind: "malformed" });
}

function envValue(baseEnv, wantedKey, caseInsensitive) {
  if (!caseInsensitive) {
    return baseEnv[wantedKey];
  }
  const actualKey = Object.keys(baseEnv).find(
    (key) => key.toLowerCase() === wantedKey.toLowerCase(),
  );
  return actualKey === undefined ? undefined : baseEnv[actualKey];
}

function identityNumber(value) {
  if (typeof value === "bigint") {
    return value.toString(10);
  }
  if (typeof value === "number" && Number.isFinite(value)) {
    return String(value);
  }
  return "unavailable";
}

export function captureExecutableIdentity(stats) {
  if (!stats || typeof stats.isFile !== "function" || !stats.isFile()) {
    throw new AdapterRuntimeError("executable is not a regular file");
  }
  return Object.freeze({
    dev: identityNumber(stats.dev),
    ino: identityNumber(stats.ino),
    mode: identityNumber(stats.mode),
    size: identityNumber(stats.size),
    mtimeNs: identityNumber(stats.mtimeNs ?? stats.mtimeMs),
    ctimeNs: identityNumber(stats.ctimeNs ?? stats.ctimeMs),
  });
}

export function sameExecutableIdentity(left, right) {
  return Boolean(
    left &&
      right &&
      left.dev === right.dev &&
      left.ino === right.ino &&
      left.mode === right.mode &&
      left.size === right.size &&
      left.mtimeNs === right.mtimeNs &&
      left.ctimeNs === right.ctimeNs,
  );
}

export async function resolveNamedExecutable(
  executableName,
  {
    baseEnv = process.env,
    platform = process.platform,
    accessImpl = access,
    realpathImpl = realpath,
    statImpl = stat,
  } = {},
) {
  if (!["git", "graphify"].includes(executableName)) {
    throw new AdapterRuntimeError("unsupported executable name");
  }
  const windows = platform === "win32";
  const pathApi = windows ? path.win32 : path.posix;
  const pathValue = envValue(baseEnv, "PATH", windows) ?? "";
  const entries = pathValue.split(windows ? ";" : ":");
  const extensions = windows
    ? (envValue(baseEnv, "PATHEXT", true) ?? ".COM;.EXE;.BAT;.CMD")
        .split(";")
        .filter(Boolean)
        .map((extension) =>
          extension.startsWith(".") ? extension : `.${extension}`,
        )
    : [""];

  for (const entry of entries) {
    if (!entry || !pathApi.isAbsolute(entry)) {
      continue;
    }
    for (const extension of extensions) {
      const candidate = pathApi.join(entry, `${executableName}${extension}`);
      try {
        await accessImpl(candidate, fsConstants.X_OK);
        const resolvedPath = await realpathImpl(candidate);
        if (!pathApi.isAbsolute(resolvedPath)) {
          continue;
        }
        await accessImpl(resolvedPath, fsConstants.X_OK);
        const identity = captureExecutableIdentity(
          await statImpl(resolvedPath, { bigint: true }),
        );
        return Object.freeze({ path: resolvedPath, identity });
      } catch {
        // Missing, inaccessible, or non-regular candidates are unavailable.
      }
    }
  }
  return null;
}

export async function resolveGraphifyExecutable(options = {}) {
  return resolveNamedExecutable("graphify", options);
}

export async function recheckExecutableIdentity(
  executable,
  { accessImpl = access, realpathImpl = realpath, statImpl = stat } = {},
) {
  if (!executable || typeof executable.path !== "string" || !executable.identity) {
    throw new AdapterRuntimeError("missing executable identity");
  }
  await accessImpl(executable.path, fsConstants.X_OK);
  const resolvedPath = await realpathImpl(executable.path);
  if (resolvedPath !== executable.path) {
    throw new AdapterRuntimeError("executable path identity changed");
  }
  const currentIdentity = captureExecutableIdentity(
    await statImpl(executable.path, { bigint: true }),
  );
  if (!sameExecutableIdentity(executable.identity, currentIdentity)) {
    throw new AdapterRuntimeError("executable identity changed");
  }
  return executable;
}

export function buildVersionArgs() {
  return Object.freeze(["--version"]);
}

export function buildExtractArgs(repoRoot = REPO_ROOT) {
  return Object.freeze([
    "extract",
    repoRoot,
    "--code-only",
    "--max-workers",
    "1",
    "--out",
    repoRoot,
  ]);
}

export function buildClusterArgs(repoRoot = REPO_ROOT) {
  return Object.freeze(["cluster-only", repoRoot]);
}

export function buildUpdateArgs(repoRoot = REPO_ROOT) {
  return Object.freeze(["update", repoRoot, "--force"]);
}

export function buildQueryArgs(question, graphPath = GRAPH_PATH) {
  return Object.freeze(["query", question, "--graph", graphPath]);
}

export function buildChildEnv({
  baseEnv = process.env,
  tempRoot,
  platform = process.platform,
} = {}) {
  if (typeof tempRoot !== "string" || !path.isAbsolute(tempRoot)) {
    throw new AdapterRuntimeError("temporary root must be absolute");
  }
  const windows = platform === "win32";
  const childEnv = {};
  const safeKeys = windows
    ? ["PATH", "SystemRoot", "WINDIR", "ComSpec", "PATHEXT", "LANG", "LC_ALL", "LC_CTYPE"]
    : ["PATH", "LANG", "LC_ALL", "LC_CTYPE"];
  for (const key of safeKeys) {
    const value = envValue(baseEnv, key, windows);
    if (typeof value === "string") {
      childEnv[key] = value;
    }
  }

  Object.assign(childEnv, {
    HOME: path.join(tempRoot, "home"),
    XDG_CONFIG_HOME: path.join(tempRoot, "xdg-config"),
    XDG_CACHE_HOME: path.join(tempRoot, "xdg-cache"),
    XDG_DATA_HOME: path.join(tempRoot, "xdg-data"),
    TMPDIR: path.join(tempRoot, "tmp"),
    TMP: path.join(tempRoot, "tmp"),
    TEMP: path.join(tempRoot, "tmp"),
    GRAPHIFY_NO_BACKUP: "1",
    GRAPHIFY_QUERY_LOG_DISABLE: "1",
  });
  if (windows) {
    Object.assign(childEnv, {
      USERPROFILE: path.join(tempRoot, "home"),
      APPDATA: path.join(tempRoot, "xdg-config"),
      LOCALAPPDATA: path.join(tempRoot, "xdg-cache"),
    });
  }
  return Object.freeze(childEnv);
}

export function resolveServerApiKey(baseEnv = process.env, platform = process.platform) {
  const apiKey = envValue(baseEnv, "GRAPHIFY_API_KEY", platform === "win32");
  if (
    typeof apiKey !== "string" ||
    apiKey.trim().length < 16 ||
    apiKey.includes("\0") ||
    apiKey.includes("\r") ||
    apiKey.includes("\n")
  ) {
    throw new AdapterRuntimeError("GRAPHIFY_API_KEY must contain at least 16 safe characters");
  }
  return apiKey;
}

export function resolveServerPort(baseEnv = process.env, platform = process.platform) {
  const configuredPort = envValue(baseEnv, "GRAPHIFY_PORT", platform === "win32");
  if (configuredPort === undefined) {
    return SHARED_GRAPH_PORT;
  }
  if (
    typeof configuredPort !== "string" ||
    !/^[1-9][0-9]{0,4}$/.test(configuredPort)
  ) {
    throw new AdapterRuntimeError("GRAPHIFY_PORT must be a canonical port from 1 to 65535");
  }
  const port = Number(configuredPort);
  if (!Number.isSafeInteger(port) || port > 65_535) {
    throw new AdapterRuntimeError("GRAPHIFY_PORT must be a canonical port from 1 to 65535");
  }
  return port;
}

export function autoRefreshEnabled(baseEnv = process.env, platform = process.platform) {
  return envValue(baseEnv, "GRAPHIFY_AUTO_REFRESH", platform === "win32") !== "0";
}

export async function createPrivateTempRoot({
  platform = process.platform,
  tempDirectory = tmpdir(),
  mkdtempImpl = mkdtemp,
  mkdirImpl = mkdir,
  chmodImpl = chmod,
  realpathImpl = realpath,
  rmImpl = rm,
} = {}) {
  let created = null;
  try {
    created = await mkdtempImpl(path.join(tempDirectory, "graphify-"));
    if (platform !== "win32") {
      await chmodImpl(created, 0o700);
    }
    const tempRoot = await realpathImpl(created);
    if (!path.isAbsolute(tempRoot)) {
      throw new AdapterRuntimeError("temporary root is not absolute");
    }
    for (const directory of [
      "home",
      "xdg-config",
      "xdg-cache",
      "xdg-data",
      "tmp",
    ]) {
      await mkdirImpl(path.join(tempRoot, directory), {
        recursive: false,
        mode: 0o700,
      });
    }
    return tempRoot;
  } catch (error) {
    if (created !== null) {
      try {
        await rmImpl(created, { recursive: true, force: true });
      } catch {
        throw new AdapterRuntimeError("temporary root cleanup failed");
      }
    }
    throw error;
  }
}

const graphAccessQueues = new Map();

function lockDirectoryIdentity(stats) {
  if (!stats || typeof stats.isDirectory !== "function" || !stats.isDirectory() || stats.isSymbolicLink()) {
    throw new GraphLockError("graph lock directory is unsafe");
  }
  return Object.freeze({
    dev: identityNumber(stats.dev),
    ino: identityNumber(stats.ino),
  });
}

function sameLockDirectoryIdentity(left, right) {
  return Boolean(left && right && left.dev === right.dev && left.ino === right.ino);
}

function validLockOwner(value, ownerSchema = GRAPH_LOCK_SCHEMA) {
  const keys = ["createdAt", "nonce", "pid", "schema", "updatedAt"];
  return Boolean(
    value &&
      !Array.isArray(value) &&
      typeof value === "object" &&
      JSON.stringify(Object.keys(value).sort()) === JSON.stringify(keys) &&
      value.schema === ownerSchema &&
      Number.isSafeInteger(value.pid) &&
      value.pid > 0 &&
      typeof value.nonce === "string" &&
      /^[a-f0-9]{32}$/.test(value.nonce) &&
      Number.isSafeInteger(value.createdAt) &&
      Number.isSafeInteger(value.updatedAt) &&
      value.createdAt >= 0 &&
      value.updatedAt >= value.createdAt
  );
}

async function inspectGraphLock(
  lockPath,
  {
    lstatImpl = lstat,
    readFileImpl = readFile,
    pathApi = path,
    ownerSchema = GRAPH_LOCK_SCHEMA,
  } = {},
) {
  const stats = await lstatImpl(lockPath, { bigint: true });
  const identity = lockDirectoryIdentity(stats);
  const mtimeMs = Number(stats.mtimeMs);
  const ownerPath = pathApi.join(lockPath, "owner.json");
  try {
    const ownerStats = await lstatImpl(ownerPath);
    if (ownerStats.isSymbolicLink() || !ownerStats.isFile()) {
      return Object.freeze({ identity, mtimeMs, owner: null });
    }
    const owner = JSON.parse(await readFileImpl(ownerPath, "utf8"));
    return Object.freeze({
      identity,
      mtimeMs,
      owner: validLockOwner(owner, ownerSchema) ? owner : null,
    });
  } catch {
    return Object.freeze({ identity, mtimeMs, owner: null });
  }
}

function processIsDefinitelyDead(pid, killImpl = process.kill.bind(process)) {
  try {
    killImpl(pid, 0);
    return false;
  } catch (error) {
    return error?.code === "ESRCH";
  }
}

export async function assertGraphLockOwned(
  lock,
  {
    lstatImpl = lstat,
    readFileImpl = readFile,
    pathApi = path,
    ownerSchema = GRAPH_LOCK_SCHEMA,
  } = {},
) {
  let observed;
  try {
    observed = await inspectGraphLock(lock.path, {
      lstatImpl,
      readFileImpl,
      pathApi,
      ownerSchema,
    });
  } catch {
    throw new GraphLockError("graph lock ownership was lost");
  }
  if (
    !sameLockDirectoryIdentity(lock.identity, observed.identity) ||
    observed.owner?.nonce !== lock.nonce ||
    observed.owner?.pid !== lock.pid
  ) {
    throw new GraphLockError("graph lock ownership was lost");
  }
  return lock;
}

async function acquireGraphLockRecoveryFence({
  repoRoot,
  pid,
  now,
  monotonicNow,
  delay,
  nonceFactory,
  waitTimeoutMs,
  pollMs,
  initializationGraceMs,
  mkdirImpl,
  lstatImpl,
  readFileImpl,
  writeFileImpl,
  rmImpl,
  killImpl,
  pathApi,
}) {
  const fencePath = pathApi.join(repoRoot, ".graphify-owner.lock.recovery");
  const startedAt = monotonicNow();
  const elapsedMs = () => Math.max(0, monotonicNow() - startedAt);
  while (elapsedMs() <= waitTimeoutMs) {
    const nonce = nonceFactory();
    try {
      await mkdirImpl(fencePath, { recursive: false, mode: 0o700 });
      const createdIdentity = lockDirectoryIdentity(await lstatImpl(fencePath, { bigint: true }));
      const createdAt = now();
      const owner = Object.freeze({
        schema: GRAPH_LOCK_RECOVERY_SCHEMA,
        pid,
        nonce,
        createdAt,
        updatedAt: createdAt,
      });
      try {
        await writeFileImpl(pathApi.join(fencePath, "owner.json"), `${JSON.stringify(owner)}\n`, {
          encoding: "utf8",
          flag: "wx",
          mode: 0o600,
        });
        const observed = await inspectGraphLock(fencePath, {
          lstatImpl,
          readFileImpl,
          pathApi,
          ownerSchema: GRAPH_LOCK_RECOVERY_SCHEMA,
        });
        if (
          !sameLockDirectoryIdentity(createdIdentity, observed.identity) ||
          observed.owner?.nonce !== nonce ||
          observed.owner.pid !== pid
        ) {
          throw new GraphLockError("graph lock recovery fence initialization failed");
        }
        return Object.freeze({ path: fencePath, identity: observed.identity, nonce, pid });
      } catch (error) {
        try {
          const observed = await inspectGraphLock(fencePath, {
            lstatImpl,
            readFileImpl,
            pathApi,
            ownerSchema: GRAPH_LOCK_RECOVERY_SCHEMA,
          });
          if (
            sameLockDirectoryIdentity(createdIdentity, observed.identity) &&
            (observed.owner === null ||
              (observed.owner.nonce === nonce && observed.owner.pid === pid))
          ) {
            await rmImpl(fencePath, { recursive: true, force: true });
          }
        } catch {
          // Ambiguous recovery-fence ownership must remain fail-closed.
        }
        throw error;
      }
    } catch (error) {
      if (error?.code !== "EEXIST") {
        if (error instanceof GraphLockError) throw error;
        throw new GraphLockError("graph lock recovery fence acquisition failed");
      }
    }

    let observed;
    try {
      observed = await inspectGraphLock(fencePath, {
        lstatImpl,
        readFileImpl,
        pathApi,
        ownerSchema: GRAPH_LOCK_RECOVERY_SCHEMA,
      });
    } catch (error) {
      if (error?.code === "ENOENT") continue;
      throw new GraphLockError("graph lock recovery fence is unsafe");
    }
    if (
      (observed.owner !== null && processIsDefinitelyDead(observed.owner.pid, killImpl)) ||
      (observed.owner === null &&
        Number.isFinite(observed.mtimeMs) &&
        now() - observed.mtimeMs > initializationGraceMs)
    ) {
      throw new GraphLockError("graph lock recovery fence is abandoned");
    }
    if (elapsedMs() >= waitTimeoutMs) break;
    await delay(pollMs);
  }
  throw new GraphLockError("graph lock recovery fence contention timed out");
}

async function releaseGraphLockRecoveryFence(fence, options) {
  await assertGraphLockOwned(fence, {
    ...options,
    ownerSchema: GRAPH_LOCK_RECOVERY_SCHEMA,
  });
  await options.rmImpl(fence.path, { recursive: true, force: false });
}

async function quarantineAbandonedLock({
  repoRoot,
  pid,
  lockPath,
  observed,
  expectedNonce,
  now,
  monotonicNow,
  nonceFactory,
  initializationGraceMs,
  lstatImpl,
  readFileImpl,
  renameImpl,
  rmImpl,
  killImpl,
  pathApi,
  delay,
  pollMs,
  fenceWaitTimeoutMs,
  mkdirImpl,
  writeFileImpl,
}) {
  const fenceOptions = {
    repoRoot,
    pid,
    now,
    monotonicNow,
    delay,
    nonceFactory,
    waitTimeoutMs: fenceWaitTimeoutMs,
    pollMs,
    initializationGraceMs,
    mkdirImpl,
    lstatImpl,
    readFileImpl,
    writeFileImpl,
    rmImpl,
    killImpl,
    pathApi,
  };
  const fence = await acquireGraphLockRecoveryFence(fenceOptions);
  try {
    const current = await inspectGraphLock(lockPath, { lstatImpl, readFileImpl, pathApi });
    if (!sameLockDirectoryIdentity(observed.identity, current.identity)) return false;
    if (expectedNonce === null) {
      if (current.owner !== null || now() - current.mtimeMs <= initializationGraceMs) return false;
    } else if (
      current.owner?.nonce !== expectedNonce ||
      !processIsDefinitelyDead(current.owner.pid, killImpl)
    ) {
      return false;
    }

    const quarantinePath = `${lockPath}.quarantine-${nonceFactory()}`;
    try {
      await renameImpl(lockPath, quarantinePath);
    } catch (error) {
      if (error?.code === "ENOENT") return false;
      throw new GraphLockError("graph lock recovery failed");
    }
    let cleanupIsOwned = false;
    try {
      const quarantined = await inspectGraphLock(quarantinePath, {
        lstatImpl,
        readFileImpl,
        pathApi,
      });
      if (!sameLockDirectoryIdentity(observed.identity, quarantined.identity)) {
        throw new GraphLockError("graph lock recovery raced");
      }
      if (expectedNonce !== null && quarantined.owner?.nonce !== expectedNonce) {
        throw new GraphLockError("graph lock recovery raced");
      }
      cleanupIsOwned = true;
    } finally {
      if (cleanupIsOwned) {
        await rmImpl(quarantinePath, { recursive: true, force: true });
      }
    }
    return true;
  } finally {
    await releaseGraphLockRecoveryFence(fence, fenceOptions);
  }
}

export async function acquireGraphAccessLock({
  repoRoot = REPO_ROOT,
  pid = process.pid,
  now = Date.now,
  monotonicNow = performance.now.bind(performance),
  delay = (milliseconds) => new Promise((resolve) => setTimeout(resolve, milliseconds)),
  nonceFactory = () => randomBytes(16).toString("hex"),
  waitTimeoutMs = GRAPH_LOCK_WAIT_TIMEOUT_MS,
  pollMs = GRAPH_LOCK_POLL_MS,
  initializationGraceMs = GRAPH_LOCK_INITIALIZATION_GRACE_MS,
  mkdirImpl = mkdir,
  lstatImpl = lstat,
  readFileImpl = readFile,
  writeFileImpl = writeFile,
  renameImpl = rename,
  rmImpl = rm,
  killImpl = process.kill.bind(process),
  pathApi = path,
} = {}) {
  const lockPath = pathApi.join(repoRoot, ".graphify-owner.lock");
  const startedAt = monotonicNow();
  const elapsedMs = () => Math.max(0, monotonicNow() - startedAt);
  while (elapsedMs() <= waitTimeoutMs) {
    const nonce = nonceFactory();
    const fenceOptions = {
      repoRoot,
      pid,
      now,
      monotonicNow,
      delay,
      nonceFactory,
      waitTimeoutMs: Math.max(0, waitTimeoutMs - elapsedMs()),
      pollMs,
      initializationGraceMs,
      mkdirImpl,
      lstatImpl,
      readFileImpl,
      writeFileImpl,
      rmImpl,
      killImpl,
      pathApi,
    };
    const fence = await acquireGraphLockRecoveryFence(fenceOptions);
    let acquiredLock = null;
    let lockAlreadyExists = false;
    try {
      try {
        await mkdirImpl(lockPath, { recursive: false, mode: 0o700 });
      } catch (error) {
        if (error?.code === "EEXIST") {
          lockAlreadyExists = true;
        } else {
          throw error;
        }
      }
      if (!lockAlreadyExists) {
        const createdIdentity = lockDirectoryIdentity(await lstatImpl(lockPath, { bigint: true }));
        const createdAt = now();
        const owner = Object.freeze({
          schema: GRAPH_LOCK_SCHEMA,
          pid,
          nonce,
          createdAt,
          updatedAt: createdAt,
        });
        try {
          await writeFileImpl(pathApi.join(lockPath, "owner.json"), `${JSON.stringify(owner)}\n`, {
            encoding: "utf8",
            flag: "wx",
            mode: 0o600,
          });
          const observed = await inspectGraphLock(lockPath, { lstatImpl, readFileImpl, pathApi });
          if (
            !sameLockDirectoryIdentity(createdIdentity, observed.identity) ||
            observed.owner?.nonce !== nonce ||
            observed.owner.pid !== pid
          ) {
            throw new GraphLockError("graph lock initialization failed");
          }
          acquiredLock = Object.freeze({
            path: lockPath,
            identity: observed.identity,
            nonce,
            pid,
          });
        } catch (error) {
          try {
            const observed = await inspectGraphLock(lockPath, { lstatImpl, readFileImpl, pathApi });
            if (
              sameLockDirectoryIdentity(createdIdentity, observed.identity) &&
              (observed.owner === null ||
                (observed.owner.nonce === nonce && observed.owner.pid === pid))
            ) {
              await rmImpl(lockPath, { recursive: true, force: true });
            }
          } catch {
            // Leave ambiguous state for bounded contention/recovery rather than deleting another owner.
          }
          throw error;
        }
      }
    } catch (error) {
      if (error instanceof GraphLockError) throw error;
      throw new GraphLockError("graph lock acquisition failed");
    } finally {
      await releaseGraphLockRecoveryFence(fence, fenceOptions);
    }
    if (acquiredLock !== null) return acquiredLock;

    let observed;
    try {
      observed = await inspectGraphLock(lockPath, { lstatImpl, readFileImpl, pathApi });
    } catch (error) {
      if (error?.code !== "ENOENT") {
        throw new GraphLockError("graph lock is unsafe");
      }
      continue;
    }
    const deadOwner = observed.owner !== null && processIsDefinitelyDead(observed.owner.pid, killImpl);
    const abandonedInitialization = observed.owner === null &&
      Number.isFinite(observed.mtimeMs) &&
      now() - observed.mtimeMs > initializationGraceMs;
    if (deadOwner || abandonedInitialization) {
      const recovered = await quarantineAbandonedLock({
        repoRoot,
        pid,
        lockPath,
        observed,
        expectedNonce: observed.owner?.nonce ?? null,
        now,
        monotonicNow,
        nonceFactory,
        initializationGraceMs,
        lstatImpl,
        readFileImpl,
        renameImpl,
        rmImpl,
        killImpl,
        pathApi,
        delay,
        pollMs,
        fenceWaitTimeoutMs: Math.max(0, waitTimeoutMs - elapsedMs()),
        mkdirImpl,
        writeFileImpl,
      });
      if (recovered) continue;
    }
    if (elapsedMs() >= waitTimeoutMs) break;
    await delay(pollMs);
  }
  throw new GraphLockError("graph lock contention timed out");
}

export async function releaseGraphAccessLock(lock, options = {}) {
  const rmImpl = options.rmImpl ?? rm;
  await assertGraphLockOwned(lock, options);
  await rmImpl(lock.path, { recursive: true, force: false });
}

export async function withGraphAccessGate(operation, options = {}) {
  if (typeof operation !== "function") throw new GraphLockError("graph access operation is invalid");
  const queueKey = options.repoRoot ?? REPO_ROOT;
  const previous = graphAccessQueues.get(queueKey) ?? Promise.resolve();
  let releaseQueue;
  const current = new Promise((resolve) => { releaseQueue = resolve; });
  graphAccessQueues.set(queueKey, current);
  await previous.catch(() => undefined);

  let lock = null;
  try {
    lock = await acquireGraphAccessLock(options);
    const assertOwned = () => assertGraphLockOwned(lock, options);
    return await operation(assertOwned);
  } finally {
    try {
      if (lock !== null) await releaseGraphAccessLock(lock, options);
    } finally {
      releaseQueue();
      if (graphAccessQueues.get(queueKey) === current) graphAccessQueues.delete(queueKey);
    }
  }
}

function isContainedPath(rootPath, candidatePath, pathApi = path) {
  const relative = pathApi.relative(rootPath, candidatePath);
  return (
    relative !== "" &&
    !relative.startsWith(`..${pathApi.sep}`) &&
    relative !== ".." &&
    !pathApi.isAbsolute(relative)
  );
}

export async function validateOutputDirectory(
  repoRoot = REPO_ROOT,
  { lstatImpl = lstat, realpathImpl = realpath, pathApi = path } = {},
) {
  const realRepoRoot = await realpathImpl(repoRoot);
  const outputDirectory = pathApi.join(repoRoot, "graphify-out");
  let outputStats;
  try {
    outputStats = await lstatImpl(outputDirectory);
  } catch (error) {
    if (error?.code === "ENOENT") return outputDirectory;
    throw new AdapterRuntimeError("graph output directory is inaccessible");
  }
  if (outputStats.isSymbolicLink() || !outputStats.isDirectory()) {
    throw new AdapterRuntimeError("graph output directory is unsafe");
  }
  const realOutputDirectory = await realpathImpl(outputDirectory);
  if (!isContainedPath(realRepoRoot, realOutputDirectory, pathApi)) {
    throw new AdapterRuntimeError("graph output directory escaped repository root");
  }
  return outputDirectory;
}

export async function validateGraphPath(
  repoRoot = REPO_ROOT,
  { lstatImpl = lstat, realpathImpl = realpath, pathApi = path } = {},
) {
  let outputDirectory;
  try {
    outputDirectory = await validateOutputDirectory(repoRoot, {
      lstatImpl,
      realpathImpl,
      pathApi,
    });
  } catch (error) {
    throw new GraphStateError(error?.message?.includes("unsafe") || error?.message?.includes("escaped")
      ? "unsafe"
      : "missing");
  }
  const graphPath = pathApi.join(outputDirectory, "graph.json");
  let graphStats;
  try {
    graphStats = await lstatImpl(graphPath);
  } catch (error) {
    if (error?.code === "ENOENT") throw new GraphStateError("missing");
    throw new GraphStateError("unsafe");
  }
  if (graphStats.isSymbolicLink() || !graphStats.isFile() || graphStats.size === 0) {
    throw new GraphStateError("unsafe");
  }
  const realOutputDirectory = await realpathImpl(outputDirectory);
  const realGraphPath = await realpathImpl(graphPath);
  if (!isContainedPath(realOutputDirectory, realGraphPath, pathApi)) {
    throw new GraphStateError("unsafe");
  }
  return graphPath;
}

export async function sanitizeGraphHtml(
  repoRoot = REPO_ROOT,
  {
    lstatImpl = lstat,
    readFileImpl = readFile,
    realpathImpl = realpath,
    renameImpl = rename,
    rmImpl = rm,
    writeFileImpl = writeFile,
    pathApi = path,
    nonce = `${process.pid}-${Date.now()}`,
  } = {},
) {
  const outputDirectory = await validateOutputDirectory(repoRoot, {
    lstatImpl,
    realpathImpl,
    pathApi,
  });
  const htmlPath = pathApi.join(outputDirectory, "graph.html");
  let htmlStats;
  try {
    htmlStats = await lstatImpl(htmlPath);
  } catch (error) {
    if (error?.code === "ENOENT") return null;
    throw new AdapterRuntimeError("graph HTML is inaccessible");
  }
  if (htmlStats.isSymbolicLink() || !htmlStats.isFile()) {
    throw new AdapterRuntimeError("graph HTML is unsafe");
  }
  const realOutputDirectory = await realpathImpl(outputDirectory);
  const realHtmlPath = await realpathImpl(htmlPath);
  if (!isContainedPath(realOutputDirectory, realHtmlPath, pathApi)) {
    throw new AdapterRuntimeError("graph HTML escaped output directory");
  }

  const original = await readFileImpl(htmlPath, "utf8");
  const normalizedRepoRoot = repoRoot.replaceAll("\\", "/");
  let sanitized = original.replaceAll(repoRoot, ".");
  if (normalizedRepoRoot !== repoRoot) {
    sanitized = sanitized.replaceAll(normalizedRepoRoot, ".");
  }
  if (sanitized === original) return htmlPath;

  const temporaryPath = pathApi.join(outputDirectory, `.graphify-html-${nonce}.tmp`);
  try {
    await writeFileImpl(temporaryPath, sanitized, {
      encoding: "utf8",
      flag: "wx",
      mode: htmlStats.mode & 0o777,
    });
    await renameImpl(temporaryPath, htmlPath);
  } finally {
    await rmImpl(temporaryPath, { force: true });
  }
  return htmlPath;
}

function isGraphSourcePath(relativePath) {
  return typeof relativePath === "string" && relativePath.length > 0 &&
    relativePath.normalize("NFC") === relativePath && !relativePath.includes("\\") &&
    !relativePath.includes("\0") && !path.posix.isAbsolute(relativePath) &&
    relativePath.split("/").every((part) => part !== "." && part !== ".." && part !== "");
}

export function buildGitInventoryArgs(repoRoot) {
  return ["-C", repoRoot, "ls-files", "-z", "--cached", "--others", "--exclude-standard"];
}

export async function runNativeBridge(action, repoRoot, options = {}) {
  const executable = options.graphifyExecutable ?? await resolveGraphifyExecutable(options);
  if (!executable) throw new FreshnessStateError("unavailable");
  await (options.recheckIdentity ?? recheckExecutableIdentity)(executable);
  const shebang = (await readFile(executable.path, "utf8")).split("\n", 1)[0];
  const interpreter = /^#!(\/[^\r\n]+)$/.exec(shebang)?.[1];
  if (!interpreter || interpreter.includes(" ")) throw new FreshnessStateError("unavailable");
  const interpreterStats = await stat(interpreter, { bigint: true });
  const python = { path: await realpath(interpreter), identity: captureExecutableIdentity(interpreterStats) };
  await recheckExecutableIdentity(python);
  const result = await (options.runProcess ?? runBoundedProcess)({
    executable: interpreter,
    args: [path.join(path.dirname(fileURLToPath(import.meta.url)), "native_inventory.py"), action, repoRoot],
    cwd: repoRoot,
    env: options.childEnv ?? process.env,
    platform: options.platform ?? process.platform,
    ...STAGE_LIMITS.inventory,
    timeoutMs: action === "coverage" ? STAGE_LIMITS.build.timeoutMs : STAGE_LIMITS.inventory.timeoutMs,
  });
  if (!result.ok) throw new FreshnessStateError("coverage-incomplete");
  try { return JSON.parse(result.stdout); }
  catch { throw new FreshnessStateError("coverage-incomplete"); }
}

export async function listGraphSourcePaths(repoRoot, options = {}) {
  const inventory = await (options.listInventory ?? ((root, opts) => runNativeBridge("inventory", root, opts)))(repoRoot, options);
  if (inventory.failures.length) throw new FreshnessStateError("coverage-incomplete");
  return Object.freeze(inventory.sourcePaths);
}

export async function validateCandidateCoverage(snapshotRoot, expected, source, options = {}) {
  const report = await runNativeBridge("coverage", snapshotRoot, options);
  const actual = report.expected.map((entry) => entry.path);
  const wanted = expected.expected.map((entry) => entry.path);
  if (JSON.stringify(actual) !== JSON.stringify(wanted)) {
    report.failures.push({ path: "(inventory)", reason: "snapshot source inventory differs from live native discovery" });
  }
  report.excluded = expected.excluded;
  report.sourceSha256 = source.sourceSha256;
  report.complete = report.failures.length === 0;
  if (!report.complete) {
    for (const failure of report.failures) {
      process.stderr.write(`[graphify-coverage] ${JSON.stringify(failure)}\n`);
    }
    throw new FreshnessStateError("coverage-incomplete");
  }
  await writeFile(path.join(snapshotRoot, "graphify-out", "coverage.json"), `${JSON.stringify(report, null, 2)}\n`, { mode: 0o644 });
  return report;
}

async function safeContainedFile(
  repoRoot,
  relativePath,
  { lstatImpl = lstat, realpathImpl = realpath, pathApi = path } = {},
) {
  const pathParts = typeof relativePath === "string"
    ? relativePath.split(pathApi.sep === "\\" ? /[\\/]/u : /\//u)
    : [];
  if (
    typeof relativePath !== "string" ||
    relativePath.length === 0 ||
    relativePath.includes("\0") ||
    pathApi.isAbsolute(relativePath) ||
    pathParts.some((part) => part === "" || part === "." || part === "..")
  ) {
    throw new FreshnessStateError("unsafe");
  }
  const candidate = pathApi.join(repoRoot, relativePath);
  let current = repoRoot;
  for (let index = 0; index < pathParts.length; index += 1) {
    current = pathApi.join(current, pathParts[index]);
    const componentStats = await lstatImpl(current);
    if (
      componentStats.isSymbolicLink() ||
      (index < pathParts.length - 1 && !componentStats.isDirectory()) ||
      (index === pathParts.length - 1 && !componentStats.isFile())
    ) {
      throw new FreshnessStateError("unsafe");
    }
  }
  const realRepoRoot = await realpathImpl(repoRoot);
  const realCandidate = await realpathImpl(candidate);
  if (!isContainedPath(realRepoRoot, realCandidate, pathApi)) {
    throw new FreshnessStateError("unsafe");
  }
  return candidate;
}

async function artifactDigest(
  outputDirectory,
  filename,
  { lstatImpl = lstat, readFileImpl = readFile, realpathImpl = realpath, pathApi = path } = {},
) {
  const artifactPath = pathApi.join(outputDirectory, filename);
  const stats = await lstatImpl(artifactPath);
  if (stats.isSymbolicLink() || !stats.isFile() || stats.size === 0) {
    throw new FreshnessStateError("unsafe");
  }
  const realOutputDirectory = await realpathImpl(outputDirectory);
  const realArtifactPath = await realpathImpl(artifactPath);
  if (!isContainedPath(realOutputDirectory, realArtifactPath, pathApi)) {
    throw new FreshnessStateError("unsafe");
  }
  return createHash("sha256").update(await readFileImpl(artifactPath)).digest("hex");
}

async function readStableContainedFile(
  rootPath,
  relativePath,
  { lstatImpl = lstat, readFileImpl = readFile, realpathImpl = realpath, pathApi = path } = {},
) {
  const filePath = await safeContainedFile(rootPath, relativePath, {
    lstatImpl,
    realpathImpl,
    pathApi,
  });
  const beforeIdentity = captureExecutableIdentity(
    await lstatImpl(filePath, { bigint: true }),
  );
  const bytes = await readFileImpl(filePath);
  const afterStats = await lstatImpl(filePath, { bigint: true });
  const afterIdentity = captureExecutableIdentity(afterStats);
  if (!sameExecutableIdentity(beforeIdentity, afterIdentity)) {
    throw new FreshnessStateError("unstable-source");
  }
  return Object.freeze({ bytes, identity: afterIdentity, stats: afterStats });
}

export async function computeSourceFingerprint(
  repoRoot = REPO_ROOT,
  {
    listSourcePaths = listGraphSourcePaths,
    lstatImpl = lstat,
    readFileImpl = readFile,
    realpathImpl = realpath,
    pathApi = path,
    ...listOptions
  } = {},
) {
  const sourcePaths = await listSourcePaths(repoRoot, listOptions);
  const contentDigest = createHash("sha256");
  const generationDigest = createHash("sha256");
  for (const relativePath of sourcePaths) {
    let source;
    try {
      source = await readStableContainedFile(repoRoot, relativePath, {
        lstatImpl,
        readFileImpl,
        realpathImpl,
        pathApi,
      });
    } catch (error) {
      if (error instanceof FreshnessStateError && error.kind === "unsafe") throw error;
      throw new FreshnessStateError("unstable-source");
    }
    for (const digest of [contentDigest, generationDigest]) {
      digest.update(relativePath);
      digest.update("\0");
      digest.update(String(source.bytes.length));
      digest.update("\0");
      digest.update(source.bytes);
      digest.update("\0");
    }
    generationDigest.update(JSON.stringify(source.identity));
    generationDigest.update("\0");
  }
  return Object.freeze({
    sourceFileCount: sourcePaths.length,
    sourceSha256: contentDigest.digest("hex"),
    sourceGenerationSha256: generationDigest.digest("hex"),
  });
}

function validateSourceInventory(sourcePaths) {
  if (
    !Array.isArray(sourcePaths) ||
    sourcePaths.some((relativePath) => !isGraphSourcePath(relativePath)) ||
    sourcePaths.some((relativePath, index) => index > 0 && relativePath <= sourcePaths[index - 1])
  ) {
    throw new FreshnessStateError("unavailable");
  }
  return sourcePaths;
}

export async function createGraphSourceSnapshot(
  repoRoot = REPO_ROOT,
  tempRoot,
  {
    listSourcePaths = listGraphSourcePaths,
    lstatImpl = lstat,
    readFileImpl = readFile,
    realpathImpl = realpath,
    mkdirImpl = mkdir,
    mkdtempImpl = mkdtemp,
    chmodImpl = chmod,
    rmImpl = rm,
    utimesImpl = utimes,
    writeFileImpl = writeFile,
    pathApi = path,
    platform = process.platform,
    ...listOptions
  } = {},
) {
  if (typeof tempRoot !== "string" || !pathApi.isAbsolute(tempRoot)) {
    throw new AdapterRuntimeError("snapshot parent must be absolute");
  }
  let created = null;
  try {
    const realTempRoot = await realpathImpl(tempRoot);
    created = await mkdtempImpl(pathApi.join(realTempRoot, "source-"));
    const snapshotRoot = await realpathImpl(created);
    if (!isContainedPath(realTempRoot, snapshotRoot, pathApi)) {
      throw new AdapterRuntimeError("source snapshot escaped temporary root");
    }
    const sourcePaths = validateSourceInventory(
      await listSourcePaths(repoRoot, listOptions),
    );
    const contentDigest = createHash("sha256");
    const generationDigest = createHash("sha256");
    const sourceDirectories = new Set([snapshotRoot]);

    for (const relativePath of sourcePaths) {
      let source;
      try {
        source = await readStableContainedFile(repoRoot, relativePath, {
          lstatImpl,
          readFileImpl,
          realpathImpl,
          pathApi,
        });
      } catch (error) {
        if (error instanceof FreshnessStateError && error.kind === "unsafe") throw error;
        throw new FreshnessStateError("unstable-source");
      }
      const destination = pathApi.join(snapshotRoot, relativePath);
      const destinationDirectory = pathApi.dirname(destination);
      await mkdirImpl(destinationDirectory, { recursive: true, mode: 0o700 });
      let directory = destinationDirectory;
      while (isContainedPath(snapshotRoot, directory, pathApi)) {
        sourceDirectories.add(directory);
        directory = pathApi.dirname(directory);
      }
      await writeFileImpl(destination, source.bytes, { flag: "wx", mode: 0o600 });
      const atimeSeconds = typeof source.stats.atimeNs === "bigint"
        ? Number(source.stats.atimeNs) / 1e9
        : Number(source.stats.atimeMs) / 1e3;
      const mtimeSeconds = typeof source.stats.mtimeNs === "bigint"
        ? Number(source.stats.mtimeNs) / 1e9
        : Number(source.stats.mtimeMs) / 1e3;
      if (![atimeSeconds, mtimeSeconds].every(Number.isFinite)) {
        throw new FreshnessStateError("unstable-source");
      }
      await utimesImpl(destination, atimeSeconds, mtimeSeconds);
      const copied = await readStableContainedFile(snapshotRoot, relativePath, {
        lstatImpl,
        readFileImpl,
        realpathImpl,
        pathApi,
      });
      if (!copied.bytes.equals(source.bytes)) {
        throw new FreshnessStateError("unstable-source");
      }
      if (platform !== "win32") await chmodImpl(destination, 0o400);
      for (const digest of [contentDigest, generationDigest]) {
        digest.update(relativePath);
        digest.update("\0");
        digest.update(String(source.bytes.length));
        digest.update("\0");
        digest.update(source.bytes);
        digest.update("\0");
      }
      generationDigest.update(JSON.stringify(source.identity));
      generationDigest.update("\0");
    }

    const outputDirectory = pathApi.join(snapshotRoot, "graphify-out");
    await mkdirImpl(outputDirectory, { recursive: false, mode: 0o700 });
    if (platform !== "win32") {
      for (const directory of [...sourceDirectories]
        .sort((left, right) => right.length - left.length)) {
        await chmodImpl(directory, 0o500);
      }
    }
    if (listSourcePaths === listGraphSourcePaths) {
      const isolation = await runNativeBridge("snapshot-inventory", snapshotRoot, listOptions);
      if (isolation.failures.length) throw new FreshnessStateError("coverage-incomplete");
    }
    return Object.freeze({
      root: snapshotRoot,
      outputDirectory,
      inventory: await (listOptions.listInventory ?? ((root, opts) => runNativeBridge("inventory", root, opts)))(repoRoot, listOptions),
      sourceGeneration: Object.freeze({
        sourceFileCount: sourcePaths.length,
        sourceSha256: contentDigest.digest("hex"),
        sourceGenerationSha256: generationDigest.digest("hex"),
      }),
    });
  } catch (error) {
    if (created !== null) {
      try {
        await cleanupGraphSourceSnapshot(created, {
          platform,
          chmodImpl,
          lstatImpl,
          readdirImpl: readdir,
          rmImpl,
          pathApi,
        });
      } catch {
        throw new AdapterRuntimeError("source snapshot cleanup failed");
      }
    }
    throw error;
  }
}

export async function cleanupGraphSourceSnapshot(
  snapshotRoot,
  {
    platform = process.platform,
    chmodImpl = chmod,
    lstatImpl = lstat,
    readdirImpl = readdir,
    rmImpl = rm,
    pathApi = path,
  } = {},
) {
  if (platform !== "win32") {
    const makeWritable = async (candidate) => {
      let stats;
      try {
        stats = await lstatImpl(candidate);
      } catch (error) {
        if (error?.code === "ENOENT") return;
        throw error;
      }
      if (stats.isSymbolicLink()) throw new AdapterRuntimeError("source snapshot is unsafe");
      if (stats.isDirectory()) {
        await chmodImpl(candidate, 0o700);
        for (const entry of await readdirImpl(candidate)) {
          await makeWritable(pathApi.join(candidate, entry));
        }
      } else {
        await chmodImpl(candidate, 0o600);
      }
    };
    await makeWritable(snapshotRoot);
  }
  await rmImpl(snapshotRoot, { recursive: true, force: true });
}

export async function seedGraphUpdateSnapshot(
  repoRoot,
  snapshotRoot,
  {
    lstatImpl = lstat,
    readFileImpl = readFile,
    realpathImpl = realpath,
    writeFileImpl = writeFile,
    pathApi = path,
  } = {},
) {
  const outputDirectory = await validateOutputDirectory(snapshotRoot, {
    lstatImpl,
    realpathImpl,
    pathApi,
  });
  for (const filename of UPDATE_SEED_ARTIFACTS) {
    const source = await readStableContainedFile(
      pathApi.join(repoRoot, "graphify-out"),
      filename,
      { lstatImpl, readFileImpl, realpathImpl, pathApi },
    );
    await writeFileImpl(pathApi.join(outputDirectory, filename), source.bytes, {
      flag: "wx",
      mode: 0o600,
    });
  }
  await writeFileImpl(pathApi.join(outputDirectory, ".graphify_root"), snapshotRoot, {
    encoding: "utf8",
    flag: "wx",
    mode: 0o600,
  });
  return outputDirectory;
}

export async function sanitizeSnapshotArtifacts(
  snapshotRoot,
  {
    lstatImpl = lstat,
    readFileImpl = readFile,
    realpathImpl = realpath,
    writeFileImpl = writeFile,
    pathApi = path,
  } = {},
) {
  const outputDirectory = await validateOutputDirectory(snapshotRoot, {
    lstatImpl,
    realpathImpl,
    pathApi,
  });
  const pathForms = [...new Set([
    snapshotRoot,
    snapshotRoot.replaceAll("\\", "/"),
    snapshotRoot.replaceAll("/", "\\/"),
    pathToFileURL(snapshotRoot).href,
  ])].sort((left, right) => right.length - left.length);

  for (const filename of PUBLISHED_GRAPH_ARTIFACTS) {
    const artifact = await readStableContainedFile(outputDirectory, filename, {
      lstatImpl,
      readFileImpl,
      realpathImpl,
      pathApi,
    });
    const original = artifact.bytes.toString("utf8");
    if (!Buffer.from(original, "utf8").equals(artifact.bytes) || original.includes("\0")) {
      throw new AdapterRuntimeError("snapshot artifact is not safe text");
    }
    let sanitized = original;
    for (const privatePath of pathForms) sanitized = sanitized.replaceAll(privatePath, ".");
    if (filename.endsWith(".json") || filename.endsWith(".json.sig")) {
      try {
        JSON.parse(sanitized);
      } catch {
        throw new AdapterRuntimeError("snapshot artifact JSON is invalid");
      }
    }
    if (pathForms.some((privatePath) => sanitized.includes(privatePath))) {
      throw new AdapterRuntimeError("snapshot artifact leaks private path");
    }
    if (sanitized !== original) {
      await writeFileImpl(pathApi.join(outputDirectory, filename), sanitized, {
        encoding: "utf8",
        flag: "w",
        mode: 0o600,
      });
    }
  }
  await validateGraphPath(snapshotRoot, { lstatImpl, realpathImpl, pathApi });
  return outputDirectory;
}

export async function publishGraphSnapshot(
  snapshotRoot,
  repoRoot = REPO_ROOT,
  {
    assertOwned = async () => undefined,
    lstatImpl = lstat,
    readFileImpl = readFile,
    realpathImpl = realpath,
    mkdirImpl = mkdir,
    renameImpl = rename,
    rmImpl = rm,
    writeFileImpl = writeFile,
    pathApi = path,
    nonce = `${process.pid}-${randomBytes(8).toString("hex")}`,
    finalize = async () => undefined,
  } = {},
) {
  const snapshotOutput = await validateOutputDirectory(snapshotRoot, {
    lstatImpl,
    realpathImpl,
    pathApi,
  });
  let outputDirectory = await validateOutputDirectory(repoRoot, {
    lstatImpl,
    realpathImpl,
    pathApi,
  });
  try {
    await mkdirImpl(outputDirectory, { recursive: false, mode: 0o755 });
  } catch (error) {
    if (error?.code !== "EEXIST") throw error;
  }
  outputDirectory = await validateOutputDirectory(repoRoot, {
    lstatImpl,
    realpathImpl,
    pathApi,
  });
  const staged = [];
  const previous = new Map();
  const transactionFiles = [...PUBLISHED_GRAPH_ARTIFACTS, "coverage-freshness.json", ".graphify_root"];
  for (const filename of transactionFiles) {
    try {
      const original = await readStableContainedFile(outputDirectory, filename, {
        lstatImpl, readFileImpl, realpathImpl, pathApi,
      });
      previous.set(filename, original.bytes);
    } catch (error) {
      if (error?.code !== "ENOENT") throw error;
      previous.set(filename, null);
    }
  }
  let changed = false;
  const backups = [];
  try {
    for (const [filename, bytes] of previous) {
      if (bytes === null) continue;
      const backup = pathApi.join(outputDirectory, `.graphify-rollback-${nonce}-${filename}`);
      await writeFileImpl(backup, bytes, { flag: "wx", mode: 0o600 });
      backups.push({ filename, path: backup });
    }
    for (const filename of PUBLISHED_GRAPH_ARTIFACTS) {
      const source = await readStableContainedFile(snapshotOutput, filename, {
        lstatImpl,
        readFileImpl,
        realpathImpl,
        pathApi,
      });
      const stagedPath = pathApi.join(outputDirectory, `.graphify-publish-${nonce}-${filename}`);
      await writeFileImpl(stagedPath, source.bytes, { flag: "wx", mode: 0o644 });
      staged.push(Object.freeze({ filename, path: stagedPath }));
    }
    await assertOwned();
    changed = true;
    await rmImpl(pathApi.join(outputDirectory, "coverage-freshness.json"), { force: true });
    await assertOwned();
    await rmImpl(pathApi.join(outputDirectory, ".graphify_root"), { force: true });
    for (const artifact of staged) {
      await assertOwned();
      await renameImpl(artifact.path, pathApi.join(outputDirectory, artifact.filename));
    }
    await assertOwned();
    await finalize();
    await assertOwned();
    return outputDirectory;
  } catch (error) {
    if (changed) {
      // Recovery uses the real filesystem operations, independently of injected
      // candidate failures. The exclusive owner gate spans commit and rollback.
      for (const [filename, bytes] of previous) {
        const destination = pathApi.join(outputDirectory, filename);
        if (bytes === null) await rm(destination, { force: true });
        else await rename(backups.find((backup) => backup.filename === filename).path, destination);
      }
    }
    throw error;
  } finally {
    // Scratch disposal does not turn a committed publication into a failed
    // candidate; accepted bytes are independent of these private files.
    await Promise.all([...staged, ...backups].map((artifact) =>
      rm(artifact.path, { force: true }).catch(() => undefined)));
  }
}

function validateSourceGeneration(source) {
  if (
    !source ||
    !Number.isSafeInteger(source.sourceFileCount) ||
    source.sourceFileCount < 0 ||
    typeof source.sourceSha256 !== "string" ||
    !/^[a-f0-9]{64}$/.test(source.sourceSha256) ||
    typeof source.sourceGenerationSha256 !== "string" ||
    !/^[a-f0-9]{64}$/.test(source.sourceGenerationSha256)
  ) {
    throw new FreshnessStateError("unavailable");
  }
  return source;
}

async function currentFreshnessRecord(repoRoot, options = {}, provenSource = null) {
  const outputDirectory = await validateOutputDirectory(repoRoot, options);
  const coverageFile = await safeContainedFile(repoRoot, "graphify-out/coverage.json", options);
  const coverage = JSON.parse(await (options.readFileImpl ?? readFile)(coverageFile, "utf8"));
  if (coverage.schema !== "graphify.coverage.v1" || coverage.parserVersion !== REQUIRED_VERSION ||
      coverage.complete !== true || !Array.isArray(coverage.failures) || coverage.failures.length ||
      !Array.isArray(coverage.expected) || !Array.isArray(coverage.extracted) || !Array.isArray(coverage.excluded) ||
      coverage.expected.some((entry) => !isGraphSourcePath(entry.path) || typeof entry.parser !== "string") ||
      coverage.extracted.some((entry) => !isGraphSourcePath(entry.path) || !Number.isSafeInteger(entry.nodes) ||
        entry.nodes < 0 || !["error-free native no-symbol result", "current AST stamp and graph contribution"].includes(entry.disposition)) ||
      coverage.excluded.some((entry) => !isGraphSourcePath(entry.path) || typeof entry.reason !== "string" || !entry.reason) ||
      JSON.stringify(coverage.expected.map((entry) => entry.path)) !== JSON.stringify(coverage.extracted.map((entry) => entry.path)) ||
      new Set(coverage.expected.map((entry) => entry.path)).size !== coverage.expected.length) {
    throw new FreshnessStateError("coverage-incomplete");
  }
  const [artifactEntries, source] = await Promise.all([
    Promise.all(PUBLISHED_GRAPH_ARTIFACTS.map(async (filename) => Object.freeze([
      filename,
      await artifactDigest(outputDirectory, filename, options),
    ]))),
    provenSource === null
      ? computeSourceFingerprint(repoRoot, options)
      : Promise.resolve(validateSourceGeneration(provenSource)),
  ]);
  if (coverage.sourceSha256 !== source.sourceSha256) throw new FreshnessStateError("stale");
  return Object.freeze({
    schema: FRESHNESS_SCHEMA,
    graphifyVersion: REQUIRED_VERSION,
    artifactSha256: Object.freeze(Object.fromEntries(artifactEntries)),
    sourceSha256: source.sourceSha256,
    sourceFileCount: source.sourceFileCount,
  });
}

export async function writeFreshnessMetadata(
  repoRoot = REPO_ROOT,
  sourceGeneration,
  {
    assertOwned = async () => undefined,
    writeFileImpl = writeFile,
    renameImpl = rename,
    rmImpl = rm,
    pathApi = path,
    nonce = `${process.pid}-${Date.now()}`,
    ...options
  } = {},
) {
  const outputDirectory = await validateOutputDirectory(repoRoot, { pathApi, ...options });
  const record = await currentFreshnessRecord(
    repoRoot,
    { pathApi, ...options },
    validateSourceGeneration(sourceGeneration),
  );
  const freshnessPath = pathApi.join(outputDirectory, "coverage-freshness.json");
  const temporaryPath = pathApi.join(outputDirectory, `.coverage-freshness-${nonce}.tmp`);
  try {
    await writeFileImpl(temporaryPath, `${JSON.stringify(record, null, 2)}\n`, {
      encoding: "utf8",
      flag: "wx",
      mode: 0o644,
    });
    await assertOwned();
    await renameImpl(temporaryPath, freshnessPath);
  } finally {
    await rmImpl(temporaryPath, { force: true });
  }
  return freshnessPath;
}

export async function validateGraphFreshness(
  repoRoot = REPO_ROOT,
  { readFileImpl = readFile, pathApi = path, ...options } = {},
) {
  let expected;
  try {
    const freshnessPath = await safeContainedFile(
      repoRoot,
      pathApi.join("graphify-out", "coverage-freshness.json"),
      { pathApi, ...options },
    );
    expected = JSON.parse(await readFileImpl(freshnessPath, "utf8"));
  } catch (error) {
    if (error instanceof FreshnessStateError && error.kind === "unsafe") throw error;
    throw new FreshnessStateError("stale");
  }
  const expectedKeys = [
    "artifactSha256",
    "graphifyVersion",
    "schema",
    "sourceFileCount",
    "sourceSha256",
  ];
  if (
    !expected ||
    Array.isArray(expected) ||
    typeof expected !== "object" ||
    JSON.stringify(Object.keys(expected).sort()) !== JSON.stringify(expectedKeys) ||
    expected.schema !== FRESHNESS_SCHEMA ||
    expected.graphifyVersion !== REQUIRED_VERSION ||
    !expected.artifactSha256 ||
    Array.isArray(expected.artifactSha256) ||
    typeof expected.artifactSha256 !== "object" ||
    JSON.stringify(Object.keys(expected.artifactSha256).sort()) !==
      JSON.stringify([...PUBLISHED_GRAPH_ARTIFACTS].sort()) ||
    !Number.isSafeInteger(expected.sourceFileCount) ||
    expected.sourceFileCount < 0 ||
    ![...Object.values(expected.artifactSha256), expected.sourceSha256]
      .every((value) => typeof value === "string" && /^[a-f0-9]{64}$/.test(value))
  ) {
    throw new FreshnessStateError("stale");
  }
  let current;
  try {
    current = await currentFreshnessRecord(repoRoot, { pathApi, ...options });
  } catch (error) {
    if (error instanceof FreshnessStateError) throw error;
    throw new FreshnessStateError("stale");
  }
  if (
    PUBLISHED_GRAPH_ARTIFACTS.some(
      (filename) => expected.artifactSha256[filename] !== current.artifactSha256[filename],
    ) ||
    expected.sourceSha256 !== current.sourceSha256 ||
    expected.sourceFileCount !== current.sourceFileCount
  ) {
    throw new FreshnessStateError("stale");
  }
  return "current";
}

function sendTerminationSignal({ child, platform, killImpl }, signal) {
  if (!Number.isInteger(child.pid) || child.pid <= 0) {
    throw new AdapterRuntimeError("child process has no valid pid");
  }
  if (platform === "win32") {
    child.kill(signal);
    return;
  }
  killImpl(-child.pid, signal);
}

export async function runBoundedProcess({
  executable,
  args,
  cwd,
  env,
  timeoutMs,
  maxStdoutBytes,
  maxStderrBytes,
  platform = process.platform,
  spawnImpl = spawn,
  killImpl = process.kill.bind(process),
  terminationGraceMs = 500,
} = {}) {
  return new Promise((resolve) => {
    let child;
    try {
      child = spawnImpl(executable, args, {
        cwd,
        env,
        shell: false,
        detached: platform !== "win32",
        stdio: ["ignore", "pipe", "pipe"],
        windowsHide: true,
      });
    } catch {
      resolve(Object.freeze({ ok: false, failure: "spawn", code: null, signal: null, stdout: "", stderr: "" }));
      return;
    }

    if (!child || !child.stdout || !child.stderr || typeof child.once !== "function") {
      resolve(Object.freeze({ ok: false, failure: "spawn", code: null, signal: null, stdout: "", stderr: "" }));
      return;
    }

    const stdoutChunks = [];
    const stderrChunks = [];
    let stdoutBytes = 0;
    let stderrBytes = 0;
    let failure = null;
    let settled = false;
    let terminationStarted = false;
    let timeoutTimer;
    let forceTimer;
    let finalTimer;

    const finish = (code, signal) => {
      if (settled) return;
      settled = true;
      clearTimeout(timeoutTimer);
      clearTimeout(forceTimer);
      clearTimeout(finalTimer);
      const stdout = Buffer.concat(stdoutChunks, stdoutBytes).toString("utf8");
      const stderr = Buffer.concat(stderrChunks, stderrBytes).toString("utf8");
      const terminalFailure = failure ?? (signal ? "signal" : code === 0 ? null : "exit");
      resolve(Object.freeze({
        ok: terminalFailure === null,
        failure: terminalFailure,
        code,
        signal,
        stdout,
        stderr,
      }));
    };

    const terminate = (reason) => {
      if (failure === null) failure = reason;
      if (terminationStarted) return;
      terminationStarted = true;
      try {
        sendTerminationSignal({ child, platform, killImpl }, "SIGTERM");
      } catch {
        failure = "termination";
      }
      forceTimer ??= setTimeout(() => {
        try {
          sendTerminationSignal({ child, platform, killImpl }, "SIGKILL");
        } catch {
          failure = "termination";
        }
        finalTimer ??= setTimeout(() => finish(null, null), terminationGraceMs);
      }, terminationGraceMs);
    };

    const capture = (stream, chunk) => {
      const buffer = Buffer.isBuffer(chunk) ? chunk : Buffer.from(chunk);
      if (stream === "stdout") {
        if (stdoutBytes + buffer.length > maxStdoutBytes) {
          terminate("stdout-overflow");
          return;
        }
        stdoutChunks.push(buffer);
        stdoutBytes += buffer.length;
        return;
      }
      if (stderrBytes + buffer.length > maxStderrBytes) {
        terminate("stderr-overflow");
        return;
      }
      stderrChunks.push(buffer);
      stderrBytes += buffer.length;
    };

    child.stdout.on("data", (chunk) => capture("stdout", chunk));
    child.stderr.on("data", (chunk) => capture("stderr", chunk));
    child.once("error", () => {
      failure = "spawn";
      if (Number.isInteger(child.pid) && child.pid > 0) {
        terminate("spawn");
      } else {
        finish(null, null);
      }
    });
    child.once("close", (code, signal) => finish(code, signal));

    timeoutTimer = setTimeout(() => terminate("timeout"), timeoutMs);
  });
}

class SharedRequestError extends Error {
  constructor(statusCode, publicMessage) {
    super(publicMessage);
    this.statusCode = statusCode;
    this.publicMessage = publicMessage;
  }
}

function sendJson(response, statusCode, payload) {
  if (response.headersSent || response.destroyed) return;
  const body = `${JSON.stringify(payload)}\n`;
  response.writeHead(statusCode, {
    "Cache-Control": "no-store",
    "Content-Type": "application/json; charset=utf-8",
    "Content-Length": Buffer.byteLength(body),
    "X-Content-Type-Options": "nosniff",
  });
  response.end(body);
}

function credentialMatches(actual, expected) {
  if (typeof actual !== "string") return false;
  const actualBytes = Buffer.from(actual);
  const expectedBytes = Buffer.from(expected);
  return actualBytes.length === expectedBytes.length && timingSafeEqual(actualBytes, expectedBytes);
}

function requestIsAuthorized(request, apiKey) {
  const authorization = request.headers.authorization;
  const bearer = typeof authorization === "string" && authorization.startsWith("Bearer ")
    ? authorization.slice("Bearer ".length)
    : null;
  const headerKey = request.headers["x-api-key"];
  const explicitKey = Array.isArray(headerKey) ? null : headerKey;
  return credentialMatches(bearer, apiKey) || credentialMatches(explicitKey, apiKey);
}

async function readJsonRequest(request, maxBytes = MAX_REQUEST_BYTES) {
  const contentLength = Number(request.headers["content-length"] ?? 0);
  if (Number.isFinite(contentLength) && contentLength > maxBytes) {
    throw new SharedRequestError(413, "request_too_large");
  }
  const chunks = [];
  let bytes = 0;
  for await (const chunk of request) {
    const buffer = Buffer.isBuffer(chunk) ? chunk : Buffer.from(chunk);
    bytes += buffer.length;
    if (bytes > maxBytes) {
      throw new SharedRequestError(413, "request_too_large");
    }
    chunks.push(buffer);
  }
  let payload;
  try {
    payload = JSON.parse(Buffer.concat(chunks, bytes).toString("utf8"));
  } catch {
    throw new SharedRequestError(400, "invalid_json");
  }
  if (!payload || Array.isArray(payload) || typeof payload !== "object") {
    throw new SharedRequestError(400, "invalid_request");
  }
  const keys = Object.keys(payload).sort();
  if (keys.length !== 1 || keys[0] !== "question") {
    throw new SharedRequestError(400, "invalid_request");
  }
  try {
    return parseCliArgs(["query", payload.question]).question;
  } catch {
    throw new SharedRequestError(400, "invalid_question");
  }
}

export async function runQueryHttpServer({
  apiKey,
  checkHealth,
  executeQuery,
  host = SHARED_GRAPH_HOST,
  port = SHARED_GRAPH_PORT,
  maxConcurrentQueries = 4,
  shutdownTimeoutMs = HTTP_SHUTDOWN_TIMEOUT_MS,
  createServerImpl = createServer,
  signalEmitter = process,
  setTimeoutImpl = setTimeout,
  clearTimeoutImpl = clearTimeout,
  onListening = () => {},
} = {}) {
  if (
    typeof apiKey !== "string" ||
    typeof checkHealth !== "function" ||
    typeof executeQuery !== "function" ||
    !Number.isSafeInteger(maxConcurrentQueries) ||
    maxConcurrentQueries <= 0 ||
    !Number.isFinite(shutdownTimeoutMs) ||
    shutdownTimeoutMs < 0
  ) {
    return Object.freeze({ ok: false, failure: "configuration" });
  }

  return new Promise((resolve) => {
    let activeQueries = 0;
    let settled = false;
    let shuttingDown = false;
    let shutdownExpired = false;
    let shutdownTimer;
    let server;
    const activeExchanges = new Set();
    const connections = new Set();

    const removeSignalHandlers = () => {
      signalEmitter.removeListener("SIGINT", shutdown);
      signalEmitter.removeListener("SIGTERM", shutdown);
    };
    const removeServerHandlers = () => {
      server?.removeListener("connection", trackConnection);
      server?.removeListener("error", handleServerError);
    };
    const finish = (result) => {
      if (settled) return;
      settled = true;
      if (shutdownTimer !== undefined) {
        clearTimeoutImpl(shutdownTimer);
        shutdownTimer = undefined;
      }
      removeSignalHandlers();
      removeServerHandlers();
      resolve(Object.freeze(result));
    };
    const destroyTransport = (transport) => {
      try {
        transport?.destroy?.();
      } catch {
        // Shutdown remains fail-closed when a transport cannot be destroyed cleanly.
      }
    };
    const expireShutdown = () => {
      if (settled) return;
      shutdownExpired = true;
      for (const exchange of activeExchanges) {
        exchange.cancelled = true;
        destroyTransport(exchange.request);
        destroyTransport(exchange.response);
      }
      for (const socket of connections) destroyTransport(socket);
      finish({ ok: false, failure: "shutdown" });
    };
    const shutdown = () => {
      if (shuttingDown || settled) return;
      shuttingDown = true;
      try {
        shutdownTimer = setTimeoutImpl(expireShutdown, shutdownTimeoutMs);
        server.close((error) => {
          finish(error || shutdownExpired
            ? { ok: false, failure: "shutdown" }
            : { ok: true, failure: null });
        });
      } catch {
        expireShutdown();
      }
    };
    const trackConnection = (socket) => {
      connections.add(socket);
      socket.once("close", () => connections.delete(socket));
    };
    const handleServerError = () => {
      finish({ ok: false, failure: shuttingDown ? "shutdown" : "listen" });
    };

    try {
      server = createServerImpl(async (request, response) => {
        if (shuttingDown || settled) {
          destroyTransport(request);
          destroyTransport(response);
          return;
        }
        const exchange = { request, response, cancelled: false };
        activeExchanges.add(exchange);
        const reply = (statusCode, payload) => {
          if (settled || exchange.cancelled) return;
          sendJson(response, statusCode, payload);
        };
        try {
          if (!requestIsAuthorized(request, apiKey)) {
            reply(401, { schema: SHARED_GRAPH_SCHEMA, error: "unauthorized" });
            return;
          }
          if (request.method === "GET" && request.url === "/health") {
            try {
              const currentFreshness = await checkHealth();
              if (currentFreshness !== "current") throw new FreshnessStateError("stale");
              reply(200, {
                schema: SHARED_GRAPH_SCHEMA,
                status: "ready",
                graph: "repository",
                freshness: "current",
              });
            } catch {
              reply(503, {
                schema: SHARED_GRAPH_SCHEMA,
                status: "unavailable",
                graph: "repository",
                freshness: "stale",
              });
            }
            return;
          }
          if (request.url !== "/query") {
            reply(404, { schema: SHARED_GRAPH_SCHEMA, error: "not_found" });
            return;
          }
          if (request.method !== "POST") {
            reply(405, { schema: SHARED_GRAPH_SCHEMA, error: "method_not_allowed" });
            return;
          }
          const contentType = String(request.headers["content-type"] ?? "")
            .split(";", 1)[0]
            .trim()
            .toLowerCase();
          if (contentType !== "application/json") {
            reply(415, { schema: SHARED_GRAPH_SCHEMA, error: "json_required" });
            return;
          }
          if (activeQueries >= maxConcurrentQueries) {
            reply(429, { schema: SHARED_GRAPH_SCHEMA, error: "busy" });
            return;
          }

          activeQueries += 1;
          try {
            let question;
            try {
              question = await readJsonRequest(request);
            } catch (error) {
              const statusCode = error instanceof SharedRequestError ? error.statusCode : 400;
              const publicMessage = error instanceof SharedRequestError
                ? error.publicMessage
                : "invalid_request";
              reply(statusCode, { schema: SHARED_GRAPH_SCHEMA, error: publicMessage });
              return;
            }

            try {
              const answer = await executeQuery(question);
              if (typeof answer !== "string") {
                throw new AdapterRuntimeError("query returned invalid output");
              }
              reply(200, {
                schema: SHARED_GRAPH_SCHEMA,
                freshness: "current",
                answer,
              });
            } catch {
              reply(503, { schema: SHARED_GRAPH_SCHEMA, error: "query_unavailable" });
            }
          } finally {
            activeQueries -= 1;
          }
        } finally {
          activeExchanges.delete(exchange);
        }
      });
    } catch {
      finish({ ok: false, failure: "listen" });
      return;
    }

    server.headersTimeout = 5_000;
    server.requestTimeout = 10_000;
    server.keepAliveTimeout = 5_000;
    server.maxRequestsPerSocket = 100;
    signalEmitter.once("SIGINT", shutdown);
    signalEmitter.once("SIGTERM", shutdown);
    server.on("connection", trackConnection);
    server.once("error", handleServerError);
    server.listen(port, host, () => {
      try {
        onListening(server.address());
      } catch {
        shutdown();
      }
    });
  });
}

const MARKER_KEYS = Object.freeze([
  "schema",
  "event",
  "action",
  "stage",
  "mode",
  "outcome",
  "freshness",
  "requiredVersion",
  "code",
  "setupCommand",
  "timestamp",
  "durationMs",
  "attempt",
]);

export function serializeMarker(fields) {
  const marker = {};
  for (const key of MARKER_KEYS) {
    const value = fields[key];
    if (
      typeof value === "string" ||
      typeof value === "number" ||
      typeof value === "boolean" ||
      value === null
    ) {
      marker[key] = value;
    }
  }
  return JSON.stringify(marker);
}

function markerFields(action, fields) {
  return {
    schema: MARKER_SCHEMA,
    event: MARKER_EVENT,
    action,
    mode: "persistent-local-code-only",
    requiredVersion: REQUIRED_VERSION,
    setupCommand: SETUP_COMMAND,
    ...fields,
  };
}

function unavailableStatusText(versionKind, automaticRefresh = true) {
  const lines = [
    `Graphify ${REQUIRED_VERSION} is unavailable.`,
    `Query auto-refresh: ${automaticRefresh ? "enabled" : "disabled"}.`,
    `Install with: ${SETUP_COMMAND}`,
  ];
  if (versionKind === "mismatch") {
    lines.push(`Replace a mismatched installation with: ${FORCE_SETUP_COMMAND}`);
  }
  return `${lines.join("\n")}\n`;
}

function availableStatusText(graphReady, freshness = "unknown", automaticRefresh = true) {
  return [
    `Graphify ${REQUIRED_VERSION} is installed.`,
    `Shared graph: ${graphReady ? "ready" : "not built"}.`,
    graphReady ? `Freshness: ${freshness}.` : null,
    !graphReady ? "Freshness: missing." : null,
    `Query auto-refresh: ${automaticRefresh ? "enabled" : "disabled"}.`,
    graphReady ? "Graph: graphify-out/graph.json" : "Manual repair: node <installed-skill>/scripts/graphify.mjs --root <absolute-repository> build",
    graphReady ? "Refresh with: node <installed-skill>/scripts/graphify.mjs --root <absolute-repository> update" : null,
    graphReady ? "Shared HTTP query: GRAPHIFY_API_KEY=<secret> node <installed-skill>/scripts/graphify.mjs --root <absolute-repository> serve" : null,
    "",
  ].filter((line) => line !== null).join("\n");
}

function sameSourceGeneration(left, right) {
  return Boolean(
    left &&
      right &&
      left.sourceFileCount === right.sourceFileCount &&
      left.sourceSha256 === right.sourceSha256 &&
      left.sourceGenerationSha256 === right.sourceGenerationSha256,
  );
}

async function inspectGraphState({ repoRoot, validateGraph, validateFreshness, freshnessOptions }) {
  try {
    await validateGraph(repoRoot);
  } catch (error) {
    if (error instanceof GraphStateError && error.kind === "missing") return "missing";
    if (error instanceof GraphStateError && error.kind === "unsafe") return "unsafe";
    throw error;
  }
  try {
    await validateFreshness(repoRoot, freshnessOptions);
    return "current";
  } catch (error) {
    if (error instanceof FreshnessStateError) {
      return error.kind === "unsafe" ? "unsafe" : "stale";
    }
    throw error;
  }
}

export async function refreshStableGraph({
  repoRoot = REPO_ROOT,
  mode,
  executable,
  tempRoot,
  childEnv,
  platform,
  freshnessOptions,
  assertOwned,
  recheckIdentity,
  runProcess,
  validateOutput,
  validateGraph,
  createSnapshot,
  seedSnapshot,
  sanitizeSnapshot,
  publishSnapshot,
  cleanupSnapshot,
  computeSource,
  writeFreshness,
  validateFreshness,
  onLifecycle = () => {},
} = {}) {
  if (!["build", "update"].includes(mode)) {
    throw new AdapterRuntimeError("invalid refresh mode");
  }
  for (let attempt = 1; attempt <= 2; attempt += 1) {
    onLifecycle({ mode, attempt, outcome: "started" });
    let snapshot = null;
    try {
      await assertOwned();
      if (mode === "build") await validateOutput(repoRoot);
      else await validateGraph(repoRoot);
      try {
        snapshot = await createSnapshot(repoRoot, tempRoot, freshnessOptions);
        validateSourceGeneration(snapshot?.sourceGeneration);
      } catch (error) {
        if (error instanceof FreshnessStateError && error.kind === "unstable-source") {
          onLifecycle({ mode, attempt, outcome: "source-changed" });
          continue;
        }
        throw error;
      }
      if (mode === "update") await seedSnapshot(repoRoot, snapshot.root, freshnessOptions);
      await assertOwned();
      await recheckIdentity(executable);
      const result = await runProcess({
        executable: executable.path,
        args: mode === "build" ? buildExtractArgs(snapshot.root) : buildUpdateArgs(snapshot.root),
        cwd: snapshot.root,
        env: childEnv,
        platform,
        ...STAGE_LIMITS[mode],
      });
      if (!result.ok) throw new AdapterRuntimeError(`${mode} failed`);
      if (mode === "build") {
        await assertOwned();
        await recheckIdentity(executable);
        const clusterEnv = Object.freeze({
          ...childEnv,
          GRAPHIFY_VIZ_NODE_LIMIT: "20000",
        });
        const clusterResult = await runProcess({
          executable: executable.path,
          args: buildClusterArgs(snapshot.root),
          cwd: snapshot.root,
          env: clusterEnv,
          platform,
          ...STAGE_LIMITS.cluster,
        });
        if (!clusterResult.ok) throw new AdapterRuntimeError("cluster failed");
      }
      await assertOwned();
      await (freshnessOptions.validateCoverage ?? validateCandidateCoverage)(snapshot.root, snapshot.inventory, snapshot.sourceGeneration, freshnessOptions);
      await sanitizeSnapshot(snapshot.root, freshnessOptions);
      await assertOwned();
      await validateGraph(snapshot.root);
      let after;
      try {
        after = validateSourceGeneration(await computeSource(repoRoot, freshnessOptions));
      } catch (error) {
        if (error instanceof FreshnessStateError && error.kind === "unstable-source") {
          onLifecycle({ mode, attempt, outcome: "source-changed" });
          continue;
        }
        throw error;
      }
      if (!sameSourceGeneration(snapshot.sourceGeneration, after)) {
        onLifecycle({ mode, attempt, outcome: "source-changed" });
        continue;
      }
      await assertOwned();
      const finalize = async () => {
        await assertOwned();
        const committedSource = validateSourceGeneration(await computeSource(repoRoot, freshnessOptions));
        if (!sameSourceGeneration(snapshot.sourceGeneration, committedSource)) {
          throw new FreshnessStateError("unstable-source");
        }
        await writeFreshness(repoRoot, snapshot.sourceGeneration, { ...freshnessOptions, assertOwned });
        await assertOwned();
        const freshness = await validateFreshness(repoRoot, freshnessOptions);
        if (freshness !== "current") throw new FreshnessStateError("stale");
      };
      await publishSnapshot(snapshot.root, repoRoot, { ...freshnessOptions, assertOwned, finalize });
      onLifecycle({ mode, attempt, outcome: "success" });
      return "current";
    } catch (error) {
      onLifecycle({ mode, attempt, outcome: "failure" });
      throw error;
    } finally {
      if (snapshot !== null) await cleanupSnapshot(snapshot.root);
    }
  }
  throw new FreshnessStateError("unstable-source");
}

export async function main(argv = process.argv.slice(2), dependencies = {}) {
  const writeStdout = dependencies.writeStdout ?? ((text) => process.stdout.write(text));
  const writeStderr = dependencies.writeStderr ?? ((text) => process.stderr.write(text));
  let command;
  let repoRoot;
  try {
    const argumentsCopy = [...argv];
    const rootIndex = argumentsCopy.indexOf("--root");
    const suppliedRoot = rootIndex < 0 ? dependencies.repoRoot : argumentsCopy.splice(rootIndex, 2)[1];
    if (argumentsCopy.length === 1 && argumentsCopy[0] === "--help") {
      writeStdout(HELP_TEXT); return 0;
    }
    if (typeof suppliedRoot !== "string" || !path.isAbsolute(suppliedRoot)) throw new AdapterArgumentError("target root required");
    repoRoot = await realpath(suppliedRoot);
    if (repoRoot !== suppliedRoot || !(await stat(repoRoot)).isDirectory()) throw new AdapterArgumentError("target root must be canonical");
    command = parseCliArgs(argumentsCopy);
  } catch {
    writeStderr("Graphify adapter arguments are invalid. Use --help.\n");
    return 2;
  }
  if (command.action === "help") {
    writeStdout(HELP_TEXT);
    return 0;
  }

  const platform = dependencies.platform ?? process.platform;
  const baseEnv = dependencies.baseEnv ?? process.env;
  const automaticRefresh = autoRefreshEnabled(baseEnv, platform);
  const now = dependencies.now ?? Date.now;
  const resolveExecutable = dependencies.resolveExecutable ?? (() =>
    resolveGraphifyExecutable({ baseEnv, platform }));
  const createTempRoot = dependencies.createTempRoot ?? (() =>
    createPrivateTempRoot({ platform }));
  const cleanupTempRoot = dependencies.cleanupTempRoot ?? ((tempRoot) =>
    rm(tempRoot, { recursive: true, force: true }));
  const recheckIdentity = dependencies.recheckIdentity ?? recheckExecutableIdentity;
  const runProcess = dependencies.runProcess ?? runBoundedProcess;
  const serveHttp = dependencies.serveHttp ?? runQueryHttpServer;
  const validateGraph = dependencies.validateGraph ?? validateGraphPath;
  const validateOutput = dependencies.validateOutput ?? validateOutputDirectory;
  const createSnapshot = dependencies.createSnapshot ?? createGraphSourceSnapshot;
  const seedSnapshot = dependencies.seedSnapshot ?? seedGraphUpdateSnapshot;
  const sanitizeSnapshot = dependencies.sanitizeSnapshot ?? sanitizeSnapshotArtifacts;
  const publishSnapshot = dependencies.publishSnapshot ?? publishGraphSnapshot;
  const cleanupSnapshot = dependencies.cleanupSnapshot ?? ((snapshotRoot) =>
    cleanupGraphSourceSnapshot(snapshotRoot, { platform }));
  const validateFreshness = dependencies.validateFreshness ?? validateGraphFreshness;
  const writeFreshness = dependencies.writeFreshness ?? writeFreshnessMetadata;
  const computeSource = dependencies.computeSource ?? computeSourceFingerprint;
  const withGraphGate = dependencies.withGraphGate ?? ((operation) =>
    withGraphAccessGate(operation, { repoRoot: repoRoot }));
  const startedAt = now();

  writeStderr(`[graphify] Shared ${command.action} started\n`);
  writeStderr(`${serializeMarker(markerFields(command.action, {
    stage: "start",
    outcome: "started",
    timestamp: new Date(startedAt).toISOString(),
  }))}\n`);

  let code = 1;
  let outcome = "failure";
  let freshness = "unknown";
  let terminalStage = "resolve";
  let pendingStdout = "";
  let tempRoot = null;

  try {
    let executable = null;
    try {
      executable = await resolveExecutable();
    } catch {
      executable = null;
    }

    if (!executable) {
      code = 3;
      outcome = "unavailable";
      freshness = "unavailable";
      if (command.action === "status") pendingStdout = unavailableStatusText("missing", automaticRefresh);
    } else {
      tempRoot = await createTempRoot();
      const childEnv = buildChildEnv({ baseEnv, tempRoot, platform });
      const freshnessOptions = { baseEnv, childEnv, platform, runProcess, graphifyExecutable: executable, validateCoverage: dependencies.validateCoverage };
      terminalStage = "version";
      let versionResult = null;
      let versionIdentityAvailable = true;
      try {
        await recheckIdentity(executable);
      } catch {
        versionIdentityAvailable = false;
      }
      if (versionIdentityAvailable) {
        try {
          versionResult = await runProcess({
            executable: executable.path,
            args: buildVersionArgs(),
            cwd: repoRoot,
            env: childEnv,
            platform,
            ...STAGE_LIMITS.version,
          });
        } catch {
          versionResult = null;
        }
      }
      const versionState = versionResult?.ok
        ? parseExactVersion(versionResult.stdout)
        : Object.freeze({ kind: "malformed" });

      if (!versionIdentityAvailable || !versionResult?.ok || versionState.kind !== "match") {
        code = 3;
        outcome = "unavailable";
        freshness = "unavailable";
        if (command.action === "status") {
          pendingStdout = unavailableStatusText(versionState.kind, automaticRefresh);
        }
      } else {
        const refreshLifecycle = (action) => ({ mode, attempt, outcome: refreshOutcome }) => {
          writeStderr(`[graphify] Shared ${action} ${mode} attempt ${attempt} ${refreshOutcome}\n`);
          writeStderr(`${serializeMarker(markerFields(action, {
            stage: mode,
            outcome: refreshOutcome,
            freshness: refreshOutcome === "success" ? "current" : mode === "build" ? "missing" : "stale",
            attempt,
            timestamp: new Date(now()).toISOString(),
          }))}\n`);
        };
        const runRefresh = (mode, assertOwned, action) => refreshStableGraph({
          repoRoot,
          mode,
          executable,
          tempRoot,
          childEnv,
          platform,
          freshnessOptions,
          assertOwned,
          recheckIdentity,
          runProcess,
          validateOutput,
          validateGraph,
          createSnapshot,
          seedSnapshot,
          sanitizeSnapshot,
          publishSnapshot,
          cleanupSnapshot,
          computeSource,
          writeFreshness,
          validateFreshness,
          onLifecycle: refreshLifecycle(action),
        });
        const queryCurrentGraph = (question, action = "auto-refresh") => withGraphGate(async (assertOwned) => {
          await assertOwned();
          let state = await inspectGraphState({ repoRoot, validateGraph, validateFreshness, freshnessOptions });
          if (state === "unsafe") throw new FreshnessStateError("unsafe");
          if (state !== "current") {
            if (!automaticRefresh) {
              if (state === "missing") throw new GraphStateError("missing");
              throw new FreshnessStateError("stale");
            }
            await runRefresh(state === "missing" ? "build" : "update", assertOwned, action);
            state = await inspectGraphState({ repoRoot, validateGraph, validateFreshness, freshnessOptions });
            if (state !== "current") throw new FreshnessStateError(state);
          }
          await assertOwned();
          await recheckIdentity(executable);
          const queryResult = await runProcess({
            executable: executable.path,
            args: buildQueryArgs(question, path.join(repoRoot, "graphify-out", "graph.json")),
            cwd: repoRoot,
            env: childEnv,
            platform,
            ...STAGE_LIMITS.query,
          });
          if (!queryResult.ok) throw new AdapterRuntimeError("query failed");
          return queryResult.stdout;
        });

        if (command.action === "status") {
          terminalStage = "graph";
          const state = await withGraphGate(async (assertOwned) => {
            await assertOwned();
            return inspectGraphState({ repoRoot, validateGraph, validateFreshness, freshnessOptions });
          });
          freshness = state;
          if (state === "current") {
            code = 0;
            outcome = "success";
            const coverage = await (dependencies.readCoverage ?? (async () => JSON.parse(await readFile(path.join(repoRoot, "graphify-out", "coverage.json"), "utf8"))))();
            pendingStdout = availableStatusText(true, state, automaticRefresh) +
              `Coverage: ${coverage.expected.length} expected, ${coverage.extracted.length} extracted, ${coverage.excluded.length} excluded, ${coverage.failures.length} failures. Reasons: graphify-out/coverage.json\n`;
          } else if (state === "missing") {
            code = 4;
            outcome = "graph-missing";
            pendingStdout = availableStatusText(false, state, automaticRefresh);
          } else {
            code = 5;
            outcome = state;
            pendingStdout = availableStatusText(true, state, automaticRefresh);
          }
        } else if (command.action === "build" || command.action === "update") {
          terminalStage = command.action;
          freshness = await withGraphGate((assertOwned) =>
            runRefresh(command.action, assertOwned, "manual-refresh"));
          code = 0;
          outcome = "success";
          pendingStdout = command.action === "build"
            ? "Shared Graphify graph built at graphify-out/graph.json\n"
            : "Shared Graphify graph updated at graphify-out/graph.json\n";
        } else if (command.action === "query") {
          terminalStage = "query";
          pendingStdout = await queryCurrentGraph(command.question);
          freshness = "current";
          code = 0;
          outcome = "success";
        } else if (command.action === "serve") {
          terminalStage = "serve";
          const apiKey = resolveServerApiKey(baseEnv, platform);
          const port = resolveServerPort(baseEnv, platform);
          await recheckIdentity(executable);
          const executeQuery = async (question) => {
            const queryStartedAt = now();
            writeStderr("[graphify] Shared remote-query started\n");
            writeStderr(`${serializeMarker(markerFields("remote-query", {
              stage: "start",
              outcome: "started",
              timestamp: new Date(queryStartedAt).toISOString(),
            }))}\n`);
            let remoteCode = 1;
            let remoteOutcome = "failure";
            let remoteFreshness = "unknown";
            try {
              const answer = await queryCurrentGraph(question);
              remoteCode = 0;
              remoteOutcome = "success";
              remoteFreshness = "current";
              return answer;
            } finally {
              const queryFinishedAt = now();
              writeStderr(`${serializeMarker(markerFields("remote-query", {
                stage: "query",
                outcome: remoteOutcome,
                freshness: remoteFreshness,
                code: remoteCode,
                timestamp: new Date(queryFinishedAt).toISOString(),
                durationMs: Math.max(0, queryFinishedAt - queryStartedAt),
              }))}\n`);
            }
          };
          const checkHealth = () => withGraphGate(async (assertOwned) => {
            await assertOwned();
            const state = await inspectGraphState({ repoRoot, validateGraph, validateFreshness, freshnessOptions });
            if (state !== "current") throw new FreshnessStateError(state);
            return state;
          });
          const serverResult = await serveHttp({
            apiKey,
            executeQuery,
            checkHealth,
            port,
            onListening: () => {
              writeStderr(`[graphify] Shared shared query endpoint ready on port ${port}\n`);
            },
          });
          if (!serverResult.ok) throw new AdapterRuntimeError("server failed");
          code = 0;
          outcome = "success";
        }
      }
    }
  } catch (error) {
    if (error instanceof GraphStateError && error.kind === "missing") {
      code = 4;
      outcome = "graph-missing";
      freshness = "missing";
      if (command.action === "update") {
        pendingStdout = "Shared Graphify graph is not built. Run: node <installed-skill>/scripts/graphify.mjs --root <absolute-repository> build\n";
      } else {
        pendingStdout = "";
      }
    } else if (
      (error instanceof GraphStateError && error.kind === "unsafe") ||
      (error instanceof FreshnessStateError && ["stale", "unsafe"].includes(error.kind))
    ) {
      code = 5;
      outcome = error.kind;
      freshness = error.kind;
      pendingStdout = "";
    } else {
      code = 1;
      outcome = "failure";
      pendingStdout = "";
    }
  } finally {
    if (tempRoot !== null) {
      try {
        await cleanupTempRoot(tempRoot);
      } catch {
        code = 1;
        outcome = "failure";
        terminalStage = "cleanup";
        pendingStdout = "";
      }
    }
  }

  if (pendingStdout) writeStdout(pendingStdout);
  const finishedAt = now();
  writeStderr(`${serializeMarker(markerFields(command.action, {
    stage: terminalStage,
    outcome,
    freshness,
    code,
    timestamp: new Date(finishedAt).toISOString(),
    durationMs: Math.max(0, finishedAt - startedAt),
  }))}\n`);
  return code;
}

const directEntry = process.argv[1]
  ? pathToFileURL(path.resolve(process.argv[1])).href === import.meta.url
  : false;

if (directEntry) {
  main()
    .then((code) => {
      process.exitCode = code;
    })
    .catch(() => {
      process.stderr.write("Graphify adapter failed safely.\n");
      process.exitCode = 1;
    });
}

import { spawnSync } from "node:child_process";
import { existsSync, readdirSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

import { compareBytewise, sha256Hex, sourceDigest } from "./digest.mjs";
import { writeNotices } from "./notices.mjs";

/**
 * Builds the committed bundle.
 *
 * The manifest is the publish point: it is removed before the first
 * destructive step and written after the last, so a failed build leaves none,
 * and no manifest means no bundle to every reader — the Go tests, the server,
 * and the next run of this script.
 */

const APP_DIR = path.resolve(fileURLToPath(import.meta.url), "..", "..");
const BUNDLE_DIR = path.join(APP_DIR, "..", "bundle");
const DIST_DIR = path.join(BUNDLE_DIR, "dist");
const MANIFEST = path.join(BUNDLE_DIR, "bundle.json");

/** The version in .nvmrc and package.json engines.node. */
export const REQUIRED_NODE = "v24.21.0";

/** The one authority this project installs from and audits against. */
export const DECLARED_REGISTRY = "https://registry.npmjs.org/";

export const SCHEMA_VERSION = 1;

/**
 * The audit's argument vector. Every flag is on the command line, the one
 * configuration layer no environment variable and no npmrc outranks:
 * --no-offline forces the bulk request to be made at all, the three --include
 * flags clear every omission the configuration can express, and --registry
 * pins the authority the advisories come from.
 */
export function auditArgs(registry) {
  return [
    "audit",
    "--json",
    `--registry=${registry}`,
    "--no-offline",
    "--include=dev",
    "--include=optional",
    "--include=peer",
  ];
}

/**
 * Runs the audit and applies its five checks without exiting, so a fixture can
 * drive it. The verdict is green only when npm exited 0, wrote a JSON object,
 * counted no vulnerable package of any severity, and audited exactly the tree
 * the lockfile describes.
 */
export function runAudit(env, args, cwd) {
  const run = spawnSync("npm", args, { cwd, env, encoding: "utf8", maxBuffer: 64 * 1024 * 1024 });
  const stdout = run.stdout ?? "";
  const stderr = run.stderr ?? "";
  if (run.error !== undefined) {
    return { verdict: refusal(`npm could not be run: ${run.error.message}`), stdout, stderr };
  }
  if (run.status !== 0) {
    return { verdict: refusal(`npm audit exited ${String(run.status)}, not 0`), stdout, stderr };
  }

  let report;
  try {
    report = JSON.parse(stdout);
  } catch (error) {
    return { verdict: refusal(`npm audit did not write JSON: ${String(error)}`), stdout, stderr };
  }
  if (report === null || typeof report !== "object" || Array.isArray(report)) {
    return { verdict: refusal("npm audit did not write a JSON object"), stdout, stderr };
  }

  const vulnerabilities = report.vulnerabilities;
  const total = report.metadata?.vulnerabilities?.total;
  if (total !== 0) {
    return { verdict: refusal(`npm audit counts ${String(total)} vulnerable packages, not 0${listAdvisories(vulnerabilities)}`), stdout, stderr };
  }
  if (vulnerabilities === null || typeof vulnerabilities !== "object" || Array.isArray(vulnerabilities) || Object.keys(vulnerabilities).length !== 0) {
    return { verdict: refusal(`npm audit reports advisories despite a total of 0${listAdvisories(vulnerabilities)}`), stdout, stderr };
  }

  // The tree that was audited must be the tree the lockfile describes: an
  // audit of an empty or different tree counts no vulnerability either.
  const lockfile = path.join(cwd, "package-lock.json");
  if (!existsSync(lockfile)) {
    return { verdict: refusal(`${lockfile} is absent, so there is no tree to audit`), stdout, stderr };
  }
  const packages = JSON.parse(readFileSync(lockfile, "utf8")).packages ?? {};
  const expected = Object.keys(packages).length - 1;
  const audited = report.metadata?.dependencies?.total;
  if (audited !== expected) {
    return { verdict: refusal(`npm audit audited ${String(audited)} dependencies, the lockfile describes ${String(expected)}`), stdout, stderr };
  }
  return { verdict: { green: true, reason: "" }, stdout, stderr };
}

function refusal(reason) {
  return { green: false, reason };
}

function listAdvisories(vulnerabilities) {
  if (vulnerabilities === null || typeof vulnerabilities !== "object") {
    return "";
  }
  const named = Object.entries(vulnerabilities).map(([name, entry]) => `${name} (${String(entry?.severity)})`);
  return named.length === 0 ? "" : `: ${named.join(", ")}`;
}

function refuse(lines) {
  for (const line of lines) {
    process.stderr.write(`${line}\n`);
  }
  process.exit(1);
}

function run(command, args) {
  const result = spawnSync(command, args, { cwd: APP_DIR, stdio: "inherit" });
  if (result.error !== undefined) {
    refuse([`${command} could not be run: ${result.error.message}`]);
  }
  if (result.status !== 0) {
    refuse([`${command} ${args.join(" ")} exited ${String(result.status)}`]);
  }
}

function binary(name) {
  const executable = path.join(APP_DIR, "node_modules", ".bin", name);
  if (!existsSync(executable)) {
    refuse([`${executable} is absent; run npm ci --ignore-scripts in ${APP_DIR}`]);
  }
  return executable;
}

function installedVersion(name) {
  const manifest = path.join(APP_DIR, "node_modules", name, "package.json");
  if (!existsSync(manifest)) {
    refuse([`${manifest} is absent; run npm ci --ignore-scripts in ${APP_DIR}`]);
  }
  return JSON.parse(readFileSync(manifest, "utf8")).version;
}

/** Every file under dist, digested over the bytes as served. */
function distFiles(dir, prefix = "") {
  const files = [];
  for (const entry of readdirSync(prefix === "" ? dir : path.join(dir, prefix), { withFileTypes: true })) {
    const relative = prefix === "" ? entry.name : `${prefix}/${entry.name}`;
    if (entry.isDirectory()) {
      files.push(...distFiles(dir, relative));
      continue;
    }
    files.push({ path: relative, sha256: sha256Hex(readFileSync(path.join(dir, relative))) });
  }
  return files.sort((left, right) => compareBytewise(left.path, right.path));
}

async function main() {
  if (process.version !== REQUIRED_NODE) {
    refuse([
      `this bundle is built with Node ${REQUIRED_NODE}, and this is ${process.version}.`,
      `Install it with a version manager that honours ${path.join(APP_DIR, ".nvmrc")}.`,
    ]);
  }

  const { verdict, stdout, stderr } = runAudit(process.env, auditArgs(DECLARED_REGISTRY), APP_DIR);
  if (!verdict.green) {
    refuse([
      `the dependency audit refuses: ${verdict.reason}`,
      "",
      "npm audit stdout:",
      stdout,
      "npm audit stderr:",
      stderr,
      "There is no way to skip this audit. Change a version in the design's tables,",
      "or pin the transitive package with an overrides entry, each a revision of",
      "plans/user-interface/g1-s8-frontend-toolchain-design.md.",
    ]);
  }

  rmSync(MANIFEST, { force: true });

  run(binary("tsc"), ["--noEmit"]);
  run(binary("vite"), ["build"]);
  writeNotices(APP_DIR, DIST_DIR);

  const { digest, files: source } = sourceDigest(APP_DIR);
  const manifest = {
    schemaVersion: SCHEMA_VERSION,
    sourceDigest: digest,
    source,
    files: distFiles(DIST_DIR),
    tools: {
      node: process.version,
      vite: installedVersion("vite"),
      typescript: installedVersion("typescript"),
      tailwindcss: installedVersion("tailwindcss"),
    },
  };
  writeFileSync(MANIFEST, `${JSON.stringify(manifest, null, 2)}\n`);
  process.stdout.write(`wrote ${MANIFEST} for ${manifest.files.length} files, source ${digest}\n`);
}

const invokedDirectly = process.argv[1] !== undefined && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url);
if (invokedDirectly) {
  await main();
}

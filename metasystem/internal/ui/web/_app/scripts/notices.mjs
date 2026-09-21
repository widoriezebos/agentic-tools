import { existsSync, readdirSync, readFileSync, writeFileSync } from "node:fs";
import path from "node:path";

/**
 * Writes the open-source notices the page links to.
 *
 * The closure is read from the lockfile rather than from the bundler: a package
 * tree-shaking drops is still attributed, because over-attribution costs a few
 * kilobytes and under-attribution is a licence breach. Development
 * dependencies, which reach no shipped byte, are not listed.
 */

const HEADER = [
  "Open-source notices for the MetaSystem interface",
  "",
  "This file lists every package in the interface bundle's runtime dependency",
  "closure, with its licence. It is generated from package-lock.json by",
  "internal/ui/web/_app/scripts/notices.mjs and is not edited by hand.",
  "",
].join("\n");

export function writeNotices(appDir, distDir) {
  const lock = JSON.parse(readFileSync(path.join(appDir, "package-lock.json"), "utf8"));
  const packages = lock.packages ?? {};
  const root = packages[""] ?? {};

  const collected = new Map();
  const pending = Object.keys(root.dependencies ?? {}).map((name) => ({ name, from: "" }));
  while (pending.length > 0) {
    const { name, from } = pending.pop();
    const location = resolve(packages, from, name);
    if (location === null) {
      throw new Error(`package-lock.json resolves no ${name} from ${from === "" ? "the project root" : from}`);
    }
    const entry = packages[location];
    // dev and optional entries reach no shipped byte.
    if (entry.dev === true || entry.optional === true || collected.has(location)) {
      continue;
    }
    collected.set(location, read(appDir, location, entry));
    for (const required of Object.keys(entry.dependencies ?? {})) {
      pending.push({ name: required, from: location });
    }
  }

  const sections = [...collected.values()]
    .sort((left, right) => (left.name === right.name ? left.version.localeCompare(right.version) : left.name.localeCompare(right.name)))
    .map((entry) => [
      "-".repeat(76),
      `${entry.name} ${entry.version}`,
      entry.license,
      "",
      entry.text.trimEnd(),
      "",
    ].join("\n"));

  const file = path.join(distDir, "THIRD-PARTY-NOTICES.txt");
  writeFileSync(file, `${HEADER}\n${sections.join("\n")}`);
  return { file, packages: collected.size };
}

/** Node's own resolution: the nearest node_modules, then each parent's. */
function resolve(packages, from, name) {
  let prefix = from;
  for (;;) {
    const candidate = prefix === "" ? `node_modules/${name}` : `${prefix}/node_modules/${name}`;
    if (Object.hasOwn(packages, candidate)) {
      return candidate;
    }
    if (prefix === "") {
      return null;
    }
    const cut = prefix.lastIndexOf("/node_modules/");
    prefix = cut < 0 ? "" : prefix.slice(0, cut);
  }
}

function read(appDir, location, entry) {
  const dir = path.join(appDir, location);
  const manifest = JSON.parse(readFileSync(path.join(dir, "package.json"), "utf8"));
  const license = identifier(entry, manifest);
  if (license === "") {
    throw new Error(`${location} declares no licence`);
  }
  const text = licenceText(dir);
  if (text === null) {
    throw new Error(`${location} ships no LICENSE or LICENCE file`);
  }
  return { name: manifest.name, version: manifest.version, license, text };
}

function identifier(entry, manifest) {
  const declared = entry.license ?? manifest.license ?? manifest.licenses;
  if (typeof declared === "string") {
    return declared;
  }
  if (Array.isArray(declared)) {
    return declared.map((one) => (typeof one === "string" ? one : one.type)).join(" OR ");
  }
  if (declared !== null && typeof declared === "object" && typeof declared.type === "string") {
    return declared.type;
  }
  return "";
}

function licenceText(dir) {
  if (!existsSync(dir)) {
    return null;
  }
  const named = readdirSync(dir).filter((name) => /^licen[cs]e/i.test(name)).sort();
  if (named.length === 0) {
    return null;
  }
  return named.map((name) => readFileSync(path.join(dir, name), "utf8")).join("\n");
}

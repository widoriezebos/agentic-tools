import { createHash } from "node:crypto";
import { readdirSync, readFileSync } from "node:fs";
import path from "node:path";

/**
 * The staleness digest, the same rule as internal/ui/web/digest.go. Both
 * implementations assert one fixture literal, so neither can drift without
 * failing.
 *
 * Walk the directory. Skip any directory named node_modules and any entry whose
 * name begins with "." except .nvmrc and .npmrc. A symbolic link anywhere is an
 * error: its target could change the build without changing the digest. Every
 * remaining regular file contributes its path relative to the directory, with
 * "/" separators and ASCII only, and the SHA-256 of its content: for a text
 * file after every CRLF becomes LF, otherwise byte for byte. Modes and times
 * are ignored.
 */

const TEXT_EXTENSIONS = new Set([
  ".ts", ".tsx", ".mts", ".cts",
  ".js", ".mjs", ".cjs", ".jsx",
  ".json", ".css", ".html", ".svg",
  ".txt", ".md",
]);

const KEPT_DOT_FILES = new Set([".nvmrc", ".npmrc"]);

/** @returns {{ digest: string, files: { path: string, sha256: string }[] }} */
export function sourceDigest(dir) {
  const files = [];
  walk(dir, "", files);
  files.sort((a, b) => compareBytewise(a.path, b.path));
  return { digest: digestOf(files), files };
}

/** Renders the sorted per-file digests and takes the digest over that text. */
export function digestOf(files) {
  const text = files.map((file) => `${file.sha256}  ${file.path}\n`).join("");
  return `sha256:${sha256Hex(Buffer.from(text, "utf8"))}`;
}

export function sha256Hex(content) {
  return createHash("sha256").update(content).digest("hex");
}

export function compareBytewise(left, right) {
  return Buffer.compare(Buffer.from(left, "utf8"), Buffer.from(right, "utf8"));
}

function walk(root, prefix, files) {
  const entries = readdirSync(prefix === "" ? root : path.join(root, prefix), { withFileTypes: true });
  for (const entry of entries) {
    const name = entry.name;
    const relative = prefix === "" ? name : `${prefix}/${name}`;
    if (entry.isDirectory()) {
      if (name === "node_modules" || name.startsWith(".")) {
        continue;
      }
      walk(root, relative, files);
      continue;
    }
    if (name.startsWith(".") && !KEPT_DOT_FILES.has(name)) {
      continue;
    }
    if (entry.isSymbolicLink()) {
      throw new Error(`${relative} is a symbolic link; the interface source tree holds none`);
    }
    if (!entry.isFile()) {
      throw new Error(`${relative} is neither a regular file nor a directory`);
    }
    if (!isAscii(relative)) {
      throw new Error(`${relative} is not an ASCII path; the interface source tree holds none`);
    }
    const raw = readFileSync(path.join(root, relative));
    files.push({ path: relative, sha256: sha256Hex(isTextFile(name) ? normaliseLineEndings(raw) : raw) });
  }
}

function isTextFile(name) {
  if (KEPT_DOT_FILES.has(name)) {
    return true;
  }
  return TEXT_EXTENSIONS.has(path.extname(name).toLowerCase());
}

/** Drops the carriage return of every CRLF, leaving every other byte alone. */
function normaliseLineEndings(content) {
  const out = Buffer.alloc(content.length);
  let written = 0;
  for (let index = 0; index < content.length; index += 1) {
    if (content[index] === 0x0d && content[index + 1] === 0x0a) {
      continue;
    }
    out[written] = content[index];
    written += 1;
  }
  return out.subarray(0, written);
}

function isAscii(value) {
  for (let index = 0; index < value.length; index += 1) {
    if (value.charCodeAt(index) > 0x7f) {
      return false;
    }
  }
  return true;
}

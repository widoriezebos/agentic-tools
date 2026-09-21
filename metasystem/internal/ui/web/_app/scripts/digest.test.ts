import { mkdirSync, mkdtempSync, symlinkSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import { describe, expect, it } from "vitest";

import { sourceDigest } from "./digest.mjs";

// The fixture and its digest are written in the design; internal/ui/web asserts
// the same literal from the Go implementation of the same rule.
const FIXTURE_DIGEST = "sha256:f8c8104462398072962c697d4b860b1e76da903a0fa41933ffc541bcbb3e1399";

function write(dir: string, name: string, content: Buffer | string): void {
  const file = path.join(dir, name);
  mkdirSync(path.dirname(file), { recursive: true });
  writeFileSync(file, content);
}

function fixture(): string {
  const dir = mkdtempSync(path.join(tmpdir(), "metasystem-digest-"));
  write(dir, ".nvmrc", "24.21.0\n");
  write(dir, "a.txt", "x\r\n");
  write(dir, path.join("b", "c.txt"), "y\n");
  write(dir, "d.bin", Buffer.from([0x0d, 0x0a]));
  write(dir, ".hidden", "z");
  write(dir, path.join("b", "node_modules", "q.txt"), "q");
  return dir;
}

describe("sourceDigest", () => {
  it("digests the fixture to the literal in the design", () => {
    const { digest, files } = sourceDigest(fixture());
    expect(digest).toBe(FIXTURE_DIGEST);
    expect(files).toEqual([
      { path: ".nvmrc", sha256: "73fb1b615e2043a933be1c0895cde4358036acc28d785692509b822aa53c761f" },
      { path: "a.txt", sha256: "73cb3858a687a8494ca3323053016282f3dad39d42cf62ca4e79dda2aac7d9ac" },
      { path: "b/c.txt", sha256: "3bb2abb69ebb27fbfe63c7639624c6ec5e331b841a5bc8c3ebc10b9285e90877" },
      { path: "d.bin", sha256: "7eb70257593da06f682a3ddda54a9d260d4fc514f645237f5ca74b08f8da61a6" },
    ]);
  });

  it("normalises text by kind and never by content", () => {
    const dir = mkdtempSync(path.join(tmpdir(), "metasystem-digest-"));
    write(dir, "same.bin", Buffer.from([0x0d, 0x0a]));
    write(dir, "same.txt", Buffer.from([0x0d, 0x0a]));
    const { files } = sourceDigest(dir);
    expect(files).toEqual([
      { path: "same.bin", sha256: "7eb70257593da06f682a3ddda54a9d260d4fc514f645237f5ca74b08f8da61a6" },
      { path: "same.txt", sha256: "01ba4719c80b6fe911b091a7c05124b64eeece964e09c058ef8f9805daca546b" },
    ]);
  });

  it("refuses a symbolic link", () => {
    const dir = mkdtempSync(path.join(tmpdir(), "metasystem-digest-"));
    write(dir, "a.txt", "x\n");
    symlinkSync(path.join(dir, "a.txt"), path.join(dir, "b.txt"));
    expect(() => sourceDigest(dir)).toThrow(/b\.txt is a symbolic link/);
  });

  it("does not reach a link inside node_modules", () => {
    const dir = mkdtempSync(path.join(tmpdir(), "metasystem-digest-"));
    write(dir, "a.txt", "x\n");
    write(dir, path.join("node_modules", "p", "index.js"), "export {};\n");
    mkdirSync(path.join(dir, "node_modules", ".bin"), { recursive: true });
    symlinkSync("../p/index.js", path.join(dir, "node_modules", ".bin", "p"));
    const { files } = sourceDigest(dir);
    expect(files).toEqual([{ path: "a.txt", sha256: "73cb3858a687a8494ca3323053016282f3dad39d42cf62ca4e79dda2aac7d9ac" }]);
  });

  it("refuses a non-ASCII path", () => {
    const dir = mkdtempSync(path.join(tmpdir(), "metasystem-digest-"));
    write(dir, "späce.txt", "x\n");
    expect(() => sourceDigest(dir)).toThrow(/is not an ASCII path/);
  });
});

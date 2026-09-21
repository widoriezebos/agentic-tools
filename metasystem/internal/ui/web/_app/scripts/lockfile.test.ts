import { readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

import { DECLARED_REGISTRY } from "./bundle.mjs";

const APP_DIR = path.resolve(fileURLToPath(import.meta.url), "..", "..");

type LockfileEntry = { resolved?: string; version?: string };
type Lockfile = { packages?: Record<string, LockfileEntry> };

function lockfile(): Lockfile {
  return JSON.parse(readFileSync(path.join(APP_DIR, "package-lock.json"), "utf8")) as Lockfile;
}

describe("package-lock.json", () => {
  // react-style-singleton reads the nonce from the get-nonce module instance it
  // imports. A second copy in the tree would be a second instance, main.tsx
  // would set the nonce on one and the scroll lock would read the other, and
  // the style would be refused by the policy.
  it("holds exactly one copy of get-nonce, at 1.0.1", () => {
    const packages = lockfile().packages ?? {};
    const copies = Object.keys(packages).filter((name) => name.endsWith("/get-nonce"));
    expect(copies).toHaveLength(1);
    expect(packages[copies[0]]?.version).toBe("1.0.1");
  });

  // What was installed and what is audited must name the same authority.
  it("resolves every package from the declared registry", () => {
    const packages = lockfile().packages ?? {};
    const foreign = Object.entries(packages)
      .filter(([, entry]) => entry.resolved !== undefined && !entry.resolved.startsWith(DECLARED_REGISTRY))
      .map(([name, entry]) => `${name} -> ${String(entry.resolved)}`);
    expect(foreign).toEqual([]);
  });
});

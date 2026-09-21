import { readdirSync, readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

/**
 * One place writes a colour, and it is src/tokens.css.
 *
 * A colour written anywhere else is a colour that exists in one theme only,
 * that no contrast test ever sees, and that nobody can find again. The scan
 * covers everything the bundle includes; a test file ships nothing to a
 * browser, so the two guards below scan it only for a stylesheet.
 */

const SRC = path.resolve(fileURLToPath(import.meta.url), "..");
const TABLE = "tokens.css";

/** A hexadecimal colour. Written so that this pattern does not match itself. */
const HEX = /#([0-9a-fA-F]{3,8})\b/;
const FUNCTIONAL = /\b(rgb|rgba|hsl|hsla|oklch|lab|color)\(/;

function files(relative = ""): string[] {
  const found: string[] = [];
  for (const entry of readdirSync(relative === "" ? SRC : path.join(SRC, relative), { withFileTypes: true })) {
    const next = relative === "" ? entry.name : `${relative}/${entry.name}`;
    if (entry.isDirectory()) {
      found.push(...files(next));
      continue;
    }
    found.push(next);
  }
  return found.sort();
}

function offenders(candidates: string[]): string[] {
  const found: string[] = [];
  for (const file of candidates) {
    const contents = readFileSync(path.join(SRC, file), "utf8");
    for (const [number, line] of contents.split("\n").entries()) {
      if (HEX.test(line) || FUNCTIONAL.test(line)) {
        found.push(`${file}:${number + 1}: ${line.trim()}`);
      }
    }
  }
  return found;
}

describe("colour literals", () => {
  it("appear in no shipped source but the token table", () => {
    const shipped = files().filter(
      (file) => /\.(ts|tsx|css)$/.test(file) && !file.endsWith(".test.ts") && file !== TABLE,
    );
    expect(shipped.length).toBeGreaterThan(10);
    expect(offenders(shipped)).toEqual([]);
  });

  it("appear in no stylesheet but the token table, test or not", () => {
    const stylesheets = files().filter((file) => file.endsWith(".css") && file !== TABLE);
    expect(stylesheets).toContain("shell/shell.css");
    expect(offenders(stylesheets)).toEqual([]);
  });

  it("are what the table itself is made of", () => {
    expect(offenders([TABLE]).length).toBeGreaterThan(30);
  });
});

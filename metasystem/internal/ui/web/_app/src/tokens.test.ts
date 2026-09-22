import { readdirSync, readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

/**
 * The token table: what it must define, and that nothing uses a token it does
 * not define. Every colour exists in both themes, because a token defined in
 * one of them is a colour that disappears in the other.
 */

const SRC = path.resolve(fileURLToPath(import.meta.url), "..");
const TOKENS = path.join(SRC, "tokens.css");

/** The colours the design names, and the only ones the application may use. */
const COLOUR_TOKENS = [
  "bg",
  "surface",
  "surface-2",
  "surface-3",
  "border",
  "border-strong",
  "text",
  "text-2",
  "text-3",
  "accent",
  "accent-hover",
  "accent-fg",
  "marker",
  "marker-bg",
  "marker-fg",
  "ok",
  "ok-bg",
  "ok-fg",
  "danger",
  "scrim",
  "shadow",
];

/** The stacking order, which is the same in both themes. */
const LAYER_TOKENS = ["z-chrome", "z-scrim", "z-sheet", "z-tooltip"];

/** The declarations of one rule, by the selector that opens it. */
function block(css: string, selector: string): Map<string, string> {
  const opening = css.indexOf(`${selector} {`);
  if (opening < 0) {
    throw new Error(`tokens.css has no ${selector} block`);
  }
  const body = css.slice(opening, css.indexOf("}", opening));
  const declarations = new Map<string, string>();
  for (const line of body.split("\n")) {
    const match = /^\s*--ms-([a-z0-9-]+):\s*(.+);\s*$/.exec(line);
    if (match !== null) {
      declarations.set(match[1], match[2].trim());
    }
  }
  return declarations;
}

const css = readFileSync(TOKENS, "utf8");
const light = block(css, ":root");
const dark = block(css, ':root[data-theme="dark"]');

function sourceFiles(relative = ""): string[] {
  const found: string[] = [];
  for (const entry of readdirSync(relative === "" ? SRC : path.join(SRC, relative), { withFileTypes: true })) {
    const next = relative === "" ? entry.name : `${relative}/${entry.name}`;
    if (entry.isDirectory()) {
      found.push(...sourceFiles(next));
      continue;
    }
    found.push(next);
  }
  return found.sort();
}

describe("the token table", () => {
  it("defines every colour the design names, in both themes", () => {
    expect([...light.keys()].sort()).toEqual([...COLOUR_TOKENS].sort());
    expect([...dark.keys()].sort()).toEqual([...COLOUR_TOKENS].sort());
  });

  it("gives every token a different value in the two themes, or the same one on purpose", () => {
    // Nothing is inherited: a token missing from the dark block would fall
    // back to the light value silently, which is what this rules out.
    for (const name of COLOUR_TOKENS) {
      expect({ name, defined: dark.has(name) }).toEqual({ name, defined: true });
      expect({ name, empty: (dark.get(name) ?? "") === "" }).toEqual({ name, empty: false });
    }
  });

  it("sets a colour scheme with each theme", () => {
    expect(css).toContain("color-scheme: light;");
    expect(css).toContain("color-scheme: dark;");
  });

  it("maps every colour into Tailwind from the token, not from a copy of its value", () => {
    const mapping = css.slice(css.indexOf("@theme inline"));
    for (const name of COLOUR_TOKENS) {
      if (name === "scrim" || name === "shadow") {
        continue;
      }
      expect({ name, mapped: mapping.includes(`--color-${name}: var(--ms-${name});`) }).toEqual({ name, mapped: true });
    }
  });
});

describe("the application", () => {
  it("uses no token the table does not define", () => {
    const defined = new Set([...COLOUR_TOKENS, ...LAYER_TOKENS]);
    const unknown: string[] = [];
    for (const file of sourceFiles()) {
      const contents = readFileSync(path.join(SRC, file), "utf8");
      for (const match of contents.matchAll(/var\(--ms-([a-z0-9-]+)\)/g)) {
        if (!defined.has(match[1])) {
          unknown.push(`${file}: --ms-${match[1]}`);
        }
      }
    }
    expect(unknown).toEqual([]);
  });

  it("defines the layer tokens exactly once", () => {
    const definitions: string[] = [];
    for (const file of sourceFiles()) {
      const contents = readFileSync(path.join(SRC, file), "utf8");
      for (const match of contents.matchAll(/--ms-(z-[a-z]+):/g)) {
        definitions.push(match[1]);
      }
    }
    expect(definitions.sort()).toEqual([...LAYER_TOKENS].sort());
  });
});

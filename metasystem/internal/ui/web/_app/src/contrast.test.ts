import { readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

/**
 * Contrast, computed from the tokens as they are written rather than restated
 * from a table. Every pair below is a place where one token is actually drawn
 * on another; text needs 4.5:1 and non-text 3:1, and the ratio is WCAG 2.1's
 * relative luminance.
 *
 * Decorative rules, the skeleton, and disabled controls are exempt by 1.4.3
 * and 1.4.11 and are not asserted here, because asserting a rule nobody has to
 * read would push the borders darker for nothing.
 */

const TOKENS = path.resolve(fileURLToPath(import.meta.url), "..", "tokens.css");
const css = readFileSync(TOKENS, "utf8");

/**
 * The declarations of one rule, read from the stylesheet itself. The parser is
 * deliberately small and lives here rather than being shared with another
 * test: a guard that reads the file it guards should not depend on the file it
 * is guarding against.
 */
function block(selector: string): Map<string, string> {
  const opening = css.indexOf(`${selector} {`);
  if (opening < 0) {
    throw new Error(`tokens.css has no ${selector} block`);
  }
  const declarations = new Map<string, string>();
  for (const line of css.slice(opening, css.indexOf("}", opening)).split("\n")) {
    const match = /^\s*--ms-([a-z0-9-]+):\s*(.+);\s*$/.exec(line);
    if (match !== null) {
      declarations.set(match[1], match[2].trim());
    }
  }
  return declarations;
}

const themes = {
  light: block(":root"),
  dark: block(':root[data-theme="dark"]'),
};

const TEXT = 4.5;
const NON_TEXT = 3;

/** Where each pair is drawn, so a failure names something a human can see. */
const pairs: { front: string; back: string; needs: number; where: string }[] = [
  ...surfaces("text", TEXT, "body text"),
  ...surfaces("text-2", TEXT, "secondary text"),
  ...surfaces("text-3", TEXT, "the slice line and placeholders"),
  { front: "accent", back: "bg", needs: TEXT, where: "a link in the work area" },
  { front: "accent", back: "surface", needs: TEXT, where: "a link on a card" },
  { front: "accent", back: "surface-2", needs: NON_TEXT, where: "the focus ring on the rail" },
  { front: "accent", back: "surface-3", needs: NON_TEXT, where: "the current rail row's icon" },
  { front: "accent-fg", back: "accent", needs: TEXT, where: "the primary button" },
  { front: "accent-fg", back: "accent-hover", needs: TEXT, where: "the primary button, hovered" },
  { front: "marker-fg", back: "marker-bg", needs: TEXT, where: "the self-hosted chip" },
  { front: "marker", back: "surface-2", needs: NON_TEXT, where: "the self-hosted stripe" },
  { front: "danger", back: "surface-2", needs: TEXT, where: "the identity conflict" },
  { front: "danger", back: "surface", needs: TEXT, where: "an error boundary's heading" },
  { front: "ok", back: "surface-2", needs: NON_TEXT, where: "the connected dot" },
  { front: "border-strong", back: "bg", needs: NON_TEXT, where: "the separator's resting line" },
  { front: "border-strong", back: "surface", needs: NON_TEXT, where: "an input boundary" },
  { front: "border-strong", back: "surface-2", needs: NON_TEXT, where: "a button in the header" },
  { front: "bg", back: "text", needs: TEXT, where: "a tooltip" },
];

function surfaces(front: string, needs: number, where: string) {
  return ["bg", "surface", "surface-2", "surface-3"].map((back) => ({ front, back, needs, where }));
}

/** WCAG 2.1 relative luminance of a six-digit hexadecimal colour. */
function luminance(colour: string): number {
  const digits = /^#([0-9a-f]{6})$/.exec(colour.trim());
  if (digits === null) {
    throw new Error(`not a six-digit colour: ${colour}`);
  }
  const channels = [0, 2, 4].map((offset) => {
    const value = Number.parseInt(digits[1].slice(offset, offset + 2), 16) / 255;
    return value <= 0.03928 ? value / 12.92 : ((value + 0.055) / 1.055) ** 2.4;
  });
  return 0.2126 * channels[0] + 0.7152 * channels[1] + 0.0722 * channels[2];
}

function contrast(front: string, back: string): number {
  const [lighter, darker] = [luminance(front), luminance(back)].sort((left, right) => right - left);
  return (lighter + 0.05) / (darker + 0.05);
}

describe("contrast", () => {
  it("is computed, not restated", () => {
    // Black on white is 21:1 and a colour on itself is 1:1. If these two are
    // wrong, every row below is meaningless.
    expect(contrast("#000000", "#ffffff")).toBeCloseTo(21, 5);
    expect(contrast("#3462d9", "#3462d9")).toBeCloseTo(1, 5);
  });

  for (const theme of ["light", "dark"] as const) {
    describe(theme, () => {
      const tokens = themes[theme];

      for (const pair of pairs) {
        it(`${pair.front} on ${pair.back} carries ${pair.where}`, () => {
          const front = tokens.get(pair.front);
          const back = tokens.get(pair.back);
          expect({ front: pair.front, defined: front !== undefined }).toEqual({ front: pair.front, defined: true });
          expect({ back: pair.back, defined: back !== undefined }).toEqual({ back: pair.back, defined: true });
          const ratio = contrast(front as string, back as string);
          expect({ pair: `${pair.front} on ${pair.back}`, ratio: ratio >= pair.needs }).toEqual({
            pair: `${pair.front} on ${pair.back}`,
            ratio: true,
          });
        });
      }
    });
  }

  it("asserts every colour token that is drawn on another", () => {
    const asserted = new Set(pairs.flatMap((pair) => [pair.front, pair.back]));
    const unasserted = [...themes.light.keys()].filter((name) => !asserted.has(name));
    // border is a decorative rule, the scrim and the shadow are translucent,
    // and surface-3 appears as a background above.
    expect(unasserted.sort()).toEqual(["border", "scrim", "shadow"]);
  });
});

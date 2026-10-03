import { readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

/**
 * What gives way when the shell is narrow, read from the stylesheet.
 *
 * No test here lays a page out, so these hold the rules the layout follows
 * rather than the pixels it lands on: on a phone the section's name keeps its
 * width and the workspace's identity is what shrinks, and beside the Project
 * Partner's field the Seeing chip is cut before the field is.
 */

/** The stylesheet without its comments, which hold braces and commas of their own. */
const CSS = readFileSync(path.resolve(fileURLToPath(import.meta.url), "..", "shell.css"), "utf8").replace(
  /\/\*[\s\S]*?\*\//g,
  "",
);

/** The declarations of every rule whose selector list names `selector`, inside `sheet`. */
function declared(sheet: string, selector: string): string {
  const found: string[] = [];
  for (const rule of sheet.matchAll(/([^{}]+)\{([^{}]*)\}/g)) {
    const selectors = rule[1].split(",").map((one) => one.trim());
    if (selectors.includes(selector)) {
      found.push(rule[2]);
    }
  }
  return found.join("\n");
}

/** Everything inside the phone's media blocks, as one sheet. */
function phone(): string {
  const blocks: string[] = [];
  const opening = "@media (max-width: 599px) {";
  for (let at = CSS.indexOf(opening); at >= 0; at = CSS.indexOf(opening, at + 1)) {
    let depth = 1;
    let end = at + opening.length;
    while (depth > 0 && end < CSS.length) {
      depth += CSS[end] === "{" ? 1 : CSS[end] === "}" ? -1 : 0;
      end += 1;
    }
    blocks.push(CSS.slice(at + opening.length, end - 1));
  }
  return blocks.join("\n");
}

describe("the header on a phone", () => {
  it("keeps the section's name whole: the name does not shrink", () => {
    expect(declared(phone(), ".ms-header-name")).toMatch(/flex:\s*0 0 auto/);
  });

  it("lets the workspace's identity give way instead, cut to what is left", () => {
    const identity = declared(phone(), ".ms-header-left > .ms-identity");
    expect(identity).toMatch(/flex:\s*0 1 auto/);
    expect(identity).toMatch(/overflow:\s*hidden/);
  });

  it("shows neither the mode chip nor the rule beside it, so no rule stands alone", () => {
    expect(declared(phone(), ".ms-header-rule")).toMatch(/display:\s*none/);
    expect(declared(phone(), ".ms-identity > .ms-chip")).toMatch(/display:\s*none/);
    expect(declared(phone(), ".ms-identity > .ms-identity-note")).toMatch(/display:\s*none/);
  });

  it("leaves the controls on the right at their own width", () => {
    expect(declared(CSS, ".ms-header-right")).toMatch(/flex:\s*0 0 auto/);
    expect(declared(phone(), ".ms-header-right")).not.toMatch(/flex|width/);
  });
});

describe("the Seeing chip beside the Project Partner's field", () => {
  it("gives way before the field does: the field keeps a floor and the chip is cut", () => {
    expect(declared(CSS, ".ms-drawer-field")).toMatch(/min-width:\s*\d{3}px/);
    const chip = declared(CSS, ".ms-drawer-about .ms-seeing-chip");
    expect(chip).toMatch(/overflow:\s*hidden/);
    expect(chip).toMatch(/text-overflow:\s*ellipsis/);
  });
});

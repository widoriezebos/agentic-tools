import { describe, expect, it } from "vitest";

import { EDITOR_THEME, HIGHLIGHT, shortcuts } from "./styling";

/**
 * The guard over the one stylesheet this application does not write in CSS.
 *
 * src/literals.test.ts reads .css and .ts files for a colour written as a
 * value, and would catch a hexadecimal here. What it cannot say is that a
 * colour was written at all: `color: "black"`, `"currentColor"` or
 * `"Highlight"` are colours no token names and no theme follows, and they
 * would pass every other guard in this build. So the rule here is the stronger
 * one: a property whose name says colour carries a token and nothing else.
 */

/** Every property whose value is a colour, whatever it is called. */
function coloursIn(style: Record<string, unknown>): [string, unknown][] {
  return Object.entries(style).filter(([property]) => /colou?r/i.test(property));
}

describe("the Markdown highlight", () => {
  it("takes every colour from the token table", () => {
    const written: string[] = [];
    for (const rule of HIGHLIGHT) {
      for (const [property, value] of coloursIn(rule as unknown as Record<string, unknown>)) {
        if (typeof value !== "string" || !value.startsWith("var(--ms-")) {
          written.push(`${property}: ${String(value)}`);
        }
      }
    }
    expect(written).toEqual([]);
  });

  it("says something about every colour it writes", () => {
    // A guard that asserted an empty list would pass over an empty spec.
    const colours = HIGHLIGHT.flatMap((rule) => coloursIn(rule as unknown as Record<string, unknown>));
    expect(colours.length).toBeGreaterThan(8);
  });

  it("mutes the marks after it sizes the headings, so the later rule wins", () => {
    // A `#` carries its heading's tag and processingInstruction at once, and
    // the rules share `color`. Order is the whole of why the mark is quiet.
    const marks = HIGHLIGHT.findIndex((rule) => "color" in rule && rule.color === "var(--ms-text-3)" && rule.fontWeight === "400");
    expect(marks).toBe(HIGHLIGHT.length - 1);
  });
});

describe("the editor theme", () => {
  it("takes every colour from the token table", () => {
    const written: string[] = [];
    for (const [selector, style] of Object.entries(EDITOR_THEME)) {
      for (const [property, value] of coloursIn(style)) {
        if (typeof value !== "string" || !value.startsWith("var(--ms-")) {
          written.push(`${selector} { ${property}: ${String(value)} }`);
        }
      }
    }
    expect(written).toEqual([]);
  });

  it("out-specifies the widget's own base theme on every rule but the wrapper", () => {
    // CodeMirror mounts its base theme first and writes some of its rules
    // against two classes on the wrapper. A selector of ours that named one
    // class would lose, silently, in the theme nobody screenshotted.
    for (const selector of Object.keys(EDITOR_THEME)) {
      expect({ selector, specific: selector === "&" || selector.startsWith("&.cm-editor") }).toEqual({
        selector,
        specific: true,
      });
    }
  });
});

describe("the shortcuts", () => {
  it("binds saving to Mod-s and leaving to Escape, and takes the key from the browser", () => {
    const said: string[] = [];
    const bindings = shortcuts({
      save: () => said.push("save"),
      cancel: () => said.push("cancel"),
    });

    expect(bindings.map((binding) => binding.key)).toEqual(["Mod-s", "Escape"]);
    for (const binding of bindings) {
      expect({ key: binding.key, prevented: binding.preventDefault }).toEqual({ key: binding.key, prevented: true });
    }
  });

  it("runs the act the page gave it, and says it handled the key", () => {
    const said: string[] = [];
    const bindings = shortcuts({
      save: () => said.push("save"),
      cancel: () => said.push("cancel"),
    });

    // The command is called with the view it is bound to; neither of these
    // reads it, which is what lets them be checked without a browser.
    const ran = bindings.map((binding) => binding.run?.(undefined as never));

    expect(ran).toEqual([true, true]);
    expect(said).toEqual(["save", "cancel"]);
  });
});

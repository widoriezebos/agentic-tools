import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { accepts, isNew, suggesting, TokenField, tokensOf } from "./TokenField";

/**
 * The field of free words that shows which words already exist.
 *
 * What it must not change is what it sends: the value is the one string the
 * act already took, so the rules that split it and join it are asserted
 * first. The rest is the help it adds — which keys end a word, what is
 * suggested, and which chip wears "new".
 */

const KNOWN = ["board", "docs", "ui"];

describe("the words one line means", () => {
  it("are separated by spaces, by commas, or by both", () => {
    expect(tokensOf("ui board")).toEqual(["ui", "board"]);
    expect(tokensOf("ui,board")).toEqual(["ui", "board"]);
    expect(tokensOf("ui, board")).toEqual(["ui", "board"]);
    expect(tokensOf("  ui   board  ")).toEqual(["ui", "board"]);
  });

  it("are none at all when nothing was typed", () => {
    expect(tokensOf("")).toEqual([]);
    expect(tokensOf("   ")).toEqual([]);
  });
});

describe("the keys that end a word", () => {
  it("are Enter, a comma and a space, as the field always took", () => {
    expect(accepts("Enter")).toBe(true);
    expect(accepts(",")).toBe(true);
    expect(accepts(" ")).toBe(true);
  });

  it("are nothing else, so a word can be typed", () => {
    for (const key of ["a", "-", "Backspace", "Tab", "Escape", "ArrowDown", "Shift"]) {
      expect({ key, ends: accepts(key) }).toEqual({ key, ends: false });
    }
  });
});

describe("what is suggested", () => {
  it("is what the board already carries, by prefix", () => {
    expect(suggesting(KNOWN, "b", [])).toEqual(["board"]);
    expect(suggesting(KNOWN, "d", [])).toEqual(["docs"]);
  });

  it("is nothing while nothing is typed, because that is not a question", () => {
    expect(suggesting(KNOWN, "", [])).toEqual([]);
    expect(suggesting(KNOWN, "  ", [])).toEqual([]);
  });

  it("leaves out the words already accepted", () => {
    expect(suggesting(KNOWN, "b", ["board"])).toEqual([]);
  });

  it("is nothing for a word this board has never carried", () => {
    expect(suggesting(KNOWN, "refun", [])).toEqual([]);
  });
});

describe("the new word", () => {
  it("is any word the board has never carried", () => {
    expect(isNew("ui", KNOWN)).toBe(false);
    expect(isNew("boad", KNOWN)).toBe(true);
  });

  // The whole point of the mark: a typo is visible as a chip that says new,
  // beside a chip that does not, before it is a label nobody meant.
  it("wears its mark on the chip, and an existing word does not", () => {
    const markup = renderToStaticMarkup(
      <TokenField id="ms-open-labels" value="ui boad" known={KNOWN} onChange={() => undefined} />,
    );
    expect(markup.split('class="ms-token-new"').length - 1).toBe(1);
    expect(markup).toContain('<span>boad</span><span class="ms-token-new">new</span>');
    expect(markup).toContain('<span>ui</span><button');
    expect(markup).toContain('aria-label="Remove boad"');
  });

  it("is a combobox with nothing suggested until something is typed", () => {
    const markup = renderToStaticMarkup(
      <TokenField id="ms-open-labels" value="" known={KNOWN} onChange={() => undefined} />,
    );
    expect(markup).toContain('role="combobox"');
    expect(markup).toContain('aria-expanded="false"');
    expect(markup).not.toContain('role="listbox"');
    expect(markup).not.toContain("ms-tokens-chosen");
  });
});

import { describe, expect, it } from "vitest";

import { tabAfter, tabShown } from "./Tabs";

/**
 * The two rules of the strip that are worth arguing about: where a key goes,
 * and which tab a page opens on. Both are functions of their inputs, so both
 * are tested as functions rather than through a rendered strip.
 */

const PROJECT = [
  { id: "intent" },
  { id: "doctrine" },
  { id: "decisions" },
  { id: "designs" },
  { id: "questions" },
  { id: "documents" },
];

describe("the keys the strip answers to", () => {
  it("walks the strip, and wraps at both ends", () => {
    expect(tabAfter("ArrowRight", 0, 6)).toBe(1);
    expect(tabAfter("ArrowRight", 5, 6)).toBe(0);
    expect(tabAfter("ArrowLeft", 5, 6)).toBe(4);
    expect(tabAfter("ArrowLeft", 0, 6)).toBe(5);
  });

  it("goes to the ends", () => {
    expect(tabAfter("Home", 3, 6)).toBe(0);
    expect(tabAfter("End", 3, 6)).toBe(5);
    expect(tabAfter("Home", 0, 6)).toBe(0);
    expect(tabAfter("End", 5, 6)).toBe(5);
  });

  // Up and down belong to the page, and Tab belongs to the browser: a strip
  // that swallowed either would take a key its human needed elsewhere.
  it("answers to nothing else", () => {
    for (const key of ["ArrowUp", "ArrowDown", "Tab", "Enter", " ", "a", "PageDown", "Escape"]) {
      expect({ key, went: tabAfter(key, 2, 6) }).toEqual({ key, went: null });
    }
  });

  it("goes nowhere in a strip with no tabs", () => {
    for (const key of ["ArrowLeft", "ArrowRight", "Home", "End"]) {
      expect({ key, went: tabAfter(key, 0, 0) }).toEqual({ key, went: null });
    }
  });

  it("stays where it is in a strip of one", () => {
    expect(tabAfter("ArrowRight", 0, 1)).toBe(0);
    expect(tabAfter("ArrowLeft", 0, 1)).toBe(0);
    expect(tabAfter("End", 0, 1)).toBe(0);
  });
});

describe("which tab a page opens on", () => {
  it("is the one the address names, over the one the browser remembers", () => {
    expect(tabShown(PROJECT, "designs", "questions")).toBe("designs");
    expect(tabShown(PROJECT, "documents", null)).toBe("documents");
  });

  it("is the remembered one where the address names no tab", () => {
    expect(tabShown(PROJECT, undefined, "questions")).toBe("questions");
    expect(tabShown(PROJECT, "", "questions")).toBe("questions");
  });

  it("is the first where the address names no tab and nothing is remembered", () => {
    expect(tabShown(PROJECT, undefined, null)).toBe("intent");
    expect(tabShown(PROJECT, "", "")).toBe("intent");
  });

  // A name this page has no tab for was asked for by the address, and the
  // answer to a wrong address is the front of the page, not a tab the human
  // opened a week ago on another page.
  it("is the first where the address names a tab this page does not have", () => {
    expect(tabShown(PROJECT, "nonsense", "questions")).toBe("intent");
    expect(tabShown(PROJECT, "goal", "designs")).toBe("intent");
  });

  it("ignores a remembered tab this page does not have", () => {
    const goal = [{ id: "decisions" }, { id: "designs" }, { id: "questions" }];
    expect(tabShown(goal, undefined, "documents")).toBe("decisions");
    expect(tabShown(goal, undefined, "designs")).toBe("designs");
  });

  it("names nothing in a strip with no tabs", () => {
    expect(tabShown([], "designs", "designs")).toBe("");
  });
});

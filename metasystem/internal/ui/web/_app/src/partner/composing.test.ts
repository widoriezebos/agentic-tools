import { describe, expect, it } from "vitest";

import { chipped, insertAt } from "./composing";

/**
 * A suggestion respects the draft.
 *
 * The finding this answers is concrete: the send path clears the shared draft
 * on acceptance, so a chip that always sent would erase a half-written
 * question. These are the two cases and the arithmetic between them.
 */

describe("what a chip does", () => {
  it("sends when the composer is empty", () => {
    expect(chipped("")).toBe("send");
    expect(chipped("   \n ")).toBe("send");
  });

  it("inserts when there is something to lose", () => {
    expect(chipped("why is this")).toBe("insert");
    expect(chipped(" a")).toBe("insert");
  });
});

describe("where the words go in", () => {
  it("is the cursor, with a space where one is needed and not where it is not", () => {
    expect(insertAt("why is this", 11, 11, "waiting?")).toEqual({
      text: "why is this waiting?",
      caret: 20,
    });
    expect(insertAt("why is this ", 12, 12, "waiting?")).toEqual({
      text: "why is this waiting?",
      caret: 20,
    });
    expect(insertAt("", 0, 0, "What is next up?")).toEqual({ text: "What is next up?", caret: 16 });
  });

  it("is the middle of a draft, where the caret is in the middle", () => {
    expect(insertAt("why is  waiting?", 7, 7, "this")).toEqual({
      text: "why is this waiting?",
      caret: 11,
    });
  });

  it("replaces a selection, the way every text field does", () => {
    expect(insertAt("why is that waiting?", 7, 11, "this")).toEqual({
      text: "why is this waiting?",
      caret: 11,
    });
  });

  it("starts a new line's words without a space in front of them", () => {
    expect(insertAt("first\n", 6, 6, "second")).toEqual({ text: "first\nsecond", caret: 12 });
  });
});

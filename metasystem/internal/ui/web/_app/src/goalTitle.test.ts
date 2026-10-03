import { describe, expect, it } from "vitest";

import { goalSentence, goalTitle } from "./goalTitle";
import { firstWords } from "./shell/GoalPicker";

/**
 * A goal named by a line of its intent is named by the words after the label
 * the intent opens with, on every line a page makes itself.
 */
describe("a goal's title from its intent", () => {
  it("starts with the words after the label", () => {
    expect(goalTitle("What: the header reads on a phone. Why: it is cut today.")).toBe(
      "the header reads on a phone. Why: it is cut today.",
    );
    expect(goalTitle("  What:\n  the header reads on a phone")).toBe("the header reads on a phone");
    expect(goalTitle("WHAT: one outcome")).toBe("one outcome");
  });

  it("is the intent as it was written where it carries no label", () => {
    expect(goalTitle("The board reads the ledger")).toBe("The board reads the ledger");
    expect(goalTitle("Whatever lands first wins")).toBe("Whatever lands first wins");
    expect(goalTitle("What:")).toBe("");
  });

  it("is what the goal picker cuts its first words from", () => {
    expect(firstWords("What: refunds are issued within a day")).toBe("refunds are issued within a day");
  });
});

/** A goal's page heads it with one sentence: the first, ended where the server ends one. */
describe("a goal's title in one sentence", () => {
  it("is the first sentence after the label, on one line", () => {
    expect(goalSentence("What: the header reads on a phone.  Why: it is cut today.")).toBe("the header reads on a phone.");
    expect(goalSentence("What:\n  the header\n reads on a phone")).toBe("the header reads on a phone");
  });

  it("is not ended by a full stop inside a word", () => {
    expect(goalSentence("Ship v1.2 of the reader, e.g.the board. Then more.")).toBe("Ship v1.2 of the reader, e.g.the board.");
    expect(goalSentence("")).toBe("");
  });
});

import { describe, expect, it } from "vitest";

import { opensAt } from "./scrolling";

/**
 * Where the conversation's column opens.
 *
 * The rule is one line and the case it exists for is one a static render cannot
 * show: a drawer that opens because a chip on a goal's row was pressed is
 * mounted by that press, and the end of the conversation is not where the human
 * asked to be. An answer in the middle of a long conversation is exactly where
 * "the end" and "the card" are different places (Sol S61-C-01).
 */

/** Three answers carrying a card, oldest first, as the column renders them. */
const CARRIED = ["t1", "t2", "t3"];

describe("where the column opens", () => {
  it("is the end where nobody asked for a card", () => {
    expect(opensAt("", CARRIED)).toBe("");
  });

  it("is an older answer's card where that is what was asked for", () => {
    expect(opensAt("t1", CARRIED)).toBe("t1");
  });

  it("is the newest answer's card where that is what was asked for", () => {
    expect(opensAt("t3", CARRIED)).toBe("t3");
  });

  /**
   * A target this column is not carrying is no target: a card a trim took, a
   * card of another kind, a conversation this column is not the one for. The
   * end is where a conversation opens when nothing else is true.
   */
  it("is the end where the target is not one of the cards here", () => {
    expect(opensAt("t9", CARRIED)).toBe("");
    expect(opensAt("t1", [])).toBe("");
  });
});

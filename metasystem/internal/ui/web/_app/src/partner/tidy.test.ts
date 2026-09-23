import { describe, expect, it } from "vitest";

import { clockOf, sawLine, shortTip, stampOf, TIP_LENGTH, whenOf } from "./capture";
import { showsSuggestions } from "./Chips";

/**
 * What the tidied conversation shortens, and what it hides.
 *
 * Two of this slice's eight points are decisions about a display string, and
 * both are the kind that reads fine by eye and is wrong in the one case nobody
 * stood in front of. A meta line has room for seven characters of a reading and
 * a clock; the forty-character hash and the ISO instant stay whole in the
 * Looked list and in the Seeing sheet, which is what makes the shortening safe.
 * The third is the rule that the suggestion pills obey, written as a predicate
 * so that it can be read on its own rather than inferred from a render.
 *
 * The clock is the browser's own, so nothing here asserts a formatted time
 * against a literal: what is asserted is the decision — the same clock on both
 * sides when it is today, and something longer that still contains the clock
 * when it is not.
 */

/** The stamp the server writes for a reading of the ledger. */
const TIP = "6984cdec52ed17c1ecd2dd85ce10b3940efba234";
const AT = "2026-09-23T17:54:00Z";
const STAMPED = `the accepted tip ${TIP}, observed ${AT}`;

describe("a stamp, read back into its parts", () => {
  it("separates the reading from the instant it was observed at", () => {
    expect(stampOf(STAMPED)).toEqual({ tip: TIP, at: AT, what: "" });
  });

  it("drops the page's own reading, which belongs to the list and the sheet", () => {
    expect(stampOf(`${STAMPED}; the page had rendered from the accepted tip 0000000`)).toEqual({
      tip: TIP,
      at: AT,
      what: "",
    });
  });

  it("carries a source that is no tip at all through as it was written", () => {
    expect(stampOf("plans/designs/reading.md as it stands")).toEqual({
      tip: "",
      at: "",
      what: "plans/designs/reading.md as it stands",
    });
  });

  it("reads a tip with no instant beside it", () => {
    expect(stampOf("the accepted tip abc1234")).toEqual({ tip: "abc1234", at: "", what: "" });
  });

  it("is nothing where there is nothing", () => {
    expect(stampOf("")).toEqual({ tip: "", at: "", what: "" });
  });
});

describe("a reading, said out loud", () => {
  it("is seven characters", () => {
    expect(shortTip(TIP)).toBe("6984cde");
    expect(shortTip(TIP)).toHaveLength(TIP_LENGTH);
  });

  it("is a short reading unchanged, rather than padded to seven", () => {
    expect(shortTip("abc")).toBe("abc");
    expect(shortTip("")).toBe("");
  });
});

describe("when something happened", () => {
  it("is the clock alone where it was today", () => {
    const now = new Date("2026-09-23T21:00:00Z");
    expect(whenOf(AT, now)).toBe(clockOf(AT));
  });

  it("puts a date before the clock where it was not today", () => {
    const now = new Date("2026-09-25T21:00:00Z");
    const said = whenOf(AT, now);
    expect(said).toContain(clockOf(AT));
    expect(said.length).toBeGreaterThan(clockOf(AT).length);
  });

  it("says nothing about an instant it cannot read", () => {
    expect(whenOf("", new Date(AT))).toBe("");
    expect(whenOf("not a time", new Date(AT))).toBe("");
  });
});

describe("the first half of a meta line", () => {
  it("is the short reading and the clock", () => {
    const now = new Date(AT);
    expect(sawLine(STAMPED, now)).toBe(`Saw tip 6984cde · ${clockOf(AT)}`);
  });

  it("is the reading alone where nothing says when it was observed", () => {
    expect(sawLine("the accepted tip 6984cdec52ed", new Date(AT))).toBe("Saw tip 6984cde");
  });

  it("names a reading that is a document rather than a tip", () => {
    expect(sawLine("plans/designs/reading.md as it stands", new Date(AT))).toBe(
      "Saw plans/designs/reading.md as it stands",
    );
  });

  it("is nothing where the answer was given no stamp", () => {
    expect(sawLine("", new Date(AT))).toBe("");
  });
});

describe("whether the suggestion pills stand", () => {
  it("is whether the field is empty", () => {
    expect(showsSuggestions("")).toBe(true);
    expect(showsSuggestions("Why is it here?")).toBe(false);
  });

  it("counts whitespace as nothing written", () => {
    expect(showsSuggestions("   ")).toBe(true);
    expect(showsSuggestions("\n\t ")).toBe(true);
  });

  it("leaves as soon as there is a sentence, and comes back when it goes", () => {
    expect(showsSuggestions("W")).toBe(false);
    expect(showsSuggestions("W".slice(1))).toBe(true);
  });
});

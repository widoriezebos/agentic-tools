import { describe, expect, it } from "vitest";

import { draftClearLabel, draftLabel, draftOf, draftSource, offersDraft, valueIn } from "./drafting";

/** One opening of a sheet, which every draft in these tests is from. */
const OPENING = "opening-1";

/**
 * A sheet a human hands over.
 *
 * What the chip says is the whole of what a human reads before they press
 * Send, so it is asserted here rather than inferred from a screenshot: which
 * sheet it came from, which of its fields stand for it, and that a sheet with
 * nothing in it is not something to offer at all.
 */

const NEW_GOAL = [
  { name: "Id", value: "refund-worker" },
  { name: "Intent", value: "Refunds are issued within a day, with nobody touching the queue." },
  { name: "First next step", value: "Take the worker to a working end state." },
  { name: "Basis", value: "   " },
];

describe("a draft offered from a sheet", () => {
  it("carries only the fields that have something in them", () => {
    expect(draftOf(OPENING, "New goal", NEW_GOAL)).toEqual({
      sheet: "New goal",
      opening: OPENING,
      fields: [
        { name: "Id", value: "refund-worker" },
        { name: "Intent", value: "Refunds are issued within a day, with nobody touching the queue." },
        { name: "First next step", value: "Take the worker to a working end state." },
      ],
      writable: [],
    });
  });

  /**
   * The empty field is the point. A sheet's writable names are not its filled-in
   * fields with the values taken off: a next step nobody has written yet is
   * exactly the field a human asks the Partner for words for, and it has to
   * travel as one although it carries nothing.
   */
  it("carries every writable field's name, including the ones with nothing in them", () => {
    const draft = draftOf(OPENING, "New goal", NEW_GOAL, ["Intent", "First next step", "Basis", " "]);
    expect(draft.writable).toEqual(["Intent", "First next step", "Basis"]);
    expect(draft.fields.map((field) => field.name)).not.toContain("Basis");
    expect(draft.opening).toBe(OPENING);
  });

  /** What one field holds, as Undo asks the sheet before it puts anything back. */
  it("answers what one field holds, and nothing for a field that is empty", () => {
    const draft = draftOf(OPENING, "New goal", NEW_GOAL, ["Intent", "Basis"]);
    expect(valueIn(draft, "Intent")).toBe("Refunds are issued within a day, with nobody touching the queue.");
    expect(valueIn(draft, "Basis")).toBe("");
    expect(valueIn(draft, "Nothing of this sheet")).toBe("");
  });

  it("is nothing to offer while nothing is filled in", () => {
    expect(offersDraft(draftOf(OPENING, "New goal", [{ name: "Id", value: "" }]))).toBe(false);
    expect(offersDraft(draftOf(OPENING, "New goal", NEW_GOAL))).toBe(true);
  });

  // The label the design names: the sheet, the id, and the intent's first
  // words. The third field is in the draft and not in the chip, because a chip
  // is a label and not a second copy of the form.
  it("says the sheet and its first two fields, the second cut to its first words", () => {
    expect(draftLabel(draftOf(OPENING, "New goal", NEW_GOAL))).toBe(
      "Draft: New goal · refund-worker · Refunds are issued within a day, with…",
    );
  });

  it("keeps a short field whole, and folds a field written over two lines onto one", () => {
    const draft = draftOf(OPENING, "New question", [{ name: "Question", value: "  Why are\n  these waiting?  " }]);
    expect(draftLabel(draft)).toBe("Draft: New question · Why are these waiting?");
  });

  it("names its source as the sheet it came from, in both places a human reads it", () => {
    const draft = draftOf(OPENING, "Change status", [{ name: "To", value: "accepted" }]);
    expect(draftSource(draft)).toBe("the Change status sheet");
    expect(draftClearLabel(draft)).toBe("Stop offering the Change status sheet");
  });
});

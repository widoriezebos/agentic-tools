import { describe, expect, it } from "vitest";

import {
  draftClearLabel,
  draftLabel,
  draftOf,
  draftSource,
  offersDraft,
  valueIn,
  writingClause,
} from "./drafting";

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
      writing: "",
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

  // The label the design names: the sheet, the id, the intent's first words, and
  // where the caret is. The third field is in the draft and not in the chip,
  // because a chip is a label and not a second copy of the form.
  it("says the sheet and its first two fields, the second cut to its first words", () => {
    expect(draftLabel(draftOf(OPENING, "New goal", NEW_GOAL))).toBe(
      "Draft: New goal · refund-worker · Refunds are issued within a day, with… · writing in nothing yet",
    );
  });

  it("keeps a short field whole, and folds a field written over two lines onto one", () => {
    const draft = draftOf(OPENING, "New question", [{ name: "Question", value: "  Why are\n  these waiting?  " }]);
    expect(draftLabel(draft)).toBe("Draft: New question · Why are these waiting? · writing in nothing yet");
  });

  /**
   * The field in hand, on the chip.
   *
   * It is on the chip because the chip is what a human can see of what the
   * Partner is being told, and since g1-s52 that includes where the caret was: a
   * request naming no field is answered about this one. "Nothing yet" is said
   * rather than left out, because it is the case in which the Partner has to ask.
   */
  it("says which field the caret is in, and says so when none has held it", () => {
    const writing = draftOf(OPENING, "Edit goal", [{ name: "Intent", value: "The board reads the ledger." }],
      ["Intent", "Next step"], "Intent");
    expect(writing.writing).toBe("Intent");
    expect(draftLabel(writing)).toBe("Draft: Edit goal · The board reads the ledger. · writing in Intent");
    // And it takes the room for that from the second field's words: once the
    // caret is somewhere, the words of the fields are on the screen in front of
    // the human, and what the chip still has to say is which draft this is and
    // which field a request naming none is about (g1-s52 Built, deferred).
    const edit = [
      { name: "Goal", value: "g1-s12" },
      { name: "Intent", value: "The board reads the accepted ledger and nothing else." },
    ];
    expect(draftLabel(draftOf(OPENING, "Edit goal", edit, ["Intent"], "Intent"))).toBe(
      "Draft: Edit goal · g1-s12 · writing in Intent",
    );
    expect(draftLabel(draftOf(OPENING, "Edit goal", edit, ["Intent"], ""))).toBe(
      "Draft: Edit goal · g1-s12 · The board reads the accepted ledger and… · writing in nothing yet",
    );
    expect(writingClause("Next step")).toBe("writing in Next step");
    expect(writingClause("")).toBe("writing in nothing yet");
    expect(writingClause("   ")).toBe("writing in nothing yet");
    // Whitespace is not a field, in the draft either.
    expect(draftOf(OPENING, "Edit goal", NEW_GOAL, ["Intent"], "  ").writing).toBe("");
  });

  it("names its source as the sheet it came from, in both places a human reads it", () => {
    const draft = draftOf(OPENING, "Change status", [{ name: "To", value: "accepted" }]);
    expect(draftSource(draft)).toBe("the Change status sheet");
    expect(draftClearLabel(draft)).toBe("Stop offering the Change status sheet");
  });
});

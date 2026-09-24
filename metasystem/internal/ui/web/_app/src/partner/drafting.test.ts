import { describe, expect, it } from "vitest";

import { draftClearLabel, draftLabel, draftOf, draftSource, offersDraft } from "./drafting";

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
    expect(draftOf("New goal", NEW_GOAL)).toEqual({
      sheet: "New goal",
      fields: [
        { name: "Id", value: "refund-worker" },
        { name: "Intent", value: "Refunds are issued within a day, with nobody touching the queue." },
        { name: "First next step", value: "Take the worker to a working end state." },
      ],
    });
  });

  it("is nothing to offer while nothing is filled in", () => {
    expect(offersDraft(draftOf("New goal", [{ name: "Id", value: "" }]))).toBe(false);
    expect(offersDraft(draftOf("New goal", NEW_GOAL))).toBe(true);
  });

  // The label the design names: the sheet, the id, and the intent's first
  // words. The third field is in the draft and not in the chip, because a chip
  // is a label and not a second copy of the form.
  it("says the sheet and its first two fields, the second cut to its first words", () => {
    expect(draftLabel(draftOf("New goal", NEW_GOAL))).toBe(
      "Draft: New goal · refund-worker · Refunds are issued within a day, with…",
    );
  });

  it("keeps a short field whole, and folds a field written over two lines onto one", () => {
    const draft = draftOf("New question", [{ name: "Question", value: "  Why are\n  these waiting?  " }]);
    expect(draftLabel(draft)).toBe("Draft: New question · Why are these waiting?");
  });

  it("names its source as the sheet it came from, in both places a human reads it", () => {
    const draft = draftOf("Change status", [{ name: "To", value: "accepted" }]);
    expect(draftSource(draft)).toBe("the Change status sheet");
    expect(draftClearLabel(draft)).toBe("Stop offering the Change status sheet");
  });
});

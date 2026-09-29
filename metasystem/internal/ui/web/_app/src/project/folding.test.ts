import { describe, expect, it } from "vitest";

import {
  CONFLICT_SAID,
  comparisonOf,
  foldConflicted,
  foldOpened,
  foldRowFailed,
  foldRowWritten,
  foldUse,
  foldWritten,
  inTheDesign,
} from "./folding";

/**
 * The section card's steps (g1-s66 D3): Use writes exactly the section, under
 * the revision it was read at; a heading that is absent or doubled is refused
 * and the draft is kept; a revision conflict re-reads the section and shows the
 * comparison against the words as they are now before Use is offered again; and
 * a Use that wrote the section without its accepted row is offered the row
 * again.
 */

const SOURCE = "# D\n\n## 4. Decisions\n\n- D1. Old.\n\n## 5. Step 1\n\nKept.\n";
const DRAFT = "## 4. Decisions\n\n- D1. New, with the budget.";

describe("the section card", () => {
  it("compares the section as it stands with the draft, the changed lines marked", () => {
    const compared = comparisonOf(foldOpened("4. Decisions", DRAFT, SOURCE, "blob:1"));
    expect(compared.state).toBe("compared");
    if (compared.state === "compared") {
      expect(compared.old.filter((line) => line.changed).map((line) => line.text)).toEqual(["- D1. Old."]);
      expect(compared.new.filter((line) => line.changed).map((line) => line.text)).toEqual(["- D1. New, with the budget."]);
    }
  });

  it("writes exactly that section, under the revision it was read at", () => {
    const used = foldUse(foldOpened("4. Decisions", DRAFT, SOURCE, "blob:1"));
    expect(used.save).toEqual({ source: SOURCE.replace("- D1. Old.", "- D1. New, with the budget."), revision: "blob:1" });
    expect(used.fold.phase).toBe("writing");
  });

  it("refuses an absent or doubled heading in words and keeps the draft", () => {
    const absent = foldUse(foldOpened("9. Gone", DRAFT, SOURCE, "blob:1"));
    expect(absent.save).toBeUndefined();
    expect(absent.fold.phase).toBe("refused");
    expect(absent.fold.said).toContain("is not in this document as it stands");
    expect(absent.fold.text).toBe(DRAFT);
    const doubled = foldUse(foldOpened("4. Decisions", DRAFT, `${SOURCE}\n## 4. Decisions\n`, "blob:1"));
    expect(doubled.fold.said).toContain("occurs 2 times");
    expect(comparisonOf(doubled.fold).state).toBe("refused");
  });

  it("after a conflict, compares against the design as it is now before Use is offered again", () => {
    const writing = foldUse(foldOpened("4. Decisions", DRAFT, SOURCE, "blob:1")).fold;
    const now = SOURCE.replace("- D1. Old.", "- D1. Changed by somebody else.");
    const conflicted = foldConflicted(writing, now, "blob:2");
    expect(conflicted.phase).toBe("comparing");
    expect(conflicted.said).toBe(CONFLICT_SAID);
    expect(conflicted.revision).toBe("blob:2");
    const compared = comparisonOf(conflicted);
    expect(compared.state === "compared" && compared.old.some((line) => line.text === "- D1. Changed by somebody else.")).toBe(true);
    expect(foldUse(conflicted).save?.revision).toBe("blob:2");
  });

  it("offers the accepted row again when the section was written without it", () => {
    const written = foldWritten(foldUse(foldOpened("4. Decisions", DRAFT, SOURCE, "blob:1")).fold, true);
    expect(written.phase).toBe("row");
    const failed = foldRowFailed(written, "the server could not be reached");
    expect(failed.phase).toBe("row");
    expect(failed.said).toBe("The section is written; its decision is not: the server could not be reached. Write the decision again.");
    expect(foldRowWritten(failed).phase).toBe("done");
    expect(foldWritten(foldUse(foldOpened("4. Decisions", DRAFT, SOURCE, "blob:1")).fold, false).phase).toBe("done");
  });

  it("knows a section the design already carries as drafted", () => {
    expect(inTheDesign(SOURCE.replace("- D1. Old.", "- D1. New, with the budget."), "4. Decisions", DRAFT)).toBe(true);
    expect(inTheDesign(SOURCE, "4. Decisions", DRAFT)).toBe(false);
  });
});

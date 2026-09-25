import { describe, expect, it } from "vitest";

import {
  attach,
  attachedDraft,
  attachedPassage,
  attachedSubject,
  draftIn,
  lifeOf,
  passageIn,
  refreshDraft,
  remove,
  retireOnSent,
  retireOnSheetClosed,
  subjectIn,
  takeBackLabel,
  type Attachment,
} from "./attachments";
import { draftOf } from "./drafting";
import type { Chosen } from "./subject";

/** One opening of a sheet, which every draft in these tests is from. */
const OPENING = "opening-1";

/**
 * How long a chip lives, and who decides.
 *
 * The rule under test is the whole of g1-s38: one list, a lifetime declared
 * where an attachment is made, two events that retire by that lifetime and
 * nothing else, and one line each chip can say about itself. A draft that
 * outlived its sheet is what started it, so the sheet's closing is asserted
 * here rather than read off a component.
 */

const GOAL: Chosen = {
  kind: "goal",
  id: "g1-s23",
  title: "g1-s23",
  source: "the accepted tip 6984cde, 17:54",
  summary: "queued",
};

const OTHER: Chosen = { ...GOAL, id: "g1-s30", title: "g1-s30" };

const PASSAGE: Chosen = {
  kind: "passage",
  id: "plans/designs/d1.md",
  title: "One client",
  source: "plans/designs/d1.md, revision blob:abc",
  summary: "",
  quote: "One client, and nothing else.",
  anchor: "the-seam",
};

const DRAFT = draftOf(OPENING, "New goal", [{ name: "Id", value: "refund-worker" }]);
const STATUS = draftOf(OPENING, "Change status", [{ name: "Status", value: "doing" }]);

/** The three, in the order a human makes them: a card, a selection, a sheet. */
function three(): readonly Attachment[] {
  return [attachedSubject(GOAL), attachedPassage(PASSAGE), attachedDraft(DRAFT)].reduce(
    (list: readonly Attachment[], made) => attach(list, made),
    [],
  );
}

describe("one list, in the order the acts were made", () => {
  it("keeps what was attached first first", () => {
    expect(three().map((held) => held.kind)).toEqual(["subject", "passage", "draft"]);
  });

  it("puts a second draft from another sheet beside the first", () => {
    const list = attach(three(), attachedDraft(STATUS));
    expect(list).toHaveLength(4);
    expect(list.at(-1)?.id).toBe("draft:Change status");
  });
});

describe("what replaces what", () => {
  it("replaces the subject with the subject, in the place it held", () => {
    const list = attach(three(), attachedSubject(OTHER));
    expect(list).toHaveLength(3);
    expect(list.map((held) => held.kind)).toEqual(["subject", "passage", "draft"]);
    expect(subjectIn(list)?.id).toBe("g1-s30");
  });

  it("replaces the passage with the passage", () => {
    const second: Chosen = { ...PASSAGE, title: "Another", quote: "Another passage." };
    const list = attach(three(), attachedPassage(second));
    expect(list).toHaveLength(3);
    expect(passageIn(list)?.quote).toBe("Another passage.");
  });

  it("replaces the draft of the same sheet, and only that one", () => {
    const list = attach(attach(three(), attachedDraft(STATUS)), attachedDraft(draftOf(OPENING, "New goal", [
      { name: "Id", value: "refund-worker" },
      { name: "Intent", value: "Refunds are issued within a day." },
    ])));
    expect(list).toHaveLength(4);
    expect(list[2].content).toEqual({
      sheet: "New goal",
      opening: OPENING,
      fields: [
        { name: "Id", value: "refund-worker" },
        { name: "Intent", value: "Refunds are issued within a day." },
      ],
      writable: [],
    });
    expect(list[3].content).toEqual(STATUS);
  });
});

describe("the two events that retire", () => {
  // A question carried the passage, so the passage has been said. The subject
  // and the draft are still what the next question is about.
  it("sends the passage with the question and keeps the rest", () => {
    const list = retireOnSent(three());
    expect(list.map((held) => held.kind)).toEqual(["subject", "draft"]);
  });

  it("retires the closed sheet's draft, and leaves another sheet's standing", () => {
    const list = retireOnSheetClosed(attach(three(), attachedDraft(STATUS)), "New goal");
    expect(list.map((held) => held.id)).toEqual(["subject", "passage", "draft:Change status"]);
  });

  // The whole of the finding this slice answers: the subject is retired by its
  // × and by nothing else, so neither event takes it and no navigation does.
  it("never takes the subject", () => {
    expect(subjectIn(retireOnSent(three()))).not.toBeNull();
    expect(subjectIn(retireOnSheetClosed(three(), "New goal"))).not.toBeNull();
  });

  it("is the same list where there was nothing to retire", () => {
    const list = [attachedSubject(GOAL)];
    expect(retireOnSent(list)).toBe(list);
    expect(retireOnSheetClosed(list, "New goal")).toBe(list);
    expect(remove(list, "passage")).toBe(list);
  });
});

describe("the × on a chip", () => {
  it("takes back that one and nothing else", () => {
    expect(remove(three(), "subject").map((held) => held.kind)).toEqual(["passage", "draft"]);
    expect(remove(three(), "draft:New goal").map((held) => held.kind)).toEqual(["subject", "passage"]);
  });

  it("says what it will stop, in the words of the act that made it", () => {
    expect(takeBackLabel(attachedSubject(GOAL))).toBe("Stop asking about Goal g1-s23");
    expect(takeBackLabel(attachedDraft(DRAFT))).toBe("Stop offering the New goal sheet");
  });
});

describe("a draft, brought up to date from its own sheet", () => {
  const written = draftOf(OPENING, "New goal", [{ name: "Id", value: "refund-worker-2" }]);

  it("replaces what was handed over with what the sheet says now", () => {
    expect(draftIn(refreshDraft(three(), written))?.fields[0].value).toBe("refund-worker-2");
  });

  it("attaches nothing for a sheet nobody handed over", () => {
    const list = [attachedSubject(GOAL)];
    expect(refreshDraft(list, written)).toBe(list);
  });
});

describe("what each chip says about its life", () => {
  it("is one line, and it is the lifetime the act declared", () => {
    expect(lifeOf(attachedSubject(GOAL))).toBe("stays until you clear it");
    expect(lifeOf(attachedPassage(PASSAGE))).toBe("goes with the next question");
    expect(lifeOf(attachedDraft(DRAFT))).toBe("goes with the New goal sheet");
    expect(lifeOf(attachedDraft(STATUS))).toBe("goes with the Change status sheet");
  });
});

describe("what the rest of the application reads from the list", () => {
  it("is the three, each by its own kind", () => {
    const list = three();
    expect(subjectIn(list)).toEqual(GOAL);
    expect(passageIn(list)).toEqual(PASSAGE);
    expect(draftIn(list)).toEqual(DRAFT);
  });

  it("is nothing where nothing was attached", () => {
    expect(subjectIn([])).toBeNull();
    expect(passageIn([])).toBeNull();
    expect(draftIn([])).toBeNull();
  });

  // A capture carries one draft, as a human is filling in one form: the sheet
  // they are standing in is the one they offered last.
  it("is the innermost sheet's draft where two were offered", () => {
    expect(draftIn(attach(three(), attachedDraft(STATUS)))).toEqual(STATUS);
  });
});

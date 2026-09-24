import { describe, expect, it } from "vitest";

import { captureOf, chipOf, readingMoved, seeingLine, sheetNote, shownRows } from "./capture";
import { draftOf } from "./drafting";
import type { Chosen } from "./subject";

/**
 * One capture per question.
 *
 * The rule under test is contract 1: the sheet, the question, the message and
 * the chip are one composition. So this asserts what a capture carries from
 * each of the three pages that make one, what the chip and the "Seeing:" line
 * read from it, and when the page has moved out from under the last one.
 */

const board = {
  view: "board",
  tip: "6984cdec52ed17c1ecd2dd85ce10b3940efba234",
  observedAt: "2026-09-23T17:54:00Z",
  window: "1 days",
  filters: ["Done reaches back 1", "tier 1"],
  lanes: [
    { id: "to-do", title: "To Do", total: 12, goals: ["g1-s23", "g1-s30"] },
    { id: "ready", title: "Ready for Work", total: 3, goals: ["g1-s31"] },
  ],
  returnTo: "/backlog?view=board&window=1&tier=1",
};

describe("the capture a board makes", () => {
  it("carries the reading, the view, the window, the filters and the rows", () => {
    const capture = captureOf({ pathname: "/backlog", page: board, chosen: null, label: "Backlog · board" });
    expect(capture).toEqual({
      section: "Backlog",
      path: "/backlog",
      view: "board",
      tip: board.tip,
      observedAt: board.observedAt,
      window: "1 days",
      label: "Backlog · board",
      filters: board.filters,
      lanes: board.lanes,
      return: board.returnTo,
    });
  });

  it("says in one line what it is showing, with the reading it rendered from", () => {
    const capture = captureOf({ pathname: "/backlog", page: board, chosen: null, label: "" });
    expect(shownRows(capture)).toBe(15);
    expect(seeingLine(capture)).toContain("Backlog · board · 15 goals shown");
    expect(seeingLine(capture)).toContain("tip 6984cde");
  });

  // The Done window is on every board, narrowed or not, so it says nothing
  // about this one and the chip carries what made this board different.
  it("wears a chip of the page, the view, the filters and the subject", () => {
    const capture = captureOf({ pathname: "/backlog", page: board, chosen: null, label: "" });
    expect(chipOf(capture)).toBe("Backlog · board · tier 1");
  });
});

describe("the capture a document makes", () => {
  const document_ = {
    kind: "document",
    subject: "plans/designs/d1.md",
    title: "A design",
    revision: "blob:abc",
    tab: "The seam",
    returnTo: "/project/doc/plans/designs/d1.md#the-seam",
  };

  it("carries the document's own id and the revision the page displayed", () => {
    const capture = captureOf({
      pathname: "/project/doc/plans/designs/d1.md",
      page: document_,
      chosen: null,
      label: "A design · The seam",
    });
    expect(capture.kind).toBe("document");
    expect(capture.subject).toBe("plans/designs/d1.md");
    expect(capture.revision).toBe("blob:abc");
    expect(capture.tab).toBe("The seam");
    expect(capture.return).toBe("/project/doc/plans/designs/d1.md#the-seam");
  });

  it("puts a selected passage in place of the subject, with where it came from", () => {
    const passage: Chosen = {
      kind: "passage",
      id: "plans/designs/d1.md",
      title: "One client",
      source: "plans/designs/d1.md, revision blob:abc",
      summary: "",
      quote: "One client, and nothing else.",
      revision: "blob:abc",
      anchor: "the-seam",
    };
    const capture = captureOf({
      pathname: "/project/doc/plans/designs/d1.md",
      page: document_,
      chosen: passage,
      label: "",
    });
    expect(capture.quote).toBe("One client, and nothing else.");
    expect(capture.quoteFrom).toBe("plans/designs/d1.md");
    expect(capture.quoteRevision).toBe("blob:abc");
    expect(capture.quoteAnchor).toBe("the-seam");
    // A passage is the subject, so the page's own is not also sent: "this"
    // means one thing at a time.
    expect(capture.subject).toBeUndefined();
  });
});

describe("the capture Overview makes", () => {
  it("names the section and the address, and nothing the page does not know", () => {
    const capture = captureOf({
      pathname: "/overview",
      page: { returnTo: "/overview" },
      chosen: null,
      label: "Overview",
    });
    expect(capture).toEqual({ section: "Overview", path: "/overview", label: "Overview", return: "/overview" });
  });

  // The chip goes back to the page the question was asked from, not to the
  // subject's own page: "this" meant something on Overview, and Overview is
  // where it meant it. The subject's own page is one click further on, from
  // the panel that pins it.
  it("goes back to the page the question was asked from", () => {
    const item: Chosen = {
      kind: "overview",
      id: "q-17",
      title: "Who decides?",
      source: "the landing page as this server composed it",
      summary: "open",
      to: "/project/questions",
    };
    const capture = captureOf({ pathname: "/overview", page: { returnTo: "/overview" }, chosen: item, label: "" });
    expect(capture.subject).toBe("q-17");
    expect(capture.kind).toBe("overview");
    expect(capture.return).toBe("/overview");
  });

  // Where the page can show the subject itself, it says so, and the chip goes
  // to that: the board with this goal named rather than the bare board.
  it("goes to the page's own address for the subject where it has one", () => {
    const goal: Chosen = {
      kind: "goal",
      id: "g1-s23",
      title: "g1-s23",
      source: "the accepted tip 6984cde, 17:54",
      summary: "queued",
      to: "/backlog/goal/g1-s23",
      at: "/backlog?goal=g1-s23&view=board&window=1",
    };
    const capture = captureOf({ pathname: "/backlog", page: { returnTo: "/backlog?view=board&window=1" }, chosen: goal, label: "" });
    expect(capture.return).toBe("/backlog?goal=g1-s23&view=board&window=1");
  });
});

describe("whether the page has moved since the last question", () => {
  it("is the reading and nothing else", () => {
    const sent = captureOf({ pathname: "/backlog", page: board, chosen: null, label: "" });
    expect(readingMoved(sent, board)).toBe(false);
    // A filter a human changed is a new capture the next question makes
    // anyway; offering to refresh for it would offer to do what sending does.
    expect(readingMoved(sent, { ...board, filters: ["tier 2"] })).toBe(false);
    expect(readingMoved(sent, { ...board, observedAt: "2026-09-23T18:00:00Z" })).toBe(true);
    expect(readingMoved(sent, { ...board, tip: "0000000" })).toBe(true);
    // A page that reads no ledger has no reading to move.
    expect(readingMoved(sent, {})).toBe(false);
    expect(readingMoved(null, board)).toBe(false);
  });
});

/**
 * A sheet is where the human is standing, and what is in it is theirs until
 * they hand it over. Both of those are facts about the capture, so both are
 * asserted from it and not from the screen.
 */
describe("the capture a sheet makes", () => {
  it("carries no sheet and no draft while no sheet is open", () => {
    const capture = captureOf({ pathname: "/backlog", page: board, chosen: null, label: "" });
    expect(capture.sheet).toBeUndefined();
    expect(capture.draft).toBeUndefined();
    expect(sheetNote(capture)).toBe("");
  });

  it("names the open sheet, and ends the Seeing line with it", () => {
    const capture = captureOf({
      pathname: "/backlog",
      page: board,
      chosen: null,
      label: "",
      sheet: "New goal",
    });
    expect(capture.sheet).toBe("New goal");
    expect(seeingLine(capture).endsWith("· New goal sheet open")).toBe(true);
  });

  // The message the question becomes says what was open over the page; the
  // address it returns to does not, so going back reopens nothing.
  it("says on the message what was open, and keeps it out of the address", () => {
    const capture = captureOf({
      pathname: "/backlog",
      page: board,
      chosen: null,
      label: "",
      sheet: "New goal",
    });
    expect(sheetNote(capture)).toBe("asked with the New goal sheet open");
    expect(capture.return).toBe(board.returnTo);
  });

  // An open sheet is not an offered one: the fields travel only because the
  // human pressed "Ask about this".
  it("carries the fields only once they have been offered", () => {
    const offered = draftOf("New goal", [
      { name: "Id", value: "refund-worker" },
      { name: "Intent", value: "Refunds are issued within a day." },
    ]);
    const capture = captureOf({
      pathname: "/backlog",
      page: board,
      chosen: null,
      label: "",
      sheet: "New goal",
      draft: offered,
    });
    expect(capture.draft).toEqual(offered);
    expect(captureOf({ pathname: "/backlog", page: board, chosen: null, label: "", sheet: "New goal", draft: null })
      .draft).toBeUndefined();
  });
});

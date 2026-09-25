import { describe, expect, it } from "vitest";

import type { Approved, Need, Page, Ruling } from "./api";
import {
  actLabel,
  ageLine,
  ANY_CLASS,
  approvalLine,
  askedLine,
  classesIn,
  defectLine,
  destinationFor,
  kindLabel,
  NO_CLASS,
  reviewChip,
  shownRulings,
  tabOrder,
  tabs,
  wayThrough,
  whenLine,
  withdrawable,
} from "./decisions";
import type { Row } from "../backlog/api";

/**
 * What the Decisions page says about a payload.
 *
 * The clock is a value rather than the wall clock, because every age and every
 * overdue line on this page is measured from it: a test that read the real
 * clock would assert a different sentence every day.
 */
const now = new Date("2026-09-25T11:00:00Z");

function need(over: Partial<Need> = {}): Need {
  return {
    kind: "approval",
    id: "g1-s40",
    title: "Approve this one",
    asked: "Approve this one for execution",
    by: "the backlog",
    since: "2026-09-20T09:00:00Z",
    deadline: "",
    silence: "it stays in To Do and no seat may claim it",
    recommend: "",
    where: { kind: "goal", id: "g1-s40" },
    act: "approve",
    command: "",
    row: null,
    ...over,
  };
}

function ruling(over: Partial<Ruling> = {}): Ruling {
  return {
    id: "R-1",
    date: "2026-09-01",
    words: "The board's two drag moves are the only acts the browser publishes",
    context: "given with the board design",
    owner: "Wido",
    class: "temporary",
    due: "2026-09-14",
    event: "",
    condition: "class=temporary due=2026-09-14",
    duePassed: true,
    mentions: ["g1-s12"],
    ...over,
  };
}

describe("a row's kind", () => {
  it("is the word the design gives it, and the server's own for a kind this build has no word for", () => {
    expect(kindLabel("ruling-review")).toBe("review");
    expect(kindLabel("approval")).toBe("approval");
    expect(kindLabel("something-later")).toBe("something-later");
  });
});

describe("where a row opens", () => {
  it("is a goal, a document, the register, the panel, the channel, or nothing", () => {
    expect(destinationFor({ kind: "goal", id: "g1-s40" })).toEqual({ kind: "link", to: "/backlog/goal/g1-s40" });
    expect(destinationFor({ kind: "record", id: "plans/designs/a.md" })).toEqual({
      kind: "link",
      to: "/project/doc/plans/designs/a.md",
    });
    expect(destinationFor({ kind: "register", id: "memory/rulings.md" })).toEqual({
      kind: "link",
      to: "/project/doc/memory/rulings.md",
    });
    expect(destinationFor({ kind: "question", id: "Q-1" })).toEqual({ kind: "link", to: "/project/questions" });
    expect(destinationFor({ kind: "notifications", id: "n-1" })).toEqual({ kind: "notifications", at: "n-1" });
    expect(destinationFor({ kind: "channel", id: "q-1" })).toEqual({ kind: "channel" });
    expect(destinationFor({ kind: "goal", id: "" })).toEqual({ kind: "none" });
    expect(destinationFor({ kind: "later", id: "x" })).toEqual({ kind: "none" });
  });

  // A seat's question is answered where the seat can hear the answer, and the
  // page says so rather than offering a link that would refuse.
  it("says how a seat's question is answered, and never offers a page for it", () => {
    expect(wayThrough({ kind: "channel", id: "q-1" })).toBe("Answer on the fleet channel with your code");
    expect(wayThrough({ kind: "register", id: "memory/rulings.md" })).toBe("Open the register");
    expect(wayThrough({ kind: "question", id: "Q-1" })).toBe("Open the register");
    expect(wayThrough({ kind: "record", id: "a.md" })).toBe("Open the record");
    expect(wayThrough({ kind: "goal", id: "g1-s40" })).toBe("Open the goal");
    expect(wayThrough({ kind: "notifications", id: "n-1" })).toBe("Open the message");
    expect(wayThrough({ kind: "later", id: "x" })).toBe("");
  });
});

describe("the line under a row", () => {
  it("says who asked, how long ago, and what silence does", () => {
    expect(askedLine(need(), now)).toBe(
      "asked by the backlog, 5 days; if you do nothing: it stays in To Do and no seat may claim it",
    );
  });

  it("drops what the record does not carry rather than inventing it", () => {
    expect(askedLine(need({ by: "", since: "" }), now)).toBe(
      "if you do nothing: it stays in To Do and no seat may claim it",
    );
    expect(askedLine(need({ since: "" }), now)).toBe(
      "asked by the backlog; if you do nothing: it stays in To Do and no seat may claim it",
    );
  });
});

describe("how long ago", () => {
  it("is the largest unit that still says something", () => {
    expect(ageLine("2026-09-25T09:00:00Z", now)).toBe("today");
    expect(ageLine("2026-09-24T09:00:00Z", now)).toBe("yesterday");
    expect(ageLine("2026-09-20T09:00:00Z", now)).toBe("5 days");
    expect(ageLine("2026-09-01T09:00:00Z", now)).toBe("3 weeks");
    expect(ageLine("2026-05-01T09:00:00Z", now)).toBe("4 months");
  });

  it("says nothing about an instant nothing recorded", () => {
    expect(ageLine("", now)).toBe("");
    expect(ageLine("not a date", now)).toBe("");
  });
});

describe("the act on a row", () => {
  it("is named for what it publishes", () => {
    expect(actLabel("approve")).toBe("Approve");
    expect(actLabel("withdraw")).toBe("Withdraw approval");
  });
});

describe("the tabs of what was decided", () => {
  it("are the design's, in its order, each with its own count in its name", () => {
    const page: Page = {
      schemaVersion: 2,
      readAt: "2026-09-25T11:00:00Z",
      signIn: false,
      needsYou: [],
      decided: {
        rulings: [ruling()],
        defects: [],
        decisions: [{ id: "d-1", title: "A decision", note: "accepted", at: "", where: { kind: "record", id: "a.md" } }],
        answered: [],
        approved: [],
        notNow: [],
      },
      counts: { needsYou: 0, asked: 0, waiting: 0, rulings: 148 },
      register: "metasystem/memory/rulings.md",
    };
    expect(tabs(page).map((tab) => tab.id)).toEqual([...tabOrder]);
    // The register's count is the whole register, not the page of it shown.
    expect(tabs(page).map((tab) => tab.title)).toEqual([
      "Rulings 148", "Decisions 1", "Answered 0", "Approved 0", "Not now 0",
    ]);
  });
});

describe("the classes a human can narrow the rulings to", () => {
  it("are the classes the register actually uses, with the ones that declare none", () => {
    const rulings = [
      ruling({ id: "R-1", class: "temporary" }),
      ruling({ id: "R-2", class: "delegated-authority" }),
      ruling({ id: "R-3", class: "temporary" }),
      ruling({ id: "R-4", class: "", due: "", condition: "" }),
    ];
    expect(classesIn(rulings)).toEqual(["delegated-authority", "temporary", NO_CLASS]);
  });

  it("offer no class no ruling carries", () => {
    expect(classesIn([ruling({ class: "temporary" })])).toEqual(["temporary"]);
  });
});

describe("the Find box", () => {
  const rulings = [
    ruling({ id: "R-1", words: "The board's two drag moves", context: "the board design", class: "temporary" }),
    ruling({ id: "R-2", words: "Model choices are delegated", context: "the lane map", class: "delegated-authority" }),
    ruling({ id: "R-3", words: "A standing rule", context: "the first day", class: "", due: "", condition: "" }),
  ];

  it("reads the words and the context and nothing else", () => {
    expect(shownRulings(rulings, "drag", ANY_CLASS).map((one) => one.id)).toEqual(["R-1"]);
    expect(shownRulings(rulings, "lane map", ANY_CLASS).map((one) => one.id)).toEqual(["R-2"]);
    // The owner is on every card and is not searched: "Wido" would return the
    // whole register.
    expect(shownRulings(rulings, "Wido", ANY_CLASS)).toEqual([]);
  });

  it("is case-blind, and an empty box narrows nothing", () => {
    expect(shownRulings(rulings, "  DRAG ", ANY_CLASS).map((one) => one.id)).toEqual(["R-1"]);
    expect(shownRulings(rulings, "", ANY_CLASS)).toHaveLength(3);
  });

  it("narrows with the class filter, which counts a ruling with no condition as its own class", () => {
    expect(shownRulings(rulings, "", "temporary").map((one) => one.id)).toEqual(["R-1"]);
    expect(shownRulings(rulings, "", NO_CLASS).map((one) => one.id)).toEqual(["R-3"]);
    expect(shownRulings(rulings, "standing", NO_CLASS).map((one) => one.id)).toEqual(["R-3"]);
    expect(shownRulings(rulings, "drag", NO_CLASS)).toEqual([]);
  });
});

describe("a ruling's review condition", () => {
  it("says a date in days rather than in arithmetic", () => {
    expect(reviewChip(ruling({ due: "2026-09-14", duePassed: true }), now)).toBe("review passed 11 days ago");
    expect(reviewChip(ruling({ due: "2026-09-24", duePassed: true }), now)).toBe("review passed 1 day ago");
    expect(reviewChip(ruling({ due: "2026-09-25", duePassed: true }), now)).toBe("review passed today");
    expect(reviewChip(ruling({ due: "2026-10-02", duePassed: false }), now)).toBe("review due 2 Oct");
  });

  // An event is shown exactly as the register wrote it, and never judged.
  it("shows an event and judges nothing about it", () => {
    const event = ruling({
      class: "assumption-dependent",
      due: "",
      event: "first-measured-report-exists",
      condition: "class=assumption-dependent event=first-measured-report-exists",
      duePassed: false,
    });
    expect(reviewChip(event, now)).toBe("review on first-measured-report-exists");
  });

  // The register carries rows with both — R-29-m2 is due on a date and on a
  // terminal re-arm, whichever comes first. The date used to win and the
  // event vanished off the card.
  it("says both where the row carries a date and an event", () => {
    const both = ruling({
      due: "2026-10-02",
      event: "terminal-re-arm",
      condition: "class=temporary due=2026-10-02 event=terminal-re-arm",
      duePassed: false,
    });
    expect(reviewChip(both, now)).toBe("review due 2 Oct · or on terminal-re-arm");
    expect(reviewChip({ ...both, due: "2026-09-14", duePassed: true }, now)).toBe(
      "review passed 11 days ago · or on terminal-re-arm",
    );
  });

  it("shows a condition the grammar refuses exactly as the register wrote it", () => {
    const prose = ruling({ class: "", due: "", event: "", condition: "standing", duePassed: false });
    expect(reviewChip(prose, now)).toBe("standing");
  });

  it("says nothing where the register wrote nothing", () => {
    expect(reviewChip(ruling({ class: "", due: "", event: "", condition: "", duePassed: false }), now)).toBe("");
  });
});

describe("the register's broken rows", () => {
  it("are one quiet line that says how many, and nothing at all where there are none", () => {
    expect(defectLine([])).toBeNull();
    expect(defectLine(["R-9: review condition needs due= or event="])).toBe(
      "1 row of the register could not be read",
    );
    expect(defectLine(["a", "b", "c"])).toBe("3 rows of the register could not be read");
  });
});

describe("which approvals carry Withdraw", () => {
  // The board's own table decides, and not a second rule written here: a
  // withdrawal is a move out of Ready for Work, and that is written down once.
  function row(lane: Row["lane"]): Row {
    return { lane } as Row;
  }

  it("is the board's own eligibility", () => {
    expect(withdrawable(row("ready"))).toBe(true);
    expect(withdrawable(row("to-do"))).toBe(false);
    expect(withdrawable(row("in-progress"))).toBe(false);
    expect(withdrawable(row("done"))).toBe(false);
  });
});

describe("an approval's own line", () => {
  function approved(over: Partial<Approved> = {}): Approved {
    return {
      id: "g1-s49",
      title: "Approved and ready to claim",
      by: "human:Wido",
      at: "2026-09-20T09:00:00Z",
      authority: "proven",
      expired: false,
      row: { lane: "ready" } as Row,
      ...over,
    };
  }

  it("is who admitted it, under what, and when", () => {
    expect(approvalLine(approved(), now)).toBe("human:Wido · proven · 5 days");
  });

  it("says when it no longer admits work", () => {
    expect(approvalLine(approved({ expired: true }), now)).toBe("human:Wido · proven · 5 days · expired");
  });
});

describe("a register date", () => {
  // The register writes a calendar date and means one. It is never read as an
  // instant, so it never lands on the day before in a western zone.
  it("is a day and not an instant", () => {
    expect(reviewChip(ruling({ due: "2026-01-01", duePassed: false }), now)).toBe("review due 1 Jan");
    expect(reviewChip(ruling({ due: "2026-12-31", duePassed: false }), now)).toBe("review due 31 Dec");
    expect(reviewChip(ruling({ due: "not-a-date", duePassed: false }), now)).toBe("review due not-a-date");
  });
});

describe("when something happened", () => {
  it("is the smallest form that still places it", () => {
    expect(whenLine("", now)).toBe("");
    expect(whenLine("not a date", now)).toBe("");
    expect(whenLine("2026-09-25T09:00:00Z", now)).toMatch(/\d/);
    expect(whenLine("2026-09-24T09:00:00Z", now)).toMatch(/^yesterday /);
    expect(whenLine("2026-09-01T09:00:00Z", now)).not.toMatch(/^yesterday /);
  });
});

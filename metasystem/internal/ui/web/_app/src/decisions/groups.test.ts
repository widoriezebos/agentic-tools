import { describe, expect, it } from "vitest";

import type { Need } from "./api";
import {
  confirmLine,
  decidedCount,
  freshLine,
  goalLine,
  groupsOf,
  openGroup,
  rowFacts,
  rowLine,
  viewTitle,
  writeLabel,
  writeStatus,
} from "./groups";
import type { Row } from "../backlog/api";

/**
 * The inbox as groups: what is on a line, in what order, and which group
 * opens.
 *
 * The clock is a value rather than the wall clock, because every age on this
 * page is measured from it: a test that read the real clock would assert a
 * different sentence every day.
 */
const now = new Date("2026-09-25T11:00:00Z");

function row(over: Partial<Row> = {}): Row {
  return {
    ref: { kind: "goal", id: "g1-s40", revision: 3 },
    where: "live", lane: "to-do", phase: "", state: "queued",
    intent: "The queue narrows by label and by origin", nextStep: "", concluded: "",
    origin: "main", priority: 2, sequence: 1, tier: 0,
    labels: [], arc: "", pinned: "", blockedBy: [], openBlockers: [], holds: [],
    sliced: false, decomposed: false, openedAt: "2026-09-20T09:00:00Z",
    doneAt: "", lastChangeAt: "", lastVerb: "", gaps: [],
    ...over,
  } as Row;
}

function need(over: Partial<Need> = {}): Need {
  return {
    kind: "approval", id: "g1-s40", title: "The queue narrows by label",
    asked: "", by: "the backlog", since: "2026-09-20T09:00:00Z",
    deadline: "", silence: "", recommend: "",
    where: { kind: "goal", id: "g1-s40" }, act: "approve", command: "", row: null,
    new: false, words: "", context: "", owner: "", class: "", due: "", path: "", goals: [],
    ...over,
  };
}

/** One of every kind, in an order that is not the design's. */
const everyKind: Need[] = [
  need({ kind: "alert", id: "n-1", title: "The steward could not reach the operator", since: "2026-09-25T09:30:00Z", new: true }),
  need({ kind: "approval", id: "g1-s40" }),
  need({ kind: "approval", id: "g1-s41", since: "2026-09-24T09:00:00Z", new: true }),
  need({ kind: "ruling-review", id: "R-2", since: "2026-09-20T00:00:00Z", words: "A temporary ruling whose review has come round. The rest of it." }),
  need({ kind: "question", id: "Q-1", asked: "Which census format?", since: "2026-09-23", new: true }),
  need({ kind: "draft", id: "d-open", title: "A draft nobody accepted", since: "2026-09-22T09:00:00Z", path: "plans/designs/draft.md" }),
  need({ kind: "landed", id: "d-landed", title: "Every goal of this one landed", since: "2026-09-24T09:00:00Z", new: true, path: "plans/designs/landed.md", goals: [{ id: "g1-s9", state: "done" }] }),
  need({ kind: "ask", id: "q-1", title: "g1-s21 · budget-above-norm", since: "2026-09-25T05:00:00Z", new: true }),
  need({ kind: "renewal", id: "g1-s42", since: "2026-09-06T00:00:00Z" }),
  need({ kind: "stopped", id: "g1-s48", since: "2026-09-25T07:00:00Z", new: true }),
  need({ kind: "parked", id: "g1-s45", since: "2026-09-24T05:00:00Z", new: true }),
];

describe("the groups", () => {
  it("are the design's, in the design's order, whatever order the server listed", () => {
    expect(groupsOf(everyKind, now).map((group) => group.id)).toEqual([
      "questions", "drafts", "landed", "asks", "alerts",
      "renewals", "stopped", "parked", "reviews", "queue",
    ]);
  });

  it("are named for what is in them", () => {
    expect(groupsOf(everyKind, now).map((group) => group.title)).toEqual([
      "Questions", "Drafts to accept", "Designs landed", "Asks from a seat", "Alerts",
      "Approvals to renew", "Stopped goals", "Parked by a seat", "Rulings past review",
      "Goals waiting for approval",
    ]);
  });

  // A line reading "Alerts 0" is a line a human has to read to learn nothing,
  // and ten of them are why this page was a wall in the first place.
  it("leaves an empty group off the page entirely", () => {
    const quiet = everyKind.filter((one) => one.kind !== "alert" && one.kind !== "draft");
    expect(groupsOf(quiet, now).map((group) => group.id)).not.toContain("alerts");
    expect(groupsOf(quiet, now).map((group) => group.id)).not.toContain("drafts");
    expect(groupsOf([], now)).toEqual([]);
  });

  it("counts its rows, ages the newest of them, and says how many are new", () => {
    const queue = groupsOf(everyKind, now).find((group) => group.id === "queue");

    expect(queue?.count).toBe(2);
    // The newest of the two, not the oldest: the line answers "is there
    // anything here I have not seen".
    expect(queue?.newest).toBe("yesterday");
    expect(queue?.fresh).toBe(1);
    expect(freshLine(queue!)).toBe("1 new");
  });

  it("says nothing about how new a group is when none of it is", () => {
    const reviews = groupsOf(everyKind, now).find((group) => group.id === "reviews");

    expect(reviews?.fresh).toBe(0);
    expect(freshLine(reviews!)).toBe("");
  });

  it("says nothing about an age where no row in the group carries a date", () => {
    const undated = groupsOf([need({ kind: "alert", id: "n-2", since: "" })], now);

    expect(undated[0].newest).toBe("");
  });

  // A ruling past its review date stays in force exactly as written until a
  // human says otherwise, and "3 weeks" on that line is a page shouting about
  // something that is not going anywhere.
  it("gives the quiet group its standing sentence instead of an age", () => {
    const groups = groupsOf(everyKind, now);

    expect(groups.find((group) => group.id === "reviews")?.standing).toBe("they stay in force");
    for (const group of groups.filter((one) => one.id !== "reviews")) {
      expect({ id: group.id, standing: group.standing }).toEqual({ id: group.id, standing: "" });
    }
  });
});

describe("which group opens", () => {
  const groups = groupsOf(everyKind, now);

  it("is the one this viewer left open", () => {
    expect(openGroup(groups, "queue")).toBe("queue");
    expect(openGroup(groups, "reviews")).toBe("reviews");
  });

  it("is the first with something new where nothing is remembered", () => {
    expect(openGroup(groups, null)).toBe("questions");
    // With the questions answered, the first group with something new is the
    // landed design rather than the drafts above it.
    const answered = everyKind.filter((one) => one.kind !== "question");
    expect(openGroup(groupsOf(answered, now), null)).toBe("landed");
  });

  it("is the first group where nothing is new at all", () => {
    const quiet = everyKind.map((one) => ({ ...one, new: false }));
    expect(openGroup(groupsOf(quiet, now), null)).toBe("questions");
  });

  // Which is what happens when the last draft is accepted and the group a
  // human left open stops existing.
  it("falls back when the remembered group is no longer on the page", () => {
    const accepted = everyKind.filter((one) => one.kind !== "draft");
    expect(openGroup(groupsOf(accepted, now), "drafts")).toBe("questions");
    expect(openGroup([], "drafts")).toBeNull();
  });
});

describe("a row's one line", () => {
  it("is the thing itself, per kind, and never a label for it", () => {
    expect(rowLine(need({ kind: "question", asked: "Which census format?" }))).toBe("Which census format?");
    // The ruling's own words, cut at the first sentence: a ruling is a
    // paragraph and a row is a line.
    expect(rowLine(need({ kind: "ruling-review", id: "R-2", words: "A temporary ruling whose review has come round. And the rest." }))).toBe(
      "A temporary ruling whose review has come round",
    );
    expect(rowLine(need({ kind: "draft", title: "A draft nobody accepted" }))).toBe("A draft nobody accepted");
    expect(rowLine(need({ kind: "approval", title: "The queue narrows by label" }))).toBe(
      "The queue narrows by label",
    );
  });

  it("falls back to the id where the record says nothing", () => {
    expect(rowLine(need({ kind: "ruling-review", id: "R-7", words: "" }))).toBe("R-7");
    expect(rowLine(need({ kind: "draft", id: "d-none", title: "" }))).toBe("d-none");
  });

  it("carries at its muted end whose it is, its tier, two labels and its age", () => {
    const facts = rowFacts(
      need({ row: row({ origin: "human", tier: 2, labels: ["browser-interface", "robustness", "memory"] }) }),
      now,
    );

    expect(facts).toEqual({
      yours: true, tier: 2, labels: ["browser-interface", "robustness"], age: "5 days",
    });
  });

  it("claims no tier, no origin and no labels for a row that carries no goal", () => {
    expect(rowFacts(need({ kind: "alert", since: "" }), now)).toEqual({
      yours: false, tier: 0, labels: [], age: "",
    });
  });
});

describe("what an open row writes", () => {
  it("accepts a draft and marks a landed design done, in the resolver's own words", () => {
    expect(writeLabel("draft")).toBe("Accept");
    expect(writeStatus("draft")).toBe("accepted");
    expect(writeLabel("landed")).toBe("Mark done");
    expect(writeStatus("landed")).toBe("done");
  });

  // The confirmation is the whole of the act's safety, so it names the file
  // and the word rather than asking "are you sure".
  it("names the file and the word the write will put in it", () => {
    expect(confirmLine(need({ kind: "draft", path: "plans/designs/draft.md" }), "accepted")).toBe(
      "Write “Status: accepted” on plans/designs/draft.md?",
    );
    expect(confirmLine(need({ kind: "landed", id: "d-landed", path: "" }), "done")).toBe(
      "Write “Status: done” on d-landed?",
    );
  });

  it("lists a landed design's goals with where each one stands", () => {
    expect(goalLine({ id: "g1-s9", state: "done" })).toBe("g1-s9 · done");
    expect(goalLine({ id: "g1-elsewhere", state: "" })).toBe("g1-elsewhere · not in this ledger");
  });
});

describe("the two views", () => {
  it("carry their counts in their names, which is what a human chooses by", () => {
    expect(viewTitle("inbox", 38)).toBe("Inbox 38");
    expect(viewTitle("decided", 61)).toBe("Decided 61");
  });

  it("count what is decided as all five of its tabs", () => {
    expect(
      decidedCount({
        rulings: [1, 2, 3], decisions: [1], answered: [1, 2], approved: [], notNow: [1],
      }),
    ).toBe(7);
  });
});

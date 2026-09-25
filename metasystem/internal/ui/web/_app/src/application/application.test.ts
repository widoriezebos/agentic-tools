import { describe, expect, it } from "vitest";

import type { Landed, Page, Problem } from "./api";
import {
  concludedLine,
  countLine,
  engineLine,
  firstSentence,
  freshLine,
  labelsIn,
  modeWords,
  shown,
  shownCount,
  statusWord,
  subjectLine,
  unreadLine,
  weeksOf,
} from "./application";

/**
 * What the page says about a payload, as sentences rather than as a shape.
 *
 * Every date below is written without a zone, so it is read in the machine's
 * own time the way a browser reads one, and the weeks come out the way a human
 * in front of the page would count them.
 */

function landed(over: Partial<Landed> = {}): Landed {
  return {
    id: "g1-s48",
    intent: "The Decisions inbox is done",
    concluded: "landed in 9017baa. The inbox groups collapse and one opens.",
    doneAt: "2026-09-24T09:00:00",
    labels: [],
    arc: "",
    new: false,
    ...over,
  };
}

function page(over: Partial<Page> = {}): Page {
  return {
    schemaVersion: 1,
    readAt: "2026-09-25T11:00:00",
    subject: "MetaSystem",
    mode: "self-hosted",
    engine: { build: "5b9d958", generation: 4, publishedAt: "2026-09-25T10:45:00" },
    counts: { landed: 429, thisMonth: 320, new: 7 },
    landed: [landed()],
    problems: { columns: [], open: [], concluded: [], unread: 0, defects: [], register: "memory/known-issues.md" },
    docs: [],
    visit: { since: "2026-09-24T11:00:00", first: false },
    ...over,
  };
}

describe("the header", () => {
  it("names the subject and the mode, and says the self-hosted case in words", () => {
    expect(subjectLine(page())).toBe("MetaSystem · self-hosted");
    expect(modeWords(page())).toContain("the instance doing the work is on Fleet");
  });

  it("names an adopted application as one built with MetaSystem, and says nothing more", () => {
    const adopted = page({ subject: "ledgerly", mode: "adopted" });
    expect(subjectLine(adopted)).toBe("ledgerly · built with MetaSystem");
    expect(modeWords(adopted)).toBe("");
  });

  // "Last published" is the whole of the honesty: presence is a record at one
  // tick, and this page never claims it is what is running.
  it("says the engine build is the last one published, with its tick", () => {
    expect(engineLine(page())).toBe(
      "last published MetaSystem engine 5b9d958 · generation 4 · published 2026-09-25 10:45",
    );
  });

  it("says so in words where this seat has published none", () => {
    expect(engineLine(page({ engine: null }))).toBe("no engine build available");
    expect(engineLine(page({ engine: { build: "", generation: 0, publishedAt: "" } }))).toBe(
      "no engine build available",
    );
  });

  // "Goals concluded", not "landed": a conclusion records an end, and some of
  // those ends are administrative.
  it("counts the history in the words a conclusion earns", () => {
    expect(countLine(page())).toBe("429 goals concluded · 320 this month · 7 since your last visit");
  });

  it("leaves out a count that has nothing to say", () => {
    expect(countLine(page({ counts: { landed: 12, thisMonth: 0, new: 0 } }))).toBe("12 goals concluded");
  });
});

describe("the weeks", () => {
  const rows = [
    landed({ id: "a", doneAt: "2026-09-25T09:00:00", new: true }),
    landed({ id: "b", doneAt: "2026-09-21T09:00:00" }),
    landed({ id: "c", doneAt: "2026-09-20T09:00:00" }),
    landed({ id: "d", doneAt: "2026-09-14T09:00:00" }),
    landed({ id: "e", doneAt: "" }),
  ];

  // A week is Monday to Sunday: the 20th is a Sunday and belongs to the week
  // that began on the 14th, not to the one starting the next morning.
  it("groups Monday to Sunday, in the order the server sent the rows", () => {
    const weeks = weeksOf(rows);
    expect(weeks.map((week) => week.title)).toEqual([
      "Week of 21 September 2026",
      "Week of 14 September 2026",
      "Nothing dated these",
    ]);
    expect(weeks.map((week) => week.count)).toEqual([2, 2, 1]);
    expect(weeks[0].rows.map((row) => row.id)).toEqual(["a", "b"]);
    expect(weeks[1].rows.map((row) => row.id)).toEqual(["c", "d"]);
  });

  it("counts how many of a week concluded since the last visit, and says nothing where none did", () => {
    const weeks = weeksOf(rows);
    expect(freshLine(weeks[0])).toBe("1 new");
    expect(freshLine(weeks[1])).toBe("");
  });

  it("has no weeks at all where nothing concluded", () => {
    expect(weeksOf([])).toEqual([]);
  });
});

describe("narrowing", () => {
  const rows = [
    landed({ id: "g1-s48", intent: "The Decisions inbox is done", concluded: "landed in 9017baa", labels: ["browser-interface"] }),
    landed({ id: "g1-s42", intent: "The fleet page reads a chain", concluded: "landed in 44de0a1", labels: ["headless-fleet", "robustness"] }),
    landed({ id: "g1-s41", intent: "The census answers the page", concluded: "Obsolete: a duplicate holding no work", labels: ["headless-fleet"] }),
  ];

  it("finds over the id, the intent and the conclusion", () => {
    expect(shown(rows, { find: "g1-s42", label: "" }).map((row) => row.id)).toEqual(["g1-s42"]);
    expect(shown(rows, { find: "inbox", label: "" }).map((row) => row.id)).toEqual(["g1-s48"]);
    expect(shown(rows, { find: "44de0a1", label: "" }).map((row) => row.id)).toEqual(["g1-s42"]);
    expect(shown(rows, { find: "nothing here", label: "" })).toEqual([]);
  });

  it("narrows to one label", () => {
    expect(shown(rows, { find: "", label: "headless-fleet" }).map((row) => row.id)).toEqual(["g1-s42", "g1-s41"]);
  });

  it("draws the chips from the rows shown, commonest first", () => {
    expect(labelsIn(rows)).toEqual([
      { label: "headless-fleet", count: 2 },
      { label: "browser-interface", count: 1 },
      { label: "robustness", count: 1 },
    ]);
  });

  it("says how many are shown of how many there are, and only where they differ", () => {
    expect(shownCount(30, 30)).toBe("30 concluded");
    expect(shownCount(30, 4)).toBe("30 concluded · 4 shown");
  });
});

describe("a row's one line", () => {
  it("is the conclusion's first statement, without the rest of it", () => {
    expect(firstSentence("landed in 9017baa. The inbox groups collapse.")).toBe("landed in 9017baa");
    expect(firstSentence("Obsolete: self-declared duplicate holding no work")).toBe(
      "Obsolete: self-declared duplicate holding no work",
    );
    expect(firstSentence("")).toBe("");
  });
});

describe("known problems", () => {
  function problem(over: Partial<Problem> = {}): Problem {
    return {
      id: "KI-2", date: "2026-08-04", what: "The suite's wall time grew", consequence: "CI cost",
      lever: "Split fast from slow", status: "ACCEPTED 2026-08-06 with a measured trigger", open: false,
      ...over,
    };
  }

  // An accepted limitation still exists, so the word travels onto the line:
  // "ACCEPTED" and "FIXED" are two different things to know about a problem.
  it("keeps the status word visible on a concluded row", () => {
    expect(statusWord(problem())).toBe("ACCEPTED");
    expect(statusWord(problem({ status: "FIXED 2026-08-06: the reader walks the tree" }))).toBe("FIXED");
    expect(statusWord(problem({ status: "RETIRED, superseded by KI-40" }))).toBe("RETIRED");
  });

  it("says how many are concluded on the disclosure that opens them", () => {
    expect(concludedLine([problem(), problem({ id: "KI-3" })])).toBe("Show concluded (2)");
  });

  // Five of this register's own open rows carry three or four cells today, and
  // a block that dropped them would claim the project knows about fewer
  // problems than it does.
  it("counts the rows the reader could not read, in its own line", () => {
    expect(unreadLine(5)).toBe("5 issues could not be read as rows");
    expect(unreadLine(1)).toBe("1 issue could not be read as a row");
    expect(unreadLine(0)).toBe("");
  });
});

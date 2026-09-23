import { describe, expect, it } from "vitest";

import type { Group } from "./api";
import {
  blockOrder,
  columns,
  destinationFor,
  DONE_TODAY,
  healthLine,
  laneName,
  laneStrip,
  moreLine,
  plural,
  progressLine,
  whenLine,
  windowLine,
} from "./overview";

/**
 * The sentences the Overview says, and the destinations it offers.
 *
 * Every instant below is built in the machine's own time zone, because that is
 * what a human reads: a fixture written as a UTC string would say a different
 * hour on a laptop in Amsterdam than on one in New York, and the rule under
 * test is about the wall clock in front of the reader.
 */

const now = new Date(2026, 8, 23, 14, 37, 0);

function at(daysAgo: number, hour: number, minute: number): string {
  const when = new Date(now.getTime());
  when.setDate(when.getDate() - daysAgo);
  when.setHours(hour, minute, 0, 0);
  return when.toISOString();
}

function group(count: number, shown: number): Group {
  return {
    count,
    items: Array.from({ length: shown }, (_, index) => ({
      id: `id-${String(index)}`,
      title: `Title ${String(index)}`,
      note: "",
      at: "",
      where: { kind: "goal", id: `id-${String(index)}` },
    })),
  };
}

describe("the window line", () => {
  it("says the day it looked back over on a first visit", () => {
    expect(windowLine(at(1, 18, 2), true, now)).toBe("in the last 24 hours");
    // A first visit says it even when the marker handed over an instant: the
    // instant is a day back from the visit's own start, and naming it would
    // name a visit nobody made.
    expect(windowLine("", true, now)).toBe("in the last 24 hours");
  });

  it("names yesterday and today in day words", () => {
    expect(windowLine(at(1, 18, 2), false, now)).toBe("since yesterday 18:02");
    expect(windowLine(at(0, 9, 5), false, now)).toBe("since today 09:05");
  });

  it("names any other day by its date", () => {
    expect(windowLine(at(4, 18, 2), false, now)).toBe("since 2026-09-19 18:02");
  });

  it("says so rather than inventing one when nothing recorded the window", () => {
    expect(windowLine("", false, now)).toBe("since a moment nothing recorded");
    expect(windowLine("the day before", false, now)).toBe("since a moment nothing recorded");
  });
});

describe("when a row's fact happened", () => {
  it("is a clock today, the word and a clock yesterday, and a date before that", () => {
    expect(whenLine(at(0, 14, 5), now)).toBe("14:05");
    expect(whenLine(at(1, 18, 2), now)).toBe("yesterday 18:02");
    expect(whenLine(at(22, 2, 0), now)).toBe("2026-09-01");
  });

  // A clock alone places nothing older than today: a goal opened three weeks
  // ago reading "02:00" looks like a goal opened this morning.
  it("never shows a clock for a day it does not name", () => {
    expect(whenLine(at(22, 2, 0), now)).not.toContain(":");
  });

  it("says nothing where nothing recorded an instant", () => {
    expect(whenLine("", now)).toBe("");
    expect(whenLine("the other day", now)).toBe("");
  });
});

describe("a design in flight", () => {
  it("says how many of the goals it names have landed", () => {
    expect(progressLine(2, 3)).toBe("2 of 3 goals done");
    expect(progressLine(0, 1)).toBe("0 of 1 goal done");
    expect(progressLine(4, 4)).toBe("4 of 4 goals done");
  });
});

describe("a capped list", () => {
  it("says what it is hiding, with the way through to it", () => {
    expect(moreLine(group(9, 3))).toBe("and 6 more →");
    expect(moreLine(group(4, 3))).toBe("and 1 more →");
  });

  it("says nothing when it is showing all of it", () => {
    expect(moreLine(group(3, 3))).toBeNull();
    expect(moreLine(group(0, 0))).toBeNull();
  });

  it("counts in whole words", () => {
    expect(plural(1, "goal")).toBe("1 goal");
    expect(plural(0, "goal")).toBe("0 goals");
    expect(plural(7, "record")).toBe("7 records");
  });
});

describe("the lane strip", () => {
  it("calls the board's lanes what the board calls them", () => {
    expect(laneName("to-do")).toBe("To Do");
    expect(laneName("ready")).toBe("Ready for Work");
    expect(laneName("in-progress")).toBe("In Progress");
    expect(laneName("review")).toBe("Review and Verification");
    expect(laneName("waiting")).toBe("Waiting");
  });

  it("calls the day's conclusions Done today, which is not a lane of the board", () => {
    expect(laneName(DONE_TODAY)).toBe("Done today");
    // The board has a Done lane, and it is read through a window of days. The
    // strip's last count is the observation day only, so it has a name of its
    // own rather than borrowing that one.
    expect(laneName("done")).toBe("Done");
    expect(DONE_TODAY).not.toBe("done");
  });

  it("lands every count on the board", () => {
    const strip = laneStrip([
      { id: "to-do", count: 5 },
      { id: DONE_TODAY, count: 1 },
    ]);
    expect(strip.map((lane) => lane.title)).toEqual(["To Do", "Done today"]);
    expect(strip.map((lane) => lane.count)).toEqual([5, 1]);
    expect(new Set(strip.map((lane) => lane.to))).toEqual(new Set(["/backlog"]));
  });

  it("shows a lane it has no name for by its own id rather than blank", () => {
    expect(laneName("something-else")).toBe("something-else");
  });
});

describe("the columns", () => {
  it("are the five blocks, each in one of them, in the design's order", () => {
    const { left, right } = columns();
    expect([...left, ...right]).toEqual([...blockOrder]);
    expect(new Set([...left, ...right]).size).toBe(blockOrder.length);
  });

  it("lead the left column with the two questions the page opens with", () => {
    expect(columns().left).toEqual(["needs-you", "changed"]);
  });
});

describe("where a row opens", () => {
  it("sends a goal to its page and a record to its document", () => {
    expect(destinationFor({ kind: "goal", id: "g1-s27" })).toEqual({
      kind: "link",
      to: "/backlog/goal/g1-s27",
    });
    expect(destinationFor({ kind: "document", id: "plans/designs/a b.md" })).toEqual({
      kind: "link",
      to: "/project/doc/plans/designs/a%20b.md",
    });
  });

  it("sends a question to the register and a lane to the board", () => {
    expect(destinationFor({ kind: "question", id: "q-1" })).toEqual({
      kind: "link",
      to: "/project/questions",
    });
    expect(destinationFor({ kind: "backlog", id: "" })).toEqual({ kind: "link", to: "/backlog" });
  });

  it("opens the panel and the sheet, which are not addresses", () => {
    expect(destinationFor({ kind: "notification", id: "n-1" })).toEqual({
      kind: "notifications",
      at: "n-1",
    });
    expect(destinationFor({ kind: "sign-in", id: "" })).toEqual({ kind: "sign-in" });
  });

  it("offers nothing for a reference this build has no surface for", () => {
    expect(destinationFor({ kind: "seat", id: "m1e" })).toEqual({ kind: "none" });
    expect(destinationFor({ kind: "goal", id: "" })).toEqual({ kind: "none" });
  });
});

describe("the calm health line", () => {
  it("names when the ledger last caught up and that the records answered", () => {
    expect(healthLine(at(0, 14, 36))).toBe("Ledger synced 14:36 · records check clean");
  });

  it("says the instant is not known rather than an epoch", () => {
    expect(healthLine("")).toBe("Ledger synced unknown · records check clean");
  });
});

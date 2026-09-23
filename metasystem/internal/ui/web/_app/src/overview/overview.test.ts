import { describe, expect, it } from "vitest";

import type { Changed, Group, Health, Item, Page } from "./api";
import {
  blockAnchor,
  blockOrder,
  changedCounts,
  columns,
  destinationFor,
  DONE_TODAY,
  glance,
  healthPills,
  laneName,
  laneStrip,
  moreLine,
  needsKinds,
  plural,
  seeAll,
  timeline,
  whenLine,
  windowLine,
} from "./overview";

/**
 * The numbers the Overview is read by, the sentences it says, and the
 * destinations it offers.
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

function item(id: string, title: string, note: string, when: string, kind = "goal"): Item {
  return { id, title, note, at: when, where: { kind, id } };
}

function noChange(): Changed {
  return {
    concluded: { count: 0, items: [] },
    moved: { count: 0, items: [] },
    records: { count: 0, items: [] },
    messages: 0,
    total: 0,
  };
}

function page(over: Partial<Page> = {}): Page {
  return {
    schemaVersion: 1,
    readAt: at(0, 14, 37),
    since: at(1, 18, 2),
    first: false,
    needsYou: {
      approvals: { count: 0, items: [] },
      questions: { count: 0, items: [] },
      drafts: { count: 0, items: [] },
      designs: { count: 0, items: [] },
      alerts: { count: 0, items: [] },
      signIn: false,
      total: 0,
    },
    changed: noChange(),
    work: {
      inProgress: [],
      next: [],
      waiting: { count: 0, id: "", title: "", reason: "", since: "" },
      lanes: [],
    },
    memory: {
      intent: { chapters: 0, summary: "" },
      doctrine: { chapters: 0, summary: "" },
      decisions: { total: 0, drafts: 0 },
      designs: { total: 0, done: 0, inFlight: 0, progress: [] },
      questions: 0,
    },
    health: { ok: true, syncedAt: at(0, 14, 36), problems: { count: 0, items: [] } },
    ...over,
  };
}

function health(problems: Item[], syncedAt: string): Health {
  return {
    ok: problems.length === 0,
    syncedAt,
    problems: { count: problems.length, items: problems },
  };
}

describe("the glance strip", () => {
  const busy = page({
    needsYou: { ...page().needsYou, total: 4 },
    changed: { ...noChange(), messages: 3, total: 9 },
    work: {
      inProgress: [{ id: "g1-s27", title: "Overview", seat: { machine: "m1", lineage: "e" }, phase: "build", at: "" }],
      next: [item("g1-s28", "Next", "ready", "")],
      waiting: { count: 2, id: "g1-s9", title: "Held", reason: "the fence", since: at(3, 9, 0) },
      lanes: [
        { id: "to-do", count: 6 },
        { id: "ready", count: 11 },
        { id: "in-progress", count: 1 },
        { id: "review", count: 0 },
        { id: "waiting", count: 2 },
        { id: DONE_TODAY, count: 3 },
      ],
    },
  });

  it("is the six numbers the page is read by, in the design's order", () => {
    expect(glance(busy).map((tile) => [tile.label, tile.value])).toEqual([
      ["Needs you", "4"],
      ["In progress", "1"],
      ["Next up", "11"],
      ["Waiting", "2"],
      ["Since your visit", "9"],
      ["Health", "ok"],
    ]);
  });

  // The server sends three of Ready for Work to list and the lane's whole
  // count to count by. A tile that said "1" beside a lane of eleven would be
  // counting the list instead of the work.
  it("counts Next up by the lane and not by the three goals it lists", () => {
    expect(busy.work.next.length).toBe(1);
    expect(glance(busy)[2].value).toBe("11");
  });

  it("marks what needs a human only while something does", () => {
    expect(glance(busy)[0].tone).toBe("accent");
    expect(glance(page())[0].tone).toBe("plain");
    expect(glance(page())[0].value).toBe("0");
  });

  it("says health in a word, and warns only when it is not ok", () => {
    expect(glance(page())[5]).toMatchObject({ value: "ok", tone: "plain", word: true });
    const unwell = page({ health: health([item("", "the fetch failed", "ledger", "", "backlog")], at(0, 12, 0)) });
    expect(glance(unwell)[5]).toMatchObject({ value: "attention", tone: "warn", word: true });
    // Health is the only one of the six that answers in a word.
    expect(glance(busy).filter((tile) => tile.word).map((tile) => tile.id)).toEqual(["health"]);
  });

  it("lands three tiles on this page and three on the board", () => {
    const tiles = glance(busy);
    expect(tiles.filter((tile) => tile.anchor).map((tile) => tile.to)).toEqual([
      `#${blockAnchor("needs-you")}`,
      `#${blockAnchor("changed")}`,
      `#${blockAnchor("health")}`,
    ]);
    expect(tiles.filter((tile) => !tile.anchor).map((tile) => tile.to)).toEqual(["/backlog", "/backlog", "/backlog"]);
  });

  it("anchors every block it names on the block's own id", () => {
    for (const id of blockOrder) {
      expect(blockAnchor(id)).toBe(`overview-${id}`);
    }
  });
});

describe("what needs a human", () => {
  it("is one row per kind, in the design's order, and only where there is one", () => {
    const needs = {
      ...page().needsYou,
      approvals: group(2, 2),
      drafts: group(1, 1),
      alerts: group(5, 3),
      total: 8,
    };
    expect(needsKinds(needs).map((kind) => [kind.id, kind.group.count])).toEqual([
      ["approvals", 2],
      ["drafts", 1],
      ["alerts", 5],
    ]);
  });

  it("sends every kind but the steward's messages to an address", () => {
    const needs = {
      ...page().needsYou,
      approvals: group(1, 1),
      questions: group(1, 1),
      drafts: group(1, 1),
      designs: group(1, 1),
      alerts: group(1, 1),
      total: 5,
    };
    expect(needsKinds(needs).map((kind) => kind.to)).toEqual([
      "/backlog",
      "/project/questions",
      "/project/designs",
      "/project/designs",
      null,
    ]);
  });

  it("is nothing at all when nothing is waiting", () => {
    expect(needsKinds(page().needsYou)).toEqual([]);
  });
});

describe("the timeline", () => {
  const changed: Changed = {
    concluded: { count: 1, items: [item("g1-s20", "The board reads the ledger", "shipped it", at(0, 11, 0))] },
    moved: {
      count: 3,
      items: [
        item("g1-s27", "The Overview at a glance", "claimed", at(0, 14, 10)),
        item("g1-s28", "The next one", "", at(0, 9, 30)),
      ],
    },
    records: {
      count: 2,
      items: [
        item("plans/a.md", "A design", "design", at(0, 13, 0), "document"),
        item("plans/b.md", "A decision", "decision", at(1, 20, 0), "document"),
      ],
    },
    messages: 4,
    total: 10,
  };

  it("merges the three lists newest first", () => {
    expect(timeline(changed).map((entry) => entry.title)).toEqual([
      "The Overview at a glance",
      "A design",
      "The board reads the ledger",
      "The next one",
      "A decision",
    ]);
  });

  it("says what happened rather than which list it came from", () => {
    expect(timeline(changed).map((entry) => entry.chip)).toEqual([
      "claimed",
      "design",
      "landed",
      "moved",
      "decision",
    ]);
  });

  it("caps the list, and an entry nothing dated never leads it", () => {
    expect(timeline(changed, 2).map((entry) => entry.title)).toEqual(["The Overview at a glance", "A design"]);
    const undated: Changed = {
      ...noChange(),
      moved: { count: 2, items: [item("g1-a", "Undated", "", ""), item("g1-b", "Dated", "", at(0, 8, 0))] },
      total: 2,
    };
    expect(timeline(undated).map((entry) => entry.title)).toEqual(["Dated", "Undated"]);
  });

  it("counts every kind, zero or not, in the same four places", () => {
    expect(changedCounts(changed)).toBe("3 moved · 1 landed · 2 records · 4 messages");
    expect(changedCounts(noChange())).toBe("0 moved · 0 landed · 0 records · 0 messages");
  });

  it("offers the board while a goal change is hidden, and the records after that", () => {
    expect(seeAll(changed, timeline(changed))).toBe("/backlog");
    const records: Changed = {
      ...noChange(),
      records: { count: 9, items: [item("plans/a.md", "A design", "design", at(0, 13, 0), "document")] },
      total: 9,
    };
    expect(seeAll(records, timeline(records))).toBe("/project/designs");
  });

  it("offers nothing where the timeline is showing all of it", () => {
    const small: Changed = {
      ...noChange(),
      concluded: { count: 1, items: [item("g1-s20", "Landed", "", at(0, 11, 0))] },
      total: 1,
    };
    expect(seeAll(small, timeline(small))).toBeNull();
  });
});

describe("the health pills", () => {
  it("says the three sources in the same three places when all is well", () => {
    expect(healthPills(health([], at(0, 14, 36)), now).map((pill) => [pill.id, pill.words, pill.tone])).toEqual([
      ["ledger", "synced 14:36", "ok"],
      ["records", "check clean", "ok"],
      ["deliveries", "all delivered", "ok"],
    ]);
  });

  it("offers nowhere to go from a pill that is fine", () => {
    expect(healthPills(health([], at(0, 14, 36)), now).every((pill) => pill.where === null)).toBe(true);
  });

  it("says a ledger that has fallen behind in the engine's own terms, measuring nothing", () => {
    const stale = health(
      [item("", "the accepted ledger is older than 30 minutes and has not been advanced", "ledger", "", "backlog")],
      at(0, 14, 7),
    );
    expect(healthPills(stale, now)[0]).toMatchObject({ words: "ledger stale", tone: "warn" });
    const older = health([item("", "the accepted ledger is not at the canonical tip", "ledger", "", "backlog")], at(0, 11, 22));
    expect(healthPills(older, now)[0].words).toBe("behind the tip");
  });

  it("says a fetch that did not happen as a failure rather than as a gap", () => {
    const failed = health(
      [item("", "the last fetch of the canonical branch did not succeed", "ledger", "", "backlog")],
      at(0, 9, 0),
    );
    expect(healthPills(failed, now)[0]).toMatchObject({ words: "fetch failed", tone: "bad" });
    expect(healthPills(failed, now)[0].where).toEqual({ kind: "backlog", id: "" });
  });

  it("says the same words whether or not the page knows when it last synced", () => {
    const undated = health([item("", "the accepted ledger is not at the canonical tip", "ledger", "", "backlog")], "");
    expect(healthPills(undated, now)[0].words).toBe("behind the tip");
  });

  it("counts the records the check refused, and opens the first of them", () => {
    const refused = health(
      [
        item("plans/a.md", "a heading nothing follows", "plans/a.md:12", "", "document"),
        item("plans/b.md", "a goal that is not a goal", "plans/b.md:3", "", "document"),
      ],
      at(0, 14, 36),
    );
    const pills = healthPills(refused, now);
    expect(pills[1]).toMatchObject({ words: "2 refusals", tone: "bad" });
    expect(pills[1].where).toEqual({ kind: "document", id: "plans/a.md" });
    // A refusal is not the ledger's problem, and the ledger pill still says
    // the ledger: the three questions are asked separately.
    expect(pills[0].words).toBe("synced 14:36");
  });

  it("counts the messages that reached nobody, and opens the first of them", () => {
    const lost = health([item("n-4", "the channel refused it", "the steward's words", at(0, 13, 0), "notification")], at(0, 14, 36));
    const pills = healthPills(lost, now);
    expect(pills[2]).toMatchObject({ words: "1 undelivered", tone: "bad" });
    expect(pills[2].where).toEqual({ kind: "notification", id: "n-4" });
    expect(pills[1].words).toBe("check clean");
  });
});

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

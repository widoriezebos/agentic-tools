import { describe, expect, it } from "vitest";

import type { Row } from "./api";
import {
  anySet,
  arcsOn,
  concludedWithin,
  matches,
  matchesArc,
  matchesRank,
  matchesSeat,
  matchesText,
  named,
  nameOf,
  noFilters,
  rankOf,
  seatsOn,
  windowTitle,
  WINDOWS,
  ANY,
  NONE,
  type Filters,
} from "./filters";

/**
 * What the board is narrowed to, and how far back Done reaches.
 *
 * The cases worth writing down are the ones a reading could get wrong: a seat
 * or an arc named "?" is still a seat and an arc; a rank of zero is a record
 * that declares none rather than a record whose rank is nought; a conclusion
 * nothing dated is in no window of days but is in "all"; and a stamp a second
 * ahead of the reader's clock has still happened.
 */

function row(id: string, over: Partial<Row> = {}): Row {
  return {
    ref: { kind: "goal", id, revision: 3 },
    where: "live",
    lane: "to-do",
    phase: "",
    state: "queued",
    intent: `Do ${id}`,
    nextStep: "",
    concluded: "",
    origin: "main",
    priority: 2,
    sequence: 1,
    tier: 3,
    labels: [],
    arc: "",
    pinned: "",
    blockedBy: [],
    holds: [],
    openBlockers: [],
    sliced: false,
    decomposed: false,
    openedAt: "2026-09-01T00:00:00Z",
    doneAt: "",
    lastChangeAt: "",
    lastVerb: "",
    gaps: [],
    ...over,
  };
}

function set(over: Partial<Filters>): Filters {
  return { ...noFilters, ...over };
}

describe("a named value", () => {
  it("survives a round trip, and is told from the two that name nothing", () => {
    expect(nameOf(named("m1e"))).toBe("m1e");
    expect(nameOf(named(NONE))).toBe(NONE);
    expect(nameOf(named(""))).toBe("");
    expect(nameOf(ANY)).toBeNull();
    expect(nameOf(NONE)).toBeNull();
  });
});

describe("the text filter", () => {
  it("matches the id and the intent, anywhere in either, in any case", () => {
    const goal = row("g1-s12", { intent: "The board, and approve by drag" });
    expect(matchesText(goal, "")).toBe(true);
    expect(matchesText(goal, "   ")).toBe(true);
    expect(matchesText(goal, "S12")).toBe(true);
    expect(matchesText(goal, "APPROVE")).toBe(true);
    expect(matchesText(goal, "  drag ")).toBe(true);
    expect(matchesText(goal, "withdraw")).toBe(false);
  });
});

describe("the rank filters", () => {
  it("take any, or exactly one of the three", () => {
    expect(matchesRank("", 0)).toBe(true);
    expect(matchesRank("2", 2)).toBe(true);
    expect(matchesRank("2", 3)).toBe(false);
  });

  // A record that declares no priority carries 0, which is not a priority and
  // must not answer to one.
  it("never match a record that declares no rank", () => {
    for (const rank of ["1", "2", "3"] as const) {
      expect(matchesRank(rank, 0)).toBe(false);
    }
  });

  it("read a stored rank, and refuse anything that is not one", () => {
    expect(rankOf("2")).toBe("2");
    expect(rankOf("4")).toBe("");
    expect(rankOf("")).toBe("");
    expect(rankOf(null)).toBe("");
  });
});

describe("the seat and arc filters", () => {
  const held = row("held", { claim: { machine: "m1e", lineage: "coordinator", at: "", landingAt: "" } });
  const free = row("free");
  const inArc = row("in-arc", { arc: "harvest" });
  const oddSeat = row("odd", { claim: { machine: NONE, lineage: "l", at: "", landingAt: "" } });

  it("tell unassigned work from a seat, and a goal in no arc from an arc", () => {
    expect(matchesSeat(held, ANY)).toBe(true);
    expect(matchesSeat(held, NONE)).toBe(false);
    expect(matchesSeat(free, NONE)).toBe(true);
    expect(matchesSeat(held, named("m1e"))).toBe(true);
    expect(matchesSeat(free, named("m1e"))).toBe(false);

    expect(matchesArc(inArc, ANY)).toBe(true);
    expect(matchesArc(inArc, NONE)).toBe(false);
    expect(matchesArc(free, NONE)).toBe(true);
    expect(matchesArc(inArc, named("harvest"))).toBe(true);
  });

  // The prefix is the whole reason this holds: a seat whose machine is spelled
  // like the sentinel is still a seat, and asking for unassigned work must not
  // return it.
  it("keep a seat named like the sentinel apart from unassigned work", () => {
    expect(matchesSeat(oddSeat, NONE)).toBe(false);
    expect(matchesSeat(oddSeat, named(NONE))).toBe(true);
    expect(matchesSeat(free, named(NONE))).toBe(false);
  });

  it("offer the seats and arcs that are on the board, sorted and without repeats", () => {
    const rows = [held, free, inArc, row("second", { arc: "harvest" }), row("b", { arc: "another" })];
    expect(seatsOn(rows)).toEqual(["m1e"]);
    expect(arcsOn(rows)).toEqual(["another", "harvest"]);
    expect(seatsOn([free])).toEqual([]);
  });
});

describe("a set of filters", () => {
  it("narrows on every field at once", () => {
    const goal = row("g1-s12", { tier: 2, priority: 1, arc: "harvest" });
    expect(matches(goal, noFilters)).toBe(true);
    expect(matches(goal, set({ text: "s12", priority: "1", tier: "2", arc: named("harvest") }))).toBe(true);
    expect(matches(goal, set({ text: "s12", priority: "3" }))).toBe(false);
    expect(matches(goal, set({ seat: NONE }))).toBe(true);
  });

  it("says whether it narrows anything, which is when clear is offered", () => {
    expect(anySet(noFilters)).toBe(false);
    expect(anySet(set({ text: "   " }))).toBe(false);
    expect(anySet(set({ text: "a" }))).toBe(true);
    expect(anySet(set({ priority: "1" }))).toBe(true);
    expect(anySet(set({ seat: NONE }))).toBe(true);
    expect(anySet(set({ arc: named("harvest") }))).toBe(true);
  });
});

describe("the Done lane's reach", () => {
  const now = new Date("2026-09-22T12:00:00Z");
  const at = (stamp: string) => row("done", { lane: "done", state: "done", doneAt: stamp });

  it("offers the windows the master's board asks for, defaulting to one day", () => {
    expect(WINDOWS.map((days) => windowTitle(days))).toEqual(["1", "2", "3", "7", "14", "30", "90", "all"]);
  });

  it("contains a conclusion inside the window and not one outside it", () => {
    expect(concludedWithin(at("2026-09-22T11:00:00Z"), 1, now)).toBe(true);
    expect(concludedWithin(at("2026-09-21T13:00:00Z"), 1, now)).toBe(true);
    expect(concludedWithin(at("2026-09-21T11:00:00Z"), 1, now)).toBe(false);
    expect(concludedWithin(at("2026-09-21T11:00:00Z"), 2, now)).toBe(true);
    // Ninety days before this instant is 2026-06-24.
    expect(concludedWithin(at("2026-07-01T00:00:00Z"), 90, now)).toBe(true);
    expect(concludedWithin(at("2026-06-01T00:00:00Z"), 90, now)).toBe(false);
  });

  // The two clocks are not the same clock. A goal concluded a second ago must
  // not vanish because this browser is a second behind the machine that
  // concluded it.
  it("counts a stamp ahead of the reader's clock as having happened", () => {
    expect(concludedWithin(at("2026-09-22T12:00:30Z"), 1, now)).toBe(true);
  });

  it("puts an undated conclusion in no window of days, and in all", () => {
    expect(concludedWithin(at(""), 1, now)).toBe(false);
    expect(concludedWithin(at(""), 90, now)).toBe(false);
    expect(concludedWithin(at(""), null, now)).toBe(true);
    expect(concludedWithin(at("not a date"), 1, now)).toBe(false);
    expect(concludedWithin(at("not a date"), null, now)).toBe(true);
  });
});

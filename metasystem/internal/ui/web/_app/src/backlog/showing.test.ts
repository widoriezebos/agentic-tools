import { describe, expect, it } from "vitest";

import type { Row } from "./api";
import { named, noFilters, type Filters } from "./filters";
import { landingFor, placementOf, showLabel, type Placement } from "./showing";

/**
 * Landing on a goal the address named.
 *
 * Each case below is a way the goal could have been out of sight when the
 * page opened, and what the landing does about it: the Done window that does
 * not reach it, the toggle that is shut over it, the filter that excludes it,
 * and the id nothing on the board carries. The rule that nothing else is
 * touched is asserted as hard as the rules that something is: a landing that
 * widened a window it did not need to widen would leave a human wondering
 * what else it had changed.
 */

const NOW = new Date("2026-09-23T12:00:00Z");

function row(over: Partial<Row> = {}): Row {
  return {
    ref: { kind: "goal", id: "g1-s23", revision: 1 },
    where: "plans/goals/g1-s23.md",
    lane: "to-do",
    phase: "",
    state: "open",
    intent: "The board lands on the goal a link named.",
    nextStep: "",
    concluded: "",
    origin: "human",
    priority: 2,
    sequence: 1,
    tier: 1,
    labels: [],
    arc: "",
    pinned: "",
    blockedBy: [],
    holds: [],
    openBlockers: [],
    sliced: false,
    decomposed: false,
    openedAt: "2026-09-01T09:00:00Z",
    doneAt: "",
    lastChangeAt: "2026-09-01T09:00:00Z",
    lastVerb: "goal open",
    gaps: [],
    ...over,
  };
}

/** A conclusion this many days before the instant the payload was observed. */
function daysAgo(days: number): string {
  return new Date(NOW.getTime() - days * 24 * 60 * 60 * 1000).toISOString();
}

describe("the link the goal page carries", () => {
  // The link promises a place, and the two views are two different places. A
  // human who reads the list is not sent to a board.
  it("names the view this browser is going to get", () => {
    expect(showLabel("board")).toBe("Show on the board →");
    expect(showLabel("list")).toBe("Show in the list →");
  });
});

describe("where a goal stands", () => {
  const open = row();
  const delivered = row({ lane: "done", doneAt: daysAgo(10) });
  const dropped = row({ lane: "abandoned", doneAt: daysAgo(10) });
  const parent = row({ lane: "done", decomposed: true, doneAt: daysAgo(10) });

  it("is the lane it is in, where the board always shows that lane", () => {
    expect(placementOf([open], "g1-s23", NOW, "board")).toEqual({ where: "open", row: open });
  });

  // The board reads Done through a window of days; the smallest window that
  // reaches this conclusion is what a landing would widen to.
  it("is Done and the window that reaches it, on the board", () => {
    expect(placementOf([delivered], "g1-s23", NOW, "board")).toEqual({
      where: "done",
      row: delivered,
      within: 14,
    });
  });

  // A conclusion nothing dated is in no window of days at all, so the only
  // reading that carries it is every recorded one.
  it("is every recorded conclusion, for a goal whose conclusion nothing dated", () => {
    const undated = row({ lane: "done" });
    expect(placementOf([undated], "g1-s23", NOW, "board")).toEqual({
      where: "done",
      row: undated,
      within: null,
    });
  });

  // The list has no window: both concluded lanes stand behind its own toggle.
  it("is the closed group, for delivered work in the list", () => {
    expect(placementOf([delivered], "g1-s23", NOW, "list")).toEqual({ where: "closed", row: delivered });
  });

  it("is the closed group for the abandoned and for a goal a split retired", () => {
    expect(placementOf([dropped], "g1-s23", NOW, "board")).toEqual({ where: "closed", row: dropped });
    expect(placementOf([parent], "g1-s23", NOW, "board")).toEqual({ where: "closed", row: parent });
  });

  // The three the reader shows nowhere: an id the payload does not carry, a
  // record this build could not place, and a draft nothing reads.
  it("is nowhere, for an id no card and no row carries", () => {
    expect(placementOf([open], "g1-s99", NOW, "board")).toEqual({ where: "missing" });
    expect(placementOf([row({ lane: "unknown" })], "g1-s23", NOW, "board")).toEqual({ where: "missing" });
    expect(placementOf([row({ lane: "draft" })], "g1-s23", NOW, "board")).toEqual({ where: "missing" });
  });
});

describe("what a landing changes", () => {
  const open: Placement = { where: "open", row: row() };
  const delivered = row({ lane: "done", doneAt: daysAgo(10) });
  const done: Placement = { where: "done", row: delivered, within: 14 };
  const closed: Placement = { where: "closed", row: row({ lane: "abandoned" }) };

  it("is nothing at all, for a goal already in view", () => {
    expect(landingFor("g1-s23", open, 1, false, noFilters)).toEqual({ notes: [] });
  });

  it("is nothing, where Done already reaches the goal", () => {
    expect(landingFor("g1-s23", done, 30, false, noFilters)).toEqual({ notes: [] });
    expect(landingFor("g1-s23", done, null, false, noFilters)).toEqual({ notes: [] });
  });

  // The smallest window that reaches it, so the goal comes into view and the
  // lane stays as short as it can be.
  it("widens Done to the smallest window that reaches the goal, and says so", () => {
    expect(landingFor("g1-s23", done, 1, false, noFilters)).toEqual({
      doneDays: 14,
      notes: ["Showing g1-s23: widened Done to 14 days"],
    });
  });

  it("widens Done to every recorded conclusion where no window of days reaches it", () => {
    const undated: Placement = { where: "done", row: row({ lane: "done" }), within: null };
    expect(landingFor("g1-s23", undated, 30, false, noFilters)).toEqual({
      doneDays: null,
      notes: ["Showing g1-s23: widened Done to every recorded conclusion"],
    });
  });

  it("opens the closed items, and only while they are shut", () => {
    expect(landingFor("g1-s23", closed, 1, false, noFilters)).toEqual({
      openClosed: true,
      notes: ["Showing g1-s23: opened closed items"],
    });
    expect(landingFor("g1-s23", closed, 1, true, noFilters)).toEqual({ notes: [] });
  });

  // A filter set is left alone unless it is the thing hiding the goal: a
  // human narrowed to their own seat has not asked for that to be undone
  // because a card they can already see was linked to.
  it("clears the filters only where they are what is hiding the goal", () => {
    const seated: Filters = { ...noFilters, seat: named("orion") };
    expect(landingFor("g1-s23", open, 1, false, seated)).toEqual({
      clearFilters: true,
      notes: ["Showing g1-s23: cleared the filters"],
    });
    const passing: Filters = { ...noFilters, text: "lands on the goal" };
    expect(landingFor("g1-s23", open, 1, false, passing)).toEqual({ notes: [] });
  });

  it("says both in one line where it had to do both", () => {
    const seated: Filters = { ...noFilters, tier: "3" };
    expect(landingFor("g1-s23", done, 1, false, seated)).toEqual({
      doneDays: 14,
      clearFilters: true,
      notes: ["Showing g1-s23: widened Done to 14 days, cleared the filters"],
    });
  });

  // Nothing is widened or opened for a goal that is not there to be shown,
  // and the page says so rather than landing silently on nothing.
  it("changes nothing and says so, for a goal the board does not carry", () => {
    expect(landingFor("g1-s99", { where: "missing" }, 1, false, noFilters)).toEqual({
      notes: ["g1-s99 is not on the board"],
    });
  });
});

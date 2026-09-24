import { describe, expect, it } from "vitest";

import type { Row } from "./api";
import {
  bandOf,
  inRankOrder,
  landedNote,
  needsConfirming,
  placementFor,
  rankOf,
  stepFor,
} from "./reorder";

/**
 * Where a card goes when it is dragged past another one.
 *
 * The case this file exists for is the first one: a band is not a lane. Two
 * goals at 2:3 and 2:4 are neighbours in the band and can be in different
 * columns, so a position computed from the column would be wrong on every
 * board with work in flight. The rest pin the engine's own arithmetic: the
 * position is counted in the band with the moved goal taken out, a request
 * for the rank a goal already has is not a request, and nothing here predicts
 * where the ledger put it.
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

/** A band of four at priority 2, spread across two lanes on purpose. */
const one = row("one", { priority: 2, sequence: 1 });
const two = row("two", { priority: 2, sequence: 2, lane: "ready", state: "approved" });
const three = row("three", { priority: 2, sequence: 3 });
const four = row("four", { priority: 2, sequence: 4, lane: "in-progress", state: "claimed" });
const other = row("other", { priority: 1, sequence: 1 });
const unranked = row("unranked", { priority: 0, sequence: 0 });
const concluded = row("concluded", { where: "archived", lane: "done", state: "done", priority: 2, sequence: 9 });
const all = [one, two, three, four, other, unranked, concluded];

describe("a priority band", () => {
  // The whole reason this is computed against the band: these four are in
  // three different lanes and are still one another's neighbours.
  it("is every live goal at that priority, whatever lane each one is in", () => {
    expect(bandOf(2, all).map((r) => r.ref.id)).toEqual(["one", "two", "three", "four"]);
    expect(bandOf(1, all).map((r) => r.ref.id)).toEqual(["other"]);
  });

  it("leaves out the goal being moved, which is the list the engine inserts into", () => {
    expect(bandOf(2, all, "two").map((r) => r.ref.id)).toEqual(["one", "three", "four"]);
  });

  // The engine compacts a band when a goal leaves the live set, so a
  // concluded goal's old rank is nobody's neighbour.
  it("holds no concluded goal, whatever rank its record still carries", () => {
    expect(bandOf(2, all).map((r) => r.ref.id)).not.toContain("concluded");
  });
});

describe("a lane's order", () => {
  it("is priority, then sequence, with the unranked after them by id", () => {
    const shuffled = [four, unranked, two, other, one, row("aardvark", { priority: 0, sequence: 0 })];
    expect(inRankOrder(shuffled).map((r) => r.ref.id)).toEqual([
      "other",
      "one",
      "two",
      "four",
      "aardvark",
      "unranked",
    ]);
  });
});

describe("a drop beside a neighbour", () => {
  it("takes the neighbour's band and the side it was dropped on", () => {
    expect(placementFor(four, one, "above", all)).toEqual({ priority: 2, sequence: 1 });
    expect(placementFor(four, one, "below", all)).toEqual({ priority: 2, sequence: 2 });
    expect(placementFor(one, four, "below", all)).toEqual({ priority: 2, sequence: 4 });
  });

  it("crosses into the neighbour's band when the neighbour is in another one", () => {
    expect(placementFor(four, other, "above", all)).toEqual({ priority: 1, sequence: 1 });
    expect(placementFor(four, other, "below", all)).toEqual({ priority: 1, sequence: 2 });
  });

  // Taking the goal out first is what makes this a no-op: dropping a card
  // just above the one below it puts it back where it was.
  it("answers null for a drop that asks for the rank the goal already has", () => {
    expect(placementFor(two, three, "above", all)).toBeNull();
    expect(placementFor(two, one, "below", all)).toBeNull();
    expect(placementFor(two, two, "above", all)).toBeNull();
  });

  it("answers null for a neighbour that is in no band at all", () => {
    expect(placementFor(one, unranked, "above", all)).toBeNull();
    expect(placementFor(one, concluded, "above", all)).toBeNull();
  });
});

describe("a step through the band", () => {
  it("moves one place, and stops at each end", () => {
    expect(stepFor(three, "up", all)).toEqual({ priority: 2, sequence: 2 });
    expect(stepFor(three, "down", all)).toEqual({ priority: 2, sequence: 4 });
    expect(stepFor(one, "up", all)).toBeNull();
    expect(stepFor(four, "down", all)).toBeNull();
  });

  it("is offered on nothing that has no rank to step", () => {
    expect(stepFor(unranked, "up", all)).toBeNull();
    expect(stepFor(unranked, "down", all)).toBeNull();
    expect(stepFor(other, "up", all)).toBeNull();
    expect(stepFor(other, "down", all)).toBeNull();
  });

  // Stepping up by one and stepping back down returns the goal to its place,
  // which is the arithmetic being checked in both directions at once.
  it("is its own inverse", () => {
    const up = stepFor(three, "up", all);
    expect(up).toEqual({ priority: 2, sequence: 2 });
    const moved = [one, { ...three, sequence: 2 }, { ...two, sequence: 3 }, four, other];
    expect(stepFor({ ...three, sequence: 2 }, "down", moved)).toEqual({ priority: 2, sequence: 3 });
  });
});

describe("what the lane says afterwards", () => {
  // The note is read from the board the act answered with, never from what
  // was asked for: the band may have shifted since this page was read.
  it("reads the rank out of the ledger's own answer", () => {
    expect(landedNote("two", [{ ...two, priority: 1, sequence: 4 }])).toBe("two moved to 1:4");
    expect(landedNote("gone", [one])).toBe("gone moved");
    expect(rankOf({ priority: 2, sequence: 5 })).toBe("2:5");
  });
});

describe("a re-rank that needs confirming", () => {
  // The test is the claim rather than the lane, so Review — which is claimed
  // work waiting to land — gets the same sentence In Progress does.
  it("is one a seat is holding, in whichever lane that puts it", () => {
    expect(needsConfirming(four)).toBe(false);
    const held = { machine: "m1e", lineage: "coordinator", at: "", landingAt: "" };
    expect(needsConfirming({ ...four, claim: held })).toBe(true);
    expect(needsConfirming({ ...one, lane: "review", claim: held })).toBe(true);
    expect(needsConfirming(one)).toBe(false);
  });
});

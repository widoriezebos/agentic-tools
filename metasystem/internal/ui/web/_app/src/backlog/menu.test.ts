import { describe, expect, it } from "vitest";

import type { Row } from "./api";
import { itemAfter, offersFor, opensMenu, ranked } from "./menu";

/**
 * What a card offers.
 *
 * The rule under test is that the menu lists what is possible and nothing
 * else: an act the lane has no verb for, and a step the band has nowhere to
 * take, are absent rather than present and dim, because a disabled row is a
 * promise that the thing exists somewhere.
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

const first = row("first", { priority: 2, sequence: 1 });
const middle = row("middle", { priority: 2, sequence: 2 });
const last = row("last", { priority: 2, sequence: 3 });
const approved = row("approved", { lane: "ready", state: "approved", priority: 2, sequence: 4 });
const held = row("held", {
  lane: "in-progress",
  state: "claimed",
  priority: 2,
  sequence: 5,
  claim: { machine: "m1e", lineage: "coordinator", at: "", landingAt: "" },
});
const concluded = row("concluded", { where: "archived", lane: "done", state: "done", priority: 0, sequence: 0 });
const all = [first, middle, last, approved, held, concluded];

const labels = (of: Row) => offersFor(of, all).map((offer) => offer.label);
const ids = (of: Row) => offersFor(of, all).map((offer) => offer.id);

describe("what a card offers", () => {
  it("names the lane's own verb, the steps its band allows, and the goal", () => {
    expect(labels(middle)).toEqual(["Approve…", "Move up", "Move down", "Set priority…", "Open goal"]);
    expect(labels(approved)).toEqual(["Withdraw approval…", "Move up", "Move down", "Set priority…", "Open goal"]);
  });

  // An end of the band has nowhere to step, so the step is not listed. The
  // alternative — listing it dim — promises an act that does not exist here.
  // A band runs across the lanes, so the goal with nowhere to step down is
  // the last of the band and not the last card of any column.
  it("leaves out a step the band has nowhere to take", () => {
    expect(ids(first)).toEqual(["approve", "down", "rank", "open"]);
    expect(ids(last)).toEqual(["approve", "up", "down", "rank", "open"]);
    expect(ids(held)).not.toContain("down");
  });

  // In Progress has no lane move on this board, and the re-rank is still
  // offered because a claim does not fix a rank. The consequence of doing it
  // is the sheet's to say, not the menu's.
  it("offers a re-rank on claimed work and no lane move", () => {
    // Last in the band, so no step down either.
    expect(ids(held)).toEqual(["up", "rank", "open"]);
  });

  // A concluded goal is in no band and has no verb from its lane, so all it
  // can do is be opened — which every card can do.
  it("offers only the goal itself where nothing else is possible", () => {
    expect(ids(concluded)).toEqual(["open"]);
    expect(ranked(concluded)).toBe(false);
    expect(ranked(middle)).toBe(true);
  });

  it("always ends with the goal, and never lists anything twice", () => {
    for (const card of all) {
      const offered = ids(card);
      expect({ id: card.ref.id, last: offered[offered.length - 1] }).toEqual({ id: card.ref.id, last: "open" });
      expect({ id: card.ref.id, size: new Set(offered).size }).toEqual({ id: card.ref.id, size: offered.length });
    }
  });
});

describe("the keystrokes that open it", () => {
  // Shift+F10 is the one every platform has; the Menu key is the one some
  // keyboards have a key for. Nothing else opens a menu, because a card that
  // opened one on an ordinary key would be a card that could not be typed past.
  it("are Shift+F10 and the Menu key, and nothing else", () => {
    expect(opensMenu("F10", true)).toBe(true);
    expect(opensMenu("ContextMenu", false)).toBe(true);
    expect(opensMenu("ContextMenu", true)).toBe(true);
    expect(opensMenu("F10", false)).toBe(false);
    expect(opensMenu("Enter", true)).toBe(false);
    expect(opensMenu("m", true)).toBe(false);
  });
});

describe("moving inside the menu", () => {
  it("is a ring, and answers only the four keys that move", () => {
    expect(itemAfter("ArrowDown", 0, 3)).toBe(1);
    expect(itemAfter("ArrowDown", 2, 3)).toBe(0);
    expect(itemAfter("ArrowUp", 0, 3)).toBe(2);
    expect(itemAfter("Home", 2, 3)).toBe(0);
    expect(itemAfter("End", 0, 3)).toBe(2);
    expect(itemAfter("Tab", 0, 3)).toBeNull();
    expect(itemAfter("ArrowDown", 0, 0)).toBeNull();
  });
});

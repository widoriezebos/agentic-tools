import { describe, expect, it } from "vitest";

import type { Row } from "./api";
import { arcOn, isParent, membersOf, parentOf } from "./split";

/**
 * A split, read out of the payload as it already stands.
 *
 * The engine records no members list and no parent pointer. It concludes the
 * parent, appends its id to the root record's decomposed list, and gives each
 * member the parent's id as its arc. So the one case worth writing down is
 * the one that distinguishes the two readings of the same field: a goal in a
 * planning arc names an arc, and a split member names a parent, and only the
 * root's decomposed list tells them apart.
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

const parent = row("g1", {
  lane: "done",
  state: "done",
  where: "archived",
  decomposed: true,
  concluded: "decomposed into arc g1: goal:g1-s1, goal:g1-s2",
  doneAt: "2026-09-10T00:00:00Z",
});
const first = row("g1-s1", { arc: "g1" });
const second = row("g1-s2", { arc: "g1" });
const planned = row("harvest-1", { arc: "harvest" });
const alone = row("loner");
const all = [parent, first, second, planned, alone];

describe("a split", () => {
  it("names the parent the root record retired, and nothing else", () => {
    expect(isParent(parent)).toBe(true);
    expect(isParent(first)).toBe(false);
    expect(isParent(planned)).toBe(false);
  });

  it("finds a member's parent, and the goals a parent became", () => {
    expect(parentOf(first, all)?.ref.id).toBe("g1");
    expect(parentOf(second, all)?.ref.id).toBe("g1");
    expect(membersOf(parent, all).map((member) => member.ref.id)).toEqual(["g1-s1", "g1-s2"]);
  });

  // An arc is the general grouping too. A goal planned into an arc nobody
  // split has no parent, and saying it did would invent a decomposition.
  it("reads a planning arc as an arc and not as a parent", () => {
    expect(parentOf(planned, all)).toBeNull();
    expect(parentOf(alone, all)).toBeNull();
    expect(arcOn(planned, all)).toBe("harvest");
    expect(arcOn(alone, all)).toBe("");
  });

  // The parent is already named in full, with a link, as "part of …".
  it("does not also show a member's parent as its arc", () => {
    expect(arcOn(first, all)).toBe("");
  });

  // A member whose parent this payload does not carry still has an arc, and
  // that is what it is shown as: the payload proves a grouping and proves no
  // split.
  it("falls back to the arc when the parent is not in the payload", () => {
    expect(parentOf(first, [first, second])).toBeNull();
    expect(arcOn(first, [first, second])).toBe("g1");
  });
});

import { describe, expect, it } from "vitest";

import type { Budget, Row } from "./api";
import { lanes, type LaneId } from "./lanes";
import {
  blockedFor,
  budgetOf,
  draftOf,
  emptyBudgetDraft,
  moveFor,
  prefillFor,
  refusalFor,
  targetsFrom,
  transitions,
} from "./moves";

/**
 * The rule behind the board.
 *
 * Two drops act and the rest are refused in place; a budget is prefilled from
 * somewhere a human can name, or from nowhere at all; and a sheet whose
 * server cannot act as the human says so on the button before it is pressed.
 */

const everyLane: LaneId[] = lanes.map((lane) => lane.id);

function row(id: string, tier: number, over: Partial<Row> = {}): Row {
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
    tier,
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

function budget(elapsed: string): Budget {
  return {
    elapsedLimit: elapsed,
    attemptLimit: 6,
    reservedJobMinutesLimit: 720,
    activeJobLimit: 1,
    reviewRoundLimit: 2,
  };
}

describe("which drops act", () => {
  it("is exactly two moves, each one a verb the engine already has", () => {
    expect(transitions).toEqual([
      { from: "to-do", to: "ready", move: "approve", verb: "goal approve" },
      { from: "ready", to: "to-do", move: "withdraw", verb: "goal unapprove" },
    ]);
  });

  it("allows a drop from every lane onto exactly the lanes in the table", () => {
    const allowed: Record<string, LaneId[]> = {};
    for (const from of everyLane) {
      allowed[from] = everyLane.filter((to) => moveFor(from, to) !== null);
    }

    expect(allowed).toEqual({
      draft: [],
      "to-do": ["ready"],
      ready: ["to-do"],
      "in-progress": [],
      review: [],
      waiting: [],
      done: [],
      abandoned: [],
      unknown: [],
    });
  });

  it("offers the same targets the board highlights", () => {
    expect(targetsFrom("to-do")).toEqual(["ready"]);
    expect(targetsFrom("ready")).toEqual(["to-do"]);
    expect(targetsFrom("in-progress")).toEqual([]);
  });

  it("names the approval and the withdrawal by their verbs", () => {
    expect(moveFor("to-do", "ready")?.move).toBe("approve");
    expect(moveFor("ready", "to-do")?.move).toBe("withdraw");
  });
});

describe("why a drop is refused", () => {
  it("says nothing about the two that act", () => {
    expect(refusalFor("to-do", "ready")).toBe("");
    expect(refusalFor("ready", "to-do")).toBe("");
  });

  it("gives every other drop one line naming what the lane is read from", () => {
    for (const from of everyLane) {
      for (const to of everyLane) {
        if (moveFor(from, to) !== null) {
          continue;
        }
        const refusal = refusalFor(from, to);
        expect({ from, to, refused: refusal !== "" }).toEqual({ from, to, refused: true });
        expect({ from, to, lines: refusal.split("\n").length }).toEqual({ from, to, lines: 1 });
      }
    }
  });

  it("tells a human dropping a card where it already is", () => {
    expect(refusalFor("in-progress", "in-progress")).toBe("This goal is already in In Progress.");
  });

  it("names the claim, the park and the landing rather than apologising", () => {
    expect(refusalFor("to-do", "in-progress")).toContain("claim");
    expect(refusalFor("to-do", "waiting")).toContain("goal park");
    expect(refusalFor("ready", "done")).toContain("landing");
  });
});

describe("the budget the sheet opens with", () => {
  const law = { "2": budget("4h"), "3": budget("8h") };

  it("prefers the tuple the goal already carries", () => {
    const goal = row("g1", 3, { budget: budget("1h") });

    expect(prefillFor(goal, law, [goal])).toEqual({ budget: budget("1h"), source: "goal" });
  });

  it("falls to the project's law for the goal's own tier", () => {
    expect(prefillFor(row("g1", 2), law, [])).toEqual({ budget: budget("4h"), source: "project" });
  });

  it("reads an untiered goal as tier three, which is what the ledger does", () => {
    expect(prefillFor(row("g1", 0), law, [])).toEqual({ budget: budget("8h"), source: "project" });
  });

  it("falls to the most recently approved goal where the project declares none", () => {
    const older = row("older", 1, {
      budget: budget("2h"),
      approved: { by: "human:Wido", at: "2026-09-01T00:00:00Z", authority: "proven", reviewBy: "", expired: false, expiredWhy: "" },
    });
    const newer = row("newer", 1, {
      budget: budget("6h"),
      approved: { by: "human:Wido", at: "2026-09-20T00:00:00Z", authority: "proven", reviewBy: "", expired: false, expiredWhy: "" },
    });

    expect(prefillFor(row("g1", 1), {}, [older, newer])).toEqual({
      budget: budget("6h"),
      source: "last-approved",
    });
  });

  it("ignores an approved goal that carries no tuple", () => {
    const approvedWithout = row("older", 1, {
      approved: { by: "human:Wido", at: "2026-09-20T00:00:00Z", authority: "proven", reviewBy: "", expired: false, expiredWhy: "" },
    });

    expect(prefillFor(row("g1", 1), {}, [approvedWithout])).toEqual({ budget: null, source: "none" });
  });

  it("invents nothing when there is nothing to prefill from", () => {
    expect(prefillFor(row("g1", 1), {}, [])).toEqual({ budget: null, source: "none" });
  });
});

describe("the tuple a sheet makes", () => {
  it("round-trips a budget through the fields", () => {
    expect(budgetOf(draftOf(budget("1d2h")))).toEqual(budget("1d2h"));
  });

  it("makes nothing from an empty sheet", () => {
    expect(budgetOf(emptyBudgetDraft)).toBeNull();
  });

  it("refuses an elapsed limit that is not a duration, and a limit that is not positive", () => {
    const whole = draftOf(budget("4h"));

    expect(budgetOf({ ...whole, elapsedLimit: "soon" })).toBeNull();
    expect(budgetOf({ ...whole, elapsedLimit: "0h" })).toBeNull();
    expect(budgetOf({ ...whole, attemptLimit: "0" })).toBeNull();
    expect(budgetOf({ ...whole, activeJobLimit: "" })).toBeNull();
  });

  it("takes no review rounds as none rather than as missing", () => {
    expect(budgetOf({ ...draftOf(budget("4h")), reviewRoundLimit: "0" })?.reviewRoundLimit).toBe(0);
  });
});

describe("why the sheet's own button is disabled", () => {
  const whole = draftOf(budget("4h"));

  // Proof is no longer a reason to disable an act. A server that has found
  // no human behind this browser says so above the fields, the press goes to
  // the route all the same, and the sign-in the route asks for is a sheet
  // this page opens and an act it retries.
  it("is the incomplete budget, and nothing about proof", () => {
    expect(blockedFor("approve", emptyBudgetDraft)).toContain("complete budget");
  });

  it("is nothing at all for a whole budget, and nothing at all to withdraw", () => {
    expect(blockedFor("approve", whole)).toBe("");
    expect(blockedFor("withdraw", emptyBudgetDraft)).toBe("");
  });
});

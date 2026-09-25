import { describe, expect, it } from "vitest";

import type { Need } from "./api";
import {
  ANY_ORIGIN,
  approvePlan,
  bandLine,
  blockedLine,
  budgetLine,
  isNarrowed,
  labelsIn,
  mayDismiss,
  maySend,
  NEEDS_ITS_BUDGET,
  noNarrowing,
  parkPlan,
  progressLine,
  runInOrder,
  RUN_IS_OVER,
  queueCount,
  SEATS,
  sendable,
  shownQueue,
  stoppedLine,
  YOURS,
  type RunState,
} from "./decisions";
import type { Budget, Row } from "../backlog/api";

/**
 * The queue's own rules: what the tools leave on screen, what a bulk act
 * would send, and what a run says while it runs and when it stops.
 */

const box: Budget = {
  elapsedLimit: "4h", attemptLimit: 6, reservedJobMinutesLimit: 720,
  activeJobLimit: 1, reviewRoundLimit: 2,
};

function row(over: Partial<Row> = {}): Row {
  return {
    ref: { kind: "goal", id: "g1", revision: 3 },
    where: "live", lane: "todo", phase: "", state: "queued",
    intent: "Make the queue a queue", nextStep: "Start g1.", concluded: "",
    origin: "main", priority: 2, sequence: 1, tier: 3,
    labels: [], arc: "", pinned: "", blockedBy: [], openBlockers: [], holds: [],
    sliced: false, decomposed: false, openedAt: "2026-09-01T00:00:00Z",
    doneAt: "", lastChangeAt: "", lastVerb: "", gaps: [],
    ...over,
  } as Row;
}

function need(id: string, over: Partial<Row> = {}, since = "2026-09-01T00:00:00Z"): Need {
  return {
    kind: "approval", id, title: `Title of ${id}`, asked: "", by: "the backlog",
    since, deadline: "", silence: "", recommend: "",
    where: { kind: "goal", id }, act: "approve", command: "",
    row: row({ ref: { kind: "goal", id, revision: 3 }, openedAt: since, ...over }),
    new: false, words: "", context: "", owner: "", class: "", due: "", path: "", goals: [],
  };
}

/** One queue row, named, for the run tests below. */
function goal(id: string): Need {
  return need(id);
}

const queue: Need[] = [
  need("g1-s40", { labels: ["browser-interface"], origin: "human", tier: 2 }, "2026-09-22T00:00:00Z"),
  need("g1-s41", { labels: ["browser-interface", "robustness"], origin: "human" }, "2026-09-20T00:00:00Z"),
  need("g1-s42", { labels: ["headless-fleet"], origin: "main" }, "2026-09-24T00:00:00Z"),
  need("g1-s43", { labels: [], origin: "main", tier: 0 }, "2026-09-10T00:00:00Z"),
];

describe("the queue's three tools", () => {
  it("finds over the id, the intent and the labels", () => {
    expect(shownQueue(queue, { ...noNarrowing, find: "g1-s42" }).map((one) => one.id)).toEqual(["g1-s42"]);
    expect(shownQueue(queue, { ...noNarrowing, find: "MAKE THE QUEUE" })).toHaveLength(4);
    expect(shownQueue(queue, { ...noNarrowing, find: "nothing says this" })).toEqual([]);
    // The label is the one the board's own box cannot answer for.
    expect(shownQueue(queue, { ...noNarrowing, find: "headless" }).map((one) => one.id)).toEqual(["g1-s42"]);
  });

  it("draws the label chips from the rows on screen, with their counts, commonest first", () => {
    expect(labelsIn(queue)).toEqual([
      { label: "browser-interface", count: 2 },
      { label: "headless-fleet", count: 1 },
      { label: "robustness", count: 1 },
    ]);
  });

  it("narrows to one label at a time", () => {
    expect(shownQueue(queue, { ...noNarrowing, label: "browser-interface" }).map((one) => one.id)).toEqual([
      "g1-s40", "g1-s41",
    ]);
  });

  it("narrows to yours or to the seats', and to neither by default", () => {
    expect(shownQueue(queue, { ...noNarrowing, origin: YOURS }).map((one) => one.id)).toEqual(["g1-s40", "g1-s41"]);
    expect(shownQueue(queue, { ...noNarrowing, origin: SEATS }).map((one) => one.id)).toEqual(["g1-s42", "g1-s43"]);
    expect(shownQueue(queue, noNarrowing)).toHaveLength(4);
    expect(isNarrowed({ ...noNarrowing, origin: ANY_ORIGIN })).toBe(false);
    expect(isNarrowed({ ...noNarrowing, label: "robustness" })).toBe(true);
  });

  it("keeps the backlog's order by default and offers newest first by openedAt", () => {
    expect(shownQueue(queue, noNarrowing).map((one) => one.id)).toEqual(["g1-s40", "g1-s41", "g1-s42", "g1-s43"]);
    expect(shownQueue(queue, { ...noNarrowing, order: "newest" }).map((one) => one.id)).toEqual([
      "g1-s42", "g1-s40", "g1-s41", "g1-s43",
    ]);
  });

  it("says what was narrowed away rather than quietly showing fewer rows", () => {
    expect(queueCount(122, 122)).toBe("122 waiting");
    expect(queueCount(122, 16)).toBe("122 waiting · 16 shown");
  });
});

describe("a queue row", () => {
  it("says what opening it says: the budget, the band, and what holds it", () => {
    expect(budgetLine(need("g1-s60", { budget: box }))).toBe(
      "4h elapsed · 6 attempts · 720 reserved job minutes · 1 active jobs · 2 review rounds",
    );
    // A row whose record carries no tuple says so rather than inventing one.
    expect(budgetLine(queue[0])).toBe("no budget recorded");
    expect(bandLine(queue[0])).toBe("priority 2, position 1");
    expect(bandLine(need("g1-s61", { priority: 0, sequence: 0 }))).toBe("no priority band");
    expect(blockedLine(queue[0])).toBe("");
    expect(blockedLine(need("g1-s62", { blockedBy: ["g1-s24"], openBlockers: ["g1-s24"] }))).toBe("waits for g1-s24");
    expect(blockedLine(need("g1-s63", { blockedBy: ["g1-s24"], openBlockers: [] }))).toBe("waits for g1-s24, all done");
  });
});

describe("acting on what was selected", () => {
  it("carries the budget prefillFor would have shown, and its source's words", () => {
    const plan = approvePlan([queue[0]], { "2": box }, []);
    expect(plan[0].budget).toEqual(box);
    expect(plan[0].source).toBe("the project's budget law for this goal's tier");
    expect(plan[0].excluded).toBe("");
  });

  it("lists a goal with no prefill, names it, and does not send it", () => {
    const plan = approvePlan(queue, {}, []);
    expect(plan.every((one) => one.excluded === NEEDS_ITS_BUDGET)).toBe(true);
    expect(sendable(plan)).toHaveLength(0);

    // One goal carries its own tuple, so it is sent and the rest are not.
    const mixed = approvePlan([need("g1-s70", { budget: box }), queue[1]], {}, []);
    expect(sendable(mixed).map((one) => one.id)).toEqual(["g1-s70"]);
    expect(mixed[1].excluded).toBe("needs its budget first: approve it alone");
  });

  it("excludes nothing from a park, which carries no budget at all", () => {
    expect(sendable(parkPlan(queue))).toHaveLength(4);
  });

  it("says how far a run has got", () => {
    expect(progressLine(7, 12)).toBe("7 of 12");
  });

  // The whole of a run's safety: it stops, it names where, it gives the
  // engine's own words, and it refuses to say whether the goal it stopped at
  // landed — because an answer can fail after its publication succeeded.
  it("stops at a refusal, names it, and calls that goal unresolved rather than refused", () => {
    const plan = parkPlan(queue);
    const line = stoppedLine(plan[2], "goal g1-s42 is claimed by m2a+implementer.", 2, 4);

    expect(line).toContain("Stopped at g1-s42");
    expect(line).toContain("goal g1-s42 is claimed by m2a+implementer. — whether it landed");
    expect(line).toContain("unresolved");
    expect(line).not.toContain("refused");
    expect(line).toContain("1 after it were not sent.");
  });

  it("says nothing about goals after the last one when the last one failed", () => {
    const plan = parkPlan(queue);
    expect(stoppedLine(plan[3], "no.", 3, 4)).not.toContain("were not sent");
  });
});

describe("a run in flight", () => {
  // Closing would not stop the loop: it publishes one goal at a time whether
  // or not anything is on screen, and the progress line is the only place
  // that says how far it has got. So the sheet refuses to be dismissed, and
  // Cancel and Escape both come back through the same guard.
  it("cannot be dismissed while it publishes", () => {
    expect(mayDismiss({ state: "ready" })).toBe(true);
    expect(mayDismiss({ state: "running", done: 3, total: 12 })).toBe(false);
    expect(mayDismiss({ state: "stopped", line: "…" })).toBe(true);
  });

  it("sends nothing more when a close is attempted, and is not closed either", async () => {
    const sent: string[] = [];
    let closed = false;
    // The one way out of the sheet, guarded exactly as the panel guards it.
    const dismiss = (run: RunState) => {
      if (mayDismiss(run)) {
        closed = true;
      }
    };
    const plan = parkPlan([goal("g1-s40"), goal("g1-s41"), goal("g1-s42")]);
    let run: RunState = { state: "running", done: 0, total: plan.length };

    const outcome = await runInOrder(
      plan,
      async (one) => {
        // A human presses Cancel, and hits Escape, part-way through.
        dismiss(run);
        sent.push(one.id);
        await Promise.resolve();
      },
      (done) => {
        run = { state: "running", done, total: plan.length };
      },
    );

    expect(closed).toBe(false);
    // The run was neither cut short nor made to send anything twice.
    expect(sent).toEqual(["g1-s40", "g1-s41", "g1-s42"]);
    expect(outcome.sent).toBe(3);
    expect(outcome.stoppedAt).toBe(null);
  });
});

describe("a run that stopped", () => {
  // The whole point of the stop rule: what was published is published once,
  // what failed is attempted once, and what came after is never sent.
  it("sends each goal before the failure once, the failed one once, and nothing after", async () => {
    const sent: string[] = [];
    const plan = parkPlan([goal("g1-s40"), goal("g1-s41"), goal("g1-s42"), goal("g1-s43")]);

    const outcome = await runInOrder(
      plan,
      async (one) => {
        sent.push(one.id);
        await Promise.resolve();
        if (one.id === "g1-s42") {
          throw new Error("goal g1-s42 is claimed by m2a+implementer.");
        }
      },
      () => undefined,
    );

    expect(sent).toEqual(["g1-s40", "g1-s41", "g1-s42"]);
    expect(outcome.sent).toBe(2);
    expect(outcome.stoppedAt?.id).toBe("g1-s42");
  });

  // And it is terminal. Pressing the button again would send g1-s40 and
  // g1-s41 a second time, from the first — the one way this sheet could
  // publish the same act twice — so it never re-enables.
  it("never sends a second run from the same sheet", async () => {
    const stopped: RunState = { state: "stopped", line: "Stopped at g1-s42: …" };

    expect(maySend({ state: "ready" }, "")).toBe(true);
    expect(maySend({ state: "running", done: 1, total: 4 }, "")).toBe(false);
    expect(maySend(stopped, "")).toBe(false);
    // Not even once whatever else is true: a stop outranks an empty blocker.
    expect(maySend(stopped, "")).toBe(false);
    expect(RUN_IS_OVER).toContain("select what is still waiting and start a new run");

    // And the loop itself is never entered again: the sheet's send guard is
    // what a second press meets.
    const sent: string[] = [];
    if (maySend(stopped, "")) {
      await runInOrder(parkPlan([goal("g1-s40")]), async (one) => {
        sent.push(one.id);
        await Promise.resolve();
      }, () => undefined);
    }
    expect(sent).toEqual([]);
  });
});

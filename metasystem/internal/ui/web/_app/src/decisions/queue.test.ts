import { describe, expect, it } from "vitest";

import type { Need } from "./api";
import {
  ANY_ORIGIN,
  actLabelFor,
  approvePlan,
  bandLine,
  blockedLine,
  budgetLine,
  headerLine,
  isNarrowed,
  labelsIn,
  NEEDS_ITS_BUDGET,
  noNarrowing,
  parkPlan,
  progressLine,
  queueCount,
  rowLabels,
  SEATS,
  sendable,
  shownQueue,
  stoppedLine,
  YOURS,
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
  };
}

const queue: Need[] = [
  need("g1-s40", { labels: ["browser-interface"], origin: "human", tier: 2 }, "2026-09-22T00:00:00Z"),
  need("g1-s41", { labels: ["browser-interface", "robustness"], origin: "human" }, "2026-09-20T00:00:00Z"),
  need("g1-s42", { labels: ["headless-fleet"], origin: "main" }, "2026-09-24T00:00:00Z"),
  need("g1-s43", { labels: [], origin: "main", tier: 0 }, "2026-09-10T00:00:00Z"),
];

describe("the header", () => {
  it("says both counts in words, because two jobs share this page", () => {
    expect(headerLine({ needsYou: 132, asked: 10, waiting: 122, rulings: 160 })).toBe(
      "10 asked of you · 122 waiting for your approval",
    );
  });
});

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
  it("shows three labels and then says how many more", () => {
    const many = need("g1-s50", { labels: ["a", "b", "c", "d", "e"] });
    expect(rowLabels(many)).toEqual({ shown: ["a", "b", "c"], more: 2 });
    expect(rowLabels(queue[2])).toEqual({ shown: ["headless-fleet"], more: 0 });
  });

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
  it("names the count on each button", () => {
    expect(actLabelFor("approve", 12)).toBe("Approve 12 selected");
    expect(actLabelFor("park", 12)).toBe("Not now for 12 selected");
  });

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

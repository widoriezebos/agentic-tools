import { describe, expect, it } from "vitest";

import { answerOf, edgeAct, type Backlog } from "./api";

/**
 * What the server's response means.
 *
 * Every ledger state answers 200 carrying why it cannot be projected, so a
 * status outside the successful range is the engine failing to answer at all,
 * and the status is what the human acts on.
 */

function absentPayload(): Backlog {
  return {
    schemaVersion: 1,
    observedAt: "2026-09-21T19:05:12Z",
    ledger: {
      state: "absent",
      tip: "",
      committedAt: "",
      freshness: { state: "behind", since: "", detail: "no fetch of the canonical branch has completed on this clone yet" },
      stale: true,
      staleAfterSeconds: 1800,
      syncMode: "",
      stateRoot: "/work/repository/metasystem",
      message: "",
      problems: [],
      fetch: {
        outcome: "never",
        startedAt: "",
        finishedAt: "",
        tip: "",
        detail: "",
        message: "",
        failures: 0,
        cadence: "connected",
        nextAt: "2026-09-21T19:05:17Z",
        succeededAt: "",
        succeededTip: "",
      },
    },
    admission: { answered: false, message: "" },
    workingTree: { liveFiles: 155, archivedFiles: 430 },
    authority: { proven: true, human: "Wido", reason: "" },
    budgetDefaults: {},
    counts: {},
    draft: { statement: "plans/goals-drafts/ has no reader in the engine" },
    rows: [],
    closed: [],
  };
}

describe("the backlog response", () => {
  it("is the payload when the engine answered", async () => {
    const payload = absentPayload();
    const response = new Response(JSON.stringify(payload), {
      status: 200,
      headers: { "Content-Type": "application/json" },
    });

    await expect(answerOf("/api/backlog", response)).resolves.toEqual(payload);
  });

  it("is a refusal carrying the engine's own words when it could not answer", async () => {
    const response = new Response('{"error":"this engine was built without a ledger reader"}', { status: 500 });

    await expect(answerOf("/api/backlog", response)).rejects.toThrow(
      "this engine was built without a ledger reader",
    );
  });

  it("names the status where the engine explained nothing", async () => {
    for (const status of [404, 503]) {
      const response = new Response("", { status });
      await expect(answerOf("/api/backlog", response)).rejects.toThrow(
        `/api/backlog answered ${String(status)}`,
      );
    }
  });

  // An act's refusal carries the engine's own sentence and the code it
  // refused under, because that pair is the whole of what a human acts on.
  it("carries an act refusal's own words and code", async () => {
    const response = new Response('{"error":"goal g1 is claimed","code":"CONFLICT"}', { status: 409 });

    await expect(answerOf("/api/backlog/goals/g1/withdraw", response)).rejects.toMatchObject({
      message: "goal g1 is claimed",
      code: "CONFLICT",
      status: 409,
    });
  });
});

/**
 * What the two edge acts send.
 *
 * The path is the goal that WAITS and the body is the goal it waits for, on
 * both routes and whichever end of the relation the page acted from. That is
 * the one thing these acts decide, so it is the one thing asserted here; that
 * the request is then made is the one call site the cut guard counts.
 */
describe("the two edge acts", () => {
  it("names the goal that waits in the path and the goal it waits for in the body", () => {
    expect(edgeAct("waits-one", "the-blocker", "block")).toEqual({
      resource: "/api/backlog/goals/waits-one/block",
      body: { blocker: "the-blocker" },
    });
    expect(edgeAct("waits-one", "the-blocker", "unblock")).toEqual({
      resource: "/api/backlog/goals/waits-one/unblock",
      body: { blocker: "the-blocker" },
    });
  });

  // A "Holds" row on this goal's page is an act on the OTHER goal: it is that
  // goal that waits, and this one that it waits for. One route pair, read
  // from either end.
  it("acts on the other goal when the relation is read the other way round", () => {
    expect(edgeAct("the-dependent", "this-goal", "unblock")).toEqual({
      resource: "/api/backlog/goals/the-dependent/unblock",
      body: { blocker: "this-goal" },
    });
  });

  it("escapes an id that would otherwise change the path", () => {
    expect(edgeAct("a/b", "c", "block").resource).toBe("/api/backlog/goals/a%2Fb/block");
  });
});

import { describe, expect, it } from "vitest";

import { answerOf, type Backlog } from "./api";

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
      stale: false,
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
      },
    },
    admission: { answered: false, message: "" },
    workingTree: { liveFiles: 155, archivedFiles: 430 },
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

    await expect(answerOf(response)).resolves.toEqual(payload);
  });

  it("is a refusal naming the status when the engine could not answer", async () => {
    const response = new Response('{"error":"this engine was built without a ledger reader"}', { status: 500 });

    await expect(answerOf(response)).rejects.toThrow("/api/backlog answered 500");
  });

  it("names any other unsuccessful status just as plainly", async () => {
    for (const status of [404, 503]) {
      const response = new Response("", { status });
      await expect(answerOf(response)).rejects.toThrow(`/api/backlog answered ${String(status)}`);
    }
  });
});

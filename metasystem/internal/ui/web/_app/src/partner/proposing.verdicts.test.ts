import { describe, expect, it } from "vitest";

import { argumentsOf, dispatchOf, verbWord, type Line } from "./proposing";

/**
 * The two acts the proposal grammar gains (g1-s69 §6): Review, the verdict, and
 * Run, the goal's candidate. A card names them in the grammar's own words and
 * dispatches them to their own routes' bodies.
 */

function line(verb: string, fields: Record<string, string>): Line {
  return { id: "t1#0", turn: "t1", index: 0, verb, goal: "g", fields, explanation: "", offered: true, state: "proposed", version: 1 } as unknown as Line;
}

describe("the verdict and the candidate as proposals", () => {
  it("are named Review and Run", () => {
    expect(verbWord("review-goal")).toBe("Review");
    expect(verbWord("app-start")).toBe("Run");
  });

  it("dispatch to their routes' own bodies, and a verdict carries no brief of the Partner's", () => {
    const review = line("review-goal", { record: "metasystem/plans/reviews/review-of-g.md", verdict: "clear-to-land" });
    expect(dispatchOf(review)).toEqual({ act: "review", id: "g", record: "metasystem/plans/reviews/review-of-g.md", verdict: "clear-to-land", work: "" });
    expect(argumentsOf(review)).toEqual([
      { label: "Record", value: "metasystem/plans/reviews/review-of-g.md" },
      { label: "Verdict", value: "clear-to-land" },
    ]);
    expect(dispatchOf(line("app-start", {}))).toEqual({ act: "run", goal: "g" });
  });
});

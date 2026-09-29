import { describe, expect, it } from "vitest";

import { argumentsOf, dispatchOf, verbWord, type Line } from "./proposing";

/**
 * The design verb in the proposal grammar (g1-s66 D1): design review in its two
 * forms. The card names it Send to critique, shows the design, the goal that
 * funds it and the reader budget, and, answering a round, which round and its
 * decisions file; it dispatches to the design's own route with the goal, the
 * budget and the round answered.
 */

function line(fields: Record<string, string>): Line {
  return { id: "t1#0", turn: "t1", index: 0, verb: "design-review", goal: "partner-runs-the-design-loop", fields, explanation: "",
    offered: true, state: "proposed", version: 1 } as unknown as Line;
}

const DESIGN = "plans/designs/user-interface/g1-s66-the-loop-from-the-room.md";

describe("design review as a proposal", () => {
  it("is named Send to critique", () => {
    expect(verbWord("design-review")).toBe("Send to critique");
  });

  it("sends a design with the goal and the budget", () => {
    const send = line({ design: DESIGN, toolCalls: "30" });
    expect(argumentsOf(send)).toEqual([
      { label: "Design", value: DESIGN },
      { label: "Funded by", value: "partner-runs-the-design-loop" },
      { label: "Reader budget", value: "30 tool calls" },
    ]);
    expect(dispatchOf(send)).toEqual({ act: "critique", design: DESIGN, goal: "partner-runs-the-design-loop", toolCalls: 30, after: 0 });
  });

  it("answers a round with its own decisions file", () => {
    const answer = line({ design: DESIGN, toolCalls: "30", dispositions: "metasystem/artifacts/agents/rev1/rounds/1/decisions.md", after: "1" });
    expect(argumentsOf(answer)).toContainEqual({ label: "Answers", value: "round 1, with metasystem/artifacts/agents/rev1/rounds/1/decisions.md" });
    expect(dispatchOf(answer)).toEqual({ act: "critique", design: DESIGN, goal: "partner-runs-the-design-loop", toolCalls: 30, after: 1 });
  });
});

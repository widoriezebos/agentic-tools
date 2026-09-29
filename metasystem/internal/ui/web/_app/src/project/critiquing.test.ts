import { describe, expect, it } from "vitest";

import {
  answerWords,
  answerable,
  cardsOf,
  foldAsk,
  fundingGoal,
  pressRefusal,
  rowFor,
  sectionFor,
  stateLine,
  type DesignLoop,
  type DesignRound,
} from "./critiquing";

/**
 * The loop from the room's rules, read without a browser (g1-s66 D1, D2, D4):
 * which goal funds the critique, what the page says about the chain, which
 * press writes which of the engine's four values and what each needs first,
 * and when Answer the round may be pressed.
 */

const round: DesignRound = {
  round: 1,
  findings: [
    { id: "S66-01", severity: "high", material: true, claim: "the act cannot start a review", evidence: "intent_delivery.go:771" },
    { id: "S66-09", severity: "low", material: false, claim: "a word is loose in §6", evidence: "§6" },
  ],
  decisions: [],
  answerable: false,
};

const loop = (over: Partial<DesignLoop> = {}): DesignLoop => ({
  design: "plans/designs/g1-s66.md", toolCalls: 30, chain: "rev1", goal: "g", state: "deciding",
  status: "completed", round: 1, limit: 2, rounds: [round], ...over,
});

describe("the funding goal (D1)", () => {
  it("is preselected when the Goals line names one approved goal", () => {
    expect(fundingGoal(["g1"], { g1: "approved" })).toEqual({ chosen: "g1", choices: ["g1"] });
    expect(fundingGoal(["g1", "g2"], { g1: "queued", g2: "claimed" })).toEqual({ chosen: "g2", choices: ["g2"] });
  });

  it("is chosen by the human when several are approved, and offered from none", () => {
    expect(fundingGoal(["g1", "g2"], { g1: "approved", g2: "approved" })).toEqual({ chosen: "", choices: ["g1", "g2"] });
    expect(fundingGoal(["g1"], { g1: "queued" })).toEqual({ chosen: "", choices: [] });
  });
});

describe("the page's line about the chain", () => {
  it("says the round, the limit and who is reading", () => {
    expect(stateLine(loop({ state: "reading", status: "running", critic: "gpt-6-astra" }))).toBe(
      "Critique · round 1 of 2 · gpt-6-astra reading",
    );
    expect(stateLine(loop({ state: "reading", status: "running" }))).toBe("Critique · round 1 of 2 · the critic reading");
    expect(stateLine(loop())).toBe("Critique · round 1 of 2 · 2 findings wait for your decisions");
    expect(stateLine(loop({ state: "answered" }))).toBe("Critique · round 1 of 2 · every finding is decided");
    expect(stateLine(loop({ state: "closed", round: 2 }))).toBe("Critique · closed at round 2");
    expect(stateLine(loop({ state: "none", chain: "" }))).toBe("");
    expect(stateLine(loop({ state: "none", chain: "", chains: 2 }))).toBe(
      "This design has 2 critique chains; the engine chooses none of them, and neither does this page",
    );
  });
});

describe("the three presses (D2)", () => {
  const material = round.findings[0];
  const small = round.findings[1];

  it("writes the engine's four values, and nothing without what it needs", () => {
    expect(rowFor("fold", material, { reasoning: "", amendment: "§4 D1 names the budget" })).toEqual({
      finding: "S66-01", disposition: "accepted", reasoning: "", amendment: "§4 D1 names the budget",
    });
    expect(rowFor("refute", material, { reasoning: "line 771 reads the flag", amendment: "" }).disposition).toBe("refuted");
    expect(rowFor("defer", small, { reasoning: "", amendment: "" }).disposition).toBe("noted");
    expect(rowFor("defer", material, { reasoning: "outside the brief's scope", amendment: "" }).disposition).toBe("out-of-scope");
  });

  it("refuses a refutation without a reason and a material deferral without its evidence", () => {
    expect(pressRefusal("refute", material, { reasoning: " ", amendment: "" })).toBe(
      "A refutation carries your reason: the check you made and what it showed.",
    );
    expect(pressRefusal("defer", material, { reasoning: "", amendment: "" })).toBe(
      "A material finding is deferred only as out of scope, with the evidence that it is outside the brief.",
    );
    expect(pressRefusal("defer", small, { reasoning: "", amendment: "" })).toBe("");
    expect(pressRefusal("fold", material, { reasoning: "", amendment: "" })).toBe("A fold carries its one-line amendment.");
  });

  it("finds the row a reload reads back from the file", () => {
    const decided = { ...round, decisions: [{ finding: "S66-01", disposition: "refuted", reasoning: "r", amendment: "" }] };
    const cards = cardsOf(decided);
    expect(cards.map((card) => [card.finding.id, card.row?.disposition ?? ""])).toEqual([["S66-01", "refuted"], ["S66-09", ""]]);
  });

  it("offers Answer the round only when every card has its row, on the round the engine says is answerable", () => {
    expect(answerable(loop())).toBe(false);
    expect(answerable(loop({ state: "answered", rounds: [{ ...round, answerable: true }] }))).toBe(true);
    expect(answerable(loop({ state: "closed", rounds: [{ ...round, answerable: true }] }))).toBe(false);
  });
});

describe("the fold (D3)", () => {
  const headings = ["The loop", "3. The room", "4. Decisions", "Outcome"];

  it("starts on the heading the finding names, and names none it cannot find", () => {
    expect(sectionFor({ ...round.findings[0], claim: "4. Decisions: the act as written cannot start a review" }, headings)).toBe("4. Decisions");
    expect(sectionFor({ ...round.findings[0], evidence: "§3 The room" }, headings)).toBe("");
  });

  it("asks the Partner for that section anew, with the finding", () => {
    const asked = foldAsk("plans/designs/g1-s66.md", "4. Decisions", round.findings[0], 1);
    expect(asked).toContain("Fold finding S66-01 of round 1");
    expect(asked).toContain("“4. Decisions” of plans/designs/g1-s66.md");
    expect(asked).toContain("the act cannot start a review");
    expect(asked).toContain("suggest with document and section");
  });
});

describe("the engine's words", () => {
  it("are shown as said, the lines it printed beside them", () => {
    expect(answerWords({ outcome: "refused", summary: "a review brief states the reader's tool-call budget, and none is configured",
      decision: "name it with --tool-calls N" })).toEqual([
      "a review brief states the reader's tool-call budget, and none is configured", "name it with --tool-calls N",
    ]);
    expect(answerWords({ outcome: "refused", summary: "the close owner stopped (exit 10) and chain rev1 is not closed",
      lines: ["cap-exhausted-human-raise: findings I1 require human action"] })).toEqual([
      "the close owner stopped (exit 10) and chain rev1 is not closed", "cap-exhausted-human-raise: findings I1 require human action",
    ]);
  });
});

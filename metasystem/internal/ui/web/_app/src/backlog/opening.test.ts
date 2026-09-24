import { describe, expect, it } from "vitest";

import {
  blockedForOpen,
  chosenStop,
  derivedTier,
  emptyIntake,
  emptyRisk,
  goalOf,
  idRefusal,
  intakeFor,
  keepsDerived,
  labelsOf,
  openNote,
  overridesTier,
  slugFrom,
  tierLine,
  unopenable,
  whichFieldRefused,
  SCORES,
  type Intake,
  type Risk,
  oneLine,
} from "./opening";
import { blockedForRank } from "./RankSheet";

/**
 * What the intake sheet will and will not send.
 *
 * The disabled states are the point: a human reads why the button is off
 * before they press it, and every reason names the field rather than saying
 * that something is missing. None of them is about proof: a server that has
 * found no human behind this browser says so above the fields and takes the
 * press all the same, because the route answers it with a sign-in this page
 * can open and an act it can send again.
 */

const whole: Intake = {
  id: "ui-new",
  intent: "The board opens a goal.",
  nextStep: "Take it to a working end state; the approach is yours.",
  tier: "",
  why: "",
  labels: "ui board",
  blocks: [],
  blockedBy: [],
};

const answered: Risk = { severity: "1", novelty: "2", exposure: "1", accumulation: "3", basis: "one seam" };

describe("the derived tier", () => {
  // Severity and novelty derive it; exposure and accumulation scale the proof
  // rather than lifting it, which is internal/goal/file.go's own rule.
  it("is the worse of severity and novelty, and nothing else", () => {
    expect(derivedTier(answered)).toBe(2);
    expect(derivedTier({ ...answered, severity: "3" })).toBe(3);
    expect(derivedTier({ ...answered, accumulation: "3", exposure: "3" })).toBe(2);
    expect(derivedTier(emptyRisk)).toBe(1);
  });

  it("is overridden only by a tier that is not it", () => {
    expect(overridesTier(whole, answered)).toBe(false);
    expect(overridesTier({ ...whole, tier: "2" }, answered)).toBe(false);
    expect(overridesTier({ ...whole, tier: "3" }, answered)).toBe(true);
  });
});

describe("the labels", () => {
  it("are separated however they were typed, and empty means none", () => {
    expect(labelsOf("ui board")).toEqual(["ui", "board"]);
    expect(labelsOf("ui, board")).toEqual(["ui", "board"]);
    expect(labelsOf("  ui ,,  board  ")).toEqual(["ui", "board"]);
    expect(labelsOf("")).toEqual([]);
    expect(labelsOf("   ")).toEqual([]);
  });
});

describe("what travels to the open route", () => {
  it("is trimmed, numbered, and carries the derived tier as zero", () => {
    expect(goalOf({ ...whole, id: "  ui-new  " }, answered)).toEqual({
      id: "ui-new",
      intent: "The board opens a goal.",
      nextStep: "Take it to a working end state; the approach is yours.",
      tier: 0,
      why: "",
      blocks: [],
      blockedBy: [],
      labels: ["ui", "board"],
      severity: 1,
      novelty: 2,
      exposure: 1,
      accumulation: 3,
      basis: "one seam",
    });
    expect(goalOf({ ...whole, tier: "3", why: "it touches the ledger" }, answered).tier).toBe(3);
  });

  // Both directions travel as lists, in the order a human chose them: the
  // first goal named on the blocked-by side is the one the engine's park
  // takes its marker from, so a body that sorted them would be changing the
  // record.
  it("carries both directions of the relation, in the order they were chosen", () => {
    const asked = goalOf({ ...whole, blocks: ["beta", "alpha"], blockedBy: ["gamma"] }, answered);
    expect(asked.blocks).toEqual(["beta", "alpha"]);
    expect(asked.blockedBy).toEqual(["gamma"]);
  });
});

describe("which picker an engine refusal is about", () => {
  // A goal that cannot be parked is refused by name. The sentence belongs
  // under the field that named that goal, two disclosures down, rather than
  // in the foot.
  it("is the picker that named the goal the refusal names", () => {
    const asked: Intake = { ...whole, blocks: ["fenced-work"], blockedBy: ["gamma"] };
    expect(whichFieldRefused("goal fenced-work is breach-stopped by stop-1", asked)).toBe("blocks");
    expect(whichFieldRefused("goal gamma is not live", asked)).toBe("blockedBy");
  });

  it("is neither when the refusal names no chosen goal", () => {
    expect(whichFieldRefused("the ledger could not be read", whole)).toBe("");
    expect(whichFieldRefused("goal ui-new already exists", { ...whole, blocks: ["alpha"] })).toBe("");
  });
});

describe("why the open button is disabled", () => {
  it("is nothing at all when the statement is complete", () => {
    expect(blockedForOpen(whole, answered)).toBe("");
  });

  // Proof is no longer a reason to disable the act, so an empty sheet's
  // first answer is the first field rather than a sentence about the server.
  it("answers an empty sheet with its first field", () => {
    expect(blockedForOpen(emptyIntake, emptyRisk)).toMatch(/named by one id/);
  });

  it("names the field that is missing, one at a time, in the order they are asked", () => {
    expect(blockedForOpen({ ...whole, id: " " }, answered)).toMatch(/named by one id/);
    expect(blockedForOpen({ ...whole, intent: "" }, answered)).toMatch(/what done looks like/);
    expect(blockedForOpen({ ...whole, nextStep: "" }, answered)).toMatch(/never a script/);
    expect(blockedForOpen(whole, { ...answered, basis: "  " })).toMatch(/basis is one line/);
  });

  // The engine requires a why for an overridden tier, so the sheet asks for
  // one rather than letting the act be refused after it is sent.
  it("asks why for a tier the risk answers did not derive", () => {
    const overridden = { ...whole, tier: "3" as const };
    expect(blockedForOpen(overridden, answered)).toMatch(/derive tier 2/);
    expect(blockedForOpen({ ...overridden, why: "it touches the ledger" }, answered)).toBe("");
  });
});

describe("what the sheet promises", () => {
  it("names the human it will act as, and says the goal arrives unapproved", () => {
    expect(openNote("Wido")).toContain("human:Wido");
    expect(openNote("")).toContain("the enrolled human");
    expect(openNote("Wido")).toContain("opening it authorizes nothing");
  });
});

describe("the intake a sheet opens on", () => {
  it("starts on the intent it was given and on nothing else", () => {
    expect(intakeFor("  The design's own title  ")).toEqual({
      ...emptyIntake,
      intent: "The design's own title",
    });
  });

  it("is the empty intake where nothing knows what the goal is for", () => {
    expect(intakeFor("")).toEqual(emptyIntake);
  });

  // A pre-filled intent is a first draft and not an act: the button is still
  // off until a human has written the rest of what the verb requires.
  it("does not on its own make the act sendable", () => {
    expect(blockedForOpen(intakeFor("The design's own title"), emptyRisk)).not.toBe("");
  });
});

describe("the id suggested from the intent", () => {
  it("is the first words, lowercased and hyphenated", () => {
    expect(slugFrom("The board opens a goal")).toBe("the-board-opens-a-goal");
    expect(slugFrom("Refunds Are Issued")).toBe("refunds-are-issued");
  });

  it("keeps only what an id may be made of", () => {
    expect(slugFrom("Refunds: issued, within a day!")).toBe("refunds-issued-within-a-day");
    expect(slugFrom("  spaced   out  ")).toBe("spaced-out");
    expect(slugFrom("CI/CD is 90% of it")).toBe("ci-cd-is-90-of-it");
  });

  it("is cut at a sane length, and cut at a word", () => {
    // Six words at the most, whatever they are.
    expect(slugFrom("one two three four five six seven eight")).toBe("one-two-three-four-five-six");
    // Forty characters at the most, and the word that would pass it is left
    // out whole rather than truncated.
    const long = slugFrom("interface refactoring programme quarterly review");
    expect(long).toBe("interface-refactoring-programme");
    expect(long.length).toBeLessThanOrEqual(40);
    // One word longer than the whole allowance is cut rather than dropped:
    // an empty suggestion would help nobody.
    expect(slugFrom("a".repeat(60))).toBe("a".repeat(40));
  });

  it("is nothing at all where there is nothing to make one from", () => {
    expect(slugFrom("")).toBe("");
    expect(slugFrom("   ")).toBe("");
    expect(slugFrom("!!! ???")).toBe("");
  });
});

describe("what the id field refuses", () => {
  it("says nothing about a field nobody has typed in", () => {
    expect(idRefusal("", ["ui-new"])).toBe("");
    expect(idRefusal("   ", ["ui-new"])).toBe("");
  });

  // internal/goal/goal.go's validId: lowercase letters, digits and hyphens.
  it("refuses anything that is not kebab-case", () => {
    expect(idRefusal("Refund Worker", [])).toMatch(/kebab-case/);
    expect(idRefusal("refund_worker", [])).toMatch(/kebab-case/);
    expect(idRefusal("RefundWorker", [])).toMatch(/kebab-case/);
    expect(idRefusal("refund-worker", [])).toBe("");
    expect(idRefusal("refund-worker-2", [])).toBe("");
  });

  // The tighter of the engine's two bounds is MaxIdBytes, which is 64.
  it("refuses an id past the bound a goal file must pass", () => {
    expect(idRefusal("a".repeat(64), [])).toBe("");
    expect(idRefusal("a".repeat(65), [])).toMatch(/at most 64 characters/);
  });

  it("refuses an id the ledger already carries, live or closed", () => {
    expect(idRefusal("ui-new", ["ui-old", "ui-new"])).toMatch(/already carries ui-new/);
    expect(idRefusal("  ui-new  ", ["ui-new"])).toMatch(/already carries ui-new/);
    expect(idRefusal("ui-newer", ["ui-new"])).toBe("");
  });
});

describe("the four risk answers as the sheet asks them", () => {
  // Not one syllable of this is the browser's: the questions are the paper's
  // and the stops are plans/severity-tiered-rigor-p2-design.md's.
  it("are the kit's four, in the engine's own order", () => {
    expect(SCORES.map((score) => score.key)).toEqual(["severity", "novelty", "exposure", "accumulation"]);
  });

  it("carry the kit's question and its three stops, and never a stop of ours", () => {
    for (const score of SCORES) {
      expect(score.question).not.toBe("");
      expect(score.stops).toHaveLength(3);
      expect(score.stops.every((stop) => stop !== "")).toBe(true);
    }
    const severity = SCORES[0];
    expect(severity.question).toBe("How severe could the harm be if the change is wrong?");
    expect(severity.stops[0]).toBe("visible and reversible on one machine");
    expect(severity.stops[2]).toBe("irreversible, or it moves authority, secrets or a landing bar");
    expect(SCORES[1].stops[2]).toBe("a new law, verb, schema, seam or role");
    expect(SCORES[2].stops[1]).toBe("every seat of the fleet");
    expect(SCORES[3].stops[0]).toBe("broadly examined since its last change");
  });

  // The tier is one line naming the two answers it came from, which is the
  // answer to "why is there a Tier here when I just chose four boxes".
  it("are read back as the tier they derive, and as the two that derived it", () => {
    expect(tierLine(answered)).toBe("Tier 2, from severity 1 and novelty 2.");
    expect(tierLine({ ...answered, severity: "3" })).toBe("Tier 3, from severity 3 and novelty 2.");
    // Exposure and accumulation move nothing about the line.
    expect(tierLine({ ...answered, exposure: "3", accumulation: "3" })).toBe("Tier 2, from severity 1 and novelty 2.");
  });
});

describe("the one phrase on screen", () => {
  // One of the three, and the one that was chosen: the other two are a hover
  // or a focus away, in the pill's own tooltip.
  it("is the chosen stop's, and its wording is the kit's", () => {
    expect(chosenStop(SCORES[0], "1")).toBe("visible and reversible on one machine");
    expect(chosenStop(SCORES[0], "3")).toBe("irreversible, or it moves authority, secrets or a landing bar");
    expect(chosenStop(SCORES[1], "2")).toBe("new logic inside an existing owner");
    expect(chosenStop(SCORES[2], "3")).toBe("every dispatch or every landing");
    expect(chosenStop(SCORES[3], "2")).toBe("several landings since");
  });

  it("is one of the three the score carries, for every score and every stop", () => {
    for (const score of SCORES) {
      for (const stop of ["1", "2", "3"] as const) {
        expect(score.stops).toContain(chosenStop(score, stop));
      }
    }
  });
});

describe("a tier chosen in the override select", () => {
  // Choosing the derived tier is not an override, so the select goes and what
  // it recorded goes with it, rather than the sheet asking why a human chose
  // the tier it had already chosen for them.
  it("is no override when it is the derived one, by name or by number", () => {
    expect(keepsDerived(answered, "")).toBe(true);
    expect(keepsDerived(answered, "2")).toBe(true);
    expect(keepsDerived({ ...answered, severity: "3" }, "3")).toBe(true);
  });

  it("is an override when it is any other tier", () => {
    expect(keepsDerived(answered, "1")).toBe(false);
    expect(keepsDerived(answered, "3")).toBe(false);
    expect(keepsDerived(emptyRisk, "2")).toBe(false);
  });
});

describe("why a seat cannot open a goal at all", () => {
  it("is the ledger, and only the ledger", () => {
    expect(unopenable("read")).toBe("");
    for (const state of ["absent", "no-ledger", "broken", "unreadable", "refused"]) {
      expect(unopenable(state)).toMatch(/ledger cannot be read/);
    }
  });
});

describe("why the set-priority button is disabled", () => {
  it("takes a band and either a position or nothing at all", () => {
    expect(blockedForRank("2", "5")).toBe("");
    expect(blockedForRank("2", "")).toBe("");
    expect(blockedForRank("2", "  ")).toBe("");
  });

  it("refuses a band that is not one and a position that is not a position", () => {
    expect(blockedForRank("4", "1")).toMatch(/1, 2, or 3/);
    expect(blockedForRank("", "1")).toMatch(/1, 2, or 3/);
    expect(blockedForRank("2", "0")).toMatch(/whole number from 1/);
    expect(blockedForRank("2", "last")).toMatch(/whole number from 1/);
    expect(blockedForRank("2", "-1")).toMatch(/whole number from 1/);
  });
});

describe("oneLine", () => {
  it("folds line breaks into one space and trims, because the ledger keeps the line", () => {
    expect(oneLine("what done\nlooks like\r\n\n  here ")).toBe("what done looks like here");
    expect(oneLine("  no breaks  ")).toBe("no breaks");
  });
});

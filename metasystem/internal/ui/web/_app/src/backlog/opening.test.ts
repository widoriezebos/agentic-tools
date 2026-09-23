import { describe, expect, it } from "vitest";

import {
  blockedForOpen,
  derivedTier,
  emptyIntake,
  emptyRisk,
  goalOf,
  intakeFor,
  labelsOf,
  openNote,
  overridesTier,
  type Intake,
  type Risk,
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
  blocks: "",
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
      blocks: "",
      labels: ["ui", "board"],
      severity: 1,
      novelty: 2,
      exposure: 1,
      accumulation: 3,
      basis: "one seam",
    });
    expect(goalOf({ ...whole, tier: "3", why: "it touches the ledger" }, answered).tier).toBe(3);
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

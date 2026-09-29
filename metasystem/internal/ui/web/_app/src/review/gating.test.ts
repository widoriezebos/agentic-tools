import { describe, expect, it } from "vitest";

import { durationWords, ELIGIBLE_WAITING, gateLine, offersLandWithoutSitting, WAITS_FOR_YOUR_REVIEW } from "./gating";
import type { Gate } from "../backlog/api";

/**
 * The card's four wordings from the project payload (g1-s70 D5, §8): the clock
 * below the tier, eligible and waiting for the holder, held by a sitting, and
 * waiting for your review, with the press offered only on the last.
 */
const now = new Date("2026-09-28T14:50:00Z");

const below: Gate = {
  tier: 1, humanFromTier: 2, autoAfter: "4h", waitsForHuman: false,
  clockFrom: "2026-09-28T14:02:00Z", autoLandsAt: "2026-09-28T18:02:00Z", eligible: false, landed: false,
};

describe("the landing gate's words", () => {
  it("says the clock below the tier, as eligibility", () => {
    expect(gateLine(below, "Wido", now)).toBe("eligible to land in 3h 12m");
  });

  it("says eligible and waiting for the holder once the grace time passed", () => {
    expect(gateLine({ ...below, eligible: true }, "Wido", now)).toBe(ELIGIBLE_WAITING);
    expect(gateLine(below, "Wido", new Date("2026-09-28T18:03:00Z"))).toBe(ELIGIBLE_WAITING);
  });

  it("says whose sitting holds it, at every tier", () => {
    const held = { ...below, heldBy: [{ by: "Wido", record: "plans/reviews/review-of-g.md", since: "2026-09-28T14:30:00Z" }] };
    expect(gateLine(held, "Wido", now)).toBe("held by your sitting");
    expect(gateLine(held, "Ann", now)).toBe("held by Wido's sitting");
    expect(offersLandWithoutSitting({ ...held, waitsForHuman: true })).toBe(false);
  });

  it("says waits for your review at or above the tier, and offers the press", () => {
    const above: Gate = { tier: 2, humanFromTier: 2, autoAfter: "4h", waitsForHuman: true, eligible: false, landed: false };
    expect(gateLine(above, "Wido", now)).toBe(WAITS_FOR_YOUR_REVIEW);
    expect(offersLandWithoutSitting(above)).toBe(true);
    const cleared = { ...above, reviewed: { kind: "clear-to-land", by: "Wido", tip: "9c1f0a2b3c4d5e6f708192a3b4c5d6e7f8091a2b" } };
    expect(gateLine(cleared, "Wido", now)).toBe("cleared to land by Wido at 9c1f0a2, waiting for the holder");
    expect(offersLandWithoutSitting(cleared)).toBe(false);
    const decided = { ...above, reviewed: { kind: "land-without-sitting", by: "Wido", tip: "9c1f0a2b3c4d5e6f708192a3b4c5d6e7f8091a2b" } };
    expect(gateLine(decided, "Wido", now)).toBe("to land without a sitting, by Wido at 9c1f0a2, waiting for the holder");
    expect(offersLandWithoutSitting({ ...above, reviewed: { kind: "send-back", by: "Wido", tip: "x" } })).toBe(true);
    expect(offersLandWithoutSitting(below)).toBe(false);
  });

  it("says nothing without a reading, and the landing once recorded", () => {
    expect(gateLine(undefined, "Wido", now)).toBe("");
    expect(gateLine({ ...below, landed: true }, "Wido", now)).toBe("landed; the goal stays open until it is done");
  });

  it("counts in at most two units", () => {
    expect(durationWords(45 * 60_000)).toBe("45m");
    expect(durationWords(4 * 3_600_000)).toBe("4h");
    expect(durationWords(50 * 3_600_000)).toBe("2d 2h");
    expect(durationWords(10_000)).toBe("1m");
  });
});

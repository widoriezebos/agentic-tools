import { describe, expect, it } from "vitest";

import type { Working } from "./api";
import { phaseWords, roundWords } from "./fleet";

describe("a round and its review limit", () => {
  it.each([
    [1, 20, "round 1"], [17, 20, "round 17"], [18, 20, "round 18 of 20"],
    [20, 20, "round 20 of 20"], [3, null, "round 3"],
  ] as const)("says round %s with limit %s as %s in the helper and rail", (round, roundLimit, words) => {
    const working: Working = {
      goal: "a-goal",
      phase: { role: "building", round, roundLimit },
      job: { id: "j1", role: "building", status: "running", startedAt: null, capMinutes: null, capEndsAt: null },
      box: null,
      chain: [],
    };
    expect(phaseWords(working, new Date("2026-10-04T12:00:00Z"))).toBe(`building ${words} on a-goal · running`);
    expect(roundWords(round, roundLimit)).toBe(words);
  });
});

import { expect, it } from "vitest";

import { expectedStages } from "./facts.mjs";

it("expects no stage of a goal the host board marks Unknown", () => {
  const goals = [
    { goal: "goal-live", stage: "build" },
    { goal: "goal-dead", stage: "review", unknown: "writer dead (pid 77)" },
  ];
  expect([...expectedStages({ seats: [{ goals }] })]).toEqual([["goal-live", "build"]]);
});

it("words a stage as the host board words it", () => {
  const goals = [
    { goal: "a", stage: "review", round: { n: 2, max: 3 } },
    { goal: "b", stage: "unit-proof", proof: { done: 120, planned: 189 } },
    { goal: "c", stage: "claimed-idle" },
  ];
  const worded = ["review round 2 of 3", "unit proof 120 of 189", "claimed idle"];
  expect([...expectedStages({ seats: [{ goals }] }).values()]).toEqual(worded);
});

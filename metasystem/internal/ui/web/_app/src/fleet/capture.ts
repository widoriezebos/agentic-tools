import type { Page } from "./api";
import { copyLine, flagWords, seenWords } from "./fleet";
import type { FleetCapture } from "../partner/api";

/**
 * What a question asked from the Fleet page carries with it.
 *
 * It is the page's own reading and never a later one. A standing is a
 * judgement made against a clock, and the server composing its own a minute
 * afterwards would answer "why is m1c unreachable" about a page nobody was
 * looking at. So the rows travel as they were displayed, and the Partner's
 * `fleet` tool is what it calls when it wants a reading of its own.
 *
 * It is bounded for the board's reason: a capture is not a listing tool, and
 * a fleet of a hundred machines must not become a hundred lines on every
 * question. What was left out is a number the block prints beside what
 * travelled.
 */

/** How many machines travel with one question. */
export const CAPTURED_MACHINES = 24;

/** How many of one machine's holds travel with it. */
export const CAPTURED_HOLDS = 8;

export function captureOfFleet(page: Page, now: Date): FleetCapture {
  return {
    source: page.copy.source,
    fetchedAt: copyLine(page, now),
    problem: page.copy.problem,
    // The flag as the page rendered it, instant and all: the capture is what
    // the human was looking at, not what the server composed.
    needsYou: page.needsYou.slice(0, CAPTURED_MACHINES).map((held) => `${held.goal} is ${flagWords(held)}`),
    machines: page.machines.slice(0, CAPTURED_MACHINES).map((machine) => ({
      machine: machine.machine,
      standing: machine.standing,
      seen: seenWords(machine, now),
      flag: flagWords(machine.holds.find((held) => held.flag !== "") ?? { flag: "", since: "" }),
      holds: machine.holds.slice(0, CAPTURED_HOLDS).map((held) => held.goal),
    })),
    total: page.machines.length,
  };
}

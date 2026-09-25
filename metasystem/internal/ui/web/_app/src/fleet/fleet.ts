import type { Health, Machine, Page, Role, Running, ThisSeat } from "./api";
import { ageBetween, dateAndTime, minuteTime, UNKNOWN } from "../backlog/format";

/**
 * The words the Fleet page says, kept apart from the elements that show them.
 *
 * The page judges nothing: the server composed the standings, the flags and
 * the needs-you selection, and every function here turns what it sent into a
 * sentence. What is decided here is only how an instant reads on a screen —
 * an age against the browser's own clock, in the browser's own time zone —
 * because that is the one thing the server cannot know.
 *
 * Nothing here sets a timer. An age on screen is the age at the moment the
 * page rendered, and the next presence attempt is what re-renders it.
 */

/** How the fleet's provenance line reads, under the table. */
export function copyLine(page: Page, now: Date): string {
  const copy = page.copy;
  const parts: string[] = [];
  if (copy.succeededAt !== "") {
    parts.push(`presence fetched ${ageBetween(copy.succeededAt, now.toISOString())} ago by ${copy.source}`);
  } else if (copy.attemptedAt === "") {
    parts.push(`presence read from ${copy.source}; this server has not fetched yet`);
  } else {
    parts.push(`presence read from ${copy.source}; no fetch of this server's own has succeeded`);
  }
  parts.push(claimsLine(page));
  return parts.join("; ");
}

/** The second half of that line: which reading the holders came from. */
function claimsLine(page: Page): string {
  if (page.claims.unavailable !== "") {
    return `claims unavailable: ${page.claims.unavailable}`;
  }
  if (page.claims.tip === "") {
    return "no accepted ledger this seat could read";
  }
  return `claims from the ledger at ${page.claims.tip.slice(0, 7)}`;
}

/** The copy's own trouble, in the server's words, or "". */
export function copyProblem(page: Page): string {
  return page.copy.problem;
}

/** How a machine's last sighting reads: an age, or why there is none. */
export function seenWords(machine: Machine, now: Date): string {
  if (machine.seen === "") {
    return "no presence";
  }
  return `${ageBetween(machine.seen, now.toISOString())} ago`;
}

/** The instant behind that age, for the row's own title. */
export function seenTitle(machine: Machine): string {
  return machine.seen === "" ? "" : dateAndTime(machine.seen);
}

/**
 * The line beside an unreachable standing: when the silence began, where this
 * seat has a frozen first observation of it. A machine with no such
 * observation says nothing rather than dating the silence from this instant.
 */
export function sinceWords(machine: Machine): string {
  if (machine.standing !== "unreachable" || machine.since === "") {
    return "";
  }
  return `since ${minuteTime(machine.since)}`;
}

/**
 * A flag with its instant rendered into it.
 *
 * The server's words carry no clock, and the instant travels beside them.
 * That is what keeps one moment in time from reading as two different times
 * on one screen: the flag and the seen column are now rendered by the same
 * clock, this browser's, rather than one of them in UTC on the server.
 */
export function flagWords(held: { flag: string; since: string }): string {
  if (held.flag === "" || held.since === "") {
    return held.flag;
  }
  return `${held.flag} since ${minuteTime(held.since)}`;
}

/** What one machine is running, in words. */
export function runningWords(running: Running | null, problem: string): string {
  if (problem !== "") {
    return problem;
  }
  if (running === null) {
    return "idle";
  }
  const parts = [running.role];
  if (running.round > 0) {
    parts.push(`round ${String(running.round)}`);
  }
  if (running.goal !== "") {
    parts.push(`on ${running.goal}`);
  }
  const words = parts.join(" ");
  return running.startedAt === null ? `${words} (not started)` : words;
}

/** The engine stamp a human compares by eye. */
export function shortEngine(engine: string): string {
  return engine.slice(0, 7);
}

/** This seat's nickname, or the sentence a checkout without one gets. */
export function seatName(seat: ThisSeat): string {
  return seat.noNickname ? "This checkout has no machine nickname and publishes no presence." : seat.machine;
}

/** Whether supervision is armed here, in the words the design names. */
export function armedWords(seat: ThisSeat): string {
  switch (seat.armed) {
    case "armed":
      return "supervision is armed here";
    case "stale":
      return "the last health verdict is older than the window this seat judges by";
    case "unreadable":
      return seat.health?.problem === "" ? "the health verdict could not be read" : (seat.health?.problem ?? "");
    default:
      return "supervision is not armed here";
  }
}

/** What this seat last published about itself, or why it has not. */
export function publicationWords(seat: ThisSeat, now: Date): string {
  if (seat.publicationProblem !== "") {
    return seat.publicationProblem;
  }
  const state = seat.publication;
  if (state === null) {
    return "no presence has been published from this checkout";
  }
  if (state.lastOutcome === "published" && state.lastSuccessAt !== "") {
    return `presence published ${ageBetween(state.lastSuccessAt, now.toISOString())} ago on rung ${String(state.rung)}`;
  }
  if (state.lastOutcome === "") {
    return "no publish has been attempted";
  }
  // The steward's own words for what happened, which is what a human acts on.
  return state.lastOutcome;
}

/** When the health verdict was recorded, said as the past observation it is. */
export function healthWords(health: Health | null, now: Date): string {
  if (health === null) {
    return "no health verdict has been recorded on this checkout";
  }
  if (health.problem !== "") {
    return health.problem;
  }
  const when = health.observedAt === "" ? UNKNOWN : `${ageBetween(health.observedAt, now.toISOString())} ago`;
  return `${health.state}, last recorded ${when}`;
}

/** The roles that are not alive, which are the ones worth reading. */
export function rolesNeedingAttention(health: Health | null): Role[] {
  return (health?.roles ?? []).filter((role) => role.status !== "alive");
}

/** The roles that are, collapsed behind a disclosure. */
export function rolesAlive(health: Health | null): Role[] {
  return (health?.roles ?? []).filter((role) => role.status === "alive");
}

/**
 * The sentence the needs-you block ends with.
 *
 * The remedies are named conditionally, because they are not interchangeable:
 * `goal steal` reassigns a claim and refuses a fenced one, and `goal resume`
 * lifts a breach fence and keeps the owner. Neither is an act this page can
 * make, so both are named as the terminal acts they are.
 */
export const NEEDS_YOU_REMEDY =
  "Nothing here acts. At a terminal, goal steal reassigns a claim, and goal resume lifts a breach fence and keeps its owner.";

/** One needs-you line: the goal, and what its holder's standing says. */
export function needsYouLine(held: { goal: string; flag: string }): string {
  return `${held.goal} is ${held.flag}`;
}

/** The empty fleet's own sentence. */
export const NO_PRESENCE =
  "No seat has published presence yet. A seat publishes from its steward tick once it runs this version and is armed.";

/**
 * What a fleet with no rows in it means, which is two different things.
 *
 * A copy that was read and found empty says nobody has published. A copy that
 * could not be read says nothing at all about anybody, and a page that
 * offered the first sentence for the second would be a page stating a fact it
 * has no evidence for.
 */
export function emptyFleetWords(page: Page): string {
  return page.copy.problem === ""
    ? NO_PRESENCE
    : `The presence copy could not be read, so this page can say nothing about any machine: ${page.copy.problem}`;
}

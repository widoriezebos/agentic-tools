import type { Box, ChainMember, Machine, Page, Running, ThisSeat, Working, WorkingJob } from "./api";
import { ageBetween, minuteTime, shortTip } from "../backlog/format";

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

/**
 * Where the presence copy came from and which reading the holders came from:
 * the first half of the verdict's "updated" title, and what a question asked
 * from this page says the page was read from.
 */
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
  return "claims from the accepted ledger";
}

/**
 * Which commit of the ledger the claims were read at, for the verdict's
 * "updated" title: a maintainer compares it, and the page reads without it.
 */
export function claimsTitle(page: Page): string | undefined {
  return page.claims.tip === "" ? undefined : `The accepted ledger at ${shortTip(page.claims.tip)}`;
}

/**
 * Where the reading on this page came from — the presence copy and the
 * ledger the claims were read at — as one sentence, for the title of the
 * verdict's "updated": a maintainer reads it, and the page reads without it.
 */
export function provenance(page: Page, now: Date): string {
  const tip = claimsTitle(page);
  return tip === undefined ? copyLine(page, now) : `${copyLine(page, now)} · ${tip}`;
}

/** The copy's own trouble, in the server's words, or "". */
export function copyProblem(page: Page): string {
  return page.copy.problem;
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

/** One goal's jobs in the opened block, with the goal named once. */
export type WorkingGroup = { goal: string; working: Working[] };

/**
 * A machine's jobs grouped by the goal they are on, each group in the place
 * its newest job held.
 *
 * Only this seat's row carries several, because only this host can read its own
 * job records, and several of them are often one goal's: a round and the
 * critique of that round are two jobs on one goal. Listed flat they repeated
 * the goal and its title once per job, which reads as several goals running at
 * once. The order is the payload's own, newest first, so nothing is reordered
 * by grouping — a group stands where its newest job stood.
 */
export function workingByGoal(working: readonly Working[]): WorkingGroup[] {
  const groups: WorkingGroup[] = [];
  for (const one of working) {
    const found = groups.find((group) => group.goal === one.goal);
    if (found === undefined) {
      groups.push({ goal: one.goal, working: [one] });
      continue;
    }
    found.working.push(one);
  }
  return groups;
}

/** Where a count of minutes stops reading as minutes: two hours. */
const ROLLS_UP_AT = 120;

/**
 * How every duration on this page reads.
 *
 * Minutes up to two hours, and hours and minutes from there: the records carry
 * minutes — a cap is a count of them and so is a box's reservation — and up to
 * two hours the count is the plainest thing to read, while "481 min" is a
 * number a human has to divide before it means anything.
 *
 * Never a day, however many hours it comes to. A budget's day is eight hours
 * in this kit, so a duration printed in days would read as two different
 * lengths depending on who read it. That is why this exists beside
 * `ageBetween`, which rolls up into days for ages nobody is measuring a budget
 * with.
 */
export function minutesWords(minutes: number): string {
  if (minutes < ROLLS_UP_AT) {
    return `${String(minutes)} min`;
  }
  const hours = Math.floor(minutes / 60);
  const rest = minutes % 60;
  return rest === 0 ? `${String(hours)} h` : `${String(hours)} h ${String(rest)} min`;
}

/** Whole minutes between two instants, floored, and never negative. */
function minutesBetween(from: string, to: Date): number {
  const at = Date.parse(from);
  if (Number.isNaN(at)) {
    return 0;
  }
  return Math.max(0, Math.floor((to.getTime() - at) / 60000));
}

/** Whole minutes to an instant, rounded up, which is how a bound rounds. */
function minutesUntil(to: string, from: Date): number | null {
  const at = Date.parse(to);
  if (Number.isNaN(at)) {
    return null;
  }
  return Math.ceil((at - from.getTime()) / 60000);
}

/**
 * The phase sentence the rail says: what the machine is in the middle
 * of, how long that job has run, and the cap it reserved.
 */
export function phaseWords(working: Working, now: Date): string {
  const parts = [working.phase.role];
  if (working.phase.round > 0) {
    parts.push(
      working.phase.roundLimit === null
        ? `round ${String(working.phase.round)}`
        : `round ${String(working.phase.round)} of ${String(working.phase.roundLimit)}`,
    );
  }
  if (working.goal !== "") {
    parts.push(`on ${working.goal}`);
  }
  return `${parts.join(" ")} · ${jobWords(working.job, now)}`;
}

/**
 * A machine's work in phase words: the phase where the payload carries one,
 * and the chain's older words where it does not. A question asked from the
 * page carries it where the page composed no Doing words of its own.
 *
 * Both branches stay, because a fleet is not one build. A machine still
 * publishing the chain alone is answered from the chain rather than reported
 * idle, and this seat's own row can have several things in flight — it names
 * the newest and counts the rest.
 */
export function workingWords(machine: Machine, now: Date): string {
  if (machine.workingProblem !== "") {
    return machine.workingProblem;
  }
  const [first, ...rest] = machine.working;
  if (first === undefined) {
    return runningWords(machine.running, "");
  }
  const words = phaseWords(first, now);
  return rest.length === 0 ? words : `${words} (and ${String(rest.length)} more in flight)`;
}

/**
 * The jobs a machine is working on right now (g1-s74 D5): none unless its
 * standing is reachable, and of its entries only those whose job's status is
 * running. A reservation's start stamp proves nothing — dispatch stamps it on
 * a record it creates pending — and a job with no status is not working.
 * This seat's row carries every local job, so a running job beside a newer
 * reservation counts.
 */
export function runningNow(machine: Machine): Working[] {
  if (machine.standing !== "reachable") {
    return [];
  }
  return machine.working.filter((one) => one.job.status === RUNNING);
}

/**
 * What the rail's Fleet row says while any machine is working: one line per
 * running job, "m1e: implementer round 2 on verbs-match-intent · running 41
 * min", in the phase words of the entry that lit the dot and never of the
 * machine's first entry, which may be a reservation. The minutes are the
 * Fleet page's own, from presence; nothing here keeps a clock. No page, or a
 * read that failed, says nothing.
 */
export function workingLines(page: Page | null, now: Date): string[] {
  if (page === null) {
    return [];
  }
  return page.machines.flatMap((machine) =>
    runningNow(machine).map((one) => `${machine.machine}: ${phaseWords(one, now)}`),
  );
}

/**
 * The three statuses a job still in flight can be in, and the one of them in
 * which it is actually being worked.
 *
 * The other two are reservations. The engine stamps a start on a record it
 * creates pending, so the status and not the stamp is what says whether
 * anything is running — and a minute count taken from that stamp would say a
 * job had been running for forty minutes while the line beside it said
 * pending.
 */
const IN_FLIGHT = new Set(["pending-setup", "pending", "running"]);
const RUNNING = "running";

/**
 * Where the job in hand stands and the cap it reserved. The status decides:
 * a reservation says what it is, a job that has ended says how it ended, and
 * only a running job is counted in minutes.
 */
export function jobWords(job: WorkingJob, now: Date): string {
  if (job.status === "") {
    return "not started";
  }
  const cap = job.capMinutes === null ? "" : `, cap ${minutesWords(job.capMinutes)}`;
  if (!IN_FLIGHT.has(job.status)) {
    // A job that has ended is what its status says. The cap it reserved is a
    // bound on work that is over, and the chain carries it.
    return job.status;
  }
  if (job.status !== RUNNING) {
    return `${job.status}${cap}`;
  }
  if (job.startedAt === null) {
    return `${RUNNING}${cap}`;
  }
  return `running ${minutesWords(minutesBetween(job.startedAt, now))}${cap}`;
}

/**
 * The one forward-looking sentence on this page, and it names the CAP rather
 * than completion: the kit measures and bounds and never forecasts. It is
 * empty where no deadline can be named.
 */
export function capWords(job: WorkingJob, now: Date): string {
  if (job.capEndsAt === null) {
    return "";
  }
  const remaining = minutesUntil(job.capEndsAt, now);
  if (remaining === null) {
    return "";
  }
  return remaining > 0
    ? `its cap ends in ${minutesWords(remaining)}`
    : `its cap ended ${minutesWords(-remaining)} ago`;
}

/** The box's attempts in words, or "" where it carries no attempt count. */
export function attemptWords(box: Box): string {
  if (box.attempts === null || box.attemptLimit === null) {
    return "";
  }
  return `attempt ${String(box.attempts)} of ${String(box.attemptLimit)}`;
}

/** What is left of the box's attempts, which is the second thing it bounds. */
export function attemptsLeftWords(box: Box): string {
  if (box.attempts === null || box.attemptLimit === null) {
    return "";
  }
  return `${String(Math.max(0, box.attemptLimit - box.attempts))} attempts left`;
}

/** The box's reserved minutes in words, or "" where it carries none. */
export function reservedWords(box: Box): string {
  if (box.reservedMinutes === null || box.reservedMinutesLimit === null) {
    return "";
  }
  // Both sides read the same way: one number rolled up into hours beside one
  // that was not would be two units in one phrase.
  return `${minutesWords(box.reservedMinutes)} of ${minutesWords(box.reservedMinutesLimit)} reserved`;
}

/**
 * How full a bar is, as a share between nothing and full.
 *
 * It clamps at one end only: a spend past its limit draws a full bar, because
 * a bar longer than its track says nothing a number beside it does not
 * already say, and the number is what a human acts on.
 */
export function barShare(used: number, limit: number): number {
  if (limit <= 0) {
    return 0;
  }
  return Math.min(1, Math.max(0, used / limit));
}

/** What a reserved minute counts, said wherever the number is. */
export const RESERVED_MEANING = "Reserved minutes count open jobs at their full cap.";

/** The sentence a goal with no box of its own gets. */
export const NO_BOX = "no box on this goal";

/** One member of the chain in words: what it was and how it stands. */
export function chainWords(member: ChainMember): string {
  const parts = [member.role];
  if (member.round > 0) {
    parts.push(`round ${String(member.round)}`);
  }
  return parts.join(" ");
}

/** When a chain member ran, in this browser's own clock. */
export function chainWhen(member: ChainMember): string {
  const parts: string[] = [];
  if (member.startedAt !== null) {
    parts.push(`started ${minuteTime(member.startedAt)}`);
  }
  if (member.endedAt !== null) {
    parts.push(`ended ${minuteTime(member.endedAt)}`);
  }
  if (member.capMinutes !== null) {
    parts.push(`cap ${minutesWords(member.capMinutes)}`);
  }
  return parts.join(" · ");
}

/**
 * Where the block a row opens to came from.
 *
 * This seat reads its own job records, so its block is as current as the
 * page. Every other machine's is what its last tick published, and saying so
 * is the difference between a reading and a reading of a reading.
 */
export function workingSource(machine: Machine, now: Date): string {
  if (machine.this) {
    return "read from this host's own job records";
  }
  if (machine.seen === "") {
    return "as published, at an instant this seat does not have";
  }
  return `as published ${ageBetween(machine.seen, now.toISOString())} ago`;
}

/** A checkout with no machine nickname has no row on the fleet; this says why. */
export const NO_NICKNAME = "This checkout has no machine nickname and publishes no presence.";

/**
 * What this seat last published about itself, for its row's opening: when,
 * or why it has not. The rung it published on is the steward's business.
 */
export function publishedWords(seat: ThisSeat, now: Date): string {
  if (seat.publicationProblem !== "") {
    return seat.publicationProblem;
  }
  const state = seat.publication;
  if (state === null) {
    return "nothing published from this checkout yet";
  }
  if (state.lastOutcome === "published" && state.lastSuccessAt !== "") {
    return `published ${ageBetween(state.lastSuccessAt, now.toISOString())} ago`;
  }
  if (state.lastOutcome === "") {
    return "no publish has been attempted";
  }
  // The steward's own words for what happened, which is what a human acts on.
  return state.lastOutcome;
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

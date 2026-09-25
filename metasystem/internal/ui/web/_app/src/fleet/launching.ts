import type { Launch, LaunchStep, Launching, Machine } from "./api";
import { ageBetween, dateAndTime } from "../backlog/format";

/**
 * What the launch sheet proposes and refuses, and what the launch card says.
 *
 * Nothing here decides whether a launch may happen: the verb holds the host
 * lock, judges the nickname, the destination and the pair, and refuses by
 * name. What is here is what a form owes the human in front of it — a
 * proposal they can accept, and a "no" before they press a button rather than
 * after — and it is written against the same rules the engine judges by, so
 * the sheet and the verb never disagree about what a nickname is.
 *
 * Nothing here sets a timer either. A launch takes minutes and rewrites its
 * record after every step; the server announces the fleet event when it does,
 * and that is what re-renders this.
 */

/** The nickname rule, exactly as seat.ValidateMachineName writes it. */
const NICKNAME = /^[A-Za-z0-9._-]+$/;

/**
 * Why this nickname cannot be the new machine's, or "" when it can.
 *
 * Both refusals are the engine's own, said here before the act is sent:
 * a name the presence publisher would refuse, and a name the fleet already
 * carries. An untouched field says nothing — it is not a mistake, and the
 * foot already names it as what the act is waiting on.
 */
export function nicknameRefusal(name: string, taken: readonly string[], thisSeat: string): string {
  const wanted = name.trim();
  if (wanted === "") {
    return "";
  }
  if (!NICKNAME.test(wanted) || wanted === "." || wanted.includes("..")) {
    return "A nickname is one word of letters, digits, dot, underscore and hyphen: it is a git ref segment and a file name, so it cannot carry a slash.";
  }
  if (wanted === thisSeat) {
    return `${wanted} is this checkout's own nickname; two machines publishing under one name publish over each other.`;
  }
  if (taken.includes(wanted)) {
    return `${wanted} is already a machine of this fleet.`;
  }
  return "";
}

/**
 * The nickname proposed for a new machine: the first free letter of this
 * host's series.
 *
 * The series is this seat's own name without its last letter, because that is
 * what makes m1b, m1c, m1d and m1e one host's machines. The scan starts at b
 * rather than a: a is where a series begins and is almost never free, and a
 * proposal nobody can accept is worse than none. A seat with no nickname, or
 * a series with no letter left, proposes nothing and the field is empty.
 */
export function proposedNickname(thisSeat: string, taken: readonly string[]): string {
  const series = /^(.*)[A-Za-z]$/.exec(thisSeat);
  if (series === null || series[1] === "") {
    return "";
  }
  for (let code = "b".charCodeAt(0); code <= "z".charCodeAt(0); code++) {
    const candidate = series[1] + String.fromCharCode(code);
    if (!taken.includes(candidate)) {
      return candidate;
    }
  }
  return "";
}

/** Every nickname this page already knows about, machines and launches both. */
export function nicknamesTaken(machines: readonly Machine[], launches: readonly Launch[]): string[] {
  const names = machines.map((machine) => machine.machine);
  for (const launched of launches) {
    if (launched.machine !== "" && !names.includes(launched.machine)) {
      names.push(launched.machine);
    }
  }
  return names;
}

/**
 * Where a machine of this nickname lands by default: beside this checkout,
 * named for the ledger remote's own repository.
 *
 * The remote's name and never a fixed word, so another project launching a
 * machine gets its own name. A seat whose layout or remote could not be read
 * proposes nothing, and the sheet asks for the path instead.
 */
export function proposedDestination(where: Launching, nickname: string): string {
  const name = nickname.trim();
  if (where.parent === "" || where.repository === "" || name === "") {
    return "";
  }
  return `${where.parent}/${where.repository}-${name}`;
}

/** Why this destination cannot be used, or "" when it can. */
export function destinationRefusal(path: string): string {
  const wanted = path.trim();
  if (wanted === "") {
    return "";
  }
  if (!wanted.startsWith("/")) {
    return "A machine lands at an absolute path on this host.";
  }
  return "";
}

/** How many days out the review date is proposed. */
export const REVIEW_DAYS = 7;

/** The review date a sheet opens on: a week from today, in the local day. */
export function proposedReviewBy(now: Date): string {
  const then = new Date(now.getTime());
  then.setDate(then.getDate() + REVIEW_DAYS);
  return isoDay(then);
}

/** Today, as the field's own minimum. */
export function earliestReviewBy(now: Date): string {
  return isoDay(now);
}

function isoDay(day: Date): string {
  const month = String(day.getMonth() + 1).padStart(2, "0");
  const date = String(day.getDate()).padStart(2, "0");
  return `${String(day.getFullYear())}-${month}-${date}`;
}

/**
 * Why this review date cannot be the machine's, or "" when it can.
 *
 * A date in the past is this side's refusal and not the engine's: the
 * validator the arming verb runs accepts one, and nothing ever compares the
 * date to a clock afterwards. A review already overdue the moment a machine
 * joins is not what choosing a date means.
 */
export function reviewByRefusal(date: string, now: Date): string {
  const wanted = date.trim();
  if (wanted === "") {
    return "";
  }
  if (!/^\d{4}-\d{2}-\d{2}$/.test(wanted)) {
    return "A review date is a day, as YYYY-MM-DD.";
  }
  if (wanted < earliestReviewBy(now)) {
    return "A date in the past is a review already overdue the moment the machine joins.";
  }
  return "";
}

/** A launch as the sheet has it filled in. */
export type LaunchDraft = {
  machine: string;
  destination: string;
  word: string;
  reviewBy: string;
};

/**
 * Why the sheet's own button is disabled, or "" when it is not.
 *
 * Nothing about proof is here: a browser nobody is signed into is still
 * offered the act, and the route answers such a press with the sign-in this
 * page can open. What is left is the human's own to fill in, and each reason
 * names the field rather than saying that something is missing.
 */
export function blockedForLaunch(
  draft: LaunchDraft,
  taken: readonly string[],
  thisSeat: string,
  now: Date,
): string {
  if (draft.machine.trim() === "") {
    return "A machine is named by one nickname, which is how every other seat will refer to it.";
  }
  const nickname = nicknameRefusal(draft.machine, taken, thisSeat);
  if (nickname !== "") {
    return nickname;
  }
  if (draft.destination.trim() === "") {
    return "A machine is a clone at a path on this host.";
  }
  const destination = destinationRefusal(draft.destination);
  if (destination !== "") {
    return destination;
  }
  if (draft.word.trim() === "") {
    return "The machine is enrolled under your own words, which are recorded on its identity.";
  }
  const review = reviewByRefusal(draft.reviewBy, now);
  if (review !== "") {
    return review;
  }
  if (draft.reviewBy.trim() === "") {
    return "The enrollment carries a review date, which is when a human re-approves it at a terminal.";
  }
  return "";
}

/** What the sheet says a machine gets, in the design's own two sentences. */
export const WHAT_IT_GETS =
  "It gets this seat's roster and local configuration, its runtime settings, and the fleet's ledger. The roster is copied from a running seat, so it is complete; edit the copy afterwards if this machine should differ.";
export const WHAT_IT_WILL_NOT_DO =
  "It starts no session; it joins, publishes presence and waits.";

/** What a sessionless machine will do, which is not nothing. */
export const IDLE_WARNING =
  "A machine with no session raises an idle alert whenever claimable work exists, and raises it again after every delivery. Start a session by hand, or stop the machine.";

/** What the temporary enrollment is, said where the word is typed. */
export const TEMPORARY_RULE =
  "The machine carries a temporary enrollment with a review due on this date. The date stops nothing: a human re-approves it, or stops the machine, at a terminal.";

/* --------------------------------------------------------------- the card -- */

/** The card's headline: what is being made, and how it is going. */
export function launchHeadline(record: Launch): string {
  switch (record.outcome) {
    case "running":
      return `Launching ${record.machine}`;
    case "done":
      return `${record.machine} joined`;
    case "armed":
      return `${record.machine} armed; presence not yet seen`;
    default:
      return `Launching ${record.machine} stopped`;
  }
}

/** The step a failed launch stopped at, or null. */
export function failedStep(record: Launch): LaunchStep | null {
  return record.steps.find((step) => step.outcome === "failed") ?? null;
}

/** Whether this launch is one the card shows at all. */
export function showsCard(record: Launch): boolean {
  return record.outcome === "running" || record.outcome === "failed" || record.outcome === "armed";
}

/** The newest launch worth a card, or null when there is none. */
export function cardFor(launches: readonly Launch[]): Launch | null {
  return launches.find(showsCard) ?? null;
}

/**
 * The one line a finished launch folds to, once its machine is in the table.
 *
 * It is the design's own sentence: the machine, when it joined, and what its
 * enrollment is. The date is rendered in the reader's own zone, like every
 * other instant on this page.
 */
export function foldedLine(record: Launch, now: Date): string {
  const joined = record.endedAt === null ? "just now" : `${ageBetween(record.endedAt, now.toISOString())} ago`;
  const review = record.reviewBy === "" ? "" : `; temporary enrollment, review due ${reviewDay(record.reviewBy)}`;
  return `${record.machine} joined ${joined}${review}`;
}

/** A review date as a human reads it: the day, in words. */
export function reviewDay(date: string): string {
  const parsed = new Date(`${date}T00:00:00`);
  if (Number.isNaN(parsed.getTime())) {
    return date;
  }
  return parsed.toLocaleDateString(undefined, { day: "numeric", month: "long" });
}

/** When a step happened, for the row's own title. */
export function stepTitle(step: LaunchStep): string {
  return step.at === "" ? "" : dateAndTime(step.at);
}

/**
 * Whether a retry of this launch will have to ask for the word again.
 *
 * It will, unless the machine is already enrolled: the record never held the
 * word, by design, so a resume that reaches the arming step has nothing to
 * arm with and the verb refuses. A launch whose enrollment step is done or
 * skipped will not reach it, and the card asks for nothing.
 */
export function retryAsksForTheWord(record: Launch): boolean {
  const enrolled = record.steps.find((step) => step.step === "enrollment");
  return enrolled === undefined || (enrolled.outcome !== "done" && enrolled.outcome !== "skipped");
}

/** What the card says a human does with a failed clone, which is delete it. */
export function failedRemedy(record: Launch): string {
  return `Nothing here removes it: ${record.destination} is a directory you delete.`;
}

/** How the health of a machine that armed but has not published is read. */
export function healthCommand(record: Launch): string {
  return `metasystem steward health --repo ${record.destination}/metasystem`;
}

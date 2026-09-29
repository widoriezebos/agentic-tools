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

/**
 * Every nickname this page already knows about, machines and launches both.
 *
 * A discarded launch holds no nickname: the human put it away, and the engine,
 * which never reads launch records, judges the name by presence, claims and
 * the folder on disk (Wido, 2026-09-29).
 */
export function nicknamesTaken(machines: readonly Machine[], launches: readonly Launch[]): string[] {
  const names = machines.map((machine) => machine.machine);
  for (const launched of launches) {
    if (launched.discardedAt === null && launched.machine !== "" && !names.includes(launched.machine)) {
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

/** A launch as the sheet has it filled in. */
export type LaunchDraft = {
  machine: string;
  destination: string;
};

/**
 * Why the sheet's own button is disabled, or "" when it is not.
 *
 * Nothing about proof is here: a browser nobody is signed into is still
 * offered the act, and the route answers such a press with the sign-in this
 * page can open. The signed-in session is the human's own enrollment (g1-s72),
 * so what is left is the nickname and the path, and each reason names the
 * field rather than saying that something is missing.
 */
export function blockedForLaunch(draft: LaunchDraft, taken: readonly string[], thisSeat: string): string {
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

/* --------------------------------------------------------------- the card -- */

/** The card's headline: what is being made, and how it is going. */
export function launchHeadline(record: Launch): string {
  switch (record.outcome) {
    case "starting":
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

/**
 * Whether this launch is one the card shows at all.
 *
 * A launch a human discarded is not: its record is kept for the trail, and
 * the page has been told to put it away.
 *
 * `starting` is a launch the server has written down and whose verb has not
 * yet named its own process. It is on screen from that moment, because the
 * act answers with it and a human who pressed Launch is owed a card.
 */
export function showsCard(record: Launch): boolean {
  if (record.discardedAt !== null) {
    return false;
  }
  return (
    record.outcome === "starting" ||
    record.outcome === "running" ||
    record.outcome === "failed" ||
    record.outcome === "armed"
  );
}

/** The newest launch worth a card, or null when there is none. */
export function cardFor(launches: readonly Launch[]): Launch | null {
  return launches.find(showsCard) ?? null;
}

/**
 * The card the fleet block shows: the launch started from this page, as the
 * server's reading now has it, or else the newest launch still worth a card.
 *
 * `hidden` is the launches whose discard this page has had answered. The
 * server's next reading carries the mark and says the same; until it arrives
 * the card is already gone, and the next launch worth one takes its place
 * exactly as it will once the reading lands.
 */
export function visibleCard(
  launches: readonly Launch[],
  started: Launch | null,
  hidden: ReadonlySet<string>,
): Launch | null {
  const gone = (record: Launch) => record.discardedAt !== null || hidden.has(record.launch);
  const own = started === null ? null : (launches.find((one) => one.launch === started.launch) ?? started);
  if (own !== null && !gone(own)) {
    return own;
  }
  return launches.find((record) => showsCard(record) && !hidden.has(record.launch)) ?? null;
}

/**
 * The one line a finished launch folds to, once its machine is in the table.
 *
 * The machine and when it joined. A machine launched from this page is its
 * human's own, enrolled by the signed-in session (g1-s72), so there is no
 * review to name; how it was enrolled is its health's to say, not this line's.
 */
export function foldedLine(record: Launch, now: Date): string {
  const joined = record.endedAt === null ? "just now" : `${ageBetween(record.endedAt, now.toISOString())} ago`;
  return `${record.machine} joined ${joined}`;
}

/** When a step happened, for the row's own title. */
export function stepTitle(step: LaunchStep): string {
  return step.at === "" ? "" : dateAndTime(step.at);
}

/**
 * What a stopped launch left on this host, while it is still there.
 *
 * Discarding the card deletes nothing, so the clone is named for as long as
 * the server finds it on disk, and not a moment after. No verb removes a
 * machine's clone yet, so the line names none.
 */
export function leftoverLine(record: Launch): string {
  if (!record.destinationPresent) {
    return "";
  }
  return `The clone at ${record.destination} stays on disk; delete it yourself.`;
}

/** How the health of a machine that armed but has not published is read. */
export function healthCommand(record: Launch): string {
  return `metasystem system check --repo ${record.destination}/metasystem`;
}

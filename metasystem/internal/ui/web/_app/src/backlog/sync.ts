import type { Ledger } from "./api";
import { clockTime, dateAndTime, minuteTime, shortTip } from "./format";

/**
 * Whether this page is reading a current ledger, said in as few words as
 * being current deserves.
 *
 * The whole report — which tip, when the project last changed, when this page
 * read it, what the server's fetch loop last found, when it looks again — used
 * to be two lines above the board, ahead of the work and read every time by a
 * human who had no question about it. It is one chip now, and the report is
 * its tooltip: a human who wants the detail asks for it, and nobody reads it
 * by accident.
 *
 * What the chip judges is the server's fetch loop and nothing else. The
 * accepted commit's age used to raise the chip's voice, and that was a
 * warning nobody could clear: Refresh asks the server to look again, and no
 * amount of looking makes the last commit younger, so the board told a human
 * something was wrong and then refused to stop telling them. A repository
 * nobody has committed to since breakfast is a quiet repository. The commit's
 * time is still in the report, as "last change", where it is a fact rather
 * than an alarm.
 *
 * So the chip raises its voice twice and no other time: a loop that has not
 * heard from the canonical branch lately, and something actually wrong — a
 * fetch that failed, or a ledger that does not project. Only the second puts
 * a line on the page, because only the second is something a human can do
 * anything about.
 *
 * Nothing here counts. Every time is the response's own, and the page sets no
 * timer, so a chip that says 10:30 says what the last read found rather than
 * what a clock has since made of it.
 */

export type SyncState = "rest" | "behind" | "wrong";

export type Sync = {
  state: SyncState;
  /** The chip's own words, short enough to read without stopping. */
  line: string;
  /** The whole report, for the chip's tooltip. */
  report: string;
  /** What is wrong and what to do, or "" where nothing is. */
  wrong: string;
};

/** What the chip says when the page has not read a ledger at all yet. */
const UNREAD = "Not read";

export function syncOf(ledger: Ledger | null, observedAt: string): Sync {
  if (ledger === null) {
    return {
      state: "wrong",
      line: UNREAD,
      report: "This page has not read the accepted ledger yet.",
      wrong: "This page has not read the accepted ledger yet. Refresh asks the server for it.",
    };
  }
  const report = reportOf(ledger, observedAt);
  if (ledger.state !== "read") {
    return { state: "wrong", line: brokenLine(ledger), report, wrong: brokenReason(ledger) };
  }
  const { state, since } = ledger.freshness;
  if (state === "failed") {
    return { state: "wrong", line: `Fetch failed ${minuteTime(since)}`, report, wrong: fetchFailure(ledger) };
  }
  if (state === "behind") {
    return { state: "behind", line: behindLine(ledger), report, wrong: "" };
  }
  return { state: "rest", line: `Synced ${minuteTime(observedAt)} · ${shortTip(ledger.tip)}`, report, wrong: "" };
}

/**
 * The report the chip carries: every fact the two lines above the board
 * carried, in the order they carried them, so that asking for the detail
 * gives the same answer reading it always did — and, at the end, the one
 * sentence saying how current the server believes it is.
 */
export function reportOf(ledger: Ledger, observedAt: string): string {
  const tip =
    ledger.tip === ""
      ? "No accepted tip in this clone"
      : `Accepted tip ${shortTip(ledger.tip)}, last change ${dateAndTime(ledger.committedAt)}`;
  const read = `${tip} · observed ${clockTime(observedAt)} · ${fetchClause(ledger)}.`;
  const freshness = freshnessLine(ledger);
  return freshness === "" ? read : `${read} ${freshness}`;
}

/** What the loop last did, and when it looks again. */
export function fetchClause(ledger: Ledger): string {
  const loop = ledger.fetch;
  const due = loop.nextAt === "" ? "server stopping" : `next fetch ${clockTime(loop.nextAt)}`;
  switch (loop.outcome) {
    case "never":
      return loop.nextAt === "" ? "server stopping" : `first fetch due ${clockTime(loop.nextAt)}`;
    case "running":
      return `fetching since ${clockTime(loop.startedAt)}`;
    case "advanced":
      return `fetched ${clockTime(loop.finishedAt)}, accepted ${shortTip(loop.tip)} · ${due}`;
    case "current":
      return `fetched ${clockTime(loop.finishedAt)}, ${loop.detail} · ${due}`;
    case "failed":
      return `fetch failed ${clockTime(loop.finishedAt)}: ${loop.message} · ${
        loop.nextAt === "" ? "server stopping" : `retry ${clockTime(loop.nextAt)}`
      }`;
  }
}

/**
 * How current the server judged itself to be, in the same terms the chip uses
 * and with the server's own reason after them. A ledger that is current says
 * nothing here: the fetch clause above has already said when it last looked
 * and what it found.
 */
export function freshnessLine(ledger: Ledger): string {
  const { state, since, detail } = ledger.freshness;
  switch (state) {
    case "current":
      return "";
    case "behind":
      return since === "" ? `Behind: ${detail}.` : `Behind since ${clockTime(since)}: ${detail}.`;
    case "failed":
      return `Fetch failed ${clockTime(since)}: ${detail}.`;
  }
}

/**
 * The chip's words for a loop that has not heard from the canonical branch
 * lately. A clone that has never fetched has no instant to be behind since,
 * and naming one it does not have would be worse than naming none.
 */
function behindLine(ledger: Ledger): string {
  const since = ledger.freshness.since;
  return since === "" ? "Behind" : `Behind since ${minuteTime(since)}`;
}

/** A fetch that failed, and what a human can do about it from here. */
function fetchFailure(ledger: Ledger): string {
  const loop = ledger.fetch;
  const again =
    loop.nextAt === ""
      ? "The server is stopping, so nothing will fetch again"
      : `The server tries again at ${clockTime(loop.nextAt)}`;
  return `The last fetch failed at ${clockTime(ledger.freshness.since)}: ${ledger.freshness.detail}. ${again}; Refresh looks again now.`;
}

/** The chip's word for a ledger that did not project. */
function brokenLine(ledger: Ledger): string {
  switch (ledger.state) {
    case "absent":
      return "No accepted tip";
    case "no-ledger":
      return "No ledger at the tip";
    case "unreadable":
      return "Ledger not valid";
    case "refused":
      return "Ledger refused";
    default:
      return "Ledger unreadable";
  }
}

/** Why it did not, in the one line that stands under the toolbar. */
function brokenReason(ledger: Ledger): string {
  switch (ledger.state) {
    case "absent":
      return "This clone has no accepted tip yet; the server's fetch loop creates one once the canonical branch validates.";
    case "no-ledger":
      return `The accepted tip ${shortTip(ledger.tip)} carries no backlog, so nothing here can be read from it.`;
    case "unreadable":
      return `The ledger at ${shortTip(ledger.tip)} does not validate, so the engine refuses the whole of it.`;
    default:
      return ledger.message === "" ? "The accepted ledger could not be read." : `${ledger.message}.`;
  }
}

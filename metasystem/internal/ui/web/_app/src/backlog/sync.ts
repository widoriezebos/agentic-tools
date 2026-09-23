import type { Ledger } from "./api";
import { ageBetween, clockTime, dateAndTime, minuteTime, shortTip } from "./format";

/**
 * Whether this page is reading a current ledger, said in as few words as
 * being current deserves.
 *
 * The whole report — which tip, when it was committed, when this page read
 * it, what the server's fetch loop last found, when it looks again, and how
 * old the tip is — used to be two lines above the board, ahead of the work
 * and read every time by a human who had no question about it. It is one
 * chip now, and the report is its tooltip: a human who wants the detail asks
 * for it, and nobody reads it by accident.
 *
 * Status that is fine is quiet. The chip raises its voice twice and no other
 * time: an accepted tip the server already calls old, and something actually
 * wrong — a fetch that failed, or a ledger that does not project. Only the
 * second puts a line on the page, because only the second is something a
 * human can do anything about.
 *
 * Nothing here counts. Every time is the response's own, and the page sets no
 * timer, so a chip that says 10:30 says what the last read found rather than
 * what a clock has since made of it.
 */

export type SyncState = "rest" | "stale" | "wrong";

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
  if (ledger.fetch.outcome === "failed") {
    return { state: "wrong", line: `Sync failed ${minuteTime(ledger.fetch.finishedAt)}`, report, wrong: fetchFailure(ledger) };
  }
  if (ledger.stale) {
    return {
      state: "stale",
      line: `Synced ${minuteTime(observedAt)} · ${ageBetween(ledger.committedAt, observedAt)} behind`,
      report,
      wrong: "",
    };
  }
  return { state: "rest", line: `Synced ${minuteTime(observedAt)} · ${shortTip(ledger.tip)}`, report, wrong: "" };
}

/**
 * The report the chip carries: every fact the two lines above the board
 * carried, in the order they carried them, so that asking for the detail
 * gives the same answer reading it always did.
 */
export function reportOf(ledger: Ledger, observedAt: string): string {
  const tip =
    ledger.tip === ""
      ? "No accepted tip in this clone"
      : `Accepted tip ${shortTip(ledger.tip)}, committed ${dateAndTime(ledger.committedAt)}`;
  const read = `${tip} · observed ${clockTime(observedAt)} · ${fetchClause(ledger)}.`;
  return ledger.stale ? `${read} ${staleLine(ledger, observedAt)}` : read;
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
 * Why an old tip is old. A tree that has not moved is not by itself a
 * problem; what the human needs is what the last look at the canonical branch
 * found, which is the difference between a quiet repository and a broken one.
 */
export function staleLine(ledger: Ledger, observedAt: string): string {
  const age = `The accepted tip is ${ageBetween(ledger.committedAt, observedAt)} old`;
  switch (ledger.fetch.outcome) {
    case "current":
    case "advanced":
      return `${age}; the last fetch found the canonical branch at this tip.`;
    case "failed":
      return `${age}; the last fetch failed: ${ledger.fetch.message}.`;
    default:
      return `${age}; no fetch has completed yet.`;
  }
}

/** A fetch that failed, and what a human can do about it from here. */
function fetchFailure(ledger: Ledger): string {
  const loop = ledger.fetch;
  const again =
    loop.nextAt === ""
      ? "The server is stopping, so nothing will fetch again"
      : `The server tries again at ${clockTime(loop.nextAt)}`;
  return `The last fetch failed at ${clockTime(loop.finishedAt)}: ${loop.message}. ${again}; Refresh asks for the accepted tip as it stands now.`;
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

import type { Row } from "./api";
import { shortTip } from "./format";
import { clockOf } from "../partner/capture";
import type { Chosen } from "../partner/subject";
import { goalPath } from "../routes";

/**
 * The board's things, as subjects.
 *
 * A subject carries its own summary rather than a way to look one up: the
 * pinned panel has to agree with what the question carried, so both are
 * composed from the same row at the same moment. A board that has moved since
 * cannot make them disagree.
 */

/** The reading a board subject was taken from, in the words the chip uses. */
export function readingOf(tip: string, observedAt: string): string {
  if (tip === "") {
    return "the accepted tip this page rendered from";
  }
  const at = clockOf(observedAt);
  return at === "" ? `the accepted tip ${shortTip(tip)}` : `the accepted tip ${shortTip(tip)}, ${at}`;
}

/**
 * One goal, as the card and the row show it.
 *
 * `to` opens the goal's own page, which is what the pinned panel links; `at`
 * is the board that was on screen with this goal named, which is where a
 * message chip goes back to.
 */
export function goalSubject(row: Row, tip: string, observedAt: string, at?: string): Chosen {
  return {
    kind: "goal",
    id: row.ref.id,
    title: row.ref.id,
    source: readingOf(tip, observedAt),
    summary: summaryOf(row),
    to: goalPath(row.ref.id),
    at,
  };
}

/** One lane, as its column shows it: what it holds, and the first of them. */
export function laneSubject(
  title: string,
  rows: readonly Row[],
  tip: string,
  observedAt: string,
  to: string,
): Chosen {
  const named = rows.slice(0, 25).map((row) => row.ref.id);
  const rest = rows.length - named.length;
  return {
    kind: "lane",
    id: title,
    title,
    source: readingOf(tip, observedAt),
    summary:
      rows.length === 0
        ? "Nothing is in this lane."
        : `${String(rows.length)} goal${rows.length === 1 ? "" : "s"}: ${named.join(", ")}${rest > 0 ? `, and ${String(rest)} more` : ""}`,
    to,
    at: to,
  };
}

/** A goal's own line, as the board's card reads it. */
function summaryOf(row: Row): string {
  const parts = [row.intent];
  if (row.state !== "") {
    parts.push(row.state);
  }
  if (row.tier > 0) {
    parts.push(`tier ${String(row.tier)}`);
  }
  if (row.priority > 0 && row.sequence > 0) {
    parts.push(`${String(row.priority)}:${String(row.sequence)}`);
  }
  if (row.claim !== undefined) {
    parts.push(`seat ${row.claim.machine}`);
  }
  return parts.join(" · ");
}

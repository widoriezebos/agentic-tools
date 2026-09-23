import { NavLink } from "react-router";

import type { Row } from "./api";
import { dateAndTime } from "./format";
import { SHOWN } from "./showing";
import { goalPath } from "../routes";
import { Chip } from "../shell/controls";

/**
 * One goal, as the record has it.
 *
 * Nothing is truncated and nothing is dropped: an intent or a next step is
 * what the human wrote, and a row that cut it off would hide the one sentence
 * that says what the work is. But a record here runs to several screens, and
 * the master's List view is for scanning and comparing, so the intent is
 * clamped to three lines until the row is opened. The clamp is CSS over the
 * whole text rather than a shortened string, so find-in-page, selection and a
 * screen reader still reach every word, and opening the row releases it. The
 * next step is the record's own continuation and appears with it.
 *
 * The identifier is the one link: it opens the goal's own page, which is where
 * the records about this goal are read. Nothing else here navigates and
 * nothing selects.
 *
 * A row the page was sent to show wears the ring for two seconds, the same
 * mark the board's card wears, so that arriving from a link lands on the goal
 * rather than at the top of a list of four hundred.
 */
export function GoalRow({ row, shown, onFaded }: { row: Row; shown: boolean; onFaded: () => void }) {
  const reasons = reasonsOf(row);
  return (
    <article
      className={shown ? `ms-goal-row ${SHOWN}` : "ms-goal-row"}
      onAnimationEnd={shown ? onFaded : undefined}
    >
      <div className="ms-goal-head">
        <NavLink className="ms-goal-id ms-mono" to={goalPath(row.ref.id)}>
          {row.ref.id}
        </NavLink>
        <Chip>{row.state}</Chip>
        {row.tier > 0 && <Chip>tier {row.tier}</Chip>}
        {rankChip(row)}
        {row.pinned !== "" && <Chip marker>pinned to {row.pinned}</Chip>}
        {row.labels.map((label) => (
          <Chip key={label}>{label}</Chip>
        ))}
        {row.arc !== "" && <Chip>arc {row.arc}</Chip>}
        {row.sliced && <Chip>sliced</Chip>}
        {row.decomposed && <Chip>split into goals</Chip>}
        <Chip>rev {row.ref.revision}</Chip>
      </div>
      <details className="ms-goal-record">
        <summary className="ms-goal-summary">
          <span className="ms-goal-intent">{row.intent}</span>
        </summary>
        {standing(row) !== "" && <p className="ms-goal-next">{standing(row)}</p>}
      </details>
      {reasons.length > 0 && (
        <ul className="ms-goal-gaps">
          {reasons.map((reason) => (
            <li key={reason}>{reason}</li>
          ))}
        </ul>
      )}
      <p className="ms-goal-history">
        opened {dateAndTime(row.openedAt)}
        {lastChange(row)}
      </p>
    </article>
  );
}

/**
 * When the record last changed, and what it was, when the record says what it
 * was. A priority compaction writes the operation's verb into every goal it
 * re-ranked, and the engine also folds a compaction into a goal's own event,
 * so a line whose reason is a rank clause may or may not describe this goal.
 * The server withholds the verb in that case rather than guess; the date is
 * exact either way, so the row says when without saying what.
 */
function lastChange(row: Row): string {
  if (row.lastChangeAt === "") {
    return "";
  }
  if (row.lastVerb === "") {
    return ` · last changed ${dateAndTime(row.lastChangeAt)}`;
  }
  return ` · last ${row.lastVerb} ${dateAndTime(row.lastChangeAt)}`;
}

function rankChip(row: Row) {
  if (row.priority > 0 && row.sequence > 0) {
    return (
      <Chip>
        {row.priority}:{row.sequence}
      </Chip>
    );
  }
  return row.where === "live" ? <Chip>unranked</Chip> : null;
}

/** What the record says is next, or what it concluded. */
function standing(row: Row): string {
  if (row.concluded !== "") {
    return row.concluded;
  }
  if (row.abandoned !== undefined && row.abandoned.because !== "") {
    return row.abandoned.because;
  }
  return row.nextStep;
}

/**
 * Everything the record leaves open or explains, one line each: the gaps the
 * server named, then the records behind the lane.
 */
export function reasonsOf(row: Row): string[] {
  const lines = [...row.gaps];
  if (row.claim !== undefined) {
    lines.push(`claimed by ${row.claim.machine} since ${dateAndTime(row.claim.at)}`);
    if (row.claim.landingAt !== "") {
      lines.push(`landing since ${dateAndTime(row.claim.landingAt)}`);
    }
  }
  if (row.fence !== undefined) {
    lines.push(`stopped ${dateAndTime(row.fence.closedAt)}: ${row.fence.reason}`);
  }
  const waiting = row.waiting;
  if (waiting !== undefined && waiting.since !== "") {
    lines.push(`parked by ${waiting.by} since ${dateAndTime(waiting.since)}, from ${waiting.from}: ${waiting.reason}`);
  }
  if (waiting !== undefined && waiting.blocker !== "") {
    lines.push(`opened to block ${waiting.blocker}`);
  }
  if (row.openBlockers.length > 0) {
    lines.push(`blocked by ${row.openBlockers.join(", ")} (open)`);
  }
  if (row.abandoned !== undefined) {
    lines.push(`dropped by ${row.abandoned.by} ${dateAndTime(row.abandoned.at)}`);
  }
  const approved = row.approved;
  if (approved !== undefined && approved.reviewBy !== "") {
    lines.push(`${approved.authority} approval by ${approved.by}, review by ${approved.reviewBy}`);
  }
  return lines;
}

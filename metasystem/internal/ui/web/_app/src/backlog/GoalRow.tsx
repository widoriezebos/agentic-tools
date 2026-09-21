import type { Row } from "./api";
import { dateAndTime } from "./format";
import { Chip } from "../shell/controls";

/**
 * One goal, as the record has it.
 *
 * Nothing is truncated: an intent or a next step is what the human wrote, and
 * a row that cut it off would hide the one sentence that says what the work
 * is. Nothing here is a link and nothing selects, because this build has no
 * goal view to open yet; the identifier is text a human can copy.
 */
export function GoalRow({ row }: { row: Row }) {
  const reasons = reasonsOf(row);
  return (
    <article className="ms-goal-row">
      <div className="ms-goal-head">
        <span className="ms-goal-id ms-mono">{row.ref.id}</span>
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
      <p className="ms-goal-intent">{row.intent}</p>
      {standing(row) !== "" && <p className="ms-goal-next">{standing(row)}</p>}
      {reasons.length > 0 && (
        <ul className="ms-goal-gaps">
          {reasons.map((reason) => (
            <li key={reason}>{reason}</li>
          ))}
        </ul>
      )}
      <p className="ms-goal-history">
        opened {dateAndTime(row.openedAt)}
        {row.lastVerb !== "" && ` · last ${row.lastVerb} ${dateAndTime(row.lastChangeAt)}`}
      </p>
    </article>
  );
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

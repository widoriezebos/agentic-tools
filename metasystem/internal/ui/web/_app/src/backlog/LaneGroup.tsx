import type { Row } from "./api";
import { shortTip } from "./format";
import { GoalRow } from "./GoalRow";
import { anchorFor, type Lane } from "./lanes";
import { helpText } from "../help/terms";

/**
 * One lane and the goals in it.
 *
 * A lane with no goals says so about a named projection -- this tip, this
 * observation -- rather than about the repository, because a lane can be empty
 * while the ledger is full. Ready says something else when the claim gate
 * could not be asked, since an empty Ready would then be a claim this build
 * cannot make.
 */
export function LaneGroup({
  lane,
  count,
  rows,
  tip,
  unanswered,
  showing,
  onFaded,
}: {
  lane: Lane;
  count: number;
  rows: Row[];
  tip: string;
  unanswered: string;
  /** The goal a link sent this page to show, while its ring is up, or "". */
  showing: string;
  /** Said when that ring's animation ends, which is what takes it down. */
  onFaded: () => void;
}) {
  return (
    <section className="ms-card" id={anchorFor(lane.id)}>
      <h2 className="ms-card-title">
        {lane.title} ({count})
      </h2>
      {/* The list has room for the sentence itself, so it says it rather than
          offering the icon that would open it. It is the same sentence the
          board's column head explains with: one register, two ways to read. */}
      <p className="ms-lane-meaning">{helpText(lane.help)}</p>
      {rows.length === 0 ? (
        <p className="ms-lane-empty">{emptyLane(lane, tip, unanswered)}</p>
      ) : (
        <div className="ms-goal-rows">
          {rows.map((row) => (
            <GoalRow key={row.ref.id} row={row} shown={row.ref.id === showing} onFaded={onFaded} />
          ))}
        </div>
      )}
    </section>
  );
}

/** The Draft lane carries a statement instead of a count and rows. */
export function DraftGroup({ lane, statement }: { lane: Lane; statement: string }) {
  return (
    <section className="ms-card" id={anchorFor(lane.id)}>
      <h2 className="ms-card-title">{lane.title}</h2>
      <p className="ms-lane-meaning">{helpText(lane.help)}</p>
      <p className="ms-lane-empty">{statement}</p>
    </section>
  );
}

export function emptyLane(lane: Lane, tip: string, unanswered: string): string {
  if (lane.id === "ready" && unanswered !== "") {
    return `Ready could not be answered at tip ${shortTip(tip)}: ${unanswered}; the approved goals it would judge are under Unknown.`;
  }
  return `No goal is in this lane at tip ${shortTip(tip)}.`;
}

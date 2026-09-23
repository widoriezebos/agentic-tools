import type { Backlog, Row } from "./api";
import { concludedWithin, matches, type Filters, type Window } from "./filters";
import { ABANDONED, DONE, laneTitle, shownLanes, SPLIT_TITLE, UNPLACEABLE, type LaneId } from "./lanes";
import { inRankOrder } from "./reorder";
import { isParent } from "./split";

/**
 * What the board is showing, lane by lane.
 *
 * It was inside the board component, which was fine while the board was the
 * only thing that needed to know. It is out here now because the Project
 * Partner needs the same answer: the goals a human is looking at are not in
 * any file — the board is built from the accepted ledger commit — so the
 * context a question carries has to name them, and it must name exactly what
 * is on screen. Two computations of "what the board shows" would be two
 * answers, and the one in the context would be the one nobody could see was
 * wrong.
 *
 * So this is the whole of it, in one pure function over the payload and the
 * three things a human can change about the view: the filters, the Done
 * window, and whether concluded work is shown.
 */

export type Column = {
  /** Stable within one board, and the lane's id for every ordinary lane. */
  key: string;
  title: string;
  /** The lane a drop onto this column would mean. */
  lane: LaneId;
  rows: Row[];
};

/**
 * What stands under the board: the work that is over, and the records this
 * build could not place. Both are readings of the same filtered set the
 * columns are, which is why they are here beside them.
 */
export function boardBelow(backlog: Backlog, filters: Filters): { closed: Row[]; unplaceable: Row[] } {
  const shown = [...backlog.rows, ...backlog.closed].filter((row) => matches(row, filters));
  const inLane = (lane: LaneId) => inRankOrder(shown.filter((row) => row.lane === lane));
  return {
    closed: [...inLane(ABANDONED), ...shown.filter(isParent)],
    unplaceable: inLane(UNPLACEABLE),
  };
}

export function boardColumns(
  backlog: Backlog,
  filters: Filters,
  reach: Window,
  closedShown: boolean,
  now: Date,
): Column[] {
  // Every row the payload carries, which is what a relationship is read
  // against: a split member says what it is part of whether or not its parent
  // passes the filter, and a parent counts all of its members.
  const all = [...backlog.rows, ...backlog.closed];
  const shown = all.filter((row) => matches(row, filters));
  // A lane reads in the order the engine ranks work in — priority, then
  // sequence — because that is the order the chip on each card claims and the
  // order a drag inside the lane rearranges.
  const inLane = (lane: LaneId) => inRankOrder(shown.filter((row) => row.lane === lane));
  const parents = shown.filter(isParent);
  const delivered = inLane(DONE).filter((row) => !isParent(row) && concludedWithin(row, reach, now));

  const columns: Column[] = [
    ...shownLanes.map((lane) => ({
      key: lane.id,
      title: lane.title,
      lane: lane.id,
      rows: lane.id === "draft" ? [] : inLane(lane.id),
    })),
    { key: DONE, title: laneTitle(DONE), lane: DONE, rows: delivered },
  ];
  if (closedShown) {
    columns.push(
      { key: ABANDONED, title: laneTitle(ABANDONED), lane: ABANDONED, rows: inLane(ABANDONED) },
      { key: "split", title: SPLIT_TITLE, lane: DONE, rows: parents },
    );
  }
  return columns;
}

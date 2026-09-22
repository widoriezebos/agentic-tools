/**
 * Where a card goes when it is dragged past another one.
 *
 * A drag inside a lane is not a move, it is a re-rank, and the rank is not
 * the lane's. `goal set-priority` places a goal in a priority band at a
 * one-based position among **every live goal at that priority**, and a band
 * is spread across the lanes: two goals at 2:3 and 2:4 can sit in To Do and
 * In Progress and still be neighbours in the band. So the position a drop
 * asks for is computed against the band and never against the column, which
 * is the one thing a reading of this that looked only at the lane would get
 * wrong on every board with work in flight.
 *
 * The engine refuses a position outside `1..len(band without the goal)+1`
 * rather than clamping it (internal/goal/order.go:102-115), and inserting
 * renumbers the whole band. Both facts are honoured here: the position is
 * computed from the band with the moved goal taken out, which is exactly the
 * list the engine inserts into, and a request that would change nothing is
 * answered with null rather than published.
 *
 * Nothing here predicts where the goal landed. The act answers with the board
 * as the ledger then stood, and the note the lane shows is read from that.
 */

import type { Row } from "./api";

/** Where a card lands: the band, and the one-based position in it. */
export type Placement = { priority: number; sequence: number };

/** Which side of a card the pointer is on when the drop happens. */
export type Side = "above" | "below";

/**
 * The live goals at one priority, in sequence order.
 *
 * Concluded goals are not in a band: the engine compacts a band when a goal
 * leaves the live set, so a done goal's old rank is nobody's neighbour.
 */
export function bandOf(priority: number, rows: readonly Row[], without = ""): Row[] {
  return rows
    .filter(
      (row) => row.where === "live" && row.priority === priority && row.ref.id !== without,
    )
    .sort((left, right) => left.sequence - right.sequence);
}

/** Cards as a lane shows them: by priority, then by sequence, then by id. */
export function inRankOrder(rows: readonly Row[]): Row[] {
  return [...rows].sort((left, right) => {
    const ranked = (row: Row) => row.priority > 0 && row.sequence > 0;
    if (ranked(left) !== ranked(right)) {
      return ranked(left) ? -1 : 1;
    }
    if (!ranked(left)) {
      return left.ref.id.localeCompare(right.ref.id);
    }
    if (left.priority !== right.priority) {
      return left.priority - right.priority;
    }
    if (left.sequence !== right.sequence) {
      return left.sequence - right.sequence;
    }
    return left.ref.id.localeCompare(right.ref.id);
  });
}

/**
 * Where a card goes when it is dropped above or below a neighbour.
 *
 * It takes the neighbour's band, because that is what "here, beside this one"
 * means, and the position that puts it on the asked-for side of the neighbour
 * once it has been taken out of the band itself. A drop that would leave the
 * goal exactly where it is answers null: there is nothing to publish, and
 * publishing it would spend a ledger transaction on a card that did not move.
 */
export function placementFor(
  moved: Row,
  neighbour: Row,
  side: Side,
  rows: readonly Row[],
): Placement | null {
  if (moved.ref.id === neighbour.ref.id || neighbour.priority < 1) {
    return null;
  }
  const band = bandOf(neighbour.priority, rows, moved.ref.id);
  const at = band.findIndex((row) => row.ref.id === neighbour.ref.id);
  if (at < 0) {
    return null;
  }
  const sequence = side === "above" ? at + 1 : at + 2;
  return settled(moved, { priority: neighbour.priority, sequence }, band);
}

/**
 * Where a card goes when the keyboard moves it one place in its own band.
 *
 * A goal at the end it is being moved towards has nowhere to go, and answers
 * null; so does a goal with no rank at all, which is not in a band yet and
 * has no neighbour to step past.
 */
export function stepFor(moved: Row, direction: "up" | "down", rows: readonly Row[]): Placement | null {
  if (moved.priority < 1) {
    return null;
  }
  const whole = bandOf(moved.priority, rows);
  const at = whole.findIndex((row) => row.ref.id === moved.ref.id);
  if (at < 0) {
    return null;
  }
  if (direction === "up" ? at < 1 : at > whole.length - 2) {
    return null;
  }
  const band = bandOf(moved.priority, rows, moved.ref.id);
  return settled(moved, { priority: moved.priority, sequence: direction === "up" ? at : at + 2 }, band);
}

/**
 * The placement, or null where it asks for the rank the goal already has.
 *
 * Inserting a goal back at the index it was taken from restores the same
 * order, so that request changes nothing — which is what the engine answers
 * with "the requested priority and sequence already hold".
 */
function settled(moved: Row, placement: Placement, band: readonly Row[]): Placement | null {
  const highest = band.length + 1;
  if (placement.sequence < 1 || placement.sequence > highest) {
    return null;
  }
  if (moved.priority === placement.priority && moved.sequence === placement.sequence) {
    return null;
  }
  return placement;
}

/** A rank as a card wears it, and as the note after a re-rank reads it. */
export function rankOf(row: { priority: number; sequence: number }): string {
  return `${String(row.priority)}:${String(row.sequence)}`;
}

/**
 * What the lane says after a re-rank, read from the board the ledger answered
 * with rather than from what was asked for. The two differ whenever the band
 * shifted under the request, which is the case worth telling a human about.
 */
export function landedNote(id: string, rows: readonly Row[]): string {
  const landed = rows.find((row) => row.ref.id === id);
  return landed === undefined ? `${id} moved` : `${id} moved to ${rankOf(landed)}`;
}

/**
 * Whether a re-rank of this goal needs confirming before it is published.
 *
 * A seat holds it. Reprioritising claimed work does not interrupt that seat —
 * the engine changes a rank and nothing else — but a human dragging a card in
 * In Progress is reasonably expecting it to, so the sheet says what the act
 * does and does not do before it is made. The test is the claim rather than
 * the lane, because Review is claimed work too and deserves the same sentence.
 */
export function needsConfirming(row: Row): boolean {
  return row.claim !== undefined;
}

/** What that sheet says the act will and will not do. */
export const claimedConsequence =
  "A seat is working on this; reprioritising changes what the fleet picks next, not this seat's work.";

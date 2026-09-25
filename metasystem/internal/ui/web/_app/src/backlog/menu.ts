/**
 * What a card offers, and where it offers it.
 *
 * Nothing is offered on the card itself. A board is a hundred cards, and a
 * control on each of them is a hundred controls competing with the work they
 * are about; the card is the record, and the record is what a human is there
 * to read. So the acts live in a menu that is not there until it is asked
 * for — the pointer's own menu, or Shift+F10 and the Menu key from the
 * keyboard — and the card carries no button at all.
 *
 * The menu lists what is possible and nothing else. An act the card's lane
 * has no verb for is absent rather than present and dim: a disabled row is a
 * promise that the thing exists somewhere, and here it does not. The one
 * exception is that every card can be opened, because every card is a record
 * with a page.
 *
 * The table is here, apart from the menu, for the reason the transition table
 * is apart from the board: it is the rule and not the rendering, and the same
 * rule answers the pointer and the keyboard.
 */

import type { Row } from "./api";
import { editable } from "./editing";
import { transitions } from "./moves";
import { stepFor } from "./reorder";

/** What one row of the menu is: which act, and what it is called. */
export type Offer = { id: OfferId; label: string };

export type OfferId = "ask" | "edit" | "approve" | "withdraw" | "up" | "down" | "rank" | "open";

/**
 * What each act is called where a human chooses it. An ellipsis means a sheet
 * opens and nothing is published yet; its absence means the act is made.
 */
const LABELS: Record<OfferId, string> = {
  ask: "Ask about this",
  edit: "Edit…",
  approve: "Approve…",
  withdraw: "Withdraw approval…",
  up: "Move up",
  down: "Move down",
  rank: "Set priority…",
  open: "Open goal",
};

/** True when this goal is in a priority band, and so has a rank to change. */
export function ranked(row: Row): boolean {
  return row.where === "live" && row.priority > 0 && row.sequence > 0;
}

/**
 * Everything this card can do, in the order the menu lists it: the lane moves
 * first, because they are what a drag between columns does; then the three
 * that re-rank, which are what a drag inside one does and what no keyboard can
 * do by dragging; then opening the goal, which is not an act on the ledger and
 * so comes last.
 */
export function offersFor(row: Row, all: readonly Row[]): Offer[] {
  // Asking about the thing is first, on every surface, because it is the one
  // act every object offers and the one a human reaches for without knowing
  // what else this card can do.
  const offered: OfferId[] = ["ask"];
  // Editing comes between Ask and the lane moves, because it is the act a
  // human reaches for about the goal itself rather than about where it
  // stands. It is offered on the one card the ledger will take it from: a
  // goal still queued that nobody has approved. Every other card is absent
  // rather than dim — a disabled row promises an act that is not there.
  if (editable(row)) {
    offered.push("edit");
  }
  for (const transition of transitions) {
    if (transition.from === row.lane) {
      offered.push(transition.move);
    }
  }
  if (ranked(row)) {
    if (stepFor(row, "up", all) !== null) {
      offered.push("up");
    }
    if (stepFor(row, "down", all) !== null) {
      offered.push("down");
    }
    offered.push("rank");
  }
  offered.push("open");
  return offered.map((id) => ({ id, label: LABELS[id] }));
}

/** Where a menu opened from the keyboard goes, since there is no pointer. */
export type At = { x: number; y: number };

/**
 * True for the two keystrokes that open a context menu without a pointer.
 * Shift+F10 is the one every platform has; the Menu key is the one some
 * keyboards have a key for.
 */
export function opensMenu(key: string, shiftKey: boolean): boolean {
  return key === "ContextMenu" || (shiftKey && key === "F10");
}

/**
 * Where a key takes the focus inside the menu, as an index, or null for a key
 * the menu does not answer to. It is a ring, like the tab strip: a human
 * holding an arrow at the end of it expects the other end.
 */
export function itemAfter(key: string, at: number, count: number): number | null {
  if (count === 0) {
    return null;
  }
  switch (key) {
    case "ArrowUp":
      return (at - 1 + count) % count;
    case "ArrowDown":
      return (at + 1) % count;
    case "Home":
      return 0;
    case "End":
      return count - 1;
    default:
      return null;
  }
}

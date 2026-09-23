import type { Row } from "./api";
import { concludedWithin, matches, WINDOWS, type Filters, type Window } from "./filters";
import { ABANDONED, DONE, DRAFT, UNPLACEABLE } from "./lanes";
import { isParent } from "./split";
import type { BacklogView } from "../storage";

/**
 * Landing on one goal, because a goal page asked for it.
 *
 * "Open in the Backlog list" opened the Backlog and nothing else: the board
 * came up at the top, scrolled to wherever it opens, and the goal a human had
 * just been reading about was in a lane off to the right, behind a Done window
 * that did not reach it, or under a toggle that was closed. The link named an
 * act it did not perform.
 *
 * What it does now is show the goal. The address carries which one, the page
 * reads it once, and the two things that can hide a card — how far Done
 * reaches and whether the closed items are open — are widened or opened far
 * enough for this one arrival, along with the filters where they are what is
 * hiding it. Every one of those is said out loud on the page, because a board
 * that quietly disagrees with the selector above it is a board a human cannot
 * trust; and none of them is written to storage, because the landing changes
 * what this page is showing and not what this browser prefers.
 *
 * The rules are here rather than in the pane so that the ones worth arguing
 * about are readable and tested: which window is the smallest one that
 * reaches a goal, what counts as hidden, and what the page says it did.
 */

/**
 * The mark the goal wears when the page has landed on it: a ring in the
 * accent colour that fades over two seconds.
 *
 * The class is the whole of it. What it looks like is in backlog.css, the
 * fade is a CSS animation, and the end of that animation is what takes the
 * class off again — so the two seconds are the stylesheet's and this build
 * still sets no timer. It is also how the pane finds the goal in order to
 * scroll it into view, which is the one thing it does to the document.
 */
export const SHOWN = "ms-goal-shown";

/**
 * What the goal page's link says, which follows the view the Backlog is
 * actually going to open in. A link promising a board to a human who reads
 * the list is a link that lies about where it goes, and which of the two this
 * browser is on is a stored preference this build already keeps.
 */
export function showLabel(view: BacklogView): string {
  return view === "list" ? "Show in the list →" : "Show on the board →";
}

/**
 * Where the goal stands on the Backlog as it is about to be rendered, which
 * is not the same question as which lane the ledger put it in.
 *
 * Missing is "no card and no row carries this": an id the payload does not
 * have, and the three the reader shows nowhere — the unplaceable, whose
 * reasons are a disclosure under the lanes, and the drafts, which nothing
 * reads.
 */
export type Placement =
  | { where: "missing" }
  /** A lane the board always shows, and the list shows above its closed group. */
  | { where: "open"; row: Row }
  /** Delivered, and read through the Done window on the board. */
  | { where: "done"; row: Row; within: Window }
  /** Abandoned or retired by a split, which is behind the closed-items toggle. */
  | { where: "closed"; row: Row };

/**
 * What a landing changes for this one arrival, and what it says it changed.
 *
 * A field that is absent is one the landing leaves alone, which is why the
 * Done window is optional rather than nullable: null is a window this build
 * offers — every recorded conclusion — and "leave the selector where it is"
 * had to be a different answer from "widen it to all".
 */
export type Landing = {
  doneDays?: Window;
  openClosed?: boolean;
  clearFilters?: boolean;
  /** The lines the page prints above the lanes, which is one or none. */
  notes: string[];
};

/**
 * Where this goal stands, read against the view the page is opening in.
 *
 * The two views hide concluded work differently: the board reads Done through
 * a window of days and keeps the abandoned and the split behind a toggle, and
 * the list has no window and keeps both concluded lanes behind its own
 * toggle. So the same record is a windowed lane on one and a closed group on
 * the other, and the landing answers what it is looking at.
 */
export function placementOf(
  all: readonly Row[],
  goal: string,
  now: Date,
  view: BacklogView,
): Placement {
  const row = all.find((candidate) => candidate.ref.id === goal);
  if (row === undefined || row.lane === UNPLACEABLE || row.lane === DRAFT) {
    return { where: "missing" };
  }
  if (isParent(row) || row.lane === ABANDONED) {
    return { where: "closed", row };
  }
  if (row.lane === DONE) {
    return view === "list" ? { where: "closed", row } : { where: "done", row, within: smallestWindow(row, now) };
  }
  return { where: "open", row };
}

/**
 * The smallest window this build offers that reaches this conclusion, which
 * is what a landing widens Done to: the goal comes into view and the rest of
 * the lane stays as short as it can be. A conclusion nothing dated is in no
 * window of days at all, so the answer there is every recorded one.
 */
function smallestWindow(row: Row, now: Date): Window {
  return WINDOWS.find((candidate) => concludedWithin(row, candidate, now)) ?? null;
}

/**
 * What the page has to change to show this goal, and the line it says it in.
 *
 * Nothing is changed that does not need changing: a goal already inside the
 * Done window widens nothing, a goal in an open lane opens nothing, and
 * filters that let it through are left set. Silence therefore means the
 * landing took the page as the human left it.
 */
export function landingFor(
  goal: string,
  placement: Placement,
  doneDays: Window,
  closedOpen: boolean,
  filters: Filters,
): Landing {
  if (placement.where === "missing") {
    return { notes: [`${goal} is not on the board`] };
  }
  const landing: Landing = { notes: [] };
  const changed: string[] = [];
  if (placement.where === "done" && !reaches(doneDays, placement.within)) {
    landing.doneDays = placement.within;
    changed.push(widened(placement.within));
  }
  if (placement.where === "closed" && !closedOpen) {
    landing.openClosed = true;
    changed.push("opened closed items");
  }
  if (!matches(placement.row, filters)) {
    landing.clearFilters = true;
    changed.push("cleared the filters");
  }
  if (changed.length > 0) {
    landing.notes.push(`Showing ${goal}: ${changed.join(", ")}`);
  }
  return landing;
}

/** True when a Done window already reaches as far back as the goal needs. */
function reaches(doneDays: Window, within: Window): boolean {
  if (doneDays === null) {
    return true;
  }
  return within !== null && doneDays >= within;
}

function widened(within: Window): string {
  if (within === null) {
    return "widened Done to every recorded conclusion";
  }
  return `widened Done to ${String(within)} day${within === 1 ? "" : "s"}`;
}

import type { HelpId } from "../help/terms";

/**
 * The lanes, in the order work moves through them.
 *
 * A lane is a reading of records that already exist, not a state a goal can be
 * put into: the server decides which lane each goal is in, and this table says
 * only what each lane is called and what being in it means. Unknown is last
 * and is never dropped, because a record this build cannot place must still be
 * visible with its reason.
 *
 * What being in a lane means used to be a sentence written here and shown as
 * the browser's own tooltip on the column head — a sentence nothing but a
 * mouse could reach. The sentences moved to the help register, where every
 * other explanation in this interface is, and the lane names the term.
 */

export type LaneId =
  | "draft"
  | "to-do"
  | "ready"
  | "in-progress"
  | "review"
  | "waiting"
  | "done"
  | "abandoned"
  | "unknown";

export type Lane = {
  id: LaneId;
  title: string;
  /**
   * The term that says what a goal in this lane is, or null for the lane that
   * is never a head: Unknown is a place in the projection and a column
   * nowhere, so there is nothing for a help icon to stand beside.
   */
  help: HelpId | null;
};

export const lanes: readonly Lane[] = [
  { id: "draft", title: "Draft", help: "lane-draft" },
  { id: "to-do", title: "To Do", help: "lane-todo" },
  { id: "ready", title: "Ready for Work", help: "lane-ready" },
  { id: "in-progress", title: "In Progress", help: "lane-progress" },
  { id: "review", title: "Review and Verification", help: "lane-review" },
  { id: "waiting", title: "Waiting", help: "lane-waiting" },
  { id: "done", title: "Done", help: "lane-done" },
  { id: "abandoned", title: "Abandoned", help: "lane-abandoned" },
  { id: "unknown", title: "Unknown", help: null },
];

/** The lanes whose goals have left the live ledger. */
export const closedLaneIds: readonly LaneId[] = ["done", "abandoned"];

/**
 * The lane a record this build cannot place lands in.
 *
 * It is a lane in the projection and a column nowhere. A column headed
 * Unknown asked a human to read an empty box on every board in order to learn
 * nothing, and on the rare board where it was not empty it offered the one
 * thing a lane cannot offer: work to be moved out of it. What the board says
 * instead is one disclosure under the lanes, beside the closed items,
 * carrying the count and, opened, the goals with the reason each one carries.
 * Nothing is hidden and nothing is a column.
 */
export const UNPLACEABLE: LaneId = "unknown";

/** The lanes a live goal can be in. Each one is a column of the board. */
export const openLanes: readonly Lane[] = lanes.filter(
  (lane) => !closedLaneIds.includes(lane.id) && lane.id !== UNPLACEABLE,
);

export const closedLanes: readonly Lane[] = lanes.filter((lane) => closedLaneIds.includes(lane.id));

/** The lane whose records this build has no reader for yet. */
export const DRAFT: LaneId = "draft";

/**
 * Whether anything reads drafts. The projection has no reader for
 * plans/goals-drafts/, so the Draft lane could only ever say that it does
 * not, in a column as wide as the lanes that carry work.
 */
export const DRAFTS_READ = false;

/**
 * The lanes this build actually shows.
 *
 * A lane with nothing behind it is not a lane a human is missing: Draft held
 * one sentence saying the engine has no reader, and it held it first, before
 * every lane that does carry work. The lane stays in the table above and
 * comes back here the day a reader does.
 */
export const shownLanes: readonly Lane[] = openLanes.filter((lane) => DRAFTS_READ || lane.id !== DRAFT);

/** The concluded lane the board always carries, read through a date window. */
export const DONE: LaneId = "done";

/** The concluded lane that stays behind the closed-items toggle. */
export const ABANDONED: LaneId = "abandoned";

export function laneTitle(id: LaneId): string {
  return laneFor(id)?.title ?? id;
}

/**
 * What the board calls the column a split parent stands in.
 *
 * It is not a lane. A goal retired by decomposition is `done` in the ledger
 * and is not a delivered outcome, and the master refuses to let the board say
 * it is one; so the board reads those records out of Done and stands them
 * here, under their members, with the closed items.
 */
export const SPLIT_TITLE = "Split into goals";
export const SPLIT_HELP: HelpId = "lane-split";

export function laneFor(id: string): Lane | null {
  return lanes.find((lane) => lane.id === id) ?? null;
}

/** The in-page anchor the lane index links to. */
export function anchorFor(id: LaneId): string {
  return `lane-${id}`;
}

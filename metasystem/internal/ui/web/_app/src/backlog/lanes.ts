/**
 * The lanes, in the order work moves through them.
 *
 * A lane is a reading of records that already exist, not a state a goal can be
 * put into: the server decides which lane each goal is in, and this table says
 * only what each lane is called and what being in it means. Unknown is last
 * and is never dropped, because a record this build cannot place must still be
 * visible with its reason.
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
  /** One sentence: what a goal in this lane is. */
  meaning: string;
};

export const lanes: readonly Lane[] = [
  {
    id: "draft",
    title: "Draft",
    meaning: "A persisted goal proposal that has not passed intake.",
  },
  {
    id: "to-do",
    title: "To Do",
    meaning: "A goal that has not been authorized for execution, with its intake or approval gaps visible.",
  },
  {
    id: "ready",
    title: "Ready for Work",
    meaning: "An approved goal the claim gate would admit right now.",
  },
  {
    id: "in-progress",
    title: "In Progress",
    meaning: "Claimed work being executed.",
  },
  {
    id: "review",
    title: "Review and Verification",
    meaning: "Claimed work that is built and waiting to land.",
  },
  {
    id: "waiting",
    title: "Waiting",
    meaning: "Work an authoritative blocker holds, with the reason and the state it waits from.",
  },
  {
    id: "done",
    title: "Done",
    meaning: "A recorded completed goal, with what it concluded.",
  },
  {
    id: "abandoned",
    title: "Abandoned",
    meaning: "Work dropped with the recorded reason.",
  },
  {
    id: "unknown",
    title: "Unknown",
    meaning: "A goal this build cannot place, with the reason on the row.",
  },
];

/** The lanes whose goals have left the live ledger, behind the closed toggle. */
export const closedLaneIds: readonly LaneId[] = ["done", "abandoned"];

/** The lanes a live goal can be in, plus the one that carries no rows. */
export const openLanes: readonly Lane[] = lanes.filter((lane) => !closedLaneIds.includes(lane.id));

export const closedLanes: readonly Lane[] = lanes.filter((lane) => closedLaneIds.includes(lane.id));

export function laneFor(id: string): Lane | null {
  return lanes.find((lane) => lane.id === id) ?? null;
}

/** The in-page anchor the lane index links to. */
export function anchorFor(id: LaneId): string {
  return `lane-${id}`;
}

import { describe, expect, it } from "vitest";

import { anchorFor, closedLaneIds, laneFor, lanes, laneTitle, openLanes, UNPLACEABLE, type Lane } from "./lanes";

/**
 * The lanes, word for word.
 *
 * The table below is the master's, written out again here so that a change to
 * a lane's name or meaning is a change a reviewer sees in a diff rather than a
 * sentence that quietly drifts.
 */
const expected: Lane[] = [
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

describe("the lanes", () => {
  it("say what the master says, word for word, in the order work moves", () => {
    expect(lanes).toEqual(expected);
  });

  it("keep a place for a goal this build cannot place", () => {
    expect(lanes.map((lane) => lane.id)).toContain("unknown");
    expect(lanes[lanes.length - 1].id).toBe("unknown");
  });

  it("name the concluded lanes, and give the live ones their columns", () => {
    expect(closedLaneIds).toEqual(["done", "abandoned"]);
    expect(openLanes.map((lane) => lane.id)).toEqual([
      "draft",
      "to-do",
      "ready",
      "in-progress",
      "review",
      "waiting",
    ]);
  });

  // The lane still exists, because a goal this build cannot place must still
  // have somewhere to be. What was removed is the column: the board says the
  // count in one line above the lanes and opens it to the reasons.
  it("keep unknown as a lane and offer it as no column", () => {
    expect(UNPLACEABLE).toBe("unknown");
    expect(laneFor(UNPLACEABLE)).not.toBeNull();
    expect(openLanes.map((lane) => lane.id)).not.toContain(UNPLACEABLE);
    expect(closedLaneIds).not.toContain(UNPLACEABLE);
  });

  it("title a lane by its id, and fall back to the id for a name that is not one", () => {
    expect(laneTitle("ready")).toBe("Ready for Work");
    expect(laneTitle("unknown")).toBe("Unknown");
    expect(laneTitle("nowhere" as Lane["id"])).toBe("nowhere");
  });

  it("name a lane, and answer null for a name that is not one", () => {
    for (const lane of lanes) {
      expect(laneFor(lane.id)?.title).toBe(lane.title);
    }
    expect(laneFor("nowhere")).toBeNull();
  });

  it("give every lane an anchor of its own", () => {
    const anchors = lanes.map((lane) => anchorFor(lane.id));
    expect(new Set(anchors).size).toBe(lanes.length);
    expect(anchorFor("to-do")).toBe("lane-to-do");
  });

  it("end a meaning as a sentence and a title as a name", () => {
    for (const lane of lanes) {
      expect({ id: lane.id, meaning: lane.meaning.endsWith(".") }).toEqual({ id: lane.id, meaning: true });
      expect({ id: lane.id, title: lane.title.endsWith(".") }).toEqual({ id: lane.id, title: false });
    }
  });
});

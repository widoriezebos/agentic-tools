import { describe, expect, it } from "vitest";

import { anchorFor, closedLaneIds, laneFor, lanes, laneTitle, openLanes, SPLIT_HELP, UNPLACEABLE, type Lane } from "./lanes";
import { HELP } from "../help/terms";

/**
 * The lanes, word for word.
 *
 * The table below is the master's, written out again here so that a change to
 * a lane's name or the term that explains it is a change a reviewer sees in a
 * diff rather than a sentence that quietly drifts. What each lane means is no
 * longer written here: it is one entry of the help register, and the tests
 * below hold that every lane names one that exists.
 */
const expected: Lane[] = [
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

  it("end a title as a name rather than as a sentence", () => {
    for (const lane of lanes) {
      expect({ id: lane.id, title: lane.title.endsWith(".") }).toEqual({ id: lane.id, title: false });
    }
  });

  // Every column head carries a help icon, so every lane that is a column
  // names a term that exists. Unknown is the one lane that is no column, and
  // it names none.
  it("name a term the help register carries, for every lane that is a column", () => {
    for (const lane of lanes) {
      if (lane.id === UNPLACEABLE) {
        expect({ id: lane.id, help: lane.help }).toEqual({ id: lane.id, help: null });
        continue;
      }
      expect({ id: lane.id, known: lane.help !== null && Object.hasOwn(HELP, lane.help) }).toEqual({
        id: lane.id,
        known: true,
      });
    }
  });

  // The split column is the board's and not the lanes': a goal retired by
  // decomposition is `done` in the ledger and is not a delivered outcome. It
  // is a column head all the same, so it has a term of its own.
  it("give the split column a term of its own", () => {
    expect(Object.hasOwn(HELP, SPLIT_HELP)).toBe(true);
    expect(HELP[SPLIT_HELP].term).toBe("Split into goals");
  });

  // No two lanes mean the same thing, so no two share a term: a lane pointed
  // at another's sentence would explain the wrong column.
  it("give each lane a term of its own", () => {
    const terms = lanes.map((lane) => lane.help).filter((help) => help !== null);
    expect(new Set(terms).size).toBe(terms.length);
  });
});

import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { minuteTime } from "../backlog/format";
import type { BoardPayload, Lane, LaneBatch, LaneOwner, LaneOwnerState } from "./api";
import { HostBlocks } from "./HostBoard";
import { LaneBlock, START_COMMAND } from "./LandingLane";

/**
 * The landing lane's panel on the Fleet page (U12, Wido 2026-09-29): the
 * host's one batch-landing lane, read from the same /api/board response the
 * host board reads. The panel shows; the `metasystem landing` verbs act, so
 * the remedy is a command a person copies and never a button.
 *
 * Rendered through its pure half, as the host board's test is: the component's
 * first act is a read and this file reaches no network.
 */

function owner(over: Partial<LaneOwner> = {}): LaneOwner {
  return {
    state: "running",
    pid: 4242,
    since: "2026-09-29T08:10:00Z",
    restarts: 0,
    last_exit: null,
    stopped_by: null,
    retry_hint: null,
    ...over,
  };
}

function batch(over: Partial<LaneBatch> = {}): LaneBatch {
  return {
    id: "b-20",
    state: "proving",
    members: [
      { goal: "goal-a", seat: "m1e" },
      { goal: "goal-b", seat: "ui" },
    ],
    waiting_for: [],
    since: "2026-09-29T09:00:00Z",
    reason: "two members ready, proof running",
    ...over,
  };
}

function lane(over: Partial<Lane> = {}): Lane {
  return {
    root: "/Users/someone/landing-root",
    registered_by: "wido",
    registered_at: "2026-09-28T12:00:00Z",
    owner: owner(),
    batch: batch(),
    next: null,
    summary: "batch b-20 proving 2 goals",
    ...over,
  };
}

// The fixture's day: every stamp above is on 2026-09-29, so the panel
// writes times of day. Pinned, so the tests do not depend on today's date.
const NOW = new Date("2026-09-29T12:00:00Z");

function draw(value: Lane | null | undefined): string {
  return renderToStaticMarkup(<LaneBlock lane={value} now={NOW} />);
}

describe("the landing lane panel", () => {
  it("heads the panel with the server's summary and names the lane root", () => {
    const markup = draw(lane());
    expect(markup).toContain("Landing lane");
    expect(markup).toContain("batch b-20 proving 2 goals");
    expect(markup).toContain("/Users/someone/landing-root");
    expect(markup).toContain("wido");
  });

  const colours: [LaneOwnerState, string][] = [
    ["running", "ms-fleet-lane-state--ok"],
    ["restarting", "ms-fleet-lane-state--warn"],
    ["stopped", "ms-fleet-lane-state--bad"],
    ["given-up", "ms-fleet-lane-state--bad"],
    ["not-started", "ms-fleet-lane-state--neutral"],
  ];
  for (const [state, colour] of colours) {
    it(`draws the owner ${state} in its own status colour`, () => {
      const markup = draw(lane({ owner: owner({ state }) }));
      expect(markup).toContain(colour);
      expect(markup).toContain(`>${state.replace("-", " ")}<`);
    });
  }

  it("says since when and how often a running owner restarted", () => {
    const markup = draw(lane({ owner: owner({ restarts: 3 }) }));
    expect(markup).toContain(`since ${minuteTime("2026-09-29T08:10:00Z")}`);
    expect(markup).toContain("pid 4242");
    expect(markup).toContain("3 restarts");
    expect(markup).not.toContain(START_COMMAND);
  });

  it("gives a given-up owner its hint, last exit and the command to start it", () => {
    const markup = draw(
      lane({
        owner: owner({
          state: "given-up",
          pid: null,
          restarts: 5,
          last_exit: "exit status 2",
          retry_hint: "five restarts in ten minutes; read the lane log",
        }),
      }),
    );
    expect(markup).toContain("five restarts in ten minutes; read the lane log");
    expect(markup).toContain("exit status 2");
    expect(markup).toContain(START_COMMAND);
    expect(markup).toContain("ms-fleet-lane-command");
    expect(markup).not.toContain("<button");
  });

  it("names who stopped a stopped owner and the command to start it", () => {
    const markup = draw(lane({ owner: owner({ state: "stopped", pid: null, stopped_by: "wido" }) }));
    expect(markup).toContain("Stopped by wido");
    expect(markup).toContain(START_COMMAND);
    expect(markup).not.toContain("<button");
  });

  it("tells a not-started owner how to start it", () => {
    const markup = draw(lane({ owner: owner({ state: "not-started", pid: null, since: null }), batch: null }));
    expect(markup).toContain(START_COMMAND);
    expect(markup).not.toContain("since unknown");
  });

  it("says there is no batch in hand when batch is null", () => {
    const markup = draw(lane({ batch: null }));
    expect(markup).toContain("No batch in hand.");
  });

  for (const state of ["collecting", "waiting", "proving", "pushing"] as const) {
    it(`draws a ${state} batch with its members and reason`, () => {
      const markup = draw(lane({ batch: batch({ state }) }));
      expect(markup).toContain("b-20");
      expect(markup).toContain(`>${state}<`);
      expect(markup).toContain("goal-a");
      expect(markup).toContain("@ m1e");
      expect(markup).toContain("goal-b");
      expect(markup).toContain("@ ui");
      expect(markup).toContain("two members ready, proof running");
      expect(markup).toContain(`since ${minuteTime("2026-09-29T09:00:00Z")}`);
    });
  }

  it("lists what a waiting batch waits for, expected in local time", () => {
    const markup = draw(
      lane({
        batch: batch({
          state: "waiting",
          waiting_for: [
            { goal: "goal-c", seat: "m1b", expected: "2026-09-29T10:30:00Z" },
            { goal: "goal-d", seat: "m1c", expected: null },
          ],
        }),
      }),
    );
    expect(markup).toContain("Waiting for");
    expect(markup).toContain("goal-c");
    expect(markup).toContain("@ m1b");
    expect(markup).toContain(`expected ${minuteTime("2026-09-29T10:30:00Z")}`);
    expect(markup).toContain("goal-d");
    expect(markup).toContain("no expected time");
  });

  it("draws the next batch collecting behind the current one", () => {
    const markup = draw(lane({ next: { id: "b-21", members: [{ goal: "goal-e", seat: "m1e" }] } }));
    expect(markup).toContain("Next");
    expect(markup).toContain("b-21");
    expect(markup).toContain("goal-e");
  });

  it("says an empty next batch has no members yet", () => {
    const markup = draw(lane({ next: { id: "b-21", members: [] } }));
    expect(markup).toContain("b-21");
    expect(markup).toContain("no members yet");
  });

  it("says a lane with no root registered has none", () => {
    const markup = draw(lane({ root: null, registered_by: null, registered_at: null }));
    expect(markup).toContain("No lane root is registered.");
  });

  it("says a host without a lane has none", () => {
    const markup = draw(null);
    expect(markup).toContain("Landing lane");
    expect(markup).toContain("No landing lane is registered on this host.");
  });

  it("says an older server does not report the lane, never that there is none", () => {
    const markup = draw(undefined);
    expect(markup).toContain("This server does not report the landing lane.");
    expect(markup).not.toContain("No landing lane is registered");
  });
});

describe("the host's blocks", () => {
  const board: BoardPayload = { readable: true, bridge: "live", seats: [], lines: [] };

  it("draws the lane above the host board", () => {
    const markup = renderToStaticMarkup(<HostBlocks board={{ ...board, lane: lane() }} />);
    expect(markup.indexOf("Landing lane")).toBeGreaterThanOrEqual(0);
    expect(markup.indexOf("Landing lane")).toBeLessThan(markup.indexOf("This host"));
  });

  it("still draws the host board from a server without the lane", () => {
    const markup = renderToStaticMarkup(<HostBlocks board={board} />);
    expect(markup).toContain("This server does not report the landing lane.");
    expect(markup).toContain("This host");
  });
});

import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { minuteTime } from "../backlog/format";
import type { BoardPayload, Lane, LaneEntry, LandNowAnswer, LaneOwner, LaneOwnerState } from "./api";
import { HostBlocks } from "./HostBoard";
import { LandNowView, LaneBlock, landNowLines, landNowOffer, START_COMMAND } from "./LandingLane";

/**
 * The landing lane's panel on the Fleet page (U12, Wido 2026-09-29): the
 * host's one batch-landing lane, read from the same /api/board response the
 * host board reads. The panel shows; the `metasystem landing` verbs act, so
 * a remedy is a command a person copies. Its one button is Land now (goal
 * fleet-card-can-land-now), offered only when work is queued and no landing
 * agent runs.
 *
 * Rendered through its pure half, as the host board's test is: the component's
 * first act is a read and this file reaches no network.
 */

function owner(over: Partial<LaneOwner> = {}): LaneOwner {
  return {
    state: "running",
    pid: 4242,
    since: "2026-09-29T08:10:00Z",
    last_exit: null,
    stopped_by: null,
    retry_hint: null,
    ...over,
  };
}

function entry(over: Partial<LaneEntry> = {}): LaneEntry {
  return {
    goal: "goal-a",
    branch: "goal/goal-a",
    sha: "1234567890abcdef",
    seat: "m1e",
    at: "2026-09-29T09:00:00Z",
    state: "waiting",
    ...over,
  };
}

function lane(over: Partial<Lane> = {}): Lane {
  return {
    root: "/Users/someone/landing-root",
    registered_by: "wido",
    registered_at: "2026-09-28T12:00:00Z",
    owner: owner(),
    summary: "landing lane /Users/someone/landing-root: landing agent running (pid 4242)",
    paused: false,
    agent_alive: true,
    queue: [],
    running_proof: null,
    last_proof: null,
    last_push: null,
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
    expect(markup).toContain("landing agent running (pid 4242)");
    expect(markup).toContain("/Users/someone/landing-root");
    expect(markup).toContain("wido");
  });

  const colours: [LaneOwnerState, string][] = [
    ["running", "ms-fleet-lane-state--ok"],
    ["idle", "ms-fleet-lane-state--neutral"],
    ["stopped", "ms-fleet-lane-state--bad"],
    ["unready", "ms-fleet-lane-state--warn"],
  ];
  for (const [state, colour] of colours) {
    it(`draws the owner ${state} in its own status colour`, () => {
      const markup = draw(lane({ owner: owner({ state }) }));
      expect(markup).toContain(colour);
      expect(markup).toContain(`>${state.replace("-", " ")}<`);
    });
  }

  it("says since when a running owner runs, with no restart count and no prompt", () => {
    const markup = draw(lane());
    expect(markup).toContain(`since ${minuteTime("2026-09-29T08:10:00Z")}`);
    expect(markup).toContain("pid 4242");
    expect(markup).not.toContain("restart");
    expect(markup).not.toContain(START_COMMAND);
  });

  it("gives an idle lane no prompt: idle is a ready lane with nothing to land", () => {
    const markup = draw(lane({ owner: owner({ state: "idle", pid: null, since: null }), agent_alive: false }));
    expect(markup).toContain(">idle<");
    expect(markup).not.toContain(START_COMMAND);
    expect(markup).not.toContain("since unknown");
  });

  it("gives an unready lane its reason and fix, and no start prompt", () => {
    const markup = draw(
      lane({
        owner: owner({
          state: "unready",
          pid: null,
          last_exit: "exit status 2",
          retry_hint: "the landing checkout has local changes; run metasystem landing status --verbose",
        }),
      }),
    );
    expect(markup).toContain("the landing checkout has local changes; run metasystem landing status --verbose");
    expect(markup).toContain("exit status 2");
    expect(markup).not.toContain(START_COMMAND);
    expect(markup).not.toContain("<button");
  });

  it("names who stopped a stopped lane, why, and the command to resume it", () => {
    const markup = draw(
      lane({ owner: owner({ state: "stopped", pid: null, stopped_by: "wido", stopped_because: "maintenance" }) }),
    );
    expect(markup).toContain("Stopped by wido: maintenance.");
    expect(markup).toContain(START_COMMAND);
    expect(markup).toContain("ms-fleet-lane-command");
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

describe("the plain lane on the card", () => {
  it("says whether the lane is paused and whether a landing agent is alive", () => {
    const open = draw(lane());
    expect(open).toContain("not paused");
    expect(open).toContain("agent alive");
    const stopped = draw(lane({ paused: true, agent_alive: false, owner: owner({ state: "stopped", pid: null }) }));
    expect(stopped).toContain(">paused<");
    expect(stopped).toContain("no agent running");
  });

  it("lists the queue's waiting and returned hand-ins with goal, seat, state and reason", () => {
    const markup = draw(
      lane({
        queue: [
          entry(),
          entry({ goal: "goal-b", seat: "ui", state: "returned", reason: "red on app-standard" }),
          entry({ goal: "goal-c", state: "landed" }),
          entry({ goal: "goal-d", state: "superseded" }),
        ],
      }),
    );
    expect(markup).toContain("Queue");
    expect(markup).toContain("goal-a");
    expect(markup).toContain("@ m1e");
    expect(markup).toContain(">waiting<");
    expect(markup).toContain("goal-b");
    expect(markup).toContain("@ ui");
    expect(markup).toContain(">returned<");
    expect(markup).toContain("red on app-standard");
    expect(markup).not.toContain("goal-c");
    expect(markup).not.toContain("goal-d");
    expect(markup).toContain("2 landed or superseded not shown");
  });

  it("says nothing waits when the queue holds nothing to land", () => {
    expect(draw(lane())).toContain("Nothing waits in the queue.");
  });

  it("draws the running proof, and a proof that died without a result", () => {
    const running = draw(
      lane({ running_proof: { tree: "abcdef1234567", since: "2026-09-29T11:00:00Z", attempt: "a-7", state: "running" } }),
    );
    expect(running).toContain("Proving");
    expect(running).toContain("abcdef1");
    expect(running).toContain("attempt a-7");
    expect(running).toContain(`since ${minuteTime("2026-09-29T11:00:00Z")}`);
    const died = draw(
      lane({ running_proof: { tree: "abcdef1234567", since: "2026-09-29T11:00:00Z", attempt: "a-7", state: "died" } }),
    );
    expect(died).toContain("died without a result");
  });

  it("draws the last proof and the last push", () => {
    const markup = draw(
      lane({
        last_proof: { tree: "t1", commit: "c0ffee1234567", result: "red", log: "/l", at: "2026-09-29T10:00:00Z", reason: "timed out" },
        last_push: { old: "0123456789ab", commit: "fedcba987654", tree: "t2", at: "2026-09-29T10:30:00Z" },
      }),
    );
    expect(markup).toContain("Last proof");
    expect(markup).toContain(">red<");
    expect(markup).toContain("c0ffee1");
    expect(markup).toContain("timed out");
    expect(markup).toContain("Last push");
    expect(markup).toContain("0123456 → fedcba9");
    expect(markup).toContain(`at ${minuteTime("2026-09-29T10:30:00Z")}`);
  });

  it("draws no batch: the plain lane has none", () => {
    expect(draw(lane())).not.toContain("batch");
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

/** A lane with work queued and no agent running: Land now's one offer. */
function queued(over: Partial<Lane> = {}): Lane {
  return lane({
    owner: owner({ state: "idle", pid: null, since: null }),
    agent_alive: false,
    queue: [entry()],
    wake: { reasons: ["queued"], unread: [] },
    ...over,
  });
}

const LAND_NOW_BUTTON = /<button[^>]*>Land now<\/button>/u;

describe("Land now on the landing lane card", () => {
  it("is offered when work is queued and no landing agent runs", () => {
    expect(landNowOffer(queued())).toEqual({ offered: true, reason: "" });
    const button = LAND_NOW_BUTTON.exec(draw(queued()));
    expect(button).not.toBe(null);
    expect(button?.[0]).not.toContain("disabled");
  });

  it("is offered for a waiting hand-in, read from the queue and not the wake", () => {
    expect(landNowOffer(queued({ wake: { reasons: [], unread: [] } })).offered).toBe(true);
    expect(landNowOffer(queued({ wake: undefined })).offered).toBe(true);
  });

  it("is not offered, and says nothing, when nothing in the queue waits", () => {
    const returned = queued({ queue: [entry({ state: "returned", reason: "red" }), entry({ goal: "goal-c", state: "landed" })] });
    expect(landNowOffer(returned)).toEqual({ offered: false, reason: "" });
    expect(draw(returned)).not.toContain("Land now");
  });

  it("reads the keeper's wake from a server that sends no queue", () => {
    expect(landNowOffer(queued({ queue: undefined })).offered).toBe(true);
    expect(landNowOffer(queued({ queue: undefined, wake: { reasons: [], unread: [] } })).offered).toBe(false);
  });

  it("is not offered while the landing agent runs, and says why in one line", () => {
    const running = queued({ owner: owner({ state: "running" }), agent_alive: true });
    const offer = landNowOffer(running);
    expect(offer.offered).toBe(false);
    expect(offer.reason).toBe("The landing agent is already running; it lands the queued work.");
    const markup = draw(running);
    expect(markup).not.toMatch(LAND_NOW_BUTTON);
    expect(markup).toContain(offer.reason);
  });

  it("is not offered while the lane is stopped, and says why in one line", () => {
    const stopped = queued({ owner: owner({ state: "stopped", pid: null, stopped_by: "wido" }), paused: true });
    const offer = landNowOffer(stopped);
    expect(offer.offered).toBe(false);
    expect(offer.reason).toBe("Land now waits until the lane is started again.");
    expect(draw(stopped)).not.toContain("<button");
  });

  it("is not offered while the lane cannot run, and says the lane's own fix in one line", () => {
    const unready = queued({
      owner: owner({
        state: "unready",
        pid: null,
        since: null,
        last_exit: "the landing checkout names no machine",
        retry_hint: "name the landing checkout's machine once (any one word), then run metasystem landing start",
      }),
    });
    const offer = landNowOffer(unready);
    expect(offer).toEqual({
      offered: false,
      reason:
        "Land now is unavailable until the lane can run: name the landing checkout's machine once (any one word), then run metasystem landing start",
    });
    expect(draw(unready)).not.toMatch(LAND_NOW_BUTTON);
  });

  it("is not offered while the lane cannot run, even with no fix to name", () => {
    const unready = queued({ owner: owner({ state: "unready", pid: null, since: null, retry_hint: null }) });
    expect(landNowOffer(unready)).toEqual({ offered: false, reason: "Land now is unavailable until the lane can run." });
  });

  const started: LandNowAnswer = {
    outcome: "confirmed",
    summary: "started the landing agent for batch b-20 (2 members)",
    next: { argv: ["metasystem", "landing", "status"], reason: "follows it" },
  };

  it("shows the verb's line 1 and line 2 after a press", () => {
    expect(landNowLines(started)).toEqual({
      line1: "started the landing agent for batch b-20 (2 members)",
      command: "metasystem landing status",
      reason: "follows it",
      refused: false,
    });
    const markup = renderToStaticMarkup(
      <LandNowView offer={{ offered: false, reason: "" }} sending={false} answer={started} problem="" onPress={() => undefined} />,
    );
    expect(markup).toContain("started the landing agent for batch b-20 (2 members)");
    expect(markup).toContain("metasystem landing status");
    expect(markup).toContain("follows it");
    expect(markup).not.toContain("confirmed");
    expect(markup).not.toContain("ms-trouble");
  });

  it("shows a repeat that started nothing as the success it is", () => {
    const unchanged: LandNowAnswer = {
      outcome: "unchanged",
      summary: "the landing agent already runs (pid 4242); nothing was started",
      next: null,
    };
    expect(landNowLines(unchanged)).toEqual({
      line1: "the landing agent already runs (pid 4242); nothing was started",
      command: "",
      reason: "",
      refused: false,
    });
  });

  it("shows a refusal's two lines in plain words, never its code", () => {
    const refused: LandNowAnswer = {
      outcome: "refused",
      summary: "this seat is at the helm, so the landing agent was not started",
      next: { argv: ["metasystem", "helm", "return"], reason: "hands the helm back" },
    };
    const markup = renderToStaticMarkup(
      <LandNowView offer={queuedOffer} sending={false} answer={refused} problem="" onPress={() => undefined} />,
    );
    expect(markup).toContain("this seat is at the helm, so the landing agent was not started");
    expect(markup).toContain("metasystem helm return");
    expect(markup).toContain("hands the helm back");
    expect(markup).toContain("ms-trouble");
    expect(markup).not.toContain(">refused<");
  });

  it("gives a line 2 with no command as its reason alone", () => {
    expect(
      landNowLines({ outcome: "refused", summary: "no work is queued; nothing was started", next: { argv: [], reason: "it starts when work joins" } }),
    ).toEqual({ line1: "no work is queued; nothing was started", command: "", reason: "it starts when work joins", refused: true });
  });

  it("shows a nothing-to-do answer's reason as line 2, with no Next command", () => {
    const empty: LandNowAnswer = {
      outcome: "unchanged",
      summary: "the landing lane at /w/landing has no queued work, so no landing agent was started",
      next: { argv: [], reason: "nothing to do; the lane is empty" },
    };
    expect(landNowLines(empty)).toEqual({
      line1: "the landing lane at /w/landing has no queued work, so no landing agent was started",
      command: "",
      reason: "nothing to do; the lane is empty",
      refused: false,
    });
    const markup = renderToStaticMarkup(
      <LandNowView offer={{ offered: false, reason: "" }} sending={false} answer={empty} problem="" onPress={() => undefined} />,
    );
    expect(markup).toContain("no queued work, so no landing agent was started");
    expect(markup).toContain("nothing to do; the lane is empty");
    expect(markup).not.toContain("Next:");
    expect(markup).not.toContain("ms-fleet-lane-command");
    expect(markup).not.toContain("ms-trouble");
  });

  it("holds the button while a press is on its way", () => {
    const markup = renderToStaticMarkup(
      <LandNowView offer={queuedOffer} sending={true} answer={null} problem="" onPress={() => undefined} />,
    );
    expect(LAND_NOW_BUTTON.exec(markup)?.[0]).toContain("disabled");
  });
});

const queuedOffer = { offered: true, reason: "" };

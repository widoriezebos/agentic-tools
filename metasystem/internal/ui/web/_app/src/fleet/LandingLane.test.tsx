import { readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { renderToStaticMarkup } from "react-dom/server";
import { MemoryRouter } from "react-router";
import { describe, expect, it } from "vitest";

import { minuteTime } from "../backlog/format";
import type { Lane, LaneEntry, LandNowAnswer, LaneOwner } from "./api";
import { LandNowView, LaneBlock, landNowLines, landNowOffer } from "./LandingLane";
import { unreadOf, type BoardReading } from "./panel";

/**
 * The landing lane's block on the Fleet page: one state word, the one button
 * that makes sense in that state (Land now, only when work waits and nothing
 * proves), and the lane's four lists — what waits, what is being proved,
 * what landed today and what came back — each item with a Details disclosure
 * for the commit, the branch and the seat. It is read from the same
 * /api/board response the Doing column and the questions are.
 *
 * Rendered through its pure half: the component's first act is a read and
 * this file reaches no network.
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
    problems: [],
    ...over,
  };
}

// The fixture's day, at the viewer's own noon: every stamp above is on that
// calendar day in every zone the suite runs in, so the panel writes times of
// day. Pinned, so the tests do not depend on today's date.
const NOW = new Date(2026, 8, 29, 12, 0, 0);

function local(hours: number, minutes = 0): string {
  return new Date(2026, 8, 29, hours, minutes, 0).toISOString().replace(/\.\d{3}Z$/, "Z");
}

/**
 * The block as the panel draws it, with what the panel's one rule says the
 * lane could not read (unreadOf) for a board read that carried this lane.
 */
function draw(value: Lane | null | undefined, titles: Record<string, string> = {}, problem = ""): string {
  const board: BoardReading =
    problem !== ""
      ? { state: "failed", message: problem }
      : { state: "read", board: { readable: true, bridge: "live", seats: [], lines: [], lane: value, questions: [], questionsProblem: "", unreadable: [] } };
  return renderToStaticMarkup(
    <MemoryRouter>
      <LaneBlock lane={value} titles={titles} problem={problem} unread={unreadOf({ state: "loading" }, board).lane} now={NOW} />
    </MemoryRouter>,
  );
}

describe("the landing lane block", () => {
  it("heads the block with the lane's one state word", () => {
    expect(draw(lane())).toContain(">Running<");
    expect(draw(lane({ owner: owner({ state: "idle", pid: null }), agent_alive: false }))).toContain(">Running<");
    expect(draw(lane({ owner: owner({ state: "stopped", stopped_by: "wido" }), paused: true, agent_alive: false }))).toContain(">Paused<");
    expect(draw(lane({ owner: owner({ state: "unready", last_exit: "x" }), agent_alive: false }))).toContain(">Needs attention<");
  });

  it("uses one word per state: never stopped, started again or not paused", () => {
    const paused = draw(lane({ owner: owner({ state: "stopped", stopped_by: "wido" }), paused: true, agent_alive: false, queue: [entry()] }));
    for (const word of ["stopped", "started again", "not paused", "agent alive"]) {
      expect(paused).not.toContain(word);
    }
    expect(paused).toContain("Land now waits until the lane is resumed.");
  });

  it("keeps the root, the agent's pid and the server's summary behind one Details disclosure", () => {
    const markup = draw(lane());
    const details = markup.slice(markup.indexOf("<details"));
    expect(markup).toContain("<summary");
    expect(details).toContain("/Users/someone/landing-root");
    expect(details).toContain("4242");
    expect(details).toContain("landing agent running (pid 4242)");
    expect(details).toContain("wido");
  });

  it("puts the last proof and the last push in that disclosure, never as lines of their own", () => {
    const markup = draw(
      lane({
        last_proof: { tree: "t1", commit: "c0ffee1234567", result: "green", log: "/l/proof.log", at: "2026-09-29T10:00:00Z" },
        last_push: { old: "0123456789ab", commit: "fedcba987654", tree: "t2", at: "2026-09-29T10:30:00Z" },
      }),
    );
    const details = markup.slice(markup.indexOf("<details"));
    expect(details).toContain("c0ffee1");
    expect(details).toContain("/l/proof.log");
    expect(details).toContain("0123456 → fedcba9");
    expect(markup.indexOf("c0ffee1")).toBeGreaterThan(markup.indexOf("<details"));
  });

  it("makes the proof log rows links to the proof's log, by the attempt each record names", () => {
    const markup = draw(
      lane({
        last_proof: { tree: "t1", commit: "c0ffee1234567", result: "red", log: "/l/a-9.log", at: "2026-09-29T10:00:00Z", attempt: "a-9" },
        running_proof: { tree: "abcdef1234567", since: local(11, 54), attempt: "a-10", state: "running", log: "/l/a-10.log" },
      }),
    );

    expect(markup).toMatch(/<dt>Proof log<\/dt><dd[^>]*><a [^>]*href="\/api\/fleet\/proof-logs\/a-9"[^>]*target="_blank"[^>]*>\/l\/a-9\.log<\/a>/);
    expect(markup).toMatch(/<dt>Log<\/dt><dd[^>]*><a [^>]*href="\/api\/fleet\/proof-logs\/a-10"[^>]*target="_blank"[^>]*>\/l\/a-10\.log<\/a>/);

    // A record that names no attempt keeps its path as words.
    const older = draw(lane({ last_proof: { tree: "t1", commit: "c0ffee1234567", result: "red", log: "/l/p.log", at: "2026-09-29T10:00:00Z" } }));
    expect(older).toContain("<dt>Proof log</dt><dd class=\"ms-mono\">/l/p.log</dd>");
  });

  it("draws a log in the Details as the panel draws its other links", () => {
    const css = readFileSync(path.join(path.dirname(fileURLToPath(import.meta.url)), "fleet.css"), "utf8");
    const links = css.slice(0, css.indexOf("{", css.indexOf(".ms-fleet-hold-title,")));

    expect(links).toContain(".ms-fleet-details-row dd a");
    expect(css).toContain(".ms-fleet-details-row dd a:hover");
  });

  it("lists what waits by title, seat and age, each with its own Details", () => {
    const markup = draw(lane({ queue: [entry({ goal: "seat-path", at: local(11, 56) })] }), { "seat-path": "Seat path lands without help." });
    expect(markup).toContain("Waiting");
    // The title is the link; the seat and the age beside it are words.
    expect(markup).toMatch(/href="\/backlog\/goal\/seat-path"[^>]*>Seat path lands without help\.<\/a>/);
    expect(markup).toContain("m1e · 4 min");
    const item = markup.slice(markup.indexOf("Seat path lands"));
    expect(item).toContain("<details");
    expect(item).toContain("goal/goal-a");
    expect(item).toContain("1234567890abcdef");
  });

  it("says how long the running proof has run, and a proof that died", () => {
    const running = draw(lane({ running_proof: { tree: "abcdef1234567", since: local(11, 54), attempt: "a-7", state: "running", log: "/l/a-7.log" } }));
    expect(running).toContain("Proving");
    expect(running).toContain("started 6 min ago");
    expect(running).toContain("/l/a-7.log");
    const died = draw(lane({ running_proof: { tree: "abcdef1234567", since: local(11), attempt: "a-7", state: "died" } }));
    expect(died).toContain("stopped without a result; the next proof runs it again");
  });

  it("lists what landed today by the push's time and the delivered sentence", () => {
    const markup = draw(
      lane({
        queue: [
          entry({ goal: "a", state: "landed", landed_at: local(9, 54), delivered: "Stuck agents now ask you on Telegram and wait." }),
          entry({ goal: "b", state: "landed", landed_at: local(6, 54) }),
        ],
      }),
      { b: "The fleet card can land work now." },
    );
    expect(markup).toContain("Landed today");
    expect(markup).toContain(minuteTime(local(9, 54)));
    expect(markup).toContain("Stuck agents now ask you on Telegram and wait.");
    expect(markup).toContain("The fleet card can land work now.");
    expect(markup.indexOf("Stuck agents")).toBeLessThan(markup.indexOf("The fleet card"));
  });

  it("lists what came back with why and the goal to open", () => {
    const markup = draw(
      lane({ queue: [entry({ goal: "lane-check-red", state: "returned", reason: "the full test run is red", returned_at: local(11) })] }),
    );
    expect(markup).toContain("Came back");
    expect(markup).toContain("lane-check-red · the full test run is red");
    expect(markup).toContain(">Open goal<");
    expect(markup).toContain('href="/backlog/goal/lane-check-red"');
  });

  it("says nothing waits when the lane holds nothing", () => {
    const markup = draw(lane());
    expect(markup).toContain("Nothing is waiting to land.");
    expect(markup).not.toContain("Waiting<");
  });

  it("says, in the block, which of the lane's records it could not read", () => {
    // The server's own sentence; its apostrophe is escaped in markup.
    const markup = draw(lane({ problems: ["the queue can't be read: permission denied"] }));
    expect(markup).toContain("Part of the landing lane could not be read: the queue can&#x27;t be read: permission denied");
    expect(markup).toContain("ms-trouble");
  });

  it("says a lane it could not read at all, in the block", () => {
    const markup = draw(undefined, {}, "board answered 500");
    expect(markup).toContain("The landing lane could not be read: board answered 500");
  });

  it("names what the running proof holds, each title opening its goal", () => {
    const markup = draw(
      lane({
        queue: [entry({ goal: "seat-path" })],
        running_proof: { tree: "t", commit: "c", since: local(11, 54), attempt: "a-9", state: "running", goals: ["seat-path"] },
      }),
      { "seat-path": "Seat path lands without help." },
    );
    const proving = markup.slice(markup.indexOf("Proving"));
    expect(proving).toMatch(/href="\/backlog\/goal\/seat-path"[^>]*>Seat path lands without help\.<\/a>/);
    expect(proving).toContain("started 6 min ago");
    expect(markup).not.toContain("Waiting");
  });

  it("says a lane whose registration could not be read, never that there is none", () => {
    const markup = draw(
      lane({
        root: null,
        registered_by: null,
        registered_at: null,
        owner: owner({ state: "unready", pid: null, since: null }),
        queue: [],
        problems: ["the lane's registration can't be read: permission denied"],
      }),
    );
    expect(markup).toContain("The landing lane could not be read: the lane&#x27;s registration can&#x27;t be read: permission denied");
    expect(markup).not.toContain("No landing lane is registered");
    expect(markup).not.toContain("Needs attention");
    expect(markup).not.toContain("Nothing is waiting to land.");
  });

  it("says a computer without a lane has none", () => {
    const markup = draw(null);
    expect(markup).toContain("Landing lane");
    expect(markup).toContain("No landing lane is registered on this computer.");
  });

  it("says a failed re-read over a last reading of no lane, never only that there is none", () => {
    const markup = draw(null, {}, "the server did not answer");
    expect(markup).toContain("The landing lane could not be read again, so what is on screen is the last reading: the server did not answer");
    expect(markup).toContain("No landing lane is registered on this computer.");
  });

  it("says an older server does not report the lane, never that there is none", () => {
    const markup = draw(undefined);
    expect(markup).toContain("This server does not report the landing lane.");
    expect(markup).not.toContain("No landing lane is registered");
  });

  it("draws no batch: the plain lane has none", () => {
    expect(draw(lane())).not.toContain("batch");
  });

  it("draws a skeleton, not nothing, while the lane is read", () => {
    const markup = renderToStaticMarkup(<LaneBlock lane={undefined} loading unread={[]} now={NOW} />);
    expect(markup).toContain("ms-skeleton");
    expect(markup).toContain('aria-busy="true"');
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
    expect(landNowOffer(queued(), [])).toEqual({ offered: true, reason: "" });
    const button = LAND_NOW_BUTTON.exec(draw(queued()));
    expect(button).not.toBe(null);
    expect(button?.[0]).not.toContain("disabled");
  });

  it("is offered for a waiting hand-in, read from the queue and not the wake", () => {
    expect(landNowOffer(queued({ wake: { reasons: [], unread: [] } }), []).offered).toBe(true);
    expect(landNowOffer(queued({ wake: undefined }), []).offered).toBe(true);
  });

  it("is not offered, and says nothing, when nothing in the queue waits", () => {
    const returned = queued({ queue: [entry({ state: "returned", reason: "red" }), entry({ goal: "goal-c", state: "landed" })] });
    expect(landNowOffer(returned, [])).toEqual({ offered: false, reason: "" });
    expect(draw(returned)).not.toContain("Land now");
  });

  it("reads the keeper's wake from a server that sends no queue", () => {
    expect(landNowOffer(queued({ queue: undefined }), []).offered).toBe(true);
    expect(landNowOffer(queued({ queue: undefined, wake: { reasons: [], unread: [] } }), []).offered).toBe(false);
  });

  it("is not offered while the landing agent runs, and says why in one line", () => {
    const running = queued({ owner: owner({ state: "running" }), agent_alive: true });
    const offer = landNowOffer(running, []);
    expect(offer.offered).toBe(false);
    expect(offer.reason).toBe("The landing agent is already running; it lands the queued work.");
    const markup = draw(running);
    expect(markup).not.toMatch(LAND_NOW_BUTTON);
    expect(markup).toContain(offer.reason);
  });

  it("is not offered while the lane is paused, and says why in one line", () => {
    const stopped = queued({ owner: owner({ state: "stopped", pid: null, stopped_by: "wido" }), paused: true });
    const offer = landNowOffer(stopped, []);
    expect(offer.offered).toBe(false);
    expect(offer.reason).toBe("Land now waits until the lane is resumed.");
    expect(draw(stopped)).not.toContain("<button");
  });

  it("is not offered while any part of the lane could not be read, and says why in one line", () => {
    const torn = queued({ problems: ["the running proof can't be read: permission denied"] });
    expect(landNowOffer(torn, torn.problems ?? [])).toEqual({ offered: false, reason: "Land now waits until the landing lane can be read." });
    expect(draw(torn)).not.toMatch(LAND_NOW_BUTTON);
    expect(draw(torn)).toContain("Land now waits until the landing lane can be read.");
  });

  it("is not offered while a proof runs, and says why in one line", () => {
    const proving = queued({ running_proof: { tree: "t", since: "2026-09-29T11:50:00Z", attempt: "a-8", state: "running" } });
    expect(landNowOffer(proving, [])).toEqual({ offered: false, reason: "Land now waits while a proof runs." });
    expect(draw(proving)).not.toMatch(LAND_NOW_BUTTON);
    const died = queued({ running_proof: { tree: "t", since: "2026-09-29T11:50:00Z", attempt: "a-8", state: "died" } });
    expect(landNowOffer(died, []).offered).toBe(true);
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
    const offer = landNowOffer(unready, []);
    expect(offer).toEqual({
      offered: false,
      reason:
        "Land now is unavailable until the lane can run: name the landing checkout's machine once (any one word), then run metasystem landing start",
    });
    expect(draw(unready)).not.toMatch(LAND_NOW_BUTTON);
  });

  it("is not offered while the lane cannot run, even with no fix to name", () => {
    const unready = queued({ owner: owner({ state: "unready", pid: null, since: null, retry_hint: null }) });
    expect(landNowOffer(unready, [])).toEqual({ offered: false, reason: "Land now is unavailable until the lane can run." });
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

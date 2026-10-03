import { describe, expect, it } from "vitest";

import { minuteTime } from "../backlog/format";
import { NEEDS_YOU_REMEDY } from "./fleet";
import type { BoardPayload, BoardSeat, Held, Lane, LaneEntry, LaneOwner, Machine, Page, ThisSeat, Working } from "./api";
import {
  doingOf,
  laneLists,
  laneState,
  needsOf,
  SCOPE,
  unreadOf,
  unreadSections,
  verdictOf,
  type BoardReading,
  type FleetReading,
} from "./panel";

/**
 * The panel's rules, apart from the elements that draw them: what the
 * verdict says and when, where each Needs you item comes from, what the
 * Doing column says, and what the landing lane's four lists hold.
 *
 * Every instant below is written against one clock, never the wall: noon on
 * the viewer's own calendar day, so "today" is the same day in every zone the
 * suite runs in.
 */

const now = new Date(2026, 9, 2, 12, 0, 0);

function at(minutes: number): string {
  return new Date(now.getTime() + minutes * 60000).toISOString().replace(/\.\d{3}Z$/, "Z");
}

function seat(over: Partial<ThisSeat> = {}): ThisSeat {
  return {
    machine: "ui",
    noNickname: false,
    armed: "armed",
    health: {
      state: "healthy",
      observedAt: at(-3),
      problem: "",
      roles: [{ role: "steward-runner", status: "alive", reason: "runner alive" }],
    },
    publication: null,
    publicationProblem: "",
    running: null,
    runningProblem: "",
    ...over,
  };
}

function held(over: Partial<Held> = {}): Held {
  return {
    goal: "one-folder",
    title: "One folder deployed and evolved.",
    lane: "in-progress",
    machine: "m1f",
    standing: "reachable",
    since: "",
    flag: "",
    ...over,
  };
}

function working(over: Partial<Working> = {}): Working {
  return {
    goal: "one-folder",
    phase: { role: "building", round: 1, roundLimit: null },
    job: { id: "l-1", role: "building", status: "running", startedAt: at(-42), capMinutes: null, capEndsAt: null },
    box: null,
    chain: [],
    ...over,
  };
}

function machine(over: Partial<Machine> = {}): Machine {
  return {
    machine: "m1f",
    standing: "reachable",
    reason: "published 1 min ago",
    ageSeconds: 60,
    seen: at(-1),
    since: "",
    running: null,
    engine: "3f9c1e2abcdef",
    generation: 4,
    holds: [held()],
    this: false,
    working: [],
    workingProblem: "",
    ...over,
  };
}

function page(over: Partial<Page> = {}): Page {
  return {
    schemaVersion: 1,
    readAt: at(0),
    copy: { source: "the interface", attemptedAt: at(-1), succeededAt: at(-1), failedAt: "", problem: "", metadataProblem: "" },
    claims: { tip: "5b9d958c0d1e", unavailable: "" },
    this: seat(),
    needsYou: [],
    machines: [machine({ machine: "ui", this: true, holds: [] }), machine()],
    launches: [],
    launching: { parent: "", repository: "" },
    ...over,
  };
}

function owner(over: Partial<LaneOwner> = {}): LaneOwner {
  return { state: "idle", pid: null, since: null, last_exit: null, stopped_by: null, retry_hint: null, ...over };
}

function entry(over: Partial<LaneEntry> = {}): LaneEntry {
  return { goal: "goal-a", branch: "goal/goal-a", sha: "1234567890abcdef", seat: "m1g", at: at(-4), state: "waiting", ...over };
}

function lane(over: Partial<Lane> = {}): Lane {
  return {
    root: "/w/landing",
    registered_by: "wido",
    registered_at: at(-600),
    owner: owner(),
    summary: "landing lane /w/landing: idle",
    paused: false,
    agent_alive: false,
    queue: [],
    running_proof: null,
    last_proof: null,
    last_push: null,
    problems: [],
    ...over,
  };
}

function board(over: Partial<BoardPayload> = {}): BoardPayload {
  return {
    readable: true,
    bridge: "live",
    seats: [],
    lines: [],
    lane: lane(),
    titles: {},
    questions: [],
    questionsProblem: "",
    ...over,
  };
}

function fleetRead(over: Partial<Page> = {}): FleetReading {
  return { state: "read", page: page(over) };
}

function boardRead(over: Partial<BoardPayload> = {}): BoardReading {
  return { state: "read", board: board(over) };
}

function needs(fleet: FleetReading, read: BoardReading) {
  return needsOf(fleet, read, now);
}

describe("the verdict", () => {
  it("is All good on this computer only when every read succeeded and nothing needs you", () => {
    const verdict = verdictOf(fleetRead(), boardRead(), [], now);

    expect(verdict.words).toBe("All good on this computer");
    expect(verdict.tone).toBe("ok");
    expect(SCOPE).toContain("on this computer");
    expect(SCOPE).toContain("Questions asked on other computers are not checked");
  });

  it("counts the seats working, the work waiting to land, and says when it was read", () => {
    const fleet = fleetRead({ machines: [machine({ working: [working()] }), machine({ machine: "m1g" })] });
    const read = boardRead({ lane: lane({ queue: [entry(), entry({ goal: "goal-b" }), entry({ goal: "goal-c", state: "landed" })] }) });

    const verdict = verdictOf(fleet, read, [], now);

    expect(verdict.facts).toEqual(["1 seat working", "2 waiting to land", `updated ${minuteTime(at(0))}`]);
  });

  it("says how many things need you when any do", () => {
    const fleet = fleetRead();
    const read = boardRead({ lane: lane({ owner: owner({ state: "stopped", stopped_by: "m1e" }), paused: true }) });

    const verdict = verdictOf(fleet, read, needs(fleet, read), now);

    expect(verdict.words).toBe("1 thing needs you");
    expect(verdict.tone).toBe("attention");
  });

  it("says which section it could not read, rather than All good", () => {
    const failed: BoardReading = { state: "failed", message: "board answered 500" };

    const verdict = verdictOf(fleetRead(), failed, [], now);

    expect(verdict.words).toBe("Can't read this computer's board, the landing lane and questions");
    expect(verdict.tone).toBe("unread");
  });

  it("names a lane whose records could not all be read, and questions it could not read", () => {
    const read = boardRead({ lane: lane({ problems: ["the queue can't be read: permission denied"] }), questionsProblem: "this server reads no questions" });

    expect(unreadSections(fleetRead(), read)).toEqual(["the landing lane", "questions"]);
    expect(verdictOf(fleetRead(), read, [], now).words).toBe("Can't read the landing lane and questions");
  });

  it("names the fleet when any of its problem fields is set, the ledger and this computer's health among them", () => {
    const fleet: FleetReading = {
      state: "read",
      page: page({
        claims: { tip: "", unavailable: "the accepted ledger could not be read" },
        this: seat({ armed: "unreadable", health: { state: "", observedAt: "", problem: "malformed", roles: [] } }),
      }),
      problem: "fleet answered 500",
    };
    const read = boardRead({ readable: false, reason: "registry: permission denied" });

    expect(unreadSections(fleet, read)).toEqual(["the fleet", "this computer's board"]);
    expect(unreadOf(fleet, read).fleet).toEqual(["fleet answered 500", "the accepted ledger could not be read", "malformed"]);
    expect(verdictOf(fleet, read, [], now).words).toBe("Can't read the fleet and this computer's board");
  });

  it("keeps each section's unread lines in one place: the fleet, the board, the lane and the questions", () => {
    const read = boardRead({
      unreadable: ["the goal ledger can't be read: broken"],
      lane: lane({ problems: ["the running proof can't be read: permission denied"] }),
      questionsProblem: "1 of its records can't be read: q-2.json: torn",
    });

    expect(unreadOf(fleetRead({ copy: { ...page().copy, problem: "presence fetch: the remote refused" } }), read)).toEqual({
      fleet: ["presence fetch: the remote refused"],
      board: ["the goal ledger can't be read: broken"],
      lane: ["the running proof can't be read: permission denied"],
      questions: ["1 of its records can't be read: q-2.json: torn"],
    });
    expect(unreadOf({ state: "loading" }, { state: "loading" })).toEqual({ fleet: [], board: [], lane: [], questions: [] });
  });

  it("never says All good over a presence copy it could not read", () => {
    const fleet = fleetRead({ copy: { ...page().copy, problem: "presence ref list: exit status 128" } });

    expect(verdictOf(fleet, boardRead(), [], now).words).toBe("Can't read the fleet");
  });

  it("counts every failed read the fleet carries, and neither a goal's box nor the Partner's metadata write", () => {
    const cases: Partial<Page>[] = [
      { this: seat({ publicationProblem: "publication state unreadable" }) },
      { this: seat({ runningProblem: "chain unread: a.json" }) },
      { machines: [machine({ this: true, workingProblem: "chain unread: b.json" })] },
    ];
    for (const over of cases) {
      expect(unreadSections(fleetRead(over), boardRead())).toEqual(["the fleet"]);
    }
    // A box that could not be projected is a fact about its goal, said where
    // the goal's work is opened, not a read of this page that failed.
    const torn = working({ box: { attempts: null, attemptLimit: null, reservedMinutes: null, reservedMinutesLimit: null, problem: "the claim episode is malformed" } });
    expect(unreadSections(fleetRead({ machines: [machine({ working: [torn] })] }), boardRead())).toEqual([]);
    const metadata = fleetRead({ copy: { ...page().copy, metadataProblem: "the Partner's presence metadata could not be written" } });
    expect(unreadSections(metadata, boardRead())).toEqual([]);
  });

  it("keeps saying what needs you beside what it could not read", () => {
    const fleet = fleetRead();
    const read = boardRead({ lane: lane({ owner: owner({ state: "stopped" }), paused: true, problems: ["the push record can't be read: x"] }) });

    const verdict = verdictOf(fleet, read, needs(fleet, read), now);

    expect(verdict.words).toBe("Can't read the landing lane · 1 thing needs you");
  });

  it("names a lane whose registration could not be read, never reading it as no lane", () => {
    const unreadable = lane({
      root: null,
      registered_by: null,
      registered_at: null,
      owner: owner({ state: "unready" }),
      summary: "this computer's landing lane record can't be read (permission denied); metasystem landing set replaces it",
      queue: [],
      problems: ["the lane's registration can't be read: permission denied"],
    });
    const read = boardRead({ lane: unreadable });

    expect(unreadSections(fleetRead(), read)).toEqual(["the landing lane"]);
    expect(verdictOf(fleetRead(), read, needs(fleetRead(), read), now)).toEqual({
      words: "Can't read the landing lane",
      tone: "unread",
      facts: ["0 seats working", `updated ${minuteTime(at(0))}`],
    });
    // A lane nobody could read is not a lane that cannot run.
    expect(needs(fleetRead(), read)).toEqual([]);
  });

  it("names this computer's board when parts of it could not be read", () => {
    const read = boardRead({ unreadable: ["m1d: seat directory unreadable: not a directory"] });

    expect(unreadSections(fleetRead(), read)).toEqual(["this computer's board"]);
  });

  it("reads as reading while nothing has answered yet", () => {
    expect(verdictOf({ state: "loading" }, { state: "loading" }, [], now)).toEqual({ words: "Reading…", tone: "reading", facts: [] });
  });
});

describe("what needs you", () => {
  it("is nothing on a calm computer", () => {
    expect(needs(fleetRead(), boardRead())).toEqual([]);
  });

  it("names a paused lane, who paused it and since when, and what resumes it", () => {
    const read = boardRead({
      lane: lane({ owner: owner({ state: "stopped", stopped_by: "m1e", since: at(-80), stopped_because: "maintenance" }), paused: true }),
    });

    const [item] = needs(fleetRead(), read);

    expect(item.words).toBe(`The landing lane is paused by m1e since ${minuteTime(at(-80))}: maintenance.`);
    expect(item.todo).toBe("It lands nothing until someone resumes it with landing start at a terminal.");
    expect(item.todo).not.toContain("metasystem landing start");
  });

  it("names a lane that cannot run and the lane's own fix", () => {
    const read = boardRead({
      lane: lane({ owner: owner({ state: "unready", last_exit: "the landing checkout has local changes", retry_hint: "commit or discard them, then run landing start" }) }),
    });

    const [item] = needs(fleetRead(), read);

    expect(item.words).toBe("The landing lane can't run: the landing checkout has local changes.");
    expect(item.todo).toBe("To fix it: commit or discard them, then run landing start");
  });

  it("names a branch that came back, by its title, with why and the goal to open", () => {
    const read = boardRead({
      titles: { "plain-lane": "Plain lane landing." },
      lane: lane({ queue: [entry({ goal: "plain-lane", state: "returned", reason: "the full test run is red", returned_at: at(-10) })] }),
    });

    const [item] = needs(fleetRead(), read);

    expect(item.words).toBe("“Plain lane landing.” came back: the full test run is red.");
    expect(item.goal).toBe("plain-lane");
  });

  it("counts a return as history once its goal was handed in again", () => {
    const read = boardRead({
      lane: lane({
        queue: [
          entry({ goal: "plain-lane", state: "returned", reason: "red", returned_at: at(-30) }),
          entry({ goal: "plain-lane", sha: "fedcba", state: "waiting", at: at(-5) }),
        ],
      }),
    });

    expect(needs(fleetRead(), read).filter((one) => one.words.includes("came back"))).toEqual([]);
  });

  it("counts a return as history once its goal is done or abandoned", () => {
    const read = boardRead({
      ended: { "lane-check-red": "abandoned", "landed-goal": "done" },
      lane: lane({
        queue: [
          entry({ goal: "lane-check-red", state: "returned", reason: "red", returned_at: at(-30) }),
          entry({ goal: "landed-goal", sha: "fedcba", state: "returned", reason: "conflict", returned_at: at(-20) }),
          entry({ goal: "plain-lane", sha: "abc123", state: "returned", reason: "red", returned_at: at(-10) }),
        ],
      }),
    });

    expect(needs(fleetRead(), read).filter((one) => one.words.includes("came back")).map((one) => one.goal)).toEqual(["plain-lane"]);
  });

  it("names a red proof with its recorded reason, until a return answers it", () => {
    const red = { tree: "t1", commit: "c0ffee1234567", result: "red", log: "/l/p.log", at: at(-20), reason: "app-standard failed" };
    const read = boardRead({ lane: lane({ last_proof: red }) });

    const [item] = needs(fleetRead(), read);
    expect(item.words).toBe("The landing lane's last proof is red: app-standard failed.");

    const answered = boardRead({
      lane: lane({ last_proof: red, queue: [entry({ state: "returned", reason: "red", returned_at: at(-15) })] }),
    });
    expect(needs(fleetRead(), answered).map((one) => one.words)).toEqual(["“goal-a” came back: red."]);

    const without = boardRead({ lane: lane({ last_proof: { ...red, reason: undefined } }) });
    expect(needs(fleetRead(), without)[0].words).toBe("The landing lane's last proof is red: the proof command failed.");
  });

  it("leaves a red proof to the landing agent while it runs, or while a newer proof runs", () => {
    const red = { tree: "t1", commit: "c1", result: "red", log: "/l", at: at(-20) };
    const alive = boardRead({ lane: lane({ last_proof: red, agent_alive: true, owner: owner({ state: "running" }) }) });
    const proving = boardRead({ lane: lane({ last_proof: red, running_proof: { tree: "t2", since: at(-2), attempt: "a-8", state: "running" } }) });

    expect(needs(fleetRead(), alive)).toEqual([]);
    expect(needs(fleetRead(), proving)).toEqual([]);
  });

  it("names this checkout's open questions, with who asks and where to answer", () => {
    const read = boardRead({
      titles: { "plain-lane": "Plain lane landing." },
      questions: [
        { id: "q-1", goal: "plain-lane", machine: "m1f", question: "Land slice 2 now?", openedAt: at(-7) },
        { id: "q-2", goal: "", about: "lane", machine: "", question: "Return the conflicting branch?", openedAt: at(-3) },
      ],
    });

    const items = needs(fleetRead(), read);

    expect(items.map((one) => one.words)).toEqual([
      "A seat asks about the landing lane: Return the conflicting branch?",
      "m1f asks about “Plain lane landing.”: Land slice 2 now?",
    ]);
    expect(items.every((one) => one.answer)).toBe(true);
  });

  it("names this computer's steward when its health record says it is not running", () => {
    const fleet = fleetRead({
      this: seat({
        armed: "not armed",
        health: {
          state: "unhealthy",
          observedAt: at(-2),
          problem: "",
          roles: [
            { role: "steward-runner", status: "dead", reason: "runner pid 41 is gone" },
            { role: "repo-watcher", status: "alive", reason: "watching" },
          ],
        },
      }),
    });

    const [item] = needs(fleet, boardRead());

    expect(item.words).toBe("This computer's steward is not running.");
    expect(item.todo).toBe("It starts again with system start at a terminal on this computer.");
  });

  it("names any other unhealthy role of this computer by its own reason", () => {
    const fleet = fleetRead({
      this: seat({
        health: {
          state: "unhealthy",
          observedAt: at(-2),
          problem: "",
          roles: [
            { role: "steward-runner", status: "alive", reason: "runner alive" },
            { role: "seat-presence", status: "dead", reason: "presence not published since 09:10" },
          ],
        },
      }),
    });

    expect(needs(fleet, boardRead())[0].words).toBe("This computer's health check failed: presence not published since 09:10.");
  });

  it("names a health record the steward stopped writing", () => {
    const fleet = fleetRead({ this: seat({ armed: "stale" }) });

    expect(needs(fleet, boardRead())[0].words).toBe(`This computer's steward has not recorded its health since ${minuteTime(at(-3))}.`);
  });

  it("says nothing about a checkout that was never armed", () => {
    expect(needs(fleetRead({ this: seat({ armed: "not armed", health: null }) }), boardRead())).toEqual([]);
  });

  it("says a silent machine once, with every goal it holds and the remedy once", () => {
    const one = held({ goal: "a", title: "First goal.", machine: "m2a", standing: "unreachable", since: at(-300), flag: "held by m2a, unreachable" });
    const two = held({ goal: "b", title: "", machine: "m2a", standing: "unreachable", since: at(-300), flag: "held by m2a, unreachable" });
    const other = held({ goal: "c", title: "Third goal.", machine: "m0b", standing: "unknown", since: "", flag: "held by m0b, which has published no presence" });

    const items = needs(fleetRead({ needsYou: [one, two, other] }), boardRead());

    expect(items.map((item) => item.words)).toEqual([
      `2 goals are held by m2a, unreachable since ${minuteTime(at(-300))}.`,
      "“Third goal.” is held by m0b, which has published no presence.",
    ]);
    expect(items[0].goals).toEqual([
      { id: "a", title: "First goal." },
      { id: "b", title: "b" },
    ]);
    expect(items[0].goal).toBe("");
    expect(items[0].todo).toBe(NEEDS_YOU_REMEDY);
    expect(items[1].goal).toBe("c");
  });

  it("keeps the goals held by a silent machine, with what a terminal does about them", () => {
    const silent = held({ goal: "tests-parallel", title: "Run the suite in parallel.", machine: "m1c", standing: "unreachable", since: at(-300), flag: "held by m1c, unreachable" });

    const [item] = needs(fleetRead({ needsYou: [silent] }), boardRead());

    expect(item.words).toBe(`“Run the suite in parallel.” is held by m1c, unreachable since ${minuteTime(at(-300))}.`);
    expect(item.goal).toBe("tests-parallel");
    expect(item.todo).toContain("goal steal");
  });

  it("lists every source in one list, newest first, the undated last", () => {
    const fleet = fleetRead({ this: seat({ armed: "stale", health: { ...seat().health!, observedAt: at(-90) } }) });
    const read = boardRead({
      lane: lane({
        owner: owner({ state: "stopped", stopped_by: "m1e", since: at(-60) }),
        paused: true,
        queue: [entry({ state: "returned", reason: "red", returned_at: at(-5) })],
      }),
      questions: [{ id: "q", goal: "", about: "machine", machine: "m1g", question: "Restart?", openedAt: at(-30) }],
    });
    const unready = boardRead({ lane: lane({ owner: owner({ state: "unready", last_exit: "x" }) }) });

    expect(needs(fleet, read).map((one) => one.key)).toEqual(["returned:goal-a:1234567890abcdef", "question:q", "lane:paused", "health"]);
    expect(needs(fleet, unready).map((one) => one.key)).toEqual(["health", "lane:unready"]);
  });
});

describe("the Doing column", () => {
  const held1 = held({ goal: "one-folder" });

  function seatOf(goals: BoardSeat["goals"], machineName = "m1f"): BoardSeat {
    return { machine: machineName, installation: "/w/m1f/metasystem", goals };
  }

  it("says the work in hand and for how long, from the seat's records", () => {
    expect(doingOf(machine({ working: [working()] }), undefined, [], now)).toEqual({ words: "building · 42 min", active: true, source: "jobs" });
  });

  it("names the build a seat launched rather than the seat session itself", () => {
    const session = working({ goal: "one-folder", phase: { role: "working", round: 0, roundLimit: null }, job: { ...working().job, id: "s", role: "working", startedAt: at(-90) } });
    const review = working({ phase: { role: "review", round: 3, roundLimit: 20 }, job: { ...working().job, id: "r", role: "review", startedAt: at(-9) } });

    expect(doingOf(machine({ working: [session, review] }), undefined, [], now).words).toBe("reviewing · round 3 of 20 · 9 min");
    expect(doingOf(machine({ working: [session] }), undefined, [], now).words).toBe("working · 90 min");
  });

  it("counts the other work in hand", () => {
    const other = working({ goal: "two", job: { ...working().job, id: "l-2" } });

    expect(doingOf(machine({ working: [working(), other] }), undefined, [], now).words).toBe("building · 42 min · and 1 more");
  });

  it("says a reservation is waiting to start rather than counting minutes", () => {
    const pending = working({ job: { ...working().job, status: "pending" } });

    expect(doingOf(machine({ working: [pending] }), undefined, [], now)).toEqual({ words: "building · waiting to start", active: false, source: "jobs" });
  });

  it("takes this computer's board card for a goal the machine holds, with the progress it records", () => {
    const proving = seatOf([{ goal: "one-folder", stage: "unit-proof", proof: { attempt: "a", done: 120, planned: 189 }, since: at(-12) }]);
    const reviewing = seatOf([{ goal: "one-folder", stage: "review", round: { n: 2, max: 3 }, since: at(-5) }]);

    expect(doingOf(machine({ holds: [held1] }), proving, [], now)).toEqual({ words: "proving · 120 of 189 · 12 min", active: true, source: "board" });
    expect(doingOf(machine({ holds: [held1] }), reviewing, [], now).words).toBe("reviewing · round 2 of 3 · 5 min");
  });

  it("drops a stale card: one for a goal the machine no longer holds, or one the board cannot believe", () => {
    const moved = seatOf([{ goal: "one-folder-deployed", stage: "build", since: at(-50), unknown: "claim moved to m1f" }]);
    const notHeld = seatOf([{ goal: "someone-elses", stage: "build", since: at(-50) }]);

    expect(doingOf(machine({ machine: "m1g", holds: [] }), moved, [], now).words).toBe("idle");
    expect(doingOf(machine({ machine: "m1g", holds: [] }), notHeld, [], now).words).toBe("idle");
  });

  it("says a machine whose work waits in the lane is waiting to land", () => {
    const joined = seatOf([{ goal: "one-folder", stage: "joined", since: at(-4) }]);

    expect(doingOf(machine({ holds: [held1] }), joined, [], now)).toEqual({ words: "waiting to land · 4 min", active: false, source: "board" });
    expect(doingOf(machine({ machine: "m1g", holds: [] }), undefined, [entry()], now)).toEqual({ words: "waiting to land", active: false, source: "lane" });
  });

  it("prefers work in hand to a card that only waits", () => {
    const joined = seatOf([{ goal: "one-folder", stage: "joined", since: at(-4) }]);

    expect(doingOf(machine({ holds: [held1], working: [working()] }), joined, [], now).words).toBe("building · 42 min");
  });

  it("says idle for a reachable machine with nothing in hand, and a dash for one nobody hears", () => {
    expect(doingOf(machine(), undefined, [], now)).toEqual({ words: "idle", active: false, source: "none" });
    expect(doingOf(machine({ standing: "unreachable", working: [working()] }), undefined, [], now)).toEqual({ words: "—", active: false, source: "none" });
  });

  it("carries the jobs reader's own problem rather than idle", () => {
    expect(doingOf(machine({ this: true, workingProblem: "chain unread: torn.json" }), undefined, [], now)).toEqual({
      words: "chain unread: torn.json",
      active: false,
      source: "problem",
    });
  });

  it("answers an older machine from the chain it publishes", () => {
    const older = machine({ running: { job: "j-9", role: "critic", round: 1, goal: "g", startedAt: at(-30) } });

    expect(doingOf(older, undefined, [], now)).toEqual({ words: "reviewing · round 1 · 30 min", active: true, source: "jobs" });
  });
});

describe("the landing lane", () => {
  it("is Running, Paused or Needs attention, one word each", () => {
    expect(laneState(lane()).word).toBe("Running");
    expect(laneState(lane({ owner: owner({ state: "running" }), agent_alive: true })).word).toBe("Running");
    expect(laneState(lane({ owner: owner({ state: "stopped" }), paused: true })).word).toBe("Paused");
    expect(laneState(lane({ owner: owner({ state: "unready" }) })).word).toBe("Needs attention");
  });

  it("lists what waits by title, seat and age, and what proves by how long it has run", () => {
    const lists = laneLists(
      lane({
        queue: [entry({ goal: "seat-path" })],
        running_proof: { tree: "abcdef1234567", commit: "c1", since: at(-6), attempt: "a-7", log: "/l/a-7.log", state: "running" },
      }),
      { "seat-path": "Seat path lands without help." },
      now,
    );

    expect(lists.waiting.map((item) => item.words)).toEqual(["Seat path lands without help. · m1g · 4 min"]);
    expect(lists.proving?.words).toBe("started 6 min ago");
  });

  it("names what the running proof holds by title, and lists it there rather than under Waiting", () => {
    const lists = laneLists(
      lane({
        queue: [entry({ goal: "seat-path" }), entry({ goal: "plain-lane", seat: "m1f" }), entry({ goal: "later", at: at(-1) })],
        running_proof: { tree: "t", commit: "c", since: at(-6), attempt: "a-9", state: "running", goals: ["seat-path", "plain-lane"] },
      }),
      { "seat-path": "Seat path lands without help.", "plain-lane": "Plain lane landing." },
      now,
    );

    expect(lists.proving?.words).toBe("Seat path lands without help. and Plain lane landing. · started 6 min ago");
    expect(lists.proving?.goals).toEqual([
      { id: "seat-path", title: "Seat path lands without help." },
      { id: "plain-lane", title: "Plain lane landing." },
    ]);
    expect(lists.waiting.map((item) => item.entry.goal)).toEqual(["later"]);
  });

  it("keeps today's words for a proof that holds none of the queue", () => {
    const lists = laneLists(lane({ queue: [entry()], running_proof: { tree: "t", since: at(-6), attempt: "a", state: "running", goals: [] } }), {}, now);

    expect(lists.proving?.words).toBe("started 6 min ago");
    expect(lists.waiting).toHaveLength(1);
  });

  it("says a proof that died without a result", () => {
    const lists = laneLists(lane({ running_proof: { tree: "t", since: at(-6), attempt: "a-7", state: "died" } }), {}, now);

    expect(lists.proving?.words).toBe("stopped without a result; the next proof runs it again");
  });

  it("lists what landed today, newest first, by the push's time and the delivered sentence, else the title", () => {
    const lists = laneLists(
      lane({
        queue: [
          entry({ goal: "a", state: "landed", landed_at: at(-200), delivered: "The fleet card can land work now." }),
          entry({ goal: "b", state: "landed", landed_at: at(-6) }),
          entry({ goal: "c", state: "landed", landed_at: "2026-10-01T09:00:00Z" }),
          entry({ goal: "d", state: "landed" }),
        ],
      }),
      { b: "Stuck agents ask on Telegram." },
      now,
    );

    expect(lists.landed.map((item) => [item.at, item.words])).toEqual([
      [at(-6), "Stuck agents ask on Telegram."],
      [at(-200), "The fleet card can land work now."],
    ]);
  });

  it("lists what came back with why, the returns since handed in again as history", () => {
    const lists = laneLists(
      lane({
        queue: [
          entry({ goal: "a", state: "returned", reason: "the full test run is red", returned_at: at(-30) }),
          entry({ goal: "b", state: "returned", reason: "merge conflict", returned_at: at(-20) }),
          entry({ goal: "b", sha: "ff", state: "waiting", at: at(-10) }),
          entry({ goal: "old", state: "returned", reason: "red", returned_at: "2026-09-20T09:00:00Z" }),
          entry({ goal: "old", sha: "ee", state: "landed" }),
        ],
      }),
      { a: "Lane check red." },
      now,
    );

    expect(lists.cameBack.map((item) => [item.title, item.words, item.again])).toEqual([
      ["b", "merge conflict", true],
      ["Lane check red.", "the full test run is red", false],
    ]);
  });
});

describe("the reading in flight", () => {
  it("leaves out what has not been read rather than counting it", () => {
    const fleet = fleetRead();
    const loading: BoardReading = { state: "loading" };

    expect(verdictOf(fleet, loading, [], now).facts).toEqual(["0 seats working", `updated ${minuteTime(at(0))}`]);
    expect(needs(fleet, loading)).toEqual([]);
  });
});

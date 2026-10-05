import { describe, expect, it } from "vitest";

import { dateAndTime, minuteTime } from "../backlog/format";
import type { BoardPayload, BoardSeat, Held, Lane, LaneEntry, LaneOwner, Machine, Page, ThisSeat, Working } from "./api";
import {
  doingOf,
  laneActOf,
  laneCounts,
  laneLists,
  laneState,
  needsOf,
  QUESTIONS_SCOPE,
  seenOf,
  stopOffer,
  unreadOf,
  unreadSections,
  verdictOf,
  type BoardReading,
  type FleetReading,
  type Need,
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
    // What the verdict checked stays beside the counts, in three words (R2-8).
    expect(QUESTIONS_SCOPE).toBe("questions: this checkout only");
  });

  it("counts the seats working, the work waiting to land, and says when it was read", () => {
    const fleet = fleetRead({ machines: [machine({ working: [working()] }), machine({ machine: "m1g" })] });
    const read = boardRead({ lane: lane({ queue: [entry(), entry({ goal: "goal-b" }), entry({ goal: "goal-c", state: "landed" })] }) });

    const verdict = verdictOf(fleet, read, [], now);

    expect(verdict.facts).toEqual(["1 seat working", "2 waiting to land"]);
    expect(verdict.updated).toBe(`updated ${minuteTime(at(0))}`);
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
      facts: ["0 seats working"],
      updated: `updated ${minuteTime(at(0))}`,
    });
    // A lane nobody could read is not a lane that cannot run.
    expect(needs(fleetRead(), read)).toEqual([]);
  });

  it("names this computer's board when parts of it could not be read", () => {
    const read = boardRead({ unreadable: ["m1d: seat directory unreadable: not a directory"] });

    expect(unreadSections(fleetRead(), read)).toEqual(["this computer's board"]);
  });

  it("reads as reading while nothing has answered yet", () => {
    expect(verdictOf({ state: "loading" }, { state: "loading" }, [], now)).toEqual({ words: "Reading…", tone: "reading", facts: [], updated: "" });
  });
});

/** The one act or the commands an item offers, counted: the rule is exactly one. */
function offers(item: Need): number {
  return [item.act !== null, item.command !== "", item.goals.length > 0].filter(Boolean).length;
}

/** A health record of this computer, with the roles given and alive ones beside them. */
function health(state: string, roles: { role: string; status: string; reason: string }[], problem = "") {
  return seat({ health: { state, observedAt: at(-2), problem, roles } });
}

describe("what needs you", () => {
  it("is nothing on a calm computer", () => {
    expect(needs(fleetRead(), boardRead())).toEqual([]);
  });

  it("offers exactly one act or exactly one command on every item, whatever its source", () => {
    const silent = [
      held({ goal: "a", title: "First goal.", machine: "m2a", standing: "unreachable", since: at(-300), flag: "held by m2a, unreachable" }),
      held({ goal: "b", title: "Second goal.", machine: "m2a", standing: "unreachable", since: at(-300), flag: "held by m2a, unreachable" }),
      held({ goal: "c", title: "Third goal.", machine: "m0b", standing: "unknown", since: "", flag: "held by m0b, which has published no presence" }),
    ];
    const fleet = fleetRead({
      needsYou: silent,
      machines: [machine({ machine: "ui", this: true, holds: [] }), machine({ holds: [held()] })],
      this: health("unhealthy", [{ role: "steward-runner", status: "dead", reason: "runner is gone" }]),
    });
    const red = { tree: "t1", commit: "c1", result: "red", log: "/l/a-9.log", at: at(-20), attempt: "a-9", reason: "the proving command exited 1" };
    const read = boardRead({
      seats: [{ machine: "m1f", installation: "/w/m1f/metasystem", goals: [{ goal: "one-folder", stage: "revise", since: at(-90), lastProgressAt: at(-68), unknown: "stalled" }] }],
      lane: lane({
        owner: owner({ state: "stopped", stopped_by: "wido", since: at(-80), stopped_because: "the batch VM is down" }),
        paused: true,
        last_proof: red,
        queue: [entry({ goal: "plain-lane", state: "returned", reason: "red", returned_at: at(-30) })],
      }),
      questions: [{ id: "q-1", goal: "", about: "lane", machine: "m1f", question: "Land slice 2 now?", openedAt: at(-7) }],
    });
    const unready = boardRead({ lane: lane({ owner: owner({ state: "unready", last_exit: "x", retry_hint: "y" }), last_proof: { ...red, attempt: undefined } }) });

    const items = [...needs(fleet, read), ...needs(fleetRead(), unready)];

    expect(items.map((item) => item.key).sort()).toEqual(
      ["health", "held:m0b:c", "held:m2a", "lane:paused", "lane:unready", "proof:a-9", `proof:${at(-20)}`, "question:q-1", "returned:plain-lane:1234567890abcdef", "stuck:m1f:one-folder"].sort(),
    );
    for (const item of items) {
      expect({ key: item.key, offers: offers(item) }).toEqual({ key: item.key, offers: 1 });
    }
  });

  it("names a paused lane, who paused it and since when, with Resume, the act that resumes it", () => {
    const read = boardRead({
      lane: lane({ owner: owner({ state: "stopped", stopped_by: "m1e", since: at(-80), stopped_because: "maintenance" }), paused: true }),
    });

    const [item] = needs(fleetRead(), read);

    expect(item.words).toBe(`The landing lane is paused by m1e since ${minuteTime(at(-80))}: maintenance.`);
    expect(item.act).toEqual({ kind: "resume" });
    expect(item.command).toBe("");
    expect(item.impact).toBe("");
  });

  it("keeps the command that resumes a paused lane where part of the lane could not be read, as Land now is withheld there", () => {
    const read = boardRead({
      lane: lane({ owner: owner({ state: "stopped", stopped_by: "m1e", since: at(-80) }), paused: true, problems: ["the running proof's record could not be read"] }),
    });

    const [item] = needs(fleetRead(), read);

    expect(item.key).toBe("lane:paused");
    expect(item.command).toBe("metasystem landing start");
    expect(item.act).toBeNull();
  });

  it("names a lane that cannot run, with landing status to type and the lane's own hint as the quiet line", () => {
    const read = boardRead({
      lane: lane({ owner: owner({ state: "unready", last_exit: "the landing checkout has local changes", retry_hint: "commit or discard them, then run landing start" }) }),
    });

    const [item] = needs(fleetRead(), read);

    expect(item.words).toBe("The landing lane can't run: the landing checkout has local changes.");
    expect(item.command).toBe("metasystem landing status");
    expect(item.note).toBe("commit or discard them, then run landing start");
    expect(item.act).toBeNull();
  });

  it("names a branch that came back by its title, the reason's first sentence under it, and the goal to open", () => {
    const read = boardRead({
      titles: { "plain-lane": "Plain lane landing." },
      lane: lane({
        queue: [entry({ goal: "plain-lane", state: "returned", reason: "the full test run is red. Rebase onto main and hand it in again.", returned_at: at(-10) })],
      }),
    });

    const [item] = needs(fleetRead(), read);

    expect(item.words).toBe("“Plain lane landing.” came back.");
    expect(item.note).toBe("the full test run is red.");
    expect(item.act).toEqual({ kind: "goal", goal: "plain-lane" });
    expect(item.command).toBe("");
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

    expect(needs(fleetRead(), read).filter((one) => one.words.includes("came back")).map((one) => one.act)).toEqual([{ kind: "goal", goal: "plain-lane" }]);
  });

  it("names a red proof with its recorded reason, until a return answers it", () => {
    const red = { tree: "t1", commit: "c0ffee1234567", result: "red", log: "/l/p.log", at: at(-20), reason: "app-standard failed" };
    const read = boardRead({ lane: lane({ last_proof: red }) });

    const [item] = needs(fleetRead(), read);
    expect(item.words).toBe("The landing lane's last proof is red: app-standard failed.");

    const answered = boardRead({
      lane: lane({ last_proof: red, queue: [entry({ state: "returned", reason: "red", returned_at: at(-15) })] }),
    });
    expect(needs(fleetRead(), answered).map((one) => one.words)).toEqual(["“goal-a” came back."]);

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

  it("gives a red proof the Open log act, by the attempt the record names, and landing status where it names none", () => {
    const red = { tree: "t1", commit: "c1", result: "red", log: "/l/a-9.log", at: at(-20), attempt: "a-9", reason: "the proving command exited 1" };

    const [item] = needs(fleetRead(), boardRead({ lane: lane({ last_proof: red }) }));
    expect(item.words).toBe("The landing lane's last proof is red: the proving command exited 1.");
    expect(item.act).toEqual({ kind: "log", href: "/api/fleet/proof-logs/a-9" });
    expect(item.command).toBe("");

    // An older record names no attempt, so there is no log to open; the
    // lane's status names the log instead.
    const [older] = needs(fleetRead(), boardRead({ lane: lane({ last_proof: { ...red, attempt: undefined } }) }));
    expect(older.act).toBeNull();
    expect(older.command).toBe("metasystem landing status");
  });

  it("names this checkout's open questions, with who asks and Answer", () => {
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
    expect(items.map((one) => one.act)).toEqual([{ kind: "answer" }, { kind: "answer" }]);
  });

  it("names this computer's steward when its health record says it is not running, with system start", () => {
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

    const items = needs(fleet, boardRead());
    expect(items).toHaveLength(1);
    const [item] = items;

    expect(item.words).toBe("This computer's steward is not running.");
    expect(item.command).toBe("metasystem system start");
    expect(item.details).toEqual([]);
  });

  it.each([
    ["retro-debt", "RETRO DEBT awaits a receipt after arc-goal:verbs-match-intent:602TW9BXPRBJ04Q9NZGSYSHT8K-m1e-c6925449"],
    ["stop-hook-duration", "the last Stop took 21s of the 60s budget on m1e; the threshold is 15s"],
    ["context-budget", "614 thousand tokens this call, trigger 105, proof line 150, proof maximum 200, ceiling 250; over the proof maximum"],
    ["ledger-attention", "the shared ledger moved to 12360c3a01ef 56m ago and is unexamined past 30m"],
    ["trunk-red", "deep validation cadence is overdue at trunk 7d0fc86923afaacb75f4252a81c92a4ae0e82ed9 tree 71e8dd435547ed4d6e19c2ede4c3870fb0cfd816"],
    ["new-health-role", "an engineer's check failed"],
    ["capability-snapshots", ""],
  ])("leaves a failing %s check to the steward, with no item or change to the verdict", (role, reason) => {
    const fleet = fleetRead({ this: health("unhealthy", [...seat().health!.roles, { role, status: "dead", reason }]) });
    const items = needs(fleet, boardRead());
    const verdict = verdictOf(fleet, boardRead(), items, now);

    expect(items).toEqual([]);
    expect(verdict).toMatchObject({ words: "All good on this computer", tone: "ok" });
    if (reason !== "") expect(JSON.stringify({ items, verdict })).not.toContain(reason);
  });

  it("names a health record it could not read, with the reader's reason and system check, and is never All good over it", () => {
    const torn = fleetRead({ this: seat({ armed: "unreadable", health: { state: "", observedAt: "", problem: "the steward's health record is malformed: unexpected end of JSON input", roles: [] } }) });

    const items = needs(torn, boardRead());

    expect(items).toHaveLength(1);
    expect(items[0].words).toBe("This computer's health record could not be read.");
    expect(items[0].details).toEqual(["the steward's health record is malformed: unexpected end of JSON input"]);
    expect(items[0].command).toBe("metasystem system check");
    expect(verdictOf(torn, boardRead(), items, now).words).not.toContain("All good");
  });

  it("names a steward that could not be checked, with system check and no Details", () => {
    const undecided = fleetRead({
      this: health("unknown", [
        { role: "steward-runner", status: "unknown", reason: "runner pid 41 could not be probed" },
        { role: "census-freshness", status: "unknown", reason: "no census success is recorded" },
      ]),
    });

    const items = needs(undecided, boardRead());

    expect(items).toHaveLength(1);
    expect(items[0].words).toBe("This computer's steward could not be checked.");
    expect(items[0].details).toEqual([]);
    expect(items[0].command).toBe("metasystem system check");
    expect(verdictOf(undecided, boardRead(), items, now)).toMatchObject({ words: "1 thing needs you", tone: "attention" });
  });

  it.each(["census-freshness", "new-health-role"])("ignores an undecided %s check, including the aggregate state", (role) => {
    const fleet = fleetRead({ this: health("unknown", [...seat().health!.roles, { role, status: "unknown", reason: "could not decide" }]) });
    const items = needs(fleet, boardRead());
    expect(items).toEqual([]);
    expect(verdictOf(fleet, boardRead(), items, now).words).toBe("All good on this computer");
  });

  it("names a health record the steward stopped writing, with system start", () => {
    const fleet = fleetRead({ this: seat({ armed: "stale" }) });

    const [item] = needs(fleet, boardRead());

    expect(item.words).toBe(`This computer's steward has not recorded its health since ${minuteTime(at(-3))}.`);
    expect(item.command).toBe("metasystem system start");
    expect(item.details).toEqual([]);
    expect(verdictOf(fleet, boardRead(), [item], now).words).toBe("1 thing needs you");
  });

  it("says nothing about a checkout that was never armed", () => {
    expect(needs(fleetRead({ this: seat({ armed: "not armed", health: null }) }), boardRead())).toEqual([]);
  });

  it("says a silent machine once, with every goal it holds and each goal's own take-over command", () => {
    const one = held({ goal: "a", title: "First goal.", machine: "m2a", standing: "unreachable", since: at(-300), flag: "held by m2a, unreachable" });
    const two = held({ goal: "b", title: "", machine: "m2a", standing: "unreachable", since: at(-300), flag: "held by m2a, unreachable" });
    const other = held({ goal: "c", title: "Third goal.", machine: "m0b", standing: "unknown", since: "", flag: "held by m0b, which has published no presence" });

    const items = needs(fleetRead({ needsYou: [one, two, other] }), boardRead());

    expect(items.map((item) => item.words)).toEqual([
      `2 goals are held by m2a, unreachable since ${minuteTime(at(-300))}.`,
      "“Third goal.” is held by m0b, which has published no presence.",
    ]);
    expect(items[0].goals).toEqual([
      { id: "a", title: "First goal.", command: 'metasystem goal claim a --take-over --reason "<why>"' },
      { id: "b", title: "b", command: 'metasystem goal claim b --take-over --reason "<why>"' },
    ]);
    expect(items[0].act).toBeNull();
    expect(items[0].command).toBe("");
    // The person reads what a take-over does before they type it (R-143-m1e).
    expect(items[0].impact).toBe(
      "Taking a goal over moves it to the machine you run this on: m2a stops holding it, and what m2a has not pushed stays on m2a. It can be taken back the same way.",
    );
    expect(items[1].impact).toBe(
      "Taking a goal over moves it to the machine you run this on: m0b stops holding it, and what m0b has not pushed stays on m0b. It can be taken back the same way.",
    );
    expect(items[1].command).toBe('metasystem goal claim c --take-over --reason "<why>"');
    expect(items[1].act).toBeNull();
    // The verb that is not one (FR-02) is said nowhere.
    expect(JSON.stringify(items)).not.toContain("goal steal");
  });

  describe("a stuck seat", () => {
    const holding = fleetRead({ machines: [machine({ machine: "ui", this: true, holds: [] }), machine({ holds: [held()] })] });

    function stuckSeat(unknown: string, stage = "build"): BoardSeat {
      return { machine: "m1f", installation: "/w/m1f/metasystem", goals: [{ goal: "one-folder", stage, since: at(-60), lastProgressAt: at(-30), unknown }] };
    }

    function stuck(unknown: string): BoardReading {
      return boardRead({ seats: [stuckSeat(unknown)] });
    }

    it("is one thing that needs you when its card stalled on a goal the machine holds, with Stop for its machine and what the stop ends", () => {
      const read = stuck("stalled");

      const [item] = needs(holding, read);

      expect(item.words).toBe(`“One folder deployed and evolved.” on m1f has not moved since ${minuteTime(at(-30))} (building).`);
      expect(item.act).toEqual({ kind: "stop", machine: "m1f" });
      expect(item.command).toBe("");
      expect(item.impact).toBe("Stopping m1f ends its seat and every job on it; its steward will not start it again.");
      expect(item.note).toBe("");
      expect(item.at).toBe(at(-30));
      expect(verdictOf(holding, read, needs(holding, read), now).words).toBe("1 thing needs you");
    });

    it("is one too when the process writing its card is gone, and names no pid", () => {
      const [item] = needs(holding, stuck("writer dead (pid 4242)"));

      expect(item.words).toBe(`“One folder deployed and evolved.” on m1f has not moved since ${minuteTime(at(-30))} (the process writing it is gone).`);
      expect(item.words).not.toContain("4242");
      expect(item.act).toEqual({ kind: "stop", machine: "m1f" });
    });

    it("keeps the command at a terminal for the seat serving this page, which gets no Stop", () => {
      const serving = fleetRead({ machines: [machine({ machine: "ui", this: true, holds: [held()] })] });
      const read = boardRead({ seats: [{ ...stuckSeat("stalled"), machine: "ui" }] });

      const [item] = needs(serving, read);

      expect(item.key).toBe("stuck:ui:one-folder");
      expect(item.command).toBe("metasystem machine stop ui");
      expect(item.act).toBeNull();
      expect(item.impact).toBe("Stopping ui ends its seat and every job on it; its steward will not start it again.");
    });

    it("keeps its line under 110 characters, cutting a long title at a word", () => {
      const long = "An adopted repository has one MetaSystem folder, metasystem/, that separates its own…";
      const fleet = fleetRead({ machines: [machine({ holds: [held({ title: long })] })] });

      const [item] = needs(fleet, boardRead({ seats: [stuckSeat("stalled", "revise")] }));

      expect(item.words.length).toBeLessThan(110);
      expect(item.words).toMatch(/^“An adopted repository has one MetaSystem folder[^”]*…” on m1f has not moved since \d\d:\d\d \(revising\)\.$/u);
    });

    it("is dropped as before for a goal the machine no longer holds, and for every other reason a card is not believed", () => {
      const released = fleetRead({ machines: [machine({ holds: [] })] });

      expect(needs(released, stuck("stalled"))).toEqual([]);
      for (const reason of ["owner pid 9 unprobeable", "owner pid 9 reused", "no owner", "claim moved to m1g", "not claimed"]) {
        expect({ reason, needs: needs(holding, stuck(reason)) }).toEqual({ reason, needs: [] });
      }
    });

    it("sorts with the rest by its last progress", () => {
      const read = boardRead({
        seats: [stuckSeat("stalled")],
        lane: lane({ owner: owner({ state: "stopped", stopped_by: "m1e", since: at(-10) }), paused: true }),
      });

      expect(needs(holding, read).map((one) => one.key)).toEqual(["lane:paused", "stuck:m1f:one-folder"]);
    });
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

describe("what the page acts on", () => {
  it("offers Pause on a running lane and Resume on a paused one, never both, and neither while the lane needs attention or could not be read", () => {
    expect(laneActOf(lane({ owner: owner({ state: "running", pid: 4242 }), agent_alive: true }), [])).toBe("pause");
    expect(laneActOf(lane(), [])).toBe("pause");
    expect(laneActOf(lane({ owner: owner({ state: "stopped", stopped_by: "wido" }), paused: true }), [])).toBe("resume");
    expect(laneActOf(lane({ owner: owner({ state: "unready", last_exit: "x" }) }), [])).toBeNull();
    // The Land now rule: any line the lane could not read withholds both.
    expect(laneActOf(lane(), ["the running proof's record could not be read"])).toBeNull();
    expect(laneActOf(lane({ paused: true, owner: owner({ state: "stopped" }) }), ["this server does not report it"])).toBeNull();
  });

  it("offers Stop for a seat of this computer other than this seat, the command for this seat, and nothing for a machine on another computer", () => {
    const seats = board({ seats: [{ machine: "m1f", installation: "/w/m1f/metasystem", goals: [] }, { machine: "ui", installation: "/w/ui/metasystem", goals: [] }] });

    expect(stopOffer(machine({ machine: "m1f" }), seats)).toBe("button");
    expect(stopOffer(machine({ machine: "ui", this: true }), seats)).toBe("command");
    expect(stopOffer(machine({ machine: "m2a" }), seats)).toBeNull();
    expect(stopOffer(machine({ machine: "ui", this: true }), board())).toBeNull();
    expect(stopOffer(machine({ machine: "m1f" }), null)).toBeNull();
  });
});

describe("the Seen column", () => {
  it("says how long ago a reachable machine was seen, with the standing's reason as its title", () => {
    const seen = seenOf(machine({ seen: at(-6), reason: "presence published 6 min ago" }), now);

    expect(seen).toMatchObject({ words: "6 min ago", tone: "" });
    expect(seen.title).toContain("presence published 6 min ago");
  });

  it("says unreachable and for how long, in the alarm tone", () => {
    const silent = machine({ standing: "unreachable", seen: at(-3 * 24 * 60), since: at(-3 * 24 * 60 + 30), reason: "no presence for 3 d, past 30 min" });

    expect(seenOf(silent, now)).toMatchObject({ words: "unreachable · 3 d", tone: "bad" });
    expect(seenOf(silent, now).title).toContain("no presence for 3 d, past 30 min");
    // When this seat first saw the silence is kept, in the title.
    expect(seenOf(silent, now).title).toContain(`unreachable since ${dateAndTime(at(-3 * 24 * 60 + 30))}`);
  });

  it("says each unknown standing in its own visible words: no presence, presence unreadable, a clock too far ahead", () => {
    const none = machine({ standing: "unknown", seen: "", ageSeconds: null, reason: "no presence record" });
    const torn = machine({ standing: "unknown", seen: "", ageSeconds: null, reason: "presence unreadable: invalid character 'x'" });
    const ahead = machine({ standing: "unknown", seen: at(120), ageSeconds: -7200, reason: "clock ahead by 2 h" });

    expect([none, torn, ahead].map((one) => seenOf(one, now).words)).toEqual(["no presence", "presence unreadable", "unknown, clock ahead by 2 h"]);
    expect([none, torn, ahead].map((one) => seenOf(one, now).tone)).toEqual(["warn", "warn", "warn"]);
    expect(seenOf(torn, now).title).toContain("invalid character 'x'");
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

    expect(doingOf(machine({ working: [session, review] }), undefined, [], now).words).toBe("reviewing · round 3 · 9 min");
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

  it.each([
    [1, 20, "round 1"], [17, 20, "round 17"], [18, 20, "round 18 of 20"],
    [20, 20, "round 20 of 20"], [3, null, "round 3"],
  ] as const)("says %s with limit %s in the jobs and board writers as %s", (round, limit, words) => {
    for (const role of ["building", "review", "revise"]) {
      const job = working({ phase: { role, round, roundLimit: limit } });
      const verb = role === "review" ? "reviewing" : role === "revise" ? "revising" : "building";
      expect(doingOf(machine({ working: [job] }), undefined, [], now).words).toBe(`${verb} · ${words} · 42 min`);
    }
    for (const stage of ["review", "revise"] as const) {
      const card = seatOf([{ goal: "one-folder", stage, round: { n: round, max: limit }, since: at(-5) }]);
      expect(doingOf(machine({ holds: [held1] }), card, [], now).words).toBe(`${stage === "review" ? "reviewing" : "revising"} · ${words} · 5 min`);
    }
  });

  it("says reviewing, round 3 for the board card without making its cap a plan", () => {
    const card = seatOf([{ goal: "one-folder", stage: "review", round: { n: 3, max: 20 }, since: at(-5) }]);
    expect(doingOf(machine(), card, [], now).words).toBe("reviewing · round 3 · 5 min");
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

  it("shows landed units on a held idle card and lets a running job win", () => {
    const landed = seatOf([{ goal: "one-folder", stage: "claimed-idle", landed: 3 }]);
    expect(doingOf(machine({ holds: [held1] }), landed, [], now)).toEqual({ words: "3 units landed", active: false, source: "board" });
    expect(doingOf(machine({ holds: [held1], working: [working()] }), landed, [], now).words).toBe("building · 42 min");
    expect(doingOf(machine({ holds: [] }), landed, [], now).words).toBe("idle");
    const empty = seatOf([{ goal: "one-folder", stage: "claimed-idle" }]);
    expect(doingOf(machine({ holds: [held1] }), empty, [], now).words).toBe("idle");
    const one = seatOf([{ goal: "one-folder", stage: "claimed-idle", landed: 1 }]);
    expect(doingOf(machine({ holds: [held1] }), one, [], now).words).toBe("1 unit landed");
  });

  it("prefers work in hand to a card that only waits", () => {
    const joined = seatOf([{ goal: "one-folder", stage: "joined", since: at(-4) }]);

    expect(doingOf(machine({ holds: [held1], working: [working()] }), joined, [], now).words).toBe("building · 42 min");
  });

  it("says idle for a reachable machine with nothing in hand, and a dash for one nobody hears", () => {
    expect(doingOf(machine(), undefined, [], now)).toEqual({ words: "idle", active: false, source: "none" });
    expect(doingOf(machine({ standing: "unreachable", working: [working()] }), undefined, [], now)).toEqual({ words: "—", active: false, source: "none" });
  });

  it("says stalled, the stage and the minutes since its last progress, and does not count it as working", () => {
    const stalled = seatOf([{ goal: "one-folder", stage: "build", since: at(-60), lastProgressAt: at(-30), unknown: "stalled" }]);
    const dead = seatOf([{ goal: "one-folder", stage: "review", since: at(-60), lastProgressAt: at(-12), unknown: "writer dead (pid 4242)" }]);

    // The jobs may still say building: a stuck seat's job runs and makes no progress.
    expect(doingOf(machine({ holds: [held1], working: [working()] }), stalled, [], now)).toEqual({
      words: "stalled · building · 30 min",
      active: false,
      source: "board",
      stalled: true,
    });
    expect(doingOf(machine({ holds: [held1] }), dead, [], now).words).toBe("stalled · reviewing · 12 min");
    const fleet = fleetRead({ machines: [machine({ holds: [held1], working: [working()] })] });
    expect(verdictOf(fleet, boardRead({ seats: [stalled] }), [], now).facts[0]).toBe("0 seats working");
  });

  it("keeps a believed card in hand ahead of a stalled one, and a stalled card of a goal it does not hold says nothing", () => {
    const both = seatOf([
      { goal: "two", stage: "build", since: at(-60), lastProgressAt: at(-30), unknown: "stalled" },
      { goal: "one-folder", stage: "review", round: { n: 2, max: 3 }, since: at(-5) },
    ]);

    expect(doingOf(machine({ holds: [held1, held({ goal: "two" })] }), both, [], now).words).toBe("reviewing · round 2 of 3 · 5 min");
    expect(doingOf(machine({ holds: [] }), both, [], now).words).toBe("idle");
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

  it("counts what waits and what proves for the heading, in one line", () => {
    const two = [entry(), entry({ goal: "goal-b" })];
    const counts = (over: Partial<Lane>) => {
      const read = lane(over);
      return laneCounts(read, laneLists(read, {}, now));
    };

    expect(counts({ queue: two })).toBe("2 waiting · nothing proving");
    expect(counts({ queue: two, running_proof: { tree: "t", since: at(-6), attempt: "a", state: "running", goals: ["goal-b"] } })).toBe("1 waiting · 1 proving");
    expect(counts({ queue: two, running_proof: { tree: "t", since: at(-6), attempt: "a", state: "running" } })).toBe("2 waiting · a proof running");
    expect(counts({ queue: two, running_proof: { tree: "t", since: at(-6), attempt: "a", state: "died" } })).toBe("2 waiting · nothing proving");
    expect(counts({ queue: two, owner: owner({ state: "stopped" }), paused: true })).toBe("2 waiting · lands nothing until resumed");
    expect(counts({})).toBe("0 waiting · nothing proving");
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

    expect(verdictOf(fleet, loading, [], now).facts).toEqual(["0 seats working"]);
    expect(verdictOf(fleet, loading, [], now).updated).toBe(`updated ${minuteTime(at(0))}`);
    expect(needs(fleet, loading)).toEqual([]);
  });
});


describe("a stuck unit", () => {
  const stuck = { step: "r2-s1", kind: "build", launch: "run-r2-s1", minutes: 52, rounds: 2, limit: 45 };
  const unitSeat: BoardSeat = { machine: "m1f", installation: "/f", goals: [{ goal: "one-folder", stage: "build", stuck }] };
  it("a stuck unit is a Needs-you item with its stop command", () => {
    const fleet = fleetRead({ machines: [machine({ holds: [held()] })] });
    const [item] = needs(fleet, boardRead({ seats: [unitSeat] }));
    expect(item.key).toBe("stuck:m1f:one-folder");
    expect(item.words).toContain("build step 52 min");
    expect(item.command).toBe("metasystem work stop j1:run-r2-s1");
    expect(item.act).toBeNull();
    expect(needs(fleetRead({ machines: [machine({ holds: [] })] }), boardRead({ seats: [unitSeat] }))).toEqual([]);
  });
  it("the card says the step and age", () => {
    expect(doingOf(machine({ holds: [held()] }), unitSeat, [], now).words).toBe("building · build step 52 min");
    const rounds = { ...unitSeat, goals: [{ goal: "one-folder", stage: "build", stuck: { ...stuck, step: "", launch: "", rounds: 3, limit: 3 } }] };
    expect(doingOf(machine({ holds: [held()] }), rounds, [], now).words).toBe("building · round 3 of 3");
    expect(needs(fleetRead({ machines: [machine({ holds: [held()] })] }), boardRead({ seats: [rounds] }))[0].command).toBe("metasystem work land --message");
  });
});

import { describe, expect, it } from "vitest";

import type { Launch, Machine } from "./api";
import * as launching from "./launching";
import {
  blockedForLaunch,
  cardFor,
  destinationRefusal,
  failedStep,
  foldedLine,
  launchHeadline,
  nicknameRefusal,
  nicknamesTaken,
  proposedDestination,
  proposedNickname,
  leftoverLine,
  showsCard,
  visibleCard,
} from "./launching";

/**
 * What the sheet refuses before the act is sent, and what the card says about
 * a launch in each of its states.
 *
 * Every refusal here is the engine's own rule said earlier. The point of
 * testing them on this side is not that the verb would miss one — it would
 * not — but that a human filling in a form learns about a nickname the
 * publisher will not take while they are still typing it.
 */

const now = new Date("2026-09-25T11:00:00Z");

function launch(over: Partial<Launch> = {}): Launch {
  return {
    schemaVersion: 1,
    launch: "01M3BQAVYXE2AT6F0JG9YB64PG",
    machine: "m1f",
    destination: "/w/agentic-tools-m1f",
    clonedCommit: "b50abb9",
    builtStamp: "b50abb9",
    process: { pid: 40912, startedAt: 1764000000 },
    startedAt: "2026-09-25T10:57:00Z",
    endedAt: null,
    outcome: "running",
    created: { destination: true, nickname: false, evidenceRoot: true },
    steps: [{ step: "clone", outcome: "done", at: "2026-09-25T10:57:10Z", words: "" }],
    orientation: "",
    discardedAt: null,
    destinationPresent: false,
    next: { session: "cd /w/agentic-tools-m1f && claude", stop: "metasystem system stop --repo /w/agentic-tools-m1f/metasystem" },
    ...over,
  };
}

function machineRow(name: string): Machine {
  return {
    machine: name,
    standing: "reachable",
    reason: "",
    ageSeconds: 30,
    seen: "2026-09-25T10:59:30Z",
    since: "",
    running: null,
    engine: "",
    generation: 4,
    holds: [],
    this: false,
    working: [],
    workingProblem: "",
  };
}

describe("the nickname the sheet proposes", () => {
  it("is the first free letter of this host's series", () => {
    expect(proposedNickname("m1u", ["m1b", "m1c", "m1d", "m1e", "m1u"])).toBe("m1f");
  });

  it("skips the series' own first letter, which is never free", () => {
    expect(proposedNickname("m1u", [])).toBe("m1b");
  });

  it("proposes nothing where this seat has no nickname to take a series from", () => {
    expect(proposedNickname("", ["m1b"])).toBe("");
    expect(proposedNickname("m", ["m1b"])).toBe("");
  });

  it("counts the launches this page is already showing as taken", () => {
    const taken = nicknamesTaken([machineRow("m1b")], [launch({ machine: "m1c" })]);
    expect(taken).toEqual(["m1b", "m1c"]);
    expect(proposedNickname("m1u", taken)).toBe("m1d");
  });

  it("does not count a discarded launch's nickname as taken", () => {
    const discarded = launch({ machine: "testbed", discardedAt: "2026-09-29T06:47:09Z", destinationPresent: false });
    const kept = launch({ machine: "m1c", discardedAt: "2026-09-29T06:47:09Z", destinationPresent: true });
    expect(nicknamesTaken([machineRow("m1b")], [discarded, kept])).toEqual(["m1b"]);
  });
});

describe("the nickname the sheet refuses", () => {
  it("refuses what the presence publisher would refuse", () => {
    expect(nicknameRefusal("m1/f", [], "m1u")).toContain("git ref segment");
    expect(nicknameRefusal(".", [], "m1u")).not.toBe("");
    expect(nicknameRefusal("a..b", [], "m1u")).not.toBe("");
    expect(nicknameRefusal("m1 f", [], "m1u")).not.toBe("");
  });

  it("refuses this checkout's own nickname", () => {
    expect(nicknameRefusal("m1u", [], "m1u")).toContain("publish over each other");
  });

  it("refuses a nickname the fleet already carries", () => {
    expect(nicknameRefusal("m1e", ["m1e"], "m1u")).toContain("already a machine");
  });

  it("says nothing about a field nobody has typed in", () => {
    expect(nicknameRefusal("", ["m1e"], "m1u")).toBe("");
  });

  it("takes the names the engine's own rule takes", () => {
    expect(nicknameRefusal("m1f", ["m1e"], "m1u")).toBe("");
    expect(nicknameRefusal("build-box.2_a", [], "m1u")).toBe("");
  });
});

describe("where the sheet proposes a machine lands", () => {
  it("is beside this checkout, named for the remote's repository", () => {
    expect(proposedDestination({ parent: "/w", repository: "agentic-tools" }, "m1f")).toBe("/w/agentic-tools-m1f");
  });

  it("proposes nothing where the server could not read either half", () => {
    expect(proposedDestination({ parent: "", repository: "agentic-tools" }, "m1f")).toBe("");
    expect(proposedDestination({ parent: "/w", repository: "" }, "m1f")).toBe("");
  });

  it("refuses a path that is not this host's", () => {
    expect(destinationRefusal("agentic-tools-m1f")).toContain("absolute path");
    expect(destinationRefusal("/w/agentic-tools-m1f")).toBe("");
  });
});

describe("why the Launch button is disabled", () => {
  const whole = {
    machine: "m1f",
    destination: "/w/agentic-tools-m1f",
  };

  // g1-s72: signed in is enough. The nickname and the path are the whole
  // form; nothing asks for words or a date.
  it("is nothing when the nickname and the path validate", () => {
    expect(blockedForLaunch(whole, ["m1e"], "m1u")).toBe("");
  });

  it("names the field rather than saying something is missing", () => {
    expect(blockedForLaunch({ ...whole, machine: "" }, [], "m1u")).toContain("nickname");
    expect(blockedForLaunch({ ...whole, destination: "" }, [], "m1u")).toContain("path on this host");
  });

  it("carries each field's own refusal up to the button", () => {
    expect(blockedForLaunch({ ...whole, machine: "m1e" }, ["m1e"], "m1u")).toContain("already a machine");
    expect(blockedForLaunch({ ...whole, destination: "w/m1f" }, [], "m1u")).toContain("absolute path");
  });
});

describe("the card a launch becomes", () => {
  it("shows a launch in flight, one that stopped, and one that armed", () => {
    expect(showsCard(launch({ outcome: "running" }))).toBe(true);
    expect(showsCard(launch({ outcome: "failed" }))).toBe(true);
    expect(showsCard(launch({ outcome: "armed" }))).toBe(true);
    expect(showsCard(launch({ outcome: "starting" }))).toBe(true);
    expect(showsCard(launch({ outcome: "done" }))).toBe(false);
  });

  it("leaves out a launch a human discarded", () => {
    expect(showsCard(launch({ outcome: "failed", discardedAt: "2026-09-29T10:00:00Z" }))).toBe(false);
    expect(showsCard(launch({ outcome: "armed", discardedAt: "2026-09-29T10:00:00Z" }))).toBe(false);
    expect(
      cardFor([
        launch({ outcome: "failed", discardedAt: "2026-09-29T10:00:00Z" }),
        launch({ launch: "01M3BQ8000000000000000000A", machine: "m1g", outcome: "failed" }),
      ])?.machine,
    ).toBe("m1g");
  });

  it("is the newest launch still worth one", () => {
    expect(cardFor([launch({ outcome: "done" }), launch({ machine: "m1g", outcome: "failed" })])?.machine).toBe("m1g");
    expect(cardFor([launch({ outcome: "done" })])).toBe(null);
    expect(cardFor([])).toBe(null);
  });

  it("heads itself with what is happening, and armed is not a failure", () => {
    expect(launchHeadline(launch({ outcome: "starting" }))).toBe("Launching m1f");
    expect(launchHeadline(launch({ outcome: "running" }))).toBe("Launching m1f");
    expect(launchHeadline(launch({ outcome: "done" }))).toBe("m1f joined");
    expect(launchHeadline(launch({ outcome: "armed" }))).toBe("m1f armed; presence not yet seen");
    expect(launchHeadline(launch({ outcome: "failed" }))).toBe("Launching m1f stopped");
  });

  it("finds the step a launch stopped at, with the owner's own words", () => {
    const stopped = launch({
      outcome: "failed",
      steps: [
        { step: "clone", outcome: "done", at: "", words: "" },
        { step: "engine", outcome: "failed", at: "", words: "go-build: refused: the fence holds" },
      ],
    });
    expect(failedStep(stopped)?.step).toBe("engine");
    expect(failedStep(stopped)?.words).toBe("go-build: refused: the fence holds");
    expect(failedStep(launch())).toBe(null);
  });

  it("folds a machine that joined to one line: the machine and when it joined", () => {
    const joined = launch({ outcome: "done", endedAt: "2026-09-25T10:58:00Z" });
    expect(foldedLine(joined, now)).toBe("m1f joined 2 min ago");
  });
});

describe("the card the fleet block shows", () => {
  const failed = launch({ outcome: "failed" });
  const older = launch({ launch: "01M3BQ8000000000000000000A", machine: "m1g", outcome: "failed" });
  const none = new Set<string>();

  it("is the newest one worth a card, when nothing was started here", () => {
    expect(visibleCard([failed, older], null, none)?.launch).toBe(failed.launch);
  });

  it("follows the server's record of a launch started here", () => {
    const started = launch({ outcome: "starting" });
    expect(visibleCard([], started, none)?.outcome).toBe("starting");
    expect(visibleCard([launch({ outcome: "running" })], started, none)?.outcome).toBe("running");
  });

  it("hides a launch the moment its discard answers, before the next read", () => {
    expect(visibleCard([failed], null, new Set([failed.launch]))).toBe(null);
    expect(visibleCard([failed, older], null, new Set([failed.launch]))?.launch).toBe(older.launch);
  });

  it("keeps it hidden from the server's record once the read carries the mark", () => {
    const discarded = { ...failed, discardedAt: "2026-09-29T10:00:00Z" };
    expect(visibleCard([discarded], null, none)).toBe(null);
    // A launch started here and then discarded stays gone too.
    expect(visibleCard([discarded], failed, none)).toBe(null);
  });
});

describe("what a stopped launch leaves on disk", () => {
  it("names the clone while it is there", () => {
    expect(leftoverLine(launch({ outcome: "failed", destinationPresent: true }))).toBe(
      "The clone at /w/agentic-tools-m1f stays on disk; delete it yourself.",
    );
  });

  it("says nothing once it is gone", () => {
    expect(leftoverLine(launch({ outcome: "failed", destinationPresent: false }))).toBe("");
  });
});

// g1-s72 D5: the interface forgets the word. Nothing on the fleet page
// proposes, refuses or explains a review date or a temporary enrollment.
describe("what the launch no longer asks for", () => {
  it("offers no review date rule and no temporary-enrollment words", () => {
    for (const gone of [
      "TEMPORARY_RULE",
      "RETRY_ASKS_AGAIN",
      "REVIEW_DAYS",
      "proposedReviewBy",
      "earliestReviewBy",
      "clientToday",
      "reviewByRefusal",
      "reviewDay",
    ]) {
      expect(Object.keys(launching)).not.toContain(gone);
    }
  });
});

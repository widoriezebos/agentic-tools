import { describe, expect, it } from "vitest";

import type { Launch, Machine } from "./api";
import {
  blockedForLaunch,
  cardFor,
  clientToday,
  destinationRefusal,
  earliestReviewBy,
  failedStep,
  foldedLine,
  launchHeadline,
  nicknameRefusal,
  nicknamesTaken,
  proposedDestination,
  proposedNickname,
  proposedReviewBy,
  reviewByRefusal,
  reviewDay,
  showsCard,
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
    reviewBy: "2026-10-02",
    created: { destination: true, nickname: false, evidenceRoot: true },
    steps: [{ step: "clone", outcome: "done", at: "2026-09-25T10:57:10Z", words: "" }],
    orientation: "",
    next: { session: "cd /w/agentic-tools-m1f && claude", stop: "metasystem stop --repo /w/agentic-tools-m1f/metasystem" },
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

describe("the review date", () => {
  it("opens a week out", () => {
    expect(proposedReviewBy(new Date(2026, 8, 25))).toBe("2026-10-02");
  });

  it("refuses a date that is not later than today, which the engine's validator does not", () => {
    expect(reviewByRefusal("2026-09-24", new Date(2026, 8, 25))).toContain("due the moment the machine joins");
    expect(reviewByRefusal("2026-09-25", new Date(2026, 8, 25))).toContain("due the moment the machine joins");
    expect(reviewByRefusal("2026-10-02", new Date(2026, 8, 25))).toBe("");
  });

  it("refuses something that is not a day at all", () => {
    expect(reviewByRefusal("next tuesday", now)).toContain("YYYY-MM-DD");
  });
});

describe("why the Launch button is disabled", () => {
  const whole = {
    machine: "m1f",
    destination: "/w/agentic-tools-m1f",
    word: "Wido says launch m1f on this host",
    reviewBy: "2026-10-02",
  };

  it("is nothing when every field validates", () => {
    expect(blockedForLaunch(whole, ["m1e"], "m1u", new Date(2026, 8, 25))).toBe("");
  });

  it("names the field rather than saying something is missing", () => {
    expect(blockedForLaunch({ ...whole, machine: "" }, [], "m1u", now)).toContain("nickname");
    expect(blockedForLaunch({ ...whole, destination: "" }, [], "m1u", now)).toContain("path on this host");
    expect(blockedForLaunch({ ...whole, word: "  " }, [], "m1u", now)).toContain("your own words");
  });

  it("carries each field's own refusal up to the button", () => {
    expect(blockedForLaunch({ ...whole, machine: "m1e" }, ["m1e"], "m1u", now)).toContain("already a machine");
    expect(blockedForLaunch({ ...whole, reviewBy: "2026-09-24" }, [], "m1u", new Date(2026, 8, 25))).toContain(
      "due the moment the machine joins",
    );
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

  it("folds a machine that joined to one line with its enrollment on it", () => {
    const joined = launch({ outcome: "done", endedAt: "2026-09-25T10:58:00Z" });
    // The day is rendered in the reader's own locale, like every other
    // instant on this page, so the line is asserted around it.
    expect(foldedLine(joined, now)).toContain("m1f joined 2 min ago; temporary enrollment, review due ");
    expect(foldedLine(joined, now)).toContain(reviewDay("2026-10-02"));
  });
});

describe("the day that travels with a launch", () => {
  it("is this browser's own day, and the earliest review date is the one after it", () => {
    const now = new Date(2026, 8, 25, 23, 50);
    expect(clientToday(now)).toBe("2026-09-25");
    expect(earliestReviewBy(now)).toBe("2026-09-26");
  });

  it("refuses a review due today, because that is due the moment the machine joins", () => {
    const now = new Date(2026, 8, 25);
    expect(reviewByRefusal("2026-09-25", now)).toContain("due the moment the machine joins");
    expect(reviewByRefusal("2026-09-26", now)).toBe("");
  });
});

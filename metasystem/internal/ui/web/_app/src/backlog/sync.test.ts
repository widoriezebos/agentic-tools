import { describe, expect, it } from "vitest";

import type { FetchClause, Ledger } from "./api";
import { clockTime, minuteTime } from "./format";
import { syncOf } from "./sync";

/**
 * The chip's three states, and the one line under the toolbar.
 *
 * What the chip says is a function of the payload and nothing else, so it is
 * tested as one. The point of each case is which voice it uses: quiet for the
 * usual answer, the marker for a tip the server itself calls old, and the
 * danger colour with a line on the page for the two things a human can do
 * something about.
 *
 * Every time below is rendered rather than written out, because a wall clock
 * is the machine's own: an assertion that only held where it was written
 * would say nothing about the machine a human runs this on.
 */

const OBSERVED = "2026-09-23T10:30:03Z";
const FINISHED = "2026-09-23T10:29:41Z";
const NEXT = "2026-09-23T10:34:41Z";

function fetched(over: Partial<FetchClause> = {}): FetchClause {
  return {
    outcome: "current",
    startedAt: "2026-09-23T10:29:40Z",
    finishedAt: FINISHED,
    tip: "",
    detail: "already at the canonical tip",
    message: "",
    failures: 0,
    cadence: "5m",
    nextAt: NEXT,
    ...over,
  };
}

function ledger(over: Partial<Ledger> = {}): Ledger {
  return {
    state: "read",
    tip: "2ef5d8d9c1b4a70f3e2d1c0b9a8f7e6d5c4b3a29",
    committedAt: "2026-09-23T10:12:00Z",
    stale: false,
    staleAfterSeconds: 3600,
    syncMode: "",
    stateRoot: "",
    message: "",
    problems: [],
    fetch: fetched(),
    ...over,
  };
}

describe("a ledger that is current", () => {
  const sync = syncOf(ledger(), OBSERVED);

  it("is at rest, and says only when this page read and what it read", () => {
    expect(sync.state).toBe("rest");
    expect(sync.line).toBe(`Synced ${minuteTime(OBSERVED)} · 2ef5d8d`);
  });

  it("puts no line on the page, because nothing is wrong", () => {
    expect(sync.wrong).toBe("");
  });

  // The two lines that stood above the board are the tooltip now, fact for
  // fact: a human who wants the report asks for it and gets all of it.
  it("carries the whole report where it is asked for", () => {
    expect(sync.report).toContain("Accepted tip 2ef5d8d, committed");
    expect(sync.report).toContain(`observed ${clockTime(OBSERVED)}`);
    expect(sync.report).toContain(`fetched ${clockTime(FINISHED)}, already at the canonical tip`);
    expect(sync.report).toContain(`next fetch ${clockTime(NEXT)}`);
  });
});

describe("a tip the server calls old", () => {
  const sync = syncOf(ledger({ stale: true, committedAt: "2026-09-23T08:12:00Z" }), OBSERVED);

  it("says how far behind it is, in the chip itself", () => {
    expect(sync.state).toBe("stale");
    expect(sync.line).toBe(`Synced ${minuteTime(OBSERVED)} · 2 h behind`);
  });

  // Old is not broken: a repository nobody has committed to for a day is a
  // quiet repository, and the age sentence is in the report for whoever asks.
  it("puts no line on the page, and says why it is old in the report", () => {
    expect(sync.wrong).toBe("");
    expect(sync.report).toContain("The accepted tip is 2 h old; the last fetch found the canonical branch at this tip.");
  });
});

/** A ledger whose loop last failed, with the message the server gave. */
function failing(message: string, nextAt = NEXT): Ledger {
  const loop = fetched({ outcome: "failed", message, nextAt });
  return { ...ledger(), fetch: loop };
}

describe("a fetch that failed", () => {
  const sync = syncOf(failing("ssh: connect: host unreachable"), OBSERVED);

  it("is wrong, and says so where the chip stands", () => {
    expect(sync.state).toBe("wrong");
    expect(sync.line).toBe(`Sync failed ${minuteTime(FINISHED)}`);
  });

  it("puts one line on the page, with the reason and what to do", () => {
    expect(sync.wrong).toContain("ssh: connect: host unreachable");
    expect(sync.wrong).toContain(`tries again at ${clockTime(NEXT)}`);
    expect(sync.wrong).toContain("Refresh");
  });

  it("says the server is stopping rather than promising a retry that is not due", () => {
    expect(syncOf(failing("host unreachable", ""), OBSERVED).wrong).toContain("The server is stopping");
  });
});

describe("a ledger that did not project", () => {
  it("is wrong for every state that is not a read, and says which", () => {
    expect(syncOf(ledger({ state: "absent", tip: "" }), OBSERVED).line).toBe("No accepted tip");
    expect(syncOf(ledger({ state: "no-ledger" }), OBSERVED).line).toBe("No ledger at the tip");
    expect(syncOf(ledger({ state: "unreadable" }), OBSERVED).line).toBe("Ledger not valid");
    expect(syncOf(ledger({ state: "refused" }), OBSERVED).line).toBe("Ledger refused");
    for (const state of ["absent", "no-ledger", "unreadable", "broken", "refused"] as const) {
      expect({ state, is: syncOf(ledger({ state }), OBSERVED).state }).toEqual({ state, is: "wrong" });
    }
  });

  it("carries a reason for each, because the pane may be the only thing that says one", () => {
    expect(syncOf(ledger({ state: "absent", tip: "" }), OBSERVED).wrong).toContain("no accepted tip yet");
    expect(syncOf(ledger({ state: "no-ledger" }), OBSERVED).wrong).toContain("carries no backlog");
    expect(syncOf(ledger({ state: "unreadable" }), OBSERVED).wrong).toContain("does not validate");
  });
});

describe("no ledger at all", () => {
  // A page that has not read yet is not a page that read something fine.
  it("is wrong, and the remedy is the read itself", () => {
    const sync = syncOf(null, "");
    expect(sync.state).toBe("wrong");
    expect(sync.line).toBe("Not read");
    expect(sync.wrong).toContain("Refresh");
  });
});

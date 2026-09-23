import { describe, expect, it } from "vitest";

import type { FetchClause, Freshness, Ledger } from "./api";
import { clockTime, dateAndTime, minuteTime } from "./format";
import { syncOf } from "./sync";

/**
 * The chip's three states, and the one line under the toolbar.
 *
 * What the chip says is a function of the payload and nothing else, so it is
 * tested as one. The point of each case is which voice it uses: quiet for the
 * usual answer, the marker for a fetch loop the server says has not heard
 * from the canonical branch lately, and the danger colour with a line on the
 * page for the two things a human can do something about.
 *
 * The accepted commit's age appears in exactly one assertion below, and it is
 * an assertion that nothing raises its voice about it. That is the rule: a
 * tree nobody has committed to since breakfast is quiet, not stale, and
 * Refresh cannot make a commit younger.
 *
 * Every time below is rendered rather than written out, because a wall clock
 * is the machine's own: an assertion that only held where it was written
 * would say nothing about the machine a human runs this on.
 */

const OBSERVED = "2026-09-23T10:30:03Z";
const FINISHED = "2026-09-23T10:29:41Z";
const NEXT = "2026-09-23T10:34:41Z";
const COMMITTED = "2026-09-23T08:14:00Z";
const TIP = "2ef5d8d9c1b4a70f3e2d1c0b9a8f7e6d5c4b3a29";

function fetched(over: Partial<FetchClause> = {}): FetchClause {
  return {
    outcome: "current",
    startedAt: "2026-09-23T10:29:40Z",
    finishedAt: FINISHED,
    tip: TIP,
    detail: "already at the canonical tip",
    message: "",
    failures: 0,
    cadence: "5m",
    nextAt: NEXT,
    succeededAt: FINISHED,
    succeededTip: TIP,
    ...over,
  };
}

function current(): Freshness {
  return { state: "current", since: FINISHED, detail: "already at the canonical tip" };
}

function ledger(over: Partial<Ledger> = {}): Ledger {
  return {
    state: "read",
    tip: TIP,
    committedAt: COMMITTED,
    freshness: current(),
    stale: false,
    staleAfterSeconds: 1800,
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
    expect(sync.report).toContain("Accepted tip 2ef5d8d");
    expect(sync.report).toContain(`observed ${clockTime(OBSERVED)}`);
    expect(sync.report).toContain(`fetched ${clockTime(FINISHED)}, already at the canonical tip`);
    expect(sync.report).toContain(`next fetch ${clockTime(NEXT)}`);
  });

  // The one place the accepted commit's age is still said, and it is said as
  // a fact rather than as an alarm.
  it("reports when the project last changed, and raises no voice about it", () => {
    const quiet = syncOf(ledger({ committedAt: "2026-09-21T08:14:00Z" }), OBSERVED);
    expect(quiet.report).toContain(`last change ${dateAndTime("2026-09-21T08:14:00Z")}`);
    expect(quiet.state).toBe("rest");
    expect(quiet.wrong).toBe("");
  });
});

describe("a fetch loop that has not heard from the canonical branch lately", () => {
  const since = "2026-09-23T09:58:00Z";
  const sync = syncOf(
    ledger({
      freshness: {
        state: "behind",
        since,
        detail: "the last fetch of the canonical branch landed more than 30 minutes ago",
      },
    }),
    OBSERVED,
  );

  it("says since when, in the chip itself", () => {
    expect(sync.state).toBe("behind");
    expect(sync.line).toBe(`Behind since ${minuteTime(since)}`);
  });

  // Behind is not broken: the loop looks again on its own, and the server's
  // reason is in the report for whoever asks.
  it("puts no line on the page, and says why in the report", () => {
    expect(sync.wrong).toBe("");
    expect(sync.report).toContain(
      `Behind since ${clockTime(since)}: the last fetch of the canonical branch landed more than 30 minutes ago.`,
    );
  });

  it("names no instant where no fetch has ever landed", () => {
    const never = syncOf(
      ledger({
        freshness: { state: "behind", since: "", detail: "no fetch of the canonical branch has completed on this clone yet" },
      }),
      OBSERVED,
    );
    expect(never.line).toBe("Behind");
    expect(never.report).toContain("Behind: no fetch of the canonical branch has completed on this clone yet.");
  });

  it("says the canonical tip it has not caught up with, in the server's words", () => {
    const ahead = syncOf(
      ledger({
        freshness: {
          state: "behind",
          since,
          detail: "the last fetch found the canonical branch at 9f3c1ab and this clone has accepted 2ef5d8d",
        },
      }),
      OBSERVED,
    );
    expect(ahead.state).toBe("behind");
    expect(ahead.report).toContain("the last fetch found the canonical branch at 9f3c1ab");
  });
});

/** A ledger whose loop last failed, with the message and the count the server gave. */
function failing(detail: string, nextAt = NEXT): Ledger {
  const loop = fetched({ outcome: "failed", message: detail, failures: 3, nextAt, tip: "", detail: "" });
  return { ...ledger(), freshness: { state: "failed", since: FINISHED, detail }, fetch: loop };
}

describe("a fetch that failed", () => {
  const detail = "ssh: connect: host unreachable (3 fetches in a row have failed)";
  const sync = syncOf(failing(detail), OBSERVED);

  it("is wrong, and says so where the chip stands", () => {
    expect(sync.state).toBe("wrong");
    expect(sync.line).toBe(`Fetch failed ${minuteTime(FINISHED)}`);
  });

  it("puts one line on the page, with the reason, the count and what to do", () => {
    expect(sync.wrong).toContain("ssh: connect: host unreachable");
    expect(sync.wrong).toContain("3 fetches in a row have failed");
    expect(sync.wrong).toContain(`tries again at ${clockTime(NEXT)}`);
    expect(sync.wrong).toContain("Refresh");
  });

  it("carries the same sentence in the report", () => {
    expect(sync.report).toContain(`Fetch failed ${clockTime(FINISHED)}: ${detail}.`);
  });

  it("says the server is stopping rather than promising a retry that is not due", () => {
    expect(syncOf(failing(detail, ""), OBSERVED).wrong).toContain("The server is stopping");
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

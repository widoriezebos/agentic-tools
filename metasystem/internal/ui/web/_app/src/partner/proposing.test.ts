import { describe, expect, it } from "vitest";

import type { Proposal } from "./api";
import {
  answeredOf,
  applyLabel,
  argumentsOf,
  askLine,
  barLine,
  cardsIn,
  dispatchOf,
  displayedFor,
  fetchFailedLine,
  footLine,
  foldedLine,
  GOAL_CHANGED,
  goesOn,
  guardFor,
  lineID,
  lineState,
  NEEDS_ITS_BUDGET,
  newestWaitingCard,
  NOT_APPLIED_SIGN_IN,
  NOT_RUN,
  offersContinue,
  offersTryAgain,
  recorded,
  runProposals,
  sendable,
  ticked,
  tierWords,
  verbWord,
  waitingAcross,
  WAS_IN_FLIGHT,
  COULD_NOT_RECORD,
  COULD_NOT_START,
  type Answered,
  type Card,
  type Displayeds,
  type Line,
  type Looked,
  type Marks,
  type Mark,
  type RunPorts,
  type Written,
} from "./proposing";
import { BacklogError, type Budget, type Row } from "../backlog/api";
import { LANDED_NOT_RECORDED } from "../backlog/editing";

/**
 * The rules around the press, without a browser.
 *
 * Everything asserted here is something a human would notice if it were wrong:
 * which lines a press sends, what each answer does to the run, what a line says,
 * and what it offers afterwards. The two rules that carry the most are the ones
 * Astra's rounds found — a refusal is passed and an unknown answer stops, and
 * what was read is what is approved — so both are driven through whole runs
 * rather than asserted one function at a time.
 */

const BOX: Budget = {
  elapsedLimit: "4h",
  attemptLimit: 6,
  reservedJobMinutesLimit: 720,
  activeJobLimit: 1,
  reviewRoundLimit: 2,
};

function proposal(over: Partial<Proposal> = {}): Proposal {
  return {
    index: 0,
    verb: "park-goal",
    goal: "fleet-presence",
    title: "Fleet presence is read from the census",
    fields: { because: "superseded by the seat inventory" },
    read: null,
    why: "the five name the fleet inventory",
    offered: true,
    reason: "",
    state: "waiting",
    words: "",
    at: "2026-09-26T12:00:00Z",
    version: 1,
    ...over,
  };
}

function row(over: Partial<Row> = {}): Row {
  return {
    ref: { kind: "goal", id: "fleet-presence", revision: 9 },
    where: "live",
    lane: "to-do",
    phase: "",
    state: "queued",
    intent: "Fleet presence is read from the census, not polled.",
    nextStep: "Read the census.",
    concluded: "",
    origin: "human",
    priority: 2,
    sequence: 4,
    tier: 2,
    labels: ["fleet"],
    arc: "",
    pinned: "",
    blockedBy: [],
    openBlockers: [],
    holds: [],
    sliced: false,
    decomposed: false,
    openedAt: "2026-08-23T00:00:00Z",
    doneAt: "",
    lastChangeAt: "",
    lastVerb: "",
    gaps: [],
    ...over,
  };
}

/** One card over the proposals given, with the marks and tuples given. */
function card(
  proposals: readonly Proposal[],
  marks: Marks = {},
  displayed: Displayeds = {},
  dismissed: readonly string[] = [],
): Card {
  const cards = cardsIn([{ turn: "t1", proposals }], marks, displayed, dismissed);
  expect(cards.length).toBe(1);
  return cards[0];
}

function lineOf(one: Card, index = 0): Line {
  return one.lines[index];
}

describe("the word on the line", () => {
  /**
   * It is the word the button on the page that offers the act uses, so a human
   * who has pressed Not now on the Decisions page reads Not now here. It is never
   * the route id: that is what the message persists and the runner dispatches on.
   */
  it("is the page's own button word for every act", () => {
    expect(verbWord("park-goal")).toBe("Not now");
    expect(verbWord("unpark-goal")).toBe("Return to queue");
    expect(verbWord("withdraw-goal")).toBe("Withdraw approval");
    expect(verbWord("approve-goal")).toBe("Approve");
    expect(verbWord("set-goal-priority")).toBe("Set priority");
    expect(verbWord("open-goal")).toBe("Open goal");
    expect(verbWord("edit-goal")).toBe("Edit");
    expect(verbWord("block-goal")).toBe("Waits for");
    expect(verbWord("unblock-goal")).toBe("No longer waits for");
  });
});

describe("what a line says it will carry", () => {
  it("is every argument, in the flags' own words", () => {
    expect(argumentsOf(lineOf(card([proposal()])))).toEqual([
      { label: "Reason", value: "superseded by the seat inventory" },
    ]);
    expect(
      argumentsOf(
        lineOf(card([proposal({ verb: "set-goal-priority", fields: { priority: "2", sequence: "5" } })])),
      ),
    ).toEqual([
      { label: "Priority", value: "2" },
      { label: "Position", value: "5" },
    ]);
    expect(
      argumentsOf(lineOf(card([proposal({ verb: "block-goal", fields: { blocker: "bank-sandbox" } })]))),
    ).toEqual([{ label: "The goal it waits for", value: "bank-sandbox" }]);
  });

  /**
   * An open shows the tier DERIVED from the four answers, with the answers named:
   * the Partner names no tier, and the interface derives what they imply by the
   * new-goal sheet's own function and sends what it derived.
   */
  it("derives an open's tier and names the four answers", () => {
    const fields = {
      intent: "Every refund lands within a day.",
      nextStep: "Read the retry loop.",
      severity: "2",
      novelty: "3",
      exposure: "2",
      accumulation: "1",
      basis: "payments, one team",
      labels: "payments, robustness",
      blockedBy: "bank-sandbox",
    };
    expect(tierWords(fields)).toBe("3, from severity 2 · novelty 3 · exposure 2 · accumulation 1");
    expect(argumentsOf(lineOf(card([proposal({ verb: "open-goal", fields })])))).toEqual([
      { label: "Intent", value: "Every refund lands within a day." },
      { label: "First next step", value: "Read the retry loop." },
      { label: "Tier", value: "3, from severity 2 · novelty 3 · exposure 2 · accumulation 1" },
      { label: "Basis", value: "payments, one team" },
      { label: "Labels", value: "payments, robustness" },
      { label: "Waits for", value: "bank-sandbox" },
    ]);
  });

  /**
   * An approve shows the intent and the next step whole from what was read, as the
   * Decisions open row does before its Approve, and the budget it would carry with
   * where that budget came from.
   */
  it("shows an approval's reviewed intent and the budget with its source", () => {
    const approve = proposal({
      verb: "approve-goal",
      fields: {},
      read: { intent: "Fleet presence is read from the census.", nextStep: "Read it.", tier: 2, labels: ["fleet"] },
    });
    const one = card([approve], {}, { [lineID("t1", 0)]: { budget: BOX, source: "project" } });
    expect(argumentsOf(lineOf(one))).toEqual([
      { label: "Intent", value: "Fleet presence is read from the census." },
      { label: "Next step", value: "Read it." },
      {
        label: "Budget",
        value:
          "4h elapsed · 6 attempts · 720 reserved job-minutes · 1 active jobs · 2 review rounds, " +
          "from the project's budget law for this goal's tier",
      },
    ]);
  });

  /**
   * An approve the interface cannot prefill a budget for is listed and excluded
   * rather than dropped or sent with five empty fields: nothing here invents a
   * budget, and the human approves it alone where the sheet can ask them.
   */
  it("excludes an approval with no budget, and says what to do about it", () => {
    const approve = proposal({ verb: "approve-goal", fields: {}, read: null });
    const one = card([approve], {}, { [lineID("t1", 0)]: { budget: null, source: "none" } });
    expect(argumentsOf(lineOf(one))).toEqual([{ label: "Budget", value: NEEDS_ITS_BUDGET }]);
    expect(sendable(one)).toEqual([]);
    expect(ticked(one)).toBe(0);
  });
});

describe("what one press sends", () => {
  it("is the ticked, waiting, sendable lines and no others", () => {
    const one = card(
      [
        proposal({ index: 0 }),
        proposal({ index: 1, goal: "refunds" }),
        proposal({ index: 2, goal: "bank-sandbox" }),
        proposal({ index: 3, goal: "gone", offered: false, reason: "the accepted tip carries no goal gone" }),
        proposal({ index: 4, goal: "already", state: "applied" }),
      ],
      { [lineID("t1", 1)]: { ticked: false, notRun: false, refusedUnsent: "" } },
    );
    expect(sendable(one).map((line) => line.goal)).toEqual(["fleet-presence", "bank-sandbox"]);
    expect(applyLabel(one)).toBe("Apply 2");
  });

  it("says Apply without a number where there is one line", () => {
    expect(applyLabel(card([proposal()]))).toBe("Apply");
  });

  /** The body each act sends, composed from the route body's own fields. */
  it("composes each act's own body", () => {
    expect(dispatchOf(lineOf(card([proposal()])))).toEqual({
      act: "park", id: "fleet-presence", because: "superseded by the seat inventory",
    });
    expect(dispatchOf(lineOf(card([proposal({ verb: "unpark-goal", fields: {} })])))).toEqual({
      act: "unpark", id: "fleet-presence",
    });
    expect(dispatchOf(lineOf(card([proposal({ verb: "withdraw-goal", fields: { reason: "the date passed" } })]))))
      .toEqual({ act: "withdraw", id: "fleet-presence", reason: "the date passed" });
    expect(dispatchOf(lineOf(card([proposal({ verb: "set-goal-priority", fields: { priority: "2" } })]))))
      .toEqual({ act: "priority", id: "fleet-presence", priority: 2, sequence: null });
    expect(dispatchOf(lineOf(card([proposal({ verb: "unblock-goal", fields: { blocker: "x" } })]))))
      .toEqual({ act: "unblock", dependent: "fleet-presence", blocker: "x" });
  });

  /**
   * An edit sends the fields the action names and no others: a field it says
   * nothing about is left exactly as the ledger has it, and an emptied label list
   * clears the labels.
   */
  it("sends only the fields an edit names", () => {
    expect(dispatchOf(lineOf(card([proposal({ verb: "edit-goal", fields: { intent: "Tighter." } })]))))
      .toEqual({ act: "edit", id: "fleet-presence", edit: { intent: "Tighter." } });
    expect(dispatchOf(lineOf(card([proposal({ verb: "edit-goal", fields: { labels: "" } })]))))
      .toEqual({ act: "edit", id: "fleet-presence", edit: { labels: [] } });
  });

  /** An open sends the tier it derived, and the lists as lists. */
  it("sends an open's derived tier and its lists", () => {
    const dispatch = dispatchOf(
      lineOf(
        card([
          proposal({
            verb: "open-goal", goal: "refund-worker",
            fields: {
              id: "refund-worker", intent: "i", nextStep: "n", basis: "b",
              severity: "2", novelty: "3", exposure: "1", accumulation: "1",
              labels: "payments, robustness", blockedBy: "bank-sandbox", blocks: "refund-report",
            },
          }),
        ]),
      ),
    );
    expect(dispatch).toEqual({
      act: "open",
      goal: {
        id: "refund-worker", intent: "i", nextStep: "n", tier: 3, why: "",
        blocks: ["refund-report"], blockedBy: ["bank-sandbox"], labels: ["payments", "robustness"],
        severity: 2, novelty: 3, exposure: 1, accumulation: 1, basis: "b",
      },
    });
  });

  /** And an approve sends the tuple the line DISPLAYED and no other. */
  it("sends the tuple an approve line displayed", () => {
    const one = card(
      [proposal({ verb: "approve-goal", fields: {} })],
      {},
      { [lineID("t1", 0)]: { budget: BOX, source: "goal" } },
    );
    expect(dispatchOf(lineOf(one))).toEqual({ act: "approve", id: "fleet-presence", budget: BOX });
  });
});

describe("what the run does with each answer", () => {
  it("passes a refusal and stops at anything that does not say what happened", () => {
    expect(goesOn({ kind: "applied", words: "" })).toBe(true);
    expect(goesOn({ kind: "refused", words: "goal is claimed" })).toBe(true);
    expect(goesOn({ kind: "unresolved", words: "nobody knows" })).toBe(false);
    expect(goesOn({ kind: "sign-in", words: NOT_APPLIED_SIGN_IN })).toBe(false);
  });

  /**
   * A definite non-write is a refusal the run passes. A redundant block answers
   * 409 `abandoned`, which used to stop the run although nothing was written
   * (Astra S58-12).
   */
  it("reads a definite no-op as a refusal, so the next line is sent", () => {
    const answered = answeredOf(new BacklogError("/act", 409, "there was nothing to do", "abandoned"));
    expect(answered).toEqual({ kind: "refused", words: "there was nothing to do" });
    expect(goesOn(answered)).toBe(true);
  });

  /**
   * The answer that says the act LANDED and its proof did not is recorded as
   * applied, with its own words. Retrying it would make a second act rather than
   * repair the first one's missing proof (Astra S58-05).
   */
  it("records a landed act whose proof was not recorded as applied", () => {
    const said = "the act landed at tip 6984cde, but its authority proof did not: no such file; do not run it again";
    const answered = answeredOf(new BacklogError("/act", 500, said, LANDED_NOT_RECORDED));
    expect(answered).toEqual({ kind: "applied", words: said });
    expect(goesOn(answered)).toBe(true);
    const one = card([proposal({ state: "applied", words: said })]);
    expect(lineState(lineOf(one))).toBe(`applied; ${said}`);
    expect(offersTryAgain(lineOf(one))).toBe(false);
  });

  /** A refusal a sign-in would remedy ends the run at that line, unpublished. */
  it("ends the run at a refusal a sign-in would remedy", () => {
    const answered = answeredOf(new BacklogError("/act", 403, "your session expired", "expired", true));
    expect(answered).toEqual({ kind: "sign-in", words: NOT_APPLIED_SIGN_IN });
    expect(recorded(answered)).toEqual({ state: "refused", words: NOT_APPLIED_SIGN_IN });
  });

  it("writes each answer's own state onto the line", () => {
    expect(recorded({ kind: "applied", words: "" })).toEqual({ state: "applied", words: "" });
    expect(recorded({ kind: "refused", words: "no" })).toEqual({ state: "refused", words: "no" });
    expect(recorded({ kind: "unresolved", words: "?" })).toEqual({ state: "unresolved", words: "?" });
  });
});

describe("what is approved is what was read", () => {
  const approve = proposal({
    verb: "approve-goal",
    fields: {},
    read: { intent: "Fleet presence is read from the census, not polled.", nextStep: "Read the census.", tier: 2, labels: ["fleet"] },
  });
  const displayed: Displayeds = { [lineID("t1", 0)]: { budget: BOX, source: "goal" } };

  it("sends a line whose goal has not moved", () => {
    const one = card([approve], {}, displayed);
    expect(guardFor(lineOf(one), [row({ budget: BOX })], {}, "current", "")).toBe("");
  });

  it("refuses a line whose goal has moved, unsent", () => {
    const one = card([approve], {}, displayed);
    for (const moved of [
      row({ intent: "Something else entirely.", budget: BOX }),
      row({ nextStep: "Read something else.", budget: BOX }),
      row({ tier: 3, budget: BOX }),
      row({ labels: ["fleet", "browser-interface"], budget: BOX }),
    ]) {
      expect(guardFor(lineOf(one), [moved], {}, "current", "")).toBe(GOAL_CHANGED);
    }
    // And a goal that is not at the tip at all.
    expect(guardFor(lineOf(one), [], {}, "current", "")).toBe(GOAL_CHANGED);
  });

  /**
   * The budget the card displayed is the budget the run sends, and a fresh
   * prefill that answers another tuple refuses the line unsent: the press that
   * confirmed one tuple must not submit a different one (Astra S58-08).
   */
  it("refuses a line whose budget a fresh prefill now answers differently", () => {
    const one = card([approve], {}, displayed);
    const other: Budget = { ...BOX, attemptLimit: 12 };
    expect(guardFor(lineOf(one), [row({ budget: other })], {}, "current", "")).toBe(GOAL_CHANGED);
    // The same tuple from another source is a different answer to "what did the
    // human read", so it is refused too.
    expect(guardFor(lineOf(one), [row()], { "2": BOX }, "current", "")).toBe(GOAL_CHANGED);
  });

  /**
   * A read whose canonical fetch failed answers 200 from the accepted tip, so the
   * compare would run against a tip that can lag. Every line that depends on what
   * the goal says is refused unsent with the fetch's own message, and the lines
   * that need no compare run as they would (Astra S58-07).
   */
  it("refuses every line that needs the compare when the fetch failed", () => {
    const one = card([approve, proposal({ index: 1, verb: "edit-goal", fields: { intent: "x" } }),
      proposal({ index: 2 })], {}, displayed);
    const said = "the remote closed the connection";
    expect(guardFor(one.lines[0], [row({ budget: BOX })], {}, "failed", said)).toBe(fetchFailedLine(said));
    expect(guardFor(one.lines[1], [row({ budget: BOX })], {}, "failed", said)).toBe(fetchFailedLine(said));
    // A park depends on nothing the goal says, so it runs.
    expect(guardFor(one.lines[2], [], {}, "failed", said)).toBe("");
  });

  it("compares only against a fetch that landed", () => {
    const one = card([approve], {}, displayed);
    for (const outcome of ["advanced", "current"]) {
      expect(guardFor(lineOf(one), [row({ budget: BOX })], {}, outcome, "")).toBe("");
    }
    for (const outcome of ["never", "running", "failed"]) {
      expect(guardFor(lineOf(one), [row({ budget: BOX })], {}, outcome, "x")).toBe(fetchFailedLine("x"));
    }
  });

  /** The tuple each approve line displays is read once, from the payload. */
  it("takes each approve line's tuple from the payload, and none where there is none", () => {
    const one = card([approve, proposal({ index: 1, verb: "approve-goal", goal: "nowhere", fields: {} })]);
    expect(displayedFor(one.lines, [row({ budget: BOX })], {})).toEqual({
      [lineID("t1", 0)]: { budget: BOX, source: "goal" },
      [lineID("t1", 1)]: { budget: null, source: "none" },
    });
  });
});

describe("what a line says about where it stands", () => {
  it("says each state in the words a human reads", () => {
    expect(lineState(lineOf(card([proposal({ state: "applying" })])), true)).toBe("applying…");
    expect(lineState(lineOf(card([proposal({ state: "applied" })])))).toBe("applied");
    expect(lineState(lineOf(card([proposal({ state: "refused", words: "goal is claimed" })]))))
      .toBe("refused: goal is claimed");
    expect(lineState(lineOf(card([proposal({ state: "unresolved", words: "nobody knows" })]))))
      .toBe("unresolved: nobody knows; check the goal before trying again");
    expect(lineState(lineOf(card([proposal({ state: "dismissed" })])))).toBe("dismissed");
    expect(lineState(lineOf(card([proposal({ offered: false, reason: "no such goal" })])))).toBe("Not offered");
  });

  /**
   * A line a page went away in the middle of says so, and never says fresh: an
   * approve applied twice is two approval records, so a card that had forgotten
   * would invite the one press that makes one.
   */
  it("says a line the page left in flight was being applied", () => {
    expect(lineState(lineOf(card([proposal({ state: "applying" })])))).toBe(WAS_IN_FLIGHT);
  });

  it("says not run for a line a stopped run never reached", () => {
    const one = card([proposal()], { [lineID("t1", 0)]: { ticked: true, notRun: true, refusedUnsent: "" } });
    expect(lineState(lineOf(one))).toBe(NOT_RUN);
    expect(offersContinue(one)).toBe(true);
  });

  it("offers Try again on a refused or unresolved line and on nothing else", () => {
    expect(offersTryAgain(lineOf(card([proposal({ state: "refused", words: "no" })])))).toBe(true);
    expect(offersTryAgain(lineOf(card([proposal({ state: "unresolved", words: "?" })])))).toBe(true);
    expect(offersTryAgain(lineOf(card([proposal({ state: "waiting" })])))).toBe(false);
    expect(offersTryAgain(lineOf(card([proposal({ state: "applied" })])))).toBe(false);
    expect(offersTryAgain(lineOf(card([proposal({ state: "dismissed" })])))).toBe(false);
    expect(offersTryAgain(lineOf(card([proposal({ offered: false })])))).toBe(false);
  });

  it("counts what happened in the foot", () => {
    const one = card([
      proposal({ index: 0, state: "applied" }),
      proposal({ index: 1, state: "applied" }),
      proposal({ index: 2, state: "refused", words: "claimed" }),
      proposal({ index: 3, state: "unresolved", words: "?" }),
    ], { [lineID("t1", 3)]: { ticked: true, notRun: false, refusedUnsent: "" } });
    expect(footLine(one)).toBe("2 applied · 1 refused · 1 unresolved");
  });

  it("puts the line's own words in the composer for Ask the Partner", () => {
    const line = lineOf(card([proposal({ state: "refused", words: "goal is claimed by m1e" })]));
    expect(askLine(line)).toBe(
      "About your proposed Not now on fleet-presence — refused: goal is claimed by m1e. What else could be done?",
    );
  });
});

describe("where a card that waits stands", () => {
  /**
   * A card of any answer but the newest that still has a waiting line folds to one
   * line, so a stale Apply is never the first thing in view and a conversation
   * does not bury a decision (D8).
   */
  it("folds an older card with a waiting line and never the newest", () => {
    const cards = cardsIn(
      [
        { turn: "t1", proposals: [proposal({ index: 0 })] },
        { turn: "t2", proposals: [proposal({ index: 0, goal: "refunds" })] },
      ],
      {},
      {},
      [],
    );
    expect(cards.map((one) => one.folded)).toEqual([true, false]);
    expect(cards[1].newest).toBe(true);
    expect(foldedLine(cards[0])).toBe("The Partner proposed earlier · 1 waiting");
  });

  /**
   * A card with nothing waiting is read as it stands. Folding a finished card
   * would hide the record of what was done, which is the one thing the card is
   * for once it has been applied.
   */
  it("leaves a card with no waiting line open, however old", () => {
    const cards = cardsIn(
      [
        { turn: "t1", proposals: [proposal({ index: 0, state: "applied" })] },
        { turn: "t2", proposals: [proposal({ index: 0, goal: "refunds" })] },
      ],
      {},
      {},
      [],
    );
    expect(cards.map((one) => one.folded)).toEqual([false, false]);
  });

  it("folds a card the human dismissed", () => {
    const cards = cardsIn([{ turn: "t1", proposals: [proposal()] }], {}, {}, ["t1"]);
    expect(cards[0].folded).toBe(true);
  });

  /** The bar counts across answers, and its press opens the newest with one. */
  it("counts every waiting action across answers", () => {
    const cards = cardsIn(
      [
        { turn: "t1", proposals: [proposal({ index: 0 }), proposal({ index: 1, state: "applied" })] },
        { turn: "t2", proposals: [proposal({ index: 0 }), proposal({ index: 1 })] },
        { turn: "t3", proposals: [proposal({ index: 0, state: "dismissed" })] },
      ],
      {},
      {},
      [],
    );
    expect(waitingAcross(cards)).toBe(3);
    expect(barLine(cards)).toBe("3 actions proposed");
    expect(newestWaitingCard(cards)).toBe("t2");
  });

  it("says nothing where nothing is waiting", () => {
    const cards = cardsIn([{ turn: "t1", proposals: [proposal({ state: "applied" })] }], {}, {}, []);
    expect(waitingAcross(cards)).toBe(0);
    expect(barLine(cards)).toBe("");
    expect(newestWaitingCard(cards)).toBe("");
  });

  it("says one action in the singular", () => {
    const cards = cardsIn([{ turn: "t1", proposals: [proposal()] }], {}, {}, []);
    expect(barLine(cards)).toBe("1 action proposed");
  });
});

describe("the run, in order", () => {
  /** A run over these lines, with every impure step recorded rather than made. */
  function driving(
    lines: readonly Line[],
    over: Partial<{
      look: Looked;
      answers: Record<string, Answered>;
      writes: Record<string, Written>;
    }> = {},
  ) {
    const sent: string[] = [];
    const written: { line: string; state: string; words: string; version: number }[] = [];
    const marked: { line: string; change: Partial<Mark> }[] = [];
    const reconciled: Proposal[] = [];
    const rereads: number[] = [];
    const signIns: (() => void)[] = [];
    const ports: RunPorts = {
      look: () =>
        Promise.resolve(over.look ?? { rows: [], defaults: {}, outcome: "current", message: "" }),
      record: (line, state, words) => {
        written.push({ line: line.id, state, words, version: line.version });
        const forced = over.writes?.[`${line.id}:${state}`];
        if (forced !== undefined) {
          return Promise.resolve(forced);
        }
        return Promise.resolve({
          kind: "written",
          proposal: { ...line, state, words, version: line.version + 1 },
        });
      },
      send: (line) => {
        sent.push(line.id);
        return Promise.resolve(over.answers?.[line.id] ?? { kind: "applied", words: "" });
      },
      mark: (line, change) => {
        marked.push({ line: line.id, change });
      },
      reconcile: (proposal) => {
        reconciled.push(proposal);
      },
      reread: () => {
        rereads.push(rereads.length + 1);
      },
      signIn: (again) => {
        signIns.push(again);
      },
    };
    return { ports, sent, written, marked, reconciled, rereads, signIns };
  }

  const three = () => card([proposal({ index: 0 }), proposal({ index: 1, goal: "refunds" }),
    proposal({ index: 2, goal: "bank-sandbox" })]).lines;

  /** Two writes per line and no more: applying before the act, the outcome after. */
  it("writes applying before the act and the outcome after, for every line", async () => {
    const lines = three();
    const driven = driving(lines);
    await runProposals(lines, driven.ports);
    expect(driven.sent).toEqual([lines[0].id, lines[1].id, lines[2].id]);
    expect(driven.written).toEqual([
      { line: lines[0].id, state: "applying", words: "", version: 1 },
      { line: lines[0].id, state: "applied", words: "", version: 2 },
      { line: lines[1].id, state: "applying", words: "", version: 1 },
      { line: lines[1].id, state: "applied", words: "", version: 2 },
      { line: lines[2].id, state: "applying", words: "", version: 1 },
      { line: lines[2].id, state: "applied", words: "", version: 2 },
    ]);
    // The page in view reads again after each confirmed act and when the run ends.
    expect(driven.rereads.length).toBe(4);
  });

  it("passes a refusal and sends the next line", async () => {
    const lines = three();
    const driven = driving(lines, {
      answers: { [lines[1].id]: { kind: "refused", words: "goal is claimed" } },
    });
    await runProposals(lines, driven.ports);
    expect(driven.sent).toEqual([lines[0].id, lines[1].id, lines[2].id]);
    expect(driven.written.filter((one) => one.line === lines[1].id)).toEqual([
      { line: lines[1].id, state: "applying", words: "", version: 1 },
      { line: lines[1].id, state: "refused", words: "goal is claimed", version: 2 },
    ]);
    expect(driven.marked.some((one) => one.change.notRun === true)).toBe(false);
  });

  it("stops at an answer that does not say what happened, and the rest say not run", async () => {
    const lines = three();
    const driven = driving(lines, {
      answers: { [lines[1].id]: { kind: "unresolved", words: "the remote closed the connection" } },
    });
    await runProposals(lines, driven.ports);
    expect(driven.sent).toEqual([lines[0].id, lines[1].id]);
    expect(driven.marked.filter((one) => one.change.notRun === true).map((one) => one.line))
      .toEqual([lines[2].id]);
  });

  /** Continue with the rest runs from the first line the stopped run never reached. */
  it("runs on from the first not-run line when Continue is pressed", async () => {
    const lines = three();
    const driven = driving(lines.slice(2));
    await runProposals(lines.slice(2), driven.ports);
    expect(driven.sent).toEqual([lines[2].id]);
  });

  /** Try again sends one line and nothing else. */
  it("sends one line for Try again", async () => {
    const lines = three();
    const driven = driving([lines[1]]);
    await runProposals([lines[1]], driven.ports);
    expect(driven.sent).toEqual([lines[1].id]);
  });

  /**
   * A line the compare refuses is refused UNSENT and the run goes on: nothing was
   * published, which is exactly what a refusal means.
   */
  it("refuses a guarded line unsent and sends the next one", async () => {
    const approve = proposal({
      index: 0, verb: "approve-goal", fields: {},
      read: { intent: "not what the ledger says", nextStep: "n", tier: 2, labels: [] },
    });
    const one = card([approve, proposal({ index: 1, goal: "refunds" })], {},
      { [lineID("t1", 0)]: { budget: BOX, source: "goal" } });
    const driven = driving(one.lines, { look: { rows: [row({ budget: BOX })], defaults: {}, outcome: "current", message: "" } });
    await runProposals(one.lines, driven.ports);
    expect(driven.sent).toEqual([one.lines[1].id]);
    expect(driven.marked).toContainEqual({ line: one.lines[0].id, change: { refusedUnsent: GOAL_CHANGED } });
  });

  /**
   * A failed `applying` write sends nothing and stops the run: the outcome of a
   * press has nowhere to be recorded, so the press must not be made.
   */
  it("sends nothing and stops where the applying write failed", async () => {
    const lines = three();
    const driven = driving(lines, {
      writes: { [`${lines[0].id}:applying`]: { kind: "failed", words: "the conversation is unreachable" } },
    });
    await runProposals(lines, driven.ports);
    expect(driven.sent).toEqual([]);
    expect(driven.marked).toContainEqual({ line: lines[0].id, change: { refusedUnsent: COULD_NOT_START } });
    expect(driven.marked.filter((one) => one.change.notRun === true).map((one) => one.line))
      .toEqual([lines[1].id, lines[2].id]);
  });

  /**
   * A write somebody else won means the line is theirs. Nothing is sent for it,
   * and the run goes past it only where what they left is settled.
   */
  it("sends nothing for a line somebody else settled, and goes on", async () => {
    const lines = three();
    const held: Proposal = { ...lines[0], state: "applied", words: "", version: 4 };
    const driven = driving(lines, { writes: { [`${lines[0].id}:applying`]: { kind: "conflict", proposal: held } } });
    await runProposals(lines, driven.ports);
    expect(driven.reconciled).toEqual([held]);
    expect(driven.sent).toEqual([lines[1].id, lines[2].id]);
  });

  it("stops where the line somebody else left is unsettled", async () => {
    const lines = three();
    const held: Proposal = { ...lines[0], state: "applying", words: "", version: 4 };
    const driven = driving(lines, { writes: { [`${lines[0].id}:applying`]: { kind: "conflict", proposal: held } } });
    await runProposals(lines, driven.ports);
    expect(driven.reconciled).toEqual([held]);
    expect(driven.sent).toEqual([]);
    expect(driven.marked.filter((one) => one.change.notRun === true).map((one) => one.line))
      .toEqual([lines[1].id, lines[2].id]);
  });

  /** An outcome write that fails leaves the line saying that much, and nothing more. */
  it("says so where the conversation could not record an outcome", async () => {
    const lines = three();
    const driven = driving(lines, {
      writes: { [`${lines[0].id}:applied`]: { kind: "failed", words: "unreachable" } },
    });
    await runProposals(lines, driven.ports);
    expect(driven.marked).toContainEqual({ line: lines[0].id, change: { refusedUnsent: COULD_NOT_RECORD } });
    // The act landed, so the run goes on: the failure was the writing down of it.
    expect(driven.sent).toEqual([lines[0].id, lines[1].id, lines[2].id]);
  });

  /**
   * The run never waits on a sheet. A refusal a sign-in would remedy ends it at
   * that line, the rest say not run, and the sheet's success runs on FROM that
   * line — so a sheet closed without signing in leaves a settled card (Astra
   * S58-06).
   */
  it("ends at a sign-in refusal and runs on from that line when the human signs in", async () => {
    const lines = three();
    const driven = driving(lines, {
      answers: { [lines[1].id]: { kind: "sign-in", words: NOT_APPLIED_SIGN_IN } },
    });
    await runProposals(lines, driven.ports);
    expect(driven.sent).toEqual([lines[0].id, lines[1].id]);
    expect(driven.written).toContainEqual({
      line: lines[1].id, state: "refused", words: NOT_APPLIED_SIGN_IN, version: 2,
    });
    expect(driven.marked.filter((one) => one.change.notRun === true).map((one) => one.line))
      .toEqual([lines[2].id]);
    expect(driven.signIns.length).toBe(1);

    // Signing in runs on from the line that asked for it, and no earlier.
    const after = driving(lines.slice(1));
    await runProposals(lines.slice(1), after.ports);
    expect(after.sent).toEqual([lines[1].id, lines[2].id]);
  });

  /** And the run asks for the page's re-read once when it ends, however it ended. */
  it("asks the page to read again when the run ends", async () => {
    const lines = three();
    const driven = driving(lines, {
      answers: { [lines[0].id]: { kind: "refused", words: "no" } },
    });
    await runProposals([lines[0]], driven.ports);
    expect(driven.rereads.length).toBe(1);
  });
});

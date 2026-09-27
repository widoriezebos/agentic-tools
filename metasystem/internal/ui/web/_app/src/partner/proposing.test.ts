import { describe, expect, it } from "vitest";

import type { Proposal } from "./api";
import {
  ALREADY_CARRIED,
  answeredOf,
  applyLabel,
  askReread,
  argumentsOf,
  askLine,
  barLine,
  cardsIn,
  carriesAlready,
  dependentsWords,
  dispatchOf,
  dismissable,
  dismissableIn,
  displayedFor,
  fetchFailedLine,
  footLine,
  foldedLine,
  GOAL_CHANGED,
  goesOn,
  guardFor,
  lineID,
  lineState,
  liveDependentsIn,
  NEEDS_ITS_BUDGET,
  needsTheCompare,
  busyAnswering,
  coverChanged,
  newestWaitingCard,
  noRun,
  nothingDeferred,
  releaseRun,
  takeRun,
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

/** One approval as the ledger records one, for a goal read as already approved. */
const APPROVAL = {
  by: "wido",
  at: "2026-09-27T09:00:00Z",
  authority: "session",
  reviewBy: "",
  expired: false,
  expiredWhy: "",
};

/** The goal as the Partner read it, which every guarded line carries. */
const READ = {
  intent: "Fleet presence is read from the census, not polled.",
  nextStep: "Read the census.",
};

/**
 * One abandon line: the reason, the successor carrying the work, and the two live
 * goals the Partner read as waiting for this one.
 */
function abandonOf(over: Partial<Proposal> = {}): Proposal {
  return proposal({
    verb: "abandon-goal",
    fields: { because: "the seat inventory carries this now", successor: "g1-s70" },
    read: { intent: READ.intent, nextStep: READ.nextStep, tier: 2, labels: ["fleet"], dependents: ["g1-s44", "g1-s45"] },
    ...over,
  });
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
   * It is the goal action's public name under the goal object, the word the
   * button on the page that offers the act uses, so a human who has pressed
   * Pause on the Decisions page reads Pause here. It is never the route id: that
   * is what the message persists and the runner dispatches on.
   */
  it("is the page's own button word for every act", () => {
    expect(verbWord("park-goal")).toBe("Pause");
    expect(verbWord("unpark-goal")).toBe("Resume");
    expect(verbWord("withdraw-goal")).toBe("Unapprove");
    expect(verbWord("approve-goal")).toBe("Approve");
    expect(verbWord("set-goal-priority")).toBe("Prioritize");
    expect(verbWord("open-goal")).toBe("Open");
    expect(verbWord("edit-goal")).toBe("Edit");
    expect(verbWord("block-goal")).toBe("Block");
    expect(verbWord("unblock-goal")).toBe("Unblock");
    expect(verbWord("abandon-goal")).toBe("Abandon");
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

  /**
   * An abandon shows the goal's intent WHOLE, then the reason, the successor
   * where one was named, and what becomes of the goals that wait for the goal.
   *
   * The intent is first and it is the whole of it, because the heading above the
   * arguments shows the TITLE — the intent cut at its first sentence — and this
   * press ends the goal for good. The multi-sentence intent below is the case
   * that tells the two apart: the title stops at the comma-free first sentence,
   * and the second sentence exists nowhere on the card but here (g1-s64 D4, Sol
   * S64-C-01). What becomes of the dependents is the engine's rule and not the
   * page's, so the human reads it before the press rather than learning it from
   * a refusal (g1-s64 D2).
   */
  it("shows an abandon's intent whole, its reason, its successor and what waits for the goal", () => {
    const whole = "Fleet presence is read from the census, not polled. It has been for a week.";
    const line = lineOf(
      card([
        abandonOf({
          read: { intent: whole, nextStep: READ.nextStep, tier: 2, labels: ["fleet"], dependents: ["g1-s44", "g1-s45"] },
        }),
      ]),
    );

    expect(argumentsOf(line)).toEqual([
      { label: "Intent", value: whole },
      { label: "Reason", value: "the seat inventory carries this now" },
      { label: "Successor", value: "g1-s70" },
      { label: "Holds up", value: "2 goals wait for it: g1-s44, g1-s45 — they will wait for g1-s70 instead" },
    ]);
    // And the heading the card shows beside the word is only the first sentence,
    // which is the whole reason the argument above has to carry the rest.
    expect(line.title).toBe("Fleet presence is read from the census");
  });

  /** A goal nothing waits for says nothing about dependents, and names no line. */
  it("says nothing of dependents on an abandon that has none", () => {
    const alone = abandonOf({
      fields: { because: "the seat inventory carries this now", successor: "" },
      read: { intent: READ.intent, nextStep: READ.nextStep, tier: 2, labels: ["fleet"] },
    });
    expect(argumentsOf(lineOf(card([alone])))).toEqual([
      { label: "Intent", value: READ.intent },
      { label: "Reason", value: "the seat inventory carries this now" },
    ]);
  });

  /**
   * A line the service admitted without a reading says nothing about an intent,
   * rather than throwing.
   *
   * Admission carries the reading on every abandon it offers, so this is not a
   * card a human meets; it is the shape the function must survive, because a
   * renderer that threw on one line would take the whole card with it.
   */
  it("says nothing of an intent on an abandon admitted without a reading", () => {
    expect(argumentsOf(lineOf(card([abandonOf({ read: null })])))).toEqual([
      { label: "Reason", value: "the seat inventory carries this now" },
      { label: "Successor", value: "g1-s70" },
    ]);
  });

  /**
   * The sentence itself: the verb agrees with the count, the ids are named, and
   * the half after the dash is what the engine will do — repoint the dependents at
   * the successor, or refuse until each is waived or abandoned at a terminal,
   * neither of which a browser can do (Astra S64-02).
   */
  it("says how many goals wait for the goal and what becomes of them", () => {
    const said = (dependents: string[] | undefined, successor: string) =>
      dependentsWords(
        lineOf(
          card([
            abandonOf({
              fields: { because: "the seat inventory carries this now", successor },
              read: { intent: READ.intent, nextStep: READ.nextStep, tier: 2, labels: ["fleet"], dependents },
            }),
          ]),
        ),
      );

    expect(said(["g1-s44", "g1-s45"], "g1-s70")).toBe(
      "2 goals wait for it: g1-s44, g1-s45 — they will wait for g1-s70 instead",
    );
    expect(said(["g1-s44", "g1-s45"], "")).toBe(
      "2 goals wait for it: g1-s44, g1-s45 — " +
        "the engine will refuse until they are waived or abandoned at a terminal",
    );
    expect(said(["g1-s44"], "g1-s70")).toBe("1 goal waits for it: g1-s44 — they will wait for g1-s70 instead");
    expect(said(["g1-s44"], "")).toBe(
      "1 goal waits for it: g1-s44 — the engine will refuse until they are waived or abandoned at a terminal",
    );
    // A read that carries no dependents, and one that carries an empty list, both
    // say nothing: a line reading "0 goals wait for it" is an argument about an
    // absence.
    expect(said(undefined, "g1-s70")).toBe("");
    expect(said([], "g1-s70")).toBe("");
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
      { [lineID("t1", 1)]: { ticked: false, notRun: false, refusedUnsent: "", unrecorded: null } },
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
    expect(dispatchOf(lineOf(card([abandonOf()])))).toEqual({
      act: "abandon", id: "fleet-presence",
      because: "the seat inventory carries this now", successor: "g1-s70",
    });
    // The successor travels empty where nobody named one, which is the body
    // saying there is none rather than this page choosing a goal.
    expect(
      dispatchOf(
        lineOf(card([abandonOf({ fields: { because: "the seat inventory carries this now" } })])),
      ),
    ).toEqual({
      act: "abandon", id: "fleet-presence",
      because: "the seat inventory carries this now", successor: "",
    });
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

describe("what an abandon compares", () => {
  /** The goal's own row, and the two live goals whose blockers name it. */
  const tip = (dependents: readonly string[] = ["g1-s44", "g1-s45"]) => [
    row(),
    ...dependents.map((id) => row({ ref: { kind: "goal", id, revision: 2 }, blockedBy: ["fleet-presence"] })),
  ];

  /**
   * An abandon is a guarded line, and it is guarded for a reason an approve is
   * not: no press undoes it. `goal reopen` is a terminal act, so a card written
   * before somebody else moved the goal must not record that it will never be
   * worked (g1-s64 D4, Astra S64-01).
   */
  it("needs the compare, as an approve and an edit do", () => {
    expect(needsTheCompare("abandon-goal")).toBe(true);
    expect(needsTheCompare("approve-goal")).toBe(true);
    expect(needsTheCompare("edit-goal")).toBe(true);
    expect(needsTheCompare("park-goal")).toBe(false);
    expect(needsTheCompare("unpark-goal")).toBe(false);
  });

  it("sends a line whose goal and dependents have not moved", () => {
    expect(guardFor(lineOf(card([abandonOf()])), tip(), {}, "current", "")).toBe("");
    // The order the rows arrive in is not the order of the read: both are id
    // order by the time they are compared.
    expect(guardFor(lineOf(card([abandonOf()])), tip().reverse(), {}, "current", "")).toBe("");
  });

  it("refuses a line whose goal has moved, unsent", () => {
    const one = card([abandonOf()]);
    for (const moved of [
      row({ intent: "Something else entirely." }),
      row({ nextStep: "Read something else." }),
      row({ tier: 3 }),
      row({ labels: ["fleet", "browser-interface"] }),
    ]) {
      expect(guardFor(lineOf(one), [moved, ...tip().slice(1)], {}, "current", "")).toBe(GOAL_CHANGED);
    }
  });

  /**
   * The dependents are compared too, in both directions. A goal that has grown a
   * dependent since the card was written is an abandon the engine would repoint or
   * refuse, and a goal that has lost one is an act the human read as something
   * else — so neither is sent on a card that says otherwise.
   */
  it("refuses a line whose live dependents are no longer the ones it named", () => {
    const one = card([abandonOf()]);
    expect(guardFor(lineOf(one), tip(["g1-s44", "g1-s45", "g1-s46"]), {}, "current", "")).toBe(GOAL_CHANGED);
    expect(guardFor(lineOf(one), tip(["g1-s44"]), {}, "current", "")).toBe(GOAL_CHANGED);
    expect(guardFor(lineOf(one), tip(["g1-s44", "g1-s46"]), {}, "current", "")).toBe(GOAL_CHANGED);
    // And a read that named none against a tip that now has one.
    const none = card([
      abandonOf({ read: { intent: READ.intent, nextStep: READ.nextStep, tier: 2, labels: ["fleet"] } }),
    ]);
    expect(guardFor(lineOf(none), tip(["g1-s44"]), {}, "current", "")).toBe(GOAL_CHANGED);
    expect(guardFor(lineOf(none), [row()], {}, "current", "")).toBe("");
  });

  /**
   * The other direction of the blocked relation, computed from the whole reading:
   * only the LIVE rows, because a goal already done or abandoned waits for
   * nothing, and counting one would refuse an abandon the engine would admit.
   */
  it("counts the live dependents and no others, in id order", () => {
    const rows = [
      ...tip(["g1-s45", "g1-s44"]),
      row({ ref: { kind: "goal", id: "g1-s12", revision: 4 }, where: "closed", blockedBy: ["fleet-presence"] }),
      row({ ref: { kind: "goal", id: "g1-s13", revision: 4 }, blockedBy: ["something-else"] }),
    ];
    expect(liveDependentsIn(rows, "fleet-presence")).toEqual(["g1-s44", "g1-s45"]);
    expect(liveDependentsIn(rows, "nothing-waits-for-this")).toEqual([]);
    expect(guardFor(lineOf(card([abandonOf()])), rows, {}, "current", "")).toBe("");
  });
});

describe("what a line says about where it stands", () => {
  it("says each state in the words a human reads", () => {
    expect(lineState(lineOf(card([proposal({ state: "applying" })])), true)).toBe("applying…");
    expect(lineState(lineOf(card([proposal({ state: "applied" })])))).toBe("applied");
    // The server leaves an empty `words` out of the payload, so an applied line
    // with nothing to say must read as "applied" and never as "applied; undefined".
    expect(lineState(lineOf(card([proposal({ state: "applied", words: undefined })])))).toBe("applied");
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
    const one = card([proposal()], { [lineID("t1", 0)]: { ticked: true, notRun: true, refusedUnsent: "", unrecorded: null } });
    expect(lineState(lineOf(one))).toBe(NOT_RUN);
    expect(offersContinue(one)).toBe(true);
  });

  /**
   * A line a page went away in the middle of offers Try again and can still be
   * put away; a line THIS page is running offers neither, because it is in
   * flight. Without both, a line left at `applying` would stay on the card for
   * good — which is why the outcome route admits `applying` to `dismissed`.
   */
  it("offers Try again and Dismiss on a line the page left in flight, and none while it runs", () => {
    const line = lineOf(card([proposal({ state: "applying" })]));
    expect(offersTryAgain(line)).toBe(true);
    expect(offersTryAgain(line, true)).toBe(false);
    expect(dismissable(line)).toBe(true);
    expect(dismissableIn(card([proposal({ state: "applying" }), proposal({ index: 1 })]).lines)).toBe(2);
    expect(dismissableIn(card([proposal({ state: "applied" })]).lines)).toBe(0);
    // And every settled state is past putting away: the record says what it says.
    for (const state of ["applied", "refused", "unresolved", "dismissed"] as const) {
      expect(dismissable(lineOf(card([proposal({ state })])))).toBe(false);
    }
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
    ], { [lineID("t1", 3)]: { ticked: true, notRun: false, refusedUnsent: "", unrecorded: null } });
    expect(footLine(one)).toBe("2 applied · 1 refused · 1 unresolved");
  });

  it("puts the line's own words in the composer for Ask the Partner", () => {
    const line = lineOf(card([proposal({ state: "refused", words: "goal is claimed by m1e" })]));
    expect(askLine(line)).toBe(
      "About your proposed Pause on fleet-presence — refused: goal is claimed by m1e. What else could be done?",
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

  /**
   * And the folded line's own press opens it (Astra C-03).
   *
   * The automatic fold is a presentation rule about what is in view, not a fact
   * about the card, so the human's press outranks it: a card nobody dismissed used
   * to have its id removed from a list it was never in, and the condition folded it
   * straight back — leaving its arguments and its controls unreachable. A press
   * records the expansion, and a dismissal folds it again, because putting the card
   * away is a later act of the human's own.
   */
  it("unfolds an older card the human expanded, and folds it again when it is dismissed", () => {
    const two = [
      { turn: "t1", proposals: [proposal({ index: 0 })] },
      { turn: "t2", proposals: [proposal({ index: 0, goal: "refunds" })] },
    ];
    expect(cardsIn(two, {}, {}, []).map((one) => one.folded)).toEqual([true, false]);
    expect(cardsIn(two, {}, {}, [], ["t1"]).map((one) => one.folded)).toEqual([false, false]);
    expect(cardsIn(two, {}, {}, ["t1"], ["t1"]).map((one) => one.folded)).toEqual([true, false]);
    // An expansion names one card and says nothing about any other.
    expect(cardsIn(two, {}, {}, [], ["t9"]).map((one) => one.folded)).toEqual([true, false]);
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

/**
 * What the goal already carries: each act's own effect, read off the reading a
 * run takes.
 *
 * It is asked of a line found at `applying` and of nothing else, and what it
 * decides is whether that line is settled without a send (Astra A-01). Every
 * effect below is the one the act's own route writes, so a goal that shows it is
 * a goal the act reached.
 */
describe("what the goal already carries", () => {
  const applying = (over: Partial<Proposal>, displayed: Displayeds = {}) =>
    lineOf(card([proposal({ state: "applying", ...over })], {}, displayed));

  it("reads an approval as carried only with the tuple the card displayed", () => {
    const line = applying(
      { verb: "approve-goal", fields: {} },
      { [lineID("t1", 0)]: { budget: BOX, source: "goal" } },
    );
    expect(carriesAlready(line, [row({ approved: APPROVAL, budget: BOX })])).toBe(true);
    // Another tuple is another act, which the freshness guard refuses on its own.
    expect(carriesAlready(line, [row({ approved: APPROVAL, budget: { ...BOX, attemptLimit: 9 } })])).toBe(false);
    expect(carriesAlready(line, [row({ budget: BOX })])).toBe(false);
  });

  it("reads an edit as carried where every field it names is what the goal says", () => {
    const line = applying({ verb: "edit-goal", fields: { intent: "A new intent.", labels: "fleet, presence" } });
    expect(carriesAlready(line, [row({ intent: "A new intent.", labels: ["fleet", "presence"] })])).toBe(true);
    expect(carriesAlready(line, [row({ intent: "A new intent.", labels: ["fleet"] })])).toBe(false);
    expect(carriesAlready(line, [row()])).toBe(false);
  });

  it("reads a pause, a resume, a priority, a block, an unblock and an abandon", () => {
    expect(carriesAlready(applying({}), [row({ state: "parked" })])).toBe(true);
    expect(carriesAlready(applying({}), [row()])).toBe(false);
    expect(carriesAlready(applying({ verb: "unpark-goal", fields: {} }), [row()])).toBe(true);
    expect(carriesAlready(applying({ verb: "unpark-goal", fields: {} }), [row({ state: "parked" })])).toBe(false);
    const priority = applying({ verb: "set-goal-priority", fields: { priority: "1", sequence: "3" } });
    expect(carriesAlready(priority, [row({ priority: 1, sequence: 3 })])).toBe(true);
    expect(carriesAlready(priority, [row({ priority: 1, sequence: 4 })])).toBe(false);
    const block = applying({ verb: "block-goal", fields: { blocker: "refunds" } });
    expect(carriesAlready(block, [row({ blockedBy: ["refunds"] })])).toBe(true);
    expect(carriesAlready(block, [row()])).toBe(false);
    const unblock = applying({ verb: "unblock-goal", fields: { blocker: "refunds" } });
    expect(carriesAlready(unblock, [row()])).toBe(true);
    expect(carriesAlready(unblock, [row({ blockedBy: ["refunds"] })])).toBe(false);
    expect(carriesAlready(applying({ verb: "abandon-goal", fields: {} }), [row({ state: "abandoned" })])).toBe(true);
    expect(carriesAlready(applying({ verb: "abandon-goal", fields: {} }), [row()])).toBe(false);
  });

  it("says nothing of a goal the reading has not got, or of an act it cannot read the effect of", () => {
    expect(carriesAlready(applying({}), [])).toBe(false);
    // An open and an unapprove are the two the engine itself refuses a second
    // time, so their lines are sent and answered rather than read off a row.
    expect(carriesAlready(applying({ verb: "open-goal", fields: {} }), [row()])).toBe(false);
    expect(carriesAlready(applying({ verb: "withdraw-goal", fields: {} }), [row()])).toBe(false);
  });
});

describe("the run, in order", () => {
  /** A run over these lines, with every impure step recorded rather than made. */
  function driving(
    lines: readonly Line[],
    over: Partial<{
      look: Looked;
      /** One reading per call, for a run that reads again after an act lands. */
      looks: readonly Looked[];
      /**
       * What each line's act answers. A list answers a line that is sent more
       * than once — the human signing in after a refusal that asked them to.
       */
      answers: Record<string, Answered | readonly Answered[]>;
      writes: Record<string, Written>;
    }> = {},
  ) {
    const sent: string[] = [];
    const looks: Looked[] = [];
    // The entry each line stands at, as the route would hold it.
    const entries = new Map<string, Proposal>();
    const written: { line: string; state: string; words: string; version: number }[] = [];
    const marked: { line: string; change: Partial<Mark> }[] = [];
    const reconciled: Proposal[] = [];
    const rereads: number[] = [];
    const signIns: (readonly Line[])[] = [];
    const ports: RunPorts = {
      look: () => {
        const at = looks.length;
        const read = over.looks?.[Math.min(at, (over.looks.length ?? 1) - 1)] ??
          over.look ?? { rows: [], defaults: {}, outcome: "current", message: "" };
        looks.push(read);
        return Promise.resolve(read);
      },
      record: (line, state, words) => {
        written.push({ line: line.id, state, words, version: line.version });
        const forced = over.writes?.[`${line.id}:${state}`];
        if (forced !== undefined) {
          return Promise.resolve(forced);
        }
        // The server's own compare-and-set, here: a write against a version the
        // entry has moved past is refused with the entry as it stands. Without
        // it this stub would admit a stale write and a run resuming on one would
        // look right (Sol S58-C-03).
        const standing = entries.get(line.id);
        if (standing !== undefined && standing.version !== line.version) {
          return Promise.resolve({ kind: "conflict", proposal: standing });
        }
        const moved: Proposal = { ...line, state, words, version: line.version + 1 };
        entries.set(line.id, moved);
        return Promise.resolve({ kind: "written", proposal: moved });
      },
      send: (line) => {
        const before = sent.filter((one) => one === line.id).length;
        sent.push(line.id);
        const canned = over.answers?.[line.id];
        if (canned === undefined) {
          return Promise.resolve({ kind: "applied", words: "" });
        }
        if (Array.isArray(canned)) {
          return Promise.resolve(canned[Math.min(before, canned.length - 1)]);
        }
        return Promise.resolve(canned as Answered);
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
      signIn: (rest) => {
        signIns.push(rest);
      },
    };
    return { ports, sent, written, marked, reconciled, rereads, signIns, looks };
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

  /**
   * An outcome write that fails keeps the ACT'S OWN ANSWER on the line.
   *
   * The write is a failure to record, not a failure to act: the approval
   * landed. A line that said only "the conversation could not record this" and
   * offered Try again invited a second one — and the persisted line is still at
   * `applying`, which the transition table would let a press move (Sol S58-C-02).
   */
  it("keeps the act's own answer where the conversation could not record it", async () => {
    const lines = three();
    const driven = driving(lines, {
      writes: { [`${lines[0].id}:applied`]: { kind: "failed", words: "unreachable" } },
    });
    await runProposals(lines, driven.ports);
    expect(driven.marked).toContainEqual({
      line: lines[0].id, change: { unrecorded: { state: "applied", words: "" } },
    });
    // Never the other mark: an unsent refusal is a different thing and would
    // take precedence over what the ledger did.
    expect(driven.marked.some((one) => one.change.refusedUnsent === COULD_NOT_RECORD)).toBe(false);
    // The act landed, so the run goes on: the failure was the writing down of it.
    expect(driven.sent).toEqual([lines[0].id, lines[1].id, lines[2].id]);
  });

  /**
   * And the card reads it as applied, says the record failed, and offers no way
   * to send it again.
   */
  it("shows a landed act whose record failed as applied, with no Try again", () => {
    const applied = card([proposal({ state: "applying" })], {
      [lineID("t1", 0)]: {
        ticked: true, notRun: false, refusedUnsent: "",
        unrecorded: { state: "applied", words: "" },
      },
    });
    expect(lineState(lineOf(applied))).toBe(`applied; ${COULD_NOT_RECORD}`);
    expect(offersTryAgain(lineOf(applied))).toBe(false);

    // A refusal that could not be recorded still offers it: nothing landed.
    const refusedLine = card([proposal({ state: "applying" })], {
      [lineID("t1", 0)]: {
        ticked: true, notRun: false, refusedUnsent: "",
        unrecorded: { state: "refused", words: "goal is claimed" },
      },
    });
    expect(lineState(lineOf(refusedLine))).toBe(`refused: goal is claimed; ${COULD_NOT_RECORD}`);
    expect(offersTryAgain(lineOf(refusedLine))).toBe(true);

    // And an unresolved one, as before.
    const unresolved = card([proposal({ state: "applying" })], {
      [lineID("t1", 0)]: {
        ticked: true, notRun: false, refusedUnsent: "",
        unrecorded: { state: "unresolved", words: "nobody knows" },
      },
    });
    expect(lineState(lineOf(unresolved)))
      .toBe(`unresolved: nobody knows; check the goal before trying again; ${COULD_NOT_RECORD}`);
    expect(offersTryAgain(lineOf(unresolved))).toBe(true);
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
      // Refused for want of a sign-in the first time, and applied the second —
      // which is what signing in and running on from that line looks like.
      answers: {
        [lines[1].id]: [
          { kind: "sign-in", words: NOT_APPLIED_SIGN_IN },
          { kind: "applied", words: "" },
        ],
      },
    });
    await runProposals(lines, driven.ports);
    expect(driven.sent).toEqual([lines[0].id, lines[1].id]);
    expect(driven.written).toContainEqual({
      line: lines[1].id, state: "refused", words: NOT_APPLIED_SIGN_IN, version: 2,
    });
    expect(driven.marked.filter((one) => one.change.notRun === true).map((one) => one.line))
      .toEqual([lines[2].id]);
    // What the sheet is handed is the lines from the one that asked for it, so
    // signing in runs on from there and no earlier.
    expect(driven.signIns.length).toBe(1);
    expect(driven.signIns[0].map((one) => one.id)).toEqual([lines[1].id, lines[2].id]);

    // And they carry the entries as the route left them. The line that asked for
    // the sign-in has been written twice — applying, then refused — so handing
    // back the line as the press first read it would write a stale version, meet
    // a settled conflict, and SKIP the very act the human signed in for
    // (Sol S58-C-03). The resumed run goes through the same server.
    expect(driven.signIns[0][0].version).toBe(3);
    await runProposals(driven.signIns[0], driven.ports);
    expect(driven.sent).toEqual([lines[0].id, lines[1].id, lines[1].id, lines[2].id]);
  });

  /**
   * An act of THIS run can be what changed the goal a later line is about.
   *
   * `[edit G, approve G]`: the edit lands and G's intent is B; the approval
   * displays A and used to compare against the reading taken before the run, so
   * it passed and authorised B. The run now reads again after a confirmed act,
   * before the next line that depends on what a goal says (Sol S58-C-01).
   */
  it("reads again after a confirmed act and refuses a later line whose goal it changed", async () => {
    const read = { intent: "Fleet presence is read from the census, not polled.", nextStep: "Read the census.", tier: 2, labels: ["fleet"] };
    const one = card(
      [
        proposal({ index: 0, verb: "edit-goal", fields: { intent: "Something else entirely." }, read }),
        proposal({ index: 1, verb: "approve-goal", fields: {}, read }),
      ],
      {},
      { [lineID("t1", 1)]: { budget: BOX, source: "goal" } },
    );
    const before: Looked = { rows: [row({ budget: BOX })], defaults: {}, outcome: "current", message: "" };
    const after: Looked = {
      rows: [row({ intent: "Something else entirely.", budget: BOX })],
      defaults: {}, outcome: "current", message: "",
    };
    const driven = driving(one.lines, { looks: [before, after] });
    await runProposals(one.lines, driven.ports);

    // The edit went; the approval did not, and says why without being sent.
    expect(driven.sent).toEqual([one.lines[0].id]);
    expect(driven.marked).toContainEqual({ line: one.lines[1].id, change: { refusedUnsent: GOAL_CHANGED } });
    // Two readings: one before the first line, one after the act that landed.
    expect(driven.looks.length).toBe(2);
  });

  /**
   * And an act somebody ELSE applied invalidates the reading just as one of this
   * run's own does (Astra C-01).
   *
   * `[edit G, approve G]` again, and this time the edit comes back as a conflict
   * carrying an applied entry: another tab made that very edit. The ledger has
   * moved, so the approval behind it must not compare against the reading taken
   * before the run — that reading is the intent the card displayed, and passing on
   * it would approve work the human never read.
   */
  it("reads again after an applied conflict and refuses the next line against the fresh reading", async () => {
    const read = { intent: READ.intent, nextStep: READ.nextStep, tier: 2, labels: ["fleet"] };
    const one = card(
      [
        proposal({ index: 0, verb: "edit-goal", fields: { intent: "Something else entirely." }, read }),
        proposal({ index: 1, verb: "approve-goal", fields: {}, read }),
      ],
      {},
      { [lineID("t1", 1)]: { budget: BOX, source: "goal" } },
    );
    const before: Looked = { rows: [row({ budget: BOX })], defaults: {}, outcome: "current", message: "" };
    const after: Looked = {
      rows: [row({ intent: "Something else entirely.", budget: BOX })],
      defaults: {}, outcome: "current", message: "",
    };
    const applied: Proposal = { ...one.lines[0], state: "applied", words: "", version: 4 };
    const driven = driving(one.lines, {
      looks: [before, after],
      writes: { [`${one.lines[0].id}:applying`]: { kind: "conflict", proposal: applied } },
    });
    await runProposals(one.lines, driven.ports);

    // The entry they left is shown, nothing is sent for it, and the approval is
    // refused against the reading taken after it rather than passed on the stale
    // one.
    expect(driven.reconciled).toEqual([applied]);
    expect(driven.looks.length).toBe(2);
    expect(driven.sent).toEqual([]);
    expect(driven.marked).toContainEqual({ line: one.lines[1].id, change: { refusedUnsent: GOAL_CHANGED } });
  });

  /** And it reads again only where a later line actually depends on a goal. */
  it("reads again once per confirmed act, and not at all where no later line is guarded", async () => {
    const lines = three();
    const driven = driving(lines);
    await runProposals(lines, driven.ports);
    // Three parks land and none of the lines after them compares anything.
    expect(driven.looks.length).toBe(1);
  });

  /**
   * A line found at `applying` was begun by somebody, and learning that must not
   * authorise a second act (Astra A-01).
   *
   * Tab A records `applying` and sends; tab B receives that version and offers
   * Try again, whose press used to write `applying` again and send a second act.
   * An approve applied twice is two approval records — the engine suppresses a
   * duplicate proven approval and not a session one — so the press reconciles
   * first: the fetch-first reading this run already took is looked at, and a goal
   * that already carries the act settles the line without a send.
   */
  it("records an applying line whose goal already carries the act, and sends nothing", async () => {
    const lines = card([proposal({ state: "applying" })]).lines;
    const driven = driving(lines, {
      look: { rows: [row({ state: "parked" })], defaults: {}, outcome: "current", message: "" },
    });
    await runProposals(lines, driven.ports);
    expect(driven.sent).toEqual([]);
    // One write and no act: the outcome, at the version the press read.
    expect(driven.written).toEqual([
      { line: lines[0].id, state: "applied", words: ALREADY_CARRIED, version: 1 },
    ]);
    expect(driven.marked).toContainEqual({ line: lines[0].id, change: { unrecorded: null } });
  });

  it("sends an applying line once where the goal does not carry the act", async () => {
    const lines = card([proposal({ state: "applying" })]).lines;
    const driven = driving(lines, {
      look: { rows: [row()], defaults: {}, outcome: "current", message: "" },
    });
    await runProposals(lines, driven.ports);
    expect(driven.sent).toEqual([lines[0].id]);
    expect(driven.written.map((one) => one.state)).toEqual(["applying", "applied"]);
  });

  /** Astra's own sequence: the approve of a goal another tab has just approved. */
  it("sends no second approval for a goal already approved with the tuple on the card", async () => {
    const one = card(
      [
        proposal({
          verb: "approve-goal", fields: {}, state: "applying",
          read: { intent: READ.intent, nextStep: READ.nextStep, tier: 2, labels: ["fleet"] },
        }),
      ],
      {},
      { [lineID("t1", 0)]: { budget: BOX, source: "goal" } },
    );
    const driven = driving(one.lines, {
      look: {
        rows: [row({ state: "approved", budget: BOX, approved: APPROVAL })],
        defaults: {}, outcome: "current", message: "",
      },
    });
    await runProposals(one.lines, driven.ports);
    // The freshness guard passes — the goal has not moved and its tuple is the
    // one the card displayed — and the act is still not sent.
    expect(driven.marked.some((one) => one.change.refusedUnsent === GOAL_CHANGED)).toBe(false);
    expect(driven.sent).toEqual([]);
    expect(driven.written).toEqual([
      { line: one.lines[0].id, state: "applied", words: ALREADY_CARRIED, version: 1 },
    ]);
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

describe("one press, one run", () => {
  /**
   * A second Apply while one run is in flight does nothing, and the guard is
   * taken SYNCHRONOUSLY: a press is not a render, so a flag in state would still
   * read as free in the same frame and send every line twice.
   */
  it("admits one run and refuses a second while it is in flight", () => {
    const guard = noRun();
    expect(takeRun(guard, "t1")).toBe(true);
    expect(takeRun(guard, "t1")).toBe(false);
    expect(takeRun(guard, "t2")).toBe(false);
    releaseRun(guard);
    expect(takeRun(guard, "t2")).toBe(true);
  });

  /**
   * The card's buttons sleep until the answer's terminal beat: until then there
   * is no message an outcome could be recorded on (Astra S58-04).
   */
  it("keeps a card asleep while its own answer is still being written", () => {
    const one = card([proposal()]);
    expect(busyAnswering(one, one.turn)).toBe(true);
    expect(busyAnswering(one, "another-turn")).toBe(false);
    expect(busyAnswering(one, "")).toBe(false);
  });
});

describe("a re-read asked while a sheet covers the work area", () => {
  /**
   * It waits. The content under an open sheet is what that sheet is rendered
   * over, and a read made then would unmount its columns and whatever the human
   * had typed into them (Astra S58-03).
   */
  it("is made at once where nothing covers the page", () => {
    const held = nothingDeferred();
    let reads = 0;
    askReread(held, false, () => {
      reads += 1;
    });
    expect(reads).toBe(1);
    expect(held.pending).toBe(false);
  });

  it("waits while a sheet is open, and is made when it closes", () => {
    const held = nothingDeferred();
    let reads = 0;
    const read = () => {
      reads += 1;
    };
    askReread(held, true, read);
    expect(reads).toBe(0);
    expect(held.pending).toBe(true);
    // Still covered: nothing yet.
    coverChanged(held, true, read);
    expect(reads).toBe(0);
    // The sheet closed.
    coverChanged(held, false, read);
    expect(reads).toBe(1);
    expect(held.pending).toBe(false);
  });

  it("is made once however many times the cover changes after it", () => {
    const held = nothingDeferred();
    let reads = 0;
    const read = () => {
      reads += 1;
    };
    askReread(held, true, read);
    coverChanged(held, false, read);
    coverChanged(held, false, read);
    coverChanged(held, true, read);
    coverChanged(held, false, read);
    expect(reads).toBe(1);
  });

  it("asks for nothing where nothing was asked for", () => {
    const held = nothingDeferred();
    let reads = 0;
    coverChanged(held, false, () => {
      reads += 1;
    });
    expect(reads).toBe(0);
  });
});

import { readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

import type { Proposal, ProposalState } from "./api";
import {
  ALREADY_CARRIED,
  answeredOf,
  APPLIED,
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
  IN_FLIGHT,
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

/**
 * The sentence the act layer answers a second press with, while this process is
 * executing the same act on the same goal (internal/ui/act). The page never
 * composes it: it echoes the one the refusal carried, which is what these tests
 * hand it. The code beside it is the page's own constant.
 */
const IN_FLIGHT_SAID = "this act on this goal is being applied by another press; the next read says what happened";

/** One approval as the ledger records one, for a goal read as already approved. */
const APPROVAL = {
  by: "wido",
  at: "2026-09-27T09:00:00Z",
  authority: "session",
  reviewBy: "",
  expired: false,
  expiredWhy: "",
};

/**
 * The same approval, relayed and expired: an approval the goal carries and
 * cannot be worked under. Replacing it is what the approve on the card is for.
 */
const EXPIRED = { ...APPROVAL, authority: "attorney", expired: true, expiredWhy: "the review date passed" };

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

/**
 * One held answer, at the entry version the page was holding it at: what the
 * act answered where the conversation could not write it down.
 */
const heldAt = (state: ProposalState, words: string, version: number): Mark["unrecorded"] =>
  ({ state, words, version }) as Mark["unrecorded"];

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

  /**
   * An act another press owns is answered 409 `in-flight`, before any read or
   * publish (Astra A-01). The tab that meets it holds NO result: the act is
   * somebody else's and may yet land, so the line is written `unresolved` in the
   * refusal's own sentence and never `refused` — a refusal would say nothing
   * landed, which this tab cannot know.
   *
   * And it STOPS the run, for the reason every unresolved answer does: the act
   * may be landing as this is read, and a line behind it may be about the very
   * goal it changes. The rest say "not run", and Continue reads again before it
   * compares them (Astra E-01).
   */
  it("holds no result for an act another press is applying, and stops there", () => {
    const answered = answeredOf(new BacklogError("/act", 409, IN_FLIGHT_SAID, IN_FLIGHT));
    expect(answered).toEqual({ kind: "in-flight", words: IN_FLIGHT_SAID });
    expect(recorded(answered)).toEqual({ state: "unresolved", words: IN_FLIGHT_SAID });
    expect(goesOn(answered)).toBe(false);
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

  /**
   * And it says nothing of an answer a SETTLED entry has accounted for
   * (Astra C-04, narrowed by E-03).
   *
   * The answer is held because the record could not be written, so it is the only
   * account of the act there is — until the record carries one. What carries one
   * is a settled entry: `applied`, `refused` or `dismissed`. A version increment
   * is not one. Another tab's bookkeeping moves the version without saying what
   * happened, and retiring the held answer on it hid the only explanation there
   * was and offered a retry over an act that had landed (Astra E-03). The mark is
   * the store's, so a drawer that recovered used to hand the inbox its obsolete
   * answer.
   */
  it("says nothing of a held answer once a settled entry accounts for it", () => {
    const held = (version: number): Marks => ({
      [lineID("t1", 0)]: {
        ticked: true, notRun: false, refusedUnsent: "",
        unrecorded: heldAt("refused", "goal is claimed", version),
      },
    });
    const settled = card([proposal({ state: "applied", version: 4 })], held(2));
    expect(lineState(lineOf(settled))).toBe(APPLIED);
    expect(offersTryAgain(lineOf(settled))).toBe(false);

    // Moved but unsettled: somebody has written the line since and said nothing
    // about what became of the act, so what this page holds is still the only
    // account of it.
    const moved = card([proposal({ state: "applying", version: 3 })], held(2));
    expect(lineState(lineOf(moved))).toBe(`refused: goal is claimed; ${COULD_NOT_RECORD}`);
    expect(offersTryAgain(lineOf(moved))).toBe(true);

    // And at its own version, as before.
    const standing = card([proposal({ state: "applying", version: 2 })], held(2));
    expect(lineState(lineOf(standing))).toBe(`refused: goal is claimed; ${COULD_NOT_RECORD}`);
    expect(offersTryAgain(lineOf(standing))).toBe(true);
  });

  /**
   * The end state of the two tabs below, read on the tab that holds the result
   * (Astra E-03).
   *
   * This tab's approve applied, its outcome write met the other tab's attempt, and
   * that tab then wrote `unresolved` at version 4 in the refusal's own sentence —
   * bookkeeping about a request that published nothing. The act LANDED, and this
   * held answer is the only thing anywhere that knows it: a line that read the
   * unresolved entry instead would say the opposite and offer the press that
   * approves the goal twice.
   */
  it("keeps a held result over another tab's unresolved bookkeeping", () => {
    const landed = (version: number): Marks => ({
      [lineID("t1", 0)]: {
        ticked: true, notRun: false, refusedUnsent: "",
        unrecorded: heldAt("applied", "", version),
      },
    });
    const bookkept = card([proposal({ state: "unresolved", words: IN_FLIGHT_SAID, version: 4 })], landed(2));
    expect(lineState(lineOf(bookkept))).toBe(`applied; ${COULD_NOT_RECORD}`);
    expect(offersTryAgain(lineOf(bookkept))).toBe(false);

    // And the settled entry retires it: the record carries the account now.
    const recorded = card([proposal({ state: "applied", version: 4 })], landed(2));
    expect(lineState(lineOf(recorded))).toBe(APPLIED);
    expect(offersTryAgain(lineOf(recorded))).toBe(false);
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
    // And an approval that has EXPIRED carries no approve at all: the engine's
    // own no-op requires an unexpired one (internal/goal/approval.go), so the
    // act writes a fresh approval and the goal is waiting for it (Astra D-03).
    expect(carriesAlready(line, [row({ approved: EXPIRED, budget: BOX })])).toBe(false);
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

  /**
   * An unapprove's effect is an approval that is GONE (Astra F-05).
   *
   * The engine answers the repeat of it applied under a browser session, so the
   * page reads the same effect the same way: a goal carrying no approval carries
   * this act. An EXPIRED approval is not nothing — it is an approval this act
   * would take off — so a goal carrying one is not carried and the line is sent.
   */
  it("reads an unapprove as carried where the goal carries no approval", () => {
    const line = applying({ verb: "withdraw-goal", fields: { reason: "the budget was wrong" } });
    expect(carriesAlready(line, [row()])).toBe(true);
    expect(carriesAlready(line, [row({ approved: APPROVAL })])).toBe(false);
    expect(carriesAlready(line, [row({ approved: EXPIRED })])).toBe(false);
  });

  /**
   * And an open's effect is the goal it asked for, whole (Astra F-05).
   *
   * Not an id that is taken: the whole requested effect — the intent, the next
   * step, the derived tier, the labels and both directions of the blocked
   * relation. A goal of that id that differs is somebody else's goal, or an
   * earlier one, and this act never reached it: the line is sent, and the engine
   * refuses it in its own words as it does today.
   */
  it("reads an open as carried where the goal that exists is the goal it asked for", () => {
    const fields = {
      intent: "Fleet presence is read from the census, not polled.",
      nextStep: "Read the census.",
      basis: "the census is the source", severity: "2", novelty: "3", exposure: "1", accumulation: "1",
      labels: "fleet", blockedBy: "refunds", blocks: "bank-sandbox",
    };
    const line = applying({ verb: "open-goal", fields });
    // The tier the line would SEND, derived from the answers it carries.
    const asked = dispatchOf(line);
    expect(asked?.act === "open" ? asked.goal.tier : 0).toBe(3);
    const opened = row({ tier: 3, blockedBy: ["refunds"], holds: ["bank-sandbox"] });
    expect(carriesAlready(line, [opened])).toBe(true);
    // The other direction of the relation is read as a relation, not as an order.
    expect(carriesAlready(line, [row({ tier: 3, blockedBy: ["refunds"], holds: ["bank-sandbox", "refunds"] })]))
      .toBe(false);
    // And every field of the effect is compared.
    expect(carriesAlready(line, [{ ...opened, intent: "Something else entirely." }])).toBe(false);
    expect(carriesAlready(line, [{ ...opened, nextStep: "Ask the fleet." }])).toBe(false);
    expect(carriesAlready(line, [{ ...opened, tier: 2 }])).toBe(false);
    expect(carriesAlready(line, [{ ...opened, labels: ["fleet", "presence"] }])).toBe(false);
    expect(carriesAlready(line, [{ ...opened, blockedBy: [] }])).toBe(false);
    expect(carriesAlready(line, [{ ...opened, holds: [] }])).toBe(false);
  });

  /**
   * An abandon is carried only where the successor is the one it asked for
   * (Astra F-04).
   *
   * The reading does not say which successor an abandon recorded — the row's
   * abandoned clause is who, when and why — so an abandon that asks for one is
   * never read off a row at all: it is sent, and the engine compares the successor
   * it recorded with the one asked for, answering applied where they are the same
   * and refusing in words where they are not.
   */
  it("reads an abandon as carried only where it asked for no successor", () => {
    const plain = applying({ verb: "abandon-goal", fields: { because: "overtaken" } });
    expect(carriesAlready(plain, [row({ state: "abandoned" })])).toBe(true);
    expect(carriesAlready(plain, [row()])).toBe(false);
    const carrying = applying({ verb: "abandon-goal", fields: { because: "overtaken", successor: "g1-s70" } });
    expect(carriesAlready(carrying, [row({ state: "abandoned" })])).toBe(false);
  });

  it("says nothing of a goal the reading has not got", () => {
    expect(carriesAlready(applying({}), [])).toBe(false);
    // An open of an id nothing carries is the act that MAKES the goal, so there
    // is nothing to read it off: the line is sent.
    expect(carriesAlready(applying({ verb: "open-goal", fields: {} }), [])).toBe(false);
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
      /**
       * What each write answers, by `line:state`. A list answers a write made
       * more than once at that state — the same line written by a second press,
       * after a run that stopped on the first one's refusal.
       */
      writes: Record<string, Written | readonly Written[]>;
    }> = {},
  ) {
    const sent: string[] = [];
    const looks: Looked[] = [];
    // The entry each line stands at, as the route would hold it.
    const entries = new Map<string, Proposal>();
    const written: { line: string; state: string; words: string; version: number }[] = [];
    // The attempt each write carried, in the order the writes were made.
    const attempts: string[] = [];
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
      record: (line, state, words, attempt) => {
        attempts.push(attempt);
        const key = `${line.id}:${state}`;
        const before = written.filter((one) => `${one.line}:${one.state}` === key).length;
        written.push({ line: line.id, state, words, version: line.version });
        const forced = over.writes?.[key];
        if (forced !== undefined) {
          return Promise.resolve(
            Array.isArray(forced) ? forced[Math.min(before, forced.length - 1)] : (forced as Written),
          );
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
    return { ports, sent, written, attempts, marked, reconciled, rereads, signIns, looks };
  }

  const three = () => card([proposal({ index: 0 }), proposal({ index: 1, goal: "refunds" }),
    proposal({ index: 2, goal: "bank-sandbox" })]).lines;

  /** The mark a page would hold after a run: every change it made, folded in. */
  function markAfter(
    from: Mark,
    changes: readonly { line: string; change: Partial<Mark> }[],
    id: string,
  ): Mark {
    return changes
      .filter((one) => one.line === id)
      .reduce((held, one) => ({ ...held, ...one.change }), from);
  }

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

  /**
   * A press is an attempt, and the attempt owns the line.
   *
   * Every run of this runner makes ONE attempt id and carries it on every write
   * it makes. It is what lets the server tell the press that owns a line from a
   * press that has lost it: the attempt is stored on the entry when the line
   * moves to `applying`, and a settle write carrying another one is refused with
   * the entry as it stands. The page never compares it — that compare is the
   * server's — and its whole part is one token per run.
   *
   * Continue and Try again are fresh presses, each with its own read, so each
   * mints its own: a continuation writing under the attempt of the run before it
   * would be claiming a line it never took.
   */
  it("carries one attempt on every write of a run, and a fresh one on the next press", async () => {
    const lines = three();
    const driven = driving(lines);
    await runProposals(lines, driven.ports);

    // Six writes — applying and the outcome, for each of three lines — under one
    // attempt.
    expect(driven.attempts.length).toBe(6);
    expect([...new Set(driven.attempts)]).toEqual([driven.attempts[0]]);
    // A random token of sixteen hex characters, as the entry carries it.
    expect(driven.attempts[0]).toMatch(/^[0-9a-f]{16}$/);

    // Try again on the last line, or Continue from it: another press, another
    // attempt.
    const again = driving(lines);
    await runProposals([lines[2]], again.ports);
    expect(again.attempts[0]).toMatch(/^[0-9a-f]{16}$/);
    expect(again.attempts[0]).not.toBe(driven.attempts[0]);
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
      // At the version the entry stands at: the write that would have moved it
      // is the one that failed.
      line: lines[0].id, change: { unrecorded: { state: "applied", words: "", version: 2 } },
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
        unrecorded: { state: "applied", words: "", version: 1 },
      },
    });
    expect(lineState(lineOf(applied))).toBe(`applied; ${COULD_NOT_RECORD}`);
    expect(offersTryAgain(lineOf(applied))).toBe(false);

    // A refusal that could not be recorded still offers it: nothing landed.
    const refusedLine = card([proposal({ state: "applying" })], {
      [lineID("t1", 0)]: {
        ticked: true, notRun: false, refusedUnsent: "",
        unrecorded: { state: "refused", words: "goal is claimed", version: 1 },
      },
    });
    expect(lineState(lineOf(refusedLine))).toBe(`refused: goal is claimed; ${COULD_NOT_RECORD}`);
    expect(offersTryAgain(lineOf(refusedLine))).toBe(true);

    // And an unresolved one, as before.
    const unresolved = card([proposal({ state: "applying" })], {
      [lineID("t1", 0)]: {
        ticked: true, notRun: false, refusedUnsent: "",
        unrecorded: { state: "unresolved", words: "nobody knows", version: 1 },
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

  /**
   * And an expired approval carries no approve (Astra D-03).
   *
   * The tuple and the fields are the ones the card displayed, so the freshness
   * guard passes, and the approval on the goal has expired: the engine writes a
   * fresh approval for it — its no-op requires an unexpired one — and the goal
   * is inadmissible for work until it does. A press that recorded `applied`
   * over it would leave the goal approved by nothing.
   */
  it("sends the approval where the one the goal carries has expired", async () => {
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
        rows: [row({ state: "approved", budget: BOX, approved: EXPIRED })],
        defaults: {}, outcome: "current", message: "",
      },
    });
    await runProposals(one.lines, driven.ports);

    expect(driven.marked.some((change) => change.change.refusedUnsent === GOAL_CHANGED)).toBe(false);
    expect(driven.sent).toEqual([one.lines[0].id]);
    expect(driven.written.map((each) => each.state)).toEqual(["applying", "applied"]);
  });

  /**
   * And the effect is inferred only from a canonical read that SUCCEEDED
   * (Astra D-02).
   *
   * The read answers 200 from the accepted ledger whether or not its own fetch
   * of the canonical branch landed, so a failed one can hand the run rows older
   * than the act: the cache says parked, the goal has resumed since, and the
   * park proposal would be settled `applied` for good over a goal that is
   * running. The three verbs that compare are already refused unsent by the same
   * rule; these are the other seven.
   */
  it("settles nothing from a read that failed, and sends nothing", async () => {
    const lines = card([proposal({ state: "applying" })]).lines;
    const driven = driving(lines, {
      look: {
        rows: [row({ state: "parked" })], defaults: {},
        outcome: "failed", message: "the remote refused the fetch",
      },
    });
    await runProposals(lines, driven.ports);

    expect(driven.sent).toEqual([]);
    // The record is moved off `applying`, because nothing else will settle it,
    // and it is moved to unresolved in the read's own words — never applied.
    expect(driven.written).toEqual([
      {
        line: lines[0].id, state: "unresolved",
        words: fetchFailedLine("the remote refused the fetch"), version: 1,
      },
    ]);
    expect(driven.marked).toContainEqual({
      line: lines[0].id, change: { refusedUnsent: fetchFailedLine("the remote refused the fetch") },
    });
  });

  /** A line that already says unresolved is left saying it, with the read's words. */
  it("writes nothing for a line another press owns where the read failed", async () => {
    const lines = card([proposal({ state: "unresolved", words: IN_FLIGHT_SAID, version: 3 })]).lines;
    const driven = driving(lines, {
      look: { rows: [row({ state: "parked" })], defaults: {}, outcome: "failed", message: "no route to host" },
    });
    await runProposals(lines, driven.ports);

    expect(driven.sent).toEqual([]);
    expect(driven.written).toEqual([]);
    expect(driven.marked).toContainEqual({
      line: lines[0].id, change: { refusedUnsent: fetchFailedLine("no route to host") },
    });
  });

  /**
   * An act another press owns is written `unresolved`, in the refusal's own
   * sentence, and the run STOPS there (Astra A-01, corrected by E-01).
   *
   * The act layer refuses the second request for one goal and one act at once,
   * before any read or publish, so this tab holds no result: the other press's
   * act may yet land, which is what `unresolved` says and what `refused` would
   * deny. And because it may be landing, the run stops exactly as it does at any
   * other unresolved answer — a line behind it can be about that goal, and the
   * reading every compare rests on was taken before the act. The rest say "not
   * run", and Continue reads again.
   */
  it("writes an act another press owns as unresolved, and stops the run there", async () => {
    const lines = three();
    const driven = driving(lines, {
      answers: { [lines[0].id]: answeredOf(new BacklogError("/act", 409, IN_FLIGHT_SAID, IN_FLIGHT)) },
    });
    await runProposals(lines, driven.ports);

    expect(driven.written.filter((one) => one.line === lines[0].id)).toEqual([
      { line: lines[0].id, state: "applying", words: "", version: 1 },
      { line: lines[0].id, state: "unresolved", words: IN_FLIGHT_SAID, version: 2 },
    ]);
    // Never `refused`: this tab holds no result at all for that act.
    expect(driven.written.some((one) => one.line === lines[0].id && one.state === "refused")).toBe(false);
    // Once, and nothing behind it.
    expect(driven.sent).toEqual([lines[0].id]);
    expect(driven.marked.filter((one) => one.change.notRun === true).map((one) => one.line))
      .toEqual([lines[1].id, lines[2].id]);
    // And the line offers Try again, as an unresolved line does.
    const after = card([proposal({ state: "unresolved", words: IN_FLIGHT_SAID })]);
    expect(offersTryAgain(lineOf(after))).toBe(true);
  });

  /**
   * Astra's own sequence, whole (E-01).
   *
   * An `applying` edit of G, retried, and behind it a waiting approve of G that
   * the card shows against the intent the Partner read. The edit is answered
   * `in-flight`: the other press has it, and it is that press's edit that changes
   * the very intent the approve is about. A run that went on would compare the
   * approve against the reading taken before it and approve work nobody read.
   *
   * So the run stops, the approve says "not run", and Continue — a fresh press
   * through the same runner — reads again fetch-first and refuses it unsent
   * against the goal as it now is.
   */
  it("leaves a later line about that goal for Continue, which compares it afresh", async () => {
    const read = { intent: READ.intent, nextStep: READ.nextStep, tier: 2, labels: ["fleet"] };
    const one = card(
      [
        proposal({ index: 0, verb: "edit-goal", fields: { intent: "Something else entirely." }, read, state: "applying" }),
        proposal({ index: 1, verb: "approve-goal", fields: {}, read }),
      ],
      {},
      { [lineID("t1", 1)]: { budget: BOX, source: "goal" } },
    );
    const before: Looked = { rows: [row({ budget: BOX })], defaults: {}, outcome: "current", message: "" };
    // The goal as the other press's edit leaves it.
    const after: Looked = {
      rows: [row({ intent: "Something else entirely.", budget: BOX })],
      defaults: {}, outcome: "current", message: "",
    };
    const driven = driving(one.lines, {
      looks: [before, after],
      answers: { [one.lines[0].id]: answeredOf(new BacklogError("/act", 409, IN_FLIGHT_SAID, IN_FLIGHT)) },
    });

    await runProposals(one.lines, driven.ports);

    // One read, one act attempted, and no approval sent on it.
    expect(driven.looks.length).toBe(1);
    expect(driven.sent).toEqual([one.lines[0].id]);
    expect(driven.marked).toContainEqual({ line: one.lines[1].id, change: { notRun: true } });

    // Continue: the lines the stopped run never reached, through the same runner.
    await runProposals([one.lines[1]], driven.ports);

    expect(driven.looks.length).toBe(2);
    expect(driven.sent).toEqual([one.lines[0].id]);
    expect(driven.marked).toContainEqual({ line: one.lines[1].id, change: { refusedUnsent: GOAL_CHANGED } });
  });

  /**
   * Astra's F-01 sequence, whole: the page never rebases, and it never goes on.
   *
   * This tab's edit of G is refused, and its outcome write is delayed. Another
   * press takes the line over meanwhile and edits G itself, so this tab's write
   * meets that press's `applying` entry — refused on the version, or on the
   * attempt where the entry it holds belongs to that press. Nothing of THIS press
   * was published, but the edit that press owns may be landing as the refusal is
   * read, and the line behind this one is an approve of the very goal it changes.
   *
   * So the run stops there. The refusal is held on the line, because nothing
   * wrote it down; the approve says "not run"; and Continue — a fresh press, with
   * its own attempt and its own read — refuses it unsent against the goal as it
   * now is. The run that went on approved work nobody had read.
   */
  it("stops where its outcome write met another press, and leaves the rest for Continue", async () => {
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
    // The goal as the other press's edit leaves it.
    const after: Looked = {
      rows: [row({ intent: "Something else entirely.", budget: BOX })],
      defaults: {}, outcome: "current", message: "",
    };
    // The entry as that press left it: its own attempt, applying, at a version
    // this tab's write cannot be against.
    const owned: Proposal = { ...one.lines[0], state: "applying", words: "", version: 4 };
    const driven = driving(one.lines, {
      looks: [before, after],
      answers: { [one.lines[0].id]: { kind: "refused", words: "goal is claimed" } },
      writes: { [`${one.lines[0].id}:refused`]: { kind: "conflict", proposal: owned } },
    });

    await runProposals(one.lines, driven.ports);

    // One read, one act attempted, and no approval sent behind it.
    expect(driven.looks.length).toBe(1);
    expect(driven.sent).toEqual([one.lines[0].id]);
    expect(driven.reconciled).toEqual([owned]);
    expect(driven.marked).toContainEqual({ line: one.lines[1].id, change: { notRun: true } });
    // And the refusal is this tab's own, held on the line at the version it was
    // sent under: nothing wrote it down.
    const held = markAfter(
      { ticked: true, notRun: false, refusedUnsent: "", unrecorded: null },
      driven.marked, one.lines[0].id,
    );
    expect(held.unrecorded).toEqual({ state: "refused", words: "goal is claimed", version: 2 });

    // Continue: the line the stopped run never reached, through the same runner.
    await runProposals([one.lines[1]], driven.ports);

    expect(driven.looks.length).toBe(2);
    expect(driven.sent).toEqual([one.lines[0].id]);
    expect(driven.marked).toContainEqual({ line: one.lines[1].id, change: { refusedUnsent: GOAL_CHANGED } });
  });

  /**
   * And that press reconciles by read first, as a press on a line found at
   * `applying` does: the act it was told another press owns may have landed
   * since, and sending a second one is the harm A-01 is about.
   *
   * The entry is taken before it is settled because the record admits `applied`
   * only from `applying` (internal/ui/partner/proposals.go) — two writes, and no
   * act.
   */
  it("reconciles a line another press owns before a retry sends anything", async () => {
    const lines = card([proposal({ state: "unresolved", words: IN_FLIGHT_SAID, version: 3 })]).lines;
    const driven = driving(lines, {
      look: { rows: [row({ state: "parked" })], defaults: {}, outcome: "current", message: "" },
    });
    await runProposals(lines, driven.ports);

    expect(driven.sent).toEqual([]);
    expect(driven.written).toEqual([
      { line: lines[0].id, state: "applying", words: "", version: 3 },
      { line: lines[0].id, state: "applied", words: ALREADY_CARRIED, version: 4 },
    ]);
  });

  /**
   * The page never rebases (Astra F-03).
   *
   * The tab that received an act's answer is the only thing that knows what the
   * ledger did, and it KEEPS that answer rather than writing it at a version it
   * was never sent under. An entry another press left `unresolved` is not an entry
   * nobody is executing: a third press writes exactly that while the press before
   * it is applying, and the older answer rebased onto it settled the line over an
   * act that had landed.
   *
   * So the answer is held on the line, the view is reconciled to the entry the
   * refusal carried, and the run stops there — the lines behind it say "not run"
   * and Continue is a fresh press with its own attempt and its own read.
   */
  it("never writes the act's answer again at the version another tab left", async () => {
    const lines = card([proposal({ index: 0 }), proposal({ index: 1, goal: "refunds" })]).lines;
    const held: Proposal = { ...lines[0], state: "unresolved", words: IN_FLIGHT_SAID, version: 4 };
    const driven = driving(lines, {
      writes: { [`${lines[0].id}:applied`]: { kind: "conflict", proposal: held } },
    });
    await runProposals(lines, driven.ports);

    expect(driven.sent).toEqual([lines[0].id]);
    // Two writes and no third: `applying` before the act, the outcome after it.
    expect(driven.written).toEqual([
      { line: lines[0].id, state: "applying", words: "", version: 1 },
      { line: lines[0].id, state: "applied", words: "", version: 2 },
    ]);
    expect(driven.reconciled).toEqual([held]);
    // The answer is this tab's own until the record carries an account of it.
    expect(driven.marked).toContainEqual({
      line: lines[0].id, change: { unrecorded: { state: "applied", words: "", version: 2 } },
    });
    // And the run stopped there.
    expect(driven.marked).toContainEqual({ line: lines[1].id, change: { notRun: true } });
  });

  /**
   * And it is written again only over an entry NOBODY is executing (Astra E-02).
   *
   * Astra's sequence: this tab's act is refused, its outcome write is delayed,
   * and another tab retries the line meanwhile — leaving the entry `applying`
   * under an act of its own that will succeed. An `applying` entry is not an
   * entry whose writer holds no result: that attempt owns the line and will
   * settle it. Rebasing this older refusal onto it settled the line `refused`
   * while the other attempt's act applied, and that tab then reconciled to the
   * refusal and threw its own answer away.
   *
   * So this tab reconciles to the attempt and keeps its own answer where it keeps
   * every answer the record does not carry: on the line, until the record says
   * what happened.
   */
  it("holds its own answer rather than writing it over another attempt", async () => {
    const lines = card([proposal()]).lines;
    const attempt: Proposal = { ...lines[0], state: "applying", words: "", version: 3 };
    const driven = driving(lines, {
      answers: { [lines[0].id]: { kind: "refused", words: "goal is claimed" } },
      writes: { [`${lines[0].id}:refused`]: { kind: "conflict", proposal: attempt } },
    });
    await runProposals(lines, driven.ports);

    // Two writes and no third: the older refusal is never written over the
    // attempt that owns the line.
    expect(driven.written).toEqual([
      { line: lines[0].id, state: "applying", words: "", version: 1 },
      { line: lines[0].id, state: "refused", words: "goal is claimed", version: 2 },
    ]);
    expect(driven.reconciled).toEqual([attempt]);
    // And this tab still holds what its own act answered, at the version it was
    // sent under, exactly as it does where the write could not be made at all.
    const after = markAfter(
      { ticked: true, notRun: false, refusedUnsent: "", unrecorded: null },
      driven.marked, lines[0].id,
    );
    expect(after.unrecorded).toEqual({ state: "refused", words: "goal is claimed", version: 2 });
    // And once that attempt has settled the line, the held refusal is shown
    // nowhere: the record carries the account now.
    const settled = card([proposal({ state: "applied", version: 4 })], { [lineID("t1", 0)]: after });
    expect(lineState(lineOf(settled))).toBe(APPLIED);
    expect(offersTryAgain(lineOf(settled))).toBe(false);
  });

  /** And a conflict with a SETTLED entry is reconciled to, never written over. */
  it("reconciles to a settled entry rather than writing its answer again", async () => {
    const lines = card([proposal()]).lines;
    const held: Proposal = { ...lines[0], state: "refused", words: "another tab refused it", version: 4 };
    const driven = driving(lines, {
      writes: { [`${lines[0].id}:applied`]: { kind: "conflict", proposal: held } },
    });
    await runProposals(lines, driven.ports);

    expect(driven.reconciled).toEqual([held]);
    expect(driven.written.filter((one) => one.state === "applied").length).toBe(1);
  });

  /**
   * A line that has recovered stops saying what went wrong before it (Astra C-04).
   *
   * An answer the conversation could not write down is held on the line for the
   * page's life, because it is the only thing that knows what the ledger did. Once
   * something NEWER establishes what happened — this run's own outcome written
   * down, or a settled entry somebody else left — that answer is history: the line
   * used to go on reading "refused: claimed; the conversation could not record
   * this" and offering Try again over a record that said applied.
   */
  it("clears an obsolete unrecorded answer when a later outcome is written, and ends applied", async () => {
    const stale: Mark = {
      ticked: true, notRun: false, refusedUnsent: "",
      unrecorded: { state: "refused", words: "goal is claimed", version: 1 },
    };
    const lines = card([proposal()], { [lineID("t1", 0)]: stale }).lines;
    const driven = driving(lines);
    await runProposals(lines, driven.ports);

    expect(driven.written).toEqual([
      { line: lines[0].id, state: "applying", words: "", version: 1 },
      { line: lines[0].id, state: "applied", words: "", version: 2 },
    ]);
    // And the line a page holding those marks would show: the record's own
    // answer, with no recovery offered over it.
    const after = markAfter(stale, driven.marked, lines[0].id);
    expect(after.unrecorded).toBeNull();
    const settled = card([proposal({ state: "applied" })], { [lineID("t1", 0)]: after });
    expect(lineState(lineOf(settled))).toBe(APPLIED);
    expect(offersTryAgain(lineOf(settled))).toBe(false);
  });

  it("clears it where a conflict shows somebody else settled the line", async () => {
    const stale: Mark = {
      ticked: true, notRun: false, refusedUnsent: "",
      unrecorded: { state: "refused", words: "goal is claimed", version: 1 },
    };
    const lines = card([proposal()], { [lineID("t1", 0)]: stale }).lines;
    const held: Proposal = { ...lines[0], state: "applied", words: "", version: 4 };
    const driven = driving(lines, {
      writes: { [`${lines[0].id}:applying`]: { kind: "conflict", proposal: held } },
    });
    await runProposals(lines, driven.ports);

    expect(markAfter(stale, driven.marked, lines[0].id).unrecorded).toBeNull();
  });

  /**
   * And it is KEPT where nothing newer establishes what happened. A line the
   * freshness guard refuses is not sent at all, so what the page knows about the
   * act that did go out is still the only account of it there is.
   */
  it("keeps an unrecorded answer where nothing newer establishes what happened", async () => {
    const stale: Mark = {
      ticked: true, notRun: false, refusedUnsent: "",
      unrecorded: { state: "refused", words: "goal is claimed", version: 1 },
    };
    const approve = proposal({
      verb: "approve-goal", fields: {},
      read: { intent: "not what the ledger says", nextStep: "n", tier: 2, labels: [] },
    });
    const one = card(
      [approve],
      { [lineID("t1", 0)]: stale },
      { [lineID("t1", 0)]: { budget: BOX, source: "goal" } },
    );
    const driven = driving(one.lines, {
      look: { rows: [row({ budget: BOX })], defaults: {}, outcome: "current", message: "" },
    });
    await runProposals(one.lines, driven.ports);

    expect(driven.sent).toEqual([]);
    expect(markAfter(stale, driven.marked, one.lines[0].id).unrecorded).toEqual(stale.unrecorded);
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

/**
 * Astra's two tabs on one line, whole (A-01).
 *
 * Tab A presses Apply on an approve; tab B learns the `applying` entry that
 * press wrote and presses the Try again it is offered while A's act is still
 * executing. Both runs are the production runner, over one entry and one goal,
 * with the outcome route's own compare-and-set and its own transition table —
 * so a write this test admits is a write the server admits.
 *
 * Three rules of the contract meet here. The act layer owns the act, so B's
 * request is refused at once with `in-flight`; B holds no result and writes
 * `unresolved`, never `refused`; and A's own answer, refused because B had moved
 * the entry to its OWN attempt while the act was out, is not written over that
 * attempt — B owns the line and settles it with its own answer, and A holds what
 * its act answered on its own line (Astra E-02). One approval is published, and
 * the account of it is not lost.
 */
describe("tabs pressing one line", () => {
  /** A promise this test opens by hand, so the two runs interleave with no timer. */
  function gate() {
    let open: () => void = () => undefined;
    const waited = new Promise<void>((resolve) => {
      open = resolve;
    });
    return { waited, open: () => { open(); } };
  }

  /**
   * What a line may become, from what it is: the outcome route's own table
   * (internal/ui/partner/proposals.go). `applied`, `refused` and `unresolved`
   * come only from `applying`, and a write of any other pair is refused with the
   * entry exactly as a stale version is.
   */
  const MAY: Readonly<Record<string, readonly string[]>> = {
    applying: ["waiting", "refused", "unresolved", "applying"],
    applied: ["applying"],
    refused: ["applying"],
    unresolved: ["applying"],
    dismissed: ["waiting", "refused", "unresolved", "applying"],
  };

  it("publishes the act once, and both tabs end on the applied entry", async () => {
    const read = { intent: READ.intent, nextStep: READ.nextStep, tier: 2, labels: ["fleet"] };
    const displayed: Displayeds = { [lineID("t1", 0)]: { budget: BOX, source: "goal" } };
    // The one entry, as the outcome route holds it, and the one goal.
    let entry: Proposal = proposal({ verb: "approve-goal", fields: {}, read });
    let approved = false;
    // What the act layer executed, and what it refused because this process was
    // already executing the same act on the same goal.
    const executed: string[] = [];
    const refused: string[] = [];
    const inFlight = new Set<string>();
    const asked: string[] = [];
    const wrote: string[] = [];
    const reconciled: Record<string, Proposal> = {};
    const marked: Record<string, Partial<Mark>[]> = { A: [], B: [] };
    const executing = gate();
    const answered = gate();
    const ended = gate();

    const portsOf = (tab: string): RunPorts => ({
      look: () => Promise.resolve({
        rows: [approved ? row({ budget: BOX, state: "approved", approved: APPROVAL }) : row({ budget: BOX })],
        defaults: {}, outcome: "current", message: "",
      }),
      record: async (line, state, words) => {
        // B's outcome write is made after A's run has ended, which is the
        // ordering the rule is about: A's result meets an entry B moved while
        // the act was out.
        if (tab === "B" && state !== "applying") {
          await ended.waited;
        }
        wrote.push(`${tab}:${state}@${String(line.version)}`);
        if (line.version !== entry.version || !(MAY[state] ?? []).includes(entry.state)) {
          return { kind: "conflict", proposal: entry };
        }
        entry = { ...entry, state, words, version: entry.version + 1 };
        return { kind: "written", proposal: entry };
      },
      send: async (line) => {
        asked.push(tab);
        const key = `${line.goal}:${line.verb}`;
        // The act layer, keyed by the goal and the act: a second request while
        // one is executing in this process is refused before any read or publish.
        if (inFlight.has(key)) {
          refused.push(tab);
          answered.open();
          return answeredOf(new BacklogError("/act", 409, IN_FLIGHT_SAID, IN_FLIGHT));
        }
        inFlight.add(key);
        executed.push(tab);
        executing.open();
        await answered.waited;
        approved = true;
        inFlight.delete(key);
        return { kind: "applied", words: "" };
      },
      mark: (line, change) => {
        marked[tab].push(change);
      },
      reconcile: (proposal) => {
        reconciled[tab] = proposal;
      },
      reread: () => undefined,
      signIn: () => undefined,
    });

    const first = runProposals(card([entry], {}, displayed).lines, portsOf("A"));
    await executing.waited;
    // The other tab renders the `applying` entry that press wrote, and presses
    // the Try again it is offered.
    const second = runProposals(card([entry], {}, displayed).lines, portsOf("B"));
    await first;
    ended.open();
    await second;

    // One act published, and the second request refused before it read anything.
    expect(asked).toEqual(["A", "B"]);
    expect(executed).toEqual(["A"]);
    expect(refused).toEqual(["B"]);
    // A's answer is never written over B's attempt, and B — which holds no
    // result — never writes `refused`. B's own answer settles nothing either: it
    // says the line is unresolved, which is the truth about it.
    expect(wrote).toEqual([
      "A:applying@1", "B:applying@2", "A:applied@2", "B:unresolved@3",
    ]);
    expect(wrote.some((one) => one.startsWith("B:refused"))).toBe(false);
    expect({ state: entry.state, version: entry.version }).toEqual({ state: "unresolved", version: 4 });
    // A reconciled to the attempt that owned the line, and kept the account of
    // the act that DID land: it is the only thing anywhere that has it.
    expect(reconciled.A?.state).toBe("applying");
    expect(marked.A).toContainEqual({ unrecorded: { state: "applied", words: "", version: 2 } });
    // And the entry B left offers the press that reconciles by read, which finds
    // the approval on the goal and settles the line with no second act.
    expect(offersTryAgain(lineOf(card([entry], {}, displayed)))).toBe(true);
  });

  /**
   * Astra's F-03 sequence, whole: three legitimate presses on one line.
   *
   * A's park is refused and its outcome write is delayed. B, rendering the entry
   * A left, presses the Try again it is offered, and B's act SUCCEEDS: the goal is
   * parked. C, rendering the entry B left, presses Try again while B's act is
   * still out, is refused `in-flight`, and writes the `unresolved` that is the
   * truth about its own press. Two older writes then arrive at an entry neither of
   * them owns.
   *
   * Neither is rebased onto it. `unresolved` does not say that nobody is
   * executing — C wrote it while B's act was landing — so A's refusal stays A's
   * own, on A's line, and B's applied answer stays B's own, on B's. The entry is
   * what C left, and the press it offers reconciles by READ: it finds the goal
   * parked and settles the line applied with no second act.
   *
   * The run that rebased A's refusal onto C's version settled the line `refused`
   * over a park that had landed, and B's own answer was thrown away with it.
   *
   * The route is modelled with its attempt as well as its version: a write that
   * moves the line to `applying` takes ownership, and a settle write carrying any
   * other attempt is refused with the entry.
   */
  it("never rebases an older answer onto a line another press took over", async () => {
    let entry: Proposal = proposal();
    let parked = false;
    const executed: string[] = [];
    const refusedInFlight: string[] = [];
    const inFlight = new Set<string>();
    const wrote: string[] = [];
    const reconciled: Record<string, Proposal> = {};
    const marked: Record<string, Partial<Mark>[]> = { A: [], B: [], C: [], D: [] };
    const aSent = gate();
    const aWrites = gate();
    const bExecuting = gate();
    const bActEnds = gate();

    const portsOf = (tab: string): RunPorts => ({
      look: () => Promise.resolve({
        rows: [parked ? row({ state: "parked" }) : row()],
        defaults: {}, outcome: "current", message: "",
      }),
      record: async (line, state, words, attempt) => {
        // A's outcome write is delayed: it arrives after two other presses have
        // moved the line, which is the whole of what the rule is about.
        if (tab === "A" && state !== "applying") {
          await aWrites.waited;
        }
        wrote.push(`${tab}:${state}@${String(line.version)}`);
        if (line.version !== entry.version || !(MAY[state] ?? []).includes(entry.state)) {
          return { kind: "conflict", proposal: entry };
        }
        // A write that moves the line to `applying` takes ownership of it; a
        // settle write must carry the attempt the entry holds.
        if (state === "applying") {
          entry = { ...entry, state, words, version: entry.version + 1, attempt };
          return { kind: "written", proposal: entry };
        }
        if (attempt !== entry.attempt) {
          return { kind: "conflict", proposal: entry };
        }
        entry = { ...entry, state, words, version: entry.version + 1 };
        return { kind: "written", proposal: entry };
      },
      send: async (line) => {
        const key = `${line.goal}:${line.verb}`;
        if (inFlight.has(key)) {
          refusedInFlight.push(tab);
          return answeredOf(new BacklogError("/act", 409, IN_FLIGHT_SAID, IN_FLIGHT));
        }
        // A's own act: the engine refused it, and that refusal is A's to record.
        if (tab === "A") {
          aSent.open();
          return { kind: "refused", words: "goal is claimed" };
        }
        inFlight.add(key);
        executed.push(tab);
        bExecuting.open();
        await bActEnds.waited;
        parked = true;
        inFlight.delete(key);
        return { kind: "applied", words: "" };
      },
      mark: (line, change) => {
        marked[tab].push(change);
      },
      reconcile: (proposal) => {
        reconciled[tab] = proposal;
      },
      reread: () => undefined,
      signIn: () => undefined,
    });

    const first = runProposals(card([entry]).lines, portsOf("A"));
    await aSent.waited;
    // The entry A left, pressed again by a tab that renders it.
    const second = runProposals(card([entry]).lines, portsOf("B"));
    await bExecuting.waited;
    // And once more, by a third, while B's act is out: this press is refused at
    // once and writes the unresolved that says so.
    await runProposals(card([entry]).lines, portsOf("C"));
    bActEnds.open();
    await second;
    aWrites.open();
    await first;

    // One act published, and the goal is parked by it.
    expect(executed).toEqual(["B"]);
    expect(refusedInFlight).toEqual(["C"]);
    expect(parked).toBe(true);
    // Six writes attempted; the last two are refused, and neither is written
    // again at the version that came back.
    expect(wrote).toEqual([
      "A:applying@1", "B:applying@2", "C:applying@3", "C:unresolved@4", "B:applied@3", "A:refused@2",
    ]);
    // The entry is what C left, and A never settled it refused.
    expect({ state: entry.state, version: entry.version }).toEqual({ state: "unresolved", version: 5 });
    // Each press holds its own answer, where every answer the record does not
    // carry is held.
    expect(marked.A).toContainEqual({ unrecorded: { state: "refused", words: "goal is claimed", version: 2 } });
    expect(marked.B).toContainEqual({ unrecorded: { state: "applied", words: "", version: 3 } });
    expect(reconciled.A?.state).toBe("unresolved");
    expect(reconciled.B?.state).toBe("unresolved");

    // And the press the entry C left offers converges it: a fresh attempt, a
    // fresh read, the park found on the goal, and no second act.
    expect(offersTryAgain(lineOf(card([entry])))).toBe(true);
    await runProposals(card([entry]).lines, portsOf("D"));

    expect(executed).toEqual(["B"]);
    expect(wrote.slice(6)).toEqual(["D:applying@5", "D:applied@6"]);
    expect({ state: entry.state, words: entry.words }).toEqual({ state: "applied", words: ALREADY_CARRIED });
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

/**
 * What the write that records an outcome carries, and what it reads back from a
 * refusal — out of the source.
 *
 * The body and the refusal are the one edge this suite cannot drive: nothing here
 * reaches the network, and a test that stubbed `fetch` would name it in a file the
 * cut guard scans. So the two halves of the attempt's protocol are read where they
 * are written, as the inbox's own shared-mark clearing is read (Astra C-04): the
 * attempt travels in the write's own body, and the refusal that says another press
 * owns the line carries the entry exactly as a stale version does.
 */
describe("the attempt on the outcome write", () => {
  const source = (file: string) =>
    readFileSync(path.resolve(fileURLToPath(import.meta.url), "..", file), "utf8");
  const API = source("api.ts");
  const IMPURE = source("applying.ts");

  it("carries the run's attempt in the write's own body", () => {
    const at = API.indexOf("export async function recordProposal(");
    expect(at).toBeGreaterThan(0);
    const body = API.slice(at, API.indexOf("function conflictOf", at));
    expect(body).toContain("attempt: string");
    expect(body).toContain("body.attempt = attempt");
  });

  it("holds the attempt the entry carries, as a field and nothing more", () => {
    expect(API).toContain("attempt?: string");
  });

  it("reads the entry out of the refusal that says another press owns the line", () => {
    const at = API.indexOf("function heldIn(");
    expect(at).toBeGreaterThan(0);
    const body = API.slice(at, API.indexOf("\n}", at));
    expect(body).toContain('"state"');
    expect(body).toContain('"attempt"');
  });

  it("passes it through the one write of the impure half", () => {
    const at = IMPURE.indexOf("export async function writeOutcome(");
    expect(at).toBeGreaterThan(0);
    const body = IMPURE.slice(at, IMPURE.indexOf("function reasonOf", at));
    expect(body).toContain("attempt");
    expect(body).toContain("state, words, attempt)");
  });
});

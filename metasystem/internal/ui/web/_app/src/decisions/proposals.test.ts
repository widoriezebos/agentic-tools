import { readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import type { Need, Proposed } from "./api";
import {
  dismissLine,
  lineOf,
  linesOf,
  notRunIn,
  portsFor,
  pressFor,
  proposalLine,
  rowID,
  toSend,
  turnOf,
  useProposals,
  type Through,
} from "./proposals";
import type { Proposal, ProposalState } from "../partner/api";
import { PartnerAs } from "../partner/store";
import type { Answered, Line, Looked, Mark, Marks, Written } from "../partner/proposing";
import { lineState, runProposals, TRY_AGAIN, WAS_IN_FLIGHT } from "../partner/proposing";

/**
 * A proposal as a row of the inbox, and the run one press makes.
 *
 * Two things are asserted here and nowhere else. A row is the CARD's own line:
 * the same id, the same version, the same state, so that applying from the inbox
 * and applying from the card are one act on one entry. And the run the inbox
 * makes is the card's own runner — every rule it keeps is proposing.ts's — so
 * what this file proves is the wiring: that each write names the line's own
 * answer and its own version, that a conflict shows the entry the route returned
 * and sends nothing for it, and that the page reads again when the run ends.
 */

const proposed: Proposed = {
  turn: "t7",
  index: 2,
  verb: "park-goal",
  fields: { because: "superseded by the seat inventory (g1-s42)" },
  read: null,
  explanation: "the inventory covers what these were for",
  state: "waiting",
  words: "",
  version: 1,
};

function need(over: Partial<Need> = {}, action: Partial<Proposed> = {}): Need {
  return {
    kind: "proposal",
    id: "t7/2",
    title: "The seat census answers which machines are alive",
    asked: "Pause · The seat census answers which machines are alive",
    by: "the Partner",
    since: "2026-09-26T09:00:00Z",
    deadline: "",
    silence: "it stays proposed; nothing is applied",
    recommend: "",
    where: { kind: "goal", id: "g1-s44" },
    act: "apply",
    command: "",
    row: null,
    new: true,
    words: "",
    context: "",
    owner: "",
    class: "",
    due: "",
    path: "",
    goals: [],
    proposal: { ...proposed, ...action },
    ...over,
  };
}

describe("a proposal as a row", () => {
  it("is the card's own line: the same id, action, version and words", () => {
    const line = lineOf(need());

    expect(line).not.toBeNull();
    expect({
      id: line?.id,
      index: line?.index,
      verb: line?.verb,
      goal: line?.goal,
      title: line?.title,
      version: line?.version,
      state: line?.state,
      why: line?.why,
      offered: line?.offered,
    }).toEqual({
      id: "t7#2",
      index: 2,
      verb: "park-goal",
      goal: "g1-s44",
      title: "The seat census answers which machines are alive",
      version: 1,
      state: "waiting",
      why: "the inventory covers what these were for",
      offered: true,
    });
    expect(line?.fields).toEqual({ because: "superseded by the seat inventory (g1-s42)" });
  });

  /**
   * The line is dated by the asking and carries the last write beside it.
   *
   * The row's `since` is when the action was proposed, and it stays that however
   * often the entry is written: a line dated by its last write sorted to the end
   * of its group and read "today" at the moment it started to carry a recovery
   * (Sol's read of g1-s60, deferred).
   */
  it("is dated by when it was proposed, with the last write beside it", () => {
    const answered = lineOf(need({ since: "2026-09-23T09:00:00Z" },
      { state: "refused", words: "goal g1-s44 is claimed by m2a", version: 3,
        updatedAt: "2026-09-26T11:59:00Z" }));

    expect(answered?.at).toBe("2026-09-23T09:00:00Z");
    expect(answered?.updatedAt).toBe("2026-09-26T11:59:00Z");
    // And a line nobody has written says so by carrying no write at all.
    expect(lineOf(need())?.updatedAt).toBeUndefined();
  });

  it("has no line at all on a row of another kind", () => {
    expect(lineOf(need({ kind: "approval", proposal: null }))).toBeNull();
    expect(linesOf([need({ kind: "approval", proposal: null }), need()])).toHaveLength(1);
  });

  it("names the answer it belongs to, out of the coordinate its id is", () => {
    const line = lineOf(need());

    expect(turnOf(line as Line)).toBe("t7");
    expect(rowID(proposed)).toBe("t7#2");
  });

  it("carries the tuple an approve row read when it opened, and none on other verbs", () => {
    const budget = {
      elapsedLimit: "8h", attemptLimit: 10, reservedJobMinutesLimit: 1200,
      activeJobLimit: 1, reviewRoundLimit: 3,
    };
    const displayed = { "t7#2": { budget, source: "project" as const } };

    const approving = lineOf(need({}, { verb: "approve-goal" }), {}, displayed);
    const parking = lineOf(need(), {}, displayed);

    expect(approving?.displayed).toEqual({ budget, source: "project" });
    expect(parking?.displayed).toBeNull();
  });
});

describe("the entry a run or a conflict left", () => {
  const moved: Proposal = {
    index: 2,
    verb: "park-goal",
    goal: "g1-s44",
    title: "The seat census answers which machines are alive",
    offered: true,
    state: "refused",
    words: "goal g1-s44 is claimed by m2a",
    at: "2026-09-26T10:00:00Z",
    version: 3,
  };

  it("stands over the payload while it is the newer of the two", () => {
    const line = lineOf(need(), {}, {}, { "t7#2": moved });

    expect({ state: line?.state, words: line?.words, version: line?.version }).toEqual({
      state: "refused",
      words: "goal g1-s44 is claimed by m2a",
      version: 3,
    });
  });

  it("gives way to a payload that has read the same write", () => {
    // The re-read after the run carries the write, at the same version. An entry
    // held from before it would put the row back where it was.
    const line = lineOf(
      need({}, { state: "applied", words: "", version: 4 }),
      {},
      {},
      { "t7#2": moved },
    );

    expect({ state: line?.state, version: line?.version }).toEqual({ state: "applied", version: 4 });
  });
});

describe("what a row's line says", () => {
  it("is the verb's word and the subject while nobody has answered it", () => {
    expect(proposalLine(need())).toBe("Pause · The seat census answers which machines are alive");
  });

  it("says a refusal on the line, in the engine's own words", () => {
    expect(proposalLine(need({}, { state: "refused", words: "goal g1-s44 is claimed by m2a" }))).toBe(
      "Pause · The seat census answers which machines are alive · refused: goal g1-s44 is claimed by m2a",
    );
  });

  it("says a line a page left in flight was being applied", () => {
    expect(proposalLine(need({}, { state: "applying" }))).toBe(
      `Pause · The seat census answers which machines are alive · ${WAS_IN_FLIGHT}`,
    );
  });

  it("says Apply on a waiting line and Try again on one already answered", () => {
    expect(pressFor(lineOf(need()) as Line)).toBe("Apply");
    expect(pressFor(lineOf(need({}, { state: "refused" })) as Line)).toBe("Try again");
    expect(pressFor(lineOf(need({}, { state: "unresolved" })) as Line)).toBe("Try again");
    expect(pressFor(lineOf(need({}, { state: "applying" })) as Line)).toBe("Try again");
  });
});

/**
 * A line whose act landed and whose outcome the conversation could not write
 * down (Sol S60-C-01).
 *
 * The record still says `applying`, because the write that would have said
 * otherwise failed. The act HAPPENED: an approve applied twice is two approval
 * records, so the one thing this row must never offer is the press that makes a
 * second one. It is the card's own rule, taken from the card's own function.
 */
describe("a line whose act landed and could not be written down", () => {
  const landed: Marks = {
    "t7#2": { ticked: true, notRun: false, refusedUnsent: "", unrecorded: { state: "applied", words: "", version: 2 } },
  };

  function line(): Line {
    return lineOf(need({}, { state: "applying", version: 2 }), landed) as Line;
  }

  it("offers no press at all, where a refused line offers Try again", () => {
    expect(pressFor(line())).toBe("");
    expect(pressFor(lineOf(need({}, { state: "applying", version: 2 })) as Line)).toBe("Try again");
  });

  it("says what the act answered and that the conversation could not record it", () => {
    expect(lineState(line())).toBe("applied; the conversation could not record this");
  });

  it("is not sent by any press that reaches the runner", () => {
    const waitingLine = lineOf(need({ id: "t7/3" }, { index: 3 })) as Line;

    expect(toSend([line(), waitingLine]).map((one) => one.id)).toEqual([waitingLine.id]);
    expect(toSend([line()])).toEqual([]);
  });
});

/* ------------------------------------------------------------ the run -- */

/** A run driven through the inbox's own ports, with the three edges faked. */
function driving(
  lines: readonly Line[],
  answers: {
    writes?: Record<string, Written>;
    sent?: Record<string, Answered>;
    looked?: Looked;
  } = {},
) {
  const wrote: string[] = [];
  const sent: string[] = [];
  const marks: { id: string; change: Partial<Mark> }[] = [];
  const shown: { id: string; state: string; version: number }[] = [];
  const held: Record<string, Proposal> = {};
  let reread = 0;
  let signedInFrom: readonly Line[] = [];

  const through: Through = {
    look: () =>
      Promise.resolve(
        answers.looked ?? { rows: [], defaults: {}, outcome: "current", message: "" },
      ),
    record: (turn, line, state, words) => {
      wrote.push(`${turn}:${String(line.index)}:${String(line.version)}:${state}:${words}`);
      const answered = answers.writes?.[`${line.id}:${state}`];
      return Promise.resolve(
        answered ?? {
          kind: "written",
          proposal: {
            index: line.index, verb: line.verb, goal: line.goal, title: line.title,
            offered: true, state, words, at: "2026-09-26T10:00:00Z", version: line.version + 1,
          },
        },
      );
    },
    send: (line) => {
      sent.push(line.id);
      return Promise.resolve(answers.sent?.[line.id] ?? { kind: "applied", words: "" });
    },
  };

  const ports = portsFor(
    {
      reconcile: (id, proposal) => {
        shown.push({ id, state: proposal.state, version: proposal.version });
        held[id] = proposal;
      },
      mark: (id, change) => {
        marks.push({ id, change });
      },
      reread: () => {
        reread += 1;
      },
      signIn: (rest) => {
        signedInFrom = rest;
      },
    },
    through,
  );

  return {
    ran: runProposals(lines, ports),
    wrote,
    sent,
    marks,
    shown,
    held,
    rereads: () => reread,
    signedInFrom: () => signedInFrom,
  };
}

/** Two lines of two different answers, which is what a bulk press can carry. */
function twoAnswers(): Line[] {
  return [
    lineOf(need()) as Line,
    lineOf(
      need({ id: "t8/0", where: { kind: "goal", id: "g1-s48" } }, { turn: "t8", index: 0, version: 5 }),
    ) as Line,
  ];
}

describe("the run the inbox makes", () => {
  it("writes each line under its own answer and its own version", async () => {
    const driven = driving(twoAnswers());
    await driven.ran;

    // Two writes per line: applying before the act, the outcome after it. The
    // second write goes at the version the first one left.
    expect(driven.wrote).toEqual([
      "t7:2:1:applying:",
      "t7:2:2:applied:",
      "t8:0:5:applying:",
      "t8:0:6:applied:",
    ]);
    expect(driven.sent).toEqual(["t7#2", "t8#0"]);
  });

  it("shows the entry a conflict returned and sends nothing for that line", async () => {
    const standing: Proposal = {
      index: 2, verb: "park-goal", goal: "g1-s44", title: "The seat census",
      offered: true, state: "applied", words: "", at: "2026-09-26T10:00:00Z", version: 4,
    };
    const lines = twoAnswers();
    const driven = driving(lines, {
      writes: { [`${lines[0].id}:applying`]: { kind: "conflict", proposal: standing } },
    });
    await driven.ran;

    // The entry the conflict returned, and then the run's own two writes on the
    // line it did send: everything the route returned is held.
    expect(driven.shown).toEqual([
      { id: "t7#2", state: "applied", version: 4 },
      { id: "t8#0", state: "applying", version: 6 },
      { id: "t8#0", state: "applied", version: 7 },
    ]);
    // Nothing was sent for the line somebody else settled, and the run went on.
    expect(driven.sent).toEqual(["t8#0"]);
  });

  it("stops where the entry a conflict returned is still in flight", async () => {
    const inFlight: Proposal = {
      index: 2, verb: "park-goal", goal: "g1-s44", title: "The seat census",
      offered: true, state: "applying", words: "", at: "2026-09-26T10:00:00Z", version: 4,
    };
    const lines = twoAnswers();
    const driven = driving(lines, {
      writes: { [`${lines[0].id}:applying`]: { kind: "conflict", proposal: inFlight } },
    });
    await driven.ran;

    expect(driven.shown).toEqual([{ id: "t7#2", state: "applying", version: 4 }]);
    expect(driven.sent).toEqual([]);
    expect(driven.marks.filter((one) => one.change.notRun === true).map((one) => one.id)).toEqual(["t8#0"]);
  });

  /**
   * The run's own writes are held, so a row never reads as in flight about a line
   * this page has just heard the answer for.
   *
   * The page held the entries a CONFLICT returned and not the ones its own writes
   * returned. A payload read in the middle of a run carries the `applying` that
   * run wrote a moment ago, so between the outcome write and the re-read that
   * follows it the row said the act was being applied while this very run knew it
   * had come back unresolved — and it corrected itself on that read (g1-s60, as
   * built). The card had no such window, because the store folds every write it
   * makes; this is the same fold, on the same runner.
   */
  it("holds the entry its own outcome write returned, not only a conflict's", async () => {
    const lines = [twoAnswers()[0]];
    const driven = driving(lines, {
      sent: { [lines[0].id]: { kind: "unresolved", words: "the answer was lost" } },
    });
    await driven.ran;

    expect(driven.shown).toEqual([
      { id: "t7#2", state: "applying", version: 2 },
      { id: "t7#2", state: "unresolved", version: 3 },
    ]);

    // The payload as a read taken mid-run holds it: the applying this run wrote,
    // at the version it wrote. What the page kept is newer, so the row reads the
    // answer and offers the recovery.
    const midRun = need({}, { state: "applying", version: 2 });
    const row = lineOf(midRun, {}, {}, driven.held) as Line;
    expect(row.state).toBe("unresolved");
    expect(row.version).toBe(3);
    expect(lineState(row)).toBe("unresolved: the answer was lost; check the goal before trying again");
    expect(pressFor(row)).toBe("Try again");
    // Holding nothing, the same row read that payload and said the act was in
    // flight, which is the ten milliseconds this closes.
    expect(lineState(lineOf(midRun) as Line)).toBe(WAS_IN_FLIGHT);
  });

  /**
   * A write the conversation refused holds nothing: the run's own rules decide
   * what the row says then — the act's known answer where it landed, and the
   * failure where nothing was sent — and an entry the route never returned is not
   * one to hold.
   */
  it("holds nothing for a write the conversation refused", async () => {
    const lines = [twoAnswers()[0]];
    const driven = driving(lines, {
      writes: { [`${lines[0].id}:applying`]: { kind: "failed", words: "the conversation could not be written" } },
    });
    await driven.ran;

    expect(driven.shown).toEqual([]);
    expect(driven.sent).toEqual([]);
  });

  it("asks the page to read again after a confirmed act and when the run ends", async () => {
    const driven = driving([twoAnswers()[0]]);
    await driven.ran;

    expect(driven.rereads()).toBe(2);
  });

  it("hands the sign-in sheet the lines from the one that asked for it", async () => {
    const lines = twoAnswers();
    const driven = driving(lines, {
      sent: { [lines[0].id]: { kind: "sign-in", words: "not applied: sign in to apply" } },
    });
    await driven.ran;

    expect(driven.signedInFrom().map((one) => one.id)).toEqual(["t7#2", "t8#0"]);
    // And at the version the run's own writes left — applying, then refused with
    // the words that say to sign in — so signing in resumes with the act that
    // asked for it rather than writing a version the server has moved past.
    expect(driven.signedInFrom()[0].version).toBe(3);
    expect(driven.signedInFrom()[0].state).toBe("refused");
  });
});

/**
 * A run that stopped, and the press that finishes it (Sol S60-C-02).
 *
 * The run stops at an answer that does not say what happened, because the act
 * may have landed and the next line may depend on it. The lines after it were
 * never sent — they are marked "not run" and nothing about them has moved — and
 * Continue with the rest is what sends them, through the same runner, as the
 * route last returned them.
 */
describe("what a stopped run leaves, and Continue with the rest", () => {
  /** Three lines of one press, the way a bulk sheet hands them over. */
  function three(): Line[] {
    return [
      lineOf(need()) as Line,
      lineOf(need({ id: "t7/3" }, { index: 3, version: 2 })) as Line,
      lineOf(need({ id: "t7/4" }, { index: 4, version: 1 })) as Line,
    ];
  }

  it("leaves the third not run where the second answers unresolved", async () => {
    const lines = three();
    const driven = driving(lines, {
      sent: { [lines[1].id]: { kind: "unresolved", words: "the answer was lost" } },
    });
    await driven.ran;

    expect(driven.sent).toEqual(["t7#2", "t7#3"]);
    expect(driven.marks.filter((one) => one.change.notRun === true).map((one) => one.id)).toEqual(["t7#4"]);
    // And the unresolved line keeps its own state: it is not "not run".
    expect(driven.wrote).toContain("t7:3:3:unresolved:the answer was lost");
  });

  it("offers Continue over exactly the lines nobody sent", () => {
    const marks: Marks = {
      "t7#4": { ticked: true, notRun: true, refusedUnsent: "", unrecorded: null },
    };
    const lines = [
      lineOf(need(), marks) as Line,
      lineOf(need({ id: "t7/4" }, { index: 4 }), marks) as Line,
    ];

    expect(notRunIn(lines).map((one) => one.id)).toEqual(["t7#4"]);
    // And the row says so, in the card's own words.
    expect(proposalLine(need({ id: "t7/4" }, { index: 4 }), lines[1])).toBe(
      "Pause · The seat census answers which machines are alive · not run",
    );
  });

  it("sends the remainder when Continue is pressed", async () => {
    const marks: Marks = {
      "t7#4": { ticked: true, notRun: true, refusedUnsent: "", unrecorded: null },
    };
    const rest = notRunIn([lineOf(need({ id: "t7/4" }, { index: 4, version: 3 }), marks) as Line]);

    const driven = driving(rest);
    await driven.ran;

    expect(driven.sent).toEqual(["t7#4"]);
    // At the version the route last returned, which is what the row holds.
    expect(driven.wrote).toEqual(["t7:4:3:applying:", "t7:4:4:applied:"]);
  });
});

/**
 * Dismiss: the one press of this group that publishes nothing.
 *
 * It says this human is not going to answer this proposal, and it goes at the
 * version the row rendered, so a line somebody else moved first answers with a
 * conflict and stays as they left it.
 */
describe("putting a line away", () => {
  it("writes dismissed under the line's own answer and version", async () => {
    const wrote: string[] = [];
    const through: Through = {
      look: () => Promise.resolve({ rows: [], defaults: {}, outcome: "current", message: "" }),
      record: (turn, line, state, words) => {
        wrote.push(`${turn}:${String(line.index)}:${String(line.version)}:${state}:${words}`);
        return Promise.resolve({
          kind: "written",
          proposal: {
            index: line.index, verb: line.verb, goal: line.goal, title: line.title, offered: true,
            state, words, at: "2026-09-26T10:00:00Z", version: line.version + 1,
          },
        });
      },
      send: () => Promise.resolve({ kind: "applied", words: "" }),
    };

    const answered = await dismissLine(lineOf(need({}, { state: "refused", version: 3 })) as Line, through);

    expect(wrote).toEqual(["t7:2:3:dismissed:"]);
    expect(answered.kind).toBe("written");
    expect(answered.kind === "written" ? answered.proposal.state : "").toBe("dismissed");
  });
});

/**
 * What the drawer already knows about a line, read from the inbox (Astra C-02).
 *
 * The row and the card are two readings of ONE record, and there is one thing the
 * drawer knows that the record does not: an act whose answer the conversation
 * could not write down. The entry still says `applying`, because the write that
 * would have said otherwise failed, and a row that offered Try again on it would
 * be offering to approve the same goal twice. The Partner's store stands above the
 * pages, so the inbox reads that mark there rather than keeping a second copy of
 * it.
 *
 * It is driven through the hook itself, under the store's own provider, because
 * the wiring is the finding: the rules are proposing.ts's and are proved there.
 */
describe("what the drawer already knows about a line", () => {
  /** One held answer, at the entry version the page was holding it at. */
  const heldAt = (state: ProposalState, words: string, version: number): Mark["unrecorded"] =>
    ({ state, words, version }) as Mark["unrecorded"];

  const landed: Marks = {
    "t7#2": { ticked: true, notRun: false, refusedUnsent: "", unrecorded: { state: "applied", words: "", version: 2 } },
  };

  /** One row of the inbox, as everything the page holds about it composes it. */
  function Probe({ shown }: { shown: Need }) {
    const applying = useProposals({ reread: () => undefined, signIn: () => undefined });
    const line = applying.lineOf(shown);
    return createElement(
      "p",
      null,
      line === null
        ? "no line"
        : `${lineState(line)} · press: ${pressFor(line) === "" ? "none" : pressFor(line)} · sends: ${String(toSend([line]).length)}`,
    );
  }

  function shownWith(held: Marks): string {
    return renderToStaticMarkup(
      createElement(PartnerAs, {
        held: { proposalMarks: held },
        children: createElement(Probe, { shown: need({}, { state: "applying", version: 2 }) }),
      }),
    );
  }

  it("is what the inbox says and what it admits a retry on", () => {
    const markup = shownWith(landed);

    expect(markup).toContain("applied; the conversation could not record this");
    expect(markup).toContain("press: none");
    expect(markup).toContain("sends: 0");
  });

  it("leaves a line the drawer knows nothing about offering Try again", () => {
    const markup = shownWith({});

    expect(markup).toContain(WAS_IN_FLIGHT);
    expect(markup).toContain(`press: ${TRY_AGAIN}`);
    expect(markup).toContain("sends: 1");
  });

  /**
   * And it says nothing of an answer the record has moved past (Astra C-04).
   *
   * The mark crosses from the drawer because the drawer knows what the record
   * does not — until something newer says what happened. Astra's sequence: the
   * drawer holds a refusal it could not write down at version 2, the retry that
   * followed it was written down, and the entry stands `applied` at version 4.
   * The inbox used to import that old mark whenever its own was null and offer
   * Try again over a settled entry, which is a second act to make.
   */
  it("says nothing of an answer older than the entry the record now holds", () => {
    const markup = renderToStaticMarkup(
      createElement(PartnerAs, {
        held: {
          proposalMarks: {
            "t7#2": {
              ticked: true, notRun: false, refusedUnsent: "",
              unrecorded: heldAt("refused", "goal g1-s44 is claimed by m2a", 2),
            },
          },
        },
        children: createElement(Probe, { shown: need({}, { state: "applied", version: 4 }) }),
      }),
    );

    expect(markup).toContain("applied");
    expect(markup).not.toContain("could not record");
    expect(markup).toContain("press: none");
    expect(markup).toContain("sends: 0");
  });
});

/**
 * And the clearing reaches the shared mark, not only this page's own
 * (Astra C-04).
 *
 * The version above stops an obsolete answer being READ; this is the other half
 * of the same finding — the answer is dropped where both surfaces read it, by
 * whichever of them recorded or reconciled the newer outcome. It is read out of
 * the source, as `partner/store.test.ts` reads the drawer's own callbacks,
 * because a press is the only thing that reaches this one: the hook's `mark` is
 * handed to the runner and this suite drives no run through a mounted hook.
 */
describe("what the inbox does when it records a newer outcome", () => {
  const SOURCE = readFileSync(path.resolve(fileURLToPath(import.meta.url), "..", "proposals.ts"), "utf8");

  it("drops the answer the store was holding, and not only its own copy", () => {
    const at = SOURCE.indexOf("const mark = useCallback(");
    expect(at).toBeGreaterThan(0);
    const body = SOURCE.slice(at, SOURCE.indexOf("const reconcile = useCallback(", at));
    // Its own marks first, as before.
    expect(body).toContain("setMarks(");
    // And the shared one, on the change that says something newer has spoken.
    expect(body).toContain("change.unrecorded === null");
    expect(body).toContain("clearProposalMark(id)");
  });
});

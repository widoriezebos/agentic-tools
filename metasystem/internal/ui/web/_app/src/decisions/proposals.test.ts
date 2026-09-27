import { describe, expect, it } from "vitest";

import type { Need, Proposed } from "./api";
import {
  dismissLine,
  lineOf,
  linesOf,
  portsFor,
  pressFor,
  proposalLine,
  rowID,
  toSend,
  turnOf,
  type Through,
} from "./proposals";
import type { Proposal } from "../partner/api";
import type { Answered, Line, Looked, Mark, Marks, Written } from "../partner/proposing";
import { lineState, runProposals, WAS_IN_FLIGHT } from "../partner/proposing";

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
    asked: "Not now · The seat census answers which machines are alive",
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
    expect(proposalLine(need())).toBe("Not now · The seat census answers which machines are alive");
  });

  it("says a refusal on the line, in the engine's own words", () => {
    expect(proposalLine(need({}, { state: "refused", words: "goal g1-s44 is claimed by m2a" }))).toBe(
      "Not now · The seat census answers which machines are alive · refused: goal g1-s44 is claimed by m2a",
    );
  });

  it("says a line a page left in flight was being applied", () => {
    expect(proposalLine(need({}, { state: "applying" }))).toBe(
      `Not now · The seat census answers which machines are alive · ${WAS_IN_FLIGHT}`,
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
    "t7#2": { ticked: true, notRun: false, refusedUnsent: "", unrecorded: { state: "applied", words: "" } },
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

    expect(driven.shown).toEqual([{ id: "t7#2", state: "applied", version: 4 }]);
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

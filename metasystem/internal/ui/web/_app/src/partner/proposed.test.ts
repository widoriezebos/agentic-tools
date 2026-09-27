import { readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

import type { Proposal, ProposalState } from "./api";
import { proposedFor, showsAt, waitsOnTheHuman } from "./proposed";
import { cardsIn, type Card } from "./proposing";

/**
 * What a goal's row reads out of the conversation.
 *
 * The claim under test is a claim about states: four of the six are a choice
 * still on the human and two are a record of one already made, and a row that
 * got that wrong would either shout about work that is done or say nothing
 * about a refusal nobody has answered. So the fixture is one conversation with
 * every state in it, across two answers and two goals, and each assertion is
 * about which lines come back and in what order.
 */

/** One entry, in the shape the outcome route and the snapshot both carry. */
function proposal(over: Partial<Proposal> = {}): Proposal {
  return {
    index: 0,
    verb: "park-goal",
    goal: "g1-s44",
    title: "The seat census answers which machines are alive",
    fields: { because: "superseded by the seat inventory (g1-s42)" },
    read: null,
    why: "the inventory covers what these were for",
    offered: true,
    reason: "",
    state: "waiting",
    words: "",
    at: "2026-09-26T09:00:00Z",
    version: 1,
    ...over,
  };
}

/**
 * Two answers, and in them every state a line can be in, over two goals.
 *
 * The older answer is `t1` and the newer is `t2`, which is the order the store
 * composes them in — oldest first — so that "newest first" is a claim about this
 * function rather than about the transcript.
 */
function conversation(): readonly Card[] {
  const older: Proposal[] = [
    proposal({ index: 0, state: "applied", words: "" }),
    proposal({ index: 1, goal: "refunds", verb: "approve-goal", state: "dismissed" }),
    proposal({ index: 2, verb: "withdraw-goal", state: "refused", words: "goal g1-s44 is claimed by m2a" }),
    proposal({ index: 3, goal: "refunds", verb: "edit-goal", state: "unresolved", words: "pushed; whether it landed is unresolved" }),
  ];
  const newer: Proposal[] = [
    proposal({ index: 0, state: "waiting" }),
    proposal({ index: 1, goal: "refunds", verb: "park-goal", state: "applying" }),
    // Never offered: the service refused it admission, so it is on the card with
    // its reason and it is nothing a row can ask the human about.
    proposal({ index: 2, verb: "set-goal-priority", offered: false, reason: "no such goal at the tip", state: "waiting" }),
  ];
  return cardsIn([{ turn: "t1", proposals: older }, { turn: "t2", proposals: newer }], {}, {}, []);
}

describe("which of a goal's proposed acts still wait on the human", () => {
  it("answers the four unsettled states and neither applied nor dismissed", () => {
    const states: ProposalState[] = ["waiting", "applying", "refused", "unresolved"];
    for (const state of states) {
      expect({ state, waits: waitsOnTheHuman(lineOf(proposal({ state }))) }).toEqual({ state, waits: true });
    }
    for (const state of ["applied", "dismissed"] as ProposalState[]) {
      expect({ state, waits: waitsOnTheHuman(lineOf(proposal({ state }))) }).toEqual({ state, waits: false });
    }
  });

  it("leaves out a line the service never offered, whatever its state says", () => {
    expect(waitsOnTheHuman(lineOf(proposal({ offered: false, state: "waiting" })))).toBe(false);
  });

  it("answers one goal's lines, newest answer first, with the card each is on", () => {
    const found = proposedFor(conversation(), "g1-s44");

    expect(found.map((line) => [line.card, line.index, line.state])).toEqual([
      ["t2", 0, "waiting"],
      ["t1", 2, "refused"],
    ]);
  });

  it("answers the other goal's lines and none of the first one's", () => {
    const found = proposedFor(conversation(), "refunds");

    expect(found.map((line) => [line.card, line.index, line.state])).toEqual([
      ["t2", 1, "applying"],
      ["t1", 3, "unresolved"],
    ]);
  });

  it("answers nothing for a goal whose every line is applied or dismissed", () => {
    const settled = cardsIn(
      [
        {
          turn: "t1",
          proposals: [proposal({ index: 0, state: "applied" }), proposal({ index: 1, state: "dismissed" })],
        },
      ],
      {},
      {},
      [],
    );

    expect(proposedFor(settled, "g1-s44")).toEqual([]);
  });

  it("answers nothing for a goal nothing was proposed on, and nothing for no goal", () => {
    expect(proposedFor(conversation(), "fleet-presence")).toEqual([]);
    expect(proposedFor(conversation(), "")).toEqual([]);
  });

  /**
   * A dismissed CARD is the human putting the conversation's copy away; the
   * lines under it are recorded dismissed by the same press, and it is the lines
   * that decide. A card folded because a newer answer stands under it says
   * nothing about whether its lines wait.
   */
  it("reads the lines and not whether the card is folded", () => {
    const cards = cardsIn([{ turn: "t1", proposals: [proposal()] }], {}, {}, ["t1"]);

    expect(cards[0].folded).toBe(true);
    expect(proposedFor(cards, "g1-s44").map((line) => line.state)).toEqual(["waiting"]);
  });
});

/**
 * Where the press goes, and how it gets there.
 *
 * The chip does one thing: it opens the conversation at the card where the line
 * it named is decided. Which card that is, is a function and is tested as one.
 * That the press takes the bar's own path to it, and that the transcript comes to
 * the card when it does, are two lines in two files that no static render can
 * exercise — an effect does not run in one, and there is no column here to
 * scroll — so they are read where they are written, as the hand-over's own guard
 * reads the store (g1-s52).
 */
describe("the card a chip's press opens at", () => {
  it("is the newest answer carrying a line that still waits on that goal", () => {
    expect(showsAt(conversation(), "g1-s44")).toBe("t2");
  });

  /**
   * The newest ANSWER with such a line, not the newest answer: a goal proposed
   * something in the older answer and settled in the newer one opens at the older
   * one, because that is where the line the chip counted is.
   */
  it("is an older answer where the newer one has nothing waiting about it", () => {
    const cards = cardsIn(
      [
        { turn: "t1", proposals: [proposal({ index: 0, state: "refused", words: "claimed by m2a" })] },
        { turn: "t2", proposals: [proposal({ index: 0, goal: "refunds" })] },
      ],
      {},
      {},
      [],
    );

    expect(showsAt(cards, "g1-s44")).toBe("t1");
  });

  it("is nothing where nothing waits, so the press cannot open an empty card", () => {
    expect(showsAt(conversation(), "fleet-presence")).toBe("");
    expect(showsAt([], "g1-s44")).toBe("");
  });
});

describe("how the press gets there", () => {
  const SRC = path.resolve(fileURLToPath(import.meta.url), "..", "..");
  const STORE = readFileSync(path.join(SRC, "partner", "store.tsx"), "utf8");
  const CARD = readFileSync(path.join(SRC, "partner", "Proposal.tsx"), "utf8");

  /**
   * The bar's own two writes and no third mechanism: the card the conversation is
   * showing, and the count of askings the shell opens the drawer on. A second way
   * to open at a card would be a second answer to which card is in view.
   */
  it("is the store's open-at-card path, the one the bar's count uses", () => {
    const press = STORE.slice(STORE.indexOf("const showProposedFor = useCallback"));
    const body = press.slice(0, press.indexOf("}, ["));
    expect(body).toContain("showsAt(proposals, goal)");
    expect(body).toContain("setShowing(card)");
    expect(body).toContain("setRevealed((at) => at + 1)");
    const bar = STORE.slice(STORE.indexOf("const showProposals = useCallback"));
    const barBody = bar.slice(0, bar.indexOf("}, ["));
    expect(barBody).toContain("setShowing(newest)");
    expect(barBody).toContain("setRevealed((at) => at + 1)");
  });

  /**
   * And on the focused page, which has no drawer for the count to open, the same
   * two writes bring the card up in the transcript. The count is among the
   * dependencies so that pressing the same chip twice moves the column twice.
   */
  it("brings the card up in the transcript, folded or not", () => {
    expect(CARD).toContain("bringUp(box.current);");
    expect(CARD).toContain("}, [showing, id, revealed]);");
    // Both shapes of the card take the ref: an older answer's card is folded to
    // one line while it has something waiting, and that line is what a press
    // opening at that answer is brought up to.
    expect(CARD.match(/box\.current = element;/g)).toHaveLength(2);
  });
});

/** One entry as the card's own line, which is what the states are read from. */
function lineOf(entry: Proposal) {
  return cardsIn([{ turn: "t1", proposals: [entry] }], {}, {}, [])[0].lines[0];
}

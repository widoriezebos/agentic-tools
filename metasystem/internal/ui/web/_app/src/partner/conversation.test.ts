import { describe, expect, it } from "vitest";

import type { PartnerEvent, Proposal, Snapshot } from "./api";
import {
  ANYONE,
  asked,
  busy,
  emptyStore,
  loaded,
  nameOf,
  received,
  refused,
  retrying,
  unavailable,
  type Store,
} from "./conversation";

/**
 * The conversation's own arithmetic: what a snapshot replaces, what a beat
 * adds, and which beats are dropped.
 *
 * The joining rule is the one that matters and the one that is easy to get
 * wrong. A reconnect re-reads the snapshot, which carries the running turn's
 * partial text and the sequence of the last beat that went into it; the beats
 * that arrive afterwards include ones the server sent before the reconnect,
 * and applying those twice would double the answer.
 */

const snapshot: Snapshot = {
  runtime: "claude",
  model: "claude-opus-5-5",
  human: "Wido",
  busy: false,
  turn: "",
  partial: "",
  partialSeq: 0,
  activity: null,
  doing: "",
  looked: null,
  suggestions: null,
  deposits: null,
  proposals: null,
  sitting: null,
  index: null,
  readOnly: "tools limited to Read, Glob and Grep",
  messages: null,
};

function beat(seq: number, kind: PartnerEvent["kind"], text = ""): PartnerEvent {
  return { turn: "t1", seq, kind, text, at: "2026-09-23T12:00:00Z" };
}

function running(): Store {
  return asked(loaded(emptyStore, snapshot), "t1", "k1", "which goals are ready?", { section: "Backlog", path: "/backlog" }, "2026-09-23T12:00:00Z");
}

describe("the conversation", () => {
  it("takes the runtime and the transcript from the snapshot", () => {
    const store = loaded(emptyStore, { ...snapshot, messages: [] });
    expect({ state: store.state, runtime: store.runtime, human: store.human }).toEqual({
      state: "ready",
      runtime: "claude",
      human: "Wido",
    });
    expect(busy(store)).toBe(false);
  });

  it("reads a running turn's partial text and where to join it", () => {
    const store = loaded(emptyStore, {
      ...snapshot,
      busy: true,
      turn: "t1",
      partial: "half an ",
      partialSeq: 3,
      activity: ["Read plans/goals/backlog.md"],
    });
    expect(busy(store)).toBe(true);
    expect(store.live).toEqual({
      turn: "t1",
      seq: 3,
      text: "half an ",
      activity: ["Read plans/goals/backlog.md"],
      suggestions: [],
      deposits: [],
      proposals: [],
      presents: [],
      doing: "",
      looked: [],
    });
  });

  it("accumulates the answer's text in the order it arrives", () => {
    let store = running();
    store = received(store, beat(1, "text", "Two goals "));
    store = received(store, beat(2, "text", "are ready."));
    expect(store.live.text).toBe("Two goals are ready.");
    expect(store.live.seq).toBe(2);
  });

  it("keeps every activity line, including the refusals", () => {
    let store = running();
    store = received(store, beat(1, "activity", "Read plans/goals/backlog.md"));
    store = received(store, beat(2, "activity", "Refused: the Partner reads this checkout and does nothing else — Write a file (edit)"));
    expect(store.live.activity).toEqual([
      "Read plans/goals/backlog.md",
      "Refused: the Partner reads this checkout and does nothing else — Write a file (edit)",
    ]);
  });

  it("drops a beat it has already counted, which is what makes a reconnect safe", () => {
    let store = running();
    store = received(store, beat(1, "text", "one"));
    store = received(store, beat(2, "text", "two"));
    // The reconnect re-reads the snapshot, which already holds both.
    store = loaded(store, { ...snapshot, busy: true, turn: "t1", partial: "onetwo", partialSeq: 2 });
    // And the stream replays the second beat.
    store = received(store, beat(2, "text", "two"));
    expect(store.live.text).toBe("onetwo");
    store = received(store, beat(3, "text", "three"));
    expect(store.live.text).toBe("onetwothree");
  });

  it("ends a turn on done, and keeps what it said as a complete message", () => {
    let store = running();
    store = received(store, beat(1, "text", "an answer"));
    store = received(store, beat(2, "done"));
    expect(busy(store)).toBe(false);
    const last = store.messages[store.messages.length - 1];
    expect({ role: last.role, text: last.text, outcome: last.outcome }).toEqual({
      role: "partner",
      text: "an answer",
      outcome: "complete",
    });
  });

  it("marks a stopped turn stopped and keeps its partial text", () => {
    let store = running();
    store = received(store, beat(1, "text", "half an answer"));
    store = received(store, beat(2, "stopped"));
    const last = store.messages[store.messages.length - 1];
    expect({ text: last.text, outcome: last.outcome }).toEqual({
      text: "half an answer",
      outcome: "stopped",
    });
    expect(busy(store)).toBe(false);
  });

  it("marks a failed turn failed and keeps the runtime's own words", () => {
    let store = running();
    store = received(store, beat(1, "error", "the Partner's runtime stopped answering"));
    const last = store.messages[store.messages.length - 1];
    expect({ outcome: last.outcome, detail: last.detail }).toEqual({
      outcome: "failed",
      detail: "the Partner's runtime stopped answering",
    });
  });

  // The server writes the answer down before it stops calling the turn
  // current, so a snapshot taken in that instant says both. The transcript is
  // the one to believe: waiting on the flag would wait for a beat that has
  // already been sent.
  it("does not resurrect a turn whose answer is already written down", () => {
    const store = loaded(emptyStore, {
      ...snapshot,
      busy: true,
      turn: "t1",
      partial: "an answer",
      partialSeq: 4,
      messages: [
        { id: "m1", turn: "t1", role: "human", text: "a question", at: "" },
        { id: "m2", turn: "t1", role: "partner", text: "an answer", at: "", outcome: "stopped" },
      ],
    });
    expect(busy(store)).toBe(false);
  });

  it("adopts a turn it has not heard of, which is a send from another tab", () => {
    let store = loaded(emptyStore, snapshot);
    store = received(store, { turn: "t9", seq: 1, kind: "text", text: "elsewhere", at: "" });
    expect({ turn: store.live.turn, text: store.live.text }).toEqual({ turn: "t9", text: "elsewhere" });
  });

  it("shows the human's question the moment the server accepts it", () => {
    const store = running();
    const first = store.messages[0];
    expect({ role: first.role, text: first.text, key: first.key }).toEqual({
      role: "human",
      text: "which goals are ready?",
      key: "k1",
    });
    expect(first.page?.section).toBe("Backlog");
    expect(busy(store)).toBe(true);
  });

  // The server starts answering the moment it admits a turn, so its first
  // beats can reach the page before the send's own 202 does. The question is
  // still shown in front of them, and nothing they carried is lost.
  it("keeps beats that arrived before the send's own answer", () => {
    let store = loaded(emptyStore, snapshot);
    store = received(store, beat(1, "activity", "Read plans/goals/backlog.md"));
    store = received(store, beat(2, "text", "Two goals "));
    store = asked(store, "t1", "k1", "which goals are ready?", { section: "Backlog", path: "/backlog" }, "");
    expect(store.live).toEqual({
      turn: "t1",
      seq: 2,
      text: "Two goals ",
      activity: ["Read plans/goals/backlog.md"],
      suggestions: [],
      deposits: [],
      proposals: [],
      presents: [],
      doing: "",
      looked: [],
    });
    expect(store.messages.map((message) => message.role)).toEqual(["human"]);
  });

  it("puts the question in front of an answer that finished before the send returned", () => {
    let store = loaded(emptyStore, snapshot);
    store = received(store, beat(1, "text", "a fast answer"));
    store = received(store, beat(2, "done"));
    store = asked(store, "t1", "k1", "a short question", { section: "Backlog", path: "/backlog" }, "");
    expect(store.messages.map((message) => message.role)).toEqual(["human", "partner"]);
    expect(busy(store)).toBe(false);
  });

  it("keeps a refusal verbatim with the line that installs the runtime", () => {
    const store = refused(loaded(emptyStore, snapshot), "Authentication required", "npm install -g x");
    expect({ refusal: store.refusal, install: store.install }).toEqual({
      refusal: "Authentication required",
      install: "npm install -g x",
    });
    expect(retrying(store).refusal).toBe("");
  });

  it("says when this seat has no Partner at all", () => {
    const store = unavailable(emptyStore, "no Partner runtime is configured on this seat");
    expect({ state: store.state, refusal: store.refusal }).toEqual({
      state: "unavailable",
      refusal: "no Partner runtime is configured on this seat",
    });
  });
});

/**
 * What the row above a question calls the human who asked it.
 *
 * The server answers with a name where anything knows one and with the word
 * "seat" where nothing does, so the one case worth asserting is that the word
 * is read as nobody in particular: a header row saying "seat" would be naming
 * a chair, and the row exists to say who spoke.
 */
describe("who a question is signed by", () => {
  it("is the human the server names", () => {
    expect(nameOf("Wido")).toBe("Wido");
    expect(nameOf("  Wido  ")).toBe("Wido");
    expect(nameOf(loaded(emptyStore, snapshot).human)).toBe("Wido");
  });

  it("is You where the server names nobody", () => {
    expect(nameOf("seat")).toBe(ANYONE);
    expect(nameOf("")).toBe(ANYONE);
    expect(nameOf("   ")).toBe(ANYONE);
    expect(nameOf(emptyStore.human)).toBe(ANYONE);
    expect(ANYONE).toBe("You");
  });
});

/**
 * A proposal beat for an answer the transcript already holds.
 *
 * It is not a beat of a running turn: it is what a human's press did to one
 * line, published by the outcome route so that a transcript open in another tab
 * — or the drawer beside the inbox the press came from — shows what happened
 * without reading the conversation again (g1-s60 D5). The answer has ended, so
 * the beat carries no sequence, and the rule that drops a duplicate beat of a
 * running turn must not drop this one.
 */
describe("what a press did to one proposed action, on the stream", () => {
  const waiting: Proposal = {
    index: 1,
    verb: "park-goal",
    goal: "g1-s44",
    title: "The seat census answers which machines are alive",
    fields: { because: "superseded by the seat inventory" },
    read: null,
    why: "the inventory covers what these were for",
    offered: true,
    state: "waiting",
    at: "2026-09-26T09:00:00Z",
    version: 1,
  };

  /** A transcript with one answered turn carrying two proposed actions. */
  function answered(): Store {
    return loaded(emptyStore, {
      ...snapshot,
      messages: [
        {
          id: "t1-partner",
          turn: "t1",
          role: "partner",
          text: "I have proposed them.",
          at: "2026-09-26T09:00:00Z",
          outcome: "complete",
          proposals: [{ ...waiting, index: 0, goal: "g1-s43" }, waiting],
        },
      ],
    });
  }

  function moved(store: Store, proposal: Proposal): Store {
    return received(store, {
      turn: "t1", seq: 0, kind: "proposal", text: "", at: "2026-09-26T10:00:00Z", proposal,
    });
  }

  it("folds the entry into the line it belongs to", () => {
    const store = moved(answered(), { ...waiting, state: "applied", version: 3 });

    const folded = store.messages[0].proposals ?? [];
    expect(folded.map((one) => [one.index, one.state, one.version])).toEqual([
      [0, "waiting", 1],
      [1, "applied", 3],
    ]);
    expect(store.live.turn).toBe("");
  });

  it("carries a refusal's own words onto the line", () => {
    const store = moved(answered(), {
      ...waiting, state: "refused", words: "goal g1-s44 is claimed by m2a", version: 3,
    });

    expect((store.messages[0].proposals ?? [])[1].words).toBe("goal g1-s44 is claimed by m2a");
  });

  it("is not dropped as a duplicate, however many arrive", () => {
    // Two presses on one card are two beats, both with no sequence of their own:
    // the running turn's numbering ended when the answer did.
    const once = moved(answered(), { ...waiting, state: "applying", version: 2 });
    const twice = moved(once, { ...waiting, state: "applied", version: 3 });

    expect((twice.messages[0].proposals ?? [])[1]).toEqual({ ...waiting, state: "applied", version: 3 });
  });

  it("keeps the newer entry when an older beat arrives after it", () => {
    // Outcome beats carry no sequence of their own — the answer they belong to
    // has ended — so the entry's version is the only order there is. A beat that
    // overtook a newer one would move the card back to a state the record has
    // left (Sol S60-C-03).
    const applied = moved(answered(), { ...waiting, state: "applied", version: 3 });
    const late = moved(applied, { ...waiting, state: "applying", version: 2 });

    expect((late.messages[0].proposals ?? [])[1]).toEqual({ ...waiting, state: "applied", version: 3 });
  });

  it("keeps the newer entry when a snapshot read before the write arrives after it", () => {
    // A snapshot is a reading of an instant, and the read can have been taken
    // before the write whose beat this page has already folded.
    const applied = moved(answered(), { ...waiting, state: "applied", version: 3 });

    const stale = loaded(applied, {
      ...snapshot,
      messages: [
        {
          id: "t1-partner",
          turn: "t1",
          role: "partner",
          text: "I have proposed them.",
          at: "2026-09-26T09:00:00Z",
          outcome: "complete",
          proposals: [{ ...waiting, index: 0, goal: "g1-s43" }, waiting],
        },
      ],
    });

    expect((stale.messages[0].proposals ?? [])[1]).toEqual({ ...waiting, state: "applied", version: 3 });
    // And the line nobody has moved is the server's own, untouched.
    expect((stale.messages[0].proposals ?? [])[0].state).toBe("waiting");
  });

  it("takes the server's answer whole on a first load, with nothing held to keep", () => {
    const first = loaded(emptyStore, {
      ...snapshot,
      messages: [
        {
          id: "t1-partner", turn: "t1", role: "partner", text: "I have proposed them.",
          at: "2026-09-26T09:00:00Z", outcome: "complete", proposals: [waiting],
        },
      ],
    });

    expect((first.messages[0].proposals ?? [])[0]).toEqual(waiting);
  });

  it("leaves a beat of a running turn to the running turn", () => {
    const store = received(running(), {
      turn: "t1", seq: 1, kind: "proposal", text: "", at: "2026-09-23T12:00:01Z", proposal: waiting,
    });

    expect(store.live.proposals).toEqual([waiting]);
    expect(store.messages.some((message) => message.role === "partner")).toBe(false);
  });
});

/**
 * A sitting is a conversation of its own (g1-s65 D16): the stream carries the
 * conversation on each beat, and a page shows only its own. A review's desk
 * item arrives as a present beat and waits on the running turn for the room.
 */
describe("the conversation a page is showing", () => {
  const room = "metasystem/plans/reviews/review-of-g1-s64.md";
  const beat = (conversation: string, kind: PartnerEvent["kind"], seq: number): PartnerEvent => ({
    turn: "t1", seq, kind, text: "words ", at: "2026-09-28T10:00:00Z", conversation,
  });

  it("takes the beats of its own conversation and none of another's", () => {
    const inRoom = loaded({ ...emptyStore, conversation: room }, { ...snapshot, conversation: room });
    expect(received(inRoom, beat(room, "text", 1)).live.text).toBe("words ");
    expect(received(inRoom, beat("", "text", 1))).toBe(inRoom);
    const drawer = loaded(emptyStore, snapshot);
    expect(received(drawer, beat(room, "text", 1))).toBe(drawer);
    expect(received(drawer, beat("", "text", 1)).live.text).toBe("words ");
  });

  it("carries a desk item on the running turn", () => {
    const inRoom = loaded({ ...emptyStore, conversation: room }, { ...snapshot, conversation: room });
    const shown = received(inRoom, {
      ...beat(room, "present", 1),
      present: { kind: "source", path: "internal/owner.go", from: 41, to: 88 },
    });
    expect(shown.live.presents).toEqual([{ kind: "source", path: "internal/owner.go", from: 41, to: 88 }]);
  });
});

/**
 * An act's answer is a snapshot taken when the turn was admitted, and the turn's
 * first beats can reach the page on the stream before that answer does. The
 * older snapshot must not undo them: a beat the page has already joined is
 * never sent again, so an Outcome card that arrived first would be lost until a
 * reload (found in the g1-s67 walkthrough: End in a shaping room).
 */
describe("an act's answer that is older than the beats already joined", () => {
  const room = "metasystem/plans/designs/sessions.md";
  const deposit = { kind: "outcome", text: "What it came to.", offered: true };

  it("keeps the running turn as the beats left it", () => {
    const opened = loaded({ ...emptyStore, conversation: room }, { ...snapshot, conversation: room });
    const joined = received(opened, {
      turn: "t2", seq: 2, kind: "deposit", text: "", at: "2026-09-28T10:00:00Z", conversation: room, deposit,
    });
    const answer = loaded(joined, { ...snapshot, conversation: room, busy: true, turn: "t2", partialSeq: 0 });
    expect(answer.live.turn).toBe("t2");
    expect(answer.live.deposits).toEqual([deposit]);
    expect(answer.live.seq).toBe(2);
  });

  it("still takes a snapshot that is newer than the beats, and one of another turn", () => {
    const opened = loaded({ ...emptyStore, conversation: room }, { ...snapshot, conversation: room });
    const joined = received(opened, {
      turn: "t2", seq: 1, kind: "text", text: "a", at: "2026-09-28T10:00:00Z", conversation: room,
    });
    const newer = loaded(joined, { ...snapshot, conversation: room, busy: true, turn: "t2", partialSeq: 3, partial: "abc" });
    expect(newer.live.text).toBe("abc");
    expect(newer.live.seq).toBe(3);
    const other = loaded(joined, { ...snapshot, conversation: room, busy: true, turn: "t3", partialSeq: 0 });
    expect(other.live.turn).toBe("t3");
  });
});

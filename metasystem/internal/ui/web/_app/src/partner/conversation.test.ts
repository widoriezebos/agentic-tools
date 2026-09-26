import { describe, expect, it } from "vitest";

import type { PartnerEvent, Snapshot } from "./api";
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

import type { Index, Look, Message, Outcome, PartnerEvent, Page, Snapshot } from "./api";

/**
 * The conversation, as one value the drawer and the focused page both read.
 *
 * Everything the page has to decide is decided here rather than in a
 * component: whether a turn is running, what it has said so far, which beats
 * it has already seen, and what the last refusal was. The server is the owner
 * of all five — the page only mirrors them — so the two ways they arrive are
 * both folded in here: the snapshot, read on load and on every reconnect, and
 * the live beats, which arrive on the one stream.
 *
 * Joining the two is the whole reason for the sequence. A reconnect re-reads
 * the snapshot, which carries the running turn's partial text and the sequence
 * of the last beat that went into it; a beat that arrived before that sequence
 * has already been counted and is dropped, and a beat after it is appended.
 * No beat is applied twice and none is lost to the reconnect.
 */

/** The running turn, as the page renders it. */
export type Live = {
  /** The turn's id, or "" when nothing is running. */
  turn: string;
  /** The sequence of the last beat folded in. */
  seq: number;
  text: string;
  activity: string[];
  /** What the turn is at this moment, in one line, kept nowhere afterwards. */
  doing: string;
  /** What this answer has been read from so far, in the order it was read. */
  looked: Look[];
};

export const nothingRunning: Live = { turn: "", seq: 0, text: "", activity: [], doing: "", looked: [] };

export type State = "loading" | "ready" | "unavailable";

export type Store = {
  state: State;
  runtime: string;
  model: string;
  human: string;
  readOnly: string;
  messages: Message[];
  live: Live;
  /** What an answer's names can be resolved against, from the same snapshot. */
  index: Index;
  /**
   * The last refusal, verbatim, and the line that installs the runtime where
   * the server gave one. It is shown as a Partner message in the danger
   * colour, and it is cleared by the next send.
   */
  refusal: string;
  install: string;
};

export const emptyStore: Store = {
  state: "loading",
  runtime: "",
  model: "",
  human: "",
  readOnly: "",
  messages: [],
  live: nothingRunning,
  index: { goals: [], records: [] },
  refusal: "",
  install: "",
};

/** True while a turn is running, which is what disables the composer. */
export function busy(store: Store): boolean {
  return store.live.turn !== "";
}

/**
 * The conversation as the server reads it. It replaces the transcript whole:
 * the server owns it, and a page that merged its own idea of it into the
 * server's would be a second account of the same conversation.
 */
export function loaded(store: Store, snapshot: Snapshot): Store {
  const messages = snapshot.messages ?? [];
  // A turn whose answer is already written down has ended, whatever the
  // snapshot says about it: the server writes the answer before it stops
  // calling the turn current, so this is the one instant in which the two
  // disagree. Believing the flag there would leave the page waiting for a
  // beat that has already been sent.
  const running =
    snapshot.busy && !messages.some((message) => message.turn === snapshot.turn && message.role === "partner");
  return {
    ...store,
    state: "ready",
    runtime: snapshot.runtime,
    model: snapshot.model,
    human: snapshot.human,
    readOnly: snapshot.readOnly,
    messages,
    index: snapshot.index ?? { goals: [], records: [] },
    live: running
      ? {
          turn: snapshot.turn,
          seq: snapshot.partialSeq,
          text: snapshot.partial,
          activity: snapshot.activity ?? [],
          doing: snapshot.doing,
          looked: snapshot.looked ?? [],
        }
      : nothingRunning,
  };
}

/** A Partner this seat does not have, or one that could not be admitted. */
export function unavailable(store: Store, reason: string): Store {
  return { ...store, state: "unavailable", refusal: reason, live: nothingRunning };
}

/**
 * One beat of a running turn.
 *
 * A beat for a turn this page has not heard of starts that turn, because the
 * only ways to reach one are a send from this page and a send from another
 * tab, and both are this human's. A beat at or below the sequence already
 * folded in has been counted; a duplicate is the ordinary case after a
 * reconnect, and dropping it is what makes the reconnect safe.
 */
export function received(store: Store, event: PartnerEvent): Store {
  const running = store.live.turn === event.turn ? store.live : { ...nothingRunning, turn: event.turn };
  if (event.seq <= running.seq) {
    return store;
  }
  const live = { ...running, seq: event.seq };
  switch (event.kind) {
    case "text":
      return { ...store, live: { ...live, text: live.text + event.text } };
    case "activity":
      return { ...store, live: { ...live, activity: [...live.activity, event.text] } };
    case "doing":
      return { ...store, live: { ...live, doing: event.text } };
    case "look":
      return event.look === undefined
        ? { ...store, live }
        : { ...store, live: { ...live, looked: [...live.looked, event.look] } };
    case "done":
      return settled(store, live, "complete", event);
    case "stopped":
      return settled(store, live, "stopped", event);
    case "error":
      return settled(store, live, "failed", event);
    default:
      return store;
  }
}

/**
 * The turn ended: what it said becomes a message with its outcome, and nothing
 * is running any more. The server has written the same message down, and the
 * next snapshot replaces this one with the server's; until then the page shows
 * the answer rather than an empty space where it was.
 */
function settled(store: Store, live: Live, outcome: Outcome, event: PartnerEvent): Store {
  const answered: Message = {
    id: `${live.turn}-partner`,
    turn: live.turn,
    role: "partner",
    text: live.text,
    at: event.at,
    outcome,
    detail: event.text,
    activity: live.activity,
    looked: live.looked,
  };
  return { ...store, messages: [...store.messages, answered], live: nothingRunning };
}

/**
 * A question this page has just sent and the server has accepted. It is shown
 * at once — the human wrote it and it is going.
 *
 * The turn may already have started here. The server begins answering the
 * moment it admits the turn, and its first beats travel on the stream while
 * the send's own 202 is still on its way back, so by the time this runs the
 * first activity line can already be on screen. Starting the turn empty here
 * would throw those away, and the refusal among them, so a turn that has
 * already begun is left exactly as it is.
 */
export function asked(store: Store, turn: string, key: string, text: string, page: Page, at: string): Store {
  const question: Message = { id: `${turn}-human`, turn, role: "human", text, at, key, page };
  const already = store.messages.some((message) => message.turn === turn && message.role === "human");
  // The answer can even have finished by now, on a short turn over a fast
  // runtime. Then the question belongs in front of it rather than after it,
  // and nothing is running any more.
  const answeredAt = store.messages.findIndex((message) => message.turn === turn);
  const messages = already
    ? store.messages
    : answeredAt < 0
      ? [...store.messages, question]
      : [...store.messages.slice(0, answeredAt), question, ...store.messages.slice(answeredAt)];
  const ended = store.messages.some((message) => message.turn === turn && message.role === "partner");
  return {
    ...store,
    messages,
    live: ended ? nothingRunning : store.live.turn === turn ? store.live : { ...nothingRunning, turn },
    refusal: "",
    install: "",
  };
}

/** A send the server refused, in its own words. */
export function refused(store: Store, reason: string, install: string): Store {
  return { ...store, refusal: reason, install };
}

/** A send the human is retrying: the refusal goes and the draft stays. */
export function retrying(store: Store): Store {
  return { ...store, refusal: "", install: "" };
}

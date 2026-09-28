import type {
  Present,
  Deposit,
  Index,
  Look,
  Message,
  Outcome,
  PartnerEvent,
  Page,
  Proposal,
  Sitting,
  Snapshot,
  Suggestion,
} from "./api";

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
  /**
   * What this answer has offered so far for the fields of the editor the human
   * handed over. They arrive while the answer is still arriving, which is the
   * whole point: the card is under the words before the words have finished.
   */
  suggestions: Suggestion[];
  /**
   * What this answer has offered the sitting's record so far. They arrive while
   * the answer is still arriving, for the suggestions' reason: the card is on
   * the transcript and on the table before the words have finished.
   */
  deposits: Deposit[];
  /**
   * The acts this answer has proposed so far. They arrive while the answer is
   * still arriving, which is what fills the card line by line; its buttons stay
   * asleep until the answer's terminal beat, because until then there is no
   * message for an outcome to be recorded on.
   */
  proposals: Proposal[];
  /**
   * What the running turn put on a review's desk, in the order it arrived
   * (g1-s65 D5). The room shows each unless the human stopped the walk; it is
   * kept nowhere else, because the desk's strip is what remembers.
   */
  presents: Present[];
};

export const nothingRunning: Live = {
  turn: "", seq: 0, text: "", activity: [], doing: "", looked: [], suggestions: [], deposits: [],
  proposals: [], presents: [],
};

export type State = "loading" | "ready" | "unavailable";

export type Store = {
  /**
   * Which conversation this is: a sitting's record, or "" for the human's own
   * (g1-s65 D16). A beat of another conversation is not this page's.
   */
  conversation: string;
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
   * The sitting this conversation is, or null.
   *
   * It is the server's, read from the snapshot rather than remembered here, so a
   * reload and a second tab agree about which record is under discussion.
   */
  sitting: Sitting | null;
  /**
   * The last refusal, verbatim, and the line that installs the runtime where
   * the server gave one. It is shown as a Partner message in the danger
   * colour, and it is cleared by the next send.
   */
  refusal: string;
  install: string;
};

export const emptyStore: Store = {
  conversation: "",
  state: "loading",
  runtime: "",
  model: "",
  human: "",
  readOnly: "",
  messages: [],
  live: nothingRunning,
  index: { goals: [], records: [] },
  sitting: null,
  refusal: "",
  install: "",
};

/** True while a turn is running, which is what disables the composer. */
export function busy(store: Store): boolean {
  return store.live.turn !== "";
}

/**
 * What a header row calls the human who asked.
 *
 * The server resolves whose conversation this is per request — the human
 * signed in at this browser, else the one its boot proof names, else the one
 * the seat configured — and where none of those knows a name it answers with
 * the word "seat". A row that said "seat" above a question would be naming a
 * chair rather than a person, so the word is read as nobody in particular and
 * the row says "You", which is what every interface says to someone it has not
 * been introduced to.
 */
export const ANYONE = "You";

/** The server's own word for a conversation nobody is named on. */
const UNNAMED = "seat";

export function nameOf(human: string): string {
  const named = human.trim();
  return named === "" || named === UNNAMED ? ANYONE : named;
}

/**
 * The conversation as the server reads it. It replaces the transcript whole:
 * the server owns it, and a page that merged its own idea of it into the
 * server's would be a second account of the same conversation.
 *
 * One exception, and it is not a second account: a proposed action already
 * folded at a HIGHER version stays. A snapshot is a reading of an instant, and
 * a reading taken before a write can arrive after the beat that carried it —
 * then a card that says applied would go back to saying waiting and stay there
 * until somebody read again (Sol S60-C-03). The version is the record's own
 * order, so the newer of the two is the one the record holds.
 */
export function loaded(store: Store, snapshot: Snapshot): Store {
  const messages = keptProposals(store, snapshot.messages ?? []);
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
    sitting: snapshot.sitting,
    live: running
      ? {
          turn: snapshot.turn,
          seq: snapshot.partialSeq,
          text: snapshot.partial,
          activity: snapshot.activity ?? [],
          doing: snapshot.doing,
          looked: snapshot.looked ?? [],
          suggestions: snapshot.suggestions ?? [],
          deposits: snapshot.deposits ?? [],
          proposals: snapshot.proposals ?? [],
          presents: [],
        }
      : nothingRunning,
  };
}

/**
 * The arriving transcript, with every proposed action this page already holds at
 * a higher version kept as it holds it.
 *
 * It compares versions and nothing else, because the version is what the record
 * itself orders writes by: every admitted write moves it, and the entry with the
 * higher one is the later of the two whichever arrived first. A first load has
 * nothing to keep and is handed the server's own answer untouched.
 */
function keptProposals(store: Store, arriving: Message[]): Message[] {
  if (store.messages.length === 0) {
    return arriving;
  }
  return arriving.map((message) => {
    const carried = message.proposals ?? [];
    if (carried.length === 0) {
      return message;
    }
    const held = store.messages.find((one) => one.turn === message.turn && one.role === message.role)?.proposals ?? [];
    if (held.length === 0) {
      return message;
    }
    let older = false;
    const proposals = carried.map((proposal) => {
      const mine = held.find((one) => one.index === proposal.index);
      if (mine === undefined || mine.version <= proposal.version) {
        return proposal;
      }
      older = true;
      return mine;
    });
    return older ? { ...message, proposals } : message;
  });
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
  // A beat of another conversation is that conversation's page's to show, and
  // this page's to ignore: the room and the drawer are two conversations.
  if ((event.conversation ?? "") !== store.conversation) {
    return store;
  }
  // A proposal beat for an answer the transcript already holds is not a beat of
  // a running turn at all: it is what a human's press did to one line, published
  // by the outcome route so that every open page folds it into the card it
  // belongs to (g1-s60 D5). It is read before the sequence below because it
  // carries none — the answer ended, and the turn's own numbering ended with it.
  if (event.kind === "proposal" && event.proposal !== undefined && answeredIn(store, event.turn)) {
    return proposalMoved(store, event.turn, event.proposal);
  }
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
    case "suggestion":
      return event.suggestion === undefined
        ? { ...store, live }
        : { ...store, live: { ...live, suggestions: [...live.suggestions, event.suggestion] } };
    case "deposit":
      return event.deposit === undefined
        ? { ...store, live }
        : { ...store, live: { ...live, deposits: [...live.deposits, event.deposit] } };
    case "proposal":
      return event.proposal === undefined
        ? { ...store, live }
        : { ...store, live: { ...live, proposals: [...live.proposals, event.proposal] } };
    case "present":
      return event.present === undefined
        ? { ...store, live }
        : { ...store, live: { ...live, presents: [...live.presents, event.present] } };
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

/** Whether the transcript already holds this turn's answer. */
function answeredIn(store: Store, turn: string): boolean {
  return store.messages.some((message) => message.turn === turn && message.role === "partner");
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
    suggestions: live.suggestions,
    deposits: live.deposits,
    proposals: live.proposals,
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

/**
 * One proposed action as the server now holds it, folded into the message that
 * carries it.
 *
 * It is how a press becomes what the card shows without a second read of the
 * whole conversation: the outcome route answers with the entry, and this puts it
 * where the card reads its lines from. A message the transcript no longer carries
 * — a trim took it — leaves the store exactly as it was.
 */
export function proposalMoved(store: Store, turn: string, held: Proposal): Store {
  let changed = false;
  const messages = store.messages.map((message) => {
    if (message.turn !== turn || message.role !== "partner" || (message.proposals ?? []).length === 0) {
      return message;
    }
    const proposals = (message.proposals ?? []).map((proposal) => {
      if (proposal.index !== held.index) {
        return proposal;
      }
      // An older beat leaves the newer entry where it is. Outcome beats carry no
      // sequence — the answer they belong to has ended — so the entry's own
      // version is the only order there is, and a beat that arrives after a
      // newer one would otherwise move the card back (Sol S60-C-03).
      if (held.version < proposal.version) {
        return proposal;
      }
      changed = true;
      return held;
    });
    return { ...message, proposals };
  });
  return changed ? { ...store, messages } : store;
}

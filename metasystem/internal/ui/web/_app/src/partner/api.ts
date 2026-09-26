/**
 * The Project Partner's resources, and the only place this section talks to
 * the server.
 *
 * Three requests go through the one request below: the conversation, which is
 * read when the page loads and again on every reconnect of the one stream; a
 * send, which happens when a human presses Send; and a stop, which happens
 * when they press Stop. Nothing here polls, nothing here sets a timer, and the
 * turn's own beats arrive on the notification stream rather than from here —
 * src/cuts.test.ts holds that by counting call sites.
 *
 * A complete answer is rendered through the reader's own preview route, which
 * is project/api.ts's call site and not a second one here: there is one
 * Markdown parser in this application and it is the server's.
 */

import type { SheetDraft } from "./drafting";

const PARTNER = "/api/partner";
const TURNS = "/api/partner/turns";
/** What the Partner will be given for the next question, from a capture. */
const SEEING = "/api/partner/seeing";
/** The sitting: one address starts one, and the one under it ends one. */
const SITTING = "/api/partner/sitting";
const SITTING_END = "/api/partner/sitting/end";
/** Drafting the sitting's outcome, which ends nothing. */
const SITTING_CLOSE = "/api/partner/sitting/close";
/** Stopping the running turn: the turn's id, with this after it. */
const STOP = "/stop";

/** One column of the board as the page is showing it. */
export type Lane = { id: string; title: string; total: number; goals: string[] };

/** One row of the Fleet table as the page displayed it. */
export type FleetMachine = {
  machine: string;
  standing: string;
  /** The age words the row showed, in this browser's own clock. */
  seen?: string;
  /** The sentence the Running column showed: the phase, the job and its cap. */
  phase?: string;
  /** Whether the human had this row's disclosure open. */
  open?: boolean;
  flag?: string;
  holds?: string[];
};

/**
 * The Fleet page as it was on screen: where the presence copy came from, the
 * machines shown with their standings and flags, and the goals the page said
 * need a human.
 *
 * It travels for the board's reason and it is bounded for the board's reason:
 * only the page knows what was displayed, and a capture is not a listing
 * tool. `total` is the whole the machines were taken from, so a block can say
 * what it left out.
 */
export type FleetCapture = {
  source?: string;
  fetchedAt?: string;
  problem?: string;
  machines?: FleetMachine[];
  needsYou?: string[];
  total?: number;
  /**
   * The launch cards the page was showing. The human's authorization is not
   * one of these fields and never travels: the word reaches the arming verb
   * and nothing else, which is what keeps it out of every conversation.
   */
  launches?: FleetLaunch[];
};

/** One launch card as it was displayed. */
export type FleetLaunch = {
  machine: string;
  outcome: string;
  destination?: string;
  step?: string;
  words?: string;
};

/**
 * The capture: what the page was showing at the moment a question was sent.
 *
 * It is one value for four things — the sheet that shows what the Partner will
 * be given, the question that carries it, the message that keeps it, and the
 * chip that returns to it — so none of them can be a different reading of the
 * same page. src/partner/capture.ts composes it.
 */
export type Page = {
  section: string;
  path: string;
  tab?: string;
  /** Which reading of the section was open: the board or the list. */
  view?: string;
  /** The reading the page rendered from, which is not the server's own. */
  tip?: string;
  observedAt?: string;
  /** How far back the Done lane reached, as the page spells it. */
  window?: string;
  kind?: string;
  subject?: string;
  title?: string;
  revision?: string;
  filters?: string[];
  /** The board as the page is showing it, lane by lane, after its filters. */
  lanes?: Lane[];
  /** What the open project tab lists, by each row's own key. */
  records?: string[];
  /** The fleet as the page was showing it, where the page is Fleet. */
  fleet?: FleetCapture;
  label?: string;
  /** A selected passage, whole, with where it came from. */
  quote?: string;
  quoteFrom?: string;
  quoteRevision?: string;
  quoteAnchor?: string;
  /**
   * The sheet the human had open when they asked, by the name on its head. It
   * says where the human was; it reopens nothing.
   */
  sheet?: string;
  /**
   * A sheet's fields as the human had filled them in when they offered them.
   * It is there only because they pressed "Ask about this": a sheet nobody
   * offered carries nothing but its name above.
   */
  draft?: SheetDraft;
  /** The address this capture returns to, composed by the page that made it. */
  return?: string;
  /**
   * The human's own stickies as the page was showing them: what the panel
   * showed while it stood open, and otherwise the ones about the thing on the
   * page. They travel because they are nowhere else — the notepad is outside
   * every checkout precisely so that no seat reads it.
   */
  stickies?: CapturedSticky[];
  /** How many are open, carried whether or not any sticky is. */
  stickiesOpen?: number;
};

/**
 * One sticky as a capture carries it: what it says, and what it is about.
 *
 * No id and no instants. A sticky in a capture is something the human wrote to
 * themselves and is looking at; the Partner neither acts on one nor dates one,
 * and a capture is not a copy of the notepad.
 */
export type CapturedSticky = { text: string; about: string[]; done?: boolean };

/**
 * One thing the Partner read, with how it ended.
 *
 * A count is not an account: "looked at three things" can hide a failed read
 * behind a number that sounds like success. So each entry names what was read,
 * the reading it was of, whether it was read whole, in part or not at all, and
 * enough of what came back to check the answer against.
 */
export type Look = {
  what: string;
  source?: string;
  outcome: "read" | "partial" | "failed";
  excerpt?: string;
  /** True on the page the human was looking at, which is not one of the N. */
  page?: boolean;
};

/**
 * Words the Partner offered for one field of the editor the human handed over.
 *
 * It is an offer and nothing else: no field was written, nothing was saved, and
 * the value the human typed stands until they press Use this. The card in the
 * transcript is rendered from this, and the field it names says one is waiting.
 */
export type Suggestion = {
  /**
   * The one opening of that editor this belongs to: the id the sheet minted
   * when it mounted. A suggestion belongs to an opening and not to a sheet's
   * name, so closing goal A's edit sheet and opening goal B's cannot wake it.
   */
  opening: string;
  /** The editor, as its head says it: "Edit goal". */
  editor: string;
  /** The field, as its own label says it: "Intent". */
  field: string;
  /** The field's whole new value, as the Partner wrote it. */
  text: string;
  /**
   * Whether the human was shown it as something to use.
   *
   * A refused one travels rather than being dropped: a human who asked for a
   * better wording and got no proposal has to be able to read why, where they
   * read the answer. It carries no opening, so nothing can offer to write it.
   */
  offered: boolean;
  /** Why it was not offered, in the words a human reads, or "" for one that was. */
  reason?: string;
};

/**
 * What a sitting is about: one record of this project, addressed by the path the
 * document reader serves it under.
 */
export type Subject = { kind: string; id: string; title: string };

/**
 * The sitting this conversation is, or null.
 *
 * It holds no working material: the record is the memory, and the four sections
 * of that record are what the table reads. What this says is which record, what
 * the sitting is for, and when it began.
 */
export type Sitting = { subject: Subject; purpose: string; startedAt: string };

/**
 * One entry the Partner offered the record of the sitting the human is in.
 *
 * It is an offer and nothing else: nothing was written, and Record it is the
 * human's press. A card the human edits before pressing keeps their words.
 */
export type Deposit = {
  /** fact, decision, question, case or outcome. */
  kind: string;
  /** The entry itself, as the Partner wrote it. */
  text: string;
  /** Where a fact can be checked. */
  anchor?: string;
  /** The reason the Partner heard for a decision. */
  reason?: string;
  /** What follows from leaving a question — or a case — open. */
  consequence?: string;
  /**
   * On a case: the clause it would become, as the Partner heard it, which the
   * Decide sheet opens with. A case is the one kind offered as a choice between
   * two entries, so it is the one kind with two clauses.
   */
  clause?: string;
  /**
   * The record it was admitted against, stamped by the server from the sitting.
   * The page writes an admitted deposit into that record and no other, so which
   * record it is cannot be this browser's guess.
   */
  subject?: Subject;
  /** Whether the human was shown it as something to record. */
  offered: boolean;
  /**
   * Why it was not offered, in the words a human reads. It is not called a
   * reason, because a decision's reason is one of the fields above.
   */
  notOffered?: string;
};

/** What the conversation can point at, for the links in an answer. */
export type Index = {
  goals: string[] | null;
  records: IndexedRecord[] | null;
};

export type IndexedRecord = { id?: string; path: string; title?: string; kind?: string };

/** What the server will be given for the next question, composed from a capture. */
export type Seeing = {
  label: string;
  source: string;
  /** The page's own reading, where it differs from the server's. */
  displayed: string;
  supplied: number;
  total: number;
  block: string;
};

export type Outcome = "complete" | "stopped" | "failed" | "refused";

export type Message = {
  id: string;
  turn: string;
  role: "human" | "partner";
  text: string;
  at: string;
  outcome?: Outcome;
  detail?: string;
  activity?: string[];
  /** What this answer was read from: the page first, then every tool call. */
  looked?: Look[] | null;
  /**
   * What this answer offered for the fields of the editor the human handed
   * over, in the order the server admitted them. The cards under the answer
   * are rendered from here.
   */
  suggestions?: Suggestion[] | null;
  /**
   * What this answer offered the sitting's record. They are offers kept with the
   * answer: the card a human presses Record it on is rendered from here, and the
   * record is what holds anything they pressed.
   */
  deposits?: Deposit[] | null;
  key?: string;
  page?: Page;
  /**
   * True on the one question this interface asked on the human's behalf: a
   * sitting's opening turn. It is in the transcript, so a reload does not turn
   * it into the human's own words.
   */
  interface?: boolean;
};

/** Everything the page needs to render the conversation from cold. */
export type Snapshot = {
  runtime: string;
  model: string;
  human: string;
  busy: boolean;
  turn: string;
  partial: string;
  partialSeq: number;
  activity: string[] | null;
  /** What the running turn is at, in one line, kept nowhere afterwards. */
  doing: string;
  /** What the running turn has read so far. */
  looked: Look[] | null;
  /** What the running turn has offered so far, so a reload keeps the cards. */
  suggestions: Suggestion[] | null;
  /** What the running turn has offered the sitting's record so far. */
  deposits: Deposit[] | null;
  /** The sitting this conversation is, read from the server and not remembered. */
  sitting: Sitting | null;
  /** The goals and records an answer's names can be resolved against. */
  index: Index | null;
  readOnly: string;
  messages: Message[] | null;
};

export type EventKind =
  | "text"
  | "activity"
  | "doing"
  | "look"
  | "suggestion"
  | "deposit"
  | "done"
  | "error"
  | "stopped";

/** One beat of a running turn, as the stream carries it. */
export type PartnerEvent = {
  turn: string;
  seq: number;
  kind: EventKind;
  text: string;
  at: string;
  /** One completed read, on a look beat and nowhere else. */
  look?: Look;
  /**
   * One admitted suggestion, on a suggestion beat and nowhere else. It arrives
   * as the server admits it, so the card is under the answer before the answer
   * has finished arriving.
   */
  suggestion?: Suggestion;
  /** One admitted deposit, on a deposit beat and nowhere else. */
  deposit?: Deposit;
};

/**
 * A response that was not what was asked for. The Partner's refusals carry
 * more than a sentence: a send while busy is a state the page shows rather
 * than an error, and a runtime that will not start carries the line that
 * installs it.
 */
export class PartnerError extends Error {
  readonly status: number;
  readonly install: string;
  /**
   * The record this request created before it was refused, by its path, or "".
   *
   * A sitting started on a title creates the draft before the sitting is opened,
   * so a refusal that comes after that has left a real record in the project. The
   * path travels so the page can offer Start again on that draft: a second press
   * for one wish is a second attempt, not a second record.
   */
  readonly draft: string;

  constructor(status: number, reason: string, install = "", draft = "") {
    super(reason === "" ? `the Partner answered ${String(status)}` : reason);
    this.name = "PartnerError";
    this.status = status;
    this.install = install;
    this.draft = draft;
  }
}

/** True when the server refused because a turn is already running. */
export function isBusy(error: unknown): boolean {
  return error instanceof PartnerError && error.status === 409;
}

/** The draft a refused request left in the project, or "". */
export function draftOf(error: unknown): string {
  return error instanceof PartnerError ? error.draft : "";
}

type Refusal = { error?: string; install?: string; draft?: string };

/**
 * The one request. A body makes it a write, and a write is a POST of JSON;
 * everything else is a read.
 */
async function request<T>(resource: string, body?: unknown, signal?: AbortSignal): Promise<T> {
  const sending = body !== undefined;
  const response = await fetch(resource, {
    signal,
    method: sending ? "POST" : "GET",
    headers: sending
      ? { Accept: "application/json", "Content-Type": "application/json" }
      : { Accept: "application/json" },
    body: sending ? JSON.stringify(body) : undefined,
  });
  if (!response.ok) {
    const refusal = await reasonOf(response);
    throw new PartnerError(response.status, refusal.error ?? "", refusal.install ?? "", refusal.draft ?? "");
  }
  return (await response.json()) as T;
}

async function reasonOf(response: Response): Promise<Refusal> {
  try {
    return (await response.json()) as Refusal;
  } catch {
    return {};
  }
}

/** The conversation as it stands: read on load and on every reconnect. */
export async function loadPartner(signal?: AbortSignal): Promise<Snapshot> {
  return request<Snapshot>(PARTNER, undefined, signal);
}

/**
 * Ask one question. The key is the page's own, so the same send twice is the
 * same turn once and a retry after a lost answer never asks twice.
 */
export async function sendTurn(key: string, text: string, about: Page): Promise<{ turn: string }> {
  return request<{ turn: string }>(TURNS, { key, text, about });
}

/**
 * What the Partner will be given for this capture.
 *
 * It is composed by the server, through the same composer the turn's own block
 * goes through, so the sheet is the block rather than a second rendering of
 * it. It speaks of the NEXT question: nothing has been sent, and reading this
 * sends nothing.
 */
export async function seeing(about: Page): Promise<Seeing> {
  return request<Seeing>(SEEING, { about });
}

/**
 * Start a sitting on one record — an existing one by its path, or a draft the
 * server creates now under the title given — and read back the conversation with
 * the sitting on it and its opening turn already in the transcript.
 *
 * It answers the whole conversation rather than the sitting alone because the
 * opening turn is a turn this page did not send: the server asked it, on the
 * human's behalf, and the page has to be handed the transcript that holds it.
 */
export async function startSitting(asked: {
  purpose: string;
  subject?: Subject;
  title?: string;
  about: Page;
}): Promise<Snapshot> {
  return request<Snapshot>(SITTING, asked);
}

/**
 * Ask the Partner to draft the sitting's closing deposit, and read back the
 * conversation with that turn already in the transcript (g1-s55 D2).
 *
 * It ends nothing. The sitting stands until the human has recorded the outcome
 * or has said they are leaving without it, because the card this turn offers is
 * admitted against the sitting's subject.
 */
export async function closeSitting(about: Page): Promise<Snapshot> {
  return request<Snapshot>(SITTING_CLOSE, { about });
}

/** End the sitting. What was recorded stays in the record. */
export async function endSitting(): Promise<Snapshot> {
  return request<Snapshot>(SITTING_END, {});
}

/** Stop the running turn, and read back what it settled as. */
export async function stopTurn(turn: string): Promise<Snapshot> {
  return request<Snapshot>(`${TURNS}/${encodeURIComponent(turn)}${STOP}`, {});
}

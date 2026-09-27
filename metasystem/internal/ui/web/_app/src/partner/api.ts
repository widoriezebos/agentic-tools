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
/** One proposed action of one turn, for the state a press writes onto it. */
const PROPOSALS = "/proposals/";

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

/**
 * The six states one proposed action passes through.
 *
 * Waiting is the offer. Applying is written BEFORE the act is sent, so a reload
 * during a run shows the line that was in flight as having been in flight rather
 * than as fresh. Applied, refused and unresolved are what the act answered, in
 * the act layer's own three distinctions. Dismissed is the human putting it away.
 */
export type ProposalState = "waiting" | "applying" | "applied" | "refused" | "unresolved" | "dismissed";

/** The goal as the Partner read it, on the three acts whose meaning depends on it. */
export type ProposalRead = {
  intent: string;
  nextStep: string;
  tier: number;
  labels: string[];
  /**
   * The goal's live dependents as the Partner read them: the goals whose own
   * blockers name it, in id order, from the observation the answer was composed
   * against. It is carried on an abandon and absent everywhere else, which is
   * why it is optional rather than an empty list — a goal nothing waits for
   * sends no list at all.
   *
   * The line says how many goals wait for the goal and what becomes of them, so
   * the press is an informed one; and the runner refuses a line whose dependents
   * have changed since, because a goal that has grown or lost a dependent is a
   * different abandon from the one the human read (g1-s64 D2, D4).
   */
  dependents?: string[];
};

/**
 * One act the Partner proposed on one goal, before the human has applied it.
 *
 * It is an offer and nothing else: nothing was published, and the press on the
 * card is the act, under the human's own sign-in. The verb is the route id the
 * runner dispatches on — never a word on a button, so a page renamed leaves
 * every persisted proposal alone — and the fields are that route's own body
 * fields under the body's own names.
 */
export type Proposal = {
  /** This action's place in its answer, which is how the outcome route names it. */
  index: number;
  verb: string;
  goal: string;
  /** The subject as the pages say it, stamped by the server from the tip. */
  title: string;
  fields?: Record<string, string> | null;
  /** The goal as it was read, on an approve and an edit, and null otherwise. */
  read?: ProposalRead | null;
  /** The Partner's own words for this action, shown as its words. */
  why?: string;
  /** Whether the human was shown it as something to apply. */
  offered: boolean;
  /** Why it was not offered, in the words a human reads, or "" for one that was. */
  reason?: string;
  state: ProposalState;
  /** What the last state change said: the engine's own sentence on a refusal. */
  words?: string;
  /**
   * When this action was PROPOSED: the instant the server admitted it, stamped
   * once and never again, so every list that dates a proposal dates the asking.
   */
  at: string;
  /**
   * When this entry was last written, and absent on one nothing has written since
   * it was admitted. It is what `at` used to carry, kept beside it rather than in
   * place of it: both facts are true of a line proposed on Monday and refused on
   * Friday.
   */
  updatedAt?: string;
  /**
   * How many times this entry has been written, the server's own admission
   * counting as the first.
   *
   * Every press sends the version it last rendered, and the server admits the
   * write only against it. A press that loses is handed the entry as it stands
   * and must show it, which is what keeps two tabs from applying one act twice.
   */
  version: number;
  /**
   * The attempt that OWNS this line: the token of the press that last moved it
   * to `applying`, as the server stored it beside the version, and absent on a
   * line no press has moved.
   *
   * The page holds it and never compares it. What compares it is the route: a
   * settle write carries the attempt its own run minted, and a write whose
   * attempt is not the one the entry holds is refused with the entry as it
   * stands, because another press owns the line now (refusal code `attempt`).
   * The page's part is to carry one token per press and to show what comes back.
   */
  attempt?: string;
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
  /**
   * The acts this answer proposed on goals, each with the state it now stands in.
   *
   * This is the record of what happened, which is why it is here and not in a
   * store of the page's own: an approve applied twice is two approval records, so
   * the one thing the page must never do is forget that a line was applied.
   */
  proposals?: Proposal[] | null;
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
  /**
   * The acts the running turn has proposed so far, so a reload mid-answer shows
   * the card that is filling rather than losing the lines that have arrived.
   */
  proposals: Proposal[] | null;
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
  | "proposal"
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
  /**
   * One admitted or refused action, on a proposal beat and nowhere else. It
   * arrives as the server admits it, so the card fills line by line while the
   * answer is still arriving.
   */
  proposal?: Proposal;
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
  /**
   * The proposed action as the server holds it, where a write lost a race for
   * one, and null otherwise. It is the whole remedy: the caller shows this
   * rather than what it had, and sends no act.
   */
  readonly held: Proposal | null;

  constructor(status: number, reason: string, install = "", draft = "", held: Proposal | null = null) {
    super(reason === "" ? `the Partner answered ${String(status)}` : reason);
    this.name = "PartnerError";
    this.status = status;
    this.install = install;
    this.draft = draft;
    this.held = held;
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

type Refusal = { error?: string; install?: string; draft?: string; code?: string; proposal?: Proposal };

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
    throw new PartnerError(response.status, refusal.error ?? "", refusal.install ?? "", refusal.draft ?? "",
      heldIn(refusal));
  }
  return (await response.json()) as T;
}

/**
 * The entry a refusal carries, and null where it carries none.
 *
 * Two codes carry one, and both mean the same thing to the caller: show the entry
 * that came back and send no act of your own. `state` is the version
 * compare-and-set, or a transition the line may not make. `attempt` is the write
 * of a press that no longer owns the line — another press moved it to `applying`
 * and owns it now — and the entry it carries is what that press left.
 */
function heldIn(refusal: Refusal): Proposal | null {
  return refusal.code === "state" || refusal.code === "attempt" ? (refusal.proposal ?? null) : null;
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

/**
 * What one entry looks like when a write loses: the entry as it stands, which the
 * caller must show instead of what it had.
 */
export class ProposalConflict extends Error {
  readonly proposal: Proposal;

  constructor(reason: string, proposal: Proposal) {
    super(reason);
    this.name = "ProposalConflict";
    this.proposal = proposal;
  }
}

/**
 * Write one state onto one action the Partner proposed, and read back the entry
 * as it now stands with the conversation beside it.
 *
 * The version is the entry's own as this page last rendered it. The server admits
 * the write only against that version and only as a transition the line may make;
 * a write it refuses comes back as a ProposalConflict carrying the entry, and the
 * caller must show that rather than send an act of its own. That is what makes two
 * tabs unable to apply one act twice.
 *
 * The attempt is the PRESS's own, one token for a whole run. A write that moves the
 * line to `applying` takes ownership of it under that token; a settle write must
 * carry the attempt the entry holds, and one that does not is refused with the
 * entry, because the press that owns the line is the only one that may settle it.
 * A dismissal carries none.
 *
 * It carries no authority and makes no act: the act itself goes to the ledger's
 * own route, under the human's session. This writes down what that answered, where
 * the proposal is.
 */
export async function recordProposal(
  turn: string,
  index: number,
  version: number,
  state: ProposalState,
  words: string,
  attempt: string,
): Promise<{ proposal: Proposal } & Snapshot> {
  const resource = `${TURNS}/${encodeURIComponent(turn)}${PROPOSALS}${String(index)}`;
  const body: { version: number; state: ProposalState; words: string; attempt?: string } =
    { version, state, words };
  // A dismissal carries none, and that is the difference the route reads: nothing
  // was applied, so no press owns the line, and the version alone decides who
  // writes it — exactly as it did before there were attempts at all.
  if (attempt !== "") {
    body.attempt = attempt;
  }
  try {
    return await request<{ proposal: Proposal } & Snapshot>(resource, body);
  } catch (error: unknown) {
    throw conflictOf(error);
  }
}

/**
 * The refusal that carries an entry, told from every other by its code. A
 * refusal without one is what it was: a PartnerError the caller shows.
 */
function conflictOf(error: unknown): unknown {
  if (!(error instanceof PartnerError) || error.status !== 409 || error.held === null) {
    return error;
  }
  return new ProposalConflict(error.message, error.held);
}
